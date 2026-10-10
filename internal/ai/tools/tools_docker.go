package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
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
	e.registry.registerBuiltin(RegisteredTool{
		Definition: Tool{
			Name:        agentcapabilities.PulseDockerToolName,
			Description: `Read host-scoped Docker update posture and Swarm state. Every advertised action requires the Docker host name or ID; use the host identity from the current resource context. Container control is planned through pulse_control using a canonical resource capability. Container updates remain unavailable until durable delivery and compensation contracts are complete.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"action": {
						Type:        "string",
						Description: "Docker action to perform; every action is scoped to the required host",
						Enum:        []string{"updates", "check_updates", "services", "tasks", "swarm"},
					},
					"container": {
						Type:        "string",
						Description: "Container name or ID (for control, update)",
					},
					"host": {
						Type:        "string",
						Description: "Required Docker host name or ID for every action",
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
				Required: []string{"action", "host"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeDocker(ctx, args)
		},
		Governance: ToolGovernance{
			ActionMode:     ToolActionRead,
			ApprovalPolicy: ToolApprovalScopeOnly,
			Summary:        "Reads Docker state; mutations use typed resource actions or remain denied.",
		},
	})
}

// executeDocker routes to the appropriate docker handler based on action
func (e *PulseToolExecutor) executeDocker(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	action, _ := args["action"].(string)
	switch action {
	case "updates":
		return e.executeListDockerUpdates(ctx, args)
	case "check_updates":
		return e.executeCheckDockerUpdates(ctx, args)
	case "services":
		return e.executeListDockerServices(ctx, args)
	case "tasks":
		return e.executeListDockerTasks(ctx, args)
	case "swarm":
		return e.executeGetSwarmStatus(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("Docker action %q is unavailable; use read actions or pulse_control type=resource for advertised lifecycle capabilities", action)), nil
	}
}

// ========== Docker Updates Handler Implementations ==========

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

	response := EmptyDockerUpdatesResponse()
	response.Updates = updates
	response.Total = len(updates)
	response.TargetID = hostID

	return NewJSONResult(response.NormalizeCollections()), nil
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
		TargetID:  hostID,
		HostName:  hostName,
		CommandID: cmdStatus.ID,
		Message:   "Update check command queued. Results will be available after the next agent report cycle (~30 seconds).",
		Command:   cmdStatus,
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

// ========== Docker Swarm Handler Implementations ==========

func (e *PulseToolExecutor) executeGetSwarmStatus(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	hostArg, _ := args["host"].(string)
	if hostArg == "" {
		return NewErrorResult(fmt.Errorf("host is required")), nil
	}

	rs, err := e.readStateForControl()
	if err != nil {
		return NewErrorResult(err), nil
	}

	for _, host := range rs.DockerHosts() {
		if host.ID() == hostArg || host.Hostname() == hostArg {
			swarm := host.Swarm()
			if swarm == nil {
				return NewTextResult(fmt.Sprintf("Docker host '%s' is not part of a Swarm cluster.", host.Hostname())), nil
			}

			response := SwarmStatusResponse{
				Host: host.Hostname(),
				Status: DockerSwarmSummary{
					NodeID:           swarm.NodeID,
					NodeRole:         swarm.NodeRole,
					LocalState:       swarm.LocalState,
					ControlAvailable: swarm.ControlAvailable,
					ClusterID:        swarm.ClusterID,
					ClusterName:      swarm.ClusterName,
					Error:            swarm.Error,
				},
			}

			return NewJSONResult(response), nil
		}
	}

	return NewTextResult(fmt.Sprintf("Docker host '%s' not found.", hostArg)), nil
}

func (e *PulseToolExecutor) executeListDockerServices(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	hostArg, _ := args["host"].(string)
	if hostArg == "" {
		return NewErrorResult(fmt.Errorf("host is required")), nil
	}

	stackFilter, _ := args["stack"].(string)

	rs, err := e.readStateForControl()
	if err != nil {
		return NewErrorResult(err), nil
	}

	for _, host := range rs.DockerHosts() {
		if host.ID() == hostArg || host.Hostname() == hostArg {
			services := host.Services()
			if len(services) == 0 {
				return NewTextResult(fmt.Sprintf("No Docker Swarm services found on host '%s'. This result covers Swarm services only; it does not mean the host has no ordinary Docker containers or container dependencies. Use pulse_query topology or search to inspect those resources. The host may not be a Swarm manager.", host.Hostname())), nil
			}

			var summaries []DockerServiceSummary
			filteredCount := 0

			for _, svc := range services {
				if stackFilter != "" && svc.Stack != stackFilter {
					continue
				}

				filteredCount++

				updateStatus := ""
				if svc.UpdateStatus != nil {
					updateStatus = svc.UpdateStatus.State
				}

				summaries = append(summaries, DockerServiceSummary{
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

			response := EmptyDockerServicesResponse()
			response.Host = host.Hostname()
			response.Services = summaries
			response.Total = len(services)
			response.Filtered = filteredCount

			return NewJSONResult(response.NormalizeCollections()), nil
		}
	}

	return NewTextResult(fmt.Sprintf("Docker host '%s' not found.", hostArg)), nil
}

func (e *PulseToolExecutor) executeListDockerTasks(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	hostArg, _ := args["host"].(string)
	if hostArg == "" {
		return NewErrorResult(fmt.Errorf("host is required")), nil
	}

	serviceFilter, _ := args["service"].(string)

	rs, err := e.readStateForControl()
	if err != nil {
		return NewErrorResult(err), nil
	}

	for _, host := range rs.DockerHosts() {
		if host.ID() == hostArg || host.Hostname() == hostArg {
			tasks := host.Tasks()
			if len(tasks) == 0 {
				return NewTextResult(fmt.Sprintf("No Docker tasks found on host '%s'. The host may not be a Swarm manager.", host.Hostname())), nil
			}

			var summaries []DockerTaskSummary

			for _, task := range tasks {
				if serviceFilter != "" && task.ServiceID != serviceFilter && task.ServiceName != serviceFilter {
					continue
				}

				summaries = append(summaries, DockerTaskSummary{
					ID:           task.ID,
					ServiceName:  task.ServiceName,
					NodeName:     task.NodeName,
					DesiredState: task.DesiredState,
					CurrentState: task.CurrentState,
					Error:        task.Error,
					StartedAt:    task.StartedAt,
				})
			}

			response := EmptyDockerTasksResponse()
			response.Host = host.Hostname()
			response.Service = serviceFilter
			response.Tasks = summaries
			response.Total = len(summaries)

			return NewJSONResult(response.NormalizeCollections()), nil
		}
	}

	return NewTextResult(fmt.Sprintf("Docker host '%s' not found.", hostArg)), nil
}
