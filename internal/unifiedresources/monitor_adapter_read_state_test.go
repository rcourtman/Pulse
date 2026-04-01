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

func TestMonitorAdapterStoragePoolViewsAttachCanonicalMetricsTarget(t *testing.T) {
	adapter := NewMonitorAdapter(NewRegistry(nil))
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	adapter.PopulateSupplementalRecords(SourceVMware, []IngestRecord{
		{
			SourceID: "vc-1:datastore:datastore-202",
			Resource: Resource{
				ID:       "storage-vmware-1",
				Type:     ResourceTypeStorage,
				Name:     "archive-tier",
				Status:   StatusOnline,
				LastSeen: now,
				Storage: &StorageMeta{
					Type:     "datastore",
					Platform: "vmware",
					Nodes:    []string{"esxi-01.lab.local"},
				},
				VMware: &VMwareData{
					ConnectionID:    "vc-1",
					EntityType:      "datastore",
					ManagedObjectID: "datastore-202",
					RuntimeHostName: "esxi-01.lab.local",
				},
			},
		},
	})

	pools := adapter.StoragePools()
	if len(pools) != 1 {
		t.Fatalf("expected 1 storage pool view, got %d", len(pools))
	}
	if got := pools[0].SourceID(); got != "vc-1:datastore:datastore-202" {
		t.Fatalf("expected canonical metrics target on storage pool view, got %q", got)
	}
}

func TestMonitorAdapterRecordsSupplementalChangeTimeline(t *testing.T) {
	store := NewMemoryStore()
	adapter := NewMonitorAdapter(NewRegistry(store))

	source := SourceAgent
	sourceID := "xcp-host-1"
	firstSeen := time.Date(2026, 3, 8, 11, 0, 0, 0, time.UTC)
	adapter.PopulateSupplementalRecords(source, []IngestRecord{
		{
			SourceID: sourceID,
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "xcp-host-1",
				Status:   StatusOnline,
				LastSeen: firstSeen,
			},
		},
	})

	resources := adapter.GetAll()
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource after initial ingest, got %d", len(resources))
	}
	resourceID := resources[0].ID

	changes, err := store.GetRecentChanges(resourceID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetRecentChanges initial: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change after initial ingest, got %d", len(changes))
	}
	if changes[0].Kind != ChangeStateTransition {
		t.Fatalf("expected creation to record a state transition, got %q", changes[0].Kind)
	}

	adapter.PopulateSupplementalRecords(source, []IngestRecord{
		{
			SourceID: sourceID,
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "xcp-host-1",
				Status:   StatusWarning,
				LastSeen: firstSeen.Add(time.Minute),
			},
		},
	})

	changes, err = store.GetRecentChanges(resourceID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetRecentChanges after update: %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes after status update, got %d", len(changes))
	}
	if changes[0].Kind != ChangeStateTransition {
		t.Fatalf("expected latest change to be a state transition, got %q", changes[0].Kind)
	}
}

func TestMonitorAdapterRecordChangeForwardsToStore(t *testing.T) {
	store := NewMemoryStore()
	adapter := NewMonitorAdapter(NewRegistry(store))
	observedAt := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)

	if err := adapter.RecordChange(ResourceChange{
		ID:         "alert-change-1",
		ResourceID: "vm-1",
		ObservedAt: observedAt,
		OccurredAt: &observedAt,
		Kind:       ChangeAlertFired,
		SourceType: SourceHeuristic,
		Confidence: ConfidenceHigh,
		Reason:     "CPU threshold exceeded",
	}); err != nil {
		t.Fatalf("RecordChange: %v", err)
	}

	changes, err := store.GetRecentChanges("vm-1", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 forwarded change, got %d", len(changes))
	}
	if changes[0].Kind != ChangeAlertFired {
		t.Fatalf("Kind = %q, want %q", changes[0].Kind, ChangeAlertFired)
	}
}

func TestMonitorAdapterGetRecentChangesForwardsToStore(t *testing.T) {
	store := NewMemoryStore()
	adapter := NewMonitorAdapter(NewRegistry(store))
	observedAt := time.Date(2026, 3, 20, 12, 5, 0, 0, time.UTC)

	if err := store.RecordChange(ResourceChange{
		ID:         "command-change-1",
		ResourceID: "vm-2",
		ObservedAt: observedAt,
		Kind:       ChangeCommandExecuted,
		SourceType: SourceAgentAction,
		Confidence: ConfidenceHigh,
	}); err != nil {
		t.Fatalf("RecordChange: %v", err)
	}

	changes, err := adapter.GetRecentChanges("vm-2", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 forwarded change, got %d", len(changes))
	}
	if changes[0].Kind != ChangeCommandExecuted {
		t.Fatalf("Kind = %q, want %q", changes[0].Kind, ChangeCommandExecuted)
	}
}
