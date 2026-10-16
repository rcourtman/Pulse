package chat

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

func TestInvestigationFinalResponseFailureRetainsReceiptAndError(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"empty", nil}, {"timeout", context.DeadlineExceeded}, {"provider", errors.New("provider unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			provider := &stubStreamingProvider{chatStream: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
				calls++
				if calls == 1 {
					callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{StopReason: "tool_use"}})
					return nil
				}
				if len(req.Tools) != 0 {
					t.Fatal("summary offered write tools")
				}
				return tc.err
			}}
			loop := NewAgenticLoop(provider, tools.NewPulseToolExecutor(tools.ExecutorConfig{}), "Investigate current evidence")
			loop.SetExecutionProfile(tools.ProfilePatrolInvestigation)
			messages, err := loop.ExecuteWithTools(context.Background(), "incomplete", []Message{{Role: "user", Content: "Check service"}}, nil, func(StreamEvent) {})
			if err == nil {
				t.Fatal("incomplete provider run was reported as successful")
			}
			if tc.err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("lost error identity: %v", err)
			}
			if calls != 2 {
				t.Fatalf("provider calls=%d, want original plus bounded summary", calls)
			}
			if len(messages) == 0 || !strings.Contains(messages[len(messages)-1].Content, "final narrative") {
				t.Fatalf("missing retained fallback receipt: %#v", messages)
			}
		})
	}
}

func TestInvestigationEmptyResponseCanRecoverWithRealModelConclusion(t *testing.T) {
	calls := 0
	p := &stubStreamingProvider{chatStream: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		calls++
		if calls == 2 {
			callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "### Conclusion\nNEEDS_ATTENTION: upstream owner must repair the dependency"}})
		}
		callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		return nil
	}}
	loop := NewAgenticLoop(p, tools.NewPulseToolExecutor(tools.ExecutorConfig{}), "Investigate")
	loop.SetExecutionProfile(tools.ProfilePatrolInvestigation)
	messages, err := loop.ExecuteWithTools(context.Background(), "conclusion", []Message{{Role: "user", Content: "Check"}}, nil, func(StreamEvent) {})
	if err != nil || !strings.Contains(messages[len(messages)-1].Content, "NEEDS_ATTENTION:") {
		t.Fatalf("real model conclusion was discarded: %v %#v", err, messages)
	}
}

func TestInvestigationSummaryRetainsCallerDeadline(t *testing.T) {
	calls := 0
	p := &stubStreamingProvider{chatStream: func(ctx context.Context, req providers.ChatRequest, cb providers.StreamCallback) error {
		calls++
		if calls == 1 {
			cb(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
			return nil
		}
		<-ctx.Done()
		return ctx.Err()
	}}
	loop := NewAgenticLoop(p, tools.NewPulseToolExecutor(tools.ExecutorConfig{}), "Investigate")
	loop.SetExecutionProfile(tools.ProfilePatrolInvestigation)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := loop.ExecuteWithTools(ctx, "deadline", []Message{{Role: "user", Content: "Check"}}, nil, func(StreamEvent) {})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline identity lost: %v", err)
	}
}

func TestInvestigationWhitespaceSummaryIsNotAConclusion(t *testing.T) {
	p := &stubStreamingProvider{chatStream: func(ctx context.Context, req providers.ChatRequest, cb providers.StreamCallback) error {
		cb(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "  \n "}})
		cb(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		return nil
	}}
	loop := NewAgenticLoop(p, tools.NewPulseToolExecutor(tools.ExecutorConfig{}), "Investigate")
	loop.SetExecutionProfile(tools.ProfilePatrolInvestigation)
	_, err := loop.ExecuteWithTools(context.Background(), "whitespace", []Message{{Role: "user", Content: "Check"}}, nil, func(StreamEvent) {})
	if err == nil {
		t.Fatal("whitespace summary became a completed investigation")
	}
}
