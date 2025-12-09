package resources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestStorePopulateFromSnapshot(t *testing.T) {
	store := NewStore()

	// Create a minimal snapshot with test data
	snapshot := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:       "node-1",
				Name:     "pve-node-1",
				Instance: "https://pve1.local:8006",
				Status:   "online",
				CPU:      0.455, // Proxmox API returns 0-1 ratio
				Memory: models.Memory{
					Total: 16000000000,
					Used:  8000000000,
					Free:  8000000000,
					Usage: 50.0,
				},
				Uptime:   86400,
				LastSeen: time.Now(),
			},
		},
		VMs: []models.VM{
			{
				ID:       "vm-100",
				VMID:     100,
				Name:     "test-vm",
				Node:     "pve-node-1",
				Instance: "https://pve1.local:8006",
				Status:   "running",
				CPU:      0.25, // Proxmox API returns 0-1 ratio
				Memory: models.Memory{
					Total: 4000000000,
					Used:  2000000000,
					Free:  2000000000,
					Usage: 50.0,
				},
			},
		},
		Containers: []models.Container{
			{
				ID:       "ct-200",
				VMID:     200,
				Name:     "test-container",
				Node:     "pve-node-1",
				Instance: "https://pve1.local:8006",
				Status:   "running",
			},
		},
		Hosts: []models.Host{
			{
				ID:       "host-1",
				Hostname: "my-host",
				Status:   "online",
			},
		},
	}

	// Populate the store
	store.PopulateFromSnapshot(snapshot)

	// Verify resources were created
	all := store.GetAll()
	if len(all) == 0 {
		t.Fatal("Expected resources to be populated, got 0")
	}

	t.Logf("Populated %d resources", len(all))

	// Check for each type
	nodes := store.Query().OfType(ResourceTypeNode).Execute()
	if len(nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(nodes))
	} else {
		t.Logf("Node: id=%s, name=%s, cpu=%.1f%%", nodes[0].ID, nodes[0].Name, nodes[0].CPU.Current)
	}

	vms := store.Query().OfType(ResourceTypeVM).Execute()
	if len(vms) != 1 {
		t.Errorf("Expected 1 VM, got %d", len(vms))
	} else {
		t.Logf("VM: id=%s, name=%s, status=%s", vms[0].ID, vms[0].Name, vms[0].Status)
	}

	containers := store.Query().OfType(ResourceTypeContainer).Execute()
	if len(containers) != 1 {
		t.Errorf("Expected 1 container, got %d", len(containers))
	} else {
		t.Logf("Container: id=%s, name=%s", containers[0].ID, containers[0].Name)
	}

	hosts := store.Query().OfType(ResourceTypeHost).Execute()
	if len(hosts) != 1 {
		t.Errorf("Expected 1 host, got %d", len(hosts))
	} else {
		t.Logf("Host: id=%s, hostname=%s", hosts[0].ID, hosts[0].Identity.Hostname)
	}

	// Test summary
	t.Logf("SUCCESS: PopulateFromSnapshot works correctly!")
	t.Logf("Total resources: %d (1 node + 1 VM + 1 container + 1 host)", len(all))
}
