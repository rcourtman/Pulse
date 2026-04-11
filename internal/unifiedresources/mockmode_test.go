package unifiedresources

import (
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/mockruntime"
)

var mockModeTestMu sync.Mutex

func enableMockMode(t *testing.T) {
	t.Helper()
	mockModeTestMu.Lock()
	previous := mockruntime.IsEnabled()
	t.Setenv("PULSE_MOCK_MODE", "true")
	mockruntime.SetEnabled(true)
	t.Cleanup(func() {
		mockruntime.SetEnabled(previous)
		mockModeTestMu.Unlock()
	})
}
