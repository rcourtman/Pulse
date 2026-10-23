package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

func investigationConclusionTestTools() []providers.Tool {
	return []providers.Tool{
		{Name: agentcapabilities.PulseQueryToolName},
		{Name: agentcapabilities.PatrolProposeActionToolName},
		{Name: agentcapabilities.PatrolConcludeInvestigationToolName},
	}
}

func newConclusionTestExecutor(capture *tools.ProposalCapture) *tools.PulseToolExecutor {
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{StateProvider: &mockStateProvider{}})
	executor.ApplyExecutionProfile(tools.ProfilePatrolInvestigation)
	if capture != nil {
		executor.SetProposalCapture(capture)
	}
	return executor
}

func toolNames(list []providers.Tool) []string {
	names := make([]string, 0, len(list))
	for _, tool := range list {
		names = append(names, tool.Name)
	}
	return names
}

func TestAgenticLoopPatrolInvestigationFinalTurnOffersOnlyTheConclusion(t *testing.T) {
	provider := &stubStreamingProvider{}
	var requests []providers.ChatRequest
	provider.chatStream = func(_ context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		requests = append(requests, req)
		if len(requests) == 1 {
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{ToolCalls: []providers.ToolCall{{ID: "evidence-1", Name: agentcapabilities.PulseQueryToolName, Input: map[string]interface{}{"action": "search", "query": "disks"}}}}})
			return nil
		}
		callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "final investigation summary"}})
		callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		return nil
	}

	loop := NewAgenticLoop(provider, newConclusionTestExecutor(nil), "system")
	loop.SetExecutionProfile(tools.ProfilePatrolInvestigation)
	loop.SetMaxTurns(2)

	if _, err := loop.ExecuteWithTools(context.Background(), "final-turn", []Message{{Role: "user", Content: "investigate"}}, investigationConclusionTestTools(), func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteWithTools: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("provider requests = %d, want 2", len(requests))
	}
	if names := toolNames(requests[1].Tools); len(names) != 1 || names[0] != agentcapabilities.PatrolConcludeInvestigationToolName {
		t.Fatalf("final-turn tools = %v, want only patrol_conclude_investigation (no proposal before the evidence budget is spent)", names)
	}
	if !strings.Contains(requests[1].System, "final turn") {
		t.Fatalf("final-turn system prompt missing the conclusion instruction: %q", requests[1].System)
	}
}

func TestAgenticLoopPatrolInvestigationAsksForSummaryAfterConclusion(t *testing.T) {
	provider := &stubStreamingProvider{}
	var requests []providers.ChatRequest
	provider.chatStream = func(_ context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		requests = append(requests, req)
		if len(requests) == 1 {
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{ToolCalls: []providers.ToolCall{{
				ID:    "conclude-1",
				Name:  agentcapabilities.PatrolConcludeInvestigationToolName,
				Input: map[string]interface{}{"outcome": "needs_attention", "reason": "the pool needs a replacement disk"},
			}}}})
			return nil
		}
		callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "### Investigation Summary\nThe pool is degraded."}})
		callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		return nil
	}

	capture := tools.NewProposalCapture(tools.ProposalIdentity{ProposalID: "p1", FindingID: "f1", InvestigationID: "i1"}, nil)
	loop := NewAgenticLoop(provider, newConclusionTestExecutor(capture), "system")
	loop.SetExecutionProfile(tools.ProfilePatrolInvestigation)
	loop.SetMaxTurns(6)

	result, err := loop.ExecuteWithTools(context.Background(), "conclude-then-summarize", []Message{{Role: "user", Content: "investigate"}}, investigationConclusionTestTools(), func(StreamEvent) {})
	if err != nil {
		t.Fatalf("ExecuteWithTools: %v", err)
	}
	conclusion := capture.Conclusion()
	if conclusion == nil || conclusion.Outcome != aicontracts.OutcomeNeedsAttention {
		t.Fatalf("recorded conclusion = %+v, want needs_attention", conclusion)
	}
	if len(requests) != 2 {
		t.Fatalf("provider requests = %d, want 2 (conclusion, then summary)", len(requests))
	}
	if len(requests[1].Tools) != 0 {
		t.Fatalf("post-conclusion tools = %v, want none: the next response is the summary", toolNames(requests[1].Tools))
	}
	if len(result) == 0 || !strings.Contains(result[len(result)-1].Content, "The pool is degraded.") {
		t.Fatalf("final result = %+v, want the summary", result)
	}
}

func TestAgenticLoopPatrolInvestigationConcludesBeforeItsDeadline(t *testing.T) {
	provider := &stubStreamingProvider{}
	var requests []providers.ChatRequest
	provider.chatStream = func(_ context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		requests = append(requests, req)
		if len(requests) == 1 {
			// A slow evidence turn leaves less than the conclusion reserve.
			time.Sleep(1100 * time.Millisecond)
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{ToolCalls: []providers.ToolCall{{ID: "evidence-1", Name: agentcapabilities.PulseQueryToolName, Input: map[string]interface{}{"action": "search", "query": "disks"}}}}})
			return nil
		}
		callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "final investigation summary"}})
		callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		return nil
	}

	loop := NewAgenticLoop(provider, newConclusionTestExecutor(nil), "system")
	loop.SetExecutionProfile(tools.ProfilePatrolInvestigation)
	loop.SetMaxTurns(12)

	// A 2s budget reserves 1s (half the budget) to conclude.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := loop.ExecuteWithTools(ctx, "deadline-reserve", []Message{{Role: "user", Content: "investigate"}}, investigationConclusionTestTools(), func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteWithTools: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("provider requests = %d, want 2", len(requests))
	}
	if names := toolNames(requests[1].Tools); len(names) != 1 || names[0] != agentcapabilities.PatrolConcludeInvestigationToolName {
		t.Fatalf("tools after the reserve was reached = %v, want only patrol_conclude_investigation", names)
	}
}

func TestInvestigationConclusionReserve(t *testing.T) {
	cases := []struct {
		name    string
		slowest time.Duration
		budget  time.Duration
		want    time.Duration
	}{
		{name: "no turn seen yet uses the minimum estimate", slowest: 0, budget: 10 * time.Minute, want: 75 * time.Second},
		// Another evidence turn plus the conclusion and summary turns.
		{name: "slow subscription turns reserve three of them", slowest: 90 * time.Second, budget: 25 * time.Minute, want: 285 * time.Second},
		{name: "near-limit subscription turns still fit", slowest: 225 * time.Second, budget: 25 * time.Minute, want: 690 * time.Second},
		{name: "short budgets keep half for evidence", slowest: 0, budget: time.Minute, want: 30 * time.Second},
	}
	for _, tc := range cases {
		if got := investigationConclusionReserve(tc.slowest, tc.budget); got != tc.want {
			t.Fatalf("%s: reserve = %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestInvestigationStreamIdleTimeoutWidensOnlySubscriptionInvestigationTurns(t *testing.T) {
	base := PatrolProviderStreamIdleTimeout
	cases := []struct {
		name     string
		profile  tools.ExecutionProfile
		provider string
		want     time.Duration
	}{
		{name: "codex subscription investigation", profile: tools.ProfilePatrolInvestigation, provider: string(providers.SubscriptionAgentCodex), want: patrolSubscriptionInvestigationTurnTimeout},
		{name: "claude subscription investigation", profile: tools.ProfilePatrolInvestigation, provider: string(providers.SubscriptionAgentClaude), want: patrolSubscriptionInvestigationTurnTimeout},
		{name: "subscription Watch keeps its stall budget", profile: tools.ProfilePatrolDetection, provider: string(providers.SubscriptionAgentCodex), want: base},
		{name: "streaming provider investigation keeps its idle budget", profile: tools.ProfilePatrolInvestigation, provider: "anthropic", want: base},
	}
	for _, tc := range cases {
		if got := investigationStreamIdleTimeout(tc.profile, tc.provider, base); got != tc.want {
			t.Fatalf("%s: stream idle timeout = %s, want %s", tc.name, got, tc.want)
		}
	}
	if got := investigationStreamIdleTimeout(tools.ProfileInteractiveAssistant, string(providers.SubscriptionAgentCodex), 0); got != 0 {
		t.Fatalf("interactive runs without a stall budget must stay unbounded, got %s", got)
	}
}

func TestInvestigationSummaryTimeoutUsesTheRemainingDeadline(t *testing.T) {
	base := 20 * time.Second
	if got := investigationSummaryTimeout(context.Background(), tools.ProfilePatrolInvestigation, base); got != base {
		t.Fatalf("no deadline: timeout = %s, want %s", got, base)
	}

	long, cancelLong := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelLong()
	if got := investigationSummaryTimeout(long, tools.ProfilePatrolInvestigation, base); got <= 4*time.Minute {
		t.Fatalf("investigation summary timeout = %s, want the remaining ~5m", got)
	}
	if got := investigationSummaryTimeout(long, tools.ProfileInteractiveAssistant, base); got != base {
		t.Fatalf("interactive summary timeout = %s, want %s", got, base)
	}

	short, cancelShort := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShort()
	if got := investigationSummaryTimeout(short, tools.ProfilePatrolInvestigation, base); got != base {
		t.Fatalf("near-deadline summary timeout = %s, want %s (the deadline still bounds it)", got, base)
	}
}
