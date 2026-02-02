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
