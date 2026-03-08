package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestSyncUnifiedResourceIncidentsCreatesAndClearsAlerts(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	resource := unifiedresources.Resource{
		ID:         "storage:tank",
		Type:       unifiedresources.ResourceTypeStorage,
		Name:       "tank",
		ParentName: "truenas-main",
		Sources:    []unifiedresources.DataSource{unifiedresources.SourceTrueNAS},
		Storage: &unifiedresources.StorageMeta{
			Platform:   "truenas",
			Topology:   "pool",
			Protection: "zfs",
			IsZFS:      true,
		},
		Incidents: []unifiedresources.ResourceIncident{{
			Provider: "truenas",
			NativeID: "alert-1",
			Code:     "truenas_volume_status",
			Severity: storagehealth.RiskWarning,
			Summary:  "Pool tank is DEGRADED",
		}},
	}

	m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})

	alertID := "unified-incident-storage-tank-truenas-alert-1-truenas-volume-status"
	assertAlertPresent(t, m, alertID)

	m.mu.RLock()
	alert := m.activeAlerts[alertID]
	m.mu.RUnlock()

	if alert.Type != "zfs-pool-state" {
		t.Fatalf("alert type = %q, want zfs-pool-state", alert.Type)
	}
	if alert.ResourceID != resource.ID {
		t.Fatalf("resource id = %q, want %q", alert.ResourceID, resource.ID)
	}
	if alert.Node != "truenas-main" {
		t.Fatalf("node = %q, want truenas-main", alert.Node)
	}

	m.SyncUnifiedResourceIncidents(nil)
	assertAlertMissing(t, m, alertID)
}

func TestSyncUnifiedResourceIncidentsIncludesConsumerImpact(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	resource := unifiedresources.Resource{
		ID:         "storage:local-lvm",
		Type:       unifiedresources.ResourceTypeStorage,
		Name:       "local-lvm",
		ParentName: "pve1",
		Storage: &unifiedresources.StorageMeta{
			Type:          "lvmthin",
			ConsumerCount: 3,
			ConsumerTypes: []string{"vm", "system-container"},
			TopConsumers: []unifiedresources.StorageConsumerMeta{
				{Name: "app01", ResourceType: unifiedresources.ResourceTypeVM, ResourceID: "vm-101", DiskCount: 1},
				{Name: "media01", ResourceType: unifiedresources.ResourceTypeSystemContainer, ResourceID: "lxc-200", DiskCount: 1},
			},
		},
		Incidents: []unifiedresources.ResourceIncident{{
			Provider: "pulse",
			NativeID: "storage-risk-1",
			Code:     "capacity_runway_low",
			Severity: storagehealth.RiskWarning,
			Summary:  "Storage local-lvm is running low on free space",
		}},
	}

	m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})

	alertID := "unified-incident-storage-local-lvm-pulse-storage-risk-1-capacity-runway-low"
	assertAlertPresent(t, m, alertID)

	m.mu.RLock()
	alert := m.activeAlerts[alertID]
	m.mu.RUnlock()

	wantMessage := "Storage local-lvm is running low on free space. Affects 3 dependent resources: app01, media01, and 1 more"
	if alert.Message != wantMessage {
		t.Fatalf("message = %q, want %q", alert.Message, wantMessage)
	}
	if got := alert.Metadata["consumerCount"]; got != 3 {
		t.Fatalf("consumerCount = %v, want 3", got)
	}
	if got := alert.Metadata["consumerImpactSummary"]; got != "Affects 3 dependent resources: app01, media01, and 1 more" {
		t.Fatalf("consumerImpactSummary = %v", got)
	}
	if got := alert.Metadata["incidentCategory"]; got != unifiedresources.IncidentCategoryCapacity {
		t.Fatalf("incidentCategory = %v", got)
	}
	if got := alert.Metadata["incidentLabel"]; got != "Capacity Pressure" {
		t.Fatalf("incidentLabel = %v", got)
	}
	if got := alert.Metadata["incidentPriority"]; got != 3203 {
		t.Fatalf("incidentPriority = %v", got)
	}
	if got := alert.Metadata["incidentImpactSummary"]; got != "Affects 3 dependent resources: app01, media01, and 1 more" {
		t.Fatalf("incidentImpactSummary = %v", got)
	}
	if got := alert.Metadata["incidentUrgency"]; got != unifiedresources.IncidentUrgencyToday {
		t.Fatalf("incidentUrgency = %v", got)
	}
	if got := alert.Metadata["incidentAction"]; got != "Plan cleanup or capacity expansion soon" {
		t.Fatalf("incidentAction = %v", got)
	}
	if got := alert.Metadata["topConsumerNames"]; got == nil {
		t.Fatal("expected topConsumerNames metadata")
	}
}

func TestSyncUnifiedResourceIncidentsMarksBackupTargetExposure(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	resource := unifiedresources.Resource{
		ID:         "storage:backup-store",
		Type:       unifiedresources.ResourceTypeStorage,
		Name:       "backup-store",
		ParentName: "pbs-main",
		Storage: &unifiedresources.StorageMeta{
			Type:          "pbs-datastore",
			Platform:      "pbs",
			Topology:      "datastore",
			Protection:    "backup-repository",
			ContentTypes:  []string{"backup"},
			ConsumerCount: 2,
			ConsumerTypes: []string{"vm", "system-container"},
			TopConsumers: []unifiedresources.StorageConsumerMeta{
				{Name: "app01", ResourceType: unifiedresources.ResourceTypeVM, ResourceID: "vm-101", DiskCount: 1},
				{Name: "media01", ResourceType: unifiedresources.ResourceTypeSystemContainer, ResourceID: "lxc-200", DiskCount: 1},
			},
		},
		Incidents: []unifiedresources.ResourceIncident{{
			Provider: "pulse",
			NativeID: "backup-risk-1",
			Code:     "backup_target_degraded",
			Severity: storagehealth.RiskCritical,
			Summary:  "Backup datastore backup-store is degraded",
		}},
	}

	m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})

	alertID := "unified-incident-storage-backup-store-pulse-backup-risk-1-backup-target-degraded"
	assertAlertPresent(t, m, alertID)

	m.mu.RLock()
	alert := m.activeAlerts[alertID]
	m.mu.RUnlock()

	if alert.Type != "backup-storage-incident" {
		t.Fatalf("alert type = %q, want backup-storage-incident", alert.Type)
	}
	wantMessage := "Backup datastore backup-store is degraded. Puts backups for 2 protected workloads at risk: app01, media01"
	if alert.Message != wantMessage {
		t.Fatalf("message = %q, want %q", alert.Message, wantMessage)
	}
	if got := alert.Metadata["backupTarget"]; got != true {
		t.Fatalf("backupTarget = %v, want true", got)
	}
	if got := alert.Metadata["protectedWorkloadCount"]; got != 2 {
		t.Fatalf("protectedWorkloadCount = %v, want 2", got)
	}
	if got := alert.Metadata["protectedWorkloadSummary"]; got != "Puts backups for 2 protected workloads at risk: app01, media01" {
		t.Fatalf("protectedWorkloadSummary = %v", got)
	}
	if got := alert.Metadata["incidentCategory"]; got != unifiedresources.IncidentCategoryRecoverability {
		t.Fatalf("incidentCategory = %v", got)
	}
	if got := alert.Metadata["incidentLabel"]; got != "Backup Coverage At Risk" {
		t.Fatalf("incidentLabel = %v", got)
	}
	if got := alert.Metadata["incidentPriority"]; got != 4502 {
		t.Fatalf("incidentPriority = %v", got)
	}
	if got := alert.Metadata["incidentImpactSummary"]; got != "Puts backups for 2 protected workloads at risk: app01, media01" {
		t.Fatalf("incidentImpactSummary = %v", got)
	}
	if got := alert.Metadata["incidentUrgency"]; got != unifiedresources.IncidentUrgencyNow {
		t.Fatalf("incidentUrgency = %v", got)
	}
	if got := alert.Metadata["incidentAction"]; got != "Restore backup target health immediately to protect recoverability" {
		t.Fatalf("incidentAction = %v", got)
	}
}

func TestSyncUnifiedResourceIncidentsMarksPBSBackupPosture(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	resource := unifiedresources.Resource{
		ID:   "pbs:main",
		Type: unifiedresources.ResourceTypePBS,
		Name: "pbs-main",
		PBS: &unifiedresources.PBSData{
			Hostname:       "pbs-main.local",
			DatastoreCount: 2,
			Datastores: []unifiedresources.PBSDatastoreMeta{
				{Name: "fast", Status: "online", Total: 100, Used: 96},
				{Name: "archive", Status: "online", Total: 100, Used: 40},
			},
			ProtectedWorkloadCount: 2,
			ProtectedWorkloadTypes: []string{"system-container", "vm"},
			ProtectedWorkloadNames: []string{"media01", "app01"},
			StorageRisk: &unifiedresources.StorageRisk{
				Level: storagehealth.RiskCritical,
				Reasons: []unifiedresources.StorageRiskReason{
					{Code: "capacity_runway_low", Severity: storagehealth.RiskCritical, Summary: "PBS datastore fast is 96% full"},
				},
			},
		},
		Incidents: []unifiedresources.ResourceIncident{{
			Provider: "pulse",
			NativeID: "pbs-instance:pbs-main:capacity_runway_low",
			Code:     "capacity_runway_low",
			Severity: storagehealth.RiskCritical,
			Summary:  "PBS datastore fast is 96% full",
		}},
	}

	m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})

	alertID := "unified-incident-pbs-main-pulse-pbs-instance-pbs-main-capacity-runway-low-capacity-runway-low"
	assertAlertPresent(t, m, alertID)

	m.mu.RLock()
	alert := m.activeAlerts[alertID]
	m.mu.RUnlock()

	if alert.Type != "backup-posture-incident" {
		t.Fatalf("alert type = %q, want backup-posture-incident", alert.Type)
	}
	wantMessage := "Backup server pbs-main has datastore capacity risk. Affects 1 backup datastore: fast"
	if alert.Message != wantMessage {
		t.Fatalf("message = %q, want %q", alert.Message, wantMessage)
	}
	if got := alert.Metadata["backupServer"]; got != true {
		t.Fatalf("backupServer = %v, want true", got)
	}
	if got := alert.Metadata["affectedDatastoreCount"]; got != 1 {
		t.Fatalf("affectedDatastoreCount = %v, want 1", got)
	}
	if got := alert.Metadata["protectedWorkloadCount"]; got != 2 {
		t.Fatalf("protectedWorkloadCount = %v, want 2", got)
	}
	if got := alert.Metadata["consumerCount"]; got != 2 {
		t.Fatalf("consumerCount = %v, want 2", got)
	}
	if got := alert.Metadata["protectedWorkloadSummary"]; got != "Puts backups for 2 protected workloads at risk: media01, app01" {
		t.Fatalf("protectedWorkloadSummary = %v", got)
	}
	if got := alert.Metadata["incidentCategory"]; got != unifiedresources.IncidentCategoryRecoverability {
		t.Fatalf("incidentCategory = %v", got)
	}
	if got := alert.Metadata["incidentLabel"]; got != "Backup Coverage At Risk" {
		t.Fatalf("incidentLabel = %v", got)
	}
	if got := alert.Metadata["incidentPriority"]; got != 4502 {
		t.Fatalf("incidentPriority = %v", got)
	}
	if got := alert.Metadata["incidentImpactSummary"]; got != "Puts backups for 2 protected workloads at risk: media01, app01" {
		t.Fatalf("incidentImpactSummary = %v", got)
	}
	if got := alert.Metadata["incidentUrgency"]; got != unifiedresources.IncidentUrgencyNow {
		t.Fatalf("incidentUrgency = %v", got)
	}
	if got := alert.Metadata["incidentAction"]; got != "Restore backup target health immediately to protect recoverability" {
		t.Fatalf("incidentAction = %v", got)
	}
}

func TestSyncUnifiedResourceIncidentsSuppressesRedundantParentAndChildAlerts(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	agentID := "agent:truenas-main"
	storageID := "storage:tank"
	resources := []unifiedresources.Resource{
		{
			ID:      agentID,
			Type:    unifiedresources.ResourceTypeAgent,
			Name:    "truenas-main",
			Sources: []unifiedresources.DataSource{unifiedresources.SourceTrueNAS},
			TrueNAS: &unifiedresources.TrueNASData{Hostname: "truenas-main"},
			Incidents: []unifiedresources.ResourceIncident{{
				Provider: "truenas",
				NativeID: "alert-1",
				Code:     "truenas_volume_status",
				Severity: storagehealth.RiskCritical,
				Summary:  "Pool tank is FAULTED",
			}},
		},
		{
			ID:       storageID,
			Type:     unifiedresources.ResourceTypeStorage,
			Name:     "tank",
			ParentID: &agentID,
			Storage: &unifiedresources.StorageMeta{
				Platform: "truenas",
				Topology: "pool",
				IsZFS:    true,
			},
			Incidents: []unifiedresources.ResourceIncident{{
				Provider: "truenas",
				NativeID: "alert-1",
				Code:     "truenas_volume_status",
				Severity: storagehealth.RiskCritical,
				Summary:  "Pool tank is FAULTED",
			}},
		},
		{
			ID:       "disk:sda",
			Type:     unifiedresources.ResourceTypePhysicalDisk,
			Name:     "sda",
			ParentID: &storageID,
			PhysicalDisk: &unifiedresources.PhysicalDiskMeta{
				DevPath: "/dev/sda",
			},
			Incidents: []unifiedresources.ResourceIncident{{
				Provider: "truenas",
				NativeID: "alert-1",
				Code:     "truenas_volume_status",
				Severity: storagehealth.RiskCritical,
				Summary:  "Pool tank is FAULTED",
			}},
		},
	}

	m.SyncUnifiedResourceIncidents(resources)

	active := m.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("expected only 1 active alert after parent/child suppression, got %d: %+v", len(active), active)
	}
	if active[0].Type != "zfs-pool-state" {
		t.Fatalf("alert type = %q, want zfs-pool-state", active[0].Type)
	}
	if active[0].ResourceID != storageID {
		t.Fatalf("resource id = %q, want %q", active[0].ResourceID, storageID)
	}
}

func TestSyncUnifiedResourceIncidentsSuppressesPBSDatastoreChildWhenParentRollsUp(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	parentID := "pbs:main"
	childID := "storage:fast"
	resources := []unifiedresources.Resource{
		{
			ID:   parentID,
			Type: unifiedresources.ResourceTypePBS,
			Name: "pbs-main",
			PBS: &unifiedresources.PBSData{
				DatastoreCount: 1,
				Datastores: []unifiedresources.PBSDatastoreMeta{
					{Name: "fast", Status: "online", Total: 100, Used: 96},
				},
				ProtectedWorkloadCount: 2,
				ProtectedWorkloadNames: []string{"media01", "app01"},
			},
			Incidents: []unifiedresources.ResourceIncident{{
				Provider: "pulse",
				NativeID: "pbs-instance:pbs-main:capacity_runway_low",
				Code:     "capacity_runway_low",
				Severity: storagehealth.RiskCritical,
				Summary:  "PBS datastore fast is 96% full",
			}},
		},
		{
			ID:       childID,
			Type:     unifiedresources.ResourceTypeStorage,
			Name:     "fast",
			ParentID: &parentID,
			Storage: &unifiedresources.StorageMeta{
				Type:         "pbs-datastore",
				Platform:     "pbs",
				Topology:     "datastore",
				Protection:   "backup-repository",
				ContentTypes: []string{"backup"},
			},
			Incidents: []unifiedresources.ResourceIncident{{
				Provider: "pulse",
				NativeID: "pbs-instance:pbs-main:capacity_runway_low",
				Code:     "capacity_runway_low",
				Severity: storagehealth.RiskCritical,
				Summary:  "PBS datastore fast is 96% full",
			}},
		},
	}

	m.SyncUnifiedResourceIncidents(resources)

	active := m.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("expected 1 active alert after PBS parent roll-up suppression, got %d: %+v", len(active), active)
	}
	if active[0].ResourceID != parentID {
		t.Fatalf("expected PBS parent alert to remain, got %q", active[0].ResourceID)
	}
	if active[0].Type != "backup-posture-incident" {
		t.Fatalf("alert type = %q, want backup-posture-incident", active[0].Type)
	}
}

func TestSyncUnifiedResourceIncidentsIncludesProtectionAndRebuildSemantics(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	resource := unifiedresources.Resource{
		ID:   "storage:array",
		Type: unifiedresources.ResourceTypeStorage,
		Name: "array",
		Storage: &unifiedresources.StorageMeta{
			Platform:   "unraid",
			Topology:   "array",
			Protection: "single-parity",
			Risk: &unifiedresources.StorageRisk{
				Level: storagehealth.RiskCritical,
				Reasons: []unifiedresources.StorageRiskReason{
					{Code: "unraid_parity_unavailable", Severity: storagehealth.RiskCritical, Summary: "Unraid parity protection is unavailable"},
					{Code: "unraid_sync_active", Severity: storagehealth.RiskWarning, Summary: "Unraid array is running parity-sync (25%)"},
				},
			},
		},
		Incidents: []unifiedresources.ResourceIncident{{
			Provider: "pulse",
			NativeID: "topology-1",
			Code:     "storage_topology_risk",
			Severity: storagehealth.RiskCritical,
			Summary:  "Unraid array protection is unavailable",
		}},
	}

	m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})

	alertID := "unified-incident-storage-array-pulse-topology-1-storage-topology-risk"
	assertAlertPresent(t, m, alertID)

	m.mu.RLock()
	alert := m.activeAlerts[alertID]
	m.mu.RUnlock()

	if got := alert.Metadata["protectionReduced"]; got != true {
		t.Fatalf("protectionReduced = %v, want true", got)
	}
	if got := alert.Metadata["rebuildInProgress"]; got != true {
		t.Fatalf("rebuildInProgress = %v, want true", got)
	}
	if got := alert.Metadata["protectionSummary"]; got != "Unraid parity protection is unavailable" {
		t.Fatalf("protectionSummary = %v", got)
	}
	if got := alert.Metadata["rebuildSummary"]; got != "Unraid array is running parity-sync (25%)" {
		t.Fatalf("rebuildSummary = %v", got)
	}
}

func TestGetActiveAlertsPrioritizesHighImpactStorageIncidents(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{
		{
			ID:   "storage:shared",
			Type: unifiedresources.ResourceTypeStorage,
			Name: "shared",
			Storage: &unifiedresources.StorageMeta{
				ConsumerCount: 4,
			},
			Incidents: []unifiedresources.ResourceIncident{{
				Provider: "pulse",
				NativeID: "a",
				Code:     "capacity_runway_low",
				Severity: storagehealth.RiskCritical,
				Summary:  "Shared storage is nearly full",
			}},
		},
		{
			ID:   "disk:sdb",
			Type: unifiedresources.ResourceTypePhysicalDisk,
			Name: "sdb",
			PhysicalDisk: &unifiedresources.PhysicalDiskMeta{
				DevPath: "/dev/sdb",
			},
			Incidents: []unifiedresources.ResourceIncident{{
				Provider: "pulse",
				NativeID: "b",
				Code:     "smart_failed",
				Severity: storagehealth.RiskCritical,
				Summary:  "Disk sdb has SMART failures",
			}},
		},
	})

	active := m.GetActiveAlerts()
	if len(active) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(active))
	}
	if active[0].ResourceID != "storage:shared" {
		t.Fatalf("expected storage incident first, got %q then %q", active[0].ResourceID, active[1].ResourceID)
	}
}

func TestGetActiveAlertsPrioritizesProtectionLossAboveRebuildOnly(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{
		{
			ID:   "storage:protection-loss",
			Type: unifiedresources.ResourceTypeStorage,
			Name: "protection-loss",
			Storage: &unifiedresources.StorageMeta{
				Risk: &unifiedresources.StorageRisk{
					Level: storagehealth.RiskCritical,
					Reasons: []unifiedresources.StorageRiskReason{
						{Code: "unraid_parity_unavailable", Severity: storagehealth.RiskCritical, Summary: "Parity unavailable"},
					},
				},
			},
			Incidents: []unifiedresources.ResourceIncident{{
				Provider: "pulse",
				NativeID: "protection",
				Code:     "storage_topology_risk",
				Severity: storagehealth.RiskCritical,
				Summary:  "Protection lost",
			}},
		},
		{
			ID:   "storage:rebuild",
			Type: unifiedresources.ResourceTypeStorage,
			Name: "rebuild",
			Storage: &unifiedresources.StorageMeta{
				Risk: &unifiedresources.StorageRisk{
					Level: storagehealth.RiskWarning,
					Reasons: []unifiedresources.StorageRiskReason{
						{Code: "raid_rebuilding", Severity: storagehealth.RiskWarning, Summary: "RAID is rebuilding"},
					},
				},
			},
			Incidents: []unifiedresources.ResourceIncident{{
				Provider: "pulse",
				NativeID: "rebuild",
				Code:     "storage_topology_risk",
				Severity: storagehealth.RiskWarning,
				Summary:  "Rebuild in progress",
			}},
		},
	})

	active := m.GetActiveAlerts()
	if len(active) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(active))
	}
	if active[0].ResourceID != "storage:protection-loss" {
		t.Fatalf("expected protection-loss alert first, got %q then %q", active[0].ResourceID, active[1].ResourceID)
	}
}

func TestGetActiveAlertsPrioritizesBackupTargetExposure(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{
		{
			ID:   "storage:backup-store",
			Type: unifiedresources.ResourceTypeStorage,
			Name: "backup-store",
			Storage: &unifiedresources.StorageMeta{
				Type:          "pbs-datastore",
				Platform:      "pbs",
				Topology:      "datastore",
				Protection:    "backup-repository",
				ContentTypes:  []string{"backup"},
				ConsumerCount: 2,
				TopConsumers: []unifiedresources.StorageConsumerMeta{
					{Name: "app01", ResourceType: unifiedresources.ResourceTypeVM, ResourceID: "vm-101", DiskCount: 1},
				},
			},
			Incidents: []unifiedresources.ResourceIncident{{
				Provider: "pulse",
				NativeID: "backup",
				Code:     "backup_target_degraded",
				Severity: storagehealth.RiskCritical,
				Summary:  "Backup target is degraded",
			}},
		},
		{
			ID:   "storage:shared",
			Type: unifiedresources.ResourceTypeStorage,
			Name: "shared",
			Storage: &unifiedresources.StorageMeta{
				ConsumerCount: 4,
			},
			Incidents: []unifiedresources.ResourceIncident{{
				Provider: "pulse",
				NativeID: "shared",
				Code:     "capacity_runway_low",
				Severity: storagehealth.RiskCritical,
				Summary:  "Shared storage is nearly full",
			}},
		},
	})

	active := m.GetActiveAlerts()
	if len(active) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(active))
	}
	if active[0].ResourceID != "storage:backup-store" {
		t.Fatalf("expected backup target incident first, got %q then %q", active[0].ResourceID, active[1].ResourceID)
	}
}

func TestGetActiveAlertsPrioritizesBackupPostureExposure(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{
		{
			ID:   "pbs:main",
			Type: unifiedresources.ResourceTypePBS,
			Name: "pbs-main",
			PBS: &unifiedresources.PBSData{
				DatastoreCount: 1,
				Datastores: []unifiedresources.PBSDatastoreMeta{
					{Name: "fast", Status: "online", Total: 100, Used: 96},
				},
				ProtectedWorkloadCount: 2,
				ProtectedWorkloadNames: []string{"media01", "app01"},
			},
			Incidents: []unifiedresources.ResourceIncident{{
				Provider: "pulse",
				NativeID: "pbs-instance:pbs-main:capacity_runway_low",
				Code:     "capacity_runway_low",
				Severity: storagehealth.RiskCritical,
				Summary:  "PBS datastore fast is 96% full",
			}},
		},
		{
			ID:   "storage:shared",
			Type: unifiedresources.ResourceTypeStorage,
			Name: "shared",
			Storage: &unifiedresources.StorageMeta{
				ConsumerCount: 4,
			},
			Incidents: []unifiedresources.ResourceIncident{{
				Provider: "pulse",
				NativeID: "shared",
				Code:     "capacity_runway_low",
				Severity: storagehealth.RiskCritical,
				Summary:  "Shared storage is nearly full",
			}},
		},
	})

	active := m.GetActiveAlerts()
	if len(active) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(active))
	}
	if active[0].ResourceID != "pbs:main" {
		t.Fatalf("expected backup posture alert first, got %q then %q", active[0].ResourceID, active[1].ResourceID)
	}
}

func TestSyncUnifiedResourceIncidentsRespectsDisableAllStorage(t *testing.T) {
	m := newTestManager(t)
	cfg := unifiedEvalBaseConfig()
	cfg.DisableAllStorage = true
	configureUnifiedEvalManager(t, m, cfg)

	resource := unifiedresources.Resource{
		ID:   "disk:ada0",
		Type: unifiedresources.ResourceTypePhysicalDisk,
		Name: "ada0",
		PhysicalDisk: &unifiedresources.PhysicalDiskMeta{
			DevPath: "/dev/ada0",
		},
		Incidents: []unifiedresources.ResourceIncident{{
			Provider: "truenas",
			NativeID: "alert-2",
			Code:     "truenas_smart",
			Severity: storagehealth.RiskCritical,
			Summary:  "Device /dev/ada0 has SMART failures",
		}},
	}

	m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})

	alertID := "unified-incident-disk-ada0-truenas-alert-2-truenas-smart"
	assertAlertMissing(t, m, alertID)
}
