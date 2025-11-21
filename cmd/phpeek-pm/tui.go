package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/gophpeek/phpeek-pm/internal/config"
	"github.com/gophpeek/phpeek-pm/internal/framework"
	"github.com/gophpeek/phpeek-pm/internal/logger"
	"github.com/gophpeek/phpeek-pm/internal/process"
	"github.com/gophpeek/phpeek-pm/internal/setup"
	"github.com/gophpeek/phpeek-pm/internal/signals"
	"github.com/gophpeek/phpeek-pm/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive terminal dashboard",
	Long: `Launch a k9s-style interactive terminal dashboard for managing processes.

The TUI provides:
- Real-time process status monitoring
- Interactive log viewing
- Process control (restart/stop/start/scale)
- Dynamic log level changes
- Keyboard-driven interface

The TUI starts the process manager in embedded mode and provides an interactive
interface for monitoring and control.`,
	Run: runTUI,
}

var (
	tuiRemote string
)

func init() {
	tuiCmd.Flags().StringVar(&tuiRemote, "remote", "", "Connect to remote API (e.g., http://localhost:8080 or unix:///var/run/phpeek-pm.sock)")
}

func runTUI(cmd *cobra.Command, args []string) {
	// Remote mode: Connect to running daemon via API
	if tuiRemote != "" {
		runTUIRemote(tuiRemote)
		return
	}

	// Embedded mode: Start manager + TUI in same process
	runTUIEmbedded()
}

func runTUIRemote(apiURL string) {
	// TODO: Implement remote mode
	// This will connect to API and use HTTP client instead of direct manager access
	fmt.Fprintf(os.Stderr, "🔗 Connecting to remote API: %s\n", apiURL)
	fmt.Fprintf(os.Stderr, "⚠️  Remote mode not yet implemented (coming in Phase 3)\n")
	fmt.Fprintf(os.Stderr, "💡 For now, use embedded mode (no --remote flag)\n")
	os.Exit(1)
}

func runTUIEmbedded() {
	// Get config path
	cfgPath := getConfigPath()

	// Setup environment
	workdir := os.Getenv("WORKDIR")
	if workdir == "" {
		workdir = "/var/www/html"
	}

	fw := framework.Detect(workdir)

	// Setup permissions
	permMgr := setup.NewPermissionManager(workdir, fw, slog.Default())
	_ = permMgr.Setup()

	// Validate system
	validator := setup.NewConfigValidator(slog.Default())
	_ = validator.ValidateAll()

	// Load configuration
	cfg, err := config.LoadWithEnvExpansion(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger (error level to reduce TUI noise)
	log := logger.New("error", "json")
	slog.SetDefault(log)

	// Create process manager
	pm := process.NewManager(cfg, log)

	// Start zombie reaper
	go signals.ReapZombies()

	// Start processes
	ctx := context.Background()
	if err := pm.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to start processes: %v\n", err)
		os.Exit(1)
	}

	// Monitor process health
	pm.MonitorProcessHealth(ctx)

	// Launch TUI
	if err := tui.Run(pm); err != nil {
		fmt.Fprintf(os.Stderr, "❌ TUI error: %v\n", err)
		os.Exit(1)
	}

	// Shutdown when TUI exits
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Global.ShutdownTimeout)*time.Second)
	defer cancel()

	if err := pm.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Shutdown completed with errors: %v\n", err)
	}
}
