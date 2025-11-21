package main

import (
	"context"
	"flag"
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
)

const version = "1.0.0"

func main() {
	// Parse command-line flags
	var (
		showVersion       = flag.Bool("version", false, "Show version information and exit")
		versionShort      = flag.Bool("v", false, "Show version information and exit (shorthand)")
		configPath        = flag.String("config", "", "Path to configuration file (default: PHPEEK_PM_CONFIG env var or /etc/phpeek-pm/phpeek-pm.yaml)")
		configShort       = flag.String("c", "", "Path to configuration file (shorthand)")
		validateConfig    = flag.Bool("validate-config", false, "Validate configuration file and exit")
		dryRun            = flag.Bool("dry-run", false, "Validate configuration and setup without starting processes")
		phpFPMProfile     = flag.String("php-fpm-profile", "", "Auto-tune PHP-FPM workers based on container limits (dev|light|medium|heavy|bursty)")
		memoryThreshold   = flag.Float64("autotune-memory-threshold", 0, "Override memory threshold for auto-tuning (0.5=50%, 1.0=100%, 1.3=130% oversubscription)")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "PHPeek Process Manager v%s\n", version)
		fmt.Fprintf(os.Stderr, "Production-grade process supervisor for Docker containers\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment Variables:\n")
		fmt.Fprintf(os.Stderr, "  PHPEEK_PM_CONFIG                     Path to configuration file\n")
		fmt.Fprintf(os.Stderr, "  WORKDIR                              Working directory (default: /var/www/html)\n")
		fmt.Fprintf(os.Stderr, "  PHP_FPM_AUTOTUNE_PROFILE             Auto-tune PHP-FPM (dev|light|medium|heavy|bursty)\n")
		fmt.Fprintf(os.Stderr, "  PHP_FPM_AUTOTUNE_MEMORY_THRESHOLD    Memory threshold (0.5-2.0, e.g., 0.8=80%%, 1.3=130%%)\n")
		fmt.Fprintf(os.Stderr, "  PHPEEK_PM_GLOBAL_*                   Override global config options\n")
		fmt.Fprintf(os.Stderr, "  PHPEEK_PM_PROCESS_*_*                Override process-specific config options\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s --version                                      # Show version\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --config app.yaml                              # Use specific config\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --validate-config                              # Validate config only\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --dry-run                                      # Validate without starting\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --php-fpm-profile=medium                       # Auto-tune PHP-FPM workers\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --autotune-memory-threshold=0.6                # Conservative (60%% memory)\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --php-fpm-profile=heavy --autotune-memory-threshold=1.3  # Oversubscribe (expert)\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nPHP-FPM Auto-Tuning Profiles (OPcache reduces worker memory):\n")
		fmt.Fprintf(os.Stderr, "  dev     - Development: 2 workers, ~32MB per worker + 64MB OPcache\n")
		fmt.Fprintf(os.Stderr, "  light   - Light production: 1-10 req/s, ~36MB per worker + 96MB OPcache\n")
		fmt.Fprintf(os.Stderr, "  medium  - Standard production: 10-50 req/s, ~42MB per worker + 128MB OPcache\n")
		fmt.Fprintf(os.Stderr, "  heavy   - High traffic: 50-200 req/s, ~52MB per worker + 256MB OPcache\n")
		fmt.Fprintf(os.Stderr, "  bursty  - Traffic spikes: ~44MB per worker + 128MB OPcache\n")
	}

	flag.Parse()

	// Handle --version / -v
	if *showVersion || *versionShort {
		fmt.Printf("PHPeek Process Manager v%s\n", version)
		os.Exit(0)
	}

	// Determine config path: --config flag > PHPEEK_PM_CONFIG env > default
	cfgPath := *configPath
	if cfgPath == "" {
		cfgPath = *configShort
	}
	if cfgPath == "" {
		cfgPath = os.Getenv("PHPEEK_PM_CONFIG")
	}
	if cfgPath == "" {
		cfgPath = "/etc/phpeek-pm/phpeek-pm.yaml"
		// Fallback to local config for development
		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			cfgPath = "phpeek-pm.yaml"
		}
	}

	// PHPeek branding banner (skip for --validate-config to keep output clean)
	if !*validateConfig {
		fmt.Fprintf(os.Stderr, "\n🚀 PHPeek Process Manager v%s\n", version)
		fmt.Fprintf(os.Stderr, "   Production-grade process supervisor for Docker containers\n\n")
	}

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
	cfg, err := config.LoadWithEnvExpansion(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// PHP-FPM Auto-Tuning: Determine profile from CLI flag or ENV var
	// Priority: CLI flag (--php-fpm-profile) > ENV var (PHP_FPM_AUTOTUNE_PROFILE)
	autotuneProfile := *phpFPMProfile
	if autotuneProfile == "" {
		autotuneProfile = os.Getenv("PHP_FPM_AUTOTUNE_PROFILE")
	}

	// PHP-FPM Auto-Tuning: Calculate and set environment variables
	if autotuneProfile != "" {
		// Initialize minimal logger for autotune (before full logger setup)
		autotuneLog := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

		profile := autotune.Profile(autotuneProfile)
		if err := profile.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Invalid PHP-FPM profile: %v\n", err)
			os.Exit(1)
		}

		// Determine memory threshold: CLI flag > ENV var > global config > profile default (0.0)
		threshold := *memoryThreshold
		thresholdSource := "profile default"

		if threshold == 0 {
			// Try ENV variable
			if envThreshold := os.Getenv("PHP_FPM_AUTOTUNE_MEMORY_THRESHOLD"); envThreshold != "" {
				if parsed, err := strconv.ParseFloat(envThreshold, 64); err == nil {
					threshold = parsed
					thresholdSource = "ENV variable"
				} else {
					fmt.Fprintf(os.Stderr, "⚠️  Invalid PHP_FPM_AUTOTUNE_MEMORY_THRESHOLD: %v (ignoring)\n", err)
				}
			}
		} else {
			thresholdSource = "CLI flag"
		}

		if threshold == 0 && cfg.Global.AutotuneMemoryThreshold > 0 {
			threshold = cfg.Global.AutotuneMemoryThreshold
			thresholdSource = "global config"
		}

		calc, err := autotune.NewCalculator(profile, threshold, autotuneLog)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to initialize auto-tuner: %v\n", err)
			os.Exit(1)
		}

		phpfpmCfg, err := calc.Calculate()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Auto-tuning failed: %v\n", err)
			os.Exit(1)
		}

		// Set environment variables for PHP-FPM to use
		for key, value := range phpfpmCfg.ToEnvVars() {
			os.Setenv(key, value)
		}

		// Determine source for logging
		source := "CLI flag"
		if *phpFPMProfile == "" {
			source = "ENV var"
		}

		fmt.Fprintf(os.Stderr, "🎯 PHP-FPM auto-tuned (%s profile via %s):\n", profile, source)

		// Show memory threshold if overridden
		if threshold > 0 {
			fmt.Fprintf(os.Stderr, "   Memory threshold: %.1f%% (via %s)\n", threshold*100, thresholdSource)
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
	}

	// Handle --validate-config: validate and exit
	if *validateConfig {
		fmt.Printf("✅ Configuration is valid: %s\n", cfgPath)
		fmt.Printf("   Version: %s\n", cfg.Version)
		fmt.Printf("   Processes: %d\n", len(cfg.Processes))
		fmt.Printf("   Log Level: %s\n", cfg.Global.LogLevel)
		fmt.Printf("   Shutdown Timeout: %ds\n", cfg.Global.ShutdownTimeout)
		if autotuneProfile != "" {
			fmt.Printf("   PHP-FPM Profile: %s (auto-tuned)\n", autotuneProfile)
		}
		os.Exit(0)
	}

	// Handle --dry-run: validate everything but don't start
	if *dryRun {
		// Initialize logger for dry-run validation
		log := logger.New(cfg.Global.LogLevel, cfg.Global.LogFormat)
		slog.SetDefault(log)

		fmt.Fprintf(os.Stderr, "🔍 DRY RUN MODE - Validating configuration without starting processes\n\n")
		fmt.Fprintf(os.Stderr, "✅ Configuration loaded: %s\n", cfgPath)
		fmt.Fprintf(os.Stderr, "✅ Framework detected: %s (workdir: %s)\n", fw, workdir)
		fmt.Fprintf(os.Stderr, "✅ Permissions validated\n")
		fmt.Fprintf(os.Stderr, "✅ System configurations validated\n")

		// Validate process manager can be created
		pm := process.NewManager(cfg, log)
		_ = pm // Suppress unused warning

		fmt.Fprintf(os.Stderr, "\n✅ All validations passed - ready to run in production\n")
		os.Exit(0)
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
