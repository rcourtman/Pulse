package adapters

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// The batch method must return exactly what per-ID lookups return for every
// ID form the per-ID method matches, including colliding VMID strings where
// the VM -> container precedence decides the winner.
func TestGetCurrentMetricsBatchMatchesPerIDLookups(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node/pve1", Name: "pve1", Instance: "inst1", CPU: 0.35, Memory: models.Memory{Usage: 60}},
		},
		VMs: []models.VM{
			{
				ID: "qemu/100", VMID: 100, Name: "webserver", Node: "pve1", Instance: "inst1",
				CPU: 45.5, Memory: models.Memory{Usage: 72.3}, Disk: models.Disk{Usage: 55},
				NetworkIn: 1024, NetworkOut: 512, DiskRead: 2048, DiskWrite: 1024,
			},
		},
		Containers: []models.Container{
			{
				ID: "lxc/104", VMID: 104, Name: "auth", Node: "pve1", Instance: "inst1",
				CPU: 12.5, Memory: models.Memory{Usage: 30}, Disk: models.Disk{Usage: 20},
			},
		},
		Storage: []models.Storage{
			{ID: "storage/local", Name: "local", Node: "pve1", Instance: "inst1", Usage: 41, Used: 41, Total: 100},
		},
	}

	adapter := NewMetricsAdapter(readStateFromSnapshot(state))
	batch := adapter.GetCurrentMetricsBatch()
	if len(batch) == 0 {
		t.Fatal("batch returned no entries for a populated state")
	}

	for id := range batch {
		perID, err := adapter.GetCurrentMetrics(id)
		if err != nil {
			t.Fatalf("GetCurrentMetrics(%q) error: %v", id, err)
		}
		if !reflect.DeepEqual(batch[id], perID) {
			t.Fatalf("metrics diverged for %q:\nbatch:  %+v\nper-ID: %+v", id, batch[id], perID)
		}
	}

	// Every monitored ID must be resolvable through the batch.
	for _, id := range adapter.GetMonitoredResourceIDs() {
		if _, ok := batch[id]; !ok {
			t.Fatalf("monitored ID %q missing from batch", id)
		}
	}
}

func TestGetCurrentMetricsBatchVMIDCollisionPrefersVM(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node/pve1", Name: "pve1", Instance: "inst1"},
		},
		VMs: []models.VM{
			{ID: "qemu/200", VMID: 200, Name: "vm-two-hundred", Node: "pve1", Instance: "inst1", CPU: 80},
		},
		Containers: []models.Container{
			{ID: "lxc/200", VMID: 200, Name: "ct-two-hundred", Node: "pve1", Instance: "inst1", CPU: 10},
		},
	}

	adapter := NewMetricsAdapter(readStateFromSnapshot(state))
	batch := adapter.GetCurrentMetricsBatch()
	perID, _ := adapter.GetCurrentMetrics(strconv.Itoa(200))
	if !reflect.DeepEqual(batch["200"], perID) {
		t.Fatalf("VMID collision winner diverged:\nbatch:  %+v\nper-ID: %+v", batch["200"], perID)
	}
}
