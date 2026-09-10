package tools

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Projection requires planning availability but must not invoke the planner.
func projectionTestPlanner(t *testing.T) TypedActionPlanner {
	t.Helper()
	return typedActionPlannerFunc(func(context.Context, string, unifiedresources.ActionRequest) (*unifiedresources.ActionPlan, error) {
		t.Fatal("tool projection must not plan or execute")
		return nil, nil
	})
}

func providerToolNameSet(list []agentcapabilities.ProviderTool) map[string]bool {
	set := make(map[string]bool, len(list))
	for _, tool := range list {
		set[tool.Name] = true
	}
	return set
}

func providerToolByName(list []agentcapabilities.ProviderTool, name string) (agentcapabilities.ProviderTool, bool) {
	for _, tool := range list {
		if tool.Name == name {
			return tool, true
		}
	}
	return agentcapabilities.ProviderTool{}, false
}

func TestPulseToolExecutorAssistantProviderToolsUsesRuntimeAvailability(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{
		ReadState:          &fakeReadState{},
		TypedActionPlanner: projectionTestPlanner(t),
		AgentServer:        &mockAgentServer{agents: []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "node-1"}}},
		ControlLevel:       ControlLevelControlled,
	})

	projected := exec.AssistantProviderTools(agentcapabilities.AssistantProviderToolOptions{IncludeQuestionTool: true})
	names := providerToolNameSet(projected)

	for _, name := range []string{agentcapabilities.PulseQueryToolName, agentcapabilities.PulseReadToolName, agentcapabilities.PulseControlToolName, agentcapabilities.PulseQuestionToolName} {
		if !names[name] {
			t.Fatalf("AssistantProviderTools did not expose available tool %s; names=%v", name, names)
		}
	}
	if names[agentcapabilities.PulseFileEditToolName] {
		t.Fatal("AssistantProviderTools exposed retired file mutation tool")
	}
	for _, name := range []string{agentcapabilities.PulseDiscoveryToolName, agentcapabilities.PatrolReportFindingToolName, agentcapabilities.PatrolAssessFindingToolName, agentcapabilities.PatrolResolveFindingToolName, agentcapabilities.PatrolGetFindingsToolName} {
		if names[name] {
			t.Fatalf("AssistantProviderTools exposed unavailable tool %s; names=%v", name, names)
		}
	}

	queryTool, ok := providerToolByName(projected, agentcapabilities.PulseQueryToolName)
	if !ok {
		t.Fatalf("AssistantProviderTools did not expose %s", agentcapabilities.PulseQueryToolName)
	}
	if queryTool.PulseGovernance == nil || queryTool.PulseGovernance.ActionMode != agentcapabilities.ActionModeRead {
		t.Fatalf("%s provider governance = %+v, want read metadata", agentcapabilities.PulseQueryToolName, queryTool.PulseGovernance)
	}
	if queryTool.BehaviorHints == nil || queryTool.BehaviorHints.ReadOnlyHint == nil || !*queryTool.BehaviorHints.ReadOnlyHint {
		t.Fatalf("%s provider behavior hints = %+v, want read-only hint", agentcapabilities.PulseQueryToolName, queryTool.BehaviorHints)
	}

	controlTool, ok := providerToolByName(projected, agentcapabilities.PulseControlToolName)
	if !ok {
		t.Fatalf("AssistantProviderTools did not expose %s", agentcapabilities.PulseControlToolName)
	}
	if controlTool.PulseGovernance == nil || controlTool.PulseGovernance.ApprovalPolicy != agentcapabilities.ApprovalPolicyActionPlan {
		t.Fatalf("%s provider governance = %+v, want action-plan metadata", agentcapabilities.PulseControlToolName, controlTool.PulseGovernance)
	}
	if controlTool.BehaviorHints == nil || controlTool.BehaviorHints.DestructiveHint == nil || !*controlTool.BehaviorHints.DestructiveHint {
		t.Fatalf("%s provider behavior hints = %+v, want destructive hint", agentcapabilities.PulseControlToolName, controlTool.BehaviorHints)
	}
	if !strings.Contains(controlTool.Description, "Proxmox VM and LXC lifecycle actions do not require the QEMU guest agent") {
		t.Fatalf("%s provider description does not distinguish lifecycle actions from guest-agent operations: %q", agentcapabilities.PulseControlToolName, controlTool.Description)
	}
	properties, ok := controlTool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s provider properties = %#v, want object", agentcapabilities.PulseControlToolName, controlTool.InputSchema["properties"])
	}
	for _, retired := range []string{"guest_id", "command", "target_host", "run_on_host", "force"} {
		if _, exists := properties[retired]; exists {
			t.Fatalf("%s provider schema still exposes retired %q input", agentcapabilities.PulseControlToolName, retired)
		}
	}
	required, ok := controlTool.InputSchema["required"].([]string)
	if !ok {
		t.Fatalf("%s provider required fields = %#v, want array", agentcapabilities.PulseControlToolName, controlTool.InputSchema["required"])
	}
	for _, field := range []string{"type", "resource_id", "action"} {
		found := false
		for _, value := range required {
			if value == field {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s provider schema required fields = %#v, missing %q", agentcapabilities.PulseControlToolName, required, field)
		}
	}
}

func TestPulseToolExecutorAssistantProviderToolsEnterThroughManifestSurfaceAffordances(t *testing.T) {
	source, err := os.ReadFile("provider_projection.go")
	if err != nil {
		t.Fatalf("read provider projection source: %v", err)
	}
	text := string(source)
	for _, required := range []string{
		"agentcapabilities.ProjectPulseAssistantProviderTools(agentcapabilities.CanonicalManifest(), nil, nil, opts)",
		"agentcapabilities.ProjectPulseAssistantProviderTools(agentcapabilities.CanonicalManifest(), e.ListTools(), e.ListToolGovernance(), opts)",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("Assistant provider tools must enter through manifest surface affordances; missing %s", required)
		}
	}
	for _, forbidden := range []string{
		"agentcapabilities.ProjectAssistantProviderTools(nil, nil, opts)",
		"agentcapabilities.ProjectAssistantProviderTools(e.ListTools(), e.ListToolGovernance(), opts)",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("Assistant provider tools bypass manifest surface affordances: found %s", forbidden)
		}
	}
}

func TestPulseToolExecutorAssistantProviderToolsHonorsInteractionMode(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{ReadState: &fakeReadState{}, ControlLevel: ControlLevelControlled})

	interactive := providerToolNameSet(exec.AssistantProviderTools(agentcapabilities.AssistantProviderToolOptions{IncludeQuestionTool: true}))
	if !interactive[agentcapabilities.PulseQuestionToolName] {
		t.Fatal("interactive Assistant provider surface must include the question tool")
	}

	nonInteractive := providerToolNameSet(exec.AssistantProviderTools(agentcapabilities.AssistantProviderToolOptions{}))
	if nonInteractive[agentcapabilities.PulseQuestionToolName] {
		t.Fatal("non-interactive Assistant provider surface must not include the question tool")
	}
}

func TestPulseToolExecutorAssistantProviderToolsHonorsControlLevel(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{
		ReadState:          &fakeReadState{},
		TypedActionPlanner: projectionTestPlanner(t),
		AgentServer:        &mockAgentServer{agents: []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "node-1"}}},
		ControlLevel:       ControlLevelReadOnly,
	})

	names := providerToolNameSet(exec.AssistantProviderTools(agentcapabilities.AssistantProviderToolOptions{}))
	if names[agentcapabilities.PulseControlToolName] || names[agentcapabilities.PulseFileEditToolName] {
		t.Fatalf("read-only Assistant provider surface exposed control tools; names=%v", names)
	}
	if !names[agentcapabilities.PulseReadToolName] {
		t.Fatalf("read-only Assistant provider surface should still expose %s; names=%v", agentcapabilities.PulseReadToolName, names)
	}
}

func TestPulseToolExecutorAssistantSurfaceToolContractUsesRuntimeProjection(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{
		ReadState:          &fakeReadState{},
		TypedActionPlanner: projectionTestPlanner(t),
		AgentServer:        &mockAgentServer{agents: []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "node-1"}}},
		ControlLevel:       ControlLevelControlled,
	})

	contract := exec.AssistantSurfaceToolContract(agentcapabilities.AssistantProviderToolOptions{IncludeQuestionTool: true})
	names := map[string]bool{}
	for _, name := range contract.ToolNames {
		names[name] = true
	}
	registryNames := map[string]bool{}
	for _, name := range contract.RegistryToolNames {
		registryNames[name] = true
	}
	nativeNames := map[string]bool{}
	for _, name := range contract.NativeToolNames {
		nativeNames[name] = true
	}

	if contract.SurfaceID != agentcapabilities.SurfaceIDPulseAssistant {
		t.Fatalf("Assistant surface id = %q", contract.SurfaceID)
	}
	if contract.ToolSource != agentcapabilities.SurfaceToolSourceAssistantRegistry {
		t.Fatalf("Assistant surface tool source = %q", contract.ToolSource)
	}
	for _, name := range []string{agentcapabilities.PulseQueryToolName, agentcapabilities.PulseReadToolName, agentcapabilities.PulseControlToolName, agentcapabilities.PulseQuestionToolName} {
		if !names[name] {
			t.Fatalf("Assistant surface contract missing %s; names=%v", name, names)
		}
	}
	for _, name := range []string{agentcapabilities.PulseQueryToolName, agentcapabilities.PulseReadToolName, agentcapabilities.PulseControlToolName} {
		if !registryNames[name] {
			t.Fatalf("Assistant registry contract missing %s; names=%v", name, registryNames)
		}
	}
	if !nativeNames[agentcapabilities.PulseQuestionToolName] {
		t.Fatalf("Assistant native contract missing question tool; names=%v", nativeNames)
	}
	if len(contract.CapabilityNames) != 0 {
		t.Fatalf("Assistant surface contract must not duplicate MCP capability names: %+v", contract.CapabilityNames)
	}
}

// Retired execution transports alone must not advertise canonical planning on
// either provider-facing projection, even when execution authority is enabled.
func TestAssistantProjectionsRequireTypedPlanner(t *testing.T) {
	for _, tc := range []struct {
		name    string
		planner TypedActionPlanner
		want    bool
	}{
		{name: "transport_only"},
		{name: "typed_planner", planner: projectionTestPlanner(t), want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exec := NewPulseToolExecutor(ExecutorConfig{
				ReadState:          &fakeReadState{},
				AgentServer:        &mockAgentServer{},
				TypedActionPlanner: tc.planner,
				ControlLevel:       ControlLevelControlled,
			})
			opts := agentcapabilities.AssistantProviderToolOptions{}
			names := providerToolNameSet(exec.AssistantProviderTools(opts))
			contract := exec.AssistantSurfaceToolContract(opts)
			found := false
			for _, name := range contract.ToolNames {
				found = found || name == agentcapabilities.PulseControlToolName
			}
			if names[agentcapabilities.PulseControlToolName] != tc.want || found != tc.want {
				t.Fatalf("provider control=%v surface control=%v want=%v", names[agentcapabilities.PulseControlToolName], found, tc.want)
			}
		})
	}
}
