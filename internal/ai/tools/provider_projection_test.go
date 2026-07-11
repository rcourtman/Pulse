package tools

import (
	"os"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

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
		ReadState:    &fakeReadState{},
		AgentServer:  &mockAgentServer{agents: []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "node-1"}}},
		ControlLevel: ControlLevelControlled,
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
	for _, name := range []string{agentcapabilities.PulseDiscoveryToolName, agentcapabilities.PatrolReportFindingToolName, agentcapabilities.PatrolResolveFindingToolName, agentcapabilities.PatrolGetFindingsToolName} {
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
		ReadState:    &fakeReadState{},
		AgentServer:  &mockAgentServer{agents: []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "node-1"}}},
		ControlLevel: ControlLevelReadOnly,
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
		ReadState:    &fakeReadState{},
		AgentServer:  &mockAgentServer{agents: []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "node-1"}}},
		ControlLevel: ControlLevelControlled,
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
