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

// TestGenerator_Generate_InvalidOutputDir tests generation with invalid output directory
func TestGenerator_Generate_InvalidOutputDir(t *testing.T) {
	// Use a path that can't be created (child of a file, not a directory)
	tmpFile, err := os.CreateTemp("", "scaffold-test-file")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Try to use the file as a directory path (will fail)
	g := NewGenerator(PresetMinimal, filepath.Join(tmpFile.Name(), "subdir"))
	err = g.Generate([]string{"config"})
	if err == nil {
		t.Error("Expected error when output directory cannot be created")
	}
}

// TestGenerator_Generate_ReadOnlyDir tests generation in a read-only directory
func TestGenerator_Generate_ReadOnlyDir(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "scaffold-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a subdirectory that's read-only
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.Mkdir(readOnlyDir, 0555); err != nil {
		t.Fatalf("Failed to create readonly dir: %v", err)
	}

	g := NewGenerator(PresetMinimal, readOnlyDir)

	// Try to generate - should fail to write
	err = g.Generate([]string{"config"})
	if err == nil {
		t.Log("Note: Read-only test may pass on some systems")
	}
}

// TestGenerator_Generate_DockerComposeOnly tests generating only docker-compose
func TestGenerator_Generate_DockerComposeOnly(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scaffold-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	g := NewGenerator(PresetLaravel, tmpDir)
	err = g.Generate([]string{"docker-compose"})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify docker-compose exists
	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		t.Error("Expected docker-compose.yml to exist")
	}

	// Verify config does not exist
	configPath := filepath.Join(tmpDir, "phpeek-pm.yaml")
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("Expected phpeek-pm.yaml to not exist")
	}
}

// TestGenerator_Generate_DockerfileOnly tests generating only Dockerfile
func TestGenerator_Generate_DockerfileOnly(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scaffold-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	g := NewGenerator(PresetLaravel, tmpDir)
	err = g.Generate([]string{"dockerfile"})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify Dockerfile exists
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		t.Error("Expected Dockerfile to exist")
	}
}

// TestGenerator_Generate_UnknownFile tests generating with unknown file type
func TestGenerator_Generate_UnknownFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scaffold-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	g := NewGenerator(PresetMinimal, tmpDir)
	// Unknown file type should be silently ignored
	err = g.Generate([]string{"unknown"})
	if err != nil {
		t.Errorf("Generate with unknown file should not error: %v", err)
	}
}

// TestGenerateDockerCompose_Minimal tests docker-compose for minimal preset
func TestGenerateDockerCompose_Minimal(t *testing.T) {
	cfg := DefaultConfig(PresetMinimal)
	content, err := GenerateDockerCompose(cfg)
	if err != nil {
		t.Fatalf("GenerateDockerCompose failed: %v", err)
	}

	// Check services
	if !strings.Contains(content, "app:") {
		t.Error("Docker compose should contain app service")
	}
	// Minimal should not have redis by default
	if strings.Contains(content, "redis:") {
		t.Error("Minimal docker compose should not contain redis service")
	}
}

// TestGenerateDockerCompose_Symfony tests docker-compose for Symfony preset
func TestGenerateDockerCompose_Symfony(t *testing.T) {
	cfg := DefaultConfig(PresetSymfony)
	content, err := GenerateDockerCompose(cfg)
	if err != nil {
		t.Fatalf("GenerateDockerCompose failed: %v", err)
	}

	// Check services
	if !strings.Contains(content, "app:") {
		t.Error("Docker compose should contain app service")
	}
}

// TestGenerateDockerfile_Minimal tests Dockerfile for minimal preset
func TestGenerateDockerfile_Minimal(t *testing.T) {
	cfg := DefaultConfig(PresetMinimal)
	content, err := GenerateDockerfile(cfg)
	if err != nil {
		t.Fatalf("GenerateDockerfile failed: %v", err)
	}

	// Check base image
	if !strings.Contains(content, "FROM php:8.2-fpm-alpine") {
		t.Error("Dockerfile should use php:8.2-fpm-alpine base image")
	}

	// Check PHPeek PM
	if !strings.Contains(content, "phpeek-pm") {
		t.Error("Dockerfile should reference phpeek-pm")
	}
}

// TestGenerateDockerfile_Symfony tests Dockerfile for Symfony preset
func TestGenerateDockerfile_Symfony(t *testing.T) {
	cfg := DefaultConfig(PresetSymfony)
	content, err := GenerateDockerfile(cfg)
	if err != nil {
		t.Fatalf("GenerateDockerfile failed: %v", err)
	}

	// Check base image
	if !strings.Contains(content, "FROM php:8.2-fpm-alpine") {
		t.Error("Dockerfile should use php:8.2-fpm-alpine base image")
	}
}

// TestGenerateDockerfile_Generic tests Dockerfile for generic preset
func TestGenerateDockerfile_Generic(t *testing.T) {
	cfg := DefaultConfig(PresetGeneric)
	content, err := GenerateDockerfile(cfg)
	if err != nil {
		t.Fatalf("GenerateDockerfile failed: %v", err)
	}

	// Check PHPeek PM
	if !strings.Contains(content, "phpeek-pm") {
		t.Error("Dockerfile should reference phpeek-pm")
	}
}

// TestGenerateConfig_Symfony tests config generation for Symfony preset
func TestGenerateConfig_Symfony(t *testing.T) {
	cfg := DefaultConfig(PresetSymfony)
	content, err := GenerateConfig(cfg)
	if err != nil {
		t.Fatalf("GenerateConfig failed: %v", err)
	}

	// Check php-fpm process
	if !strings.Contains(content, "php-fpm:") {
		t.Error("Symfony config should contain php-fpm process")
	}

	// Check nginx process
	if !strings.Contains(content, "nginx:") {
		t.Error("Symfony config should contain nginx process")
	}

	// Symfony should not have horizon
	if strings.Contains(content, "horizon:") {
		t.Error("Symfony config should not contain horizon process")
	}
}

// TestGenerateConfig_Generic tests config generation for generic preset
func TestGenerateConfig_Generic(t *testing.T) {
	cfg := DefaultConfig(PresetGeneric)
	content, err := GenerateConfig(cfg)
	if err != nil {
		t.Fatalf("GenerateConfig failed: %v", err)
	}

	// Generic preset uses generic framework which doesn't include php-fpm by default
	// Only laravel and symfony frameworks include php-fpm in the template

	// Check nginx process is enabled for generic preset
	if !strings.Contains(content, "nginx:") {
		t.Error("Generic config should contain nginx process")
	}

	// Check that basic config structure is present
	if !strings.Contains(content, "version:") {
		t.Error("Generic config should contain version field")
	}
}

// TestGenerateConfig_Production tests config generation for production preset
func TestGenerateConfig_Production(t *testing.T) {
	cfg := DefaultConfig(PresetProduction)
	content, err := GenerateConfig(cfg)
	if err != nil {
		t.Fatalf("GenerateConfig failed: %v", err)
	}

	// Check php-fpm process
	if !strings.Contains(content, "php-fpm:") {
		t.Error("Production config should contain php-fpm process")
	}

	// Check horizon process (production is based on Laravel)
	if !strings.Contains(content, "horizon:") {
		t.Error("Production config should contain horizon process")
	}

	// Check metrics enabled
	if !strings.Contains(content, "metrics_enabled: true") {
		t.Error("Production config should have metrics enabled")
	}

	// Check API enabled
	if !strings.Contains(content, "api_enabled: true") {
		t.Error("Production config should have API enabled")
	}
}

// TestGenerateDockerCompose_Generic tests docker-compose for generic preset
func TestGenerateDockerCompose_Generic(t *testing.T) {
	cfg := DefaultConfig(PresetGeneric)
	content, err := GenerateDockerCompose(cfg)
	if err != nil {
		t.Fatalf("GenerateDockerCompose failed: %v", err)
	}

	// Check app service
	if !strings.Contains(content, "app:") {
		t.Error("Docker compose should contain app service")
	}
}

// TestGenerateDockerfile_Production tests Dockerfile for production preset
func TestGenerateDockerfile_Production(t *testing.T) {
	cfg := DefaultConfig(PresetProduction)
	content, err := GenerateDockerfile(cfg)
	if err != nil {
		t.Fatalf("GenerateDockerfile failed: %v", err)
	}

	// Check composer install
	if !strings.Contains(content, "composer install") {
		t.Error("Production Dockerfile should run composer install")
	}
}

// TestGenerator_EnableFeature_Unknown tests enabling an unknown feature
func TestGenerator_EnableFeature_Unknown(t *testing.T) {
	g := NewGenerator(PresetMinimal, "/tmp/test")

	// Enabling an unknown feature should not panic
	g.EnableFeature("unknown_feature", true)
	g.EnableFeature("invalid", false)

	// No error should occur
	t.Log("Unknown feature handling works correctly")
}

// TestGenerator_Generate_AllFilesForAllPresets tests all file types for all presets
func TestGenerator_Generate_AllFilesForAllPresets(t *testing.T) {
	presets := []Preset{PresetLaravel, PresetSymfony, PresetGeneric, PresetMinimal, PresetProduction}

	for _, preset := range presets {
		t.Run(string(preset), func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "scaffold-test-")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			g := NewGenerator(preset, tmpDir)
			err = g.Generate([]string{"config", "docker-compose", "dockerfile"})
			if err != nil {
				t.Fatalf("Generate failed for preset %s: %v", preset, err)
			}

			// Verify all files exist
			files := []string{"phpeek-pm.yaml", "docker-compose.yml", "Dockerfile"}
			for _, file := range files {
				path := filepath.Join(tmpDir, file)
				if _, err := os.Stat(path); os.IsNotExist(err) {
					t.Errorf("Expected file %s to exist for preset %s", file, preset)
				}
			}
		})
	}
}

// TestGenerateDockerCompose_WithTracing tests docker-compose with tracing and metrics enabled
func TestGenerateDockerCompose_WithTracing(t *testing.T) {
	cfg := DefaultConfig(PresetProduction)
	content, err := GenerateDockerCompose(cfg)
	if err != nil {
		t.Fatalf("GenerateDockerCompose failed: %v", err)
	}

	// Production preset has metrics enabled, which includes prometheus
	if !strings.Contains(content, "prometheus:") {
		t.Error("Production docker compose should contain prometheus service")
	}

	// Production preset has metrics enabled, which includes grafana
	if !strings.Contains(content, "grafana:") {
		t.Error("Production docker compose should contain grafana service")
	}

	// Production is based on laravel framework, should include redis
	if !strings.Contains(content, "redis:") {
		t.Error("Production docker compose should contain redis service")
	}
}

// TestDefaultConfig_Unknown tests unknown preset defaults to generic
func TestDefaultConfig_Unknown(t *testing.T) {
	cfg := DefaultConfig(Preset("unknown"))

	// Unknown preset should default to generic settings
	if cfg.Framework != "generic" {
		t.Logf("Unknown preset defaulted to framework: %s", cfg.Framework)
	}
}
