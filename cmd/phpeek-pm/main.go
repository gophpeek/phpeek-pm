package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gophpeek/phpeek-pm/internal/api"
	"github.com/gophpeek/phpeek-pm/internal/config"
	"github.com/gophpeek/phpeek-pm/internal/framework"
	"github.com/gophpeek/phpeek-pm/internal/logger"
	"github.com/gophpeek/phpeek-pm/internal/metrics"
	"github.com/gophpeek/phpeek-pm/internal/process"
	"github.com/gophpeek/phpeek-pm/internal/setup"
	"github.com/gophpeek/phpeek-pm/internal/signals"
)

const version = "1.0.0"

func main() {
	// PHPeek branding banner
	fmt.Fprintf(os.Stderr, "\n🚀 PHPeek Process Manager v%s\n", version)
	fmt.Fprintf(os.Stderr, "   Production-grade process supervisor for Docker containers\n\n")

	// Phase 1.5: Determine working directory
	workdir := os.Getenv("WORKDIR")
	if workdir == "" {
		workdir = "/var/www/html"
	}

	// Phase 1.5: Detect framework
	fw := framework.Detect(workdir)
	fmt.Fprintf(os.Stderr, "📦 Detected framework: %s (workdir: %s)\n", fw, workdir)

	// Phase 1.5: Setup permissions
	permMgr := setup.NewPermissionManager(workdir, fw, slog.Default())
	if err := permMgr.Setup(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Permission setup completed with warnings: %v\n", err)
	}

	// Phase 1.5: Validate configurations
	validator := setup.NewConfigValidator(slog.Default())
	if err := validator.ValidateAll(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Configuration validation failed: %v\n", err)
		os.Exit(1)
	}

	// Load configuration with environment variable expansion
	configPath := os.Getenv("PHPEEK_PM_CONFIG")
	if configPath == "" {
		configPath = "/etc/phpeek-pm/phpeek-pm.yaml"
		// Fallback to local config for development
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			configPath = "phpeek-pm.yaml"
		}
	}

	cfg, err := config.LoadWithEnvExpansion(configPath)
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
		"framework", fw,
		"workdir", workdir,
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

	// Start metrics server if enabled
	var metricsServer *metrics.Server
	if cfg.Global.MetricsEnabled {
		metricsPort := cfg.Global.MetricsPort
		if metricsPort == 0 {
			metricsPort = 9090 // Default metrics port
		}
		metricsPath := cfg.Global.MetricsPath
		if metricsPath == "" {
			metricsPath = "/metrics" // Default metrics path
		}

		metricsServer = metrics.NewServer(metricsPort, metricsPath, log)
		if err := metricsServer.Start(ctx); err != nil {
			slog.Error("Failed to start metrics server", "error", err)
			os.Exit(1)
		}
		slog.Info("Metrics server started",
			"port", metricsPort,
			"path", metricsPath,
		)

		// Set build info metric
		metrics.SetBuildInfo(version, "go1.x")
	}

	// Start all processes
	if err := pm.Start(ctx); err != nil {
		slog.Error("Failed to start processes", "error", err)
		os.Exit(1)
	}

	slog.Info("All processes started successfully")

	// Start monitoring for all processes dying
	pm.MonitorProcessHealth(ctx)

	// Start API server if enabled
	var apiServer *api.Server
	if cfg.Global.APIEnabled {
		apiPort := cfg.Global.APIPort
		if apiPort == 0 {
			apiPort = 8080 // Default API port
		}

		apiServer = api.NewServer(apiPort, cfg.Global.APIAuth, pm, log)
		if err := apiServer.Start(ctx); err != nil {
			slog.Error("Failed to start API server", "error", err)
			os.Exit(1)
		}
		slog.Info("API server started",
			"port", apiPort,
			"auth", cfg.Global.APIAuth != "",
		)
	}

	// Wait for either shutdown signal or all processes dying
	var shutdownReason string
	select {
	case sig := <-sigChan:
		shutdownReason = fmt.Sprintf("signal: %s", sig.String())
		slog.Info("Received shutdown signal", "signal", sig.String())
	case <-pm.AllDeadChannel():
		shutdownReason = "all processes died"
		slog.Warn("All managed processes have died")
	}

	// Initiate graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		time.Duration(cfg.Global.ShutdownTimeout)*time.Second,
	)
	defer shutdownCancel()

	slog.Info("Initiating graceful shutdown",
		"reason", shutdownReason,
		"timeout", cfg.Global.ShutdownTimeout,
	)

	if err := pm.Shutdown(shutdownCtx); err != nil {
		slog.Error("Shutdown completed with errors", "error", err)
		os.Exit(1)
	}

	// Stop API server if running
	if apiServer != nil {
		if err := apiServer.Stop(shutdownCtx); err != nil {
			slog.Warn("API server shutdown error", "error", err)
		}
	}

	// Stop metrics server if running
	if metricsServer != nil {
		if err := metricsServer.Stop(shutdownCtx); err != nil {
			slog.Warn("Metrics server shutdown error", "error", err)
		}
	}

	slog.Info("PHPeek PM shutdown complete")
}
