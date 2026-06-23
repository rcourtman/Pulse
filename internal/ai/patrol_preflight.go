package ai

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// PatrolPreflightResult captures the outcome of a one-shot tool-call
// preflight against the configured (or overridden) Patrol provider+model.
//
// Unlike a connection test, which only lists models, the preflight
// exercises the full chat-completions path with a minimal tool
// definition. This surfaces real failure modes — no tool-capable endpoint
// available, model genuinely lacking tool support, or the model declining
// the tool — at configuration time instead of waiting for the next
// scheduled Patrol run to silently fail.
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

// patrolPreflightCache holds the most recent PatrolPreflightResult plus
// the wall-clock time it was recorded. Surfaced through the AI settings
// response so the UI can render a "last verified" indicator without
// requiring operators to re-run preflight on every page load.
type patrolPreflightCache struct {
	mu         sync.RWMutex
	result     *PatrolPreflightResult
	recordedAt time.Time
}

// CachedPatrolPreflight returns the most recent preflight result and the
// time it was recorded, or nil + zero time if preflight has never run on
// this Service instance.
func (s *Service) CachedPatrolPreflight() (*PatrolPreflightResult, time.Time) {
	s.patrolPreflightCache.mu.RLock()
	defer s.patrolPreflightCache.mu.RUnlock()
	if s.patrolPreflightCache.result == nil {
		return nil, time.Time{}
	}
	// Return a defensive copy so callers can't mutate the cache.
	clone := *s.patrolPreflightCache.result
	return &clone, s.patrolPreflightCache.recordedAt
}

// recordPatrolPreflight stores the result in the cache. Called after
// every RunPatrolToolPreflight invocation so the most recent outcome is
// always observable, including failures.
func (s *Service) recordPatrolPreflight(result PatrolPreflightResult, at time.Time) {
	s.patrolPreflightCache.mu.Lock()
	defer s.patrolPreflightCache.mu.Unlock()
	clone := result
	s.patrolPreflightCache.result = &clone
	s.patrolPreflightCache.recordedAt = at
}

// TriggerPatrolPreflightAsync runs RunPatrolToolPreflight in the
// background so callers (notably the settings save handler) don't block
// on a 5-10 second LLM round-trip. The result populates the preflight
// cache and the next /api/settings/ai poll surfaces it through the
// patrol_preflight snapshot. The 30s timeout matches the manual
// preflight handler so a stuck provider can't pin the goroutine
// indefinitely.
func (s *Service) TriggerPatrolPreflightAsync(provider, model string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.RunPatrolToolPreflight(ctx, provider, model)
	}()
}

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
//
// cost-recording-exempt: connectivity/tool-call self-test, not user
// workload. Operator-triggered preflight is observability-shaped, not
// billed-feature-shaped, and should not pollute the AI usage dashboard.
// This is the only intentional exception to the cost-recording audit.
func (s *Service) RunPatrolToolPreflight(ctx context.Context, providerName, model string) PatrolPreflightResult {
	started := time.Now()

	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()

	result := PatrolPreflightResult{}

	if cfg == nil {
		result.Cause = PatrolFailureCauseSettingsPersistence
		result.Title = "Pulse Patrol: Assistant & Patrol settings unavailable"
		result.Summary = "Assistant & Patrol settings could not be loaded"
		result.Recommendation = "Confirm Pulse settings persistence is healthy, then re-run preflight."
		result.DurationMs = time.Since(started).Milliseconds()
		s.recordPatrolPreflight(result, time.Now())
		return result
	}
	if !cfg.Enabled {
		result.Cause = PatrolFailureCauseAssistantDisabled
		result.Title = "Pulse Patrol: Assistant disabled"
		result.Summary = "Pulse Assistant is not enabled"
		result.Recommendation = "Enable Pulse Assistant in Assistant & Patrol settings, then re-run preflight."
		result.DurationMs = time.Since(started).Milliseconds()
		s.recordPatrolPreflight(result, time.Now())
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
		s.recordPatrolPreflight(result, time.Now())
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
		s.recordPatrolPreflight(result, time.Now())
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
		MaxTokens: 256,
	}

	resp, err := provider.Chat(ctx, req)
	result.DurationMs = time.Since(started).Milliseconds()

	if err != nil {
		applyPatrolPreflightDiagnostic(&result, err)
		s.recordPatrolPreflight(result, time.Now())
		return result
	}

	result.Success = true
	result.ToolCallObserved = resp != nil && len(resp.ToolCalls) > 0
	if result.ToolCallObserved {
		result.Cause = PatrolFailureCauseNone
		result.Title = "Pulse Patrol: Preflight succeeded"
		result.Summary = "Provider accepted the preflight request and the model emitted a tool call."
		s.recordPatrolPreflight(result, time.Now())
		s.resolvePatrolRuntimeFailureAfterToolPreflight(result)
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
	s.recordPatrolPreflight(result, time.Now())
	return result
}

func (s *Service) resolvePatrolRuntimeFailureAfterToolPreflight(result PatrolPreflightResult) bool {
	if s == nil || !result.Success || !result.ToolCallObserved {
		return false
	}
	patrol := s.GetPatrolService()
	if patrol == nil {
		return false
	}
	return patrol.resolvePatrolRuntimeFailureFinding("patrol_preflight_success")
}

func applyPatrolPreflightDiagnostic(result *PatrolPreflightResult, err error) {
	failure := patrolRuntimeFailureFromError(err)
	result.Cause = failure.Cause
	result.Title = failure.Title
	result.Summary = failure.Summary
	result.Description = failure.Description
	result.Recommendation = failure.Recommendation
}
