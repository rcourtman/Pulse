package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPulseToolExecutor_ExecuteReadLogs_Fallbacks(t *testing.T) {
	ctx := context.Background()

	t.Run("DockerSourceWithoutContainerFallsBackToDockerPs", func(t *testing.T) {
		t.Setenv("PULSE_STRICT_RESOLUTION", "false")

		agentSrv := &mockAgentServer{}
		agentSrv.On("GetConnectedAgents").Return([]agentexec.ConnectedAgent{
			{AgentID: "agent1", Hostname: "node1"},
		})
		agentSrv.On("ExecuteCommand", mock.Anything, "agent1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
			return payload.TargetType == "host" &&
				payload.TargetID == "" &&
				strings.Contains(payload.Command, "docker ps --format") &&
				strings.Contains(payload.Command, "head -20")
		})).Return(&agentexec.CommandResultPayload{
			Stdout:   "container-a\tUp 5h",
			ExitCode: 0,
		}, nil).Once()

		exec := NewPulseToolExecutor(ExecutorConfig{AgentServer: agentSrv})
		result, err := exec.executeReadLogs(ctx, map[string]interface{}{
			"action":      "logs",
			"source":      "docker",
			"target_host": "node1",
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
		require.NotEmpty(t, result.Content)
		assert.Contains(t, result.Content[0].Text, "container-a")
		agentSrv.AssertExpectations(t)
	})

	t.Run("JournalSourceWithoutUnitFallsBackToGlobalJournal", func(t *testing.T) {
		t.Setenv("PULSE_STRICT_RESOLUTION", "false")

		agentSrv := &mockAgentServer{}
		agentSrv.On("GetConnectedAgents").Return([]agentexec.ConnectedAgent{
			{AgentID: "agent1", Hostname: "node1"},
		})
		agentSrv.On("ExecuteCommand", mock.Anything, "agent1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
			return payload.TargetType == "host" &&
				payload.TargetID == "" &&
				payload.Command == "journalctl --since '1h' -n 50 --no-pager"
		})).Return(&agentexec.CommandResultPayload{
			Stdout:   "journal output",
			ExitCode: 0,
		}, nil).Once()

		exec := NewPulseToolExecutor(ExecutorConfig{AgentServer: agentSrv})
		result, err := exec.executeReadLogs(ctx, map[string]interface{}{
			"action":      "logs",
			"source":      "journal",
			"target_host": "node1",
			"since":       "1h",
			"lines":       50,
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
		require.NotEmpty(t, result.Content)
		assert.Contains(t, result.Content[0].Text, "journal output")
		agentSrv.AssertExpectations(t)
	})

	t.Run("MissingSourceInfersDockerAndUnknownSourceFallsBackToJournal", func(t *testing.T) {
		t.Setenv("PULSE_STRICT_RESOLUTION", "false")

		agentSrv := &mockAgentServer{}
		agentSrv.On("GetConnectedAgents").Return([]agentexec.ConnectedAgent{
			{AgentID: "agent1", Hostname: "node1"},
		})
		agentSrv.On("ExecuteCommand", mock.Anything, "agent1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
			return payload.Command == "docker logs --tail 100 'homepage'" && payload.TargetType == "host"
		})).Return(&agentexec.CommandResultPayload{
			Stdout:   "docker log line",
			ExitCode: 0,
		}, nil).Once()
		agentSrv.On("ExecuteCommand", mock.Anything, "agent1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
			return payload.Command == "journalctl -n 30 --no-pager" && payload.TargetType == "host"
		})).Return(&agentexec.CommandResultPayload{
			Stdout:   "journal fallback line",
			ExitCode: 0,
		}, nil).Once()

		exec := NewPulseToolExecutor(ExecutorConfig{AgentServer: agentSrv})

		result, err := exec.executeReadLogs(ctx, map[string]interface{}{
			"action":      "logs",
			"target_host": "node1",
			"container":   "homepage",
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
		require.NotEmpty(t, result.Content)
		assert.Contains(t, result.Content[0].Text, "docker log line")

		result, err = exec.executeReadLogs(ctx, map[string]interface{}{
			"action":      "logs",
			"source":      "syslog",
			"target_host": "node1",
			"lines":       30,
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
		require.NotEmpty(t, result.Content)
		assert.Contains(t, result.Content[0].Text, "journal fallback line")

		agentSrv.AssertExpectations(t)
	})
}
