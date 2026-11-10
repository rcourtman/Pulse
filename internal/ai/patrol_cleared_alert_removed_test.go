package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Removing a container clears its alerts. The check that follows a cleared
// alert has nothing to look at then, and it must not leave a failed run in the
// history for something the operator did on purpose.
func TestRunScopedPatrolClearedAlertOnRemovedContainerRecordsNoRun(t *testing.T) {
	removedContainer := strings.Repeat("ab12", 16)
	removedAlertResource := alerts.DockerContainerResourceID("dock-agent-1", removedContainer, "web")
	newService := func() *PatrolService {
		ps := NewPatrolService(nil, &mockStateProvider{state: models.StateSnapshot{
			DockerHosts: []models.DockerHost{{ID: "dock-agent-1", Hostname: "docker-box", Status: "online", LastSeen: time.Now(),
				Containers: []models.DockerContainer{{ID: strings.Repeat("cd34", 16), Name: "db", State: "running"}}}},
			Nodes: []models.Node{{ID: "node-a", Name: "duplicate"}, {ID: "node-b", Name: "duplicate"}},
		}})
		ps.SetConfig(PatrolConfig{Enabled: true, AnalyzeDocker: true, AnalyzeNodes: true, Interval: time.Hour})
		return ps
	}

	t.Run("cleared alert on a removed container", func(t *testing.T) {
		ps := newService()
		ps.runScopedPatrol(t.Context(), AlertClearedPatrolScope("docker-container-state-web", removedAlertResource, "app-container"))
		if runs := ps.runHistoryStore.GetRecent(5); len(runs) != 0 {
			t.Fatalf("a cleared alert on a removed container left a run in the history: %+v", runs[0])
		}
		if ps.runInProgress {
			t.Fatal("the skipped check kept the Patrol run slot")
		}
	})

	t.Run("firing alert on a resource that does not resolve still fails", func(t *testing.T) {
		ps := newService()
		ps.runScopedPatrol(t.Context(), AlertTriggeredPatrolScope("docker-container-state-web", removedAlertResource, "app-container", "docker-container-state"))
		runs := ps.runHistoryStore.GetRecent(5)
		if len(runs) != 1 || runs[0].Status != "error" || runs[0].ErrorSummary != "Patrol scope could not be resolved" {
			t.Fatalf("an alert that fires on an unresolved resource must keep its failed run, got %+v", runs)
		}
	})

	t.Run("cleared alert on an ambiguous identity still fails", func(t *testing.T) {
		ps := newService()
		ps.runScopedPatrol(t.Context(), AlertClearedPatrolScope("node-offline-duplicate", "duplicate", "node"))
		runs := ps.runHistoryStore.GetRecent(5)
		if len(runs) != 1 || runs[0].Status != "error" {
			t.Fatalf("an ambiguous cleared-alert scope may still be there and must keep its failed run, got %+v", runs)
		}
	})

	t.Run("a run a person started is always recorded", func(t *testing.T) {
		ps := newService()
		start, accepted := ps.beginRun("scoped")
		if !accepted {
			t.Fatal("could not reserve the run slot")
		}
		ps.runScopedPatrolWithStart(t.Context(), AlertClearedPatrolScope("docker-container-state-web", removedAlertResource, "app-container"), start, false)
		runs := ps.runHistoryStore.GetRecent(5)
		if len(runs) != 1 || runs[0].ID != start.id || runs[0].Status != "error" {
			t.Fatalf("an accepted run must end in a record, got %+v", runs)
		}
	})
}

// The order a Docker report is handled in: the collected state drops the
// removed container, the alert manager clears its alert, and only then is the
// resource inventory rebuilt. The cleared-alert check starts in between, so
// the inventory still lists the container that the collected state no longer
// has. Seen on a rig one second after each "Alert cleared".
func TestRunScopedPatrolClearedAlertWhileTheInventoryStillListsTheContainer(t *testing.T) {
	const dockerHost = "dock-agent-1"
	container := models.DockerContainer{ID: strings.Repeat("ab12", 16), Name: "web", State: "exited", Status: "Exited (139) 2 minutes ago", ExitCode: 139}
	hostWith := func(containers ...models.DockerContainer) models.DockerHost {
		return models.DockerHost{ID: dockerHost, DisplayName: "docker-box", Hostname: "docker-box", Status: "online", LastSeen: time.Now(), Containers: containers}
	}

	// The real alert manager raises the state alert and clears it when the
	// container is removed.
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	alertConfig := manager.GetConfig()
	alertConfig.Enabled = true
	manager.UpdateConfig(alertConfig)
	stateAlert := func() *alerts.Alert {
		for _, active := range manager.GetActiveAlerts() {
			if active.Type == "docker-container-state" {
				found := active
				return &found
			}
		}
		return nil
	}
	var raised *alerts.Alert
	for deadline := time.Now().Add(20 * time.Second); raised == nil; raised = stateAlert() {
		if time.Now().After(deadline) {
			t.Fatalf("the alert manager raised no container state alert, active: %+v", manager.GetActiveAlerts())
		}
		manager.CheckDockerHost(hostWith(container))
		time.Sleep(100 * time.Millisecond)
	}
	for deadline := time.Now().Add(20 * time.Second); stateAlert() != nil; {
		if time.Now().After(deadline) {
			t.Fatal("the alert did not clear after the container was removed")
		}
		manager.CheckDockerHost(hostWith())
		time.Sleep(100 * time.Millisecond)
	}
	t.Logf("alert %s on %s cleared", raised.ID, raised.ResourceID)

	before := models.StateSnapshot{DockerHosts: []models.DockerHost{hostWith(container)}}
	after := models.StateSnapshot{
		DockerHosts: []models.DockerHost{hostWith()},
		RecentlyResolved: []models.ResolvedAlert{{ResolvedTime: time.Now(), Alert: models.Alert{
			ID: raised.ID, Type: raised.Type, Level: string(raised.Level), ResourceID: raised.ResourceID,
			ResourceName: raised.ResourceName, Message: raised.Message, StartTime: raised.StartTime,
		}}},
	}

	for _, tc := range []struct {
		name      string
		inventory models.StateSnapshot
	}{
		{"inventory one report behind", before},
		{"inventory caught up", after},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := unifiedresources.NewRegistry(nil)
			registry.IngestSnapshot(tc.inventory)
			ps := NewPatrolService(nil, &mockStateProvider{state: after})
			ps.SetReadState(registry)
			ps.SetUnifiedResourceProvider(unifiedresources.NewUnifiedAIAdapter(registry))
			ps.SetConfig(PatrolConfig{Enabled: true, AnalyzeDocker: true, Interval: time.Hour})

			ps.runScopedPatrol(t.Context(), AlertClearedPatrolScope(raised.ID, raised.ResourceID, "app-container"))

			if runs := ps.runHistoryStore.GetRecent(5); len(runs) != 0 {
				t.Fatalf("a cleared alert on a removed container left a run in the history: status %q, %q, %q",
					runs[0].Status, runs[0].FindingsSummary, runs[0].ErrorSummary)
			}
			if ps.runInProgress {
				t.Fatal("the skipped check kept the Patrol run slot")
			}
		})
	}
}
