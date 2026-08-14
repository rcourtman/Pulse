package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// strandedActionDispatchWindow bounds how long a committed dispatch may sit
// without any durable completion evidence before Pulse stops waiting and
// terminalizes the audit row. It is deliberately the same threshold the
// pulse-intelligence telemetry uses to call an executing action stuck, so an
// action can never be reported stuck while reconciliation is still waiting on
// it. Every typed operation timeout is far shorter than this window, so an
// in-flight mutation is never cut short by it.
const strandedActionDispatchWindow = pulseIntelligenceStuckExecutingThreshold

// strandedActionDispatchClassifiable reports whether a non-terminal receipt
// query carries enough protocol meaning to be reasoned about at all. An
// unrecognized status is not evidence of anything, so the attempt stays
// receipt_pending rather than being validated or terminalized.
func strandedActionDispatchClassifiable(status operationreceipt.QueryStatus) bool {
	return status == operationreceipt.QueryNotFound || status == operationreceipt.QueryFoundInterrupted
}

// strandedActionDispatch converts an authenticated, correlated non-terminal
// receipt query into terminal audit truth when the agent can no longer produce
// completion evidence for the committed attempt. It returns found=false while
// evidence may still arrive, so the deliberate receipt_pending semantics stay:
// a transport error never reaches here, and a fresh missing receipt keeps
// waiting. Callers must have validated the query against the attempt identity
// and must not call it for a terminal receipt, which carries real result truth.
func strandedActionDispatch(query operationreceipt.QueryResult, record unified.ActionAuditRecord, attempt unified.ActionDispatchAttempt, now time.Time) (*unified.ExecutionResult, unified.ActionDispatchReceipt, bool) {
	reasonCode, summary, stranded := strandedActionDispatchReason(query, attempt, now)
	if !stranded {
		return nil, unified.ActionDispatchReceipt{}, false
	}
	result := actionlifecycle.StrandedDispatchResult(reasonCode, summary, record.Plan.RollbackAvailable)
	receipt := unified.ActionDispatchReceipt{
		AttemptID: attempt.ID, ActionID: record.ID, TransportRequestID: attempt.ID, ReceivedAt: now.UTC(),
	}
	return result, receipt, true
}

// strandedActionDispatchReason classifies a non-terminal receipt query. An
// interrupted or tombstoned receipt is unrecoverable the moment it is read:
// the agent-side store only marks a receipt interrupted when the agent
// restarted after admitting the operation, and no interrupted or tombstoned
// receipt can ever become terminal again. A receipt that is merely still
// accepted or started, or one the agent has no record of at all, may still be
// completed, so those only strand once the bounded window has elapsed.
func strandedActionDispatchReason(query operationreceipt.QueryResult, attempt unified.ActionDispatchAttempt, now time.Time) (string, string, bool) {
	switch query.Status {
	case operationreceipt.QueryNotFound:
		if !strandedActionDispatchWindowElapsed(attempt, now) {
			return "", "", false
		}
		return "dispatch_evidence_missing", "The Pulse agent has no record of this operation and no completion evidence arrived within the reconciliation window, so Pulse cannot confirm whether it took effect. Check the resource directly before retrying.", true
	case operationreceipt.QueryFoundInterrupted:
		if query.Record == nil {
			return "", "", false
		}
		switch query.Record.State {
		case operationreceipt.StateInterrupted:
			return "dispatch_evidence_interrupted", "The Pulse agent restarted while this operation was running and kept no durable completion evidence, so Pulse cannot confirm whether it took effect. Check the resource directly before retrying.", true
		case operationreceipt.StateTombstone:
			return "dispatch_evidence_discarded", "The Pulse agent discarded this operation's durable completion evidence before Pulse could read it, so Pulse cannot confirm whether it took effect. Check the resource directly before retrying.", true
		default:
			if !strandedActionDispatchWindowElapsed(attempt, now) {
				return "", "", false
			}
			return "dispatch_evidence_incomplete", "The Pulse agent still reports this operation as unfinished long after it was dispatched, so Pulse cannot confirm whether it took effect. Check the resource directly before retrying.", true
		}
	default:
		return "", "", false
	}
}

// strandedActionDispatchWindowElapsed measures from the last durable transport
// transition on the attempt. An attempt with no usable timestamp never strands
// on its own; the operator override remains the escape hatch for it.
func strandedActionDispatchWindowElapsed(attempt unified.ActionDispatchAttempt, now time.Time) bool {
	since := attempt.UpdatedAt
	if since.Before(attempt.CreatedAt) {
		since = attempt.CreatedAt
	}
	if since.IsZero() || now.IsZero() {
		return false
	}
	return !now.UTC().Before(since.UTC().Add(strandedActionDispatchWindow))
}

type actionAgentCommander interface {
	ExecuteCommand(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error)
	GetAgentForHost(hostname string) (string, bool)
	IsAgentConnected(agentID string) bool
}

type actionPreflightAgentCommander interface {
	PreflightAction(context.Context, string, agentexec.ActionPreflightPayload) (*agentexec.ActionPreflightResultPayload, error)
}

type scopedActionAgentCommander interface {
	IsAgentConnectedForOrganization(organizationID, agentID string) bool
	GetAgentForHostForOrganization(organizationID, hostname string) (string, bool)
	GetAgentForTokenForOrganization(organizationID, tokenID string) (string, bool)
}

type scopedAgentOperationReceiptCapability interface {
	AgentOperationReceiptVersionForOrganization(organizationID, agentID string) int
}

type tenantAgentServer interface {
	GetConnectedAgents() []agentexec.ConnectedAgent
	ExecuteCommand(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error)
}

func tenantAgentServerForOrganization(server *agentexec.Server, organizationID string) tenantAgentServer {
	if server == nil {
		return nil
	}
	return server.ForOrganization(organizationID)
}

func agentCommandContext(ctx context.Context) context.Context {
	return agentexec.WithOrganizationID(ctx, GetOrgID(ctx))
}

func isAgentCommandConnected(ctx context.Context, agents any, agentID string) bool {
	if scoped, ok := agents.(scopedActionAgentCommander); ok {
		return scoped.IsAgentConnectedForOrganization(GetOrgID(ctx), agentID)
	}
	if legacy, ok := agents.(interface{ IsAgentConnected(string) bool }); ok {
		return legacy.IsAgentConnected(agentID)
	}
	return false
}

func commandAgentForHost(ctx context.Context, agents actionAgentCommander, hostname string) (string, bool) {
	if scoped, ok := agents.(scopedActionAgentCommander); ok {
		return scoped.GetAgentForHostForOrganization(GetOrgID(ctx), hostname)
	}
	if agents == nil {
		return "", false
	}
	return agents.GetAgentForHost(hostname)
}

func commandAgentForToken(ctx context.Context, agents actionAgentCommander, tokenID string) (string, bool) {
	if strings.TrimSpace(tokenID) == "" {
		return "", false
	}
	if scoped, ok := agents.(scopedActionAgentCommander); ok {
		return scoped.GetAgentForTokenForOrganization(GetOrgID(ctx), tokenID)
	}
	return "", false
}

func liveAgentOperationReceiptVersion(ctx context.Context, agents any, agentID string) int {
	if scoped, ok := agents.(scopedAgentOperationReceiptCapability); ok {
		return scoped.AgentOperationReceiptVersionForOrganization(GetOrgID(ctx), agentID)
	}
	if capability, ok := agents.(agentOperationReceiptCapability); ok {
		return capability.AgentOperationReceiptVersion(agentID)
	}
	return 0
}

type actionHandlerProvider interface {
	ActionHandlerNames() []string
}

type actionDispatchOperationProvider interface {
	ActionDispatchOperationKinds() []string
}

type routedActionExecutor struct {
	resources   *ResourceHandlers
	byHandler   map[string]ActionExecutor
	byOperation map[string]ActionExecutor
}

func newRoutedActionExecutor(resources *ResourceHandlers, executors ...ActionExecutor) ActionExecutor {
	if resources == nil {
		return nil
	}
	routed := routedActionExecutor{
		resources:   resources,
		byHandler:   map[string]ActionExecutor{},
		byOperation: map[string]ActionExecutor{},
	}
	for _, executor := range executors {
		if executor == nil {
			continue
		}
		provider, ok := executor.(actionHandlerProvider)
		if !ok {
			continue
		}
		for _, handler := range provider.ActionHandlerNames() {
			handler = strings.TrimSpace(handler)
			if handler != "" {
				routed.byHandler[handler] = executor
			}
		}
		if operations, ok := executor.(actionDispatchOperationProvider); ok {
			for _, operation := range operations.ActionDispatchOperationKinds() {
				operation = strings.TrimSpace(operation)
				if operation != "" {
					routed.byOperation[operation] = executor
				}
			}
		}
	}
	if len(routed.byHandler) == 0 {
		return nil
	}
	return routed
}

func (e routedActionExecutor) ExecuteAction(ctx context.Context, record unified.ActionAuditRecord) (*unified.ExecutionResult, error) {
	normalized, err := unified.NormalizeActionAuditRecord(record)
	if err != nil {
		return nil, err
	}
	executor, err := e.executorForAction(ctx, normalized.Request)
	if err != nil {
		return nil, err
	}
	return executor.ExecuteAction(ctx, normalized)
}

func (e routedActionExecutor) BindActionDispatch(ctx context.Context, record unified.ActionAuditRecord, attempt unified.ActionDispatchAttempt) (unified.ActionDispatchAttempt, error) {
	executor, err := e.executorForAction(ctx, record.Request)
	if err != nil {
		return unified.ActionDispatchAttempt{}, err
	}
	binder, ok := executor.(actionlifecycle.DispatchBinder)
	if !ok {
		return attempt, nil
	}
	return binder.BindActionDispatch(ctx, record, attempt)
}

func (e routedActionExecutor) ReconcileActionDispatch(ctx context.Context, record unified.ActionAuditRecord, attempt unified.ActionDispatchAttempt) (*unified.ExecutionResult, unified.ActionDispatchReceipt, bool, error) {
	executor, err := e.executorForDispatchAttempt(record, attempt)
	if err != nil {
		return nil, unified.ActionDispatchReceipt{}, false, err
	}
	reconciler, ok := executor.(actionlifecycle.DispatchReconciler)
	if !ok {
		return nil, unified.ActionDispatchReceipt{}, false, nil
	}
	return reconciler.ReconcileActionDispatch(ctx, record, attempt)
}

// executorForDispatchAttempt routes query-only recovery from the immutable
// operation binding committed before dispatch. Current inventory is
// deliberately not consulted: a successful mutation can replace the resource,
// while a safe preflight refusal can remove the capability that originally
// admitted the action. Either outcome must still be able to consume its exact
// durable receipt and terminalize the audit row.
func (e routedActionExecutor) executorForDispatchAttempt(record unified.ActionAuditRecord, attempt unified.ActionDispatchAttempt) (ActionExecutor, error) {
	if strings.TrimSpace(attempt.ActionID) == "" || attempt.ActionID != record.ID {
		return nil, fmt.Errorf("durable dispatch attempt does not belong to action %q", record.ID)
	}
	operation := strings.TrimSpace(attempt.OperationKind)
	if operation == "" {
		return nil, fmt.Errorf("durable dispatch operation binding is missing")
	}
	executor := e.byOperation[operation]
	if executor == nil {
		return nil, fmt.Errorf("unsupported durable dispatch operation %q", operation)
	}
	return executor, nil
}

func (e routedActionExecutor) CheckActionAvailable(ctx context.Context, req unified.ActionRequest, resource unified.Resource) unified.ResourceActionReadiness {
	capability, ok := resourceCapabilityByName(resource.Capabilities, req.CapabilityName)
	if !ok || strings.TrimSpace(capability.InternalHandler) == "" {
		return unified.ResourceActionReadiness{}
	}
	executor := e.byHandler[strings.TrimSpace(capability.InternalHandler)]
	if executor == nil {
		return unified.ResourceActionReadiness{
			Name:       strings.TrimSpace(req.CapabilityName),
			Available:  false,
			ReasonCode: "unsupported_handler",
			Reason:     "This action is not routed through a supported executor.",
		}
	}
	checker, ok := executor.(ActionAvailabilityChecker)
	if !ok {
		return unified.ResourceActionReadiness{}
	}
	return checker.CheckActionAvailable(ctx, req, resource)
}

func (e routedActionExecutor) CheckActionFeasible(ctx context.Context, actionID string, req unified.ActionRequest, resource unified.Resource) unified.ResourceActionReadiness {
	capability, ok := resourceCapabilityByName(resource.Capabilities, req.CapabilityName)
	if !ok || strings.TrimSpace(capability.InternalHandler) == "" {
		return unified.ResourceActionReadiness{}
	}
	executor := e.byHandler[strings.TrimSpace(capability.InternalHandler)]
	if executor == nil {
		return unified.ResourceActionReadiness{}
	}
	checker, ok := executor.(interface {
		CheckActionFeasible(context.Context, string, unified.ActionRequest, unified.Resource) unified.ResourceActionReadiness
	})
	if !ok {
		return unified.ResourceActionReadiness{}
	}
	return checker.CheckActionFeasible(ctx, actionID, req, resource)
}

func actionPreflightReadiness(capability string, result *agentexec.ActionPreflightResultPayload, err error) unified.ResourceActionReadiness {
	readiness := unified.ResourceActionReadiness{Name: strings.TrimSpace(capability), Available: false}
	if err != nil || result == nil {
		readiness.ReasonCode = "agent_preflight_unavailable"
		readiness.Reason = "Pulse could not confirm that the exact operation is currently feasible on the target agent."
		return readiness
	}
	if result.Feasible {
		readiness.Available = true
		return readiness
	}
	readiness.ReasonCode = strings.TrimSpace(result.ReasonCode)
	switch readiness.ReasonCode {
	case agentexec.ActionRefusalCapabilityUnavailable:
		readiness.Reason = "The target agent no longer exposes the required operation."
	case agentexec.ActionRefusalTargetInspectionUnavailable:
		readiness.Reason = "The target agent could not inspect the resource safely."
	case agentexec.ActionRefusalTargetStateChanged:
		readiness.Reason = "The target state changed after this action was planned. Refresh the plan before approving it."
	case agentexec.ActionRefusalTargetPreconditionFailed:
		readiness.Reason = "The target no longer satisfies the operation's local preconditions. Refresh the plan before approving it."
	case agentexec.ActionRefusalPackageManagerBusy:
		readiness.Reason = "The host package manager is busy. Retry readiness after the current package operation finishes."
	case agentexec.ActionRefusalPackagePreflightFailed:
		readiness.Reason = "The target agent could not inspect package update feasibility safely."
	case agentexec.ActionRefusalPackageInventoryChanged:
		readiness.Reason = "The package inventory changed after this action was planned. Refresh the plan before approving it."
	case agentexec.ActionRefusalPackageManagerUnhealthy:
		readiness.Reason = "The host package manager needs recovery before updates can run safely."
	case agentexec.ActionRefusalCleanupPreflightFailed:
		readiness.Reason = "The target agent could not inspect package-cache cleanup feasibility safely."
	case agentexec.ActionRefusalCleanupInventoryChanged:
		readiness.Reason = "The package-cache inventory changed after this action was planned. Refresh the plan before approving it."
	case agentexec.ActionRefusalContractInvalid:
		readiness.Reason = "The exact operation contract is no longer valid. Refresh the plan before approving it."
	default:
		readiness.Reason = "The target agent refused the operation during its read-only feasibility check."
	}
	return readiness
}

func (e routedActionExecutor) executorForAction(ctx context.Context, req unified.ActionRequest) (ActionExecutor, error) {
	if e.resources == nil {
		return nil, fmt.Errorf("resource handler unavailable")
	}
	registry, err := e.resources.buildRegistry(GetOrgID(ctx))
	if err != nil {
		return nil, err
	}
	resource, ok := registry.Get(req.ResourceID)
	if !ok || resource == nil {
		return nil, fmt.Errorf("resource %q is no longer present", req.ResourceID)
	}
	capability, ok := resourceCapabilityByName(resource.Capabilities, req.CapabilityName)
	if !ok {
		return nil, fmt.Errorf("resource %q does not currently advertise %s capability", req.ResourceID, req.CapabilityName)
	}
	handler := strings.TrimSpace(capability.InternalHandler)
	if handler == "" {
		return nil, fmt.Errorf("resource %q capability %s has no executor handler", req.ResourceID, req.CapabilityName)
	}
	executor := e.byHandler[handler]
	if executor == nil {
		return nil, fmt.Errorf("resource %q capability %s is routed through unsupported handler %q", req.ResourceID, req.CapabilityName, handler)
	}
	return executor, nil
}

func resourceCapabilityByName(capabilities []unified.ResourceCapability, name string) (unified.ResourceCapability, bool) {
	name = strings.TrimSpace(name)
	for _, capability := range capabilities {
		if strings.TrimSpace(capability.Name) == name {
			return capability, true
		}
	}
	return unified.ResourceCapability{}, false
}
