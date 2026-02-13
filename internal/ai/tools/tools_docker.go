package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/safety"
	"github.com/rs/zerolog/log"
)

const (
	// dockerUpdateQueueMaxAttempts bounds retries for queuing update-related commands.
	dockerUpdateQueueMaxAttempts = 3

	// dockerUpdateQueueRetryBaseDelay is the initial exponential-backoff delay for retryable queue errors.
	dockerUpdateQueueRetryBaseDelay = 25 * time.Millisecond

	// dockerUpdateQueueRetryMaxDelay caps exponential backoff for queue retries.
	dockerUpdateQueueRetryMaxDelay = 250 * time.Millisecond
)

var dockerUpdateQueueSleepFn = sleepWithContext

// registerDockerTools registers the pulse_docker tool
func (e *PulseToolExecutor) registerDockerTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_docker",
			Description: `Manage Docker containers, updates, and Swarm services. Actions: control, updates, check_updates, update, services, tasks, swarm.`,
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

	if redacted, n := safety.RedactSensitiveText(output); n > 0 {
		output = redacted + fmt.Sprintf("\n\n[redacted %d sensitive value(s)]", n)
	}

	verify := e.verifyDockerContainerState(ctx, routing, container.ID, operation)
	verify["ok"] = result.ExitCode == 0

	response := map[string]interface{}{
		"success":      result.ExitCode == 0,
		"action":       "control",
		"operation":    operation,
		"container":    container.Name,
		"container_id": container.ID,
		"host":         dockerHost.Hostname,
		"command":      command,
		"exit_code":    result.ExitCode,
		"output":       output,
		"verification": verify,
	}

	return NewJSONResultWithIsError(response, result.ExitCode != 0), nil
}

// ========== Docker Updates Handler Implementations ==========

func (e *PulseToolExecutor) verifyDockerContainerState(ctx context.Context, routing CommandRoutingResult, containerID, operation string) map[string]interface{} {
	expectRunning := false
	switch operation {
	case "start", "restart":
		expectRunning = true
	case "stop":
		expectRunning = false
	}

	inspectCmd := fmt.Sprintf("docker inspect -f '{{.State.Status}} {{.State.Running}}' %s", shellEscape(containerID))

	var lastOut string
	var lastExit int
	for attempt := 1; attempt <= 3; attempt++ {
		res, err := e.agentServer.ExecuteCommand(ctx, routing.AgentID, agentexec.ExecuteCommandPayload{
			Command:    inspectCmd,
			TargetType: routing.TargetType,
			TargetID:   routing.TargetID,
		})
		if err != nil {
			return map[string]interface{}{"confirmed": false, "method": "docker_inspect", "command": inspectCmd, "note": err.Error()}
		}
		lastExit = res.ExitCode
		lastOut = strings.TrimSpace(res.Stdout + "\n" + res.Stderr)

		fields := strings.Fields(strings.ToLower(lastOut))
		observedRunning := false
		observedStatus := ""
		if len(fields) >= 1 {
			observedStatus = fields[0]
		}
		if len(fields) >= 2 {
			observedRunning = fields[1] == "true"
		}

		if res.ExitCode == 0 && observedStatus != "" && observedRunning == expectRunning {
			return map[string]interface{}{
				"confirmed": true,
				"method":    "docker_inspect",
				"command":   inspectCmd,
				"expected":  map[string]interface{}{"running": expectRunning},
				"observed":  map[string]interface{}{"status": observedStatus, "running": observedRunning},
			}
		}

		// Allow a brief settle window for restart/start/stop to propagate.
		waitTimer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !waitTimer.Stop() {
				select {
				case <-waitTimer.C:
				default:
				}
			}
			return map[string]interface{}{"confirmed": false, "method": "docker_inspect", "command": inspectCmd, "note": "context canceled", "raw": lastOut, "exit_code": lastExit}
		case <-waitTimer.C:
		}
	}

	return map[string]interface{}{
		"confirmed": false,
		"method":    "docker_inspect",
		"command":   inspectCmd,
		"expected":  map[string]interface{}{"running": expectRunning},
		"raw":       lastOut,
		"exit_code": lastExit,
	}
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	if ctx == nil {
		time.Sleep(duration)
		return nil
	}

	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func dockerUpdateQueueRetryDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return dockerUpdateQueueRetryBaseDelay
	}

	delay := dockerUpdateQueueRetryBaseDelay * time.Duration(1<<(attempt-1))
	if delay > dockerUpdateQueueRetryMaxDelay {
		return dockerUpdateQueueRetryMaxDelay
	}
	return delay
}

func isTransientUpdateQueueError(err error) bool {
	if err == nil {
		return false
	}

	if isTransientError(err) {
		return true
	}

	msg := strings.ToLower(err.Error())
	transientPatterns := []string{
		"temporary failure",
		"queue full",
		"resource busy",
		"database is locked",
		"deadlock",
		"unexpected eof",
		"eof",
		"try again",
	}

	for _, pattern := range transientPatterns {
		if strings.Contains(msg, pattern) {
			return true
		}
	}

	return false
}

func (e *PulseToolExecutor) queueDockerUpdateCommandWithRetry(ctx context.Context, operation string, run func() (DockerCommandStatus, error)) (DockerCommandStatus, error) {
	var lastErr error

	for attempt := 1; attempt <= dockerUpdateQueueMaxAttempts; attempt++ {
		status, err := run()
		if err == nil {
			return status, nil
		}

		lastErr = err
		if !isTransientUpdateQueueError(err) {
			return DockerCommandStatus{}, err
		}
		if attempt == dockerUpdateQueueMaxAttempts {
			break
		}

		backoff := dockerUpdateQueueRetryDelay(attempt)
		log.Warn().
			Err(err).
			Str("operation", operation).
			Int("attempt", attempt).
			Dur("retry_in", backoff).
			Msg("[pulse_docker] transient update queue failure, retrying")

		if sleepErr := dockerUpdateQueueSleepFn(ctx, backoff); sleepErr != nil {
			return DockerCommandStatus{}, fmt.Errorf("%s canceled while waiting to retry: %w", operation, sleepErr)
		}
	}

	if lastErr == nil {
		return DockerCommandStatus{}, fmt.Errorf("%s failed without an error", operation)
	}

	return DockerCommandStatus{}, fmt.Errorf("%s failed after %d attempts: %w", operation, dockerUpdateQueueMaxAttempts, lastErr)
}

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

func (e *PulseToolExecutor) executeCheckDockerUpdates(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
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
	cmdStatus, err := e.queueDockerUpdateCommandWithRetry(ctx, "trigger update check", func() (DockerCommandStatus, error) {
		return e.updatesProvider.TriggerUpdateCheck(hostID)
	})
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
	cmdStatus, err := e.queueDockerUpdateCommandWithRetry(ctx, "queue container update", func() (DockerCommandStatus, error) {
		return e.updatesProvider.UpdateContainer(dockerHost.ID, container.ID, containerName)
	})
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

	rs, err := e.readStateForControl()
	if err != nil {
		return hostArg
	}
	for _, host := range rs.DockerHosts() {
		if host.ID() == hostArg || host.HostSourceID() == hostArg || host.Hostname() == hostArg || host.Name() == hostArg {
			// Return source ID when available (updates provider uses raw model IDs)
			if sid := host.HostSourceID(); sid != "" {
				return sid
			}
			return host.ID()
		}
	}
	return hostArg // Return as-is if not found
}

func (e *PulseToolExecutor) getDockerHostName(hostID string) string {
	rs, err := e.readStateForControl()
	if err != nil {
		return hostID
	}
	for _, host := range rs.DockerHosts() {
		if host.ID() == hostID || host.HostSourceID() == hostID {
			if host.Name() != "" {
				return host.Name()
			}
			if host.Hostname() != "" {
				return host.Hostname()
			}
			return host.ID()
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
