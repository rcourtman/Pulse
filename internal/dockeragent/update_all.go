package dockeragent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

type updateAllCommandPayload struct {
	ContainerIDs []string `json:"containerIds"`
}

func decodeUpdateAllPayload(payload map[string]any) (updateAllCommandPayload, error) {
	var commandPayload updateAllCommandPayload

	body, err := jsonMarshalFn(payload)
	if err != nil {
		return commandPayload, fmt.Errorf("marshal update_all command payload: %w", err)
	}

	if err := json.Unmarshal(body, &commandPayload); err != nil {
		return commandPayload, fmt.Errorf("decode update_all command payload: %w", err)
	}

	if len(commandPayload.ContainerIDs) == 0 {
		return commandPayload, errors.New("missing containerIds in payload")
	}

	seen := make(map[string]struct{}, len(commandPayload.ContainerIDs))
	normalized := make([]string, 0, len(commandPayload.ContainerIDs))
	for _, raw := range commandPayload.ContainerIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	if len(normalized) == 0 {
		return commandPayload, errors.New("containerIds contained no valid container IDs")
	}
	commandPayload.ContainerIDs = normalized

	return commandPayload, nil
}

// handleUpdateAllCommand handles the update_all command from Pulse.
// It updates each container sequentially to avoid overloading the runtime and registry.
func (a *Agent) handleUpdateAllCommand(ctx context.Context, target TargetConfig, command agentsdocker.Command) error {
	commandPayload, err := decodeUpdateAllPayload(command.Payload)
	if err != nil {
		a.logger.Error().Err(err).Msg("Update-all command missing or invalid containerIds in payload")
		if err := a.sendCommandAck(ctx, target, command.ID, agentsdocker.CommandStatusFailed, "Missing containerIds in payload"); err != nil {
			a.logger.Error().
				Err(err).
				Str("commandID", command.ID).
				Str("target", target.URL).
				Msg("Failed to send failure acknowledgement")
		}
		return nil
	}

	containerIDs := commandPayload.ContainerIDs
	total := len(containerIDs)

	a.logger.Info().
		Str("commandID", command.ID).
		Int("containers", total).
		Msg("Received update_all command from Pulse")

	if err := a.sendCommandAck(ctx, target, command.ID, agentsdocker.CommandStatusAcknowledged, fmt.Sprintf("Starting batch update (%d containers)", total)); err != nil {
		a.logger.Error().
			Err(err).
			Str("commandID", command.ID).
			Str("target", target.URL).
			Msg("Failed to send acknowledgement to Pulse")
		return nil
	}

	failures := 0
	for i, containerID := range containerIDs {
		index := i + 1

		progressFn := func(step string) {
			msg := fmt.Sprintf("[%d/%d] %s", index, total, step)
			if err := a.sendCommandAck(ctx, target, command.ID, agentsdocker.CommandStatusInProgress, msg); err != nil {
				a.logger.Warn().
					Err(err).
					Str("commandID", command.ID).
					Str("containerId", containerID).
					Str("target", target.URL).
					Msg("Failed to send batch progress update")
			}
		}

		progressFn(fmt.Sprintf("Updating container %s...", containerID))
		result := a.updateContainerWithProgress(ctx, containerID, func(step string) {
			progressFn(fmt.Sprintf("%s: %s", containerID, step))
		})

		if !result.Success {
			failures++
			progressFn(fmt.Sprintf("%s: FAILED (%s)", containerID, strings.TrimSpace(result.Error)))
			continue
		}

		progressFn(fmt.Sprintf("%s: completed (%s)", containerID, result.ContainerName))
	}

	status := agentsdocker.CommandStatusCompleted
	message := fmt.Sprintf("Batch update completed (%d/%d containers updated)", total-failures, total)
	if failures > 0 {
		status = agentsdocker.CommandStatusFailed
		message = fmt.Sprintf("Batch update finished with failures (%d/%d containers updated)", total-failures, total)
	}

	if err := a.sendCommandAck(ctx, target, command.ID, status, message); err != nil {
		a.logger.Error().
			Err(err).
			Str("commandID", command.ID).
			Str("target", target.URL).
			Str("status", status).
			Msg("Failed to send completion acknowledgement to Pulse")
	}

	return nil
}
