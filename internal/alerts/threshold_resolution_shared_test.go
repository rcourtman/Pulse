package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func boolPtr(v bool) *bool {
	return &v
}

func TestApplyThresholdOverrideIncludesLifecycleFields(t *testing.T) {
	m := newTestManager(t)

	base := ThresholdConfig{
		PoweredOffSeverity: AlertLevelWarning,
		Backup: &BackupAlertConfig{
			Enabled:       true,
			WarningDays:   3,
			CriticalDays:  7,
			FreshHours:    24,
			StaleHours:    72,
			AlertOrphaned: boolPtr(true),
			IgnoreVMIDs:   []string{"100"},
		},
		Snapshot: &SnapshotAlertConfig{
			Enabled:      true,
			WarningDays:  7,
			CriticalDays: 14,
		},
	}

	override := ThresholdConfig{
		PoweredOffSeverity: AlertLevelCritical,
		Backup: &BackupAlertConfig{
			Enabled:       false,
			WarningDays:   10,
			CriticalDays:  20,
			FreshHours:    12,
			StaleHours:    36,
			AlertOrphaned: boolPtr(false),
			IgnoreVMIDs:   []string{"200", "201"},
		},
		Snapshot: &SnapshotAlertConfig{
			Enabled:         false,
			WarningDays:     30,
			CriticalDays:    60,
			WarningSizeGiB:  10,
			CriticalSizeGiB: 20,
		},
	}

	got := m.applyThresholdOverride(base, override)

	if got.PoweredOffSeverity != AlertLevelCritical {
		t.Fatalf("PoweredOffSeverity = %q, want %q", got.PoweredOffSeverity, AlertLevelCritical)
	}
	if got.Backup == nil || got.Backup.Enabled {
		t.Fatalf("Backup override not applied: %+v", got.Backup)
	}
	if got.Backup.AlertOrphaned == nil || *got.Backup.AlertOrphaned {
		t.Fatalf("Backup AlertOrphaned override not applied: %+v", got.Backup)
	}
	if len(got.Backup.IgnoreVMIDs) != 2 || got.Backup.IgnoreVMIDs[0] != "200" {
		t.Fatalf("Backup IgnoreVMIDs override not applied: %+v", got.Backup.IgnoreVMIDs)
	}
	if got.Snapshot == nil || got.Snapshot.Enabled {
		t.Fatalf("Snapshot override not applied: %+v", got.Snapshot)
	}
}

func TestCheckGuestStoppedUsesResolvedThresholdsForPoweredOff(t *testing.T) {
	m := newTestManager(t)
	guestID := BuildGuestKey("pve1", "node1", 100)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.CustomRules = []CustomAlertRule{
		{
			Name:     "disable-powered-off",
			Enabled:  true,
			Priority: 10,
			FilterConditions: FilterStack{
				LogicalOperator: "AND",
			},
			Thresholds: ThresholdConfig{
				DisableConnectivity: true,
			},
		},
	}
	m.mu.Unlock()

	vm := models.VM{
		ID:       guestID,
		VMID:     100,
		Name:     "app01",
		Node:     "node1",
		Instance: "pve1",
		Status:   "stopped",
	}

	m.CheckGuest(vm, "pve1")
	m.CheckGuest(vm, "pve1")

	m.mu.RLock()
	_, alertExists := m.activeAlerts["guest-powered-off-"+guestID]
	_, confirmationExists := m.offlineConfirmations[guestID]
	m.mu.RUnlock()

	if alertExists {
		t.Fatalf("expected no powered-off alert for %q when resolved thresholds disable connectivity", guestID)
	}
	if confirmationExists {
		t.Fatalf("expected no powered-off confirmations for %q when resolved thresholds disable connectivity", guestID)
	}
}

func TestCheckUnifiedResourceUsesCanonicalGuestOverrideKey(t *testing.T) {
	m := newTestManager(t)
	resourceID := BuildGuestKey("pve1", "node1", 100)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.TimeThresholds = map[string]int{}
	m.config.GuestDefaults = ThresholdConfig{
		CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
	}
	m.config.Overrides = map[string]ThresholdConfig{
		resourceID: {
			CPU: &HysteresisThreshold{Trigger: 60, Clear: 55},
		},
	}
	m.mu.Unlock()

	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:       resourceID,
		Type:     "vm",
		Name:     "app01",
		Node:     "node1",
		Instance: "pve1",
		CPU:      &UnifiedResourceMetric{Percent: 65},
	})

	m.mu.RLock()
	_, exists := m.activeAlerts[resourceID+"-cpu"]
	m.mu.RUnlock()

	if !exists {
		t.Fatalf("expected canonical resource ID %q to be used for override lookup and alert IDs", resourceID)
	}
}

func TestCheckNodeDisabledOverrideClearsExistingAlerts(t *testing.T) {
	m := newTestManager(t)
	node := models.Node{
		ID:       "node/pve-1",
		Name:     "pve-1",
		Instance: "pve1",
		Status:   "offline",
	}

	m.mu.Lock()
	m.config.Enabled = true
	m.config.Overrides = map[string]ThresholdConfig{
		node.ID: {Disabled: true},
	}
	m.activeAlerts[node.ID+"-cpu"] = &Alert{ID: node.ID + "-cpu"}
	m.activeAlerts["node-offline-"+node.ID] = &Alert{ID: "node-offline-" + node.ID}
	m.nodeOfflineCount[node.ID] = 3
	m.mu.Unlock()

	m.CheckNode(node)

	m.mu.RLock()
	_, cpuExists := m.activeAlerts[node.ID+"-cpu"]
	_, offlineExists := m.activeAlerts["node-offline-"+node.ID]
	_, countExists := m.nodeOfflineCount[node.ID]
	m.mu.RUnlock()

	if cpuExists {
		t.Fatalf("expected CPU alert to be cleared for disabled node override")
	}
	if offlineExists {
		t.Fatalf("expected offline alert to be cleared for disabled node override")
	}
	if countExists {
		t.Fatalf("expected node offline tracking to be cleared for disabled node override")
	}
}
