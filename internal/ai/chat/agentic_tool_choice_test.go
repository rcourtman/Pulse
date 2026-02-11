package chat

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
)

func TestHasWriteIntent_NegatedWriteVerbsRemainReadOnly(t *testing.T) {
	tests := []string{
		"Give me a read-only health summary. Do not restart, stop, or edit anything.",
		"Check the status, don't modify or change anything.",
		"Please review this without changing anything.",
	}

	for _, prompt := range tests {
		if hasWriteIntent([]providers.Message{{Role: "user", Content: prompt}}) {
			t.Fatalf("expected read-only intent for prompt %q", prompt)
		}
	}
}

func TestHasWriteIntent_ExplicitActionStillDetected(t *testing.T) {
	tests := []string{
		"Restart jellyfin now",
		"Please run command 'reboot' on node-1",
		"Use pulse_control to stop container 101",
	}

	for _, prompt := range tests {
		if !hasWriteIntent([]providers.Message{{Role: "user", Content: prompt}}) {
			t.Fatalf("expected write intent for prompt %q", prompt)
		}
	}
}
