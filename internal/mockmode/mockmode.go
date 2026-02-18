package mockmode

import (
	"os"
	"strings"
)

// IsEnabled reports whether Pulse is running in mock mode.
//
// This intentionally reads the environment variable instead of importing the full
// internal/mock package, to avoid dependency cycles in low-level packages.
func IsEnabled() bool {
	return strings.TrimSpace(os.Getenv("PULSE_MOCK_MODE")) == "true"
}
