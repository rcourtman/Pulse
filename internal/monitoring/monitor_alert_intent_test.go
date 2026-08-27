package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestInstallOperatorIntentResolverProjectsCanonicalResourcePolicy(t *testing.T) {
	store := unifiedresources.NewMemoryStore()
	registry := unifiedresources.NewRegistry(store)
	registry.IngestResources([]unifiedresources.Resource{{
		ID: "vm:101", Type: unifiedresources.ResourceTypeVM, Name: "database",
	}})
	if err := store.SetResourceOperatorState(unifiedresources.ResourceOperatorState{
		CanonicalID:    "vm:101",
		MonitoringMode: unifiedresources.MonitoringModeMuted,
		LifecycleState: unifiedresources.LifecycleStateRetired,
	}); err != nil {
		t.Fatal(err)
	}

	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	monitor := &Monitor{alertManager: manager}
	monitor.installOperatorIntentResolver(unifiedresources.NewMonitorAdapter(registry))
	preview, err := manager.PreviewIntentPolicy(alerts.AlertIntentPolicyPreviewRequest{
		ResourceID: "vm:101", ResourceType: "vm", Signal: string(alerts.AlertIntentSignalOffline), ConditionActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Reason != "operator_retired" || preview.Status != "expected_transient" {
		t.Fatalf("canonical operator policy preview = %+v", preview)
	}
}

func TestInstallOperatorIntentResolverInheritsScopedMaintenanceFromParent(t *testing.T) {
	store := unifiedresources.NewMemoryStore()
	registry := unifiedresources.NewRegistry(store)
	parentID := "node:pve-a"
	registry.IngestResources([]unifiedresources.Resource{
		{ID: parentID, Type: unifiedresources.ResourceTypeAgent, Name: "pve-a"},
		{ID: "vm:101", Type: unifiedresources.ResourceTypeVM, Name: "database", ParentID: &parentID},
	})
	now := time.Now().UTC()
	start, end := now.Add(-time.Hour), now.Add(2*time.Hour)
	if err := store.SetResourceOperatorState(unifiedresources.ResourceOperatorState{
		CanonicalID:        parentID,
		MaintenanceStartAt: &start,
		MaintenanceEndAt:   &end,
		MaintenanceReason:  "hypervisor patching",
		MaintenanceScope:   unifiedresources.MaintenanceScopeResourceAndDescendants,
	}); err != nil {
		t.Fatal(err)
	}

	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	monitor := &Monitor{alertManager: manager}
	monitor.installOperatorIntentResolver(unifiedresources.NewMonitorAdapter(registry))
	preview, err := manager.PreviewIntentPolicy(alerts.AlertIntentPolicyPreviewRequest{
		ResourceID: "vm:101", ResourceType: "vm", Signal: string(alerts.AlertIntentSignalOffline), ConditionActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Reason != "operator_maintenance" || preview.Status != "expected_transient" {
		t.Fatalf("inherited maintenance preview = %+v", preview)
	}
	if preview.EligibleAt == nil || !preview.EligibleAt.Equal(end) {
		t.Fatalf("eligibleAt = %v, want %v", preview.EligibleAt, end)
	}

	// Scope is explicit: the same parent window must not leak to descendants
	// when changed back to resource-only.
	if err := store.SetResourceOperatorState(unifiedresources.ResourceOperatorState{
		CanonicalID:        parentID,
		MaintenanceStartAt: &start,
		MaintenanceEndAt:   &end,
		MaintenanceScope:   unifiedresources.MaintenanceScopeResource,
	}); err != nil {
		t.Fatal(err)
	}
	preview, err = manager.PreviewIntentPolicy(alerts.AlertIntentPolicyPreviewRequest{
		ResourceID: "vm:101", ResourceType: "vm", Signal: string(alerts.AlertIntentSignalOffline), ConditionActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Status != "would_activate" {
		t.Fatalf("resource-only parent window leaked to child: %+v", preview)
	}
}

func TestPlatformMonitorResourceIdentityConstructors(t *testing.T) {
	if got := PBSMonitorResourceID("backup-main"); got != "pbs-backup-main" {
		t.Fatalf("PBS monitor resource ID = %q", got)
	}
	if got := PMGMonitorResourceID("mail-main"); got != "pmg-mail-main" {
		t.Fatalf("PMG monitor resource ID = %q", got)
	}
}

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

// A guest covered by a running multi-guest vzdump job has no task of its own;
// pollBackupTasks synthesizes one from the job log. That synthetic task must
// count as backup-intent evidence so the guest's alerts are suppressed while
// the job is backing it up, and must stop counting once its section finishes.
func TestResolveBackupIntentContextAcceptsSynthesizedJobGuestTask(t *testing.T) {
	now := time.Date(2026, 7, 25, 2, 5, 0, 0, time.UTC)
	jobTask := models.BackupTask{
		ID:        "pve-a-UPID:node-a:000E9F2C:0AC734B2:68A1B2C3:vzdump::root@pam:",
		Node:      "node-a",
		Instance:  "pve-a",
		Type:      "vzdump",
		StartTime: now.Add(-4 * time.Minute),
		// no EndTime: the job is still running
	}

	synthesized := parseVzdumpJobLog(jobTask, "UPID:node-a:000E9F2C:0AC734B2:68A1B2C3:vzdump::root@pam:", []proxmox.TaskLogLine{
		{LineNumber: 1, Text: "INFO: Starting Backup of VM 101 (qemu)"},
		{LineNumber: 2, Text: "INFO: Finished Backup of VM 101 (00:01:30)"},
		{LineNumber: 3, Text: "INFO: Starting Backup of VM 102 (lxc)"},
	})
	for i := range synthesized {
		synthesized[i].ObservedAt = now.Add(-30 * time.Second)
	}

	state := models.NewState()
	state.UpdateBackupTasksForInstance("pve-a", append([]models.BackupTask{jobTask}, synthesized...))
	monitor := &Monitor{state: state}

	context, found := monitor.resolveBackupIntentContext("", "pve-a", "node-a", 102, now)
	if !found || !context.Active {
		t.Fatalf("guest being backed up by running job did not resolve: found=%v context=%+v", found, context)
	}

	if got, ok := monitor.resolveBackupIntentContext("", "pve-a", "node-a", 101, now); ok {
		t.Fatalf("guest whose job section already finished should not carry intent: %+v", got)
	}
}
