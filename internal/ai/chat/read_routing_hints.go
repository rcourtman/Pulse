package chat

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type readRoutingMode string

const (
	readRoutingNone           readRoutingMode = ""
	readRoutingTargetHost     readRoutingMode = "target_host"
	readRoutingNativeResource readRoutingMode = "native_resource"
	readRoutingQueryOnly      readRoutingMode = "query_only"
)

type readRoutingHint struct {
	mode readRoutingMode
	ref  string
}

func buildReadRoutingHint(kind, adapter, targetHost, displayName, resourceID string) readRoutingHint {
	canonicalKind := tools.CanonicalDiscoveryResourceType(kind)
	adapter = strings.ToLower(strings.TrimSpace(adapter))
	targetHost = strings.TrimSpace(targetHost)
	resourceRef := firstNonEmptyTrimmed(displayName, resourceID)

	switch {
	case canonicalKind == "app-container" && adapter == "truenas":
		return readRoutingHint{mode: readRoutingNativeResource, ref: resourceRef}
	case isQueryOnlyAPIBackedReadResource(canonicalKind, adapter):
		return readRoutingHint{mode: readRoutingQueryOnly, ref: resourceRef}
	case targetHost != "":
		return readRoutingHint{mode: readRoutingTargetHost, ref: targetHost}
	case resourceRef != "":
		return readRoutingHint{mode: readRoutingTargetHost, ref: resourceRef}
	}

	return readRoutingHint{}
}

func readRoutingHintForMention(mention ResourceMention) readRoutingHint {
	resourceRef := firstNonEmptyTrimmed(mention.Name, mention.UnifiedResourceID, mention.ResourceID)
	return buildReadRoutingHint(mention.ResourceType, mention.Adapter, mention.TargetHost, resourceRef, mention.ResourceID)
}

func readRoutingHintForResolvedResource(kind, adapter, targetHost, displayName, resourceID string) readRoutingHint {
	return buildReadRoutingHint(kind, adapter, targetHost, displayName, resourceID)
}

func (h readRoutingHint) targetHintSuffix() string {
	switch h.mode {
	case readRoutingTargetHost:
		if h.ref != "" {
			return fmt.Sprintf(" Use target_host=\"%s\".", h.ref)
		}
	case readRoutingNativeResource, readRoutingQueryOnly:
		if h.ref != "" {
			return fmt.Sprintf(" Use resource_id=\"%s\".", h.ref)
		}
	}
	return ""
}

func (h readRoutingHint) recentLogsInstruction(resourceLabel string) string {
	switch h.mode {
	case readRoutingTargetHost:
		if h.ref == "" {
			return ""
		}
		return fmt.Sprintf("Instruction: Show logs for %s (last 50 lines). Use pulse_read action=logs target_host=\"%s\" lines=50.", resourceLabel, h.ref)
	case readRoutingNativeResource:
		if h.ref == "" {
			return ""
		}
		return fmt.Sprintf("Instruction: Show logs for %s (last 50 lines). Use pulse_read action=logs resource_id=\"%s\" lines=50.", resourceLabel, h.ref)
	case readRoutingQueryOnly:
		if h.ref == "" {
			return ""
		}
		return fmt.Sprintf("Instruction: %s does not support shared log reads. Use pulse_query action=get resource_id=\"%s\" to inspect current status, alerts, recent activity, and metrics instead.", resourceLabel, h.ref)
	default:
		return ""
	}
}

func (h readRoutingHint) prefetchInstruction() string {
	switch h.mode {
	case readRoutingNativeResource:
		if h.ref == "" {
			return "Native API-backed app. Use shared resource_id-based reads instead of Docker-style target routing."
		}
		return fmt.Sprintf("Native API-backed app. Use pulse_read action=logs resource_id=\"%s\" or pulse_query action=config resource_id=\"%s\". Do NOT use pulse_discovery or target_host.", h.ref, h.ref)
	case readRoutingQueryOnly:
		if h.ref == "" {
			return "API-backed read-only resource. Use pulse_query get/search on the shared resource identity. Do NOT use target_host, pulse_control, or pulse_discovery."
		}
		return fmt.Sprintf("API-backed read-only resource. Use pulse_query action=get resource_id=\"%s\" for status, alerts, recent activity, and metrics. Do NOT use target_host, pulse_control, or pulse_discovery.", h.ref)
	default:
		return ""
	}
}

func mentionUsesDockerRouting(mention ResourceMention) bool {
	if tools.CanonicalDiscoveryResourceType(mention.ResourceType) != "app-container" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(mention.Adapter), "truenas") {
		return false
	}
	return mention.DockerHostType != "" || mention.DockerHostName != "" || len(mention.BindMounts) > 0
}

func isQueryOnlyAPIBackedReadResource(kind, adapter string) bool {
	switch strings.ToLower(strings.TrimSpace(adapter)) {
	case "vmware-vsphere":
		switch kind {
		case "agent", "vm", "storage":
			return true
		}
	case "truenas":
		switch kind {
		case "agent", "storage", "physical-disk":
			return true
		}
	}
	return false
}

func mentionAdapterFromResolved(resource *unifiedresources.Resource) string {
	if resource == nil {
		return ""
	}

	switch {
	case resource.TrueNAS != nil || resourceHasTagCaseInsensitive(*resource, "truenas"):
		return "truenas"
	case resource.VMware != nil:
		return "vmware-vsphere"
	case resource.Proxmox != nil:
		return "proxmox"
	case resource.Docker != nil:
		return "docker"
	case resource.Agent != nil && resource.Agent.CommandsEnabled:
		return "direct"
	case resource.Agent != nil && strings.TrimSpace(resource.Agent.Platform) != "":
		return strings.ToLower(strings.TrimSpace(resource.Agent.Platform))
	default:
		return ""
	}
}

func resourceHasTagCaseInsensitive(resource unifiedresources.Resource, needle string) bool {
	for _, tag := range resource.Tags {
		if strings.EqualFold(strings.TrimSpace(tag), needle) {
			return true
		}
	}
	return false
}
