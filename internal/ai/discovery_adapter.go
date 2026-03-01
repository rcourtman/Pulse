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
