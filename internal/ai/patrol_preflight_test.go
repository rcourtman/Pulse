package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func TestRunPatrolToolPreflight_RequestShapeIncludesVerifyTool(t *testing.T) {
	// Locks the preflight request shape so a future refactor can't
	// silently strip the tool definition or tool_choice (which would
	// turn preflight into a connection-test that misses the original
	// failure mode).
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
	if captured["tool_choice"] == nil {
		t.Fatalf("expected tool_choice to be set in preflight request, got %v", captured)
	}
}
