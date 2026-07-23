package ai

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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
