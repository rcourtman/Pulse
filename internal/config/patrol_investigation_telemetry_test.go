package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

func closedMeasurement(id string, at time.Time, outcome aicontracts.InvestigationOutcome) *aicontracts.InvestigationSession {
	return &aicontracts.InvestigationSession{ID: id, FindingID: "customer-secret-finding", Status: aicontracts.InvestigationStatusCompleted, StartedAt: at.Add(-time.Minute), CompletedAt: &at, Outcome: outcome, Summary: "customer-secret-summary", Error: "customer-secret-error", HandoffReason: aicontracts.HandoffMissingEvidence}
}

func TestPatrolInvestigationTelemetrySurvivesCleanupRestartAndActionUpdates(t *testing.T) {
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), "absent-key"))
	dir := t.TempDir()
	p := NewConfigPersistence(dir)
	at := time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)
	if got := p.PatrolInvestigationTelemetry(at); got.Available {
		t.Fatal("absent ledger reported measured zero")
	}
	if err := p.EnsurePatrolInvestigationTelemetry(at); err != nil {
		t.Fatal(err)
	}
	session := closedMeasurement("customer-secret-run", at.Add(time.Minute), aicontracts.OutcomeFixQueued)
	for i := 0; i < 3; i++ {
		if err := p.RecordPatrolInvestigationMeasurement("customer-secret-tenant", session, at.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	// Deleting the entire findings history cannot erase independent run history.
	if err := p.SaveAIFindings(map[string]*AIFindingRecord{}); err != nil {
		t.Fatal(err)
	}
	p = NewConfigPersistence(dir)
	session.Outcome = aicontracts.OutcomeFixVerified
	if err := p.RecordPatrolInvestigationMeasurement("customer-secret-tenant", session, at.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	got := p.PatrolInvestigationTelemetry(at.Add(48 * time.Hour))
	if !got.Available || got.Runs != 1 || got.Outcomes["fix_verified"] != 1 || got.Outcomes["fix_queued"] != 0 || got.ObservationDays != 2 || len(got.Handoffs) != 0 {
		t.Fatalf("after cleanup/restart/verification: %+v", got)
	}
	raw, err := os.ReadFile(filepath.Join(dir, patrolInvestigationTelemetryFile))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "customer-secret") {
		t.Fatalf("customer content retained: %s", raw)
	}
	info, err := os.Stat(filepath.Join(dir, patrolInvestigationTelemetryFile))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("permissions: %v %v", info, err)
	}
	// A verification update does not extend the run's original 30-day window.
	got = p.PatrolInvestigationTelemetry(at.Add(31 * 24 * time.Hour))
	if !got.Available || got.Runs != 0 || got.ObservationDays != 30 {
		t.Fatalf("expired runs: %+v", got)
	}
}

func TestPatrolInvestigationTelemetryCountsRetriesAndTenantsSeparately(t *testing.T) {
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), "absent-key"))
	dir := t.TempDir()
	p := NewConfigPersistence(dir)
	at := time.Now().UTC().Add(-time.Hour)
	if err := p.EnsurePatrolInvestigationTelemetry(at); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			instance := NewConfigPersistence(dir)
			session := closedMeasurement(fmt.Sprintf("retry-%d", i%12), at.Add(time.Minute), aicontracts.OutcomeNeedsAttention)
			if err := instance.RecordPatrolInvestigationMeasurement(fmt.Sprintf("tenant-%d", i/12), session, at.Add(time.Hour)); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	got := p.PatrolInvestigationTelemetry(at.Add(time.Hour))
	if !got.Available || got.Runs != 24 || got.Handoffs[aicontracts.HandoffMissingEvidence] != 24 {
		t.Fatalf("distinct tenant/run counts: %+v", got)
	}
}

func TestPatrolInvestigationTelemetryCoverageAndFailedWrites(t *testing.T) {
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), "absent-key"))
	dir := t.TempDir()
	p := NewConfigPersistence(dir)
	at := time.Now().UTC().Add(-time.Hour)
	if err := p.EnsurePatrolInvestigationTelemetry(at); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, patrolInvestigationTelemetryFile)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("corrupt-customer-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := p.EnsurePatrolInvestigationTelemetry(at); err == nil {
		t.Fatal("corrupt ledger reset")
	}
	s := closedMeasurement("first", at.Add(time.Minute), aicontracts.OutcomeCannotFix)
	s.HandoffReason = "customer-secret-reason"
	if err := p.RecordPatrolInvestigationMeasurement("tenant", s, at.Add(time.Hour)); err == nil {
		t.Fatal("corrupt read accepted")
	}
	if p.PatrolInvestigationTelemetry(at.Add(time.Hour)).Available {
		t.Fatal("failed observation reported available")
	}
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	// A later write retries all pending observations, not just its own event.
	if err := p.RecordPatrolInvestigationMeasurement("tenant", closedMeasurement("second", at.Add(2*time.Minute), aicontracts.OutcomeResolved), at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got := p.PatrolInvestigationTelemetry(at.Add(time.Hour))
	if !got.Available || got.Runs != 2 || got.Handoffs[aicontracts.HandoffUnspecified] != 1 {
		t.Fatalf("pending recovery: %+v", got)
	}
}

func TestPatrolInvestigationTelemetryStartsWithoutHistoricalBackfill(t *testing.T) {
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), "absent-key"))
	p := NewConfigPersistence(t.TempDir())
	at := time.Now().UTC()
	if err := p.EnsurePatrolInvestigationTelemetry(at); err != nil {
		t.Fatal(err)
	}
	if err := p.RecordPatrolInvestigationMeasurement("tenant", closedMeasurement("old", at.Add(-time.Hour), aicontracts.OutcomeNeedsAttention), at); err != nil {
		t.Fatal(err)
	}
	if err := p.RecordPatrolInvestigationMeasurement("tenant", &aicontracts.InvestigationSession{ID: "running", Status: aicontracts.InvestigationStatusRunning}, at); err != nil {
		t.Fatal(err)
	}
	got := p.PatrolInvestigationTelemetry(at)
	if !got.Available || got.Runs != 0 || got.ObservationDays != 0 {
		t.Fatalf("new observation: %+v", got)
	}
	// Re-initializing on restart must retain the original coverage start.
	if err := p.EnsurePatrolInvestigationTelemetry(at.Add(48 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	if p.PatrolInvestigationTelemetry(at.Add(48*time.Hour)).ObservationDays != 2 {
		t.Fatal("observation start moved")
	}
}

func TestPatrolInvestigationTelemetryUsesObjectiveAccessFacts(t *testing.T) {
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), "absent-key"))
	p := NewConfigPersistence(t.TempDir())
	at := time.Now().UTC().Add(-time.Hour)
	if err := p.EnsurePatrolInvestigationTelemetry(at); err != nil {
		t.Fatal(err)
	}
	s := closedMeasurement("run", at.Add(time.Minute), aicontracts.OutcomeNeedsAttention)
	s.CommandsDisabled = &aicontracts.CommandsDisabledHost{}
	if err := p.RecordPatrolInvestigationMeasurement("tenant", s, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if p.PatrolInvestigationTelemetry(at.Add(time.Hour)).Handoffs[aicontracts.HandoffCommandsDisabled] != 1 {
		t.Fatal("model label overrode commands-off fact")
	}
}

type failInvestigationLedgerRename struct{ FileSystem }

func (f failInvestigationLedgerRename) Rename(oldPath, newPath string) error { return os.ErrPermission }

func TestPatrolInvestigationTelemetryFailedWriteIsUnmeasuredUntilRetried(t *testing.T) {
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), "absent-key"))
	p := NewConfigPersistence(t.TempDir())
	at := time.Now().UTC().Add(-time.Hour)
	if err := p.EnsurePatrolInvestigationTelemetry(at); err != nil {
		t.Fatal(err)
	}
	originalFS := p.fs
	p.fs = failInvestigationLedgerRename{originalFS}
	if err := p.RecordPatrolInvestigationMeasurement("tenant", closedMeasurement("run", at.Add(time.Minute), aicontracts.OutcomeResolved), at.Add(time.Hour)); err == nil {
		t.Fatal("failed atomic replace accepted")
	}
	if p.PatrolInvestigationTelemetry(at.Add(time.Hour)).Available {
		t.Fatal("lost write reported a complete observation")
	}
	p.fs = originalFS
	if err := p.RecordPatrolInvestigationMeasurements("tenant", nil, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got := p.PatrolInvestigationTelemetry(at.Add(time.Hour))
	if !got.Available || got.Runs != 1 || got.Outcomes["resolved"] != 1 {
		t.Fatalf("retry did not recover the observation: %+v", got)
	}
}

func TestPatrolInvestigationTelemetryIsNotRolledBackWithConfiguration(t *testing.T) {
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), "absent-key"))
	p := NewConfigPersistence(t.TempDir())
	at := time.Now().UTC().Add(-time.Hour)
	if err := p.EnsurePatrolInvestigationTelemetry(at); err != nil {
		t.Fatal(err)
	}
	tx, err := newImportTransaction(p.configDir)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Cleanup()
	p.beginTransaction(tx)
	if err := p.RecordPatrolInvestigationMeasurement("tenant", closedMeasurement("run", at.Add(time.Minute), aicontracts.OutcomeResolved), at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	p.endTransaction(tx)
	tx.Rollback()
	got := p.PatrolInvestigationTelemetry(at.Add(time.Hour))
	if !got.Available || got.Runs != 1 {
		t.Fatalf("unrelated rollback erased history: %+v", got)
	}
}
