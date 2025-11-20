package setup

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/gophpeek/phpeek-pm/internal/framework"
)

func TestPermissionManager_Setup(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name      string
		framework framework.Framework
		setupFunc func(string) error
		checkDirs []string
	}{
		{
			name:      "Laravel setup",
			framework: framework.Laravel,
			setupFunc: func(dir string) error {
				// Create artisan to make it a Laravel project
				return os.WriteFile(filepath.Join(dir, "artisan"), []byte("#!/usr/bin/env php"), 0644)
			},
			checkDirs: []string{
				"storage/framework/sessions",
				"storage/framework/views",
				"storage/framework/cache",
				"storage/logs",
				"bootstrap/cache",
			},
		},
		{
			name:      "Symfony setup",
			framework: framework.Symfony,
			setupFunc: func(dir string) error {
				return nil
			},
			checkDirs: []string{
				"var/cache",
				"var/log",
			},
		},
		{
			name:      "WordPress setup",
			framework: framework.WordPress,
			setupFunc: func(dir string) error {
				return nil
			},
			checkDirs: []string{
				"wp-content/uploads",
			},
		},
		{
			name:      "Generic framework (no-op)",
			framework: framework.Generic,
			setupFunc: func(dir string) error {
				return nil
			},
			checkDirs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir, err := os.MkdirTemp("", "phpeek-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Setup test files if needed
			if err := tt.setupFunc(tmpDir); err != nil {
				t.Fatalf("Setup func failed: %v", err)
			}

			// Create permission manager and run setup
			pm := NewPermissionManager(tmpDir, tt.framework, logger)
			if err := pm.Setup(); err != nil {
				t.Errorf("Setup() error = %v", err)
			}

			// Verify directories were created
			for _, dir := range tt.checkDirs {
				fullPath := filepath.Join(tmpDir, dir)
				if _, err := os.Stat(fullPath); os.IsNotExist(err) {
					t.Errorf("Expected directory %s was not created", dir)
				}
			}
		})
	}
}

func TestPermissionManager_CreateDir(t *testing.T) {
	logger := slog.Default()
	tmpDir, err := os.MkdirTemp("", "phpeek-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pm := NewPermissionManager(tmpDir, framework.Generic, logger)

	testPath := filepath.Join(tmpDir, "nested", "dir", "structure")
	if err := pm.createDir(testPath, 0755); err != nil {
		t.Errorf("createDir() error = %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Error("Directory was not created")
	}
}

func TestNewPermissionManager(t *testing.T) {
	logger := slog.Default()
	workdir := "/var/www/html"
	fw := framework.Laravel

	pm := NewPermissionManager(workdir, fw, logger)

	if pm == nil {
		t.Fatal("NewPermissionManager returned nil")
	}

	if pm.workdir != workdir {
		t.Errorf("workdir = %v, want %v", pm.workdir, workdir)
	}

	if pm.framework != fw {
		t.Errorf("framework = %v, want %v", pm.framework, fw)
	}

	if pm.logger == nil {
		t.Error("logger is nil")
	}
}
