package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gophpeek/phpeek-pm/internal/process"
)

// APIClient connects to a running PHPeek PM daemon via API
type APIClient struct {
	baseURL string
	auth    string
	client  *http.Client
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL, auth string) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		auth:    auth,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ListProcesses fetches process list from API
func (c *APIClient) ListProcesses() ([]process.ProcessInfo, error) {
	url := fmt.Sprintf("%s/api/v1/processes", c.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.auth != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.auth))
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Processes []process.ProcessInfo `json:"processes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Processes, nil
}

// StartProcess starts a stopped process
func (c *APIClient) StartProcess(name string) error {
	return c.processAction(name, "start")
}

// StopProcess stops a running process
func (c *APIClient) StopProcess(name string) error {
	return c.processAction(name, "stop")
}

// RestartProcess restarts a process
func (c *APIClient) RestartProcess(name string) error {
	return c.processAction(name, "restart")
}

// ScaleProcess scales a process
func (c *APIClient) ScaleProcess(name string, desired int) error {
	url := fmt.Sprintf("%s/api/v1/processes/%s/scale", c.baseURL, name)

	body := fmt.Sprintf(`{"desired":%d}`, desired)

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.auth != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.auth))
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("scale failed: %s", string(respBody))
	}

	return nil
}

// processAction performs a process action (start/stop/restart)
func (c *APIClient) processAction(name, action string) error {
	url := fmt.Sprintf("%s/api/v1/processes/%s/%s", c.baseURL, name, action)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if c.auth != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.auth))
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s failed: %s", action, string(body))
	}

	return nil
}

// HealthCheck checks if API is reachable
func (c *APIClient) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/health", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("API not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API unhealthy: status %d", resp.StatusCode)
	}

	return nil
}
