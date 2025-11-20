package framework

import (
	"os"
	"path/filepath"
)

// Framework represents a detected PHP framework
type Framework string

const (
	Laravel   Framework = "laravel"
	Symfony   Framework = "symfony"
	WordPress Framework = "wordpress"
	Generic   Framework = "generic"
)

// Detect identifies the PHP framework in the given directory
func Detect(dir string) Framework {
	// Laravel: check for artisan file
	if fileExists(filepath.Join(dir, "artisan")) {
		return Laravel
	}

	// Symfony: check for bin/console and var/cache
	if fileExists(filepath.Join(dir, "bin", "console")) &&
		dirExists(filepath.Join(dir, "var", "cache")) {
		return Symfony
	}

	// WordPress: check for wp-config.php
	if fileExists(filepath.Join(dir, "wp-config.php")) {
		return WordPress
	}

	return Generic
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// String returns the string representation of the Framework
func (f Framework) String() string {
	return string(f)
}
