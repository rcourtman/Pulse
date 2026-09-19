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
	if err := a.TypedContainerUpdatePreflight(ctx, runtime, containerID, expectedImageDigest); err != nil {
		return agentexec.DockerContainerUpdateOutcome{}, err
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

// TypedContainerUpdatePreflight performs only the daemon reads needed to
// prove that the exact planned container and image digest remain current.
func (a *Agent) TypedContainerUpdatePreflight(ctx context.Context, runtime, containerID, expectedImageDigest string) error {
	if a == nil || a.docker == nil {
		return agentexec.NewActionPreflightError(agentexec.ActionRefusalCapabilityUnavailable, fmt.Errorf("docker module is not connected to a container runtime"))
	}
	requestedRuntime := strings.ToLower(strings.TrimSpace(runtime))
	if requestedRuntime != "" && requestedRuntime != strings.ToLower(string(a.runtime)) {
		return agentexec.NewActionPreflightError(agentexec.ActionRefusalCapabilityUnavailable, fmt.Errorf("container runtime mismatch: module runs %s", a.runtime))
	}

	inspect, err := dockerCallWithRetry(ctx, dockerUpdateCallTimeout, func(callCtx context.Context) (containertypes.InspectResponse, error) {
		return a.docker.ContainerInspect(callCtx, containerID)
	})
	if err != nil {
		return agentexec.NewActionPreflightError(agentexec.ActionRefusalTargetInspectionUnavailable, fmt.Errorf("container preflight inspect unavailable: %v", annotateDockerConnectionError(err)))
	}
	if expectedImageDigest != "" {
		expectedImageDigest = strings.TrimSpace(expectedImageDigest)
		localImageID := strings.TrimSpace(inspect.Image)
		matchesLocalID := strings.EqualFold(localImageID, expectedImageDigest)
		matchesRepoDigest := false
		repoDigest := ""
		if !matchesLocalID {
			imageName := ""
			if inspect.Config != nil {
				imageName = inspect.Config.Image
			}
			// An image can carry several RepoDigests (#2110), so accept the
			// planned digest when it matches any of them rather than only the
			// first entry.
			repoDigests, _, _, _ := a.getImageRepoDigests(ctx, localImageID, imageName)
			for _, candidate := range repoDigests {
				candidate = strings.TrimSpace(candidate)
				if candidate == "" {
					continue
				}
				if repoDigest == "" {
					repoDigest = candidate
				}
				if strings.EqualFold(candidate, expectedImageDigest) {
					repoDigest = candidate
					matchesRepoDigest = true
					break
				}
			}
		}
		if !matchesLocalID && !matchesRepoDigest {
			if repoDigest == "" {
				repoDigest = "unavailable"
			}
			return agentexec.NewActionPreflightError(agentexec.ActionRefusalTargetPreconditionFailed, fmt.Errorf("container image digest no longer matches the planned update (expected %s, local image id %s, local repo digest %s)", expectedImageDigest, localImageID, repoDigest))
		}
	}
	return nil
}
