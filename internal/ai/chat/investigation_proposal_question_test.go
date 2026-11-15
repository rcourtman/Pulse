package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	ur "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// On Docker fault set v1 D2, qwen3:8b read the crashed worker's logs, found
// the segfault, and ended its investigation in prose: "Use
// patrol_propose_action with the update capability on the container's
// resource ID". It called nothing, so the run ended needs_attention with no
// proposal to approve. An investigation that ends in prose without proposing
// or concluding is now asked once, with the actions Pulse can run, and the
// turn that answers may only propose or conclude.
func TestInvestigationEndingInProseIsAskedOnceToProposeOrConclude(t *testing.T) {
	const prose = "### Root Cause\nThe worker segfaulted and stays exited.\n\n### Recommendation\nStart the container again."

	type requestSeen struct {
		tools    []string
		lastUser string
		system   string
	}
	run := func(t *testing.T, firstTurn func(callback providers.StreamCallback), capabilities []string, maxTurns int) ([]requestSeen, *InvestigationRunResult) {
		t.Helper()
		store, err := NewSessionStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		var seen []requestSeen
		provider := &stubStreamingProvider{}
		provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			names := make([]string, 0, len(req.Tools))
			for _, tool := range req.Tools {
				names = append(names, tool.Name)
			}
			lastUser := ""
			for _, msg := range req.Messages {
				if msg.Role == "user" {
					lastUser = msg.Content
				}
			}
			seen = append(seen, requestSeen{tools: names, lastUser: lastUser, system: req.System})
			switch len(seen) {
			case 1:
				firstTurn(callback)
			case 2:
				callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{ToolCalls: []providers.ToolCall{{
					ID: "proposal-1", Name: agentcapabilities.PatrolProposeActionToolName,
					Input: map[string]interface{}{"resource_id": "vm:42", "capability_name": "start", "reason": "The worker crashed and stays exited."},
				}}}})
			default:
				callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: prose}})
				callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
			}
			return nil
		}
		service := &Service{
			started: true, sessions: store,
			executor:              tools.NewPulseToolExecutor(tools.ExecutorConfig{StateProvider: &mockStateProvider{}}),
			cfg:                   &config.AIConfig{PatrolModel: "mock:model", ControlLevel: config.ControlLevelReadOnly},
			patrolProviderFactory: func(string) (providers.StreamingProvider, error) { return provider, nil },
		}
		result, err := service.ExecuteInvestigationStream(context.Background(), InvestigationRunRequest{
			SessionID: "proposal-question-" + strings.ReplaceAll(t.Name(), "/", "-"), Prompt: "Investigate the crashed worker.", SystemPrompt: "Investigate.",
			MaxTurns: maxTurns, MaxEvidenceCalls: 3, ResourceType: "vm", ResourceID: "vm:42",
			Identity: tools.ProposalIdentity{ProposalID: "request-1", FindingID: "finding-1", InvestigationID: "investigation-1"},
			Planner: func(_ context.Context, p tools.CapturedProposal) (ur.ActionAuditRecord, error) {
				return ur.ActionAuditRecord{ID: "act_1", State: ur.ActionStatePending}, nil
			},
			Catalog: func(context.Context, string) ([]ur.ResourceCapability, error) {
				out := make([]ur.ResourceCapability, 0, len(capabilities))
				for _, name := range capabilities {
					out = append(out, ur.ResourceCapability{Name: name})
				}
				return out, nil
			},
		}, func(StreamEvent) {})
		if err != nil {
			t.Fatal(err)
		}
		return seen, result
	}
	endsInProse := func(callback providers.StreamCallback) {
		callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: prose}})
		callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
	}

	t.Run("ends in prose", func(t *testing.T) {
		seen, result := run(t, endsInProse, []string{"start", "restart", "update"}, 6)
		if len(seen) < 2 {
			t.Fatalf("the model was asked %d times, want a follow-up turn", len(seen))
		}
		for _, want := range []string{"Pulse can run these actions for vm:42: start, restart, update.", "propose the smallest one", agentcapabilities.PatrolConcludeInvestigationToolName} {
			if !strings.Contains(seen[1].lastUser, want) {
				t.Errorf("follow-up question lacks %q: %q", want, seen[1].lastUser)
			}
		}
		if got := strings.Join(seen[1].tools, ","); got != agentcapabilities.PatrolProposeActionToolName+","+agentcapabilities.PatrolConcludeInvestigationToolName &&
			got != agentcapabilities.PatrolConcludeInvestigationToolName+","+agentcapabilities.PatrolProposeActionToolName {
			t.Errorf("the answering turn offered %v, want only the proposal and conclusion tools", seen[1].tools)
		}
		if result.Proposal == nil || result.Proposal.CapabilityName != "start" {
			t.Fatalf("proposal = %+v, want the start the follow-up produced", result.Proposal)
		}
	})

	t.Run("already concluded", func(t *testing.T) {
		seen, _ := run(t, func(callback providers.StreamCallback) {
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{ToolCalls: []providers.ToolCall{{
				ID: "conclude-1", Name: agentcapabilities.PatrolConcludeInvestigationToolName,
				Input: map[string]interface{}{"outcome": "needs_attention", "summary": "Only a person can fix this."},
			}}}})
		}, []string{"start"}, 6)
		for _, request := range seen[1:] {
			if strings.Contains(request.lastUser, "Before you finish") {
				t.Fatalf("a run that recorded its conclusion was asked again: %q", request.lastUser)
			}
		}
	})

	t.Run("nothing Pulse can run", func(t *testing.T) {
		seen, result := run(t, endsInProse, nil, 6)
		if len(seen) != 1 {
			t.Fatalf("with no actions for the resource the model was asked %d times, want once", len(seen))
		}
		if result.Proposal != nil {
			t.Fatalf("unexpected proposal %+v", result.Proposal)
		}
	})

	// A slow model reaches the final turn through the deadline reserve. On
	// D1, qwen3:8b found that the web worker had died, was offered only the
	// conclusion tool, and wrote "restart the container" in prose. The final
	// turn now also offers a proposal when Pulse can run an action.
	gathersEvidence := func(callback providers.StreamCallback) {
		callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{ToolCalls: []providers.ToolCall{{
			ID: "evidence-1", Name: agentcapabilities.PulseQueryToolName, Input: map[string]interface{}{"action": "health"},
		}}}})
	}
	t.Run("final turn offers a proposal", func(t *testing.T) {
		seen, result := run(t, gathersEvidence, []string{"start", "restart"}, 2)
		if len(seen) < 2 {
			t.Fatalf("the model was asked %d times, want a final turn", len(seen))
		}
		final := seen[1]
		if !strings.Contains(strings.Join(final.tools, ","), agentcapabilities.PatrolProposeActionToolName) ||
			!strings.Contains(strings.Join(final.tools, ","), agentcapabilities.PatrolConcludeInvestigationToolName) {
			t.Errorf("final turn offered %v, want the proposal and conclusion tools", final.tools)
		}
		for _, want := range []string{"This is the investigation's final turn.", "Pulse can run these actions for vm:42: start, restart."} {
			if !strings.Contains(final.system, want) {
				t.Errorf("final-turn system note lacks %q", want)
			}
		}
		if result.Proposal == nil || result.Proposal.CapabilityName != "start" {
			t.Fatalf("proposal = %+v, want the start made on the final turn", result.Proposal)
		}
	})

	t.Run("final turn with nothing Pulse can run", func(t *testing.T) {
		seen, _ := run(t, gathersEvidence, nil, 2)
		if len(seen) < 2 {
			t.Fatalf("the model was asked %d times, want a final turn", len(seen))
		}
		if strings.Contains(strings.Join(seen[1].tools, ","), agentcapabilities.PatrolProposeActionToolName) {
			t.Errorf("with no actions and evidence left, the final turn offered a proposal: %v", seen[1].tools)
		}
	})
}
