package ai

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Exercise the Patrol result -> usage ledger -> budget boundary, not merely
// the price calculator. The chat-service fixture deliberately reports no
// ordinary input, output, prose or tools to mask the cache-only partial case.
func TestPatrolCacheOnlyUsageReachesBudget(t *testing.T) {
	interrupted := errors.New("provider interrupted")
	for _, tc := range []struct {
		name        string
		input       int
		output      int
		creation    int
		read        int
		providerErr error
		wantUSD     float64
	}{
		{name: "cache write", creation: 10_000, wantUSD: 0.025},
		{name: "cache read", read: 100_000, wantUSD: 0.02},
		{name: "partial cache write", creation: 10_000, providerErr: interrupted, wantUSD: 0.025},
		{name: "partial cache read", read: 100_000, providerErr: interrupted, wantUSD: 0.02},
		{name: "ordinary input", input: 10_000, wantUSD: 0.02},
		{name: "output only", output: 2_000, wantUSD: 0.02},
		{name: "empty partial", providerErr: interrupted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := cost.NewStore(cost.DefaultMaxDays)
			svc := &Service{
				cfg: &config.AIConfig{
					Enabled: true, PatrolModel: "anthropic:claude-sonnet-5", CostBudgetUSD30d: 0.01,
				},
				provider:  &mockProvider{},
				costStore: store,
			}
			svc.SetChatService(&mockChatService{
				executor: tools.NewPulseToolExecutor(tools.ExecutorConfig{}),
				executePatrolStreamFunc: func(context.Context, PatrolExecuteRequest, ChatStreamCallback) (*PatrolStreamResponse, error) {
					return &PatrolStreamResponse{
						InputTokens: tc.input, OutputTokens: tc.output,
						CacheCreationInputTokens: tc.creation, CacheReadInputTokens: tc.read,
					}, tc.providerErr
				},
			})
			patrol := NewPatrolService(svc, nil)
			if err := svc.CheckBudget("patrol"); err != nil {
				t.Fatalf("empty ledger blocked the first run: %v", err)
			}
			result, err := patrol.runAIAnalysisState(context.Background(),
				patrolRuntimeStateForTest(patrol, models.StateSnapshot{}),
				&PatrolScope{NoStream: true, Depth: PatrolDepthQuick}, "cache-usage-test")
			if !errors.Is(err, tc.providerErr) {
				t.Fatalf("provider outcome changed: %v", err)
			}
			if tc.wantUSD > 0 {
				if result == nil || result.InputTokens != tc.input || result.OutputTokens != tc.output ||
					result.CacheCreationInputTokens != tc.creation || result.CacheReadInputTokens != tc.read {
					t.Fatalf("Patrol discarded or merged usage buckets: %+v", result)
				}
			} else if result != nil {
				t.Fatalf("empty failed run fabricated a partial result: %+v", result)
			}
			got := store.GetSummary(30)
			if math.Abs(got.Totals.EstimatedUSD-tc.wantUSD) > 1e-9 {
				t.Fatalf("ledger cost = %.9f, want %.9f", got.Totals.EstimatedUSD, tc.wantUSD)
			}
			if got.Totals.InputTokens != int64(tc.input) || got.Totals.OutputTokens != int64(tc.output) ||
				got.Totals.CacheCreationInputTokens != int64(tc.creation) || got.Totals.CacheReadInputTokens != int64(tc.read) {
				t.Fatalf("ledger usage buckets changed: %+v", got.Totals)
			}
			budgetErr := svc.CheckBudget("patrol")
			if tc.wantUSD > 0 {
				if !got.Totals.PricingKnown || !errors.Is(budgetErr, ErrCostBudgetExceeded) {
					t.Fatalf("billable usage did not block the next run: %+v, %v", got.Totals, budgetErr)
				}
			} else if budgetErr != nil {
				t.Fatalf("empty failed run consumed budget: %v", budgetErr)
			}
		})
	}
}

func TestProjectPatrolCostIncludesFullyCachedRuns(t *testing.T) {
	now := time.Date(2026, 10, 4, 2, 0, 0, 0, time.UTC)
	runs := []PatrolRunRecord{
		{StartedAt: now.Add(-6 * time.Hour), TriggerReason: string(TriggerReasonScheduled), CacheCreationInputTokens: 10_000, CacheReadInputTokens: 90_000, OutputTokens: 1_000},
		{StartedAt: now.Add(-12 * time.Hour), TriggerReason: string(TriggerReasonScheduled), CacheCreationInputTokens: 20_000, CacheReadInputTokens: 100_000, OutputTokens: 1_000},
		{StartedAt: now.Add(-18 * time.Hour), TriggerReason: string(TriggerReasonScheduled), CacheCreationInputTokens: 30_000, CacheReadInputTokens: 110_000, OutputTokens: 1_000},
		// Skipped and expired runs must still be excluded. The skipped run is
		// also older than the triggered run, so it cannot extend its rate window.
		{StartedAt: now.Add(-29 * 24 * time.Hour), TriggerReason: string(TriggerReasonScheduled)},
		{StartedAt: now.Add(-31 * 24 * time.Hour), TriggerReason: string(TriggerReasonScheduled), CacheReadInputTokens: 1_000_000},
		// A cache-only partial scoped run counts toward triggered spend, not
		// the scheduled-run median, despite zero ordinary input/output.
		{StartedAt: now.Add(-30 * time.Hour), TriggerReason: string(TriggerReasonAlertFired), ScopeResourceIDs: []string{"vm/100"}, CacheReadInputTokens: 50_000},
	}
	got := ProjectPatrolCost(PatrolCostProjectionInput{
		Provider: "anthropic", Model: "claude-sonnet-5", IntervalMinutes: 360, Runs: runs, Now: now,
	})
	if got.PerRunSource != PatrolCostPerRunSourceHistory || got.HistoryRunCount != 3 ||
		got.PerRunInputTokens != 0 || got.PerRunOutputTokens != 1_000 ||
		got.PerRunCacheCreationInputTokens != 20_000 || got.PerRunCacheReadInputTokens != 100_000 {
		t.Fatalf("fully cached runs were excluded from the median: %+v", got)
	}
	// Median: 0.02M*2.5 write + 0.1M*0.2 read + 0.001M*10 output = $0.08.
	// Four runs/day at six-hour intervals gives $9.60/30d. This synthetic
	// history proves accounting, not cache reuse between six-hourly calls.
	if !got.PricingKnown || got.ScheduledRunsPerDay != 4 || got.PerRunUSD != 0.08 || got.ScheduledProjected30dUSD != 9.6 {
		t.Fatalf("incorrect cached scheduled cost: %+v", got)
	}
	// One $0.01 scoped run over 1.25 observed days -> $0.24/30d.
	if got.TriggeredRunsPerDay != 0.8 || got.TriggeredPerRunUSD != 0.01 || got.Projected30dUSD != 9.84 {
		t.Fatalf("incorrect cache-only triggered cost: %+v", got)
	}
}
