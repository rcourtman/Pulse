package alerts

import (
	"testing"
	"time"
)

func TestCleanupStaleMaps(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)

	// Populate maps with old data
	oldTime := time.Now().Add(-25 * time.Hour)
	recentTime := time.Now().Add(-1 * time.Hour)

	m.mu.Lock()
	// Flapping history
	m.flappingHistory["stale-flapping"] = []time.Time{oldTime}
	m.flappingHistory["recent-flapping"] = []time.Time{recentTime}
	m.flappingActive["stale-flapping"] = true

	// Suppressed until
	m.suppressedUntil["expired-suppression"] = oldTime
	m.suppressedUntil["active-suppression"] = time.Now().Add(1 * time.Hour)

	// Pending alerts
	m.pendingAlerts["stale-pending"] = oldTime
	m.pendingAlerts["recent-pending"] = recentTime

	// Should not be cleaned if active alert exists
	m.flappingHistory["active-alert-flapping"] = []time.Time{oldTime}
	m.activeAlerts["active-alert-flapping"] = &Alert{ID: "active-alert-flapping"}

	// Empty history should be cleaned
	m.flappingHistory["empty-flapping"] = []time.Time{}

	m.mu.Unlock()

	// Run cleanup
	m.cleanupStaleMaps()

	// Verify
	m.mu.Lock()
	if _, exists := m.flappingHistory["stale-flapping"]; exists {
		t.Error("stale-flapping should get removed")
	}
	if _, exists := m.flappingActive["stale-flapping"]; exists {
		t.Error("stale-flapping active flag should get removed")
	}
	if _, exists := m.flappingHistory["recent-flapping"]; !exists {
		t.Error("recent-flapping should NOT get removed")
	}
	if _, exists := m.flappingHistory["empty-flapping"]; exists {
		t.Error("empty-flapping should get removed")
	}

	if _, exists := m.suppressedUntil["expired-suppression"]; exists {
		t.Error("expired-suppression should get removed")
	}
	if _, exists := m.suppressedUntil["active-suppression"]; !exists {
		t.Error("active-suppression should NOT get removed")
	}

	if _, exists := m.pendingAlerts["stale-pending"]; exists {
		t.Error("stale-pending should get removed")
	}
	if _, exists := m.pendingAlerts["recent-pending"]; !exists {
		t.Error("recent-pending should NOT get removed")
	}

	if _, exists := m.flappingHistory["active-alert-flapping"]; !exists {
		t.Error("active-alert-flapping should NOT get removed")
	}
	m.mu.Unlock()

	// Should not be cleaned if active alert exists for pending
	m.mu.Lock()
	m.pendingAlerts["active-alert-pending"] = oldTime
	m.activeAlerts["active-alert-pending"] = &Alert{ID: "active-alert-pending"}
	m.mu.Unlock()

	// Run cleanup
	m.cleanupStaleMaps()

	// Verify
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.pendingAlerts["active-alert-pending"]; !exists {
		t.Error("active-alert-pending should NOT get removed")
	}
}
