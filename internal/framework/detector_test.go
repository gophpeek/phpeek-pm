package framework

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(string) error
		want      Framework
	}{
		{
			name: "detect Laravel",
			setupFunc: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "artisan"), []byte("#!/usr/bin/env php"), 0644)
			},
			want: Laravel,
		},
		{
			name: "detect Symfony",
			setupFunc: func(dir string) error {
				binDir := filepath.Join(dir, "bin")
				varDir := filepath.Join(dir, "var", "cache")
				if err := os.MkdirAll(binDir, 0755); err != nil {
					return err
				}
				if err := os.MkdirAll(varDir, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(binDir, "console"), []byte("#!/usr/bin/env php"), 0755)
			},
			want: Symfony,
		},
		{
			name: "detect WordPress",
			setupFunc: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "wp-config.php"), []byte("<?php"), 0644)
			},
			want: WordPress,
		},
		{
			name: "detect Generic",
			setupFunc: func(dir string) error {
				// Empty directory
				return nil
			},
			want: Generic,
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

			// Setup test files
			if err := tt.setupFunc(tmpDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			// Test detection
			got := Detect(tmpDir)
			if got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "phpeek-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "existing file",
			path: testFile,
			want: true,
		},
		{
			name: "non-existent file",
			path: filepath.Join(tmpDir, "nonexistent.txt"),
			want: false,
		},
		{
			name: "directory not file",
			path: tmpDir,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileExists(tt.path)
			if got != tt.want {
				t.Errorf("fileExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDirExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "phpeek-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "existing directory",
			path: tmpDir,
			want: true,
		},
		{
			name: "non-existent directory",
			path: filepath.Join(tmpDir, "nonexistent"),
			want: false,
		},
		{
			name: "file not directory",
			path: testFile,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dirExists(tt.path)
			if got != tt.want {
				t.Errorf("dirExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFrameworkString(t *testing.T) {
	tests := []struct {
		fw   Framework
		want string
	}{
		{Laravel, "laravel"},
		{Symfony, "symfony"},
		{WordPress, "wordpress"},
		{Generic, "generic"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.fw.String()
			if got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}
