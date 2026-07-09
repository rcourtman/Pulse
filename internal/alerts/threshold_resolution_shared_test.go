package alerts

import (
	"testing"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
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

	exists := testHasActiveAlert(t, m, canonicalMetricStateID(resourceID, "cpu"))

	if !exists {
		t.Fatalf("expected canonical resource ID %q to be used for override lookup and alert IDs", resourceID)
	}
}

func TestCheckUnifiedResourceUsesStableClusteredGuestOverrideKey(t *testing.T) {
	m := newTestManager(t)
	resourceID := BuildGuestKey("pve1", "node2", 101)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.TimeThresholds = map[string]int{}
	m.config.GuestDefaults = ThresholdConfig{
		CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
	}
	m.config.Overrides = map[string]ThresholdConfig{
		stableGuestOverrideKey("pve1", 101): {
			CPU: &HysteresisThreshold{Trigger: 60, Clear: 55},
		},
	}
	m.mu.Unlock()

	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:       resourceID,
		Type:     "vm",
		Name:     "app02",
		Node:     "node2",
		Instance: "pve1",
		CPU:      &UnifiedResourceMetric{Percent: 65},
	})

	exists := testHasActiveAlert(t, m, canonicalMetricStateID(resourceID, "cpu"))
	if !exists {
		t.Fatalf("expected stable clustered guest override to resolve for canonical resource ID %q", resourceID)
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
	cpuState, cpuAlert := testNewCanonicalAlert(node.ID, canonicalMetricSpecID(node.ID, "cpu"), string(alertspecs.AlertSpecKindMetricThreshold), "cpu")
	offlineState, offlineAlert := testNewCanonicalAlert(node.ID, canonicalConnectivitySpecID(node.ID), string(alertspecs.AlertSpecKindConnectivity), "offline")
	m.setActiveAlertNoLock(cpuState, cpuAlert)
	m.setActiveAlertNoLock(offlineState, offlineAlert)
	m.nodeOfflineCount[node.ID] = 3
	m.mu.Unlock()

	m.CheckNode(node)

	m.mu.RLock()
	_, cpuExists := m.activeAlerts[cpuState]
	_, offlineExists := m.activeAlerts[offlineState]
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

func TestReevaluateActiveAlertsUsesSharedAgentOverrideResolution(t *testing.T) {
	m := newTestManager(t)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.AgentDefaults = ThresholdConfig{
		CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
	}
	m.config.Overrides = map[string]ThresholdConfig{
		"host1": {
			Disabled: true,
		},
	}
	m.activeAlerts["agent:host1-cpu"] = &Alert{
		ID:        "agent:host1-cpu",
		Type:      "cpu",
		Value:     95,
		Threshold: 80,
		Metadata: map[string]interface{}{
			"resourceType": "Agent",
		},
	}
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts["agent:host1-cpu"]
	m.mu.RUnlock()

	if exists {
		t.Fatalf("expected reevaluation to resolve agent alert using raw host override key")
	}
}

func TestReevaluateActiveAlertsUsesSharedStorageOverrideResolution(t *testing.T) {
	m := newTestManager(t)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.StorageDefault = HysteresisThreshold{Trigger: 85, Clear: 80}
	m.config.Overrides = map[string]ThresholdConfig{
		"storage1": {
			Disabled: true,
		},
	}
	m.activeAlerts["storage1-usage"] = &Alert{
		ID:        "storage1-usage",
		Type:      "usage",
		Value:     95,
		Threshold: 85,
		Instance:  "Storage",
	}
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts["storage1-usage"]
	m.mu.RUnlock()

	if exists {
		t.Fatalf("expected reevaluation to resolve storage alert when shared override disables storage alerting")
	}
}

func TestReevaluateActiveAlertsUsesStorageMetadataForCephPoolAlias(t *testing.T) {
	m := newTestManager(t)
	resourceID := "pve5-ceph-pool-data_replication"

	m.mu.Lock()
	m.config.Enabled = true
	m.config.StorageDefault = HysteresisThreshold{Trigger: 95, Clear: 90}
	m.config.Overrides = map[string]ThresholdConfig{
		"agent:pve5-ceph-pool-data_replication": {
			Usage: &HysteresisThreshold{Trigger: 50, Clear: 45},
		},
	}
	state, alert := testNewCanonicalAlert(resourceID, canonicalMetricSpecID(resourceID, "usage"), string(alertspecs.AlertSpecKindMetricThreshold), "usage")
	alert.Value = 60
	alert.Threshold = 50
	alert.ResourceName = "data_replication"
	alert.Node = "pve5"
	alert.Instance = "pve5"
	alert.Metadata = map[string]interface{}{
		"resourceType": "Storage",
	}
	m.setActiveAlertNoLock(state, alert)
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts[state]
	m.mu.RUnlock()

	if !exists {
		t.Fatalf("expected Ceph pool storage alert to remain active under agent-prefixed alias override")
	}
}

func TestStorageThresholdResolutionUsesAliasIDs(t *testing.T) {
	m := newTestManager(t)

	m.mu.Lock()
	m.config.StorageDefault = HysteresisThreshold{Trigger: 95, Clear: 90}
	m.config.Overrides = map[string]ThresholdConfig{
		"agent:pve5-ceph-pool-data_replication": {
			Usage: &HysteresisThreshold{Trigger: 50, Clear: 45},
		},
	}
	thresholds := m.resolveStorageThresholdsNoLock(models.Storage{
		ID:       "pve5-ceph-pool-data_replication",
		AliasIDs: []string{"agent:pve5-ceph-pool-data_replication"},
	})
	m.mu.Unlock()

	if thresholds.Usage == nil {
		t.Fatalf("expected usage threshold to be resolved")
	}
	if thresholds.Usage.Trigger != 50 || thresholds.Usage.Clear != 45 {
		t.Fatalf("usage threshold = %#v, want alias override trigger 50 clear 45", thresholds.Usage)
	}
}

func TestCheckStorageOfflineUsesSharedThresholdResolution(t *testing.T) {
	m := newTestManager(t)
	storage := models.Storage{
		ID:     "storage1",
		Name:   "tank",
		Status: "offline",
	}

	m.mu.Lock()
	m.config.Overrides = map[string]ThresholdConfig{
		storage.ID: {
			DisableConnectivity: true,
		},
	}
	m.activeAlerts["storage-offline-"+storage.ID] = &Alert{ID: "storage-offline-" + storage.ID}
	m.offlineConfirmations[storage.ID] = 1
	m.mu.Unlock()

	m.checkStorageOffline(storage)

	m.mu.RLock()
	_, alertExists := m.activeAlerts["storage-offline-"+storage.ID]
	_, confirmExists := m.offlineConfirmations[storage.ID]
	m.mu.RUnlock()

	if alertExists {
		t.Fatalf("expected storage offline alert to clear when shared thresholds disable connectivity")
	}
	if confirmExists {
		t.Fatalf("expected storage offline confirmations to clear when shared thresholds disable connectivity")
	}
}

func TestCheckSnapshotsUsesGuestContextForCustomRules(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	cfg := AlertConfig{
		Enabled: true,
		SnapshotDefaults: SnapshotAlertConfig{
			Enabled:      true,
			WarningDays:  7,
			CriticalDays: 14,
		},
		CustomRules: []CustomAlertRule{
			{
				Name:     "db-snapshots",
				Enabled:  true,
				Priority: 10,
				FilterConditions: FilterStack{
					LogicalOperator: "AND",
					Filters: []FilterCondition{
						{Type: "text", Field: "name", Value: "db"},
					},
				},
				Thresholds: ThresholdConfig{
					Snapshot: &SnapshotAlertConfig{
						Enabled:      true,
						WarningDays:  15,
						CriticalDays: 20,
					},
				},
			},
		},
	}
	m.UpdateConfig(cfg)
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	now := time.Now()
	snapshots := []models.GuestSnapshot{
		{
			ID:       "inst-node-100-weekly",
			Name:     "weekly",
			Node:     "node",
			Instance: "inst",
			Type:     "qemu",
			VMID:     100,
			Time:     now.Add(-10 * 24 * time.Hour),
		},
		{
			ID:       "inst-node-101-weekly",
			Name:     "weekly",
			Node:     "node",
			Instance: "inst",
			Type:     "qemu",
			VMID:     101,
			Time:     now.Add(-10 * 24 * time.Hour),
		},
	}
	guestLookups := map[string]GuestLookup{
		BuildGuestKey("inst", "node", 100): {Name: "db-server"},
		BuildGuestKey("inst", "node", 101): {Name: "web-server"},
	}

	m.CheckSnapshotsForInstance("inst", snapshots, guestLookups)

	m.mu.RLock()
	_, dbExists := testLookupActiveAlert(t, m, "snapshot-age-inst-node-100-weekly")
	_, webExists := testLookupActiveAlert(t, m, "snapshot-age-inst-node-101-weekly")
	m.mu.RUnlock()

	if dbExists {
		t.Fatalf("expected db snapshot alert to be suppressed by custom rule thresholds")
	}
	if !webExists {
		t.Fatalf("expected non-matching snapshot alert to use default thresholds")
	}
}

func TestCheckBackupsUsesGuestContextForCustomRules(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	cfg := AlertConfig{
		Enabled: true,
		BackupDefaults: BackupAlertConfig{
			Enabled:      true,
			WarningDays:  7,
			CriticalDays: 14,
		},
		CustomRules: []CustomAlertRule{
			{
				Name:     "db-backups",
				Enabled:  true,
				Priority: 10,
				FilterConditions: FilterStack{
					LogicalOperator: "AND",
					Filters: []FilterCondition{
						{Type: "text", Field: "name", Value: "db"},
					},
				},
				Thresholds: ThresholdConfig{
					Backup: &BackupAlertConfig{
						Enabled:      true,
						WarningDays:  15,
						CriticalDays: 20,
					},
				},
			},
		},
	}
	m.UpdateConfig(cfg)

	now := time.Now()
	rollups := []recovery.ProtectionRollup{
		{
			RollupID: "db-rollup",
			SubjectRef: &recovery.ExternalRef{
				Type:      "proxmox-vm",
				Namespace: "inst",
				Name:      "db-server",
				ID:        BuildGuestKey("inst", "node", 100),
				Class:     "node",
			},
			LastSuccessAt: ptrTime(now.Add(-10 * 24 * time.Hour)),
			LastOutcome:   recovery.OutcomeSuccess,
			Providers:     []recovery.Provider{recovery.ProviderProxmoxPVE},
		},
		{
			RollupID: "web-rollup",
			SubjectRef: &recovery.ExternalRef{
				Type:      "proxmox-vm",
				Namespace: "inst",
				Name:      "web-server",
				ID:        BuildGuestKey("inst", "node", 101),
				Class:     "node",
			},
			LastSuccessAt: ptrTime(now.Add(-10 * 24 * time.Hour)),
			LastOutcome:   recovery.OutcomeSuccess,
			Providers:     []recovery.Provider{recovery.ProviderProxmoxPVE},
		},
	}

	guest100 := GuestLookup{
		ResourceID: BuildGuestKey("inst", "node", 100),
		Name:       "db-server",
		Instance:   "inst",
		Node:       "node",
		Type:       "qemu",
		VMID:       100,
	}
	guest101 := GuestLookup{
		ResourceID: BuildGuestKey("inst", "node", 101),
		Name:       "web-server",
		Instance:   "inst",
		Node:       "node",
		Type:       "qemu",
		VMID:       101,
	}
	guestsByKey := map[string]GuestLookup{
		guest100.ResourceID: guest100,
		guest101.ResourceID: guest101,
	}
	guestsByVMID := map[string][]GuestLookup{
		"100": {guest100},
		"101": {guest101},
	}

	m.CheckBackups(rollups, guestsByKey, guestsByVMID)

	m.mu.RLock()
	_, dbExists := testLookupActiveAlert(t, m, "backup-age-"+sanitizeAlertKey(guest100.ResourceID))
	_, webExists := testLookupActiveAlert(t, m, "backup-age-"+sanitizeAlertKey(guest101.ResourceID))
	m.mu.RUnlock()

	if dbExists {
		t.Fatalf("expected db backup alert to be suppressed by custom rule thresholds")
	}
	if !webExists {
		t.Fatalf("expected non-matching backup alert to use default thresholds")
	}
}

func TestReevaluateActiveAlertsUsesGuestContextForMetricCustomRules(t *testing.T) {
	m := newTestManager(t)
	resourceID := BuildGuestKey("pve1", "node1", 100)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.GuestDefaults = ThresholdConfig{
		CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
	}
	m.config.CustomRules = []CustomAlertRule{
		{
			Name:     "db-metrics",
			Enabled:  true,
			Priority: 10,
			FilterConditions: FilterStack{
				LogicalOperator: "AND",
				Filters: []FilterCondition{
					{Type: "text", Field: "name", Value: "db"},
				},
			},
			Thresholds: ThresholdConfig{
				CPU: &HysteresisThreshold{Trigger: 95, Clear: 90},
			},
		},
	}
	state, alert := testNewCanonicalAlert(resourceID, canonicalMetricSpecID(resourceID, "cpu"), string(alertspecs.AlertSpecKindMetricThreshold), "cpu")
	alert.Value = 90
	alert.Threshold = 80
	alert.ResourceName = "db-server"
	alert.Node = "node1"
	alert.Instance = "pve1"
	alert.Metadata = map[string]interface{}{
		"resourceType": "vm",
	}
	m.setActiveAlertNoLock(state, alert)
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts[state]
	m.mu.RUnlock()

	if exists {
		t.Fatalf("expected guest metric alert to resolve when custom rule raises the trigger")
	}
}

func TestReevaluateActiveAlertsUsesGuestContextForBackupCustomRules(t *testing.T) {
	m := newTestManager(t)
	resourceID := BuildGuestKey("pve1", "node1", 100)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.BackupDefaults = BackupAlertConfig{
		Enabled:      true,
		WarningDays:  7,
		CriticalDays: 14,
	}
	m.config.CustomRules = []CustomAlertRule{
		{
			Name:     "db-backup-reeval",
			Enabled:  true,
			Priority: 10,
			FilterConditions: FilterStack{
				LogicalOperator: "AND",
				Filters: []FilterCondition{
					{Type: "text", Field: "name", Value: "db"},
				},
			},
			Thresholds: ThresholdConfig{
				Backup: &BackupAlertConfig{
					Enabled:      true,
					WarningDays:  15,
					CriticalDays: 20,
				},
			},
		},
	}
	state, alert := testNewCanonicalAlert(resourceID, resourceID+"-backup-age", string(alertspecs.AlertSpecKindPostureThreshold), "backup-age")
	alert.Value = 10
	alert.Threshold = 7
	alert.ResourceName = "db-server backup"
	alert.Node = "node1"
	alert.Instance = "pve1"
	alert.Metadata = map[string]interface{}{
		"ageDays":       10.0,
		"guestName":     "db-server",
		"guestType":     "qemu",
		"guestInstance": "pve1",
		"guestNode":     "node1",
		"guestVmid":     100,
		"orphaned":      false,
	}
	m.setActiveAlertNoLock(state, alert)
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts[state]
	m.mu.RUnlock()

	if exists {
		t.Fatalf("expected guest backup alert to resolve when custom rule raises backup thresholds")
	}
}

func TestReevaluateActiveAlertsUsesGuestContextForPoweredOffCustomRules(t *testing.T) {
	m := newTestManager(t)
	resourceID := BuildGuestKey("pve1", "node1", 100)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.CustomRules = []CustomAlertRule{
		{
			Name:     "db-no-powered-off",
			Enabled:  true,
			Priority: 10,
			FilterConditions: FilterStack{
				LogicalOperator: "AND",
				Filters: []FilterCondition{
					{Type: "text", Field: "name", Value: "db"},
				},
			},
			Thresholds: ThresholdConfig{
				DisableConnectivity: true,
			},
		},
	}
	state, alert := testNewCanonicalAlert(resourceID, canonicalPoweredStateSpecID(resourceID), string(alertspecs.AlertSpecKindPoweredState), "powered-off")
	alert.ResourceName = "db-server"
	alert.Node = "node1"
	alert.Instance = "pve1"
	alert.Metadata = map[string]interface{}{
		"resourceType": "vm",
	}
	m.setActiveAlertNoLock(state, alert)
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts[state]
	m.mu.RUnlock()

	if exists {
		t.Fatalf("expected guest powered-off alert to resolve when custom rule disables connectivity")
	}
}

func TestReevaluateActiveAlertsUsesPBSResourceTypeMetadata(t *testing.T) {
	m := newTestManager(t)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.PBSDefaults = ThresholdConfig{
		CPU:    &HysteresisThreshold{Trigger: 99, Clear: 94},
		Memory: &HysteresisThreshold{Trigger: 0, Clear: 0},
	}
	state, alert := testNewCanonicalAlert("pbs-1", canonicalMetricSpecID("pbs-1", "memory"), string(alertspecs.AlertSpecKindMetricThreshold), "memory")
	alert.Value = 90.8
	alert.Threshold = 85
	alert.ResourceName = "pbs"
	alert.Node = "pbs.local"
	alert.Instance = "pbs"
	alert.Metadata = map[string]interface{}{
		"resourceType": "PBS",
	}
	m.setActiveAlertNoLock(state, alert)
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts[state]
	m.mu.RUnlock()

	if exists {
		t.Fatalf("expected PBS metric alert to resolve when PBS memory threshold is disabled")
	}
}

func TestReevaluateActiveAlertsUsesKubernetesResourceTypeMetadata(t *testing.T) {
	m := newTestManager(t)

	resourceID := "k8s:prod/ns:default/pod:api-7d9f"
	m.mu.Lock()
	m.config.Enabled = true
	m.config.KubernetesDefaults = ThresholdConfig{
		CPU: &HysteresisThreshold{Trigger: 0, Clear: 0},
	}
	state, alert := testNewCanonicalAlert(resourceID, canonicalMetricSpecID(resourceID, "cpu"), string(alertspecs.AlertSpecKindMetricThreshold), "cpu")
	alert.Value = 88
	alert.Threshold = 80
	alert.ResourceName = "api-7d9f"
	alert.Node = "worker-1"
	alert.Instance = "prod"
	alert.Metadata = map[string]interface{}{
		"resourceType": "Kubernetes Pod",
	}
	m.setActiveAlertNoLock(state, alert)
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts[state]
	m.mu.RUnlock()

	if exists {
		t.Fatalf("expected Kubernetes metric alert to resolve when Kubernetes CPU threshold is disabled")
	}
}

func TestReevaluateActiveAlertsUsesTrueNASResourceTypeMetadata(t *testing.T) {
	m := newTestManager(t)

	resourceID := "storage:truenas-main/pool:tank"
	m.mu.Lock()
	m.config.Enabled = true
	m.config.StorageDefault = HysteresisThreshold{Trigger: 95, Clear: 90}
	m.config.TrueNASDefaults = ThresholdConfig{
		Usage: &HysteresisThreshold{Trigger: 0, Clear: 0},
	}
	state, alert := testNewCanonicalAlert(resourceID, canonicalMetricSpecID(resourceID, "usage"), string(alertspecs.AlertSpecKindMetricThreshold), "usage")
	alert.Value = 90
	alert.Threshold = 85
	alert.ResourceName = "tank"
	alert.Node = "truenas-main"
	alert.Instance = "TrueNAS"
	alert.Metadata = map[string]interface{}{
		"resourceType": "TrueNAS Pool",
	}
	m.setActiveAlertNoLock(state, alert)
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts[state]
	m.mu.RUnlock()

	if exists {
		t.Fatalf("expected TrueNAS metric alert to resolve when TrueNAS usage threshold is disabled")
	}
}

func TestReevaluateActiveAlertsUsesVMwareResourceTypeMetadata(t *testing.T) {
	m := newTestManager(t)

	resourceID := "vmware:vc-1:datastore:datastore-301"
	m.mu.Lock()
	m.config.Enabled = true
	m.config.StorageDefault = HysteresisThreshold{Trigger: 95, Clear: 90}
	m.config.VMwareDefaults = ThresholdConfig{
		Usage: &HysteresisThreshold{Trigger: 0, Clear: 0},
	}
	state, alert := testNewCanonicalAlert(resourceID, canonicalMetricSpecID(resourceID, "usage"), string(alertspecs.AlertSpecKindMetricThreshold), "usage")
	alert.Value = 90
	alert.Threshold = 85
	alert.ResourceName = "nvme-primary"
	alert.Node = "Lab Datacenter"
	alert.Instance = "Lab vCenter"
	alert.Metadata = map[string]interface{}{
		"resourceType": "vSphere Datastore",
	}
	m.setActiveAlertNoLock(state, alert)
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts[state]
	m.mu.RUnlock()

	if exists {
		t.Fatalf("expected vSphere metric alert to resolve when vSphere usage threshold is disabled")
	}
}
