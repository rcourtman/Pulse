package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

const mainLoopLeakedToolCall = `pulse_read(target_host="current_resource", command="df -h")`

// mainLoopLeakProvider makes the first (tool-enabled) turn stream a clean
// prefix followed by a leaked textual tool call, with no structured tool
// calls. Any later turn streams summary, if set.
func mainLoopLeakProvider(prefix, summary string, calls *int) *stubStreamingProvider {
	return &stubStreamingProvider{chatStream: func(ctx context.Context, req providers.ChatRequest, cb providers.StreamCallback) error {
		*calls++
		if *calls == 1 {
			cb(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: prefix}})
			cb(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: mainLoopLeakedToolCall}})
		} else if summary != "" {
			cb(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: summary}})
		}
		cb(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		return nil
	}}
}

func TestInteractiveMainLoopKeepsCleanPrefixBeforeLeakedToolCall(t *testing.T) {
	prefix := "The root filesystem on pve1 is 92% full, driven by /var/log. "
	calls := 0
	loop := NewAgenticLoop(mainLoopLeakProvider(prefix, "unexpected summary", &calls), tools.NewPulseToolExecutor(tools.ExecutorConfig{}), "Answer the user")
	var events []StreamEvent
	messages, err := loop.ExecuteWithTools(context.Background(), "main-leak-chat", []Message{{Role: "user", Content: "Why is pve1 alerting?"}}, nil, func(event StreamEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatalf("interactive chat failed on a usable narrative: %v", err)
	}
	if calls != 1 {
		t.Fatalf("provider calls=%d, want the streamed answer without a summary turn", calls)
	}
	if len(messages) == 0 {
		t.Fatal("no messages returned")
	}
	final := messages[len(messages)-1].Content
	if strings.TrimSpace(final) != strings.TrimSpace(prefix) {
		t.Fatalf("persisted final message = %q, want the clean streamed prefix %q", final, prefix)
	}
	if streamed := streamedContent(t, events); streamed != final {
		t.Fatalf("streamed content %q differs from persisted message %q", streamed, final)
	}
}

func TestInvestigationMainLoopLeakedToolCallIsNotAConclusion(t *testing.T) {
	// The orchestrator classifies unmarked prose as NEEDS_ATTENTION, which is
	// terminal, so a "next I will check" prefix cut off by a leaked tool call
	// must not end the investigation. It takes the same bounded summary turn
	// as an empty final turn, and the failure receipt if that yields nothing.
	prefix := "I will check disk usage on pve1 next. "
	calls := 0
	loop := NewAgenticLoop(mainLoopLeakProvider(prefix, "", &calls), tools.NewPulseToolExecutor(tools.ExecutorConfig{}), "Investigate")
	loop.SetExecutionProfile(tools.ProfilePatrolInvestigation)
	messages, err := loop.ExecuteWithTools(context.Background(), "main-leak-investigation", []Message{{Role: "user", Content: "Check"}}, nil, func(StreamEvent) {})
	if err == nil {
		t.Fatal("leaked tool-call turn became a completed investigation")
	}
	if calls != 2 {
		t.Fatalf("provider calls=%d, want original plus bounded summary", calls)
	}
	if len(messages) == 0 || !strings.Contains(messages[len(messages)-1].Content, "final narrative") {
		t.Fatalf("missing retained fallback receipt: %#v", messages)
	}
	for _, msg := range messages {
		if strings.Contains(msg.Content, strings.TrimSpace(prefix)) {
			t.Fatalf("leaked turn prefix persisted as investigation output: %#v", messages)
		}
	}
}

func TestInvestigationMainLoopLeakedToolCallRecoversWithRealConclusion(t *testing.T) {
	conclusion := "### Conclusion\nNEEDS_ATTENTION: /var/log on pve1 needs rotation by the owner"
	calls := 0
	loop := NewAgenticLoop(mainLoopLeakProvider("I will check disk usage on pve1 next. ", conclusion, &calls), tools.NewPulseToolExecutor(tools.ExecutorConfig{}), "Investigate")
	loop.SetExecutionProfile(tools.ProfilePatrolInvestigation)
	messages, err := loop.ExecuteWithTools(context.Background(), "main-leak-recovered", []Message{{Role: "user", Content: "Check"}}, nil, func(StreamEvent) {})
	if err != nil {
		t.Fatalf("real summary conclusion was rejected: %v", err)
	}
	if calls != 2 {
		t.Fatalf("provider calls=%d, want original plus bounded summary", calls)
	}
	if got := messages[len(messages)-1].Content; got != conclusion {
		t.Fatalf("final message = %q, want the summary conclusion", got)
	}
}
