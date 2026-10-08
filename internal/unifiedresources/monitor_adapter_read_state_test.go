package unifiedresources

import (
	"fmt"
	"reflect"
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

func TestMonitorAdapterCanonicalReferencePreservesAliasAmbiguity(t *testing.T) {
	registry := NewRegistry(nil)
	adapter := NewMonitorAdapter(registry)
	registry.IngestResources([]Resource{
		{ID: "host-a", Type: ResourceTypeAgent, Agent: &AgentData{AgentID: "one"}},
		{ID: "host-b", Type: ResourceTypeAgent, Agent: &AgentData{AgentID: "shared"}},
		{ID: "host-c", Type: ResourceTypeAgent, Agent: &AgentData{AgentID: "shared"}},
	})

	if id, ok := adapter.ResolveCanonicalResourceID("AGENT:ONE"); !ok || id != "host-a" {
		t.Fatalf("unique alias resolved to %q, %v; want host-a", id, ok)
	}
	if id, ok := adapter.ResolveCanonicalResourceID("agent:shared"); ok {
		t.Fatalf("ambiguous alias resolved to %q", id)
	}
	if id, ok := adapter.ResolveCanonicalResourceID("host-b"); !ok || id != "host-b" {
		t.Fatalf("exact ID resolved to %q, %v; want host-b", id, ok)
	}
}

func TestMonitorAdapterResolvesCanonicalResourceAncestorsNearestFirst(t *testing.T) {
	registry := NewRegistry(NewMemoryStore())
	adapter := NewMonitorAdapter(registry)
	clusterID := "cluster:analytics"
	nodeID := "node:pve-a"
	vmID := "vm:pve-a:101"
	adapter.PopulateSupplementalRecords(SourceProxmox, []IngestRecord{
		{SourceID: clusterID, Resource: Resource{ID: clusterID, Type: ResourceTypeAgent, Name: "analytics"}},
		{SourceID: nodeID, ParentSourceID: clusterID, Resource: Resource{ID: nodeID, Type: ResourceTypeAgent, Name: "pve-a"}},
		{SourceID: vmID, ParentSourceID: nodeID, Resource: Resource{ID: vmID, Type: ResourceTypeVM, Name: "vm-101"}},
	})

	canonicalVM, found := adapter.ResolveCanonicalResourceID(vmID)
	if !found {
		t.Fatalf("ResolveCanonicalResourceID(%q) did not find VM", vmID)
	}
	canonicalNode, _ := adapter.ResolveCanonicalResourceID(nodeID)
	canonicalCluster, _ := adapter.ResolveCanonicalResourceID(clusterID)
	want := []string{canonicalNode, canonicalCluster}
	if got := adapter.ResolveCanonicalResourceAncestors(canonicalVM); !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveCanonicalResourceAncestors() = %v, want %v", got, want)
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

// A link whose side arrives as a record joins during record ingest, so it
// must judge freshness by the adapter's thresholds as a snapshot-time link
// does. Under a five-minute vSphere threshold a VM polled three minutes ago
// is current, and its own memory reading outranks its in-guest agent's.
func TestMonitorAdapterJoinsLinkedRecordsWithConfiguredStaleThresholds(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		Hosts: []models.Host{{
			ID:        "host-app-guest",
			Hostname:  "app-guest",
			MachineID: "machine-app-guest",
			Status:    "online",
			LastSeen:  now,
			Memory:    models.Memory{Total: 8 << 30, Used: 6 << 30, Free: 2 << 30, Usage: 75},
		}},
		LastUpdate: now,
	}
	vmRecords := []IngestRecord{{
		SourceID: "vc-1:vm:vm-42",
		Resource: Resource{
			Type:       ResourceTypeVM,
			Technology: "vmware",
			Name:       "app-guest",
			Status:     StatusOnline,
			LastSeen:   now.Add(-3 * time.Minute),
			Metrics:    &ResourceMetrics{Memory: &MetricValue{Percent: 40, Source: SourceVMware}},
			VMware:     &VMwareData{ConnectionID: "vc-1", ManagedObjectID: "vm-42", EntityType: "vm"},
		},
	}}
	thresholds := map[DataSource]time.Duration{SourceVMware: 5 * time.Minute}

	unlinked := NewMonitorAdapter(NewRegistry(nil))
	unlinked.PopulateSnapshotAndSupplemental(snapshot, map[DataSource][]IngestRecord{SourceVMware: vmRecords})
	vms, hosts := unlinked.VMs(), unlinked.Hosts()
	if len(vms) != 1 || len(hosts) != 1 {
		t.Fatalf("unlinked estate = %d VMs, %d agents, want one of each", len(vms), len(hosts))
	}
	link := ResourceLink{ResourceA: vms[0].ID(), ResourceB: hosts[0].ID(), PrimaryID: hosts[0].ID()}

	for _, path := range []struct {
		name string
		read func(adapter *MonitorAdapter) ReadState
	}{
		{"rebuild", func(adapter *MonitorAdapter) ReadState {
			adapter.PopulateSnapshotAndSupplemental(snapshot, map[DataSource][]IngestRecord{SourceVMware: vmRecords})
			return adapter
		}},
		{"supplemental", func(adapter *MonitorAdapter) ReadState {
			adapter.PopulateFromSnapshot(snapshot)
			adapter.PopulateSupplementalRecords(SourceVMware, vmRecords)
			return adapter
		}},
		{"overlay", func(adapter *MonitorAdapter) ReadState {
			adapter.PopulateFromSnapshot(snapshot)
			return ReadStateWithRecords(adapter, SourceVMware, vmRecords)
		}},
	} {
		t.Run(path.name, func(t *testing.T) {
			store := NewMemoryStore()
			if err := store.AddLink(link); err != nil {
				t.Fatalf("add link: %v", err)
			}
			readState := path.read(NewMonitorAdapterWithStaleThresholds(NewRegistry(store), thresholds))

			if hosts := readState.Hosts(); len(hosts) != 0 {
				t.Fatalf("linked agent still listed standalone: %s", hosts[0].ID())
			}
			vms := readState.VMs()
			if len(vms) != 1 || vms[0].ID() != link.ResourceA {
				t.Fatalf("VMs = %d, want the linked VM %s", len(vms), link.ResourceA)
			}
			if vms[0].r.Metrics == nil || vms[0].r.Metrics.Memory == nil {
				t.Fatal("linked VM lost its memory reading")
			}
			if memory := vms[0].r.Metrics.Memory; memory.Source != SourceVMware || memory.Percent != 40 {
				t.Fatalf("linked VM memory = %.0f%% from %s, want vSphere's current 40%%", memory.Percent, memory.Source)
			}
		})
	}
}

// Every source merge into an existing row judges metric freshness by the
// adapter's configured thresholds, as the rebuild's links and stale pass do.
// A linked vSphere VM first reports no memory, so its in-guest agent supplies
// it; a refresh then brings vSphere memory observed three minutes ago, current
// under a five-minute threshold. The live supplemental refresh and the
// read-state overlay merge that record into the existing row, and must show
// vSphere's memory as a rebuild from the same records does. A Proxmox node
// polled every two minutes is likewise current at ninety seconds, so the
// rebuild keeps its readings over those of its auto-linked agent, silent for
// seventy-five seconds.
func TestMonitorAdapterSourceMergesUseConfiguredStaleThresholds(t *testing.T) {
	now := time.Now().UTC()
	thresholds := map[DataSource]time.Duration{SourceProxmox: 4 * time.Minute, SourceVMware: 5 * time.Minute}

	t.Run("linked vSphere VM refresh", func(t *testing.T) {
		snapshot := models.StateSnapshot{
			Hosts: []models.Host{{
				ID:        "host-app-guest",
				Hostname:  "app-guest",
				MachineID: "machine-app-guest",
				Status:    "online",
				LastSeen:  now,
				Memory:    models.Memory{Total: 8 << 30, Used: 6 << 30, Free: 2 << 30, Usage: 75},
			}},
			LastUpdate: now,
		}
		vmRecord := func(lastSeen time.Time, metrics *ResourceMetrics) []IngestRecord {
			return []IngestRecord{{
				SourceID: "vc-1:vm:vm-42",
				Resource: Resource{
					Type:       ResourceTypeVM,
					Technology: "vmware",
					Name:       "app-guest",
					Status:     StatusOnline,
					LastSeen:   lastSeen,
					Metrics:    metrics,
					VMware:     &VMwareData{ConnectionID: "vc-1", ManagedObjectID: "vm-42", EntityType: "vm"},
				},
			}}
		}
		initial := vmRecord(now.Add(-4*time.Minute), &ResourceMetrics{CPU: &MetricValue{Percent: 10, Source: SourceVMware}})
		refreshedAt := now.Add(-3 * time.Minute)
		refreshed := vmRecord(refreshedAt, &ResourceMetrics{
			CPU:    &MetricValue{Percent: 10, Source: SourceVMware},
			Memory: &MetricValue{Percent: 40, Source: SourceVMware},
		})

		unlinked := NewMonitorAdapter(NewRegistry(nil))
		unlinked.PopulateSnapshotAndSupplemental(snapshot, map[DataSource][]IngestRecord{SourceVMware: initial})
		vms, hosts := unlinked.VMs(), unlinked.Hosts()
		if len(vms) != 1 || len(hosts) != 1 {
			t.Fatalf("unlinked estate = %d VMs, %d agents, want one of each", len(vms), len(hosts))
		}
		link := ResourceLink{ResourceA: vms[0].ID(), ResourceB: hosts[0].ID(), PrimaryID: hosts[0].ID()}

		for _, path := range []struct {
			name string
			read func(adapter *MonitorAdapter) ReadState
		}{
			{"rebuild", func(adapter *MonitorAdapter) ReadState {
				adapter.PopulateSnapshotAndSupplemental(snapshot, map[DataSource][]IngestRecord{SourceVMware: refreshed})
				return adapter
			}},
			{"supplemental", func(adapter *MonitorAdapter) ReadState {
				adapter.PopulateSnapshotAndSupplemental(snapshot, map[DataSource][]IngestRecord{SourceVMware: initial})
				adapter.PopulateSupplementalRecords(SourceVMware, refreshed)
				return adapter
			}},
			{"overlay", func(adapter *MonitorAdapter) ReadState {
				adapter.PopulateSnapshotAndSupplemental(snapshot, map[DataSource][]IngestRecord{SourceVMware: initial})
				return ReadStateWithRecords(adapter, SourceVMware, refreshed)
			}},
		} {
			t.Run(path.name, func(t *testing.T) {
				store := NewMemoryStore()
				if err := store.AddLink(link); err != nil {
					t.Fatalf("add link: %v", err)
				}
				readState := path.read(NewMonitorAdapterWithStaleThresholds(NewRegistry(store), thresholds))

				vms := readState.VMs()
				if len(vms) != 1 || vms[0].ID() != link.ResourceA || len(readState.Hosts()) != 0 {
					t.Fatalf("estate = %d VMs, %d standalone agents, want only the linked VM %s", len(vms), len(readState.Hosts()), link.ResourceA)
				}
				if got := vms[0].r.SourceStatus[SourceVMware]; got.Status != "online" || !got.LastSeen.Equal(refreshedAt) {
					t.Fatalf("vSphere sighting = %+v, want the refresh, current under the configured threshold", got)
				}
				if vms[0].r.Metrics == nil || vms[0].r.Metrics.Memory == nil {
					t.Fatal("linked VM lost its memory reading")
				}
				if memory := vms[0].r.Metrics.Memory; memory.Source != SourceVMware || memory.Percent != 40 {
					t.Fatalf("linked VM memory = %.0f%% from %s, want vSphere's current 40%%", memory.Percent, memory.Source)
				}
			})
		}
	})

	t.Run("node with auto-linked agent", func(t *testing.T) {
		adapter := NewMonitorAdapterWithStaleThresholds(NewRegistry(nil), thresholds)
		adapter.PopulateFromSnapshot(models.StateSnapshot{
			Nodes: []models.Node{{
				ID:            "homelab-pve1",
				Name:          "pve1",
				Instance:      "homelab",
				Status:        "online",
				LinkedAgentID: "host-pve1",
				CPU:           0.12,
				Memory:        models.Memory{Total: 64 << 30, Used: 16 << 30, Free: 48 << 30, Usage: 25},
				LastSeen:      now.Add(-90 * time.Second),
			}},
			Hosts: []models.Host{{
				ID:              "host-pve1",
				MachineID:       "machine-pve1",
				Hostname:        "pve1",
				LinkedNodeID:    "homelab-pve1",
				Status:          "online",
				CPUUsage:        80,
				Memory:          models.Memory{Total: 64 << 30, Used: 48 << 30, Free: 16 << 30, Usage: 75},
				LastSeen:        now.Add(-75 * time.Second),
				IntervalSeconds: 30,
			}},
			LastUpdate: now,
		})

		nodes := adapter.Nodes()
		if len(nodes) != 1 {
			t.Fatalf("nodes = %d, want the node merged with its agent", len(nodes))
		}
		node := nodes[0].r
		if poll := node.SourceStatus[SourceProxmox]; poll.Status != "online" || !poll.LastSeen.Equal(now.Add(-90*time.Second)) {
			t.Fatalf("Proxmox sighting = %+v, want the current poll from ninety seconds ago", poll)
		}
		if got := node.SourceStatus[SourceAgent].Status; got != "stale" {
			t.Fatalf("agent sighting = %q, want stale", got)
		}
		if node.Metrics == nil || node.Metrics.CPU == nil || node.Metrics.Memory == nil {
			t.Fatalf("merged node lost its readings: %+v", node.Metrics)
		}
		if cpu := node.Metrics.CPU; cpu.Source != SourceProxmox || cpu.Percent != 12 {
			t.Fatalf("node CPU = %.0f%% from %s, want the current poll's 12%%", cpu.Percent, cpu.Source)
		}
		if memory := node.Metrics.Memory; memory.Source != SourceProxmox || memory.Percent != 25 {
			t.Fatalf("node memory = %.0f%% from %s, want the current poll's 25%%", memory.Percent, memory.Source)
		}
	})
}

func TestHostContinuityCannotOverwriteCurrentResource(t *testing.T) {
	now := time.Now().UTC()
	for _, mode := range []string{"machine-identity", "source-identity", "future-timestamp", "provider-only"} {
		t.Run(mode, func(t *testing.T) {
			registry := NewRegistry(nil)
			current := models.Host{ID: "live-agent", MachineID: "machine", Hostname: "pve", Status: "online", LastSeen: now, CPUUsage: 23}
			if mode == "provider-only" {
				registry.IngestSnapshot(models.StateSnapshot{Nodes: []models.Node{{
					ID: "pve-node", Name: "pve", Status: "online", LastSeen: now, LinkedAgentID: "saved-agent",
				}}})
			} else {
				registry.IngestRecords(SourceAgent, []IngestRecord{HostIngestRecord(current)})
			}
			saved := models.Host{ID: "saved-agent", MachineID: "machine", Hostname: "pve", Status: "offline", LastSeen: now.Add(-time.Minute)}
			if mode == "source-identity" {
				saved.ID = current.ID
			}
			if mode == "future-timestamp" {
				saved.LastSeen = now.Add(time.Hour)
			}
			if mode == "provider-only" {
				saved.LinkedNodeID = "pve-node"
			}
			base := NewMonitorAdapter(registry)
			before := base.GetAll()
			record := HostIngestRecord(saved)
			record.SupersededCanonicalIDs = []string{"agent:do-not-adopt-this-history"}
			overlay := ReadStateWithHostContinuity(base, []IngestRecord{record})
			after := overlay.(*MonitorAdapter).GetAll()
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("saved enrollment changed canonical observation:\nbefore=%+v\nafter=%+v", before, after)
			}
			if !reflect.DeepEqual(before, base.GetAll()) {
				t.Fatal("saved enrollment changed original registry")
			}
		})
	}
}

func TestHostContinuityRetainsDistinctMachinesWithSameHostname(t *testing.T) {
	now := time.Now().UTC()
	registry := NewRegistry(nil)
	registry.IngestRecords(SourceAgent, []IngestRecord{HostIngestRecord(models.Host{
		ID: "site-a-agent", Hostname: "pve", MachineID: "site-a-machine", Status: "online", LastSeen: now,
	})})
	view := ReadStateWithHostContinuity(NewMonitorAdapter(registry), []IngestRecord{HostIngestRecord(models.Host{
		ID: "site-b-agent", Hostname: "pve", MachineID: "site-b-machine", Status: "offline", LastSeen: now.Add(-time.Minute),
	})})
	if len(view.Hosts()) != 2 {
		t.Fatalf("hostname alone hid a separate machine: %d hosts", len(view.Hosts()))
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
	if got := MonitoredSystemCount(adapter); got != 1 {
		t.Fatalf("initial monitored-system count = %d, want 1", got)
	}

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

func TestMonitorAdapterKeepsOfflineProxmoxMemberWithoutRemovalChurn(t *testing.T) {
	store := NewMemoryStore()
	adapter := NewMonitorAdapter(NewRegistry(store))
	seen := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	node := models.Node{
		ID:               "production-pve-b",
		Name:             "pve-b",
		Instance:         "cluster-api",
		Status:           "online",
		Type:             "node",
		IsClusterMember:  true,
		ClusterName:      "production",
		LastSeen:         seen,
		ConnectionHealth: "healthy",
	}

	adapter.PopulateFromSnapshot(models.StateSnapshot{
		Nodes:      []models.Node{node},
		LastUpdate: seen,
	})
	resources := adapter.GetAll()
	if len(resources) != 1 {
		t.Fatalf("initial resources = %d, want 1", len(resources))
	}
	resourceID := resources[0].ID

	node.Status = "offline"
	node.ConnectionHealth = "error"
	node.CPU = 0
	node.Uptime = 0
	adapter.PopulateFromSnapshot(models.StateSnapshot{
		Nodes:      []models.Node{node},
		LastUpdate: seen.Add(time.Minute),
	})
	resources = adapter.GetAll()
	if len(resources) != 1 || resources[0].ID != resourceID || resources[0].Status == StatusOnline {
		t.Fatalf("offline membership projection = %+v, want stable non-online resource %q", resources, resourceID)
	}
	if resources[0].Proxmox == nil || resources[0].Proxmox.ConnectionHealth != "error" {
		t.Fatalf("offline membership lost provider reachability truth: %+v", resources[0].Proxmox)
	}
	if got := MonitoredSystemCount(adapter); got != 1 {
		t.Fatalf("offline member monitored-system count = %d, want 1", got)
	}

	changes, err := store.GetRecentChanges(resourceID, time.Time{}, 20)
	if err != nil {
		t.Fatalf("GetRecentChanges before removal: %v", err)
	}
	for _, change := range changes {
		if change.Metadata != nil && change.Metadata["changeType"] == "resource_removed" {
			t.Fatalf("offline transition emitted resource removal: %+v", change)
		}
	}

	// The monitor only emits this empty snapshot after its explicit,
	// repeatedly-confirmed membership reconciliation retires the node.
	adapter.PopulateFromSnapshot(models.StateSnapshot{LastUpdate: seen.Add(2 * time.Minute)})
	if resources = adapter.GetAll(); len(resources) != 0 {
		t.Fatalf("confirmed removal left %d canonical resources: %+v", len(resources), resources)
	}
	if got := MonitoredSystemCount(adapter); got != 0 {
		t.Fatalf("confirmed removal monitored-system count = %d, want 0", got)
	}
	changes, err = store.GetRecentChanges(resourceID, time.Time{}, 20)
	if err != nil {
		t.Fatalf("GetRecentChanges after removal: %v", err)
	}
	foundRemoval := false
	for _, change := range changes {
		if change.Metadata != nil && change.Metadata["changeType"] == "resource_removed" {
			foundRemoval = true
		}
	}
	if !foundRemoval {
		t.Fatalf("confirmed removal did not emit resource_removed: %+v", changes)
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

func TestMonitorAdapterStalenessDoesNotEmitRemovalButAuthoritativeOmissionDoes(t *testing.T) {
	store := NewMemoryStore()
	adapter := NewMonitorAdapter(NewRegistry(store))
	now := time.Now().UTC()
	container := func(vmid int, name string, seen time.Time) models.Container {
		return models.Container{
			ID:       fmt.Sprintf("lab:node-a:%d", vmid),
			VMID:     vmid,
			Name:     name,
			Node:     "node-a",
			Instance: "lab",
			Status:   "running",
			Type:     "lxc",
			LastSeen: seen,
		}
	}

	adapter.PopulateFromSnapshot(models.StateSnapshot{
		LastUpdate: now,
		Containers: []models.Container{
			container(101, "alpha", now),
			container(102, "beta", now),
		},
	})
	initial := adapter.GetByType(ResourceTypeSystemContainer)
	if len(initial) != 2 {
		t.Fatalf("initial container count = %d, want 2", len(initial))
	}
	var removedID string
	for _, resource := range initial {
		if resource.Name == "beta" {
			removedID = resource.ID
		}
	}
	if removedID == "" {
		t.Fatal("beta canonical ID not found")
	}

	staleSeen := now.Add(-5 * time.Minute)
	adapter.PopulateFromSnapshot(models.StateSnapshot{
		LastUpdate: now.Add(time.Second),
		Containers: []models.Container{
			container(101, "alpha", staleSeen),
			container(102, "beta", staleSeen),
		},
	})
	if got := len(adapter.GetByType(ResourceTypeSystemContainer)); got != 2 {
		t.Fatalf("stale refresh container count = %d, want 2", got)
	}
	if changes, err := store.GetRecentChanges(removedID, time.Time{}, 20); err != nil {
		t.Fatalf("GetRecentChanges before deletion: %v", err)
	} else {
		for _, change := range changes {
			if change.Metadata["changeType"] == "resource_removed" {
				t.Fatalf("staleness emitted a removal: %+v", change)
			}
		}
	}

	adapter.PopulateFromSnapshot(models.StateSnapshot{
		LastUpdate: now.Add(2 * time.Second),
		Containers: []models.Container{
			container(101, "alpha", now.Add(2*time.Second)),
		},
	})
	if got := len(adapter.GetByType(ResourceTypeSystemContainer)); got != 1 {
		t.Fatalf("authoritative deletion container count = %d, want 1", got)
	}
	changes, err := store.GetRecentChanges(removedID, time.Time{}, 20)
	if err != nil {
		t.Fatalf("GetRecentChanges after deletion: %v", err)
	}
	removals := 0
	for _, change := range changes {
		if change.Metadata["changeType"] == "resource_removed" {
			removals++
		}
	}
	if removals != 1 {
		t.Fatalf("resource removal history count = %d, want 1: %+v", removals, changes)
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

// Host continuity overlays are requested on every canonical read-state lookup.
// One built from a registry generation is reused until that generation, the
// requested records, or overlayReadStateMaxAge changes, and never mutates the
// live adapter.
func TestHostContinuityOverlayReusedWithinRegistryGeneration(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		LastUpdate: now,
		Hosts:      []models.Host{{ID: "live-1", Hostname: "live.example", Status: "online", LastSeen: now}},
	}
	adapter := NewMonitorAdapter(NewRegistry(nil))
	adapter.PopulateFromSnapshot(snapshot)
	offline := models.Host{ID: "offline-1", Hostname: "gone.example", Status: "offline", LastSeen: now.Add(-time.Hour)}
	hasHost := func(readState ReadState, hostname string) bool {
		for _, host := range readState.Hosts() {
			if host.Hostname() == hostname {
				return true
			}
		}
		return false
	}

	first := ReadStateWithHostContinuity(adapter, []IngestRecord{HostIngestRecord(offline)})
	if !hasHost(first, "gone.example") {
		t.Fatal("continuity overlay does not include the offline host")
	}
	// Records are rebuilt per request and carry a fresh UpdatedAt stamp.
	if again := ReadStateWithHostContinuity(adapter, []IngestRecord{HostIngestRecord(offline)}); again != first {
		t.Fatal("identical continuity request rebuilt the overlay within one generation")
	}
	if hasHost(adapter, "gone.example") {
		t.Fatal("continuity overlay mutated the live adapter")
	}

	other := offline
	other.ID, other.Hostname = "offline-2", "also-gone.example"
	if changed := ReadStateWithHostContinuity(adapter, []IngestRecord{HostIngestRecord(other)}); changed == first || !hasHost(changed, "also-gone.example") {
		t.Fatal("different continuity records reused the previous overlay")
	}

	snapshot.LastUpdate = now.Add(time.Second)
	adapter.PopulateFromSnapshot(snapshot)
	next := ReadStateWithHostContinuity(adapter, []IngestRecord{HostIngestRecord(offline)})
	if next == first || !hasHost(next, "gone.example") {
		t.Fatal("a new registry generation reused the previous generation's overlay")
	}

	var cache overlayReadStateCache
	registry := adapter.currentRegistry()
	key, _ := overlayRecordsKey(SourceAgent, []IngestRecord{HostIngestRecord(offline)}, true)
	cache.store(registry, now, key, now, next)
	if cache.lookup(registry, now, key, now.Add(overlayReadStateMaxAge-time.Millisecond)) != next {
		t.Fatal("overlay was not reused within its maximum age")
	}
	if cache.lookup(registry, now, key, now.Add(overlayReadStateMaxAge)) != nil {
		t.Fatal("overlay was reused past its maximum age")
	}
}

func TestMonitorAdapterProjectionSnapshotKeepsReplacedGeneration(t *testing.T) {
	var absent *MonitorAdapter
	rows, targets := absent.GetAllWithMetricsTargets()
	if rows != nil || targets != nil {
		t.Fatal("nil adapter invented a projection")
	}
	adapter := NewMonitorAdapter(NewRegistry(nil))
	snapshot := readRefreshSnapshot("before")
	adapter.PopulateFromSnapshot(snapshot)
	first, firstTargets := adapter.GetAllWithMetricsTargets()
	snapshot.VMs[0].Name = "after"
	snapshot.VMs[0].ID = "new-node:201"
	snapshot.VMs[0].VMID = 201
	// Observation time is deliberately unchanged.
	adapter.PopulateFromSnapshot(snapshot)
	second, secondTargets := adapter.GetAllWithMetricsTargets()
	if len(first) != 1 || len(second) != 1 || first[0].Name != "before" || second[0].Name != "after" {
		t.Fatal("replacement lost complete fresh resource content")
	}
	if firstTargets[first[0].ID].ResourceID == secondTargets[second[0].ID].ResourceID {
		t.Fatal("replacement retained old history coordinates")
	}
	for _, r := range second {
		if want := adapter.MetricsTargetForResource(r.ID); want == nil || *want != secondTargets[r.ID] {
			t.Fatal("target differs from the generation's point resolver")
		}
	}
	other := NewMonitorAdapter(NewRegistry(nil))
	snapshot.VMs[0].ID = "other-tenant:501"
	snapshot.VMs[0].Name = "other-tenant"
	other.PopulateFromSnapshot(snapshot)
	otherRows, otherTargets := other.GetAllWithMetricsTargets()
	if reflect.DeepEqual(otherRows, second) || reflect.DeepEqual(otherTargets, secondTargets) {
		t.Fatal("separate adapters shared a tenant projection")
	}
}
