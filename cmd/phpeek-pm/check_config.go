package main

import (
	"fmt"
	"os"

	"github.com/gophpeek/phpeek-pm/internal/autotune"
	"github.com/gophpeek/phpeek-pm/internal/config"
	"github.com/spf13/cobra"
)

var checkConfigCmd = &cobra.Command{
	Use:   "check-config",
	Short: "Validate configuration file",
	Long:  `Validate the PHPeek PM configuration file and report any errors or warnings`,
	Run:   runCheckConfig,
}

func init() {
	checkConfigCmd.Flags().Bool("strict", false, "Fail on warnings (not just errors)")
}

func runCheckConfig(cmd *cobra.Command, args []string) {
	strict, _ := cmd.Flags().GetBool("strict")

	// Get config path from persistent flag or default
	cfgPath := getConfigPath()

	// Load configuration
	cfg, err := config.LoadWithEnvExpansion(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Configuration load failed: %v\n", err)
		os.Exit(1)
	}

	// Apply defaults and validate
	cfg.SetDefaults()
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Configuration validation failed: %v\n", err)
		os.Exit(1)
	}

	// Check for auto-tuning profile if specified
	autotuneProfile := os.Getenv("PHP_FPM_AUTOTUNE_PROFILE")
	if cfg.Global.AutotuneMemoryThreshold > 0 && autotuneProfile != "" {
		profile := autotune.Profile(autotuneProfile)
		if err := profile.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Invalid PHP-FPM profile: %v\n", err)
			os.Exit(1)
		}
	}

	// Print validation success
	fmt.Printf("✅ Configuration is valid: %s\n", cfgPath)
	fmt.Printf("   Version: %s\n", cfg.Version)
	fmt.Printf("   Processes: %d\n", len(cfg.Processes))
	fmt.Printf("   Log Level: %s\n", cfg.Global.LogLevel)
	fmt.Printf("   Shutdown Timeout: %ds\n", cfg.Global.ShutdownTimeout)

	if autotuneProfile != "" {
		fmt.Printf("   PHP-FPM Profile: %s (auto-tuned)\n", autotuneProfile)
	}

	// Check for warnings (future)
	warnings := []string{}

	// Report warnings if any
	if len(warnings) > 0 {
		fmt.Println("\n⚠️  Warnings:")
		for _, w := range warnings {
			fmt.Printf("   - %s\n", w)
		}

		if strict {
			fmt.Println("\n❌ Validation failed in strict mode (warnings present)")
			os.Exit(1)
		}
	}

	fmt.Println("\n✅ Configuration ready for use")
}

// getConfigPath determines configuration file path
func getConfigPath() string {
	// Try persistent flag (set by root command)
	if cfgFile != "" {
		return cfgFile
	}

	// Try environment variable
	if envPath := os.Getenv("PHPEEK_PM_CONFIG"); envPath != "" {
		return envPath
	}

	// Default paths
	defaultPath := "/etc/phpeek-pm/phpeek-pm.yaml"
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}

	// Fallback to local
	return "phpeek-pm.yaml"
}
