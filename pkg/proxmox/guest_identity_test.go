package proxmox

import (
	"context"
	"net/http"
	"testing"
)

func TestExtractClusterID(t *testing.T) {
	t.Parallel()

	clusterStatuses := []ClusterStatus{
		{Type: "cluster", Name: "prod", Quorate: 1, ClusterID: "6d2e4c1b-9a8f-4e7d-b3c2-1f0e9d8c7b6a"},
		{Type: "node", Name: "pve1", Online: 1, ClusterID: "ignored-node-id"},
	}
	if got := ExtractClusterID(clusterStatuses); got != "6d2e4c1b-9a8f-4e7d-b3c2-1f0e9d8c7b6a" {
		t.Fatalf("ExtractClusterID(cluster) = %q, want cluster entry ID", got)
	}

	standaloneStatuses := []ClusterStatus{
		{Type: "node", Name: "pve-solo", Online: 1, ClusterID: "11112222-3333-4444-5555-666677778888"},
	}
	if got := ExtractClusterID(standaloneStatuses); got != "11112222-3333-4444-5555-666677778888" {
		t.Fatalf("ExtractClusterID(standalone) = %q, want node entry ID", got)
	}

	pre93Statuses := []ClusterStatus{
		{Type: "cluster", Name: "legacy", Quorate: 1},
		{Type: "node", Name: "pve1", Online: 1},
	}
	if got := ExtractClusterID(pre93Statuses); got != "" {
		t.Fatalf("ExtractClusterID(pre-9.3) = %q, want empty", got)
	}
}

func TestPVE93IdentityFieldsDecode(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/cluster/status":
			writeJSON(t, w, map[string]interface{}{
				"data": []map[string]interface{}{
					{"type": "cluster", "id": "cluster", "name": "lab", "quorate": 1, "cluster-id": "cid-lab-93"},
					{"type": "node", "id": "node/pve1", "name": "pve1", "online": 1, "local": 1},
				},
			})
		case "/api2/json/cluster/resources":
			writeJSON(t, w, map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"id":     "qemu/100",
						"type":   "qemu",
						"node":   "pve1",
						"vmid":   100,
						"name":   "app-vm",
						"status": "running",
						"uid":    "0195a0b1-c2d3-7e4f-8a9b-0c1d2e3f4a5b",
						"rtime":  1764000100,
					},
					{
						"id":     "lxc/101",
						"type":   "lxc",
						"node":   "pve1",
						"vmid":   101,
						"name":   "dns-ct",
						"status": "running",
						"uid":    "0195a0b1-c2d3-7e4f-8a9b-ffffffffffff",
						"rtime":  1764000200,
					},
				},
			})
		case "/api2/json/nodes/pve1/qemu/100/status/current":
			writeJSON(t, w, map[string]interface{}{
				"data": map[string]interface{}{
					"vmid":   100,
					"status": "running",
					"uid":    "0195a0b1-c2d3-7e4f-8a9b-0c1d2e3f4a5b",
					"rtime":  1764000100,
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	statuses, err := client.GetClusterStatus(context.Background())
	if err != nil {
		t.Fatalf("GetClusterStatus error: %v", err)
	}
	if got := ExtractClusterID(statuses); got != "cid-lab-93" {
		t.Fatalf("cluster-id = %q, want cid-lab-93", got)
	}

	resources, err := client.GetClusterResources(context.Background(), "vm")
	if err != nil {
		t.Fatalf("GetClusterResources error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("len(resources) = %d, want 2", len(resources))
	}
	if resources[0].UID != "0195a0b1-c2d3-7e4f-8a9b-0c1d2e3f4a5b" || resources[0].RTime != 1764000100 {
		t.Fatalf("VM identity = (%q, %d), want (uid, 1764000100)", resources[0].UID, resources[0].RTime)
	}
	if resources[1].UID != "0195a0b1-c2d3-7e4f-8a9b-ffffffffffff" || resources[1].RTime != 1764000200 {
		t.Fatalf("LXC identity = (%q, %d), want (uid, 1764000200)", resources[1].UID, resources[1].RTime)
	}

	vmStatus, err := client.GetVMStatus(context.Background(), "pve1", 100)
	if err != nil {
		t.Fatalf("GetVMStatus error: %v", err)
	}
	if vmStatus.UID != "0195a0b1-c2d3-7e4f-8a9b-0c1d2e3f4a5b" || vmStatus.RTime != 1764000100 {
		t.Fatalf("VMStatus identity = (%q, %d), want (uid, 1764000100)", vmStatus.UID, vmStatus.RTime)
	}
}
