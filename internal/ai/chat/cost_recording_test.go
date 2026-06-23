package chat

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
)

// TestRecordChatTurnCost_RecordsWhenStoreConfigured verifies the
// canonical cost-recording site for user chat turns: when the
// service has a cost store and the loop accumulated tokens, a
// cost.UsageEvent lands in the store with UseCase="chat".
//
// This is the chat-side anchor for the same "every Chat caller
// records cost" invariant the internal/ai audit test enforces.
// The chat package can't use the function-local audit because
// recording is delegated from the loop (which calls ChatStream) to
// the orchestrator (which holds the cost store) — see
// recordChatTurnCost in service.go. So we test the orchestrator's
// recording site directly.
func TestRecordChatTurnCost_RecordsWhenStoreConfigured(t *testing.T) {
	store := cost.NewStore(7)
	svc := &Service{costStore: store}
	loop := &AgenticLoop{
		totalInputTokens:  120,
		totalOutputTokens: 45,
		totalToolCalls:    3,
	}

	svc.recordChatTurnCost(loop, "anthropic:claude-test", sessionHandoffKindResourceContext)

	events := store.ListEvents(1)
	if len(events) != 1 {
		t.Fatalf("expected 1 cost event, got %d", len(events))
	}
	ev := events[0]
	if ev.UseCase != "chat" {
		t.Errorf("UseCase = %q, want chat", ev.UseCase)
	}
	if ev.ContextScope != sessionHandoffKindResourceContext {
		t.Errorf("ContextScope = %q, want %q", ev.ContextScope, sessionHandoffKindResourceContext)
	}
	if ev.ToolCallCount != 3 {
		t.Errorf("ToolCallCount = %d, want 3", ev.ToolCallCount)
	}
	if ev.InputTokens != 120 || ev.OutputTokens != 45 {
		t.Errorf("tokens = (%d, %d), want (120, 45)", ev.InputTokens, ev.OutputTokens)
	}
	if ev.RequestModel != "anthropic:claude-test" {
		t.Errorf("RequestModel = %q, want anthropic:claude-test", ev.RequestModel)
	}
	if ev.Provider != "anthropic" {
		t.Errorf("Provider = %q, want anthropic", ev.Provider)
	}
}

// TestRecordChatTurnCost_NoopWhenStoreNil verifies the recorder is
// a no-op when the cost store is not configured. Self-hosted users
// without cost tracking enabled should not see panics or errors.
func TestRecordChatTurnCost_NoopWhenStoreNil(t *testing.T) {
	svc := &Service{costStore: nil}
	loop := &AgenticLoop{
		totalInputTokens:  100,
		totalOutputTokens: 50,
	}
	// Must not panic.
	svc.recordChatTurnCost(loop, "anthropic:claude-test", "")
}

// TestRecordChatTurnCost_NoopWhenZeroTokens verifies the recorder
// skips when the loop terminated before producing any tokens (e.g.
// budget gate rejected the request before the first turn). Zero-
// token events would pollute the dashboard.
func TestRecordChatTurnCost_NoopWhenZeroTokens(t *testing.T) {
	store := cost.NewStore(7)
	svc := &Service{costStore: store}
	loop := &AgenticLoop{
		totalInputTokens:  0,
		totalOutputTokens: 0,
	}
	svc.recordChatTurnCost(loop, "anthropic:claude-test", "")

	if events := store.ListEvents(1); len(events) != 0 {
		t.Errorf("expected 0 events for zero-token loop, got %d", len(events))
	}
}

// TestRecordChatTurnCost_HandlesMalformedModel verifies provider
// extraction tolerates model strings without a provider prefix
// (defensive — the canonical pipeline always sends provider:model
// but a future caller might not).
func TestRecordChatTurnCost_HandlesMalformedModel(t *testing.T) {
	store := cost.NewStore(7)
	svc := &Service{costStore: store}
	loop := &AgenticLoop{totalInputTokens: 10, totalOutputTokens: 5}

	svc.recordChatTurnCost(loop, "bare-model-name", "")

	events := store.ListEvents(1)
	if len(events) != 1 {
		t.Fatalf("expected 1 event even for malformed model, got %d", len(events))
	}
	if events[0].Provider != "" {
		t.Errorf("Provider should be empty for malformed model, got %q", events[0].Provider)
	}
	if events[0].RequestModel != "bare-model-name" {
		t.Errorf("RequestModel should be passed through verbatim, got %q", events[0].RequestModel)
	}
}

func TestAssistantContextScopeForChatTurn_ClassifiesGovernedContext(t *testing.T) {
	tests := []struct {
		name          string
		req           ExecuteRequest
		findingID     string
		context       string
		resources     []HandoffResource
		actions       []HandoffAction
		metadata      HandoffMetadata
		mentionsFound bool
		want          string
	}{
		{
			name:      "finding handoff",
			findingID: "finding-123",
			want:      sessionHandoffKindPatrolFinding,
		},
		{
			name: "governed action",
			actions: []HandoffAction{{
				ActionID: "action-123",
			}},
			want: "governed_action",
		},
		{
			name: "patrol run metadata",
			metadata: HandoffMetadata{
				Kind:  sessionHandoffKindPatrolRun,
				RunID: "run-123",
			},
			want: sessionHandoffKindPatrolRun,
		},
		{
			name: "resource handoff",
			resources: []HandoffResource{{
				ID:   "vm:node:100",
				Type: "vm",
			}},
			want: sessionHandoffKindResourceContext,
		},
		{
			name: "structured mention",
			req: ExecuteRequest{Mentions: []StructuredMention{{
				ID:   "vm:node:100",
				Name: "web-01",
				Type: "vm",
			}}},
			want: sessionHandoffKindResourceContext,
		},
		{
			name:    "scoped context fallback",
			context: "[Context]\nOperator-selected state",
			want:    sessionHandoffKindScopedContext,
		},
		{
			name: "plain chat",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := assistantContextScopeForChatTurn(
				tt.req,
				tt.findingID,
				tt.context,
				tt.resources,
				tt.actions,
				tt.metadata,
				tt.mentionsFound,
			)
			if got != tt.want {
				t.Fatalf("scope = %q, want %q", got, tt.want)
			}
		})
	}
}
