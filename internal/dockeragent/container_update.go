package dockeragent

import (
	"context"
<<<<<<< HEAD
	"encoding/json"
=======
>>>>>>> refactor/parallel-05-error-handling
	"errors"
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

type updateContainerCommandPayload struct {
	ContainerID string `json:"containerId"`
}

func decodeUpdateContainerPayload(payload map[string]any) (updateContainerCommandPayload, error) {
	var commandPayload updateContainerCommandPayload

	body, err := jsonMarshalFn(payload)
	if err != nil {
		return commandPayload, fmt.Errorf("marshal update command payload: %w", err)
	}

	if err := json.Unmarshal(body, &commandPayload); err != nil {
		return commandPayload, fmt.Errorf("decode update command payload: %w", err)
	}

	commandPayload.ContainerID = strings.TrimSpace(commandPayload.ContainerID)
	if commandPayload.ContainerID == "" {
		return commandPayload, errors.New("missing containerId in payload")
	}

	return commandPayload, nil
}

// handleUpdateContainerCommand handles the update_container command from Pulse.
func (a *Agent) handleUpdateContainerCommand(ctx context.Context, target TargetConfig, command agentsdocker.Command) error {
	commandPayload, err := decodeUpdateContainerPayload(command.Payload)
	if err != nil {
		a.logger.Error().Err(err).Msg("Update command missing or invalid containerId in payload")
		if err := a.sendCommandAck(ctx, target, command.ID, agentsdocker.CommandStatusFailed, "Missing containerId in payload"); err != nil {
			a.logger.Error().Err(err).Msg("Failed to send failure acknowledgement")
		}
		return nil
	}
	containerID := commandPayload.ContainerID

	a.logger.Info().
		Str("commandID", command.ID).
		Str("containerId", containerID).
		Msg("Received update_container command from Pulse")

	// Send acknowledgement that we're starting the update
	if err := a.sendCommandAck(ctx, target, command.ID, agentsdocker.CommandStatusAcknowledged, "Starting container update"); err != nil {
		a.logger.Error().Err(err).Msg("Failed to send acknowledgement to Pulse")
		return nil
	}

	// Create a progress callback to send step updates to Pulse
	progressFn := func(step string) {
		// Send progress update (using "in_progress" status with step message)
		if err := a.sendCommandAck(ctx, target, command.ID, agentsdocker.CommandStatusInProgress, step); err != nil {
			a.logger.Warn().Err(err).Str("step", step).Msg("Failed to send progress update")
		}
	}

	// Perform the update with progress tracking
	result := a.updateContainerWithProgress(ctx, containerID, progressFn)

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

// updateContainerWithProgress performs the actual container update operation with progress reporting.
// This is the core logic that:
// 1. Inspects the current container configuration
// 2. Pulls the latest image
// 3. Stops and renames the old container (for backup)
// 4. Creates a new container with the same config
// 5. Starts the new container
// 6. Cleans up on success or rolls back on failure
//
// The progressFn callback is called at each step to report progress to Pulse.
func (a *Agent) updateContainerWithProgress(ctx context.Context, containerID string, progressFn func(step string)) ContainerUpdateResult {
	updateCtx, cancel := context.WithTimeout(ctx, dockerUpdateOverallTimeout)
	defer cancel()
	ctx = updateCtx

	result := ContainerUpdateResult{
		ContainerID: containerID,
	}

	// Helper to report progress (handles nil progressFn)
	reportProgress := func(step string) {
		if progressFn != nil {
			progressFn(step)
		}
	}

	// 1. Inspect the current container to get its full configuration
	inspect, err := dockerCallWithRetry(ctx, dockerUpdateCallTimeout, func(callCtx context.Context) (container.InspectResponse, error) {
		return a.docker.ContainerInspect(callCtx, containerID)
	})
	if err != nil {
		result.Error = fmt.Sprintf("Failed to inspect container: %v", annotateDockerConnectionError(err))
		a.logger.Error().Err(err).Str("containerId", containerID).Msg("Failed to inspect container for update")
		return result
	}

	result.ContainerName = strings.TrimPrefix(inspect.Name, "/")
	result.OldImageDigest = inspect.Image

	// Reject updates for backup containers (created during previous updates)
	if strings.Contains(result.ContainerName, backupContainerMarker) {
		result.Error = "Cannot update backup containers - these are temporary and should be cleaned up"
		a.logger.Warn().
			Str("container", result.ContainerName).
			Msg("Rejecting update request for backup container")
		return result
	}

	a.logger.Info().
		Str("container", result.ContainerName).
		Str("image", inspect.Config.Image).
		Msg("Starting container update")

	// 2. Pull the latest image
	imageName := inspect.Config.Image
	reportProgress(fmt.Sprintf("Pulling image %s...", imageName))
	a.logger.Info().Str("image", imageName).Msg("Pulling latest image")

	pullCtx, pullCancel := context.WithTimeout(ctx, dockerUpdateCallTimeout)
	pullResp, err := a.docker.ImagePull(pullCtx, imageName, image.PullOptions{})
	if err != nil {
		pullCancel()
		result.Error = fmt.Sprintf("Failed to pull image %s: %v", imageName, annotateDockerConnectionError(err))
		a.logger.Error().Err(err).Str("image", imageName).Msg("Failed to pull latest image")
		return result
	}
<<<<<<< HEAD
	// Consume the pull response to ensure the pull completes
	_, _ = io.Copy(io.Discard, pullResp)
	_ = pullResp.Close()
	pullCancel()
=======
	// Consume and close the pull response to ensure the pull completes and
	// response-stream failures are not silently ignored.
	if err := drainAndClosePullResponse(pullResp); err != nil {
		result.Error = fmt.Sprintf("Failed to process image pull response: %v", err)
		a.logger.Error().Err(err).Str("image", imageName).Msg("Failed to finalize image pull response")
		return result
	}
>>>>>>> refactor/parallel-05-error-handling

	a.logger.Info().Str("image", imageName).Msg("Successfully pulled latest image")

	// 3. Stop the current container
	reportProgress(fmt.Sprintf("Stopping container %s...", result.ContainerName))
	stopTimeout := 30 // seconds
	_, err = dockerCallWithRetry(ctx, dockerUpdateCallTimeout, func(callCtx context.Context) (struct{}, error) {
		err := a.docker.ContainerStop(callCtx, containerID, container.StopOptions{Timeout: &stopTimeout})
		return struct{}{}, err
	})
	if err != nil {
		result.Error = fmt.Sprintf("Failed to stop container: %v", annotateDockerConnectionError(err))
		a.logger.Error().Err(err).Str("container", result.ContainerName).Msg("Failed to stop container")
		return result
	}

	a.logger.Info().Str("container", result.ContainerName).Msg("Container stopped")

	// 4. Rename the old container for backup
	backupName := result.ContainerName + backupContainerMarker + nowFn().Format("20060102_150405")
	_, err = dockerCallWithRetry(ctx, dockerUpdateCallTimeout, func(callCtx context.Context) (struct{}, error) {
		err := a.docker.ContainerRename(callCtx, containerID, backupName)
		return struct{}{}, err
	})
	if err != nil {
		result.Error = fmt.Sprintf("Failed to rename container for backup: %v", annotateDockerConnectionError(err))
		a.logger.Error().Err(err).Str("container", result.ContainerName).Msg("Failed to rename container for backup")
		// Try to restart the original container
		if restartErr := a.docker.ContainerStart(ctx, containerID, container.StartOptions{}); restartErr != nil {
			a.logger.Warn().
				Err(restartErr).
				Str("container", result.ContainerName).
				Msg("Rollback step failed: restart original container after backup rename failure")
		}
		return result
	}

	result.BackupCreated = true
	result.BackupContainer = backupName
	a.logger.Info().Str("backup", backupName).Msg("Container renamed for backup")

	reportProgress(fmt.Sprintf("Creating new container %s...", result.ContainerName))

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
				Aliases:    netConfig.Aliases,
				IPAMConfig: netConfig.IPAMConfig,
				Links:      netConfig.Links,
				NetworkID:  netConfig.NetworkID,
				EndpointID: "", // Will be assigned
				Gateway:    "", // Will be assigned
				IPAddress:  "", // Will be assigned
				MacAddress: netConfig.MacAddress,
				DriverOpts: netConfig.DriverOpts,
			}
			break // Only set one network during creation
		}
	}

	// 6. Create a new container with the same configuration
	createResp, err := dockerCallWithRetry(ctx, dockerUpdateCallTimeout, func(callCtx context.Context) (container.CreateResponse, error) {
		return a.docker.ContainerCreate(
			callCtx,
			inspect.Config,
			inspect.HostConfig,
			networkingConfig,
			nil, // Platform
			result.ContainerName,
		)
	})
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create new container: %v", annotateDockerConnectionError(err))
		a.logger.Error().Err(err).Str("container", result.ContainerName).Msg("Failed to create new container")
		// Rollback: rename backup back to original name
		a.rollbackRenameAndRestart(ctx, backupName, result.ContainerName, containerID)
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
	reportProgress(fmt.Sprintf("Starting container %s...", result.ContainerName))
	_, err = dockerCallWithRetry(ctx, dockerUpdateCallTimeout, func(callCtx context.Context) (struct{}, error) {
		err := a.docker.ContainerStart(callCtx, newContainerID, container.StartOptions{})
		return struct{}{}, err
	})
	if err != nil {
		result.Error = fmt.Sprintf("Failed to start new container: %v", annotateDockerConnectionError(err))
		a.logger.Error().Err(err).Str("container", result.ContainerName).Msg("Failed to start new container")
		// Rollback: remove new container, rename backup back
		a.rollbackRemoveRenameAndRestart(ctx, newContainerID, backupName, result.ContainerName, containerID)
		return result
	}

	a.logger.Info().Str("container", result.ContainerName).Msg("New container started, verifying stability...")

	// 9. Verify container stability
	reportProgress("Verifying container stability...")
	// Wait a few seconds to ensure it doesn't crash immediately
	sleepFn(5 * time.Second)

	verifyInspect, err := dockerCallWithRetry(ctx, dockerUpdateCallTimeout, func(callCtx context.Context) (container.InspectResponse, error) {
		return a.docker.ContainerInspect(callCtx, newContainerID)
	})
	if err != nil {
		result.Error = fmt.Sprintf("Failed to inspect new container during verification: %v", annotateDockerConnectionError(err))
		a.logger.Error().Err(err).Str("container", result.ContainerName).Msg("Failed to verify container stability")
		// Rollback
		a.rollbackRemoveRenameAndRestart(ctx, newContainerID, backupName, result.ContainerName, containerID)
		return result
	}

	// Check if running
	if !verifyInspect.State.Running {
		result.Error = fmt.Sprintf("New container crashed immediately (exit code %d): %s", verifyInspect.State.ExitCode, verifyInspect.State.Error)
		a.logger.Error().Str("container", result.ContainerName).Int("exitCode", verifyInspect.State.ExitCode).Msg("New container crashed, rolling back")
		// Rollback
		a.rollbackRemoveRenameAndRestart(ctx, newContainerID, backupName, result.ContainerName, containerID)
		return result
	}

	// Check health if available
	if verifyInspect.State.Health != nil && verifyInspect.State.Health.Status == "unhealthy" {
		result.Error = "New container reported unhealthy status"
		a.logger.Error().Str("container", result.ContainerName).Msg("New container unhealthy, rolling back")
		// Rollback
		a.rollbackRemoveRenameAndRestart(ctx, newContainerID, backupName, result.ContainerName, containerID)
		return result
	}

	result.NewImageDigest = verifyInspect.Image

	// 10. Schedule cleanup of backup container after a delay
	// This gives time to verify the new container is working
	go func() {
		sleepFn(5 * time.Minute)
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), dockerCleanupCallTimeout)
		defer cleanupCancel()
		if err := a.docker.ContainerRemove(cleanupCtx, backupName, container.RemoveOptions{Force: true}); err != nil {
			a.logger.Warn().Err(err).Str("backup", backupName).Msg("Failed to cleanup backup container")
		} else {
			a.logger.Info().Str("backup", backupName).Msg("Backup container cleaned up")
		}
	}()

	result.Success = true
	a.logger.Info().
		Str("container", result.ContainerName).
		Str("oldDigest", shortDigest(result.OldImageDigest)).
		Str("newDigest", shortDigest(result.NewImageDigest)).
		Msg("Container update completed successfully")

	return result
}

func drainAndClosePullResponse(pullResp io.ReadCloser) error {
	if pullResp == nil {
		return errors.New("pull response body is nil")
	}

	_, drainErr := io.Copy(io.Discard, pullResp)
	closeErr := pullResp.Close()

	if drainErr != nil && closeErr != nil {
		return errors.Join(
			fmt.Errorf("drain pull response body: %w", drainErr),
			fmt.Errorf("close pull response body: %w", closeErr),
		)
	}
	if drainErr != nil {
		return fmt.Errorf("drain pull response body: %w", drainErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close pull response body: %w", closeErr)
	}

	return nil
}

func (a *Agent) rollbackRenameAndRestart(ctx context.Context, backupName, originalName, originalID string) {
	if err := a.docker.ContainerRename(ctx, backupName, originalName); err != nil {
		a.logger.Warn().
			Err(err).
			Str("backup", backupName).
			Str("container", originalName).
			Msg("Rollback step failed: restore original container name")
	}
	if err := a.docker.ContainerStart(ctx, originalID, container.StartOptions{}); err != nil {
		a.logger.Warn().
			Err(err).
			Str("container", originalName).
			Msg("Rollback step failed: restart original container")
	}
}

func (a *Agent) rollbackRemoveRenameAndRestart(ctx context.Context, newContainerID, backupName, originalName, originalID string) {
	if err := a.docker.ContainerRemove(ctx, newContainerID, container.RemoveOptions{Force: true}); err != nil {
		a.logger.Warn().
			Err(err).
			Str("containerId", newContainerID).
			Msg("Rollback step failed: remove replacement container")
	}
	a.rollbackRenameAndRestart(ctx, backupName, originalName, originalID)
}

func shortDigest(digest string) string {
	const digestPreviewLen = 12
	if len(digest) <= digestPreviewLen {
		return digest
	}
	return digest[:digestPreviewLen]
}
