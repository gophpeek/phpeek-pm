package schedule

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/gophpeek/phpeek-pm/internal/config"
	"github.com/gophpeek/phpeek-pm/internal/logger"
)

// ProcessExecutor executes scheduled process commands directly
type ProcessExecutor struct {
	configs    map[string]ProcessConfig          // Process name -> config
	logWriters map[string]*logger.ProcessWriter  // Process name -> combined log writer
	logger     *slog.Logger
	mu         sync.RWMutex
}

// ProcessConfig contains the configuration needed to execute a scheduled process
type ProcessConfig struct {
	Command    []string
	WorkingDir string
	Env        map[string]string
	Timeout    time.Duration          // 0 = no timeout
	Logging    *config.LoggingConfig  // Logging configuration for output capture
}

// NewProcessExecutor creates a new ProcessExecutor
func NewProcessExecutor(log *slog.Logger) *ProcessExecutor {
	return &ProcessExecutor{
		configs:    make(map[string]ProcessConfig),
		logWriters: make(map[string]*logger.ProcessWriter),
		logger:     log.With("component", "process_executor"),
	}
}

// RegisterProcess registers a process configuration for execution
// Creates a ProcessWriter for log capture if logging config is provided
func (e *ProcessExecutor) RegisterProcess(name string, cfg ProcessConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.configs[name] = cfg

	// Create ProcessWriter for log capture
	// Use "scheduled" as the instance ID since scheduled jobs run one at a time
	pw, err := logger.NewProcessWriter(e.logger, name, "scheduled", "combined", cfg.Logging)
	if err != nil {
		return fmt.Errorf("failed to create log writer for %s: %w", name, err)
	}
	e.logWriters[name] = pw

	e.logger.Debug("registered process for scheduling", "name", name, "command", cfg.Command)
	return nil
}

// UnregisterProcess removes a process configuration
func (e *ProcessExecutor) UnregisterProcess(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.configs, name)
	delete(e.logWriters, name)
	e.logger.Debug("unregistered process from scheduling", "name", name)
}

// Execute runs the process and returns when complete
func (e *ProcessExecutor) Execute(ctx context.Context, processName string) (int, error) {
	e.mu.RLock()
	cfg, exists := e.configs[processName]
	logWriter := e.logWriters[processName]
	e.mu.RUnlock()

	if !exists {
		return -1, fmt.Errorf("process %q not registered", processName)
	}

	if len(cfg.Command) == 0 {
		return -1, fmt.Errorf("process %q has no command", processName)
	}

	// Apply timeout if configured
	execCtx := ctx
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	// Create command
	cmd := exec.CommandContext(execCtx, cfg.Command[0], cfg.Command[1:]...)

	// Set working directory
	if cfg.WorkingDir != "" {
		cmd.Dir = cfg.WorkingDir
	}

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Add process name to environment
	cmd.Env = append(cmd.Env, fmt.Sprintf("PHPEEK_PM_PROCESS=%s", processName))
	cmd.Env = append(cmd.Env, "PHPEEK_PM_SCHEDULED=true")

	// Capture output using ProcessWriter for log storage
	// Also write to stdout/stderr so output appears in terminal/daemon logs
	var stdout, stderr io.Writer
	if logWriter != nil {
		stdout = io.MultiWriter(os.Stdout, logWriter)
		stderr = io.MultiWriter(os.Stderr, logWriter)
	} else {
		stdout = os.Stdout
		stderr = os.Stderr
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	e.logger.Info("executing scheduled process",
		"process", processName,
		"command", cfg.Command,
	)

	// Add lifecycle event: process started
	if logWriter != nil {
		logWriter.AddEvent("▶ Process started")
	}

	// Run the command
	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	// Flush log writer to ensure all output is captured
	if logWriter != nil {
		logWriter.Flush()
	}

	// Get exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			} else {
				exitCode = 1
			}
		} else if ctx.Err() == context.DeadlineExceeded {
			// Add lifecycle event: process timed out
			if logWriter != nil {
				logWriter.AddEvent(fmt.Sprintf("⏱ Process timed out after %v", cfg.Timeout))
			}
			e.logger.Error("scheduled process timed out",
				"process", processName,
				"timeout", cfg.Timeout,
				"duration", duration,
			)
			return -1, fmt.Errorf("process timed out after %v", cfg.Timeout)
		} else if ctx.Err() == context.Canceled {
			// Add lifecycle event: process cancelled
			if logWriter != nil {
				logWriter.AddEvent("⊘ Process cancelled")
			}
			e.logger.Warn("scheduled process cancelled",
				"process", processName,
				"duration", duration,
			)
			return -1, fmt.Errorf("process cancelled")
		} else {
			// Add lifecycle event: failed to start
			if logWriter != nil {
				logWriter.AddEvent(fmt.Sprintf("✗ Process failed to start: %v", err))
			}
			e.logger.Error("scheduled process failed to start",
				"process", processName,
				"error", err,
			)
			return -1, fmt.Errorf("failed to start process: %w", err)
		}
	}

	// Add lifecycle event: process exited
	if logWriter != nil {
		if exitCode == 0 {
			logWriter.AddEvent(fmt.Sprintf("✓ Process exited (code: %d, duration: %s)", exitCode, duration.Round(time.Millisecond)))
		} else {
			logWriter.AddEvent(fmt.Sprintf("✗ Process failed (code: %d, duration: %s)", exitCode, duration.Round(time.Millisecond)))
		}
	}

	e.logger.Info("scheduled process completed",
		"process", processName,
		"exit_code", exitCode,
		"duration", duration,
	)

	if exitCode != 0 {
		return exitCode, fmt.Errorf("process exited with code %d", exitCode)
	}

	return exitCode, nil
}

// GetLogs returns log entries for a scheduled process
// If limit > 0, returns only the most recent 'limit' entries
func (e *ProcessExecutor) GetLogs(processName string, limit int) []logger.LogEntry {
	e.mu.RLock()
	pw, exists := e.logWriters[processName]
	e.mu.RUnlock()

	if !exists || pw == nil {
		return []logger.LogEntry{}
	}

	var logs []logger.LogEntry
	if limit > 0 {
		logs = pw.GetRecentLogs(limit)
	} else {
		logs = pw.GetLogs()
	}

	// Sort by timestamp (newest first)
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Timestamp.After(logs[j].Timestamp)
	})

	return logs
}

// HasProcess checks if a process is registered with the executor
func (e *ProcessExecutor) HasProcess(processName string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, exists := e.configs[processName]
	return exists
}
