package alerts

import (
	"testing"
	"time"
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
	resourceID := "test-vm-101"
	alertID := canonicalMetricStateID(resourceID, "cpu")
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
	_, alertStillActive := testLookupActiveAlert(t, manager, alertID)
	manager.mu.RUnlock()

	if alertStillActive {
		t.Errorf("Expected alert to be resolved after threshold increase, but it's still active")
	}

	if finalAlertCount != 0 {
		t.Errorf("Expected 0 active alerts after threshold update, got %d", finalAlertCount)
	}

	// Verify the alert was added to recently resolved
	manager.resolvedMutex.RLock()
	_, wasResolved := manager.recentlyResolved[canonicalMetricStateID(resourceID, "cpu")]
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
	resourceID := "test-vm-202"
	alertID := canonicalMetricStateID(resourceID, "cpu")
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
	_, alertStillActive := testLookupActiveAlert(t, manager, alertID)
	manager.mu.RUnlock()

	if alertStillActive {
		t.Errorf("Expected alert to be resolved after override threshold increase, but it's still active")
	}
}

func TestReevaluateActiveAlertsUsesStableClusterGuestOverrideAcrossNodeMove(t *testing.T) {
	manager := NewManager()

	manager.mu.Lock()
	manager.activeAlerts = make(map[string]*Alert)
	manager.mu.Unlock()

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

	resourceID := BuildGuestKey("pve1", "node2", 202)
	alertID := canonicalMetricStateID(resourceID, "cpu")
	alert := &Alert{
		ID:           alertID,
		Type:         "cpu",
		Level:        AlertLevelWarning,
		ResourceID:   resourceID,
		ResourceName: "moved-vm",
		Node:         "node2",
		Instance:     "pve1",
		Message:      "CPU usage is 91%",
		Value:        91.0,
		Threshold:    80.0,
		StartTime:    time.Now().Add(-5 * time.Minute),
		LastSeen:     time.Now(),
	}

	manager.mu.Lock()
	manager.activeAlerts[alertID] = alert
	manager.mu.Unlock()

	updatedConfig := initialConfig
	updatedConfig.Overrides[stableGuestOverrideKey("pve1", 202)] = ThresholdConfig{
		CPU: &HysteresisThreshold{Trigger: 95, Clear: 90},
	}
	manager.UpdateConfig(updatedConfig)

	time.Sleep(100 * time.Millisecond)

	manager.mu.RLock()
	_, alertStillActive := testLookupActiveAlert(t, manager, alertID)
	manager.mu.RUnlock()

	if alertStillActive {
		t.Fatalf("expected alert to resolve after stable clustered override threshold increase")
	}
}

func TestCheckMetricMigratesGuestAlertAcrossNodeMove(t *testing.T) {
	manager := newTestManager(t)
	manager.ClearActiveAlerts()

	oldResourceID := BuildGuestKey("pve1", "node1", 101)
	newResourceID := BuildGuestKey("pve1", "node2", 101)
	oldState := canonicalMetricStateID(oldResourceID, "cpu")
	newState := canonicalMetricStateID(newResourceID, "cpu")
	start := time.Now().Add(-10 * time.Minute)
	ackTime := start.Add(time.Minute)

	alert := &Alert{
		ID:              oldState,
		Type:            "cpu",
		Level:           AlertLevelWarning,
		ResourceID:      oldResourceID,
		CanonicalSpecID: canonicalMetricSpecID(oldResourceID, "cpu"),
		CanonicalKind:   "metric-threshold",
		CanonicalState:  oldState,
		ResourceName:    "vm101",
		Node:            "node1",
		Instance:        "pve1",
		Message:         "VM cpu at 95%",
		Value:           95,
		Threshold:       80,
		StartTime:       start,
		LastSeen:        start.Add(5 * time.Minute),
		Acknowledged:    true,
		AckUser:         "tester",
		AckTime:         &ackTime,
	}

	manager.mu.Lock()
	manager.setActiveAlertNoLock(oldState, alert)
	manager.recentAlerts[oldState] = alert
	manager.suppressedUntil[oldState] = start.Add(2 * time.Minute)
	manager.alertRateLimit[oldState] = []time.Time{start.Add(30 * time.Second)}
	manager.ackStateByCanonical[oldState] = ackRecord{
		acknowledged: true,
		user:         "tester",
		time:         ackTime,
	}
	manager.flappingHistory[oldState] = []time.Time{start.Add(45 * time.Second)}
	manager.flappingActive[oldState] = true
	manager.historyManager.AddAlert(*alert)
	manager.mu.Unlock()

	manager.checkMetric(newResourceID, "vm101", "node2", "pve1", "VM", "cpu", 92, &HysteresisThreshold{Trigger: 80, Clear: 70}, nil)

	manager.mu.RLock()
	migrated, exists := manager.activeAlerts[newState]
	_, oldExists := manager.activeAlerts[oldState]
	_, oldAckExists := manager.ackStateByCanonical[oldState]
	newAck, newAckExists := manager.ackStateByCanonical[newState]
	_, oldSuppressed := manager.suppressedUntil[oldState]
	_, newSuppressed := manager.suppressedUntil[newState]
	_, oldRateLimit := manager.alertRateLimit[oldState]
	_, newRateLimit := manager.alertRateLimit[newState]
	_, oldFlapping := manager.flappingHistory[oldState]
	_, newFlapping := manager.flappingHistory[newState]
	manager.mu.RUnlock()

	if oldExists {
		t.Fatal("expected old node-scoped alert to be removed")
	}
	if !exists {
		t.Fatal("expected alert to migrate to the current node-scoped canonical state")
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
		t.Fatalf("expected canonical ack state to move to new alert ID, old=%v new=%v", oldAckExists, newAckExists)
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

	history := manager.GetAlertHistory(1)
	if len(history) != 1 || history[0].ID != newState {
		t.Fatalf("expected history entry to follow migrated alert ID, got %#v", history)
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
	resourceID := "test-vm-303"
	alertID := canonicalMetricStateID(resourceID, "cpu")
	alert := &Alert{
		ID:           alertID,
		Type:         "cpu",
		Level:        AlertLevelWarning,
		ResourceID:   resourceID,
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
	_, alertStillActive := testLookupActiveAlert(t, manager, alertID)
	manager.mu.RUnlock()

	if !alertStillActive {
		t.Errorf("Expected alert to remain active since value (96%%) is still above new threshold (90%%)")
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
