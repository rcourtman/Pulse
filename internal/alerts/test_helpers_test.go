package alerts

import "testing"

func testLookupActiveAlert(t testing.TB, m *Manager, alertID string) (*Alert, bool) {
	t.Helper()

	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.getActiveAlertNoLock(alertID)
}

func testRequireActiveAlert(t testing.TB, m *Manager, alertID string) *Alert {
	t.Helper()

	alert, exists := testLookupActiveAlert(t, m, alertID)
	if !exists || alert == nil {
		t.Fatalf("expected active alert %q", alertID)
	}
	return alert
}

func testHasActiveAlert(t testing.TB, m *Manager, alertID string) bool {
	t.Helper()

	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.hasActiveAlertNoLock(alertID)
}

func testLookupResolvedAlert(t testing.TB, m *Manager, alertID string) (*ResolvedAlert, bool) {
	t.Helper()

	m.resolvedMutex.RLock()
	defer m.resolvedMutex.RUnlock()

	return m.getResolvedAlertNoLock(alertID)
}
