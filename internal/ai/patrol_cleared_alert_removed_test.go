package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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
