package mock

import "testing"

// mustSetEnabled toggles the package mock-mode state and fails the test on
// error so callers don't repeat the check at every toggle.
func mustSetEnabled(t testing.TB, enabled bool) {
	t.Helper()
	if err := SetEnabled(enabled); err != nil {
		t.Fatalf("SetEnabled(%v): %v", enabled, err)
	}
}
