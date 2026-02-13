package dockeragent

import (
	"context"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
)

// cleanupOrphanedBackups searches for and removes any Pulse backup containers
// (created during updates) that are older than 15 minutes.
func (a *Agent) cleanupOrphanedBackups(ctx context.Context) {
	a.logger.Debug().Msg("Checking for orphaned backup containers")

	// List all containers (including stopped ones)
	list, err := dockerCallWithRetry(ctx, dockerCleanupCallTimeout, func(callCtx context.Context) ([]container.Summary, error) {
		return a.docker.ContainerList(callCtx, container.ListOptions{
			All: true,
		})
	})
	if err != nil {
		a.logger.Warn().Err(annotateDockerConnectionError(err)).Msg("Failed to list containers for cleanup")
		return
	}

	for _, c := range list {
		if !isBackupContainer(c.Names) {
			continue
		}

		// Check age based on the timestamp in the name, not the container's creation date
		// (Renaming a container does not change its creation date)
		parts := strings.Split(c.Names[0], backupContainerMarker)
		if len(parts) < 2 {
			continue
		}

		timestampStr := parts[len(parts)-1]
		backupTime, err := time.Parse("20060102_150405", timestampStr)
		if err != nil {
			// If we can't parse the timestamp, fall back to creation date as a safety
			created := time.Unix(c.Created, 0)
			if time.Since(created) < 15*time.Minute {
				continue
			}
			backupTime = created
		}

		if time.Since(backupTime) > 15*time.Minute {
			a.logger.Info().
				Str("container", c.Names[0]).
				Time("backupTime", backupTime).
				Msg("Removing orphaned backup container")

			_, err := dockerCallWithRetry(ctx, dockerCleanupCallTimeout, func(callCtx context.Context) (struct{}, error) {
				err := a.docker.ContainerRemove(callCtx, c.ID, container.RemoveOptions{Force: true})
				return struct{}{}, err
			})
			if err != nil {
				a.logger.Warn().Err(annotateDockerConnectionError(err)).Str("id", c.ID).Msg("Failed to remove orphaned backup container")
			}
		}
	}
}
