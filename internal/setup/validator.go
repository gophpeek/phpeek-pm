package setup

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// ConfigValidator validates PHP-FPM and Nginx configurations
type ConfigValidator struct {
	logger *slog.Logger
}

// NewConfigValidator creates a new config validator
func NewConfigValidator(log *slog.Logger) *ConfigValidator {
	return &ConfigValidator{logger: log}
}

// ValidateAll validates PHP-FPM and Nginx configurations
func (cv *ConfigValidator) ValidateAll() error {
	cv.logger.Info("Validating configurations...")

	if err := cv.ValidatePHPFPM(); err != nil {
		return fmt.Errorf("PHP-FPM config invalid: %w", err)
	}
	cv.logger.Debug("PHP-FPM config valid")

	if err := cv.ValidateNginx(); err != nil {
		return fmt.Errorf("Nginx config invalid: %w", err)
	}
	cv.logger.Debug("Nginx config valid")

	cv.logger.Info("All configurations valid")
	return nil
}

// ValidatePHPFPM validates PHP-FPM configuration
func (cv *ConfigValidator) ValidatePHPFPM() error {
	// Check if php-fpm binary exists
	if _, err := exec.LookPath("php-fpm"); err != nil {
		cv.logger.Debug("PHP-FPM binary not found, skipping validation")
		return nil // Not an error - might not be used
	}

	cmd := exec.Command("php-fpm", "-t")
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}

	// Check for success message
	if !strings.Contains(string(output), "test is successful") {
		return fmt.Errorf("unexpected output: %s", string(output))
	}

	return nil
}

// ValidateNginx validates Nginx configuration
func (cv *ConfigValidator) ValidateNginx() error {
	// Check if nginx binary exists
	if _, err := exec.LookPath("nginx"); err != nil {
		cv.logger.Debug("Nginx binary not found, skipping validation")
		return nil // Not an error - might not be used
	}

	cmd := exec.Command("nginx", "-t")
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}

	// Check for success messages
	outputStr := string(output)
	if !strings.Contains(outputStr, "syntax is ok") ||
		!strings.Contains(outputStr, "test is successful") {
		return fmt.Errorf("unexpected output: %s", outputStr)
	}

	return nil
}
