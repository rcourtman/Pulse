package api

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
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
	result, err := e.agents.ExecuteHostUpdate(ctx, agentID, agentexec.HostUpdatePayload{
		RequestID:             attempt.ID,
		ActionID:              record.ID,
		Operation:             agentexec.HostUpdateOperationInstall,
		ExpectedInventoryHash: resource.Agent.PackageUpdates.InventoryHash,
		Timeout:               900,
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("host update agent returned no result")
	}

	output := fmt.Sprintf("APT package updates: %d pending before, %d pending after; reboot required: %t", result.Before.PendingCount, result.After.PendingCount, result.After.RebootRequired)
	execution := &unified.ExecutionResult{
		Success:      result.Success,
		Output:       output,
		Verification: projectHostUpdateVerification(result),
	}
	if !result.Success {
		execution.ErrorMessage = "host package update failed on the agent"
	}
	return execution, nil
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
	if status.CheckedAt.IsZero() || e.currentTime().Sub(status.CheckedAt.UTC()) > unified.HostPackageUpdateFreshness {
		return fmt.Errorf("host package update inventory is stale")
	}
	if status.PendingCount <= 0 {
		return fmt.Errorf("host has no pending package updates")
	}
	agentID := strings.TrimSpace(resource.Agent.AgentID)
	if agentID == "" || e.agents == nil || !e.agents.IsAgentConnected(agentID) {
		return fmt.Errorf("host command agent is disconnected")
	}
	return nil
}

func (e hostUpdateActionExecutor) currentTime() time.Time {
	if e.now != nil {
		return e.now().UTC()
	}
	return time.Now().UTC()
}

func projectHostUpdateVerification(result *agentexec.HostUpdateResultPayload) *unified.ActionVerificationResult {
	if result == nil {
		return &unified.ActionVerificationResult{Ran: false, Note: "agent returned no verification evidence"}
	}
	note := fmt.Sprintf("pending packages %d -> %d; reboot required=%t", result.Before.PendingCount, result.After.PendingCount, result.After.RebootRequired)
	switch strings.TrimSpace(result.Verification) {
	case agentexec.HostUpdateVerificationVerified:
		if !result.Success || !result.After.Supported || result.After.Manager != "apt" || result.After.PendingCount != 0 || !hostPackageUpdateInventoryHashPattern.MatchString(result.After.InventoryHash) {
			return &unified.ActionVerificationResult{Ran: false, Note: "agent returned invalid verification evidence"}
		}
		return &unified.ActionVerificationResult{Ran: true, Success: true, Command: "typed:host_update/install_os_updates", Output: note, Note: note, RanAt: time.Now().UTC()}
	case agentexec.HostUpdateVerificationFailed:
		return &unified.ActionVerificationResult{Ran: true, Success: false, Command: "typed:host_update/install_os_updates", Output: note, Note: "post-update package inventory did not confirm the intended state", RanAt: time.Now().UTC()}
	default:
		return &unified.ActionVerificationResult{Ran: false, Note: "post-update package inventory was inconclusive"}
	}
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
