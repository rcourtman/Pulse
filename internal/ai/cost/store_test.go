package cost

import (
	"math"
	"testing"
	"time"
)

func TestSummaryGroupsByProviderModelAndDailyTotals(t *testing.T) {
	store := NewStore(90)
	now := time.Now()

	day1 := now.Add(-24 * time.Hour)
	day2 := now.Add(-48 * time.Hour)

	store.Record(UsageEvent{
		Timestamp:    day1,
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
		InputTokens:  100,
		OutputTokens: 50,
		UseCase:      "chat",
	})
	store.Record(UsageEvent{
		Timestamp:    day1,
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
		InputTokens:  10,
		OutputTokens: 5,
		UseCase:      "chat",
	})
	store.Record(UsageEvent{
		Timestamp:    day2,
		Provider:     "openai",
		RequestModel: "openai:gpt-4o-mini",
		InputTokens:  20,
		OutputTokens: 10,
		UseCase:      "patrol",
	})
	store.Record(UsageEvent{
		Timestamp:    now,
		Provider:     "anthropic",
		RequestModel: "anthropic:claude-opus-4-5-20251101",
		InputTokens:  200,
		OutputTokens: 100,
		UseCase:      "chat",
	})

	summary := store.GetSummary(3)

	if len(summary.ProviderModels) != 3 {
		t.Fatalf("expected 3 provider models, got %d", len(summary.ProviderModels))
	}

	type key struct{ provider, model string }
	got := make(map[key]ProviderModelSummary)
	for _, pm := range summary.ProviderModels {
		got[key{pm.Provider, pm.Model}] = pm
	}

	openaiGpt4o := got[key{"openai", "gpt-4o"}]
	if openaiGpt4o.InputTokens != 110 || openaiGpt4o.OutputTokens != 55 {
		t.Fatalf("openai gpt-4o tokens wrong: %+v", openaiGpt4o)
	}
	if !openaiGpt4o.PricingKnown || openaiGpt4o.EstimatedUSD <= 0 {
		t.Fatalf("expected openai gpt-4o pricing to be known with positive USD, got %+v", openaiGpt4o)
	}

	openaiMini := got[key{"openai", "gpt-4o-mini"}]
	if openaiMini.InputTokens != 20 || openaiMini.OutputTokens != 10 {
		t.Fatalf("openai gpt-4o-mini tokens wrong: %+v", openaiMini)
	}

	anthropicOpus := got[key{"anthropic", "claude-opus-4-5-20251101"}]
	if anthropicOpus.InputTokens != 200 || anthropicOpus.OutputTokens != 100 {
		t.Fatalf("anthropic opus tokens wrong: %+v", anthropicOpus)
	}
	if !anthropicOpus.PricingKnown || anthropicOpus.EstimatedUSD <= 0 {
		t.Fatalf("expected anthropic opus pricing to be known with positive USD, got %+v", anthropicOpus)
	}

	// Daily totals across all providers.
	dailyGot := make(map[string]DailySummary)
	for _, d := range summary.DailyTotals {
		dailyGot[d.Date] = d
	}

	d1Key := day1.Format("2006-01-02")
	if dailyGot[d1Key].InputTokens != 110 || dailyGot[d1Key].OutputTokens != 55 {
		t.Fatalf("daily totals for %s wrong: %+v", d1Key, dailyGot[d1Key])
	}

	d2Key := day2.Format("2006-01-02")
	if dailyGot[d2Key].InputTokens != 20 || dailyGot[d2Key].OutputTokens != 10 {
		t.Fatalf("daily totals for %s wrong: %+v", d2Key, dailyGot[d2Key])
	}

	todayKey := now.Format("2006-01-02")
	if dailyGot[todayKey].InputTokens != 200 || dailyGot[todayKey].OutputTokens != 100 {
		t.Fatalf("daily totals for %s wrong: %+v", todayKey, dailyGot[todayKey])
	}

	// Use-case rollups.
	useCases := make(map[string]UseCaseSummary)
	for _, uc := range summary.UseCases {
		useCases[uc.UseCase] = uc
	}
	if useCases["chat"].TotalTokens != (110 + 55 + 200 + 100) {
		t.Fatalf("chat use-case totals wrong: %+v", useCases["chat"])
	}
	if useCases["patrol"].TotalTokens != (20 + 10) {
		t.Fatalf("patrol use-case totals wrong: %+v", useCases["patrol"])
	}
}

func TestRetentionTrimsOldEvents(t *testing.T) {
	store := NewStore(1)
	old := time.Now().Add(-48 * time.Hour)

	store.Record(UsageEvent{
		Timestamp:    old,
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
		InputTokens:  10,
		OutputTokens: 10,
	})

	summary := store.GetSummary(7)
	if len(summary.ProviderModels) != 0 {
		t.Fatalf("expected old event to be trimmed, got %+v", summary.ProviderModels)
	}
}

func TestSummaryTruncationReflectsRetentionWindow(t *testing.T) {
	store := NewStore(30)
	now := time.Now()

	store.Record(UsageEvent{
		Timestamp:    now.Add(-10 * 24 * time.Hour),
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
		InputTokens:  10,
		OutputTokens: 10,
		UseCase:      "chat",
	})

	summary := store.GetSummary(365)
	if !summary.Truncated {
		t.Fatalf("expected summary to be truncated when requesting beyond retention")
	}
	if summary.RetentionDays != 30 || summary.EffectiveDays != 30 {
		t.Fatalf("unexpected retention/effective days: %+v", summary)
	}
}

func TestEstimateUSDKnownAndUnknownModels(t *testing.T) {
	usd, ok, _ := EstimateUSD("openai", "gpt-4o", 1_000_000, 2_000_000)
	if !ok {
		t.Fatalf("expected pricing to be known for gpt-4o")
	}
	expected := 35.0 // 1M input * $5 + 2M output * $15
	if math.Abs(usd-expected) > 0.0001 {
		t.Fatalf("expected usd %.4f, got %.4f", expected, usd)
	}

	usd, ok, _ = EstimateUSD("deepseek", "deepseek-reasoner", 1_000_000, 1_000_000)
	if !ok {
		t.Fatalf("expected deepseek pricing to be known for deepseek-reasoner")
	}
	expected = 0.70 // 1M input * $0.28 + 1M output * $0.42
	if math.Abs(usd-expected) > 0.0001 {
		t.Fatalf("expected usd %.4f, got %.4f", expected, usd)
	}
}
