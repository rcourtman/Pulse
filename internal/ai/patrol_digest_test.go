package ai

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func digestTestAction(id string, createdAt time.Time, state unifiedresources.ActionState, verified bool, surface string) unifiedresources.ActionAuditRecord {
	record := unifiedresources.ActionAuditRecord{
		ID:        id,
		CreatedAt: createdAt,
		UpdatedAt: createdAt.Add(time.Minute),
		State:     state,
		Request: unifiedresources.ActionRequest{
			RequestID: id + "-request", ResourceID: "app-container:test", CapabilityName: "restart", RequestedBy: "pulse_patrol",
		},
		Plan:   unifiedresources.ActionPlan{ActionID: id, RequestID: id + "-request", Allowed: true},
		Origin: &unifiedresources.ActionOrigin{Surface: surface, FindingID: id + "-finding"},
	}
	switch state {
	case unifiedresources.ActionStateCompleted:
		record.Approvals = []unifiedresources.ActionApprovalRecord{{
			Actor: "operator", Outcome: unifiedresources.OutcomeApproved, Timestamp: createdAt.Add(30 * time.Second),
		}}
		record.Result = &unifiedresources.ExecutionResult{
			Success:      true,
			Verification: &unifiedresources.ActionVerificationResult{Ran: true, Success: verified, RanAt: record.UpdatedAt},
		}
		record.VerificationOutcome = unifiedresources.VerificationOutcome{Status: unifiedresources.VerificationFailed}
		if verified {
			record.VerificationOutcome = unifiedresources.VerificationOutcome{
				Status: unifiedresources.VerificationVerified, EvidenceSummary: "Service healthy after restart.",
			}
		}
	case unifiedresources.ActionStateFailed:
		record.Approvals = []unifiedresources.ActionApprovalRecord{{
			Actor: "operator", Outcome: unifiedresources.OutcomeApproved, Timestamp: createdAt.Add(30 * time.Second),
		}}
		record.Result = &unifiedresources.ExecutionResult{Success: false, ErrorMessage: "restart failed"}
	case unifiedresources.ActionStateRejected:
		record.Approvals = []unifiedresources.ActionApprovalRecord{{
			Actor: "operator", Outcome: unifiedresources.OutcomeRejected, Timestamp: createdAt.Add(30 * time.Second),
		}}
	}
	return record
}

func TestBuildPatrolDigestRollsUpTheWeek(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	resolvedAt := now.Add(-2 * day)
	investigatedAt := now.Add(-3 * day)
	dismissedAt := now.Add(-4 * day)

	runs := []PatrolRunRecord{
		{ID: "run-scheduled", StartedAt: now.Add(-1 * day), CompletedAt: now.Add(-1*day + time.Minute), TriggerReason: string(TriggerReasonScheduled), ResourcesChecked: 40, NewFindings: 3, ResolvedFindings: 2, Status: "issues_found"},
		{ID: "run-alert", StartedAt: now.Add(-2 * day), CompletedAt: now.Add(-2*day + time.Minute), TriggerReason: string(TriggerReasonAlertFired), AlertIdentifier: "alert-a", ResourcesChecked: 1, NewFindings: 1, Status: "issues_found"},
		{ID: "run-alert-again", StartedAt: now.Add(-3 * day), CompletedAt: now.Add(-3*day + time.Minute), TriggerReason: string(TriggerReasonAlertFlapping), AlertIdentifier: "alert-a", ResourcesChecked: 1, Status: "healthy"},
		{ID: "run-manual-error", StartedAt: now.Add(-5 * day), CompletedAt: now.Add(-5*day + time.Minute), TriggerReason: string(TriggerReasonManual), ResourcesChecked: 38, Status: "error", ErrorCount: 1},
		{ID: "run-old", StartedAt: now.Add(-10 * day), CompletedAt: now.Add(-10*day + time.Minute), TriggerReason: string(TriggerReasonScheduled), ResourcesChecked: 99, NewFindings: 9, ResolvedFindings: 9, Status: "issues_found"},
	}

	findings := []*Finding{
		{ID: "open-critical", Severity: FindingSeverityCritical, DetectedAt: now.Add(-1 * day)},
		{ID: "open-warning", Severity: FindingSeverityWarning, DetectedAt: now.Add(-2 * day)},
		{ID: "old-open", Severity: FindingSeverityCritical, DetectedAt: now.Add(-20 * day)},
		{ID: "manually-resolved", Severity: FindingSeverityWarning, DetectedAt: now.Add(-6 * day), ResolvedAt: &resolvedAt, AutoResolved: false},
		{ID: "auto-resolved", Severity: FindingSeverityWarning, DetectedAt: now.Add(-6 * day), ResolvedAt: &resolvedAt, AutoResolved: true},
		{ID: "dismissed", Severity: FindingSeverityWatch, DetectedAt: now.Add(-5 * day), DismissedReason: "expected_behavior", Lifecycle: []FindingLifecycleEvent{
			{At: dismissedAt, Type: "dismissed", Metadata: map[string]string{"reason": "expected_behavior"}},
			{At: dismissedAt, Type: "suppressed"},
		}},
		{ID: "legacy-investigated", Severity: FindingSeverityWarning, DetectedAt: now.Add(-6 * day), LastInvestigatedAt: &investigatedAt, InvestigationOutcome: "fix_verified"},
		nil,
	}

	actions := []unifiedresources.ActionAuditRecord{
		digestTestAction("verified", now.Add(-1*day), unifiedresources.ActionStateCompleted, true, "patrol"),
		digestTestAction("unverified", now.Add(-2*day), unifiedresources.ActionStateCompleted, false, "patrol"),
		digestTestAction("failed", now.Add(-2*day), unifiedresources.ActionStateFailed, false, "patrol"),
		digestTestAction("rejected", now.Add(-3*day), unifiedresources.ActionStateRejected, false, "patrol"),
		digestTestAction("pending-old", now.Add(-12*day), unifiedresources.ActionStatePending, false, "patrol"),
		digestTestAction("pending-new", now.Add(-time.Hour), unifiedresources.ActionStatePending, false, "patrol"),
		digestTestAction("assistant", now.Add(-1*day), unifiedresources.ActionStateCompleted, true, "assistant"),
		digestTestAction("old-verified", now.Add(-15*day), unifiedresources.ActionStateCompleted, true, "patrol"),
	}

	usage := []cost.UsageEvent{
		{Timestamp: now.Add(-1 * day), Provider: "openai", RequestModel: "gpt-4o-mini", UseCase: "patrol", InputTokens: 1_000_000, OutputTokens: 100_000},
		{Timestamp: now.Add(-2 * day), Provider: "openai", RequestModel: "gpt-4o-mini", UseCase: "PATROL", InputTokens: 1_000_000, OutputTokens: 0},
		{Timestamp: now.Add(-2 * day), Provider: "openai", RequestModel: "gpt-4o-mini", UseCase: "chat", InputTokens: 5_000_000},
		{Timestamp: now.Add(-9 * day), Provider: "openai", RequestModel: "gpt-4o-mini", UseCase: "patrol", InputTokens: 5_000_000},
	}

	digest := BuildPatrolDigest(PatrolDigestInput{
		Now: now, Days: 7, Mode: "Approval",
		Runs: runs, RunHistoryCapacity: MaxPatrolRunHistory,
		Findings: findings, Actions: actions, Usage: usage,
	})

	if digest.Mode != config.PatrolAutonomyApproval {
		t.Fatalf("mode = %q", digest.Mode)
	}
	if !digest.Window.HistoryComplete || digest.Window.HistorySince != nil || digest.Window.Days != 7 {
		t.Fatalf("window = %+v", digest.Window)
	}
	if got := digest.Runs; got.Total != 4 || got.Scheduled != 1 || got.EventTriggered != 2 || got.Manual != 1 || got.Failed != 1 || got.Checks != 80 || got.ResourcesCovered != 40 {
		t.Fatalf("runs = %+v", got)
	}
	if digest.Runs.LastRunAt == nil || !digest.Runs.LastRunAt.Equal(now.Add(-1*day+time.Minute)) {
		t.Fatalf("last run = %v", digest.Runs.LastRunAt)
	}
	if digest.Alerts.Reviewed != 1 {
		t.Fatalf("alerts reviewed = %d, want the one distinct alert", digest.Alerts.Reviewed)
	}
	if got := digest.Findings; got.New != 4 || got.AutoResolved != 2 || got.Resolved != 3 || got.Dismissed != 1 || got.Suppressed != 1 {
		t.Fatalf("findings = %+v", got)
	}
	// open-critical and open-warning were raised in the window and are still open;
	// legacy-investigated is open too. old-open predates the window and the
	// dismissed finding is not open.
	if got := digest.Findings.OpenBySeverity; got.Critical != 1 || got.Warning != 2 || got.Watch != 0 || got.Info != 0 {
		t.Fatalf("open by severity = %+v", got)
	}
	// Investigations are counted from the run rows the findings store writes
	// (TestBuildPatrolDigestCountsEachInvestigationRunOnce). Summary fields
	// with no run in the lifecycle, as on legacy-investigated, are not a run.
	if digest.Investigations.Total != 0 || len(digest.Investigations.ByOutcome) != 0 {
		t.Fatalf("investigations = %+v", digest.Investigations)
	}
	if got := digest.Actions; got.Proposed != 5 || got.Approved != 3 || got.Rejected != 1 || got.Executed != 3 || got.Verified != 1 || got.Failed != 1 || got.Pending != 2 {
		t.Fatalf("actions = %+v", got)
	}
	if got := digest.Spend; got.Calls != 2 || got.InputTokens != 2_000_000 || got.OutputTokens != 100_000 || !got.PricingKnown {
		t.Fatalf("spend = %+v", got)
	}
	// gpt-4o-mini: 0.15 USD per million input, 0.60 per million output.
	if want := 0.15*2 + 0.60*0.1; digest.Spend.EstimatedUSD < want-0.0001 || digest.Spend.EstimatedUSD > want+0.0001 {
		t.Fatalf("estimated usd = %f, want %f", digest.Spend.EstimatedUSD, want)
	}
}

// digestTestRuns drives investigation runs through the findings store's own
// writers, the way the orchestrator does: a run is marked running with no
// outcome, then its end records the status, outcome and time.
type digestTestRuns struct {
	store *FindingsStore
}

func newDigestTestRuns(ids ...string) digestTestRuns {
	runs := digestTestRuns{store: NewFindingsStore()}
	for _, id := range ids {
		runs.store.Add(&Finding{ID: id, ResourceID: "vm-" + id, ResourceName: id, Severity: FindingSeverityWarning, Category: FindingCategoryReliability, Title: "Finding " + id})
	}
	return runs
}

func (r digestTestRuns) start(id, session string) {
	f := r.store.Get(id)
	r.store.UpdateInvestigation(id, session, string(InvestigationStatusRunning), "", f.LastInvestigatedAt, f.InvestigationAttempts+1)
}

func (r digestTestRuns) finish(id, session string, status InvestigationStatus, outcome InvestigationOutcome) {
	f := r.store.Get(id)
	ended := time.Now()
	r.store.UpdateInvestigation(id, session, string(status), string(outcome), &ended, f.InvestigationAttempts)
}

func TestBuildPatrolDigestCountsEachInvestigationRunOnce(t *testing.T) {
	t.Parallel()
	runs := newDigestTestRuns("twice", "approved", "verified-in-run", "verified-then-done", "verified-then-error", "regressed-mid-run", "interrupted", "recovered-verified", "running")

	// Two runs on one finding count twice: an answer, then a provider error
	// that left no outcome.
	runs.start("twice", "chat-1")
	runs.finish("twice", "chat-1", InvestigationStatusCompleted, InvestigationOutcomeNeedsAttention)
	runs.start("twice", "chat-2")
	runs.finish("twice", "chat-2", InvestigationStatusFailed, "")

	// A queued fix that is later run and verified is one investigation that
	// ended fixed and verified, not one per action step.
	runs.start("approved", "chat-3")
	runs.finish("approved", "chat-3", InvestigationStatusCompleted, InvestigationOutcomeFixQueued)
	runs.store.UpdateInvestigationOutcome("approved", string(InvestigationOutcomeFixExecuted))
	runs.store.UpdateInvestigationOutcome("approved", string(InvestigationOutcomeFixVerified))

	// A run whose own fix was verified, after an earlier run timed out.
	runs.start("verified-in-run", "chat-4")
	runs.finish("verified-in-run", "chat-4", InvestigationStatusFailed, InvestigationOutcomeTimedOut)
	runs.start("verified-in-run", "chat-5")
	runs.finish("verified-in-run", "chat-5", InvestigationStatusCompleted, InvestigationOutcomeFixVerified)

	// The action lifecycle can verify a run's fix before the run itself ends.
	// The run still counts once and keeps the verified result, whether its own
	// end then succeeds or fails.
	for _, ending := range []struct {
		id, session string
		status      InvestigationStatus
		outcome     InvestigationOutcome
	}{
		{"verified-then-done", "chat-9", InvestigationStatusCompleted, InvestigationOutcomeFixVerified},
		{"verified-then-error", "chat-10", InvestigationStatusFailed, ""},
	} {
		runs.start(ending.id, ending.session)
		session := &InvestigationSession{FindingID: ending.id, SessionID: ending.session, StartedAt: time.Now()}
		if applied, _ := runs.store.ReconcileInvestigationOutcome(ending.id, session, string(InvestigationOutcomeFixVerified)); !applied {
			t.Fatalf("%s: verification of the running session was not applied", ending.id)
		}
		runs.finish(ending.id, ending.session, ending.status, ending.outcome)
	}

	// The finding is resolved and detected again while a run is in progress.
	// The regression clears the run's status, and the run still counts when it
	// ends.
	runs.start("regressed-mid-run", "chat-11")
	if !runs.store.Resolve("regressed-mid-run", false) {
		t.Fatal("resolve the investigated finding")
	}
	runs.store.Add(&Finding{ID: "regressed-mid-run", ResourceID: "vm-regressed-mid-run", ResourceName: "regressed-mid-run", Severity: FindingSeverityWarning, Category: FindingCategoryReliability, Title: "Finding regressed-mid-run"})
	if f := runs.store.Get("regressed-mid-run"); f.RegressionCount != 1 || f.InvestigationStatus != "" {
		t.Fatalf("regression = %d, status %q; want the run's status cleared by one regression", f.RegressionCount, f.InvestigationStatus)
	}
	runs.finish("regressed-mid-run", "chat-11", InvestigationStatusCompleted, InvestigationOutcomeNeedsAttention)

	// Stuck-run recovery closes runs the orchestrator lost, including one whose
	// session already held a verified result.
	runs.start("interrupted", "chat-6")
	runs.store.RecoverStuckInvestigation("interrupted", "chat-6", time.Time{}, InvestigationStatusFailed, InvestigationOutcomeInterrupted, time.Now())
	runs.start("recovered-verified", "chat-7")
	runs.store.RecoverStuckInvestigation("recovered-verified", "chat-7", time.Time{}, InvestigationStatusCompleted, InvestigationOutcomeFixVerified, time.Now())

	// A run still in progress has not finished, so it does not count yet.
	runs.start("running", "chat-8")

	digest := BuildPatrolDigest(PatrolDigestInput{Now: time.Now(), Days: 7, Findings: runs.store.GetAll(nil)})
	want := map[string]int{"needs_attention": 2, "failed": 1, "fix_verified": 5, "timed_out": 1, "interrupted": 1}
	if digest.Investigations.Total != 10 || !reflect.DeepEqual(digest.Investigations.ByOutcome, want) {
		t.Fatalf("investigations = %+v, want 10 runs %v", digest.Investigations, want)
	}

	// A week on, every run finished before the window opened.
	later := BuildPatrolDigest(PatrolDigestInput{Now: time.Now().Add(8 * 24 * time.Hour), Days: 7, Findings: runs.store.GetAll(nil)})
	if later.Investigations.Total != 0 || len(later.Investigations.ByOutcome) != 0 {
		t.Fatalf("investigations a week later = %+v, want none", later.Investigations)
	}
}

func TestBuildPatrolDigestCountsARunInTheWindowItFinished(t *testing.T) {
	t.Parallel()
	runs := newDigestTestRuns("approved")
	runs.start("approved", "chat-1")
	runs.finish("approved", "chat-1", InvestigationStatusCompleted, InvestigationOutcomeFixQueued)
	runs.store.UpdateInvestigationOutcome("approved", string(InvestigationOutcomeFixExecuted))

	findings := runs.store.GetAll(nil)
	if len(findings) != 1 {
		t.Fatalf("findings = %d", len(findings))
	}
	// Move the run ten days back and leave its fix running inside the window.
	// GetAll copies only the finding, so copy the lifecycle before editing it.
	f := findings[0]
	f.Lifecycle = append([]FindingLifecycleEvent(nil), f.Lifecycle...)
	tenDays := 10 * 24 * time.Hour
	for i := range f.Lifecycle {
		if f.Lifecycle[i].Type != "investigation_outcome" {
			f.Lifecycle[i].At = f.Lifecycle[i].At.Add(-tenDays)
		}
	}

	digest := BuildPatrolDigest(PatrolDigestInput{Now: time.Now(), Days: 7, Findings: findings})
	if digest.Investigations.Total != 0 || len(digest.Investigations.ByOutcome) != 0 {
		t.Fatalf("investigations = %+v, want none: the run finished before the window and its fix is not another investigation", digest.Investigations)
	}
	digest = BuildPatrolDigest(PatrolDigestInput{Now: time.Now(), Days: 14, Findings: findings})
	if digest.Investigations.Total != 1 || digest.Investigations.ByOutcome["fix_executed"] != 1 {
		t.Fatalf("investigations = %+v, want the run once with its fix's latest outcome", digest.Investigations)
	}
}

func TestBuildPatrolDigestFlagsTruncatedHistoryAndUnknownPricing(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	runs := make([]PatrolRunRecord, 0, 5)
	for i := 0; i < 5; i++ {
		runs = append(runs, PatrolRunRecord{
			ID:        "run-" + string(rune('a'+i)),
			StartedAt: now.Add(-time.Duration(i+1) * time.Hour),
			Status:    "healthy",
		})
	}
	digest := BuildPatrolDigest(PatrolDigestInput{
		Now: now, Days: 7, Runs: runs, RunHistoryCapacity: 5,
		Usage: []cost.UsageEvent{{Timestamp: now.Add(-time.Hour), Provider: "custom", RequestModel: "mystery-model", UseCase: "patrol", InputTokens: 10}},
	})
	if digest.Window.HistoryComplete || digest.Window.HistorySince == nil || !digest.Window.HistorySince.Equal(now.Add(-5*time.Hour)) {
		t.Fatalf("window = %+v, want truncated history since the oldest retained run", digest.Window)
	}
	if digest.Mode != config.PatrolAutonomyMonitor {
		t.Fatalf("mode = %q, want monitor default", digest.Mode)
	}
	if digest.Spend.Calls != 1 || digest.Spend.PricingKnown || digest.Spend.EstimatedUSD != 0 {
		t.Fatalf("spend = %+v, want unknown pricing reported, not zeroed as known", digest.Spend)
	}

	// Below capacity the store still holds everything, so the window is complete.
	digest = BuildPatrolDigest(PatrolDigestInput{Now: now, Days: 7, Runs: runs[:3], RunHistoryCapacity: 5})
	if !digest.Window.HistoryComplete {
		t.Fatalf("window = %+v, want complete history below capacity", digest.Window)
	}
}

func TestBuildPatrolDigestEmptyInputIsZeroNotNil(t *testing.T) {
	t.Parallel()
	digest := BuildPatrolDigest(PatrolDigestInput{Days: 99})
	if digest.Window.Days != PatrolDigestMaxDays {
		t.Fatalf("days = %d, want clamp to %d", digest.Window.Days, PatrolDigestMaxDays)
	}
	if digest.Investigations.ByOutcome == nil {
		t.Fatal("by_outcome must serialise as an object, not null")
	}
	if !digest.Spend.PricingKnown {
		t.Fatal("no calls means nothing has unknown pricing")
	}
	if NormalizePatrolDigestDays(0) != PatrolDigestDefaultDays || NormalizePatrolDigestDays(3) != 3 {
		t.Fatal("NormalizePatrolDigestDays defaults and passthrough")
	}
	order := PatrolDigestOutcomeOrder(map[string]int{"resolved": 1, "needs_attention": 2, "zzz_custom": 1})
	if len(order) != 3 || order[0] != "needs_attention" || order[1] != "resolved" || order[2] != "zzz_custom" {
		t.Fatalf("outcome order = %v", order)
	}
}

func TestSummarizeDigestSpendPricesPromptCacheBuckets(t *testing.T) {
	t.Parallel()
	now := time.Now()
	events := []cost.UsageEvent{
		{Timestamp: now, Provider: "anthropic", RequestModel: "anthropic:claude-sonnet-5", UseCase: "patrol",
			InputTokens: 100_000, OutputTokens: 10_000, CacheCreationInputTokens: 200_000, CacheReadInputTokens: 1_000_000},
		// Not Patrol: ignored.
		{Timestamp: now, Provider: "anthropic", RequestModel: "anthropic:claude-sonnet-5", UseCase: "chat", InputTokens: 5},
	}
	spend := summarizeDigestSpend(events, func(time.Time) bool { return true })
	if spend.Calls != 1 || spend.InputTokens != 100_000 || spend.OutputTokens != 10_000 {
		t.Fatalf("spend = %+v", spend)
	}
	if spend.CacheCreationInputTokens != 200_000 || spend.CacheReadInputTokens != 1_000_000 {
		t.Fatalf("cache buckets missing from digest spend: %+v", spend)
	}
	// Priced at Anthropic cache rates, not the input rate for every token.
	if want := 0.2 + 0.1 + 0.5 + 0.2; !spend.PricingKnown || math.Abs(spend.EstimatedUSD-want) > 1e-9 {
		t.Fatalf("estimated usd = %f (known=%v), want %f", spend.EstimatedUSD, spend.PricingKnown, want)
	}
}
