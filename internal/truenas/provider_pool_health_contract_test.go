package truenas

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestProviderProjectsFullZFSHealthAndActionableDatasetAppIncidents(t *testing.T) {
	observedAt := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	records := FixtureRecords(FixtureSnapshot{
		CollectedAt: observedAt,
		System:      SystemInfo{Hostname: "nas-a", Healthy: true},
		Pools: []Pool{{
			ID:             "7",
			GUID:           "pool-guid",
			Name:           "tank",
			Status:         "DEGRADED",
			StatusDetail:   "One or more devices is unavailable",
			ReadErrors:     1,
			ChecksumErrors: 2,
			Scan: &PoolScan{
				Function:   "RESILVER",
				State:      "SCANNING",
				Percentage: 33.3,
			},
			VDevs: []PoolVDev{
				{ID: "mirror-guid", GUID: "mirror-guid", Name: "mirror-0", Type: "MIRROR", Role: "data", Status: "DEGRADED"},
				{ID: "disk-a", ParentID: "mirror-guid", GUID: "disk-a", Name: "sda", Disk: "sda", Path: "/dev/sda2", Type: "DISK", Role: "data", Status: "ONLINE"},
				{ID: "disk-b", ParentID: "mirror-guid", GUID: "disk-b", Name: "sdb", Disk: "sdb", Path: "/dev/disk/by-partuuid/missing", Type: "UNAVAIL_DISK", Role: "data", Status: "UNAVAIL", Missing: true},
				{ID: "spare", GUID: "spare", Name: "sdc", Disk: "sdc", Path: "/dev/sdc", Type: "DISK", Role: "spare", Status: "AVAIL"},
			},
			DiskMembers: []PoolDiskMember{
				{Disk: "sda", Status: "ONLINE", Role: "data"},
				{Disk: "sdb", Status: "UNAVAIL", Role: "data", Missing: true},
				{Disk: "sdc", Status: "AVAIL", Role: "spare"},
			},
		}},
		Datasets: []Dataset{
			{ID: "tank/locked", Name: "tank/locked", Pool: "tank", Locked: true},
			{ID: "tank/receive", Name: "tank/receive", Pool: "tank", Mounted: true, ReadOnly: true, ReadOnlyReason: DatasetReadOnlyReplicationTarget},
		},
		Disks: []Disk{
			{ID: "disk-a", Name: "sda", Pool: "tank", Status: "ONLINE"},
			{ID: "disk-b", Name: "sdb", Pool: "tank", Status: "UNAVAIL"},
			{ID: "spare", Name: "sdc", Pool: "tank", Status: "AVAIL"},
		},
		Apps: []App{
			{ID: "crashed", Name: "crashed", State: "CRASHED"},
			{ID: "stopped", Name: "stopped", State: "STOPPED"},
			{ID: "partial", Name: "partial", State: "RUNNING", Containers: []AppContainer{{ID: "worker", ServiceName: "worker", State: "exited"}}},
		},
	})

	pool := requirePoolRecord(t, records, "tank").Resource
	if pool.Storage == nil || pool.Storage.ZFSPool == nil || pool.Storage.PoolHealth == nil {
		t.Fatalf("full pool health contract missing: %+v", pool.Storage)
	}
	if pool.Storage.Topology != "mirror" {
		t.Fatalf("topology = %q", pool.Storage.Topology)
	}
	if pool.Storage.ZFSReadErrors != 1 || pool.Storage.ZFSChecksumErrors != 2 {
		t.Fatalf("flattened errors = %+v", pool.Storage)
	}
	if pool.Storage.ZFSPool.ScanDetails == nil || pool.Storage.ZFSPool.ScanDetails.Function != "RESILVER" || len(pool.Storage.ZFSPool.Devices) != 4 {
		t.Fatalf("full ZFS report = %+v", pool.Storage.ZFSPool)
	}
	if pool.Storage.PoolHealth.CanonicalState != "DEGRADED" ||
		pool.Storage.PoolHealth.Severity != storagehealth.RiskCritical ||
		!strings.Contains(pool.Storage.PoolHealth.Recommendation, "affected vdev member") {
		t.Fatalf("canonical pool health = %+v", pool.Storage.PoolHealth)
	}
	if !poolHealthHasIncidentCode(pool.Incidents, "zfs_pool_state") ||
		!poolHealthHasIncidentCode(pool.Incidents, "zfs_resilver_active") ||
		!poolHealthHasIncidentCode(pool.Incidents, "zfs_device_missing") {
		t.Fatalf("pool incidents = %+v", pool.Incidents)
	}
	for _, incident := range pool.Incidents {
		if incident.Provider == "truenas" && incident.ConfirmationsRequired != 2 {
			t.Fatalf("synthetic incident confirmation contract = %+v", incident)
		}
	}

	locked := requireRecordByNameAndType(t, records, "tank/locked", unifiedresources.ResourceTypeStorage)
	if !poolHealthHasIncidentCode(locked.Resource.Incidents, "zfs_dataset_locked") {
		t.Fatalf("locked dataset incidents = %+v", locked.Resource.Incidents)
	}
	receive := requireRecordByNameAndType(t, records, "tank/receive", unifiedresources.ResourceTypeStorage)
	if receive.Resource.Status != unifiedresources.StatusOnline || len(receive.Resource.Incidents) != 0 {
		t.Fatalf("replication target readonly must remain healthy: %+v", receive.Resource)
	}

	for name, code := range map[string]string{
		"crashed": "truenas_app_crashed",
		"stopped": "truenas_app_stopped",
		"partial": "truenas_app_container_failed",
	} {
		record := requireRecordByNameAndType(t, records, name, unifiedresources.ResourceTypeAppContainer)
		if !poolHealthHasIncidentCode(record.Resource.Incidents, code) {
			t.Fatalf("%s incidents = %+v", name, record.Resource.Incidents)
		}
	}
}

func TestNativeTrueNASPoolAlertSuppressesEquivalentSyntheticSignalsOnly(t *testing.T) {
	records := FixtureRecords(FixtureSnapshot{
		CollectedAt: time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC),
		System:      SystemInfo{Hostname: "nas-a", Healthy: true},
		Pools: []Pool{{
			ID:         "7",
			Name:       "tank",
			Status:     "DEGRADED",
			ReadErrors: 3,
		}},
		Alerts: []Alert{{
			ID:       "native-volume",
			Level:    "CRITICAL",
			Source:   "VolumeStatus",
			Message:  "Pool tank is DEGRADED",
			Datetime: time.Date(2026, 7, 24, 9, 0, 0, 0, time.UTC),
		}},
	})

	pool := requirePoolRecord(t, records, "tank").Resource
	if poolHealthHasIncidentCode(pool.Incidents, "zfs_pool_state") {
		t.Fatalf("native volume alert must suppress equivalent state alert: %+v", pool.Incidents)
	}
	if !poolHealthHasIncidentCode(pool.Incidents, "truenas_volume_status") || !poolHealthHasIncidentCode(pool.Incidents, "zfs_pool_errors") {
		t.Fatalf("native alert and distinct error evidence must both survive: %+v", pool.Incidents)
	}
}

func poolHealthHasIncidentCode(incidents []unifiedresources.ResourceIncident, code string) bool {
	for _, incident := range incidents {
		if incident.Code == code {
			return true
		}
	}
	return false
}

func requireRecordByNameAndType(t *testing.T, records []unifiedresources.IngestRecord, name string, resourceType unifiedresources.ResourceType) unifiedresources.IngestRecord {
	t.Helper()
	for _, record := range records {
		if record.Resource.Name == name && record.Resource.Type == resourceType {
			return record
		}
	}
	t.Fatalf("missing %s %q in %+v", resourceType, name, records)
	return unifiedresources.IngestRecord{}
}
