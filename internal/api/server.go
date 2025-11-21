package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gophpeek/phpeek-pm/internal/process"
)

// Server provides REST API for process management
type Server struct {
	port    int
	auth    string
	manager *process.Manager
	server  *http.Server
	logger  *slog.Logger
}

// NewServer creates a new API server
func NewServer(port int, auth string, manager *process.Manager, log *slog.Logger) *Server {
	return &Server{
		port:    port,
		auth:    auth,
		manager: manager,
		logger:  log,
	}
}

// Start starts the API server
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/processes", s.authMiddleware(s.handleProcesses))
	mux.HandleFunc("/api/v1/processes/", s.authMiddleware(s.handleProcessAction))

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.logger.Info("Starting API server",
		"port", s.port,
	)

	// Start server in background
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("API server failed", "error", err)
		}
	}()

	return nil
}

// Stop gracefully stops the API server
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	s.logger.Info("Stopping API server")

	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Error("Failed to stop API server gracefully", "error", err)
		return err
	}

	s.logger.Info("API server stopped")
	return nil
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
			s.respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		next(w, r)
	}
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
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	processes := s.manager.ListProcesses()
	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"processes": processes,
	})
}

// handleProcessAction handles process-specific actions
func (s *Server) handleProcessAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse path: /api/v1/processes/{name}/{action}
	path := r.URL.Path
	var processName, action string

	// Simple path parsing
	if len(path) > len("/api/v1/processes/") {
		pathParts := path[len("/api/v1/processes/"):]
		// Find first slash
		idx := 0
		for i, c := range pathParts {
			if c == '/' {
				idx = i
				break
			}
		}
		if idx > 0 {
			processName = pathParts[:idx]
			action = pathParts[idx+1:]
		} else {
			processName = pathParts
		}
	}

	if processName == "" {
		s.respondError(w, http.StatusBadRequest, "process name required")
		return
	}

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
		s.respondError(w, http.StatusBadRequest, "unknown action")
	}
}

// handleRestart restarts a process
func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request, processName string) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := s.manager.RestartProcess(ctx, processName); err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("restart failed: %v", err))
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
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("stop failed: %v", err))
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
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("start failed: %v", err))
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
		Desired int `json:"desired"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Desired < 1 {
		s.respondError(w, http.StatusBadRequest, "desired scale must be >= 1")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := s.manager.ScaleProcess(ctx, processName, req.Desired); err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("scale failed: %v", err))
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "scaled",
		"process": processName,
		"desired": req.Desired,
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

// Port returns the port the server is listening on
func (s *Server) Port() int {
	return s.port
}
