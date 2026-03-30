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
		}},
		VMs: []InventoryVM{{
			VM:            "vm-201",
			Name:          "app-01",
			PowerState:    "POWERED_ON",
			CPUCount:      4,
			MemorySizeMiB: 8192,
		}},
		Datastores: []InventoryDatastore{{
			Datastore: "datastore-11",
			Name:      "nvme-primary",
			Type:      "VMFS",
			FreeSpace: 40,
			Capacity:  100,
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
	if hostRecord.Resource.Status != unifiedresources.StatusOnline {
		t.Fatalf("host status = %q, want %q", hostRecord.Resource.Status, unifiedresources.StatusOnline)
	}
	if hostRecord.Resource.VMware == nil || hostRecord.Resource.VMware.ManagedObjectID != "host-101" {
		t.Fatalf("expected VMware host metadata, got %+v", hostRecord.Resource.VMware)
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
	if vmRecord.Resource.Status != unifiedresources.StatusOnline {
		t.Fatalf("vm status = %q, want %q", vmRecord.Resource.Status, unifiedresources.StatusOnline)
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
	if datastoreRecord.Resource.Metrics == nil || datastoreRecord.Resource.Metrics.Disk == nil {
		t.Fatal("expected datastore disk metrics to be populated")
	}
	if got := datastoreRecord.Resource.Metrics.Disk.Percent; got != 60 {
		t.Fatalf("datastore disk usage percent = %v, want 60", got)
	}
}
