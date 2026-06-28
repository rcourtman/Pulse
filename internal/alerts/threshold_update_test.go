package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// TestReevaluateActiveAlertsOnThresholdChange tests that active alerts are re-evaluated when thresholds change
func TestReevaluateActiveAlertsOnThresholdChange(t *testing.T) {
	// Create a new alert manager
	manager := NewManager()

	// Clear any alerts loaded from disk
	manager.mu.Lock()
	manager.activeAlerts = make(map[string]*Alert)
	manager.mu.Unlock()

	// Set initial configuration with CPU threshold at 80%
	initialConfig := AlertConfig{
		Enabled: true,
		GuestDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
		},
		NodeDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
		},
		StorageDefault: HysteresisThreshold{Trigger: 85, Clear: 80},
		Overrides:      make(map[string]ThresholdConfig),
	}
	manager.UpdateConfig(initialConfig)

	// Manually create an active alert for a VM with 91% CPU
	alertID := "test-vm-101-cpu"
	alert := &Alert{
		ID:           alertID,
		Type:         "cpu",
		Level:        AlertLevelWarning,
		ResourceID:   "test-vm-101",
		ResourceName: "test-vm",
		Node:         "pve1",
		Instance:     "qemu/101",
		Message:      "CPU usage is 91%",
		Value:        91.0,
		Threshold:    80.0,
		StartTime:    time.Now().Add(-5 * time.Minute),
		LastSeen:     time.Now(),
	}

	manager.mu.Lock()
	manager.activeAlerts[alertID] = alert
	initialAlertCount := len(manager.activeAlerts)
	manager.mu.Unlock()

	// Verify the alert is active
	if initialAlertCount != 1 {
		t.Fatalf("Expected 1 active alert, got %d", initialAlertCount)
	}

	// Update configuration with higher CPU threshold at 95%
	updatedConfig := initialConfig
	updatedConfig.GuestDefaults.CPU = &HysteresisThreshold{Trigger: 95, Clear: 90}
	manager.UpdateConfig(updatedConfig)

	// Wait a moment for any goroutines to complete
	time.Sleep(100 * time.Millisecond)

	// Check if the alert was resolved
	manager.mu.RLock()
	finalAlertCount := len(manager.activeAlerts)
	_, alertStillActive := manager.activeAlerts[alertID]
	manager.mu.RUnlock()

	if alertStillActive {
		t.Errorf("Expected alert to be resolved after threshold increase, but it's still active")
	}

	if finalAlertCount != 0 {
		t.Errorf("Expected 0 active alerts after threshold update, got %d", finalAlertCount)
	}

	// Verify the alert was added to recently resolved
	manager.resolvedMutex.RLock()
	_, wasResolved := manager.recentlyResolved[alertID]
	manager.resolvedMutex.RUnlock()

	if !wasResolved {
		t.Errorf("Expected alert to be in recently resolved list")
	}
}

// TestReevaluateActiveAlertsWithOverride tests threshold changes with resource-specific overrides
func TestReevaluateActiveAlertsWithOverride(t *testing.T) {
	manager := NewManager()

	// Clear any alerts loaded from disk
	manager.mu.Lock()
	manager.activeAlerts = make(map[string]*Alert)
	manager.mu.Unlock()

	// Set initial configuration
	initialConfig := AlertConfig{
		Enabled: true,
		GuestDefaults: ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
		},
		NodeDefaults: ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
		},
		StorageDefault: HysteresisThreshold{Trigger: 85, Clear: 80},
		Overrides:      make(map[string]ThresholdConfig),
	}
	manager.UpdateConfig(initialConfig)

	// Create an active alert for a specific VM with 91% CPU
	alertID := "test-vm-202-cpu"
	resourceID := "test-vm-202"
	alert := &Alert{
		ID:           alertID,
		Type:         "cpu",
		Level:        AlertLevelWarning,
		ResourceID:   resourceID,
		ResourceName: "test-vm",
		Node:         "pve1",
		Instance:     "qemu/101",
		Message:      "CPU usage is 91%",
		Value:        91.0,
		Threshold:    80.0,
		StartTime:    time.Now().Add(-5 * time.Minute),
		LastSeen:     time.Now(),
	}

	manager.mu.Lock()
	manager.activeAlerts[alertID] = alert
	manager.mu.Unlock()

	// Update configuration with an override for this specific VM to 95%
	updatedConfig := initialConfig
	updatedConfig.Overrides[resourceID] = ThresholdConfig{
		CPU: &HysteresisThreshold{Trigger: 95, Clear: 90},
	}
	manager.UpdateConfig(updatedConfig)

	// Wait a moment for any goroutines to complete
	time.Sleep(100 * time.Millisecond)

	// Check if the alert was resolved
	manager.mu.RLock()
	_, alertStillActive := manager.activeAlerts[alertID]
	manager.mu.RUnlock()

	if alertStillActive {
		t.Errorf("Expected alert to be resolved after override threshold increase, but it's still active")
	}
}

func TestReevaluateActiveAlertsWithLinkedHostOverride(t *testing.T) {
	manager := NewManager()

	manager.mu.Lock()
	manager.activeAlerts = make(map[string]*Alert)
	manager.mu.Unlock()

	initialConfig := AlertConfig{
		Enabled: true,
		HostDefaults: ThresholdConfig{
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
		},
		Overrides: make(map[string]ThresholdConfig),
	}
	manager.UpdateConfig(initialConfig)

	alertID := "host:host-proxmoxn3-memory"
	alert := &Alert{
		ID:           alertID,
		Type:         "memory",
		Level:        AlertLevelWarning,
		ResourceID:   "host:host-proxmoxn3",
		ResourceName: "proxmoxn3 (Host Agent)",
		Node:         "proxmoxn3",
		Instance:     "linux",
		Message:      "Host memory at 90.6%",
		Value:        90.6,
		Threshold:    85.0,
		StartTime:    time.Now().Add(-5 * time.Minute),
		LastSeen:     time.Now(),
		Metadata: map[string]interface{}{
			"resourceType": "Host",
			"hostId":       "host-proxmoxn3",
			"linkedNodeId": "ProxmoxCluster-proxmoxn3",
		},
	}

	manager.mu.Lock()
	manager.activeAlerts[alertID] = alert
	manager.mu.Unlock()

	updatedConfig := initialConfig
	updatedConfig.Overrides["ProxmoxCluster-proxmoxn3"] = ThresholdConfig{
		Memory: &HysteresisThreshold{Trigger: 97, Clear: 92},
	}
	manager.UpdateConfig(updatedConfig)

	time.Sleep(100 * time.Millisecond)

	manager.mu.RLock()
	_, alertStillActive := manager.activeAlerts[alertID]
	manager.mu.RUnlock()

	if alertStillActive {
		t.Errorf("expected linked host alert to be resolved after linked node override increase")
	}
}

// TestReevaluateActiveAlertsStillAboveThreshold tests that alerts stay active if still above threshold
func TestReevaluateActiveAlertsStillAboveThreshold(t *testing.T) {
	manager := NewManager()

	// Clear any alerts loaded from disk
	manager.mu.Lock()
	manager.activeAlerts = make(map[string]*Alert)
	manager.mu.Unlock()

	// Set initial configuration
	initialConfig := AlertConfig{
		Enabled: true,
		GuestDefaults: ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
		},
		NodeDefaults: ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
		},
		StorageDefault: HysteresisThreshold{Trigger: 85, Clear: 80},
		Overrides:      make(map[string]ThresholdConfig),
	}
	manager.UpdateConfig(initialConfig)

	// Create an active alert with 96% CPU
	alertID := "test-vm-303-cpu"
	alert := &Alert{
		ID:           alertID,
		Type:         "cpu",
		Level:        AlertLevelWarning,
		ResourceID:   "test-vm-303",
		ResourceName: "test-vm",
		Node:         "pve1",
		Instance:     "qemu/101",
		Message:      "CPU usage is 96%",
		Value:        96.0,
		Threshold:    80.0,
		StartTime:    time.Now().Add(-5 * time.Minute),
		LastSeen:     time.Now(),
	}

	manager.mu.Lock()
	manager.activeAlerts[alertID] = alert
	manager.mu.Unlock()

	// Update configuration with higher threshold at 90%, but alert value (96%) is still above it
	updatedConfig := initialConfig
	updatedConfig.GuestDefaults.CPU = &HysteresisThreshold{Trigger: 90, Clear: 85}
	manager.UpdateConfig(updatedConfig)

	// Wait a moment for any goroutines to complete
	time.Sleep(100 * time.Millisecond)

	// Alert should still be active since 96% > 90% trigger
	manager.mu.RLock()
	_, alertStillActive := manager.activeAlerts[alertID]
	manager.mu.RUnlock()

	if !alertStillActive {
		t.Errorf("Expected alert to remain active since value (96%%) is still above new threshold (90%%)")
	}
}

func TestCheckMetricMigratesGuestAlertAcrossNodeMove(t *testing.T) {
	manager := newTestManager(t)
	manager.ClearActiveAlerts()

	oldResourceID := BuildGuestKey("pve1", "node1", 101)
	newResourceID := BuildGuestKey("pve1", "node2", 101)
	oldAlertID := oldResourceID + "-cpu"
	newAlertID := newResourceID + "-cpu"
	start := time.Now().Add(-10 * time.Minute)
	ackTime := start.Add(1 * time.Minute)

	alert := &Alert{
		ID:           oldAlertID,
		Type:         "cpu",
		Level:        AlertLevelWarning,
		ResourceID:   oldResourceID,
		ResourceName: "vm101",
		Node:         "node1",
		Instance:     "pve1",
		Message:      "VM cpu at 95%",
		Value:        95,
		Threshold:    80,
		StartTime:    start,
		LastSeen:     start.Add(5 * time.Minute),
		Acknowledged: true,
		AckUser:      "tester",
		AckTime:      &ackTime,
	}

	manager.mu.Lock()
	manager.activeAlerts[oldAlertID] = alert
	manager.recentAlerts[oldAlertID] = alert
	manager.suppressedUntil[oldAlertID] = start.Add(2 * time.Minute)
	manager.alertRateLimit[oldAlertID] = []time.Time{start.Add(30 * time.Second)}
	manager.ackState[oldAlertID] = ackRecord{
		acknowledged: true,
		user:         "tester",
		time:         ackTime,
	}
	manager.flappingHistory[oldAlertID] = []time.Time{start.Add(45 * time.Second)}
	manager.flappingActive[oldAlertID] = true
	manager.historyManager.AddAlert(*alert)
	manager.mu.Unlock()

	manager.checkMetric(newResourceID, "vm101", "node2", "pve1", "VM", "cpu", 92, &HysteresisThreshold{Trigger: 80, Clear: 70}, nil)

	manager.mu.RLock()
	migrated, exists := manager.activeAlerts[newAlertID]
	_, oldExists := manager.activeAlerts[oldAlertID]
	_, oldAckExists := manager.ackState[oldAlertID]
	newAck, newAckExists := manager.ackState[newAlertID]
	_, oldSuppressed := manager.suppressedUntil[oldAlertID]
	_, newSuppressed := manager.suppressedUntil[newAlertID]
	_, oldRateLimit := manager.alertRateLimit[oldAlertID]
	_, newRateLimit := manager.alertRateLimit[newAlertID]
	_, oldFlapping := manager.flappingHistory[oldAlertID]
	_, newFlapping := manager.flappingHistory[newAlertID]
	manager.mu.RUnlock()

	if oldExists {
		t.Fatal("expected old node-scoped alert to be removed")
	}
	if !exists {
		t.Fatal("expected alert to migrate to the current node-scoped ID")
	}
	if migrated.Node != "node2" || migrated.ResourceID != newResourceID {
		t.Fatalf("expected migrated alert to target node2, got node=%q resource=%q", migrated.Node, migrated.ResourceID)
	}
	if !migrated.Acknowledged || migrated.AckUser != "tester" {
		t.Fatalf("expected migrated alert acknowledgment to be preserved, got %#v", migrated)
	}
	if migrated.StartTime != start {
		t.Fatalf("expected migrated alert start time to be preserved, got %v want %v", migrated.StartTime, start)
	}
	if oldAckExists || !newAckExists || !newAck.acknowledged {
		t.Fatalf("expected ack state to move to new alert ID, old=%v new=%v", oldAckExists, newAckExists)
	}
	if oldSuppressed || !newSuppressed {
		t.Fatalf("expected suppression window to move to new alert ID, old=%v new=%v", oldSuppressed, newSuppressed)
	}
	if oldRateLimit || !newRateLimit {
		t.Fatalf("expected rate limit state to move to new alert ID, old=%v new=%v", oldRateLimit, newRateLimit)
	}
	if oldFlapping || !newFlapping {
		t.Fatalf("expected flapping state to move to new alert ID, old=%v new=%v", oldFlapping, newFlapping)
	}

	history := manager.historyManager.GetAllHistory(1)
	if len(history) != 1 || history[0].ID != newAlertID {
		t.Fatalf("expected history entry to follow migrated alert ID, got %#v", history)
	}
}

func TestCheckMetricMigratesGuestDiskAlertAcrossNodeMove(t *testing.T) {
	manager := newTestManager(t)
	manager.ClearActiveAlerts()

	oldResourceID := BuildGuestKey("pve1", "node1", 101) + "-disk-root-scsi0"
	newResourceID := BuildGuestKey("pve1", "node2", 101) + "-disk-root-scsi0"
	oldAlertID := oldResourceID + "-disk"
	newAlertID := newResourceID + "-disk"
	start := time.Now().Add(-10 * time.Minute)

	alert := &Alert{
		ID:           oldAlertID,
		Type:         "disk",
		Level:        AlertLevelWarning,
		ResourceID:   oldResourceID,
		ResourceName: "vm101",
		Node:         "node1",
		Instance:     "pve1",
		Message:      "VM disk at 95%",
		Value:        95,
		Threshold:    90,
		StartTime:    start,
		LastSeen:     start.Add(5 * time.Minute),
	}

	manager.mu.Lock()
	manager.activeAlerts[oldAlertID] = alert
	manager.historyManager.AddAlert(*alert)
	manager.mu.Unlock()

	manager.checkMetric(newResourceID, "vm101", "node2", "pve1", "VM", "disk", 96, &HysteresisThreshold{Trigger: 90, Clear: 85}, nil)

	manager.mu.RLock()
	migrated, exists := manager.activeAlerts[newAlertID]
	_, oldExists := manager.activeAlerts[oldAlertID]
	manager.mu.RUnlock()

	if oldExists {
		t.Fatal("expected old node-scoped per-disk alert to be removed")
	}
	if !exists {
		t.Fatal("expected per-disk alert to migrate to the current node-scoped ID")
	}
	if migrated.ResourceID != newResourceID || migrated.StartTime != start {
		t.Fatalf("unexpected migrated disk alert: %#v", migrated)
	}
}

func TestCheckMetricDoesNotMigrateGuestDiskAlertAcrossGuestIdentity(t *testing.T) {
	manager := newTestManager(t)
	manager.ClearActiveAlerts()

	oldResourceID := BuildGuestKey("proxmox2", "proxmox2", 107) + "-disk-local-lvm-vm-107-disk-0"
	newResourceID := BuildGuestKey("proxmox2", "proxmox2", 108) + "-disk-local-lvm-vm-108-disk-0"
	oldAlertID := oldResourceID + "-disk"
	newAlertID := newResourceID + "-disk"
	start := time.Now().Add(-10 * time.Minute)

	alert := &Alert{
		ID:           oldAlertID,
		Type:         "disk",
		Level:        AlertLevelWarning,
		ResourceID:   oldResourceID,
		ResourceName: "wireguard",
		Node:         "proxmox2",
		Instance:     "proxmox2",
		Message:      "wireguard disk at 92.5%",
		Value:        92.5,
		Threshold:    90,
		StartTime:    start,
		LastSeen:     start.Add(5 * time.Minute),
	}

	manager.mu.Lock()
	manager.activeAlerts[oldAlertID] = alert
	manager.historyManager.AddAlert(*alert)
	manager.mu.Unlock()

	manager.checkMetric(newResourceID, "pulse", "proxmox2", "proxmox2", "VM", "disk", 63.6, &HysteresisThreshold{Trigger: 90, Clear: 85}, nil)

	manager.mu.RLock()
	preserved, oldExists := manager.activeAlerts[oldAlertID]
	_, newExists := manager.activeAlerts[newAlertID]
	manager.mu.RUnlock()

	if !oldExists {
		t.Fatal("expected guest 107 disk alert to stay on guest 107")
	}
	if preserved.ResourceID != oldResourceID || preserved.ResourceName != "wireguard" {
		t.Fatalf("guest 107 alert was mutated: %#v", preserved)
	}
	if newExists {
		t.Fatal("did not expect a guest 108 disk alert to be created")
	}
	if resolved := manager.GetResolvedAlert(newAlertID); resolved != nil {
		t.Fatalf("did not expect guest 108 resolved alert after evaluating guest 108 below threshold: %#v", resolved)
	}
}

func TestCheckMetricResolvesGuestAlertAfterNodeMove(t *testing.T) {
	manager := newTestManager(t)
	manager.ClearActiveAlerts()

	oldResourceID := BuildGuestKey("pve1", "node1", 202)
	newResourceID := BuildGuestKey("pve1", "node2", 202)
	oldAlertID := oldResourceID + "-cpu"
	newAlertID := newResourceID + "-cpu"
	start := time.Now().Add(-15 * time.Minute)

	alert := &Alert{
		ID:           oldAlertID,
		Type:         "cpu",
		Level:        AlertLevelWarning,
		ResourceID:   oldResourceID,
		ResourceName: "vm202",
		Node:         "node1",
		Instance:     "pve1",
		Message:      "VM cpu at 95%",
		Value:        95,
		Threshold:    80,
		StartTime:    start,
		LastSeen:     start.Add(10 * time.Minute),
	}

	manager.mu.Lock()
	manager.activeAlerts[oldAlertID] = alert
	manager.historyManager.AddAlert(*alert)
	manager.mu.Unlock()

	manager.checkMetric(newResourceID, "vm202", "node2", "pve1", "VM", "cpu", 5, &HysteresisThreshold{Trigger: 80, Clear: 70}, nil)

	manager.mu.RLock()
	_, oldExists := manager.activeAlerts[oldAlertID]
	_, newExists := manager.activeAlerts[newAlertID]
	manager.mu.RUnlock()

	if oldExists || newExists {
		t.Fatalf("expected migrated guest alert to resolve after node move, old=%v new=%v", oldExists, newExists)
	}

	resolved := manager.GetResolvedAlert(newAlertID)
	if resolved == nil || resolved.Alert == nil {
		t.Fatal("expected resolved alert to be recorded under the current node-scoped ID")
	}
	if resolved.Alert.ResourceID != newResourceID {
		t.Fatalf("expected resolved alert resource ID %q, got %q", newResourceID, resolved.Alert.ResourceID)
	}

	history := manager.historyManager.GetAllHistory(1)
	if len(history) != 1 || history[0].ID != newAlertID {
		t.Fatalf("expected history entry to resolve under migrated alert ID, got %#v", history)
	}
}

func TestReevaluateActiveStorageAlertsOnThresholdChange(t *testing.T) {
	manager := NewManager()

	manager.mu.Lock()
	manager.activeAlerts = make(map[string]*Alert)
	manager.mu.Unlock()

	initialConfig := AlertConfig{
		Enabled:        true,
		StorageDefault: HysteresisThreshold{Trigger: 85, Clear: 80},
		Overrides:      make(map[string]ThresholdConfig),
	}
	manager.UpdateConfig(initialConfig)
	manager.mu.Lock()
	manager.config.TimeThreshold = 0
	manager.config.TimeThresholds = map[string]int{}
	manager.mu.Unlock()

	storage := models.Storage{
		ID:       "Main-cluster-ceph-pool",
		Name:     "ceph-pool",
		Node:     "cluster",
		Instance: "Main",
		Status:   "available",
		Usage:    90,
	}
	manager.CheckStorage(storage)

	manager.mu.RLock()
	_, existsBefore := manager.activeAlerts["Main-cluster-ceph-pool-usage"]
	manager.mu.RUnlock()
	if !existsBefore {
		t.Fatalf("expected storage alert to exist before threshold update")
	}

	updatedConfig := initialConfig
	updatedConfig.StorageDefault = HysteresisThreshold{Trigger: 95, Clear: 90}
	manager.UpdateConfig(updatedConfig)

	time.Sleep(100 * time.Millisecond)

	manager.mu.RLock()
	_, existsAfter := manager.activeAlerts["Main-cluster-ceph-pool-usage"]
	manager.mu.RUnlock()
	if existsAfter {
		t.Errorf("expected storage alert to be resolved after threshold increase")
	}
}

// TestReevaluateActiveAlertsGuestNotMisclassifiedAsNode tests that guest alerts
// are not misclassified as node alerts when Instance == Node (single-node setups).
// This is the root cause of GitHub #1145.
func TestReevaluateActiveAlertsGuestNotMisclassifiedAsNode(t *testing.T) {
	manager := NewManager()

	manager.mu.Lock()
	manager.activeAlerts = make(map[string]*Alert)
	manager.mu.Unlock()

	// Configure: guest memory disabled (trigger=0), node memory enabled
	config := AlertConfig{
		Enabled: true,
		GuestDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 0, Clear: 0}, // Disabled
		},
		NodeDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80}, // Enabled
		},
		StorageDefault: HysteresisThreshold{Trigger: 85, Clear: 80},
		Overrides:      make(map[string]ThresholdConfig),
	}
	manager.UpdateConfig(config)

	// Create a guest alert where Instance == Node (single-node setup).
	// Guest resource IDs contain ":" (format instance:node:vmid).
	alertID := "pve1:pve1:101-memory"
	alert := &Alert{
		ID:           alertID,
		Type:         "memory",
		Level:        AlertLevelWarning,
		ResourceID:   "pve1:pve1:101",
		ResourceName: "test-vm",
		Node:         "pve1",
		Instance:     "pve1", // Same as Node — triggers the bug
		Message:      "Memory usage is 90%",
		Value:        90.0,
		Threshold:    85.0,
		StartTime:    time.Now().Add(-5 * time.Minute),
		LastSeen:     time.Now(),
	}

	manager.mu.Lock()
	manager.activeAlerts[alertID] = alert
	manager.mu.Unlock()

	// Re-apply same config to trigger reevaluation
	manager.UpdateConfig(config)
	time.Sleep(100 * time.Millisecond)

	// The guest alert should be resolved because GuestDefaults.Memory is disabled
	manager.mu.RLock()
	_, alertStillActive := manager.activeAlerts[alertID]
	manager.mu.RUnlock()

	if alertStillActive {
		t.Errorf("Guest alert should have been resolved when guest memory threshold is disabled, but it was misclassified as a node alert")
	}
}

func TestReevaluateActiveAlertsGuestUsesStableClusterOverrideAcrossNodeMove(t *testing.T) {
	manager := NewManager()

	manager.mu.Lock()
	manager.activeAlerts = make(map[string]*Alert)
	manager.mu.Unlock()

	currentResourceID := BuildGuestKey("pve1", "node2", 101)
	alertID := currentResourceID + "-memory"

	config := AlertConfig{
		Enabled: true,
		GuestDefaults: ThresholdConfig{
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
		},
		NodeDefaults:   ThresholdConfig{},
		StorageDefault: HysteresisThreshold{Trigger: 85, Clear: 80},
		Overrides:      map[string]ThresholdConfig{"pve1-101": {Memory: &HysteresisThreshold{Trigger: 95, Clear: 90}}},
	}
	manager.UpdateConfig(config)

	manager.mu.Lock()
	manager.activeAlerts[alertID] = &Alert{
		ID:         alertID,
		Type:       "memory",
		Level:      AlertLevelWarning,
		ResourceID: currentResourceID,
		Node:       "node2",
		Instance:   "pve1",
		Value:      88,
		Threshold:  85,
		StartTime:  time.Now().Add(-5 * time.Minute),
		LastSeen:   time.Now(),
	}
	manager.mu.Unlock()

	manager.UpdateConfig(config)
	time.Sleep(100 * time.Millisecond)

	manager.mu.RLock()
	_, exists := manager.activeAlerts[alertID]
	manager.mu.RUnlock()

	if exists {
		t.Fatalf("expected guest alert to be resolved when stable clustered override threshold is above current value")
	}
}
