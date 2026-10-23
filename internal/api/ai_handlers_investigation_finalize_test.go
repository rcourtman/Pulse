package api

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// runFinalization records what finalizing a closed run sent out.
type runFinalization struct {
	published []*ai.Finding
	pushes    []relay.PushNotificationPayload
}

// watchRunFinalization captures the Patrol service's unified publications and
// pushes. Set it before adding findings: setting the unified callback syncs
// the active findings at once.
func watchRunFinalization(patrol *ai.PatrolService) *runFinalization {
	sent := &runFinalization{}
	patrol.SetUnifiedFindingCallback(func(f *ai.Finding) bool {
		sent.published = append(sent.published, f)
		return true
	})
	patrol.SetPushNotifyCallback(func(n relay.PushNotificationPayload) {
		sent.pushes = append(sent.pushes, n)
	})
	return sent
}

// A run is finalized when its close lands, whether it completed or failed: its
// record shows the run's final state, the unified finding is published once and
// a planned fix awaiting approval asks the operator to approve it. A run that
// planned a fix and then failed, normally on its final model call, used to get
// none of this. An action's outcome is pushed when its transition is
// reconciled, so a close does not push it again.
func TestOrchestratorRunCloseFinalizesTheRun(t *testing.T) {
	cases := []struct {
		name     string
		status   aicontracts.InvestigationStatus
		outcome  aicontracts.InvestigationOutcome
		fail     string
		action   *aicontracts.ActionReference
		approval bool
	}{
		{
			name:     "failed after planning a fix",
			status:   aicontracts.InvestigationStatusFailed,
			outcome:  aicontracts.OutcomeFixQueued,
			fail:     "investigation final response failed: context deadline exceeded",
			action:   &aicontracts.ActionReference{ActionID: "action-1", State: "pending_approval"},
			approval: true,
		},
		{
			name:     "completed with a planned fix",
			status:   aicontracts.InvestigationStatusCompleted,
			outcome:  aicontracts.OutcomeFixQueued,
			action:   &aicontracts.ActionReference{ActionID: "action-1", State: "pending_approval"},
			approval: true,
		},
		{
			name:    "completed with a verified fix",
			status:  aicontracts.InvestigationStatusCompleted,
			outcome: aicontracts.OutcomeFixVerified,
			action:  &aicontracts.ActionReference{ActionID: "action-1", State: "completed", VerificationStatus: "verified"},
		},
		{
			name:    "timed out without a plan",
			status:  aicontracts.InvestigationStatusFailed,
			outcome: aicontracts.OutcomeTimedOut,
			fail:    "context deadline exceeded",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orchestratorFindings, sessions, patrol := orchestratorPatrolFor(t, nil)
			sent := watchRunFinalization(patrol)
			findings := patrol.GetFindings()
			findings.Add(investigatedFinding("finding-1"))
			run := startOrchestratorRun(t, orchestratorFindings, sessions, "finding-1", "chat-1")
			if len(sent.published) != 0 || len(sent.pushes) != 0 {
				t.Fatalf("starting a run finalized it: %+v", sent)
			}

			// The orchestrator records the run's end on its session before
			// it closes the run on the finding.
			session := sessions.GetLatestByFinding("finding-1")
			session.Action = tc.action
			sessions.Update(session)
			if tc.fail != "" {
				sessions.Fail(session.ID, tc.fail)
				sessions.SetOutcome(session.ID, tc.outcome)
			} else {
				sessions.Complete(session.ID, tc.outcome, "The backup service exits on a stale mount.", nil)
			}
			if !finishOrchestratorRun(orchestratorFindings, run, tc.status, tc.outcome) {
				t.Fatal("the run's close was refused")
			}

			record := findings.Get("finding-1").InvestigationRecord
			if record == nil || record.Status != tc.status || record.Outcome != tc.outcome || record.CompletedAt == nil || record.Error != tc.fail {
				t.Fatalf("record does not show the closed run: %+v", record)
			}
			if tc.action != nil && (record.Action == nil || record.Action.ActionID != tc.action.ActionID) {
				t.Fatalf("record lost the planned action: %+v", record.Action)
			}
			if len(sent.published) != 1 || sent.published[0].InvestigationOutcome != string(tc.outcome) {
				t.Fatalf("published %d findings, want the closed run once: %+v", len(sent.published), sent.published)
			}
			switch {
			case tc.approval && (len(sent.pushes) != 1 || sent.pushes[0].Type != relay.PushTypeApprovalRequest || sent.pushes[0].ActionID != tc.action.ActionID):
				t.Fatalf("pushes = %+v, want one approval request for %s", sent.pushes, tc.action.ActionID)
			case !tc.approval && len(sent.pushes) != 0:
				t.Fatalf("pushes = %+v, want none", sent.pushes)
			}
		})
	}
}

// The orchestrator writes a run's action back as it was planned. When the plan
// moved on while the final model call was pending, here by expiring, the close
// takes the action from the action audit: the record shows the expired plan and
// nobody is asked to approve it.
func TestOrchestratorRunCloseTakesTheActionFromTheAudit(t *testing.T) {
	audits := unifiedresources.NewMemoryStore()
	orchestratorFindings, sessions, patrol := orchestratorPatrolFor(t, audits)
	sent := watchRunFinalization(patrol)
	findings := patrol.GetFindings()
	findings.Add(investigatedFinding("finding-1"))
	run := startOrchestratorRun(t, orchestratorFindings, sessions, "finding-1", "chat-1")

	session := sessions.GetLatestByFinding("finding-1")
	now := time.Now().UTC()
	if _, _, err := audits.CreateActionAudit(unifiedresources.ActionAuditRecord{
		ID: "act-1", CreatedAt: now, UpdatedAt: now, State: unifiedresources.ActionStateExpired,
		Request: unifiedresources.ActionRequest{RequestID: "proposal-1", ResourceID: "vm-finding-1", CapabilityName: "restart", RequestedBy: "pulse_patrol"},
		Plan:    unifiedresources.ActionPlan{ActionID: "act-1", RequestID: "proposal-1", Allowed: true},
		Origin:  &unifiedresources.ActionOrigin{Surface: patrolActionOriginSurface, FindingID: "finding-1", InvestigationID: session.ID, ProposalID: "proposal-1"},
	}, nil); err != nil {
		t.Fatal(err)
	}
	session.Action = &aicontracts.ActionReference{ActionID: "act-1", State: "pending_approval"}
	sessions.Update(session)
	sessions.Fail(session.ID, "investigation final response failed: context deadline exceeded")
	sessions.SetOutcome(session.ID, aicontracts.OutcomeFixQueued)
	if !finishOrchestratorRun(orchestratorFindings, run, aicontracts.InvestigationStatusFailed, aicontracts.OutcomeFixQueued) {
		t.Fatal("the run's close was refused")
	}

	record := findings.Get("finding-1").InvestigationRecord
	if record == nil || record.Action == nil || record.Action.State != string(unifiedresources.ActionStateExpired) || record.Outcome != aicontracts.OutcomeNeedsAttention {
		t.Fatalf("record kept the plan as written back: %+v", record)
	}
	if got := sessions.GetLatestByFinding("finding-1"); got.Action == nil || got.Action.State != string(unifiedresources.ActionStateExpired) {
		t.Fatalf("session kept the plan as written back: %+v", got.Action)
	}
	for _, push := range sent.pushes {
		if push.Type == relay.PushTypeApprovalRequest {
			t.Fatalf("an expired plan asked for approval: %+v", sent.pushes)
		}
	}
}

// A close that does not land, because a newer run replaced the one reporting
// back, finalizes nothing.
func TestOrchestratorRunCloseThatDoesNotLandIsNotFinalized(t *testing.T) {
	orchestratorFindings, sessions, patrol := orchestratorPatrolFor(t, nil)
	sent := watchRunFinalization(patrol)
	findings := patrol.GetFindings()
	findings.Add(investigatedFinding("finding-1"))
	runA := startOrchestratorRun(t, orchestratorFindings, sessions, "finding-1", "chat-a")
	findings.RecoverStuckInvestigation("finding-1", "chat-a", time.Time{}, ai.InvestigationStatusFailed, ai.InvestigationOutcomeTimedOut, time.Now())
	startOrchestratorRun(t, orchestratorFindings, sessions, "finding-1", "chat-b")

	if finishOrchestratorRun(orchestratorFindings, runA, aicontracts.InvestigationStatusFailed, aicontracts.OutcomeFixQueued) {
		t.Fatal("an earlier run overwrote the run that replaced it")
	}
	if len(sent.published) != 0 || len(sent.pushes) != 0 || findings.Get("finding-1").InvestigationRecord != nil {
		t.Fatalf("a close that did not land was finalized: %+v", sent)
	}
}
