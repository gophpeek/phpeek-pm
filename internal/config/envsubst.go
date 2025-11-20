package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// ExpandEnv expands environment variables in config content
// Supports ${VAR:-default} and ${VAR} syntax
func ExpandEnv(content string) string {
	// Pattern: ${VAR:-default} or ${VAR}
	pattern := regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)

	return pattern.ReplaceAllStringFunc(content, func(match string) string {
		parts := pattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}

		varName := parts[1]
		defaultValue := ""
		if len(parts) >= 3 {
			defaultValue = parts[2]
		}

		// Get from environment or use default
		if value := os.Getenv(varName); value != "" {
			return value
		}

		return defaultValue
	})
}

// LoadWithEnvExpansion loads config file and expands environment variables
func LoadWithEnvExpansion(path string) (*Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Expand environment variables
	expanded := ExpandEnv(string(content))

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults and validate
	cfg.SetDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}
