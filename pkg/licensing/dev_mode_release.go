//go:build release

package licensing

// isDemoMode always returns false in release builds.
// The PULSE_MOCK_MODE env var is ignored to prevent feature-gate bypass.
func isDemoMode() bool { return false }

// isDevMode always returns false in release builds.
// The PULSE_DEV env var is ignored to prevent feature-gate bypass.
func isDevMode() bool { return false }

// isLicenseValidationDevMode always returns false in release builds.
// The PULSE_LICENSE_DEV_MODE env var is ignored to prevent signature bypass.
func isLicenseValidationDevMode() bool { return false }

// allowPublicKeyEnvOverride returns false in release builds, preventing
// PULSE_LICENSE_PUBLIC_KEY from overriding the embedded key.
func allowPublicKeyEnvOverride() bool { return false }
