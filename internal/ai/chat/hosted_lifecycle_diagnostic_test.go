package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

const diagnosticModel = "google/gemini-2.5-flash"
const diagnosticPrompt = "Find the five VMs named win-01 through win-05 and prepare their reboot plans. Do not execute any action."

// A hard logical-call boundary also counts final-response/recovery calls, which
// may occur outside the loop's ordinary turn budget. The wrapped production
// adapter retains its own transport retry/fallback behavior. No forced tool
// selection.
type lifecycleDiagnosticProvider struct {
	providers.StreamingProvider
	requests []providers.ChatRequest
	events   []providers.StreamEvent
}

func (p *lifecycleDiagnosticProvider) Chat(context.Context, providers.ChatRequest) (*providers.ChatResponse, error) {
	return nil, fmt.Errorf("diagnostic disallows non-streaming provider calls")
}

func (p *lifecycleDiagnosticProvider) ChatStream(ctx context.Context, req providers.ChatRequest, cb providers.StreamCallback) error {
	if len(p.requests) >= 8 {
		return fmt.Errorf("diagnostic eight-call limit reached")
	}
	req.Model = diagnosticModel
	req.MaxTokens = 2048
	// Bound the provider-neutral request before the production adapter projects
	// it to its wire format. This is not a wire-byte or billing-attempt counter.
	body, err := json.Marshal(req)
	if err != nil || len(body) > 128*1024 {
		return fmt.Errorf("diagnostic input size limit")
	}
	if req.ToolChoice != nil && req.ToolChoice.Type == providers.ToolChoiceRequired {
		return fmt.Errorf("diagnostic disallows forced tool selection")
	}
	p.requests = append(p.requests, req)
	return p.StreamingProvider.ChatStream(ctx, req, func(event providers.StreamEvent) {
		p.events = append(p.events, event)
		cb(event)
	})
}

// Only fixed synthetic reboot plans can be recorded. No executor is installed,
// no approval is granted, and even an arbitrary model call cannot reach a host.
type lifecycleDiagnosticPlanner struct{ gateTestPlanner }

func (p *lifecycleDiagnosticPlanner) PlanTypedAction(ctx context.Context, org string, req unifiedresources.ActionRequest) (*unifiedresources.ActionPlan, error) {
	valid := false
	for i := 1; i <= 5; i++ {
		if req.ResourceID == fmt.Sprintf("vm-pve-win-%02d", i) {
			valid = true
		}
	}
	if !valid || req.CapabilityName != "reboot" {
		return nil, fmt.Errorf("synthetic diagnostic accepts only fixed VM reboot plans")
	}
	return p.gateTestPlanner.PlanTypedAction(ctx, org, req)
}

type lifecycleDiagnosticResult struct {
	Events   []providers.StreamEvent          `json:"events"`
	Requests []providers.ChatRequest          `json:"requests"`
	Messages []Message                        `json:"messages"`
	Plans    []unifiedresources.ActionRequest `json:"plans"`
	Error    string                           `json:"error,omitempty"`
}

func runLifecycleDiagnostic(t *testing.T, backend providers.StreamingProvider, session string) lifecycleDiagnosticResult {
	t.Helper()
	vms := make([]unifiedresources.Resource, 0, 5)
	for i := 1; i <= 5; i++ {
		vms = append(vms, gateTestProxmoxVM(fmt.Sprintf("win-%02d", i), 100+i))
	}
	planner := &lifecycleDiagnosticPlanner{}
	// Every data source is synthetic or nil. In particular AgentServer,
	// discovery, command, action-execution and persistent stores are absent.
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: &mockStateProvider{}, UnifiedResourceProvider: plainTextResourceTestProvider(vms...),
		TypedActionPlanner: planner, ControlLevel: tools.ControlLevelControlled,
	})
	executor.SetResolvedContext(NewResolvedContext(session))
	service := &Service{executor: executor}
	provider := &lifecycleDiagnosticProvider{StreamingProvider: backend}
	loop := NewAgenticLoop(provider, executor, service.buildSystemPrompt())
	loop.SetMaxTurns(8)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	messages, err := loop.ExecuteWithTools(ctx, session, []Message{{Role: "user", Content: diagnosticPrompt}}, nil, func(StreamEvent) {})
	result := lifecycleDiagnosticResult{Events: provider.events, Requests: provider.requests, Messages: messages, Plans: planner.snapshot()}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

// Explicit opt-in only. Run this exact entry point in a secret-contained private
// workflow after review; normal tests never read credentials or call a provider.
func TestHostedLifecycleDiagnostic(t *testing.T) {
	if os.Getenv("PULSE_HOSTED_LIFECYCLE_DIAGNOSTIC") != "1" {
		t.Skip("requires reviewed hosted diagnostic workflow")
	}
	key := os.Getenv("OPENROUTER_API_KEY")
	require.NotEmpty(t, key, "project OpenRouter credential required")
	for i := 1; i <= 3; i++ {
		t.Run(fmt.Sprintf("session_%d", i), func(t *testing.T) {
			backend := providers.NewOpenAICompatibleClient("openrouter", key, diagnosticModel, "https://openrouter.ai/api/v1", 120*time.Second)
			result := runLifecycleDiagnostic(t, backend, fmt.Sprintf("synthetic-%d", i))
			// Requests contain full rendered production instructions and tool results,
			// never HTTP auth headers. Redact the credential even from provider errors.
			body, err := json.Marshal(result)
			require.NoError(t, err)
			t.Log(strings.ReplaceAll(string(body), key, "[REDACTED]"))
			if result.Error != "" {
				t.Errorf("hosted session failed; see sanitised diagnostic record")
			}
			approvals := map[string]bool{}
			for _, message := range result.Messages {
				if message.ToolResult == nil || message.ToolResult.IsError {
					continue
				}
				var payload struct {
					ResourceID       string `json:"resource_id"`
					Planned          bool   `json:"planned"`
					RequiresApproval bool   `json:"requires_approval"`
				}
				if json.Unmarshal([]byte(message.ToolResult.Content), &payload) == nil && payload.Planned && payload.RequiresApproval {
					approvals[payload.ResourceID] = true
				}
			}
			for i := 1; i <= 5; i++ {
				if !approvals[fmt.Sprintf("vm-pve-win-%02d", i)] {
					t.Errorf("missing approval-required receipt for synthetic target %d; not acceptance", i)
				}
			}
		})
	}
}

func TestLifecycleDiagnosticFixture(t *testing.T) {
	for _, submit := range []bool{true, false} {
		t.Run(fmt.Sprintf("submit_%t", submit), func(t *testing.T) {
			turn := 0
			backend := &stubStreamingProvider{}
			backend.chatStream = func(_ context.Context, req providers.ChatRequest, cb providers.StreamCallback) error {
				turn++
				require.Equal(t, 2048, req.MaxTokens)
				names := map[string]bool{}
				for _, tool := range req.Tools {
					names[tool.Name] = true
				}
				require.True(t, names["pulse_control"], "scripted calls must be offered to a real provider")
				require.Contains(t, req.System, "pulse_control")
				require.Equal(t, diagnosticModel, req.Model)
				require.Contains(t, req.System, "first-party in-app Pulse Intelligence")
				require.Contains(t, req.System, "QEMU guest agent")
				calls := []providers.ToolCall{}
				if turn == 1 {
					calls = append(calls, providers.ToolCall{ID: "query", Name: "pulse_query", Input: map[string]interface{}{"action": "search", "query": "win", "type": "vm"}})
				}
				if turn == 2 && submit {
					vms := []unifiedresources.Resource{}
					for i := 1; i <= 5; i++ {
						vms = append(vms, gateTestProxmoxVM(fmt.Sprintf("win-%02d", i), 100+i))
					}
					calls = gateTestControlCalls(vms, "reboot")
				}
				if len(calls) == 0 {
					cb(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "No action executed."}})
				}
				cb(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{ToolCalls: calls}})
				return nil
			}
			result := runLifecycleDiagnostic(t, backend, "local-synthetic")
			require.Empty(t, result.Error)
			expected := 0
			if submit {
				expected = 5
			}
			require.Len(t, result.Plans, expected)
			approvals := 0
			for _, message := range result.Messages {
				if message.ToolResult != nil && strings.HasPrefix(message.ToolResult.ToolUseID, "c-") {
					require.False(t, message.ToolResult.IsError, message.ToolResult.Content)
					require.Contains(t, message.ToolResult.Content, `"requires_approval":true`)
					approvals++
				}
			}
			require.Equal(t, expected, approvals)
		})
	}
}

func TestLifecycleDiagnosticProviderLimits(t *testing.T) {
	backend := &stubStreamingProvider{}
	calls := 0
	backend.chatStream = func(_ context.Context, req providers.ChatRequest, _ providers.StreamCallback) error {
		calls++
		require.Equal(t, 2048, req.MaxTokens)
		return nil
	}
	p := &lifecycleDiagnosticProvider{StreamingProvider: backend}
	_, err := p.Chat(context.Background(), providers.ChatRequest{})
	require.Error(t, err)
	require.Error(t, p.ChatStream(context.Background(), providers.ChatRequest{System: strings.Repeat("x", 128*1024)}, nil))
	require.Error(t, p.ChatStream(context.Background(), providers.ChatRequest{ToolChoice: &providers.ToolChoice{Type: providers.ToolChoiceRequired}}, nil))
	for i := 0; i < 8; i++ {
		require.NoError(t, p.ChatStream(context.Background(), providers.ChatRequest{}, nil))
	}
	require.Error(t, p.ChatStream(context.Background(), providers.ChatRequest{}, nil))
	require.Equal(t, 8, calls)
}

func TestLifecycleDiagnosticPlannerRejectsOtherActions(t *testing.T) {
	p := &lifecycleDiagnosticPlanner{}
	for _, req := range []unifiedresources.ActionRequest{
		{ResourceID: "vm-pve-win-01", CapabilityName: "stop"},
		{ResourceID: "external", CapabilityName: "reboot"},
	} {
		_, err := p.PlanTypedAction(context.Background(), "", req)
		require.Error(t, err)
	}
	require.Empty(t, p.snapshot())
}

func TestLifecycleDiagnosticGatewayWire(t *testing.T) {
	var mu sync.Mutex
	turn := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if req["model"] != diagnosticModel {
			t.Error("wrong model")
		}
		if req["max_tokens"] != float64(2048) && req["max_completion_tokens"] != float64(2048) {
			t.Error("missing token cap")
		}
		offered := map[string]bool{}
		for _, raw := range req["tools"].([]any) {
			tool := raw.(map[string]any)["function"].(map[string]any)
			offered[tool["name"].(string)] = true
		}
		if !offered["pulse_control"] || !offered["pulse_query"] {
			t.Error("gateway must receive query and plan tools")
		}
		mu.Lock()
		turn++
		n := turn
		mu.Unlock()
		calls := []providers.ToolCall{}
		if n == 1 {
			calls = append(calls, providers.ToolCall{ID: "query", Name: "pulse_query", Input: map[string]interface{}{"action": "search", "query": "win", "type": "vm"}})
		}
		if n == 2 {
			vms := []unifiedresources.Resource{}
			for i := 1; i <= 5; i++ {
				vms = append(vms, gateTestProxmoxVM(fmt.Sprintf("win-%02d", i), 100+i))
			}
			calls = gateTestControlCalls(vms, "reboot")
		}
		delta := map[string]any{"content": "No action executed."}
		finish := "stop"
		if len(calls) > 0 {
			wire := []any{}
			for i, call := range calls {
				args, _ := json.Marshal(call.Input)
				wire = append(wire, map[string]any{"index": i, "id": call.ID, "type": "function", "function": map[string]any{"name": call.Name, "arguments": string(args)}})
			}
			delta = map[string]any{"tool_calls": wire}
			finish = "tool_calls"
		}
		payload, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": delta}}})
		end, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{}, "finish_reason": finish}}})
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\ndata: %s\n\ndata: [DONE]\n\n", payload, end)
	}))
	defer server.Close()
	backend := providers.NewOpenAICompatibleClient("openrouter", "synthetic-key", diagnosticModel, server.URL, 120*time.Second)
	result := runLifecycleDiagnostic(t, backend, "synthetic-wire")
	require.Empty(t, result.Error)
	require.Len(t, result.Requests, 3)
	require.Len(t, result.Plans, 5)
	require.NotEmpty(t, result.Events)
	for _, message := range result.Messages {
		if message.ToolResult != nil {
			require.False(t, message.ToolResult.IsError, message.ToolResult.Content)
		}
	}
}

// The hosted model's wildcard queries are not interpreted as globs. A literal
// broader query can discover the same five targets; this is not model recovery
// acceptance and must not silently broaden a lifecycle action's target set.
func TestLifecycleDiagnosticLiteralSearch(t *testing.T) {
	vms := make([]unifiedresources.Resource, 0, 5)
	for i := 1; i <= 5; i++ {
		vms = append(vms, gateTestProxmoxVM(fmt.Sprintf("win-%02d", i), 100+i))
	}
	e := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: &mockStateProvider{}, UnifiedResourceProvider: plainTextResourceTestProvider(vms...),
	})
	for _, tc := range []struct {
		query string
		total int
	}{
		{"win-0?", 0}, {"win-0*", 0}, {"win-0", 5}, {"win-01", 1},
	} {
		t.Run(tc.query, func(t *testing.T) {
			result, err := e.ExecuteTool(context.Background(), "pulse_query", map[string]interface{}{"action": "search", "query": tc.query, "type": "vm", "limit": 5})
			require.NoError(t, err)
			require.False(t, result.IsError, result.Content)
			var payload struct {
				Total int `json:"total"`
			}
			require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &payload))
			require.Equal(t, tc.total, payload.Total)
		})
	}
}
