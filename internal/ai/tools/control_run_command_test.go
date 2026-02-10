package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func mustParseJSONMap(t *testing.T, text string) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(text), &out))
	return out
}

func TestPulseToolExecutor_ExecuteRunCommand(t *testing.T) {
	ctx := context.Background()

	t.Run("MissingCommand", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{})
		result, err := exec.executeRunCommand(ctx, map[string]interface{}{})
		assert.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "command is required")
	})

	t.Run("PolicyBlocked", func(t *testing.T) {
		policy := &mockCommandPolicy{}
		policy.On("Evaluate", "rm -rf /").Return(agentexec.PolicyBlock).Once()

		exec := NewPulseToolExecutor(ExecutorConfig{Policy: policy})
		result, err := exec.executeRunCommand(ctx, map[string]interface{}{
			"command": "rm -rf /",
		})
		assert.NoError(t, err)
		assert.False(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "POLICY_BLOCKED")
		policy.AssertExpectations(t)
	})

	t.Run("TargetHostRequired", func(t *testing.T) {
		agentSrv := &mockAgentServer{agents: []agentexec.ConnectedAgent{
			{AgentID: "a1", Hostname: "node1"},
			{AgentID: "a2", Hostname: "node2"},
		}}
		exec := NewPulseToolExecutor(ExecutorConfig{AgentServer: agentSrv})

		result, err := exec.executeRunCommand(ctx, map[string]interface{}{
			"command": "ls",
		})
		assert.NoError(t, err)
		assert.Contains(t, result.Content[0].Text, "Multiple agents are connected")
	})

	t.Run("ControlledRequiresApproval", func(t *testing.T) {
		approval.SetStore(nil)
		exec := NewPulseToolExecutor(ExecutorConfig{ControlLevel: ControlLevelControlled})
		result, err := exec.executeRunCommand(ctx, map[string]interface{}{
			"command": "ls",
		})
		assert.NoError(t, err)
		assert.Contains(t, result.Content[0].Text, "APPROVAL_REQUIRED")
	})

	t.Run("ExecuteSuccess", func(t *testing.T) {
		agentSrv := &mockAgentServer{}
		agentSrv.On("GetConnectedAgents").Return([]agentexec.ConnectedAgent{
			{AgentID: "agent1", Hostname: "node1"},
		}).Twice()
		agentSrv.On("ExecuteCommand", mock.Anything, "agent1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
			// For direct host targets, TargetID is empty - resolveTargetForCommand returns "" for host type
			return payload.Command == "uptime" && payload.TargetType == "host" && payload.TargetID == ""
		})).Return(&agentexec.CommandResultPayload{
			Stdout:   "ok",
			ExitCode: 0,
		}, nil).Once()

		exec := NewPulseToolExecutor(ExecutorConfig{AgentServer: agentSrv})
		exec.SetContext("host", "host1", false)

		result, err := exec.executeRunCommand(ctx, map[string]interface{}{
			"command":     "uptime",
			"run_on_host": true,
		})
		assert.NoError(t, err)
		resp := mustParseJSONMap(t, result.Content[0].Text)
		assert.Equal(t, true, resp["success"])
		assert.Equal(t, float64(0), resp["exit_code"])
		assert.Contains(t, resp["output"].(string), "ok")
		if v, ok := resp["verification"].(map[string]interface{}); ok {
			assert.Equal(t, true, v["ok"])
		}
		agentSrv.AssertExpectations(t)
	})
}

func TestPulseToolExecutor_RunCommandLXCRouting(t *testing.T) {
	ctx := context.Background()

	t.Run("LXCCommandRoutedCorrectly", func(t *testing.T) {
		// Test that commands targeting LXCs are routed with correct target type/ID
		// The agent handles sh -c wrapping, so tool just sends raw command
		agents := []agentexec.ConnectedAgent{{AgentID: "proxmox-agent", Hostname: "delly"}}
		mockAgent := &mockAgentServer{}
		mockAgent.On("GetConnectedAgents").Return(agents)
		mockAgent.On("ExecuteCommand", mock.Anything, "proxmox-agent", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			// Tool sends raw command, agent will wrap in sh -c
			return cmd.TargetType == "container" &&
				cmd.TargetID == "108" &&
				cmd.Command == "grep pattern /var/log/*.log"
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "matched line",
		}, nil)

		state := models.StateSnapshot{
			Containers: []models.Container{
				{VMID: 108, Name: "jellyfin", Node: "delly"},
			},
		}

		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider: &mockStateProvider{state: state},
			AgentServer:   mockAgent,
			ControlLevel:  ControlLevelAutonomous,
		})
		result, err := exec.executeRunCommand(ctx, map[string]interface{}{
			"command":     "grep pattern /var/log/*.log",
			"target_host": "jellyfin",
		})
		require.NoError(t, err)
		resp := mustParseJSONMap(t, result.Content[0].Text)
		assert.Equal(t, true, resp["success"])
		assert.Equal(t, "jellyfin", resp["target_host"])
		mockAgent.AssertExpectations(t)
	})

	t.Run("VMCommandRoutedCorrectly", func(t *testing.T) {
		// Test that commands targeting VMs are routed with correct target type/ID
		agents := []agentexec.ConnectedAgent{{AgentID: "proxmox-agent", Hostname: "delly"}}
		mockAgent := &mockAgentServer{}
		mockAgent.On("GetConnectedAgents").Return(agents)
		mockAgent.On("ExecuteCommand", mock.Anything, "proxmox-agent", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return cmd.TargetType == "vm" &&
				cmd.TargetID == "100" &&
				cmd.Command == "ls /tmp/*.txt"
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "result",
		}, nil)

		state := models.StateSnapshot{
			VMs: []models.VM{
				{VMID: 100, Name: "test-vm", Node: "delly"},
			},
		}

		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider: &mockStateProvider{state: state},
			AgentServer:   mockAgent,
			ControlLevel:  ControlLevelAutonomous,
		})
		result, err := exec.executeRunCommand(ctx, map[string]interface{}{
			"command":     "ls /tmp/*.txt",
			"target_host": "test-vm",
		})
		require.NoError(t, err)
		resp := mustParseJSONMap(t, result.Content[0].Text)
		assert.Equal(t, true, resp["success"])
		assert.Equal(t, "test-vm", resp["target_host"])
		mockAgent.AssertExpectations(t)
	})

	t.Run("DirectHostRoutedCorrectly", func(t *testing.T) {
		// Direct host commands have target type "host"
		agents := []agentexec.ConnectedAgent{{AgentID: "host-agent", Hostname: "tower"}}
		mockAgent := &mockAgentServer{}
		mockAgent.On("GetConnectedAgents").Return(agents)
		mockAgent.On("ExecuteCommand", mock.Anything, "host-agent", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return cmd.TargetType == "host" &&
				cmd.Command == "ls /tmp/*.txt"
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "files",
		}, nil)

		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider: &mockStateProvider{state: models.StateSnapshot{}},
			AgentServer:   mockAgent,
			ControlLevel:  ControlLevelAutonomous,
		})
		result, err := exec.executeRunCommand(ctx, map[string]interface{}{
			"command":     "ls /tmp/*.txt",
			"target_host": "tower",
		})
		require.NoError(t, err)
		resp := mustParseJSONMap(t, result.Content[0].Text)
		assert.Equal(t, true, resp["success"])
		assert.Equal(t, "tower", resp["target_host"])
		mockAgent.AssertExpectations(t)
	})
}

func TestPulseToolExecutor_FindAgentForCommand(t *testing.T) {
	t.Run("NoAgentServer", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{})
		assert.Empty(t, exec.findAgentForCommand(false, ""))
	})

	t.Run("NoAgents", func(t *testing.T) {
		agentSrv := &mockAgentServer{}
		exec := NewPulseToolExecutor(ExecutorConfig{AgentServer: agentSrv})
		assert.Empty(t, exec.findAgentForCommand(false, ""))
	})

	t.Run("TargetHostMatches", func(t *testing.T) {
		agentSrv := &mockAgentServer{agents: []agentexec.ConnectedAgent{
			{AgentID: "a1", Hostname: "node1"},
		}}
		exec := NewPulseToolExecutor(ExecutorConfig{AgentServer: agentSrv})
		assert.Equal(t, "a1", exec.findAgentForCommand(false, "a1"))
	})

	t.Run("MultipleAgentsNoTarget", func(t *testing.T) {
		agentSrv := &mockAgentServer{agents: []agentexec.ConnectedAgent{
			{AgentID: "a1", Hostname: "node1"},
			{AgentID: "a2", Hostname: "node2"},
		}}
		exec := NewPulseToolExecutor(ExecutorConfig{AgentServer: agentSrv})
		assert.Empty(t, exec.findAgentForCommand(false, ""))
	})

	t.Run("SingleAgentNoTarget", func(t *testing.T) {
		agentSrv := &mockAgentServer{agents: []agentexec.ConnectedAgent{
			{AgentID: "a1", Hostname: "node1"},
		}}
		exec := NewPulseToolExecutor(ExecutorConfig{AgentServer: agentSrv})
		assert.Equal(t, "a1", exec.findAgentForCommand(false, ""))
	})
}
