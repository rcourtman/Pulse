package eval

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBranchcov0723Am_PatrolAssertSignalCoverage exercises every branch of the
// assertion closure returned by PatrolAssertSignalCoverage. Each subtest builds
// its own PatrolRunResult so none depends on state mutated by another.
func TestBranchcov0723Am_PatrolAssertSignalCoverage(t *testing.T) {
	tests := []struct {
		name       string
		minRate    float64
		result     *PatrolRunResult
		wantPassed bool
		wantMsg    string
	}{
		{
			name:       "nil result fails with no result available",
			minRate:    0.5,
			result:     nil,
			wantPassed: false,
			wantMsg:    "No result available",
		},
		{
			name:    "quality nil and no tool calls yields not measured",
			minRate: 0.5,
			result: &PatrolRunResult{
				// Quality left nil so the assertion invokes EvaluatePatrolQuality,
				// which returns CoverageKnown=false when no tool calls were captured.
				ToolCalls: nil,
			},
			wantPassed: true,
			wantMsg:    "Signal coverage not measured (no tool calls captured)",
		},
		{
			name:    "coverage known false passes with not measured",
			minRate: 0.5,
			result: &PatrolRunResult{
				Quality: &PatrolQualityReport{CoverageKnown: false},
			},
			wantPassed: true,
			wantMsg:    "Signal coverage not measured (no tool calls captured)",
		},
		{
			name:    "zero signals passes with no deterministic signals",
			minRate: 0.5,
			result: &PatrolRunResult{
				Quality: &PatrolQualityReport{
					CoverageKnown: true,
					SignalsTotal:  0,
				},
			},
			wantPassed: true,
			wantMsg:    "No deterministic signals detected",
		},
		{
			name:    "coverage above min passes",
			minRate: 0.5,
			result: &PatrolRunResult{
				Quality: &PatrolQualityReport{
					CoverageKnown:  true,
					SignalsTotal:   4,
					SignalsMatched: 3,
					SignalCoverage: 0.75,
				},
			},
			wantPassed: true,
			wantMsg:    "Signal coverage 75% (3/4), min 50%",
		},
		{
			// The comparison operator is >=, so coverage equal to min passes.
			name:    "coverage exactly at min passes via >= operator",
			minRate: 0.5,
			result: &PatrolRunResult{
				Quality: &PatrolQualityReport{
					CoverageKnown:  true,
					SignalsTotal:   4,
					SignalsMatched: 2,
					SignalCoverage: 0.5,
				},
			},
			wantPassed: true,
			wantMsg:    "Signal coverage 50% (2/4), min 50%",
		},
		{
			name:    "coverage below min fails with below message",
			minRate: 0.5,
			result: &PatrolRunResult{
				Quality: &PatrolQualityReport{
					CoverageKnown:  true,
					SignalsTotal:   5,
					SignalsMatched: 2,
					SignalCoverage: 0.4,
				},
			},
			wantPassed: false,
			wantMsg:    "Signal coverage 40% (2/5) below min 50%",
		},
		{
			// minRate=0: any non-negative coverage satisfies >=, including 0%
			// with SignalsTotal>0 (skips the SignalsTotal==0 early-return).
			name:    "minRate zero passes zero coverage with signals present",
			minRate: 0,
			result: &PatrolRunResult{
				Quality: &PatrolQualityReport{
					CoverageKnown:  true,
					SignalsTotal:   4,
					SignalsMatched: 0,
					SignalCoverage: 0,
				},
			},
			wantPassed: true,
			wantMsg:    "Signal coverage 0% (0/4), min 0%",
		},
		{
			name:    "minRate one fails when coverage below one",
			minRate: 1,
			result: &PatrolRunResult{
				Quality: &PatrolQualityReport{
					CoverageKnown:  true,
					SignalsTotal:   2,
					SignalsMatched: 1,
					SignalCoverage: 0.5,
				},
			},
			wantPassed: false,
			wantMsg:    "Signal coverage 50% (1/2) below min 100%",
		},
		{
			name:    "minRate one passes when coverage is full",
			minRate: 1,
			result: &PatrolRunResult{
				Quality: &PatrolQualityReport{
					CoverageKnown:  true,
					SignalsTotal:   2,
					SignalsMatched: 2,
					SignalCoverage: 1.0,
				},
			},
			wantPassed: true,
			wantMsg:    "Signal coverage 100% (2/2), min 100%",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertion := PatrolAssertSignalCoverage(tc.minRate)
			got := assertion(tc.result)
			assert.Equal(t, tc.wantPassed, got.Passed, "Message: %s", got.Message)
			assert.Equal(t, "signal_coverage", got.Name)
			assert.Equal(t, tc.wantMsg, got.Message)
		})
	}
}

// TestBranchcov0723Am_ApprovalWriteCommand covers every branch of the pure
// approvalWriteCommand helper, including the zero-value evalTargets.
func TestBranchcov0723Am_ApprovalWriteCommand(t *testing.T) {
	tests := []struct {
		name string
		t    evalTargets
		want string
	}{
		{
			name: "zero value evalTargets yields default touch command",
			t:    evalTargets{},
			want: "touch /tmp/pulse_eval_approval",
		},
		{
			name: "literal true yields default touch command",
			t:    evalTargets{WriteCommand: "true"},
			want: "touch /tmp/pulse_eval_approval",
		},
		{
			name: "whitespace padded true trims to true and yields default",
			t:    evalTargets{WriteCommand: "  true  "},
			want: "touch /tmp/pulse_eval_approval",
		},
		{
			name: "whitespace only trims to empty and yields default",
			t:    evalTargets{WriteCommand: "   "},
			want: "touch /tmp/pulse_eval_approval",
		},
		{
			name: "custom command returned unchanged when already trimmed",
			t:    evalTargets{WriteCommand: "echo hello"},
			want: "echo hello",
		},
		{
			name: "custom command returned trimmed",
			t:    evalTargets{WriteCommand: "  echo hello  "},
			want: "echo hello",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := approvalWriteCommand(tc.t)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestBranchcov0723Am_PatrolSignalCoverageMin covers the two arms of the pure
// env-driven patrolSignalCoverageMin helper.
func TestBranchcov0723Am_PatrolSignalCoverageMin(t *testing.T) {
	t.Run("default when env unset", func(t *testing.T) {
		ensureEnvUnset(t, "EVAL_PATROL_SIGNAL_COVERAGE_MIN")
		assert.InDelta(t, 0.75, patrolSignalCoverageMin(), 1e-9)
	})

	t.Run("env value is returned when set", func(t *testing.T) {
		t.Setenv("EVAL_PATROL_SIGNAL_COVERAGE_MIN", "0.9")
		assert.InDelta(t, 0.9, patrolSignalCoverageMin(), 1e-9)
	})
}

// ensureEnvUnset removes the env var for the duration of the test and restores
// the prior value (if any) on cleanup.
func ensureEnvUnset(t *testing.T, key string) {
	t.Helper()
	prev, hadPrev := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if hadPrev {
			_ = os.Setenv(key, prev)
		}
	})
}
