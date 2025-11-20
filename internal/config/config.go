package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load loads configuration from YAML file and environment variables
// Priority: Environment variables > YAML file > Defaults
func Load() (*Config, error) {
	// Default config path
	configPath := os.Getenv("PHPEEK_PM_CONFIG")
	if configPath == "" {
		configPath = "/etc/phpeek-pm/phpeek-pm.yaml"
		// Fallback to local config if system path doesn't exist
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			configPath = "phpeek-pm.yaml"
		}
	}

	cfg := &Config{
		Processes: make(map[string]*Process),
	}

	// Try to load YAML config
	if _, err := os.Stat(configPath); err == nil {
		if err := loadYAML(configPath, cfg); err != nil {
			return nil, fmt.Errorf("failed to load YAML config: %w", err)
		}
	} else {
		// No config file, use env vars only
		fmt.Fprintf(os.Stderr, "ℹ️  No config file found, using environment variables only\n")
	}

	// Apply defaults
	cfg.SetDefaults()

	// Override with environment variables
	applyEnvOverrides(cfg)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// loadYAML loads configuration from a YAML file
func loadYAML(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return err
	}

	return nil
}

// applyEnvOverrides applies environment variable overrides
// Environment variables follow the pattern: PHPEEK_PM_<SECTION>_<KEY>
func applyEnvOverrides(cfg *Config) {
	// Global settings
	if v := os.Getenv("PHPEEK_PM_GLOBAL_SHUTDOWN_TIMEOUT"); v != "" {
		var timeout int
		if _, err := fmt.Sscanf(v, "%d", &timeout); err == nil {
			cfg.Global.ShutdownTimeout = timeout
		}
	}
	if v := os.Getenv("PHPEEK_PM_GLOBAL_LOG_LEVEL"); v != "" {
		cfg.Global.LogLevel = v
	}
	if v := os.Getenv("PHPEEK_PM_GLOBAL_LOG_FORMAT"); v != "" {
		cfg.Global.LogFormat = v
	}
	if v := os.Getenv("PHPEEK_PM_GLOBAL_METRICS_PORT"); v != "" {
		var port int
		if _, err := fmt.Sscanf(v, "%d", &port); err == nil {
			cfg.Global.MetricsPort = port
		}
	}
	if v := os.Getenv("PHPEEK_PM_GLOBAL_METRICS_ENABLED"); v != "" {
		cfg.Global.MetricsEnabled = v == "true"
	}
	if v := os.Getenv("PHPEEK_PM_GLOBAL_API_PORT"); v != "" {
		var port int
		if _, err := fmt.Sscanf(v, "%d", &port); err == nil {
			cfg.Global.APIPort = port
		}
	}
	if v := os.Getenv("PHPEEK_PM_GLOBAL_API_ENABLED"); v != "" {
		cfg.Global.APIEnabled = v == "true"
	}
	if v := os.Getenv("PHPEEK_PM_GLOBAL_API_AUTH"); v != "" {
		cfg.Global.APIAuth = v
	}

	// Process-specific overrides
	for name, proc := range cfg.Processes {
		envPrefix := fmt.Sprintf("PHPEEK_PM_PROCESS_%s_", strings.ToUpper(strings.ReplaceAll(name, "-", "_")))

		if v := os.Getenv(envPrefix + "ENABLED"); v != "" {
			proc.Enabled = v == "true"
		}
		if v := os.Getenv(envPrefix + "SCALE"); v != "" {
			var scale int
			if _, err := fmt.Sscanf(v, "%d", &scale); err == nil {
				proc.Scale = scale
			}
		}
		if v := os.Getenv(envPrefix + "RESTART"); v != "" {
			proc.Restart = v
		}
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate global settings
	if c.Global.ShutdownTimeout < 0 {
		return fmt.Errorf("shutdown_timeout must be positive")
	}
	if c.Global.LogLevel != "debug" && c.Global.LogLevel != "info" &&
		c.Global.LogLevel != "warn" && c.Global.LogLevel != "error" {
		return fmt.Errorf("invalid log_level: %s", c.Global.LogLevel)
	}
	if c.Global.LogFormat != "json" && c.Global.LogFormat != "text" {
		return fmt.Errorf("invalid log_format: %s", c.Global.LogFormat)
	}

	// Validate processes
	if len(c.Processes) == 0 {
		return fmt.Errorf("no processes defined")
	}

	for name, proc := range c.Processes {
		if len(proc.Command) == 0 {
			return fmt.Errorf("process %s has no command", name)
		}
		if proc.Restart != "always" && proc.Restart != "on-failure" && proc.Restart != "never" {
			return fmt.Errorf("process %s has invalid restart policy: %s", name, proc.Restart)
		}
		if proc.Scale < 1 {
			return fmt.Errorf("process %s has invalid scale: %d", name, proc.Scale)
		}

		// Validate health check
		if proc.HealthCheck != nil {
			hc := proc.HealthCheck
			if hc.Type != "tcp" && hc.Type != "http" && hc.Type != "exec" {
				return fmt.Errorf("process %s has invalid health check type: %s", name, hc.Type)
			}
			if hc.Type == "tcp" && hc.Address == "" {
				return fmt.Errorf("process %s has tcp health check but no address", name)
			}
			if hc.Type == "http" && hc.URL == "" {
				return fmt.Errorf("process %s has http health check but no url", name)
			}
			if hc.Type == "exec" && len(hc.Command) == 0 {
				return fmt.Errorf("process %s has exec health check but no command", name)
			}
		}
	}

	// Check for circular dependencies
	if err := c.checkCircularDependencies(); err != nil {
		return err
	}

	return nil
}

// checkCircularDependencies checks for circular dependencies in process definitions
func (c *Config) checkCircularDependencies() error {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for name := range c.Processes {
		if !visited[name] {
			if c.hasCycle(name, visited, recStack) {
				return fmt.Errorf("circular dependency detected involving process: %s", name)
			}
		}
	}

	return nil
}

func (c *Config) hasCycle(name string, visited, recStack map[string]bool) bool {
	visited[name] = true
	recStack[name] = true

	proc, exists := c.Processes[name]
	if !exists {
		return false
	}

	for _, dep := range proc.DependsOn {
		if !visited[dep] {
			if c.hasCycle(dep, visited, recStack) {
				return true
			}
		} else if recStack[dep] {
			return true
		}
	}

	recStack[name] = false
	return false
}
