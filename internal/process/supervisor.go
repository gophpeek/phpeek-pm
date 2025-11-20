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
	"github.com/gophpeek/phpeek-pm/internal/hooks"
	"github.com/gophpeek/phpeek-pm/internal/logger"
	"github.com/gophpeek/phpeek-pm/internal/metrics"
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
	name             string
	config           *config.Process
	globalConfig     *config.GlobalConfig
	logger           *slog.Logger
	instances        []*Instance
	state            ProcessState
	healthMonitor    *HealthMonitor
	healthStatus     <-chan HealthStatus
	lastHealthStatus *HealthStatus // Last received health status
	ready            bool          // Whether process has passed readiness checks
	restartPolicy    RestartPolicy
	deathNotifier    func(string) // Callback when all instances are dead
	ctx              context.Context
	cancel           context.CancelFunc
	mu               sync.RWMutex
}

// Instance represents a single process instance
type Instance struct {
	id           string
	cmd          *exec.Cmd
	state        ProcessState
	pid          int
	started      time.Time
	restartCount int
	stdoutWriter *logger.ProcessWriter
	stderrWriter *logger.ProcessWriter
	mu           sync.RWMutex
}

// NewSupervisor creates a new process supervisor
func NewSupervisor(name string, cfg *config.Process, globalCfg *config.GlobalConfig, logger *slog.Logger) *Supervisor {
	// Get restart backoff and max attempts from global config
	backoff := time.Duration(globalCfg.RestartBackoff) * time.Second
	maxAttempts := globalCfg.MaxRestartAttempts

	return &Supervisor{
		name:          name,
		config:        cfg,
		globalConfig:  globalCfg,
		logger:        logger.With("process", name),
		instances:     make([]*Instance, 0, cfg.Scale),
		state:         StateStopped,
		restartPolicy: NewRestartPolicy(cfg.Restart, maxAttempts, backoff),
	}
}

// SetDeathNotifier sets the callback for when all instances are dead
func (s *Supervisor) SetDeathNotifier(notifier func(string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deathNotifier = notifier
}

// Start starts all instances of the process
func (s *Supervisor) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create context for this supervisor's lifetime
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.state = StateStarting

	// Start instances based on scale
	for i := 0; i < s.config.Scale; i++ {
		instanceID := fmt.Sprintf("%s-%d", s.name, i)

		instance, err := s.startInstance(s.ctx, instanceID)
		if err != nil {
			s.state = StateFailed
			s.cancel()
			return fmt.Errorf("failed to start instance %s: %w", instanceID, err)
		}

		s.instances = append(s.instances, instance)
	}

	s.state = StateRunning

	// Start health monitoring if configured
	if s.config.HealthCheck != nil {
		monitor, err := NewHealthMonitor(s.name, s.config.HealthCheck, s.logger)
		if err != nil {
			s.logger.Warn("Failed to create health monitor",
				"error", err,
			)
		} else {
			s.healthMonitor = monitor
			s.healthStatus = monitor.Start(s.ctx)

			// Monitor health status in background
			go s.handleHealthStatus(s.ctx)
		}
	}

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
	stdoutWriter, err := logger.NewProcessWriter(s.logger, instanceID, "stdout", s.config.Logging)
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout writer: %w", err)
	}
	cmd.Stdout = stdoutWriter

	stderrWriter, err := logger.NewProcessWriter(s.logger, instanceID, "stderr", s.config.Logging)
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr writer: %w", err)
	}
	cmd.Stderr = stderrWriter

	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	startTime := time.Now()
	instance := &Instance{
		id:           instanceID,
		cmd:          cmd,
		state:        StateRunning,
		pid:          cmd.Process.Pid,
		started:      startTime,
		stdoutWriter: stdoutWriter,
		stderrWriter: stderrWriter,
	}

	s.logger.Info("Process instance started",
		"instance_id", instanceID,
		"pid", instance.pid,
	)

	// Record metrics
	metrics.RecordProcessStart(s.name, instanceID, float64(startTime.Unix()))

	// Monitor process in background
	go s.monitorInstance(instance)

	return instance, nil
}

// monitorInstance monitors a process instance and handles restarts
func (s *Supervisor) monitorInstance(instance *Instance) {
	err := instance.cmd.Wait()

	// CRITICAL: Flush any remaining buffered output before marking as stopped
	// This prevents data loss from incomplete line buffers or multiline buffers
	if instance.stdoutWriter != nil {
		instance.stdoutWriter.Flush()
	}
	if instance.stderrWriter != nil {
		instance.stderrWriter.Flush()
	}

	instance.mu.Lock()
	exitCode := instance.cmd.ProcessState.ExitCode()
	instance.state = StateStopped
	restartCount := instance.restartCount
	instance.mu.Unlock()

	// Record process stop metrics
	metrics.RecordProcessStop(s.name, instance.id, exitCode)

	if err != nil {
		s.logger.Error("Process instance exited with error",
			"instance_id", instance.id,
			"exit_code", exitCode,
			"restart_count", restartCount,
			"error", err,
		)
	} else {
		s.logger.Info("Process instance exited",
			"instance_id", instance.id,
			"exit_code", exitCode,
			"restart_count", restartCount,
		)
	}

	// Check if we should restart
	if s.restartPolicy.ShouldRestart(exitCode, restartCount) {
		backoff := s.restartPolicy.BackoffDuration(restartCount)

		// Determine restart reason
		restartReason := "crash"
		if exitCode == 0 {
			restartReason = "normal_exit"
		}

		s.logger.Info("Restarting process instance",
			"instance_id", instance.id,
			"backoff", backoff,
			"attempt", restartCount+1,
			"reason", restartReason,
		)

		// Record restart metric
		metrics.RecordProcessRestart(s.name, restartReason)

		// Wait for backoff period
		select {
		case <-time.After(backoff):
		case <-s.ctx.Done():
			return
		}

		// Attempt restart
		newInstance, err := s.startInstance(s.ctx, instance.id)
		if err != nil {
			s.logger.Error("Failed to restart process instance",
				"instance_id", instance.id,
				"error", err,
			)
			return
		}

		// Update restart count
		newInstance.mu.Lock()
		newInstance.restartCount = restartCount + 1
		newInstance.mu.Unlock()

		// Replace old instance with new one
		s.mu.Lock()
		for i, inst := range s.instances {
			if inst.id == instance.id {
				s.instances[i] = newInstance
				break
			}
		}
		s.mu.Unlock()
	} else {
		s.logger.Warn("Process instance will not be restarted",
			"instance_id", instance.id,
			"exit_code", exitCode,
			"restart_count", restartCount,
		)

		// Check if all instances are now dead
		s.checkAllInstancesDead()
	}
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

// IsReady returns whether the process has passed its readiness checks
func (s *Supervisor) IsReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ready
}

// WaitForReady blocks until the process becomes ready (passes health checks)
// or the context times out. This is used for readiness checks during startup.
func (s *Supervisor) WaitForReady(ctx context.Context, timeout time.Duration) error {
	s.mu.RLock()
	hc := s.config.HealthCheck
	s.mu.RUnlock()

	// If no health check configured, consider immediately ready
	if hc == nil {
		s.logger.Debug("No health check configured, considering process ready immediately")
		return nil
	}

	// If health check is not in readiness mode, consider immediately ready
	if hc.Mode != "readiness" && hc.Mode != "both" {
		s.logger.Debug("Health check not configured for readiness, considering process ready immediately",
			"mode", hc.Mode,
		)
		return nil
	}

	s.logger.Info("Waiting for process to become ready",
		"timeout", timeout,
		"initial_delay", hc.InitialDelay,
	)

	// Create timeout context
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Poll for readiness status
	ticker := time.NewTicker(500 * time.Millisecond) // Check every 500ms
	defer ticker.Stop()

	for {
		// Check if process is ready
		s.mu.RLock()
		isReady := s.ready
		lastStatus := s.lastHealthStatus
		s.mu.RUnlock()

		if isReady {
			s.logger.Info("Process is now ready")
			return nil
		}

		// Log current status for debugging
		if lastStatus != nil && !lastStatus.Healthy {
			s.logger.Debug("Process not yet ready, waiting for health check to pass",
				"error", lastStatus.Error,
			)
		}

		// Wait for next check or timeout
		select {
		case <-ticker.C:
			// Continue checking
			continue
		case <-waitCtx.Done():
			s.mu.RLock()
			finalStatus := s.lastHealthStatus
			s.mu.RUnlock()

			if finalStatus != nil && finalStatus.Error != nil {
				return fmt.Errorf("timeout waiting for process to become ready after %v: last error: %w", timeout, finalStatus.Error)
			}
			return fmt.Errorf("timeout waiting for process to become ready after %v", timeout)
		}
	}
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

	// Execute per-process pre-stop hook if configured
	if s.config.Shutdown != nil && s.config.Shutdown.PreStopHook != nil {
		s.logger.Info("Executing pre-stop hook",
			"instance_id", instance.id,
			"hook", s.config.Shutdown.PreStopHook.Name,
		)

		hookExecutor := hooks.NewExecutor(s.logger)
		if err := hookExecutor.ExecuteWithType(ctx, s.config.Shutdown.PreStopHook, "pre_stop"); err != nil {
			s.logger.Warn("Pre-stop hook failed",
				"instance_id", instance.id,
				"error", err,
			)
			// Continue with shutdown even if hook fails
		}
	}

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

// handleHealthStatus monitors health check results and handles failures
func (s *Supervisor) handleHealthStatus(ctx context.Context) {
	for {
		select {
		case status, ok := <-s.healthStatus:
			if !ok {
				return
			}

			// Store the latest health status
			s.mu.Lock()
			s.lastHealthStatus = &status
			// Mark as ready if health check ACTUALLY succeeds and in readiness mode
			// Use LastCheckSucceeded to distinguish actual success from "not failed threshold yet"
			if status.LastCheckSucceeded && s.config.HealthCheck != nil &&
				(s.config.HealthCheck.Mode == "readiness" || s.config.HealthCheck.Mode == "both") {
				if !s.ready {
					s.logger.Info("Process passed readiness check")
					s.ready = true
				}
			}
			s.mu.Unlock()

			if !status.Healthy {
				s.logger.Error("Process unhealthy, triggering restart",
					"error", status.Error,
				)

				// Record health check restart
				metrics.RecordProcessRestart(s.name, "health_check")

				// Restart all instances that are currently running
				s.mu.Lock()
				for _, instance := range s.instances {
					instance.mu.Lock()
					if instance.state == StateRunning {
						s.logger.Info("Health check failure - restarting instance",
							"instance_id", instance.id,
							"pid", instance.pid,
						)

						// Kill the unhealthy instance
						if instance.cmd.Process != nil {
							_ = instance.cmd.Process.Kill()
						}

						// The monitorInstance goroutine will handle the restart
						// based on the restart policy
					}
					instance.mu.Unlock()
				}
				s.mu.Unlock()
			}
		case <-ctx.Done():
			return
		}
	}
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

// GetState returns the current supervisor state
func (s *Supervisor) GetState() ProcessState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// InstanceInfo represents exported instance information
type InstanceInfo struct {
	ID           string
	State        string
	PID          int
	StartedAt    int64
	RestartCount int
}

// GetInstances returns information about all instances
func (s *Supervisor) GetInstances() []InstanceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instances := make([]InstanceInfo, 0, len(s.instances))
	for _, inst := range s.instances {
		inst.mu.RLock()
		instances = append(instances, InstanceInfo{
			ID:           inst.id,
			State:        string(inst.state),
			PID:          inst.pid,
			StartedAt:    inst.started.Unix(),
			RestartCount: inst.restartCount,
		})
		inst.mu.RUnlock()
	}

	return instances
}

// checkAllInstancesDead checks if all instances are dead and notifies manager
func (s *Supervisor) checkAllInstancesDead() {
	s.mu.RLock()
	allDead := true
	for _, inst := range s.instances {
		inst.mu.RLock()
		if inst.state == StateRunning {
			allDead = false
		}
		inst.mu.RUnlock()
		if !allDead {
			break
		}
	}
	notifier := s.deathNotifier
	s.mu.RUnlock()

	if allDead && len(s.instances) > 0 && notifier != nil {
		s.logger.Debug("All instances dead, notifying manager")
		notifier(s.name)
	}
}
