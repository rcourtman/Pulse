package alerts

import (
	"testing"
)

// Regression: checkMetric stores canonical-identity alerts under the
// canonical state key, but its hysteresis resolution removed by the legacy
// "<resourceID>-<metric>" ID, which is not registered as an alias. The
// removal silently no-oped: a resolved notification went out while the
// alert stayed active and re-resolved on every subsequent poll. Guest
// per-disk usage alerts were the remaining production caller of this path.
func TestCheckMetricResolveRemovesCanonicallyKeyedAlert(t *testing.T) {
	manager := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)

	manager.mu.Lock()
	manager.config.Enabled = true
	manager.config.TimeThresholds = map[string]int{}
	manager.mu.Unlock()

	threshold := &HysteresisThreshold{Trigger: 80, Clear: 75}
	resourceID := "guest-123/disk:root"

	manager.checkMetric(resourceID, "test-vm", "node1", "qemu/123", "VM", "disk", 85, threshold, nil)

	manager.mu.Lock()
	activeAfterFire := len(manager.activeAlerts)
	manager.mu.Unlock()
	if activeAfterFire != 1 {
		t.Fatalf("active alerts after fire = %d, want 1", activeAfterFire)
	}

	manager.checkMetric(resourceID, "test-vm", "node1", "qemu/123", "VM", "disk", 70, threshold, nil)

	manager.mu.Lock()
	activeAfterResolve := len(manager.activeAlerts)
	manager.mu.Unlock()
	if activeAfterResolve != 0 {
		t.Fatalf("active alerts after resolve = %d, want 0 (stale alert left behind)", activeAfterResolve)
	}

	if resolved := manager.GetResolvedAlert(buildCanonicalStateID(resourceID, "metric-threshold:disk")); resolved == nil {
		t.Fatal("expected a recently-resolved entry for the cleared alert")
	}
}
