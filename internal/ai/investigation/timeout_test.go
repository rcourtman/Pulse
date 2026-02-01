package investigation

import (
	"context"
	"fmt"
	"testing"
)

func TestIsTimeoutError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"bare deadline exceeded", context.DeadlineExceeded, true},
		{"wrapped deadline exceeded", fmt.Errorf("failed: %w", context.DeadlineExceeded), true},
		{"string contains deadline", fmt.Errorf("context deadline exceeded during stream"), true},
		{"unrelated error", fmt.Errorf("connection refused"), false},
		{"empty error", fmt.Errorf(""), false},
		{"context canceled", context.Canceled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTimeoutError(tt.err)
			if got != tt.expected {
				t.Errorf("isTimeoutError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestStore_SetOutcome(t *testing.T) {
	store := NewStore("")

	session := store.Create("finding-1", "session-1")

	// Set outcome on existing session
	ok := store.SetOutcome(session.ID, OutcomeTimedOut)
	if !ok {
		t.Fatalf("expected SetOutcome to succeed")
	}

	retrieved := store.Get(session.ID)
	if retrieved.Outcome != OutcomeTimedOut {
		t.Fatalf("expected outcome %s, got %s", OutcomeTimedOut, retrieved.Outcome)
	}

	// Set outcome on non-existent session
	ok = store.SetOutcome("nonexistent", OutcomeTimedOut)
	if ok {
		t.Fatalf("expected SetOutcome to fail for nonexistent session")
	}
}

func TestDefaultConfig_TimeoutCooldown(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.TimeoutCooldownDuration == 0 {
		t.Fatalf("expected non-zero TimeoutCooldownDuration")
	}
	if cfg.TimeoutCooldownDuration >= cfg.CooldownDuration {
		t.Fatalf("expected TimeoutCooldownDuration (%v) < CooldownDuration (%v)",
			cfg.TimeoutCooldownDuration, cfg.CooldownDuration)
	}
}
