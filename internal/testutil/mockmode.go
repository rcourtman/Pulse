package testutil

import (
	"strconv"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
)

var mockModeTestMu sync.Mutex

// SetMockMode synchronizes the canonical in-process mock runtime state for a
// test and keeps the legacy env flag aligned for code paths that still read it.
func SetMockMode(t *testing.T, enabled bool) {
	t.Helper()

	mockModeTestMu.Lock()
	previous := mock.IsMockEnabled()
	t.Setenv("PULSE_MOCK_MODE", strconv.FormatBool(enabled))
	if err := mock.SetEnabled(enabled); err != nil {
		mockModeTestMu.Unlock()
		t.Fatalf("set mock mode: %v", err)
	}
	t.Cleanup(func() {
		if err := mock.SetEnabled(previous); err != nil {
			t.Fatalf("restore mock mode: %v", err)
		}
		mockModeTestMu.Unlock()
	})
}
