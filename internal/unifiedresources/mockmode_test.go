package unifiedresources

import "testing"

func enableMockMode(t *testing.T) {
	t.Helper()
	t.Setenv("PULSE_MOCK_MODE", "true")
}
