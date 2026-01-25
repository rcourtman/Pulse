package ai

import (
	"context"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/aidiscovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// discoveryCommandAdapter adapts agentexec.Server to aidiscovery.CommandExecutor
type discoveryCommandAdapter struct {
	server *agentexec.Server
}

// newDiscoveryCommandAdapter creates a new adapter
func newDiscoveryCommandAdapter(server *agentexec.Server) *discoveryCommandAdapter {
	return &discoveryCommandAdapter{server: server}
}

// ExecuteCommand implements aidiscovery.CommandExecutor
func (a *discoveryCommandAdapter) ExecuteCommand(ctx context.Context, agentID string, cmd aidiscovery.ExecuteCommandPayload) (*aidiscovery.CommandResultPayload, error) {
	if a.server == nil {
		return &aidiscovery.CommandResultPayload{
			RequestID: cmd.RequestID,
			Success:   false,
			Error:     "agent server not available",
		}, nil
	}

	// Convert to agentexec types
	execCmd := agentexec.ExecuteCommandPayload{
		RequestID:  cmd.RequestID,
		Command:    cmd.Command,
		TargetType: cmd.TargetType,
		TargetID:   cmd.TargetID,
		Timeout:    cmd.Timeout,
	}

	result, err := a.server.ExecuteCommand(ctx, agentID, execCmd)
	if err != nil {
		return &aidiscovery.CommandResultPayload{
			RequestID: cmd.RequestID,
			Success:   false,
			Error:     err.Error(),
		}, nil
	}

	// Convert result back
	return &aidiscovery.CommandResultPayload{
		RequestID: result.RequestID,
		Success:   result.Success,
		Stdout:    result.Stdout,
		Stderr:    result.Stderr,
		ExitCode:  result.ExitCode,
		Error:     result.Error,
		Duration:  result.Duration,
	}, nil
}

// GetConnectedAgents implements aidiscovery.CommandExecutor
func (a *discoveryCommandAdapter) GetConnectedAgents() []aidiscovery.ConnectedAgent {
	if a.server == nil {
		return nil
	}

	agents := a.server.GetConnectedAgents()
	result := make([]aidiscovery.ConnectedAgent, len(agents))
	for i, agent := range agents {
		result[i] = aidiscovery.ConnectedAgent{
			AgentID:     agent.AgentID,
			Hostname:    agent.Hostname,
			Version:     agent.Version,
			Platform:    agent.Platform,
			Tags:        agent.Tags,
			ConnectedAt: agent.ConnectedAt,
		}
	}
	return result
}

// IsAgentConnected implements aidiscovery.CommandExecutor
func (a *discoveryCommandAdapter) IsAgentConnected(agentID string) bool {
	if a.server == nil {
		return false
	}
	for _, agent := range a.server.GetConnectedAgents() {
		if agent.AgentID == agentID {
			return true
		}
	}
	return false
}

// discoveryStateAdapter adapts StateProvider to aidiscovery.StateProvider
type discoveryStateAdapter struct {
	provider StateProvider
}

// newDiscoveryStateAdapter creates a new state adapter
func newDiscoveryStateAdapter(provider StateProvider) *discoveryStateAdapter {
	return &discoveryStateAdapter{provider: provider}
}

// GetState implements aidiscovery.StateProvider
func (a *discoveryStateAdapter) GetState() aidiscovery.StateSnapshot {
	if a.provider == nil {
		return aidiscovery.StateSnapshot{}
	}

	state := a.provider.GetState()

	// Convert VMs
	vms := make([]aidiscovery.VM, len(state.VMs))
	for i, vm := range state.VMs {
		vms[i] = aidiscovery.VM{
			VMID:     vm.VMID,
			Name:     vm.Name,
			Node:     vm.Node,
			Status:   vm.Status,
			Instance: vm.Instance,
		}
	}

	// Convert Containers
	containers := make([]aidiscovery.Container, len(state.Containers))
	for i, c := range state.Containers {
		containers[i] = aidiscovery.Container{
			VMID:     c.VMID,
			Name:     c.Name,
			Node:     c.Node,
			Status:   c.Status,
			Instance: c.Instance,
		}
	}

	// Convert Docker hosts
	dockerHosts := make([]aidiscovery.DockerHost, len(state.DockerHosts))
	for i, dh := range state.DockerHosts {
		containers := make([]aidiscovery.DockerContainer, len(dh.Containers))
		for j, dc := range dh.Containers {
			ports := make([]aidiscovery.DockerPort, len(dc.Ports))
			for k, p := range dc.Ports {
				ports[k] = aidiscovery.DockerPort{
					PublicPort:  p.PublicPort,
					PrivatePort: p.PrivatePort,
					Protocol:    p.Protocol,
				}
			}
			mounts := make([]aidiscovery.DockerMount, len(dc.Mounts))
			for k, m := range dc.Mounts {
				mounts[k] = aidiscovery.DockerMount{
					Source:      m.Source,
					Destination: m.Destination,
				}
			}
			containers[j] = aidiscovery.DockerContainer{
				ID:     dc.ID,
				Name:   dc.Name,
				Image:  dc.Image,
				Status: dc.Status,
				Ports:  ports,
				Labels: dc.Labels,
				Mounts: mounts,
			}
		}
		dockerHosts[i] = aidiscovery.DockerHost{
			AgentID:    dh.AgentID,
			Hostname:   dh.Hostname,
			Containers: containers,
		}
	}

	return aidiscovery.StateSnapshot{
		VMs:         vms,
		Containers:  containers,
		DockerHosts: dockerHosts,
	}
}

// StateProvider interface expected by the adapter (mirrors models.StateSnapshot fields)
type discoveryStateProviderInterface interface {
	GetState() models.StateSnapshot
}
