package utils

import (
	"os"
	"strings"
)

// GetDataDir returns the data directory path from environment or default
func GetDataDir() string {
	if dir := strings.TrimSpace(os.Getenv("PULSE_DATA_DIR")); dir != "" {
		return dir
	}
	return "/etc/pulse"
}
