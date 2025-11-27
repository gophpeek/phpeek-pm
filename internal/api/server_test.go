package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gophpeek/phpeek-pm/internal/audit"
	"github.com/gophpeek/phpeek-pm/internal/config"
	"github.com/gophpeek/phpeek-pm/internal/process"
)

// createTestManager creates a real manager with minimal config for testing
func createTestManager(t *testing.T) *process.Manager {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			ShutdownTimeout:    30,
			LogLevel:           "error",
			MaxRestartAttempts: 3,
			RestartBackoff:     5,
		},
		Processes: map[string]*config.Process{
			"test-process": {
				Enabled:      true,
				Command:      []string{"sleep", "300"},
				Restart:      "never",
				Scale:        1,
				InitialState: "stopped", // Don't auto-start in tests
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	auditLogger := audit.NewLogger(logger, false)
	mgr := process.NewManager(cfg, logger, auditLogger)

	// Start the manager to initialize supervisors (but processes won't start because InitialState="stopped")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Failed to start test manager: %v", err)
	}

	return mgr
}

// createTestServer creates a test server with optional auth and ACL
func createTestServer(t *testing.T, auth string, aclCfg *config.ACLConfig) *Server {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	mgr := createTestManager(t)
	return NewServer(9180, "", auth, aclCfg, nil, false, mgr, logger)
}

// TestServer_Health tests the health endpoint
func TestServer_Health(t *testing.T) {
	server := createTestServer(t, "", nil)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "GET returns healthy",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status":"healthy"}`,
		},
		{
			name:           "POST not allowed",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   `{"error":"method not allowed"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v1/health", nil)
			w := httptest.NewRecorder()

			server.handleHealth(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			body := w.Body.String()
			if body != tt.expectedBody+"\n" && body != tt.expectedBody {
				t.Errorf("Expected body %q, got %q", tt.expectedBody, body)
			}
		})
	}
}

// TestServer_ListProcesses tests the process listing endpoint
func TestServer_ListProcesses(t *testing.T) {
	server := createTestServer(t, "", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/processes", nil)
	w := httptest.NewRecorder()

	server.handleProcesses(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	processes, ok := response["processes"].([]interface{})
	if !ok {
		t.Fatal("Response does not contain processes array")
	}

	if len(processes) != 1 {
		t.Errorf("Expected 1 process, got %d", len(processes))
	}
}

// TestServer_ScaleProcess tests the scale endpoint
func TestServer_ScaleProcess(t *testing.T) {
	tests := []struct {
		name           string
		processName    string
		desired        int
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "valid scale up",
			processName:    "test-process",
			desired:        5,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "valid scale down",
			processName:    "test-process",
			desired:        1,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "invalid scale zero",
			processName:    "test-process",
			desired:        0,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "non-existent process",
			processName:    "non-existent",
			desired:        3,
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(t, "", nil)

			body, _ := json.Marshal(map[string]int{"desired": tt.desired})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/processes/"+tt.processName+"/scale", bytes.NewReader(body))
			w := httptest.NewRecorder()

			server.handleScale(w, req, tt.processName)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response map[string]interface{}
			json.NewDecoder(w.Body).Decode(&response)

			if tt.expectError {
				if _, hasError := response["error"]; !hasError {
					t.Error("Expected error in response")
				}
			} else {
				if status, ok := response["status"].(string); !ok || status != "scaled" {
					t.Errorf("Expected status 'scaled', got %v", response["status"])
				}
			}
		})
	}
}

func TestServer_ScaleProcessDelta(t *testing.T) {
	server := createTestServer(t, "", nil)

	body, _ := json.Marshal(map[string]int{"delta": 1})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/processes/test-process/scale", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleScale(w, req, "test-process")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if status, ok := resp["status"].(string); !ok || status != "scaled" {
		t.Fatalf("expected scaled status, got %v", resp["status"])
	}
}

// TestAuthMiddleware tests authentication middleware
func TestAuthMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		serverAuth     string
		requestAuth    string
		expectedStatus int
	}{
		{
			name:           "no auth required",
			serverAuth:     "",
			requestAuth:    "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid token",
			serverAuth:     "secret-token",
			requestAuth:    "Bearer secret-token",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid token",
			serverAuth:     "secret-token",
			requestAuth:    "Bearer wrong-token",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing token",
			serverAuth:     "secret-token",
			requestAuth:    "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "malformed header",
			serverAuth:     "secret-token",
			requestAuth:    "secret-token",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(t, tt.serverAuth, nil)

			// Create a test handler that returns 200 OK if auth passes
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			})

			// Wrap with auth middleware
			handler := server.authMiddleware(testHandler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.requestAuth != "" {
				req.Header.Set("Authorization", tt.requestAuth)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestRateLimiter tests the rate limiting functionality
func TestRateLimiter(t *testing.T) {
	rl := newRateLimiter(10, 20) // 10 req/s, burst 20

	// Test normal requests (should all pass with burst capacity)
	for i := 0; i < 20; i++ {
		if !rl.allow("192.168.1.1") {
			t.Errorf("Request %d should be allowed (within burst)", i+1)
		}
	}

	// Next request should be denied (burst exhausted)
	if rl.allow("192.168.1.1") {
		t.Error("Request 21 should be denied (burst exhausted)")
	}

	// Different IP should have independent limit
	if !rl.allow("192.168.1.2") {
		t.Error("Request from different IP should be allowed")
	}
}

// TestRateLimiter_Refill tests token bucket refill
func TestRateLimiter_Refill(t *testing.T) {
	rl := newRateLimiter(10, 10) // 10 req/s, burst 10

	// Exhaust tokens
	for i := 0; i < 10; i++ {
		rl.allow("192.168.1.1")
	}

	// Should be denied
	if rl.allow("192.168.1.1") {
		t.Error("Request should be denied (tokens exhausted)")
	}

	// Wait for refill (1 second should add ~10 tokens)
	time.Sleep(1100 * time.Millisecond)

	// Should be allowed again
	if !rl.allow("192.168.1.1") {
		t.Error("Request should be allowed after refill")
	}
}

// TestTokenBucket_Allow tests token bucket algorithm
func TestTokenBucket_Allow(t *testing.T) {
	tb := newTokenBucket(5.0, 10) // 5 tokens/sec, capacity 10

	// Initial capacity should allow 10 requests
	for i := 0; i < 10; i++ {
		if !tb.allow() {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 11th request should be denied
	if tb.allow() {
		t.Error("Request 11 should be denied")
	}

	// Wait for refill (0.2 seconds = 1 token)
	time.Sleep(220 * time.Millisecond)

	// Should be allowed again
	if !tb.allow() {
		t.Error("Request should be allowed after partial refill")
	}
}

// TestServer_StartStop tests server lifecycle
func TestServer_StartStop(t *testing.T) {
	server := createTestServer(t, "", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start server
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Stop server
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	if err := server.Stop(stopCtx); err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}
}

// TestServer_HandleRestart tests the restart endpoint
func TestServer_HandleRestart(t *testing.T) {
	tests := []struct {
		name           string
		processName    string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "restart existing process",
			processName:    "test-process",
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "restart non-existent process",
			processName:    "non-existent",
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(t, "", nil)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/processes/"+tt.processName+"/restart", nil)
			w := httptest.NewRecorder()

			server.handleRestart(w, req, tt.processName)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response map[string]interface{}
			json.NewDecoder(w.Body).Decode(&response)

			if tt.expectError {
				if _, hasError := response["error"]; !hasError {
					t.Error("Expected error in response")
				}
			}
		})
	}
}

// TestServer_HandleStop tests the stop endpoint
func TestServer_HandleStop(t *testing.T) {
	tests := []struct {
		name           string
		processName    string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "stop existing process",
			processName:    "test-process",
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "stop non-existent process",
			processName:    "non-existent",
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(t, "", nil)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/processes/"+tt.processName+"/stop", nil)
			w := httptest.NewRecorder()

			server.handleStop(w, req, tt.processName)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response map[string]interface{}
			json.NewDecoder(w.Body).Decode(&response)

			if tt.expectError {
				if _, hasError := response["error"]; !hasError {
					t.Error("Expected error in response")
				}
			}
		})
	}
}

// TestServer_HandleStart tests the start endpoint
func TestServer_HandleStart(t *testing.T) {
	tests := []struct {
		name           string
		processName    string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "start existing process",
			processName:    "test-process",
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "start non-existent process",
			processName:    "non-existent",
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(t, "", nil)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/processes/"+tt.processName+"/start", nil)
			w := httptest.NewRecorder()

			server.handleStart(w, req, tt.processName)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response map[string]interface{}
			json.NewDecoder(w.Body).Decode(&response)

			if tt.expectError {
				if _, hasError := response["error"]; !hasError {
					t.Error("Expected error in response")
				}
			}
		})
	}
}

// TestServer_ConfigSave tests the config save endpoint
func TestServer_ConfigSave(t *testing.T) {
	server := createTestServer(t, "", nil)

	// Note: This will fail without a config path, but we're testing the endpoint
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/save", nil)
	w := httptest.NewRecorder()

	server.handleConfigSave(w, req)

	// Should return an error status since no config path is set
	if w.Code == http.StatusOK {
		// OK response is also acceptable if SaveConfig succeeds
		var response map[string]interface{}
		json.NewDecoder(w.Body).Decode(&response)
		if status, ok := response["status"].(string); !ok || status != "saved" {
			t.Errorf("Expected status 'saved', got %v", response["status"])
		}
	} else if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", w.Code)
	}
}

// TestServer_ConfigReload tests the config reload endpoint
func TestServer_ConfigReload(t *testing.T) {
	server := createTestServer(t, "", nil)

	// Note: This will fail without a config path, but we're testing the endpoint
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/reload", nil)
	w := httptest.NewRecorder()

	server.handleConfigReload(w, req)

	// Should return an error status since no config path is set
	if w.Code == http.StatusOK {
		// OK response is also acceptable if ReloadConfig succeeds
		var response map[string]interface{}
		json.NewDecoder(w.Body).Decode(&response)
		if status, ok := response["status"].(string); !ok || status != "reloaded" {
			t.Errorf("Expected status 'reloaded', got %v", response["status"])
		}
	} else if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", w.Code)
	}
}

// TestRateLimitMiddleware tests rate limiting middleware integration
func TestRateLimitMiddleware(t *testing.T) {
	server := createTestServer(t, "", nil)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	handler := server.rateLimitMiddleware(testHandler)

	// First 200 requests should pass (burst capacity)
	for i := 0; i < 200; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d should pass (within burst), got status %d", i+1, w.Code)
			break
		}
	}

	// Next request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected rate limit (429), got %d", w.Code)
	}
}

// TestACLMiddleware tests IP-based ACL middleware
func TestACLMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		aclConfig      *config.ACLConfig
		remoteAddr     string
		forwardedFor   string
		expectedStatus int
	}{
		{
			name:           "no ACL configured",
			aclConfig:      nil,
			remoteAddr:     "192.168.1.1:12345",
			expectedStatus: http.StatusOK,
		},
		{
			name: "allowed IP",
			aclConfig: &config.ACLConfig{
				Enabled:   true,
				Mode:      "allow",
				AllowList: []string{"192.168.1.0/24"},
			},
			remoteAddr:     "192.168.1.50:12345",
			expectedStatus: http.StatusOK,
		},
		{
			name: "denied IP",
			aclConfig: &config.ACLConfig{
				Enabled:   true,
				Mode:      "allow",
				AllowList: []string{"192.168.1.0/24"},
			},
			remoteAddr:     "10.0.0.1:12345",
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "explicitly denied IP",
			aclConfig: &config.ACLConfig{
				Enabled:  true,
				Mode:     "deny",
				DenyList: []string{"192.168.1.100"},
			},
			remoteAddr:     "192.168.1.100:12345",
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "X-Forwarded-For allowed",
			aclConfig: &config.ACLConfig{
				Enabled:    true,
				Mode:       "allow",
				AllowList:  []string{"203.0.113.0/24"},
				TrustProxy: true,
			},
			remoteAddr:     "192.168.1.1:12345",
			forwardedFor:   "203.0.113.50",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(t, "", tt.aclConfig)

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			})

			handler := server.aclMiddleware(testHandler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.forwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.forwardedFor)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestServer_InvalidJSON tests malformed JSON request bodies
func TestServer_InvalidJSON(t *testing.T) {
	server := createTestServer(t, "", nil)

	// Test scale endpoint with invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/api/v1/processes/test-process/scale", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleScale(w, req, "test-process")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid JSON, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if _, hasError := response["error"]; !hasError {
		t.Error("Expected error in response for invalid JSON")
	}
}

// TestServer_HandleAddProcess tests adding a new process dynamically
func TestServer_HandleAddProcess(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		expectedStatus int
	}{
		{
			name:           "invalid JSON",
			body:           `{invalid json}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing name",
			body:           `{"process": {"enabled": true, "command": ["echo", "test"]}}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing process config",
			body:           `{"name": "test-proc"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "valid add process",
			body:           `{"name": "dynamic-process", "process": {"enabled": true, "command": ["echo", "test"], "restart": "never", "scale": 1, "initial_state": "stopped"}}`,
			expectedStatus: http.StatusCreated, // Expecting 201 on success
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(t, "", nil)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/processes", bytes.NewReader([]byte(tt.body)))
			w := httptest.NewRecorder()

			server.handleAddProcess(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestServer_HandleUpdateProcess tests updating an existing process
func TestServer_HandleUpdateProcess(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		processName    string
		expectedStatus int
	}{
		{
			name:           "invalid JSON",
			body:           `{invalid json}`,
			processName:    "test-process",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing process config",
			body:           `{"some": "value"}`,
			processName:    "test-process",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "valid update",
			body:           `{"process": {"enabled": true, "command": ["sleep", "600"], "restart": "never", "scale": 2, "initial_state": "stopped"}}`,
			processName:    "test-process",
			expectedStatus: http.StatusOK, // Success expected if manager supports it
		},
		{
			name:           "non-existent process",
			body:           `{"process": {"enabled": true, "command": ["echo", "test"], "restart": "never", "scale": 1, "initial_state": "stopped"}}`,
			processName:    "non-existent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(t, "", nil)

			req := httptest.NewRequest(http.MethodPut, "/api/v1/processes/"+tt.processName, bytes.NewReader([]byte(tt.body)))
			w := httptest.NewRecorder()

			server.handleUpdateProcess(w, req, tt.processName)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestServer_HandleRemoveProcess tests removing a process
func TestServer_HandleRemoveProcess(t *testing.T) {
	tests := []struct {
		name           string
		processName    string
		expectedStatus int
	}{
		{
			name:           "valid remove existing process",
			processName:    "test-process",
			expectedStatus: http.StatusOK, // Success if manager supports removal
		},
		{
			name:           "remove non-existent process",
			processName:    "non-existent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(t, "", nil)

			req := httptest.NewRequest(http.MethodDelete, "/api/v1/processes/"+tt.processName, nil)
			w := httptest.NewRecorder()

			server.handleRemoveProcess(w, req, tt.processName)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestServer_Port tests the Port() getter method
func TestServer_Port(t *testing.T) {
	server := createTestServer(t, "", nil)

	port := server.Port()
	if port != 9180 {
		t.Errorf("Expected port 9180, got %d", port)
	}
}

// TestServer_HandleProcessAction tests process action routing
func TestServer_HandleProcessAction(t *testing.T) {
	server := createTestServer(t, "", nil)

	tests := []struct {
		name   string
		action string
		method string
	}{
		{"start action", "start", http.MethodPost},
		{"stop action", "stop", http.MethodPost},
		{"restart action", "restart", http.MethodPost},
		{"scale action", "scale", http.MethodPost},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.action == "scale" {
				body, _ = json.Marshal(map[string]int{"desired": 2})
			}

			req := httptest.NewRequest(tt.method, "/api/v1/processes/test-process/"+tt.action, bytes.NewReader(body))
			w := httptest.NewRecorder()

			server.handleProcessAction(w, req)

			// Should get 200 or 500 (not 404)
			if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError && w.Code != http.StatusBadRequest {
				t.Errorf("Expected 200/400/500, got %d", w.Code)
			}
		})
	}
}

// TestServer_HandleProcessAction_InvalidAction tests invalid action handling
func TestServer_HandleProcessAction_InvalidAction(t *testing.T) {
	server := createTestServer(t, "", nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/processes/test-process/invalid", nil)
	w := httptest.NewRecorder()

	server.handleProcessAction(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid action, got %d", w.Code)
	}
}

// TestServer_HandleProcesses_MethodNotAllowed tests method validation for processes endpoint
func TestServer_HandleProcesses_MethodNotAllowed(t *testing.T) {
	server := createTestServer(t, "", nil)

	// DELETE is not allowed on /processes (only GET and POST)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/processes", nil)
	w := httptest.NewRecorder()

	server.handleProcesses(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for DELETE on /processes, got %d", w.Code)
	}
}

// TestServer_HandleProcesses_PostRouting tests POST routing to handleAddProcess
func TestServer_HandleProcesses_PostRouting(t *testing.T) {
	server := createTestServer(t, "", nil)

	// POST should route to handleAddProcess
	body := `{"name": "new-process", "process": {"enabled": true, "command": ["echo", "test"], "restart": "never", "scale": 1, "initial_state": "stopped"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/processes", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()

	server.handleProcesses(w, req)

	// Should get 201 Created if successful
	if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 201 or 500, got %d", w.Code)
	}
}

// TestServer_StartSocketListener tests Unix socket listener setup
func TestServer_StartSocketListener(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	mgr := createTestManager(t)

	tests := []struct {
		name        string
		socketPath  string
		expectError bool
	}{
		{
			name:        "valid socket path",
			socketPath:  "/tmp/phpeek-pm-test.sock",
			expectError: false,
		},
		{
			name:        "invalid socket path directory",
			socketPath:  "/nonexistent/directory/test.sock",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up socket if it exists
			os.Remove(tt.socketPath)
			defer os.Remove(tt.socketPath)

			server := NewServer(9180, tt.socketPath, "", nil, nil, false, mgr, logger)

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			err := server.startSocketListener(testHandler)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			// If successful, verify socket exists and has correct permissions
			if err == nil {
				info, statErr := os.Stat(tt.socketPath)
				if statErr != nil {
					t.Fatalf("Socket file not created: %v", statErr)
				}

				// Check permissions (should be 0600)
				if info.Mode().Perm() != 0600 {
					t.Errorf("Expected permissions 0600, got %o", info.Mode().Perm())
				}

				// Clean up server
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				server.Stop(ctx)
			}
		})
	}
}

// TestServer_HandleGetLogs tests the process logs endpoint
func TestServer_HandleGetLogs(t *testing.T) {
	tests := []struct {
		name           string
		processName    string
		queryParams    string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "get logs for existing process",
			processName:    "test-process",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "get logs with limit parameter",
			processName:    "test-process",
			queryParams:    "?limit=50",
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "get logs with invalid limit",
			processName:    "test-process",
			queryParams:    "?limit=invalid",
			expectedStatus: http.StatusOK, // Falls back to default limit
			expectError:    false,
		},
		{
			name:           "get logs for non-existent process",
			processName:    "non-existent",
			queryParams:    "",
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(t, "", nil)

			url := "/api/v1/processes/" + tt.processName + "/logs" + tt.queryParams
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()

			server.handleGetLogs(w, req, tt.processName)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response map[string]interface{}
			json.NewDecoder(w.Body).Decode(&response)

			if tt.expectError {
				if _, hasError := response["error"]; !hasError {
					t.Error("Expected error in response")
				}
			} else {
				// Verify response structure
				if process, ok := response["process"].(string); !ok || process != tt.processName {
					t.Errorf("Expected process %s, got %v", tt.processName, response["process"])
				}
				if _, hasLogs := response["logs"]; !hasLogs {
					t.Error("Expected logs array in response")
				}
				if _, hasCount := response["count"]; !hasCount {
					t.Error("Expected count in response")
				}
			}
		})
	}
}

// TestServer_HandleGetProcess tests the get process config endpoint
func TestServer_HandleGetProcess(t *testing.T) {
	tests := []struct {
		name           string
		processName    string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "get existing process config",
			processName:    "test-process",
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "get non-existent process config",
			processName:    "non-existent",
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(t, "", nil)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/processes/"+tt.processName, nil)
			w := httptest.NewRecorder()

			server.handleGetProcess(w, req, tt.processName)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response map[string]interface{}
			json.NewDecoder(w.Body).Decode(&response)

			if tt.expectError {
				if _, hasError := response["error"]; !hasError {
					t.Error("Expected error in response")
				}
			} else {
				// Verify response structure
				if process, ok := response["process"].(string); !ok || process != tt.processName {
					t.Errorf("Expected process %s, got %v", tt.processName, response["process"])
				}
				if _, hasConfig := response["config"]; !hasConfig {
					t.Error("Expected config in response")
				}
			}
		})
	}
}

// TestServer_HandleStackLogs tests the stack logs aggregation endpoint
func TestServer_HandleStackLogs(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		queryParams    string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "GET stack logs default limit",
			method:         http.MethodGet,
			queryParams:    "",
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "GET stack logs with limit",
			method:         http.MethodGet,
			queryParams:    "?limit=200",
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "GET stack logs with invalid limit",
			method:         http.MethodGet,
			queryParams:    "?limit=invalid",
			expectedStatus: http.StatusOK, // Falls back to default
			expectError:    false,
		},
		{
			name:           "POST not allowed",
			method:         http.MethodPost,
			queryParams:    "",
			expectedStatus: http.StatusMethodNotAllowed,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(t, "", nil)

			url := "/api/v1/logs" + tt.queryParams
			req := httptest.NewRequest(tt.method, url, nil)
			w := httptest.NewRecorder()

			server.handleStackLogs(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response map[string]interface{}
			json.NewDecoder(w.Body).Decode(&response)

			if tt.expectError {
				if _, hasError := response["error"]; !hasError {
					t.Error("Expected error in response")
				}
			} else {
				// Verify response structure
				if scope, ok := response["scope"].(string); !ok || scope != "stack" {
					t.Errorf("Expected scope 'stack', got %v", response["scope"])
				}
				if _, hasLogs := response["logs"]; !hasLogs {
					t.Error("Expected logs array in response")
				}
				if _, hasCount := response["count"]; !hasCount {
					t.Error("Expected count in response")
				}
				if _, hasLimit := response["limit"]; !hasLimit {
					t.Error("Expected limit in response")
				}
			}
		})
	}
}

// TestServer_SocketCleanup tests socket file cleanup on server stop
func TestServer_SocketCleanup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	mgr := createTestManager(t)
	socketPath := "/tmp/phpeek-pm-cleanup-test.sock"

	// Clean up any existing socket
	os.Remove(socketPath)
	defer os.Remove(socketPath)

	server := NewServer(9180, socketPath, "", nil, nil, false, mgr, logger)

	// Start server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Give socket time to start
	time.Sleep(100 * time.Millisecond)

	// Verify socket exists
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Error("Socket file should exist after server start")
	}

	// Stop server
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	if err := server.Stop(stopCtx); err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}

	// Verify socket is removed
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Error("Socket file should be removed after server stop")
	}
}

// TestServer_HandleGetLogs_LimitParsing tests various limit parameter formats
func TestServer_HandleGetLogs_LimitParsing(t *testing.T) {
	server := createTestServer(t, "", nil)

	tests := []struct {
		name          string
		limit         string
		expectedLimit int
	}{
		{"default limit", "", 100},
		{"valid limit", "?limit=50", 50},
		{"zero limit", "?limit=0", 100},       // Falls back to default
		{"negative limit", "?limit=-10", 100}, // Falls back to default
		{"non-numeric limit", "?limit=abc", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/api/v1/processes/test-process/logs" + tt.limit
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()

			server.handleGetLogs(w, req, "test-process")

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				json.NewDecoder(w.Body).Decode(&response)

				if limit, ok := response["limit"].(float64); ok {
					if int(limit) != tt.expectedLimit {
						t.Errorf("Expected limit %d, got %d", tt.expectedLimit, int(limit))
					}
				}
			}
		})
	}
}

// TestServer_HandleStackLogs_LimitParsing tests various limit parameter formats for stack logs
func TestServer_HandleStackLogs_LimitParsing(t *testing.T) {
	server := createTestServer(t, "", nil)

	tests := []struct {
		name          string
		limit         string
		expectedLimit int
	}{
		{"default limit", "", 100},
		{"valid limit", "?limit=200", 200},
		{"zero limit", "?limit=0", 100},       // Falls back to default
		{"negative limit", "?limit=-5", 100},  // Falls back to default
		{"non-numeric limit", "?limit=xyz", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/api/v1/logs" + tt.limit
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()

			server.handleStackLogs(w, req)

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				json.NewDecoder(w.Body).Decode(&response)

				if limit, ok := response["limit"].(float64); ok {
					if int(limit) != tt.expectedLimit {
						t.Errorf("Expected limit %d, got %d", tt.expectedLimit, int(limit))
					}
				}
			}
		})
	}
}

// TestServer_PanicRecoveryMiddleware tests panic recovery
func TestServer_PanicRecoveryMiddleware(t *testing.T) {
	server := createTestServer(t, "", nil)

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	handler := server.panicRecoveryMiddleware(panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 after panic, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if _, hasError := response["error"]; !hasError {
		t.Error("Expected error in response after panic")
	}
}

// TestServer_BodyLimitMiddleware tests request body size limiting
func TestServer_BodyLimitMiddleware(t *testing.T) {
	server := createTestServer(t, "", nil)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to read body
		var data map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&data)
		if err != nil {
			server.respondError(w, http.StatusBadRequest, "invalid body")
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := server.bodyLimitMiddleware(testHandler)

	tests := []struct {
		name       string
		bodySize   int
		expectFail bool
	}{
		{
			name:       "small body passes",
			bodySize:   1024,
			expectFail: false,
		},
		{
			name:       "normal body passes",
			bodySize:   1024 * 100,
			expectFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := make([]byte, tt.bodySize)
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// Body should be readable or limited
			if w.Code == http.StatusOK || w.Code == http.StatusBadRequest {
				// Expected behavior
			} else {
				t.Errorf("Unexpected status code: %d", w.Code)
			}
		})
	}
}

// TestServer_HandleProcessAction_PathParsing tests various path parsing scenarios
func TestServer_HandleProcessAction_PathParsing(t *testing.T) {
	server := createTestServer(t, "", nil)

	tests := []struct {
		name           string
		path           string
		method         string
		expectedStatus int
	}{
		{
			name:           "invalid path - too short",
			path:           "/api/v1/processes/",
			method:         http.MethodPost,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "GET without action (get process config)",
			path:           "/api/v1/processes/test-process",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "PUT without action (update process)",
			path:           "/api/v1/processes/test-process",
			method:         http.MethodPut,
			expectedStatus: http.StatusBadRequest, // Missing process config in body
		},
		{
			name:           "DELETE without action (remove process)",
			path:           "/api/v1/processes/test-process",
			method:         http.MethodDelete,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			server.handleProcessAction(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestRateLimiter_CleanupVisitors tests the visitor cleanup goroutine
func TestRateLimiter_CleanupVisitors(t *testing.T) {
	rl := newRateLimiter(10, 10)

	// Add some visitors
	rl.allow("192.168.1.1")
	rl.allow("192.168.1.2")
	rl.allow("192.168.1.3")

	// Verify visitors exist
	rl.mu.RLock()
	visitorCount := len(rl.visitors)
	rl.mu.RUnlock()

	if visitorCount != 3 {
		t.Errorf("Expected 3 visitors, got %d", visitorCount)
	}

	// Stop the rate limiter
	rl.stop()

	// Wait a bit for cleanup goroutine to exit
	time.Sleep(100 * time.Millisecond)

	// Cleanup goroutine should have stopped
	// This test mainly ensures no panics occur during cleanup
}

// TestServer_HandleConfigSave_EdgeCases tests config save error handling
func TestServer_HandleConfigSave_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "GET method not allowed",
			method:         http.MethodGet,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "PUT method not allowed",
			method:         http.MethodPut,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "DELETE method not allowed",
			method:         http.MethodDelete,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(t, "", nil)

			req := httptest.NewRequest(tt.method, "/api/v1/config/save", nil)
			w := httptest.NewRecorder()

			server.handleConfigSave(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestServer_HandleConfigReload_EdgeCases tests config reload error handling
func TestServer_HandleConfigReload_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "GET method not allowed",
			method:         http.MethodGet,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "PUT method not allowed",
			method:         http.MethodPut,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "DELETE method not allowed",
			method:         http.MethodDelete,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(t, "", nil)

			req := httptest.NewRequest(tt.method, "/api/v1/config/reload", nil)
			w := httptest.NewRecorder()

			server.handleConfigReload(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestServer_HandleMetricsHistory tests the metrics history endpoint
func TestServer_HandleMetricsHistory(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		queryParams    string
		expectedStatus int
	}{
		{
			name:           "POST method not allowed",
			method:         http.MethodPost,
			queryParams:    "?process=test&instance=test-0",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "missing process parameter",
			method:         http.MethodGet,
			queryParams:    "?instance=test-0",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing instance parameter",
			method:         http.MethodGet,
			queryParams:    "?process=test",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid since parameter",
			method:         http.MethodGet,
			queryParams:    "?process=test&instance=test-0&since=invalid",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid limit parameter - negative",
			method:         http.MethodGet,
			queryParams:    "?process=test&instance=test-0&limit=-10",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid limit parameter - too large",
			method:         http.MethodGet,
			queryParams:    "?process=test&instance=test-0&limit=20000",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "valid request returns data",
			method:         http.MethodGet,
			queryParams:    "?process=test&instance=test-0",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid request with Unix timestamp",
			method:         http.MethodGet,
			queryParams:    "?process=test&instance=test-0&since=1700000000",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid request with RFC3339 timestamp",
			method:         http.MethodGet,
			queryParams:    "?process=test&instance=test-0&since=2023-11-15T10:00:00Z",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid request with limit",
			method:         http.MethodGet,
			queryParams:    "?process=test&instance=test-0&limit=50",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(t, "", nil)

			url := "/api/v1/metrics/history" + tt.queryParams
			req := httptest.NewRequest(tt.method, url, nil)
			w := httptest.NewRecorder()

			server.handleMetricsHistory(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestServer_RespondJSON_ErrorCases tests JSON encoding edge cases
func TestServer_RespondJSON_ErrorCases(t *testing.T) {
	server := createTestServer(t, "", nil)

	// Test with a value that can be marshaled
	w := httptest.NewRecorder()
	server.respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// Verify content type is set
	if contentType := w.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

// TestServer_NewServer_WithTLS tests server creation with TLS config
func TestServer_NewServer_WithTLS(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	mgr := createTestManager(t)

	tlsConfig := &config.TLSConfig{
		Enabled:  true,
		CertFile: "/path/to/cert.pem",
		KeyFile:  "/path/to/key.pem",
	}

	server := NewServer(9443, "", "", nil, tlsConfig, false, mgr, logger)

	if server == nil {
		t.Fatal("Expected server to be created with TLS config")
	}

	if server.Port() != 9443 {
		t.Errorf("Expected port 9443, got %d", server.Port())
	}
}

// TestServer_Start_WithInvalidTLS tests starting server with invalid TLS config
func TestServer_Start_WithInvalidTLS(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	mgr := createTestManager(t)

	tlsConfig := &config.TLSConfig{
		Enabled:  true,
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
	}

	server := NewServer(9443, "", "", nil, tlsConfig, false, mgr, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Should fail to start with invalid cert files
	err := server.Start(ctx)
	if err == nil {
		t.Error("Expected error starting server with invalid TLS files")
		server.Stop(ctx)
	}
}

// TestServer_ACLMiddleware_EdgeCases tests ACL middleware edge cases
func TestServer_ACLMiddleware_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		aclConfig      *config.ACLConfig
		remoteAddr     string
		expectedStatus int
	}{
		{
			name: "invalid IP address format",
			aclConfig: &config.ACLConfig{
				Enabled:   true,
				Mode:      "allow",
				AllowList: []string{"192.168.1.0/24"},
			},
			remoteAddr:     "invalid-ip",
			expectedStatus: http.StatusBadRequest, // Invalid IP returns 400
		},
		{
			name: "missing port in remote addr",
			aclConfig: &config.ACLConfig{
				Enabled:   true,
				Mode:      "allow",
				AllowList: []string{"192.168.1.0/24"},
			},
			remoteAddr:     "192.168.1.50",
			expectedStatus: http.StatusOK, // Should handle missing port gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(t, "", tt.aclConfig)

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := server.aclMiddleware(testHandler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}
