package agentcapabilities

import (
	"errors"
	"fmt"
	"strings"
)

const (
	ActionTargetTypeAgent           = "agent"
	ActionTargetTypeSystemContainer = "system-container"
	ActionTargetTypeVM              = "vm"
)

const (
	actionTargetTypeAllowedDescription   = "agent, system-container, vm"
	actionResourceTypeAllowedDescription = "vm, system-container, oci-container, app-container, pod, agent, node, docker-host, k8s-cluster, k8s-node, k8s-deployment, k8s-service, storage, disk, pbs, pmg, proxmox, ceph"
)

// NormalizeActionTargetType returns the canonical governed action target type
// used by Pulse Assistant and external-agent adapters.
func NormalizeActionTargetType(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// IsActionTargetType reports whether targetType is one of the shared governed
// action target kinds.
func IsActionTargetType(targetType string) bool {
	switch NormalizeActionTargetType(targetType) {
	case ActionTargetTypeAgent, ActionTargetTypeSystemContainer, ActionTargetTypeVM:
		return true
	default:
		return false
	}
}

// ActionTargetTypeAllowedDescription returns the operator-facing list of
// action target kinds accepted by governed execution surfaces.
func ActionTargetTypeAllowedDescription() string {
	return actionTargetTypeAllowedDescription
}

// NormalizeAndValidateOptionalActionTargetType validates an optional action
// target type while preserving the empty value for callers that default later.
func NormalizeAndValidateOptionalActionTargetType(raw string) (string, error) {
	targetType := NormalizeActionTargetType(raw)
	if targetType == "" {
		return "", nil
	}
	if !IsActionTargetType(targetType) {
		return "", fmt.Errorf("invalid target_type")
	}
	return targetType, nil
}

// NormalizeActionResourceType canonicalizes resource type aliases that map to
// governed action targets. It is intentionally narrow to action-routing needs,
// not a general resource model normalizer.
func NormalizeActionResourceType(raw string) string {
	switch normalized := strings.ToLower(strings.TrimSpace(raw)); normalized {
	case "truenas":
		return ActionTargetTypeAgent
	default:
		return normalized
	}
}

// ActionTargetTypeForResourceType maps a Pulse resource type to the governed
// action target type used for Assistant approvals and external-agent actions.
func ActionTargetTypeForResourceType(raw string) (string, error) {
	switch resourceType := NormalizeActionResourceType(raw); resourceType {
	case ActionTargetTypeVM:
		return ActionTargetTypeVM, nil
	case ActionTargetTypeSystemContainer, "oci-container":
		return ActionTargetTypeSystemContainer, nil
	case ActionTargetTypeAgent, "node", "docker-host", "app-container", "pod", "k8s-node", "k8s-cluster", "k8s-deployment", "k8s-service", "storage", "disk", "pbs", "pmg", "proxmox", "ceph":
		return ActionTargetTypeAgent, nil
	case "":
		return "", errors.New("resource_type is required")
	default:
		return "", fmt.Errorf("unsupported resource_type %q (allowed: %s)", raw, actionResourceTypeAllowedDescription)
	}
}
