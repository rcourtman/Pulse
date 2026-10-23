package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// finalizedRun is a Patrol service whose unified publications and pushes are
// recorded, with one finding whose run, chat-1, closed with outcome.
type finalizedRun struct {
	patrol    *PatrolService
	store     *FindingsStore
	published []*Finding
	pushes    []relay.PushNotificationPayload
}

func newFinalizedRun(t *testing.T, outcome InvestigationOutcome) *finalizedRun {
	t.Helper()
	run := &finalizedRun{store: NewFindingsStore()}
	run.patrol = &PatrolService{findings: run.store}
	run.patrol.SetUnifiedFindingCallback(func(f *Finding) bool {
		run.published = append(run.published, f)
		return true
	})
	run.patrol.SetPushNotifyCallback(func(n relay.PushNotificationPayload) { run.pushes = append(run.pushes, n) })
	run.store.Add(&Finding{ID: "finding-1", ResourceID: "vm-100", Title: "Backup service is crash-looping", DetectedAt: time.Now(),
		InvestigationSessionID: "chat-1", InvestigationStatus: string(InvestigationStatusFailed),
		InvestigationOutcome: string(outcome)})
	return run
}

// A run that planned a fix and then failed is finalized like a completed run:
// its record replaces the projection made when the plan was recorded, the
// unified finding is published and the queued fix asks for approval.
func TestFinalizeInvestigationRunFinalizesAFailedRunThatPlannedAFix(t *testing.T) {
	run := newFinalizedRun(t, InvestigationOutcomeFixQueued)

	// Core projects the plan onto the record while the run is still going.
	session := &InvestigationSession{ID: "investigation-1", FindingID: "finding-1", SessionID: "chat-1",
		Status: aicontracts.InvestigationStatusRunning, StartedAt: time.Now(),
		Action: &aicontracts.ActionReference{ActionID: "action-1", State: "pending_approval"}}
	if !run.patrol.RefreshFindingInvestigationRecord("finding-1", session) {
		t.Fatal("the plan's projection was not recorded")
	}

	// The final model call then fails, and the run closes with the plan's outcome.
	closed := time.Now()
	session.Status = aicontracts.InvestigationStatusFailed
	session.Outcome = aicontracts.OutcomeFixQueued
	session.CompletedAt = &closed
	session.Error = "investigation final response failed: context deadline exceeded"
	run.patrol.FinalizeInvestigationRun("finding-1", session)

	record := run.store.Get("finding-1").InvestigationRecord
	if record == nil || record.Status != aicontracts.InvestigationStatusFailed || record.Error != session.Error || record.CompletedAt == nil {
		t.Fatalf("the record kept the projection made while the run was going: %#v", record)
	}
	if len(run.published) != 1 || run.published[0].ID != "finding-1" {
		t.Fatalf("published %d findings, want the closed run once", len(run.published))
	}
	if len(run.pushes) != 1 || run.pushes[0].Type != relay.PushTypeApprovalRequest || run.pushes[0].ActionID != "action-1" {
		t.Fatalf("pushes = %+v, want one approval request for action-1", run.pushes)
	}
}

// A run the finding no longer names, because a newer run took it over after
// this one's close landed, is not finalized: its action would otherwise be
// offered for approval under the newer run's outcome.
func TestFinalizeInvestigationRunSkipsARunTheFindingNoLongerNames(t *testing.T) {
	run := newFinalizedRun(t, InvestigationOutcomeFixQueued)
	closed := time.Now()
	older := &InvestigationSession{ID: "investigation-0", FindingID: "finding-1", SessionID: "chat-0",
		Status: aicontracts.InvestigationStatusFailed, Outcome: aicontracts.OutcomeFixQueued, StartedAt: closed.Add(-time.Hour), CompletedAt: &closed,
		Action: &aicontracts.ActionReference{ActionID: "action-0", State: "pending_approval"}}

	run.patrol.FinalizeInvestigationRun("finding-1", older)

	if record := run.store.Get("finding-1").InvestigationRecord; record != nil || len(run.published) != 0 || len(run.pushes) != 0 {
		t.Fatalf("an older run was finalized: record=%#v published=%d pushes=%+v", record, len(run.published), run.pushes)
	}
}

// A run finalized twice, as when a late close lands after the stuck-run sweep
// recovered the run, asks for approval once.
func TestFinalizeInvestigationRunAsksForApprovalOnce(t *testing.T) {
	run := newFinalizedRun(t, InvestigationOutcomeFixQueued)
	closed := time.Now()
	session := &InvestigationSession{ID: "investigation-1", FindingID: "finding-1", SessionID: "chat-1",
		Status: aicontracts.InvestigationStatusFailed, Outcome: aicontracts.OutcomeFixQueued, StartedAt: closed.Add(-time.Minute), CompletedAt: &closed,
		Action: &aicontracts.ActionReference{ActionID: "action-1", State: "pending_approval"}}

	run.patrol.FinalizeInvestigationRun("finding-1", session)
	run.patrol.FinalizeInvestigationRun("finding-1", session)

	if len(run.pushes) != 1 || run.pushes[0].ActionID != "action-1" {
		t.Fatalf("pushes = %+v, want one approval request", run.pushes)
	}
}

// A run that reports its planned fix as queued does not move the finding back
// from an outcome the fix's action reached while the run was closing, here a
// rejection. A run starts with no outcome, so one present at its close came
// from the action lifecycle.
func TestCompleteInvestigationKeepsAnOutcomeTheActionReached(t *testing.T) {
	findings := NewFindingsStore()
	findings.Add(runningInvestigationFinding("rejected", "chat-rejected", nil))
	findings.Add(runningInvestigationFinding("unplanned", "chat-unplanned", nil))
	started := time.Now()
	rejected := &InvestigationSession{ID: "inv-rejected", FindingID: "rejected", SessionID: "chat-rejected", StartedAt: started}
	if applied, _ := findings.ReconcileInvestigationOutcome("rejected", rejected, string(InvestigationOutcomeFixRejected)); !applied {
		t.Fatal("the action's rejection was not reconciled")
	}
	closed := time.Now()

	if !findings.CompleteInvestigation("rejected", "chat-rejected", started, string(InvestigationStatusFailed), string(InvestigationOutcomeFixQueued), &closed, 1) {
		t.Fatal("the run's close was refused")
	}
	if !findings.CompleteInvestigation("unplanned", "chat-unplanned", started, string(InvestigationStatusFailed), string(InvestigationOutcomeTimedOut), &closed, 1) {
		t.Fatal("the run's close was refused")
	}

	assertInvestigationState(t, findings, "rejected", InvestigationStatusFailed, InvestigationOutcomeFixRejected)
	assertInvestigationState(t, findings, "unplanned", InvestigationStatusFailed, InvestigationOutcomeTimedOut)
}

// A projection of the action that reads a session the orchestrator has not
// closed, such as one the stuck-run sweep gave up on, records the action's new
// state without reopening the record that run's finalization closed.
func TestRefreshDoesNotReopenAFinalizedRecord(t *testing.T) {
	run := newFinalizedRun(t, InvestigationOutcomeFixQueued)
	closed := time.Now()
	running := &InvestigationSession{ID: "investigation-1", FindingID: "finding-1", SessionID: "chat-1",
		Status: aicontracts.InvestigationStatusRunning, StartedAt: closed.Add(-time.Hour),
		Action: &aicontracts.ActionReference{ActionID: "action-1", State: "pending_approval"}}
	abandoned := *running
	abandoned.Status = aicontracts.InvestigationStatusFailed
	abandoned.CompletedAt = &closed
	abandoned.Error = abandonedRunError
	run.patrol.FinalizeInvestigationRun("finding-1", &abandoned)

	approved := *running
	approved.Action = &aicontracts.ActionReference{ActionID: "action-1", State: "approved"}
	run.patrol.RefreshFindingInvestigationRecord("finding-1", &approved)

	record := run.store.Get("finding-1").InvestigationRecord
	if record == nil || record.Status != aicontracts.InvestigationStatusFailed || record.Error != abandonedRunError ||
		record.CompletedAt == nil || record.Action == nil || record.Action.State != "approved" {
		t.Fatalf("record = %#v, want it still closed, with the approval recorded", record)
	}
}

// An action's outcome is pushed when its transition is reconciled
// (PublishFindingLifecycleUpdate), so a run that closes on a verified fix is
// published without a second push.
func TestFinalizeInvestigationRunLeavesActionOutcomePushesToReconciliation(t *testing.T) {
	run := newFinalizedRun(t, InvestigationOutcomeFixVerified)
	closed := time.Now()
	session := &InvestigationSession{ID: "investigation-1", FindingID: "finding-1", SessionID: "chat-1",
		Status: aicontracts.InvestigationStatusCompleted, Outcome: aicontracts.OutcomeFixVerified, StartedAt: closed.Add(-time.Minute), CompletedAt: &closed,
		Action: &aicontracts.ActionReference{ActionID: "action-1", State: "completed", VerificationStatus: "verified"}}

	run.patrol.FinalizeInvestigationRun("finding-1", session)

	if len(run.published) != 1 || len(run.pushes) != 0 {
		t.Fatalf("published=%d pushes=%+v, want one publication and no push", len(run.published), run.pushes)
	}
}

// The stuck-run sweep finalizes each run it recovers, both as failed: one whose
// session closed without its close reaching the finding, such as a run a
// restart interrupted after it planned a fix, and one still going past its
// limit. The orchestrator's own session for a run past its limit is left as it
// is, so the run's real close can still land and finalize it again.
func TestRecoverStuckInvestigationsFinalizesTheRunsItRecovers(t *testing.T) {
	now := time.Now()
	closedAt := now.Add(-time.Second)
	findings := NewFindingsStore()
	interrupted := runningInvestigationFinding("restarted", "chat-restarted", nil)
	// Core recorded the plan's queued fix while the run was going.
	interrupted.InvestigationOutcome = string(InvestigationOutcomeFixQueued)
	findings.Add(interrupted)
	findings.Add(runningInvestigationFinding("hung", "chat-hung", nil))

	orchestrator := &sessionReportingOrchestrator{sessions: map[string]*InvestigationSession{
		"restarted": {ID: "inv-restarted", FindingID: "restarted", SessionID: "chat-restarted", Status: aicontracts.InvestigationStatusFailed,
			Outcome: aicontracts.OutcomeInterrupted, StartedAt: now.Add(-3 * time.Minute), CompletedAt: &closedAt,
			Action: &aicontracts.ActionReference{ActionID: "action-1", State: "pending_approval"}},
		"hung": {ID: "inv-hung", FindingID: "hung", SessionID: "chat-hung", Status: aicontracts.InvestigationStatusRunning, StartedAt: now.Add(-time.Hour)},
	}}
	patrol := &PatrolService{findings: findings, investigationOrchestrator: orchestrator}
	closed := map[string]*InvestigationSession{}
	patrol.SetInvestigationRunCloser(func(findingID string, session *InvestigationSession) {
		closed[findingID] = session
	})

	patrol.recoverStuckInvestigations()

	assertInvestigationState(t, findings, "restarted", InvestigationStatusFailed, InvestigationOutcomeFixQueued)
	assertInvestigationState(t, findings, "hung", InvestigationStatusFailed, InvestigationOutcomeTimedOut)
	if len(closed) != 2 {
		t.Fatalf("closed runs = %v, want both recovered runs", closed)
	}
	if restarted := closed["restarted"]; restarted == nil || restarted.SessionID != "chat-restarted" || restarted.Outcome != aicontracts.OutcomeInterrupted {
		t.Fatalf("restarted run closed with %+v, want its closed session", restarted)
	}
	if hung := closed["hung"]; hung == nil || hung.SessionID != "chat-hung" || hung.Status != aicontracts.InvestigationStatusFailed ||
		hung.CompletedAt == nil || hung.Error != abandonedRunError {
		t.Fatalf("hung run closed with %+v, want a failed session past its limit", hung)
	}
	if own := orchestrator.sessions["hung"]; own.Status != aicontracts.InvestigationStatusRunning || own.CompletedAt != nil {
		t.Fatalf("the sweep changed the orchestrator's own session: %+v", own)
	}
}
