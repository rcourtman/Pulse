package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/safety"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const (
	dockerContainerLifecycleHandler = "docker.container.lifecycle"
	dockerContainerUpdateHandler    = "docker.container.update"
)

type dockerContainerActionExecutor struct {
	resources *ResourceHandlers
	agents    actionAgentCommander
	observer  dockerContainerPostconditionObserver
}

type dockerContainerPostconditionObservation struct {
	ObserverID  string
	TrustDomain string
	Method      string
	Snapshot    agentexec.DockerContainerLifecycleSnapshot
	ReceivedAt  time.Time
}

type dockerContainerPostconditionObserver interface {
	ObserveDockerContainer(context.Context, string, string) (dockerContainerPostconditionObservation, error)
}

type dockerContainerLifecycleAgentCommander interface {
	ExecuteDockerContainerLifecycle(context.Context, string, agentexec.DockerContainerLifecyclePayload) (*agentexec.DockerContainerLifecycleResultPayload, error)
}

type dockerContainerUpdateAgentCommander interface {
	ExecuteDockerContainerUpdate(context.Context, string, agentexec.DockerContainerUpdatePayload) (*agentexec.DockerContainerUpdateResultPayload, error)
}

func isDockerContainerUpdateOperation(operation string) bool {
	return strings.TrimSpace(operation) == "update"
}

// dockerContainerOperationHandler returns the internal handler a capability
// must advertise for the requested operation, so an update request can never
// ride a lifecycle capability or vice versa.
func dockerContainerOperationHandler(operation string) string {
	if isDockerContainerUpdateOperation(operation) {
		return dockerContainerUpdateHandler
	}
	return dockerContainerLifecycleHandler
}

func (e dockerContainerActionExecutor) BindActionDispatch(ctx context.Context, record unified.ActionAuditRecord, attempt unified.ActionDispatchAttempt) (unified.ActionDispatchAttempt, error) {
	if isDockerContainerUpdateOperation(record.Request.CapabilityName) {
		resource, err := e.currentDockerContainerResource(ctx, record.Request.ResourceID, record.Request.CapabilityName)
		if err != nil {
			return unified.ActionDispatchAttempt{}, err
		}
		runtime, err := dockerContainerRuntime(resource)
		if err != nil {
			return unified.ActionDispatchAttempt{}, err
		}
		agentID, err := e.connectedDockerCommandAgentID(resource)
		if err != nil {
			return unified.ActionDispatchAttempt{}, err
		}
		req, err := dockerContainerUpdateRequest(attempt.ID, record.ID, runtime, resource)
		if err != nil {
			return unified.ActionDispatchAttempt{}, err
		}
		return unified.BindActionDispatchAttempt(attempt, unified.ActionDispatchBinding{OperationKind: req.Operation, OperationVersion: req.OperationVersion, RequestDigest: req.RequestDigest, AgentID: agentID})
	}
	operation, err := dockerAgentLifecycleOperation(record.Request.CapabilityName)
	if err != nil {
		return unified.ActionDispatchAttempt{}, err
	}
	resource, err := e.currentDockerContainerResource(ctx, record.Request.ResourceID, record.Request.CapabilityName)
	if err != nil {
		return unified.ActionDispatchAttempt{}, err
	}
	runtime, err := dockerContainerRuntime(resource)
	if err != nil {
		return unified.ActionDispatchAttempt{}, err
	}
	agentID, err := e.connectedDockerCommandAgentID(resource)
	if err != nil {
		return unified.ActionDispatchAttempt{}, err
	}
	req := dockerContainerLifecycleRequest(attempt.ID, record.ID, operation, runtime, resource)
	if err := agentexec.BindDockerContainerLifecyclePayload(&req); err != nil {
		return unified.ActionDispatchAttempt{}, err
	}
	return unified.BindActionDispatchAttempt(attempt, unified.ActionDispatchBinding{OperationKind: req.Operation, OperationVersion: req.OperationVersion, RequestDigest: req.RequestDigest, AgentID: agentID})
}

func newDockerContainerActionExecutor(resources *ResourceHandlers, agents actionAgentCommander) ActionExecutor {
	if resources == nil || agents == nil {
		return nil
	}
	return dockerContainerActionExecutor{resources: resources, agents: agents}
}

func (e dockerContainerActionExecutor) ActionHandlerNames() []string {
	return []string{dockerContainerLifecycleHandler, dockerContainerUpdateHandler}
}

func (e dockerContainerActionExecutor) ActionDispatchOperationKinds() []string {
	return []string{
		agentexec.DockerContainerOperationStart,
		agentexec.DockerContainerOperationStop,
		agentexec.DockerContainerOperationRestart,
		agentexec.DockerContainerOperationUpdate,
	}
}

func (e dockerContainerActionExecutor) ExecuteAction(ctx context.Context, record unified.ActionAuditRecord) (*unified.ExecutionResult, error) {
	attempt, ok := actionlifecycle.DispatchAttemptFromContext(ctx)
	if !ok || attempt.ActionID != record.ID {
		return nil, fmt.Errorf("committed action dispatch authority is required")
	}
	record, err := unified.NormalizeActionAuditRecord(record)
	if err != nil {
		return nil, err
	}
	operation := strings.TrimSpace(record.Request.CapabilityName)
	if isDockerContainerUpdateOperation(operation) {
		return e.executeDockerContainerUpdate(ctx, record, attempt)
	}
	if !isDockerContainerLifecycleOperation(operation) {
		return nil, fmt.Errorf("unsupported docker container lifecycle capability %q", operation)
	}

	resource, err := e.currentDockerContainerResource(ctx, record.Request.ResourceID, operation)
	if err != nil {
		return nil, err
	}
	runtime, err := dockerContainerRuntime(resource)
	if err != nil {
		return nil, err
	}
	if dockerContainerRef(resource) == "" {
		return nil, fmt.Errorf("docker container resource %q has no executable container id", record.Request.ResourceID)
	}
	agentID, err := e.connectedDockerCommandAgentID(resource)
	if err != nil {
		return nil, err
	}

	typedAgents, ok := e.agents.(dockerContainerLifecycleAgentCommander)
	if !ok {
		return nil, fmt.Errorf("typed docker container lifecycle agent is unavailable")
	}
	agentOperation, err := dockerAgentLifecycleOperation(operation)
	if err != nil {
		return nil, err
	}
	req := dockerContainerLifecycleRequest(attempt.ID, record.ID, agentOperation, runtime, resource)
	if err := agentexec.BindDockerContainerLifecyclePayload(&req); err != nil {
		return nil, err
	}
	if agentexec.DockerContainerLifecycleOperationIdentity(agentID, req) != (operationreceipt.Identity{AttemptID: attempt.ID, ActionID: attempt.ActionID, OperationKind: attempt.OperationKind, OperationVersion: attempt.OperationVersion, RequestDigest: attempt.RequestDigest, AgentID: attempt.AgentID}) {
		return nil, fmt.Errorf("docker container lifecycle dispatch binding drift")
	}
	result, err := typedAgents.ExecuteDockerContainerLifecycle(ctx, agentID, req)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("docker container lifecycle agent returned no result")
	}
	receivedAt := time.Now().UTC()
	if err := agentexec.ValidateDockerContainerLifecycleResultForRequest(req, *result); err != nil {
		return nil, fmt.Errorf("invalid docker container lifecycle agent result: %w", err)
	}
	var independent *dockerContainerPostconditionObservation
	if e.observer != nil {
		if observation, observeErr := e.observer.ObserveDockerContainer(ctx, record.ID, req.ContainerID); observeErr == nil {
			independent = &observation
		}
	}
	return dockerContainerExecutionResult(record.Request.ResourceID, agentID, req, *result, independent, receivedAt)
}

func (e dockerContainerActionExecutor) executeDockerContainerUpdate(ctx context.Context, record unified.ActionAuditRecord, attempt unified.ActionDispatchAttempt) (*unified.ExecutionResult, error) {
	operation := strings.TrimSpace(record.Request.CapabilityName)
	resource, err := e.currentDockerContainerResource(ctx, record.Request.ResourceID, operation)
	if err != nil {
		return nil, err
	}
	runtime, err := dockerContainerRuntime(resource)
	if err != nil {
		return nil, err
	}
	if dockerContainerRef(resource) == "" {
		return nil, fmt.Errorf("docker container resource %q has no executable container id", record.Request.ResourceID)
	}
	agentID, err := e.connectedDockerCommandAgentID(resource)
	if err != nil {
		return nil, err
	}
	typedAgents, ok := e.agents.(dockerContainerUpdateAgentCommander)
	if !ok {
		return nil, fmt.Errorf("typed docker container update agent is unavailable")
	}
	req, err := dockerContainerUpdateRequest(attempt.ID, record.ID, runtime, resource)
	if err != nil {
		return nil, err
	}
	if agentexec.DockerContainerUpdateOperationIdentity(agentID, req) != (operationreceipt.Identity{AttemptID: attempt.ID, ActionID: attempt.ActionID, OperationKind: attempt.OperationKind, OperationVersion: attempt.OperationVersion, RequestDigest: attempt.RequestDigest, AgentID: attempt.AgentID}) {
		return nil, fmt.Errorf("docker container update dispatch binding drift")
	}
	result, err := typedAgents.ExecuteDockerContainerUpdate(ctx, agentID, req)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("docker container update agent returned no result")
	}
	receivedAt := time.Now().UTC()
	if err := agentexec.ValidateDockerContainerUpdateResultForRequest(req, *result); err != nil {
		return nil, fmt.Errorf("invalid docker container update agent result: %w", err)
	}
	var independent *dockerContainerPostconditionObservation
	if e.observer != nil && result.NewContainerID != "" {
		if observation, observeErr := e.observer.ObserveDockerContainer(ctx, record.ID, result.NewContainerID); observeErr == nil {
			independent = &observation
		}
	}
	return dockerContainerUpdateExecutionResult(record.Request.ResourceID, agentID, *result, independent, receivedAt)
}

func dockerContainerUpdateRequest(attemptID, actionID, runtime string, resource unified.Resource) (agentexec.DockerContainerUpdatePayload, error) {
	expectedDigest := ""
	if resource.Docker != nil && resource.Docker.UpdateStatus != nil {
		expectedDigest = strings.ToLower(strings.TrimSpace(resource.Docker.UpdateStatus.CurrentDigest))
	}
	if expectedDigest == "" {
		return agentexec.DockerContainerUpdatePayload{}, fmt.Errorf("resource %q has no current image digest to bind the update against", resource.ID)
	}
	req := agentexec.DockerContainerUpdatePayload{
		RequestID: attemptID, ActionID: actionID, Runtime: runtime,
		ContainerID: dockerContainerRef(resource), ExpectedImageDigest: expectedDigest,
	}
	if err := agentexec.BindDockerContainerUpdatePayload(&req); err != nil {
		return agentexec.DockerContainerUpdatePayload{}, err
	}
	return req, nil
}

func (e dockerContainerActionExecutor) ReconcileActionDispatch(ctx context.Context, record unified.ActionAuditRecord, attempt unified.ActionDispatchAttempt) (*unified.ExecutionResult, unified.ActionDispatchReceipt, bool, error) {
	querier, ok := e.agents.(operationReceiptQuerier)
	if !ok {
		return nil, unified.ActionDispatchReceipt{}, false, nil
	}
	identity := operationreceipt.Identity{AttemptID: attempt.ID, ActionID: attempt.ActionID, OperationKind: attempt.OperationKind, OperationVersion: attempt.OperationVersion, RequestDigest: attempt.RequestDigest, AgentID: attempt.AgentID}
	query, err := querier.QueryAgentOperation(ctx, attempt.AgentID, identity)
	if err != nil {
		return nil, unified.ActionDispatchReceipt{}, false, err
	}
	if query.Status != operationreceipt.QueryFoundTerminal {
		return nil, unified.ActionDispatchReceipt{}, false, nil
	}
	receivedAt := time.Now().UTC()
	if err := agentexec.ValidateOperationQueryResultForIdentity(query, identity, receivedAt); err != nil {
		return nil, unified.ActionDispatchReceipt{}, false, err
	}
	if attempt.OperationKind == agentexec.DockerContainerOperationUpdate {
		result, err := agentexec.DecodeDockerContainerUpdateResultPayload(query.Record.Result)
		if err != nil {
			return nil, unified.ActionDispatchReceipt{}, false, err
		}
		var independent *dockerContainerPostconditionObservation
		if e.observer != nil && result.NewContainerID != "" {
			if observation, observeErr := e.observer.ObserveDockerContainer(ctx, record.ID, result.NewContainerID); observeErr == nil {
				independent = &observation
			}
		}
		execution, err := dockerContainerUpdateExecutionResult(record.Request.ResourceID, attempt.AgentID, result, independent, receivedAt)
		if err != nil {
			return nil, unified.ActionDispatchReceipt{}, false, err
		}
		receipt := unified.ActionDispatchReceipt{AttemptID: attempt.ID, ActionID: record.ID, TransportRequestID: attempt.ID, ReceivedAt: receivedAt}
		return execution, receipt, true, nil
	}
	result, err := agentexec.DecodeDockerContainerLifecycleResultPayload(query.Record.Result)
	if err != nil {
		return nil, unified.ActionDispatchReceipt{}, false, err
	}
	req := agentexec.DockerContainerLifecyclePayload{RequestID: attempt.ID, ActionID: record.ID, Operation: attempt.OperationKind, OperationVersion: attempt.OperationVersion, RequestDigest: attempt.RequestDigest, Runtime: "docker", ContainerID: result.ContainerID, ExpectedState: result.Before.State, ExpectedStartedAt: result.Before.StartedAt}
	var independent *dockerContainerPostconditionObservation
	if e.observer != nil {
		if observation, observeErr := e.observer.ObserveDockerContainer(ctx, record.ID, result.ContainerID); observeErr == nil {
			independent = &observation
		}
	}
	execution, err := dockerContainerExecutionResult(record.Request.ResourceID, attempt.AgentID, req, result, independent, receivedAt)
	if err != nil {
		return nil, unified.ActionDispatchReceipt{}, false, err
	}
	receipt := unified.ActionDispatchReceipt{AttemptID: attempt.ID, ActionID: record.ID, TransportRequestID: attempt.ID, ReceivedAt: receivedAt}
	return execution, receipt, true, nil
}

func (e dockerContainerActionExecutor) CheckActionAvailable(ctx context.Context, req unified.ActionRequest, resource unified.Resource) unified.ResourceActionReadiness {
	operation := strings.TrimSpace(req.CapabilityName)
	capability, ok := findDockerLifecycleCapability(resource.Capabilities, operation)
	if !ok || capability.InternalHandler != dockerContainerOperationHandler(operation) {
		return unified.ResourceActionReadiness{}
	}
	readiness := unified.ResourceActionReadiness{Name: operation, Available: true}
	if e.agents == nil {
		return unavailableDockerActionReadiness(operation, "command_agent_unavailable", "Docker / Podman command execution is not available.")
	}
	if isDockerContainerUpdateOperation(operation) {
		if _, ok := e.agents.(dockerContainerUpdateAgentCommander); !ok {
			return unavailableDockerActionReadiness(operation, "typed_operation_unavailable", "Typed Docker / Podman update execution is not available.")
		}
	} else if _, ok := e.agents.(dockerContainerLifecycleAgentCommander); !ok {
		return unavailableDockerActionReadiness(operation, "typed_operation_unavailable", "Typed Docker / Podman lifecycle execution is not available.")
	}
	if _, err := e.executableDockerContainerResource(ctx, resource, operation); err != nil {
		return unavailableDockerActionReadiness(operation, dockerActionUnavailableReasonCode(err), dockerActionUnavailableReason(err))
	}
	if _, err := e.connectedDockerCommandAgentID(resource); err != nil {
		return unavailableDockerActionReadiness(operation, "command_agent_disconnected", "Docker / Podman command agent is not connected.")
	}
	agentID, _ := e.connectedDockerCommandAgentID(resource)
	liveCapability, supported := e.agents.(agentOperationReceiptCapability)
	if !supported || liveCapability.AgentOperationReceiptVersion(agentID) != operationreceipt.ProtocolVersion {
		return unavailableDockerActionReadiness(operation, "operation_receipt_unsupported", "The Pulse agent on this host cannot run reviewed actions: it is on an older version, or its durable state directory is unavailable. Update the agent, or check the agent logs if it is already current, then retry.")
	}
	return readiness
}

func (e dockerContainerActionExecutor) currentDockerContainerResource(ctx context.Context, resourceID, operation string) (unified.Resource, error) {
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
	return e.executableDockerContainerResource(ctx, *resource, operation)
}

func (e dockerContainerActionExecutor) executableDockerContainerResource(_ context.Context, resource unified.Resource, operation string) (unified.Resource, error) {
	if resource.Type != unified.ResourceTypeAppContainer || resource.Docker == nil {
		return unified.Resource{}, fmt.Errorf("resource %q is not a Docker or Podman container", resource.ID)
	}
	if status := resource.SourceStatus[unified.SourceDocker]; strings.EqualFold(strings.TrimSpace(status.Status), "stale") || strings.EqualFold(strings.TrimSpace(status.Status), "offline") || strings.EqualFold(strings.TrimSpace(status.Status), "missing") {
		return unified.Resource{}, fmt.Errorf("docker inventory for resource %q is %s", resource.ID, status.Status)
	}
	if resource.Docker.Security != nil && resource.Docker.Security.MutatingCommandsBlocked {
		reason := strings.TrimSpace(resource.Docker.Security.MutatingCommandsBlockedReason)
		if reason == "" {
			reason = "Docker daemon-mutating commands are disabled for this host"
		}
		return unified.Resource{}, fmt.Errorf("docker container lifecycle action is blocked by host policy: %s", reason)
	}
	if capability, ok := findDockerLifecycleCapability(resource.Capabilities, operation); !ok {
		return unified.Resource{}, fmt.Errorf("resource %q does not currently advertise %s capability", resource.ID, operation)
	} else if capability.InternalHandler != dockerContainerOperationHandler(operation) {
		return unified.Resource{}, fmt.Errorf("resource %q advertises %s through unsupported handler %q", resource.ID, operation, capability.InternalHandler)
	}
	return resource, nil
}

func (e dockerContainerActionExecutor) connectedDockerCommandAgentID(resource unified.Resource) (string, error) {
	if e.agents == nil {
		return "", fmt.Errorf("docker container command agent is not connected")
	}
	if resource.Docker == nil {
		return "", fmt.Errorf("docker resource metadata missing")
	}
	if agentID := strings.TrimSpace(resource.Docker.AgentID); agentID != "" && e.agents.IsAgentConnected(agentID) {
		return agentID, nil
	}
	if agentID, ok := e.agents.GetAgentForHost(strings.TrimSpace(resource.Docker.Hostname)); ok {
		agentID = strings.TrimSpace(agentID)
		if agentID != "" && e.agents.IsAgentConnected(agentID) {
			return agentID, nil
		}
	}
	return "", fmt.Errorf("docker container command agent is not connected")
}

func unavailableDockerActionReadiness(operation, reasonCode, reason string) unified.ResourceActionReadiness {
	return unified.ResourceActionReadiness{
		Name:       strings.TrimSpace(operation),
		Available:  false,
		ReasonCode: strings.TrimSpace(reasonCode),
		Reason:     strings.TrimSpace(reason),
	}
}

func dockerActionUnavailableReasonCode(err error) string {
	if err == nil {
		return "unavailable"
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "not a docker or podman container"):
		return "unsupported_resource"
	case strings.Contains(message, "inventory") && (strings.Contains(message, "stale") || strings.Contains(message, "offline") || strings.Contains(message, "missing")):
		return "stale_inventory"
	case strings.Contains(message, "blocked by host policy"):
		return "host_policy_blocked"
	case strings.Contains(message, "does not currently advertise"):
		return "capability_unavailable"
	case strings.Contains(message, "unsupported handler"):
		return "unsupported_handler"
	default:
		return "unavailable"
	}
}

func dockerActionUnavailableReason(err error) string {
	switch dockerActionUnavailableReasonCode(err) {
	case "unsupported_resource":
		return "Resource is not a Docker or Podman container."
	case "stale_inventory":
		return "Docker / Podman inventory is not fresh enough to run lifecycle actions."
	case "host_policy_blocked":
		return "Docker / Podman host policy blocks mutating lifecycle actions."
	case "capability_unavailable":
		return "Pulse does not currently advertise a fresh command capability for this container."
	case "unsupported_handler":
		return "This container action is not routed through the supported lifecycle executor."
	default:
		return "Docker / Podman lifecycle action is not currently available."
	}
}

func findDockerLifecycleCapability(capabilities []unified.ResourceCapability, operation string) (unified.ResourceCapability, bool) {
	operation = strings.TrimSpace(operation)
	for _, capability := range capabilities {
		if strings.TrimSpace(capability.Name) == operation {
			return capability, true
		}
	}
	return unified.ResourceCapability{}, false
}

func dockerContainerRuntime(resource unified.Resource) (string, error) {
	if resource.Docker == nil {
		return "", fmt.Errorf("docker resource metadata missing")
	}
	switch strings.ToLower(strings.TrimSpace(firstNonEmpty(resource.Docker.Runtime, resource.Technology))) {
	case "docker":
		return "docker", nil
	case "podman":
		return "podman", nil
	default:
		return "", fmt.Errorf("unsupported container runtime %q", firstNonEmpty(resource.Docker.Runtime, resource.Technology))
	}
}

func dockerContainerRef(resource unified.Resource) string {
	if resource.Docker == nil {
		return ""
	}
	if ref := strings.TrimSpace(resource.Docker.ContainerID); ref != "" {
		return ref
	}
	return ""
}

func dockerAgentLifecycleOperation(operation string) (string, error) {
	switch strings.TrimSpace(operation) {
	case "start", agentexec.DockerContainerOperationStart:
		return agentexec.DockerContainerOperationStart, nil
	case "stop", agentexec.DockerContainerOperationStop:
		return agentexec.DockerContainerOperationStop, nil
	case "restart", agentexec.DockerContainerOperationRestart:
		return agentexec.DockerContainerOperationRestart, nil
	default:
		return "", fmt.Errorf("unsupported docker container lifecycle capability %q", operation)
	}
}

func dockerContainerLifecycleRequest(attemptID, actionID, operation, runtime string, resource unified.Resource) agentexec.DockerContainerLifecyclePayload {
	startedAt := time.Time{}
	if resource.Docker != nil && resource.Docker.StartedAt != nil {
		startedAt = resource.Docker.StartedAt.UTC()
	}
	return agentexec.DockerContainerLifecyclePayload{
		RequestID: attemptID, ActionID: actionID, Operation: operation, Runtime: runtime,
		ContainerID: dockerContainerRef(resource), ExpectedState: strings.ToLower(strings.TrimSpace(resource.Docker.ContainerState)), ExpectedStartedAt: startedAt, Timeout: 120,
	}
}

func isDockerContainerLifecycleOperation(operation string) bool {
	switch strings.TrimSpace(operation) {
	case "start", "stop", "restart":
		return true
	default:
		return false
	}
}

func commandOutput(result *agentexec.CommandResultPayload) string {
	if result == nil {
		return ""
	}
	parts := []string{}
	if stdout := strings.TrimSpace(result.Stdout); stdout != "" {
		parts = append(parts, stdout)
	}
	if stderr := strings.TrimSpace(result.Stderr); stderr != "" {
		parts = append(parts, stderr)
	}
	if errText := strings.TrimSpace(result.Error); errText != "" {
		parts = append(parts, errText)
	}
	return strings.Join(parts, "\n")
}

func redactActionOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	redacted, n := safety.RedactSensitiveText(output)
	if n > 0 {
		return strings.TrimSpace(redacted) + fmt.Sprintf("\n\n[redacted %d sensitive value(s)]", n)
	}
	return output
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
