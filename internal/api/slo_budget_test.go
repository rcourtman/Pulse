package api

import (
	"testing"
	"time"
)

func TestAPISLOBudgetSelection(t *testing.T) {
	const local = 90 * time.Millisecond
	const hosted = 550 * time.Millisecond
	for _, tc := range []struct {
		name, actions, runID, strict string
		target                       time.Duration
		enforced                     bool
	}{
		{"local", "", "", "", local, false},
		{"shared-worker", "true", "", "", local, false},
		{"hosted", "true", "123", "", hosted, true},
		{"strict-local", "", "", "true", local, true},
		{"strict-shared-worker", "true", "", "true", local, true},
		{"strict-hosted", "true", "123", "true", local, true},
		{"disabled-hosted", "true", "123", "false", hosted, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GITHUB_ACTIONS", tc.actions)
			t.Setenv("GITHUB_RUN_ID", tc.runID)
			t.Setenv("PULSE_STRICT_LOCAL_API_SLOS", tc.strict)
			if got := effectiveAPISLOTarget(local, hosted); got != tc.target {
				t.Fatalf("target=%v, want %v", got, tc.target)
			}
			if got := apiLatencyBudgetEnforced(); got != tc.enforced {
				t.Fatalf("enforced=%v, want %v", got, tc.enforced)
			}
		})
	}
}
