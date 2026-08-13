package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func createInstallablePatrolObserver(t *testing.T, store *PatrolObjectiveStore, now time.Time, resourceIDs ...string) PatrolObjective {
	t.Helper()
	objective, err := store.Create(CreatePatrolObjectiveInput{
		Brief:       "Keep scoped resources online",
		ResourceIDs: resourceIDs,
	}, now)
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}
	objective, err = store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
		ExpectedRevision: objective.Revision,
		Interpretation:   "Every scoped canonical resource remains online.",
		TriggerKinds:     []PatrolObserverTriggerKind{PatrolObserverTriggerInterval},
		ProbeJSON:        `{"runtime":"pulse-resource-state/v1","path":"status","operator":"equals","value":"online","sample_interval_seconds":10,"wake_after_consecutive_failures":2}`,
		WakeEvidence:     "A scoped resource is not online for two consecutive samples.",
		RequirementsJSON: `{}`,
		Actor:            "patrol:model",
	}, now)
	if err != nil {
		t.Fatalf("propose observer: %v", err)
	}
	return objective
}

func TestPatrolObserverRuntimeInstallsEvaluatesAndLeasesWithoutObjectiveRevisionChurn(t *testing.T) {
	now := time.Date(2026, 8, 14, 1, 0, 0, 0, time.UTC)
	store := NewInMemoryPatrolObjectiveStore()
	objective := createInstallablePatrolObserver(t, store, now, "node-1")
	proposalRevision := objective.Revision
	patrol := NewPatrolService(nil, mockPatrolStateProvider{state: models.StateSnapshot{
		Nodes: []models.Node{{ID: "node-1", Name: "node-1", Status: "online"}},
	}})
	patrol.SetObjectiveStore(store)

	patrol.processObjectiveObservers(now.Add(time.Second))
	got, ok := store.Get(objective.ID, now.Add(time.Second))
	if !ok || got.Observer == nil {
		t.Fatalf("installed objective missing: %+v, found=%v", got, ok)
	}
	if got.Observer.State != PatrolObserverInstalled || got.Coverage.State != PatrolObjectiveCovered || got.Coverage.ReasonCode != "observer_healthy" {
		t.Fatalf("installed observer = %+v, coverage = %+v", got.Observer, got.Coverage)
	}
	if got.Observer.ValidUntil == nil || got.Observer.LastEvidenceAt == nil {
		t.Fatalf("observer lease was not persisted: %+v", got.Observer)
	}
	if got.Revision != proposalRevision+2 {
		t.Fatalf("objective revision = %d, want proposal revision + validation/install only (%d)", got.Revision, proposalRevision+2)
	}

	patrol.processObjectiveObservers(now.Add(2 * time.Minute))
	refreshed, _ := store.Get(objective.ID, now.Add(2*time.Minute))
	if refreshed.Revision != got.Revision {
		t.Fatalf("health heartbeat changed objective revision from %d to %d", got.Revision, refreshed.Revision)
	}
	if !refreshed.Observer.ValidUntil.After(*got.Observer.ValidUntil) {
		t.Fatalf("health lease did not advance: old=%v new=%v", got.Observer.ValidUntil, refreshed.Observer.ValidUntil)
	}
}

func TestPatrolObserverRuntimeRecordsExplicitUnsupportedValidationReason(t *testing.T) {
	now := time.Date(2026, 8, 14, 1, 0, 0, 0, time.UTC)
	store := NewInMemoryPatrolObjectiveStore()
	objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep playback smooth", ResourceIDs: []string{"jellyfin"}}, now)
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}
	objective, err = store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
		ExpectedRevision: objective.Revision,
		Interpretation:   "Observe playback buffering events.",
		TriggerKinds:     []PatrolObserverTriggerKind{PatrolObserverTriggerAPI},
		ProbeJSON:        `{"runtime":"jellyfin-api/v1","event":"playback"}`,
		WakeEvidence:     "Playback buffers.",
		RequirementsJSON: `{"network":["jellyfin"]}`,
	}, now)
	if err != nil {
		t.Fatalf("propose observer: %v", err)
	}
	patrol := NewPatrolService(nil, nil)
	patrol.SetObjectiveStore(store)
	patrol.processObjectiveObservers(now.Add(time.Second))

	got, _ := store.Get(objective.ID, now.Add(time.Second))
	if got.Observer == nil || got.Observer.State != PatrolObserverRejected {
		t.Fatalf("unsupported proposal state = %+v", got.Observer)
	}
	if got.Observer.FailureCode != "observer_trigger_unsupported" || got.Coverage.ReasonCode != "observer_trigger_unsupported" {
		t.Fatalf("validation failure was not projected explicitly: observer=%+v coverage=%+v", got.Observer, got.Coverage)
	}
	if got.Coverage.State != PatrolObjectiveUncovered || !strings.Contains(got.Coverage.Summary, "could not be validated") {
		t.Fatalf("unsupported proposal coverage = %+v", got.Coverage)
	}
	revised, err := store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
		ExpectedRevision: got.Revision,
		Interpretation:   "Keep the canonical Jellyfin resource online.",
		TriggerKinds:     []PatrolObserverTriggerKind{PatrolObserverTriggerInterval},
		ProbeJSON:        `{"runtime":"pulse-resource-state/v1","path":"status","operator":"equals","value":"online","sample_interval_seconds":30,"wake_after_consecutive_failures":2}`,
		WakeEvidence:     "The Jellyfin resource is not online twice.",
		RequirementsJSON: `{}`,
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("replace rejected observer proposal: %v", err)
	}
	if revised.Observer == nil || revised.Observer.Version != got.Observer.Version+1 || revised.Observer.State != PatrolObserverProposed {
		t.Fatalf("replacement proposal = %+v", revised.Observer)
	}
}

func TestPatrolObserverRuntimeWakesModelOnlyAfterLocalFailureWindow(t *testing.T) {
	now := time.Date(2026, 8, 14, 1, 0, 0, 0, time.UTC)
	store := NewInMemoryPatrolObjectiveStore()
	objective := createInstallablePatrolObserver(t, store, now, "camera-1")
	patrol := NewPatrolService(nil, mockPatrolStateProvider{state: models.StateSnapshot{
		Hosts: []models.Host{{ID: "camera-1", Hostname: "camera-1", Status: "offline"}},
	}})
	patrol.SetObjectiveStore(store)
	tm := NewTriggerManager(TriggerManagerConfig{MaxPendingTriggers: 10})
	patrol.SetTriggerManager(tm)

	patrol.processObjectiveObservers(now.Add(time.Second))
	if got := tm.GetPendingCount(); got != 0 {
		t.Fatalf("observer woke Patrol after first failed local sample; pending=%d", got)
	}
	patrol.processObjectiveObservers(now.Add(11 * time.Second))
	if got := tm.GetPendingCount(); got != 1 {
		t.Fatalf("observer did not queue one Patrol wake after failure window; pending=%d", got)
	}
	patrol.processObjectiveObservers(now.Add(21 * time.Second))
	if got := tm.GetPendingCount(); got != 1 {
		t.Fatalf("observer repeated wake while failure remained active; pending=%d", got)
	}
	got, _ := store.Get(objective.ID, now.Add(21*time.Second))
	if got.Coverage.State != PatrolObjectiveCovered {
		t.Fatalf("runtime health and objective outcome were conflated: %+v", got.Coverage)
	}
}

func TestValidatePatrolObserverArtifactRejectsUnknownExecutableFields(t *testing.T) {
	now := time.Now().UTC()
	store := NewInMemoryPatrolObjectiveStore()
	objective := createInstallablePatrolObserver(t, store, now, "node-1")
	artifact, _ := store.GetObserverArtifact(objective.ID)
	artifact.Probe = []byte(`{"runtime":"pulse-resource-state/v1","path":"status","operator":"equals","value":"online","sample_interval_seconds":30,"wake_after_consecutive_failures":2,"command":"rm -rf /"}`)
	_, err := validatePatrolObserverArtifact(objective, artifact)
	if err == nil || err.code != "observer_probe_invalid" {
		t.Fatalf("unknown executable field validation error = %v", err)
	}
}

func TestPatrolObjectiveIntentEditInvalidatesInstalledObserver(t *testing.T) {
	now := time.Date(2026, 8, 14, 1, 0, 0, 0, time.UTC)
	store := NewInMemoryPatrolObjectiveStore()
	objective := createInstallablePatrolObserver(t, store, now, "node-1")
	patrol := NewPatrolService(nil, mockPatrolStateProvider{state: models.StateSnapshot{
		Nodes: []models.Node{{ID: "node-1", Status: "online"}},
	}})
	patrol.SetObjectiveStore(store)
	patrol.processObjectiveObservers(now.Add(time.Second))
	installed, _ := store.Get(objective.ID, now.Add(time.Second))
	if installed.Coverage.State != PatrolObjectiveCovered {
		t.Fatalf("observer was not installed before edit: %+v", installed.Coverage)
	}

	revisedBrief := "Keep this node online and its workload responsive"
	updated, err := store.Update(objective.ID, UpdatePatrolObjectiveInput{
		ExpectedRevision: installed.Revision,
		Brief:            &revisedBrief,
		Actor:            "operator",
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("update objective intent: %v", err)
	}
	if updated.Observer == nil || updated.Observer.State != PatrolObserverDisabled || updated.Observer.ValidUntil != nil {
		t.Fatalf("intent edit retained stale observer authority: %+v", updated.Observer)
	}
	if updated.Coverage.State != PatrolObjectiveUncovered || updated.Coverage.ReasonCode != "observer_disabled" {
		t.Fatalf("intent edit coverage = %+v", updated.Coverage)
	}
}
