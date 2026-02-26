//go:build !release

package mock

import "os"

// mockModeFromEnv reads PULSE_MOCK_MODE from the environment.
// Only available in non-release builds.
func mockModeFromEnv() bool {
	return os.Getenv("PULSE_MOCK_MODE") == "true"
}
