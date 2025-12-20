package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestNewCostPersistenceAdapter(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)

	adapter := NewCostPersistenceAdapter(persistence)
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}
	if adapter.config != persistence {
		t.Fatal("expected config to match persistence")
	}
}

func TestCostPersistenceAdapter_SaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	adapter := NewCostPersistenceAdapter(persistence)

	events := []cost.UsageEvent{
		{
			Timestamp:     time.Now(),
			Provider:      "openai",
			RequestModel:  "gpt-4",
			ResponseModel: "gpt-4",
			UseCase:       "patrol",
			InputTokens:   100,
			OutputTokens:  50,
			TargetType:    "vm",
			TargetID:      "node1-100",
			FindingID:     "finding-123",
		},
		{
			Timestamp:     time.Now().Add(-time.Hour),
			Provider:      "anthropic",
			RequestModel:  "claude-3-sonnet",
			ResponseModel: "claude-3-sonnet",
			UseCase:       "chat",
			InputTokens:   200,
			OutputTokens:  100,
			TargetType:    "container",
			TargetID:      "node1-101",
			FindingID:     "",
		},
	}

	// Save events
	err := adapter.SaveUsageHistory(events)
	if err != nil {
		t.Fatalf("failed to save usage history: %v", err)
	}

	// Load events back
	loaded, err := adapter.LoadUsageHistory()
	if err != nil {
		t.Fatalf("failed to load usage history: %v", err)
	}

	if len(loaded) != len(events) {
		t.Fatalf("expected %d events, got %d", len(events), len(loaded))
	}

	// Verify first event
	if loaded[0].Provider != events[0].Provider {
		t.Errorf("expected provider %q, got %q", events[0].Provider, loaded[0].Provider)
	}
	if loaded[0].InputTokens != events[0].InputTokens {
		t.Errorf("expected input tokens %d, got %d", events[0].InputTokens, loaded[0].InputTokens)
	}
	if loaded[0].UseCase != events[0].UseCase {
		t.Errorf("expected use case %q, got %q", events[0].UseCase, loaded[0].UseCase)
	}
}

func TestCostPersistenceAdapter_LoadEmpty(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	adapter := NewCostPersistenceAdapter(persistence)

	// Load from empty persistence should return empty slice, not error
	loaded, err := adapter.LoadUsageHistory()
	if err != nil {
		t.Fatalf("expected no error for empty load, got: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected empty slice, got %d events", len(loaded))
	}
}

func TestCostPersistenceAdapter_SaveEmpty(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	adapter := NewCostPersistenceAdapter(persistence)

	// Save empty slice should work
	err := adapter.SaveUsageHistory([]cost.UsageEvent{})
	if err != nil {
		t.Fatalf("failed to save empty usage history: %v", err)
	}
}
