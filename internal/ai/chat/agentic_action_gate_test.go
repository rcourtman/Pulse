package chat

// Model-owned continuation retains actual tool authority and conversation evidence.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

func gateTestProxmoxVM(name string, vmid int) unifiedresources.Resource {
	capabilities := []unifiedresources.ResourceCapability{}
	for _, operation := range []string{"shutdown", "reboot", "stop"} {
		capabilities = append(capabilities, unifiedresources.ResourceCapability{
			Name:                 operation,
			Type:                 unifiedresources.CapabilityTypeCommon,
			Description:          "Proxmox VM lifecycle " + operation,
			MinimumApprovalLevel: unifiedresources.ApprovalAdmin,
			Platform:             "qemu",
			InternalHandler:      "proxmox.vm.lifecycle",
		})
	}
	return unifiedresources.Resource{
		ID:           "vm-pve-" + name,
		Type:         unifiedresources.ResourceTypeVM,
		Name:         name,
		Status:       unifiedresources.StatusOnline,
		ParentName:   "pve",
		Capabilities: capabilities,
		Proxmox: &unifiedresources.ProxmoxData{
			SourceID: fmt.Sprintf("pve:pve:%d", vmid),
			NodeName: "pve",
			Instance: "pve",
			VMID:     vmid,
		},
	}
}

type gateTestPlanner struct {
	mu       sync.Mutex
	requests []unifiedresources.ActionRequest
}

func (p *gateTestPlanner) PlanTypedAction(_ context.Context, _ string, req unifiedresources.ActionRequest) (*unifiedresources.ActionPlan, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, req)
	return &unifiedresources.ActionPlan{
		ActionID:         fmt.Sprintf("action-%d", len(p.requests)),
		RequestID:        req.RequestID,
		Allowed:          true,
		RequiresApproval: true,
		ApprovalPolicy:   unifiedresources.ApprovalAdmin,
		PlanHash:         "hash",
	}, nil
}

func (p *gateTestPlanner) snapshot() []unifiedresources.ActionRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]unifiedresources.ActionRequest(nil), p.requests...)
}

func newGateTestExecutor(t *testing.T, planner tools.TypedActionPlanner, vms ...unifiedresources.Resource) *tools.PulseToolExecutor {
	t.Helper()
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider:           &mockStateProvider{},
		AgentServer:             &mockAgentServer{},
		UnifiedResourceProvider: plainTextResourceTestProvider(vms...),
		TypedActionPlanner:      planner,
		ControlLevel:            tools.ControlLevelControlled,
	})
	exec.SetResolvedContext(NewResolvedContext("gate-session"))
	return exec
}

func gateTestControlCalls(vms []unifiedresources.Resource, action string) []providers.ToolCall {
	calls := make([]providers.ToolCall, 0, len(vms))
	for i, vm := range vms {
		calls = append(calls, providers.ToolCall{
			ID:   fmt.Sprintf("c-%d", i+1),
			Name: "pulse_control",
			Input: map[string]interface{}{
				"type":        "resource",
				"resource_id": vm.ID,
				"action":      action,
			},
		})
	}
	return calls
}

func TestAgenticLoop_ModelPlansBulkLifecycleWithoutSyntheticVerification(t *testing.T) {
	vms := []unifiedresources.Resource{
		gateTestProxmoxVM("win-01", 101), gateTestProxmoxVM("win-02", 102),
		gateTestProxmoxVM("win-03", 103), gateTestProxmoxVM("win-04", 104), gateTestProxmoxVM("win-05", 105),
	}
	planner := &gateTestPlanner{}
	executor := newGateTestExecutor(t, planner, vms...)
	const final = "Five reboot plans are awaiting approval. No VM has been restarted."
	turn := 0
	provider := &stubStreamingProvider{}
	provider.chatStream = func(_ context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		turn++
		switch turn {
		case 1:
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{ToolCalls: []providers.ToolCall{{ID: "q-1", Name: "pulse_query", Input: map[string]interface{}{"action": "search", "query": "win", "type": "vm"}}}}})
		case 2:
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{ToolCalls: gateTestControlCalls(vms, "reboot")}})
		case 3:
			require.NotEmpty(t, req.Tools, "preparing plans must leave investigation tools available")
			callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: final}})
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		default:
			t.Fatalf("unexpected synthetic continuation turn %d", turn)
		}
		return nil
	}
	var streamed strings.Builder
	loop := NewAgenticLoop(provider, executor, "Use canonical capability facts and explain pending approval honestly.")
	messages, err := loop.ExecuteWithTools(context.Background(), "bulk-plans", []Message{{Role: "user", Content: "Reboot the five Windows VMs matching win-."}}, nil, func(event StreamEvent) {
		if event.Type == "content" {
			var data ContentData
			require.NoError(t, json.Unmarshal(event.Data, &data))
			streamed.WriteString(data.Text)
		}
	})
	require.NoError(t, err)
	require.Equal(t, 3, turn)
	require.Len(t, planner.snapshot(), 5)
	var saved strings.Builder
	planned := map[string]bool{}
	for _, msg := range messages {
		if msg.Role == "assistant" {
			saved.WriteString(msg.Content)
		}
		if msg.ToolResult == nil || !strings.HasPrefix(msg.ToolResult.ToolUseID, "c-") {
			continue
		}
		require.False(t, msg.ToolResult.IsError, msg.ToolResult.Content)
		var payload map[string]any
		require.NoError(t, json.Unmarshal([]byte(msg.ToolResult.Content), &payload))
		require.Equal(t, true, payload["planned"])
		require.Equal(t, true, payload["requires_approval"])
		planned[fmt.Sprint(payload["resource_id"])] = true
	}
	require.Len(t, planned, 5)
	require.Equal(t, final, streamed.String())
	require.Equal(t, streamed.String(), saved.String(), "saved history must preserve the answer that was streamed")
}

func TestAgenticLoop_ModelMayConcludeWithoutAnAction(t *testing.T) {
	for _, prompt := range []string{"Please reboot win-01", "Why did win-01 reboot last night?"} {
		t.Run(prompt, func(t *testing.T) {
			planner := &gateTestPlanner{}
			executor := newGateTestExecutor(t, planner, gateTestProxmoxVM("win-01", 101))
			turn := 0
			const answer = "The current snapshot cannot establish whether rebooting is appropriate. I have prepared no action."
			provider := &stubStreamingProvider{}
			provider.chatStream = func(_ context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
				turn++
				if turn == 1 {
					callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{ToolCalls: []providers.ToolCall{{ID: "q-1", Name: "pulse_query", Input: map[string]interface{}{"action": "search", "query": "win"}}}}})
				} else {
					require.Equal(t, 2, turn, "lifecycle words must not force a corrective provider turn")
					callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: answer}})
					callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
				}
				return nil
			}
			loop := NewAgenticLoop(provider, executor, "base prompt")
			messages, err := loop.ExecuteWithTools(context.Background(), "no-action", []Message{{Role: "user", Content: prompt}}, nil, func(StreamEvent) {})
			require.NoError(t, err)
			require.Equal(t, 2, turn)
			require.Empty(t, planner.snapshot())
			require.Equal(t, answer, messages[len(messages)-1].Content)
		})
	}
}
