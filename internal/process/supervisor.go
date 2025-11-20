package process

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/gophpeek/phpeek-pm/internal/config"
	"github.com/gophpeek/phpeek-pm/internal/logger"
)

// ProcessState represents the state of a process
type ProcessState string

const (
	StateStarting ProcessState = "starting"
	StateRunning  ProcessState = "running"
	StateStopping ProcessState = "stopping"
	StateStopped  ProcessState = "stopped"
	StateFailed   ProcessState = "failed"
)

// Supervisor supervises a single process (potentially with multiple instances)
type Supervisor struct {
	name      string
	config    *config.Process
	logger    *slog.Logger
	instances []*Instance
	state     ProcessState
	mu        sync.RWMutex
}

// Instance represents a single process instance
type Instance struct {
	id      string
	cmd     *exec.Cmd
	state   ProcessState
	pid     int
	started time.Time
	mu      sync.RWMutex
}

// NewSupervisor creates a new process supervisor
func NewSupervisor(name string, cfg *config.Process, logger *slog.Logger) *Supervisor {
	return &Supervisor{
		name:      name,
		config:    cfg,
		logger:    logger.With("process", name),
		instances: make([]*Instance, 0, cfg.Scale),
		state:     StateStopped,
	}
}

// Start starts all instances of the process
func (s *Supervisor) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = StateStarting

	// Start instances based on scale
	for i := 0; i < s.config.Scale; i++ {
		instanceID := fmt.Sprintf("%s-%d", s.name, i)

		instance, err := s.startInstance(ctx, instanceID)
		if err != nil {
			s.state = StateFailed
			return fmt.Errorf("failed to start instance %s: %w", instanceID, err)
		}

		s.instances = append(s.instances, instance)
	}

	s.state = StateRunning
	return nil
}

// startInstance starts a single process instance
func (s *Supervisor) startInstance(ctx context.Context, instanceID string) (*Instance, error) {
	s.logger.Info("Starting process instance",
		"instance_id", instanceID,
		"command", s.config.Command,
	)

	// Create command
	cmd := exec.CommandContext(ctx, s.config.Command[0], s.config.Command[1:]...)

	// Set environment variables
	cmd.Env = append(os.Environ(), s.envVars(instanceID)...)

	// Setup stdout/stderr capture with structured logging
	cmd.Stdout = &logger.ProcessWriter{
		Logger:     s.logger,
		InstanceID: instanceID,
		Stream:     "stdout",
	}
	cmd.Stderr = &logger.ProcessWriter{
		Logger:     s.logger,
		InstanceID: instanceID,
		Stream:     "stderr",
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	instance := &Instance{
		id:      instanceID,
		cmd:     cmd,
		state:   StateRunning,
		pid:     cmd.Process.Pid,
		started: time.Now(),
	}

	s.logger.Info("Process instance started",
		"instance_id", instanceID,
		"pid", instance.pid,
	)

	// Monitor process in background
	go s.monitorInstance(instance)

	return instance, nil
}

// monitorInstance monitors a process instance and handles restarts
func (s *Supervisor) monitorInstance(instance *Instance) {
	err := instance.cmd.Wait()

	instance.mu.Lock()
	exitCode := instance.cmd.ProcessState.ExitCode()
	instance.state = StateStopped
	instance.mu.Unlock()

	if err != nil {
		s.logger.Error("Process instance exited with error",
			"instance_id", instance.id,
			"exit_code", exitCode,
			"error", err,
		)
	} else {
		s.logger.Info("Process instance exited",
			"instance_id", instance.id,
			"exit_code", exitCode,
		)
	}

	// TODO: Handle restart policy in Phase 2
	// For now, we just log the exit
}

// Stop gracefully stops all process instances
func (s *Supervisor) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = StateStopping

	var wg sync.WaitGroup
	errChan := make(chan error, len(s.instances))

	// Stop all instances
	for _, instance := range s.instances {
		wg.Add(1)
		go func(inst *Instance) {
			defer wg.Done()

			if err := s.stopInstance(ctx, inst); err != nil {
				errChan <- fmt.Errorf("instance %s: %w", inst.id, err)
			}
		}(instance)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	s.state = StateStopped

	if len(errs) > 0 {
		return fmt.Errorf("stop completed with %d errors: %v", len(errs), errs)
	}

	return nil
}

// stopInstance stops a single process instance
func (s *Supervisor) stopInstance(ctx context.Context, instance *Instance) error {
	instance.mu.Lock()
	if instance.state != StateRunning {
		instance.mu.Unlock()
		return nil
	}
	instance.state = StateStopping
	instance.mu.Unlock()

	s.logger.Info("Stopping process instance",
		"instance_id", instance.id,
		"pid", instance.pid,
	)

	// Get shutdown signal (default SIGTERM)
	sig := syscall.SIGTERM
	if s.config.Shutdown != nil && s.config.Shutdown.Signal != "" {
		sig = parseSignal(s.config.Shutdown.Signal)
	}

	// Send shutdown signal
	if err := instance.cmd.Process.Signal(sig); err != nil {
		return fmt.Errorf("failed to send signal: %w", err)
	}

	// Wait for graceful shutdown with timeout
	timeout := 30 * time.Second
	if s.config.Shutdown != nil && s.config.Shutdown.Timeout > 0 {
		timeout = time.Duration(s.config.Shutdown.Timeout) * time.Second
	}

	done := make(chan error, 1)
	go func() {
		done <- instance.cmd.Wait()
	}()

	select {
	case <-done:
		s.logger.Info("Process instance stopped gracefully",
			"instance_id", instance.id,
		)
		return nil

	case <-time.After(timeout):
		s.logger.Warn("Process instance did not stop gracefully, force killing",
			"instance_id", instance.id,
			"timeout", timeout,
		)

		// Force kill
		if err := instance.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}

		<-done // Wait for process to actually exit
		return nil
	}
}

// envVars returns environment variables for a process instance
func (s *Supervisor) envVars(instanceID string) []string {
	envs := make([]string, 0, len(s.config.Env)+2)

	// Add configured environment variables
	for key, value := range s.config.Env {
		envs = append(envs, fmt.Sprintf("%s=%s", key, value))
	}

	// Add instance-specific variables
	envs = append(envs,
		fmt.Sprintf("PHPEEK_PM_PROCESS_NAME=%s", s.name),
		fmt.Sprintf("PHPEEK_PM_INSTANCE_ID=%s", instanceID),
	)

	return envs
}

// parseSignal converts signal name to syscall.Signal
func parseSignal(name string) syscall.Signal {
	switch name {
	case "SIGTERM":
		return syscall.SIGTERM
	case "SIGQUIT":
		return syscall.SIGQUIT
	case "SIGINT":
		return syscall.SIGINT
	case "SIGKILL":
		return syscall.SIGKILL
	default:
		return syscall.SIGTERM
	}
}
