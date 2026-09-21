package ai

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestPatrolObjectiveRecoveryRestoresSavedMissingObserver(t *testing.T) {
	now := time.Now().UTC()
	dir := t.TempDir()
	store, err := NewPatrolObjectiveStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	objective, err := store.Create(CreatePatrolObjectiveInput{
		Brief: "Keep critical services available", ResourceIDs: []string{"node-1"},
	}, now.Add(-4*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// The saved intent survives a restart. The original in-memory trigger does not.
	reloaded, err := NewPatrolObjectiveStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	patrol := NewPatrolService(nil, mockPatrolStateProvider{state: models.StateSnapshot{
		Nodes: []models.Node{{ID: "node-1", Name: "node-1", Status: "online"}},
	}})
	patrol.SetObjectiveStore(reloaded)
	tm := NewTriggerManager(DefaultTriggerManagerConfig())
	patrol.SetTriggerManager(tm)
	patrol.processObjectiveObservers(now)
	if tm.GetPendingCount() != 1 {
		t.Fatalf("saved active objective was stranded without setup: pending=%d", tm.GetPendingCount())
	}
	queued := tm.pendingTriggers[0]
	if queued.Reason != TriggerReasonObjectiveChanged || queued.ObjectiveContext == nil || queued.ObjectiveContext.ObjectiveID != objective.ID || queued.ObjectiveContext.Revision != objective.Revision {
		t.Fatalf("recovery lost retained intent: %+v", queued)
	}
	patrol.processObjectiveObservers(now.Add(time.Second))
	if tm.GetPendingCount() != 1 {
		t.Fatal("repeated reconciliation duplicated setup")
	}
	got, _ := reloaded.Get(objective.ID, now)
	if got.Revision != objective.Revision || got.Observer != nil || got.Coverage.State != PatrolObjectiveUncovered {
		t.Fatalf("scheduling invented coverage or changed intent: %+v", got)
	}
	if !patrol.beginObjectivePlanning(queued, now) {
		t.Fatal("recovered setup was not admitted")
	}
	// Substitute only the model's proposal. Validation, installation, and
	// measured coverage still run through the real observer runtime.
	_, err = reloaded.ProposeObserver(objective.ID, ProposePatrolObserverInput{
		ExpectedRevision: queued.ObjectiveContext.Revision, EvidenceFit: PatrolObserverEvidenceFitDirect,
		Interpretation: "The selected node remains online", TriggerKinds: []PatrolObserverTriggerKind{PatrolObserverTriggerInterval},
		ProbeJSON:    `{"runtime":"pulse-resource-state/v1","path":"status","operator":"equals","value":"online","sample_interval_seconds":10,"wake_after_consecutive_failures":2}`,
		WakeEvidence: "Node offline", RequirementsJSON: `{}`,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	patrol.processObjectiveObservers(now.Add(2 * time.Second))
	covered, _ := reloaded.Get(objective.ID, now.Add(2*time.Second))
	if covered.Coverage.State != PatrolObjectiveCovered || covered.Observer.State != PatrolObserverInstalled {
		t.Fatalf("recovered setup did not install and verify the observer: %+v", covered)
	}
}

func TestPatrolObjectiveRecoveryRespectsRuntimeAndOperatorState(t *testing.T) {
	for _, state := range []string{"disabled", "blocked", "busy", "paused", "archived"} {
		t.Run(state, func(t *testing.T) {
			now := time.Now().UTC()
			store := NewInMemoryPatrolObjectiveStore()
			objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep services available", ResourceIDs: []string{"node-1"}}, now)
			if err != nil {
				t.Fatal(err)
			}
			patrol := NewPatrolService(nil, nil)
			patrol.SetObjectiveStore(store)
			tm := NewTriggerManager(DefaultTriggerManagerConfig())
			patrol.SetTriggerManager(tm)
			switch state {
			case "disabled":
				patrol.config.Enabled = false
			case "blocked":
				patrol.config.RuntimeBlockedReason = "Provider unavailable"
			case "busy":
				patrol.runInProgress = true
			default:
				status := PatrolObjectiveStatus(state)
				if _, err := store.Update(objective.ID, UpdatePatrolObjectiveInput{ExpectedRevision: objective.Revision, Status: &status}, now); err != nil {
					t.Fatal(err)
				}
			}
			patrol.processObjectiveObservers(now)
			if tm.GetPendingCount() != 0 {
				t.Fatal("setup bypassed runtime or operator state")
			}
			if state == "busy" {
				patrol.runInProgress = false
				patrol.processObjectiveObservers(now.Add(time.Second))
				if tm.GetPendingCount() != 1 {
					t.Fatal("setup did not recover when the worker became idle")
				}
			}
		})
	}
}

func TestPatrolObjectiveRecoveryRetainsIntentWhenQueueRejectsSetup(t *testing.T) {
	now := time.Now().UTC()
	store := NewInMemoryPatrolObjectiveStore()
	if _, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep services available", ResourceIDs: []string{"node-1"}}, now); err != nil {
		t.Fatal(err)
	}
	patrol := NewPatrolService(nil, nil)
	patrol.SetObjectiveStore(store)
	tm := NewTriggerManager(TriggerManagerConfig{MaxPendingTriggers: 1})
	patrol.SetTriggerManager(tm)
	tm.TriggerPatrol(PatrolScope{Reason: TriggerReasonManual, Priority: triggerPriorityObjective + 1})
	patrol.processObjectiveObservers(now)
	if tm.GetPendingCount() != 1 || tm.pendingTriggers[0].Reason != TriggerReasonManual {
		t.Fatal("recovery displaced higher-priority work")
	}
	tm.pendingTriggers = nil
	patrol.processObjectiveObservers(now.Add(time.Second))
	if tm.GetPendingCount() != 1 || tm.pendingTriggers[0].Reason != TriggerReasonObjectiveChanged {
		t.Fatal("queue rejection permanently lost setup")
	}
}

func TestPatrolObjectiveRecoveryPacesDeliveredAttemptsAcrossRestart(t *testing.T) {
	now := time.Now().UTC()
	dir := t.TempDir()
	store, err := NewPatrolObjectiveStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep services available", ResourceIDs: []string{"node-1"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	interval := DefaultPatrolConfig().GetInterval()
	accepted, err := store.beginObserverPlanning(objective.ID, objective.Revision, now, interval)
	if err != nil || !accepted {
		t.Fatalf("start setup: accepted=%v err=%v", accepted, err)
	}
	reloaded, err := NewPatrolObjectiveStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	saved, _ := reloaded.Get(objective.ID, now)
	if saved.Revision != objective.Revision || !saved.UpdatedAt.Equal(objective.UpdatedAt) || saved.LastPlanningAttemptAt == nil {
		t.Fatalf("attempt changed intent or lost pacing: %+v", saved)
	}
	patrol := NewPatrolService(nil, nil)
	patrol.SetObjectiveStore(reloaded)
	tm := NewTriggerManager(DefaultTriggerManagerConfig())
	patrol.SetTriggerManager(tm)
	// No run history or in-memory scheduling state survives this restart.
	patrol.processObjectiveObservers(now.Add(time.Second))
	if tm.GetPendingCount() != 0 {
		t.Fatal("restart erased provider-call pacing")
	}
	patrol.processObjectiveObservers(now.Add(interval))
	if tm.GetPendingCount() != 1 {
		t.Fatal("missing proposal did not retry at the configured Patrol interval")
	}
}

func TestPatrolObjectiveRecoveryRejectsStaleQueuedIntent(t *testing.T) {
	for _, change := range []string{"edited", "paused", "deleted", "proposal", "attempted"} {
		t.Run(change, func(t *testing.T) {
			now := time.Now().UTC()
			store := NewInMemoryPatrolObjectiveStore()
			objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep services available", ResourceIDs: []string{"node-1"}}, now)
			if err != nil {
				t.Fatal(err)
			}
			patrol := NewPatrolService(nil, nil)
			patrol.SetObjectiveStore(store)
			tm := NewTriggerManager(DefaultTriggerManagerConfig())
			patrol.SetTriggerManager(tm)
			patrol.QueueObjectiveCoverage(objective)
			scope := tm.pendingTriggers[0]
			switch change {
			case "edited":
				brief := "Keep storage available"
				_, err = store.Update(objective.ID, UpdatePatrolObjectiveInput{ExpectedRevision: objective.Revision, Brief: &brief}, now)
			case "paused":
				status := PatrolObjectivePaused
				_, err = store.Update(objective.ID, UpdatePatrolObjectiveInput{ExpectedRevision: objective.Revision, Status: &status}, now)
			case "deleted":
				err = store.Delete(objective.ID, objective.Revision)
			case "proposal":
				_, err = store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
					ExpectedRevision: objective.Revision, EvidenceFit: PatrolObserverEvidenceFitDirect,
					Interpretation: "Observe scoped resource state", TriggerKinds: []PatrolObserverTriggerKind{PatrolObserverTriggerInterval},
					ProbeJSON:    `{"runtime":"pulse-resource-state/v1","path":"status","operator":"equals","value":"online","sample_interval_seconds":10,"wake_after_consecutive_failures":2}`,
					WakeEvidence: "Resource offline", RequirementsJSON: `{}`,
				}, now)
			case "attempted":
				_, err = store.beginObserverPlanning(objective.ID, objective.Revision, now, patrol.config.GetInterval())
			}
			if err != nil {
				t.Fatal(err)
			}
			before := len(patrol.runHistoryStore.GetAll())
			patrol.runScopedPatrol(context.Background(), scope)
			if len(patrol.runHistoryStore.GetAll()) != before || patrol.runInProgress {
				t.Fatal("obsolete setup started a run or retained the worker slot")
			}
		})
	}
}

func TestPatrolObjectiveRecoveryReplansEditedObserverThenInstalls(t *testing.T) {
	now := time.Now().UTC()
	store := NewInMemoryPatrolObjectiveStore()
	objective := createInstallablePatrolObserver(t, store, now, "node-1")
	patrol := NewPatrolService(nil, mockPatrolStateProvider{state: models.StateSnapshot{
		Nodes: []models.Node{{ID: "node-1", Name: "node-1", Status: "online"}},
	}})
	patrol.SetObjectiveStore(store)
	tm := NewTriggerManager(DefaultTriggerManagerConfig())
	patrol.SetTriggerManager(tm)
	patrol.processObjectiveObservers(now)
	installed, _ := store.Get(objective.ID, now)
	if tm.GetPendingCount() != 0 || installed.Coverage.State != PatrolObjectiveCovered {
		t.Fatal("existing observer was replanned instead of installed")
	}
	brief := "Keep this node available"
	updated, err := store.Update(objective.ID, UpdatePatrolObjectiveInput{ExpectedRevision: installed.Revision, Brief: &brief}, now)
	if err != nil {
		t.Fatal(err)
	}
	patrol.processObjectiveObservers(now.Add(time.Second))
	if tm.GetPendingCount() != 1 || tm.pendingTriggers[0].ObjectiveContext.Revision != updated.Revision {
		t.Fatal("disabled observer left revised intent without setup")
	}
	_, err = store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
		ExpectedRevision: updated.Revision, EvidenceFit: PatrolObserverEvidenceFitDirect,
		Interpretation: "The node remains online", TriggerKinds: []PatrolObserverTriggerKind{PatrolObserverTriggerInterval},
		ProbeJSON:    `{"runtime":"pulse-resource-state/v1","path":"status","operator":"equals","value":"online","sample_interval_seconds":10,"wake_after_consecutive_failures":2}`,
		WakeEvidence: "Node offline", RequirementsJSON: `{}`,
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	patrol.processObjectiveObservers(now.Add(3 * time.Second))
	covered, _ := store.Get(objective.ID, now.Add(3*time.Second))
	if covered.Coverage.State != PatrolObjectiveCovered || covered.Observer.State != PatrolObserverInstalled {
		t.Fatalf("recovered proposal did not acquire real coverage: %+v", covered)
	}
}

func TestPatrolObjectiveRecoveryClaimsAttemptAtomically(t *testing.T) {
	now := time.Now().UTC()
	store := NewInMemoryPatrolObjectiveStore()
	objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep services available", ResourceIDs: []string{"node-1"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			ok, err := store.beginObserverPlanning(objective.ID, objective.Revision, now, time.Hour)
			if err != nil {
				t.Error(err)
			}
			if ok {
				accepted.Add(1)
			}
		})
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("accepted %d concurrent attempts", accepted.Load())
	}
	brief := "Keep the selected node available"
	updated, err := store.Update(objective.ID, UpdatePatrolObjectiveInput{ExpectedRevision: objective.Revision, Brief: &brief}, now)
	if err != nil {
		t.Fatal(err)
	}
	if updated.LastPlanningAttemptAt != nil {
		t.Fatal("new intent inherited the previous attempt's delay")
	}
	if ok, err := store.beginObserverPlanning(updated.ID, updated.Revision, now, time.Hour); !ok || err != nil {
		t.Fatalf("new revision was not admitted: accepted=%v err=%v", ok, err)
	}
}

func TestPatrolObjectiveRecoveryDoesNotCallProviderWithoutDurableAttempt(t *testing.T) {
	now := time.Now().UTC()
	store := NewInMemoryPatrolObjectiveStore()
	objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep services available", ResourceIDs: []string{"node-1"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	// A directory cannot be replaced by the atomic file rename, on any platform.
	store.filePath = t.TempDir()
	accepted, err := store.beginObserverPlanning(objective.ID, objective.Revision, now, time.Hour)
	if err == nil || accepted {
		t.Fatalf("failed persistence admitted provider work: accepted=%v err=%v", accepted, err)
	}
	got, _ := store.Get(objective.ID, now)
	if got.LastPlanningAttemptAt != nil {
		t.Fatal("failed persistence suppressed a later recovery attempt")
	}
}
