package ai

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestIssue1597PatrolGuestFallbackUsesCanonicalProxmoxCPUPercent(t *testing.T) {
	rows := patrolGuestInventoryRows(patrolRuntimeState{
		VMs: []models.VM{{
			ID:   "vm-301",
			Name: "vm-301",
			CPU:  0.0058,
			CPUs: 8,
		}},
		Containers: []models.Container{{
			ID:   "ct-302",
			Name: "ct-302",
			Type: "lxc",
			CPU:  0.0058,
			CPUs: 1,
		}},
	}, nil, nil)

	if len(rows) != 2 {
		t.Fatalf("guest rows = %d, want VM and LXC: %+v", len(rows), rows)
	}
	for _, row := range rows {
		if row.cpu != 0.58 {
			t.Fatalf("%s CPU = %v, want canonical proxmox 0.58", row.id, row.cpu)
		}
	}
}

func TestGuestCPUUnavailableIsNotIdleInPatrolInputs(t *testing.T) {
	for _, cpu := range []float64{-1, 0, .98} {
		snapshot := models.StateSnapshot{VMs: []models.VM{{ID: "lab:node:106", VMID: 106, Name: "vm", Type: "qemu", CPU: cpu}}, Containers: []models.Container{{ID: "lab:node:107", VMID: 107, Name: "ct", Type: "lxc", CPU: cpu}}}
		adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
		adapter.PopulateFromSnapshot(snapshot)
		for name, state := range map[string]patrolRuntimeState{"legacy": {VMs: snapshot.VMs, Containers: snapshot.Containers}, "read-state": {readState: adapter}} {
			rows := patrolGuestInventoryRows(state, nil, nil)
			if len(rows) != 2 {
				t.Fatalf("%s lost guests: %+v", name, rows)
			}
			want := cpu * 100
			if cpu < 0 {
				want = -1
			}
			for _, row := range rows {
				if row.cpu != want {
					t.Fatalf("%s CPU input=%v want %v", name, row.cpu, want)
				}
				label := patrolGuestCPUDisplay(row.cpu)
				if cpu < 0 && label != "unavailable" {
					t.Fatalf("unavailable CPU rendered as %s", label)
				}
				if cpu == 0 && label != "0%" {
					t.Fatal("measured zero lost")
				}
			}
		}
	}
}
