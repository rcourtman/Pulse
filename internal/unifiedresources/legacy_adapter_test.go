package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestLegacyAdapter_PopulateFromSnapshot_PreservesLegacyIDsAndParents(t *testing.T) {
	now := time.Now().UTC()
	adapter := NewLegacyAdapter(nil)
	adapter.PopulateFromSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "node-1", Status: "online"},
		},
		VMs: []models.VM{
			{ID: "vm-1", Name: "vm-1", Node: "node-1", Status: "running"},
		},
		Containers: []models.Container{
			{ID: "ct-1", Name: "ct-1", Node: "node-1", Status: "running"},
		},
		Storage: []models.Storage{
			{ID: "storage-1", Name: "storage-1", Node: "node-1", Status: "online", Total: 100, Used: 50, Free: 50, Usage: 50, Enabled: true, Active: true},
		},
		DockerHosts: []models.DockerHost{
			{
				ID:       "dh-1",
				Hostname: "dh-1",
				Status:   "online",
				LastSeen: now,
				Containers: []models.DockerContainer{
					{ID: "dc-1", Name: "dc-1", State: "running", Status: "running"},
				},
			},
		},
	})

	node, ok := adapter.Get("node-1")
	if !ok {
		t.Fatal("expected proxmox node legacy ID to be preserved")
	}
	if node.Type != LegacyResourceTypeNode {
		t.Fatalf("expected node type, got %q", node.Type)
	}

	vm, ok := adapter.Get("vm-1")
	if !ok {
		t.Fatal("expected vm legacy ID to be preserved")
	}
	if vm.ParentID != "node-1" {
		t.Fatalf("expected vm parent node-1, got %q", vm.ParentID)
	}

	ct, ok := adapter.Get("ct-1")
	if !ok {
		t.Fatal("expected container legacy ID to be preserved")
	}
	if ct.ParentID != "node-1" {
		t.Fatalf("expected container parent node-1, got %q", ct.ParentID)
	}

	storage, ok := adapter.Get("storage-1")
	if !ok {
		t.Fatal("expected storage resource to be ingested")
	}
	if storage.ParentID != "node-1" {
		t.Fatalf("expected storage parent node-1, got %q", storage.ParentID)
	}

	dockerContainer, ok := adapter.Get("dh-1/dc-1")
	if !ok {
		t.Fatal("expected docker container legacy ID to use hostID/containerID")
	}
	if dockerContainer.ParentID != "dh-1" {
		t.Fatalf("expected docker container parent dh-1, got %q", dockerContainer.ParentID)
	}

	stats := adapter.GetStats()
	if stats.TotalResources != 6 {
		t.Fatalf("expected 6 resources total, got %d", stats.TotalResources)
	}
}

func TestLegacyAdapter_MergedNodePrefersProxmoxLegacyID(t *testing.T) {
	adapter := NewLegacyAdapter(nil)
	adapter.PopulateFromSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "shared-host", Status: "online"},
		},
		Hosts: []models.Host{
			{ID: "host-1", Hostname: "shared-host", Status: "online"},
		},
	})

	if _, ok := adapter.Get("node-1"); !ok {
		t.Fatal("expected merged node/host resource to retain proxmox node legacy ID")
	}
}
