package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
)

// mustSetMockEnabled toggles the package mock-mode state and fails the test
// on error so callers don't repeat the check at every toggle.
func mustSetMockEnabled(t testing.TB, enabled bool) {
	t.Helper()
	if err := mock.SetEnabled(enabled); err != nil {
		t.Fatalf("mock.SetEnabled(%v): %v", enabled, err)
	}
}

// mustSetMonitorMockMode flips a monitor between mock and real data and fails
// the test on error.
func mustSetMonitorMockMode(t testing.TB, m *Monitor, enabled bool) {
	t.Helper()
	if err := m.SetMockMode(enabled); err != nil {
		t.Fatalf("SetMockMode(%v): %v", enabled, err)
	}
}
