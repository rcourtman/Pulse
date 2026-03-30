package vmware

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestProviderRecords_ProjectCanonicalVMwareResources(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() { SetFeatureEnabled(previous) })

	collectedAt := time.Date(2026, time.March, 30, 18, 15, 0, 0, time.UTC)
	provider := NewProvider(InventorySnapshot{
		ConnectionID:   "vc-1",
		ConnectionName: "Lab VC",
		VCenterHost:    "vc.lab.local",
		CollectedAt:    collectedAt,
		Hosts: []InventoryHost{{
			Host:            "host-101",
			Name:            "esxi-01.lab.local",
			ConnectionState: "CONNECTED",
			PowerState:      "POWERED_ON",
			HostUUID:        "uuid-host-1",
			OverallStatus:   "yellow",
			Metrics: &InventoryMetrics{
				CPUPercent:              float64Ptr(21.4),
				MemoryPercent:           float64Ptr(63.2),
				MemoryUsedBytes:         int64Ptr(27144105984),
				MemoryTotalBytes:        int64Ptr(42949672960),
				NetInBytesPerSecond:     float64Ptr(1024),
				NetOutBytesPerSecond:    float64Ptr(2048),
				DiskReadBytesPerSecond:  float64Ptr(4096),
				DiskWriteBytesPerSecond: float64Ptr(8192),
			},
			TriggeredAlarms: []InventoryAlarm{{
				Alarm:         "alarm-11",
				Name:          "Host connection degraded",
				OverallStatus: "yellow",
				TriggeredAt:   collectedAt.Add(-2 * time.Minute),
			}},
			RecentTasks: []InventoryTask{{
				Task:      "task-11",
				Name:      "Reconnect host",
				State:     "running",
				StartedAt: collectedAt.Add(-1 * time.Minute),
			}},
		}},
		VMs: []InventoryVM{{
			VM:            "vm-201",
			Name:          "app-01",
			PowerState:    "POWERED_ON",
			CPUCount:      4,
			MemorySizeMiB: 8192,
			OverallStatus: "green",
			Metrics: &InventoryMetrics{
				CPUPercent:              float64Ptr(38.1),
				MemoryPercent:           float64Ptr(57.5),
				MemoryUsedBytes:         int64Ptr(5033164800),
				MemoryTotalBytes:        int64Ptr(8589934592),
				NetInBytesPerSecond:     float64Ptr(512),
				NetOutBytesPerSecond:    float64Ptr(768),
				DiskReadBytesPerSecond:  float64Ptr(1536),
				DiskWriteBytesPerSecond: float64Ptr(2048),
			},
			TriggeredAlarms: []InventoryAlarm{{
				Alarm:         "alarm-21",
				Name:          "VM replication fault",
				OverallStatus: "red",
				TriggeredAt:   collectedAt.Add(-3 * time.Minute),
			}},
			RecentTasks: []InventoryTask{{
				Task:      "task-21",
				Name:      "Create snapshot",
				State:     "success",
				StartedAt: collectedAt.Add(-5 * time.Minute),
			}},
			SnapshotCount: 2,
		}},
		Datastores: []InventoryDatastore{{
			Datastore:     "datastore-11",
			Name:          "nvme-primary",
			Type:          "VMFS",
			FreeSpace:     40,
			Capacity:      100,
			OverallStatus: "yellow",
			RecentTasks: []InventoryTask{{
				Task:      "task-31",
				Name:      "Refresh datastore",
				State:     "queued",
				StartedAt: collectedAt.Add(-4 * time.Minute),
			}},
		}},
	})

	records := provider.Records()
	if len(records) != 3 {
		t.Fatalf("expected 3 VMware records, got %d", len(records))
	}

	hostRecord := records[0]
	if hostRecord.SourceID != "vc-1:host:host-101" {
		t.Fatalf("host source id = %q, want vc-1:host:host-101", hostRecord.SourceID)
	}
	if hostRecord.Resource.Type != unifiedresources.ResourceTypeAgent {
		t.Fatalf("host resource type = %q, want %q", hostRecord.Resource.Type, unifiedresources.ResourceTypeAgent)
	}
	if hostRecord.Resource.Status != unifiedresources.StatusWarning {
		t.Fatalf("host status = %q, want %q", hostRecord.Resource.Status, unifiedresources.StatusWarning)
	}
	if hostRecord.Resource.VMware == nil || hostRecord.Resource.VMware.ManagedObjectID != "host-101" {
		t.Fatalf("expected VMware host metadata, got %+v", hostRecord.Resource.VMware)
	}
	if got := hostRecord.Resource.VMware.ActiveAlarmCount; got != 1 {
		t.Fatalf("host active alarm count = %d, want 1", got)
	}
	if got := hostRecord.Resource.VMware.RecentTaskSummary; got != "Reconnect host (running)" {
		t.Fatalf("host recent task summary = %q, want %q", got, "Reconnect host (running)")
	}
	if hostRecord.Resource.Metrics == nil || hostRecord.Resource.Metrics.CPU == nil || hostRecord.Resource.Metrics.Memory == nil {
		t.Fatalf("expected VMware host metrics, got %+v", hostRecord.Resource.Metrics)
	}
	if got := hostRecord.Resource.Metrics.CPU.Percent; got != 21.4 {
		t.Fatalf("host cpu percent = %v, want 21.4", got)
	}
	if got := hostRecord.Resource.Metrics.NetOut.Value; got != 2048 {
		t.Fatalf("host net out = %v, want 2048", got)
	}
	if len(hostRecord.Resource.Incidents) != 1 || hostRecord.Resource.Incidents[0].Code != "vmware_alarm_state" {
		t.Fatalf("expected host VMware incident projection, got %+v", hostRecord.Resource.Incidents)
	}
	if hostRecord.Identity.DMIUUID != "uuid-host-1" {
		t.Fatalf("host identity DMI UUID = %q, want uuid-host-1", hostRecord.Identity.DMIUUID)
	}
	if hostRecord.Resource.LastSeen != collectedAt {
		t.Fatalf("host last seen = %v, want %v", hostRecord.Resource.LastSeen, collectedAt)
	}

	vmRecord := records[1]
	if vmRecord.SourceID != "vc-1:vm:vm-201" {
		t.Fatalf("vm source id = %q, want vc-1:vm:vm-201", vmRecord.SourceID)
	}
	if vmRecord.Resource.Type != unifiedresources.ResourceTypeVM {
		t.Fatalf("vm resource type = %q, want %q", vmRecord.Resource.Type, unifiedresources.ResourceTypeVM)
	}
	if vmRecord.Resource.VMware == nil || vmRecord.Resource.VMware.CPUCount != 4 {
		t.Fatalf("expected VMware VM metadata with cpu count, got %+v", vmRecord.Resource.VMware)
	}
	if vmRecord.Resource.Status != unifiedresources.StatusWarning {
		t.Fatalf("vm status = %q, want %q", vmRecord.Resource.Status, unifiedresources.StatusWarning)
	}
	if got := vmRecord.Resource.VMware.ActiveAlarmSummary; got != "VM replication fault (red)" {
		t.Fatalf("vm active alarm summary = %q, want %q", got, "VM replication fault (red)")
	}
	if got := vmRecord.Resource.VMware.SnapshotCount; got != 2 {
		t.Fatalf("vm snapshot count = %d, want 2", got)
	}
	if vmRecord.Resource.Metrics == nil || vmRecord.Resource.Metrics.CPU == nil || vmRecord.Resource.Metrics.Memory == nil {
		t.Fatalf("expected VMware VM metrics, got %+v", vmRecord.Resource.Metrics)
	}
	if got := vmRecord.Resource.Metrics.Memory.Percent; got != 57.5 {
		t.Fatalf("vm memory percent = %v, want 57.5", got)
	}
	if got := vmRecord.Resource.Metrics.DiskWrite.Value; got != 2048 {
		t.Fatalf("vm disk write = %v, want 2048", got)
	}
	if len(vmRecord.Resource.Incidents) != 1 || vmRecord.Resource.Incidents[0].Code != "vmware_alarm_state" {
		t.Fatalf("expected VMware VM incident projection, got %+v", vmRecord.Resource.Incidents)
	}

	datastoreRecord := records[2]
	if datastoreRecord.SourceID != "vc-1:datastore:datastore-11" {
		t.Fatalf("datastore source id = %q, want vc-1:datastore:datastore-11", datastoreRecord.SourceID)
	}
	if datastoreRecord.Resource.Type != unifiedresources.ResourceTypeStorage {
		t.Fatalf("datastore resource type = %q, want %q", datastoreRecord.Resource.Type, unifiedresources.ResourceTypeStorage)
	}
	if datastoreRecord.Resource.Storage == nil || datastoreRecord.Resource.Storage.Platform != "vmware-vsphere" {
		t.Fatalf("expected canonical VMware storage metadata, got %+v", datastoreRecord.Resource.Storage)
	}
	if datastoreRecord.Resource.Status != unifiedresources.StatusWarning {
		t.Fatalf("datastore status = %q, want %q", datastoreRecord.Resource.Status, unifiedresources.StatusWarning)
	}
	if datastoreRecord.Resource.Metrics == nil || datastoreRecord.Resource.Metrics.Disk == nil {
		t.Fatal("expected datastore disk metrics to be populated")
	}
	if got := datastoreRecord.Resource.Metrics.Disk.Percent; got != 60 {
		t.Fatalf("datastore disk usage percent = %v, want 60", got)
	}
	if len(datastoreRecord.Resource.Incidents) != 1 || datastoreRecord.Resource.Incidents[0].Code != "vmware_health_state" {
		t.Fatalf("expected VMware datastore health incident, got %+v", datastoreRecord.Resource.Incidents)
	}
	if got := datastoreRecord.Resource.VMware.RecentTaskSummary; got != "Refresh datastore (queued)" {
		t.Fatalf("datastore recent task summary = %q, want %q", got, "Refresh datastore (queued)")
	}
}
