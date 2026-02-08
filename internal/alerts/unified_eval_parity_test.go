package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func disableTestTimeThresholds(m *Manager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.config.MetricTimeThresholds = nil
}

// TestParityCheckGuestUsesEvaluateUnifiedMetrics verifies that CheckGuest
// produces the same alerts as a direct evaluateUnifiedMetrics call for
// standard metrics (CPU, memory, disk, I/O).
func TestParityCheckGuestUsesEvaluateUnifiedMetrics(t *testing.T) {
	// Create two managers with identical config
	guestMgr := NewManager()
	unifiedMgr := NewManager()

	cfg := AlertConfig{
		Enabled: true,
		GuestDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
		},
	}
	guestMgr.UpdateConfig(cfg)
	unifiedMgr.UpdateConfig(cfg)
	disableTestTimeThresholds(guestMgr)
	disableTestTimeThresholds(unifiedMgr)

	// Call CheckGuest with a VM above CPU threshold
	vm := models.VM{
		ID:     "qemu/100",
		Name:   "test-vm",
		Node:   "pve-1",
		Status: "running",
		CPU:    0.90, // 90% (CheckGuest multiplies by 100)
		Memory: models.Memory{Usage: 70},
		Disk:   models.Disk{Usage: 50},
	}
	guestMgr.CheckGuest(vm, "test")

	// Call evaluateUnifiedMetrics with same data
	unifiedMgr.evaluateUnifiedMetrics(&UnifiedResourceInput{
		ID:       "qemu/100",
		Type:     "vm",
		Name:     "test-vm",
		Node:     "pve-1",
		Instance: "test",
		CPU:      &UnifiedResourceMetric{Percent: 90},
		Memory:   &UnifiedResourceMetric{Percent: 70},
		Disk:     &UnifiedResourceMetric{Percent: 50},
	}, cfg.GuestDefaults, nil)

	// Both should have one CPU alert
	guestAlerts := guestMgr.GetActiveAlerts()
	unifiedAlerts := unifiedMgr.GetActiveAlerts()

	if len(guestAlerts) != len(unifiedAlerts) {
		t.Fatalf("alert count mismatch: CheckGuest=%d, evaluateUnifiedMetrics=%d",
			len(guestAlerts), len(unifiedAlerts))
	}

	if len(guestAlerts) != 1 {
		t.Fatalf("expected 1 alert (CPU), got %d", len(guestAlerts))
	}

	if guestAlerts[0].Type != unifiedAlerts[0].Type {
		t.Fatalf("alert type mismatch: %s vs %s", guestAlerts[0].Type, unifiedAlerts[0].Type)
	}

	if guestAlerts[0].ResourceID != unifiedAlerts[0].ResourceID {
		t.Fatalf("resource ID mismatch: %s vs %s", guestAlerts[0].ResourceID, unifiedAlerts[0].ResourceID)
	}
}

// TestParityCheckNodeUsesEvaluateUnifiedMetrics verifies that CheckNode
// produces the same metric alerts as evaluateUnifiedMetrics for CPU, memory, disk.
func TestParityCheckNodeUsesEvaluateUnifiedMetrics(t *testing.T) {
	nodeMgr := NewManager()
	unifiedMgr := NewManager()

	cfg := AlertConfig{
		Enabled: true,
		NodeDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:   &HysteresisThreshold{Trigger: 90, Clear: 85},
		},
	}
	nodeMgr.UpdateConfig(cfg)
	unifiedMgr.UpdateConfig(cfg)
	disableTestTimeThresholds(nodeMgr)
	disableTestTimeThresholds(unifiedMgr)

	node := models.Node{
		ID:       "node/pve-1",
		Name:     "pve-1",
		Instance: "pve-1",
		Status:   "online",
		CPU:      0.92, // 92% (CheckNode multiplies by 100)
		Memory:   models.Memory{Usage: 88},
		Disk:     models.Disk{Usage: 50},
	}
	nodeMgr.CheckNode(node)

	unifiedMgr.evaluateUnifiedMetrics(&UnifiedResourceInput{
		ID:       "node/pve-1",
		Type:     "node",
		Name:     "pve-1",
		Node:     "pve-1",
		Instance: "pve-1",
		CPU:      &UnifiedResourceMetric{Percent: 92},
		Memory:   &UnifiedResourceMetric{Percent: 88},
		Disk:     &UnifiedResourceMetric{Percent: 50},
	}, cfg.NodeDefaults, nil)

	nodeAlerts := nodeMgr.GetActiveAlerts()
	unifiedAlerts := unifiedMgr.GetActiveAlerts()

	if len(nodeAlerts) != len(unifiedAlerts) {
		t.Fatalf("alert count mismatch: CheckNode=%d, evaluateUnifiedMetrics=%d",
			len(nodeAlerts), len(unifiedAlerts))
	}

	// Should have 2 alerts: CPU (92 > 80) and Memory (88 > 85)
	if len(nodeAlerts) != 2 {
		t.Fatalf("expected 2 alerts (CPU + Memory), got %d", len(nodeAlerts))
	}
}

// TestParityCheckPBSUsesEvaluateUnifiedMetrics verifies that CheckPBS
// produces the same metric alerts as evaluateUnifiedMetrics for CPU, memory.
func TestParityCheckPBSUsesEvaluateUnifiedMetrics(t *testing.T) {
	pbsMgr := NewManager()
	unifiedMgr := NewManager()

	cpuThreshold := &HysteresisThreshold{Trigger: 80, Clear: 75}
	memThreshold := &HysteresisThreshold{Trigger: 85, Clear: 80}

	cfg := AlertConfig{
		Enabled: true,
		PBSDefaults: ThresholdConfig{
			CPU:    cpuThreshold,
			Memory: memThreshold,
		},
	}
	pbsMgr.UpdateConfig(cfg)
	unifiedMgr.UpdateConfig(cfg)
	disableTestTimeThresholds(pbsMgr)
	disableTestTimeThresholds(unifiedMgr)

	pbs := models.PBSInstance{
		ID:     "pbs-1",
		Name:   "backup-server",
		Host:   "192.168.1.100",
		Status: "online",
		CPU:    85, // already percentage
		Memory: 60,
	}
	pbsMgr.CheckPBS(pbs)

	unifiedMgr.evaluateUnifiedMetrics(&UnifiedResourceInput{
		ID:       "pbs-1",
		Type:     "pbs",
		Name:     "backup-server",
		Node:     "192.168.1.100",
		Instance: "backup-server",
		CPU:      &UnifiedResourceMetric{Percent: 85},
		Memory:   &UnifiedResourceMetric{Percent: 60},
	}, ThresholdConfig{CPU: cpuThreshold, Memory: memThreshold}, nil)

	pbsAlerts := pbsMgr.GetActiveAlerts()
	unifiedAlerts := unifiedMgr.GetActiveAlerts()

	if len(pbsAlerts) != len(unifiedAlerts) {
		t.Fatalf("alert count mismatch: CheckPBS=%d, evaluateUnifiedMetrics=%d",
			len(pbsAlerts), len(unifiedAlerts))
	}

	// Should have 1 alert: CPU (85 > 80)
	if len(pbsAlerts) != 1 {
		t.Fatalf("expected 1 alert (CPU), got %d", len(pbsAlerts))
	}
}
