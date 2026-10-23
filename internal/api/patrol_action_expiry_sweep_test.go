package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// patrolPlanAudit is a pending Patrol plan for an investigation, planned at
// planned and expiring at expires.
func patrolPlanAudit(id, investigationID string, planned, expires time.Time) unified.ActionAuditRecord {
	return unified.ActionAuditRecord{
		ID:        id,
		CreatedAt: planned,
		UpdatedAt: planned,
		State:     unified.ActionStatePending,
		Request:   boundActionTestRequest("proposal-"+id, "vm:42", "restart", "Clear the stuck mount", "pulse_patrol"),
		Plan: unified.ActionPlan{
			ActionID:            id,
			RequestID:           "proposal-" + id,
			Allowed:             true,
			RequiresApproval:    true,
			ApprovalPolicy:      unified.ApprovalAdmin,
			ApprovalRequirement: unified.ApprovalRequirementForFloor(unified.ApprovalAdmin),
			PlannedAt:           planned,
			ExpiresAt:           expires,
			ResourceVersion:     "resource:sha256:test",
			PolicyVersion:       "policy:sha256:test",
			PlanHash:            "sha256:" + id,
		},
		Origin: &unified.ActionOrigin{Surface: patrolActionOriginSurface, FindingID: "finding-1", InvestigationID: investigationID, ProposalID: "proposal-" + id},
	}
}

// patrolSweepFixture is a finding whose run failed after planning a fix, with
// Patrol reconciling the action handlers' transitions.
type patrolSweepFixture struct {
	handlers       *ResourceHandlers
	store          unified.ResourceStore
	findings       *ai.FindingsStore
	investigations *testInvestigationStore
	investigation  *aicontracts.InvestigationSession
}

func newPatrolSweepFixture(t *testing.T, currentActionID string) patrolSweepFixture {
	t.Helper()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	svc := ai.NewService(nil, nil)
	svc.SetStateProvider(&MockStateProvider{})
	findings := svc.GetPatrolService().GetFindings()
	findings.Add(&ai.Finding{ID: "finding-1", ResourceID: "vm:42", Title: "Unhealthy service", Severity: ai.FindingSeverityWarning,
		InvestigationSessionID: "session-1", InvestigationStatus: string(aicontracts.InvestigationStatusFailed),
		InvestigationOutcome: string(aicontracts.OutcomeFixQueued)})
	investigations := newTestInvestigationStore()
	investigation := investigations.Create("finding-1", "session-1")
	investigation.Status = aicontracts.InvestigationStatusFailed
	investigation.Outcome = aicontracts.OutcomeFixQueued
	investigation.Action = &aicontracts.ActionReference{ActionID: currentActionID, State: string(unified.ActionStatePending)}
	investigations.Update(investigation)

	patrolHandler := &AISettingsHandler{defaultAIService: svc,
		investigationStores:   map[string]aicontracts.InvestigationStore{"default": investigations},
		resourceStoreProvider: func(string) (unified.ResourceStore, error) { return store, nil },
	}
	h.SetActionTransitionPublisher(patrolHandler.ReconcilePatrolActionTransition)
	return patrolSweepFixture{handlers: h, store: store, findings: findings, investigations: investigations, investigation: investigation}
}

func (f patrolSweepFixture) readInbox(t *testing.T) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/actions?view=pending", nil)
	f.handlers.HandleListActions(rec, actionHandlerTestRequest(req, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("inbox status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

// Reading the action inbox sweeps plans past their expiry. A Patrol fix whose
// plan expires in that sweep moves its investigation and finding off the
// queued fix, as the action's own expiry does, rather than leaving them queued
// until someone opens the investigation.
func TestActionInboxSweepReconcilesAnExpiredPatrolPlan(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	f := newPatrolSweepFixture(t, "act-1")
	if err := f.store.RecordActionAudit(patrolPlanAudit("act-1", f.investigation.ID, now.Add(-10*time.Minute), now.Add(-5*time.Minute))); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	f.readInbox(t)

	if got := f.investigations.Get(f.investigation.ID); got.Outcome != aicontracts.OutcomeNeedsAttention || got.Action == nil || got.Action.State != string(unified.ActionStateExpired) {
		t.Fatalf("investigation kept the expired plan queued: outcome=%q action=%+v", got.Outcome, got.Action)
	}
	if got := f.findings.Get("finding-1"); got.InvestigationOutcome != string(aicontracts.OutcomeNeedsAttention) {
		t.Fatalf("finding kept the expired plan queued: outcome=%q", got.InvestigationOutcome)
	}
}

// A refresh plans a replacement under the same investigation and leaves the
// replaced plan to expire on its own. That plan's swept expiry must not take
// the investigation back from its replacement, which still waits for approval.
// The refresh lineage decides, not the clock: here the replacement's planning
// time reads earlier, as after the clock was set back.
func TestActionInboxSweepLeavesAReplacementPlanInPlace(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	f := newPatrolSweepFixture(t, "act-2")
	if err := f.store.RecordActionAudit(patrolPlanAudit("act-1", f.investigation.ID, now.Add(-10*time.Minute), now.Add(-5*time.Minute))); err != nil {
		t.Fatalf("RecordActionAudit(replaced): %v", err)
	}
	replacement := patrolPlanAudit("act-2", f.investigation.ID, now.Add(-20*time.Minute), now.Add(time.Hour))
	replacement.Request.RequestID = "refresh:act-1"
	replacement.Plan.RequestID = "refresh:act-1"
	replacement.Origin.ProposalID = "proposal-act-1"
	if err := f.store.RecordActionAudit(replacement); err != nil {
		t.Fatalf("RecordActionAudit(replacement): %v", err)
	}

	f.readInbox(t)

	if replaced, found, err := f.store.GetActionAudit("act-1"); err != nil || !found || replaced.State != unified.ActionStateExpired {
		t.Fatalf("the replaced plan did not expire in the sweep: %+v found=%v err=%v", replaced, found, err)
	}
	if got := f.investigations.Get(f.investigation.ID); got.Outcome != aicontracts.OutcomeFixQueued || got.Action == nil || got.Action.ActionID != "act-2" {
		t.Fatalf("the replaced plan's expiry took the investigation: outcome=%q action=%+v", got.Outcome, got.Action)
	}
	if got := f.findings.Get("finding-1"); got.InvestigationOutcome != string(aicontracts.OutcomeFixQueued) {
		t.Fatalf("the replaced plan's expiry moved the finding: outcome=%q", got.InvestigationOutcome)
	}
}
