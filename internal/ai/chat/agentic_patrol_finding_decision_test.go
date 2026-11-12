package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

const findingDecisionTestPrompt = "Before you finish: Pulse has a warning alert active on \"worker\". Record a finding or say that none is needed."

func findingDecisionTestTools() []providers.Tool {
	return []providers.Tool{
		{Name: agentcapabilities.PulseQueryToolName},
		{Name: agentcapabilities.PatrolReportFindingToolName},
		{Name: agentcapabilities.PatrolAssessFindingToolName},
	}
}

func findingDecisionReportCall() providers.ToolCall {
	return providers.ToolCall{ID: "report-crash", Name: agentcapabilities.PatrolReportFindingToolName, Input: map[string]interface{}{
		"key":            "crashed-sigsegv",
		"severity":       "warning",
		"category":       "reliability",
		"resource_id":    "app-container-worker",
		"resource_name":  "worker",
		"resource_type":  "app-container",
		"title":          "worker crashed with SIGSEGV",
		"description":    "The container exited with code 139.",
		"recommendation": "Read the container log for the crash and check the image.",
		"evidence":       "State exited, exit code 139.",
	}}
}

func newFindingDecisionTestLoop(provider *stubStreamingProvider, prompt string) (*AgenticLoop, *repairTestPatrolFindingCreator) {
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	executor.ApplyExecutionProfile(tools.ProfilePatrolDetection)
	creator := &repairTestPatrolFindingCreator{}
	creator.GetActiveFindings("", "")
	executor.SetPatrolFindingCreator(creator)
	loop := NewAgenticLoop(provider, executor, "full Patrol prompt")
	loop.SetExecutionProfile(tools.ProfilePatrolDetection)
	loop.SetMaxTurns(4)
	loop.SetPatrolFindingDecisionPrompt(prompt)
	return loop, creator
}

// A Watch run on a resource with an active alert that ends in prose and
// records nothing is asked once, in the same conversation, to record what it
// described or to decline. The turn that answers has the decision tools only.
func TestAgenticLoop_WatchRunWithoutAFindingDecisionIsAskedOnce(t *testing.T) {
	t.Run("the model declines", func(t *testing.T) {
		provider := &stubStreamingProvider{}
		loop, creator := newFindingDecisionTestLoop(provider, findingDecisionTestPrompt)
		calls := 0
		provider.chatStream = func(_ context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			calls++
			switch calls {
			case 1:
				if !hasProviderTool(req.Tools, agentcapabilities.PulseQueryToolName) {
					t.Fatalf("first turn lost its evidence tools: %+v", req.Tools)
				}
				callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "Exit code 0. This looks like a finished job."}})
			case 2:
				if req.System != patrolFinalFindingDecisionSystemPrompt {
					t.Fatalf("follow-up turn system prompt = %q", req.System)
				}
				if len(req.Tools) != 2 || hasProviderTool(req.Tools, agentcapabilities.PulseQueryToolName) {
					t.Fatalf("follow-up turn tools = %+v, want report and assess only", req.Tools)
				}
				last := req.Messages[len(req.Messages)-1]
				if last.Role != "user" || last.Content != findingDecisionTestPrompt {
					t.Fatalf("follow-up turn must end with the question as a user message, got %+v", last)
				}
				if previous := req.Messages[len(req.Messages)-2]; previous.Role != "assistant" || !strings.Contains(previous.Content, "finished job") {
					t.Fatalf("follow-up turn lost the model's own analysis: %+v", previous)
				}
				callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: " No finding is needed: the job finished."}})
			default:
				t.Fatalf("unexpected provider call %d", calls)
			}
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1}})
			return nil
		}

		result, err := loop.ExecuteWithTools(context.Background(), "decline", []Message{{Role: "user", Content: "check the worker"}}, findingDecisionTestTools(), func(StreamEvent) {})
		if err != nil {
			t.Fatalf("ExecuteWithTools: %v", err)
		}
		if calls != 2 || len(creator.created) != 0 {
			t.Fatalf("provider calls=%d findings=%d, want 2 calls and no finding", calls, len(creator.created))
		}
		text := ""
		for _, message := range result {
			if message.Role == "assistant" {
				text += message.Content
			}
		}
		if !strings.Contains(text, "finished job") || !strings.Contains(text, "No finding is needed") {
			t.Fatalf("the run's text must keep the analysis and the answer, got %q", text)
		}
	})

	t.Run("the model records what it described", func(t *testing.T) {
		provider := &stubStreamingProvider{}
		loop, creator := newFindingDecisionTestLoop(provider, findingDecisionTestPrompt)
		calls := 0
		provider.chatStream = func(_ context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			calls++
			switch calls {
			case 1:
				callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "Exit code 139 is a reliability symptom beyond the exited state."}})
				callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
			case 2:
				callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{ToolCalls: []providers.ToolCall{findingDecisionReportCall()}}})
			case 3:
				callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: " Recorded the crash."}})
				callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
			default:
				t.Fatalf("unexpected provider call %d: the question must be asked once", calls)
			}
			return nil
		}

		if _, err := loop.ExecuteWithTools(context.Background(), "record", []Message{{Role: "user", Content: "check the worker"}}, findingDecisionTestTools(), func(StreamEvent) {}); err != nil {
			t.Fatalf("ExecuteWithTools: %v", err)
		}
		if calls != 3 || len(creator.created) != 1 || creator.created[0].Key != "crashed-sigsegv" {
			t.Fatalf("provider calls=%d findings=%+v, want the crash recorded in the follow-up turn", calls, creator.created)
		}
	})

	t.Run("prose again is not asked twice", func(t *testing.T) {
		provider := &stubStreamingProvider{}
		loop, _ := newFindingDecisionTestLoop(provider, findingDecisionTestPrompt)
		calls := 0
		provider.chatStream = func(_ context.Context, _ providers.ChatRequest, callback providers.StreamCallback) error {
			calls++
			callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "It crashed."}})
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
			return nil
		}
		if _, err := loop.ExecuteWithTools(context.Background(), "prose-twice", []Message{{Role: "user", Content: "check the worker"}}, findingDecisionTestTools(), func(StreamEvent) {}); err != nil {
			t.Fatalf("ExecuteWithTools: %v", err)
		}
		if calls != 2 {
			t.Fatalf("provider calls = %d, want the run and one follow-up", calls)
		}
	})
}

// The question is for runs that decided nothing. A run that recorded a
// finding, a run Patrol did not ask it for, and a profile other than Watch
// end as before.
func TestAgenticLoop_FindingDecisionQuestionIsNotAskedWhenItDoesNotApply(t *testing.T) {
	t.Run("the run already recorded a finding", func(t *testing.T) {
		provider := &stubStreamingProvider{}
		loop, creator := newFindingDecisionTestLoop(provider, findingDecisionTestPrompt)
		calls := 0
		provider.chatStream = func(_ context.Context, _ providers.ChatRequest, callback providers.StreamCallback) error {
			calls++
			if calls == 1 {
				callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{ToolCalls: []providers.ToolCall{findingDecisionReportCall()}}})
				return nil
			}
			callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "Recorded the crash."}})
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
			return nil
		}
		if _, err := loop.ExecuteWithTools(context.Background(), "already-recorded", []Message{{Role: "user", Content: "check the worker"}}, findingDecisionTestTools(), func(StreamEvent) {}); err != nil {
			t.Fatalf("ExecuteWithTools: %v", err)
		}
		if calls != 2 || len(creator.created) != 1 {
			t.Fatalf("provider calls=%d findings=%d, want 2 and 1 with no follow-up", calls, len(creator.created))
		}
	})

	t.Run("Patrol set no question", func(t *testing.T) {
		provider := &stubStreamingProvider{}
		loop, _ := newFindingDecisionTestLoop(provider, "")
		calls := 0
		provider.chatStream = func(_ context.Context, _ providers.ChatRequest, callback providers.StreamCallback) error {
			calls++
			callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "All clear."}})
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
			return nil
		}
		if _, err := loop.ExecuteWithTools(context.Background(), "no-question", []Message{{Role: "user", Content: "check the worker"}}, findingDecisionTestTools(), func(StreamEvent) {}); err != nil {
			t.Fatalf("ExecuteWithTools: %v", err)
		}
		if calls != 1 {
			t.Fatalf("provider calls = %d, want 1", calls)
		}
	})

	t.Run("the last turn was already decision-only", func(t *testing.T) {
		provider := &stubStreamingProvider{}
		loop, _ := newFindingDecisionTestLoop(provider, findingDecisionTestPrompt)
		loop.SetMaxTurns(1)
		calls := 0
		provider.chatStream = func(_ context.Context, _ providers.ChatRequest, callback providers.StreamCallback) error {
			calls++
			callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "It crashed."}})
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
			return nil
		}
		if _, err := loop.ExecuteWithTools(context.Background(), "single-turn", []Message{{Role: "user", Content: "check the worker"}}, findingDecisionTestTools(), func(StreamEvent) {}); err != nil {
			t.Fatalf("ExecuteWithTools: %v", err)
		}
		if calls != 1 {
			t.Fatalf("provider calls = %d, want 1: the only turn already offered the decision tools alone", calls)
		}
	})
}

// The final decision turn tells the model that real-time alerts own a stopped
// resource. It must carry the exception too, or the model's last word on a
// container Pulse raised no alert for is that an alert covers it.
func TestFinalFindingDecisionPromptCarriesTheNoAlertException(t *testing.T) {
	if !strings.Contains(patrolFinalFindingDecisionSystemPrompt, "owned by real-time alerts") {
		t.Fatal("the final decision prompt no longer states the stopped-resource rule this exception belongs to")
	}
	if !strings.Contains(patrolFinalFindingDecisionSystemPrompt, tools.PatrolUnalertedStoppedContainerRule) {
		t.Fatal("the final decision prompt states the stopped-resource rule without its no-alert exception")
	}
}
