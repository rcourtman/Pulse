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
	// Test additional maps
	m.mu.Lock()
	// Offline confirmations
	m.offlineConfirmations["stale-node"] = 3
	m.activeAlerts["node:active-node:offline"] = &Alert{ID: "node:active-node:offline"}
	m.offlineConfirmations["active-node"] = 3

	// Node offline count
	m.nodeOfflineCount["stale-node-legacy"] = 5
	m.activeAlerts["node:active-node-legacy:offline"] = &Alert{ID: "node:active-node-legacy:offline"}
	m.nodeOfflineCount["active-node-legacy"] = 5

	// Docker state confirm
	m.dockerStateConfirm["stale-container"] = 2
	m.activeAlerts["docker:active-container:state"] = &Alert{ID: "docker:active-container:state"}
	m.dockerStateConfirm["active-container"] = 2

	// Docker offline count
	m.dockerOfflineCount["stale-host"] = 4
	m.activeAlerts["docker:active-host:offline"] = &Alert{ID: "docker:active-host:offline"}
	m.dockerOfflineCount["active-host"] = 4

	// Docker restart tracking
	m.dockerRestartTracking["stale-restart"] = &dockerRestartRecord{lastChecked: oldTime}
	m.dockerLastExitCode["stale-restart"] = 1
	m.dockerRestartTracking["recent-restart"] = &dockerRestartRecord{lastChecked: recentTime}

	// Alert rate limit
	m.alertRateLimit["stale-rate"] = []time.Time{oldTime.Add(-2 * time.Hour)}
	m.alertRateLimit["mixed-rate"] = []time.Time{oldTime, recentTime}

	// Recent alerts
	m.recentAlerts["stale-recent"] = &Alert{ID: "stale-recent", LastSeen: oldTime}
	m.recentAlerts["recent-recent"] = &Alert{ID: "recent-recent", LastSeen: recentTime}

	// Ack state
	m.ackState["stale-ack"] = ackRecord{inactiveAt: oldTime}
	m.ackState["stale-ack-no-inactive"] = ackRecord{time: oldTime}
	m.ackState["recent-ack"] = ackRecord{inactiveAt: recentTime}
	m.activeAlerts["active-ack"] = &Alert{ID: "active-ack"}
	m.ackState["active-ack"] = ackRecord{inactiveAt: oldTime}

	m.mu.Unlock()

	// Run cleanup
	m.cleanupStaleMaps()

	// Verify additional maps
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.offlineConfirmations["stale-node"]; exists {
		t.Error("stale-node offline connection should be removed")
	}
	if _, exists := m.offlineConfirmations["active-node"]; !exists {
		t.Error("active-node offline connection should NOT be removed")
	}

	if _, exists := m.nodeOfflineCount["stale-node-legacy"]; exists {
		t.Error("stale-node-legacy should be removed")
	}
	if _, exists := m.nodeOfflineCount["active-node-legacy"]; !exists {
		t.Error("active-node-legacy should NOT be removed")
	}

	if _, exists := m.dockerStateConfirm["stale-container"]; exists {
		t.Error("stale-container should be removed")
	}
	if _, exists := m.dockerStateConfirm["active-container"]; !exists {
		t.Error("active-container should NOT be removed")
	}

	if _, exists := m.dockerOfflineCount["stale-host"]; exists {
		t.Error("stale-host should be removed")
	}
	if _, exists := m.dockerOfflineCount["active-host"]; !exists {
		t.Error("active-host should NOT be removed")
	}

	if _, exists := m.dockerRestartTracking["stale-restart"]; exists {
		t.Error("stale-restart should be removed")
	}
	if _, exists := m.dockerLastExitCode["stale-restart"]; exists {
		t.Error("stale-restart exit code should be removed")
	}
	if _, exists := m.dockerRestartTracking["recent-restart"]; !exists {
		t.Error("recent-restart should NOT be removed")
	}

	if _, exists := m.alertRateLimit["stale-rate"]; exists {
		t.Error("stale-rate should be removed")
	}
	if times, exists := m.alertRateLimit["mixed-rate"]; !exists || len(times) != 1 {
		t.Errorf("mixed-rate should have 1 entry, got %v", len(times))
	}

	if _, exists := m.recentAlerts["stale-recent"]; exists {
		t.Error("stale-recent should be removed")
	}
	if _, exists := m.recentAlerts["recent-recent"]; !exists {
		t.Error("recent-recent should NOT be removed")
	}

	if _, exists := m.ackState["stale-ack"]; exists {
		t.Error("stale-ack should be removed")
	}
	if _, exists := m.ackState["stale-ack-no-inactive"]; exists {
		t.Error("stale-ack-no-inactive should be removed")
	}
	if _, exists := m.ackState["recent-ack"]; !exists {
		t.Error("recent-ack should NOT be removed")
	}
	if _, exists := m.ackState["active-ack"]; !exists {
		t.Error("active-ack should NOT be removed")
	}
}
