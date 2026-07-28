package api

// Regression coverage for issue #1640: the "Patrol tools" readiness check read
// the cached snapshot's tool-protocol dimension on its own, so an evaluation
// that was interrupted after every tool scenario passed — tool protocol pass,
// overall status not assessed — reported "Patrol ready" from a check that
// never completed. Readiness now requires the completed overall verdict, and
// an inconclusive snapshot may not record a failure either: a not-ready tools
// check disables the Patrol run control, and #1640 promises an interrupted or
// cancelled check never blocks Patrol from running in Watch mode.

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// issue1640Snapshot builds a cached readiness snapshot with the given overall
// verdict and tool-protocol dimension, leaving the rest of the shape as the
// evaluator produces it.
func issue1640Snapshot(status string, success bool, cause ai.PatrolFailureCause, toolStatus, toolSummary string) *ai.PatrolModelReadinessResult {
	snapshot := &ai.PatrolModelReadinessResult{
		ProbeVersion: ai.PatrolModelReadinessProbeVersion,
		Status:       status,
		Success:      success,
		Cause:        cause,
		Provider:     "ollama",
		Model:        "test-model",
	}
	snapshot.Dimensions.ToolProtocol = ai.PatrolModelReadinessDimension{
		Status:   toolStatus,
		Summary:  toolSummary,
		Attempts: 3,
		Passed:   3,
	}
	return snapshot
}

func issue1640OllamaStaticCheck() patrolToolsCheck {
	return patrolToolsCheck{
		Status:  patrolReadinessWarning,
		Cause:   ai.PatrolFailureCauseModelToolSupportUnverified,
		Message: "Ollama connectivity alone does not prove tool support.",
		Action:  "open_provider_settings",
	}
}

func TestIssue1640ToolsCheckRequiresCompletedVerdict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		snapshot        *ai.PatrolModelReadinessResult
		static          patrolToolsCheck
		wantStatus      string
		wantRunnable    bool
		wantCause       ai.PatrolFailureCause
		wantMessagePart string
	}{
		{
			// The regression: every tool scenario passed, then the run was cut
			// short. The dimension says pass; the run has no verdict.
			name: "interrupted run with a passing tool dimension is not readiness",
			snapshot: issue1640Snapshot(
				ai.PatrolModelReadinessNotAssessed, false, ai.PatrolFailureCauseInterrupted,
				ai.PatrolModelReadinessPass,
				"All scenarios passed before the run was interrupted; multi-turn continuation was not assessed.",
			),
			static:          issue1640OllamaStaticCheck(),
			wantStatus:      patrolReadinessWarning,
			wantRunnable:    true,
			wantCause:       ai.PatrolFailureCauseInterrupted,
			wantMessagePart: "did not complete",
		},
		{
			name: "interrupted run with a partial tool dimension does not blame the model",
			snapshot: issue1640Snapshot(
				ai.PatrolModelReadinessNotAssessed, false, ai.PatrolFailureCauseInterrupted,
				ai.PatrolModelReadinessNotAssessed,
				"Interrupted after 1/3 scenarios passed; the remaining scenarios were not assessed.",
			),
			static:          issue1640OllamaStaticCheck(),
			wantStatus:      patrolReadinessWarning,
			wantRunnable:    true,
			wantCause:       ai.PatrolFailureCauseInterrupted,
			wantMessagePart: "did not complete",
		},
		{
			// A recovered panic on the evaluation path is a Pulse defect, not a
			// model verdict, and carries the same non-evidence status.
			name: "internal error carries no verdict either",
			snapshot: issue1640Snapshot(
				ai.PatrolModelReadinessFail, false, ai.PatrolFailureCauseInternalError,
				ai.PatrolModelReadinessNotAssessed, "Not assessed.",
			),
			static:       issue1640OllamaStaticCheck(),
			wantStatus:   patrolReadinessWarning,
			wantRunnable: true,
			wantCause:    ai.PatrolFailureCauseInternalError,
		},
		{
			name: "completed passing evaluation reports ready",
			snapshot: issue1640Snapshot(
				ai.PatrolModelReadinessPass, true, ai.PatrolFailureCauseNone,
				ai.PatrolModelReadinessPass,
				"Exact tool protocol passed 3/3 scenarios.",
			),
			static:          issue1640OllamaStaticCheck(),
			wantStatus:      patrolReadinessReady,
			wantRunnable:    true,
			wantCause:       ai.PatrolFailureCauseNone,
			wantMessagePart: "passed 3/3 scenarios",
		},
		{
			name: "completed failing evaluation blocks",
			snapshot: issue1640Snapshot(
				ai.PatrolModelReadinessWarning, false, ai.PatrolFailureCauseModelUnsupportedTools,
				ai.PatrolModelReadinessFail,
				"Exact tool protocol passed 0/3 scenarios.",
			),
			static:          issue1640OllamaStaticCheck(),
			wantStatus:      patrolReadinessNotReady,
			wantRunnable:    false,
			wantCause:       ai.PatrolFailureCauseModelUnsupportedTools,
			wantMessagePart: "last readiness evaluation",
		},
		{
			// A completed run whose tool protocol passed but whose overall
			// verdict did not: the dimension that failed carries the verdict on
			// its own check, so the tools check reports what it measured
			// without turning a latency warning into a hard block.
			name: "completed run short of an overall pass warns instead of claiming ready",
			snapshot: issue1640Snapshot(
				ai.PatrolModelReadinessWarning, false, ai.PatrolFailureCauseLatencyUnsuitable,
				ai.PatrolModelReadinessPass,
				"Exact tool protocol passed 3/3 scenarios.",
			),
			static:          issue1640OllamaStaticCheck(),
			wantStatus:      patrolReadinessWarning,
			wantRunnable:    true,
			wantCause:       ai.PatrolFailureCauseLatencyUnsuitable,
			wantMessagePart: "did not verify the model",
		},
		{
			// An incomplete run proves nothing that could clear a config-level
			// block, so the base classifier's verdict stands.
			name: "config-level block survives an inconclusive snapshot",
			snapshot: issue1640Snapshot(
				ai.PatrolModelReadinessNotAssessed, false, ai.PatrolFailureCauseInterrupted,
				ai.PatrolModelReadinessPass, "All scenarios passed before the run was interrupted.",
			),
			static: patrolToolsCheck{
				Status:  patrolReadinessNotReady,
				Cause:   ai.PatrolFailureCauseModelUnsupportedTools,
				Message: "The selected Patrol model is a reasoning-only model family.",
				Action:  "open_provider_settings",
			},
			wantStatus:      patrolReadinessNotReady,
			wantRunnable:    false,
			wantCause:       ai.PatrolFailureCauseModelUnsupportedTools,
			wantMessagePart: "reasoning-only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := patrolToolsCheckFromModelReadiness(tt.snapshot, "2m ago", tt.static)

			assert.Equal(t, tt.wantStatus, got.Status)
			assert.Equal(t, tt.wantCause, got.Cause)
			if tt.wantMessagePart != "" {
				assert.Contains(t, got.Message, tt.wantMessagePart)
			}
			require.NotEmpty(t, got.Message)

			// The readiness payload the Patrol page consumes: a not-ready check
			// is what disables the run control, so assert the runnability the
			// operator actually gets.
			readiness := summarizePatrolReadiness("ollama", "ollama:test-model", []PatrolReadinessCheck{{
				ID:      "tools",
				Status:  got.Status,
				Cause:   patrolFailureCauseResponse(got.Cause),
				Label:   "Patrol tools",
				Message: got.Message,
				Action:  got.Action,
			}})
			assert.Equal(t, tt.wantStatus, readiness.Status)
			assert.Equal(t, tt.wantRunnable, readiness.Ready)
		})
	}
}

// An absent snapshot keeps the base-config classifier untouched — the fallback
// an inconclusive snapshot mirrors.
func TestIssue1640AbsentSnapshotKeepsBaseConfigReadiness(t *testing.T) {
	t.Parallel()

	static := issue1640OllamaStaticCheck()
	assert.Equal(t, static, patrolToolsCheckFromModelReadiness(nil, "just now", static))
}
