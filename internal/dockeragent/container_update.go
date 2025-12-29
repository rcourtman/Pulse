package dockeragent

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// ContainerUpdateResult captures the outcome of a container update operation.
type ContainerUpdateResult struct {
	Success         bool   `json:"success"`
	ContainerID     string `json:"containerId"`
	ContainerName   string `json:"containerName"`
	OldImageDigest  string `json:"oldImageDigest,omitempty"`
	NewImageDigest  string `json:"newImageDigest,omitempty"`
	BackupCreated   bool   `json:"backupCreated"`
	BackupContainer string `json:"backupContainer,omitempty"`
	Error           string `json:"error,omitempty"`
}

// handleUpdateContainerCommand handles the update_container command from Pulse.
func (a *Agent) handleUpdateContainerCommand(ctx context.Context, target TargetConfig, command agentsdocker.Command) error {
	containerID, ok := command.Payload["containerId"].(string)
	if !ok || containerID == "" {
		a.logger.Error().Msg("Update command missing containerId in payload")
		if err := a.sendCommandAck(ctx, target, command.ID, agentsdocker.CommandStatusFailed, "Missing containerId in payload"); err != nil {
			a.logger.Error().Err(err).Msg("Failed to send failure acknowledgement")
		}
		return nil
	}

	a.logger.Info().
		Str("commandID", command.ID).
		Str("containerId", containerID).
		Msg("Received update_container command from Pulse")

	// Send acknowledgement that we're starting the update
	if err := a.sendCommandAck(ctx, target, command.ID, agentsdocker.CommandStatusAcknowledged, "Starting container update"); err != nil {
		a.logger.Error().Err(err).Msg("Failed to send acknowledgement to Pulse")
		return nil
	}

	// Perform the update
	result := a.updateContainer(ctx, containerID)

	// Send completion status
	status := agentsdocker.CommandStatusCompleted
	message := fmt.Sprintf("Container %s updated successfully", result.ContainerName)
	if !result.Success {
		status = agentsdocker.CommandStatusFailed
		message = result.Error
	}

	if err := a.sendCommandAck(ctx, target, command.ID, status, message); err != nil {
		a.logger.Error().Err(err).Msg("Failed to send completion acknowledgement to Pulse")
	}

	return nil
}

// updateContainer performs the actual container update operation.
// This is the core logic that:
// 1. Inspects the current container configuration
// 2. Pulls the latest image
// 3. Stops and renames the old container (for backup)
// 4. Creates a new container with the same config
// 5. Starts the new container
// 6. Cleans up on success or rolls back on failure
func (a *Agent) updateContainer(ctx context.Context, containerID string) ContainerUpdateResult {
	result := ContainerUpdateResult{
		ContainerID: containerID,
	}

	// 1. Inspect the current container to get its full configuration
	inspect, err := a.docker.ContainerInspect(ctx, containerID)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to inspect container: %v", err)
		a.logger.Error().Err(err).Str("containerId", containerID).Msg("Failed to inspect container for update")
		return result
	}

	result.ContainerName = strings.TrimPrefix(inspect.Name, "/")
	result.OldImageDigest = inspect.Image

	a.logger.Info().
		Str("container", result.ContainerName).
		Str("image", inspect.Config.Image).
		Msg("Starting container update")

	// 2. Pull the latest image
	imageName := inspect.Config.Image
	a.logger.Info().Str("image", imageName).Msg("Pulling latest image")

	pullResp, err := a.docker.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		result.Error = fmt.Sprintf("Failed to pull image %s: %v", imageName, err)
		a.logger.Error().Err(err).Str("image", imageName).Msg("Failed to pull latest image")
		return result
	}
	// Consume the pull response to ensure the pull completes
	_, _ = io.Copy(io.Discard, pullResp)
	pullResp.Close()

	a.logger.Info().Str("image", imageName).Msg("Successfully pulled latest image")

	// 3. Stop the current container
	stopTimeout := 30 // seconds
	if err := a.docker.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &stopTimeout}); err != nil {
		result.Error = fmt.Sprintf("Failed to stop container: %v", err)
		a.logger.Error().Err(err).Str("container", result.ContainerName).Msg("Failed to stop container")
		return result
	}

	a.logger.Info().Str("container", result.ContainerName).Msg("Container stopped")

	// 4. Rename the old container for backup
	backupName := result.ContainerName + "_pulse_backup_" + nowFn().Format("20060102_150405")
	if err := a.docker.ContainerRename(ctx, containerID, backupName); err != nil {
		result.Error = fmt.Sprintf("Failed to rename container for backup: %v", err)
		a.logger.Error().Err(err).Str("container", result.ContainerName).Msg("Failed to rename container for backup")
		// Try to restart the original container
		_ = a.docker.ContainerStart(ctx, containerID, container.StartOptions{})
		return result
	}

	result.BackupCreated = true
	result.BackupContainer = backupName
	a.logger.Info().Str("backup", backupName).Msg("Container renamed for backup")

	// 5. Prepare network configuration
	// We need to handle network settings carefully
	var networkingConfig *network.NetworkingConfig
	if len(inspect.NetworkSettings.Networks) > 0 {
		networkingConfig = &network.NetworkingConfig{
			EndpointsConfig: make(map[string]*network.EndpointSettings),
		}
		// Only set the first network here; we'll connect to others after creation
		for netName, netConfig := range inspect.NetworkSettings.Networks {
			networkingConfig.EndpointsConfig[netName] = &network.EndpointSettings{
				Aliases:     netConfig.Aliases,
				IPAMConfig:  netConfig.IPAMConfig,
				Links:       netConfig.Links,
				NetworkID:   netConfig.NetworkID,
				EndpointID:  "", // Will be assigned
				Gateway:     "", // Will be assigned
				IPAddress:   "", // Will be assigned
				MacAddress:  netConfig.MacAddress,
				DriverOpts:  netConfig.DriverOpts,
			}
			break // Only set one network during creation
		}
	}

	// 6. Create a new container with the same configuration
	createResp, err := a.docker.ContainerCreate(
		ctx,
		inspect.Config,
		inspect.HostConfig,
		networkingConfig,
		nil, // Platform
		result.ContainerName,
	)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create new container: %v", err)
		a.logger.Error().Err(err).Str("container", result.ContainerName).Msg("Failed to create new container")
		// Rollback: rename backup back to original name
		_ = a.docker.ContainerRename(ctx, backupName, result.ContainerName)
		_ = a.docker.ContainerStart(ctx, containerID, container.StartOptions{})
		return result
	}

	newContainerID := createResp.ID
	a.logger.Info().Str("newContainerId", newContainerID).Msg("New container created")

	// 7. Connect to additional networks (if more than one)
	networkCount := 0
	for netName, netConfig := range inspect.NetworkSettings.Networks {
		networkCount++
		if networkCount == 1 {
			continue // Skip the first one, already connected during creation
		}
		endpointConfig := &network.EndpointSettings{
			Aliases:    netConfig.Aliases,
			IPAMConfig: netConfig.IPAMConfig,
			Links:      netConfig.Links,
			MacAddress: netConfig.MacAddress,
			DriverOpts: netConfig.DriverOpts,
		}
		if err := a.docker.NetworkConnect(ctx, netName, newContainerID, endpointConfig); err != nil {
			a.logger.Warn().Err(err).Str("network", netName).Msg("Failed to connect to network, continuing anyway")
		}
	}

	// 8. Start the new container
	if err := a.docker.ContainerStart(ctx, newContainerID, container.StartOptions{}); err != nil {
		result.Error = fmt.Sprintf("Failed to start new container: %v", err)
		a.logger.Error().Err(err).Str("container", result.ContainerName).Msg("Failed to start new container")
		// Rollback: remove new container, rename backup back
		_ = a.docker.ContainerRemove(ctx, newContainerID, container.RemoveOptions{Force: true})
		_ = a.docker.ContainerRename(ctx, backupName, result.ContainerName)
		_ = a.docker.ContainerStart(ctx, containerID, container.StartOptions{})
		return result
	}

	a.logger.Info().Str("container", result.ContainerName).Msg("New container started, verifying stability...")

	// 9. Verify container stability
	// Wait a few seconds to ensure it doesn't crash immediately
	sleepFn(5 * time.Second)

	verifyInspect, err := a.docker.ContainerInspect(ctx, newContainerID)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to inspect new container during verification: %v", err)
		a.logger.Error().Err(err).Str("container", result.ContainerName).Msg("Failed to verify container stability")
		// Rollback
		_ = a.docker.ContainerRemove(ctx, newContainerID, container.RemoveOptions{Force: true})
		_ = a.docker.ContainerRename(ctx, backupName, result.ContainerName)
		_ = a.docker.ContainerStart(ctx, containerID, container.StartOptions{})
		return result
	}

	// Check if running
	if !verifyInspect.State.Running {
		result.Error = fmt.Sprintf("New container crashed immediately (exit code %d): %s", verifyInspect.State.ExitCode, verifyInspect.State.Error)
		a.logger.Error().Str("container", result.ContainerName).Int("exitCode", verifyInspect.State.ExitCode).Msg("New container crashed, rolling back")
		// Rollback
		_ = a.docker.ContainerRemove(ctx, newContainerID, container.RemoveOptions{Force: true})
		_ = a.docker.ContainerRename(ctx, backupName, result.ContainerName)
		_ = a.docker.ContainerStart(ctx, containerID, container.StartOptions{})
		return result
	}

	// Check health if available
	if verifyInspect.State.Health != nil && verifyInspect.State.Health.Status == "unhealthy" {
		result.Error = "New container reported unhealthy status"
		a.logger.Error().Str("container", result.ContainerName).Msg("New container unhealthy, rolling back")
		// Rollback
		_ = a.docker.ContainerRemove(ctx, newContainerID, container.RemoveOptions{Force: true})
		_ = a.docker.ContainerRename(ctx, backupName, result.ContainerName)
		_ = a.docker.ContainerStart(ctx, containerID, container.StartOptions{})
		return result
	}

	result.NewImageDigest = verifyInspect.Image

	// 10. Schedule cleanup of backup container after a delay
	// This gives time to verify the new container is working
	go func() {
		sleepFn(5 * time.Minute)
		cleanupCtx := context.Background()
		if err := a.docker.ContainerRemove(cleanupCtx, backupName, container.RemoveOptions{Force: true}); err != nil {
			a.logger.Warn().Err(err).Str("backup", backupName).Msg("Failed to cleanup backup container")
		} else {
			a.logger.Info().Str("backup", backupName).Msg("Backup container cleaned up")
		}
	}()

	result.Success = true
	a.logger.Info().
		Str("container", result.ContainerName).
		Str("oldDigest", result.OldImageDigest[:12]).
		Str("newDigest", result.NewImageDigest[:12]).
		Msg("Container update completed successfully")

	return result
}
