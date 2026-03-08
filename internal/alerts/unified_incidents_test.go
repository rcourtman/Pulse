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
	if got := alert.Metadata["topConsumerNames"]; got == nil {
		t.Fatal("expected topConsumerNames metadata")
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
