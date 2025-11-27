package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig_Laravel(t *testing.T) {
	cfg := DefaultConfig(PresetLaravel)

	if cfg.Preset != PresetLaravel {
		t.Errorf("Expected preset Laravel, got %s", cfg.Preset)
	}
	if cfg.Framework != "laravel" {
		t.Errorf("Expected framework laravel, got %s", cfg.Framework)
	}
	if !cfg.EnableNginx {
		t.Error("Expected EnableNginx to be true for Laravel")
	}
	if !cfg.EnableHorizon {
		t.Error("Expected EnableHorizon to be true for Laravel")
	}
	if !cfg.EnableQueue {
		t.Error("Expected EnableQueue to be true for Laravel")
	}
	if !cfg.EnableScheduler {
		t.Error("Expected EnableScheduler to be true for Laravel")
	}
}

func TestDefaultConfig_Symfony(t *testing.T) {
	cfg := DefaultConfig(PresetSymfony)

	if cfg.Preset != PresetSymfony {
		t.Errorf("Expected preset Symfony, got %s", cfg.Preset)
	}
	if cfg.Framework != "symfony" {
		t.Errorf("Expected framework symfony, got %s", cfg.Framework)
	}
	if !cfg.EnableNginx {
		t.Error("Expected EnableNginx to be true for Symfony")
	}
	if !cfg.EnableQueue {
		t.Error("Expected EnableQueue to be true for Symfony")
	}
	if cfg.EnableHorizon {
		t.Error("Expected EnableHorizon to be false for Symfony")
	}
}

func TestDefaultConfig_Production(t *testing.T) {
	cfg := DefaultConfig(PresetProduction)

	if cfg.Preset != PresetProduction {
		t.Errorf("Expected preset Production, got %s", cfg.Preset)
	}
	if !cfg.EnableTracing {
		t.Error("Expected EnableTracing to be true for Production")
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("Expected log level warn for Production, got %s", cfg.LogLevel)
	}
}

func TestDefaultConfig_Minimal(t *testing.T) {
	cfg := DefaultConfig(PresetMinimal)

	if cfg.Preset != PresetMinimal {
		t.Errorf("Expected preset Minimal, got %s", cfg.Preset)
	}
	if cfg.EnableNginx {
		t.Error("Expected EnableNginx to be false for Minimal")
	}
	if cfg.EnableHorizon {
		t.Error("Expected EnableHorizon to be false for Minimal")
	}
	if cfg.EnableQueue {
		t.Error("Expected EnableQueue to be false for Minimal")
	}
	if cfg.EnableScheduler {
		t.Error("Expected EnableScheduler to be false for Minimal")
	}
	if cfg.EnableMetrics {
		t.Error("Expected EnableMetrics to be false for Minimal")
	}
	if cfg.EnableAPI {
		t.Error("Expected EnableAPI to be false for Minimal")
	}
}

func TestDefaultConfig_Generic(t *testing.T) {
	cfg := DefaultConfig(PresetGeneric)

	if cfg.Preset != PresetGeneric {
		t.Errorf("Expected preset Generic, got %s", cfg.Preset)
	}
	if cfg.Framework != "generic" {
		t.Errorf("Expected framework generic, got %s", cfg.Framework)
	}
	if !cfg.EnableNginx {
		t.Error("Expected EnableNginx to be true for Generic")
	}
}

func TestDefaultConfig_Defaults(t *testing.T) {
	cfg := DefaultConfig(PresetLaravel)

	if cfg.AppName != "my-app" {
		t.Errorf("Expected default AppName 'my-app', got '%s'", cfg.AppName)
	}
	if cfg.WorkDir != "/var/www/html" {
		t.Errorf("Expected default WorkDir '/var/www/html', got '%s'", cfg.WorkDir)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("Expected default LogLevel 'info', got '%s'", cfg.LogLevel)
	}
	if cfg.QueueWorkers != 3 {
		t.Errorf("Expected default QueueWorkers 3, got %d", cfg.QueueWorkers)
	}
	if cfg.QueueConnection != "redis" {
		t.Errorf("Expected default QueueConnection 'redis', got '%s'", cfg.QueueConnection)
	}
}

func TestGenerateConfig_Laravel(t *testing.T) {
	cfg := DefaultConfig(PresetLaravel)
	content, err := GenerateConfig(cfg)
	if err != nil {
		t.Fatalf("GenerateConfig failed: %v", err)
	}

	// Check version
	if !strings.Contains(content, `version: "1.0"`) {
		t.Error("Config should contain version 1.0")
	}

	// Check php-fpm process
	if !strings.Contains(content, "php-fpm:") {
		t.Error("Laravel config should contain php-fpm process")
	}

	// Check nginx process
	if !strings.Contains(content, "nginx:") {
		t.Error("Laravel config should contain nginx process")
	}

	// Check horizon process
	if !strings.Contains(content, "horizon:") {
		t.Error("Laravel config should contain horizon process")
	}

	// Check queue-default process
	if !strings.Contains(content, "queue-default:") {
		t.Error("Laravel config should contain queue-default process")
	}

	// Check scheduler process
	if !strings.Contains(content, "scheduler:") {
		t.Error("Laravel config should contain scheduler process")
	}
}

func TestGenerateConfig_Minimal(t *testing.T) {
	cfg := DefaultConfig(PresetMinimal)
	content, err := GenerateConfig(cfg)
	if err != nil {
		t.Fatalf("GenerateConfig failed: %v", err)
	}

	// Minimal should not have nginx
	if strings.Contains(content, "nginx:") {
		t.Error("Minimal config should not contain nginx process")
	}

	// Minimal should not have horizon
	if strings.Contains(content, "horizon:") {
		t.Error("Minimal config should not contain horizon process")
	}
}

func TestGenerateDockerCompose_Laravel(t *testing.T) {
	cfg := DefaultConfig(PresetLaravel)
	content, err := GenerateDockerCompose(cfg)
	if err != nil {
		t.Fatalf("GenerateDockerCompose failed: %v", err)
	}

	// Check version
	if !strings.Contains(content, "version: '3.8'") {
		t.Error("Docker compose should contain version 3.8")
	}

	// Check services
	if !strings.Contains(content, "app:") {
		t.Error("Docker compose should contain app service")
	}
	if !strings.Contains(content, "db:") {
		t.Error("Laravel docker compose should contain db service")
	}
	if !strings.Contains(content, "redis:") {
		t.Error("Laravel docker compose should contain redis service")
	}
}

func TestGenerateDockerCompose_WithMetrics(t *testing.T) {
	cfg := DefaultConfig(PresetProduction)
	content, err := GenerateDockerCompose(cfg)
	if err != nil {
		t.Fatalf("GenerateDockerCompose failed: %v", err)
	}

	// Check Prometheus
	if !strings.Contains(content, "prometheus:") {
		t.Error("Production docker compose should contain prometheus service")
	}

	// Check Grafana
	if !strings.Contains(content, "grafana:") {
		t.Error("Production docker compose should contain grafana service")
	}
}

func TestGenerateDockerfile(t *testing.T) {
	cfg := DefaultConfig(PresetLaravel)
	content, err := GenerateDockerfile(cfg)
	if err != nil {
		t.Fatalf("GenerateDockerfile failed: %v", err)
	}

	// Check base image
	if !strings.Contains(content, "FROM php:8.2-fpm-alpine") {
		t.Error("Dockerfile should use php:8.2-fpm-alpine base image")
	}

	// Check composer
	if !strings.Contains(content, "composer install") {
		t.Error("Laravel Dockerfile should run composer install")
	}

	// Check PHPeek PM
	if !strings.Contains(content, "phpeek-pm") {
		t.Error("Dockerfile should reference phpeek-pm")
	}
}

func TestNewGenerator(t *testing.T) {
	g := NewGenerator(PresetLaravel, "/tmp/test-output")

	if g.config == nil {
		t.Error("Generator config should not be nil")
	}
	if g.config.Preset != PresetLaravel {
		t.Errorf("Expected preset Laravel, got %s", g.config.Preset)
	}
	if g.outDir != "/tmp/test-output" {
		t.Errorf("Expected outDir /tmp/test-output, got %s", g.outDir)
	}
}

func TestNewGeneratorWithConfig(t *testing.T) {
	cfg := &Config{
		AppName:       "custom-app",
		EnableMetrics: true,
	}

	g := NewGeneratorWithConfig(cfg, "/tmp/test-output")

	if g.config.AppName != "custom-app" {
		t.Errorf("Expected AppName 'custom-app', got '%s'", g.config.AppName)
	}
}

func TestGenerator_SetAppName(t *testing.T) {
	g := NewGenerator(PresetLaravel, "/tmp/test")
	g.SetAppName("my-custom-app")

	if g.config.AppName != "my-custom-app" {
		t.Errorf("Expected AppName 'my-custom-app', got '%s'", g.config.AppName)
	}
}

func TestGenerator_SetLogLevel(t *testing.T) {
	g := NewGenerator(PresetLaravel, "/tmp/test")
	g.SetLogLevel("debug")

	if g.config.LogLevel != "debug" {
		t.Errorf("Expected LogLevel 'debug', got '%s'", g.config.LogLevel)
	}
}

func TestGenerator_EnableFeature(t *testing.T) {
	g := NewGenerator(PresetMinimal, "/tmp/test")

	tests := []struct {
		feature  string
		expected *bool
	}{
		{"nginx", &g.config.EnableNginx},
		{"horizon", &g.config.EnableHorizon},
		{"queue", &g.config.EnableQueue},
		{"scheduler", &g.config.EnableScheduler},
		{"metrics", &g.config.EnableMetrics},
		{"api", &g.config.EnableAPI},
		{"tracing", &g.config.EnableTracing},
	}

	for _, tt := range tests {
		t.Run(tt.feature, func(t *testing.T) {
			g.EnableFeature(tt.feature, true)
			if !*tt.expected {
				t.Errorf("Expected %s to be enabled", tt.feature)
			}
			g.EnableFeature(tt.feature, false)
			if *tt.expected {
				t.Errorf("Expected %s to be disabled", tt.feature)
			}
		})
	}
}

func TestGenerator_SetQueueWorkers(t *testing.T) {
	g := NewGenerator(PresetLaravel, "/tmp/test")
	g.SetQueueWorkers(10)

	if g.config.QueueWorkers != 10 {
		t.Errorf("Expected QueueWorkers 10, got %d", g.config.QueueWorkers)
	}
}

func TestGenerator_SetQueueConnection(t *testing.T) {
	g := NewGenerator(PresetLaravel, "/tmp/test")
	g.SetQueueConnection("database")

	if g.config.QueueConnection != "database" {
		t.Errorf("Expected QueueConnection 'database', got '%s'", g.config.QueueConnection)
	}
}

func TestGenerator_GetConfig(t *testing.T) {
	g := NewGenerator(PresetLaravel, "/tmp/test")
	cfg := g.GetConfig()

	if cfg != g.config {
		t.Error("GetConfig should return the generator's config")
	}
}

func TestGenerator_PreviewConfig(t *testing.T) {
	g := NewGenerator(PresetLaravel, "/tmp/test")
	content, err := g.PreviewConfig()

	if err != nil {
		t.Fatalf("PreviewConfig failed: %v", err)
	}
	if content == "" {
		t.Error("PreviewConfig should return non-empty content")
	}
}

func TestGenerator_Generate(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "scaffold-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	g := NewGenerator(PresetLaravel, tmpDir)

	// Generate all files
	err = g.Generate([]string{"config", "docker-compose", "dockerfile"})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify files exist
	files := []string{"phpeek-pm.yaml", "docker-compose.yml", "Dockerfile"}
	for _, file := range files {
		path := filepath.Join(tmpDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s to exist", file)
		}
	}
}

func TestGenerator_Generate_ConfigOnly(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "scaffold-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	g := NewGenerator(PresetMinimal, tmpDir)

	// Generate config only
	err = g.Generate([]string{"config"})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify config exists
	configPath := filepath.Join(tmpDir, "phpeek-pm.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected phpeek-pm.yaml to exist")
	}

	// Verify docker-compose does not exist
	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	if _, err := os.Stat(composePath); !os.IsNotExist(err) {
		t.Error("Expected docker-compose.yml to not exist")
	}
}

func TestValidPresets(t *testing.T) {
	presets := ValidPresets()

	expected := []string{"laravel", "symfony", "generic", "minimal", "production"}
	if len(presets) != len(expected) {
		t.Errorf("Expected %d presets, got %d", len(expected), len(presets))
	}

	for _, exp := range expected {
		found := false
		for _, p := range presets {
			if p == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected preset '%s' to be in ValidPresets()", exp)
		}
	}
}

func TestGenerateConfig_WithTracing(t *testing.T) {
	cfg := DefaultConfig(PresetProduction)
	content, err := GenerateConfig(cfg)
	if err != nil {
		t.Fatalf("GenerateConfig failed: %v", err)
	}

	// Check tracing configuration
	if !strings.Contains(content, "tracing_enabled: true") {
		t.Error("Production config should have tracing enabled")
	}
	if !strings.Contains(content, "tracing_exporter: otlp-grpc") {
		t.Error("Production config should have tracing exporter")
	}
	if !strings.Contains(content, "tracing_sample_rate: 0.1") {
		t.Error("Production config should have tracing sample rate")
	}
}
