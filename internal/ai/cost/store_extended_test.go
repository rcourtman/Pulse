package cost

import (
	"testing"
	"time"
)

// Additional tests to improve coverage

func TestNewStore_DefaultMaxDays(t *testing.T) {
	// Test with default
	store := NewStore(0)
	if store == nil {
		t.Fatal("Expected non-nil store")
	}

	// Should use default max days (365)
	if store.maxDays != DefaultMaxDays {
		t.Errorf("Expected default max days %d, got %d", DefaultMaxDays, store.maxDays)
	}
}

func TestNewStore_CustomMaxDays(t *testing.T) {
	store := NewStore(30)
	if store.maxDays != 30 {
		t.Errorf("Expected 30 max days, got %d", store.maxDays)
	}
}

func TestListEvents_Empty(t *testing.T) {
	store := NewStore(30)
	events := store.ListEvents(30)

	if len(events) != 0 {
		t.Errorf("Expected empty events, got %d", len(events))
	}
}

func TestListEvents_WithData(t *testing.T) {
	store := NewStore(30)
	now := time.Now()

	store.Record(UsageEvent{
		Timestamp:    now,
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
		InputTokens:  100,
		OutputTokens: 50,
		UseCase:      "chat",
	})

	events := store.ListEvents(30)
	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}
}

func TestListEvents_FiltersByDays(t *testing.T) {
	store := NewStore(30)
	now := time.Now()

	// Event from 10 days ago
	store.Record(UsageEvent{
		Timestamp:    now.Add(-10 * 24 * time.Hour),
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
		InputTokens:  100,
		OutputTokens: 50,
	})

	// Request only last 5 days - should not include the event
	events := store.ListEvents(5)
	if len(events) != 0 {
		t.Errorf("Expected 0 events for 5 day window, got %d", len(events))
	}

	// Request last 15 days - should include
	events = store.ListEvents(15)
	if len(events) != 1 {
		t.Errorf("Expected 1 event for 15 day window, got %d", len(events))
	}
}

func TestGetSummary_Empty(t *testing.T) {
	store := NewStore(30)
	summary := store.GetSummary(30)

	if len(summary.ProviderModels) != 0 {
		t.Errorf("Expected empty provider models, got %d", len(summary.ProviderModels))
	}
	if summary.Totals.TotalTokens != 0 {
		t.Errorf("Expected 0 total tokens, got %d", summary.Totals.TotalTokens)
	}
}

func TestFlush_NoPersistence(t *testing.T) {
	store := NewStore(30)

	// Record something
	store.Record(UsageEvent{
		Timestamp:    time.Now(),
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
		InputTokens:  100,
		OutputTokens: 50,
	})

	// Flush without persistence should not error
	err := store.Flush()
	if err != nil {
		t.Errorf("Flush without persistence should not error: %v", err)
	}
}

func TestEstimateUSD_UnknownProvider(t *testing.T) {
	usd, ok, _ := EstimateUSD("unknown_provider", "unknown_model", 1000, 1000)

	if ok {
		t.Error("Should not find pricing for unknown provider")
	}
	if usd != 0 {
		t.Errorf("Expected 0 USD for unknown provider, got %f", usd)
	}
}

func TestEstimateUSD_OllamaFree(t *testing.T) {
	usd, ok, _ := EstimateUSD("ollama", "llama2", 1_000_000, 1_000_000)

	if !ok {
		t.Error("Ollama pricing should be known (free)")
	}
	if usd != 0 {
		t.Errorf("Ollama should be free, got $%f", usd)
	}
}

func TestEstimateUSD_AnthropicModels(t *testing.T) {
	tests := []struct {
		model string
		known bool
	}{
		{"claude-sonnet-4-20250514", true},  // matches claude-sonnet*
		{"claude-opus-4-20250514", true},    // matches claude-opus*
		{"claude-haiku-something", true},    // matches claude-haiku*
		{"unknown-claude", false},           // no match
	}

	for _, tt := range tests {
		_, ok, _ := EstimateUSD("anthropic", tt.model, 1000, 1000)
		if ok != tt.known {
			t.Errorf("EstimateUSD(anthropic, %s): expected known=%v, got %v", tt.model, tt.known, ok)
		}
	}
}

func TestEstimateUSD_OpenAIModels(t *testing.T) {
	tests := []struct {
		model string
		known bool
	}{
		{"gpt-4o", true},           // matches gpt-4o*
		{"gpt-4o-mini", true},      // matches gpt-4o-mini*
		{"gpt-4o-mini-2024", true}, // matches gpt-4o-mini*
		{"unknown-gpt", false},     // no match
	}

	for _, tt := range tests {
		_, ok, _ := EstimateUSD("openai", tt.model, 1000, 1000)
		if ok != tt.known {
			t.Errorf("EstimateUSD(openai, %s): expected known=%v, got %v", tt.model, tt.known, ok)
		}
	}
}

func TestEstimateUSD_DeepSeekModels(t *testing.T) {
	tests := []struct {
		model    string
		known    bool
	}{
		{"deepseek-chat", true},
		{"deepseek-coder", true},
		{"deepseek-reasoner", true},
		{"unknown-deepseek", false},
	}

	for _, tt := range tests {
		_, ok, _ := EstimateUSD("deepseek", tt.model, 1000, 1000)
		if ok != tt.known {
			t.Errorf("EstimateUSD(deepseek, %s): expected known=%v, got %v", tt.model, tt.known, ok)
		}
	}
}

func TestSummary_Totals(t *testing.T) {
	store := NewStore(30)
	now := time.Now()

	store.Record(UsageEvent{
		Timestamp:    now,
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
		InputTokens:  100,
		OutputTokens: 50,
	})
	store.Record(UsageEvent{
		Timestamp:    now,
		Provider:     "anthropic",
		RequestModel: "anthropic:claude-sonnet-4-20250514",
		InputTokens:  200,
		OutputTokens: 100,
	})

	summary := store.GetSummary(30)

	// Check totals
	if summary.Totals.InputTokens != 300 {
		t.Errorf("Expected 300 input tokens, got %d", summary.Totals.InputTokens)
	}
	if summary.Totals.OutputTokens != 150 {
		t.Errorf("Expected 150 output tokens, got %d", summary.Totals.OutputTokens)
	}
	if summary.Totals.TotalTokens != 450 {
		t.Errorf("Expected 450 total tokens, got %d", summary.Totals.TotalTokens)
	}
}

func TestSummary_RetentionInfo(t *testing.T) {
	store := NewStore(30)
	summary := store.GetSummary(7)

	if summary.RetentionDays != 30 {
		t.Errorf("Expected retention days 30, got %d", summary.RetentionDays)
	}
	if summary.EffectiveDays != 7 {
		t.Errorf("Expected effective days 7, got %d", summary.EffectiveDays)
	}
}



func TestClear_MultipleTimes(t *testing.T) {
	store := NewStore(30)

	// Record, clear, record, clear
	store.Record(UsageEvent{
		Timestamp:    time.Now(),
		Provider:     "openai",
		RequestModel: "openai:gpt-4o",
		InputTokens:  100,
		OutputTokens: 50,
	})
	store.Clear()

	store.Record(UsageEvent{
		Timestamp:    time.Now(),
		Provider:     "anthropic",
		RequestModel: "anthropic:claude-sonnet",
		InputTokens:  100,
		OutputTokens: 50,
	})
	store.Clear()

	summary := store.GetSummary(30)
	if summary.Totals.TotalTokens != 0 {
		t.Errorf("Expected 0 tokens after clear, got %d", summary.Totals.TotalTokens)
	}
}
