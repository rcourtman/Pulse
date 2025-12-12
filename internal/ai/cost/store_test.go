package cost

import (
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

	openaiMini := got[key{"openai", "gpt-4o-mini"}]
	if openaiMini.InputTokens != 20 || openaiMini.OutputTokens != 10 {
		t.Fatalf("openai gpt-4o-mini tokens wrong: %+v", openaiMini)
	}

	anthropicOpus := got[key{"anthropic", "claude-opus-4-5-20251101"}]
	if anthropicOpus.InputTokens != 200 || anthropicOpus.OutputTokens != 100 {
		t.Fatalf("anthropic opus tokens wrong: %+v", anthropicOpus)
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
