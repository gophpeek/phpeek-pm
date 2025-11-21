package setup

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gophpeek/phpeek-pm/internal/framework"
)

// PermissionManager handles directory creation and permission setup
type PermissionManager struct {
	logger    *slog.Logger
	workdir   string
	framework framework.Framework
}

// NewPermissionManager creates a new permission manager
func NewPermissionManager(workdir string, fw framework.Framework, log *slog.Logger) *PermissionManager {
	return &PermissionManager{
		logger:    log,
		workdir:   workdir,
		framework: fw,
	}
}

// Setup creates necessary directories and sets permissions
func (pm *PermissionManager) Setup() error {
	pm.logger.Info("Setting up permissions", "framework", pm.framework)

	// Detect read-only root filesystem
	if IsReadOnlyRoot() {
		pm.logger.Info("Read-only root filesystem detected, skipping permission setup",
			"info", "Runtime state will use /run/phpeek-pm (tmpfs)")
		return nil
	}

	switch pm.framework {
	case framework.Laravel:
		return pm.setupLaravel()
	case framework.Symfony:
		return pm.setupSymfony()
	case framework.WordPress:
		return pm.setupWordPress()
	default:
		pm.logger.Debug("Generic framework, skipping permission setup")
		return nil
	}
}

func (pm *PermissionManager) setupLaravel() error {
	dirs := []string{
		filepath.Join(pm.workdir, "storage", "framework", "sessions"),
		filepath.Join(pm.workdir, "storage", "framework", "views"),
		filepath.Join(pm.workdir, "storage", "framework", "cache"),
		filepath.Join(pm.workdir, "storage", "logs"),
		filepath.Join(pm.workdir, "bootstrap", "cache"),
	}

	for _, dir := range dirs {
		if err := pm.createDir(dir, 0775); err != nil {
			pm.logger.Warn("Failed to create directory", "dir", dir, "error", err)
		}
	}

	// Set ownership (www-data UID/GID typically 82 on Alpine, 33 on Debian)
	// Note: This will fail silently if not running as root
	pm.chownRecursive(filepath.Join(pm.workdir, "storage"), 82, 82)
	pm.chownRecursive(filepath.Join(pm.workdir, "bootstrap", "cache"), 82, 82)

	return nil
}

func (pm *PermissionManager) setupSymfony() error {
	dirs := []string{
		filepath.Join(pm.workdir, "var", "cache"),
		filepath.Join(pm.workdir, "var", "log"),
	}

	for _, dir := range dirs {
		if err := pm.createDir(dir, 0775); err != nil {
			pm.logger.Warn("Failed to create directory", "dir", dir, "error", err)
		}
	}

	pm.chownRecursive(filepath.Join(pm.workdir, "var"), 82, 82)
	return nil
}

func (pm *PermissionManager) setupWordPress() error {
	dir := filepath.Join(pm.workdir, "wp-content", "uploads")
	if err := pm.createDir(dir, 0775); err != nil {
		pm.logger.Warn("Failed to create uploads directory", "error", err)
	}

	pm.chownRecursive(filepath.Join(pm.workdir, "wp-content"), 82, 82)
	return nil
}

func (pm *PermissionManager) createDir(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (pm *PermissionManager) chownRecursive(path string, uid, gid int) {
	// Note: This will fail silently if not running as root
	// That's acceptable - in dev environments permissions may not matter
	_ = filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err == nil {
			_ = os.Chown(name, uid, gid)
		}
		return nil
	})
}
