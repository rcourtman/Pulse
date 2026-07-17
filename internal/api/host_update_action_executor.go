package api

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const (
	hostPackageUpdateActionHandler = "host.package_updates"
	hostPackageUpdateCapability    = "install_os_updates"
)

var hostPackageUpdateInventoryHashPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

type hostUpdateAgentCommander interface {
	ExecuteHostUpdate(ctx context.Context, agentID string, req agentexec.HostUpdatePayload) (*agentexec.HostUpdateResultPayload, error)
	IsAgentConnected(agentID string) bool
}

type operationReceiptQuerier interface {
	QueryAgentOperation(context.Context, string, operationreceipt.Identity) (operationreceipt.QueryResult, error)
}
type agentOperationReceiptCapability interface{ AgentOperationReceiptVersion(string) int }

func (e hostUpdateActionExecutor) BindActionDispatch(ctx context.Context, record unified.ActionAuditRecord, attempt unified.ActionDispatchAttempt) (unified.ActionDispatchAttempt, error) {
	resource, err := e.currentResource(ctx, record.Request.ResourceID)
	if err != nil {
		return unified.ActionDispatchAttempt{}, err
	}
	req := agentexec.HostUpdatePayload{RequestID: attempt.ID, ActionID: record.ID, Operation: agentexec.HostUpdateOperationInstall, ExpectedInventoryHash: resource.Agent.PackageUpdates.InventoryHash}
	if err := agentexec.BindHostUpdatePayload(&req); err != nil {
		return unified.ActionDispatchAttempt{}, err
	}
	return unified.BindActionDispatchAttempt(attempt, unified.ActionDispatchBinding{OperationKind: req.Operation, OperationVersion: req.OperationVersion, RequestDigest: req.RequestDigest, AgentID: resource.Agent.AgentID})
}

type hostUpdateActionExecutor struct {
	resources *ResourceHandlers
	agents    hostUpdateAgentCommander
	now       func() time.Time
}

func newHostUpdateActionExecutor(resources *ResourceHandlers, agents hostUpdateAgentCommander) ActionExecutor {
	if resources == nil || agents == nil {
		return nil
	}
	return hostUpdateActionExecutor{resources: resources, agents: agents, now: time.Now}
}

func (e hostUpdateActionExecutor) ActionHandlerNames() []string {
	return []string{hostPackageUpdateActionHandler}
}

func (e hostUpdateActionExecutor) CheckActionAvailable(ctx context.Context, req unified.ActionRequest, resource unified.Resource) unified.ResourceActionReadiness {
	if strings.TrimSpace(req.CapabilityName) != hostPackageUpdateCapability {
		return unified.ResourceActionReadiness{}
	}
	capability, ok := resourceCapabilityByName(resource.Capabilities, hostPackageUpdateCapability)
	if !ok || capability.InternalHandler != hostPackageUpdateActionHandler {
		return unified.ResourceActionReadiness{}
	}
	if err := e.validateResource(resource); err != nil {
		return unified.ResourceActionReadiness{
			Name:       hostPackageUpdateCapability,
			Available:  false,
			ReasonCode: hostUpdateUnavailableReasonCode(err),
			Reason:     hostUpdateUnavailableReason(err),
		}
	}
	return unified.ResourceActionReadiness{Name: hostPackageUpdateCapability, Available: true}
}

func (e hostUpdateActionExecutor) ExecuteAction(ctx context.Context, record unified.ActionAuditRecord) (*unified.ExecutionResult, error) {
	attempt, ok := actionlifecycle.DispatchAttemptFromContext(ctx)
	if !ok || attempt.ActionID != record.ID {
		return nil, fmt.Errorf("committed action dispatch authority is required")
	}
	record, err := unified.NormalizeActionAuditRecord(record)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(record.Request.CapabilityName) != hostPackageUpdateCapability {
		return nil, fmt.Errorf("unsupported host update capability %q", record.Request.CapabilityName)
	}
	resource, err := e.currentResource(ctx, record.Request.ResourceID)
	if err != nil {
		return nil, err
	}
	agentID := strings.TrimSpace(resource.Agent.AgentID)
	req := agentexec.HostUpdatePayload{
		RequestID:             attempt.ID,
		ActionID:              record.ID,
		Operation:             agentexec.HostUpdateOperationInstall,
		ExpectedInventoryHash: resource.Agent.PackageUpdates.InventoryHash,
		Timeout:               900,
	}
	if err := agentexec.BindHostUpdatePayload(&req); err != nil {
		return nil, err
	}
	if agentexec.HostUpdateOperationIdentity(agentID, req) != (operationreceipt.Identity{AttemptID: attempt.ID, ActionID: attempt.ActionID, OperationKind: attempt.OperationKind, OperationVersion: attempt.OperationVersion, RequestDigest: attempt.RequestDigest, AgentID: attempt.AgentID}) {
		return nil, fmt.Errorf("host update dispatch binding drift")
	}
	result, err := e.agents.ExecuteHostUpdate(ctx, agentID, req)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("host update agent returned no result")
	}
	receivedAt := e.currentTime()
	if err := agentexec.ValidateHostUpdateResultForRequestAt(req, *result, receivedAt); err != nil {
		return nil, fmt.Errorf("invalid host update agent result: %w", err)
	}

	output := hostUpdateResultSummary(*result)
	beforeBound := result.Before.InventoryHash == resource.Agent.PackageUpdates.InventoryHash
	return hostAPTExecutionResult(record.Request.ResourceID, agentID, agentexec.HostUpdateOperationInstall, output, result.Success, result.MutationStarted, result.Verification, beforeBound, true, result.HealthChecked, result.PackageManagerHealthy, result.RecoveryRequired, result.Before.CheckedAt, result.After.CheckedAt, receivedAt, receivedAt)
}

func (e hostUpdateActionExecutor) ReconcileActionDispatch(ctx context.Context, record unified.ActionAuditRecord, attempt unified.ActionDispatchAttempt) (*unified.ExecutionResult, unified.ActionDispatchReceipt, bool, error) {
	identity := operationreceipt.Identity{AttemptID: attempt.ID, ActionID: attempt.ActionID, OperationKind: attempt.OperationKind, OperationVersion: attempt.OperationVersion, RequestDigest: attempt.RequestDigest, AgentID: attempt.AgentID}
	querier, ok := e.agents.(operationReceiptQuerier)
	if !ok {
		return nil, unified.ActionDispatchReceipt{}, false, nil
	}
	query, err := querier.QueryAgentOperation(ctx, attempt.AgentID, identity)
	if err != nil {
		return nil, unified.ActionDispatchReceipt{}, false, err
	}
	if query.Status != operationreceipt.QueryFoundTerminal {
		return nil, unified.ActionDispatchReceipt{}, false, nil
	}
	receivedAt := e.currentTime()
	if err := agentexec.ValidateOperationQueryResultForIdentity(query, identity, receivedAt); err != nil {
		return nil, unified.ActionDispatchReceipt{}, false, err
	}
	result, err := agentexec.DecodeHostUpdateResultPayload(query.Record.Result)
	if err != nil {
		return nil, unified.ActionDispatchReceipt{}, false, err
	}
	req := agentexec.HostUpdatePayload{RequestID: attempt.ID, ActionID: record.ID, Operation: attempt.OperationKind, OperationVersion: attempt.OperationVersion, RequestDigest: attempt.RequestDigest, ExpectedInventoryHash: result.Before.InventoryHash}
	if err := agentexec.ValidateHostUpdateResultForRequest(req, result); err != nil {
		return nil, unified.ActionDispatchReceipt{}, false, err
	}
	output := hostUpdateResultSummary(result)
	execution, buildErr := hostAPTExecutionResult(record.Request.ResourceID, attempt.AgentID, attempt.OperationKind, output, result.Success, result.MutationStarted, result.Verification, true, true, result.HealthChecked, result.PackageManagerHealthy, result.RecoveryRequired, result.Before.CheckedAt, result.After.CheckedAt, query.Record.TerminalAt, receivedAt)
	if buildErr != nil {
		return nil, unified.ActionDispatchReceipt{}, false, buildErr
	}
	receipt := unified.ActionDispatchReceipt{AttemptID: attempt.ID, ActionID: record.ID, TransportRequestID: attempt.ID, ReceivedAt: receivedAt}
	return execution, receipt, true, nil
}

func hostUpdateResultSummary(result agentexec.HostUpdateResultPayload) string {
	health := "unknown"
	if result.HealthChecked {
		health = "unhealthy"
		if result.PackageManagerHealthy {
			health = "healthy"
		}
	}
	return fmt.Sprintf("APT package updates: phase=%s; %d pending before, %d pending after; package manager health: %s; recovery required: %t; reboot required: %t", result.ExecutionPhase, result.Before.PendingCount, result.After.PendingCount, health, result.RecoveryRequired, result.After.RebootRequired)
}

func (e hostUpdateActionExecutor) currentResource(ctx context.Context, resourceID string) (unified.Resource, error) {
	if e.resources == nil {
		return unified.Resource{}, fmt.Errorf("resource handler unavailable")
	}
	registry, err := e.resources.buildRegistry(GetOrgID(ctx))
	if err != nil {
		return unified.Resource{}, err
	}
	resource, ok := registry.Get(resourceID)
	if !ok || resource == nil {
		return unified.Resource{}, fmt.Errorf("resource %q is no longer present", resourceID)
	}
	if err := e.validateResource(*resource); err != nil {
		return unified.Resource{}, err
	}
	return *resource, nil
}

func (e hostUpdateActionExecutor) validateResource(resource unified.Resource) error {
	if resource.Type != unified.ResourceTypeAgent || resource.Agent == nil {
		return fmt.Errorf("resource is not an agent-managed host")
	}
	capability, ok := resourceCapabilityByName(resource.Capabilities, hostPackageUpdateCapability)
	if !ok || capability.InternalHandler != hostPackageUpdateActionHandler {
		return fmt.Errorf("host does not currently advertise typed OS updates")
	}
	if !resource.Agent.CommandsEnabled {
		return fmt.Errorf("host command operations are disabled")
	}
	if resource.Agent.OperationReceiptVersion != operationreceipt.ProtocolVersion {
		return fmt.Errorf("durable operation receipt protocol is unsupported")
	}
	status := resource.Agent.PackageUpdates
	if status == nil || !status.Supported || strings.TrimSpace(status.Manager) != "apt" {
		return fmt.Errorf("host package manager is unsupported")
	}
	if strings.TrimSpace(status.Error) != "" {
		return fmt.Errorf("host package update inventory has an error")
	}
	if !hostPackageUpdateInventoryHashPattern.MatchString(strings.TrimSpace(status.InventoryHash)) {
		return fmt.Errorf("host package update inventory fingerprint is invalid")
	}
	if !unified.HostPackageUpdateTelemetryFresh(status, e.currentTime()) {
		return fmt.Errorf("host package update inventory is stale")
	}
	if status.PendingCount <= 0 {
		return fmt.Errorf("host has no pending package updates")
	}
	agentID := strings.TrimSpace(resource.Agent.AgentID)
	liveCapability, supported := e.agents.(agentOperationReceiptCapability)
	if agentID == "" || e.agents == nil || !e.agents.IsAgentConnected(agentID) {
		return fmt.Errorf("host command agent is disconnected")
	}
	if !supported || liveCapability.AgentOperationReceiptVersion(agentID) != operationreceipt.ProtocolVersion {
		return fmt.Errorf("live durable operation receipt protocol is unsupported")
	}
	return nil
}

func (e hostUpdateActionExecutor) currentTime() time.Time {
	if e.now != nil {
		return e.now().UTC()
	}
	return time.Now().UTC()
}

func hostUpdateUnavailableReasonCode(err error) string {
	if err == nil {
		return "unavailable"
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "not an agent-managed host"):
		return "unsupported_resource"
	case strings.Contains(message, "disabled"):
		return "host_commands_disabled"
	case strings.Contains(message, "receipt protocol"):
		return "operation_receipt_unsupported"
	case strings.Contains(message, "unsupported"):
		return "unsupported_package_manager"
	case strings.Contains(message, "inventory") && strings.Contains(message, "error"):
		return "package_inventory_error"
	case strings.Contains(message, "fingerprint"):
		return "invalid_package_inventory"
	case strings.Contains(message, "stale"):
		return "stale_package_inventory"
	case strings.Contains(message, "no pending"):
		return "no_pending_updates"
	case strings.Contains(message, "disconnected"):
		return "command_agent_disconnected"
	default:
		return "capability_unavailable"
	}
}

func hostUpdateUnavailableReason(err error) string {
	switch hostUpdateUnavailableReasonCode(err) {
	case "unsupported_resource":
		return "This resource is not an agent-managed host."
	case "host_commands_disabled":
		return "Typed host operations are disabled for this agent."
	case "operation_receipt_unsupported":
		return "The Pulse agent on this host cannot run reviewed actions: it is on an older version, or its durable state directory is unavailable. Update the agent, or check the agent logs if it is already current, then retry."
	case "unsupported_package_manager":
		return "This host does not expose a supported package manager."
	case "package_inventory_error":
		return "Pulse could not inspect the host's package update state."
	case "invalid_package_inventory":
		return "The host package inventory cannot be verified safely."
	case "stale_package_inventory":
		return "The host package inventory is too old to update safely."
	case "no_pending_updates":
		return "The host has no pending standard package updates."
	case "command_agent_disconnected":
		return "The host command agent is not connected."
	default:
		return "Typed host package updates are not currently available."
	}
}
