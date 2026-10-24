package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

type sessionReportingOrchestrator struct {
	stubInvestigationOrchestrator
	sessions map[string]*InvestigationSession
	store    aicontracts.InvestigationStore
}

func (s *sessionReportingOrchestrator) GetInvestigationByFinding(findingID string) *InvestigationSession {
	return s.sessions[findingID]
}

func (s *sessionReportingOrchestrator) GetStore() aicontracts.InvestigationStore {
	return s.store
}

// findingSessionsStore is a session store that only answers which sessions a
// finding has recorded, the one query the stuck-run sweep makes.
type findingSessionsStore struct {
	aicontracts.InvestigationStore
	byFinding map[string][]*InvestigationSession
}

func (s findingSessionsStore) GetByFinding(findingID string) []*InvestigationSession {
	return s.byFinding[findingID]
}

func runningInvestigationFinding(id, sessionID string, lastInvestigated *time.Time) *Finding {
	return &Finding{
		ID:                     id,
		ResourceID:             "vm:" + id,
		ResourceType:           "vm",
		Title:                  "Investigation " + id,
		Severity:               FindingSeverityWarning,
		Category:               FindingCategoryReliability,
		InvestigationSessionID: sessionID,
		InvestigationStatus:    string(InvestigationStatusRunning),
		LastInvestigatedAt:     lastInvestigated,
		InvestigationAttempts:  1,
	}
}

func assertInvestigationState(t *testing.T, findings *FindingsStore, id string, status InvestigationStatus, outcome InvestigationOutcome) {
	t.Helper()
	got := findings.Get(id)
	if got.InvestigationStatus != string(status) || got.InvestigationOutcome != string(outcome) {
		t.Fatalf("%s = %s/%s, want %s/%s", id, got.InvestigationStatus, got.InvestigationOutcome, status, outcome)
	}
}

func TestRecoverStuckInvestigationsMeasuresLiveRunsFromTheirOwnStart(t *testing.T) {
	now := time.Now()
	previousRun := now.Add(-2 * time.Hour)
	findings := NewFindingsStore()
	// A re-investigation still running: LastInvestigatedAt is the previous run.
	findings.Add(runningInvestigationFinding("live", "chat-live", &previousRun))
	// A first investigation whose run outlived its ceiling.
	findings.Add(runningInvestigationFinding("stuck", "chat-stuck", nil))

	orchestrator := &sessionReportingOrchestrator{sessions: map[string]*InvestigationSession{
		"live":  {ID: "inv-live", FindingID: "live", SessionID: "chat-live", Status: aicontracts.InvestigationStatusRunning, StartedAt: now.Add(-time.Minute)},
		"stuck": {ID: "inv-stuck", FindingID: "stuck", SessionID: "chat-stuck", Status: aicontracts.InvestigationStatusRunning, StartedAt: now.Add(-time.Hour)},
	}}
	patrol := &PatrolService{findings: findings, investigationOrchestrator: orchestrator}
	patrol.recoverStuckInvestigations()

	assertInvestigationState(t, findings, "live", InvestigationStatusRunning, "")
	assertInvestigationState(t, findings, "stuck", InvestigationStatusFailed, InvestigationOutcomeTimedOut)
}

func TestRecoverStuckInvestigationsKeepsALiveRunToItsOwnDeadline(t *testing.T) {
	now := time.Now()
	findings := NewFindingsStore()
	findings.Add(runningInvestigationFinding("long", "chat-long", nil))
	findings.Add(runningInvestigationFinding("overdue", "chat-overdue", nil))

	// Both runs started 16 minutes ago under a 25-minute timeout that has
	// since been lowered to the 10-minute default, a 15-minute ceiling.
	started := now.Add(-16 * time.Minute)
	longDeadline := started.Add(25 * time.Minute)
	overdueDeadline := now.Add(-investigationRunGrace - time.Minute)
	orchestrator := &sessionReportingOrchestrator{sessions: map[string]*InvestigationSession{
		"long":    {ID: "inv-long", FindingID: "long", SessionID: "chat-long", Status: aicontracts.InvestigationStatusRunning, StartedAt: started, Deadline: &longDeadline},
		"overdue": {ID: "inv-overdue", FindingID: "overdue", SessionID: "chat-overdue", Status: aicontracts.InvestigationStatusRunning, StartedAt: started, Deadline: &overdueDeadline},
	}}
	patrol := &PatrolService{findings: findings, investigationOrchestrator: orchestrator}
	patrol.recoverStuckInvestigations()

	assertInvestigationState(t, findings, "long", InvestigationStatusRunning, "")
	assertInvestigationState(t, findings, "overdue", InvestigationStatusFailed, InvestigationOutcomeTimedOut)
}

func TestRecoverStuckInvestigationsRecordsRestartsAsInterrupted(t *testing.T) {
	now := time.Now()
	previousRun := now.Add(-2 * time.Hour)
	justClosed := now.Add(-time.Second)
	closedAWhileAgo := now.Add(-investigationRunGrace - time.Minute)
	findings := NewFindingsStore()
	// Closed as interrupted when the orchestrator reloaded after a restart.
	findings.Add(runningInvestigationFinding("restarted", "chat-restarted", &previousRun))
	// A first run the orchestrator holds no record of. Before, nothing ever
	// released it, which also blocked a manual re-run.
	findings.Add(runningInvestigationFinding("orphan", "chat-gone", nil))
	// Closed with a result moments ago; the orchestrator's own update is due.
	findings.Add(runningInvestigationFinding("settling", "chat-settling", nil))
	// Closed without a result long enough ago that no update is coming.
	findings.Add(runningInvestigationFinding("abandoned", "chat-abandoned", nil))

	orchestrator := &sessionReportingOrchestrator{sessions: map[string]*InvestigationSession{
		"restarted": {ID: "inv-restarted", FindingID: "restarted", SessionID: "chat-restarted", Status: aicontracts.InvestigationStatusFailed, Outcome: aicontracts.OutcomeInterrupted, StartedAt: now.Add(-3 * time.Minute), CompletedAt: &justClosed},
		"settling":  {ID: "inv-settling", FindingID: "settling", SessionID: "chat-settling", Status: aicontracts.InvestigationStatusCompleted, Outcome: aicontracts.OutcomeResolved, StartedAt: now.Add(-3 * time.Minute), CompletedAt: &justClosed},
		"abandoned": {ID: "inv-abandoned", FindingID: "abandoned", SessionID: "chat-abandoned", Status: aicontracts.InvestigationStatusFailed, StartedAt: now.Add(-time.Hour), CompletedAt: &closedAWhileAgo},
	}}
	patrol := &PatrolService{findings: findings, investigationOrchestrator: orchestrator}
	patrol.recoverStuckInvestigations()

	assertInvestigationState(t, findings, "restarted", InvestigationStatusFailed, InvestigationOutcomeInterrupted)
	assertInvestigationState(t, findings, "orphan", InvestigationStatusFailed, InvestigationOutcomeInterrupted)
	assertInvestigationState(t, findings, "settling", InvestigationStatusRunning, "")
	assertInvestigationState(t, findings, "abandoned", InvestigationStatusFailed, InvestigationOutcomeTimedOut)
	// The interrupted run still spends its attempt, so a run that crashes
	// Pulse cannot retry forever.
	if got := findings.Get("orphan").InvestigationAttempts; got != 1 {
		t.Fatalf("orphan attempts = %d, want 1", got)
	}
	if !findings.Get("orphan").ShouldInvestigate("approval") {
		t.Fatal("an interrupted first run must be eligible for another investigation")
	}
	// The close counts as the latest investigation, so the digest sees it.
	if got := findings.Get("orphan").LastInvestigatedAt; got == nil || got.Before(now) {
		t.Fatalf("orphan LastInvestigatedAt = %v, want the recovery time", got)
	}
}

func TestRecoverStuckInvestigationsKeepsAnActionOutcome(t *testing.T) {
	now := time.Now()
	closedAWhileAgo := now.Add(-investigationRunGrace - time.Minute)
	findings := NewFindingsStore()
	for _, id := range []string{"queued", "queued-interrupted", "queued-closed"} {
		f := runningInvestigationFinding(id, "chat-"+id, nil)
		// The action lifecycle recorded the queued fix while the run was
		// still running, and a restart came before the run finished.
		f.InvestigationOutcome = string(InvestigationOutcomeFixQueued)
		findings.Add(f)
	}
	orchestrator := &sessionReportingOrchestrator{sessions: map[string]*InvestigationSession{
		"queued":             {ID: "inv-queued", FindingID: "queued", SessionID: "chat-queued", Status: aicontracts.InvestigationStatusFailed, Outcome: aicontracts.OutcomeFixQueued, StartedAt: now.Add(-time.Hour), CompletedAt: &closedAWhileAgo},
		"queued-interrupted": {ID: "inv-queued-interrupted", FindingID: "queued-interrupted", SessionID: "chat-queued-interrupted", Status: aicontracts.InvestigationStatusFailed, Outcome: aicontracts.OutcomeInterrupted, StartedAt: now.Add(-time.Hour), CompletedAt: &closedAWhileAgo},
		// This run closed itself, but its update never reached the finding.
		"queued-closed": {ID: "inv-queued-closed", FindingID: "queued-closed", SessionID: "chat-queued-closed", Status: aicontracts.InvestigationStatusCompleted, Outcome: aicontracts.OutcomeFixQueued, StartedAt: now.Add(-time.Hour), CompletedAt: &closedAWhileAgo},
	}}
	patrol := &PatrolService{findings: findings, investigationOrchestrator: orchestrator}
	patrol.recoverStuckInvestigations()

	// Each run keeps the status it closed with: a run cut short after it
	// planned the fix failed, as it reads when such a run closes itself.
	for id, status := range map[string]InvestigationStatus{
		"queued":             InvestigationStatusFailed,
		"queued-interrupted": InvestigationStatusFailed,
		"queued-closed":      InvestigationStatusCompleted,
	} {
		assertInvestigationState(t, findings, id, status, InvestigationOutcomeFixQueued)
		if findings.Get(id).ShouldInvestigate("approval") {
			t.Fatalf("%s: a queued fix must keep blocking another investigation", id)
		}
	}
}

func TestRecoverStuckInvestigationOnlyClosesTheRunItWasGiven(t *testing.T) {
	now := time.Now()
	findings := NewFindingsStore()
	// A newer run started after the sweep read the finding.
	findings.Add(runningInvestigationFinding("moved-on", "chat-newer", nil))
	// The run finished on its own after the sweep read the finding.
	finished := runningInvestigationFinding("finished", "chat-finished", nil)
	finished.InvestigationStatus = string(InvestigationStatusCompleted)
	finished.InvestigationOutcome = string(InvestigationOutcomeResolved)
	findings.Add(finished)

	if findings.RecoverStuckInvestigation("moved-on", "chat-older", time.Time{}, InvestigationStatusFailed, InvestigationOutcomeInterrupted, now) {
		t.Fatal("recovering an older run overwrote the newer one")
	}
	assertInvestigationState(t, findings, "moved-on", InvestigationStatusRunning, "")
	if findings.RecoverStuckInvestigation("finished", "chat-finished", time.Time{}, InvestigationStatusFailed, InvestigationOutcomeTimedOut, now) {
		t.Fatal("recovery overwrote a finished run")
	}
	assertInvestigationState(t, findings, "finished", InvestigationStatusCompleted, InvestigationOutcomeResolved)
	if findings.RecoverStuckInvestigation("missing", "chat-missing", time.Time{}, InvestigationStatusFailed, InvestigationOutcomeTimedOut, now) {
		t.Fatal("recovery reported success for an unknown finding")
	}
}

func TestRecoverStuckInvestigationsKeepsALiveRunBehindANewerSession(t *testing.T) {
	now := time.Now()
	findings := NewFindingsStore()
	findings.Add(runningInvestigationFinding("running", "chat-running", nil))
	findings.Add(runningInvestigationFinding("pending", "chat-pending", nil))
	// A second admission for each finding created its session but has not
	// marked the finding yet, so it is the finding's latest session.
	newer := func(id string) *InvestigationSession {
		return &InvestigationSession{ID: "inv-newer-" + id, FindingID: id, SessionID: "chat-newer-" + id, Status: aicontracts.InvestigationStatusPending, StartedAt: now}
	}
	store := findingSessionsStore{byFinding: map[string][]*InvestigationSession{
		"running": {{ID: "inv-running", FindingID: "running", SessionID: "chat-running", Status: aicontracts.InvestigationStatusRunning, StartedAt: now.Add(-time.Minute)}, newer("running")},
		// Marked on the finding, not yet running in the store.
		"pending": {{ID: "inv-pending", FindingID: "pending", SessionID: "chat-pending", Status: aicontracts.InvestigationStatusPending, StartedAt: now.Add(-time.Minute)}, newer("pending")},
	}}
	orchestrator := &sessionReportingOrchestrator{
		sessions: map[string]*InvestigationSession{"running": newer("running"), "pending": newer("pending")},
		store:    store,
	}
	patrol := &PatrolService{findings: findings, investigationOrchestrator: orchestrator}
	patrol.recoverStuckInvestigations()

	assertInvestigationState(t, findings, "running", InvestigationStatusRunning, "")
	assertInvestigationState(t, findings, "pending", InvestigationStatusRunning, "")

	// Without the store, a newer latest session leaves the named run unknown,
	// and the sweep leaves it alone rather than guess.
	patrol.investigationOrchestrator = &sessionReportingOrchestrator{sessions: orchestrator.sessions}
	patrol.recoverStuckInvestigations()
	assertInvestigationState(t, findings, "running", InvestigationStatusRunning, "")
}

func TestRecoverStuckInvestigationsKeepsAResultFromBeforeARegression(t *testing.T) {
	now := time.Now()
	closedAWhileAgo := now.Add(-investigationRunGrace - time.Minute)
	regressedAt := now.Add(-10 * time.Minute)
	findings := NewFindingsStore()
	f := runningInvestigationFinding("regressed", "chat-old", nil)
	f.LastRegressionAt = &regressedAt
	findings.Add(f)
	// The run started before the finding regressed, and its session recorded
	// a verified fix for the earlier occurrence.
	store := findingSessionsStore{byFinding: map[string][]*InvestigationSession{
		"regressed": {{ID: "inv-old", FindingID: "regressed", SessionID: "chat-old", Status: aicontracts.InvestigationStatusCompleted, Outcome: aicontracts.OutcomeFixVerified, StartedAt: regressedAt.Add(-10 * time.Minute), CompletedAt: &closedAWhileAgo}},
	}}
	patrol := &PatrolService{findings: findings, investigationOrchestrator: &sessionReportingOrchestrator{store: store}}
	patrol.recoverStuckInvestigations()

	assertInvestigationState(t, findings, "regressed", InvestigationStatusFailed, InvestigationOutcomeInterrupted)
	if got := findings.Get("regressed"); got.ResolvedAt != nil {
		t.Fatal("an earlier occurrence's verified fix resolved the current one")
	}
}

func TestRecoverStuckInvestigationsAppliesTheSessionResult(t *testing.T) {
	now := time.Now()
	closedAWhileAgo := now.Add(-investigationRunGrace - time.Minute)
	findings := NewFindingsStore()
	// The session store saved the run's result; the finding store did not
	// before a crash.
	findings.Add(runningInvestigationFinding("resolved", "chat-resolved", nil))
	findings.Add(runningInvestigationFinding("queued", "chat-queued", nil))
	orchestrator := &sessionReportingOrchestrator{sessions: map[string]*InvestigationSession{
		"resolved": {ID: "inv-resolved", FindingID: "resolved", SessionID: "chat-resolved", Status: aicontracts.InvestigationStatusCompleted, Outcome: aicontracts.OutcomeResolved, StartedAt: now.Add(-time.Hour), CompletedAt: &closedAWhileAgo},
		// Closed at load after a restart while its proposal was being submitted.
		"queued": {ID: "inv-queued", FindingID: "queued", SessionID: "chat-queued", Status: aicontracts.InvestigationStatusFailed, Outcome: aicontracts.OutcomeFixQueued, StartedAt: now.Add(-time.Hour), CompletedAt: &closedAWhileAgo},
	}}
	patrol := &PatrolService{findings: findings, investigationOrchestrator: orchestrator}
	patrol.recoverStuckInvestigations()

	assertInvestigationState(t, findings, "resolved", InvestigationStatusCompleted, InvestigationOutcomeResolved)
	assertInvestigationState(t, findings, "queued", InvestigationStatusFailed, InvestigationOutcomeFixQueued)
	for _, id := range []string{"resolved", "queued"} {
		if findings.Get(id).ShouldInvestigate("approval") {
			t.Fatalf("%s: the run's recorded result must not be retried as a timeout", id)
		}
	}
}

func TestRecoverStuckInvestigationsWithoutOrchestratorUsesLastInvestigation(t *testing.T) {
	now := time.Now()
	previousRun := now.Add(-2 * time.Hour)
	findings := NewFindingsStore()
	findings.Add(runningInvestigationFinding("old", "chat-old", &previousRun))
	findings.Add(runningInvestigationFinding("unknown", "chat-unknown", nil))
	patrol := &PatrolService{findings: findings}
	patrol.recoverStuckInvestigations()

	assertInvestigationState(t, findings, "old", InvestigationStatusFailed, InvestigationOutcomeTimedOut)
	assertInvestigationState(t, findings, "unknown", InvestigationStatusRunning, "")
}

func TestInvestigationRunCeilingFollowsTheConfiguredTimeout(t *testing.T) {
	if got, want := (&PatrolService{}).investigationRunCeiling(), 10*time.Minute+investigationRunGrace; got != want {
		t.Fatalf("default ceiling = %s, want %s", got, want)
	}
	cfg := config.NewDefaultAIConfig()
	cfg.PatrolModel = config.AIProviderCodexSubscription + ":gpt-5.6-luna"
	patrol := &PatrolService{aiService: &Service{cfg: cfg}}
	if got, want := patrol.investigationRunCeiling(), time.Duration(config.DefaultPatrolSubscriptionInvestigationTimeoutSec)*time.Second+investigationRunGrace; got != want {
		t.Fatalf("subscription ceiling = %s, want %s", got, want)
	}
}
