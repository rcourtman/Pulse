package dockeragent

import (
	"context"
	"fmt"
	"strings"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

// TypedContainerUpdate runs a container image update for the typed command
// channel. It refuses when the runtime or the container's current image
// digest no longer matches what the plan was bound to, then delegates to the
// same pull/backup/recreate/rollback implementation the module has always
// used and reports the neutral outcome the unified agent bridge forwards.
func (a *Agent) TypedContainerUpdate(ctx context.Context, runtime, containerID, expectedImageDigest string, progress func(string)) (agentexec.DockerContainerUpdateOutcome, error) {
	if a == nil || a.docker == nil {
		return agentexec.DockerContainerUpdateOutcome{}, fmt.Errorf("docker module is not connected to a container runtime")
	}
	requestedRuntime := strings.ToLower(strings.TrimSpace(runtime))
	if requestedRuntime != "" && requestedRuntime != strings.ToLower(string(a.runtime)) {
		return agentexec.DockerContainerUpdateOutcome{}, fmt.Errorf("container runtime mismatch: module runs %s", a.runtime)
	}

	inspect, err := dockerCallWithRetry(ctx, dockerUpdateCallTimeout, func(callCtx context.Context) (containertypes.InspectResponse, error) {
		return a.docker.ContainerInspect(callCtx, containerID)
	})
	if err != nil {
		return agentexec.DockerContainerUpdateOutcome{}, fmt.Errorf("container preflight inspect unavailable: %v", annotateDockerConnectionError(err))
	}
	if expectedImageDigest != "" && !strings.EqualFold(strings.TrimSpace(inspect.Image), expectedImageDigest) {
		return agentexec.DockerContainerUpdateOutcome{}, fmt.Errorf("container image digest no longer matches the planned update")
	}

	result := a.updateContainerWithProgress(ctx, containerID, progress)
	return agentexec.DockerContainerUpdateOutcome{
		Success:           result.Success,
		ContainerName:     result.ContainerName,
		OldContainerID:    result.OldContainerID,
		NewContainerID:    result.NewContainerID,
		OldImageDigest:    result.OldImageDigest,
		NewImageDigest:    result.NewImageDigest,
		BackupCreated:     result.BackupCreated,
		BackupContainer:   result.BackupContainer,
		RollbackAttempted: result.RollbackAttempted,
		RolledBack:        result.RolledBack,
		Error:             result.Error,
	}, nil
}
