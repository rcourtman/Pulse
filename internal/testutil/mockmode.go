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
	setMockMode(t, enabled, nil)
}

// SetMockModeWithConfig uses an isolated estate for the lifetime of a test.
func SetMockModeWithConfig(t *testing.T, enabled bool, cfg mock.MockConfig) {
	t.Helper()
	setMockMode(t, enabled, &cfg)
}

func setMockMode(t *testing.T, enabled bool, cfg *mock.MockConfig) {
	t.Helper()
	mockModeTestMu.Lock()
	t.Cleanup(mockModeTestMu.Unlock)
	previous := mock.IsMockEnabled()
	t.Setenv("PULSE_MOCK_MODE", strconv.FormatBool(enabled))
	if cfg != nil {
		original := mock.GetConfig()
		t.Cleanup(func() { mock.SetMockConfig(original) })
		mock.SetMockConfig(*cfg)
	}
	t.Cleanup(func() {
		if err := mock.SetEnabled(previous); err != nil {
			t.Errorf("restore mock mode: %v", err)
		}
	})
	if err := mock.SetEnabled(enabled); err != nil {
		t.Fatalf("set mock mode: %v", err)
	}
}
