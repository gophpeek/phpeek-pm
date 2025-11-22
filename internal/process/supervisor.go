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
	StateStarting  ProcessState = "starting"
	StateRunning   ProcessState = "running"
	StateStopping  ProcessState = "stopping"
	StateStopped   ProcessState = "stopped"
	StateFailed    ProcessState = "failed"
	StateCompleted ProcessState = "completed" // For oneshot services that ran successfully
)

// Supervisor supervises a single process (potentially with multiple instances)
type Supervisor struct {
	name          string
	config        *config.Process
	logger        *slog.Logger
	instances     []*Instance
	state         ProcessState
	healthMonitor *HealthMonitor
	healthStatus  <-chan HealthStatus
	restartPolicy RestartPolicy
	deathNotifier func(string) // Callback when all instances are dead
	ctx           context.Context
	cancel        context.CancelFunc
	readinessCh   chan struct{} // Closed when service becomes ready
	isReady       bool          // Track readiness state
	mu            sync.RWMutex
}

// Instance represents a single process instance
type Instance struct {
	id           string
	cmd          *exec.Cmd
	state        ProcessState
	pid          int
	started      time.Time
	restartCount int
	mu           sync.RWMutex
}

// NewSupervisor creates a new process supervisor
func NewSupervisor(name string, cfg *config.Process, globalCfg *config.GlobalConfig, logger *slog.Logger) *Supervisor {
	// Get restart backoff from global config (default: 5 seconds)
	backoff := time.Duration(globalCfg.RestartBackoff) * time.Second

	// Get max attempts from global config (default: 3)
	maxAttempts := globalCfg.MaxRestartAttempts

	return &Supervisor{
		name:          name,
		config:        cfg,
		logger:        logger.With("process", name),
		instances:     make([]*Instance, 0, cfg.Scale),
		state:         StateStopped,
		restartPolicy: NewRestartPolicy(cfg.Restart, maxAttempts, backoff),
		readinessCh:   make(chan struct{}),
		isReady:       false,
	}
}

// SetDeathNotifier sets the callback for when all instances are dead
func (s *Supervisor) SetDeathNotifier(notifier func(string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deathNotifier = notifier
}

// MarkReadyImmediately marks the service as ready without waiting
// Used for stopped processes to prevent dependency deadlocks
func (s *Supervisor) MarkReadyImmediately() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isReady {
		s.isReady = true
		close(s.readinessCh)
		s.logger.Debug("Service marked as ready immediately (stopped state)")
	}
}

// WaitForReadiness waits for the service to become ready (health check passes)
// Returns nil when ready, error on timeout or context cancellation
func (s *Supervisor) WaitForReadiness(ctx context.Context, timeout time.Duration) error {
	// If no health check configured, consider immediately ready
	if s.config.HealthCheck == nil {
		s.logger.Debug("No health check configured, considering ready immediately")
		return nil
	}

	// Check health check mode - only wait if readiness or both
	mode := s.config.HealthCheck.Mode
	if mode != "readiness" && mode != "both" && mode != "" {
		// mode is "liveness" or unset (defaults to "both")
		if mode == "liveness" {
			s.logger.Debug("Health check mode is liveness-only, not waiting for readiness")
			return nil
		}
	}

	s.logger.Info("Waiting for service readiness",
		"timeout", timeout,
		"health_check_type", s.config.HealthCheck.Type,
	)

	// Wait for readiness with timeout
	timeoutCh := time.After(timeout)
	select {
	case <-s.readinessCh:
		s.logger.Info("Service ready")
		return nil
	case <-timeoutCh:
		return fmt.Errorf("service did not become ready within %v", timeout)
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting for readiness")
	}
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
			// Consider ready immediately if health monitor fails to start
			s.mu.Lock()
			if !s.isReady {
				s.isReady = true
				close(s.readinessCh)
			}
			s.mu.Unlock()
		} else {
			s.healthMonitor = monitor
			s.healthStatus = monitor.Start(s.ctx)

			// Monitor health status in background
			go s.handleHealthStatus(s.ctx)
		}
	} else {
		// No health check configured - consider ready immediately
		s.mu.Lock()
		if !s.isReady {
			s.isReady = true
			close(s.readinessCh)
			s.logger.Debug("No health check configured, marking as ready immediately")
		}
		s.mu.Unlock()
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

	// CRITICAL: Put subprocess in its own process group
	// This prevents Ctrl+C (SIGINT) from propagating to child processes
	// Without this, Ctrl+C kills children → manager thinks crash → restarts
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group for this process
	}

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

	startTime := time.Now()
	instance := &Instance{
		id:      instanceID,
		cmd:     cmd,
		state:   StateRunning,
		pid:     cmd.Process.Pid,
		started: startTime,
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

	instance.mu.Lock()
	exitCode := instance.cmd.ProcessState.ExitCode()
	instance.state = StateStopped
	restartCount := instance.restartCount
	instance.mu.Unlock()

	// Record process stop metrics
	metrics.RecordProcessStop(s.name, instance.id, exitCode)

	// Handle oneshot processes differently
	isOneshot := s.config.Type == "oneshot"

	if isOneshot {
		// Oneshot services don't restart - they run once and complete
		instance.mu.Lock()
		if exitCode == 0 {
			instance.state = StateCompleted
			s.logger.Info("Oneshot process completed successfully",
				"instance_id", instance.id,
			)
		} else {
			instance.state = StateFailed
			s.logger.Error("Oneshot process failed",
				"instance_id", instance.id,
				"exit_code", exitCode,
				"error", err,
			)
		}
		instance.mu.Unlock()

		// Signal readiness if completed successfully (allows dependents to proceed)
		if exitCode == 0 {
			s.mu.Lock()
			if !s.isReady {
				s.isReady = true
				close(s.readinessCh)
				s.logger.Info("Oneshot completed, signaling readiness to dependents")
			}
			s.mu.Unlock()
		}

		// Check if all instances complete/failed
		s.checkAllInstancesDead()
		return
	}

	// Longrun service handling
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

	// Check if we should restart (longrun only)
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
	// Since we use Setpgid, we need to send signal to the process group
	// This ensures all children of the process also receive the signal
	pgid, err := syscall.Getpgid(instance.pid)
	if err != nil {
		// Fallback to single process signal if we can't get pgid
		s.logger.Warn("Failed to get process group, sending signal to process only",
			"instance_id", instance.id,
			"error", err,
		)
		if err := instance.cmd.Process.Signal(sig); err != nil {
			return fmt.Errorf("failed to send signal: %w", err)
		}
	} else {
		// Send signal to entire process group (negative PID)
		if err := syscall.Kill(-pgid, sig); err != nil {
			return fmt.Errorf("failed to send signal to process group: %w", err)
		}
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

			if status.Healthy {
				// Signal readiness on first successful health check
				s.mu.Lock()
				if !s.isReady {
					s.isReady = true
					close(s.readinessCh)
					s.logger.Info("Service is ready (health check passed)")
				}
				s.mu.Unlock()
			} else if !status.Healthy {
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
