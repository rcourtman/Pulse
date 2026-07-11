package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	ur "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type fakeStateProvider struct{}

func (f fakeStateProvider) ReadSnapshot() models.StateSnapshot {
	return models.StateSnapshot{}
}

type fakeAgentServer struct{}

func (f fakeAgentServer) GetConnectedAgents() []agentexec.ConnectedAgent {
	return []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "node-1"}}
}

func (f fakeAgentServer) ExecuteCommand(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
	return &agentexec.CommandResultPayload{Stdout: "ok", ExitCode: 0}, nil
}

type fakeCanonicalReadState struct{}

func (f *fakeCanonicalReadState) VMs() []*ur.VMView                           { return nil }
func (f *fakeCanonicalReadState) Containers() []*ur.ContainerView             { return nil }
func (f *fakeCanonicalReadState) Nodes() []*ur.NodeView                       { return nil }
func (f *fakeCanonicalReadState) Hosts() []*ur.HostView                       { return nil }
func (f *fakeCanonicalReadState) DockerHosts() []*ur.DockerHostView           { return nil }
func (f *fakeCanonicalReadState) DockerContainers() []*ur.DockerContainerView { return nil }
func (f *fakeCanonicalReadState) StoragePools() []*ur.StoragePoolView         { return nil }
func (f *fakeCanonicalReadState) PhysicalDisks() []*ur.PhysicalDiskView       { return nil }
func (f *fakeCanonicalReadState) PBSInstances() []*ur.PBSInstanceView         { return nil }
func (f *fakeCanonicalReadState) PMGInstances() []*ur.PMGInstanceView         { return nil }
func (f *fakeCanonicalReadState) K8sClusters() []*ur.K8sClusterView           { return nil }
func (f *fakeCanonicalReadState) K8sNodes() []*ur.K8sNodeView                 { return nil }
func (f *fakeCanonicalReadState) Pods() []*ur.PodView                         { return nil }
func (f *fakeCanonicalReadState) K8sDeployments() []*ur.K8sDeploymentView     { return nil }
func (f *fakeCanonicalReadState) Workloads() []*ur.WorkloadView               { return nil }
func (f *fakeCanonicalReadState) Infrastructure() []*ur.InfrastructureView    { return nil }

type fakeRecoveryPointsProvider struct {
	points []recovery.RecoveryPoint
}

func (f *fakeRecoveryPointsProvider) ListPoints(_ context.Context, opts recovery.ListPointsOptions) ([]recovery.RecoveryPoint, int, error) {
	filtered := make([]recovery.RecoveryPoint, 0, len(f.points))
	for _, point := range f.points {
		if opts.Provider != "" && point.Provider != opts.Provider {
			continue
		}
		if opts.Kind != "" && point.Kind != opts.Kind {
			continue
		}
		filtered = append(filtered, point)
	}

	total := len(filtered)
	if opts.Limit <= 0 {
		return filtered, total, nil
	}
	page := opts.Page
	if page <= 0 {
		page = 1
	}
	start := (page - 1) * opts.Limit
	if start >= total {
		return []recovery.RecoveryPoint{}, total, nil
	}
	end := start + opts.Limit
	if end > total {
		end = total
	}
	return filtered[start:end], total, nil
}

func toolNameSet(list []providers.Tool) map[string]bool {
	set := make(map[string]bool, len(list))
	for _, tool := range list {
		set[tool.Name] = true
	}
	return set
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestToolsForExecutionMode_InteractiveExposesGovernedTools(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: fakeStateProvider{},
		AgentServer:   fakeAgentServer{},
		ReadState:     &fakeCanonicalReadState{},
		ControlLevel:  tools.ControlLevelControlled,
	})

	svc := &Service{
		executor: exec,
		cfg: &config.AIConfig{
			PatrolAnalyzeDocker:  true,
			PatrolAnalyzeStorage: true,
		},
	}

	interactiveTools := svc.toolsForExecutor(nil, false)
	interactiveSet := toolNameSet(interactiveTools)
	if !interactiveSet["pulse_kubernetes"] ||
		!interactiveSet["pulse_storage"] ||
		!interactiveSet["pulse_pmg"] ||
		!interactiveSet["pulse_docker"] {
		t.Fatalf("expected interactive chat to expose all governed tools to the selected model")
	}
	if !interactiveSet[pulseQuestionToolName] {
		t.Fatalf("expected interactive chat to expose the clarification tool")
	}
}

func TestService_ExecuteStream_ToolManifestIsModelOwned(t *testing.T) {
	// Tool selection is model-owned: every interactive turn that reaches the
	// selected model carries the same full governed manifest, regardless of
	// prompt wording. These prompts were previously keyword-scoped to
	// text-only or query-only manifests (greetings, exact-reply diagnostics,
	// "before using tools" phrasing, inventory breakdown wording, short
	// lookups) — the model now sees the same tools as diagnostic/action
	// prompts and decides for itself whether to use them.
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	var toolCounts []int
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			toolCounts = append(toolCounts, len(req.Tools))
			callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "ok"}})
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1}})
			return nil
		},
	}

	service := NewService(Config{
		AIConfig:      &config.AIConfig{ControlLevel: config.ControlLevelControlled},
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
	})
	service.sessions = store
	service.provider = provider
	service.started = true

	prompts := []string{
		"hi",
		"thanks",
		"Reply exactly: PULSE_CHAT_OK",
		"Before using any tools, tell me your plan.",
		"give me a detailed inventory breakdown by node",
		"hows esphome",
		"Alerts count",
		"show me the logs for vm 100",
	}
	for i, prompt := range prompts {
		if err := service.ExecuteStream(context.Background(), ExecuteRequest{
			SessionID: fmt.Sprintf("model-owned-manifest-%d", i),
			Prompt:    prompt,
		}, func(StreamEvent) {}); err != nil {
			t.Fatalf("ExecuteStream(%q) failed: %v", prompt, err)
		}
	}

	if len(toolCounts) != len(prompts) {
		t.Fatalf("toolCounts = %#v, want %d provider calls", toolCounts, len(prompts))
	}
	for i, count := range toolCounts {
		if count == 0 {
			t.Fatalf("prompt %q reached the model with no tools; the manifest must be model-owned", prompts[i])
		}
		if count != toolCounts[0] {
			t.Fatalf("prompt %q got %d tools, want the same governed manifest (%d) for every turn", prompts[i], count, toolCounts[0])
		}
	}
}

func TestAssistantPromptQualifiesForLocalInventoryCount(t *testing.T) {
	// The deterministic count-only inventory shortcut is the single surviving
	// prompt-classified path, and it only answers locally — it never decides
	// what tools the model sees. The gate must stay conservative: any hint of
	// intent beyond a pure count sends the turn to the model.
	tests := []struct {
		prompt string
		want   bool
	}{
		{"how many devices in this", true},
		{"how many vms do i have", true},
		{"container count", true},
		// Qualified counts must reach the model with full tools.
		{"how many vms have errors", false},
		{"how many containers are using high cpu", false},
		// Explicit operator tool intent is the contract escape hatch.
		{"Use read-only tools only. On node delly, count entries in /dev with `ls /dev | wc -l`; answer with the number only.", false},
		// Detail/breakdown prompts are not count-only.
		{"give me a detailed inventory breakdown by node", false},
		// Not inventory counts at all.
		{"hows esphome", false},
		{"hi", false},
		{"Alerts count", false},
	}
	for _, tt := range tests {
		normalized := normalizeAssistantToolRoutingPrompt(tt.prompt)
		if got := assistantPromptQualifiesForLocalInventoryCount(normalized); got != tt.want {
			t.Fatalf("assistantPromptQualifiesForLocalInventoryCount(%q) = %v, want %v", tt.prompt, got, tt.want)
		}
	}
}

func TestAssistantSurfaceToolContractUsesRuntimeAssistantProjection(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: fakeStateProvider{},
		AgentServer:   fakeAgentServer{},
		ReadState:     &fakeCanonicalReadState{},
		ControlLevel:  tools.ControlLevelControlled,
	})
	svc := &Service{
		executor: exec,
		cfg:      &config.AIConfig{ControlLevel: config.ControlLevelControlled},
	}

	interactive := svc.AssistantSurfaceToolContract(context.Background())
	if interactive.SurfaceID != agentcapabilities.SurfaceIDPulseAssistant {
		t.Fatalf("surface id = %q", interactive.SurfaceID)
	}
	if interactive.ToolSource != agentcapabilities.SurfaceToolSourceAssistantRegistry {
		t.Fatalf("tool source = %q", interactive.ToolSource)
	}
	if !stringSliceContains(interactive.ToolNames, agentcapabilities.PulseQueryToolName) {
		t.Fatalf("interactive surface missing registry query tool: %#v", interactive.ToolNames)
	}
	if !stringSliceContains(interactive.RegistryToolNames, agentcapabilities.PulseQueryToolName) {
		t.Fatalf("interactive surface missing registry bucket: %#v", interactive.RegistryToolNames)
	}
	if !stringSliceContains(interactive.NativeToolNames, agentcapabilities.PulseQuestionToolName) {
		t.Fatalf("interactive surface missing native question bucket: %#v", interactive.NativeToolNames)
	}
	if len(interactive.CapabilityNames) != 0 {
		t.Fatalf("Assistant surface must not expose MCP capability names: %#v", interactive.CapabilityNames)
	}

	svc.SetAutonomousMode(true)
	autonomous := svc.AssistantSurfaceToolContract(context.Background())
	if stringSliceContains(autonomous.NativeToolNames, agentcapabilities.PulseQuestionToolName) ||
		stringSliceContains(autonomous.ToolNames, agentcapabilities.PulseQuestionToolName) {
		t.Fatalf("autonomous Assistant surface must not expose interactive question tool: %+v", autonomous)
	}
	if !stringSliceContains(autonomous.RegistryToolNames, agentcapabilities.PulseQueryToolName) {
		t.Fatalf("autonomous surface should retain registry tools: %#v", autonomous.RegistryToolNames)
	}
}

func TestToolsForExecutionMode_AutonomousNonPatrolExposesGovernedTools(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: fakeStateProvider{},
		AgentServer:   fakeAgentServer{},
		ControlLevel:  tools.ControlLevelControlled,
	})

	svc := &Service{executor: exec}
	toolsList := svc.toolsForExecutor(nil, true)
	set := toolNameSet(toolsList)
	if !set["pulse_storage"] {
		t.Fatalf("expected storage tool to be exposed without prompt keyword routing")
	}
	if !set["pulse_control"] || !set["pulse_file_edit"] || !set["pulse_docker"] {
		t.Fatalf("expected autonomous non-patrol mode to expose governed write tools")
	}
	if set[pulseQuestionToolName] {
		t.Fatalf("expected autonomous mode to exclude the interactive clarification tool")
	}
}

func TestBuildSystemPrompt_DoesNotClaimGenericVMControl(t *testing.T) {
	svc := &Service{}

	prompt := svc.buildSystemPrompt()

	if !strings.Contains(prompt, "You are Pulse Assistant, the first-party in-app Pulse Intelligence surface") {
		t.Fatalf("expected system prompt to name the native Assistant surface, got %q", prompt)
	}
	if strings.Contains(prompt, "You are Pulse AI") {
		t.Fatalf("system prompt must not use legacy Pulse AI surface identity, got %q", prompt)
	}
	if strings.Contains(prompt, "Run commands on hosts, containers, and VMs") {
		t.Fatalf("expected system prompt to avoid blanket VM control claim, got %q", prompt)
	}
	if !strings.Contains(prompt, "Not every VM or container supports control") {
		t.Fatalf("expected system prompt to call out read-only resource platforms, got %q", prompt)
	}
	if !strings.Contains(prompt, "control only resources that explicitly support shared Pulse actions") {
		t.Fatalf("expected system prompt to describe capability-bound pulse_control usage, got %q", prompt)
	}
	if !strings.Contains(prompt, "## AVAILABLE TOOL GOVERNANCE") {
		t.Fatalf("expected system prompt to include generated tool governance section, got %q", prompt)
	}
	if !strings.Contains(prompt, "pulse_kubernetes") {
		t.Fatalf("expected system prompt to include the governed Kubernetes tool contract, got %q", prompt)
	}
	if !strings.Contains(prompt, "Do not use emoji, warning icons, or decorative symbols") {
		t.Fatalf("expected system prompt to keep Assistant formatting operational, got %q", prompt)
	}
}

func TestBuildSystemPrompt_IncludesProvenanceGuidance(t *testing.T) {
	svc := &Service{}

	prompt := svc.buildSystemPrompt()

	for _, expected := range []string{
		"## GROUNDING & PROVENANCE",
		"attribute it briefly so the user can trust and verify it",
		"Do not present stale or cached context as current",
		"Keep attribution concise and inline",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected provenance guidance %q in system prompt, got %q", expected, prompt)
		}
	}
}

func TestBuildSystemPrompt_CurrentResourceRequiresResourceHandoff(t *testing.T) {
	svc := &Service{}

	prompt := svc.buildSystemPrompt()

	for _, expected := range []string{
		"The placeholder current_resource is valid only when this turn includes Pulse resource context",
		"either from a resource-context handoff or from Pulse backend resource-reference resolution",
		"If no attached resource context is present, do not use target_host=\"current_resource\" or resource_id=\"current_resource\"",
		// Resolve-before-asking: the Assistant must try to identify the target
		// with read-only tools and proceed against a sole plausible match for
		// read-only diagnostics, instead of deflecting every "run X" request
		// back to the user. Placeholder targets remain forbidden.
		"Resolve a missing target yourself before asking",
		"run read-only diagnostics against it and name the target in your answer",
		"Ask for the target only when several plausible targets remain after looking, or when the action changes state",
		"Never guess a target you did not resolve",
		"Do not attempt a tool call with current_resource or another placeholder",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected current_resource boundary %q in system prompt, got %q", expected, prompt)
		}
	}
}

func TestBuildSystemPrompt_IncludesSharedPulseIntelligenceOperatingInstructions(t *testing.T) {
	svc := &Service{}

	prompt := svc.buildSystemPrompt()
	shared := agentcapabilities.BuildPulseAssistantOperatingInstructions()

	if !strings.Contains(prompt, shared) {
		t.Fatalf("expected Assistant system prompt to include shared Pulse Intelligence operating instructions")
	}
	if strings.Contains(prompt, "resources, prompts") || strings.Contains(prompt, "capability metadata as the source of truth") {
		t.Fatalf("Assistant system prompt must not advertise MCP-only operating affordances, got %q", prompt)
	}
}

func TestBuildToolGovernancePromptSection_FallbackDiscoveryMatchesRunContract(t *testing.T) {
	svc := &Service{}

	prompt := svc.buildToolGovernancePromptSection()

	// Discovery's run subaction is read-only evidence collection that
	// updates only the discovery cache, so the mutation-derived action
	// mode is read (the pre-descriptor manifest declared it mixed).
	if !strings.Contains(prompt, "pulse_discovery: mode=read") {
		t.Fatalf("expected fallback governance to classify pulse_discovery as read, got %q", prompt)
	}
	if !strings.Contains(prompt, "run uses read-only evidence collection and updates the discovery cache") {
		t.Fatalf("expected fallback governance to describe discovery refresh behavior, got %q", prompt)
	}
}

func TestBuildToolGovernancePromptSection_FallbackUsesCanonicalRegistry(t *testing.T) {
	fallback := fallbackAssistantToolGovernance()
	canonical := tools.CanonicalToolGovernanceForManifestSurface(
		tools.ControlLevelControlled,
		agentcapabilities.CanonicalManifest(),
		agentcapabilities.SurfaceIDPulseAssistant,
	)

	if len(fallback) != len(canonical) {
		t.Fatalf("fallback governance length = %d, want canonical registry length %d", len(fallback), len(canonical))
	}
	for i := range canonical {
		if fallback[i] != canonical[i] {
			t.Fatalf("fallback governance[%d] = %#v, want canonical registry descriptor %#v", i, fallback[i], canonical[i])
		}
	}

	prompt := (&Service{}).buildToolGovernancePromptSection()
	for _, tool := range canonical {
		if !strings.Contains(prompt, tool.Name+": mode="+string(tool.ActionMode)) {
			t.Fatalf("expected fallback prompt to include canonical governance for %s, got %q", tool.Name, prompt)
		}
	}
	if strings.Contains(prompt, "patrol_report_finding") ||
		strings.Contains(prompt, "patrol_resolve_finding") ||
		strings.Contains(prompt, "patrol_get_findings") {
		t.Fatalf("expected Assistant fallback prompt to exclude Patrol-only tools, got %q", prompt)
	}
}

func TestBuildToolGovernancePromptSection_FallbackOfferedNamesUseAssistantAffordances(t *testing.T) {
	fallback := fallbackAssistantToolGovernance()
	names := fallbackAssistantToolGovernanceOfferedNames(fallback)

	if len(names) != len(fallback)+1 {
		t.Fatalf("fallback offered names length = %d, want governance names plus question tool", len(names))
	}
	for i, tool := range fallback {
		if names[i] != tool.Name {
			t.Fatalf("fallback offered names[%d] = %q, want %q", i, names[i], tool.Name)
		}
	}
	if names[len(names)-1] != agentcapabilities.PulseQuestionToolName {
		t.Fatalf("fallback offered names must include manifest-enabled question tool last, got %#v", names)
	}
}

func TestBuildToolGovernancePromptSection_OfferedToolsUseCanonicalFallback(t *testing.T) {
	svc := &Service{}

	prompt := svc.buildToolGovernancePromptSectionForOfferedTools([]providers.Tool{{Name: "pulse_discovery"}})

	if !strings.Contains(prompt, "pulse_discovery: mode=read; approval=scope_only (no approval required; run uses read-only evidence collection and updates the discovery cache)") {
		t.Fatalf("expected offered fallback prompt to use canonical discovery governance, got %q", prompt)
	}
	if strings.Contains(prompt, "pulse_control:") {
		t.Fatalf("expected offered fallback prompt to exclude non-offered control tools, got %q", prompt)
	}
}

func TestBuildToolGovernancePromptSection_OfferedToolsUseProviderMetadata(t *testing.T) {
	svc := &Service{}

	prompt := svc.buildToolGovernancePromptSectionForOfferedTools([]providers.Tool{
		{
			Name: "pulse_discovery",
			PulseGovernance: &agentcapabilities.ToolGovernanceDescriptor{
				Name:            "pulse_discovery",
				Description:     "Discover infrastructure.",
				ActionMode:      agentcapabilities.ActionModeMixed,
				ApprovalPolicy:  agentcapabilities.ApprovalPolicyScopeOnly,
				ApprovalSummary: "metadata-owned approval summary",
				Summary:         "metadata-owned discovery summary",
			},
		},
	})

	if !strings.Contains(prompt, "pulse_discovery: mode=mixed; approval=scope_only (metadata-owned approval summary); metadata-owned discovery summary") {
		t.Fatalf("expected offered tool metadata to drive governance prompt, got %q", prompt)
	}
	if strings.Contains(prompt, "run uses read-only evidence collection and updates the discovery cache") {
		t.Fatalf("expected offered metadata to avoid executor fallback governance, got %q", prompt)
	}
}

func TestBuildToolGovernancePromptSection_OfferedQuestionToolStaysInteractive(t *testing.T) {
	svc := &Service{}

	prompt := svc.buildToolGovernancePromptSectionForOfferedTools([]providers.Tool{{Name: pulseQuestionToolName}})

	if !strings.Contains(prompt, agentcapabilities.PulseQuestionToolGovernancePromptLine()) {
		t.Fatalf("expected Assistant question tool to keep interactive governance, got %q", prompt)
	}
	if strings.Contains(prompt, "pulse_question: mode=read") {
		t.Fatalf("expected Assistant question tool not to use generic fallback governance, got %q", prompt)
	}
	if strings.Contains(prompt, "No Pulse tools are offered") {
		t.Fatalf("expected question-only manifest to be described as an offered interactive tool, got %q", prompt)
	}
}

func TestBuildToolGovernancePromptSection_NoOfferedToolsUsesSharedNoToolsGuidance(t *testing.T) {
	svc := &Service{}

	prompt := svc.buildToolGovernancePromptSectionForOfferedTools([]providers.Tool{})

	if !strings.Contains(prompt, "No Pulse tools are offered for this turn") {
		t.Fatalf("expected no-tools guidance, got %q", prompt)
	}
	if strings.Contains(prompt, "pulse_question:") {
		t.Fatalf("expected no Assistant question tool when no tools are offered, got %q", prompt)
	}
}

func TestToolsForExecutionMode_RecoveryOnlyKeepsStorage(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		RecoveryPointsProvider: &fakeRecoveryPointsProvider{points: []recovery.RecoveryPoint{{
			ID:       "pve-snapshot:before-upgrade",
			Provider: recovery.ProviderProxmoxPVE,
			Kind:     recovery.KindSnapshot,
			Mode:     recovery.ModeLocal,
			Outcome:  recovery.OutcomeSuccess,
		}}},
		ControlLevel: tools.ControlLevelControlled,
	})

	svc := &Service{executor: exec}
	toolsList := svc.toolsForExecutor(nil, false)
	set := toolNameSet(toolsList)
	if !set["pulse_storage"] {
		t.Fatalf("expected storage tool to be kept when recovery points are the only storage source")
	}
}

func TestToolsForExecutionMode_PatrolScopeUsesConfigNotPrompt(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: fakeStateProvider{},
		AgentServer:   fakeAgentServer{},
		ControlLevel:  tools.ControlLevelControlled,
	})

	svc := &Service{
		executor: exec,
		cfg: &config.AIConfig{
			PatrolAnalyzeDocker:  false,
			PatrolAnalyzeStorage: false,
		},
	}

	exec.ApplyExecutionProfile(tools.ProfilePatrolDetection)
	filtered := svc.toolsForExecutor(exec, false)
	set := toolNameSet(filtered)

	if set["pulse_docker"] {
		t.Fatalf("expected pulse_docker to follow disabled Patrol subsystem scope")
	}
	if set["pulse_storage"] {
		t.Fatalf("expected pulse_storage to follow disabled Patrol subsystem scope")
	}
	if !set["pulse_query"] {
		t.Fatalf("expected core read/query tools to remain available to the Patrol model")
	}
}

func testRunCommandToolDefinition() tools.Tool {
	return tools.Tool{
		Name: agentcapabilities.PulseRunCommandToolName,
		InputSchema: tools.InputSchema{Properties: map[string]tools.PropertySchema{
			"command":     {Type: "string"},
			"run_on_host": {Type: "boolean"},
			"target_host": {Type: "string"},
		}},
	}
}

func TestExecuteCommand_SuccessAndExitCode(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	exec.RegisterTool(tools.RegisteredTool{
		Invocation: tools.StaticInvocation(agentcapabilities.ToolCallKindWrite, agentcapabilities.MutationPulseState),
		Definition: testRunCommandToolDefinition(),
		Handler: func(ctx context.Context, exec *tools.PulseToolExecutor, args map[string]interface{}) (tools.CallToolResult, error) {
			return tools.NewTextResult("Command failed (exit code 7): boom"), nil
		},
	})

	svc := &Service{executor: exec}

	output, code, err := svc.ExecuteCommand(context.Background(), "uptime", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 7 {
		t.Fatalf("expected exit code 7, got %d", code)
	}
	if !strings.Contains(output, "Command failed") {
		t.Fatalf("expected command output, got: %s", output)
	}
}

func TestExecuteCommand_ErrorAndApprovalPaths(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	exec.RegisterTool(tools.RegisteredTool{
		Invocation: tools.StaticInvocation(agentcapabilities.ToolCallKindWrite, agentcapabilities.MutationPulseState),
		Definition: testRunCommandToolDefinition(),
		Handler: func(ctx context.Context, exec *tools.PulseToolExecutor, args map[string]interface{}) (tools.CallToolResult, error) {
			return tools.NewErrorResult(context.Canceled), nil
		},
	})

	svc := &Service{executor: exec}

	_, code, err := svc.ExecuteCommand(context.Background(), "uptime", "")
	if err == nil || code != 1 {
		t.Fatalf("expected error with exit code 1")
	}

	// Registry entries are append-only, so the approval scenario uses a
	// fresh executor instead of swapping the handler in place.
	approvalExec := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	approvalExec.RegisterTool(tools.RegisteredTool{
		Invocation: tools.StaticInvocation(agentcapabilities.ToolCallKindWrite, agentcapabilities.MutationPulseState),
		Definition: testRunCommandToolDefinition(),
		Handler: func(ctx context.Context, exec *tools.PulseToolExecutor, args map[string]interface{}) (tools.CallToolResult, error) {
			return tools.NewTextResult("APPROVAL_REQUIRED: requires approval"), nil
		},
	})
	svc = &Service{executor: approvalExec}

	_, _, err = svc.ExecuteCommand(context.Background(), "uptime", "")
	if err == nil {
		t.Fatalf("expected approval error")
	}
}

func TestExecuteCommandUsesSharedResultTextProjection(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	exec.RegisterTool(tools.RegisteredTool{
		Invocation: tools.StaticInvocation(agentcapabilities.ToolCallKindWrite, agentcapabilities.MutationPulseState),
		Definition: testRunCommandToolDefinition(),
		Handler: func(ctx context.Context, exec *tools.PulseToolExecutor, args map[string]interface{}) (tools.CallToolResult, error) {
			return tools.CallToolResult{
				Content: []tools.Content{
					{Type: "text", Text: "stdout"},
					{Type: "resource", URI: "file://ignored"},
					{Type: "text"},
					{Type: "text", Text: "stderr"},
				},
			}, nil
		},
	})

	svc := &Service{executor: exec}
	output, code, err := svc.ExecuteCommand(context.Background(), "uptime", "")
	if err != nil {
		t.Fatalf("ExecuteCommand: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if output != "stdout\nstderr" {
		t.Fatalf("output = %q, want shared result text projection", output)
	}
}

func TestExecuteAssistantTool_ErrorsAndSuccess(t *testing.T) {
	// Registry entries are append-only, so each handler scenario runs on
	// a fresh executor instead of swapping the handler in place.
	newService := func(handler func(ctx context.Context, exec *tools.PulseToolExecutor, args map[string]interface{}) (tools.CallToolResult, error)) *Service {
		exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
		exec.RegisterTool(tools.RegisteredTool{
			Invocation: tools.StaticInvocation(agentcapabilities.ToolCallKindWrite, agentcapabilities.MutationPulseState),
			Definition: tools.Tool{Name: "test_tool"},
			Handler:    handler,
		})
		return &Service{executor: exec}
	}

	svc := newService(func(ctx context.Context, exec *tools.PulseToolExecutor, args map[string]interface{}) (tools.CallToolResult, error) {
		return tools.NewErrorResult(context.DeadlineExceeded), nil
	})
	if _, err := svc.ExecuteAssistantTool(context.Background(), "test_tool", map[string]interface{}{}); err == nil {
		t.Fatalf("expected tool error")
	}

	svc = newService(func(ctx context.Context, exec *tools.PulseToolExecutor, args map[string]interface{}) (tools.CallToolResult, error) {
		return tools.NewTextResult("POLICY_BLOCKED: nope"), nil
	})
	if _, err := svc.ExecuteAssistantTool(context.Background(), "test_tool", map[string]interface{}{}); err == nil {
		t.Fatalf("expected policy blocked error")
	}

	svc = newService(func(ctx context.Context, exec *tools.PulseToolExecutor, args map[string]interface{}) (tools.CallToolResult, error) {
		return tools.NewTextResult("ok"), nil
	})
	output, err := svc.ExecuteAssistantTool(context.Background(), "test_tool", map[string]interface{}{})
	if err != nil || output != "ok" {
		t.Fatalf("expected success, got output=%q err=%v", output, err)
	}
}

func TestExecuteAssistantToolUsesSharedResultTextProjection(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	exec.RegisterTool(tools.RegisteredTool{
		Invocation: tools.StaticInvocation(agentcapabilities.ToolCallKindWrite, agentcapabilities.MutationPulseState),
		Definition: tools.Tool{Name: "test_tool"},
		Handler: func(ctx context.Context, exec *tools.PulseToolExecutor, args map[string]interface{}) (tools.CallToolResult, error) {
			return tools.CallToolResult{
				Content: []tools.Content{
					{Type: "text", Text: "first"},
					{Type: "resource", URI: "file://ignored"},
					{Type: "text"},
					{Type: "text", Text: "second"},
				},
			}, nil
		},
	})

	svc := &Service{executor: exec}
	output, err := svc.ExecuteAssistantTool(context.Background(), "test_tool", map[string]interface{}{})
	if err != nil {
		t.Fatalf("ExecuteAssistantTool: %v", err)
	}
	if output != "first\nsecond" {
		t.Fatalf("output = %q, want shared result text projection", output)
	}
}

func TestDirectToolExecutionUsesSharedResultExecutionMapping(t *testing.T) {
	src, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	text := string(src)

	if !strings.Contains(text, "agentcapabilities.InterpretDirectToolExecution(result") {
		t.Fatalf("direct tool execution must use shared result execution mapping")
	}
	for _, fragment := range []string{
		"func (s *Service) executeAssistantRegistryToolDirect(",
		`outcome, executionErr := s.executeAssistantRegistryToolDirect(ctx, agentcapabilities.PulseRunCommandToolName, args, agentcapabilities.DirectToolExecutionOptions{`,
		`outcome, executionErr := s.executeAssistantRegistryToolDirect(ctx, toolName, args, agentcapabilities.DirectToolExecutionOptions{`,
		`result, toolErr := executor.ExecuteTool(ctx, toolName, args)`,
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("direct Assistant registry execution must use shared service helper; missing %s", fragment)
		}
	}
	if !strings.Contains(text, "strings.TrimSpace(agentcapabilities.ToolResultText(result))") {
		t.Fatalf("direct topology context must use shared result text projection")
	}
	for _, fragment := range []string{
		"agentcapabilities.HasApprovalRequiredToolMarker(resultText)",
		"agentcapabilities.HasPolicyBlockedToolMarker(resultText)",
		"interpreted := agentcapabilities.InterpretToolResult(result)",
		"interpreted := agentcapabilities.InterpretMCPToolResult(result)",
		"if interpreted.IsError",
		"if interpreted.ApprovalRequired",
		"if interpreted.PolicyBlocked",
		"FormatToolResult(result)",
		"agentcapabilities.DirectMCPToolExecutionOptions",
		"agentcapabilities.InterpretDirectMCPToolExecution(result",
		"agentcapabilities.MCPToolResultText(result)",
	} {
		if strings.Contains(text, fragment) {
			t.Fatalf("direct tool execution must not duplicate shared result execution mapping; found %s", fragment)
		}
	}
}

func TestAgenticLoopUsesSharedProviderToolResultConstruction(t *testing.T) {
	src, err := os.ReadFile("agentic.go")
	if err != nil {
		t.Fatalf("read agentic.go: %v", err)
	}
	text := string(src)

	for _, fragment := range []string{
		"agentcapabilities.ParseApprovalRequiredToolMarkerData(resultText)",
		"inputWithApproval = agentcapabilities.WithApprovalArgument(inputWithApproval, approvalData.ApprovalID)",
		"agentcapabilities.NewProviderToolResultFromToolResult(tc.ID, result)",
		"projection := newProviderToolResultContextProjection(tc.ID, resultText, isError)",
		"ToolResult: &projection.Transcript",
		"ToolResult: &projection.Model",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("agentic loop must use shared provider tool-result construction; missing %s", fragment)
		}
	}
	if strings.Contains(text, "resultText = FormatToolResult(result)") {
		t.Fatalf("agentic loop must not flatten tool results into provider context locally")
	}
	if !strings.Contains(text, "toolCalls = agentcapabilities.NormalizeProviderToolCallsForExecution(data.ToolCalls)") {
		t.Fatalf("agentic loop must normalize provider-emitted tool calls through the shared tools/call projection before execution")
	}
	if strings.Contains(text, "toolCalls = data.ToolCalls") {
		t.Fatalf("agentic loop must not execute raw provider tool-call names or inputs")
	}
	for _, forbidden := range []string{
		"agentcapabilities.ApprovalRequiredToolMarkerPayloadJSON(resultText)",
		"var approvalData struct",
		"\n\t\t\t\t\t\t\tagentcapabilities.WithApprovalArgument(inputWithApproval, approvalData.ApprovalID)",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("agentic loop must use shared approval marker/replay helpers correctly; found %s", forbidden)
		}
	}
	for _, forbidden := range []string{
		"ToolResult: &providers.ToolResult{",
		"ToolResult: &ToolResult{",
		"agentcapabilities.NewProviderToolResultFromMCP(tc.ID, result)",
		"modelResultText := truncateToolResultForModel(resultText)",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("agentic loop must not assemble provider tool-result structs locally; found %s", forbidden)
		}
	}
}

func TestAgenticLoopUsesSharedVerificationEvidenceParser(t *testing.T) {
	agenticSrc, err := os.ReadFile("agentic.go")
	if err != nil {
		t.Fatalf("read agentic.go: %v", err)
	}
	if !strings.Contains(string(agenticSrc), "agentcapabilities.ToolResultHasVerificationOK(resultText)") {
		t.Fatalf("agentic loop must use the shared tool-result verification parser for write self-verification")
	}

	verificationSrc, err := os.ReadFile("agentic_verification.go")
	if err != nil {
		t.Fatalf("read agentic_verification.go: %v", err)
	}
	if strings.Contains(string(verificationSrc), "func toolResultHasVerificationOK(") {
		t.Fatalf("chat must not preserve a local verification evidence parser")
	}
}

func TestAgenticLoopUsesSharedToolResultErrorCodeParser(t *testing.T) {
	src, err := os.ReadFile("agentic.go")
	if err != nil {
		t.Fatalf("read agentic.go: %v", err)
	}
	text := string(src)
	if !strings.Contains(text, "agentcapabilities.ToolResultHasErrorCode(resultText, agentcapabilities.ErrCodeStrictResolution)") {
		t.Fatalf("agentic loop must use the shared tool-result error-code parser for strict-resolution recovery")
	}
	if strings.Contains(text, `strings.Contains(resultText, "STRICT_RESOLUTION")`) {
		t.Fatalf("agentic loop must not branch on local strict-resolution string matching")
	}
}

func TestProviderMessageConversionUsesSharedProviderToolResultConstruction(t *testing.T) {
	src, err := os.ReadFile("agentic_context.go")
	if err != nil {
		t.Fatalf("read agentic_context.go: %v", err)
	}
	text := string(src)

	for _, fragment := range []string{
		"projection := newProviderToolResultContextProjection(m.ToolResult.ToolUseID, m.ToolResult.Content, m.ToolResult.IsError)",
		"pm.ToolResult = &projection.Model",
		"agentcapabilities.NewProviderToolErrorResult(",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("provider message conversion must use shared provider tool-result construction; missing %s", fragment)
		}
	}
	for _, forbidden := range []string{
		"func newProviderToolResult(",
		"func newProviderToolErrorResult(",
		"ToolResult: &providers.ToolResult{",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("provider message conversion must not preserve Assistant-local provider-result wrappers or structs; found %s", forbidden)
		}
	}
}

func TestExecuteAssistantTool_PulseStorageSnapshotsToleratesMalformedRecoveryMetadata(t *testing.T) {
	completedAt := time.Date(2026, 2, 24, 10, 30, 0, 0, time.UTC)
	svc := &Service{
		executor: tools.NewPulseToolExecutor(tools.ExecutorConfig{
			StateProvider: fakeStateProvider{},
			ReadState:     &fakeCanonicalReadState{},
			RecoveryPointsProvider: &fakeRecoveryPointsProvider{points: []recovery.RecoveryPoint{
				{
					ID:       "pve-snapshot:snap-100-before-upgrade",
					Provider: recovery.ProviderProxmoxPVE,
					Kind:     recovery.KindSnapshot,
					Mode:     recovery.ModeLocal,
					Outcome:  recovery.OutcomeSuccess,
					SubjectRef: &recovery.ExternalRef{
						Type:      "vm",
						Name:      "100",
						ID:        "100",
						Namespace: "pve1",
						Class:     "node1",
					},
					Display: &recovery.RecoveryPointDisplay{
						SubjectLabel:   "100",
						ItemType:       "vm",
						ClusterLabel:   "pve1",
						NodeHostLabel:  "node1",
						EntityIDLabel:  "100",
						DetailsSummary: "before-upgrade",
					},
					CompletedAt: &completedAt,
				},
			}},
		}),
	}

	output, err := svc.ExecuteAssistantTool(context.Background(), "pulse_storage", map[string]interface{}{
		"type":     "snapshots",
		"guest_id": "100",
		"instance": "pve1",
	})
	if err != nil {
		t.Fatalf("ExecuteAssistantTool(pulse_storage snapshots): %v", err)
	}

	var resp tools.SnapshotsResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("unmarshal snapshots response: %v\noutput=%s", err, output)
	}
	if resp.Total != 1 || resp.Filtered != 1 || len(resp.Snapshots) != 1 {
		t.Fatalf("unexpected snapshots counts: total=%d filtered=%d len=%d", resp.Total, resp.Filtered, len(resp.Snapshots))
	}
	if resp.Snapshots[0].VMID != 100 || resp.Snapshots[0].Instance != "pve1" || resp.Snapshots[0].Node != "node1" {
		t.Fatalf("unexpected snapshot identity: %#v", resp.Snapshots[0])
	}
	if resp.Snapshots[0].Type != "vm" || resp.Snapshots[0].SnapshotName != "before-upgrade" {
		t.Fatalf("unexpected snapshot canonical fields: %#v", resp.Snapshots[0])
	}
}

func TestExecuteAssistantTool_PulseStorageBackupTasksToleratesMalformedRecoveryMetadata(t *testing.T) {
	startedAt := time.Date(2026, 2, 24, 11, 0, 0, 0, time.UTC)
	completedAt := time.Date(2026, 2, 24, 11, 15, 0, 0, time.UTC)
	svc := &Service{
		executor: tools.NewPulseToolExecutor(tools.ExecutorConfig{
			StateProvider: fakeStateProvider{},
			ReadState:     &fakeCanonicalReadState{},
			RecoveryPointsProvider: &fakeRecoveryPointsProvider{points: []recovery.RecoveryPoint{
				{
					ID:       "pve-task:task-101-backup",
					Provider: recovery.ProviderProxmoxPVE,
					Kind:     recovery.KindBackup,
					Mode:     recovery.ModeLocal,
					Outcome:  recovery.OutcomeSuccess,
					SubjectRef: &recovery.ExternalRef{
						Type:      "vm",
						Name:      "101",
						ID:        "101",
						Namespace: "pve1",
						Class:     "node1",
					},
					Display: &recovery.RecoveryPointDisplay{
						SubjectLabel:   "101",
						ItemType:       "vm",
						ClusterLabel:   "pve1",
						NodeHostLabel:  "node1",
						EntityIDLabel:  "101",
						DetailsSummary: "completed successfully",
					},
					StartedAt:   &startedAt,
					CompletedAt: &completedAt,
				},
			}},
		}),
	}

	output, err := svc.ExecuteAssistantTool(context.Background(), "pulse_storage", map[string]interface{}{
		"type":     "backup_tasks",
		"guest_id": "101",
		"instance": "pve1",
		"status":   "OK",
	})
	if err != nil {
		t.Fatalf("ExecuteAssistantTool(pulse_storage backup_tasks): %v", err)
	}

	var resp tools.BackupTasksListResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("unmarshal backup tasks response: %v\noutput=%s", err, output)
	}
	if resp.Total != 1 || resp.Filtered != 1 || len(resp.Tasks) != 1 {
		t.Fatalf("unexpected backup task counts: total=%d filtered=%d len=%d", resp.Total, resp.Filtered, len(resp.Tasks))
	}
	if resp.Tasks[0].VMID != 101 || resp.Tasks[0].Instance != "pve1" || resp.Tasks[0].Node != "node1" {
		t.Fatalf("unexpected backup task identity: %#v", resp.Tasks[0])
	}
	if resp.Tasks[0].Type != "vm" || resp.Tasks[0].Status != "OK" {
		t.Fatalf("unexpected backup task canonical fields: %#v", resp.Tasks[0])
	}
}

func TestService_StartInitializesActionAuditStore(t *testing.T) {
	svc := NewService(Config{
		AIConfig: &config.AIConfig{
			ChatModel: "mock:model",
		},
		DataDir: t.TempDir(),
		OrgID:   "org-a",
	})
	svc.providerFactory = func(modelStr string) (providers.StreamingProvider, error) {
		return &mockStreamingProvider{}, nil
	}

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}
	if svc.actionAuditStore == nil {
		t.Fatalf("expected action audit store to be initialized")
	}
	if svc.executor == nil {
		t.Fatalf("expected executor to be initialized")
	}
	if svc.executor.GetActionAuditStore() == nil {
		t.Fatalf("expected executor action audit store to be set")
	}

	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("stop service: %v", err)
	}
	if svc.actionAuditStore != nil {
		t.Fatalf("expected action audit store to be cleared on stop")
	}
}

func TestExecutionProfilePromptModes(t *testing.T) {
	loop := &AgenticLoop{baseSystemPrompt: "base"}

	prompt := loop.getSystemPrompt()
	if !strings.Contains(prompt, "EXECUTION MODE: Controlled") {
		t.Fatalf("default interactive profile should describe controlled mode, got %q", prompt)
	}

	loop.SetAutonomousMode(true)
	prompt = loop.getSystemPrompt()
	if !strings.Contains(prompt, "EXECUTION MODE: Autonomous") {
		t.Fatalf("interactive autonomous should describe autonomous mode, got %q", prompt)
	}

	// Patrol profiles own their mode text and never claim controlled or
	// autonomous execution, regardless of inherited autonomy flags.
	loop.SetExecutionProfile(tools.ProfilePatrolDetection)
	prompt = loop.getSystemPrompt()
	if !strings.Contains(prompt, "EXECUTION MODE: Patrol detection") ||
		strings.Contains(prompt, "EXECUTION MODE: Autonomous") ||
		strings.Contains(prompt, "EXECUTION MODE: Controlled") {
		t.Fatalf("detection profile prompt wrong: %q", prompt)
	}

	loop.SetExecutionProfile(tools.ProfilePatrolInvestigation)
	prompt = loop.getSystemPrompt()
	if !strings.Contains(prompt, "EXECUTION MODE: Patrol investigation") {
		t.Fatalf("investigation profile prompt wrong: %q", prompt)
	}
	if !strings.Contains(prompt, "propose a typed action") {
		t.Fatalf("investigation prompt must direct toward typed proposals, got %q", prompt)
	}
}

func TestToolsForExecutorHidesQuestionToolForPatrolProfiles(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		ControlLevel:  tools.ControlLevelControlled,
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
	})
	svc := &Service{executor: exec, cfg: &config.AIConfig{}}

	interactive := svc.toolsForExecutor(nil, false)
	if !toolNameSet(interactive)[pulseQuestionToolName] {
		t.Fatal("interactive non-autonomous manifest must offer pulse_question")
	}

	detection := exec.Clone()
	detection.ApplyExecutionProfile(tools.ProfilePatrolDetection)
	detectionTools := svc.toolsForExecutor(detection, false)
	if toolNameSet(detectionTools)[pulseQuestionToolName] {
		t.Fatal("detection profile must hide pulse_question")
	}

	investigation := exec.Clone()
	investigation.ApplyExecutionProfile(tools.ProfilePatrolInvestigation)
	investigationTools := svc.toolsForExecutor(investigation, false)
	set := toolNameSet(investigationTools)
	if set[pulseQuestionToolName] {
		t.Fatal("investigation profile must hide pulse_question")
	}
	if set["patrol_report_finding"] || set["pulse_control"] || set["pulse_file_edit"] {
		t.Fatalf("investigation manifest must not offer mutation tools, got %v", set)
	}
	if !set["pulse_query"] || !set["pulse_read"] {
		t.Fatalf("investigation manifest must keep read tools, got %v", set)
	}
}

func TestNonInteractiveProfileBlocksQuestionPersistsPairAndContinues(t *testing.T) {
	turn := 0
	provider := &stubStreamingProvider{}
	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		turn++
		switch turn {
		case 1:
			// The model fabricates an interactive question alongside a
			// legitimate read in the same provider turn.
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{
				ToolCalls: []providers.ToolCall{
					{ID: "q-1", Name: pulseQuestionToolName, Input: map[string]interface{}{"question": "which host?"}},
					{ID: "r-1", Name: "pulse_query", Input: map[string]interface{}{"action": "health"}},
				},
			}})
		default:
			callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "diagnosis complete"}})
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		}
		return nil
	}

	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
	})
	exec.ApplyExecutionProfile(tools.ProfilePatrolDetection)

	loop := NewAgenticLoop(provider, exec, "base prompt")
	loop.SetExecutionProfile(tools.ProfilePatrolDetection)

	var sawWaiting bool
	messages, err := loop.ExecuteWithTools(
		context.Background(),
		"session-noninteractive",
		[]Message{{Role: "user", Content: "investigate"}},
		nil,
		func(event StreamEvent) {
			if strings.Contains(string(event.Data), "Waiting for your answer") {
				sawWaiting = true
			}
		},
	)
	if err != nil {
		t.Fatalf("ExecuteWithTools: %v", err)
	}
	if sawWaiting {
		t.Fatal("non-interactive profile must not emit a waiting-for-answer event")
	}
	if turn < 2 {
		t.Fatalf("inference must continue after the blocked question, provider turns = %d", turn)
	}

	var questionCallSeen, questionResultSeen, queryResultSeen, finalTextSeen bool
	for _, msg := range messages {
		for _, tc := range msg.ToolCalls {
			if tc.Name == pulseQuestionToolName {
				questionCallSeen = true
			}
		}
		if msg.ToolResult != nil {
			switch msg.ToolResult.ToolUseID {
			case "q-1":
				questionResultSeen = true
				if !msg.ToolResult.IsError || !strings.Contains(msg.ToolResult.Content, "non-interactive") {
					t.Fatalf("question result must be a non-interactive error, got %+v", msg.ToolResult)
				}
			case "r-1":
				queryResultSeen = true
			}
		}
		if msg.Role == "assistant" && strings.Contains(msg.Content, "diagnosis complete") {
			finalTextSeen = true
		}
	}
	if !questionCallSeen {
		t.Fatal("persisted transcript must retain the question tool call")
	}
	if !questionResultSeen {
		t.Fatal("persisted transcript must pair the question call with its refusal result")
	}
	if !queryResultSeen {
		t.Fatal("sibling tool call from the same provider turn must still execute")
	}
	if !finalTextSeen {
		t.Fatal("loop must continue to the model's final answer after the blocked question")
	}
}

func TestInvestigationLoopRedactsProposalParamsEverywhereDurable(t *testing.T) {
	turn := 0
	provider := &stubStreamingProvider{}
	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		turn++
		if turn == 1 {
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{
				ToolCalls: []providers.ToolCall{{
					ID:   "p-1",
					Name: agentcapabilities.PatrolProposeActionToolName,
					Input: map[string]interface{}{
						"resource_id":     "vm:42",
						"capability_name": "restart",
						"params":          map[string]interface{}{"mode": "graceful"},
						"reason":          "recover the stalled web tier",
					},
				}},
			}})
			return nil
		}
		// The provider continuation must still see the raw params from
		// its own prior call.
		for _, msg := range req.Messages {
			for _, tc := range msg.ToolCalls {
				if tc.Name == agentcapabilities.PatrolProposeActionToolName {
					params, ok := tc.Input["params"].(map[string]interface{})
					if !ok || params["mode"] != "graceful" {
						t.Errorf("provider continuation lost raw proposal params: %#v", tc.Input["params"])
					}
				}
			}
		}
		callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "diagnosis: restart proposed"}})
		callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		return nil
	}

	capture := tools.NewProposalCapture(
		tools.ProposalIdentity{ProposalID: "prop-9", FindingID: "f-9", InvestigationID: "inv-9"},
		func(ctx context.Context, resourceID string) ([]ur.ResourceCapability, error) {
			return []ur.ResourceCapability{{
				Name: "restart",
				Params: []ur.CapabilityParam{
					{Name: "mode", Type: "string", Required: true, Enum: []string{"graceful", "force"}},
				},
			}}, nil
		})
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	exec.ApplyExecutionProfile(tools.ProfilePatrolInvestigation)
	exec.SetProposalCapture(capture)

	loop := NewAgenticLoop(provider, exec, "base prompt")
	loop.SetExecutionProfile(tools.ProfilePatrolInvestigation)

	var streamedRawParam bool
	messages, err := loop.ExecuteWithTools(
		context.Background(),
		"session-investigation",
		[]Message{{Role: "user", Content: "investigate finding f-9"}},
		nil,
		func(event StreamEvent) {
			if strings.Contains(string(event.Data), "graceful") {
				streamedRawParam = true
			}
		},
	)
	if err != nil {
		t.Fatalf("ExecuteWithTools: %v", err)
	}
	if streamedRawParam {
		t.Fatal("stream events must never expose proposal parameter values")
	}

	// The durable transcript keeps the call but with redacted params.
	var sawCall bool
	for _, msg := range messages {
		for _, tc := range msg.ToolCalls {
			if tc.Name != agentcapabilities.PatrolProposeActionToolName {
				continue
			}
			sawCall = true
			if tc.Input["params"] != agentcapabilities.RedactedProposalParamsMarker {
				t.Fatalf("transcript tool call must carry redacted params, got %#v", tc.Input["params"])
			}
		}
	}
	if !sawCall {
		t.Fatal("transcript must retain the proposal tool call")
	}

	// The structured capture holds the validated raw values.
	proposal, failed, outcomeErr := capture.Outcome()
	if outcomeErr != nil || failed != 0 {
		t.Fatalf("outcome = (%v, %d), want clean capture", outcomeErr, failed)
	}
	if proposal == nil || proposal.Params["mode"] != "graceful" || proposal.Identity.FindingID != "f-9" || proposal.InvocationID != "p-1" {
		t.Fatalf("captured proposal = %#v", proposal)
	}
}

func TestProposalRawInputOverrideNeverLeaksThroughProgressEvents(t *testing.T) {
	var events []string
	callback := func(event StreamEvent) { events = append(events, string(event.Data)) }

	// Providers stream the accumulating raw tool-call JSON as a
	// RawInput override on progress events; for exposure-restricted
	// tools that override must be discarded, not substituted back in
	// after projection.
	secret := `{"params":{"join_token":"super-secret-value"}}`
	emitToolProgressEventWithRawInput(callback, "p-1",
		agentcapabilities.PatrolProposeActionToolName,
		map[string]interface{}{
			"resource_id": "vm:42",
			"params":      map[string]interface{}{"join_token": "super-secret-value"},
		},
		secret, "streaming", "Receiving tool call.")

	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
	if strings.Contains(events[0], "super-secret-value") {
		t.Fatalf("progress event leaked raw proposal params: %s", events[0])
	}
	if !strings.Contains(events[0], agentcapabilities.RedactedProposalParamsMarker) {
		t.Fatalf("progress event should carry the projected form, got %s", events[0])
	}

	// Unrestricted tools keep the provider's raw override.
	events = nil
	emitToolProgressEventWithRawInput(callback, "q-1", "pulse_query",
		map[string]interface{}{"action": "health"},
		`{"action":"health","extra":"raw-override"}`, "streaming", "Receiving tool call.")
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
	if !strings.Contains(events[0], "raw-override") {
		t.Fatalf("unrestricted tools should keep the raw override, got %s", events[0])
	}
}

func TestInvestigationRunRequiresIdentityBeforeStarting(t *testing.T) {
	svc := &Service{}
	_, err := svc.ExecuteInvestigationStream(context.Background(), InvestigationRunRequest{
		Prompt:   "investigate",
		Identity: tools.ProposalIdentity{FindingID: "f-1"}, // missing investigation ID
	}, func(StreamEvent) {})
	if err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("expected identity precondition error, got %v", err)
	}
}
