package dockeragent

import (
	"context"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
)

// cleanupOrphanedBackups searches for and removes any Pulse backup containers
// (created during updates) that are older than 1 hour.
func (a *Agent) cleanupOrphanedBackups(ctx context.Context) {
	a.logger.Debug().Msg("Checking for orphaned backup containers")

	// List all containers (including stopped ones)
	list, err := a.docker.ContainerList(ctx, container.ListOptions{
		All: true,
	})
	if err != nil {
		a.logger.Warn().Err(err).Msg("Failed to list containers for cleanup")
		return
	}

	for _, c := range list {
		// Check if it's a backup container
		// Name format:  originalName + "_pulse_backup_" + timestamp
		isBackup := false
		for _, name := range c.Names {
			if strings.Contains(name, "_pulse_backup_") {
				isBackup = true
				break
			}
		}

		if !isBackup {
			continue
		}

		// Check age
		created := time.Unix(c.Created, 0)
		if time.Since(created) > 1*time.Hour {
			a.logger.Info().
				Str("container", c.Names[0]).
				Time("created", created).
				Msg("Removing orphaned backup container")

			if err := a.docker.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true}); err != nil {
				a.logger.Warn().Err(err).Str("id", c.ID).Msg("Failed to remove orphaned backup container")
			}
		}
	}
}
