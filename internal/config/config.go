package config

import (
	"fmt"
	"os"
	"time"

	"github.com/robfig/cron/v3"
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

	return LoadWithEnvExpansion(configPath)
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
		if proc.Type != "oneshot" && proc.Type != "longrun" {
			return fmt.Errorf("process %s has invalid type: %s (must be oneshot or longrun)", name, proc.Type)
		}
		if proc.InitialState != "running" && proc.InitialState != "stopped" {
			return fmt.Errorf("process %s has invalid initial_state: %s (must be running or stopped)", name, proc.InitialState)
		}
		if proc.Restart != "always" && proc.Restart != "on-failure" && proc.Restart != "never" {
			return fmt.Errorf("process %s has invalid restart policy: %s", name, proc.Restart)
		}
		if proc.Scale < 1 {
			return fmt.Errorf("process %s has invalid scale: %d", name, proc.Scale)
		}
		if proc.MaxScale > 0 && proc.Scale > proc.MaxScale {
			return fmt.Errorf("process %s has scale (%d) exceeding max_scale (%d)", name, proc.Scale, proc.MaxScale)
		}

		// Oneshot validation
		if proc.Type == "oneshot" {
			if proc.Restart == "always" {
				return fmt.Errorf("oneshot process %s cannot have restart: always", name)
			}
			if proc.Scale > 1 {
				return fmt.Errorf("oneshot process %s cannot have scale > 1 (got %d)", name, proc.Scale)
			}
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

		// Validate schedule expression
		if proc.Schedule != "" {
			parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
			if _, err := parser.Parse(proc.Schedule); err != nil {
				return fmt.Errorf("process %s has invalid schedule expression %q: %w", name, proc.Schedule, err)
			}
			// Validate timezone
			if proc.ScheduleTimezone != "" && proc.ScheduleTimezone != "UTC" && proc.ScheduleTimezone != "Local" {
				return fmt.Errorf("process %s has invalid schedule_timezone: %s (must be UTC or Local)", name, proc.ScheduleTimezone)
			}
			// Validate timeout duration
			if proc.ScheduleTimeout != "" {
				if _, err := time.ParseDuration(proc.ScheduleTimeout); err != nil {
					return fmt.Errorf("process %s has invalid schedule_timeout %q: %w", name, proc.ScheduleTimeout, err)
				}
			}
			// Validate max_concurrent
			if proc.ScheduleMaxConcurrent < 0 {
				return fmt.Errorf("process %s has invalid schedule_max_concurrent: %d (must be >= 0)", name, proc.ScheduleMaxConcurrent)
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

// Save writes the configuration to a file
func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
