package tui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gophpeek/phpeek-pm/internal/config"
	"github.com/gophpeek/phpeek-pm/internal/logger"
	"github.com/gophpeek/phpeek-pm/internal/process"
)

// TestNewAPIClient tests client creation
func TestNewAPIClient(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		auth    string
	}{
		{
			name:    "with base URL and auth",
			baseURL: "http://localhost:9180",
			auth:    "test-token",
		},
		{
			name:    "with base URL no auth",
			baseURL: "http://localhost:9180",
			auth:    "",
		},
		{
			name:    "empty base URL",
			baseURL: "",
			auth:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewAPIClient(tt.baseURL, tt.auth)

			if client == nil {
				t.Fatal("Expected non-nil client")
			}

			if client.baseURL != tt.baseURL {
				t.Errorf("Expected baseURL %s, got %s", tt.baseURL, client.baseURL)
			}

			if client.auth != tt.auth {
				t.Errorf("Expected auth %s, got %s", tt.auth, client.auth)
			}

			if client.client == nil {
				t.Error("Expected non-nil HTTP client")
			}

			// Should fall back to TCP since no socket exists
			if client.socketPath != "" {
				t.Errorf("Expected empty socketPath for non-existent socket, got %s", client.socketPath)
			}
		})
	}
}

// TestAPIClient_getURL tests URL construction
func TestAPIClient_getURL(t *testing.T) {
	tests := []struct {
		name       string
		baseURL    string
		socketPath string
		path       string
		expected   string
	}{
		{
			name:       "TCP URL",
			baseURL:    "http://localhost:9180",
			socketPath: "",
			path:       "/api/v1/health",
			expected:   "http://localhost:9180/api/v1/health",
		},
		{
			name:       "socket URL",
			baseURL:    "",
			socketPath: "/tmp/phpeek.sock",
			path:       "/api/v1/processes",
			expected:   "http://unix/api/v1/processes",
		},
		{
			name:       "root path",
			baseURL:    "http://api:9000",
			socketPath: "",
			path:       "/",
			expected:   "http://api:9000/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &APIClient{
				baseURL:    tt.baseURL,
				socketPath: tt.socketPath,
			}

			url := client.getURL(tt.path)
			if url != tt.expected {
				t.Errorf("Expected URL %s, got %s", tt.expected, url)
			}
		})
	}
}

// TestAPIClient_ListProcesses tests fetching process list
func TestAPIClient_ListProcesses(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse interface{}
		serverStatus   int
		auth           string
		wantErr        bool
		expectedCount  int
	}{
		{
			name: "successful list",
			serverResponse: map[string]interface{}{
				"processes": []interface{}{
					map[string]interface{}{
						"name":     "php-fpm",
						"status":   "running",
						"pid":      1234,
						"uptime":   120,
						"restarts": 0,
					},
					map[string]interface{}{
						"name":     "nginx",
						"status":   "running",
						"pid":      5678,
						"uptime":   120,
						"restarts": 0,
					},
				},
			},
			serverStatus:  http.StatusOK,
			wantErr:       false,
			expectedCount: 2,
		},
		{
			name:           "empty list",
			serverResponse: map[string]interface{}{"processes": []interface{}{}},
			serverStatus:   http.StatusOK,
			wantErr:        false,
			expectedCount:  0,
		},
		{
			name:           "server error",
			serverResponse: map[string]interface{}{"error": "internal error"},
			serverStatus:   http.StatusInternalServerError,
			wantErr:        true,
		},
		{
			name:           "unauthorized",
			serverResponse: map[string]interface{}{"error": "unauthorized"},
			serverStatus:   http.StatusUnauthorized,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/processes" {
					t.Errorf("Expected path /api/v1/processes, got %s", r.URL.Path)
				}

				if r.Method != "GET" {
					t.Errorf("Expected GET method, got %s", r.Method)
				}

				// Check auth header if provided
				if tt.auth != "" {
					auth := r.Header.Get("Authorization")
					expected := "Bearer " + tt.auth
					if auth != expected {
						t.Errorf("Expected Authorization header %s, got %s", expected, auth)
					}
				}

				w.WriteHeader(tt.serverStatus)
				json.NewEncoder(w).Encode(tt.serverResponse)
			}))
			defer server.Close()

			client := NewAPIClient(server.URL, tt.auth)
			processes, err := client.ListProcesses()

			if (err != nil) != tt.wantErr {
				t.Errorf("ListProcesses() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(processes) != tt.expectedCount {
					t.Errorf("Expected %d processes, got %d", tt.expectedCount, len(processes))
				}
			}
		})
	}
}

func TestAPIClient_GetLogs(t *testing.T) {
	expectedLogs := []logger.LogEntry{
		{
			ProcessName: "app",
			InstanceID:  "app-0",
			Stream:      "stdout",
			Message:     "hello",
			Level:       "info",
			Timestamp:   time.Unix(1700000000, 0),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/processes/app/logs" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("limit") != "50" {
			t.Fatalf("expected limit=50, got %s", r.URL.Query().Get("limit"))
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("expected auth header")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"process": "app",
			"logs":    expectedLogs,
		})
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "token")
	logs, err := client.GetLogs("app", 50)
	if err != nil {
		t.Fatalf("GetLogs returned error: %v", err)
	}
	if len(logs) != 1 || logs[0].Message != "hello" || logs[0].ProcessName != "app" {
		t.Fatalf("unexpected logs response: %#v", logs)
	}
}

func TestAPIClient_GetStackLogs(t *testing.T) {
	expectedLogs := []logger.LogEntry{
		{
			ProcessName: "stack",
			InstanceID:  "stack-1",
			Stream:      "stderr",
			Message:     "world",
			Level:       "error",
			Timestamp:   time.Unix(1700000100, 0),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/logs" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"scope": "stack",
			"logs":  expectedLogs,
		})
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "")
	logs, err := client.GetStackLogs(0)
	if err != nil {
		t.Fatalf("GetStackLogs returned error: %v", err)
	}
	if len(logs) != 1 || logs[0].Message != "world" {
		t.Fatalf("unexpected stack logs: %#v", logs)
	}
}

func TestAPIClient_DeleteProcess(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/processes/app" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "")
	if err := client.DeleteProcess("app"); err != nil {
		t.Fatalf("DeleteProcess returned error: %v", err)
	}
	if !called {
		t.Fatal("server was not called")
	}
}

func TestAPIClient_UpdateProcess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}
		var payload struct {
			Process config.Process `json:"process"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if payload.Process.Scale != 2 {
			t.Fatalf("unexpected payload: %+v", payload.Process)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "")
	proc := &config.Process{
		Enabled: true,
		Command: []string{"php"},
		Scale:   2,
		Restart: "always",
	}
	if err := client.UpdateProcess("app", proc); err != nil {
		t.Fatalf("UpdateProcess returned error: %v", err)
	}
}

func TestAPIClient_GetProcessConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/processes/app" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"process": "app",
			"config": map[string]interface{}{
				"enabled": true,
				"command": []string{"php"},
				"scale":   3,
				"restart": "on-failure",
			},
		})
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "")
	cfg, err := client.GetProcessConfig("app")
	if err != nil {
		t.Fatalf("GetProcessConfig returned error: %v", err)
	}
	if cfg.Scale != 3 || cfg.Restart != "on-failure" || len(cfg.Command) != 1 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

// TestAPIClient_StartProcess tests starting a process
func TestAPIClient_StartProcess(t *testing.T) {
	tests := []struct {
		name         string
		processName  string
		serverStatus int
		wantErr      bool
	}{
		{
			name:         "successful start",
			processName:  "php-fpm",
			serverStatus: http.StatusOK,
			wantErr:      false,
		},
		{
			name:         "accepted status",
			processName:  "nginx",
			serverStatus: http.StatusAccepted,
			wantErr:      false,
		},
		{
			name:         "process not found",
			processName:  "unknown",
			serverStatus: http.StatusNotFound,
			wantErr:      true,
		},
		{
			name:         "already running",
			processName:  "php-fpm",
			serverStatus: http.StatusBadRequest,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := "/api/v1/processes/" + tt.processName + "/start"
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
				}

				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}

				w.WriteHeader(tt.serverStatus)
				if tt.wantErr {
					json.NewEncoder(w).Encode(map[string]string{"error": "operation failed"})
				}
			}))
			defer server.Close()

			client := NewAPIClient(server.URL, "")
			err := client.StartProcess(tt.processName)

			if (err != nil) != tt.wantErr {
				t.Errorf("StartProcess() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestAPIClient_StopProcess tests stopping a process
func TestAPIClient_StopProcess(t *testing.T) {
	tests := []struct {
		name         string
		processName  string
		serverStatus int
		wantErr      bool
	}{
		{
			name:         "successful stop",
			processName:  "php-fpm",
			serverStatus: http.StatusOK,
			wantErr:      false,
		},
		{
			name:         "process not found",
			processName:  "unknown",
			serverStatus: http.StatusNotFound,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := "/api/v1/processes/" + tt.processName + "/stop"
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
				}

				w.WriteHeader(tt.serverStatus)
			}))
			defer server.Close()

			client := NewAPIClient(server.URL, "")
			err := client.StopProcess(tt.processName)

			if (err != nil) != tt.wantErr {
				t.Errorf("StopProcess() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestAPIClient_RestartProcess tests restarting a process
func TestAPIClient_RestartProcess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/processes/test-proc/restart" {
			t.Errorf("Expected path /api/v1/processes/test-proc/restart, got %s", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "")
	err := client.RestartProcess("test-proc")

	if err != nil {
		t.Errorf("RestartProcess() unexpected error: %v", err)
	}
}

// TestAPIClient_ScaleProcess tests scaling a process
func TestAPIClient_ScaleProcess(t *testing.T) {
	tests := []struct {
		name         string
		processName  string
		desired      int
		serverStatus int
		wantErr      bool
	}{
		{
			name:         "successful scale",
			processName:  "worker",
			desired:      5,
			serverStatus: http.StatusOK,
			wantErr:      false,
		},
		{
			name:         "invalid scale count",
			processName:  "worker",
			desired:      -1,
			serverStatus: http.StatusBadRequest,
			wantErr:      true,
		},
		{
			name:         "process not found",
			processName:  "unknown",
			desired:      3,
			serverStatus: http.StatusNotFound,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := "/api/v1/processes/" + tt.processName + "/scale"
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
				}

				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}

				// Check Content-Type
				if ct := r.Header.Get("Content-Type"); ct != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %s", ct)
				}

				// Decode and check body
				var body map[string]int
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("Failed to decode request body: %v", err)
				}

				if body["desired"] != tt.desired {
					t.Errorf("Expected desired %d, got %d", tt.desired, body["desired"])
				}

				w.WriteHeader(tt.serverStatus)
				if tt.wantErr {
					json.NewEncoder(w).Encode(map[string]string{"error": "scale failed"})
				}
			}))
			defer server.Close()

			client := NewAPIClient(server.URL, "")
			err := client.ScaleProcess(tt.processName, tt.desired)

			if (err != nil) != tt.wantErr {
				t.Errorf("ScaleProcess() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestAPIClient_HealthCheck tests health check endpoint
func TestAPIClient_HealthCheck(t *testing.T) {
	tests := []struct {
		name         string
		serverStatus int
		timeout      time.Duration
		wantErr      bool
	}{
		{
			name:         "healthy",
			serverStatus: http.StatusOK,
			timeout:      5 * time.Second,
			wantErr:      false,
		},
		{
			name:         "unhealthy",
			serverStatus: http.StatusServiceUnavailable,
			timeout:      5 * time.Second,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/health" {
					t.Errorf("Expected path /api/v1/health, got %s", r.URL.Path)
				}

				if r.Method != "GET" {
					t.Errorf("Expected GET method, got %s", r.Method)
				}

				w.WriteHeader(tt.serverStatus)
			}))
			defer server.Close()

			client := NewAPIClient(server.URL, "")

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			err := client.HealthCheck(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("HealthCheck() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestAPIClient_AddProcess tests adding a new process
func TestAPIClient_AddProcess(t *testing.T) {
	tests := []struct {
		name         string
		processName  string
		command      []string
		scale        int
		restart      string
		enabled      bool
		serverStatus int
		wantErr      bool
	}{
		{
			name:         "successful add",
			processName:  "new-worker",
			command:      []string{"php", "artisan", "queue:work"},
			scale:        2,
			restart:      "always",
			enabled:      true,
			serverStatus: http.StatusCreated,
			wantErr:      false,
		},
		{
			name:         "ok status accepted",
			processName:  "test-proc",
			command:      []string{"echo", "test"},
			scale:        1,
			restart:      "never",
			enabled:      false,
			serverStatus: http.StatusOK,
			wantErr:      false,
		},
		{
			name:         "process already exists",
			processName:  "php-fpm",
			command:      []string{"php-fpm", "-F"},
			scale:        1,
			restart:      "always",
			enabled:      true,
			serverStatus: http.StatusConflict,
			wantErr:      true,
		},
		{
			name:         "invalid request",
			processName:  "",
			command:      []string{},
			scale:        0,
			restart:      "invalid",
			enabled:      true,
			serverStatus: http.StatusBadRequest,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/processes" {
					t.Errorf("Expected path /api/v1/processes, got %s", r.URL.Path)
				}

				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}

				// Check Content-Type
				if ct := r.Header.Get("Content-Type"); ct != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %s", ct)
				}

				// Decode and validate body
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("Failed to decode request body: %v", err)
				}

				if body["name"] != tt.processName {
					t.Errorf("Expected name %s, got %v", tt.processName, body["name"])
				}

				w.WriteHeader(tt.serverStatus)
				if tt.wantErr {
					json.NewEncoder(w).Encode(map[string]string{"error": "operation failed"})
				} else {
					json.NewEncoder(w).Encode(map[string]string{"message": "success"})
				}
			}))
			defer server.Close()

			client := NewAPIClient(server.URL, "")

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := client.AddProcess(ctx, tt.processName, tt.command, tt.scale, tt.restart, tt.enabled)

			if (err != nil) != tt.wantErr {
				t.Errorf("AddProcess() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestAPIClient_ListProcesses_WithAuth tests authentication header
func TestAPIClient_ListProcesses_WithAuth(t *testing.T) {
	expectedAuth := "test-token-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+expectedAuth {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"processes": []process.ProcessInfo{},
		})
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, expectedAuth)
	_, err := client.ListProcesses()

	if err != nil {
		t.Errorf("ListProcesses() with auth failed: %v", err)
	}
}
