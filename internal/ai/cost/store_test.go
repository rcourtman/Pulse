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

	// Target rollups should be empty for these events (no targets set).
	if len(summary.Targets) != 0 {
		t.Fatalf("expected no target rollups, got %+v", summary.Targets)
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

func TestClearEmptiesUsageHistory(t *testing.T) {
	store := NewStore(30)
	store.Record(UsageEvent{
		Timestamp:    time.Now(),
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
		InputTokens:  10,
		OutputTokens: 10,
		UseCase:      "chat",
	})
	if len(store.GetSummary(30).ProviderModels) == 0 {
		t.Fatalf("expected usage to be recorded before clear")
	}
	if err := store.Clear(); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	if got := store.GetSummary(30); len(got.ProviderModels) != 0 || got.Totals.TotalTokens != 0 {
		t.Fatalf("expected empty summary after clear, got %+v", got)
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

func TestSummaryTargetsRollup(t *testing.T) {
	store := NewStore(30)
	now := time.Now()

	store.Record(UsageEvent{
		Timestamp:    now,
		Provider:     "deepseek",
		RequestModel: "deepseek:deepseek-chat",
		InputTokens:  1000,
		OutputTokens: 100,
		UseCase:      "chat",
		TargetType:   "vm",
		TargetID:     "101",
	})
	store.Record(UsageEvent{
		Timestamp:    now,
		Provider:     "deepseek",
		RequestModel: "deepseek:deepseek-chat",
		InputTokens:  2000,
		OutputTokens: 200,
		UseCase:      "chat",
		TargetType:   "vm",
		TargetID:     "101",
	})

	summary := store.GetSummary(7)
	if len(summary.Targets) != 1 {
		t.Fatalf("expected 1 target summary, got %d", len(summary.Targets))
	}
	got := summary.Targets[0]
	if got.TargetType != "vm" || got.TargetID != "101" || got.Calls != 2 {
		t.Fatalf("unexpected target summary: %+v", got)
	}
	if got.TotalTokens != 3300 {
		t.Fatalf("unexpected target token totals: %+v", got)
	}
	if !got.PricingKnown || got.EstimatedUSD <= 0 {
		t.Fatalf("expected pricing known with positive USD: %+v", got)
	}
}

// mockPersistence implements Persistence for testing
type mockPersistence struct {
	events []UsageEvent
	saved  []UsageEvent
}

func (m *mockPersistence) LoadUsageHistory() ([]UsageEvent, error) {
	return m.events, nil
}

func (m *mockPersistence) SaveUsageHistory(events []UsageEvent) error {
	m.saved = events
	return nil
}

func TestSetPersistence_NilPersistence(t *testing.T) {
	store := NewStore(30)

	// Setting nil persistence should be a no-op
	err := store.SetPersistence(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetPersistence_LoadsExistingHistory(t *testing.T) {
	store := NewStore(30)
	now := time.Now()

	// Create mock persistence with pre-existing events
	mock := &mockPersistence{
		events: []UsageEvent{
			{
				Timestamp:    now.Add(-1 * time.Hour),
				Provider:     "openai",
				RequestModel: "openai:gpt-4o",
				InputTokens:  500,
				OutputTokens: 250,
				UseCase:      "chat",
			},
		},
	}

	// Set persistence - should load existing events
	err := store.SetPersistence(mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify events were loaded
	summary := store.GetSummary(7)
	if len(summary.ProviderModels) != 1 {
		t.Fatalf("expected 1 provider model from loaded events, got %d", len(summary.ProviderModels))
	}
	if summary.ProviderModels[0].InputTokens != 500 {
		t.Fatalf("expected 500 input tokens from loaded events, got %d", summary.ProviderModels[0].InputTokens)
	}
}

func TestSetPersistence_TrimsOldEventsOnLoad(t *testing.T) {
	store := NewStore(1) // 1 day retention
	now := time.Now()

	// Create mock with old events (beyond retention)
	mock := &mockPersistence{
		events: []UsageEvent{
			{
				Timestamp:    now.Add(-72 * time.Hour), // 3 days old
				Provider:     "openai",
				RequestModel: "openai:gpt-4o",
				InputTokens:  1000,
				OutputTokens: 500,
			},
		},
	}

	err := store.SetPersistence(mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Old events should be trimmed
	summary := store.GetSummary(7)
	if len(summary.ProviderModels) != 0 {
		t.Fatalf("expected old events to be trimmed, got %d provider models", len(summary.ProviderModels))
	}
}

func TestGetPersistenceStatus_NoPersistence(t *testing.T) {
	store := NewStore(30)

	lastErr, lastSaveTime, hasPersistence := store.GetPersistenceStatus()

	if lastErr != nil {
		t.Error("expected no error when no persistence")
	}
	if !lastSaveTime.IsZero() {
		t.Error("expected zero time when no persistence")
	}
	if hasPersistence {
		t.Error("expected hasPersistence to be false")
	}
}

func TestGetPersistenceStatus_WithPersistence(t *testing.T) {
	store := NewStore(30)
	store.saveDebounce = 5 * time.Millisecond

	mock := &mockPersistence{}
	err := store.SetPersistence(mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, _, hasPersistence := store.GetPersistenceStatus()
	if !hasPersistence {
		t.Error("expected hasPersistence to be true")
	}

	// Trigger a save
	store.Record(UsageEvent{
		Timestamp:    time.Now(),
		Provider:     "test",
		RequestModel: "test:model",
		InputTokens:  100,
		OutputTokens: 50,
	})

	// Wait for debounced save
	time.Sleep(50 * time.Millisecond)

	lastErr, lastSaveTime, _ := store.GetPersistenceStatus()
	if lastErr != nil {
		t.Errorf("expected no error after successful save, got %v", lastErr)
	}
	if lastSaveTime.IsZero() {
		t.Error("expected lastSaveTime to be set after successful save")
	}
}

// errorPersistence is a mock that returns an error on save
type errorPersistence struct{}

func (e *errorPersistence) LoadUsageHistory() ([]UsageEvent, error) {
	return nil, nil
}

func (e *errorPersistence) SaveUsageHistory(events []UsageEvent) error {
	return errSaveTestError
}

var errSaveTestError = &testError{"forced save error"}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestSetOnSaveError_CallbackCalled(t *testing.T) {
	store := NewStore(30)
	store.saveDebounce = 5 * time.Millisecond

	errReceived := make(chan error, 1)
	store.SetOnSaveError(func(err error) {
		errReceived <- err
	})

	err := store.SetPersistence(&errorPersistence{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Trigger a save
	store.Record(UsageEvent{
		Timestamp:    time.Now(),
		Provider:     "test",
		RequestModel: "test:model",
		InputTokens:  100,
		OutputTokens: 50,
	})

	// Wait for callback
	select {
	case err := <-errReceived:
		if err == nil {
			t.Error("expected error in callback")
		}
		if err.Error() != "forced save error" {
			t.Errorf("expected 'forced save error', got %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for error callback")
	}

	// Check persistence status reflects error
	lastErr, _, _ := store.GetPersistenceStatus()
	if lastErr == nil {
		t.Error("expected lastSaveError to be set after failed save")
	}
}
