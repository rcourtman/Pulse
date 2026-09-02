package ai

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestProjectPatrolCost_DefaultRunUsesMeasuredTokensAndPriceTable(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	got := ProjectPatrolCost(PatrolCostProjectionInput{
		Provider:        "gemini",
		Model:           "gemini-2.5-flash",
		IntervalMinutes: 360,
		BudgetUSD30d:    20,
		Now:             now,
	})
	if got.PerRunSource != PatrolCostPerRunSourceDefault || got.PerRunInputTokens != DefaultPatrolRunInputTokens {
		t.Fatalf("expected default per-run estimate, got %+v", got)
	}
	if !got.PricingKnown || !got.BilledPerToken {
		t.Fatalf("expected known per-token pricing for gemini flash, got %+v", got)
	}
	// 104,528 * 0.30/M + 4,491 * 2.50/M = 0.0314 + 0.0112 = 0.0426 per run.
	if got.PerRunUSD < 0.042 || got.PerRunUSD > 0.044 {
		t.Fatalf("unexpected per-run cost %.4f", got.PerRunUSD)
	}
	if got.ScheduledRunsPerDay != 4 {
		t.Fatalf("expected 4 scheduled runs a day at 6h, got %v", got.ScheduledRunsPerDay)
	}
	// 4 runs * 30 days * 0.0426 = 5.11.
	if got.ScheduledProjected30dUSD < 5.0 || got.ScheduledProjected30dUSD > 5.3 {
		t.Fatalf("unexpected scheduled 30d projection %.4f", got.ScheduledProjected30dUSD)
	}
	if got.Projected30dUSD != got.ScheduledProjected30dUSD {
		t.Fatalf("no triggered history: projection should equal scheduled, got %+v", got)
	}
	if len(got.IntervalEstimates) != len(PatrolCostIntervalOptionsMinutes) {
		t.Fatalf("expected one estimate per preset, got %d", len(got.IntervalEstimates))
	}
	// Half of a $20 budget is $10; 6h ($5.11) already fits, so keep 6h.
	if got.RecommendedIntervalMinutes != 360 || got.RecommendationReason != PatrolCostRecommendationFitsBudgetShare {
		t.Fatalf("expected 6h recommendation within budget share, got %d (%s)", got.RecommendedIntervalMinutes, got.RecommendationReason)
	}
	if got.RecommendationTargetUSD != 10 {
		t.Fatalf("expected $10 target, got %v", got.RecommendationTargetUSD)
	}
}

func TestProjectPatrolCost_ExpensiveModelSlowsScheduleToFitBudgetShare(t *testing.T) {
	got := ProjectPatrolCost(PatrolCostProjectionInput{
		Provider:        "anthropic",
		Model:           "claude-sonnet-5",
		IntervalMinutes: 360,
		BudgetUSD30d:    20,
	})
	// Sonnet 5: 104,528*2/M + 4,491*10/M = 0.209 + 0.045 = 0.254 per run.
	// 6h -> $30.5, 12h -> $15.2, daily -> $7.6: only daily fits under $10.
	if got.RecommendedIntervalMinutes != 1440 || got.RecommendationReason != PatrolCostRecommendationFitsBudgetShare {
		t.Fatalf("expected daily recommendation, got %d (%s) scheduled=%.2f", got.RecommendedIntervalMinutes, got.RecommendationReason, got.ScheduledProjected30dUSD)
	}
	if got.ScheduledProjected30dUSD < 29 || got.ScheduledProjected30dUSD > 32 {
		t.Fatalf("unexpected 6h projection for sonnet %.2f", got.ScheduledProjected30dUSD)
	}
}

func TestProjectPatrolCost_OpusExceedsBudgetShareEvenDaily(t *testing.T) {
	got := ProjectPatrolCost(PatrolCostProjectionInput{
		Provider:        "anthropic",
		Model:           "claude-fable-5-1",
		IntervalMinutes: 360,
	})
	// No budget configured: the $20 reference applies. Fable 5: 104,528*10/M
	// + 4,491*50/M = 1.045 + 0.225 = 1.27 per run; daily is $38 > $10.
	if got.RecommendedIntervalMinutes != 1440 || got.RecommendationReason != PatrolCostRecommendationExceedsEvenDaily {
		t.Fatalf("expected exceeds-even-daily, got %d (%s)", got.RecommendedIntervalMinutes, got.RecommendationReason)
	}
	if got.RecommendationTargetUSD != PatrolCostReferenceBudgetUSD*PatrolCostRecommendedBudgetShare {
		t.Fatalf("expected reference-budget target, got %v", got.RecommendationTargetUSD)
	}
}

func TestProjectPatrolCost_LocalModelIsNotBilledPerToken(t *testing.T) {
	got := ProjectPatrolCost(PatrolCostProjectionInput{
		Provider:        "ollama",
		Model:           "qwen3:8b",
		IntervalMinutes: 60,
	})
	if !got.PricingKnown || got.BilledPerToken {
		t.Fatalf("expected local model to be known and unbilled, got %+v", got)
	}
	if got.Projected30dUSD != 0 || got.RecommendedIntervalMinutes != 0 || got.RecommendationReason != PatrolCostRecommendationNotBilledPerToken {
		t.Fatalf("expected zero projection and no schedule change for local model, got %+v", got)
	}
}

func TestProjectPatrolCost_UnknownPricingStaysHonest(t *testing.T) {
	got := ProjectPatrolCost(PatrolCostProjectionInput{
		Provider:        "openai",
		Model:           "gpt-9-unpriced",
		IntervalMinutes: 360,
	})
	if got.PricingKnown || got.BilledPerToken || got.Projected30dUSD != 0 {
		t.Fatalf("expected unknown pricing with no projection, got %+v", got)
	}
	if got.RecommendationReason != PatrolCostRecommendationPricingUnknown {
		t.Fatalf("expected pricing-unknown reason, got %q", got.RecommendationReason)
	}
}

func TestProjectPatrolCost_UsesInstallHistoryMedianAndTriggeredRate(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	runs := []PatrolRunRecord{
		{StartedAt: now.Add(-6 * time.Hour), TriggerReason: string(TriggerReasonScheduled), InputTokens: 200_000, OutputTokens: 6_000},
		{StartedAt: now.Add(-12 * time.Hour), TriggerReason: string(TriggerReasonScheduled), InputTokens: 220_000, OutputTokens: 8_000},
		{StartedAt: now.Add(-18 * time.Hour), TriggerReason: string(TriggerReasonManual), InputTokens: 240_000, OutputTokens: 7_000},
		// Skipped run (budget) records no tokens and must not drag the median down.
		{StartedAt: now.Add(-24 * time.Hour), TriggerReason: string(TriggerReasonScheduled), InputTokens: 0},
		// Alert-triggered scoped runs count toward the triggered rate, not the full-run median.
		{StartedAt: now.Add(-30 * time.Hour), TriggerReason: string(TriggerReasonAlertFired), ScopeResourceIDs: []string{"vm/100"}, InputTokens: 30_000, OutputTokens: 1_000},
		{StartedAt: now.Add(-40 * time.Hour), TriggerReason: string(TriggerReasonAlertFired), ScopeResourceIDs: []string{"vm/101"}, InputTokens: 32_000, OutputTokens: 1_200},
		// Outside the 30-day window: ignored.
		{StartedAt: now.Add(-31 * 24 * time.Hour), TriggerReason: string(TriggerReasonScheduled), InputTokens: 900_000, OutputTokens: 50_000},
	}
	got := ProjectPatrolCost(PatrolCostProjectionInput{
		Provider:        "gemini",
		Model:           "gemini-2.5-flash",
		IntervalMinutes: 360,
		BudgetUSD30d:    20,
		Runs:            runs,
		Now:             now,
	})
	if got.PerRunSource != PatrolCostPerRunSourceHistory || got.HistoryRunCount != 3 {
		t.Fatalf("expected history-based estimate from 3 full runs, got %+v", got)
	}
	if got.PerRunInputTokens != 220_000 || got.PerRunOutputTokens != 7_000 {
		t.Fatalf("expected median of full runs, got in=%d out=%d", got.PerRunInputTokens, got.PerRunOutputTokens)
	}
	// Two triggered runs over an observed window of 40h (clamped to >= 1 day): 2/1.667 = 1.2 a day.
	if got.TriggeredRunsPerDay < 1.1 || got.TriggeredRunsPerDay > 1.3 {
		t.Fatalf("unexpected triggered rate %.3f", got.TriggeredRunsPerDay)
	}
	if got.TriggeredPerRunUSD <= 0 || got.Projected30dUSD <= got.ScheduledProjected30dUSD {
		t.Fatalf("expected triggered runs to add to the projection, got %+v", got)
	}
}

func TestProjectPatrolCost_BudgetReachedMirrorsEnforcement(t *testing.T) {
	store := cost.NewStore(30)
	store.Record(cost.UsageEvent{Timestamp: time.Now(), Provider: "anthropic", RequestModel: "claude-opus-5", UseCase: "patrol", InputTokens: 3_000_000, OutputTokens: 200_000})
	summary := store.GetSummary(30)
	got := ProjectPatrolCost(PatrolCostProjectionInput{
		Provider:        "anthropic",
		Model:           "claude-opus-5",
		IntervalMinutes: 360,
		BudgetUSD30d:    20,
		Spend:           summary,
	})
	if !got.Spend30dKnown || got.Spend30dUSD < 19.9 {
		t.Fatalf("expected known spend of about $20, got %+v", got)
	}
	if !got.BudgetReached || got.PatrolSpend30dUSD != got.Spend30dUSD {
		t.Fatalf("expected budget reached with patrol spend attributed, got %+v", got)
	}
}

func TestProjectPatrolCostForConfig_ResolvesPatrolModelAndInterval(t *testing.T) {
	cfg := config.NewDefaultAIConfig()
	cfg.Model = "gemini:gemini-2.5-flash"
	cfg.PatrolModel = "anthropic:claude-haiku-4-5"
	cfg.PatrolIntervalMinutes = 720
	cfg.CostBudgetUSD30d = 15
	got := ProjectPatrolCostForConfig(cfg, "", -1, nil, cost.Summary{}, time.Time{})
	if got.Provider != "anthropic" || got.Model != "claude-haiku-4-5" || got.ModelRoute != "anthropic:claude-haiku-4-5" {
		t.Fatalf("expected patrol override to win, got %+v", got)
	}
	if got.IntervalMinutes != 720 || got.BudgetUSD30d != 15 {
		t.Fatalf("expected config interval and budget, got %+v", got)
	}
	pending := ProjectPatrolCostForConfig(cfg, "gemini:gemini-2.5-flash", 60, nil, cost.Summary{}, time.Time{})
	if pending.Provider != "gemini" || pending.IntervalMinutes != 60 {
		t.Fatalf("expected pending selection to override config, got %+v", pending)
	}
}

func TestCostBudgetExceededError_IsSentinelAndKeepsLegacyWording(t *testing.T) {
	err := &CostBudgetExceededError{SpentUSD: 27.08, BudgetUSD: 20, Days: 30}
	if !errors.Is(err, ErrCostBudgetExceeded) {
		t.Fatal("expected budget error to match the sentinel")
	}
	if !strings.Contains(err.Error(), "budget exceeded (27.08/20.00 USD over 30 days)") {
		t.Fatalf("unexpected wording %q", err.Error())
	}
	reason := patrolBudgetBlockedReason(err)
	if !strings.Contains(reason, "$27.08") || !strings.Contains(reason, "$20.00") || !strings.Contains(reason, "Provider & Models") {
		t.Fatalf("blocked reason should name spend, limit, and where to fix it: %q", reason)
	}
}

func TestPatrolRuntimeFailureFromError_BudgetExhaustedIsNotAProviderFault(t *testing.T) {
	failure := patrolRuntimeFailureFromError(&CostBudgetExceededError{SpentUSD: 21, BudgetUSD: 20, Days: 30})
	if failure.Cause != PatrolFailureCauseBudgetExhausted {
		t.Fatalf("expected budget cause, got %q", failure.Cause)
	}
	if !strings.Contains(failure.Recommendation, "Provider & Models") {
		t.Fatalf("recommendation should point at the budget setting: %q", failure.Recommendation)
	}
	if strings.Contains(strings.ToLower(failure.Summary), "provider") {
		t.Fatalf("summary must not blame the provider: %q", failure.Summary)
	}
}

func TestBlockOnExhaustedBudget_SetsPatrolBlockedState(t *testing.T) {
	p := &PatrolService{}
	err := &CostBudgetExceededError{SpentUSD: 21, BudgetUSD: 20, Days: 30}
	p.blockOnExhaustedBudget(err, patrolRuntimeFailureFromError(err))
	p.mu.RLock()
	reason, cause := p.lastBlockedReason, p.lastBlockedCause
	p.mu.RUnlock()
	if cause != PatrolFailureCauseBudgetExhausted || !strings.Contains(reason, "used up") {
		t.Fatalf("expected blocked state for budget, got cause=%q reason=%q", cause, reason)
	}
	// Any other failure leaves the block state alone.
	other := &PatrolService{}
	other.blockOnExhaustedBudget(errors.New("boom"), patrolRuntimeFailureFromError(errors.New("boom")))
	if other.lastBlockedCause != "" {
		t.Fatalf("non-budget failures must not block, got %q", other.lastBlockedCause)
	}
}
