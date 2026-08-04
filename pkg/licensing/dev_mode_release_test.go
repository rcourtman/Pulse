//go:build release

package licensing

import "testing"

func TestReleaseBuildIgnoresMockModeForCommercialStartup(t *testing.T) {
	t.Setenv("PULSE_MOCK_MODE", "true")

	if IsDemoMode() {
		t.Fatal("release build must ignore PULSE_MOCK_MODE")
	}
}
