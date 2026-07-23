package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

func TestMonitorAdapterResolvesCanonicalOperatorIntentCapabilities(t *testing.T) {
	registry := NewRegistry(NewMemoryStore())
	adapter := NewMonitorAdapter(registry)
	adapter.PopulateSupplementalRecords(SourceProxmox, []IngestRecord{{
		SourceID: "pve-a:vm:101",
		Resource: Resource{
			ID:   "vm:pve-a:101",
			Type: ResourceTypeVM,
			Name: "vm-101",
		},
	}})

	canonicalID, found := adapter.ResolveCanonicalResourceID("pve-a:vm:101")
	if !found || canonicalID == "" {
		t.Fatalf("ResolveCanonicalResourceID() = %q, %v", canonicalID, found)
	}
	operatorState := ResourceOperatorState{
		CanonicalID:          canonicalID,
		IntentionallyOffline: true,
		MaintenanceReason:    "planned hardware work",
	}
	if err := registry.store.SetResourceOperatorState(operatorState); err != nil {
		t.Fatalf("SetResourceOperatorState() error = %v", err)
	}
	got, found, err := adapter.GetResourceOperatorState(canonicalID)
	if err != nil || !found {
		t.Fatalf("GetResourceOperatorState() found=%v error=%v", found, err)
	}
	if !got.IntentionallyOffline || got.MaintenanceReason != operatorState.MaintenanceReason {
		t.Fatalf("operator state = %+v, want persisted intent", got)
	}
}

func TestMonitorAdapterPhysicalDiskReadStateRetainsProxmoxIdentityAfterSMARTMerge(t *testing.T) {
	adapter := NewMonitorAdapter(NewRegistry(nil))
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)

	adapter.PopulateFromSnapshot(models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:       "host-pve",
				Hostname: "pve",
				Status:   "online",
				LastSeen: now,
				Sensors: models.HostSensorSummary{
					SMART: []models.HostDiskSMART{
						{
							Device:      "/dev/nvme0n1",
							Model:       "KINGSTON SNV3S2000G",
							Serial:      "SERIAL-NVME-0",
							Type:        "nvme",
							SizeBytes:   2_000_398_934_016,
							Temperature: 37,
							Health:      "PASSED",
						},
					},
				},
			},
		},
		PhysicalDisks: []models.PhysicalDisk{
			{
				ID:          "homelab-pve--dev-nvme0n1",
				Node:        "pve",
				Instance:    "homelab",
				DevPath:     "/dev/nvme0n1",
				Model:       "KINGSTON SNV3S2000G",
				Serial:      "SERIAL-NVME-0",
				Type:        "nvme",
				Size:        2_000_398_934_016,
				Health:      "PASSED",
				LastChecked: now,
			},
		},
	})

	disks := adapter.PhysicalDisks()
	if len(disks) != 1 {
		t.Fatalf("physical disk count = %d, want 1", len(disks))
	}
	disk := disks[0]
	if disk.Node() != "pve" || disk.Instance() != "homelab" {
		t.Fatalf("merged disk lost Proxmox identity: node=%q instance=%q", disk.Node(), disk.Instance())
	}
	if disk.Temperature() != 37 {
		t.Fatalf("merged disk temperature = %d, want 37", disk.Temperature())
	}
	if disk.SizeBytes() != 2_000_398_934_016 {
		t.Fatalf("merged disk sizeBytes = %d, want 2000398934016", disk.SizeBytes())
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

func TestReadStateWithRecordsClonesMonitorAdapterAndOverlaysRecords(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	registry := NewRegistry(nil)
	registry.IngestRecords(SourceAgent, []IngestRecord{
		HostIngestRecord(models.Host{
			ID:       "host-1",
			Hostname: "host-1.local",
			Status:   "online",
			LastSeen: now,
		}),
	})

	base := NewMonitorAdapter(registry)
	overlay := ReadStateWithRecords(base, SourceAgent, []IngestRecord{
		HostIngestRecord(models.Host{
			ID:       "host-2",
			Hostname: "host-2.local",
			Status:   "online",
			LastSeen: now.Add(time.Minute),
		}),
	})

	if got := len(base.Hosts()); got != 1 {
		t.Fatalf("base host count = %d, want 1", got)
	}
	if got := len(overlay.Hosts()); got != 2 {
		t.Fatalf("overlay host count = %d, want 2", got)
	}
}

func TestReadStateWithRecordsPreservesConfiguredStaleThresholds(t *testing.T) {
	seen := time.Now().UTC().Add(-90 * time.Second).Truncate(time.Millisecond)
	base := NewMonitorAdapterWithStaleThresholds(NewRegistry(nil), map[DataSource]time.Duration{
		SourceProxmox: 10 * time.Minute,
	})
	base.PopulateFromSnapshot(models.StateSnapshot{
		VMs: []models.VM{{
			ID:       "cluster-a:pve-a:101",
			Name:     "db",
			Node:     "pve-a",
			Instance: "cluster-a",
			VMID:     101,
			Status:   "running",
			Type:     "qemu",
			LastSeen: seen,
		}},
	})

	baseVMs := base.VMs()
	if len(baseVMs) != 1 {
		t.Fatalf("base VM count = %d, want 1", len(baseVMs))
	}
	if baseVMs[0].Status() != StatusOnline {
		t.Fatalf("base VM status = %q, want online", baseVMs[0].Status())
	}

	overlay := ReadStateWithRecords(base, SourceAgent, []IngestRecord{
		HostIngestRecord(models.Host{
			ID:       "host-1",
			Hostname: "host-1.local",
			Status:   "online",
			LastSeen: time.Now().UTC(),
		}),
	})

	overlayVMs := overlay.VMs()
	if len(overlayVMs) != 1 {
		t.Fatalf("overlay VM count = %d, want 1", len(overlayVMs))
	}
	if overlayVMs[0].Status() != StatusOnline {
		t.Fatalf("overlay VM status = %q, want online", overlayVMs[0].Status())
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

func TestMonitorAdapterAvailabilityChecksSurviveRebuildAndRecordDeletionByCheckID(t *testing.T) {
	store := NewMemoryStore()
	adapter := NewMonitorAdapter(NewRegistry(store))
	now := time.Date(2026, 7, 23, 20, 0, 0, 0, time.UTC)
	hostID := MachineIdentityCanonicalID(ResourceTypeAgent, "machine-core2026")
	snapshot := models.StateSnapshot{
		Hosts: []models.Host{{
			ID:        "agent-core2026",
			Hostname:  "core2026",
			MachineID: "machine-core2026",
			Status:    "online",
			LastSeen:  now,
		}},
		LastUpdate: now,
	}
	records := map[DataSource][]IngestRecord{
		SourceAvailability: {
			availabilityProbeRecord("stats-pv", "192.0.2.70", &AvailabilityData{
				LinkedResourceID: hostID,
				Address:          "192.0.2.70",
				Protocol:         "https",
				Enabled:          true,
				Available:        true,
			}),
		},
	}

	adapter.PopulateSnapshotAndSupplemental(snapshot, records)
	checks := adapter.currentRegistry().ListByType(ResourceTypeNetworkEndpoint)
	if len(checks) != 1 {
		t.Fatalf("check count after rebuild = %d, want 1", len(checks))
	}
	checkID := checks[0].ID
	host, ok := adapter.currentRegistry().Get(hostID)
	if !ok || len(AvailabilityChecksForResource(*host)) != 1 {
		t.Fatalf("host projection after rebuild = %+v", host)
	}

	// A reload/restart rebuild must preserve the canonical check row instead
	// of seeding the provider mapping from the host projection.
	snapshot.LastUpdate = now.Add(time.Minute)
	adapter.PopulateSnapshotAndSupplemental(snapshot, records)
	checks = adapter.currentRegistry().ListByType(ResourceTypeNetworkEndpoint)
	if len(checks) != 1 || checks[0].ID != checkID {
		t.Fatalf("check after restart = %+v, want stable ID %q", checks, checkID)
	}

	// Deleting the configured check means the next atomic replacement carries
	// no availability records. Both the row and its host projection disappear.
	snapshot.LastUpdate = now.Add(2 * time.Minute)
	adapter.PopulateSnapshotAndSupplemental(snapshot, nil)
	if got := adapter.currentRegistry().ListByType(ResourceTypeNetworkEndpoint); len(got) != 0 {
		t.Fatalf("check count after deletion = %d, want 0", len(got))
	}
	host, ok = adapter.currentRegistry().Get(hostID)
	if !ok || len(AvailabilityChecksForResource(*host)) != 0 {
		t.Fatalf("host retained deleted projection: %+v", host)
	}

	changes, err := store.GetRecentChanges(checkID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetRecentChanges(%q): %v", checkID, err)
	}
	changeTypes := map[string]bool{}
	for _, change := range changes {
		if change.Metadata != nil {
			if changeType, _ := change.Metadata["changeType"].(string); changeType != "" {
				changeTypes[changeType] = true
			}
		}
	}
	if !changeTypes["resource_created"] || !changeTypes["resource_removed"] {
		t.Fatalf("check history change types = %+v, want create and remove", changeTypes)
	}
}

func TestMonitorAdapterIngestsAvailabilityAfterCorrelatableSupplementalSources(t *testing.T) {
	adapter := NewMonitorAdapter(NewRegistry(nil))
	now := time.Date(2026, 7, 23, 21, 0, 0, 0, time.UTC)
	host := models.Host{
		ID:        "agent-supplemental",
		Hostname:  "supplemental-host",
		MachineID: "supplemental-machine",
		Status:    "online",
		LastSeen:  now,
	}
	hostID := MachineIdentityCanonicalID(ResourceTypeAgent, host.MachineID)

	adapter.PopulateSnapshotAndSupplemental(models.StateSnapshot{LastUpdate: now}, map[DataSource][]IngestRecord{
		SourceAvailability: {
			availabilityProbeRecord("supplemental-check", "192.0.2.75", &AvailabilityData{
				LinkedResourceID: hostID,
				Address:          "192.0.2.75",
				Protocol:         "tcp",
				Enabled:          true,
				Available:        true,
			}),
		},
		SourceAgent: {HostIngestRecord(host)},
	})

	check := availabilityEndpointByTarget(t, adapter.currentRegistry(), "supplemental-check")
	if len(check.Relationships) != 1 || check.Relationships[0].TargetID != hostID {
		t.Fatalf("availability was ingested before its supplemental target: %+v", check.Relationships)
	}
	projectedHost, ok := adapter.currentRegistry().Get(hostID)
	if !ok || len(AvailabilityChecksForResource(*projectedHost)) != 1 {
		t.Fatalf("supplemental host projection = %+v", projectedHost)
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

// The monitor adapter is the durable store-backed registry owner, so its
// rebuild paths must persist canonical identity pins: a later boot window
// (agent not yet checked in) relies on them to derive the same canonical host
// ID it used in steady state.
func TestMonitorAdapterRebuildPersistsIdentityPins(t *testing.T) {
	store := NewMemoryStore()
	adapter := NewMonitorAdapter(NewRegistry(store))

	const machineID = "7d465a78-test-machine-id"
	adapter.PopulateFromSnapshot(models.StateSnapshot{
		Nodes: []models.Node{{
			ID:          "homelab-delly",
			Name:        "delly",
			Instance:    "homelab",
			ClusterName: "homelab",
			Status:      "online",
		}},
		Hosts: []models.Host{{
			ID:           machineID,
			MachineID:    machineID,
			Hostname:     "delly",
			LinkedNodeID: "homelab-delly",
		}},
		LastUpdate: time.Now().UTC(),
	})

	pins, err := store.ListResourceIdentityPins()
	if err != nil {
		t.Fatalf("list identity pins: %v", err)
	}
	steadyID := buildHashID(ResourceTypeAgent, "machine:"+machineID)
	found := false
	for _, pin := range pins {
		if pin.CanonicalID == steadyID && pin.MachineID == machineID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected snapshot rebuild to persist a machine-keyed identity pin, got %+v", pins)
	}

	// A restart boot window rebuilds from a snapshot that does not contain
	// the agent host yet; the pinned identity must keep the canonical ID.
	adapter.PopulateFromSnapshot(models.StateSnapshot{
		Nodes: []models.Node{{
			ID:          "homelab-delly",
			Name:        "delly",
			Instance:    "homelab",
			ClusterName: "homelab",
			Status:      "online",
		}},
		LastUpdate: time.Now().UTC(),
	})

	for _, resource := range adapter.GetAll() {
		if resource.Proxmox != nil && resource.Proxmox.NodeName == "delly" {
			if resource.ID != steadyID {
				t.Fatalf("boot-window rebuild minted %q, want pinned %q", resource.ID, steadyID)
			}
			return
		}
	}
	t.Fatalf("expected delly node resource in boot-window rebuild")
}
