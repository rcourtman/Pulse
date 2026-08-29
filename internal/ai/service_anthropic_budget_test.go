package ai

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestServiceCheckBudgetUsesCurrentAnthropicRates(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		budgetUSD float64
		wantErr   bool
	}{
		{name: "current Opus remains below budget", model: "claude-opus-5", budgetUSD: 20},
		{name: "dated current Opus remains below budget", model: "claude-opus-4-5-20251101", budgetUSD: 20},
		{name: "legacy Opus still exceeds budget", model: "claude-opus-4-1-20250805", budgetUSD: 20, wantErr: true},
		{name: "Sonnet 5 uses lower current rate", model: "claude-sonnet-5", budgetUSD: 8},
		{name: "Fable 5 remains budget enforceable", model: "claude-fable-5", budgetUSD: 20, wantErr: true},
		{name: "Mythos 5 remains budget enforceable", model: "claude-mythos-5", budgetUSD: 20, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := cost.NewStore(cost.DefaultMaxDays)
			store.Record(cost.UsageEvent{
				Provider:     "anthropic",
				RequestModel: test.model,
				InputTokens:  1_000_000,
				OutputTokens: 500_000,
				UseCase:      "patrol",
			})
			svc := &Service{cfg: &config.AIConfig{CostBudgetUSD30d: test.budgetUSD}, costStore: store}
			err := svc.CheckBudget("patrol")
			if (err != nil) != test.wantErr {
				t.Fatalf("CheckBudget() error = %v, wantErr %v; summary = %+v", err, test.wantErr, store.GetSummary(30).Totals)
			}
		})
	}
}
