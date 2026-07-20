package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestResolveBackupIntentContextRequiresFreshActiveMatchingEvidence(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	state := models.NewState()
	state.UpdateBackupTasksForInstance("pve-a", []models.BackupTask{
		{
			ID:         "active-101",
			Instance:   "pve-a",
			Node:       "node-a",
			VMID:       101,
			Status:     "running",
			ObservedAt: now.Add(-time.Minute),
		},
		{
			ID:         "stale-102",
			Instance:   "pve-a",
			Node:       "node-a",
			VMID:       102,
			Status:     "running",
			ObservedAt: now.Add(-backupIntentEvidenceMaxAge - time.Second),
		},
		{
			ID:         "finished-103",
			Instance:   "pve-a",
			Node:       "node-a",
			VMID:       103,
			Status:     "OK",
			ObservedAt: now.Add(-time.Minute),
			EndTime:    now.Add(-30 * time.Second),
		},
	})

	monitor := &Monitor{state: state}
	context, found := monitor.resolveBackupIntentContext("", "pve-a", "node-a", 101, now)
	if !found || !context.Active {
		t.Fatalf("fresh active task did not resolve: found=%v context=%+v", found, context)
	}
	if context.ObservedAt != now.Add(-time.Minute) {
		t.Fatalf("observedAt = %v, want %v", context.ObservedAt, now.Add(-time.Minute))
	}
	if context.Evidence != "pve_vzdump_task:active-101" {
		t.Fatalf("evidence = %q, want active task identity", context.Evidence)
	}

	for _, tc := range []struct {
		name     string
		instance string
		node     string
		vmid     int
	}{
		{name: "wrong instance", instance: "pve-b", node: "node-a", vmid: 101},
		{name: "wrong node", instance: "pve-a", node: "node-b", vmid: 101},
		{name: "stale", instance: "pve-a", node: "node-a", vmid: 102},
		{name: "finished", instance: "pve-a", node: "node-a", vmid: 103},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, ok := monitor.resolveBackupIntentContext("", tc.instance, tc.node, tc.vmid, now); ok {
				t.Fatalf("unexpected backup intent context: %+v", got)
			}
		})
	}
}
