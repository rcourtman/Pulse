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
	hostStorageCleanupActionHandler = "host.storage_cleanup"
	hostStorageCleanupCapability    = "clean_package_cache"
)

var hostStorageCleanupFingerprintPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

type hostStorageCleanupAgentCommander interface {
	ExecuteHostStorageCleanup(ctx context.Context, agentID string, req agentexec.HostStorageCleanupPayload) (*agentexec.HostStorageCleanupResultPayload, error)
	IsAgentConnected(agentID string) bool
}

type hostStorageCleanupActionExecutor struct {
	resources *ResourceHandlers
	agents    hostStorageCleanupAgentCommander
	now       func() time.Time
}

func newHostStorageCleanupActionExecutor(resources *ResourceHandlers, agents hostStorageCleanupAgentCommander) ActionExecutor {
	if resources == nil || agents == nil {
		return nil
	}
	return hostStorageCleanupActionExecutor{resources: resources, agents: agents, now: time.Now}
}

func (e hostStorageCleanupActionExecutor) ActionHandlerNames() []string {
	return []string{hostStorageCleanupActionHandler}
}

func (e hostStorageCleanupActionExecutor) CheckActionAvailable(_ context.Context, req unified.ActionRequest, resource unified.Resource) unified.ResourceActionReadiness {
	if strings.TrimSpace(req.CapabilityName) != hostStorageCleanupCapability {
		return unified.ResourceActionReadiness{}
	}
	capability, ok := resourceCapabilityByName(resource.Capabilities, hostStorageCleanupCapability)
	if !ok || capability.InternalHandler != hostStorageCleanupActionHandler {
		return unified.ResourceActionReadiness{}
	}
	if err := e.validateResource(resource); err != nil {
		return unified.ResourceActionReadiness{
			Name:       hostStorageCleanupCapability,
			Available:  false,
			ReasonCode: hostStorageCleanupUnavailableReasonCode(err),
			Reason:     hostStorageCleanupUnavailableReason(err),
		}
	}
	return unified.ResourceActionReadiness{Name: hostStorageCleanupCapability, Available: true}
}

func (e hostStorageCleanupActionExecutor) ExecuteAction(ctx context.Context, record unified.ActionAuditRecord) (*unified.ExecutionResult, error) {
	attempt, ok := actionlifecycle.DispatchAttemptFromContext(ctx)
	if !ok || attempt.ActionID != record.ID {
		return nil, fmt.Errorf("committed action dispatch authority is required")
	}
	record, err := unified.NormalizeActionAuditRecord(record)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(record.Request.CapabilityName) != hostStorageCleanupCapability {
		return nil, fmt.Errorf("unsupported host storage cleanup capability %q", record.Request.CapabilityName)
	}
	resource, err := e.currentResource(ctx, record.Request.ResourceID)
	if err != nil {
		return nil, err
	}
	result, err := e.agents.ExecuteHostStorageCleanup(ctx, strings.TrimSpace(resource.Agent.AgentID), agentexec.HostStorageCleanupPayload{
		RequestID:           attempt.ID,
		ActionID:            record.ID,
		Operation:           agentexec.HostStorageCleanupOperationPackageCache,
		ExpectedFingerprint: resource.Agent.StorageCleanup.Fingerprint,
		Timeout:             300,
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("host storage cleanup agent returned no result")
	}

	output := fmt.Sprintf("APT package cache: %d bytes before, %d bytes after, %d bytes reclaimed", result.Before.ReclaimableBytes, result.After.ReclaimableBytes, result.ReclaimedBytes)
	beforeBound := result.Before.Fingerprint == resource.Agent.StorageCleanup.Fingerprint
	return hostAPTExecutionResult(record.Request.ResourceID, resource.Agent.AgentID, agentexec.HostStorageCleanupOperationPackageCache, output, result.Success, result.MutationStarted, result.Verification, beforeBound, result.Before.CheckedAt, result.After.CheckedAt, e.currentTime())
}

func (e hostStorageCleanupActionExecutor) currentResource(ctx context.Context, resourceID string) (unified.Resource, error) {
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

func (e hostStorageCleanupActionExecutor) validateResource(resource unified.Resource) error {
	if resource.Type != unified.ResourceTypeAgent || resource.Agent == nil {
		return fmt.Errorf("resource is not an agent-managed host")
	}
	capability, ok := resourceCapabilityByName(resource.Capabilities, hostStorageCleanupCapability)
	if !ok || capability.InternalHandler != hostStorageCleanupActionHandler {
		return fmt.Errorf("host does not currently advertise typed package-cache cleanup")
	}
	if !resource.Agent.CommandsEnabled {
		return fmt.Errorf("host command operations are disabled")
	}
	status := resource.Agent.StorageCleanup
	if status == nil || !status.Supported || strings.TrimSpace(status.Provider) != "apt-package-cache" {
		return fmt.Errorf("host storage cleanup provider is unsupported")
	}
	if strings.TrimSpace(status.Error) != "" {
		return fmt.Errorf("host storage cleanup inventory has an error")
	}
	if !hostStorageCleanupFingerprintPattern.MatchString(strings.TrimSpace(status.Fingerprint)) {
		return fmt.Errorf("host storage cleanup fingerprint is invalid")
	}
	if !unified.HostStorageCleanupTelemetryFresh(status, e.currentTime()) {
		return fmt.Errorf("host storage cleanup inventory is stale")
	}
	if status.ReclaimableBytes < unified.HostStorageCleanupMinReclaimableBytes {
		return fmt.Errorf("host package cache has insufficient reclaimable space")
	}
	if _, ok := unified.HostStorageCleanupPressureDisk(resource.Agent.Disks); !ok {
		return fmt.Errorf("package cache filesystem is not under storage pressure")
	}
	agentID := strings.TrimSpace(resource.Agent.AgentID)
	if agentID == "" || e.agents == nil || !e.agents.IsAgentConnected(agentID) {
		return fmt.Errorf("host command agent is disconnected")
	}
	return nil
}

func (e hostStorageCleanupActionExecutor) currentTime() time.Time {
	if e.now != nil {
		return e.now().UTC()
	}
	return time.Now().UTC()
}

func hostStorageCleanupUnavailableReasonCode(err error) string {
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
		return "unsupported_cleanup_provider"
	case strings.Contains(message, "inventory") && strings.Contains(message, "error"):
		return "cleanup_inventory_error"
	case strings.Contains(message, "fingerprint"):
		return "invalid_cleanup_inventory"
	case strings.Contains(message, "stale"):
		return "stale_cleanup_inventory"
	case strings.Contains(message, "insufficient"):
		return "insufficient_reclaimable_space"
	case strings.Contains(message, "not under storage pressure"):
		return "storage_pressure_cleared"
	case strings.Contains(message, "disconnected"):
		return "command_agent_disconnected"
	default:
		return "capability_unavailable"
	}
}

func hostStorageCleanupUnavailableReason(err error) string {
	switch hostStorageCleanupUnavailableReasonCode(err) {
	case "unsupported_resource":
		return "This resource is not an agent-managed host."
	case "host_commands_disabled":
		return "Typed host operations are disabled for this agent."
	case "unsupported_cleanup_provider":
		return "This host does not expose a supported package-cache cleanup provider."
	case "cleanup_inventory_error":
		return "Pulse could not inspect reclaimable package-cache storage."
	case "invalid_cleanup_inventory":
		return "The package-cache inventory cannot be verified safely."
	case "stale_cleanup_inventory":
		return "The package-cache inventory is too old to clean safely."
	case "insufficient_reclaimable_space":
		return "The package cache no longer contains enough reclaimable data."
	case "storage_pressure_cleared":
		return "The filesystem containing the package cache is no longer under storage pressure."
	case "command_agent_disconnected":
		return "The host command agent is not connected."
	default:
		return "Typed package-cache cleanup is not currently available."
	}
}
