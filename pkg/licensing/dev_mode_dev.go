//go:build !release

package licensing

import (
	"os"
	"strings"
)

// isDemoMode returns true if the demo/mock mode is enabled.
// Only available in non-release builds.
func isDemoMode() bool {
	return strings.EqualFold(os.Getenv("PULSE_MOCK_MODE"), "true")
}

// isDevMode returns true if running in development mode.
// Only available in non-release builds.
func isDevMode() bool {
	return strings.EqualFold(os.Getenv("PULSE_DEV"), "true")
}

// isLicenseValidationDevMode returns true if license signature validation
// should be skipped. Only available in non-release builds.
func isLicenseValidationDevMode() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("PULSE_LICENSE_DEV_MODE")), "true")
}

// allowPublicKeyEnvOverride returns true in dev builds, allowing
// PULSE_LICENSE_PUBLIC_KEY to override the embedded key.
func allowPublicKeyEnvOverride() bool { return true }
