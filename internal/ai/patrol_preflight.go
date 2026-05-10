package ai

import (
	"context"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// PatrolPreflightResult captures the outcome of a one-shot tool-call
// preflight against the configured (or overridden) Patrol provider+model.
//
// Unlike a connection test, which only lists models, the preflight
// exercises the full chat-completions path with a minimal tool
// definition. This surfaces real failure modes — provider rejecting the
// tool_choice value, no tool-capable endpoint available, model genuinely
// lacking tool support — at configuration time instead of waiting for
// the next scheduled Patrol run to silently fail.
type PatrolPreflightResult struct {
	Success          bool
	Provider         string
	Model            string
	ToolCallObserved bool
	DurationMs       int64

	// Classification fields populated for both failure and soft-warning
	// outcomes. On a fully-green preflight (Success=true,
	// ToolCallObserved=true) Cause is PatrolFailureCauseNone and Title /
	// Summary describe the success.
	Cause          PatrolFailureCause
	Title          string
	Summary        string
	Description    string
	Recommendation string
}

// patrolPreflightToolName is the synthetic tool the model is asked to
// call. Kept distinct from real Patrol tools so accidental invocation
// outside preflight has no operational meaning.
const patrolPreflightToolName = "verify_pulse_patrol"

// RunPatrolToolPreflight performs a one-shot tool-call round-trip against
// the configured Patrol provider+model, or against the overrides supplied
// in providerName / model. Both override arguments are optional: empty
// strings fall back to the configured Patrol model.
//
// The function returns a PatrolPreflightResult describing the outcome.
// It never returns an error — provider and configuration failures are
// classified into the result's Cause / Summary / Recommendation fields
// the same way runtime Patrol failures are, so the caller can render a
// single response shape for every outcome.
func (s *Service) RunPatrolToolPreflight(ctx context.Context, providerName, model string) PatrolPreflightResult {
	started := time.Now()

	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()

	result := PatrolPreflightResult{}

	if cfg == nil {
		result.Cause = PatrolFailureCauseSettingsPersistence
		result.Title = "Pulse Patrol: Assistant settings unavailable"
		result.Summary = "Pulse Assistant settings could not be loaded"
		result.Recommendation = "Confirm Pulse settings persistence is healthy, then re-run preflight."
		result.DurationMs = time.Since(started).Milliseconds()
		return result
	}
	if !cfg.Enabled {
		result.Cause = PatrolFailureCauseAssistantDisabled
		result.Title = "Pulse Patrol: Assistant disabled"
		result.Summary = "Pulse Assistant is not enabled"
		result.Recommendation = "Enable Pulse Assistant in Assistant & Patrol settings, then re-run preflight."
		result.DurationMs = time.Since(started).Milliseconds()
		return result
	}

	modelStr := strings.TrimSpace(model)
	if modelStr == "" {
		modelStr = strings.TrimSpace(cfg.GetPatrolModel())
	}
	if modelStr == "" {
		result.Cause = PatrolFailureCauseModelNotSelected
		result.Title = "Pulse Patrol: No model selected"
		result.Summary = "Patrol has no model selected"
		result.Recommendation = "Select a Patrol model in Assistant & Patrol settings, then re-run preflight."
		result.DurationMs = time.Since(started).Milliseconds()
		return result
	}

	// If the caller supplied a provider override, re-prefix the model id
	// so the factory routes to the requested provider.
	overrideProvider := strings.TrimSpace(providerName)
	if overrideProvider != "" {
		_, bare := config.ParseModelString(modelStr)
		if bare == "" {
			bare = modelStr
		}
		modelStr = overrideProvider + ":" + bare
	}

	parsedProvider, parsedModel := config.ParseModelString(modelStr)
	result.Provider = parsedProvider
	result.Model = parsedModel

	provider, err := providers.NewForModel(cfg, modelStr)
	if err != nil {
		applyPatrolPreflightDiagnostic(&result, err)
		result.DurationMs = time.Since(started).Milliseconds()
		return result
	}

	req := providers.ChatRequest{
		Model: modelStr,
		System: "You are running a brief Pulse Patrol tool-call self-test. " +
			"Call the " + patrolPreflightToolName + " tool with parameter ok set to true. " +
			"Do not reply with any other text.",
		Messages: []providers.Message{
			{Role: "user", Content: "Run the Pulse Patrol tool-call self-test."},
		},
		Tools: []providers.Tool{
			{
				Name:        patrolPreflightToolName,
				Description: "Confirm Pulse Patrol can receive a tool call. Always pass ok=true.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"ok": map[string]interface{}{
							"type":        "boolean",
							"description": "Always pass true.",
						},
					},
					"required":             []string{"ok"},
					"additionalProperties": false,
				},
			},
		},
		ToolChoice: &providers.ToolChoice{Type: providers.ToolChoiceAny},
		MaxTokens:  256,
	}

	resp, err := provider.Chat(ctx, req)
	result.DurationMs = time.Since(started).Milliseconds()

	if err != nil {
		applyPatrolPreflightDiagnostic(&result, err)
		return result
	}

	result.Success = true
	result.ToolCallObserved = resp != nil && len(resp.ToolCalls) > 0
	if result.ToolCallObserved {
		result.Cause = PatrolFailureCauseNone
		result.Title = "Pulse Patrol: Preflight succeeded"
		result.Summary = "Provider accepted the preflight request and the model emitted a tool call."
		return result
	}

	// Soft warning: provider accepted the request shape (no error) but
	// the model returned plain text instead of calling the verify tool.
	// Patrol may still work in practice, but we flag this so the operator
	// can run a real Patrol pass to confirm before relying on it.
	result.Cause = PatrolFailureCauseModelToolSupportUnverified
	result.Title = "Pulse Patrol: Model did not emit a tool call during preflight"
	result.Summary = "Provider accepted the preflight request but the model did not emit a tool call. Patrol may still work in practice."
	result.Recommendation = "Trigger a real Patrol run to confirm tool calling. If that fails, switch to a model with stronger tool-following behaviour."
	return result
}

func applyPatrolPreflightDiagnostic(result *PatrolPreflightResult, err error) {
	failure := patrolRuntimeFailureFromError(err)
	result.Cause = failure.Cause
	result.Title = failure.Title
	result.Summary = failure.Summary
	result.Description = failure.Description
	result.Recommendation = failure.Recommendation
}
