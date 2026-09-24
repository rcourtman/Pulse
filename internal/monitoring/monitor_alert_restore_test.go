package monitoring

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestInstallOperatorIntentResolverReconcilesRestoredAlerts(t *testing.T) {
	now := time.Now().UTC()
	start, end := now.Add(-time.Hour), now.Add(time.Hour)
	for name, policy := range map[string]unifiedresources.ResourceOperatorState{
		"muted":       {MonitoringMode: unifiedresources.MonitoringModeMuted},
		"retired":     {LifecycleState: unifiedresources.LifecycleStateRetired},
		"maintenance": {MaintenanceStartAt: &start, MaintenanceEndAt: &end},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			fixture := []alerts.Alert{}
			store := unifiedresources.NewMemoryStore()
			registry := unifiedresources.NewRegistry(store)
			for _, id := range []string{"vm:101", "vm:202"} {
				registry.IngestResources([]unifiedresources.Resource{{ID: id, Type: unifiedresources.ResourceTypeVM}})
				fixture = append(fixture, alerts.Alert{
					ID: id + "::metric-threshold:cpu", ResourceID: id, ResourceName: id,
					Type: "cpu", Level: alerts.AlertLevelWarning, StartTime: start, LastSeen: now,
				})
			}
			policy.CanonicalID = "vm:101"
			if err := store.SetResourceOperatorState(policy); err != nil {
				t.Fatal(err)
			}
			payload, err := json.Marshal(fixture)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(dir, "alerts"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "alerts", "active-alerts.json"), payload, 0o600); err != nil {
				t.Fatal(err)
			}
			// Runtime startup restores durable alerts before the resource store
			// and its persisted operator policies are attached to the monitor.
			manager := alerts.NewManagerWithDataDir(dir, alerts.WithDurableAlertStore())
			t.Cleanup(manager.Stop)
			if got := len(manager.GetActiveAlerts()); got != 2 {
				t.Fatalf("restored alerts before policy attachment = %d, want 2", got)
			}
			monitor := &Monitor{alertManager: manager, state: models.NewState()}
			monitor.state.UpdateActiveAlerts(monitor.activeAlertsSnapshot())
			monitor.installOperatorIntentResolver(unifiedresources.NewMonitorAdapter(registry))
			active := manager.GetActiveAlerts()
			if len(active) != 1 || active[0].ResourceID != "vm:202" {
				t.Fatalf("policy attachment retained suppressed alerts or cleared the control: %+v", active)
			}
			if got := monitor.state.GetSnapshot().ActiveAlerts; len(got) != 1 || got[0].ResourceID != "vm:202" {
				t.Fatalf("shared state retained a suppressed restored alert: %+v", got)
			}
			manager.Stop()
			restarted := alerts.NewManagerWithDataDir(dir, alerts.WithDurableAlertStore())
			t.Cleanup(restarted.Stop)
			active = restarted.GetActiveAlerts()
			if len(active) != 1 || active[0].ResourceID != "vm:202" {
				t.Fatalf("suppressed alert returned after another restart: %+v", active)
			}
		})
	}
}

func TestResourcePublicationReconcilesRestoredNativeAlertAliases(t *testing.T) {
	dir := t.TempDir()
	store, err := unifiedresources.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := models.StateSnapshot{
		Nodes: []models.Node{{ID: "pve-node", Name: "pve1", Instance: "pve", Status: "online", LinkedAgentID: "host-1"}},
		Hosts: []models.Host{{ID: "host-1", Hostname: "pve1", MachineID: "machine-1", LinkedNodeID: "pve-node", Status: "online", LastSeen: time.Now()}},
	}
	beforeRestart := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store))
	beforeRestart.PopulateFromSnapshot(snapshot)
	canonicalID, found := beforeRestart.ResolveCanonicalResourceID("pve-node")
	if !found || canonicalID == "pve-node" {
		t.Fatalf("expected a distinct merged canonical agent ID, got %q, found=%v", canonicalID, found)
	}
	if err := store.SetResourceOperatorState(unifiedresources.ResourceOperatorState{
		CanonicalID: canonicalID, MonitoringMode: unifiedresources.MonitoringModeMuted,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = unifiedresources.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	now := time.Now().UTC()
	var restored []alerts.Alert
	for _, id := range []string{"pve-node", "agent:host-1", "unaffected"} {
		restored = append(restored, alerts.Alert{
			ID: id + "::connectivity", ResourceID: id, ResourceName: id,
			Type: "connectivity", Level: alerts.AlertLevelWarning, StartTime: now.Add(-time.Hour), LastSeen: now,
		})
	}
	payload, err := json.Marshal(restored)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "alerts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "alerts", "active-alerts.json"), payload, 0o600); err != nil {
		t.Fatal(err)
	}
	manager := alerts.NewManagerWithDataDir(dir, alerts.WithDurableAlertStore())
	t.Cleanup(manager.Stop)
	if got := len(manager.GetActiveAlerts()); got != 3 {
		t.Fatalf("restored alerts = %d, want 3", got)
	}
	monitor := &Monitor{alertManager: manager, state: models.NewState()}
	// Startup can attach the policy store before any resource observation.
	// The persisted canonical policy cannot yet resolve these native aliases.
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store)))
	if got := len(manager.GetActiveAlerts()); got != 3 {
		t.Fatalf("alerts before resource publication = %d, want 3", got)
	}
	monitor.updateResourceStore(snapshot)
	active := manager.GetActiveAlerts()
	if len(active) != 1 || active[0].ResourceID != "unaffected" {
		t.Fatalf("resource publication did not reconcile restored aliases: %+v", active)
	}
	if got := monitor.state.GetSnapshot().ActiveAlerts; len(got) != 1 || got[0].ResourceID != "unaffected" {
		t.Fatalf("shared state retained suppressed native aliases: %+v", got)
	}
}
