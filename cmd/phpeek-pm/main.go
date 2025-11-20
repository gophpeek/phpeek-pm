package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gophpeek/phpeek-pm/internal/config"
	"github.com/gophpeek/phpeek-pm/internal/logger"
	"github.com/gophpeek/phpeek-pm/internal/process"
	"github.com/gophpeek/phpeek-pm/internal/signals"
)

const version = "1.0.0"

func main() {
	// PHPeek branding banner
	fmt.Fprintf(os.Stderr, "\n🚀 PHPeek Process Manager v%s\n", version)
	fmt.Fprintf(os.Stderr, "   Production-grade process supervisor for Docker containers\n\n")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured logger
	log := logger.New(cfg.Global.LogLevel, cfg.Global.LogFormat)
	slog.SetDefault(log)

	slog.Info("PHPeek PM starting",
		"version", version,
		"pid", os.Getpid(),
		"log_level", cfg.Global.LogLevel,
		"processes", len(cfg.Processes),
	)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling (PID 1 capable)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	// Start zombie reaper (critical for PID 1)
	go signals.ReapZombies()

	// Create process manager
	pm := process.NewManager(cfg, log)

	// Start all processes
	if err := pm.Start(ctx); err != nil {
		slog.Error("Failed to start processes", "error", err)
		os.Exit(1)
	}

	slog.Info("All processes started successfully")

	// Wait for shutdown signal
	sig := <-sigChan
	slog.Info("Received shutdown signal", "signal", sig.String())

	// Initiate graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		time.Duration(cfg.Global.ShutdownTimeout)*time.Second,
	)
	defer shutdownCancel()

	slog.Info("Initiating graceful shutdown",
		"timeout", cfg.Global.ShutdownTimeout,
	)

	if err := pm.Shutdown(shutdownCtx); err != nil {
		slog.Error("Shutdown completed with errors", "error", err)
		os.Exit(1)
	}

	slog.Info("PHPeek PM shutdown complete")
}
