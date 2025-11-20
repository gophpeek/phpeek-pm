package process

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gophpeek/phpeek-pm/internal/config"
)

// HeartbeatClient handles HTTP heartbeat/dead man's switch notifications
type HeartbeatClient struct {
	config *config.HeartbeatConfig
	logger *slog.Logger
	client *http.Client
}

// NewHeartbeatClient creates a new heartbeat client
func NewHeartbeatClient(cfg *config.HeartbeatConfig, logger *slog.Logger) *HeartbeatClient {
	timeout := 30 * time.Second
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Second
	}

	return &HeartbeatClient{
		config: cfg,
		logger: logger,
		client: &http.Client{
			Timeout: timeout,
			// Don't follow redirects by default
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// PingSuccess sends a success notification
func (h *HeartbeatClient) PingSuccess(ctx context.Context) error {
	if h.config.SuccessURL == "" {
		return nil
	}

	return h.ping(ctx, h.config.SuccessURL, "success", "")
}

// PingFailure sends a failure notification with error details
func (h *HeartbeatClient) PingFailure(ctx context.Context, message string) error {
	if h.config.FailureURL == "" {
		return nil
	}

	return h.ping(ctx, h.config.FailureURL, "failure", message)
}

// ping performs the actual HTTP request with retry logic
func (h *HeartbeatClient) ping(ctx context.Context, url, pingType, message string) error {
	retryCount := h.config.RetryCount
	if retryCount <= 0 {
		retryCount = 3 // Default: 3 attempts
	}

	retryDelay := time.Duration(h.config.RetryDelay) * time.Second
	if retryDelay <= 0 {
		retryDelay = 5 * time.Second // Default: 5 seconds
	}

	var lastErr error
	for attempt := 1; attempt <= retryCount; attempt++ {
		err := h.doRequest(ctx, url, message)
		if err == nil {
			if attempt > 1 {
				h.logger.Info("Heartbeat ping succeeded after retry",
					"type", pingType,
					"attempt", attempt,
				)
			}
			return nil
		}

		lastErr = err
		h.logger.Warn("Heartbeat ping failed",
			"type", pingType,
			"attempt", attempt,
			"max_attempts", retryCount,
			"error", err,
		)

		// Don't sleep on last attempt
		if attempt < retryCount {
			select {
			case <-time.After(retryDelay):
				// Continue to next attempt
			case <-ctx.Done():
				return fmt.Errorf("heartbeat ping cancelled: %w", ctx.Err())
			}
		}
	}

	return fmt.Errorf("heartbeat ping failed after %d attempts: %w", retryCount, lastErr)
}

// doRequest performs a single HTTP request
func (h *HeartbeatClient) doRequest(ctx context.Context, url, message string) error {
	// Determine HTTP method
	method := h.config.Method
	if method == "" {
		method = http.MethodPost // Default: POST
	}

	// Create request
	var body io.Reader
	if message != "" {
		body = strings.NewReader(message)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add custom headers
	if message != "" {
		req.Header.Set("Content-Type", "text/plain")
	}
	for key, value := range h.config.Headers {
		req.Header.Set(key, value)
	}

	// Add user agent
	req.Header.Set("User-Agent", "PHPeek-PM/1.0")

	// Perform request
	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	h.logger.Debug("Heartbeat ping successful",
		"url", url,
		"status", resp.StatusCode,
	)

	return nil
}

// Close closes the heartbeat client
func (h *HeartbeatClient) Close() error {
	h.client.CloseIdleConnections()
	return nil
}
