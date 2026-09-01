package hostagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
)

type proxmoxGuestCommandRunner func(context.Context, string, ...string) ([]byte, error)
type proxmoxTypedActionRunner func(context.Context, []string, typedActionCatalog, string, ...string) typedActionCommandResult

type proxmoxGuestLifecycleManager struct {
	run proxmoxGuestCommandRunner
	now func() time.Time
}

func newProxmoxGuestLifecycleManager() *proxmoxGuestLifecycleManager {
	return newProxmoxGuestLifecycleManagerWithTypedRunner(runTypedActionCommand)
}

func newProxmoxGuestLifecycleManagerWithTypedRunner(run proxmoxTypedActionRunner) *proxmoxGuestLifecycleManager {
	return &proxmoxGuestLifecycleManager{
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			catalog := typedActionCatalogProxmox
			if len(args) > 0 {
				catalog = typedActionCatalogForProxmoxInvocation(name, args[0])
			}
			result := run(ctx, nil, catalog, name, args...)
			return []byte(result.stdout), result.err
		},
		now: time.Now,
	}
}

func typedActionCatalogForProxmoxInvocation(tool, verb string) typedActionCatalog {
	if isProxmoxHandoffLifecycleInvocation(tool, verb) {
		return typedActionCatalogProxmoxHandoff
	}
	return typedActionCatalogProxmox
}

func (m *proxmoxGuestLifecycleManager) Apply(ctx context.Context, req agentexec.ProxmoxGuestLifecyclePayload) (result agentexec.ProxmoxGuestLifecycleResultPayload) {
	started := time.Now()
	result = agentexec.ProxmoxGuestLifecycleResultPayload{
		RequestID: req.RequestID, ActionID: req.ActionID, Operation: req.Operation,
		OperationVersion: req.OperationVersion, RequestDigest: req.RequestDigest,
		GuestKind: req.GuestKind, VMID: req.VMID, ExecutionPhase: agentexec.ProxmoxGuestPhasePreflight,
	}
	defer func() { result.Duration = time.Since(started).Milliseconds() }()
	if err := agentexec.ValidateProxmoxGuestLifecyclePayload(&req); err != nil {
		result.ReasonCode, result.Error = agentexec.ActionRefusalContractInvalid, "typed Proxmox lifecycle preflight refused"
		return result
	}
	before, err := m.inspect(ctx, req.GuestKind, req.VMID)
	result.Before = before
	if err != nil {
		result.ReasonCode, result.Error = agentexec.ActionRefusalTargetInspectionUnavailable, "Proxmox guest preflight status unavailable"
		return result
	}
	if before.Status != req.ExpectedStatus {
		result.ReasonCode, result.Error = agentexec.ActionRefusalTargetStateChanged, "Proxmox guest state changed before dispatch"
		return result
	}
	result.ExecutionPhase, result.MutationStarted = agentexec.ProxmoxGuestPhaseMutate, true
	tool := "qm"
	if req.GuestKind == "ct" {
		tool = "pct"
	}
	// This is the entire mutation catalog: fixed tool, fixed verb, decimal VMID.
	if _, err := m.run(ctx, tool, req.Operation, strconv.Itoa(req.VMID)); err != nil {
		if ctx.Err() != nil || errors.Is(err, errTypedActionContainmentIndeterminate) {
			result.Error = "Proxmox guest lifecycle mutation was interrupted after dispatch; target state requires recovery inspection"
		} else {
			result.Error = "Proxmox guest lifecycle mutation did not complete"
		}
		return result
	}
	result.MutationCompleted, result.ExecutionPhase = true, agentexec.ProxmoxGuestPhaseVerify
	after, err := m.inspect(ctx, req.GuestKind, req.VMID)
	result.After = after
	if err != nil {
		result.Error = "Proxmox guest postcondition status unavailable"
		return result
	}
	result.ReadbackRan = true
	if proxmoxGuestLifecyclePostcondition(req.Operation, after.Status) {
		result.ExecutionPhase = agentexec.ProxmoxGuestPhaseComplete
		return result
	}
	result.Error = "Proxmox guest postcondition contradicted the requested state"
	return result
}

func (m *proxmoxGuestLifecycleManager) inspect(ctx context.Context, kind string, vmid int) (agentexec.ProxmoxGuestLifecycleSnapshot, error) {
	tool := "qm"
	if kind == "ct" {
		tool = "pct"
	}
	out, err := m.run(ctx, tool, "status", strconv.Itoa(vmid))
	if err != nil {
		return agentexec.ProxmoxGuestLifecycleSnapshot{}, err
	}
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(string(out))))
	if len(fields) != 2 || fields[0] != "status:" || (fields[1] != "running" && fields[1] != "stopped") {
		return agentexec.ProxmoxGuestLifecycleSnapshot{}, fmt.Errorf("unexpected Proxmox status response")
	}
	now := time.Now().UTC()
	if m.now != nil {
		now = m.now().UTC()
	}
	return agentexec.ProxmoxGuestLifecycleSnapshot{Status: fields[1], ObservedAt: now}, nil
}

func proxmoxGuestLifecyclePostcondition(operation, status string) bool {
	if operation == "stop" || operation == "shutdown" {
		return status == "stopped"
	}
	return status == "running"
}

func (c *CommandClient) handleProxmoxGuestLifecycle(ctx context.Context, conn *websocket.Conn, payload agentexec.ProxmoxGuestLifecyclePayload) {
	identity := agentexec.ProxmoxGuestLifecycleOperationIdentity(c.agentID, payload)
	record, admitted, err := c.admitOperation(identity)
	if err != nil {
		return
	}
	if !admitted {
		if record.State == operationreceipt.StateTerminal {
			var result agentexec.ProxmoxGuestLifecycleResultPayload
			if json.Unmarshal(record.Result, &result) == nil {
				c.sendProxmoxGuestLifecycleResult(conn, result)
			}
		}
		return
	}
	if _, err := c.operationReceipts.MarkStarted(identity); err != nil {
		return
	}
	operationCtx, cancel := context.WithTimeout(ctx, time.Duration(payload.Timeout)*time.Second)
	state, registered := c.registerActiveCommand(conn, payload.RequestID, cancel)
	defer c.finishCancellableRequest(conn, payload.RequestID, state)
	defer cancel()
	result := agentexec.ProxmoxGuestLifecycleResultPayload{
		RequestID: payload.RequestID, ActionID: payload.ActionID, Operation: payload.Operation,
		OperationVersion: payload.OperationVersion, RequestDigest: payload.RequestDigest,
		GuestKind: payload.GuestKind, VMID: payload.VMID, ExecutionPhase: agentexec.ProxmoxGuestPhasePreflight,
	}
	if !registered || operationCtx.Err() != nil {
		result.Error = "Proxmox guest lifecycle canceled before mutation dispatch"
	} else {
		result = c.proxmoxGuestLifecycle.Apply(operationCtx, payload)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return
	}
	if _, err := c.operationReceipts.Complete(identity, operationreceipt.TerminalEnvelope{Kind: agentexec.ProxmoxGuestLifecycleReceiptKind, Version: agentexec.ProxmoxGuestLifecycleReceiptVersion, Payload: encoded}); err != nil {
		return
	}
	c.sendProxmoxGuestLifecycleResult(conn, result)
}

func (c *CommandClient) sendProxmoxGuestLifecycleResult(conn *websocket.Conn, result agentexec.ProxmoxGuestLifecycleResultPayload) {
	encoded, err := json.Marshal(result)
	if err != nil {
		return
	}
	c.connMu.Lock()
	err = conn.WriteJSON(wsMessage{Type: msgTypeProxmoxGuestLifecycleResult, ID: result.RequestID, Timestamp: time.Now(), Payload: encoded})
	c.connMu.Unlock()
	if err != nil {
		c.logger.Error().Err(err).Str("request_id", result.RequestID).Msg("Failed to send Proxmox lifecycle result")
	}
}
