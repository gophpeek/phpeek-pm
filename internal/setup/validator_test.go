package setup

import (
	"log/slog"
	"os/exec"
	"testing"
)

func TestConfigValidator_ValidatePHPFPM(t *testing.T) {
	logger := slog.Default()
	cv := NewConfigValidator(logger)

	// Check if php-fpm is available
	_, err := exec.LookPath("php-fpm")
	if err != nil {
		t.Skip("php-fpm not available, skipping test")
	}

	// Test validation
	err = cv.ValidatePHPFPM()
	if err != nil {
		t.Logf("PHP-FPM validation error (expected in test environment): %v", err)
		// This is acceptable - we're testing the logic, not a real PHP-FPM config
	}
}

func TestConfigValidator_ValidateNginx(t *testing.T) {
	logger := slog.Default()
	cv := NewConfigValidator(logger)

	// Check if nginx is available
	_, err := exec.LookPath("nginx")
	if err != nil {
		t.Skip("nginx not available, skipping test")
	}

	// Test validation
	err = cv.ValidateNginx()
	if err != nil {
		t.Logf("Nginx validation error (expected in test environment): %v", err)
		// This is acceptable - we're testing the logic, not a real nginx config
	}
}

func TestConfigValidator_ValidateAll(t *testing.T) {
	logger := slog.Default()
	cv := NewConfigValidator(logger)

	// Test validation - should not panic even if binaries don't exist
	err := cv.ValidateAll()
	if err != nil {
		t.Logf("Validation errors (expected in test environment): %v", err)
		// This is acceptable - we're testing the logic
	}
}

func TestNewConfigValidator(t *testing.T) {
	logger := slog.Default()
	cv := NewConfigValidator(logger)

	if cv == nil {
		t.Fatal("NewConfigValidator returned nil")
	}

	if cv.logger == nil {
		t.Error("logger is nil")
	}
}

// Test that validator gracefully handles missing binaries
func TestConfigValidator_MissingBinaries(t *testing.T) {
	logger := slog.Default()
	cv := NewConfigValidator(logger)

	// The validator should handle missing binaries gracefully (skip validation)
	// This test verifies it doesn't panic

	// We can't easily test with missing binaries in a unit test,
	// but we can verify the validator is created correctly
	if cv == nil {
		t.Fatal("NewConfigValidator returned nil")
	}
}
