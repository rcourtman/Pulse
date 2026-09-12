package alerts

import (
	"testing"
	"time"
)

func TestCleanupStaleMapsDoesNotManufactureRecovery(t *testing.T) {
	m := newTestManager(t)
	old := time.Now().Add(-25 * time.Hour)
	ids := []string{"guest-powered-state", "node-connectivity", "pbs-connectivity", "agent-connectivity", SystemAlertID(NotificationDeliveryAlertType)}
	m.mu.Lock()
	for _, id := range ids {
		m.setActiveAlertNoLock(id, &Alert{ID: id, ResourceID: id, StartTime: old, LastSeen: old, Level: AlertLevelWarning})
	}
	m.mu.Unlock()

	// An absent observation is not recovery, even across repeated cleanup runs.
	for sweep := 0; sweep < 2; sweep++ {
		m.cleanupStaleMaps()
		if got := len(m.GetActiveAlerts()); got != len(ids) {
			t.Fatalf("cleanup %d retained %d alerts, want %d without recovery evidence", sweep, got, len(ids))
		}
		if got := m.GetRecentlyResolved(); len(got) != 0 {
			t.Fatalf("cleanup manufactured recovery events: %+v", got)
		}
	}

	// Explicit resolution remains available and records the actual transition.
	if !m.ClearAlert(ids[0]) {
		t.Fatal("explicit resolution failed")
	}
	m.cleanupStaleMaps()
	if got := len(m.GetActiveAlerts()); got != len(ids)-1 {
		t.Fatalf("active alerts after explicit resolution = %d", got)
	}
	if got := m.GetRecentlyResolved(); len(got) != 1 || got[0].Alert.ID != ids[0] {
		t.Fatalf("expected only the explicitly resolved occurrence, got %+v", got)
	}
}
