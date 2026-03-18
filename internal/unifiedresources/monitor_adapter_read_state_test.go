package unifiedresources

import (
	"testing"
	"time"
)

var _ ReadState = (*MonitorAdapter)(nil)

func TestMonitorAdapterReadStateForwardsToRegistry(t *testing.T) {
	adapter := NewMonitorAdapter(NewRegistry(nil))
	nodeID := "node-1"

	adapter.PopulateSupplementalRecords(SourceProxmox, []IngestRecord{
		{
			SourceID: "node-1",
			Resource: Resource{
				ID:     nodeID,
				Type:   ResourceTypeAgent,
				Name:   "node-1",
				Status: StatusOnline,
				Proxmox: &ProxmoxData{
					NodeName: "node-1",
				},
			},
		},
		{
			SourceID: "vm-101",
			Resource: Resource{
				ID:       "vm-101",
				Type:     ResourceTypeVM,
				Name:     "vm-101",
				Status:   StatusOnline,
				ParentID: &nodeID,
				Proxmox: &ProxmoxData{
					VMID:     101,
					NodeName: "node-1",
				},
			},
		},
		{
			SourceID: "storage-local",
			Resource: Resource{
				ID:       "storage-local",
				Type:     ResourceTypeStorage,
				Name:     "local",
				Status:   StatusOnline,
				ParentID: &nodeID,
				Storage: &StorageMeta{
					Type: "dir",
				},
			},
		},
		{
			SourceID: "disk-serial-1",
			Resource: Resource{
				ID:       "disk-serial-1",
				Type:     ResourceTypePhysicalDisk,
				Name:     "disk-serial-1",
				Status:   StatusOnline,
				ParentID: &nodeID,
				MetricsTarget: &MetricsTarget{
					ResourceType: "disk",
					ResourceID:   "SERIAL-1",
				},
				PhysicalDisk: &PhysicalDiskMeta{
					DevPath:     "/dev/sda",
					Model:       "disk-serial-1",
					Serial:      "SERIAL-1",
					Temperature: 35,
				},
				Proxmox: &ProxmoxData{
					NodeName: "node-1",
					Instance: "lab",
				},
			},
		},
	})

	if got := len(adapter.Nodes()); got != 1 {
		t.Fatalf("expected 1 node view, got %d", got)
	}
	if got := len(adapter.VMs()); got != 1 {
		t.Fatalf("expected 1 VM view, got %d", got)
	}
	if got := len(adapter.StoragePools()); got != 1 {
		t.Fatalf("expected 1 storage pool view, got %d", got)
	}
	if got := len(adapter.PhysicalDisks()); got != 1 {
		t.Fatalf("expected 1 physical disk view, got %d", got)
	}
	if got := len(adapter.Workloads()); got == 0 {
		t.Fatal("expected workload views from registry-backed adapter")
	}
	if got := len(adapter.Infrastructure()); got == 0 {
		t.Fatal("expected infrastructure views from registry-backed adapter")
	}
}

func TestMonitorAdapterReadStateReturnsClonedIncidents(t *testing.T) {
	adapter := NewMonitorAdapter(NewRegistry(nil))
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)

	adapter.PopulateSupplementalRecords(SourceTrueNAS, []IngestRecord{
		{
			SourceID: "system:tn1",
			Resource: Resource{
				ID:       "tn1",
				Type:     ResourceTypeAgent,
				Name:     "tn1",
				Status:   StatusWarning,
				LastSeen: now,
				Incidents: []ResourceIncident{{
					Provider:  "truenas",
					NativeID:  "alert-1",
					Code:      "truenas_volume_status",
					Severity:  "warning",
					Summary:   "Pool archive state is DEGRADED",
					StartedAt: now,
				}},
				TrueNAS: &TrueNASData{
					Hostname: "tn1",
					StorageRisk: &StorageRisk{
						Level: "warning",
					},
				},
			},
		},
	})

	first := adapter.GetAll()
	if len(first) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(first))
	}
	if len(first[0].Incidents) != 1 {
		t.Fatalf("expected incidents on cloned resource, got %+v", first[0].Incidents)
	}
	first[0].Incidents[0].Summary = "mutated"
	first[0].TrueNAS.StorageRisk.Level = "critical"

	second := adapter.GetAll()
	if len(second) != 1 {
		t.Fatalf("expected 1 resource on second read, got %d", len(second))
	}
	if got := second[0].Incidents[0].Summary; got != "Pool archive state is DEGRADED" {
		t.Fatalf("expected incident summary to be cloned, got %q", got)
	}
	if got := second[0].TrueNAS.StorageRisk.Level; got != "warning" {
		t.Fatalf("expected truenas storage risk to be cloned, got %q", got)
	}
}
