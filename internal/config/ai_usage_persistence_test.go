package config

import (
	"testing"
	"time"
)

func TestSaveLoadAIUsageHistory(t *testing.T) {
	dir := t.TempDir()
	cp := NewConfigPersistence(dir)

	now := time.Now()
	events := []AIUsageEventRecord{
		{
			Timestamp:    now,
			Provider:     "openai",
			RequestModel: "openai:gpt-4o",
			InputTokens:  123,
			OutputTokens: 45,
			UseCase:      "chat",
			TargetType:   "vm",
			TargetID:     "vm-101",
		},
	}

	if err := cp.SaveAIUsageHistory(events); err != nil {
		t.Fatalf("SaveAIUsageHistory: %v", err)
	}

	loaded, err := cp.LoadAIUsageHistory()
	if err != nil {
		t.Fatalf("LoadAIUsageHistory: %v", err)
	}

	if len(loaded.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(loaded.Events))
	}
	if loaded.Events[0].Provider != "openai" || loaded.Events[0].InputTokens != 123 {
		t.Fatalf("loaded event mismatch: %+v", loaded.Events[0])
	}
}
