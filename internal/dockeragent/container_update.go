package dockeragent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// ContainerUpdateResult captures the outcome of a container update operation.
type ContainerUpdateResult struct {
	Success         bool   `json:"success"`
	ContainerID     string `json:"containerId"`
	OldContainerID  string `json:"oldContainerId,omitempty"`
	NewContainerID  string `json:"newContainerId,omitempty"`
	ContainerName   string `json:"containerName"`
	OldImageDigest  string `json:"oldImageDigest,omitempty"`
	NewImageDigest  string `json:"newImageDigest,omitempty"`
	BackupCreated   bool   `json:"backupCreated"`
	BackupContainer string `json:"backupContainer,omitempty"`
	// RollbackAttempted / RolledBack record compensation truth: whether a
	// failure path tried to restore the original container, and whether the
	// restore itself succeeded.
	RollbackAttempted bool   `json:"rollbackAttempted,omitempty"`
	RolledBack        bool   `json:"rolledBack,omitempty"`
	Error             string `json:"error,omitempty"`
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

// sanitizeSharedNamespaceConfig strips settings that the daemon derives from
// the network mode. A container sharing another container's network namespace
// (network_mode: container:<id> / service:<x>) or the host's cannot carry its
// own hostname, ports, links, or DNS options: the daemon rejects the create
// with "conflicting options: ... and the network mode", which would leave the
// update stuck after the backup rename.
func sanitizeSharedNamespaceConfig(config *containertypes.Config, hostConfig *containertypes.HostConfig) {
	if config == nil || hostConfig == nil {
		return
	}
	if !hostConfig.NetworkMode.IsContainer() && !hostConfig.NetworkMode.IsHost() {
		return
	}
	config.Hostname = ""
	config.Domainname = ""
	if hostConfig.NetworkMode.IsContainer() {
		config.ExposedPorts = nil
		hostConfig.PortBindings = nil
		hostConfig.PublishAllPorts = false
		hostConfig.Links = nil
		hostConfig.DNS = nil
		hostConfig.DNSOptions = nil
		hostConfig.DNSSearch = nil
		hostConfig.ExtraHosts = nil
	}
}

type recreateNetworkAttachment struct {
	name     string
	endpoint *network.EndpointSettings
}

type containerRecreatePlan struct {
	config             *containertypes.Config
	hostConfig         *containertypes.HostConfig
	networkingConfig   *network.NetworkingConfig
	additionalNetworks []recreateNetworkAttachment
}

// buildContainerRecreatePlan turns inspect output into create input. Inspect
// mixes desired configuration with daemon-generated runtime observations, so
// it must not be passed back to ContainerCreate unchanged.
func buildContainerRecreatePlan(inspect containertypes.InspectResponse) (containerRecreatePlan, error) {
	if inspect.Config == nil {
		return containerRecreatePlan{}, errors.New("inspect response is missing container configuration")
	}
	if inspect.HostConfig == nil {
		return containerRecreatePlan{}, errors.New("inspect response is missing host configuration")
	}

	config := *inspect.Config
	hostConfig := *inspect.HostConfig
	if daemonGeneratedHostname(config.Hostname, inspect.ID) {
		config.Hostname = ""
	}
	sanitizeSharedNamespaceConfig(&config, &hostConfig)

	plan := containerRecreatePlan{
		config:     &config,
		hostConfig: &hostConfig,
	}
	if hostConfig.NetworkMode.IsContainer() || hostConfig.NetworkMode.IsHost() ||
		inspect.NetworkSettings == nil || len(inspect.NetworkSettings.Networks) == 0 {
		return plan, nil
	}

	networkNames := make([]string, 0, len(inspect.NetworkSettings.Networks))
	for name, endpoint := range inspect.NetworkSettings.Networks {
		if strings.TrimSpace(name) == "" || endpoint == nil {
			continue
		}
		networkNames = append(networkNames, name)
	}
	if len(networkNames) == 0 {
		return plan, nil
	}
	sort.Strings(networkNames)

	primaryNetwork := string(hostConfig.NetworkMode)
	if primaryNetwork == "" || primaryNetwork == "default" {
		primaryNetwork = "bridge"
	}
	if _, ok := inspect.NetworkSettings.Networks[primaryNetwork]; !ok {
		primaryNetwork = networkNames[0]
	}

	plan.networkingConfig = &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			primaryNetwork: desiredEndpointSettings(inspect.NetworkSettings.Networks[primaryNetwork]),
		},
	}
	for _, name := range networkNames {
		if name == primaryNetwork {
			continue
		}
		plan.additionalNetworks = append(plan.additionalNetworks, recreateNetworkAttachment{
			name:     name,
			endpoint: desiredEndpointSettings(inspect.NetworkSettings.Networks[name]),
		})
	}
	return plan, nil
}

// desiredEndpointSettings preserves operator-configurable endpoint settings
// and deliberately drops observed runtime fields such as endpoint IDs,
// gateways, assigned addresses, prefix lengths, and generated DNS names.
func desiredEndpointSettings(endpoint *network.EndpointSettings) *network.EndpointSettings {
	if endpoint == nil {
		return &network.EndpointSettings{}
	}
	return &network.EndpointSettings{
		IPAMConfig: endpoint.IPAMConfig.Copy(),
		Links:      append([]string(nil), endpoint.Links...),
		Aliases:    append([]string(nil), endpoint.Aliases...),
		DriverOpts: cloneStringMap(endpoint.DriverOpts),
		GwPriority: endpoint.GwPriority,
		MacAddress: append(network.HardwareAddr(nil), endpoint.MacAddress...),
	}
}

func daemonGeneratedHostname(hostname, containerID string) bool {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	containerID = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(containerID)), "sha256:")
	if hostname == "" || containerID == "" {
		return false
	}
	return hostname == containerID || (len(hostname) == 12 && strings.HasPrefix(containerID, hostname))
}

// handleUpdateContainerCommand handles the update_container command from Pulse.
func (a *Agent) handleUpdateContainerCommand(ctx context.Context, target TargetConfig, command agentsdocker.Command) error {
	commandPayload, err := decodeUpdateContainerPayload(command.Payload)
	if err != nil {
		a.logger.Error().Err(err).Msg("Update command missing or invalid containerId in payload")
		if err := a.sendCommandAck(ctx, target, command.ID, agentsdocker.CommandStatusFailed, "Missing containerId in payload"); err != nil {
			a.logger.Error().
				Err(err).
				Str("commandID", command.ID).
				Str("target", target.URL).
				Msg("Failed to send failure acknowledgement")
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
		a.logger.Error().
			Err(err).
			Str("commandID", command.ID).
			Str("containerId", containerID).
			Str("target", target.URL).
			Msg("Failed to send acknowledgement to Pulse")
		return nil
	}

	// Create a progress callback to send step updates to Pulse
	progressFn := func(step string) {
		// Send progress update (using "in_progress" status with step message)
		if err := a.sendCommandAck(ctx, target, command.ID, agentsdocker.CommandStatusInProgress, step); err != nil {
			a.logger.Warn().
				Err(err).
				Str("commandID", command.ID).
				Str("containerId", containerID).
				Str("target", target.URL).
				Str("step", step).
				Msg("Failed to send progress update")
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

	var payload map[string]any
	if result.Success && result.OldContainerID != "" && result.NewContainerID != "" && result.OldContainerID != result.NewContainerID {
		payload = map[string]any{
			"oldContainerId": result.OldContainerID,
			"newContainerId": result.NewContainerID,
		}
	}

	if payload != nil {
		err = a.sendCommandAckWithPayload(ctx, target, command.ID, status, message, payload)
	} else {
		err = a.sendCommandAck(ctx, target, command.ID, status, message)
	}
	if err != nil {
		a.logger.Error().
			Err(err).
			Str("commandID", command.ID).
			Str("containerId", containerID).
			Str("target", target.URL).
			Str("status", status).
			Msg("Failed to send completion acknowledgement to Pulse")
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
		ContainerID:    containerID,
		OldContainerID: containerID,
	}

	// Helper to report progress (handles nil progressFn)
	reportProgress := func(step string) {
		if progressFn != nil {
			progressFn(step)
		}
	}

	// 1. Inspect the current container to get its full configuration
	inspect, err := dockerCallWithRetry(ctx, dockerUpdateCallTimeout, func(callCtx context.Context) (containertypes.InspectResponse, error) {
		return a.docker.ContainerInspect(callCtx, containerID)
	})
	if err != nil {
		result.Error = fmt.Sprintf("Failed to inspect container: %v", annotateDockerConnectionError(err))
		a.logger.Error().Err(err).Str("containerId", containerID).Msg("Failed to inspect container for update")
		return result
	}

	result.ContainerName = strings.TrimPrefix(inspect.Name, "/")
	result.OldImageDigest = inspect.Image
	wasRunning := inspect.State != nil && inspect.State.Running

	// Reject updates for backup containers (created during previous updates)
	if strings.Contains(result.ContainerName, backupContainerMarker) {
		result.Error = "Cannot update backup containers - these are temporary and should be cleaned up"
		a.logger.Warn().
			Str("container", result.ContainerName).
			Msg("Rejecting update request for backup container")
		return result
	}

	recreatePlan, err := buildContainerRecreatePlan(inspect)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to prepare container recreation: %v", err)
		a.logger.Error().Err(err).Str("container", result.ContainerName).Msg("Invalid inspect response for container update")
		return result
	}

	a.logger.Info().
		Str("container", result.ContainerName).
		Str("image", recreatePlan.config.Image).
		Bool("wasRunning", wasRunning).
		Msg("Starting container update")

	// 2. Pull the latest image
	imageName := recreatePlan.config.Image
	reportProgress(fmt.Sprintf("Pulling image %s...", imageName))
	a.logger.Info().Str("image", imageName).Msg("Pulling latest image")

	pullCtx, pullCancel := context.WithTimeout(ctx, dockerUpdateCallTimeout)
	pullResp, err := a.docker.ImagePull(pullCtx, imageName, dockerImagePullOptions{})
	if err != nil {
		pullCancel()
		result.Error = fmt.Sprintf("Failed to pull image %s: %v", imageName, annotateDockerConnectionError(err))
		a.logger.Error().Err(err).Str("image", imageName).Msg("Failed to pull latest image")
		return result
	}
	// Consume the pull response to ensure the pull completes
	_, _ = io.Copy(io.Discard, pullResp)
	_ = pullResp.Close()
	pullCancel()

	a.logger.Info().Str("image", imageName).Msg("Successfully pulled latest image")

	// 3. Stop the current container (only if it was running).
	if wasRunning {
		reportProgress(fmt.Sprintf("Stopping container %s...", result.ContainerName))
		stopTimeout := 30 // seconds
		_, err = dockerCallWithRetry(ctx, dockerUpdateCallTimeout, func(callCtx context.Context) (struct{}, error) {
			err := a.docker.ContainerStop(callCtx, containerID, dockerContainerStopOptions{Timeout: &stopTimeout})
			return struct{}{}, err
		})
		if err != nil {
			result.Error = fmt.Sprintf("Failed to stop container: %v", annotateDockerConnectionError(err))
			a.logger.Error().Err(err).Str("container", result.ContainerName).Msg("Failed to stop container")
			return result
		}

		a.logger.Info().Str("container", result.ContainerName).Msg("Container stopped")
	}

	// 4. Rename the old container for backup
	backupName := result.ContainerName + backupContainerMarker + nowFn().Format("20060102_150405")
	_, err = dockerCallWithRetry(ctx, dockerUpdateCallTimeout, func(callCtx context.Context) (struct{}, error) {
		err := a.docker.ContainerRename(callCtx, containerID, backupName)
		return struct{}{}, err
	})
	if err != nil {
		result.Error = fmt.Sprintf("Failed to rename container for backup: %v", annotateDockerConnectionError(err))
		a.logger.Error().Err(err).Str("container", result.ContainerName).Msg("Failed to rename container for backup")
		if wasRunning {
			// Try to restart the original container.
			if restartErr := a.docker.ContainerStart(ctx, containerID, dockerContainerStartOptions{}); restartErr != nil {
				a.logger.Warn().
					Err(restartErr).
					Str("container", result.ContainerName).
					Msg("Rollback step failed: restart original container after backup rename failure")
			}
		}
		return result
	}

	result.BackupCreated = true
	result.BackupContainer = backupName
	a.logger.Info().Str("backup", backupName).Msg("Container renamed for backup")

	reportProgress(fmt.Sprintf("Creating new container %s...", result.ContainerName))

	// 5. Create a new container with the normalized desired configuration.
	createResp, err := dockerCallWithRetry(ctx, dockerUpdateCallTimeout, func(callCtx context.Context) (containertypes.CreateResponse, error) {
		return a.docker.ContainerCreate(
			callCtx,
			recreatePlan.config,
			recreatePlan.hostConfig,
			recreatePlan.networkingConfig,
			nil, // Platform
			result.ContainerName,
		)
	})
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create new container: %v", annotateDockerConnectionError(err))
		a.logger.Error().Err(err).Str("container", result.ContainerName).Msg("Failed to create new container")
		// Rollback: rename backup back to original name
		result.RollbackAttempted = true
		result.RolledBack = a.rollbackRenameAndRestart(ctx, backupName, result.ContainerName, containerID, wasRunning)
		return result
	}

	newContainerID := createResp.ID
	result.NewContainerID = newContainerID
	result.ContainerID = newContainerID
	a.logger.Info().Str("newContainerId", newContainerID).Msg("New container created")

	// 6. Restore every additional network before starting the replacement.
	for _, attachment := range recreatePlan.additionalNetworks {
		reportProgress(fmt.Sprintf("Connecting container %s to network %s...", result.ContainerName, attachment.name))
		_, err := dockerCallWithRetry(ctx, dockerUpdateCallTimeout, func(callCtx context.Context) (struct{}, error) {
			err := a.docker.NetworkConnect(callCtx, attachment.name, newContainerID, attachment.endpoint)
			return struct{}{}, err
		})
		if err != nil {
			result.Error = fmt.Sprintf("Failed to connect new container to network %s: %v", attachment.name, annotateDockerConnectionError(err))
			a.logger.Error().Err(err).Str("network", attachment.name).Str("container", result.ContainerName).Msg("Failed to restore container network attachment")
			result.RollbackAttempted = true
			result.RolledBack = a.rollbackRemoveRenameAndRestart(ctx, newContainerID, backupName, result.ContainerName, containerID, wasRunning)
			return result
		}
	}

	// 7. Start and verify the new container only if the original was running.
	if wasRunning {
		reportProgress(fmt.Sprintf("Starting container %s...", result.ContainerName))
		_, err = dockerCallWithRetry(ctx, dockerUpdateCallTimeout, func(callCtx context.Context) (struct{}, error) {
			err := a.docker.ContainerStart(callCtx, newContainerID, dockerContainerStartOptions{})
			return struct{}{}, err
		})
		if err != nil {
			result.Error = fmt.Sprintf("Failed to start new container: %v", annotateDockerConnectionError(err))
			a.logger.Error().Err(err).Str("container", result.ContainerName).Msg("Failed to start new container")
			// Rollback: remove new container, rename backup back
			result.RollbackAttempted = true
			result.RolledBack = a.rollbackRemoveRenameAndRestart(ctx, newContainerID, backupName, result.ContainerName, containerID, wasRunning)
			return result
		}

		a.logger.Info().Str("container", result.ContainerName).Msg("New container started, verifying stability...")

		// 8. Verify container stability
		reportProgress("Verifying container stability...")
		// Wait a few seconds to ensure it doesn't crash immediately
		sleepFn(5 * time.Second)
	}

	verifyInspect, err := dockerCallWithRetry(ctx, dockerUpdateCallTimeout, func(callCtx context.Context) (containertypes.InspectResponse, error) {
		return a.docker.ContainerInspect(callCtx, newContainerID)
	})
	if err != nil {
		result.Error = fmt.Sprintf("Failed to inspect new container: %v", annotateDockerConnectionError(err))
		a.logger.Error().Err(err).Str("container", result.ContainerName).Msg("Failed to inspect new container")
		result.RollbackAttempted = true
		result.RolledBack = a.rollbackRemoveRenameAndRestart(ctx, newContainerID, backupName, result.ContainerName, containerID, wasRunning)
		return result
	}

	if wasRunning {
		// Check if running
		if verifyInspect.State != nil && !verifyInspect.State.Running {
			exitCode := 0
			stateErr := ""
			if verifyInspect.State != nil {
				exitCode = verifyInspect.State.ExitCode
				stateErr = verifyInspect.State.Error
			}
			result.Error = fmt.Sprintf("New container crashed immediately (exit code %d): %s", exitCode, stateErr)
			a.logger.Error().Str("container", result.ContainerName).Int("exitCode", exitCode).Msg("New container crashed, rolling back")
			// Rollback
			result.RollbackAttempted = true
			result.RolledBack = a.rollbackRemoveRenameAndRestart(ctx, newContainerID, backupName, result.ContainerName, containerID, wasRunning)
			return result
		}

		// Check health if available
		if verifyInspect.State != nil && verifyInspect.State.Health != nil && verifyInspect.State.Health.Status == "unhealthy" {
			result.Error = "New container reported unhealthy status"
			a.logger.Error().Str("container", result.ContainerName).Msg("New container unhealthy, rolling back")
			// Rollback
			result.RollbackAttempted = true
			result.RolledBack = a.rollbackRemoveRenameAndRestart(ctx, newContainerID, backupName, result.ContainerName, containerID, wasRunning)
			return result
		}
	}

	result.NewImageDigest = verifyInspect.Image

	// 9. Schedule cleanup of backup container after a delay.
	// This gives time to verify the new container is working.
	a.runAsync(func(asyncCtx context.Context) {
		if !a.waitForAsyncDelay(5 * time.Minute) {
			return
		}
		if err := a.docker.ContainerRemove(asyncCtx, backupName, dockerContainerRemoveOptions{Force: true}); err != nil {
			a.logger.Warn().Err(err).Str("backup", backupName).Msg("Failed to cleanup backup container")
		} else {
			a.logger.Info().Str("backup", backupName).Msg("Backup container cleaned up")
		}
	})

	result.Success = true
	a.logger.Info().
		Str("container", result.ContainerName).
		Str("oldDigest", shortDigest(result.OldImageDigest)).
		Str("newDigest", shortDigest(result.NewImageDigest)).
		Msg("Container update completed successfully")

	return result
}

func (a *Agent) rollbackRenameAndRestart(ctx context.Context, backupName, originalName, originalID string, restart bool) bool {
	restored := true
	if err := a.docker.ContainerRename(ctx, backupName, originalName); err != nil {
		restored = false
		a.logger.Warn().
			Err(err).
			Str("backup", backupName).
			Str("container", originalName).
			Msg("Rollback step failed: restore original container name")
	}
	if restart {
		if err := a.docker.ContainerStart(ctx, originalID, dockerContainerStartOptions{}); err != nil {
			restored = false
			a.logger.Warn().
				Err(err).
				Str("container", originalName).
				Msg("Rollback step failed: restart original container")
		}
	}
	return restored
}

func (a *Agent) rollbackRemoveRenameAndRestart(ctx context.Context, newContainerID, backupName, originalName, originalID string, restart bool) bool {
	restored := true
	if err := a.docker.ContainerRemove(ctx, newContainerID, dockerContainerRemoveOptions{Force: true}); err != nil {
		restored = false
		a.logger.Warn().
			Err(err).
			Str("containerId", newContainerID).
			Msg("Rollback step failed: remove replacement container")
	}
	return a.rollbackRenameAndRestart(ctx, backupName, originalName, originalID, restart) && restored
}

func shortDigest(digest string) string {
	const digestPreviewLen = 12
	if len(digest) <= digestPreviewLen {
		return digest
	}
	return digest[:digestPreviewLen]
}
