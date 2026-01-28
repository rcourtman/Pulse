package tools

import (
	"context"
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rs/zerolog/log"
)

// registerDockerTools registers the consolidated pulse_docker tool
func (e *PulseToolExecutor) registerDockerTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_docker",
			Description: `Manage Docker containers, updates, and Swarm services.

Actions:
- control: Start, stop, or restart containers
- updates: List containers with pending image updates
- check_updates: Trigger update check on a host
- update: Update a container to latest image (requires control permission)
- services: List Docker Swarm services
- tasks: List Docker Swarm tasks
- swarm: Get Swarm cluster status

To check Docker container logs or run commands inside containers, use pulse_control with type="command":
  command="docker logs jellyfin --tail 100"
  command="docker exec jellyfin cat /config/log/log.txt"

Examples:
- Restart container: action="control", container="nginx", operation="restart"
- List updates: action="updates", host="Tower"
- Update container: action="update", container="nginx", host="Tower"`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"action": {
						Type:        "string",
						Description: "Docker action to perform",
						Enum:        []string{"control", "updates", "check_updates", "update", "services", "tasks", "swarm"},
					},
					"container": {
						Type:        "string",
						Description: "Container name or ID (for control, update)",
					},
					"host": {
						Type:        "string",
						Description: "Docker host name or ID",
					},
					"operation": {
						Type:        "string",
						Description: "Control operation: start, stop, restart (for action: control)",
						Enum:        []string{"start", "stop", "restart"},
					},
					"service": {
						Type:        "string",
						Description: "Filter by service name or ID (for tasks)",
					},
					"stack": {
						Type:        "string",
						Description: "Filter by stack name (for services)",
					},
				},
				Required: []string{"action"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeDocker(ctx, args)
		},
	})
}

// executeDocker routes to the appropriate docker handler based on action
func (e *PulseToolExecutor) executeDocker(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	action, _ := args["action"].(string)
	switch action {
	case "control":
		return e.executeDockerControl(ctx, args)
	case "updates":
		// Uses existing function from tools_infrastructure.go
		return e.executeListDockerUpdates(ctx, args)
	case "check_updates":
		// Uses existing function from tools_infrastructure.go
		return e.executeCheckDockerUpdates(ctx, args)
	case "update":
		// Uses existing function from tools_infrastructure.go
		return e.executeUpdateDockerContainer(ctx, args)
	case "services":
		// Uses existing function from tools_infrastructure.go
		return e.executeListDockerServices(ctx, args)
	case "tasks":
		// Uses existing function from tools_infrastructure.go
		return e.executeListDockerTasks(ctx, args)
	case "swarm":
		// Uses existing function from tools_infrastructure.go
		return e.executeGetSwarmStatus(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("unknown action: %s. Use: control, updates, check_updates, update, services, tasks, swarm", action)), nil
	}
}

// executeDockerControl handles start/stop/restart of Docker containers
// This is a new consolidated handler that merges pulse_control_docker functionality
func (e *PulseToolExecutor) executeDockerControl(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	containerName, _ := args["container"].(string)
	hostName, _ := args["host"].(string)
	operation, _ := args["operation"].(string)

	if containerName == "" {
		return NewErrorResult(fmt.Errorf("container name is required")), nil
	}
	if operation == "" {
		return NewErrorResult(fmt.Errorf("operation is required (start, stop, restart)")), nil
	}

	validOperations := map[string]bool{"start": true, "stop": true, "restart": true}
	if !validOperations[operation] {
		return NewErrorResult(fmt.Errorf("invalid operation: %s. Use start, stop, or restart", operation)), nil
	}

	// Check if read-only mode
	if e.controlLevel == ControlLevelReadOnly {
		return NewTextResult("Docker control actions are not available in read-only mode."), nil
	}

	// Check if this is a pre-approved execution
	preApproved := isPreApproved(args)

	container, dockerHost, err := e.resolveDockerContainer(containerName, hostName)
	if err != nil {
		return NewTextResult(fmt.Sprintf("Could not find Docker container '%s': %v", containerName, err)), nil
	}

	command := fmt.Sprintf("docker %s %s", operation, container.Name)

	// Get the agent hostname for approval records
	agentHostname := e.getAgentHostnameForDockerHost(dockerHost)

	// Skip approval checks if pre-approved
	if !preApproved && e.policy != nil {
		decision := e.policy.Evaluate(command)
		if decision == agentexec.PolicyBlock {
			return NewTextResult(formatPolicyBlocked(command, "This command is blocked by security policy")), nil
		}
		if decision == agentexec.PolicyRequireApproval && !e.isAutonomous {
			approvalID := createApprovalRecord(command, "docker", container.Name, agentHostname, fmt.Sprintf("%s Docker container %s", operation, container.Name))
			return NewTextResult(formatDockerApprovalNeeded(container.Name, dockerHost.Hostname, operation, command, approvalID)), nil
		}
	}

	// Check control level
	if !preApproved && e.controlLevel == ControlLevelControlled {
		approvalID := createApprovalRecord(command, "docker", container.Name, agentHostname, fmt.Sprintf("%s Docker container %s", operation, container.Name))
		return NewTextResult(formatDockerApprovalNeeded(container.Name, dockerHost.Hostname, operation, command, approvalID)), nil
	}

	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	// Resolve the Docker host to the correct agent and routing info (with full provenance)
	routing := e.resolveDockerHostRoutingFull(dockerHost)
	if routing.AgentID == "" {
		if routing.TargetType == "container" || routing.TargetType == "vm" {
			return NewTextResult(fmt.Sprintf("Docker host '%s' is a %s but no agent is available on its Proxmox host. Install Pulse Unified Agent on the Proxmox node.", dockerHost.Hostname, routing.TargetType)), nil
		}
		return NewTextResult(fmt.Sprintf("No agent available on Docker host '%s'. Install Pulse Unified Agent on the host to enable control.", dockerHost.Hostname)), nil
	}

	log.Debug().
		Str("docker_host", dockerHost.Hostname).
		Str("agent_id", routing.AgentID).
		Str("agent_host", routing.AgentHostname).
		Str("resolved_kind", routing.ResolvedKind).
		Str("resolved_node", routing.ResolvedNode).
		Str("transport", routing.Transport).
		Str("target_type", routing.TargetType).
		Str("target_id", routing.TargetID).
		Msg("[pulse_docker] Routing docker command execution")

	result, err := e.agentServer.ExecuteCommand(ctx, routing.AgentID, agentexec.ExecuteCommandPayload{
		Command:    command,
		TargetType: routing.TargetType,
		TargetID:   routing.TargetID,
	})
	if err != nil {
		return NewErrorResult(err), nil
	}

	output := result.Stdout
	if result.Stderr != "" {
		output += "\n" + result.Stderr
	}

	if result.ExitCode == 0 {
		return NewTextResult(fmt.Sprintf("Successfully executed 'docker %s' on container '%s' (host: %s). State updates in ~10s.\n%s", operation, container.Name, dockerHost.Hostname, output)), nil
	}

	return NewTextResult(fmt.Sprintf("Command failed (exit code %d):\n%s", result.ExitCode, output)), nil
}
