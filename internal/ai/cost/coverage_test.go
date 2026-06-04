package cost

import (
	"errors"
	"testing"
	"time"
)

type savePersistence struct {
	saveCh  chan []UsageEvent
	loadErr error
	saveErr error
}

func (p *savePersistence) LoadUsageHistory() ([]UsageEvent, error) {
	if p.loadErr != nil {
		return nil, p.loadErr
	}
	return nil, nil
}

func (p *savePersistence) SaveUsageHistory(events []UsageEvent) error {
	if p.saveCh != nil {
		p.saveCh <- events
	}
	return p.saveErr
}

func TestNormalizeModelForProvider(t *testing.T) {
	model := normalizeModelForProvider("openai", "openai:gpt-4o", "")
	if model != "gpt-4o" {
		t.Fatalf("expected gpt-4o, got %q", model)
	}

	model = normalizeModelForProvider("openai", "anthropic:claude", "")
	if model != "anthropic:claude" {
		t.Fatalf("expected request model passthrough, got %q", model)
	}

	model = normalizeModelForProvider("openai", "", "openai:gpt-4o")
	if model != "gpt-4o" {
		t.Fatalf("expected gpt-4o from response model, got %q", model)
	}

	model = normalizeModelForProvider("openai", "", "anthropic:claude")
	if model != "anthropic:claude" {
		t.Fatalf("expected response model passthrough, got %q", model)
	}
}

func TestLookupPriceAndPatterns(t *testing.T) {
	if _, ok := lookupPrice("", "gpt-4o", 0); ok {
		t.Fatal("expected empty provider to be unknown")
	}
	if _, ok := lookupPrice("openai", "", 0); ok {
		t.Fatal("expected empty model to be unknown")
	}
	if _, ok := lookupPrice("unknown", "model", 0); ok {
		t.Fatal("expected unknown provider to be unknown")
	}

	price, ok := lookupPrice("openai", "gpt-4o-mini", 0)
	if !ok || price.InputUSDPerMTok == 0 {
		t.Fatalf("expected openai pricing match, got ok=%v price=%+v", ok, price)
	}

	if !matchPattern("gpt-4o-mini", "gpt-4o*") {
		t.Fatal("expected prefix pattern to match")
	}
	if !matchPattern("anything", "*") {
		t.Fatal("expected wildcard pattern to match")
	}
	if matchPattern("gpt-4o-mini", "gpt-4o") {
		t.Fatal("expected exact match to fail")
	}
	if !matchPattern("gpt-4o", "gpt-4o") {
		t.Fatal("expected exact match to succeed")
	}
}

func TestLookupPriceUsesTieredGeminiPricing(t *testing.T) {
	under200k, ok := lookupPrice("gemini", "gemini-2.5-pro", 150_000)
	if !ok {
		t.Fatal("expected Gemini 2.5 Pro pricing to resolve")
	}
	if under200k.InputUSDPerMTok != 1.25 || under200k.OutputUSDPerMTok != 10.00 {
		t.Fatalf("unexpected <=200k tier: %+v", under200k)
	}

	over200k, ok := lookupPrice("gemini", "gemini-2.5-pro", 250_000)
	if !ok {
		t.Fatal("expected Gemini 2.5 Pro high-tier pricing to resolve")
	}
	if over200k.InputUSDPerMTok != 2.50 || over200k.OutputUSDPerMTok != 15.00 {
		t.Fatalf("unexpected >200k tier: %+v", over200k)
	}

	pro31, ok := lookupPrice("gemini", "gemini-3.1-pro-preview", 150_000)
	if !ok {
		t.Fatal("expected Gemini 3.1 Pro Preview pricing to resolve")
	}
	if pro31.InputUSDPerMTok != 2.00 || pro31.OutputUSDPerMTok != 12.00 {
		t.Fatalf("unexpected Gemini 3.1 Pro Preview pricing: %+v", pro31)
	}

	flash35, ok := lookupPrice("gemini", "gemini-3.5-flash", 50_000)
	if !ok {
		t.Fatal("expected Gemini 3.5 Flash pricing to resolve")
	}
	if flash35.InputUSDPerMTok != 1.50 || flash35.OutputUSDPerMTok != 9.00 {
		t.Fatalf("unexpected Gemini 3.5 Flash pricing: %+v", flash35)
	}

	flash25, ok := lookupPrice("gemini", "gemini-2.5-flash", 50_000)
	if !ok {
		t.Fatal("expected Gemini 2.5 Flash pricing to resolve")
	}
	if flash25.InputUSDPerMTok != 0.30 || flash25.OutputUSDPerMTok != 2.50 {
		t.Fatalf("unexpected Gemini 2.5 Flash pricing: %+v", flash25)
	}

	flashLite25, ok := lookupPrice("gemini", "gemini-2.5-flash-lite", 50_000)
	if !ok {
		t.Fatal("expected Gemini 2.5 Flash-Lite pricing to resolve")
	}
	if flashLite25.InputUSDPerMTok != 0.10 || flashLite25.OutputUSDPerMTok != 0.40 {
		t.Fatalf("unexpected Gemini 2.5 Flash-Lite pricing: %+v", flashLite25)
	}
}

func TestSetPersistence_LoadError(t *testing.T) {
	store := NewStore(30)
	mock := &savePersistence{loadErr: errors.New("load failed")}
	if err := store.SetPersistence(mock); err == nil {
		t.Fatal("expected load error")
	}
}

func TestRecord_SetsTimestampAndTrims(t *testing.T) {
	store := NewStore(1)
	store.Record(UsageEvent{
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
		InputTokens:  10,
		OutputTokens: 5,
	})
	if store.events[0].Timestamp.IsZero() {
		t.Fatal("expected timestamp to be set")
	}

	old := time.Now().Add(-72 * time.Hour)
	store.Record(UsageEvent{
		Timestamp:    old,
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
	})
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, e := range store.events {
		if e.Timestamp.Before(time.Now().Add(-48 * time.Hour)) {
			t.Fatal("expected old events to be trimmed")
		}
	}
}

func TestTrimLocked_NoRetention(t *testing.T) {
	store := NewStore(1)
	store.maxDays = 0
	store.events = []UsageEvent{{Timestamp: time.Now().Add(-365 * 24 * time.Hour)}}
	store.trimLocked(time.Now())
	if len(store.events) != 1 {
		t.Fatalf("expected events preserved, got %d", len(store.events))
	}
}

func TestListEvents_DefaultAndMaxDays(t *testing.T) {
	store := NewStore(7)
	now := time.Now()
	store.Record(UsageEvent{
		Timestamp:    now.Add(-8 * 24 * time.Hour),
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
	})

	events := store.ListEvents(0)
	if len(events) != 0 {
		t.Fatalf("expected no events with maxDays cutoff, got %d", len(events))
	}
}

func TestGetSummary_UnknownPricing(t *testing.T) {
	store := NewStore(30)
	store.Record(UsageEvent{
		Timestamp:    time.Now(),
		Provider:     "unknown",
		RequestModel: "mystery-model",
		InputTokens:  100,
		OutputTokens: 50,
	})

	summary := store.GetSummary(7)
	if summary.Totals.PricingKnown {
		t.Fatal("expected pricing to be unknown")
	}
	if summary.Totals.EstimatedUSD != 0 {
		t.Fatalf("expected zero USD, got %f", summary.Totals.EstimatedUSD)
	}
}

func TestGetSummary_DefaultDays(t *testing.T) {
	store := NewStore(30)
	summary := store.GetSummary(0)
	if summary.Days != 30 {
		t.Fatalf("expected default days=30, got %d", summary.Days)
	}
}

func TestScheduleSaveLocked_DebounceAndCancel(t *testing.T) {
	store := NewStore(30)
	store.saveDebounce = 20 * time.Millisecond

	saveCh := make(chan []UsageEvent, 1)
	store.persistence = &savePersistence{saveCh: saveCh}

	store.mu.Lock()
	store.scheduleSaveLocked()
	store.mu.Unlock()

	time.Sleep(5 * time.Millisecond)
	store.mu.Lock()
	store.savePending = false
	store.mu.Unlock()

	select {
	case <-saveCh:
		t.Fatal("expected no save when pending cleared")
	case <-time.After(40 * time.Millisecond):
	}
}

func TestScheduleSaveLocked_StopTimer(t *testing.T) {
	store := NewStore(30)
	store.saveDebounce = 20 * time.Millisecond
	store.persistence = &savePersistence{}

	store.mu.Lock()
	store.scheduleSaveLocked()
	store.mu.Unlock()

	time.Sleep(5 * time.Millisecond)

	store.mu.Lock()
	store.scheduleSaveLocked()
	store.mu.Unlock()
}

func TestScheduleSaveLocked_LogsError(t *testing.T) {
	store := NewStore(30)
	store.saveDebounce = 5 * time.Millisecond
	store.persistence = &savePersistence{saveErr: errors.New("save failed")}

	store.Record(UsageEvent{
		Timestamp:    time.Now(),
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
	})

	time.Sleep(20 * time.Millisecond)
}

func TestScheduleSaveLocked_Saves(t *testing.T) {
	store := NewStore(30)
	store.saveDebounce = 5 * time.Millisecond
	saveCh := make(chan []UsageEvent, 1)
	store.persistence = &savePersistence{saveCh: saveCh}

	store.Record(UsageEvent{
		Timestamp:    time.Now(),
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
	})

	select {
	case <-saveCh:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("expected save to fire")
	}
}

func TestFlush_ReturnsPersistenceError(t *testing.T) {
	store := NewStore(30)
	store.persistence = &savePersistence{saveErr: errors.New("save failed")}
	store.Record(UsageEvent{
		Timestamp:    time.Now(),
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
	})
	if err := store.Flush(); err == nil {
		t.Fatal("expected flush error")
	}
}

func TestSummarizeUseCases_OrderAndUnknown(t *testing.T) {
	events := []UsageEvent{
		{UseCase: "", InputTokens: 1},
		{UseCase: "patrol", InputTokens: 1},
		{UseCase: "chat", InputTokens: 1},
		{UseCase: "custom", InputTokens: 1},
		{UseCase: "alpha", InputTokens: 1},
	}
	rollup := summarizeUseCases(events)

	if len(rollup) != 5 {
		t.Fatalf("expected 5 use cases, got %d", len(rollup))
	}

	names := make([]string, 0, len(rollup))
	for _, uc := range rollup {
		names = append(names, uc.UseCase)
	}
	if names[0] != "chat" || names[1] != "patrol" || names[2] != "unknown" {
		t.Fatalf("unexpected order: %v", names)
	}
	if names[3] != "alpha" || names[4] != "custom" {
		t.Fatalf("expected alpha/custom ordering at end, got %v", names)
	}
}

func TestSummarizeTargets_SortAndLimit(t *testing.T) {
	var events []UsageEvent
	for i := 0; i < 25; i++ {
		events = append(events, UsageEvent{
			Provider:     "ollama",
			RequestModel: "llama3",
			InputTokens:  100 + i,
			OutputTokens: 10,
			TargetType:   "vm",
			TargetID:     "id-" + string(rune('a'+i)),
		})
	}
	events = append(events, UsageEvent{
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
		InputTokens:  1000,
		OutputTokens: 100,
		TargetType:   "node",
		TargetID:     "node-1",
	})
	events = append(events, UsageEvent{
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
		InputTokens:  1000,
		OutputTokens: 100,
		TargetType:   "",
		TargetID:     "skip",
	})
	events = append(events, UsageEvent{
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
		InputTokens:  1000,
		OutputTokens: 100,
		TargetType:   "node",
		TargetID:     "",
	})

	rollup := summarizeTargets(events)
	if len(rollup) != 20 {
		t.Fatalf("expected 20 targets after limit, got %d", len(rollup))
	}
	if rollup[0].TargetID != "node-1" || rollup[0].TargetType != "node" {
		t.Fatalf("expected highest cost target first, got %+v", rollup[0])
	}
}

func TestSummarizeTargets_SortTiebreakers(t *testing.T) {
	events := []UsageEvent{
		{
			Provider:     "ollama",
			RequestModel: "llama3",
			InputTokens:  100,
			OutputTokens: 10,
			TargetType:   "vm",
			TargetID:     "b",
		},
		{
			Provider:     "ollama",
			RequestModel: "llama3",
			InputTokens:  100,
			OutputTokens: 10,
			TargetType:   "node",
			TargetID:     "a",
		},
		{
			Provider:     "ollama",
			RequestModel: "llama3",
			InputTokens:  100,
			OutputTokens: 10,
			TargetType:   "vm",
			TargetID:     "a",
		},
	}

	rollup := summarizeTargets(events)
	if len(rollup) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(rollup))
	}
	if rollup[0].TargetType != "node" {
		t.Fatalf("expected node target first, got %+v", rollup[0])
	}
	if rollup[1].TargetID != "a" || rollup[2].TargetID != "b" {
		t.Fatalf("expected target ID ordering, got %+v", rollup)
	}
}
