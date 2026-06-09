package chat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

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

	interactiveTools := svc.toolsForExecutionMode(false, false)
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

func TestToolsForAssistantTurn_DirectTextWithholdsTools(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: fakeStateProvider{},
		AgentServer:   fakeAgentServer{},
		ReadState:     &fakeCanonicalReadState{},
		ControlLevel:  tools.ControlLevelControlled,
	})

	svc := &Service{executor: exec}
	toolsList := svc.toolsForAssistantTurn("Reply exactly: PULSE_CHAT_OK", false, false, false)

	if len(toolsList) != 0 {
		t.Fatalf("expected direct text turn to withhold tools, got %#v", toolNameSet(toolsList))
	}
	prompt := svc.buildSystemPromptForOfferedTools(toolsList)
	if strings.Contains(prompt, "pulse_query") || strings.Contains(prompt, "pulse_read") {
		t.Fatalf("expected text-only system prompt not to advertise tools, got %q", prompt)
	}
	if !strings.Contains(prompt, "No Pulse tools are offered for this turn") {
		t.Fatalf("expected text-only system prompt to state the tool boundary, got %q", prompt)
	}
}

func TestToolsForAssistantTurn_InventoryCountUsesQueryOnly(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: fakeStateProvider{},
		AgentServer:   fakeAgentServer{},
		ReadState:     &fakeCanonicalReadState{},
		ControlLevel:  tools.ControlLevelControlled,
	})

	svc := &Service{executor: exec}
	toolsList := svc.toolsForAssistantTurn("how many devices in this", false, false, false)
	set := toolNameSet(toolsList)

	if !set["pulse_query"] {
		t.Fatalf("expected inventory count prompt to expose pulse_query, got %#v", set)
	}
	if !set[pulseQuestionToolName] {
		t.Fatalf("expected interactive inventory prompt to keep clarification available, got %#v", set)
	}
	for _, blocked := range []string{"pulse_read", "pulse_control", "pulse_file_edit", "pulse_docker"} {
		if set[blocked] {
			t.Fatalf("expected inventory count prompt not to expose %s, got %#v", blocked, set)
		}
	}

	prompt := svc.buildSystemPromptForOfferedTools(toolsList)
	if !strings.Contains(prompt, "pulse_query: mode=read") {
		t.Fatalf("expected query-only system prompt to advertise pulse_query, got %q", prompt)
	}
	if strings.Contains(prompt, "pulse_read") || strings.Contains(prompt, "pulse_control") {
		t.Fatalf("expected query-only system prompt not to advertise shell/control tools, got %q", prompt)
	}
}

func TestToolsForAssistantTurn_ExplicitCommandCountKeepsFullManifest(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: fakeStateProvider{},
		AgentServer:   fakeAgentServer{},
		ReadState:     &fakeCanonicalReadState{},
		ControlLevel:  tools.ControlLevelControlled,
	})

	svc := &Service{executor: exec}
	toolsList := svc.toolsForAssistantTurn("Use read-only tools only. On node delly, count entries in /dev with `ls /dev | wc -l`; answer with the number only.", false, false, false)
	set := toolNameSet(toolsList)

	for _, required := range []string{"pulse_read", "pulse_query", pulseQuestionToolName} {
		if !set[required] {
			t.Fatalf("expected explicit command count prompt to expose %s, got %#v", required, set)
		}
	}
}

func TestToolsForAssistantTurn_InventoryBreakdownUsesQueryOnly(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: fakeStateProvider{},
		AgentServer:   fakeAgentServer{},
		ReadState:     &fakeCanonicalReadState{},
		ControlLevel:  tools.ControlLevelControlled,
	})

	svc := &Service{executor: exec}
	toolsList := svc.toolsForAssistantTurn("give me a detailed inventory breakdown by node", false, false, false)
	set := toolNameSet(toolsList)

	if !set["pulse_query"] {
		t.Fatalf("expected inventory breakdown prompt to expose pulse_query, got %#v", set)
	}
	for _, blocked := range []string{"pulse_read", "pulse_control", "pulse_file_edit", "pulse_docker"} {
		if set[blocked] {
			t.Fatalf("expected inventory breakdown prompt not to expose %s, got %#v", blocked, set)
		}
	}
}

func TestToolsForAssistantTurn_ActionAndDiagnosticsKeepFullManifest(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: fakeStateProvider{},
		AgentServer:   fakeAgentServer{},
		ReadState:     &fakeCanonicalReadState{},
		ControlLevel:  tools.ControlLevelControlled,
	})

	svc := &Service{executor: exec}
	for _, prompt := range []string{
		"show me the logs for vm 100",
		"restart vm 100",
		"diagnose high cpu on minipc",
	} {
		toolsList := svc.toolsForAssistantTurn(prompt, false, false, false)
		set := toolNameSet(toolsList)
		if !set["pulse_read"] || !set["pulse_control"] || !set["pulse_query"] {
			t.Fatalf("expected full manifest for %q, got %#v", prompt, set)
		}
	}
}

func TestToolsForAssistantTurn_AlertPromptsKeepFullManifest(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: fakeStateProvider{},
		AgentServer:   fakeAgentServer{},
		ReadState:     &fakeCanonicalReadState{},
		ControlLevel:  tools.ControlLevelControlled,
	})

	svc := &Service{executor: exec}
	for _, prompt := range []string{
		"Alerts count",
		"active alerts",
		"findings count",
	} {
		toolsList := svc.toolsForAssistantTurn(prompt, false, false, false)
		set := toolNameSet(toolsList)
		if !set["pulse_alerts"] || !set["pulse_query"] {
			t.Fatalf("expected full manifest for %q, got %#v", prompt, set)
		}
		promptText := svc.buildSystemPromptForOfferedTools(toolsList)
		if strings.Contains(promptText, "No Pulse tools are offered for this turn") {
			t.Fatalf("expected alert prompt to avoid text-only tool boundary, got %q", promptText)
		}
		if !strings.Contains(promptText, "pulse_alerts: mode=mixed") {
			t.Fatalf("expected alert prompt system prompt to advertise pulse_alerts, got %q", promptText)
		}
	}
}

func TestToolsForAssistantTurn_ShortResourceLookupGetsTools(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: fakeStateProvider{},
		AgentServer:   fakeAgentServer{},
		ReadState:     &fakeCanonicalReadState{},
		ControlLevel:  tools.ControlLevelControlled,
	})
	svc := &Service{executor: exec}

	// Short prompts that name a resource are lookups, not chit-chat. They must be
	// offered tools so the Assistant can inspect the resource from Pulse data
	// instead of telling the user it has no way to check.
	for _, prompt := range []string{"hows esphome", "check frigate", "grafana cpu", "esphome status"} {
		toolsList := svc.toolsForAssistantTurn(prompt, false, false, false)
		if len(toolsList) == 0 {
			t.Fatalf("short resource lookup %q was offered no tools", prompt)
		}
		promptText := svc.buildSystemPromptForOfferedTools(toolsList)
		if strings.Contains(promptText, "No Pulse tools are offered for this turn") {
			t.Fatalf("short resource lookup %q got the text-only tool boundary, got %q", prompt, promptText)
		}
	}

	// Explicit greetings / meta-questions remain text-only.
	for _, prompt := range []string{"hi", "hello there", "thanks", "who are you"} {
		toolsList := svc.toolsForAssistantTurn(prompt, false, false, false)
		if len(toolsList) != 0 {
			t.Fatalf("conversational prompt %q should be text-only, got %d tools", prompt, len(toolsList))
		}
	}
}

func TestToolsForAssistantTurn_ModelHandoffKeepsFullManifest(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: fakeStateProvider{},
		AgentServer:   fakeAgentServer{},
		ReadState:     &fakeCanonicalReadState{},
		ControlLevel:  tools.ControlLevelControlled,
	})

	svc := &Service{executor: exec}
	toolsList := svc.toolsForAssistantTurn("Reply exactly: PULSE_CHAT_OK", false, false, true)
	set := toolNameSet(toolsList)
	if !set["pulse_read"] || !set["pulse_control"] || !set["pulse_query"] {
		t.Fatalf("expected model handoff turns to keep full manifest, got %#v", set)
	}
}

func TestToolsForExecutionMode_AutonomousNonPatrolExposesGovernedTools(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: fakeStateProvider{},
		AgentServer:   fakeAgentServer{},
		ControlLevel:  tools.ControlLevelControlled,
	})

	svc := &Service{executor: exec}
	toolsList := svc.toolsForExecutionMode(true, false)
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
		"Missing target information is not a safe default.",
		"In autonomous mode, ask for the missing target in normal assistant text instead of attempting a tool call with current_resource",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected current_resource boundary %q in system prompt, got %q", expected, prompt)
		}
	}
}

func TestBuildToolGovernancePromptSection_FallbackDiscoveryMatchesRunContract(t *testing.T) {
	svc := &Service{}

	prompt := svc.buildToolGovernancePromptSection()

	if !strings.Contains(prompt, "pulse_discovery: mode=mixed") {
		t.Fatalf("expected fallback governance to classify pulse_discovery as mixed, got %q", prompt)
	}
	if !strings.Contains(prompt, "run uses read-only evidence collection and updates the discovery cache") {
		t.Fatalf("expected fallback governance to describe discovery refresh behavior, got %q", prompt)
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
	toolsList := svc.toolsForExecutionMode(false, false)
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

	filtered := svc.toolsForExecutionMode(true, true)
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

func TestExecuteCommand_SuccessAndExitCode(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	exec.RegisterTool(tools.RegisteredTool{
		Definition: tools.Tool{Name: "pulse_run_command"},
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
		Definition: tools.Tool{Name: "pulse_run_command"},
		Handler: func(ctx context.Context, exec *tools.PulseToolExecutor, args map[string]interface{}) (tools.CallToolResult, error) {
			return tools.NewErrorResult(context.Canceled), nil
		},
	})

	svc := &Service{executor: exec}

	_, code, err := svc.ExecuteCommand(context.Background(), "uptime", "")
	if err == nil || code != 1 {
		t.Fatalf("expected error with exit code 1")
	}

	exec.RegisterTool(tools.RegisteredTool{
		Definition: tools.Tool{Name: "pulse_run_command"},
		Handler: func(ctx context.Context, exec *tools.PulseToolExecutor, args map[string]interface{}) (tools.CallToolResult, error) {
			return tools.NewTextResult("APPROVAL_REQUIRED: requires approval"), nil
		},
	})

	_, _, err = svc.ExecuteCommand(context.Background(), "uptime", "")
	if err == nil {
		t.Fatalf("expected approval error")
	}
}

func TestExecuteMCPTool_ErrorsAndSuccess(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	exec.RegisterTool(tools.RegisteredTool{
		Definition: tools.Tool{Name: "test_tool"},
		Handler: func(ctx context.Context, exec *tools.PulseToolExecutor, args map[string]interface{}) (tools.CallToolResult, error) {
			return tools.NewErrorResult(context.DeadlineExceeded), nil
		},
	})

	svc := &Service{executor: exec}

	_, err := svc.ExecuteMCPTool(context.Background(), "test_tool", map[string]interface{}{})
	if err == nil {
		t.Fatalf("expected tool error")
	}

	exec.RegisterTool(tools.RegisteredTool{
		Definition: tools.Tool{Name: "test_tool"},
		Handler: func(ctx context.Context, exec *tools.PulseToolExecutor, args map[string]interface{}) (tools.CallToolResult, error) {
			return tools.NewTextResult("POLICY_BLOCKED: nope"), nil
		},
	})
	_, err = svc.ExecuteMCPTool(context.Background(), "test_tool", map[string]interface{}{})
	if err == nil {
		t.Fatalf("expected policy blocked error")
	}

	exec.RegisterTool(tools.RegisteredTool{
		Definition: tools.Tool{Name: "test_tool"},
		Handler: func(ctx context.Context, exec *tools.PulseToolExecutor, args map[string]interface{}) (tools.CallToolResult, error) {
			return tools.NewTextResult("ok"), nil
		},
	})
	output, err := svc.ExecuteMCPTool(context.Background(), "test_tool", map[string]interface{}{})
	if err != nil || output != "ok" {
		t.Fatalf("expected success, got output=%q err=%v", output, err)
	}
}

func TestExecuteMCPTool_PulseStorageSnapshotsToleratesMalformedRecoveryMetadata(t *testing.T) {
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

	output, err := svc.ExecuteMCPTool(context.Background(), "pulse_storage", map[string]interface{}{
		"type":     "snapshots",
		"guest_id": "100",
		"instance": "pve1",
	})
	if err != nil {
		t.Fatalf("ExecuteMCPTool(pulse_storage snapshots): %v", err)
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

func TestExecuteMCPTool_PulseStorageBackupTasksToleratesMalformedRecoveryMetadata(t *testing.T) {
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

	output, err := svc.ExecuteMCPTool(context.Background(), "pulse_storage", map[string]interface{}{
		"type":     "backup_tasks",
		"guest_id": "101",
		"instance": "pve1",
		"status":   "OK",
	})
	if err != nil {
		t.Fatalf("ExecuteMCPTool(pulse_storage backup_tasks): %v", err)
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
