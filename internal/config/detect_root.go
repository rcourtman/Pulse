package config

import (
	"os"
	"path/filepath"
	"strings"
)

var (
	osExecutable = os.Executable
	osGetwd      = os.Getwd
)

// detectAppRoot attempts to find the application root directory
func detectAppRoot() string {
	// 1. Check environment variable
	if root := os.Getenv("PULSE_APP_ROOT"); root != "" {
		return root
	}

	// 2. Get executable path
	exe, err := osExecutable()
	if err == nil {
		// If running via "go run", executable is in /tmp, which isn't helpful for finding source files
		// But in production, it's correct.
		// Check if we are in a temp dir (go run)
		if strings.Contains(exe, os.TempDir()) || strings.Contains(exe, "/var/folders/") {
			// Fallback to current working directory
			if cwd, err := osGetwd(); err == nil {
				return cwd
			}
		}
		return filepath.Dir(exe)
	}

	// 3. Fallback to current working directory
	if cwd, err := osGetwd(); err == nil {
		return cwd
	}

	return "."
}
