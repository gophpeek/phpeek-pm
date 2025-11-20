package process

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"time"

	"github.com/gophpeek/phpeek-pm/internal/config"
	"github.com/gophpeek/phpeek-pm/internal/metrics"
)

// HealthChecker defines the interface for health checks
type HealthChecker interface {
	Check(ctx context.Context) error
}

// NewHealthChecker creates appropriate health checker based on config
func NewHealthChecker(cfg *config.HealthCheck) (HealthChecker, error) {
	if cfg == nil {
		return &NoOpHealthChecker{}, nil
	}

	switch cfg.Type {
	case "tcp":
		return &TCPHealthChecker{address: cfg.Address}, nil
	case "http":
		return &HTTPHealthChecker{
			url:            cfg.URL,
			expectedStatus: cfg.ExpectedStatus,
		}, nil
	case "exec":
		return &ExecHealthChecker{command: cfg.Command}, nil
	default:
		return nil, fmt.Errorf("unknown health check type: %s", cfg.Type)
	}
}

// NoOpHealthChecker always succeeds (for processes without health checks)
type NoOpHealthChecker struct{}

func (n *NoOpHealthChecker) Check(ctx context.Context) error {
	return nil
}

// TCPHealthChecker checks if TCP port is accepting connections
type TCPHealthChecker struct {
	address string
}

func (t *TCPHealthChecker) Check(ctx context.Context) error {
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", t.address)
	if err != nil {
		return fmt.Errorf("tcp connection failed: %w", err)
	}
	conn.Close()
	return nil
}

// HTTPHealthChecker performs HTTP GET and validates status code
type HTTPHealthChecker struct {
	url            string
	expectedStatus int
}

func (h *HTTPHealthChecker) Check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", h.url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != h.expectedStatus {
		return fmt.Errorf("unexpected status code: got %d, want %d", resp.StatusCode, h.expectedStatus)
	}

	return nil
}

// ExecHealthChecker runs a command and checks exit code
type ExecHealthChecker struct {
	command []string
}

func (e *ExecHealthChecker) Check(ctx context.Context) error {
	if len(e.command) == 0 {
		return fmt.Errorf("no command specified")
	}

	cmd := exec.CommandContext(ctx, e.command[0], e.command[1:]...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("health check command failed: %w", err)
	}

	return nil
}

// HealthMonitor continuously monitors process health
type HealthMonitor struct {
	processName        string
	checker            HealthChecker
	config             *config.HealthCheck
	consecutiveFails   int
	consecutiveSuccess int
	currentlyHealthy   bool
	logger             *slog.Logger
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(processName string, cfg *config.HealthCheck, log *slog.Logger) (*HealthMonitor, error) {
	checker, err := NewHealthChecker(cfg)
	if err != nil {
		return nil, err
	}

	return &HealthMonitor{
		processName:      processName,
		checker:          checker,
		config:           cfg,
		currentlyHealthy: true, // Start optimistic
		logger:           log,
	}, nil
}

// Start begins health check monitoring
func (hm *HealthMonitor) Start(ctx context.Context) <-chan HealthStatus {
	statusCh := make(chan HealthStatus, 1)

	go func() {
		defer close(statusCh)

		// Wait for initial delay
		if hm.config != nil && hm.config.InitialDelay > 0 {
			select {
			case <-time.After(time.Duration(hm.config.InitialDelay) * time.Second):
			case <-ctx.Done():
				return
			}
		}

		ticker := time.NewTicker(time.Duration(hm.config.Period) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				status := hm.performCheck(ctx)
				statusCh <- status
			case <-ctx.Done():
				return
			}
		}
	}()

	return statusCh
}

func (hm *HealthMonitor) performCheck(ctx context.Context) HealthStatus {
	checkCtx, cancel := context.WithTimeout(ctx, time.Duration(hm.config.Timeout)*time.Second)
	defer cancel()

	// Measure health check duration
	startTime := time.Now()
	err := hm.checker.Check(checkCtx)
	duration := time.Since(startTime).Seconds()

	// Determine success threshold (default 1 if not configured)
	successThreshold := hm.config.SuccessThreshold
	if successThreshold <= 0 {
		successThreshold = 1
	}

	if err != nil {
		// Health check failed
		hm.consecutiveFails++
		hm.consecutiveSuccess = 0 // Reset success counter on failure

		// Record metrics for failure
		metrics.RecordHealthCheck(hm.processName, hm.config.Type, duration, false)
		metrics.RecordHealthCheckFailures(hm.processName, hm.consecutiveFails)

		hm.logger.Warn("Health check failed",
			"consecutive_fails", hm.consecutiveFails,
			"failure_threshold", hm.config.FailureThreshold,
			"error", err,
		)

		// Mark as unhealthy only if we've exceeded failure threshold
		if hm.consecutiveFails >= hm.config.FailureThreshold {
			if hm.currentlyHealthy {
				hm.logger.Error("Process marked as unhealthy",
					"consecutive_fails", hm.consecutiveFails,
				)
				hm.currentlyHealthy = false
			}
			return HealthStatus{Healthy: false, Error: err}
		}
		// Still considered healthy until threshold reached
		return HealthStatus{Healthy: true, Error: nil}
	}

	// Health check succeeded
	hm.consecutiveSuccess++
	hm.consecutiveFails = 0 // Reset failure counter on success

	// Record metrics for success
	metrics.RecordHealthCheck(hm.processName, hm.config.Type, duration, true)
	metrics.RecordHealthCheckFailures(hm.processName, 0)

	// If currently unhealthy, require success threshold to be met before recovering
	if !hm.currentlyHealthy {
		if hm.consecutiveSuccess >= successThreshold {
			hm.logger.Info("Health check recovered",
				"consecutive_successes", hm.consecutiveSuccess,
				"success_threshold", successThreshold,
			)
			hm.currentlyHealthy = true
			return HealthStatus{Healthy: true, Error: nil}
		}
		// Still unhealthy, waiting for more successes
		hm.logger.Debug("Health check succeeded but waiting for threshold",
			"consecutive_successes", hm.consecutiveSuccess,
			"success_threshold", successThreshold,
		)
		return HealthStatus{Healthy: false, Error: nil}
	}

	// Already healthy and check succeeded
	return HealthStatus{Healthy: true, Error: nil}
}

// HealthStatus represents the result of a health check
type HealthStatus struct {
	Healthy bool
	Error   error
}
