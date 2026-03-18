package utils

import (
	"os"
	"strings"
)

// ResolveDataDirWithDefault returns the canonical Pulse data directory.
func ResolveDataDirWithDefault(explicit string, defaultDir string) string {
	if dir := strings.TrimSpace(explicit); dir != "" {
		return dir
	}
	if dir := strings.TrimSpace(os.Getenv("PULSE_DATA_DIR")); dir != "" {
		return dir
	}
	return defaultDir
}

// ResolveDataDir returns the canonical Pulse data directory.
func ResolveDataDir(explicit string) string {
	return ResolveDataDirWithDefault(explicit, "/etc/pulse")
}

// GetDataDir returns the data directory path from environment or default.
func GetDataDir() string {
	return ResolveDataDir("")
}
