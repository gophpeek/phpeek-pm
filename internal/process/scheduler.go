package process

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"sync"
	"time"

	"github.com/gophpeek/phpeek-pm/internal/config"
	"github.com/gophpeek/phpeek-pm/internal/logger"
	"github.com/gophpeek/phpeek-pm/internal/metrics"
	"github.com/robfig/cron/v3"
)

// Scheduler manages scheduled (cron-like) task execution
type Scheduler struct {
	name         string
	config       *config.Process
	logger       *slog.Logger
	cron         *cron.Cron
	heartbeat    *HeartbeatClient
	lastRun      time.Time
	lastDuration time.Duration
	lastExitCode int
	runCount     int64
	successCount int64
	failureCount int64
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewScheduler creates a new scheduler for a scheduled process
func NewScheduler(name string, cfg *config.Process, logger *slog.Logger) (*Scheduler, error) {
	// Validate schedule expression
	if cfg.Schedule == "" {
		return nil, fmt.Errorf("schedule expression is required for scheduled processes")
	}

	// Parse schedule to validate it (standard 5-field cron format)
	_, err := cron.ParseStandard(cfg.Schedule)
	if err != nil {
		return nil, fmt.Errorf("invalid schedule expression %q: %w", cfg.Schedule, err)
	}

	// Create heartbeat client if configured
	var heartbeatClient *HeartbeatClient
	if cfg.Heartbeat != nil {
		heartbeatClient = NewHeartbeatClient(cfg.Heartbeat, logger.With("component", "heartbeat"))
	}

	return &Scheduler{
		name:      name,
		config:    cfg,
		logger:    logger.With("process", name, "type", "scheduled"),
		cron:      cron.New(), // Standard 5-field cron format (minute, hour, day, month, weekday)
		heartbeat: heartbeatClient,
	}, nil
}

// Start starts the scheduler
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	// Add the scheduled job
	_, err := s.cron.AddFunc(s.config.Schedule, func() {
		s.runScheduledTask()
	})
	if err != nil {
		return fmt.Errorf("failed to add scheduled task: %w", err)
	}

	s.logger.Info("Scheduler started",
		"schedule", s.config.Schedule,
		"next_run", s.cron.Entries()[0].Next,
	)

	// Record metrics
	metrics.RecordScheduledTask(s.name, "started")
	metrics.RecordScheduledTaskNextRun(s.name, float64(s.cron.Entries()[0].Next.Unix()))

	// Start the cron scheduler
	s.cron.Start()

	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}

	s.logger.Info("Stopping scheduler")

	// Stop cron gracefully
	stopCtx := s.cron.Stop()

	// Wait for running jobs with timeout
	select {
	case <-stopCtx.Done():
		s.logger.Info("Scheduler stopped gracefully")
	case <-time.After(30 * time.Second):
		s.logger.Warn("Scheduler stop timeout, forcing shutdown")
	case <-ctx.Done():
		s.logger.Warn("Scheduler stop cancelled by context")
	}

	return nil
}

// runScheduledTask executes the scheduled task
func (s *Scheduler) runScheduledTask() {
	s.mu.Lock()
	s.runCount++
	runID := s.runCount
	s.mu.Unlock()

	startTime := time.Now()

	s.logger.Info("Scheduled task starting",
		"run_id", runID,
		"schedule", s.config.Schedule,
	)

	// Execute the task
	exitCode, err := s.executeTask(startTime, runID)

	duration := time.Since(startTime)

	// Update statistics
	s.mu.Lock()
	s.lastRun = startTime
	s.lastDuration = duration
	s.lastExitCode = exitCode
	if exitCode == 0 {
		s.successCount++
	} else {
		s.failureCount++
	}
	s.mu.Unlock()

	// Record metrics
	metrics.RecordScheduledTaskLastRun(s.name, float64(startTime.Unix()))
	metrics.RecordScheduledTaskLastExitCode(s.name, exitCode)

	// Update next run time metric
	entries := s.cron.Entries()
	if len(entries) > 0 {
		metrics.RecordScheduledTaskNextRun(s.name, float64(entries[0].Next.Unix()))
	}

	// Log result
	if err != nil || exitCode != 0 {
		s.logger.Error("Scheduled task failed",
			"run_id", runID,
			"exit_code", exitCode,
			"duration", duration,
			"error", err,
		)

		// Send failure heartbeat
		if s.heartbeat != nil {
			_ = s.heartbeat.PingFailure(s.ctx, fmt.Sprintf("exit_code=%d", exitCode))
		}

		// Record failure metric
		metrics.RecordScheduledTask(s.name, "failed")
	} else {
		s.logger.Info("Scheduled task completed",
			"run_id", runID,
			"duration", duration,
		)

		// Send success heartbeat
		if s.heartbeat != nil {
			_ = s.heartbeat.PingSuccess(s.ctx)
		}

		// Record success metric
		metrics.RecordScheduledTask(s.name, "success")
	}

	// Record duration metric
	metrics.RecordScheduledTaskDuration(s.name, duration.Seconds())
}

// executeTask runs the actual command
func (s *Scheduler) executeTask(startTime time.Time, runID int64) (int, error) {
	// Create command with task-specific ID
	instanceID := fmt.Sprintf("%s-run-%d", s.name, runID)

	cmd := exec.CommandContext(s.ctx, s.config.Command[0], s.config.Command[1:]...)

	// Set environment variables
	cmd.Env = s.envVars(instanceID, startTime)

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

	// Start the command
	if err := cmd.Start(); err != nil {
		return -1, fmt.Errorf("failed to start command: %w", err)
	}

	s.logger.Debug("Task process started",
		"run_id", runID,
		"instance_id", instanceID,
		"pid", cmd.Process.Pid,
	)

	// Wait for completion
	err := cmd.Wait()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return exitCode, err
}

// envVars returns environment variables for the scheduled task
func (s *Scheduler) envVars(instanceID string, startTime time.Time) []string {
	envs := make([]string, 0, len(s.config.Env)+5)

	// Add configured environment variables
	for key, value := range s.config.Env {
		envs = append(envs, fmt.Sprintf("%s=%s", key, value))
	}

	// Add PHPeek-specific variables
	envs = append(envs,
		fmt.Sprintf("PHPEEK_PM_PROCESS_NAME=%s", s.name),
		fmt.Sprintf("PHPEEK_PM_INSTANCE_ID=%s", instanceID),
		"PHPEEK_PM_SCHEDULED=true",
		fmt.Sprintf("PHPEEK_PM_SCHEDULE=%s", s.config.Schedule),
		fmt.Sprintf("PHPEEK_PM_START_TIME=%d", startTime.Unix()),
	)

	return envs
}

// GetStats returns scheduler statistics
func (s *Scheduler) GetStats() SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var nextRun time.Time
	entries := s.cron.Entries()
	if len(entries) > 0 {
		nextRun = entries[0].Next
	}

	return SchedulerStats{
		Name:         s.name,
		Schedule:     s.config.Schedule,
		LastRun:      s.lastRun,
		LastDuration: s.lastDuration,
		LastExitCode: s.lastExitCode,
		NextRun:      nextRun,
		RunCount:     s.runCount,
		SuccessCount: s.successCount,
		FailureCount: s.failureCount,
	}
}

// SchedulerStats contains scheduler statistics
type SchedulerStats struct {
	Name         string
	Schedule     string
	LastRun      time.Time
	LastDuration time.Duration
	LastExitCode int
	NextRun      time.Time
	RunCount     int64
	SuccessCount int64
	FailureCount int64
}
