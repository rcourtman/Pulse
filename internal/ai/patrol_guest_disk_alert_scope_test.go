package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// guestDiskAlertFromManager has the real alert manager raise a disk alert for
// one filesystem of a Proxmox guest, and returns it as the monitor publishes it.
func guestDiskAlertFromManager(t *testing.T, guest any, instance string) models.Alert {
	t.Helper()
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	cfg := manager.GetConfig()
	cfg.Enabled = true
	cfg.TimeThresholds = map[string]int{"guest": 0, "node": 0, "storage": 0, "pbs": 0, "agent": 0}
	cfg.MetricTimeThresholds = map[string]map[string]int{"all": {"disk": 0}, "guest": {"disk": 0}}
	manager.UpdateConfig(cfg)

	deadline := time.Now().Add(20 * time.Second)
	for {
		manager.CheckGuest(guest, instance)
		for _, active := range manager.GetActiveAlerts() {
			if active.Type == "disk" && strings.Contains(active.ResourceID, "-disk-") {
				return models.Alert{
					ID: active.ID, Type: active.Type, Level: string(active.Level),
					ResourceID: active.ResourceID, ResourceName: active.ResourceName,
					Node: active.Node, Instance: active.Instance, Message: active.Message,
					Value: active.Value, Threshold: active.Threshold, StartTime: active.StartTime,
					Metadata: active.Metadata,
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the alert manager raised no filesystem alert, active: %+v", manager.GetActiveAlerts())
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// A Proxmox guest's filesystem alert names the filesystem
// ("<instance>:<node>:<vmid>-disk-<key>"), which is not a resource. The runs
// it starts must resolve to the guest that owns the filesystem, as a host
// filesystem alert resolves to its host.
func TestPatrolGuestFilesystemAlertScopeResolvesToItsGuest(t *testing.T) {
	for _, tc := range []struct {
		name         string
		resourceType string
		snapshot     func() (models.StateSnapshot, any)
	}{
		{"container", "system-container", func() (models.StateSnapshot, any) {
			guest := models.Container{
				ID: "pmxlab:pmxlab:111", VMID: 111, Name: "fill-ct", Node: "pmxlab", Instance: "pmxlab", Status: "running", Type: "lxc",
				Disk:  models.Disk{Total: 8 << 30, Used: 7 << 30, Free: 1 << 30, Usage: 91},
				Disks: []models.Disk{{Device: "local:111/vm-111-disk-0.raw", Total: 8 << 30, Used: 7 << 30, Free: 1 << 30, Usage: 91}},
			}
			return models.StateSnapshot{Containers: []models.Container{guest}}, guest
		}},
		{"virtual machine", "vm", func() (models.StateSnapshot, any) {
			guest := models.VM{
				ID: "pmxlab:pmxlab:120", VMID: 120, Name: "fill-vm", Node: "pmxlab", Instance: "pmxlab", Status: "running", Type: "qemu",
				Disk:  models.Disk{Total: 8 << 30, Used: 7 << 30, Free: 1 << 30, Usage: 91},
				Disks: []models.Disk{{Mountpoint: "/var", Device: "/dev/sda2", Total: 8 << 30, Used: 7 << 30, Free: 1 << 30, Usage: 91}},
			}
			return models.StateSnapshot{VMs: []models.VM{guest}}, guest
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot, guest := tc.snapshot()
			snapshot.Nodes = []models.Node{{ID: "pmxlab-pmxlab", Name: "pmxlab", Instance: "pmxlab", Status: "online"}}
			alert := guestDiskAlertFromManager(t, guest, "pmxlab")
			t.Logf("alert %s on resource %s", alert.ID, alert.ResourceID)
			resolvedAlert := alert
			snapshot.ActiveAlerts = []models.Alert{alert}
			snapshot.RecentlyResolved = []models.ResolvedAlert{{Alert: resolvedAlert, ResolvedTime: time.Now()}}

			registry := unifiedresources.NewRegistry(nil)
			registry.IngestSnapshot(snapshot)
			state := newPatrolRuntimeStateWithProviders(snapshot, registry, unifiedresources.NewUnifiedAIAdapter(registry))
			var guestID string
			for _, resource := range unifiedresources.NewUnifiedAIAdapter(registry).GetByType(unifiedresources.ResourceType(tc.resourceType)) {
				guestID = resource.ID
			}
			if guestID == "" {
				t.Fatalf("inventory has no %s", tc.resourceType)
			}

			for _, scope := range []PatrolScope{
				AlertTriggeredPatrolScope(alert.ID, alert.ResourceID, tc.resourceType, alert.Type),
				// The flapping trigger passes the alert kind as the type.
				FlappingPostmortemPatrolScope(alert.ID, alert.ResourceID, "metric-threshold", alert.Type),
				AlertClearedPatrolScope(alert.ID, alert.ResourceID, tc.resourceType),
			} {
				resolved, resolution := resolvePatrolScopeState(state, scope)
				if len(resolution.UnmatchedResourceIDs)+len(resolution.AmbiguousResourceIDs) != 0 || len(resolution.EffectiveResourceIDs) == 0 {
					t.Errorf("%s: scope did not resolve: unmatched=%v ambiguous=%v effective=%v",
						scope.Reason, resolution.UnmatchedResourceIDs, resolution.AmbiguousResourceIDs, resolution.EffectiveResourceIDs)
					continue
				}
				if !containsScopeID(resolution.ResolvedResourceIDs, guestID) {
					t.Errorf("%s: resolved to %v, want the guest %s", scope.Reason, resolution.ResolvedResourceIDs, guestID)
				}
				if len(resolved.ResourceTypes) != 1 || resolved.ResourceTypes[0] != tc.resourceType {
					t.Errorf("%s: scope types = %v, want [%s]", scope.Reason, resolved.ResourceTypes, tc.resourceType)
				}
				if resolved.AlertContext == nil || resolved.AlertContext.ResourceID != alert.ResourceID {
					t.Errorf("%s: lost the alert's own context: %+v", scope.Reason, resolved.AlertContext)
				}
				filtered := filterPatrolStateByScopeState(state, resolved)
				if resources := filtered.unifiedResourceProvider.GetAll(); len(resources) == 0 {
					t.Errorf("%s: the scoped state holds no resource", scope.Reason)
				}
			}

			// What else reads the same binding: the alert review no longer
			// takes the guest for removed, and the inbox names the guest.
			if !patrolAlertSubjectInInventory(AlertInfo{ResourceID: alert.ResourceID}, state) {
				t.Error("the alert review would describe this alert's guest as not found")
			}
			if got := AttentionSubjectResourceID(registry, alert.ResourceID); got != guestID {
				t.Errorf("attention subject = %q, want the guest %s", got, guestID)
			}

			// A reference that only looks like a filesystem of this guest is not
			// an alert, and a caller cannot use the shape to pick a scope.
			_, resolution := resolvePatrolScopeState(state, PatrolScope{ResourceIDs: []string{alert.ResourceID}, Reason: TriggerReasonManual})
			if len(resolution.UnmatchedResourceIDs) != 1 {
				t.Errorf("a filesystem reference without its alert resolved: %+v", resolution)
			}
		})
	}
}
