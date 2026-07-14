package hostagent

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
)

// DockerContainerUpdater bridges the typed container update operation to the
// Docker / Podman module's update implementation when the unified agent runs
// both modules. Implementations own pull, backup, recreate, verification, and
// rollback; the error return is reserved for infrastructural refusals (module
// missing, runtime mismatch, preflight drift) as opposed to update failures,
// which arrive inside the outcome.
type DockerContainerUpdater interface {
	TypedContainerUpdate(ctx context.Context, runtime, containerID, expectedImageDigest string, progress func(string)) (agentexec.DockerContainerUpdateOutcome, error)
}

func (c *CommandClient) handleDockerContainerUpdate(ctx context.Context, conn *websocket.Conn, payload agentexec.DockerContainerUpdatePayload) {
	identity := agentexec.DockerContainerUpdateOperationIdentity(c.agentID, payload)
	record, admitted, err := c.admitOperation(identity)
	if err != nil {
		c.logger.Warn().Err(err).Str("request_id", payload.RequestID).Msg("Docker update durable admission refused")
		return
	}
	if !admitted {
		if record.State == operationreceipt.StateTerminal {
			c.replayDockerContainerUpdate(conn, record)
		}
		return
	}
	if _, err := c.operationReceipts.MarkStarted(identity); err != nil {
		c.logger.Warn().Err(err).Str("request_id", payload.RequestID).Msg("Docker update durable start refused")
		return
	}
	timeout := time.Duration(payload.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	operationCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := c.runDockerContainerUpdate(operationCtx, payload)
	encoded, err := json.Marshal(result)
	if err != nil {
		return
	}
	if _, err := c.operationReceipts.Complete(identity, operationreceipt.TerminalEnvelope{Kind: agentexec.DockerContainerUpdateReceiptKind, Version: agentexec.DockerContainerUpdateReceiptVersion, Payload: encoded}); err != nil {
		c.logger.Error().Err(err).Str("request_id", payload.RequestID).Msg("Failed to persist docker update terminal receipt")
		return
	}
	c.sendDockerContainerUpdateResult(conn, result)
}

func (c *CommandClient) runDockerContainerUpdate(ctx context.Context, payload agentexec.DockerContainerUpdatePayload) agentexec.DockerContainerUpdateResultPayload {
	started := time.Now()
	result := agentexec.DockerContainerUpdateResultPayload{
		RequestID: payload.RequestID, ActionID: payload.ActionID, Operation: payload.Operation,
		OperationVersion: payload.OperationVersion, RequestDigest: payload.RequestDigest, ContainerID: payload.ContainerID,
		ExecutionPhase: agentexec.DockerContainerPhasePreflight,
	}
	defer func() { result.Duration = time.Since(started).Milliseconds() }()

	if err := agentexec.ValidateDockerContainerUpdatePayload(&payload); err != nil {
		c.logger.Warn().Err(err).Str("request_id", payload.RequestID).Msg("Docker update preflight refused: invalid payload")
		result.Error = "typed container update preflight refused"
		return result
	}
	if c.dockerUpdater == nil {
		c.logger.Warn().Str("request_id", payload.RequestID).Msg("Docker update refused: no docker module bridge wired")
		result.Error = "docker module is not available on this agent"
		return result
	}
	progress := func(step string) {
		c.logger.Info().Str("request_id", payload.RequestID).Str("container_id", payload.ContainerID).Msg(strings.TrimSpace(step))
	}
	outcome, err := c.dockerUpdater.TypedContainerUpdate(ctx, payload.Runtime, payload.ContainerID, payload.ExpectedImageDigest, progress)
	if err != nil {
		c.logger.Warn().Err(err).Str("request_id", payload.RequestID).Str("container_id", payload.ContainerID).Msg("Docker update refused before mutation")
		result.Error = boundDockerUpdateError(err.Error())
		return result
	}

	result.ContainerName = outcome.ContainerName
	result.OldImageDigest = outcome.OldImageDigest
	result.NewImageDigest = outcome.NewImageDigest
	result.BackupCreated = outcome.BackupCreated
	result.BackupContainer = outcome.BackupContainer
	result.RollbackAttempted = outcome.RollbackAttempted
	result.RolledBack = outcome.RolledBack
	// The backup rename is the first mutating step the outcome can attest to;
	// failures before it (inspect, pull, stop) leave the tree unmutated.
	result.MutationStarted = outcome.BackupCreated || outcome.RollbackAttempted || outcome.Success
	if result.MutationStarted {
		result.ExecutionPhase = agentexec.DockerContainerPhaseMutate
	}
	result.NewContainerID = strings.ToLower(strings.TrimSpace(outcome.NewContainerID))
	if !outcome.Success || result.NewContainerID == "" {
		result.Error = boundDockerUpdateError(outcome.Error)
		if result.Error == "" {
			result.Error = "container update did not complete"
		}
		return result
	}

	result.MutationCompleted = true
	result.ExecutionPhase = agentexec.DockerContainerPhaseVerify

	// The module verified the replacement (start + stability window) before
	// declaring success; read the replacement state back through the same
	// inspector the lifecycle path uses so the server gets an observation.
	if inspector, ok := c.dockerLifecycle.(*localDockerLifecycleManager); ok && inspector != nil && result.NewContainerID != "" {
		if after, inspectErr := inspector.inspect(ctx, payload.Runtime, result.NewContainerID); inspectErr == nil {
			result.After = after
			result.ReadbackRan = true
		}
	}
	result.ExecutionPhase = agentexec.DockerContainerPhaseComplete
	return result
}

func boundDockerUpdateError(message string) string {
	message = strings.TrimSpace(message)
	if len(message) > 1024 {
		message = message[:1024]
	}
	return message
}

func (c *CommandClient) sendDockerContainerUpdateResult(conn *websocket.Conn, result agentexec.DockerContainerUpdateResultPayload) {
	encoded, err := json.Marshal(result)
	if err != nil {
		return
	}
	msg := wsMessage{Type: msgTypeDockerContainerUpdateResult, ID: result.RequestID, Timestamp: time.Now(), Payload: encoded}
	c.connMu.Lock()
	err = conn.WriteJSON(msg)
	c.connMu.Unlock()
	if err != nil {
		c.logger.Error().Err(err).Str("request_id", result.RequestID).Msg("Failed to send docker update result")
	}
}

func (c *CommandClient) replayDockerContainerUpdate(conn *websocket.Conn, record operationreceipt.Record) {
	var result agentexec.DockerContainerUpdateResultPayload
	if err := json.Unmarshal(record.Result, &result); err == nil {
		c.sendDockerContainerUpdateResult(conn, result)
	}
}
