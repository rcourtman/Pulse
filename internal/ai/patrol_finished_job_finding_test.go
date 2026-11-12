package ai

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// stoppedContainerReport is a report that only says the container is not
// running, which is what the alert-ownership gate judges.
func stoppedContainerReport(resourceID, name string) tools.PatrolFindingInput {
	return tools.PatrolFindingInput{
		ResourceID: resourceID, ResourceName: name, ResourceType: "app-container",
		Key: tools.PatrolUnalertedStoppedServiceKey, Severity: "warning", Category: "reliability",
		Title:          "Service container is exited and should be running",
		Description:    "The container serves a web interface and has been exited since the host was last touched.",
		Impact:         "The service is unreachable.",
		Recommendation: "Start the container and give it a restart policy.",
		Evidence:       "Current provider state is exited with code 0.",
	}
}

// cleanlyExited returns a container that exited 0 after a short run, under
// the given restart policy.
func cleanlyExited(id, name, restartPolicy string, ran time.Duration) models.DockerContainer {
	finished := time.Now().Add(-2 * time.Hour)
	started := finished.Add(-ran)
	return models.DockerContainer{
		ID: id, Name: name, State: "exited", Status: "Exited (0) 2 hours ago",
		ExitCode: 0, RestartPolicy: restartPolicy, StartedAt: &started, FinishedAt: &finished,
	}
}

// Pulse raises no state alert for a container it reads as a finished job. A
// report that such a container should be running is then the only word Pulse
// has on it, so the adapter keeps it instead of answering that an alert owns
// the stopped state.
func TestPatrolFindingAdapterKeepsStoppedReportWhenNoAlertWillFire(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := newPatrolRuntimeState(models.StateSnapshot{
		DockerHosts: []models.DockerHost{{
			ID: "docker-host-1",
			Containers: []models.DockerContainer{
				cleanlyExited("quiet", "wiki", "no", 4*time.Minute),
				cleanlyExited("alerting", "paperless", "unless-stopped", 4*time.Minute),
				cleanlyExited("long-run", "jellyfin", "no", 9*24*time.Hour),
				cleanlyExited("old-agent", "legacy", "", 4*time.Minute),
			},
		}},
	})
	adapter := newPatrolFindingCreatorAdapterState(ps, state)
	adapter.setFindingScope([]string{"quiet", "alerting", "long-run", "old-agent"})

	findingID, isNew, err := adapter.CreateFinding(stoppedContainerReport("quiet", "wiki"))
	if err != nil || !isNew {
		t.Fatalf("report on a container no alert covers = (%q, %t, %v), want a new finding", findingID, isNew, err)
	}
	if stored := ps.findings.Get(findingID); stored == nil {
		t.Fatal("the finding on a container no alert covers was not stored")
	}

	// Every container the alert subsystem does alert on keeps the old answer.
	for _, tc := range []struct{ id, name, why string }{
		{"alerting", "paperless", "Docker would restart a clean exit, so it was stopped and alerts"},
		{"long-run", "jellyfin", "it ran for days, so it is a stopped service and alerts"},
		{"old-agent", "legacy", "its agent reports no restart policy, so it alerts"},
	} {
		id, created, createErr := adapter.CreateFinding(stoppedContainerReport(tc.id, tc.name))
		var ownedByAlerts *tools.PatrolFindingOwnedByAlertsError
		if !errors.As(createErr, &ownedByAlerts) || created {
			t.Fatalf("%s: report = (%q, %t, %v), want the alert-owned no-op (%s)", tc.name, id, created, createErr, tc.why)
		}
		if stored := ps.findings.Get(id); stored != nil {
			t.Fatalf("%s: an alert-owned report was stored", tc.name)
		}
	}
}

// Production reads containers through the unified view, which does not carry
// the restart policy. The answer is joined from the agent's own report by the
// ID the alert subsystem gives the container, so the same two containers get
// the same two answers there.
func TestPatrolFindingAdapterFinishedJobAnswerSurvivesTheUnifiedView(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := newPatrolRuntimeState(models.StateSnapshot{
		DockerHosts: []models.DockerHost{{
			ID: "docker-host-1",
			Containers: []models.DockerContainer{
				cleanlyExited("c-quiet", "wiki", "no", 4*time.Minute),
				cleanlyExited("c-alerting", "paperless", "unless-stopped", 4*time.Minute),
			},
		}},
	})
	view := func(resourceID, containerID, name string) *unifiedresources.DockerContainerView {
		v := unifiedresources.NewDockerContainerView(&unifiedresources.Resource{
			ID:     resourceID,
			Name:   name,
			Type:   unifiedresources.ResourceTypeAppContainer,
			Status: unifiedresources.StatusOffline,
			Docker: &unifiedresources.DockerData{
				HostSourceID:   "docker-host-1",
				ContainerID:    containerID,
				ContainerState: "exited",
			},
		})
		return &v
	}
	state.readState = &mockReadState{dockerCtrs: []*unifiedresources.DockerContainerView{
		view("app-container-quiet", "c-quiet", "wiki"),
		view("app-container-alerting", "c-alerting", "paperless"),
		// A container the agent reports nothing about in this snapshot has no
		// answer, and keeps the old one.
		view("app-container-unknown", "c-unknown", "mystery"),
	}}

	rows := map[string]bool{}
	for _, row := range patrolAppContainerRows(state, nil) {
		rows[row.name] = row.finishedJob
	}
	if !rows["wiki"] || rows["paperless"] || rows["mystery"] {
		t.Fatalf("finished-job answers through the unified view = %v, want only wiki", rows)
	}

	adapter := newPatrolFindingCreatorAdapterState(ps, state)
	adapter.setFindingScope([]string{"app-container-quiet", "app-container-alerting", "app-container-unknown"})

	if id, isNew, err := adapter.CreateFinding(stoppedContainerReport("app-container-quiet", "wiki")); err != nil || !isNew {
		t.Fatalf("report on a container no alert covers = (%q, %t, %v), want a new finding", id, isNew, err)
	}
	for _, tc := range []struct{ id, name string }{
		{"app-container-alerting", "paperless"},
		{"app-container-unknown", "mystery"},
	} {
		_, created, err := adapter.CreateFinding(stoppedContainerReport(tc.id, tc.name))
		var ownedByAlerts *tools.PatrolFindingOwnedByAlertsError
		if !errors.As(err, &ownedByAlerts) || created {
			t.Fatalf("%s: report = (%t, %v), want the alert-owned no-op", tc.name, created, err)
		}
	}
}

// A run follows its flags. A stopped container no alert covers gets a
// watch-level flag, so a scheduled run looks at it and judges job or service,
// and its seed row says no alert exists. Containers the alert subsystem does
// cover get no such flag.
func TestTriageFlagsStoppedContainersNoAlertCovers(t *testing.T) {
	state := newPatrolRuntimeState(models.StateSnapshot{
		DockerHosts: []models.DockerHost{{
			ID: "docker-host-1",
			Containers: []models.DockerContainer{
				cleanlyExited("quiet", "wiki", "no", 4*time.Minute),
				cleanlyExited("alerting", "paperless", "unless-stopped", 4*time.Minute),
				{ID: "up", Name: "grafana", State: "running", RestartPolicy: "no"},
			},
		}},
	})

	flags, _ := triageUnalertedStoppedContainersState(state, nil, nil)
	if len(flags) != 1 {
		t.Fatalf("flags = %#v, want one for the container no alert covers", flags)
	}
	flag := flags[0]
	if flag.ResourceID != "quiet" || flag.ResourceType != "app-container" || flag.Severity != "watch" || flag.Category != triageUnalertedStopCategory {
		t.Fatalf("flag = %#v", flag)
	}
	if label := triageFlagLabel(flag); label != "Stopped, No Alert" {
		t.Fatalf("flag label = %q", label)
	}

	ps := NewPatrolService(nil, nil)
	triage := &TriageResult{Flags: flags, FlaggedIDs: map[string]bool{flag.ResourceID: true}}
	seed, _ := ps.buildTriageSeedContextState(triage, state, &PatrolScope{
		ResourceIDs: []string{"quiet", "alerting"},
		Depth:       PatrolDepthQuick,
	}, nil)
	for _, expected := range []string{
		"Stopped, No Alert",
		"| wiki | quiet | exited" + patrolUnalertedStoppedStateSuffix + " |",
		"| paperless | alerting | exited |",
	} {
		if !strings.Contains(seed, expected) {
			t.Fatalf("seed is missing %q:\n%s", expected, seed)
		}
	}
}

// A host that keeps dozens of exited init containers must not turn every run
// into a review of all of them.
func TestTriageBoundsStoppedContainersNoAlertCovers(t *testing.T) {
	containers := make([]models.DockerContainer, 0, 30)
	for i := 0; i < 30; i++ {
		id := fmt.Sprintf("init-%02d", i)
		containers = append(containers, cleanlyExited(id, id, "no", 3*time.Second))
	}
	state := newPatrolRuntimeState(models.StateSnapshot{
		DockerHosts: []models.DockerHost{{ID: "docker-host-1", Containers: containers}},
	})

	flags, _ := triageUnalertedStoppedContainersState(state, nil, nil)
	if len(flags) != triageUnalertedStopMaxFlags {
		t.Fatalf("got %d flags, want the bound of %d", len(flags), triageUnalertedStopMaxFlags)
	}
	if flags[0].ResourceID != "init-00" || flags[len(flags)-1].ResourceID != fmt.Sprintf("init-%02d", triageUnalertedStopMaxFlags-1) {
		t.Fatalf("flags are not the first by name: %s .. %s", flags[0].ResourceID, flags[len(flags)-1].ResourceID)
	}
}

// The exception tells the model which key to use. It has to be a key the
// ownership gate would otherwise drop, so that keeping the finding is the
// gate's decision about this container and not an accident of wording.
func TestNoAlertExceptionKeyIsOneTheGateDecides(t *testing.T) {
	if !strings.Contains(tools.PatrolUnalertedStoppedContainerRule, tools.PatrolUnalertedStoppedServiceKey) {
		t.Fatal("the exception does not name the key the gate keeps")
	}
	finding := &Finding{ResourceID: "quiet", ResourceName: "wiki", ResourceType: "app-container", Category: FindingCategoryReliability, Key: tools.PatrolUnalertedStoppedServiceKey}
	covered := newPatrolFindingCreatorAdapterState(NewPatrolService(nil, nil), newPatrolRuntimeState(models.StateSnapshot{
		DockerHosts: []models.DockerHost{{ID: "docker-host-1", Containers: []models.DockerContainer{cleanlyExited("quiet", "wiki", "unless-stopped", 4*time.Minute)}}},
	}))
	if !covered.findingOwnedByAlerts(finding) {
		t.Fatalf("key %q on an alerting container should read as alert-owned", finding.Key)
	}
	uncovered := newPatrolFindingCreatorAdapterState(NewPatrolService(nil, nil), newPatrolRuntimeState(models.StateSnapshot{
		DockerHosts: []models.DockerHost{{ID: "docker-host-1", Containers: []models.DockerContainer{cleanlyExited("quiet", "wiki", "no", 4*time.Minute)}}},
	}))
	if uncovered.findingOwnedByAlerts(finding) {
		t.Fatalf("key %q on a container no alert covers must not read as alert-owned", finding.Key)
	}
}

// Judging an exit costs model work, and a finished migration does not become
// a service by being looked at again. A run over the whole estate asks about
// each exit once; a run the operator scoped to the container always asks; a
// job that runs again is a new exit.
func TestTriageAsksAboutEachUnalertedStopOnce(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	job := cleanlyExited("job", "db-migrate", "no", 3*time.Second)
	stateOf := func(containers ...models.DockerContainer) patrolRuntimeState {
		return newPatrolRuntimeState(models.StateSnapshot{
			DockerHosts: []models.DockerHost{{ID: "docker-host-1", Containers: containers}},
		})
	}
	state := stateOf(job)

	flags, keys := ps.triageUnalertedStops(state, nil)
	if len(flags) != 1 || len(keys) != 1 {
		t.Fatalf("first run: flags=%d keys=%v, want one", len(flags), keys)
	}
	// A run that never completed remembers nothing.
	if flags, _ := ps.triageUnalertedStops(state, nil); len(flags) != 1 {
		t.Fatalf("a run that did not complete must leave the exit to be asked again, got %d flags", len(flags))
	}

	ps.rememberUnalertedStopsJudged(keys, state)
	if flags, _ := ps.triageUnalertedStops(state, nil); len(flags) != 0 {
		t.Fatalf("the exit was judged, but the next run was asked again: %#v", flags)
	}
	if flags, _ := ps.triageUnalertedStops(state, map[string]bool{"job": true}); len(flags) != 1 {
		t.Fatalf("a run scoped to the container must still ask, got %d flags", len(flags))
	}

	// The same container runs again and finishes later: a new exit.
	again := job
	later := job.FinishedAt.Add(30 * time.Minute)
	started := later.Add(-3 * time.Second)
	again.StartedAt, again.FinishedAt = &started, &later
	rerun := stateOf(again)
	flags, keys = ps.triageUnalertedStops(rerun, nil)
	if len(flags) != 1 {
		t.Fatalf("a job that ran again is a new exit, got %d flags", len(flags))
	}

	// Remembering against the new snapshot forgets the exit it no longer holds.
	ps.rememberUnalertedStopsJudged(keys, rerun)
	ps.unalertedStopsMu.Lock()
	remembered := len(ps.unalertedStopsJudged)
	ps.unalertedStopsMu.Unlock()
	if remembered != 1 {
		t.Fatalf("remembered %d exits, want only the one the snapshot holds", remembered)
	}
}
