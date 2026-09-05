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
			{ID: "partial", Name: "partial", State: "CRASHED", Containers: []AppContainer{{ID: "worker", ServiceName: "worker", State: "crashed"}}},
			// One-shot init containers exit cleanly and stay EXITED for the life
			// of a healthy app, so they must project no incident at all.
			{ID: "oneshot", Name: "oneshot", State: "RUNNING", Containers: []AppContainer{
				{ID: "oneshot-permissions", ServiceName: "permissions", State: "exited"},
				{ID: "oneshot-web", ServiceName: "web", State: "running"},
			}},
		},
	})

	pool := requirePoolRecord(t, records, "tank").Resource
	if pool.Storage == nil || pool.Storage.ZFSPool == nil || pool.Storage.PoolHealth == nil {
		t.Fatalf("full pool health contract missing: %+v", pool.Storage)
	}
	// Topology is the cross-provider discriminator every pool consumer keys on
	// and must stay "pool" even for a pool with a rich vdev layout; the layout
	// itself rides in VDevLayout.
	if pool.Storage.Topology != "pool" {
		t.Fatalf("topology = %q, want \"pool\"", pool.Storage.Topology)
	}
	if pool.Storage.VDevLayout != "mirror" {
		t.Fatalf("vdev layout = %q, want \"mirror\"", pool.Storage.VDevLayout)
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

	oneshot := requireRecordByNameAndType(t, records, "oneshot", unifiedresources.ResourceTypeAppContainer)
	if len(oneshot.Resource.Incidents) != 0 {
		t.Fatalf("healthy app with a completed one-shot container must raise nothing, got %+v", oneshot.Resource.Incidents)
	}
	if oneshot.Resource.Status != unifiedresources.StatusOnline {
		t.Fatalf("oneshot status = %q, want online", oneshot.Resource.Status)
	}
	if oneshot.Resource.Docker == nil || oneshot.Resource.Docker.ContainerState != "running" {
		t.Fatalf("oneshot container state must not follow an arbitrary init container: %+v", oneshot.Resource.Docker)
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

// TestPoolTopologyStaysDiscriminatorAcrossVDevLayouts pins the boundary that
// broke in 599c8e634: the frontend identifies a TrueNAS pool solely by
// storage.topology == "pool" (TrueNAS pools are transported as
// ResourceTypeStorage, so resource.type is never "pool"), and the producer
// briefly published the vdev layout there instead. Every fixture in this
// package omits VDevs, so the projection kept returning "pool" and both sides
// stayed green while disagreeing on real hardware. Drive real layouts here.
func TestPoolTopologyStaysDiscriminatorAcrossVDevLayouts(t *testing.T) {
	observedAt := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name       string
		vdevs      []PoolVDev
		members    []PoolDiskMember
		wantLayout string
	}{
		{name: "no vdevs reported", wantLayout: ""},
		{
			name:       "mirror",
			vdevs:      []PoolVDev{{ID: "m0", Name: "mirror-0", Type: "MIRROR", Role: "data", Status: "ONLINE"}},
			wantLayout: "mirror",
		},
		{
			name:       "raidz2",
			vdevs:      []PoolVDev{{ID: "z0", Name: "raidz2-0", Type: "RAIDZ2", Role: "data", Status: "ONLINE"}},
			wantLayout: "raidz2",
		},
		{
			name: "striped single disks",
			vdevs: []PoolVDev{
				{ID: "d0", Name: "sda", Type: "DISK", Role: "data", Status: "ONLINE"},
				{ID: "d1", Name: "sdb", Type: "DISK", Role: "data", Status: "ONLINE"},
			},
			members: []PoolDiskMember{
				{Disk: "sda", Status: "ONLINE", Role: "data"},
				{Disk: "sdb", Status: "ONLINE", Role: "data"},
			},
			wantLayout: "stripe",
		},
		{
			name: "mixed data vdev types",
			vdevs: []PoolVDev{
				{ID: "m0", Name: "mirror-0", Type: "MIRROR", Role: "data", Status: "ONLINE"},
				{ID: "s0", Name: "special-0", Type: "SPECIAL", Role: "data", Status: "ONLINE"},
			},
			wantLayout: "mirror+special",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			records := FixtureRecords(FixtureSnapshot{
				CollectedAt: observedAt,
				System:      SystemInfo{Hostname: "nas-a", Healthy: true},
				Pools: []Pool{{
					ID:          "1",
					GUID:        "pool-guid",
					Name:        "tank",
					Status:      "ONLINE",
					VDevs:       tc.vdevs,
					DiskMembers: tc.members,
				}},
			})

			pool := requirePoolRecord(t, records, "tank").Resource
			if pool.Storage == nil {
				t.Fatalf("pool record carries no storage meta: %+v", pool)
			}
			if pool.Storage.Topology != "pool" {
				t.Fatalf("topology = %q, want %q: a non-%q value drops the pool out of the TrueNAS page entirely",
					pool.Storage.Topology, "pool", "pool")
			}
			if pool.Storage.VDevLayout != tc.wantLayout {
				t.Fatalf("vdev layout = %q, want %q", pool.Storage.VDevLayout, tc.wantLayout)
			}
		})
	}
}

func TestIncidentProjectionPreservesNativeSeverity(t *testing.T) {
	for _, level := range []string{"INFO", "NOTICE"} {
		t.Run(level, func(t *testing.T) {
			incident, ok := incidentFromAlert(Alert{ID: "condition-1", Level: " " + strings.ToLower(level) + " ", Message: "Provider condition"})
			if !ok {
				t.Fatal("native condition omitted")
			}
			if incident.NativeSeverity != level {
				t.Fatalf("native severity = %q, want %q", incident.NativeSeverity, level)
			}
			if incident.Severity != storagehealth.RiskMonitor {
				t.Fatalf("canonical severity inflated: %v", incident.Severity)
			}
			if incident.NativeID != "condition-1" {
				t.Fatalf("native identity changed: %q", incident.NativeID)
			}
		})
	}
}
