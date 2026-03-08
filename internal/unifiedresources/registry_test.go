package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

func TestResourceRegistry_ListByType(t *testing.T) {
	rr := NewRegistry(nil)

	now := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "host-1",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "host-1",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{MachineID: "machine-1"},
		},
	})

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{
			SourceID: "vm-100",
			Resource: Resource{
				Type:     ResourceTypeVM,
				Name:     "vm-100",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"vm-100"}},
		},
		{
			SourceID: "vm-101",
			Resource: Resource{
				Type:     ResourceTypeVM,
				Name:     "vm-101",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"vm-101"}},
		},
		{
			SourceID: "ct-200",
			Resource: Resource{
				Type:     ResourceTypeSystemContainer,
				Name:     "ct-200",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"ct-200"}},
		},
	})

	got := rr.ListByType(ResourceTypeVM)
	if len(got) != 2 {
		t.Fatalf("expected 2 VMs, got %d", len(got))
	}
	for _, r := range got {
		if r.Type != ResourceTypeVM {
			t.Fatalf("expected all resources to be type %q, got %q", ResourceTypeVM, r.Type)
		}
	}

	// Deterministic ordering (sorted by ID).
	wantIDs := []string{
		rr.sourceSpecificID(ResourceTypeVM, SourceProxmox, "vm-100"),
		rr.sourceSpecificID(ResourceTypeVM, SourceProxmox, "vm-101"),
	}
	// Hash order isn't meaningful; the contract is lexicographic by ID.
	if wantIDs[0] > wantIDs[1] {
		wantIDs[0], wantIDs[1] = wantIDs[1], wantIDs[0]
	}
	if got[0].ID != wantIDs[0] || got[1].ID != wantIDs[1] {
		t.Fatalf("expected IDs %v, got [%s %s]", wantIDs, got[0].ID, got[1].ID)
	}

	// Returned resources should be copies (mutating the result does not mutate the registry).
	origName := got[0].Name
	got[0].Name = "mutated"
	if r, ok := rr.Get(got[0].ID); !ok || r == nil {
		t.Fatalf("expected Get(%q) to succeed", got[0].ID)
	} else if r.Name != origName {
		t.Fatalf("expected registry resource name %q, got %q", origName, r.Name)
	}
}

func TestResourceRegistry_ListByType_Empty(t *testing.T) {
	rr := NewRegistry(nil)
	if got := rr.ListByType(ResourceTypeVM); len(got) != 0 {
		t.Fatalf("expected empty result, got %d", len(got))
	}
}

func TestResourceRegistry_IngestRecords_UnknownSource(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC)

	customSource := DataSource("xcp")
	rr.IngestRecords(customSource, []IngestRecord{
		{
			SourceID: "host-1",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "xcp-host-1",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"xcp-host-1"}},
		},
	})

	hosts := rr.ListByType(ResourceTypeAgent)
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host for custom source, got %d", len(hosts))
	}
	if hosts[0].Name != "xcp-host-1" {
		t.Fatalf("expected host name xcp-host-1, got %q", hosts[0].Name)
	}
	targets := rr.SourceTargets(hosts[0].ID)
	if len(targets) != 1 {
		t.Fatalf("expected 1 source target, got %d", len(targets))
	}
	if targets[0].Source != customSource {
		t.Fatalf("expected custom source %q, got %q", customSource, targets[0].Source)
	}
	if targets[0].SourceID != "host-1" {
		t.Fatalf("expected source ID host-1, got %q", targets[0].SourceID)
	}
}

func TestResourceRegistry_IngestSnapshotUnifiesLinkedProxmoxNodeViewsByHostIdentity(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:              "homelab-minipc",
				Name:            "minipc",
				Instance:        "homelab-entry",
				ClusterName:     "homelab",
				IsClusterMember: true,
				Host:            "https://10.0.0.5:8006",
				LinkedAgentID:   "host-1",
				Status:          "online",
				LastSeen:        now,
			},
			{
				ID:            "standalone-minipc",
				Name:          "minipc",
				Instance:      "minipc-standalone",
				Host:          "https://10.0.0.5:8006",
				LinkedAgentID: "host-1",
				Status:        "online",
				LastSeen:      now,
			},
		},
		Hosts: []models.Host{
			{
				ID:        "host-1",
				Hostname:  "minipc.local",
				MachineID: "machine-1",
				ReportIP:  "10.0.0.5",
				Status:    "online",
				LastSeen:  now,
				NetworkInterfaces: []models.HostNetworkInterface{
					{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"10.0.0.5/24"}},
				},
			},
		},
	})

	agents := rr.ListByType(ResourceTypeAgent)
	if len(agents) != 1 {
		t.Fatalf("expected 1 unified agent resource, got %d", len(agents))
	}
	resource := agents[0]
	if resource.Identity.MachineID != "machine-1" {
		t.Fatalf("MachineID = %q, want machine-1", resource.Identity.MachineID)
	}
	if !containsDataSource(resource.Sources, SourceAgent) || !containsDataSource(resource.Sources, SourceProxmox) {
		t.Fatalf("expected merged agent+proxmox sources, got %+v", resource.Sources)
	}
	if resource.Proxmox == nil || resource.Proxmox.NodeName != "minipc" {
		t.Fatalf("expected proxmox node metadata for minipc, got %+v", resource.Proxmox)
	}
}

func TestResourceRegistry_IngestSnapshotUnifiesAsymmetricLinkedProxmoxNodeViewsByHostIdentity(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:              "homelab-minipc",
				Name:            "minipc",
				Instance:        "homelab-entry",
				ClusterName:     "homelab",
				IsClusterMember: true,
				Host:            "https://10.0.0.5:8006",
				LinkedAgentID:   "host-1",
				Status:          "online",
				LastSeen:        now,
			},
			{
				ID:              "homelab-minipc-shadow",
				Name:            "minipc",
				Instance:        "homelab-shadow",
				ClusterName:     "homelab",
				IsClusterMember: true,
				Host:            "https://10.0.0.5:8006",
				Status:          "online",
				LastSeen:        now.Add(-time.Minute),
			},
		},
		Hosts: []models.Host{
			{
				ID:           "host-1",
				Hostname:     "minipc.local",
				MachineID:    "machine-1",
				ReportIP:     "10.0.0.5",
				Status:       "online",
				LastSeen:     now,
				LinkedNodeID: "homelab-minipc",
				NetworkInterfaces: []models.HostNetworkInterface{
					{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"10.0.0.5/24"}},
				},
			},
		},
	})

	agents := rr.ListByType(ResourceTypeAgent)
	if len(agents) != 1 {
		t.Fatalf("expected 1 unified agent resource, got %d", len(agents))
	}
	resource := agents[0]
	if resource.Identity.MachineID != "machine-1" {
		t.Fatalf("MachineID = %q, want machine-1", resource.Identity.MachineID)
	}
	if resource.Proxmox == nil {
		t.Fatalf("expected proxmox metadata")
	}
	if got := resource.Proxmox.LinkedAgentID; got != "host-1" {
		t.Fatalf("LinkedAgentID = %q, want host-1", got)
	}
	targets := rr.SourceTargets(resource.ID)
	if len(targets) != 3 {
		t.Fatalf("expected 3 source targets (2 proxmox + 1 agent), got %d", len(targets))
	}
}

func TestResourceRegistry_IngestSnapshotUnifiesHostLinkedProxmoxNodeViewsByHostIdentity(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:              "homelab-delly",
				Name:            "delly",
				Instance:        "homelab-entry",
				ClusterName:     "homelab",
				IsClusterMember: true,
				Host:            "https://10.0.0.9:8006",
				Status:          "online",
				LastSeen:        now,
			},
			{
				ID:              "homelab-delly-shadow",
				Name:            "delly",
				Instance:        "homelab-shadow",
				ClusterName:     "homelab",
				IsClusterMember: true,
				Host:            "https://10.0.0.9:8006",
				Status:          "online",
				LastSeen:        now.Add(-time.Minute),
			},
		},
		Hosts: []models.Host{
			{
				ID:           "host-1",
				Hostname:     "delly.local",
				MachineID:    "machine-delly",
				ReportIP:     "10.0.0.9",
				Status:       "online",
				LastSeen:     now,
				LinkedNodeID: "homelab-delly",
				NetworkInterfaces: []models.HostNetworkInterface{
					{Name: "eth0", MAC: "00:11:22:33:44:66", Addresses: []string{"10.0.0.9/24"}},
				},
			},
		},
	})

	agents := rr.ListByType(ResourceTypeAgent)
	if len(agents) != 1 {
		t.Fatalf("expected 1 unified agent resource, got %d", len(agents))
	}
	resource := agents[0]
	if resource.Identity.MachineID != "machine-delly" {
		t.Fatalf("MachineID = %q, want machine-delly", resource.Identity.MachineID)
	}
	if resource.Proxmox == nil {
		t.Fatalf("expected proxmox metadata")
	}
	if got := resource.Proxmox.LinkedAgentID; got != "host-1" {
		t.Fatalf("LinkedAgentID = %q, want host-1", got)
	}
	targets := rr.SourceTargets(resource.ID)
	if len(targets) != 3 {
		t.Fatalf("expected 3 source targets (2 proxmox + 1 agent), got %d", len(targets))
	}
}

func TestResourceRegistry_IngestSnapshotUnifiesHostLinkedProxmoxNodeViewsAcrossEndpointForms(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:       "minipc-ip-view",
				Name:     "minipc",
				Instance: "standalone-ip",
				Host:     "https://10.0.0.5:8006",
				Status:   "online",
				LastSeen: now,
			},
			{
				ID:       "minipc-hostname-view",
				Name:     "minipc",
				Instance: "standalone-hostname",
				Host:     "https://minipc.local:8006",
				Status:   "online",
				LastSeen: now.Add(-time.Minute),
			},
		},
		Hosts: []models.Host{
			{
				ID:           "host-1",
				Hostname:     "minipc.local",
				MachineID:    "machine-minipc",
				ReportIP:     "10.0.0.5",
				Status:       "online",
				LastSeen:     now,
				LinkedNodeID: "minipc-ip-view",
				NetworkInterfaces: []models.HostNetworkInterface{
					{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"10.0.0.5/24"}},
				},
			},
		},
	})

	agents := rr.ListByType(ResourceTypeAgent)
	if len(agents) != 1 {
		t.Fatalf("expected 1 unified agent resource, got %d", len(agents))
	}
	resource := agents[0]
	if resource.Identity.MachineID != "machine-minipc" {
		t.Fatalf("MachineID = %q, want machine-minipc", resource.Identity.MachineID)
	}
	if resource.Proxmox == nil {
		t.Fatalf("expected proxmox metadata")
	}
	targets := rr.SourceTargets(resource.ID)
	if len(targets) != 3 {
		t.Fatalf("expected 3 source targets (2 proxmox + 1 agent), got %d", len(targets))
	}
}

func TestResourceRegistry_IngestSnapshotCreatesPhysicalDisksFromHostSMART(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:       "host-tower",
				Hostname: "tower",
				Status:   "online",
				LastSeen: now,
				Disks: []models.Disk{
					{Device: "/dev/sdb", Total: 12 * 1024, Mountpoint: "/mnt/disk1"},
				},
				Sensors: models.HostSensorSummary{
					SMART: []models.HostDiskSMART{
						{
							Device:      "/dev/sdb",
							Model:       "Seagate IronWolf",
							Serial:      "SERIAL-TOWER-1",
							Type:        "sata",
							Temperature: 37,
							Health:      "PASSED",
						},
					},
				},
			},
		},
	})

	disks := rr.ListByType(ResourceTypePhysicalDisk)
	if len(disks) != 1 {
		t.Fatalf("expected 1 physical disk resource, got %d", len(disks))
	}
	disk := disks[0]
	if !containsDataSource(disk.Sources, SourceAgent) {
		t.Fatalf("expected agent-backed physical disk source, got %+v", disk.Sources)
	}
	if disk.ParentID == nil {
		t.Fatalf("expected host parent on physical disk")
	}
	parent, ok := rr.Get(*disk.ParentID)
	if !ok || parent == nil || parent.Name != "tower" {
		t.Fatalf("expected disk parent to resolve to tower host, got %+v", parent)
	}
	if disk.PhysicalDisk == nil || disk.PhysicalDisk.Serial != "SERIAL-TOWER-1" {
		t.Fatalf("expected SMART-backed disk metadata, got %+v", disk.PhysicalDisk)
	}
	if disk.Identity.MachineID != "SERIAL-TOWER-1" {
		t.Fatalf("MachineID = %q, want SERIAL-TOWER-1", disk.Identity.MachineID)
	}
	if disk.PhysicalDisk.Risk != nil {
		t.Fatalf("expected healthy disk to have no risk payload, got %+v", disk.PhysicalDisk.Risk)
	}
}

func TestResourceRegistry_IngestSnapshotMergesAgentAndProxmoxPhysicalDisksByIdentity(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		PhysicalDisks: []models.PhysicalDisk{
			{
				ID:          "pve-disk-1",
				Node:        "tower",
				Instance:    "pve-tower",
				DevPath:     "/dev/sdb",
				Model:       "Seagate IronWolf",
				Serial:      "SERIAL-TOWER-1",
				Type:        "sata",
				Health:      "PASSED",
				Temperature: 34,
				LastChecked: now,
			},
		},
		Hosts: []models.Host{
			{
				ID:       "host-tower",
				Hostname: "tower",
				Status:   "online",
				LastSeen: now,
				Sensors: models.HostSensorSummary{
					SMART: []models.HostDiskSMART{
						{
							Device:      "/dev/sdb",
							Model:       "Seagate IronWolf",
							Serial:      "SERIAL-TOWER-1",
							Type:        "sata",
							Temperature: 37,
							Health:      "PASSED",
						},
					},
				},
			},
		},
	})

	disks := rr.ListByType(ResourceTypePhysicalDisk)
	if len(disks) != 1 {
		t.Fatalf("expected 1 merged physical disk resource, got %d", len(disks))
	}
	disk := disks[0]
	if !containsDataSource(disk.Sources, SourceAgent) || !containsDataSource(disk.Sources, SourceProxmox) {
		t.Fatalf("expected merged proxmox+agent disk sources, got %+v", disk.Sources)
	}
	if disk.PhysicalDisk == nil || disk.PhysicalDisk.Temperature != 37 {
		t.Fatalf("expected merged SMART temperature from agent disk, got %+v", disk.PhysicalDisk)
	}
}

func TestResourceRegistry_IngestSnapshotPropagatesUnraidDiskRole(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:       "host-tower",
				Hostname: "tower",
				Status:   "online",
				LastSeen: now,
				Unraid: &models.HostUnraidStorage{
					ArrayStarted: true,
					Disks: []models.HostUnraidDisk{
						{Name: "parity", Device: "/dev/sdb", Role: "parity", Status: "online", Serial: "SERIAL-TOWER-1"},
					},
				},
				Sensors: models.HostSensorSummary{
					SMART: []models.HostDiskSMART{
						{
							Device:      "/dev/sdb",
							Model:       "Seagate IronWolf",
							Serial:      "SERIAL-TOWER-1",
							Type:        "sata",
							Temperature: 37,
							Health:      "PASSED",
						},
					},
				},
			},
		},
	})

	disks := rr.ListByType(ResourceTypePhysicalDisk)
	if len(disks) != 1 {
		t.Fatalf("expected 1 physical disk resource, got %d", len(disks))
	}
	if disks[0].PhysicalDisk == nil {
		t.Fatal("expected physical disk metadata")
	}
	if got := disks[0].PhysicalDisk.StorageRole; got != "parity" {
		t.Fatalf("storageRole = %q, want parity", got)
	}
	if got := disks[0].PhysicalDisk.StorageGroup; got != "unraid-array" {
		t.Fatalf("storageGroup = %q, want unraid-array", got)
	}
}

func TestResourceRegistry_IngestSnapshotCreatesUnraidStorageResource(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:        "host-tower",
				Hostname:  "tower",
				Status:    "online",
				LastSeen:  now,
				MachineID: "machine-tower",
				Disks: []models.Disk{
					{Mountpoint: "/mnt/user", Total: 1000, Used: 400, Free: 600, Usage: 40},
				},
				Unraid: &models.HostUnraidStorage{
					ArrayStarted: true,
					ArrayState:   "STARTED",
					NumProtected: 1,
					Disks: []models.HostUnraidDisk{
						{Name: "parity", Role: "parity", Status: "online"},
						{Name: "disk1", Role: "data", Status: "online"},
					},
				},
			},
		},
	})

	storage := rr.ListByType(ResourceTypeStorage)
	if len(storage) != 1 {
		t.Fatalf("expected 1 unraid storage resource, got %d", len(storage))
	}
	resource := storage[0]
	if !containsDataSource(resource.Sources, SourceAgent) {
		t.Fatalf("expected agent-backed storage source, got %+v", resource.Sources)
	}
	if resource.ParentID == nil {
		t.Fatalf("expected host parent on unraid storage")
	}
	if resource.Storage == nil || resource.Storage.Type != "unraid-array" || resource.Storage.Platform != "unraid" {
		t.Fatalf("expected unraid storage metadata, got %+v", resource.Storage)
	}
	if resource.Metrics == nil || resource.Metrics.Disk == nil || resource.Metrics.Disk.Percent != 40 {
		t.Fatalf("expected disk metrics from unraid storage, got %+v", resource.Metrics)
	}
}

func TestResourceRegistry_IngestSnapshotDerivesPhysicalDiskRisk(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)
	pending := int64(2)

	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:       "host-risky",
				Hostname: "tower",
				Status:   "online",
				LastSeen: now,
				Sensors: models.HostSensorSummary{
					SMART: []models.HostDiskSMART{
						{
							Device:      "/dev/sdc",
							Model:       "Seagate IronWolf",
							Serial:      "SERIAL-RISK-1",
							Type:        "sata",
							Temperature: 64,
							Health:      "PASSED",
							Attributes: &models.SMARTAttributes{
								PendingSectors: &pending,
							},
						},
					},
				},
			},
		},
	})

	disks := rr.ListByType(ResourceTypePhysicalDisk)
	if len(disks) != 1 {
		t.Fatalf("expected 1 physical disk resource, got %d", len(disks))
	}
	disk := disks[0]
	if disk.Status != StatusWarning {
		t.Fatalf("Status = %q, want %q", disk.Status, StatusWarning)
	}
	if disk.PhysicalDisk == nil || disk.PhysicalDisk.Risk == nil {
		t.Fatalf("expected disk risk payload, got %+v", disk.PhysicalDisk)
	}
	if disk.PhysicalDisk.Risk.Level != storagehealth.RiskCritical {
		t.Fatalf("risk level = %q, want %q", disk.PhysicalDisk.Risk.Level, storagehealth.RiskCritical)
	}
	if len(disk.PhysicalDisk.Risk.Reasons) == 0 || disk.PhysicalDisk.Risk.Reasons[0].Code != "pending_sectors" {
		t.Fatalf("expected pending sectors reason, got %+v", disk.PhysicalDisk.Risk.Reasons)
	}
}

func TestResourceRegistry_BuildChildCounts_ReparentClearsOldParentCount(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 2, 12, 1, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{
			SourceID: "node-a",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "node-a",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"node-a"}},
		},
		{
			SourceID: "node-b",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "node-b",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"node-b"}},
		},
		{
			SourceID:       "vm-100",
			ParentSourceID: "node-a",
			Resource: Resource{
				Type:     ResourceTypeVM,
				Name:     "vm-100",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"vm-100"}},
		},
	})

	parentAID := rr.sourceSpecificID(ResourceTypeAgent, SourceProxmox, "node-a")
	parentBID := rr.sourceSpecificID(ResourceTypeAgent, SourceProxmox, "node-b")
	vmID := rr.sourceSpecificID(ResourceTypeVM, SourceProxmox, "vm-100")

	parentA, ok := rr.Get(parentAID)
	if !ok {
		t.Fatalf("expected parent A %q to exist", parentAID)
	}
	parentB, ok := rr.Get(parentBID)
	if !ok {
		t.Fatalf("expected parent B %q to exist", parentBID)
	}
	vm, ok := rr.Get(vmID)
	if !ok {
		t.Fatalf("expected vm %q to exist", vmID)
	}
	if parentA.ChildCount != 1 || parentB.ChildCount != 0 {
		t.Fatalf("expected initial child counts parentA=1 parentB=0, got parentA=%d parentB=%d", parentA.ChildCount, parentB.ChildCount)
	}
	if vm.ParentName != "node-a" {
		t.Fatalf("expected vm parent name %q, got %q", "node-a", vm.ParentName)
	}

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{
			SourceID: "node-a",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "node-a",
				Status:   StatusOnline,
				LastSeen: now.Add(30 * time.Second),
			},
			Identity: ResourceIdentity{Hostnames: []string{"node-a"}},
		},
		{
			SourceID: "node-b",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "node-b",
				Status:   StatusOnline,
				LastSeen: now.Add(30 * time.Second),
			},
			Identity: ResourceIdentity{Hostnames: []string{"node-b"}},
		},
		{
			SourceID:       "vm-100",
			ParentSourceID: "node-b",
			Resource: Resource{
				Type:     ResourceTypeVM,
				Name:     "vm-100",
				Status:   StatusOnline,
				LastSeen: now.Add(30 * time.Second),
			},
			Identity: ResourceIdentity{Hostnames: []string{"vm-100"}},
		},
	})

	parentA, ok = rr.Get(parentAID)
	if !ok {
		t.Fatalf("expected parent A %q to exist after reparent", parentAID)
	}
	parentB, ok = rr.Get(parentBID)
	if !ok {
		t.Fatalf("expected parent B %q to exist after reparent", parentBID)
	}
	vm, ok = rr.Get(vmID)
	if !ok {
		t.Fatalf("expected vm %q to exist after reparent", vmID)
	}
	if parentA.ChildCount != 0 || parentB.ChildCount != 1 {
		t.Fatalf("expected child counts parentA=0 parentB=1 after reparent, got parentA=%d parentB=%d", parentA.ChildCount, parentB.ChildCount)
	}
	if vm.ParentName != "node-b" {
		t.Fatalf("expected vm parent name %q after reparent, got %q", "node-b", vm.ParentName)
	}
}
