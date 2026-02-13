package ai

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
)

const (
	defaultDiscoveryCommandTimeoutSeconds = 60
	minDiscoveryCommandTimeoutSeconds     = 1
)

// discoveryCommandAdapter adapts agentexec.Server to servicediscovery.CommandExecutor
type discoveryCommandAdapter struct {
	server *agentexec.Server
}

// newDiscoveryCommandAdapter creates a new adapter
func newDiscoveryCommandAdapter(server *agentexec.Server) *discoveryCommandAdapter {
	return &discoveryCommandAdapter{server: server}
}

// ExecuteCommand implements servicediscovery.CommandExecutor
func (a *discoveryCommandAdapter) ExecuteCommand(ctx context.Context, agentID string, cmd servicediscovery.ExecuteCommandPayload) (*servicediscovery.CommandResultPayload, error) {
	ctx = nonNilContext(ctx)
	execCmd := normalizeDiscoveryExecuteCommandPayload(ctx, cmd)

	if a.server == nil {
		return &servicediscovery.CommandResultPayload{
			RequestID: execCmd.RequestID,
			Success:   false,
			Error:     "agent server not available",
		}, nil
	}

	result, err := a.server.ExecuteCommand(ctx, agentID, execCmd)
	if err != nil {
		return &servicediscovery.CommandResultPayload{
			RequestID: execCmd.RequestID,
			Success:   false,
			Error:     err.Error(),
		}, nil
	}
	if result == nil {
		return &servicediscovery.CommandResultPayload{
			RequestID: execCmd.RequestID,
			Success:   false,
			Error:     "agent server returned no command result",
		}, nil
	}

	resultRequestID := result.RequestID
	if strings.TrimSpace(resultRequestID) == "" {
		resultRequestID = execCmd.RequestID
	}

	// Convert result back
	return &servicediscovery.CommandResultPayload{
		RequestID: resultRequestID,
		Success:   result.Success,
		Stdout:    result.Stdout,
		Stderr:    result.Stderr,
		ExitCode:  result.ExitCode,
		Error:     result.Error,
		Duration:  result.Duration,
	}, nil
}

// GetConnectedAgents implements servicediscovery.CommandExecutor
func (a *discoveryCommandAdapter) GetConnectedAgents() []servicediscovery.ConnectedAgent {
	if a.server == nil {
		return nil
	}

	agents := a.server.GetConnectedAgents()
	result := make([]servicediscovery.ConnectedAgent, len(agents))
	for i, agent := range agents {
		result[i] = servicediscovery.ConnectedAgent{
			AgentID:     agent.AgentID,
			Hostname:    agent.Hostname,
			Version:     agent.Version,
			Platform:    agent.Platform,
			Tags:        cloneStringSlice(agent.Tags),
			ConnectedAt: agent.ConnectedAt,
		}
	}
	return result
}

// IsAgentConnected implements servicediscovery.CommandExecutor
func (a *discoveryCommandAdapter) IsAgentConnected(agentID string) bool {
	if a.server == nil {
		return false
	}
	return a.server.IsAgentConnected(agentID)
}

// discoveryStateAdapter adapts StateProvider to servicediscovery.StateProvider
type discoveryStateAdapter struct {
	provider StateProvider
}

// newDiscoveryStateAdapter creates a new state adapter
func newDiscoveryStateAdapter(provider StateProvider) *discoveryStateAdapter {
	return &discoveryStateAdapter{provider: provider}
}

// GetState implements servicediscovery.StateProvider
func (a *discoveryStateAdapter) GetState() servicediscovery.StateSnapshot {
	if a.provider == nil {
		return servicediscovery.StateSnapshot{}
	}

	state := a.provider.GetState()

	// Convert VMs
	vms := make([]servicediscovery.VM, len(state.VMs))
	for i, vm := range state.VMs {
		vms[i] = servicediscovery.VM{
			VMID:        vm.VMID,
			Name:        vm.Name,
			Node:        vm.Node,
			Status:      vm.Status,
			Instance:    vm.Instance,
			IPAddresses: cloneStringSlice(vm.IPAddresses),
		}
	}

	// Convert Containers
	containers := make([]servicediscovery.Container, len(state.Containers))
	for i, c := range state.Containers {
		containers[i] = servicediscovery.Container{
			VMID:        c.VMID,
			Name:        c.Name,
			Node:        c.Node,
			Status:      c.Status,
			Instance:    c.Instance,
			IPAddresses: cloneStringSlice(c.IPAddresses),
		}
	}

	// Convert Docker hosts
	dockerHosts := make([]servicediscovery.DockerHost, len(state.DockerHosts))
	for i, dh := range state.DockerHosts {
		containers := make([]servicediscovery.DockerContainer, len(dh.Containers))
		for j, dc := range dh.Containers {
			ports := make([]servicediscovery.DockerPort, len(dc.Ports))
			for k, p := range dc.Ports {
				ports[k] = servicediscovery.DockerPort{
					PublicPort:  p.PublicPort,
					PrivatePort: p.PrivatePort,
					Protocol:    p.Protocol,
				}
			}
			mounts := make([]servicediscovery.DockerMount, len(dc.Mounts))
			for k, m := range dc.Mounts {
				mounts[k] = servicediscovery.DockerMount{
					Source:      m.Source,
					Destination: m.Destination,
				}
			}
			containers[j] = servicediscovery.DockerContainer{
				ID:     dc.ID,
				Name:   dc.Name,
				Image:  dc.Image,
				Status: dc.Status,
				Ports:  ports,
				Labels: cloneStringMap(dc.Labels),
				Mounts: mounts,
			}
		}
		dockerHosts[i] = servicediscovery.DockerHost{
			AgentID:    dh.AgentID,
			Hostname:   dh.Hostname,
			Containers: containers,
		}
	}

	// Convert Hosts
	hosts := make([]servicediscovery.Host, len(state.Hosts))
	for i, h := range state.Hosts {
		hosts[i] = servicediscovery.Host{
			ID:            h.ID,
			Hostname:      h.Hostname,
			DisplayName:   h.DisplayName,
			Platform:      h.Platform,
			OSName:        h.OSName,
			OSVersion:     h.OSVersion,
			KernelVersion: h.KernelVersion,
			Architecture:  h.Architecture,
			CPUCount:      h.CPUCount,
			Status:        h.Status,
			Tags:          cloneStringSlice(h.Tags),
		}
	}

	// Convert Nodes
	nodes := make([]servicediscovery.Node, len(state.Nodes))
	for i, n := range state.Nodes {
		nodes[i] = servicediscovery.Node{
			ID:                n.ID,
			Name:              n.Name,
			LinkedHostAgentID: n.LinkedHostAgentID,
		}
	}

	return servicediscovery.StateSnapshot{
		VMs:         vms,
		Containers:  containers,
		DockerHosts: dockerHosts,
		Hosts:       hosts,
		Nodes:       nodes,
	}
}

func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func normalizeDiscoveryExecuteCommandPayload(ctx context.Context, cmd servicediscovery.ExecuteCommandPayload) agentexec.ExecuteCommandPayload {
	requestID := strings.TrimSpace(cmd.RequestID)
	if requestID == "" {
		requestID = uuid.NewString()
	}

	timeoutSeconds := cmd.Timeout
	if timeoutSeconds <= 0 {
		timeoutSeconds = defaultDiscoveryCommandTimeoutSeconds
	}

	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			timeoutSeconds = minDiscoveryCommandTimeoutSeconds
		} else {
			maxTimeout := int((remaining + time.Second - 1) / time.Second)
			if maxTimeout < minDiscoveryCommandTimeoutSeconds {
				maxTimeout = minDiscoveryCommandTimeoutSeconds
			}
			if timeoutSeconds > maxTimeout {
				timeoutSeconds = maxTimeout
			}
		}
	}

	return agentexec.ExecuteCommandPayload{
		RequestID:  requestID,
		Command:    cmd.Command,
		TargetType: cmd.TargetType,
		TargetID:   cmd.TargetID,
		Timeout:    timeoutSeconds,
	}
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for k, v := range values {
		cloned[k] = v
	}
	return cloned
}
