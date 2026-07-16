package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// patrolPreflightTestService spins up an in-memory OpenAI-compatible server
// and an AI Service configured to route through it via the OpenAI provider.
// Tests exercise the full RunPatrolToolPreflight path (factory ->
// provider.Chat -> classification) without mocking the provider layer.
type patrolPreflightTestService struct {
	svc    *Service
	server *httptest.Server
}

func (h *patrolPreflightTestService) close() {
	if h.server != nil {
		h.server.Close()
	}
}

func newPatrolPreflightTestService(t *testing.T, model string, handler http.HandlerFunc) *patrolPreflightTestService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	svc.cfg = &config.AIConfig{
		Enabled:       true,
		Model:         model,
		PatrolModel:   model,
		OpenAIAPIKey:  "sk-test",
		OpenAIBaseURL: server.URL,
	}
	return &patrolPreflightTestService{svc: svc, server: server}
}

func TestPatrolPreflightTimeoutIsRouteAware(t *testing.T) {
	tests := []struct {
		provider string
		want     time.Duration
	}{
		{provider: config.AIProviderOpenAI, want: 30 * time.Second},
		{provider: config.AIProviderAnthropic, want: 30 * time.Second},
		{provider: config.AIProviderCodexSubscription, want: 2 * time.Minute},
		{provider: config.AIProviderClaudeSubscription, want: 2 * time.Minute},
		{provider: "", want: 30 * time.Second},
		{provider: "future-provider", want: 30 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			if got := patrolPreflightTimeout(tt.provider); got != tt.want {
				t.Fatalf("patrolPreflightTimeout(%q) = %s, want %s", tt.provider, got, tt.want)
			}
		})
	}
}

func TestRunPatrolToolPreflightSubscriptionRoutePreservesCallerCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake subscription CLI uses a POSIX shell script")
	}
	binDir := t.TempDir()
	claudePath := filepath.Join(binDir, "claude")
	if err := os.WriteFile(claudePath, []byte(`#!/bin/sh
while :; do :; done
`), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	svc.cfg = &config.AIConfig{
		Enabled:                   true,
		Model:                     "claude-subscription:opus",
		PatrolModel:               "claude-subscription:opus",
		ClaudeSubscriptionEnabled: true,
		RequestTimeoutSeconds:     1,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	started := time.Now()
	result := svc.RunPatrolToolPreflight(ctx, "", "")
	if result.Success {
		t.Fatalf("expected caller cancellation to fail subscription preflight, got %+v", result)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("subscription caller cancellation took %s", elapsed)
	}
}

func TestRunPatrolToolPreflightPreservesCallerCancellation(t *testing.T) {
	h := newPatrolPreflightTestService(t, "openai:gpt-4o-mini", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"late","model":"gpt-4o-mini","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"late"}}]}`))
	})
	defer h.close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	result := h.svc.RunPatrolToolPreflight(ctx, "", "")
	if result.Success {
		t.Fatalf("expected caller cancellation to fail preflight, got %+v", result)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("caller cancellation took %s", elapsed)
	}
}

func TestRunPatrolToolPreflight_Success_ToolCallObserved(t *testing.T) {
	// Provider accepts the request and the model emits a tool call —
	// the green-path outcome we want operators to see.
	h := newPatrolPreflightTestService(t, "openai:gpt-4o-mini", func(w http.ResponseWriter, r *http.Request) {
		var got map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got["model"] != "gpt-4o-mini" {
			t.Fatalf("model = %v", got["model"])
		}
		if got["tools"] == nil {
			t.Fatalf("expected tools in request, got %v", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-preflight-ok",
			"model": "gpt-4o-mini",
			"choices": [{
				"finish_reason": "tool_calls",
				"message": {
					"role": "assistant",
					"content": "",
					"tool_calls": [{
						"id": "call_1",
						"type": "function",
						"function": {"name": "verify_pulse_patrol", "arguments": "{\"ok\":true}"}
					}]
				}
			}]
		}`))
	})
	defer h.close()

	result := h.svc.RunPatrolToolPreflight(context.Background(), "", "")
	if !result.Success {
		t.Fatalf("expected Success, got %+v", result)
	}
	if !result.ToolCallObserved {
		t.Fatalf("expected ToolCallObserved, got %+v", result)
	}
	if result.Cause != PatrolFailureCauseNone {
		t.Fatalf("unexpected cause %q", result.Cause)
	}
	if result.Provider != config.AIProviderOpenAI {
		t.Fatalf("unexpected provider %q", result.Provider)
	}
	if result.Model != "gpt-4o-mini" {
		t.Fatalf("unexpected model %q", result.Model)
	}
	if !strings.Contains(result.Title, "succeeded") {
		t.Fatalf("expected success title, got %q", result.Title)
	}
}

func TestRunPatrolToolPreflight_SuccessResolvesPreviousRuntimeFailure(t *testing.T) {
	h := newPatrolPreflightTestService(t, "openai:gpt-4o-mini", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-preflight-ok",
			"model": "gpt-4o-mini",
			"choices": [{
				"finish_reason": "tool_calls",
				"message": {
					"role": "assistant",
					"content": "",
					"tool_calls": [{
						"id": "call_1",
						"type": "function",
						"function": {"name": "verify_pulse_patrol", "arguments": "{\"ok\":true}"}
					}]
				}
			}]
		}`))
	})
	defer h.close()

	patrol := NewPatrolService(h.svc, nil)
	h.svc.patrolService = patrol
	runtimeFinding := newPatrolRuntimeFailureFinding(patrolRuntimeFailureFromError(errors.New("provider connection failed")), time.Now().Add(-time.Hour))
	patrol.recordFinding(runtimeFinding)

	result := h.svc.RunPatrolToolPreflight(context.Background(), "", "")
	if !result.Success || !result.ToolCallObserved {
		t.Fatalf("expected green preflight, got %+v", result)
	}

	finding := patrol.findings.Get(runtimeFinding.ID)
	if finding == nil {
		t.Fatal("expected runtime finding to remain available as resolved history")
	}
	if !finding.IsResolved() {
		t.Fatalf("expected successful preflight to auto-resolve runtime finding, got resolved_at=%v", finding.ResolvedAt)
	}
}

func TestRunPatrolToolPreflight_Success_NoToolCall(t *testing.T) {
	// Provider accepts the request but the model returned plain text
	// instead of calling the verify tool. Pulse soft-warns rather than
	// hard-failing because the model may still work in practice.
	h := newPatrolPreflightTestService(t, "openai:gpt-4o-mini", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-preflight-no-tool",
			"model": "gpt-4o-mini",
			"choices": [{
				"finish_reason": "stop",
				"message": {"role": "assistant", "content": "ok"}
			}]
		}`))
	})
	defer h.close()

	result := h.svc.RunPatrolToolPreflight(context.Background(), "", "")
	if !result.Success {
		t.Fatalf("expected Success (soft warning still counts as success), got %+v", result)
	}
	if result.ToolCallObserved {
		t.Fatalf("expected ToolCallObserved=false, got %+v", result)
	}
	if result.Cause != PatrolFailureCauseModelToolSupportUnverified {
		t.Fatalf("unexpected cause %q", result.Cause)
	}
	if !strings.Contains(result.Recommendation, "real Patrol") {
		t.Fatalf("expected recommendation to mention real Patrol run, got %q", result.Recommendation)
	}
}

func TestRunPatrolToolPreflight_NoToolCallKeepsPreviousRuntimeFailureActive(t *testing.T) {
	h := newPatrolPreflightTestService(t, "openai:gpt-4o-mini", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-preflight-no-tool",
			"model": "gpt-4o-mini",
			"choices": [{
				"finish_reason": "stop",
				"message": {"role": "assistant", "content": "ok"}
			}]
		}`))
	})
	defer h.close()

	patrol := NewPatrolService(h.svc, nil)
	h.svc.patrolService = patrol
	runtimeFinding := newPatrolRuntimeFailureFinding(patrolRuntimeFailureFromError(errors.New("provider connection failed")), time.Now().Add(-time.Hour))
	patrol.recordFinding(runtimeFinding)

	result := h.svc.RunPatrolToolPreflight(context.Background(), "", "")
	if !result.Success || result.ToolCallObserved {
		t.Fatalf("expected soft-warning preflight, got %+v", result)
	}

	finding := patrol.findings.Get(runtimeFinding.ID)
	if finding == nil {
		t.Fatal("expected runtime finding to remain available")
	}
	if finding.IsResolved() {
		t.Fatalf("expected no-tool-call preflight to leave runtime finding active, got resolved_at=%v", finding.ResolvedAt)
	}
}

func TestRunPatrolToolPreflight_ToolChoiceRejected(t *testing.T) {
	// Provider returns the DeepSeek-style 400 that motivated the
	// classifier split. Preflight must surface the more accurate
	// tool_choice_rejected cause, not the generic "model unsupported".
	h := newPatrolPreflightTestService(t, "openai:test-model", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"deepseek-reasoner does not support this tool_choice","type":"invalid_request_error"}}`))
	})
	defer h.close()

	result := h.svc.RunPatrolToolPreflight(context.Background(), "", "")
	if result.Success {
		t.Fatalf("expected failure, got %+v", result)
	}
	if result.Cause != PatrolFailureCauseToolChoiceRejected {
		t.Fatalf("unexpected cause %q (want %q)", result.Cause, PatrolFailureCauseToolChoiceRejected)
	}
	if !strings.Contains(result.Recommendation, "automatic tool selection") {
		t.Fatalf("expected recommendation to mention automatic tool selection, got %q", result.Recommendation)
	}
}

func TestRunPatrolToolPreflight_NoToolCapableEndpoint(t *testing.T) {
	// Routing failure — no available endpoint supports tools for the
	// requested model. OpenRouter is the canonical source of this
	// wording in production; the classifier pattern-matches on the
	// error string regardless of transport, so we use the OpenAI
	// provider as a stand-in (OpenRouter base URL isn't configurable).
	h := newPatrolPreflightTestService(t, "openai:test-model", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"No endpoints found that support the provided 'tool_choice' value."}}`))
	})
	defer h.close()

	result := h.svc.RunPatrolToolPreflight(context.Background(), "", "")
	if result.Success {
		t.Fatalf("expected failure, got %+v", result)
	}
	if result.Cause != PatrolFailureCauseNoToolCapableEndpoint {
		t.Fatalf("unexpected cause %q (want %q)", result.Cause, PatrolFailureCauseNoToolCapableEndpoint)
	}
	if !strings.Contains(result.Recommendation, "routing") && !strings.Contains(result.Recommendation, "filters") {
		t.Fatalf("expected recommendation to mention routing/filters, got %q", result.Recommendation)
	}
}

func TestRunPatrolToolPreflight_AssistantDisabled(t *testing.T) {
	svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	svc.cfg = &config.AIConfig{Enabled: false, PatrolModel: "openai:gpt-4o-mini"}

	result := svc.RunPatrolToolPreflight(context.Background(), "", "")
	if result.Success {
		t.Fatalf("expected failure when assistant disabled, got %+v", result)
	}
	if result.Cause != PatrolFailureCauseAssistantDisabled {
		t.Fatalf("unexpected cause %q", result.Cause)
	}
}

func TestRunPatrolToolPreflight_SettingsUnavailableUsesAssistantPatrolCopy(t *testing.T) {
	svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	svc.cfg = nil

	result := svc.RunPatrolToolPreflight(context.Background(), "", "")
	if result.Success {
		t.Fatalf("expected failure when settings are unavailable, got %+v", result)
	}
	if result.Cause != PatrolFailureCauseSettingsPersistence {
		t.Fatalf("unexpected cause %q", result.Cause)
	}
	if result.Title != "Pulse Patrol: Pulse Intelligence settings unavailable" {
		t.Fatalf("unexpected title %q", result.Title)
	}
	if result.Summary != "Pulse Intelligence settings could not be loaded" {
		t.Fatalf("unexpected summary %q", result.Summary)
	}
}

func TestRunPatrolToolPreflight_NoModelSelected(t *testing.T) {
	svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	svc.cfg = &config.AIConfig{Enabled: true, OpenAIAPIKey: "sk-test"}

	result := svc.RunPatrolToolPreflight(context.Background(), "", "")
	if result.Success {
		t.Fatalf("expected failure when no model selected, got %+v", result)
	}
	if result.Cause != PatrolFailureCauseModelNotSelected {
		t.Fatalf("unexpected cause %q", result.Cause)
	}
}

func TestRunPatrolToolPreflight_PopulatesCacheOnSuccessAndFailure(t *testing.T) {
	// Cache must reflect the most recent preflight outcome regardless of
	// whether it succeeded, soft-warned, or failed — that's what powers
	// the "last verified" indicator on the settings page.
	called := 0
	h := newPatrolPreflightTestService(t, "openai:gpt-4o-mini", func(w http.ResponseWriter, r *http.Request) {
		called++
		if called == 1 {
			// First call: green path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"x","model":"gpt-4o-mini","choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"1","type":"function","function":{"name":"verify_pulse_patrol","arguments":"{\"ok\":true}"}}]}}]}`))
			return
		}
		// Second call: provider error
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"deepseek-reasoner does not support this tool_choice"}}`))
	})
	defer h.close()

	// Cache is empty before any preflight runs.
	cached, recordedAt := h.svc.CachedPatrolPreflight()
	if cached != nil {
		t.Fatalf("expected empty cache before first preflight, got %+v", cached)
	}
	if !recordedAt.IsZero() {
		t.Fatalf("expected zero recorded time before first preflight, got %v", recordedAt)
	}

	// First run records the success path.
	first := h.svc.RunPatrolToolPreflight(context.Background(), "", "")
	if !first.Success {
		t.Fatalf("expected first preflight to succeed, got %+v", first)
	}
	cached, recordedAt = h.svc.CachedPatrolPreflight()
	if cached == nil || !cached.Success || !cached.ToolCallObserved {
		t.Fatalf("cache did not capture success path, got %+v", cached)
	}
	if recordedAt.IsZero() {
		t.Fatalf("expected non-zero recorded time after first preflight")
	}
	firstRecorded := recordedAt

	// Second run records the failure path, superseding the first cache entry.
	second := h.svc.RunPatrolToolPreflight(context.Background(), "", "")
	if second.Success {
		t.Fatalf("expected second preflight to fail, got %+v", second)
	}
	cached, recordedAt = h.svc.CachedPatrolPreflight()
	if cached == nil || cached.Success {
		t.Fatalf("cache did not capture failure path, got %+v", cached)
	}
	if cached.Cause != PatrolFailureCauseToolChoiceRejected {
		t.Fatalf("cache did not preserve classified cause, got %q", cached.Cause)
	}
	if !recordedAt.After(firstRecorded) {
		t.Fatalf("expected recorded time to advance, got %v -> %v", firstRecorded, recordedAt)
	}
}

func TestCachedPatrolPreflight_ReturnsDefensiveCopy(t *testing.T) {
	// Mutating the returned result must not corrupt the cache.
	h := newPatrolPreflightTestService(t, "openai:gpt-4o-mini", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","model":"gpt-4o-mini","choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"1","type":"function","function":{"name":"verify_pulse_patrol","arguments":"{\"ok\":true}"}}]}}]}`))
	})
	defer h.close()

	_ = h.svc.RunPatrolToolPreflight(context.Background(), "", "")
	cached, _ := h.svc.CachedPatrolPreflight()
	if cached == nil {
		t.Fatalf("expected cached result")
	}
	cached.Success = false
	cached.Title = "tampered"

	// Re-read should still show the original successful result.
	again, _ := h.svc.CachedPatrolPreflight()
	if !again.Success || again.Title == "tampered" {
		t.Fatalf("cache returned mutable reference: original=%+v, re-read=%+v", cached, again)
	}
}

func TestTriggerPatrolPreflightAsync_PopulatesCacheInBackground(t *testing.T) {
	// The settings save handler dispatches preflight via this entrypoint
	// so the save response isn't blocked on a 5-10s LLM round-trip.
	// Verify the goroutine actually populates the cache and the caller
	// returns immediately.
	called := make(chan struct{}, 1)
	h := newPatrolPreflightTestService(t, "openai:gpt-4o-mini", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","model":"gpt-4o-mini","choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"1","type":"function","function":{"name":"verify_pulse_patrol","arguments":"{\"ok\":true}"}}]}}]}`))
		select {
		case called <- struct{}{}:
		default:
		}
	})
	defer h.close()

	h.svc.TriggerPatrolPreflightAsync("", "")

	// Caller does not block — wait for the background goroutine to hit
	// the test server.
	select {
	case <-called:
	case <-time.After(5 * time.Second):
		t.Fatalf("async preflight did not invoke provider within 5s")
	}

	// Cache must populate eventually after the goroutine finishes.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		cached, _ := h.svc.CachedPatrolPreflight()
		if cached != nil && cached.Success && cached.ToolCallObserved {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	cached, _ := h.svc.CachedPatrolPreflight()
	t.Fatalf("expected async preflight to populate cache with green result, got %+v", cached)
}

func TestRunPatrolToolPreflight_RequestShapeIncludesVerifyTool(t *testing.T) {
	// Locks the preflight request shape so a future refactor can't silently
	// strip the tool definition, while keeping tool selection model-owned.
	var captured map[string]interface{}
	h := newPatrolPreflightTestService(t, "openai:gpt-4o-mini", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","model":"gpt-4o-mini","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`))
	})
	defer h.close()

	_ = h.svc.RunPatrolToolPreflight(context.Background(), "", "")

	tools, ok := captured["tools"].([]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("expected exactly one tool in preflight request, got %v", captured["tools"])
	}
	tool := tools[0].(map[string]interface{})
	fn := tool["function"].(map[string]interface{})
	if fn["name"] != patrolPreflightToolName {
		t.Fatalf("expected tool name %q, got %v", patrolPreflightToolName, fn["name"])
	}
	if captured["tool_choice"] != nil {
		t.Fatalf("expected preflight to keep tool selection automatic, got %v", captured["tool_choice"])
	}
}
