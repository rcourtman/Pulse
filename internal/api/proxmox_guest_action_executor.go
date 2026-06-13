package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

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
}

func newProxmoxGuestActionExecutor(resources *ResourceHandlers, agents actionAgentCommander) ActionExecutor {
	if resources == nil || agents == nil {
		return nil
	}
	return proxmoxGuestActionExecutor{resources: resources, agents: agents}
}

func (e proxmoxGuestActionExecutor) ActionHandlerNames() []string {
	return []string{proxmoxVMLifecycleHandler, proxmoxCTLifecycleHandler}
}

func (e proxmoxGuestActionExecutor) ExecuteAction(ctx context.Context, record unified.ActionAuditRecord) (*unified.ExecutionResult, error) {
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
	agentID, err := e.connectedProxmoxNodeCommandAgentID(resource)
	if err != nil {
		return nil, err
	}

	command := proxmoxGuestLifecycleCommand(kind, operation, vmid)
	result, err := e.agents.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
		RequestID:  record.ID,
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
	execution := &unified.ExecutionResult{
		Success: result.ExitCode == 0,
		Output:  output,
	}
	if result.ExitCode != 0 {
		execution.ErrorMessage = strings.TrimSpace(firstNonEmpty(result.Error, output, fmt.Sprintf("proxmox guest %s exited with status %d", operation, result.ExitCode)))
		return execution, nil
	}

	verification := e.verifyProxmoxGuestState(ctx, agentID, record.ID, kind, vmid, operation)
	execution.Verification = verification
	if verification != nil && verification.Ran && !verification.Success {
		execution.Success = false
		execution.ErrorMessage = "proxmox guest lifecycle action completed but verification did not confirm the expected state"
	}
	return execution, nil
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
	if _, err := e.connectedProxmoxNodeCommandAgentID(resource); err != nil {
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

func (e proxmoxGuestActionExecutor) connectedProxmoxNodeCommandAgentID(resource unified.Resource) (string, error) {
	if e.agents == nil {
		return "", fmt.Errorf("proxmox node command agent is not connected")
	}
	if resource.Proxmox == nil {
		return "", fmt.Errorf("proxmox resource metadata missing")
	}
	if agentID := strings.TrimSpace(resource.Proxmox.LinkedAgentID); agentID != "" && e.agents.IsAgentConnected(agentID) {
		return agentID, nil
	}
	if agentID, ok := e.agents.GetAgentForHost(strings.TrimSpace(resource.Proxmox.NodeName)); ok {
		agentID = strings.TrimSpace(agentID)
		if agentID != "" && e.agents.IsAgentConnected(agentID) {
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

func (e proxmoxGuestActionExecutor) verifyProxmoxGuestState(ctx context.Context, agentID, actionID string, kind proxmoxGuestKind, vmid int, operation string) *unified.ActionVerificationResult {
	expected := proxmoxExpectedGuestStatus(operation)
	if expected == "" {
		return &unified.ActionVerificationResult{Ran: false}
	}
	command := proxmoxGuestStatusCommand(kind, vmid)

	var lastOutput string
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

		result, err := e.agents.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
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
		if result.ExitCode == 0 && parseProxmoxGuestStatus(lastOutput) == expected {
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
		Note:    "expected status " + expected,
	}
}

func proxmoxExpectedGuestStatus(operation string) string {
	switch strings.TrimSpace(operation) {
	case "start", "reboot":
		return "running"
	case "shutdown", "stop":
		return "stopped"
	default:
		return ""
	}
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
