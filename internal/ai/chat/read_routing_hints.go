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

func (h readRoutingHint) targetFact() string {
	switch h.mode {
	case readRoutingTargetHost:
		if h.ref != "" {
			return fmt.Sprintf("tool addressing fact: target_host=%q", h.ref)
		}
	case readRoutingNativeResource, readRoutingQueryOnly:
		if h.ref != "" {
			return fmt.Sprintf("tool addressing fact: resource_id=%q", h.ref)
		}
	}
	return ""
}

func (h readRoutingHint) prefetchContext() string {
	switch h.mode {
	case readRoutingNativeResource:
		if h.ref == "" {
			return "Native API-backed app. Shared reads address this resource by resource_id rather than Docker-style target routing."
		}
		return fmt.Sprintf("Native API-backed app. Shared reads address this resource by resource_id=%q; target_host is not valid for this resource.", h.ref)
	case readRoutingQueryOnly:
		if h.ref == "" {
			return "API-backed read-only resource. Shared reads address this resource by resource_id; target_host and control routing are not valid for this resource."
		}
		return fmt.Sprintf("API-backed read-only resource. Status, alerts, recent activity, and metrics are available through resource_id=%q; target_host and control routing are not valid for this resource.", h.ref)
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
