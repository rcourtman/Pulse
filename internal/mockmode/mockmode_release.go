//go:build release

package mockmode

// IsEnabled always returns false in release builds.
// The PULSE_MOCK_MODE env var is ignored to prevent mock behavior in production.
func IsEnabled() bool { return false }
