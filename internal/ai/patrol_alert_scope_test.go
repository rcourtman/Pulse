package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Control: selecting a known owner must preserve the exact triggering child
// alert. This exposes why a string-prefix rewrite alone is not a complete fix.
func TestSupportHostDiskOwnerScopeRetainsTrigger(t *testing.T) {
	host := models.Host{ID: "repro-host-source-01", Hostname: "docker-host-ext-01", MachineID: "repro-machine-01", Disks: []models.Disk{{Mountpoint: "/docker-data", Usage: 85}}}
	alert := models.Alert{ID: "repro-disk-alert", ResourceID: "agent:repro-host-source-01/disk:docker-data", Type: "disk", Metadata: map[string]interface{}{"hostId": host.ID, "mountpoint": "/docker-data"}}
	snapshot := models.StateSnapshot{Hosts: []models.Host{host}, ActiveAlerts: []models.Alert{alert}}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	for _, unified := range []bool{false, true} {
		name := "snapshot"
		state := newPatrolRuntimeState(snapshot)
		parentID := host.ID
		if unified {
			name = "unified"
			state = newPatrolRuntimeStateWithProviders(snapshot, registry, unifiedresources.NewUnifiedAIAdapter(registry))
			_, id, found := registry.GetByReference(host.ID)
			if !found {
				t.Fatal("canonical owner missing")
			}
			parentID = id
		}
		t.Run(name, func(t *testing.T) {
			resolved, resolution := resolvePatrolScopeState(state, PatrolScope{ResourceIDs: []string{parentID}, AlertIdentifier: alert.ID})
			if len(resolution.UnmatchedResourceIDs) != 0 || len(resolution.EffectiveResourceIDs) == 0 {
				t.Fatalf("owner control did not resolve: %+v", resolution)
			}
			filtered := filterPatrolStateByScopeState(state, resolved)
			t.Logf("resolved=%+v effective=%v hosts=%d active_alerts=%d", resolved, resolution.EffectiveResourceIDs, len(filtered.Hosts), len(filtered.ActiveAlerts))
			if len(filtered.ActiveAlerts) != 1 || filtered.ActiveAlerts[0].ID != alert.ID {
				t.Fatalf("owner scope discarded the exact disk alert despite AlertIdentifier=%q", alert.ID)
			}
		})
	}
}

func TestPatrolDiskAlertRecurrenceUsesCurrentEvidence(t *testing.T) {
	active := models.Alert{ID: "disk-alert", ResourceID: "agent:source-host/disk:data", Type: "disk", Value: 85, Threshold: 85}
	old := active
	old.Value = 95
	latest := active
	latest.Value = 75
	state := newPatrolRuntimeState(models.StateSnapshot{Hosts: []models.Host{{ID: "source-host", MachineID: "machine"}}, ActiveAlerts: []models.Alert{active}, RecentlyResolved: []models.ResolvedAlert{{Alert: old, ResolvedTime: time.Unix(1, 0)}, {Alert: latest, ResolvedTime: time.Unix(2, 0)}}})
	for _, tc := range []struct {
		reason TriggerReason
		value  float64
	}{{TriggerReasonManual, 85}, {TriggerReasonAlertCleared, 75}} {
		resolved, resolution := resolvePatrolScopeState(state, PatrolScope{ResourceIDs: []string{active.ResourceID}, AlertIdentifier: active.ID, Reason: tc.reason})
		if len(resolution.EffectiveResourceIDs) == 0 || resolved.AlertContext == nil || resolved.AlertContext.Value != tc.value {
			t.Fatalf("wrong occurrence evidence for %s: %+v, %+v", tc.reason, resolution, resolved.AlertContext)
		}
	}
}

func TestPatrolDiskAlertOwnerAndEvidence(t *testing.T) {
	for _, ownerType := range []string{"agent", "vm", "system-container", "node"} {
		t.Run(ownerType, func(t *testing.T) {
			host := models.Host{ID: "disk-host-source", Hostname: "disk-host", MachineID: "disk-machine"}
			alert := models.Alert{ID: "disk-alert", ResourceID: "agent:disk-host-source/disk:docker-data", ResourceName: "disk-host (/docker-data)", Type: "disk", Level: "warning", Value: 85, Threshold: 85, Message: "Disk usage reached 85%", Metadata: map[string]interface{}{"mountpoint": "/docker-data", "device": "/dev/sdb1"}}
			unrelated := models.Alert{ID: "other-alert", ResourceID: "agent:other-host/disk:docker-data", Type: "disk"}
			snapshot := models.StateSnapshot{Hosts: []models.Host{host, {ID: "other-host", MachineID: "other-machine"}}, ActiveAlerts: []models.Alert{alert, unrelated}, RecentlyResolved: []models.ResolvedAlert{{Alert: alert}, {Alert: unrelated}}}
			// Keep this occurrence distinct; ID reuse is covered separately above.
			snapshot.RecentlyResolved[0].ID = "resolved-disk-alert"
			ownerSourceID := host.ID
			switch ownerType {
			case "vm":
				ownerSourceID = "vm-owner"
				snapshot.VMs = []models.VM{{ID: ownerSourceID, VMID: 101, Instance: "pve", Node: "node-owner", Name: "linked-vm"}}
			case "system-container":
				ownerSourceID = "ct-owner"
				snapshot.Containers = []models.Container{{ID: ownerSourceID, VMID: 102, Instance: "pve", Node: "node-owner", Name: "linked-container"}}
			case "node":
				ownerSourceID = "node-owner"
				snapshot.Nodes = []models.Node{{ID: ownerSourceID, Name: "linked-node", Instance: "pve"}}
			}
			store := unifiedresources.NewMemoryStore()
			registry := unifiedresources.NewRegistry(store)
			registry.IngestSnapshot(snapshot)
			_, hostID, hostFound := registry.GetByReference(host.ID)
			_, ownerID, ownerFound := registry.GetByReference(ownerSourceID)
			if !hostFound || !ownerFound {
				t.Fatal("fixture owner missing")
			}
			if ownerType != "agent" {
				if err := store.AddLink(unifiedresources.ResourceLink{ResourceA: hostID, ResourceB: ownerID, PrimaryID: ownerID}); err != nil {
					t.Fatal(err)
				}
				registry = unifiedresources.NewRegistry(store)
				registry.IngestSnapshot(snapshot)
			}
			state := newPatrolRuntimeStateWithProviders(snapshot, registry, unifiedresources.NewUnifiedAIAdapter(registry))
			for _, scope := range []PatrolScope{
				{ResourceIDs: []string{alert.ResourceID}, AlertIdentifier: alert.ID, Reason: TriggerReasonManual},
				AlertTriggeredPatrolScope(alert.ID, alert.ResourceID, "agent", "disk"),
				AlertClearedPatrolScope("resolved-disk-alert", alert.ResourceID, "agent"),
			} {
				resolved, resolution := resolvePatrolScopeState(state, scope)
				if len(resolution.UnmatchedResourceIDs)+len(resolution.AmbiguousResourceIDs) != 0 || len(resolution.EffectiveResourceIDs) == 0 {
					t.Fatalf("%s admission failed: %+v", scope.Reason, resolution)
				}
				// Admission and execution each resolve against current inventory.
				resolved, resolution = resolvePatrolScopeState(state, resolved)
				if len(resolution.UnmatchedResourceIDs)+len(resolution.AmbiguousResourceIDs) != 0 || len(resolution.EffectiveResourceIDs) == 0 {
					t.Fatalf("%s execution failed: %+v", scope.Reason, resolution)
				}
				filtered := filterPatrolStateByScopeState(state, resolved)
				if len(filtered.ActiveAlerts) != 1 || filtered.ActiveAlerts[0].ResourceID != alert.ResourceID || len(filtered.RecentlyResolved) != 1 {
					t.Fatalf("lost subject or leaked neighbor evidence: %+v", filtered.ActiveAlerts)
				}
				if resource := filtered.unifiedResourceProvider.GetAll(); len(resource) != 1 || resource[0].ID != ownerID {
					t.Fatalf("scope expanded beyond canonical owner: %+v", resource)
				}
				if resolved.AlertContext == nil || resolved.AlertContext.ResourceID != alert.ResourceID || resolved.AlertContext.Value != 85 || resolved.AlertContext.Threshold != 85 {
					t.Fatalf("lost trusted alert evidence: %+v", resolved.AlertContext)
				}
				prompt := buildScopeSection(&resolved, resolution.EffectiveResourceIDs)
				for _, evidence := range []string{alert.ResourceID, "/docker-data", "/dev/sdb1", "value 85.0, threshold 85.0", alert.Message} {
					if !strings.Contains(prompt, evidence) {
						t.Fatalf("missing %q in prompt: %s", evidence, prompt)
					}
				}
			}
		})
	}
}

func TestPatrolDiskAlertRejectsUntrustedSubjects(t *testing.T) {
	for _, scenario := range []string{"missing-alert", "wrong-alert", "duplicate-alert", "wrong-subject", "removed-owner", "display-name-owner", "malformed-child", "source-collision"} {
		t.Run(scenario, func(t *testing.T) {
			alert := models.Alert{ID: "disk-alert", ResourceID: "agent:disk-host-source/disk:docker-data", Type: "disk"}
			snapshot := models.StateSnapshot{Hosts: []models.Host{{ID: "disk-host-source", Hostname: "disk-host", MachineID: "machine"}}, ActiveAlerts: []models.Alert{alert}}
			scope := PatrolScope{ResourceIDs: []string{alert.ResourceID}, AlertIdentifier: alert.ID, AlertContext: &PatrolAlertContext{Message: "caller-authored"}}
			switch scenario {
			case "missing-alert":
				snapshot.ActiveAlerts = nil
			case "wrong-alert":
				scope.AlertIdentifier = "different-alert"
			case "duplicate-alert":
				snapshot.ActiveAlerts = append(snapshot.ActiveAlerts, alert)
			case "wrong-subject":
				scope.ResourceIDs[0] = "agent:disk-host-source/disk:other"
			case "removed-owner":
				snapshot.Hosts = nil
			case "display-name-owner":
				snapshot.ActiveAlerts[0].ResourceID = "agent:disk-host/disk:docker-data"
				scope.ResourceIDs[0] = snapshot.ActiveAlerts[0].ResourceID
			case "malformed-child":
				snapshot.ActiveAlerts[0].ResourceID += "/nested"
				scope.ResourceIDs[0] = snapshot.ActiveAlerts[0].ResourceID
			case "source-collision":
				snapshot.Nodes = []models.Node{{ID: "disk-host-source", Name: "unrelated-node", Instance: "pve"}}
			}
			resolved, resolution := resolvePatrolScopeState(newPatrolRuntimeState(snapshot), scope)
			if len(resolution.UnmatchedResourceIDs)+len(resolution.AmbiguousResourceIDs) != 1 || len(resolution.EffectiveResourceIDs) != 0 || resolved.AlertContext != nil {
				t.Fatalf("unsafe scope accepted: %+v, context=%+v", resolution, resolved.AlertContext)
			}
		})
	}
}
