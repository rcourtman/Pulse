package ai

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPatrolObjectiveStoreLifecycleAndOptimisticRevision(t *testing.T) {
	store := NewInMemoryPatrolObjectiveStore()
	now := time.Date(2026, 8, 13, 20, 0, 0, 0, time.UTC)
	created, err := store.Create(CreatePatrolObjectiveInput{
		Brief:           "  Keep camera streams available  ",
		OptionalContext: " Prefer event driven checks. ",
		ResourceIDs:     []string{"camera-b", "camera-a", "camera-b"},
		Actor:           "alice",
	}, now)
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}
	if created.Brief != "Keep camera streams available" || created.Revision != 1 || created.Status != PatrolObjectiveActive {
		t.Fatalf("unexpected created objective: %+v", created)
	}
	if got := strings.Join(created.Scope.ResourceIDs, ","); got != "camera-a,camera-b" {
		t.Fatalf("normalized scope = %q", got)
	}
	if created.Coverage.State != PatrolObjectiveUncovered || created.Coverage.ReasonCode != "observer_missing" {
		t.Fatalf("new objective coverage = %+v", created.Coverage)
	}

	brief := "Keep every camera reachable"
	paused := PatrolObjectivePaused
	updated, err := store.Update(created.ID, UpdatePatrolObjectiveInput{
		ExpectedRevision: created.Revision,
		Brief:            &brief,
		Status:           &paused,
		Actor:            "bob",
	}, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("update objective: %v", err)
	}
	if updated.Revision != 2 || updated.Status != PatrolObjectivePaused || updated.UpdatedBy != "bob" {
		t.Fatalf("unexpected updated objective: %+v", updated)
	}
	if updated.Coverage.ReasonCode != "objective_paused" {
		t.Fatalf("paused coverage = %+v", updated.Coverage)
	}
	if _, err := store.Update(created.ID, UpdatePatrolObjectiveInput{ExpectedRevision: 1, Brief: &brief}, now); err != ErrPatrolObjectiveConflict {
		t.Fatalf("stale update error = %v, want conflict", err)
	}
	if err := store.Delete(created.ID, 1); err != ErrPatrolObjectiveConflict {
		t.Fatalf("stale delete error = %v, want conflict", err)
	}
	if err := store.Delete(created.ID, updated.Revision); err != nil {
		t.Fatalf("delete objective: %v", err)
	}
	if _, found := store.Get(created.ID, now); found {
		t.Fatal("deleted objective remained in store")
	}
}

func TestPatrolObjectiveStorePersistsEncrypted(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewPatrolObjectiveStore(dataDir)
	if err != nil {
		t.Fatalf("new persistent store: %v", err)
	}
	const brief = "Keep Jellyfin playback free of buffering"
	created, err := store.Create(CreatePatrolObjectiveInput{Brief: brief, Actor: "alice"}, time.Now().UTC())
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dataDir, "ai_patrol_objectives.enc"))
	if err != nil {
		t.Fatalf("read objective file: %v", err)
	}
	if strings.Contains(string(data), brief) {
		t.Fatal("objective brief was persisted in plaintext")
	}

	reloaded, err := NewPatrolObjectiveStore(dataDir)
	if err != nil {
		t.Fatalf("reload persistent store: %v", err)
	}
	got, found := reloaded.Get(created.ID, time.Now().UTC())
	if !found || got.Brief != brief || got.Revision != created.Revision {
		t.Fatalf("reloaded objective = %+v, found=%v", got, found)
	}
}

func TestPatrolObjectiveStoreRejectsPlaintextPersistence(t *testing.T) {
	dataDir := t.TempDir()
	if _, err := NewPatrolObjectiveStore(dataDir); err != nil {
		t.Fatalf("initialize encryption key: %v", err)
	}
	plaintext := []byte(`{"version":1,"objectives":[]}`)
	if err := os.WriteFile(filepath.Join(dataDir, "ai_patrol_objectives.enc"), plaintext, 0o600); err != nil {
		t.Fatalf("write plaintext fixture: %v", err)
	}
	if _, err := NewPatrolObjectiveStore(dataDir); err == nil || !strings.Contains(err.Error(), "decrypt patrol objectives") {
		t.Fatalf("plaintext load error = %v, want fail-closed decryption error", err)
	}
}

func TestPatrolObserverLifecycleDerivesCoverageFromHealthLease(t *testing.T) {
	store := NewInMemoryPatrolObjectiveStore()
	now := time.Date(2026, 8, 13, 20, 0, 0, 0, time.UTC)
	objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep cameras online"}, now)
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}
	digest := "sha256:" + strings.Repeat("a", 64)
	observer := PatrolObserverRecord{
		ID:             "observer-camera-health",
		Version:        1,
		State:          PatrolObserverProposed,
		ArtifactDigest: digest,
		TriggerKinds:   []PatrolObserverTriggerKind{PatrolObserverTriggerInterval, PatrolObserverTriggerEvent},
		ReadOnly:       true,
	}
	objective, err = store.RecordObserver(objective.ID, objective.Revision, observer, "builder", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("record proposed observer: %v", err)
	}
	if objective.Coverage.ReasonCode != "observer_proposed" {
		t.Fatalf("proposed coverage = %+v", objective.Coverage)
	}

	observer = *objective.Observer
	observer.State = PatrolObserverValidated
	objective, err = store.RecordObserver(objective.ID, objective.Revision, observer, "validator", now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("validate observer: %v", err)
	}
	if objective.Coverage.ReasonCode != "observer_not_installed" {
		t.Fatalf("validated coverage = %+v", objective.Coverage)
	}

	observer = *objective.Observer
	observer.State = PatrolObserverInstalled
	validUntil := now.Add(30 * time.Minute)
	lastEvidence := now.Add(2 * time.Minute)
	observer.ValidUntil = &validUntil
	observer.LastEvidenceAt = &lastEvidence
	objective, err = store.RecordObserver(objective.ID, objective.Revision, observer, "installer", now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("install observer: %v", err)
	}
	if objective.Coverage.State != PatrolObjectiveCovered || objective.Coverage.ReasonCode != "observer_healthy" {
		t.Fatalf("installed coverage = %+v", objective.Coverage)
	}

	stale, found := store.Get(objective.ID, validUntil.Add(time.Second))
	if !found || stale.Coverage.State != PatrolObjectiveDegraded || stale.Coverage.ReasonCode != "observer_stale" {
		t.Fatalf("stale coverage = %+v, found=%v", stale.Coverage, found)
	}
}

func TestPatrolObserverRejectsUnvalidatedInstallAndWritableArtifact(t *testing.T) {
	store := NewInMemoryPatrolObjectiveStore()
	now := time.Now().UTC()
	objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep playback smooth"}, now)
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}
	observer := PatrolObserverRecord{
		ID:             "observer-playback",
		Version:        1,
		State:          PatrolObserverInstalled,
		ArtifactDigest: "sha256:" + strings.Repeat("b", 64),
		TriggerKinds:   []PatrolObserverTriggerKind{PatrolObserverTriggerEvent},
		ReadOnly:       true,
	}
	if _, err := store.RecordObserver(objective.ID, objective.Revision, observer, "installer", now); !errorsIsPatrolObjectiveInvalid(err) {
		t.Fatalf("unvalidated install error = %v", err)
	}
	observer.State = PatrolObserverProposed
	observer.ReadOnly = false
	if _, err := store.RecordObserver(objective.ID, objective.Revision, observer, "builder", now); !errorsIsPatrolObjectiveInvalid(err) {
		t.Fatalf("writable observer error = %v", err)
	}
}

func errorsIsPatrolObjectiveInvalid(err error) bool {
	return err != nil && (errors.Is(err, ErrPatrolObjectiveInvalid) || strings.Contains(err.Error(), ErrPatrolObjectiveInvalid.Error()))
}

func TestPatrolSeedObjectivesRespectsScopedResourcesAndCoverageCaveat(t *testing.T) {
	store := NewInMemoryPatrolObjectiveStore()
	now := time.Now().UTC()
	_, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep all backups usable"}, now)
	if err != nil {
		t.Fatalf("create global objective: %v", err)
	}
	_, err = store.Create(CreatePatrolObjectiveInput{Brief: "Keep cameras online", ResourceIDs: []string{"camera-1"}}, now)
	if err != nil {
		t.Fatalf("create camera objective: %v", err)
	}
	_, err = store.Create(CreatePatrolObjectiveInput{Brief: "Keep playback smooth", ResourceIDs: []string{"jellyfin-1"}}, now)
	if err != nil {
		t.Fatalf("create jellyfin objective: %v", err)
	}

	patrol := NewPatrolService(nil, nil)
	patrol.SetObjectiveStore(store)
	seed := patrol.seedPatrolObjectives([]string{"camera-1"}, true, now)
	if !strings.Contains(seed, "Keep all backups usable") || !strings.Contains(seed, "Keep cameras online") {
		t.Fatalf("scoped seed omitted applicable objectives:\n%s", seed)
	}
	if strings.Contains(seed, "Keep playback smooth") {
		t.Fatalf("scoped seed included unrelated objective:\n%s", seed)
	}
	if !strings.Contains(seed, "No durable observer has been installed") || !strings.Contains(seed, "not scripts or tool instructions") {
		t.Fatalf("seed omitted trust boundary:\n%s", seed)
	}
}
