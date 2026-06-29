package alerts

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func testGuestDiskCleanupConfig() AlertConfig {
	return AlertConfig{
		Enabled: true,
		GuestDefaults: ThresholdConfig{
			Disk: &HysteresisThreshold{Trigger: 90, Clear: 85},
		},
		TimeThreshold:  0,
		TimeThresholds: map[string]int{},
		Overrides:      make(map[string]ThresholdConfig),
	}
}

func TestCheckGuestClearsStalePerDiskAlertsWhenDiskSetChanges(t *testing.T) {
	m := newTestManager(t)
	m.UpdateConfig(testGuestDiskCleanupConfig())
	m.mu.Lock()
	m.config.ActivationState = ActivationActive
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	guestID := "cluster:node:101"
	staleResourceID := guestID + "-disk-old-root"
	staleAlertID := fmt.Sprintf("%s-disk", staleResourceID)

	m.mu.Lock()
	m.activeAlerts[staleAlertID] = &Alert{
		ID:           staleAlertID,
		Type:         "disk",
		ResourceID:   staleResourceID,
		ResourceName: "test-vm",
		Node:         "node",
		Instance:     "cluster",
		StartTime:    time.Now().Add(-time.Hour),
		LastSeen:     time.Now().Add(-time.Minute),
	}
	m.mu.Unlock()

	vm := models.VM{
		ID:       guestID,
		Name:     "test-vm",
		Node:     "node",
		Instance: "cluster",
		Status:   "running",
		CPU:      0.05,
		Memory: models.Memory{
			Usage: 20,
		},
		Disk: models.Disk{
			Usage: 20,
		},
		Disks: []models.Disk{
			{
				Mountpoint: "/",
				Device:     "scsi0",
				Total:      100,
				Used:       20,
				Free:       80,
				Usage:      20,
			},
		},
	}

	m.CheckGuest(vm, "cluster")

	m.mu.RLock()
	_, exists := m.activeAlerts[staleAlertID]
	m.mu.RUnlock()
	if exists {
		t.Fatalf("expected stale per-disk guest alert %q to be cleared", staleAlertID)
	}
}

func TestCheckGuestClearsPerDiskAlertsWhenGuestStopsRunning(t *testing.T) {
	m := newTestManager(t)
	m.UpdateConfig(testGuestDiskCleanupConfig())
	m.mu.Lock()
	m.config.ActivationState = ActivationActive
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	guestID := "cluster:node:102"
	staleResourceID := guestID + "-disk-root-scsi0"
	staleAlertID := fmt.Sprintf("%s-disk", staleResourceID)

	m.mu.Lock()
	m.activeAlerts[staleAlertID] = &Alert{
		ID:           staleAlertID,
		Type:         "disk",
		ResourceID:   staleResourceID,
		ResourceName: "test-vm",
		Node:         "node",
		Instance:     "cluster",
		StartTime:    time.Now().Add(-time.Hour),
		LastSeen:     time.Now().Add(-time.Minute),
	}
	m.mu.Unlock()

	vm := models.VM{
		ID:       guestID,
		Name:     "test-vm",
		Node:     "node",
		Instance: "cluster",
		Status:   "stopped",
	}

	m.CheckGuest(vm, "cluster")

	m.mu.RLock()
	_, exists := m.activeAlerts[staleAlertID]
	m.mu.RUnlock()
	if exists {
		t.Fatalf("expected per-disk guest alert %q to be cleared for stopped guest", staleAlertID)
	}
}

func TestCheckGuestPerDiskAlertDoesNotMigrateAcrossGuestIdentity(t *testing.T) {
	m := newTestManager(t)
	m.UpdateConfig(testGuestDiskCleanupConfig())
	m.mu.Lock()
	m.config.ActivationState = ActivationActive
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	oldGuestID := BuildGuestKey("cluster", "node-a", 1)
	oldAlertID := fmt.Sprintf("%s-disk", oldGuestID)
	start := time.Now().Add(-30 * time.Minute)

	m.mu.Lock()
	m.activeAlerts[oldAlertID] = &Alert{
		ID:           oldAlertID,
		Type:         "disk",
		ResourceID:   oldGuestID,
		ResourceName: "vm-1",
		Node:         "node-a",
		Instance:     "cluster",
		StartTime:    start,
		LastSeen:     start.Add(20 * time.Minute),
		Value:        95,
		Threshold:    90,
	}
	m.mu.Unlock()

	newGuestID := BuildGuestKey("cluster", "node-b", 101)
	vm := models.VM{
		ID:       newGuestID,
		VMID:     101,
		Name:     "vm-101",
		Node:     "node-b",
		Instance: "cluster",
		Status:   "running",
		Memory:   models.Memory{Usage: 20},
		Disk:     models.Disk{Usage: 20},
		Disks: []models.Disk{
			{
				Total: 100,
				Used:  95,
				Free:  5,
				Usage: 95,
			},
		},
	}

	m.CheckGuest(vm, "cluster")

	newAlertID := fmt.Sprintf("%s-disk", guestDiskResourceID(newGuestID, "disk-1"))

	m.mu.RLock()
	oldAlert, oldExists := m.activeAlerts[oldAlertID]
	newAlert, newExists := m.activeAlerts[newAlertID]
	m.mu.RUnlock()

	if !oldExists {
		t.Fatalf("expected standing alert %q to remain on its original guest", oldAlertID)
	}
	if oldAlert.ResourceID != oldGuestID || oldAlert.ResourceName != "vm-1" {
		t.Fatalf("standing alert migrated to the wrong guest: %#v", oldAlert)
	}
	if !newExists {
		t.Fatalf("expected separate per-disk alert %q for the new guest", newAlertID)
	}
	if newAlert.ResourceID != guestDiskResourceID(newGuestID, "disk-1") || newAlert.ResourceName != "vm-101" {
		t.Fatalf("new disk alert has wrong identity: %#v", newAlert)
	}
}

func TestCheckGuestPerDiskAlertKeepsIdentityWhenDeviceEvidenceChanges(t *testing.T) {
	m := newTestManager(t)
	m.UpdateConfig(testGuestDiskCleanupConfig())
	m.mu.Lock()
	m.config.ActivationState = ActivationActive
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	guestID := BuildGuestKey("cluster", "node-a", 201)
	vm := models.VM{
		ID:       guestID,
		VMID:     201,
		Name:     "vm-201",
		Node:     "node-a",
		Instance: "cluster",
		Status:   "running",
		Memory:   models.Memory{Usage: 20},
		Disk:     models.Disk{Usage: 20},
		Disks: []models.Disk{
			{
				Mountpoint: "/",
				Device:     "scsi0",
				Total:      100,
				Used:       95,
				Free:       5,
				Usage:      95,
			},
		},
	}

	m.CheckGuest(vm, "cluster")

	alertID := fmt.Sprintf("%s-disk", guestDiskResourceID(guestID, "root"))

	m.mu.RLock()
	firstAlert, exists := m.activeAlerts[alertID]
	m.mu.RUnlock()
	if !exists {
		t.Fatalf("expected per-disk alert %q", alertID)
	}
	firstStart := firstAlert.StartTime
	historyCount := len(m.historyManager.GetAllHistory(0))
	if historyCount != 1 {
		t.Fatalf("expected one history entry after first alert, got %d", historyCount)
	}

	vm.Disks[0].Device = "virtio0"
	vm.Disks[0].Usage = 96
	vm.Disks[0].Used = 96
	vm.Disks[0].Free = 4

	m.CheckGuest(vm, "cluster")

	m.mu.RLock()
	secondAlert, exists := m.activeAlerts[alertID]
	activeCount := len(m.activeAlerts)
	m.mu.RUnlock()
	if !exists {
		t.Fatalf("expected per-disk alert %q to remain active", alertID)
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active alert after device evidence changed, got %d", activeCount)
	}
	if !secondAlert.StartTime.Equal(firstStart) {
		t.Fatalf("expected alert start time to be preserved, got %v want %v", secondAlert.StartTime, firstStart)
	}
	if historyAfter := len(m.historyManager.GetAllHistory(0)); historyAfter != historyCount {
		t.Fatalf("expected no new history entry for same disk identity, got %d want %d", historyAfter, historyCount)
	}
}

func TestCheckGuestMigratesLegacyPerDiskAlertToStableIdentity(t *testing.T) {
	m := newTestManager(t)
	m.UpdateConfig(testGuestDiskCleanupConfig())
	m.mu.Lock()
	m.config.ActivationState = ActivationActive
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	guestID := BuildGuestKey("cluster", "node-a", 301)
	legacyResourceID := fmt.Sprintf("%s-disk-root-scsi0", guestID)
	legacyAlertID := fmt.Sprintf("%s-disk", legacyResourceID)
	stableResourceID := guestDiskResourceID(guestID, "root")
	stableAlertID := fmt.Sprintf("%s-disk", stableResourceID)
	start := time.Now().Add(-45 * time.Minute)
	lastNotified := start.Add(5 * time.Minute)

	legacyAlert := &Alert{
		ID:           legacyAlertID,
		Type:         "disk",
		ResourceID:   legacyResourceID,
		ResourceName: "vm-301",
		Node:         "node-a",
		Instance:     "cluster",
		StartTime:    start,
		LastSeen:     start.Add(40 * time.Minute),
		LastNotified: &lastNotified,
		Value:        95,
		Threshold:    90,
		Metadata: map[string]interface{}{
			"mountpoint": "/",
			"device":     "scsi0",
			"label":      "/",
		},
	}

	m.mu.Lock()
	m.activeAlerts[legacyAlertID] = legacyAlert
	m.recentAlerts[legacyAlertID] = legacyAlert
	m.alertRateLimit[legacyAlertID] = []time.Time{lastNotified}
	m.historyManager.AddAlert(*legacyAlert)
	m.mu.Unlock()

	vm := models.VM{
		ID:       guestID,
		VMID:     301,
		Name:     "vm-301",
		Node:     "node-a",
		Instance: "cluster",
		Status:   "running",
		Memory:   models.Memory{Usage: 20},
		Disk:     models.Disk{Usage: 20},
		Disks: []models.Disk{
			{
				Mountpoint: "/",
				Total:      100,
				Used:       96,
				Free:       4,
				Usage:      96,
			},
		},
	}

	m.CheckGuest(vm, "cluster")

	m.mu.RLock()
	_, legacyActive := m.activeAlerts[legacyAlertID]
	stableAlert, stableActive := m.activeAlerts[stableAlertID]
	_, oldRateLimit := m.alertRateLimit[legacyAlertID]
	_, newRateLimit := m.alertRateLimit[stableAlertID]
	m.mu.RUnlock()

	if legacyActive {
		t.Fatalf("expected legacy alert %q to migrate to stable identity", legacyAlertID)
	}
	if !stableActive {
		t.Fatalf("expected stable alert %q after migration", stableAlertID)
	}
	if stableAlert.ResourceID != stableResourceID {
		t.Fatalf("expected stable resource ID %q, got %q", stableResourceID, stableAlert.ResourceID)
	}
	if !stableAlert.StartTime.Equal(start) {
		t.Fatalf("expected start time to survive migration, got %v want %v", stableAlert.StartTime, start)
	}
	if stableAlert.LastNotified == nil || !stableAlert.LastNotified.Equal(lastNotified) {
		t.Fatalf("expected notification cooldown timestamp to survive migration, got %#v", stableAlert.LastNotified)
	}
	if oldRateLimit || !newRateLimit {
		t.Fatalf("expected rate-limit state to move to stable alert ID, old=%v new=%v", oldRateLimit, newRateLimit)
	}

	history := m.historyManager.GetAllHistory(0)
	if len(history) != 1 {
		t.Fatalf("expected migration to preserve a single history entry, got %d", len(history))
	}
	if history[0].ID != stableAlertID || history[0].ResourceID != stableResourceID {
		t.Fatalf("expected history to migrate to stable identity, got %#v", history[0])
	}
}
