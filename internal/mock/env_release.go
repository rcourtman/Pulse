//go:build release

package mock

// mockModeFromEnv always returns false in release builds.
// The PULSE_MOCK_MODE env var is ignored to prevent mock data in production.
func mockModeFromEnv() bool { return false }
