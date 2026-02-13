package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/investigation"
)

type adapterLicenseChecker struct{}

func (adapterLicenseChecker) HasFeature(string) bool { return true }

func newInvestigationAdapterForTest(config investigation.InvestigationConfig) (*InvestigationOrchestratorAdapter, *investigation.Store) {
	store := investigation.NewStore("")
	orchestrator := investigation.NewOrchestrator(nil, store, nil, nil, config)
	return NewInvestigationOrchestratorAdapter(orchestrator), store
}

func TestInvestigationOrchestratorAdapter_GetInvestigationByFinding(t *testing.T) {
	adapter, store := newInvestigationAdapterForTest(investigation.DefaultConfig())

	if got := adapter.GetInvestigationByFinding("missing"); got != nil {
		t.Fatalf("expected nil for unknown finding, got %#v", got)
	}

	session := store.Create("finding-1", "chat-1")
	updated := store.Get(session.ID)
	completedAt := time.Now().UTC().Add(2 * time.Minute)
	updated.Status = investigation.StatusCompleted
	updated.TurnCount = 3
	updated.Outcome = investigation.OutcomeFixExecuted
	updated.CompletedAt = &completedAt
	updated.ToolsAvailable = []string{"discovery.read", "tools.control"}
	updated.ToolsUsed = []string{"discovery.read"}
	updated.EvidenceIDs = []string{"ev-1", "ev-2"}
	updated.Summary = "investigation completed"
	updated.Error = "none"
	updated.ApprovalID = "approval-1"
	updated.ProposedFix = &investigation.Fix{
		ID:          "fix-1",
		Description: "restart service",
		Commands:    []string{"systemctl restart pulse"},
		RiskLevel:   "medium",
		Destructive: false,
		TargetHost:  "host-1",
		Rationale:   "clears stale process state",
	}
	if ok := store.Update(updated); !ok {
		t.Fatalf("expected store update to succeed")
	}

	got := adapter.GetInvestigationByFinding("finding-1")
	if got == nil {
		t.Fatalf("expected investigation session")
	}
	if got.ID != updated.ID {
		t.Fatalf("id = %q, want %q", got.ID, updated.ID)
	}
	if got.Status != string(investigation.StatusCompleted) {
		t.Fatalf("status = %q, want %q", got.Status, investigation.StatusCompleted)
	}
	if got.Outcome != string(investigation.OutcomeFixExecuted) {
		t.Fatalf("outcome = %q, want %q", got.Outcome, investigation.OutcomeFixExecuted)
	}
	if got.CompletedAt == nil || !got.CompletedAt.Equal(completedAt) {
		t.Fatalf("completedAt = %v, want %v", got.CompletedAt, completedAt)
	}
	if got.ProposedFix == nil {
		t.Fatalf("expected proposed fix conversion")
	}
	if got.ProposedFix.ID != "fix-1" || got.ProposedFix.TargetHost != "host-1" {
		t.Fatalf("unexpected proposed fix conversion: %#v", got.ProposedFix)
	}
	if len(got.ToolsAvailable) != 2 || len(got.EvidenceIDs) != 2 {
		t.Fatalf("expected converted slices, got tools=%v evidence=%v", got.ToolsAvailable, got.EvidenceIDs)
	}
}

func TestInvestigationOrchestratorAdapter_GetRunningAndFixedCounts(t *testing.T) {
	adapter, store := newInvestigationAdapterForTest(investigation.DefaultConfig())

	running := store.Create("finding-running", "chat-running")
	if ok := store.UpdateStatus(running.ID, investigation.StatusRunning); !ok {
		t.Fatalf("expected running status update")
	}

	fixed := store.Create("finding-fixed", "chat-fixed")
	if ok := store.Complete(fixed.ID, investigation.OutcomeFixExecuted, "done", nil); !ok {
		t.Fatalf("expected complete to succeed")
	}

	notFixed := store.Create("finding-not-fixed", "chat-not-fixed")
	if ok := store.Complete(notFixed.ID, investigation.OutcomeCannotFix, "manual", nil); !ok {
		t.Fatalf("expected non-fixed completion to succeed")
	}

	if got := adapter.GetRunningCount(); got != 1 {
		t.Fatalf("running count = %d, want 1", got)
	}
	if got := adapter.GetFixedCount(); got != 1 {
		t.Fatalf("fixed count = %d, want 1", got)
	}
}

func TestInvestigationOrchestratorAdapter_DelegatesLifecycleAndSetters(t *testing.T) {
	config := investigation.DefaultConfig()
	config.MaxConcurrent = 1
	adapter, _ := newInvestigationAdapterForTest(config)

	if !adapter.CanStartInvestigation() {
		t.Fatalf("expected CanStartInvestigation=true with no running sessions")
	}

	err := adapter.ReinvestigateFinding(context.Background(), "finding-1", "full")
	if err == nil || !strings.Contains(err.Error(), "findings store not configured") {
		t.Fatalf("expected findings-store error, got %v", err)
	}

	if err := adapter.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}

	adapter.SetFixVerifier(nil)
	adapter.SetMetricsCallback()
	adapter.SetLicenseChecker(adapterLicenseChecker{})
}

func TestPatrolMetricsCallback_RecordsCounters(t *testing.T) {
	metrics := GetPatrolMetrics()
	callback := &patrolMetricsCallback{}

	outcomeLabel := "adapter-test-outcome"
	verifyLabel := "adapter-test-verify"

	outcomeBefore := testutil.ToFloat64(metrics.investigationOutcome.WithLabelValues(outcomeLabel))
	verifyBefore := testutil.ToFloat64(metrics.fixVerification.WithLabelValues(verifyLabel))

	callback.RecordInvestigationOutcome(outcomeLabel)
	callback.RecordFixVerification(verifyLabel)

	outcomeAfter := testutil.ToFloat64(metrics.investigationOutcome.WithLabelValues(outcomeLabel))
	verifyAfter := testutil.ToFloat64(metrics.fixVerification.WithLabelValues(verifyLabel))

	if outcomeAfter < outcomeBefore+1 {
		t.Fatalf("expected investigation outcome counter to increase, before=%f after=%f", outcomeBefore, outcomeAfter)
	}
	if verifyAfter < verifyBefore+1 {
		t.Fatalf("expected fix verification counter to increase, before=%f after=%f", verifyBefore, verifyAfter)
	}
}

func TestPatrolFixVerifier_DelegatesToPatrolService(t *testing.T) {
	verifier := &patrolFixVerifier{}

	ok, err := verifier.VerifyFixResolved(context.Background(), &investigation.Finding{})
	if ok {
		t.Fatalf("expected unresolved result with nil patrol service")
	}
	if !errors.Is(err, investigation.ErrVerificationUnknown) {
		t.Fatalf("expected verification unknown error, got %v", err)
	}
}
