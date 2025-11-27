package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gophpeek/phpeek-pm/internal/acl"
	"github.com/gophpeek/phpeek-pm/internal/audit"
	"github.com/gophpeek/phpeek-pm/internal/config"
	"github.com/gophpeek/phpeek-pm/internal/process"
	tlsmgr "github.com/gophpeek/phpeek-pm/internal/tls"
)

// maxRequestBodySize limits request body to prevent memory exhaustion attacks
const maxRequestBodySize = 8 * 1024 * 1024 // 8MB

// rateLimiter implements a token bucket rate limiter per client IP
type rateLimiter struct {
	visitors        map[string]*visitor
	mu              sync.RWMutex
	rate            int // requests per second
	burst           int // burst capacity
	cleanupInterval time.Duration
	stopCh          chan struct{}  // Signal to stop cleanup goroutine
	wg              sync.WaitGroup // Tracks cleanup goroutine lifecycle
}

// visitor tracks rate limit state for a single IP
type visitor struct {
	limiter  *tokenBucket
	lastSeen time.Time
}

// tokenBucket implements token bucket algorithm for rate limiting
type tokenBucket struct {
	tokens     float64
	capacity   float64
	refillRate float64
	lastRefill time.Time
	mu         sync.Mutex
}

// newRateLimiter creates a new rate limiter
// rate: requests per second, burst: maximum burst size
func newRateLimiter(rate, burst int) *rateLimiter {
	rl := &rateLimiter{
		visitors:        make(map[string]*visitor),
		rate:            rate,
		burst:           burst,
		cleanupInterval: 5 * time.Minute,
		stopCh:          make(chan struct{}),
	}

	// Start cleanup goroutine to remove stale entries (tracked by WaitGroup)
	rl.wg.Add(1)
	go rl.cleanupVisitors()

	return rl
}

// stop terminates the cleanup goroutine and waits for it to finish
func (rl *rateLimiter) stop() {
	close(rl.stopCh)
	rl.wg.Wait() // Wait for cleanup goroutine to terminate
}

// allow checks if request from this IP should be allowed
func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.RLock()
	v, exists := rl.visitors[ip]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		// Check again after acquiring write lock (double-checked locking)
		v, exists = rl.visitors[ip]
		if !exists {
			v = &visitor{
				limiter:  newTokenBucket(float64(rl.rate), rl.burst),
				lastSeen: time.Now(),
			}
			rl.visitors[ip] = v
		}
		rl.mu.Unlock()
	}

	v.lastSeen = time.Now()
	return v.limiter.allow()
}

// cleanupVisitors removes stale visitor entries
func (rl *rateLimiter) cleanupVisitors() {
	defer rl.wg.Done() // Signal completion when goroutine exits

	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.mu.Lock()
			for ip, v := range rl.visitors {
				if time.Since(v.lastSeen) > 10*time.Minute {
					delete(rl.visitors, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// newTokenBucket creates a new token bucket
func newTokenBucket(refillRate float64, capacity int) *tokenBucket {
	return &tokenBucket{
		tokens:     float64(capacity),
		capacity:   float64(capacity),
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// allow checks if a token can be consumed (request allowed)
func (tb *tokenBucket) allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()

	// Refill tokens based on elapsed time
	tb.tokens += elapsed * tb.refillRate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastRefill = now

	// Consume a token if available
	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}

	return false
}

// Server provides REST API for process management
type Server struct {
	port         int
	socketPath   string
	auth         string
	manager      *process.Manager
	server       *http.Server
	socketServer *http.Server
	logger       *slog.Logger
	rateLimiter  *rateLimiter
	aclConfig    *config.ACLConfig
	aclChecker   *acl.Checker
	tlsConfig    *config.TLSConfig
	tlsManager   *tlsmgr.Manager
	auditLogger  *audit.Logger
}

// NewServer creates a new API server
// Rate limiting: 100 requests/second with burst of 200
func NewServer(port int, socketPath string, auth string, aclCfg *config.ACLConfig, tlsCfg *config.TLSConfig, auditEnabled bool, manager *process.Manager, log *slog.Logger) *Server {
	// Create ACL checker if enabled
	var aclChecker *acl.Checker
	if aclCfg != nil && aclCfg.Enabled {
		checker, err := acl.NewChecker(aclCfg)
		if err != nil {
			log.Error("Failed to create ACL checker", "error", err)
		} else {
			aclChecker = checker
			log.Info("ACL enabled", "mode", aclCfg.Mode, "allow_count", len(aclCfg.AllowList), "deny_count", len(aclCfg.DenyList))
		}
	}

	// Create audit logger
	auditLogger := audit.NewLogger(log, auditEnabled)
	if auditEnabled {
		log.Info("Audit logging enabled for API server")
	}

	return &Server{
		port:        port,
		socketPath:  socketPath,
		auth:        auth,
		aclConfig:   aclCfg,
		aclChecker:  aclChecker,
		tlsConfig:   tlsCfg,
		manager:     manager,
		logger:      log,
		rateLimiter: newRateLimiter(100, 200),
		auditLogger: auditLogger,
	}
}

// Start starts the API server (both TCP and Unix socket if configured)
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// API routes with full middleware stack: panicRecovery -> bodyLimit -> rateLimit -> auth -> handler
	// Health endpoint: rate limited but no auth required
	mux.HandleFunc("/api/v1/health", s.wrapHandler(s.handleHealth, false))
	// Protected endpoints: full middleware stack with auth
	mux.HandleFunc("/api/v1/processes", s.wrapHandler(s.handleProcesses, true))
	mux.HandleFunc("/api/v1/processes/", s.wrapHandler(s.handleProcessAction, true))
	mux.HandleFunc("/api/v1/logs", s.wrapHandler(s.handleStackLogs, true))
	// Config management endpoints
	mux.HandleFunc("/api/v1/config/save", s.wrapHandler(s.handleConfigSave, true))
	mux.HandleFunc("/api/v1/config/reload", s.wrapHandler(s.handleConfigReload, true))
	// Metrics endpoints
	mux.HandleFunc("/api/v1/metrics/history", s.wrapHandler(s.handleMetricsHistory, true))

	// Wrap mux with ACL middleware if enabled (applied to all routes, TCP only)
	var tcpHandler http.Handler = mux
	var socketHandler http.Handler = mux

	if s.aclChecker != nil {
		tcpHandler = s.aclMiddleware(mux)
		s.logger.Info("ACL middleware enabled for API server (TCP only)")
		// Unix socket doesn't need ACL (file permissions provide security)
	}

	// Start Unix socket listener (if configured)
	if s.socketPath != "" {
		if err := s.startSocketListener(socketHandler); err != nil {
			s.logger.Warn("Failed to start Unix socket listener", "error", err, "path", s.socketPath)
			// Continue with TCP-only mode
		}
	}

	// Start TCP listener
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      tcpHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Setup TLS if enabled (TCP only)
	var scheme string
	if s.tlsConfig != nil && s.tlsConfig.Enabled {
		tlsMgr, err := tlsmgr.NewManager(s.tlsConfig, s.logger)
		if err != nil {
			return fmt.Errorf("failed to create TLS manager: %w", err)
		}

		tlsConf, err := tlsMgr.GetTLSConfig()
		if err != nil {
			return fmt.Errorf("failed to get TLS config: %w", err)
		}

		s.server.TLSConfig = tlsConf
		s.tlsManager = tlsMgr
		scheme = "https"
	} else {
		scheme = "http"
	}

	s.logger.Info("Starting API server (TCP)",
		"scheme", scheme,
		"port", s.port,
		"tls_enabled", s.tlsConfig != nil && s.tlsConfig.Enabled,
	)

	// Start TCP server in background
	go func() {
		var err error
		if s.tlsConfig != nil && s.tlsConfig.Enabled {
			// TLS is configured in server.TLSConfig, but we still need to call ListenAndServeTLS
			// Pass empty cert/key since we're using GetCertificate callback
			err = s.server.ListenAndServeTLS("", "")
		} else {
			err = s.server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			s.logger.Error("API server (TCP) failed", "error", err)
		}
	}()

	return nil
}

// startSocketListener starts the Unix socket listener
func (s *Server) startSocketListener(handler http.Handler) error {
	// Remove existing socket file if it exists
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create Unix socket listener
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket listener: %w", err)
	}

	// Set socket permissions (0600 = owner only)
	if err := os.Chmod(s.socketPath, 0600); err != nil {
		listener.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	s.socketServer = &http.Server{
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.logger.Info("Starting API server (Unix socket)",
		"path", s.socketPath,
		"permissions", "0600",
	)

	// Start socket server in background
	go func() {
		if err := s.socketServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error("API server (socket) failed", "error", err)
		}
	}()

	return nil
}

// Stop gracefully stops the API server (both TCP and socket)
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping API server")

	var errors []error

	// Stop rate limiter cleanup goroutine
	if s.rateLimiter != nil {
		s.rateLimiter.stop()
	}

	// Stop TLS manager if running
	if s.tlsManager != nil {
		s.tlsManager.Stop()
	}

	// Stop TCP server
	if s.server != nil {
		if err := s.server.Shutdown(ctx); err != nil {
			s.logger.Error("Failed to stop API server (TCP) gracefully", "error", err)
			errors = append(errors, fmt.Errorf("TCP: %w", err))
		}
	}

	// Stop Unix socket server
	if s.socketServer != nil {
		if err := s.socketServer.Shutdown(ctx); err != nil {
			s.logger.Error("Failed to stop API server (socket) gracefully", "error", err)
			errors = append(errors, fmt.Errorf("socket: %w", err))
		}

		// Clean up socket file
		if s.socketPath != "" {
			if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
				s.logger.Warn("Failed to remove socket file", "error", err)
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errors)
	}

	s.logger.Info("API server stopped")
	return nil
}

// aclMiddleware wraps ACL checking with audit logging
func (s *Server) aclMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.aclChecker == nil {
			next.ServeHTTP(w, r)
			return
		}

		// Extract client IP
		ip, err := s.aclChecker.ExtractIP(r)
		if err != nil {
			s.auditLogger.LogACLDeny(r.RemoteAddr, r.URL.Path, "invalid IP format")
			http.Error(w, "Unable to determine client IP", http.StatusBadRequest)
			return
		}

		// Check ACL
		if !s.aclChecker.IsAllowed(ip) {
			s.auditLogger.LogACLDeny(ip.String(), r.URL.Path, "IP not in allow list")
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		// IP allowed, continue
		next.ServeHTTP(w, r)
	})
}

// rateLimitMiddleware applies rate limiting per client IP
func (s *Server) rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract client IP (handles X-Forwarded-For and X-Real-IP)
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.Header.Get("X-Real-IP")
		}
		if ip == "" {
			ip = r.RemoteAddr
		}

		// Check rate limit
		if !s.rateLimiter.allow(ip) {
			s.logger.Warn("Rate limit exceeded",
				"ip", ip,
				"path", r.URL.Path,
			)
			s.auditLogger.LogRateLimit(ip, r.URL.Path)
			s.respondError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		next(w, r)
	}
}

// authMiddleware checks Bearer token authentication
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.auth == "" {
			// No auth required
			next(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		expectedAuth := fmt.Sprintf("Bearer %s", s.auth)

		if authHeader != expectedAuth {
			// Extract IP for audit log
			ip := r.RemoteAddr
			if s.aclChecker != nil {
				if clientIP, err := s.aclChecker.ExtractIP(r); err == nil {
					ip = clientIP.String()
				}
			}
			reason := "invalid or missing bearer token"
			if authHeader == "" {
				reason = "missing authorization header"
			}
			s.auditLogger.LogAuthFailure(ip, r.URL.Path, reason)
			s.respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		next(w, r)
	}
}

// panicRecoveryMiddleware recovers from panics and returns 500 error
func (s *Server) panicRecoveryMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				s.logger.Error("Panic recovered in API handler",
					"error", err,
					"path", r.URL.Path,
					"method", r.Method,
					"stack", string(debug.Stack()),
				)
				s.respondError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next(w, r)
	}
}

// bodyLimitMiddleware limits request body size to prevent memory exhaustion
func (s *Server) bodyLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
		}
		next(w, r)
	}
}

// wrapHandler applies the full middleware stack to a handler.
// Middleware order (outermost to innermost): panicRecovery -> bodyLimit -> rateLimit -> [auth] -> handler
// The auth middleware is only applied when requireAuth is true.
func (s *Server) wrapHandler(handler http.HandlerFunc, requireAuth bool) http.HandlerFunc {
	h := handler
	if requireAuth {
		h = s.authMiddleware(h)
	}
	h = s.rateLimitMiddleware(h)
	h = s.bodyLimitMiddleware(h)
	h = s.panicRecoveryMiddleware(h)
	return h
}

// handleHealth returns health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

// handleProcesses lists all processes
func (s *Server) handleProcesses(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// List all processes
		processes := s.manager.ListProcesses()
		s.respondJSON(w, http.StatusOK, map[string]interface{}{
			"processes": processes,
		})

	case http.MethodPost:
		// Add new process
		s.handleAddProcess(w, r)

	default:
		s.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleProcessAction handles process-specific actions
func (s *Server) handleProcessAction(w http.ResponseWriter, r *http.Request) {
	// Support GET (logs), POST (actions), PUT (update), DELETE (remove)
	if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodDelete {
		s.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse path: /api/v1/processes/{name}/{action}
	path := r.URL.Path
	var processName, action string

	// Robust path parsing with validation
	if len(path) <= len("/api/v1/processes/") {
		s.respondError(w, http.StatusBadRequest, "invalid path")
		return
	}

	pathParts := path[len("/api/v1/processes/"):]

	// Find first slash to split name and action
	idx := -1
	for i, c := range pathParts {
		if c == '/' {
			idx = i
			break
		}
	}

	if idx > 0 && idx < len(pathParts)-1 {
		processName = pathParts[:idx]
		action = pathParts[idx+1:]
	} else if idx == -1 {
		// No slash - just process name (no action)
		processName = pathParts
		action = ""
	}

	// Validate process name
	if processName == "" {
		s.respondError(w, http.StatusBadRequest, "process name required")
		return
	}

	// Handle HTTP methods
	switch r.Method {
	case http.MethodGet:
		// GET /api/v1/processes/{name} - get process config
		// GET /api/v1/processes/{name}/logs - get process logs
		if action == "" {
			s.handleGetProcess(w, r, processName)
		} else if action == "logs" {
			s.handleGetLogs(w, r, processName)
		} else {
			s.respondError(w, http.StatusBadRequest, "unknown GET action")
		}
		return

	case http.MethodPut:
		// PUT /api/v1/processes/{name} - update process
		s.handleUpdateProcess(w, r, processName)
		return

	case http.MethodDelete:
		// DELETE /api/v1/processes/{name} - remove process
		s.handleRemoveProcess(w, r, processName)
		return

	case http.MethodPost:
		// POST /api/v1/processes/{name}/{action} - perform action
		// Validate action for POST
		if action == "" {
			s.respondError(w, http.StatusBadRequest, "action required (start|stop|restart|scale)")
			return
		}

		// Route to appropriate handler
		switch action {
		case "restart":
			s.handleRestart(w, r, processName)
		case "stop":
			s.handleStop(w, r, processName)
		case "start":
			s.handleStart(w, r, processName)
		case "scale":
			s.handleScale(w, r, processName)
		default:
			s.respondError(w, http.StatusBadRequest, fmt.Sprintf("unknown action: %s (valid: start|stop|restart|scale)", action))
		}
	}
}

// handleRestart restarts a process
func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request, processName string) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := s.manager.RestartProcess(ctx, processName); err != nil {
		s.respondError(w, httpStatusFromError(err), fmt.Sprintf("restart failed: %v", err))
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]string{
		"status":  "restarted",
		"process": processName,
	})
}

// handleStop stops a process
func (s *Server) handleStop(w http.ResponseWriter, r *http.Request, processName string) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := s.manager.StopProcess(ctx, processName); err != nil {
		s.respondError(w, httpStatusFromError(err), fmt.Sprintf("stop failed: %v", err))
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]string{
		"status":  "stopped",
		"process": processName,
	})
}

// handleStart starts a process
func (s *Server) handleStart(w http.ResponseWriter, r *http.Request, processName string) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := s.manager.StartProcess(ctx, processName); err != nil {
		s.respondError(w, httpStatusFromError(err), fmt.Sprintf("start failed: %v", err))
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]string{
		"status":  "started",
		"process": processName,
	})
}

// handleScale scales a process
func (s *Server) handleScale(w http.ResponseWriter, r *http.Request, processName string) {
	// Parse scale request
	var req struct {
		Desired int  `json:"desired"`
		Delta   *int `json:"delta"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if req.Delta != nil {
		if err := s.manager.AdjustScale(ctx, processName, *req.Delta); err != nil {
			s.respondError(w, httpStatusFromError(err), fmt.Sprintf("scale failed: %v", err))
			return
		}
		s.respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "scaled",
			"process": processName,
			"delta":   *req.Delta,
		})
		return
	}

	if req.Desired < 1 {
		s.respondError(w, http.StatusBadRequest, "desired scale must be >= 1")
		return
	}

	if err := s.manager.ScaleProcess(ctx, processName, req.Desired); err != nil {
		s.respondError(w, httpStatusFromError(err), fmt.Sprintf("scale failed: %v", err))
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "scaled",
		"process": processName,
		"desired": req.Desired,
	})
}

// handleGetLogs retrieves logs for a process
// Supports optional ?limit=N query parameter (default: 100)
func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request, processName string) {
	// Parse optional limit query parameter
	limit := 100 // Default limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Get logs from manager
	logs, err := s.manager.GetLogs(processName, limit)
	if err != nil {
		s.respondError(w, http.StatusNotFound, fmt.Sprintf("failed to get logs: %v", err))
		return
	}

	// Return logs as JSON array
	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"process": processName,
		"limit":   limit,
		"count":   len(logs),
		"logs":    logs,
	})
}

// handleGetProcess returns process configuration details
func (s *Server) handleGetProcess(w http.ResponseWriter, r *http.Request, processName string) {
	cfg, err := s.manager.GetProcessConfig(processName)
	if err != nil {
		s.respondError(w, http.StatusNotFound, fmt.Sprintf("failed to get process: %v", err))
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"process": processName,
		"config":  cfg,
	})
}

// handleStackLogs aggregates logs across the entire stack
func (s *Server) handleStackLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	logs := s.manager.GetStackLogs(limit)

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"scope": "stack",
		"limit": limit,
		"count": len(logs),
		"logs":  logs,
	})
}

// handleAddProcess adds a new process
func (s *Server) handleAddProcess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string          `json:"name"`
		Process *config.Process `json:"process"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	// Validate required fields
	if req.Name == "" {
		s.respondError(w, http.StatusBadRequest, "process name is required")
		return
	}
	if req.Process == nil {
		s.respondError(w, http.StatusBadRequest, "process configuration is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := s.manager.AddProcess(ctx, req.Name, req.Process); err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to add process: %v", err))
		return
	}

	s.respondJSON(w, http.StatusCreated, map[string]interface{}{
		"status":  "created",
		"process": req.Name,
		"message": "Process added successfully",
	})
}

// handleUpdateProcess updates an existing process
func (s *Server) handleUpdateProcess(w http.ResponseWriter, r *http.Request, processName string) {
	var req struct {
		Process *config.Process `json:"process"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	if req.Process == nil {
		s.respondError(w, http.StatusBadRequest, "process configuration is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := s.manager.UpdateProcess(ctx, processName, req.Process); err != nil {
		s.respondError(w, httpStatusFromError(err), fmt.Sprintf("failed to update process: %v", err))
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "updated",
		"process": processName,
		"message": "Process updated successfully",
	})
}

// handleRemoveProcess removes a process
func (s *Server) handleRemoveProcess(w http.ResponseWriter, r *http.Request, processName string) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := s.manager.RemoveProcess(ctx, processName); err != nil {
		s.respondError(w, httpStatusFromError(err), fmt.Sprintf("failed to remove process: %v", err))
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "removed",
		"process": processName,
		"message": "Process removed successfully",
	})
}

// handleConfigSave saves the current configuration to file
func (s *Server) handleConfigSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := s.manager.SaveConfig(); err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save config: %v", err))
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "saved",
		"message": "Configuration saved successfully",
	})
}

// handleConfigReload reloads the configuration from file
func (s *Server) handleConfigReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	if err := s.manager.ReloadConfig(ctx); err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to reload config: %v", err))
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "reloaded",
		"message": "Configuration reloaded successfully",
	})
}

// respondJSON sends a JSON response
func (s *Server) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("Failed to encode JSON response", "error", err)
	}
}

// respondError sends an error response
func (s *Server) respondError(w http.ResponseWriter, status int, message string) {
	s.respondJSON(w, status, map[string]string{
		"error": message,
	})
}

func httpStatusFromError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	lowered := strings.ToLower(err.Error())
	if strings.Contains(lowered, "not found") || strings.Contains(lowered, "does not exist") {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

// Port returns the port the server is listening on
func (s *Server) Port() int {
	return s.port
}

// handleMetricsHistory returns historical resource metrics for a process instance
// GET /api/v1/metrics/history?process=name&instance=id&since=timestamp&limit=N
func (s *Server) handleMetricsHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if resource metrics are enabled
	collector := s.manager.GetResourceCollector()
	if collector == nil {
		http.Error(w, `{"error":"Resource metrics not enabled"}`, http.StatusServiceUnavailable)
		return
	}

	// Parse query parameters
	query := r.URL.Query()
	processName := query.Get("process")
	instanceID := query.Get("instance")

	if processName == "" || instanceID == "" {
		http.Error(w, `{"error":"Missing required parameters: process and instance"}`, http.StatusBadRequest)
		return
	}

	// Parse optional parameters
	var since time.Time
	if sinceStr := query.Get("since"); sinceStr != "" {
		sinceInt, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			// Try Unix timestamp
			var unixTime int64
			if _, err := fmt.Sscanf(sinceStr, "%d", &unixTime); err != nil {
				http.Error(w, `{"error":"Invalid since parameter format (use RFC3339 or Unix timestamp)"}`, http.StatusBadRequest)
				return
			}
			since = time.Unix(unixTime, 0)
		} else {
			since = sinceInt
		}
	} else {
		// Default: last hour
		since = time.Now().Add(-1 * time.Hour)
	}

	limit := 100 // default limit
	if limitStr := query.Get("limit"); limitStr != "" {
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil || limit <= 0 || limit > 10000 {
			http.Error(w, `{"error":"Invalid limit parameter (must be 1-10000)"}`, http.StatusBadRequest)
			return
		}
	}

	// Get metrics history
	history := collector.GetHistory(processName, instanceID, since, limit)

	// Build response
	response := map[string]interface{}{
		"process":  processName,
		"instance": instanceID,
		"since":    since.Format(time.RFC3339),
		"limit":    limit,
		"samples":  len(history),
		"data":     history,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
