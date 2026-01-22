package tools

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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
			return payload.Command == "uptime" && payload.TargetType == "host" && payload.TargetID == "host1"
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
		assert.Contains(t, result.Content[0].Text, "Command completed successfully")
		assert.Contains(t, result.Content[0].Text, "ok")
		agentSrv.AssertExpectations(t)
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
