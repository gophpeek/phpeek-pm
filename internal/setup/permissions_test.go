package setup

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestPermissionManager_Setup(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name      string
		setupFunc func(string) error
		checkDirs []string
	}{
		{
			name: "Laravel setup",
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
			name: "Symfony setup",
			setupFunc: func(dir string) error {
				// Create bin/console and var/cache to make it a Symfony project
				if err := os.MkdirAll(filepath.Join(dir, "bin"), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(dir, "bin", "console"), []byte("#!/usr/bin/env php"), 0644); err != nil {
					return err
				}
				return os.MkdirAll(filepath.Join(dir, "var", "cache"), 0755)
			},
			checkDirs: []string{
				"var/cache",
				"var/log",
			},
		},
		{
			name: "WordPress setup",
			setupFunc: func(dir string) error {
				// Create wp-config.php to make it a WordPress project
				return os.WriteFile(filepath.Join(dir, "wp-config.php"), []byte("<?php"), 0644)
			},
			checkDirs: []string{
				"wp-content/uploads",
			},
		},
		{
			name: "Generic framework (no-op)",
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
			pm := NewPermissionManager(tmpDir, logger)
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

	pm := NewPermissionManager(tmpDir, logger)

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

	pm := NewPermissionManager(workdir, logger)

	if pm == nil {
		t.Fatal("NewPermissionManager returned nil")
	}

	if pm.workdir != workdir {
		t.Errorf("workdir = %v, want %v", pm.workdir, workdir)
	}

	if pm.logger == nil {
		t.Error("logger is nil")
	}
}

func TestDetectFramework(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name     string
		setup    func(string) error
		expected Framework
	}{
		{
			name: "Laravel detection",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "artisan"), []byte("#!/usr/bin/env php"), 0644)
			},
			expected: FrameworkLaravel,
		},
		{
			name: "Symfony detection",
			setup: func(dir string) error {
				if err := os.MkdirAll(filepath.Join(dir, "bin"), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(dir, "bin", "console"), []byte("#!/usr/bin/env php"), 0644); err != nil {
					return err
				}
				return os.MkdirAll(filepath.Join(dir, "var", "cache"), 0755)
			},
			expected: FrameworkSymfony,
		},
		{
			name: "WordPress detection",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "wp-config.php"), []byte("<?php"), 0644)
			},
			expected: FrameworkWordPress,
		},
		{
			name: "Generic (no markers)",
			setup: func(dir string) error {
				return nil
			},
			expected: FrameworkGeneric,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "phpeek-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			if err := tt.setup(tmpDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			pm := NewPermissionManager(tmpDir, logger)
			detected := pm.detectFramework()

			if detected != tt.expected {
				t.Errorf("detectFramework() = %v, want %v", detected, tt.expected)
			}
		})
	}
}
