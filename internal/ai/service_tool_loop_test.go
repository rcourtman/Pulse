package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func installEmptyServiceToolLoopApprovalStore(t *testing.T) *approval.Store {
	t.Helper()
	store, err := approval.NewStore(approval.StoreConfig{DataDir: t.TempDir(), DisablePersistence: true})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	previous := approval.GetStore()
	approval.SetStore(store)
	t.Cleanup(func() { approval.SetStore(previous) })
	return store
}

func requireServiceToolLoopFailure(t *testing.T, err error, reason ServiceToolLoopFailureReason) *ServiceToolLoopError {
	t.Helper()
	var loopErr *ServiceToolLoopError
	if !errors.As(err, &loopErr) {
		t.Fatalf("error = %v, want *ServiceToolLoopError", err)
	}
	if loopErr.Reason != reason {
		t.Fatalf("loop failure reason = %q, want %q", loopErr.Reason, reason)
	}
	return loopErr
}

func TestService_Execute_RepeatedDeniedToolCallTerminatesBounded(t *testing.T) {
	store := installEmptyServiceToolLoopApprovalStore(t)
	dispatches := 0
	svc := NewService(config.NewConfigPersistence(t.TempDir()), &mockAgentServer{
		agents: []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "agent-1"}},
		executeFunc: func(context.Context, string, agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
			dispatches++
			return &agentexec.CommandResultPayload{Success: true}, nil
		},
	})
	svc.cfg = &config.AIConfig{Enabled: true, ControlLevel: config.ControlLevelAutonomous, Model: "anthropic:test"}
	providerCalls := 0
	svc.provider = &mockProvider{chatFunc: func(context.Context, providers.ChatRequest) (*providers.ChatResponse, error) {
		providerCalls++
		return &providers.ChatResponse{
			ToolCalls: []providers.ToolCall{{ID: "changing-id", Name: "run_command", Input: map[string]interface{}{
				"command": "touch /tmp/must-not-run", "target_host": "agent-1",
			}}},
			StopReason: "tool_use",
		}, nil
	}}

	_, err := svc.Execute(context.Background(), ExecuteRequest{TargetType: "agent"})
	loopErr := requireServiceToolLoopFailure(t, err, ServiceToolLoopFailureRepeatedDenied)
	if providerCalls != 2 || loopErr.ProviderCalls != 2 {
		t.Fatalf("provider calls = %d error_calls=%d, want 2", providerCalls, loopErr.ProviderCalls)
	}
	if dispatches != 0 {
		t.Fatalf("agent dispatches = %d, want 0", dispatches)
	}
	if got := len(store.GetPendingApprovals()); got != 0 {
		t.Fatalf("persisted approvals = %d, want 0", got)
	}
}

func TestServiceToolLoop_MaxIterationsBoundStreamingAndNonStreaming(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		name := "non-streaming"
		if streaming {
			name = "streaming"
		}
		t.Run(name, func(t *testing.T) {
			store := installEmptyServiceToolLoopApprovalStore(t)
			svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
			svc.cfg = &config.AIConfig{Enabled: true, ControlLevel: config.ControlLevelReadOnly, Model: "anthropic:test"}
			providerCalls := 0
			svc.provider = &mockProvider{chatFunc: func(context.Context, providers.ChatRequest) (*providers.ChatResponse, error) {
				providerCalls++
				return &providers.ChatResponse{
					ToolCalls:  []providers.ToolCall{{ID: "fetch", Name: "fetch_url", Input: map[string]interface{}{"url": ""}}},
					StopReason: "tool_use",
				}, nil
			}}

			var (
				err               error
				loopFailureEvents int
			)
			if streaming {
				_, err = svc.ExecuteStream(context.Background(), ExecuteRequest{}, func(event StreamEvent) {
					if data, ok := event.Data.(ServiceToolLoopFailureData); event.Type == "error" && ok && data.Reason == ServiceToolLoopFailureMaxIterations {
						loopFailureEvents++
					}
				})
			} else {
				_, err = svc.Execute(context.Background(), ExecuteRequest{})
			}
			loopErr := requireServiceToolLoopFailure(t, err, ServiceToolLoopFailureMaxIterations)
			if providerCalls != MaxServiceToolIterations || loopErr.ProviderCalls != MaxServiceToolIterations {
				t.Fatalf("provider calls = %d error_calls=%d, want %d", providerCalls, loopErr.ProviderCalls, MaxServiceToolIterations)
			}
			if got := len(store.GetPendingApprovals()); got != 0 {
				t.Fatalf("persisted approvals = %d, want 0", got)
			}
			if streaming && loopFailureEvents != 1 {
				t.Fatalf("typed max-iteration stream failures = %d, want 1", loopFailureEvents)
			}
		})
	}
}

func TestServiceToolLoop_AllowsTerminalResponseAtProviderCallBudget(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		name := "non-streaming"
		if streaming {
			name = "streaming"
		}
		t.Run(name, func(t *testing.T) {
			svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
			svc.cfg = &config.AIConfig{Enabled: true, ControlLevel: config.ControlLevelReadOnly, Model: "anthropic:test"}
			providerCalls := 0
			svc.provider = &mockProvider{chatFunc: func(context.Context, providers.ChatRequest) (*providers.ChatResponse, error) {
				providerCalls++
				if providerCalls == MaxServiceToolIterations {
					return &providers.ChatResponse{Content: "bounded conversation complete", StopReason: "end_turn"}, nil
				}
				return &providers.ChatResponse{
					ToolCalls:  []providers.ToolCall{{ID: "fetch", Name: "fetch_url", Input: map[string]interface{}{"url": ""}}},
					StopReason: "tool_use",
				}, nil
			}}

			var (
				resp *ExecuteResponse
				err  error
			)
			if streaming {
				resp, err = svc.ExecuteStream(context.Background(), ExecuteRequest{}, func(StreamEvent) {})
			} else {
				resp, err = svc.Execute(context.Background(), ExecuteRequest{})
			}
			if err != nil {
				t.Fatalf("bounded conversation failed: %v", err)
			}
			if providerCalls != MaxServiceToolIterations {
				t.Fatalf("provider calls = %d, want %d", providerCalls, MaxServiceToolIterations)
			}
			if resp == nil || resp.Content != "bounded conversation complete" {
				t.Fatalf("response = %#v, want terminal bounded content", resp)
			}
		})
	}
}
