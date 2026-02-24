package tools

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveDockerContainer(t *testing.T) {
	t.Run("NoStateProvider", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{})
		_, _, err := exec.resolveDockerContainer("web", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read state not available")
	})

	t.Run("HostMatch", func(t *testing.T) {
		state := models.StateSnapshot{
			DockerHosts: []models.DockerHost{
				{
					ID:       "host1",
					Hostname: "dock1",
					Containers: []models.DockerContainer{
						{ID: "abc123", Name: "web"},
					},
				},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})

		container, host, err := exec.resolveDockerContainer("web", "dock1")
		require.NoError(t, err)
		assert.Equal(t, "web", container.Name)
		assert.Equal(t, "host1", host.ID)
	})

	t.Run("HostNotFound", func(t *testing.T) {
		state := models.StateSnapshot{
			DockerHosts: []models.DockerHost{
				{
					ID:       "host1",
					Hostname: "dock1",
					Containers: []models.DockerContainer{
						{ID: "abc123", Name: "web"},
					},
				},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})

		_, _, err := exec.resolveDockerContainer("web", "missing")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found on host")
	})

	t.Run("MultipleHosts", func(t *testing.T) {
		state := models.StateSnapshot{
			DockerHosts: []models.DockerHost{
				{
					ID:          "host1",
					DisplayName: "Dock One",
					Containers: []models.DockerContainer{
						{ID: "abcdef", Name: "web"},
					},
				},
				{
					ID:          "host2",
					DisplayName: "Dock Two",
					Containers: []models.DockerContainer{
						{ID: "abc999", Name: "web"},
					},
				},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})

		_, _, err := exec.resolveDockerContainer("web", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "multiple Docker hosts")
		assert.Contains(t, err.Error(), "Dock One")
	})

	t.Run("IDPrefixMatch", func(t *testing.T) {
		state := models.StateSnapshot{
			DockerHosts: []models.DockerHost{
				{
					ID:       "host1",
					Hostname: "dock1",
					Containers: []models.DockerContainer{
						{ID: "abcdef", Name: "web"},
					},
				},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})

		container, _, err := exec.resolveDockerContainer("abc", "")
		require.NoError(t, err)
		assert.Equal(t, "abcdef", container.ID)
	})

	t.Run("PreservesAgentID", func(t *testing.T) {
		state := models.StateSnapshot{
			DockerHosts: []models.DockerHost{
				{
					ID:       "host1",
					AgentID:  "agent1",
					Hostname: "dock1",
					Containers: []models.DockerContainer{
						{ID: "abc123", Name: "web"},
					},
				},
			},
		}
		agentSrv := &mockAgentServer{agents: []agentexec.ConnectedAgent{
			{AgentID: "agent1", Hostname: "node1"},
		}}
		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider: &mockStateProvider{state: state},
			AgentServer:   agentSrv,
		})

		_, host, err := exec.resolveDockerContainer("web", "dock1")
		require.NoError(t, err)
		assert.Equal(t, "agent1", host.AgentID)
		assert.Equal(t, "node1", exec.getAgentHostnameForDockerHost(host))
	})
}

func TestGetAgentHostnameForDockerHost(t *testing.T) {
	t.Run("NoAgentServer", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{})
		host := &models.DockerHost{Hostname: "dock1"}
		assert.Equal(t, "dock1", exec.getAgentHostnameForDockerHost(host))
	})

	t.Run("AgentIDMatch", func(t *testing.T) {
		agentSrv := &mockAgentServer{agents: []agentexec.ConnectedAgent{
			{AgentID: "agent1", Hostname: "node1"},
		}}
		exec := NewPulseToolExecutor(ExecutorConfig{AgentServer: agentSrv})
		host := &models.DockerHost{AgentID: "agent1", Hostname: "dock1"}
		assert.Equal(t, "node1", exec.getAgentHostnameForDockerHost(host))
	})

	t.Run("FallbackHostname", func(t *testing.T) {
		agentSrv := &mockAgentServer{agents: []agentexec.ConnectedAgent{
			{AgentID: "agent2", Hostname: "node2"},
		}}
		exec := NewPulseToolExecutor(ExecutorConfig{AgentServer: agentSrv})
		host := &models.DockerHost{AgentID: "agent1", Hostname: "dock1"}
		assert.Equal(t, "dock1", exec.getAgentHostnameForDockerHost(host))
	})
}
