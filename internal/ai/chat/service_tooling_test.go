package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type fakeStateProvider struct{}

func (f fakeStateProvider) GetState() models.StateSnapshot {
	return models.StateSnapshot{}
}

type fakeAgentServer struct{}

func (f fakeAgentServer) GetConnectedAgents() []agentexec.ConnectedAgent {
	return []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "node-1"}}
}

func (f fakeAgentServer) ExecuteCommand(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
	return &agentexec.CommandResultPayload{Stdout: "ok", ExitCode: 0}, nil
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
