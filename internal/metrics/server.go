package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server serves Prometheus metrics
type Server struct {
	port   int
	path   string
	server *http.Server
	logger *slog.Logger
}

// NewServer creates a new metrics server
func NewServer(port int, path string, log *slog.Logger) *Server {
	if path == "" {
		path = "/metrics"
	}

	return &Server{
		port:   port,
		path:   path,
		logger: log,
	}
}

// Start starts the metrics server
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Prometheus metrics endpoint
	mux.Handle(s.path, promhttp.Handler())

	// Health endpoint for the metrics server itself
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.logger.Info("Starting metrics server",
		"port", s.port,
		"path", s.path,
	)

	// Start server in background
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Metrics server failed", "error", err)
		}
	}()

	return nil
}

// Stop gracefully stops the metrics server
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	s.logger.Info("Stopping metrics server")

	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Error("Failed to stop metrics server gracefully", "error", err)
		return err
	}

	s.logger.Info("Metrics server stopped")
	return nil
}

// Port returns the port the server is listening on
func (s *Server) Port() int {
	return s.port
}
