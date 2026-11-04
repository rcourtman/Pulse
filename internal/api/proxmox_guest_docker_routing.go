package api

import (
	"context"
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// proxmoxGuestDockerCommandExecutor runs Docker discovery commands only through
// the agent the monitor has proven to run on the guest's own Proxmox node. It
// never looks an agent up by node name: another connection can have a node of
// that name, and pct exec would then inspect a different machine's guest under
// the same VMID.
func proxmoxGuestDockerCommandExecutor(server *agentexec.Server, monitor *monitoring.Monitor) monitoring.ProxmoxGuestCommandExecutor {
	return func(ctx context.Context, container models.Container, command string, timeout int) (string, int, error) {
		nodeAgent, proven := monitor.ProxmoxNodeAgentForContainer(container)
		if !proven {
			return "", -1, monitoring.ErrNoProvenProxmoxNodeAgent
		}
		organizationID := monitor.GetOrgID()
		agentID, connected := proxmoxNodeAgentCommandSession(server, monitor, organizationID, nodeAgent)
		if !connected {
			return "", -1, fmt.Errorf("no live command session for agent %s on Proxmox node %s", nodeAgent.AgentID, container.Node)
		}
		result, err := server.ExecuteCommand(agentexec.WithOrganizationID(ctx, organizationID), agentID, agentexec.ExecuteCommandPayload{
			RequestID: fmt.Sprintf("docker-check-%d", time.Now().UnixNano()),
			Command:   command,
			Timeout:   timeout,
			Trusted:   true,
		})
		if err != nil {
			return "", -1, err
		}
		return result.Stdout + result.Stderr, result.ExitCode, nil
	}
}

// proxmoxNodeAgentCommandSession finds the live command session of one proven
// agent. A session normally registers under the agent's host ID. One bound to an
// earlier ID is found through the agent's token, and must still present the
// agent's hostname and be one no other known agent could own.
func proxmoxNodeAgentCommandSession(server *agentexec.Server, monitor *monitoring.Monitor, organizationID string, agent monitoring.ProxmoxNodeAgent) (string, bool) {
	if agentID, ok := server.GetAgentForIdentityForOrganization(organizationID, agent.AgentID, agent.Hostname); ok {
		return agentID, true
	}
	if agent.TokenID == "" {
		return "", false
	}
	sessionID, ok := server.GetAgentForTokenForOrganization(organizationID, agent.TokenID)
	if !ok {
		return "", false
	}
	if _, ok := server.GetAgentForIdentityForOrganization(organizationID, sessionID, agent.Hostname); !ok {
		return "", false
	}
	if !monitor.ProxmoxNodeAgentOwnsTokenSession(agent, sessionID) {
		return "", false
	}
	return sessionID, true
}
