//go:build !release

package mockruntime

import (
	"os"
	"strings"
)

func startupEnabledFromEnv() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("PULSE_MOCK_MODE")), "true")
}

// ValidateEnablement always permits mock fixtures in non-release builds.
func ValidateEnablement(enable bool) error {
	return nil
}
