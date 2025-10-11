package utils

import (
	"os"
)

// GetDataDir returns the data directory path from environment or default
func GetDataDir() string {
	if dir := os.Getenv("PULSE_DATA_DIR"); dir != "" {
		return dir
	}
	return "/etc/pulse"
}
