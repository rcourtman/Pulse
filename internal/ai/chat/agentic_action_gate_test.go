package chat

// Regression transcript for GitHub issue #1782. A paying operator asked the
// Assistant (Gemini, Controlled mode) to reboot five Windows VMs matching a
// name pattern. The model resolved all five with pulse_query, then ended the
// run with a markdown report ("Next steps", an invented prerequisite) and
// never submitted pulse_control. The expected behaviour is one governed
// pulse_control plan per target, each awaiting approval in Pulse.
//
// The scripted provider below reproduces the field transcript turn by turn.
// On a build without the advertised-action gate the run ends at turn 2 with
// the report and zero pulse_control calls, which is the failing assertion.

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

const gateTestReport = `## Summary
I found five Windows VMs matching "win": win-01, win-02, win-03, win-04, win-05.

## Limitation
The reboot could not be scheduled because these VMs are not yet bound to a discovery session in the current context.

## Next steps
1. Run a discovery for the VMs.
2. Ask me again and I will reboot them.`

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

func gateTestContainsBlock(req providers.ChatRequest) bool {
	for _, msg := range req.Messages {
		if msg.Role == "user" && strings.Contains(msg.Content, "BLOCKED: the user asked you to reboot") {
			return true
		}
	}
	return false
}

// TestAgenticLoop_BulkLifecycleRequestEndsInPulseControlPlans is the #1782
// transcript: resolve five VMs, try to end with a report, and prove the run
// instead submits one governed plan per target before answering.
func TestAgenticLoop_BulkLifecycleRequestEndsInPulseControlPlans(t *testing.T) {
	vms := []unifiedresources.Resource{
		gateTestProxmoxVM("win-01", 101),
		gateTestProxmoxVM("win-02", 102),
		gateTestProxmoxVM("win-03", 103),
		gateTestProxmoxVM("win-04", 104),
		gateTestProxmoxVM("win-05", 105),
	}
	planner := &gateTestPlanner{}
	exec := newGateTestExecutor(t, planner, vms...)

	var (
		mu               sync.Mutex
		turn             int
		blockSeenAtTurn  int
		requestsPerTurn  []providers.ChatRequest
		reportStreamed   bool
		finalAnswerTurns int
	)
	provider := &stubStreamingProvider{}
	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		mu.Lock()
		turn++
		current := turn
		requestsPerTurn = append(requestsPerTurn, req)
		if gateTestContainsBlock(req) && blockSeenAtTurn == 0 {
			blockSeenAtTurn = current
		}
		mu.Unlock()

		switch current {
		case 1:
			// The model resolves the targets exactly as in the field.
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{
				ToolCalls: []providers.ToolCall{{
					ID:    "q-1",
					Name:  "pulse_query",
					Input: map[string]interface{}{"action": "search", "query": "win", "type": "vm"},
				}},
			}})
		case 2:
			// The field failure: a report with an invented prerequisite.
			mu.Lock()
			reportStreamed = true
			mu.Unlock()
			callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: gateTestReport}})
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		case 3:
			// Steered by the gate, the model submits one plan per target.
			require.True(t, gateTestContainsBlock(req), "turn 3 must carry the advertised-action correction")
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{ToolCalls: gateTestControlCalls(vms, "reboot")}})
		case 4:
			// Post-write verification read demanded by the FSM.
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{
				ToolCalls: []providers.ToolCall{{
					ID:    "q-2",
					Name:  "pulse_query",
					Input: map[string]interface{}{"action": "search", "query": "win", "type": "vm"},
				}},
			}})
		default:
			mu.Lock()
			finalAnswerTurns++
			mu.Unlock()
			callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "Planned a reboot for all five VMs; approve them in Pulse to proceed."}})
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		}
		return nil
	}

	loop := NewAgenticLoop(provider, exec, "base prompt")
	loop.SetSessionFSM(NewSessionFSM())

	messages, err := loop.ExecuteWithTools(
		context.Background(),
		"gate-session",
		[]Message{{Role: "user", Content: "Reboot all my Windows VMs whose name starts with win-. There should be five of them."}},
		nil,
		func(StreamEvent) {},
	)
	require.NoError(t, err)

	planned := map[string]bool{}
	for _, msg := range messages {
		if msg.ToolResult != nil && msg.ToolResult.ToolUseID == "q-1" {
			require.False(t, msg.ToolResult.IsError, "resolution query must succeed: %s", msg.ToolResult.Content)
			require.Contains(t, msg.ToolResult.Content, "win-05", "resolution query must list every target: %s", msg.ToolResult.Content)
		}
		if msg.ToolResult == nil || !strings.HasPrefix(msg.ToolResult.ToolUseID, "c-") {
			continue
		}
		require.False(t, msg.ToolResult.IsError, "pulse_control must plan, not error: %s", msg.ToolResult.Content)
		var payload map[string]any
		require.NoError(t, json.Unmarshal([]byte(msg.ToolResult.Content), &payload), msg.ToolResult.Content)
		require.Equal(t, true, payload["planned"], payload)
		require.Equal(t, true, payload["requires_approval"], "Controlled mode plans wait for approval: %v", payload)
		require.Equal(t, "reboot", payload["capability"], payload)
		planned[fmt.Sprint(payload["resource_id"])] = true
	}
	require.Len(t, planned, len(vms), "one governed plan per resolved target; got %v", planned)

	requests := planner.snapshot()
	require.Len(t, requests, len(vms))
	for _, req := range requests {
		require.True(t, strings.HasPrefix(req.ResourceID, "vm-pve-win-0"), "plans must carry the canonical unified id, got %q", req.ResourceID)
		require.Equal(t, "reboot", req.CapabilityName)
	}

	mu.Lock()
	defer mu.Unlock()
	require.True(t, reportStreamed, "the scripted field report must have been produced")
	require.Equal(t, 3, blockSeenAtTurn, "the gate must refuse the report once and steer the very next turn")
	require.Equal(t, 1, finalAnswerTurns, "after planning and verifying, the answer is accepted")
	require.Len(t, requestsPerTurn, 5)
	require.True(t, hasFinalAssistantText(messages))
}

// TestAgenticLoop_AdvertisedActionGateFailsOpenAfterOneRefusal pins the
// bounded escape hatch: a model that still answers in prose after the single
// correction is not livelocked, and the run ends with its answer.
func TestAgenticLoop_AdvertisedActionGateFailsOpenAfterOneRefusal(t *testing.T) {
	vms := []unifiedresources.Resource{gateTestProxmoxVM("win-01", 101)}
	planner := &gateTestPlanner{}
	exec := newGateTestExecutor(t, planner, vms...)

	turn := 0
	provider := &stubStreamingProvider{}
	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		turn++
		if turn == 1 {
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{
				ToolCalls: []providers.ToolCall{{ID: "q-1", Name: "pulse_query", Input: map[string]interface{}{"action": "search", "query": "win"}}},
			}})
			return nil
		}
		callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "I will not reboot win-01: its console shows an in-progress Windows update (pulse_read evidence above)."}})
		callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		return nil
	}

	loop := NewAgenticLoop(provider, exec, "base prompt")
	loop.SetSessionFSM(NewSessionFSM())
	messages, err := loop.ExecuteWithTools(context.Background(), "gate-failopen", []Message{{Role: "user", Content: "please reboot win-01"}}, nil, func(StreamEvent) {})
	require.NoError(t, err)
	require.Equal(t, 3, turn, "query, refused prose, accepted prose")
	require.Empty(t, planner.snapshot(), "the gate steers; it never submits on the model's behalf")
	require.True(t, hasFinalAssistantText(messages))
}

// TestAgenticLoop_AdvertisedActionGateLeavesQuestionsAlone pins that an
// operator asking *about* a lifecycle event keeps a normal investigative
// answer: the gate only applies to action requests.
func TestAgenticLoop_AdvertisedActionGateLeavesQuestionsAlone(t *testing.T) {
	vms := []unifiedresources.Resource{gateTestProxmoxVM("win-01", 101)}
	planner := &gateTestPlanner{}
	exec := newGateTestExecutor(t, planner, vms...)

	turn := 0
	provider := &stubStreamingProvider{}
	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		turn++
		require.False(t, gateTestContainsBlock(req), "a question must never trip the advertised-action gate")
		if turn == 1 {
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{
				ToolCalls: []providers.ToolCall{{ID: "q-1", Name: "pulse_query", Input: map[string]interface{}{"action": "search", "query": "win"}}},
			}})
			return nil
		}
		callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "win-01 is running; nothing in the current state explains a reboot."}})
		callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		return nil
	}

	loop := NewAgenticLoop(provider, exec, "base prompt")
	loop.SetSessionFSM(NewSessionFSM())
	_, err := loop.ExecuteWithTools(context.Background(), "gate-question", []Message{{Role: "user", Content: "Why did win-01 reboot last night?"}}, nil, func(StreamEvent) {})
	require.NoError(t, err)
	require.Equal(t, 2, turn)
	require.Empty(t, planner.snapshot())
}

func TestRequestedLifecycleAction(t *testing.T) {
	cases := []struct {
		text   string
		action string
		ok     bool
	}{
		{"Reboot all my Windows VMs matching win-*", "reboot", true},
		{"can you restart the five win VMs?", "reboot", true},
		{"Please power-cycle win-01", "reboot", true},
		{"shut down win-02 gracefully", "shutdown", true},
		{"stop win-03 now", "stop", true},
		{"start win-04 again", "start", true},
		{"Why did win-01 reboot last night?", "", false},
		{"Is it safe to stop win-02?", "", false},
		{"Should I restart win-03?", "", false},
		{"how is my infrastructure doing?", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		action, ok := requestedLifecycleAction(tc.text)
		require.Equal(t, tc.ok, ok, tc.text)
		require.Equal(t, tc.action, action, tc.text)
	}
}
