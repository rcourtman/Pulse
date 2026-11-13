package monitoring

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Check the real detector/intent/callback path, not just reference resolution.
// These are safe inventory fixtures; no node is rebooted and no datastore is
// changed. In particular, the datastore keeps its original alert subject.
func TestMaintenanceAlertSubjectsRespectWindowsAndResumeNotifications(t *testing.T) {
	for _, subject := range []string{"pve:site-a", "pbs:backup-a", "pbs-backup-a-main"} {
		t.Run(subject, func(t *testing.T) {
			now := time.Now().UTC()
			store := unifiedresources.NewMemoryStore()
			registry := unifiedresources.NewRegistry(store)
			registry.IngestSnapshot(models.StateSnapshot{
				Nodes: []models.Node{{ID: "site-a-node1", Instance: "site-a", Name: "node1", LinkedAgentID: "host-a", Status: "online", LastSeen: now}},
				Hosts: []models.Host{{ID: "host-a", Hostname: "node1", MachineID: "machine-a", Status: "online", LastSeen: now}},
				PBSInstances: []models.PBSInstance{{ID: "pbs-backup-a", Name: "backup-a", Host: "https://backup-a.invalid:8007", Status: "online", LastSeen: now,
					Datastores: []models.PBSDatastore{{Name: "main", Status: "unavailable"}}}},
			})
			ownerRef, resourceType := "agent:host-a", "connection"
			if subject != "pve:site-a" {
				ownerRef = "pbs-backup-a"
			}
			if subject == "pbs-backup-a-main" {
				resourceType = "storage"
			}
			ownerID, ok := registry.ResolveReferenceID(ownerRef)
			if !ok {
				t.Fatalf("fixture owner %q missing", ownerRef)
			}
			setWindow := func(start, end time.Time, scope unifiedresources.MaintenanceScope) {
				t.Helper()
				if err := store.SetResourceOperatorState(unifiedresources.ResourceOperatorState{
					CanonicalID: ownerID, MaintenanceStartAt: &start, MaintenanceEndAt: &end,
					MaintenanceScope: scope, MaintenanceReason: "planned reboot",
				}); err != nil {
					t.Fatal(err)
				}
			}
			for _, phase := range []struct {
				name       string
				start, end time.Time
				scope      unifiedresources.MaintenanceScope
				suppressed bool
			}{
				{"future", now.Add(time.Hour), now.Add(2 * time.Hour), unifiedresources.MaintenanceScopeResourceAndDescendants, false},
				{"active", now.Add(-time.Hour), now.Add(time.Hour), unifiedresources.MaintenanceScopeResourceAndDescendants, true},
				{"expired", now.Add(-2 * time.Hour), now.Add(-time.Hour), unifiedresources.MaintenanceScopeResourceAndDescendants, false},
				{"resource-only", now.Add(-time.Hour), now.Add(time.Hour), unifiedresources.MaintenanceScopeResource, subject != "pbs-backup-a-main"},
			} {
				t.Run(phase.name, func(t *testing.T) {
					setWindow(phase.start, phase.end, phase.scope)
					manager := alerts.NewManagerWithDataDir(t.TempDir())
					t.Cleanup(manager.Stop)
					manager.UpdateConfig(alerts.AlertConfig{Enabled: true, ActivationState: alerts.ActivationActive, TimeThresholds: map[string]int{"storage": 0}})
					fired := make(chan *alerts.Alert, 8)
					manager.SetAlertCallback(func(a *alerts.Alert) { fired <- a.Clone() })
					monitor := &Monitor{alertManager: manager}
					monitor.installOperatorIntentResolver(unifiedresources.NewMonitorAdapter(registry))
					observe := func(healthy bool) {
						if resourceType == "storage" {
							status := "unavailable"
							if healthy {
								status = "available"
							}
							manager.CheckStorage(models.Storage{ID: subject, AliasIDs: []string{"pbs-backup-a/main"}, Instance: "pbs-backup-a", Node: "backup-a", Name: "main", Type: "pbs", Status: status})
						} else {
							kind := alerts.ConnectionTypePVE
							if subject == "pbs:backup-a" {
								kind = alerts.ConnectionTypePBS
							}
							state := alerts.ConnectionStateUnreachable
							if healthy {
								state = alerts.ConnectionStateActive
							}
							manager.CheckConnection(alerts.ConnectionSnapshot{ID: subject, PolicyResourceID: ownerRef, Name: subject, Type: kind, State: state, Enabled: true})
						}
					}
					preview, err := manager.PreviewIntentPolicy(alerts.AlertIntentPolicyPreviewRequest{ResourceID: subject, ResourceType: resourceType, Signal: string(alerts.AlertIntentSignalOffline), ConditionActive: true})
					if err != nil {
						t.Fatal(err)
					}
					for range 5 {
						observe(false)
					}
					active := manager.GetActiveAlerts()
					// Retain actual producer output for the consuming-surface proof.
					encoded, err := json.Marshal(map[string]any{"subject": subject, "phase": phase.name, "preview": preview, "alerts": active, "resources": registry.List()})
					if err != nil {
						t.Fatal(err)
					}
					t.Logf("MAINTENANCE_SUBJECT_FIXTURE %s", encoded)
					if phase.suppressed {
						if len(active) != 0 || preview.Reason != "operator_maintenance" || preview.EligibleAt == nil || !preview.EligibleAt.Equal(phase.end) {
							t.Errorf("window bypassed: active=%+v, preview=%+v", active, preview)
						}
						select {
						case a := <-fired:
							t.Errorf("notification dispatched inside window: %+v", a)
						default:
						}
						// Expiration is not a permanent mute. The next observations
						// of the same outage must notify with the same subject/key.
						setWindow(now.Add(-2*time.Hour), now.Add(-time.Hour), phase.scope)
						for range 5 {
							observe(false)
						}
						active = manager.GetActiveAlerts()
					}
					if len(active) != 1 || active[0].ResourceID != subject {
						t.Fatalf("post-window incident = %+v, want subject %q", active, subject)
					}
					incident := active[0]
					select {
					case a := <-fired:
						if a.ID != incident.ID || a.ResourceID != subject {
							t.Errorf("callback changed occurrence identity: %+v", a)
						}
					case <-time.After(3 * time.Second):
						t.Fatal("post-window notification callback missing")
					}
					// More failure evidence neither duplicates nor renames it.
					for range 3 {
						observe(false)
					}
					if current := manager.GetActiveAlerts(); len(current) != 1 || current[0].ID != incident.ID || !current[0].StartTime.Equal(incident.StartTime) {
						t.Errorf("outage identity changed: %+v", current)
					}
					for range 5 {
						observe(true)
					}
					if got := manager.GetActiveAlerts(); len(got) != 0 {
						t.Errorf("healthy recovery left incidents: %+v", got)
					}
					if resolved := manager.GetResolvedAlert(incident.ID); resolved == nil || resolved.ResourceID != subject {
						t.Errorf("recovery lost original occurrence/history: %+v", resolved)
					}
				})
			}
		})
	}
}

func TestMaintenanceAlertSubjectsKeepUnrelatedResourcesAndRecurringScope(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	store := unifiedresources.NewMemoryStore()
	registry := unifiedresources.NewRegistry(store)
	snapshot := models.StateSnapshot{}
	for _, site := range []string{"a", "b"} {
		snapshot.Nodes = append(snapshot.Nodes, models.Node{ID: "site-" + site + "-node1", Instance: "site-" + site, Name: "node1", LinkedAgentID: "host-" + site, Status: "online", LastSeen: now})
		snapshot.Hosts = append(snapshot.Hosts, models.Host{ID: "host-" + site, Hostname: "node1", MachineID: "machine-" + site, Status: "online", LastSeen: now})
		snapshot.PBSInstances = append(snapshot.PBSInstances, models.PBSInstance{ID: "pbs-backup-" + site, Name: "backup-" + site, Host: "https://backup-" + site + ".invalid:8007", Status: "online", LastSeen: now, Datastores: []models.PBSDatastore{{Name: "main", Status: "unavailable"}}})
	}
	registry.IngestSnapshot(snapshot)
	minute := now.Hour()*60 + now.Minute()
	for _, ownerRef := range []string{"agent:host-a", "pbs-backup-a"} {
		ownerID, ok := registry.ResolveReferenceID(ownerRef)
		if !ok {
			t.Fatalf("fixture owner %q missing", ownerRef)
		}
		if err := store.SetResourceOperatorState(unifiedresources.ResourceOperatorState{
			CanonicalID: ownerID, MaintenanceScope: unifiedresources.MaintenanceScopeResourceAndDescendants,
			MaintenanceRecurrence: &unifiedresources.RecurringMaintenanceWindow{
				Timezone: "UTC", Weekdays: []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"},
				StartMinute: (minute + 1440 - 60) % 1440, EndMinute: (minute + 60) % 1440,
			},
		}); err != nil {
			t.Fatal(err)
		}
	}
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	manager.UpdateConfig(alerts.AlertConfig{Enabled: true, ActivationState: alerts.ActivationActive, TimeThresholds: map[string]int{"storage": 0}})
	monitor := &Monitor{alertManager: manager}
	monitor.installOperatorIntentResolver(unifiedresources.NewMonitorAdapter(registry))
	for range 5 {
		for _, site := range []string{"a", "b"} {
			manager.CheckConnection(alerts.ConnectionSnapshot{ID: "pve:site-" + site, Name: "site-" + site, Type: alerts.ConnectionTypePVE, State: alerts.ConnectionStateUnreachable, Enabled: true})
			manager.CheckConnection(alerts.ConnectionSnapshot{ID: "pbs:backup-" + site, PolicyResourceID: "pbs-backup-" + site, Name: "backup-" + site, Type: alerts.ConnectionTypePBS, State: alerts.ConnectionStateUnreachable, Enabled: true})
			manager.CheckStorage(models.Storage{ID: "pbs-backup-" + site + "-main", Instance: "pbs-backup-" + site, Node: "backup-" + site, Name: "main", Type: "pbs", Status: "unavailable"})
		}
	}
	active := manager.GetActiveAlerts()
	encoded, err := json.Marshal(map[string]any{"phase": "recurring-scoped-estate", "alerts": active, "resources": registry.List()})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("MAINTENANCE_SUBJECT_FIXTURE %s", encoded)
	want := map[string]bool{"pve:site-b": true, "pbs:backup-b": true, "pbs-backup-b-main": true}
	for _, a := range active {
		if !want[a.ResourceID] {
			t.Errorf("scheduled subject notified or control duplicated: %+v", a)
		}
		delete(want, a.ResourceID)
	}
	if len(want) != 0 {
		t.Errorf("maintenance suppressed unrelated resources: %v", want)
	}
}
