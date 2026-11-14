package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

// A caller without the action-execute capability keeps the Assistant for
// Pulse's own data, but no tool that runs a command on a host is offered or
// executed for them. Unbound executors (Patrol and other system runs) are not
// restricted.

func newHostCommandExecutor(t *testing.T, agentSrv *mockAgentServer) *PulseToolExecutor {
	t.Helper()
	exec := NewPulseToolExecutor(ExecutorConfig{AgentServer: agentSrv})
	exec.SetControlLevel(ControlLevelControlled)
	exec.ApplyExecutionProfile(ProfileInteractiveAssistant)
	exec.SetExecuteAuthority(true)
	return exec
}

func kubernetesTypes(tools []Tool) []string {
	for _, tool := range tools {
		if tool.Name == agentcapabilities.PulseKubernetesToolName {
			return tool.InputSchema.Properties["type"].Enum
		}
	}
	return nil
}

func TestHostCommandToolsWithheldWithoutAuthority(t *testing.T) {
	agentSrv := &mockAgentServer{}
	exec := newHostCommandExecutor(t, agentSrv)
	exec.SetHostCommandAuthority(false)
	require.True(t, exec.HostCommandsWithheld())

	projected := exec.registry.ListTools(exec.invocationPolicy())
	assert.False(t, hasToolNamed(projected, agentcapabilities.PulseReadToolName), "pulse_read offered without host-command authority")
	assert.True(t, hasToolNamed(projected, agentcapabilities.PulseKubernetesToolName), "read-only kubernetes inventory stays available")
	assert.NotContains(t, kubernetesTypes(projected), "logs")
	assert.Contains(t, kubernetesTypes(projected), "pods")
	assert.False(t, hasToolNamed(exec.ListTools(), agentcapabilities.PulseReadToolName))

	for _, action := range []string{"exec", "file", "find", "tail", "logs"} {
		result, err := exec.registry.Execute(context.Background(), exec, agentcapabilities.PulseReadToolName, map[string]interface{}{
			"action":      action,
			"command":     "cat /var/log/syslog",
			"path":        "/var/log/syslog",
			"pattern":     "*.log",
			"target_host": "node1",
		})
		require.NoError(t, err)
		assert.True(t, result.IsError, action)
		text := agentcapabilities.ToolResultText(result)
		assert.Contains(t, text, "not allowed to run actions", action)
		assert.Contains(t, text, agentcapabilities.PulseReadToolName, action)
	}
	result, err := exec.registry.Execute(context.Background(), exec, agentcapabilities.PulseKubernetesToolName, map[string]interface{}{
		"type": "logs", "cluster": "c1", "pod": "p1",
	})
	require.NoError(t, err)
	assert.Contains(t, agentcapabilities.ToolResultText(result), "not allowed to run actions")

	// mockAgentServer fails the test on any unexpected ExecuteCommand call.
	agentSrv.AssertNotCalled(t, "ExecuteCommand")
}

func TestHostCommandToolsOfferedWithAuthority(t *testing.T) {
	exec := newHostCommandExecutor(t, &mockAgentServer{})
	exec.SetHostCommandAuthority(true)
	assert.False(t, exec.HostCommandsWithheld())

	projected := exec.registry.ListTools(exec.invocationPolicy())
	assert.True(t, hasToolNamed(projected, agentcapabilities.PulseReadToolName))
	assert.Contains(t, kubernetesTypes(projected), "logs")
}

func TestHostCommandAuthorityUnboundLeavesSystemRunsAlone(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{AgentServer: &mockAgentServer{}})
	exec.SetControlLevel(ControlLevelReadOnly)
	exec.ApplyExecutionProfile(ProfilePatrolInvestigation)
	assert.False(t, exec.HostCommandsWithheld())
	assert.True(t, hasToolNamed(exec.registry.ListTools(exec.invocationPolicy()), agentcapabilities.PulseReadToolName))
}

func TestHostCommandAuthorityIsCloneIsolated(t *testing.T) {
	original := newHostCommandExecutor(t, &mockAgentServer{})
	original.SetHostCommandAuthority(true)

	clone := original.Clone()
	clone.SetHostCommandAuthority(false)
	assert.False(t, original.HostCommandsWithheld(), "restricting a clone must not restrict the original")
	assert.True(t, clone.HostCommandsWithheld())
	assert.True(t, clone.Clone().HostCommandsWithheld(), "the restriction survives further cloning")
}

func dockerActions(tools []Tool) []string {
	for _, tool := range tools {
		if tool.Name == agentcapabilities.PulseDockerToolName {
			return tool.InputSchema.Properties["action"].Enum
		}
	}
	return nil
}

func TestDockerUpdateCheckQueuesNothingWithoutHostCommandAuthority(t *testing.T) {
	updates := &mockUpdatesProvider{}
	exec := NewPulseToolExecutor(ExecutorConfig{UpdatesProvider: updates})
	exec.SetControlLevel(ControlLevelControlled)
	exec.ApplyExecutionProfile(ProfileInteractiveAssistant)
	exec.SetExecuteAuthority(true)
	exec.SetHostCommandAuthority(false)

	projected := exec.registry.ListTools(exec.invocationPolicy())
	assert.NotContains(t, dockerActions(projected), "check_updates")
	assert.Contains(t, dockerActions(projected), "updates", "cached update listings stay available")

	result, err := exec.registry.Execute(context.Background(), exec, agentcapabilities.PulseDockerToolName, map[string]interface{}{
		"action": "check_updates", "host": "tower",
	})
	require.NoError(t, err)
	assert.Contains(t, agentcapabilities.ToolResultText(result), "not allowed to run actions")
	updates.AssertNotCalled(t, "TriggerUpdateCheck")

	exec.SetHostCommandAuthority(true)
	assert.Contains(t, dockerActions(exec.registry.ListTools(exec.invocationPolicy())), "check_updates")
}
