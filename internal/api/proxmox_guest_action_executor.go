package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const (
	proxmoxVMLifecycleHandler = "proxmox.vm.lifecycle"
	proxmoxCTLifecycleHandler = "proxmox.ct.lifecycle"
)

type proxmoxGuestKind string

const (
	proxmoxGuestVM proxmoxGuestKind = "vm"
	proxmoxGuestCT proxmoxGuestKind = "ct"
)

type proxmoxGuestActionExecutor struct {
	resources *ResourceHandlers
	agents    actionAgentCommander
	observer  proxmoxGuestPostconditionObserver
}

type proxmoxGuestLifecycleSnapshot struct {
	Instance   string
	Node       string
	VMID       int
	Kind       proxmoxGuestKind
	Status     string
	Uptime     uint64
	ObservedAt time.Time
}

type proxmoxGuestPostconditionObservation struct {
	ObserverID  string
	TrustDomain string
	Method      string
	Snapshot    proxmoxGuestLifecycleSnapshot
	ReceivedAt  time.Time
}

type proxmoxGuestPostconditionObserver interface {
	ObserveProxmoxGuest(context.Context, string, string, string, int, proxmoxGuestKind) (proxmoxGuestPostconditionObservation, error)
}

func newProxmoxGuestActionExecutor(resources *ResourceHandlers, agents actionAgentCommander, observer proxmoxGuestPostconditionObserver) ActionExecutor {
	if resources == nil || agents == nil {
		return nil
	}
	return proxmoxGuestActionExecutor{resources: resources, agents: agents, observer: observer}
}

func (e proxmoxGuestActionExecutor) ActionHandlerNames() []string {
	return []string{proxmoxVMLifecycleHandler, proxmoxCTLifecycleHandler}
}

func (e proxmoxGuestActionExecutor) ExecuteAction(ctx context.Context, record unified.ActionAuditRecord) (*unified.ExecutionResult, error) {
	attempt, ok := actionlifecycle.DispatchAttemptFromContext(ctx)
	if !ok || attempt.ActionID != record.ID {
		return nil, fmt.Errorf("committed action dispatch authority is required")
	}
	record, err := unified.NormalizeActionAuditRecord(record)
	if err != nil {
		return nil, err
	}
	operation := strings.TrimSpace(record.Request.CapabilityName)
	if !isProxmoxGuestLifecycleOperation(operation) {
		return nil, fmt.Errorf("unsupported proxmox guest lifecycle capability %q", operation)
	}

	resource, kind, err := e.currentProxmoxGuestResource(ctx, record.Request.ResourceID, operation)
	if err != nil {
		return nil, err
	}
	vmid := resource.Proxmox.VMID
	agentID, err := e.connectedProxmoxNodeCommandAgentID(ctx, resource)
	if err != nil {
		return nil, err
	}
	var independentBefore *proxmoxGuestPostconditionObservation
	if e.observer != nil {
		if observation, observeErr := e.observer.ObserveProxmoxGuest(ctx, record.Request.ResourceID, resource.Proxmox.Instance, resource.Proxmox.NodeName, vmid, kind); observeErr == nil && validProxmoxGuestObservation(resource, kind, observation) {
			independentBefore = &observation
		}
	}

	command := proxmoxGuestLifecycleCommand(kind, operation, vmid)
	actionStartedAt := time.Now().UTC()
	result, err := e.agents.ExecuteCommand(agentCommandContext(ctx), agentID, agentexec.ExecuteCommandPayload{
		RequestID:  attempt.ID,
		Command:    command,
		ApprovalID: record.ID,
		TargetType: "agent",
		Timeout:    proxmoxGuestLifecycleTimeout(operation),
		Trusted:    true,
	})
	if err != nil {
		return nil, err
	}

	output := redactActionOutput(commandOutput(result))
	if result.ExitCode != 0 {
		return proxmoxGuestExecutionResult(record.ID, record.Request.ResourceID, agentID, kind, operation, result.ExitCode, output, result.Error, nil, independentBefore, nil, agentexec.PostconditionEvaluation{}, actionStartedAt)
	}

	agentVerification := e.verifyProxmoxGuestState(ctx, agentID, record.ID, kind, vmid, operation, actionStartedAt)
	independentAfter, independentEvaluation := e.observeProxmoxGuestPostcondition(ctx, record.Request.ResourceID, resource, kind, operation, independentBefore, actionStartedAt)
	return proxmoxGuestExecutionResult(record.ID, record.Request.ResourceID, agentID, kind, operation, result.ExitCode, output, result.Error, agentVerification, independentBefore, independentAfter, independentEvaluation, actionStartedAt)
}

func (e proxmoxGuestActionExecutor) CheckActionAvailable(ctx context.Context, req unified.ActionRequest, resource unified.Resource) unified.ResourceActionReadiness {
	operation := strings.TrimSpace(req.CapabilityName)
	if _, ok := findProxmoxLifecycleCapability(resource.Capabilities, operation); !ok {
		return unified.ResourceActionReadiness{}
	}
	readiness := unified.ResourceActionReadiness{Name: operation, Available: true}
	if e.agents == nil {
		return unavailableProxmoxActionReadiness(operation, "command_agent_unavailable", "Proxmox command execution is not available.")
	}
	if _, _, err := e.executableProxmoxGuestResource(resource, operation); err != nil {
		return unavailableProxmoxActionReadiness(operation, proxmoxActionUnavailableReasonCode(err), proxmoxActionUnavailableReason(err))
	}
	if _, err := e.connectedProxmoxNodeCommandAgentID(ctx, resource); err != nil {
		return unavailableProxmoxActionReadiness(operation, "command_agent_disconnected", "Proxmox node command agent is not connected.")
	}
	return readiness
}

func (e proxmoxGuestActionExecutor) currentProxmoxGuestResource(ctx context.Context, resourceID, operation string) (unified.Resource, proxmoxGuestKind, error) {
	if e.resources == nil {
		return unified.Resource{}, "", fmt.Errorf("resource handler unavailable")
	}
	registry, err := e.resources.buildRegistry(GetOrgID(ctx))
	if err != nil {
		return unified.Resource{}, "", err
	}
	resource, ok := registry.Get(resourceID)
	if !ok || resource == nil {
		return unified.Resource{}, "", fmt.Errorf("resource %q is no longer present", resourceID)
	}
	return e.executableProxmoxGuestResource(*resource, operation)
}

func (e proxmoxGuestActionExecutor) executableProxmoxGuestResource(resource unified.Resource, operation string) (unified.Resource, proxmoxGuestKind, error) {
	kind, handler, err := proxmoxGuestKindAndHandler(resource)
	if err != nil {
		return unified.Resource{}, "", err
	}
	if resource.Proxmox == nil {
		return unified.Resource{}, "", fmt.Errorf("proxmox resource metadata missing")
	}
	if resource.Proxmox.VMID <= 0 {
		return unified.Resource{}, "", fmt.Errorf("proxmox guest resource %q has no executable vmid", resource.ID)
	}
	if strings.TrimSpace(resource.Proxmox.NodeName) == "" {
		return unified.Resource{}, "", fmt.Errorf("proxmox guest resource %q has no owning node", resource.ID)
	}
	if resource.Proxmox.Template {
		return unified.Resource{}, "", fmt.Errorf("proxmox guest resource %q is a template", resource.ID)
	}
	if lock := strings.TrimSpace(resource.Proxmox.Lock); lock != "" {
		return unified.Resource{}, "", fmt.Errorf("proxmox guest resource %q is locked: %s", resource.ID, lock)
	}
	if status := resource.SourceStatus[unified.SourceProxmox]; strings.EqualFold(strings.TrimSpace(status.Status), "stale") || strings.EqualFold(strings.TrimSpace(status.Status), "offline") || strings.EqualFold(strings.TrimSpace(status.Status), "missing") {
		return unified.Resource{}, "", fmt.Errorf("proxmox inventory for resource %q is %s", resource.ID, status.Status)
	}
	capability, ok := findProxmoxLifecycleCapability(resource.Capabilities, operation)
	if !ok {
		return unified.Resource{}, "", fmt.Errorf("resource %q does not currently advertise %s capability", resource.ID, operation)
	}
	if capability.InternalHandler != handler {
		return unified.Resource{}, "", fmt.Errorf("resource %q advertises %s through unsupported handler %q", resource.ID, operation, capability.InternalHandler)
	}
	return resource, kind, nil
}

func (e proxmoxGuestActionExecutor) connectedProxmoxNodeCommandAgentID(ctx context.Context, resource unified.Resource) (string, error) {
	if e.agents == nil {
		return "", fmt.Errorf("proxmox node command agent is not connected")
	}
	if resource.Proxmox == nil {
		return "", fmt.Errorf("proxmox resource metadata missing")
	}
	if agentID := strings.TrimSpace(resource.Proxmox.LinkedAgentID); agentID != "" && isAgentCommandConnected(ctx, e.agents, agentID) {
		return agentID, nil
	}
	if agentID, ok := commandAgentForHost(ctx, e.agents, strings.TrimSpace(resource.Proxmox.NodeName)); ok {
		agentID = strings.TrimSpace(agentID)
		if agentID != "" && isAgentCommandConnected(ctx, e.agents, agentID) {
			return agentID, nil
		}
	}
	return "", fmt.Errorf("proxmox node command agent is not connected")
}

func proxmoxGuestKindAndHandler(resource unified.Resource) (proxmoxGuestKind, string, error) {
	switch resource.Type {
	case unified.ResourceTypeVM:
		return proxmoxGuestVM, proxmoxVMLifecycleHandler, nil
	case unified.ResourceTypeSystemContainer:
		return proxmoxGuestCT, proxmoxCTLifecycleHandler, nil
	default:
		return "", "", fmt.Errorf("resource %q is not a Proxmox VM or LXC", resource.ID)
	}
}

func unavailableProxmoxActionReadiness(operation, reasonCode, reason string) unified.ResourceActionReadiness {
	return unified.ResourceActionReadiness{
		Name:       strings.TrimSpace(operation),
		Available:  false,
		ReasonCode: strings.TrimSpace(reasonCode),
		Reason:     strings.TrimSpace(reason),
	}
}

func proxmoxActionUnavailableReasonCode(err error) string {
	if err == nil {
		return "unavailable"
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "not a proxmox vm or lxc"):
		return "unsupported_resource"
	case strings.Contains(message, "inventory") && (strings.Contains(message, "stale") || strings.Contains(message, "offline") || strings.Contains(message, "missing")):
		return "stale_inventory"
	case strings.Contains(message, "template"):
		return "template"
	case strings.Contains(message, "locked"):
		return "guest_locked"
	case strings.Contains(message, "does not currently advertise"):
		return "capability_unavailable"
	case strings.Contains(message, "unsupported handler"):
		return "unsupported_handler"
	case strings.Contains(message, "vmid") || strings.Contains(message, "owning node"):
		return "incomplete_metadata"
	default:
		return "unavailable"
	}
}

func proxmoxActionUnavailableReason(err error) string {
	switch proxmoxActionUnavailableReasonCode(err) {
	case "unsupported_resource":
		return "Resource is not a Proxmox VM or LXC."
	case "stale_inventory":
		return "Proxmox inventory is not fresh enough to run lifecycle actions."
	case "template":
		return "Proxmox templates cannot run lifecycle actions."
	case "guest_locked":
		return "Proxmox reports this guest is locked."
	case "capability_unavailable":
		return "Pulse does not currently advertise a fresh Proxmox lifecycle capability for this guest."
	case "unsupported_handler":
		return "This Proxmox action is not routed through the supported lifecycle executor."
	case "incomplete_metadata":
		return "Proxmox guest metadata is missing the VMID or owning node needed for command execution."
	default:
		return "Proxmox lifecycle action is not currently available."
	}
}

func findProxmoxLifecycleCapability(capabilities []unified.ResourceCapability, operation string) (unified.ResourceCapability, bool) {
	operation = strings.TrimSpace(operation)
	for _, capability := range capabilities {
		if strings.TrimSpace(capability.Name) == operation && isProxmoxLifecycleHandler(capability.InternalHandler) {
			return capability, true
		}
	}
	return unified.ResourceCapability{}, false
}

func isProxmoxLifecycleHandler(handler string) bool {
	switch strings.TrimSpace(handler) {
	case proxmoxVMLifecycleHandler, proxmoxCTLifecycleHandler:
		return true
	default:
		return false
	}
}

func isProxmoxGuestLifecycleOperation(operation string) bool {
	switch strings.TrimSpace(operation) {
	case "start", "shutdown", "reboot", "stop":
		return true
	default:
		return false
	}
}

func proxmoxGuestLifecycleCommand(kind proxmoxGuestKind, operation string, vmid int) string {
	tool := "qm"
	if kind == proxmoxGuestCT {
		tool = "pct"
	}
	return strings.Join([]string{tool, strings.TrimSpace(operation), strconv.Itoa(vmid)}, " ")
}

func proxmoxGuestStatusCommand(kind proxmoxGuestKind, vmid int) string {
	tool := "qm"
	if kind == proxmoxGuestCT {
		tool = "pct"
	}
	return strings.Join([]string{tool, "status", strconv.Itoa(vmid)}, " ")
}

func proxmoxGuestLifecycleTimeout(operation string) int {
	switch strings.TrimSpace(operation) {
	case "shutdown", "reboot":
		return 180
	default:
		return 120
	}
}

func (e proxmoxGuestActionExecutor) verifyProxmoxGuestState(ctx context.Context, agentID, actionID string, kind proxmoxGuestKind, vmid int, operation string, actionStartedAt time.Time) *unified.ActionVerificationResult {
	capability := proxmoxPostconditionCapability(kind, operation)
	if _, ok := agentexec.LookupCapabilityPostcondition(capability); !ok {
		return &unified.ActionVerificationResult{Ran: false, Note: "No registered postcondition is available for this Proxmox action."}
	}
	if strings.EqualFold(strings.TrimSpace(operation), "reboot") {
		return &unified.ActionVerificationResult{Ran: false, Note: "The executing agent's status-only read cannot prove that the guest restarted."}
	}
	command := proxmoxGuestStatusCommand(kind, vmid)

	var lastOutput string
	var lastEvaluation agentexec.PostconditionEvaluation
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(1 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return &unified.ActionVerificationResult{Ran: false}
			case <-timer.C:
			}
		}

		result, err := e.agents.ExecuteCommand(agentCommandContext(ctx), agentID, agentexec.ExecuteCommandPayload{
			RequestID:  fmt.Sprintf("%s-verify-%d", actionID, attempt+1),
			Command:    command,
			ApprovalID: actionID,
			TargetType: "agent",
			Timeout:    30,
			Trusted:    true,
		})
		if err != nil {
			return &unified.ActionVerificationResult{Ran: false}
		}
		lastOutput = redactActionOutput(commandOutput(result))
		if result.ExitCode != 0 {
			continue
		}
		lastEvaluation, _ = agentexec.EvaluateCapabilityPostcondition(capability, nil, proxmoxGuestPostconditionValues(kind, parseProxmoxGuestStatus(lastOutput), 0), actionStartedAt)
		if lastEvaluation.Conclusive && lastEvaluation.Matched {
			return &unified.ActionVerificationResult{
				Ran:     true,
				Command: command,
				Output:  lastOutput,
				Success: true,
				RanAt:   time.Now().UTC(),
			}
		}
	}

	return &unified.ActionVerificationResult{
		Ran:     true,
		Command: command,
		Output:  lastOutput,
		Success: false,
		RanAt:   time.Now().UTC(),
		Note:    firstNonEmpty(lastEvaluation.ReasonCode, "postcondition was not confirmed"),
	}
}

func (e proxmoxGuestActionExecutor) observeProxmoxGuestPostcondition(ctx context.Context, resourceID string, resource unified.Resource, kind proxmoxGuestKind, operation string, before *proxmoxGuestPostconditionObservation, actionStartedAt time.Time) (*proxmoxGuestPostconditionObservation, agentexec.PostconditionEvaluation) {
	if e.observer == nil || resource.Proxmox == nil {
		return nil, agentexec.PostconditionEvaluation{}
	}
	capability := proxmoxPostconditionCapability(kind, operation)
	postcondition, ok := agentexec.LookupCapabilityPostcondition(capability)
	if !ok {
		return nil, agentexec.PostconditionEvaluation{ReasonCode: "postcondition_unregistered"}
	}
	window := postcondition.Window
	if window <= 0 {
		window = 2 * time.Minute
	}
	verifyCtx, cancel := context.WithTimeout(ctx, window)
	defer cancel()

	var beforeValues map[agentexec.PostconditionField]string
	if before != nil {
		beforeValues = proxmoxGuestPostconditionValues(kind, before.Snapshot.Status, before.Snapshot.Uptime)
	}
	var last *proxmoxGuestPostconditionObservation
	lastEvaluation := agentexec.PostconditionEvaluation{ReasonCode: "independent_observation_unavailable"}
	for {
		observation, err := e.observer.ObserveProxmoxGuest(verifyCtx, resourceID, resource.Proxmox.Instance, resource.Proxmox.NodeName, resource.Proxmox.VMID, kind)
		if err == nil && validProxmoxGuestObservation(resource, kind, observation) {
			last = &observation
			lastEvaluation, _ = agentexec.EvaluateCapabilityPostcondition(capability, beforeValues, proxmoxGuestPostconditionValues(kind, observation.Snapshot.Status, observation.Snapshot.Uptime), actionStartedAt)
			if lastEvaluation.Conclusive && lastEvaluation.Matched {
				return last, lastEvaluation
			}
		}

		timer := time.NewTimer(time.Second)
		select {
		case <-verifyCtx.Done():
			timer.Stop()
			return last, lastEvaluation
		case <-timer.C:
		}
	}
}

func proxmoxPostconditionCapability(kind proxmoxGuestKind, operation string) string {
	tool := "qm"
	if kind == proxmoxGuestCT {
		tool = "pct"
	}
	return tool + "." + strings.ToLower(strings.TrimSpace(operation))
}

func proxmoxGuestPostconditionValues(kind proxmoxGuestKind, status string, uptime uint64) map[agentexec.PostconditionField]string {
	statusField := agentexec.FieldVMStatus
	if kind == proxmoxGuestCT {
		statusField = agentexec.FieldContainerStatus
	}
	return map[agentexec.PostconditionField]string{
		statusField:                strings.ToLower(strings.TrimSpace(status)),
		agentexec.FieldGuestUptime: strconv.FormatUint(uptime, 10),
	}
}

func validProxmoxGuestObservation(resource unified.Resource, kind proxmoxGuestKind, observation proxmoxGuestPostconditionObservation) bool {
	if resource.Proxmox == nil || observation.Snapshot.VMID != resource.Proxmox.VMID || observation.Snapshot.Kind != kind {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(observation.Snapshot.Instance), strings.TrimSpace(resource.Proxmox.Instance)) &&
		strings.EqualFold(strings.TrimSpace(observation.Snapshot.Node), strings.TrimSpace(resource.Proxmox.NodeName)) &&
		strings.TrimSpace(observation.Snapshot.Status) != "" && !observation.Snapshot.ObservedAt.IsZero() && !observation.ReceivedAt.IsZero()
}

func parseProxmoxGuestStatus(output string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(output)))
	for i, field := range fields {
		field = strings.TrimSuffix(field, ":")
		if field == "status" && i+1 < len(fields) {
			return strings.Trim(strings.TrimSuffix(fields[i+1], ":"), " \t\r\n")
		}
		if field == "running" || field == "stopped" {
			return field
		}
	}
	return ""
}
