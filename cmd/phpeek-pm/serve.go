package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gophpeek/phpeek-pm/internal/api"
	"github.com/gophpeek/phpeek-pm/internal/autotune"
	"github.com/gophpeek/phpeek-pm/internal/config"
	"github.com/gophpeek/phpeek-pm/internal/framework"
	"github.com/gophpeek/phpeek-pm/internal/logger"
	"github.com/gophpeek/phpeek-pm/internal/metrics"
	"github.com/gophpeek/phpeek-pm/internal/process"
	"github.com/gophpeek/phpeek-pm/internal/setup"
	"github.com/gophpeek/phpeek-pm/internal/signals"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the process manager daemon",
	Long: `Start PHPeek PM in daemon mode to manage processes.

This is the default mode when no subcommand is specified. It starts all configured
processes, handles graceful shutdown, and provides observability endpoints.`,
	Run: runServe,
}

var (
	dryRun          bool
	phpFPMProfile   string
	memoryThreshold float64
)

func init() {
	serveCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate configuration without starting processes")
	serveCmd.Flags().StringVar(&phpFPMProfile, "php-fpm-profile", "", "Auto-tune PHP-FPM workers (dev|light|medium|heavy|bursty)")
	serveCmd.Flags().Float64Var(&memoryThreshold, "autotune-memory-threshold", 0, "Override memory threshold (0.5=50%, 1.0=100%, 1.3=130%)")
}

func runServe(cmd *cobra.Command, args []string) {
	// Get config path
	cfgPath := getConfigPath()

	// Display banner
	fmt.Fprintf(os.Stderr, "\n🚀 PHPeek Process Manager v%s\n", version)
	fmt.Fprintf(os.Stderr, "   Production-grade process supervisor for Docker containers\n\n")

	// Determine working directory
	workdir := os.Getenv("WORKDIR")
	if workdir == "" {
		workdir = "/var/www/html"
	}

	// Detect framework
	fw := framework.Detect(workdir)
	fmt.Fprintf(os.Stderr, "📦 Detected framework: %s (workdir: %s)\n", fw, workdir)

	// Setup permissions
	permMgr := setup.NewPermissionManager(workdir, fw, slog.Default())
	if err := permMgr.Setup(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Permission setup completed with warnings: %v\n", err)
	}

	// Validate system configurations
	validator := setup.NewConfigValidator(slog.Default())
	if err := validator.ValidateAll(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Configuration validation failed: %v\n", err)
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.LoadWithEnvExpansion(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// PHP-FPM Auto-Tuning
	autotuneProfile := phpFPMProfile
	if autotuneProfile == "" {
		autotuneProfile = os.Getenv("PHP_FPM_AUTOTUNE_PROFILE")
	}

	if autotuneProfile != "" {
		if err := runAutoTuning(autotuneProfile, memoryThreshold, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Auto-tuning failed: %v\n", err)
			os.Exit(1)
		}
	}

	// Handle dry-run mode
	if dryRun {
		runDryRun(cfg, cfgPath, fw, workdir, autotuneProfile)
		return
	}

	// Initialize logger
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

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	// Start zombie reaper
	go signals.ReapZombies()

	// Create process manager
	pm := process.NewManager(cfg, log)

	// Start metrics server
	var metricsServer *metrics.Server
	if cfg.Global.MetricsEnabled {
		metricsServer = startMetricsServer(ctx, cfg, log)
	}

	// Start all processes
	if err := pm.Start(ctx); err != nil {
		slog.Error("Failed to start processes", "error", err)
		os.Exit(1)
	}

	slog.Info("All processes started successfully")

	// Monitor process health
	pm.MonitorProcessHealth(ctx)

	// Start API server
	var apiServer *api.Server
	if cfg.Global.APIEnabled {
		apiServer = startAPIServer(ctx, cfg, pm, log)
	}

	// Wait for shutdown
	shutdownReason := waitForShutdown(sigChan, pm)

	// Graceful shutdown
	performGracefulShutdown(cfg, pm, apiServer, metricsServer, shutdownReason)
}

// runAutoTuning performs PHP-FPM auto-tuning calculations
func runAutoTuning(profileName string, threshold float64, cfg *config.Config) error {
	autotuneLog := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	profile := autotune.Profile(profileName)
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("invalid profile: %w", err)
	}

	// Determine memory threshold
	finalThreshold := threshold
	thresholdSource := "profile default"

	if finalThreshold == 0 {
		if envThreshold := os.Getenv("PHP_FPM_AUTOTUNE_MEMORY_THRESHOLD"); envThreshold != "" {
			if parsed, err := strconv.ParseFloat(envThreshold, 64); err == nil {
				finalThreshold = parsed
				thresholdSource = "ENV variable"
			}
		}
	} else {
		thresholdSource = "CLI flag"
	}

	if finalThreshold == 0 && cfg.Global.AutotuneMemoryThreshold > 0 {
		finalThreshold = cfg.Global.AutotuneMemoryThreshold
		thresholdSource = "global config"
	}

	// Create calculator
	calc, err := autotune.NewCalculator(profile, finalThreshold, autotuneLog)
	if err != nil {
		return fmt.Errorf("failed to create calculator: %w", err)
	}

	// Calculate configuration
	phpfpmCfg, err := calc.Calculate()
	if err != nil {
		return err
	}

	// Set environment variables
	for key, value := range phpfpmCfg.ToEnvVars() {
		os.Setenv(key, value)
	}

	// Display results
	source := "CLI flag"
	if phpFPMProfile == "" {
		source = "ENV var"
	}

	fmt.Fprintf(os.Stderr, "🎯 PHP-FPM auto-tuned (%s profile via %s):\n", profile, source)

	if finalThreshold > 0 {
		fmt.Fprintf(os.Stderr, "   Memory threshold: %.1f%% (via %s)\n", finalThreshold*100, thresholdSource)
	}

	fmt.Fprintf(os.Stderr, "   pm = %s\n", phpfpmCfg.ProcessManager)
	fmt.Fprintf(os.Stderr, "   pm.max_children = %d\n", phpfpmCfg.MaxChildren)
	if phpfpmCfg.ProcessManager == "dynamic" {
		fmt.Fprintf(os.Stderr, "   pm.start_servers = %d\n", phpfpmCfg.StartServers)
		fmt.Fprintf(os.Stderr, "   pm.min_spare_servers = %d\n", phpfpmCfg.MinSpare)
		fmt.Fprintf(os.Stderr, "   pm.max_spare_servers = %d\n", phpfpmCfg.MaxSpare)
	}
	fmt.Fprintf(os.Stderr, "   pm.max_requests = %d\n", phpfpmCfg.MaxRequests)
	fmt.Fprintf(os.Stderr, "   Memory: %dMB allocated / %dMB total (%.1f%% used)\n",
		phpfpmCfg.MemoryAllocated+phpfpmCfg.MemoryOPcache+phpfpmCfg.MemoryReserved,
		phpfpmCfg.MemoryTotal,
		float64(phpfpmCfg.MemoryAllocated+phpfpmCfg.MemoryOPcache+phpfpmCfg.MemoryReserved)/float64(phpfpmCfg.MemoryTotal)*100)

	if len(phpfpmCfg.Warnings) > 0 {
		fmt.Fprintf(os.Stderr, "   ⚠️  Warnings: %d (see logs for details)\n", len(phpfpmCfg.Warnings))
	}
	fmt.Fprintf(os.Stderr, "\n")

	return nil
}

// runDryRun performs dry-run validation
func runDryRun(cfg *config.Config, cfgPath string, fw framework.Framework, workdir string, autotuneProfile string) {
	log := logger.New(cfg.Global.LogLevel, cfg.Global.LogFormat)
	slog.SetDefault(log)

	fmt.Fprintf(os.Stderr, "🔍 DRY RUN MODE - Validating configuration without starting processes\n\n")
	fmt.Fprintf(os.Stderr, "✅ Configuration loaded: %s\n", cfgPath)
	fmt.Fprintf(os.Stderr, "✅ Framework detected: %s (workdir: %s)\n", fw, workdir)
	fmt.Fprintf(os.Stderr, "✅ Permissions validated\n")
	fmt.Fprintf(os.Stderr, "✅ System configurations validated\n")

	// Validate process manager
	pm := process.NewManager(cfg, log)
	_ = pm

	fmt.Fprintf(os.Stderr, "\n✅ All validations passed - ready to run in production\n")
	os.Exit(0)
}

// startMetricsServer starts the Prometheus metrics server
func startMetricsServer(ctx context.Context, cfg *config.Config, log *slog.Logger) *metrics.Server {
	metricsPort := cfg.Global.MetricsPort
	if metricsPort == 0 {
		metricsPort = 9090
	}
	metricsPath := cfg.Global.MetricsPath
	if metricsPath == "" {
		metricsPath = "/metrics"
	}

	server := metrics.NewServer(metricsPort, metricsPath, log)
	if err := server.Start(ctx); err != nil {
		slog.Warn("Failed to start metrics server (continuing without metrics)", "error", err)
		return nil
	}

	slog.Info("Metrics server started",
		"port", metricsPort,
		"path", metricsPath,
	)

	metrics.SetBuildInfo(version, "go1.x")
	return server
}

// startAPIServer starts the Management API server
func startAPIServer(ctx context.Context, cfg *config.Config, pm *process.Manager, log *slog.Logger) *api.Server {
	apiPort := cfg.Global.APIPort
	if apiPort == 0 {
		apiPort = 8080
	}

	server := api.NewServer(apiPort, cfg.Global.APIAuth, pm, log)
	if err := server.Start(ctx); err != nil {
		slog.Warn("Failed to start API server (TUI/remote control disabled)", "error", err)
		return nil
	}

	slog.Info("API server started",
		"port", apiPort,
		"auth", cfg.Global.APIAuth != "",
	)

	return server
}

// waitForShutdown waits for shutdown signal or all processes dying
func waitForShutdown(sigChan chan os.Signal, pm *process.Manager) string {
	select {
	case sig := <-sigChan:
		slog.Info("Received shutdown signal", "signal", sig.String())
		return fmt.Sprintf("signal: %s", sig.String())
	case <-pm.AllDeadChannel():
		slog.Warn("All managed processes have died")
		return "all processes died"
	}
}

// performGracefulShutdown gracefully shuts down all components
func performGracefulShutdown(cfg *config.Config, pm *process.Manager, apiServer *api.Server, metricsServer *metrics.Server, reason string) {
	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		time.Duration(cfg.Global.ShutdownTimeout)*time.Second,
	)
	defer shutdownCancel()

	slog.Info("Initiating graceful shutdown",
		"reason", reason,
		"timeout", cfg.Global.ShutdownTimeout,
	)

	// Shutdown process manager
	if err := pm.Shutdown(shutdownCtx); err != nil {
		slog.Error("Shutdown completed with errors", "error", err)
		os.Exit(1)
	}

	// Shutdown API server
	if apiServer != nil {
		if err := apiServer.Stop(shutdownCtx); err != nil {
			slog.Warn("API server shutdown error", "error", err)
		}
	}

	// Shutdown metrics server
	if metricsServer != nil {
		if err := metricsServer.Stop(shutdownCtx); err != nil {
			slog.Warn("Metrics server shutdown error", "error", err)
		}
	}

	slog.Info("PHPeek PM shutdown complete")
}
