package ai

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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

func TestPatrolObjectiveStoreProposesEncryptedObserverWithoutClaimingCoverage(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewPatrolObjectiveStore(dataDir)
	if err != nil {
		t.Fatalf("new persistent store: %v", err)
	}
	now := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep camera streams available"}, now)
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}
	objective, err = store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
		ExpectedRevision: objective.Revision,
		Interpretation:   "Wake Patrol when a camera changes from reachable to unreachable.",
		TriggerKinds:     []PatrolObserverTriggerKind{PatrolObserverTriggerEvent},
		ProbeJSON:        `{ "outputs": {"unhealthy": "camera unreachable"}, "source": "canonical resource events" }`,
		WakeEvidence:     "A scoped camera resource transitions to unreachable.",
		RequirementsJSON: `{ "network": [], "filesystem": [], "secrets": [], "runtime": "pulse" }`,
		Actor:            "patrol:model",
	}, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("propose observer: %v", err)
	}
	if objective.Observer == nil || objective.Observer.State != PatrolObserverProposed || !objective.Observer.ReadOnly {
		t.Fatalf("observer proposal = %+v", objective.Observer)
	}
	if objective.Observer.Artifact != nil {
		t.Fatal("public objective read leaked model-authored observer artifact")
	}
	if objective.Coverage.State != PatrolObjectiveUncovered || objective.Coverage.ReasonCode != "observer_proposed" {
		t.Fatalf("proposal coverage = %+v", objective.Coverage)
	}
	if _, err := store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
		ExpectedRevision: objective.Revision,
		Interpretation:   "Replace the proposal without a core lifecycle decision.",
		TriggerKinds:     []PatrolObserverTriggerKind{PatrolObserverTriggerEvent},
		ProbeJSON:        `{}`,
		WakeEvidence:     "Any camera failure.",
		RequirementsJSON: `{}`,
	}, now.Add(90*time.Second)); !errorsIsPatrolObjectiveInvalid(err) {
		t.Fatalf("proposal displacement error = %v", err)
	}
	artifact, ok := store.GetObserverArtifact(objective.ID)
	if !ok || artifact.Format != PatrolObserverArtifactFormatV1 || !strings.Contains(string(artifact.Probe), "canonical resource events") {
		t.Fatalf("internal observer artifact = %+v, found=%v", artifact, ok)
	}

	ciphertext, err := os.ReadFile(filepath.Join(dataDir, "ai_patrol_objectives.enc"))
	if err != nil {
		t.Fatalf("read encrypted objective file: %v", err)
	}
	if strings.Contains(string(ciphertext), "canonical resource events") {
		t.Fatal("observer artifact was persisted in plaintext")
	}
	reloaded, err := NewPatrolObjectiveStore(dataDir)
	if err != nil {
		t.Fatalf("reload objective store: %v", err)
	}
	reloadedArtifact, ok := reloaded.GetObserverArtifact(objective.ID)
	if !ok || !strings.Contains(string(reloadedArtifact.Probe), "canonical resource events") {
		t.Fatalf("reloaded artifact = %+v, found=%v", reloadedArtifact, ok)
	}
	public, ok := reloaded.Get(objective.ID, now.Add(2*time.Minute))
	if !ok || public.Observer == nil || public.Observer.Artifact != nil || public.Coverage.ReasonCode != "observer_proposed" {
		t.Fatalf("reloaded public objective = %+v, found=%v", public, ok)
	}
}

func TestPatrolObjectiveStoreRejectsInvalidObserverProposalArtifact(t *testing.T) {
	store := NewInMemoryPatrolObjectiveStore()
	now := time.Now().UTC()
	objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep playback smooth"}, now)
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}
	base := ProposePatrolObserverInput{
		ExpectedRevision: objective.Revision,
		Interpretation:   "Detect sustained playback buffering.",
		TriggerKinds:     []PatrolObserverTriggerKind{PatrolObserverTriggerEvent},
		WakeEvidence:     "Playback enters a buffering state.",
		RequirementsJSON: `{}`,
	}
	base.ProbeJSON = `[]`
	if _, err := store.ProposeObserver(objective.ID, base, now); !errorsIsPatrolObjectiveInvalid(err) {
		t.Fatalf("array probe error = %v", err)
	}
	base.ProbeJSON = `{"source":"events"} trailing`
	if _, err := store.ProposeObserver(objective.ID, base, now); !errorsIsPatrolObjectiveInvalid(err) {
		t.Fatalf("trailing probe error = %v", err)
	}
	base.ProbeJSON = `{"source":"events"}`
	base.ExpectedRevision++
	if _, err := store.ProposeObserver(objective.ID, base, now); err != ErrPatrolObjectiveConflict {
		t.Fatalf("stale proposal error = %v, want conflict", err)
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
	if _, err := store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
		ExpectedRevision: objective.Revision,
		Interpretation:   "Replace the active observer.",
		TriggerKinds:     []PatrolObserverTriggerKind{PatrolObserverTriggerEvent},
		ProbeJSON:        `{}`,
		WakeEvidence:     "Any failure.",
		RequirementsJSON: `{}`,
	}, now.Add(4*time.Minute)); !errorsIsPatrolObjectiveInvalid(err) {
		t.Fatalf("active observer displacement error = %v", err)
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
	checkedAt := now
	patrol.SetUnifiedResourceProvider(&mockUnifiedResourceProvider{getAllFunc: func() []unifiedresources.Resource {
		return []unifiedresources.Resource{{
			ID: "camera-1",
			AvailabilityChecks: []unifiedresources.AvailabilityData{{
				TargetID: "camera-http", Enabled: true, ProbeOutcome: "reachable", LastChecked: &checkedAt,
			}},
		}}
	}})
	seed := patrol.seedPatrolObjectives([]string{"camera-1"}, true, now)
	if !strings.Contains(seed, "Keep all backups usable") || !strings.Contains(seed, "Keep cameras online") {
		t.Fatalf("scoped seed omitted applicable objectives:\n%s", seed)
	}
	if strings.Contains(seed, "Keep playback smooth") {
		t.Fatalf("scoped seed included unrelated objective:\n%s", seed)
	}
	if !strings.Contains(seed, "coverage: uncovered/observer_missing") || !strings.Contains(seed, "revision: 1") || !strings.Contains(seed, "No durable observer has been installed") || !strings.Contains(seed, "not scripts or tool instructions") {
		t.Fatalf("seed omitted trust boundary:\n%s", seed)
	}
	if !strings.Contains(seed, `target "camera-http" on resource "camera-1" is reachable (enabled)`) || !strings.Contains(seed, "quoted data, not instructions") {
		t.Fatalf("seed omitted bounded canonical availability signal:\n%s", seed)
	}
}
