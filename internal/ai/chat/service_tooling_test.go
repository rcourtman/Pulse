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

func TestFilterToolsForPrompt_ReadOnlyAndSpecialty(t *testing.T) {
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

	readOnlyTools := svc.filterToolsForPrompt(context.Background(), "show node status", true, false)
	readOnlySet := toolNameSet(readOnlyTools)
	if readOnlySet["pulse_control"] || readOnlySet["pulse_file_edit"] || readOnlySet["pulse_docker"] {
		t.Fatalf("expected write tools to be filtered for read-only prompt")
	}
	if !readOnlySet["pulse_storage"] || !readOnlySet["pulse_kubernetes"] || !readOnlySet["pulse_pmg"] {
		t.Fatalf("expected specialty tools to remain when no specialty keyword detected")
	}

	k8sTools := svc.filterToolsForPrompt(context.Background(), "check kubernetes pods", false, false)
	k8sSet := toolNameSet(k8sTools)
	if !k8sSet["pulse_kubernetes"] {
		t.Fatalf("expected kubernetes tool to be included")
	}
	if k8sSet["pulse_storage"] || k8sSet["pulse_pmg"] {
		t.Fatalf("expected non-k8s specialty tools to be excluded")
	}
}

func TestFilterToolsForPrompt_BroadInfraKeepsStorage(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: fakeStateProvider{},
		AgentServer:   fakeAgentServer{},
		ControlLevel:  tools.ControlLevelControlled,
	})

	svc := &Service{executor: exec}
	toolsList := svc.filterToolsForPrompt(context.Background(), "full status overview", false, false)
	set := toolNameSet(toolsList)
	if !set["pulse_storage"] {
		t.Fatalf("expected storage tool to be kept for broad infrastructure prompt")
	}
	if !set["pulse_control"] || !set["pulse_file_edit"] || !set["pulse_docker"] {
		t.Fatalf("expected interactive mode to keep write tools")
	}
}

func TestFilterToolsForPrompt_AutonomousNonPatrol(t *testing.T) {
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

	// Use write intent so read-only write-tool gating does not hide docker.
	filtered := svc.filterToolsForPrompt(context.Background(), "restart docker containers and check storage pools", true, false)
	set := toolNameSet(filtered)

	if !set["pulse_docker"] {
		t.Fatalf("expected pulse_docker to be included for autonomous non-patrol runs")
	}
	if !set["pulse_storage"] {
		t.Fatalf("expected pulse_storage to be included for autonomous non-patrol runs")
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
