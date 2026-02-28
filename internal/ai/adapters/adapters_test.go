package adapters

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// readStateFromSnapshot creates a ReadState from a models.StateSnapshot for testing.
func readStateFromSnapshot(snapshot models.StateSnapshot) unifiedresources.ReadState {
	rr := unifiedresources.NewRegistry(nil)
	rr.IngestSnapshot(snapshot)
	return rr
}

func TestForecastDataAdapter_NilHistory(t *testing.T) {
	adapter := NewForecastDataAdapter(nil)
	if adapter != nil {
		t.Error("Expected nil adapter for nil history")
	}
}

func TestMetricsAdapter_GetCurrentMetrics_VM(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node/pve1", Name: "pve1", Instance: "inst1"},
		},
		VMs: []models.VM{
			{
				ID:       "qemu/100",
				VMID:     100,
				Name:     "webserver",
				Node:     "pve1",
				Instance: "inst1",
				CPU:      45.5,
				Memory: models.Memory{
					Usage: 72.3,
					Used:  1024,
					Total: 2048,
				},
				Disk: models.Disk{
					Usage: 55.0,
					Used:  5000,
					Total: 10000,
				},
				NetworkIn:  1024000,
				NetworkOut: 512000,
				DiskRead:   2048000,
				DiskWrite:  1024000,
			},
		},
	}

	rs := readStateFromSnapshot(state)
	adapter := NewMetricsAdapter(rs)

	// Get the unified resource ID from ReadState
	vms := rs.VMs()
	if len(vms) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(vms))
	}
	vmID := vms[0].ID()

	metrics, err := adapter.GetCurrentMetrics(vmID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if metrics["cpu"] != 45.5 {
		t.Errorf("Expected CPU 45.5, got %f", metrics["cpu"])
	}
	if metrics["memory"] != 72.3 {
		t.Errorf("Expected memory 72.3, got %f", metrics["memory"])
	}
	if metrics["disk"] != 55.0 {
		t.Errorf("Expected disk 55.0, got %f", metrics["disk"])
	}
	if metrics["netin"] != 1024000 {
		t.Errorf("Expected netin 1024000, got %f", metrics["netin"])
	}
}

func TestMetricsAdapter_GetCurrentMetrics_Container(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node/pve1", Name: "pve1", Instance: "inst1"},
		},
		Containers: []models.Container{
			{
				ID:       "lxc/101",
				VMID:     101,
				Name:     "container1",
				Node:     "pve1",
				Instance: "inst1",
				CPU:      25.0,
				Memory: models.Memory{
					Usage: 45.0,
					Used:  512,
					Total: 1024,
				},
				Disk: models.Disk{
					Usage: 30.0,
					Used:  3000,
					Total: 10000,
				},
				NetworkIn:  500000,
				NetworkOut: 250000,
				DiskRead:   1000000,
				DiskWrite:  500000,
			},
		},
	}

	rs := readStateFromSnapshot(state)
	adapter := NewMetricsAdapter(rs)

	containers := rs.Containers()
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	ctID := containers[0].ID()

	metrics, err := adapter.GetCurrentMetrics(ctID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if metrics["cpu"] != 25.0 {
		t.Errorf("Expected CPU 25.0, got %f", metrics["cpu"])
	}
	if metrics["memory"] != 45.0 {
		t.Errorf("Expected memory 45.0, got %f", metrics["memory"])
	}
}

func TestMetricsAdapter_GetCurrentMetrics_Node(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:       "node/pve1",
				Name:     "pve1",
				Instance: "inst1",
				CPU:      25.5,
				Memory: models.Memory{
					Usage: 65.0,
					Used:  6500,
					Total: 10000,
				},
				Disk: models.Disk{
					Usage: 40.0,
					Used:  4000,
					Total: 10000,
				},
			},
		},
	}

	rs := readStateFromSnapshot(state)
	adapter := NewMetricsAdapter(rs)

	nodes := rs.Nodes()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	nodeID := nodes[0].ID()

	metrics, err := adapter.GetCurrentMetrics(nodeID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if metrics["cpu"] != 25.5 {
		t.Errorf("Expected CPU 25.5, got %f", metrics["cpu"])
	}
	if metrics["memory"] != 65.0 {
		t.Errorf("Expected memory 65.0, got %f", metrics["memory"])
	}
	if metrics["disk"] != 40.0 {
		t.Errorf("Expected disk 40.0, got %f", metrics["disk"])
	}
}

func TestMetricsAdapter_GetCurrentMetrics_NodeByName(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:       "node/pve1",
				Name:     "pve1",
				Instance: "inst1",
				CPU:      25.5,
				Memory: models.Memory{
					Usage: 65.0,
					Used:  6500,
					Total: 10000,
				},
				Disk: models.Disk{
					Usage: 40.0,
					Used:  4000,
					Total: 10000,
				},
			},
		},
	}

	adapter := NewMetricsAdapter(readStateFromSnapshot(state))

	// Node lookup by name should still work
	metrics, err := adapter.GetCurrentMetrics("pve1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if metrics["cpu"] != 25.5 {
		t.Errorf("Expected CPU 25.5 when matching by name, got %f", metrics["cpu"])
	}
}

func TestMetricsAdapter_GetCurrentMetrics_Storage(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node/pve1", Name: "pve1", Instance: "inst1"},
		},
		Storage: []models.Storage{
			{
				ID:       "storage/local-zfs",
				Name:     "local-zfs",
				Node:     "pve1",
				Instance: "inst1",
				Used:     50000000000,
				Total:    100000000000,
				Usage:    50.0,
			},
		},
	}

	rs := readStateFromSnapshot(state)
	adapter := NewMetricsAdapter(rs)

	pools := rs.StoragePools()
	if len(pools) != 1 {
		t.Fatalf("expected 1 storage pool, got %d", len(pools))
	}
	storageID := pools[0].ID()

	metrics, err := adapter.GetCurrentMetrics(storageID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if metrics["disk"] != 50.0 {
		t.Errorf("Expected disk 50.0, got %f", metrics["disk"])
	}
	if metrics["used"] != 50000000000 {
		t.Errorf("Expected used 50000000000, got %f", metrics["used"])
	}
	if metrics["total"] != 100000000000 {
		t.Errorf("Expected total 100000000000, got %f", metrics["total"])
	}
}

func TestMetricsAdapter_GetCurrentMetrics_StorageByName(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node/pve1", Name: "pve1", Instance: "inst1"},
		},
		Storage: []models.Storage{
			{
				ID:       "storage/local-zfs",
				Name:     "local-zfs",
				Node:     "pve1",
				Instance: "inst1",
				Used:     50000000000,
				Total:    100000000000,
				Usage:    50.0,
			},
		},
	}

	adapter := NewMetricsAdapter(readStateFromSnapshot(state))

	// Storage lookup by name should still work
	metrics, err := adapter.GetCurrentMetrics("local-zfs")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if metrics["disk"] != 50.0 {
		t.Errorf("Expected disk 50.0, got %f", metrics["disk"])
	}
}

func TestMetricsAdapter_GetCurrentMetrics_NotFound(t *testing.T) {
	state := models.StateSnapshot{}

	adapter := NewMetricsAdapter(readStateFromSnapshot(state))

	metrics, err := adapter.GetCurrentMetrics("nonexistent")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(metrics) != 0 {
		t.Errorf("Expected empty metrics, got %d entries", len(metrics))
	}
}

func TestCommandExecutorAdapter_Disabled(t *testing.T) {
	adapter := NewCommandExecutorAdapter()

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	output, err := adapter.Execute(ctx, "pve1", "echo test")
	if err == nil {
		t.Error("Expected error for disabled command execution")
	}
	if output != "" {
		t.Errorf("Expected empty output, got '%s'", output)
	}

	// Verify error type
	_, ok := err.(*CommandExecutionDisabledError)
	if !ok {
		t.Errorf("Expected CommandExecutionDisabledError, got %T", err)
	}
}

func TestCommandExecutionDisabledError_Message(t *testing.T) {
	err := &CommandExecutionDisabledError{
		Target:  "pve1",
		Command: "test command",
	}

	msg := err.Error()
	if msg == "" {
		t.Error("Expected non-empty error message")
	}
	if msg != "command execution is disabled - commands must be run manually" {
		t.Errorf("Unexpected error message: %s", msg)
	}
}

func TestMetricsAdapter_NilReadState(t *testing.T) {
	adapter := NewMetricsAdapter(nil)
	if adapter != nil {
		t.Error("Expected nil adapter for nil ReadState")
	}
}

func TestMetricsAdapter_VMIDMatch(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node/pve1", Name: "pve1", Instance: "inst1"},
		},
		VMs: []models.VM{
			{
				ID:       "qemu/100",
				VMID:     100,
				Name:     "webserver",
				Node:     "pve1",
				Instance: "inst1",
				CPU:      45.5,
				Memory: models.Memory{
					Usage: 72.3,
					Used:  1024,
					Total: 2048,
				},
				Disk: models.Disk{
					Usage: 55.0,
					Used:  5000,
					Total: 10000,
				},
			},
		},
	}

	adapter := NewMetricsAdapter(readStateFromSnapshot(state))

	// Test lookup by VMID string
	metrics, err := adapter.GetCurrentMetrics(fmt.Sprintf("%d", 100))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if metrics["cpu"] != 45.5 {
		t.Errorf("Expected CPU 45.5 when matching by VMID, got %f", metrics["cpu"])
	}
}

func TestMetricsAdapter_IDConsistency(t *testing.T) {
	// Verify that GetMonitoredResourceIDs returns IDs that work with GetCurrentMetrics
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node/pve1", Name: "pve1", Instance: "inst1"},
		},
		VMs: []models.VM{
			{
				ID: "qemu/100", VMID: 100, Name: "vm1", Node: "pve1", Instance: "inst1",
				CPU: 50.0, Memory: models.Memory{Usage: 60.0, Used: 600, Total: 1000},
				Disk: models.Disk{Usage: 70.0, Used: 700, Total: 1000},
			},
		},
	}

	adapter := NewMetricsAdapter(readStateFromSnapshot(state))
	ids := adapter.GetMonitoredResourceIDs()
	if len(ids) < 1 {
		t.Fatalf("expected at least 1 ID, got %d", len(ids))
	}

	// Each ID from GetMonitoredResourceIDs should be usable with GetCurrentMetrics
	foundVM := false
	for _, id := range ids {
		metrics, err := adapter.GetCurrentMetrics(id)
		if err != nil {
			t.Errorf("GetCurrentMetrics(%q) error: %v", id, err)
			continue
		}
		if cpu, ok := metrics["cpu"]; ok && cpu == 50.0 {
			foundVM = true
		}
	}
	if !foundVM {
		t.Error("Expected to find VM metrics via GetMonitoredResourceIDs() IDs")
	}
}

func TestMetricsAdapter_SourceIDMatch(t *testing.T) {
	// Verify that GetCurrentMetrics can find resources by their Proxmox source ID
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node/pve1", Name: "pve1", Instance: "inst1"},
		},
		VMs: []models.VM{
			{
				ID: "qemu/100", VMID: 100, Name: "webserver", Node: "pve1", Instance: "inst1",
				CPU: 45.5, Memory: models.Memory{Usage: 72.3, Used: 1024, Total: 2048},
				Disk: models.Disk{Usage: 55.0, Used: 5000, Total: 10000},
			},
		},
		Storage: []models.Storage{
			{
				ID: "storage/local-zfs", Name: "local-zfs", Node: "pve1", Instance: "inst1",
				Used: 50000000000, Total: 100000000000, Usage: 50.0,
			},
		},
	}

	adapter := NewMetricsAdapter(readStateFromSnapshot(state))

	// Lookup VM by Proxmox source ID
	metrics, err := adapter.GetCurrentMetrics("qemu/100")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if metrics["cpu"] != 45.5 {
		t.Errorf("Expected CPU 45.5 via source ID, got %f", metrics["cpu"])
	}

	// Lookup storage by Proxmox source ID
	metrics, err = adapter.GetCurrentMetrics("storage/local-zfs")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if metrics["disk"] != 50.0 {
		t.Errorf("Expected disk 50.0 via source ID, got %f", metrics["disk"])
	}
}
