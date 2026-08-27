package dockeragent

import (
	"context"
	"fmt"
	"strings"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

// InspectDockerContainerLifecycle reads the exact daemon state used to bind a
// typed lifecycle action. Unified agents expose this narrow bridge to the host
// command channel so preflight and verification use the already-connected
// Docker / Podman API rather than assuming an external CLI exists.
func (a *Agent) InspectDockerContainerLifecycle(ctx context.Context, runtime, containerID string) (agentexec.DockerContainerLifecycleSnapshot, error) {
	if err := a.validateLifecycleRuntime(runtime); err != nil {
		return agentexec.DockerContainerLifecycleSnapshot{}, err
	}
	containerID = strings.ToLower(strings.TrimSpace(containerID))
	if containerID == "" {
		return agentexec.DockerContainerLifecycleSnapshot{}, fmt.Errorf("container id is required")
	}
	inspect, err := dockerCallWithRetry(ctx, dockerUpdateCallTimeout, func(callCtx context.Context) (containertypes.InspectResponse, error) {
		return a.docker.ContainerInspect(callCtx, containerID)
	})
	if err != nil {
		return agentexec.DockerContainerLifecycleSnapshot{}, fmt.Errorf("container inspect unavailable: %w", annotateDockerConnectionError(err))
	}
	if inspect.State == nil {
		return agentexec.DockerContainerLifecycleSnapshot{}, fmt.Errorf("container inspect returned no state")
	}
	health := agentexec.DockerContainerHealthNone
	if inspect.State.Health != nil {
		health = strings.ToLower(strings.TrimSpace(string(inspect.State.Health.Status)))
		if !agentexec.IsDockerContainerHealth(health) {
			return agentexec.DockerContainerLifecycleSnapshot{}, fmt.Errorf("container inspect returned unsupported health status")
		}
	}
	startedAt := time.Time{}
	if value := strings.TrimSpace(inspect.State.StartedAt); value != "" && !strings.HasPrefix(value, "0001-") {
		startedAt, err = time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return agentexec.DockerContainerLifecycleSnapshot{}, fmt.Errorf("decode container start time: %w", err)
		}
	}
	return agentexec.DockerContainerLifecycleSnapshot{
		ContainerID:  strings.ToLower(strings.TrimSpace(inspect.ID)),
		State:        strings.ToLower(strings.TrimSpace(string(inspect.State.Status))),
		Running:      inspect.State.Running,
		Health:       health,
		StartedAt:    startedAt.UTC(),
		RestartCount: inspect.RestartCount,
		ObservedAt:   time.Now().UTC(),
	}, nil
}

// MutateDockerContainerLifecycle executes exactly one allowlisted daemon API
// operation. Mutation calls are not automatically retried: an ambiguous
// transport failure must be reconciled by the lifecycle readback rather than
// risking a duplicate action.
func (a *Agent) MutateDockerContainerLifecycle(ctx context.Context, runtime, operation, containerID string) error {
	if err := a.validateLifecycleRuntime(runtime); err != nil {
		return err
	}
	containerID = strings.ToLower(strings.TrimSpace(containerID))
	if containerID == "" {
		return fmt.Errorf("container id is required")
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerUpdateCallTimeout)
	defer cancel()
	switch strings.ToLower(strings.TrimSpace(operation)) {
	case "start":
		return a.docker.ContainerStart(callCtx, containerID, dockerContainerStartOptions{})
	case "stop":
		return a.docker.ContainerStop(callCtx, containerID, dockerContainerStopOptions{})
	case "restart":
		return a.docker.ContainerRestart(callCtx, containerID, dockerContainerRestartOptions{})
	default:
		return fmt.Errorf("unsupported container lifecycle operation")
	}
}

func (a *Agent) validateLifecycleRuntime(runtime string) error {
	if a == nil || a.docker == nil {
		return fmt.Errorf("container runtime module is not connected")
	}
	requested := strings.ToLower(strings.TrimSpace(runtime))
	connected := strings.ToLower(strings.TrimSpace(string(a.runtime)))
	if requested != "" && requested != connected {
		return fmt.Errorf("container runtime mismatch: module runs %s", connected)
	}
	return nil
}
