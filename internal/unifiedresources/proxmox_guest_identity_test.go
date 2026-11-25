package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestProxmoxGuestUIDFirstBindPreservesPre93CanonicalID(t *testing.T) {
	now := time.Now().UTC()
	rr := NewRegistry(nil)

	pre93 := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "homelab-pve1", Name: "pve1", Instance: "homelab", ClusterName: "homelab", IsClusterMember: true, Status: "online", LastSeen: now},
		},
		VMs: []models.VM{
			{ID: "homelab:pve1:100", VMID: 100, Name: "web-01", Node: "pve1", Instance: "homelab", Status: "running", LastSeen: now},
		},
	}
	rr.IngestSnapshot(pre93)
	beforeVMs := rr.ListByType(ResourceTypeVM)
	if len(beforeVMs) != 1 {
		t.Fatalf("expected 1 VM before PVE 9.3 upgrade, got %d", len(beforeVMs))
	}
	wantID := ProxmoxGuestCanonicalID(ResourceTypeVM, "homelab", 100)
	if beforeVMs[0].ID != wantID {
		t.Fatalf("pre-9.3 VM ID = %q, want %q", beforeVMs[0].ID, wantID)
	}

	upgraded := models.StateSnapshot{
		Nodes: pre93.Nodes,
		VMs: []models.VM{
			{
				ID:         "homelab:pve1:100",
				VMID:       100,
				GuestUID:   "0195a0b1-c2d3-7e4f-8a9b-000000000100",
				GuestRTime: 1764000100,
				Name:       "web-01",
				Node:       "pve1",
				Instance:   "homelab",
				Status:     "running",
				LastSeen:   now.Add(time.Minute),
			},
		},
	}
	rr.IngestSnapshot(upgraded)

	afterVMs := rr.ListByType(ResourceTypeVM)
	if len(afterVMs) != 1 {
		t.Fatalf("expected 1 VM after first UID bind, got %d", len(afterVMs))
	}
	if afterVMs[0].ID != wantID {
		t.Fatalf("first UID bind changed canonical ID: got %q, want %q", afterVMs[0].ID, wantID)
	}
	if afterVMs[0].Proxmox == nil || afterVMs[0].Proxmox.GuestUID != "0195a0b1-c2d3-7e4f-8a9b-000000000100" {
		t.Fatalf("expected GuestUID to bind onto existing VM, got %+v", afterVMs[0].Proxmox)
	}
}

func TestProxmoxGuestVMIDReuseMintsDistinctCanonicalIDAndSurvivesMigration(t *testing.T) {
	now := time.Now().UTC()
	rr := NewRegistry(nil)

	nodes := []models.Node{
		{ID: "homelab-pve1", Name: "pve1", Instance: "homelab", ClusterName: "homelab", IsClusterMember: true, Status: "online", LastSeen: now},
		{ID: "homelab-pve2", Name: "pve2", Instance: "homelab", ClusterName: "homelab", IsClusterMember: true, Status: "online", LastSeen: now},
	}

	// Original guest at VMID 100 with UID 1.
	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: nodes,
		VMs: []models.VM{
			{
				ID:         "homelab:pve1:100",
				VMID:       100,
				GuestUID:   "uid-guest-old",
				GuestRTime: 1764000100,
				Name:       "old-db",
				Node:       "pve1",
				Instance:   "homelab",
				Status:     "running",
				LastSeen:   now,
			},
		},
	})
	oldID := ProxmoxGuestCanonicalID(ResourceTypeVM, "homelab", 100)
	if _, ok := rr.Get(oldID); !ok {
		t.Fatalf("expected original guest at %q", oldID)
	}

	// Delete original guest in an intervening poll so slot history must still fence reuse.
	rr.IngestSnapshot(models.StateSnapshot{Nodes: nodes})

	// Create a brand-new guest reusing VMID 100 with UID 2.
	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: nodes,
		VMs: []models.VM{
			{
				ID:         "homelab:pve1:100",
				VMID:       100,
				GuestUID:   "uid-guest-new",
				GuestRTime: 1764000900,
				Name:       "new-cache",
				Node:       "pve1",
				Instance:   "homelab",
				Status:     "running",
				LastSeen:   now.Add(2 * time.Minute),
			},
		},
	})

	wantNewID := ProxmoxGuestUIDCanonicalID(ResourceTypeVM, "homelab", "uid-guest-new")
	if wantNewID == "" || wantNewID == oldID {
		t.Fatalf("expected distinct UID canonical ID, got %q (old %q)", wantNewID, oldID)
	}
	newVM, ok := rr.Get(wantNewID)
	if !ok || newVM == nil {
		t.Fatalf("expected replacement VM to mint %q, live VMs: %+v", wantNewID, rr.ListByType(ResourceTypeVM))
	}
	if len(newVM.SupersededCanonicalIDs) != 0 {
		t.Fatalf("VMID-reused guest must not inherit superseded eras from deleted guest, got %v", newVM.SupersededCanonicalIDs)
	}

	// Live-migrate the replacement guest from pve1 to pve2; it must stay on wantNewID.
	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: nodes,
		VMs: []models.VM{
			{
				ID:         "homelab:pve2:100",
				VMID:       100,
				GuestUID:   "uid-guest-new",
				GuestRTime: 1764000900,
				Name:       "new-cache",
				Node:       "pve2",
				Instance:   "homelab",
				Status:     "running",
				LastSeen:   now.Add(3 * time.Minute),
			},
		},
	})
	migrated, ok := rr.Get(wantNewID)
	if !ok || migrated == nil || migrated.Proxmox == nil || migrated.Proxmox.NodeName != "pve2" {
		t.Fatalf("expected migrated replacement guest to remain on %q with node pve2, got %+v", wantNewID, migrated)
	}
}

func TestProxmoxNodesWithDistinctClusterIDsNeverAutoMerge(t *testing.T) {
	now := time.Now().UTC()
	rr := NewRegistry(nil)

	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:              "prod-pve1",
				Name:            "pve1",
				Instance:        "prod",
				Host:            "https://10.0.1.10:8006",
				ClusterName:     "homelab",
				ClusterID:       "cid-cluster-alpha",
				IsClusterMember: true,
				Status:          "online",
				LastSeen:        now,
			},
			{
				ID:              "lab-pve1",
				Name:            "pve1",
				Instance:        "lab",
				Host:            "https://10.0.2.10:8006",
				ClusterName:     "homelab",
				ClusterID:       "cid-cluster-beta",
				IsClusterMember: true,
				Status:          "online",
				LastSeen:        now,
			},
		},
	})

	nodes := rr.ListByType(ResourceTypeAgent)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 separate nodes for distinct PVE 9.3 cluster-ids, got %d", len(nodes))
	}
	if nodes[0].ID == nodes[1].ID {
		t.Fatalf("nodes with distinct cluster-ids merged onto %q", nodes[0].ID)
	}
}
