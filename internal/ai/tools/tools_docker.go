package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rs/zerolog/log"
)

// registerDockerTools registers the pulse_docker tool
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
		return e.executeListDockerUpdates(ctx, args)
	case "check_updates":
		return e.executeCheckDockerUpdates(ctx, args)
	case "update":
		return e.executeUpdateDockerContainer(ctx, args)
	case "services":
		return e.executeListDockerServices(ctx, args)
	case "tasks":
		return e.executeListDockerTasks(ctx, args)
	case "swarm":
		return e.executeGetSwarmStatus(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("unknown action: %s. Use: control, updates, check_updates, update, services, tasks, swarm", action)), nil
	}
}

// executeDockerControl handles start/stop/restart of Docker containers
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

// ========== Docker Updates Handler Implementations ==========

func (e *PulseToolExecutor) executeListDockerUpdates(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.updatesProvider == nil {
		return NewTextResult("Docker update information not available. Ensure updates provider is configured."), nil
	}

	hostFilter, _ := args["host"].(string)

	// Resolve host name to ID if needed
	hostID := e.resolveDockerHostID(hostFilter)

	updates := e.updatesProvider.GetPendingUpdates(hostID)

	// Ensure non-nil slice
	if updates == nil {
		updates = []ContainerUpdateInfo{}
	}

	response := DockerUpdatesResponse{
		Updates: updates,
		Total:   len(updates),
		HostID:  hostID,
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeCheckDockerUpdates(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.updatesProvider == nil {
		return NewTextResult("Docker update checking not available. Ensure updates provider is configured."), nil
	}

	hostArg, _ := args["host"].(string)
	if hostArg == "" {
		return NewErrorResult(fmt.Errorf("host is required")), nil
	}

	// Resolve host name to ID
	hostID := e.resolveDockerHostID(hostArg)
	if hostID == "" {
		return NewTextResult(fmt.Sprintf("Docker host '%s' not found.", hostArg)), nil
	}

	hostName := e.getDockerHostName(hostID)

	// Trigger the update check
	cmdStatus, err := e.updatesProvider.TriggerUpdateCheck(hostID)
	if err != nil {
		return NewTextResult(fmt.Sprintf("Failed to trigger update check: %v", err)), nil
	}

	response := DockerCheckUpdatesResponse{
		Success:   true,
		HostID:    hostID,
		HostName:  hostName,
		CommandID: cmdStatus.ID,
		Message:   "Update check command queued. Results will be available after the next agent report cycle (~30 seconds).",
		Command:   cmdStatus,
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeUpdateDockerContainer(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.updatesProvider == nil {
		return NewTextResult("Docker update functionality not available. Ensure updates provider is configured."), nil
	}

	containerArg, _ := args["container"].(string)
	hostArg, _ := args["host"].(string)

	if containerArg == "" {
		return NewErrorResult(fmt.Errorf("container is required")), nil
	}
	if hostArg == "" {
		return NewErrorResult(fmt.Errorf("host is required")), nil
	}

	// Check if update actions are enabled
	if !e.updatesProvider.IsUpdateActionsEnabled() {
		return NewTextResult("Docker container updates are disabled by server configuration. Set PULSE_DISABLE_DOCKER_UPDATE_ACTIONS=false or enable in Settings to allow updates."), nil
	}

	// Resolve container and host
	container, dockerHost, err := e.resolveDockerContainer(containerArg, hostArg)
	if err != nil {
		return NewTextResult(fmt.Sprintf("Could not find container '%s' on host '%s': %v", containerArg, hostArg, err)), nil
	}

	containerName := trimContainerName(container.Name)

	// Controlled mode - require approval
	if e.controlLevel == ControlLevelControlled {
		command := fmt.Sprintf("docker update %s", containerName)
		agentHostname := e.getAgentHostnameForDockerHost(dockerHost)
		approvalID := createApprovalRecord(command, "docker", container.ID, agentHostname, fmt.Sprintf("Update container %s to latest image", containerName))
		return NewTextResult(formatDockerUpdateApprovalNeeded(containerName, dockerHost.Hostname, approvalID)), nil
	}

	// Autonomous mode - execute directly
	cmdStatus, err := e.updatesProvider.UpdateContainer(dockerHost.ID, container.ID, containerName)
	if err != nil {
		return NewTextResult(fmt.Sprintf("Failed to queue update command: %v", err)), nil
	}

	response := DockerUpdateContainerResponse{
		Success:       true,
		HostID:        dockerHost.ID,
		ContainerID:   container.ID,
		ContainerName: containerName,
		CommandID:     cmdStatus.ID,
		Message:       fmt.Sprintf("Update command queued for container '%s'. The agent will pull the latest image and recreate the container.", containerName),
		Command:       cmdStatus,
	}

	return NewJSONResult(response), nil
}

// Helper methods for Docker updates

func (e *PulseToolExecutor) resolveDockerHostID(hostArg string) string {
	if hostArg == "" {
		return ""
	}
	if e.stateProvider == nil {
		return hostArg
	}

	state := e.stateProvider.GetState()
	for _, host := range state.DockerHosts {
		if host.ID == hostArg || host.Hostname == hostArg || host.DisplayName == hostArg {
			return host.ID
		}
	}
	return hostArg // Return as-is if not found (provider will handle error)
}

func (e *PulseToolExecutor) getDockerHostName(hostID string) string {
	if e.stateProvider == nil {
		return hostID
	}

	state := e.stateProvider.GetState()
	for _, host := range state.DockerHosts {
		if host.ID == hostID {
			if host.DisplayName != "" {
				return host.DisplayName
			}
			return host.Hostname
		}
	}
	return hostID
}

func formatDockerUpdateApprovalNeeded(containerName, hostName, approvalID string) string {
	payload := map[string]interface{}{
		"type":           "approval_required",
		"approval_id":    approvalID,
		"container_name": containerName,
		"docker_host":    hostName,
		"action":         "update",
		"command":        fmt.Sprintf("docker update %s (pull latest + recreate)", containerName),
		"how_to_approve": "Click the approval button in the chat to execute this update.",
		"do_not_retry":   true,
	}
	b, _ := json.Marshal(payload)
	return "APPROVAL_REQUIRED: " + string(b)
}

func trimLeadingSlash(name string) string {
	if len(name) > 0 && name[0] == '/' {
		return name[1:]
	}
	return name
}

// ========== Docker Swarm Handler Implementations ==========

func (e *PulseToolExecutor) executeGetSwarmStatus(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	hostArg, _ := args["host"].(string)
	if hostArg == "" {
		return NewErrorResult(fmt.Errorf("host is required")), nil
	}

	state := e.stateProvider.GetState()

	for _, host := range state.DockerHosts {
		if host.ID == hostArg || host.Hostname == hostArg || host.DisplayName == hostArg || host.CustomDisplayName == hostArg {
			if host.Swarm == nil {
				return NewTextResult(fmt.Sprintf("Docker host '%s' is not part of a Swarm cluster.", host.Hostname)), nil
			}

			response := SwarmStatusResponse{
				Host: host.Hostname,
				Status: DockerSwarmSummary{
					NodeID:           host.Swarm.NodeID,
					NodeRole:         host.Swarm.NodeRole,
					LocalState:       host.Swarm.LocalState,
					ControlAvailable: host.Swarm.ControlAvailable,
					ClusterID:        host.Swarm.ClusterID,
					ClusterName:      host.Swarm.ClusterName,
					Error:            host.Swarm.Error,
				},
			}

			return NewJSONResult(response), nil
		}
	}

	return NewTextResult(fmt.Sprintf("Docker host '%s' not found.", hostArg)), nil
}

func (e *PulseToolExecutor) executeListDockerServices(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	hostArg, _ := args["host"].(string)
	if hostArg == "" {
		return NewErrorResult(fmt.Errorf("host is required")), nil
	}

	stackFilter, _ := args["stack"].(string)

	state := e.stateProvider.GetState()

	for _, host := range state.DockerHosts {
		if host.ID == hostArg || host.Hostname == hostArg || host.DisplayName == hostArg || host.CustomDisplayName == hostArg {
			if len(host.Services) == 0 {
				return NewTextResult(fmt.Sprintf("No Docker services found on host '%s'. The host may not be a Swarm manager.", host.Hostname)), nil
			}

			var services []DockerServiceSummary
			filteredCount := 0

			for _, svc := range host.Services {
				if stackFilter != "" && svc.Stack != stackFilter {
					continue
				}

				filteredCount++

				updateStatus := ""
				if svc.UpdateStatus != nil {
					updateStatus = svc.UpdateStatus.State
				}

				services = append(services, DockerServiceSummary{
					ID:           svc.ID,
					Name:         svc.Name,
					Stack:        svc.Stack,
					Image:        svc.Image,
					Mode:         svc.Mode,
					DesiredTasks: svc.DesiredTasks,
					RunningTasks: svc.RunningTasks,
					UpdateStatus: updateStatus,
				})
			}

			if services == nil {
				services = []DockerServiceSummary{}
			}

			response := DockerServicesResponse{
				Host:     host.Hostname,
				Services: services,
				Total:    len(host.Services),
				Filtered: filteredCount,
			}

			return NewJSONResult(response), nil
		}
	}

	return NewTextResult(fmt.Sprintf("Docker host '%s' not found.", hostArg)), nil
}

func (e *PulseToolExecutor) executeListDockerTasks(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	hostArg, _ := args["host"].(string)
	if hostArg == "" {
		return NewErrorResult(fmt.Errorf("host is required")), nil
	}

	serviceFilter, _ := args["service"].(string)

	state := e.stateProvider.GetState()

	for _, host := range state.DockerHosts {
		if host.ID == hostArg || host.Hostname == hostArg || host.DisplayName == hostArg || host.CustomDisplayName == hostArg {
			if len(host.Tasks) == 0 {
				return NewTextResult(fmt.Sprintf("No Docker tasks found on host '%s'. The host may not be a Swarm manager.", host.Hostname)), nil
			}

			var tasks []DockerTaskSummary

			for _, task := range host.Tasks {
				if serviceFilter != "" && task.ServiceID != serviceFilter && task.ServiceName != serviceFilter {
					continue
				}

				tasks = append(tasks, DockerTaskSummary{
					ID:           task.ID,
					ServiceName:  task.ServiceName,
					NodeName:     task.NodeName,
					DesiredState: task.DesiredState,
					CurrentState: task.CurrentState,
					Error:        task.Error,
					StartedAt:    task.StartedAt,
				})
			}

			if tasks == nil {
				tasks = []DockerTaskSummary{}
			}

			response := DockerTasksResponse{
				Host:    host.Hostname,
				Service: serviceFilter,
				Tasks:   tasks,
				Total:   len(tasks),
			}

			return NewJSONResult(response), nil
		}
	}

	return NewTextResult(fmt.Sprintf("Docker host '%s' not found.", hostArg)), nil
}
