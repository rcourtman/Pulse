package tools

import (
	"fmt"
	"strings"
	"time"
)

const currentResourceHandle = "current_resource"

const currentResourceSelectionWindow = 365 * 24 * time.Hour

type sortedExplicitResourceProvider interface {
	GetRecentlyAccessedResourcesSorted(window time.Duration, max int) []string
}

func isCurrentResourceReference(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case currentResourceHandle, "attached_resource", "selected_resource", "this_resource", "redacted by policy":
		return true
	default:
		return false
	}
}

func (e *PulseToolExecutor) resolveCurrentResource() (ResolvedResourceInfo, error) {
	if e == nil || e.resolvedContext == nil {
		return nil, fmt.Errorf("%s is unavailable because no resource context is attached", currentResourceHandle)
	}

	recentIDs := e.resolvedContext.GetRecentlyAccessedResources(currentResourceSelectionWindow)
	if sorted, ok := e.resolvedContext.(sortedExplicitResourceProvider); ok {
		recentIDs = sorted.GetRecentlyAccessedResourcesSorted(currentResourceSelectionWindow, 1)
	}
	resolved := make([]ResolvedResourceInfo, 0, len(recentIDs))
	seen := make(map[string]struct{}, len(recentIDs))
	for _, id := range recentIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if resource, ok := e.resolvedContext.GetResolvedResourceByID(id); ok && resource != nil {
			resolved = append(resolved, resource)
		}
	}

	switch len(resolved) {
	case 0:
		return nil, fmt.Errorf("%s is unavailable because no single attached resource is selected", currentResourceHandle)
	case 1:
		return resolved[0], nil
	default:
		return nil, fmt.Errorf("%s is ambiguous because multiple resources were recently selected; use pulse_query get for the intended resource first", currentResourceHandle)
	}
}

func canonicalQueryTypeForResolvedResource(resource ResolvedResourceInfo) string {
	if resource == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(resource.GetKind())) {
	case "lxc", "container", "system-container":
		return "system-container"
	case "vm", "agent", "app-container", "storage":
		return strings.ToLower(strings.TrimSpace(resource.GetKind()))
	case "node", "docker-host":
		return "agent"
	default:
		return strings.ToLower(strings.TrimSpace(resource.GetResourceType()))
	}
}

func canonicalQueryIDForResolvedResource(resource ResolvedResourceInfo) string {
	if resource == nil {
		return ""
	}
	for _, candidate := range []string{
		resource.GetProviderUID(),
		resource.GetResourceID(),
	} {
		if candidate = strings.TrimSpace(candidate); candidate != "" {
			return candidate
		}
	}
	for _, alias := range resource.GetAliases() {
		if alias = strings.TrimSpace(alias); alias != "" {
			return alias
		}
	}
	return ""
}

func resolvedResourceKindMatchesLocation(resource ResolvedResourceInfo, locType string) bool {
	if resource == nil {
		return false
	}
	kind := canonicalQueryTypeForResolvedResource(resource)
	locType = strings.ToLower(strings.TrimSpace(locType))
	switch kind {
	case "system-container":
		return locType == "system-container" || locType == "lxc"
	case "vm":
		return locType == "vm"
	case "agent":
		return locType == "agent" || locType == "node" || locType == "docker-host"
	case "app-container":
		return locType == "app-container" || locType == "docker-host"
	default:
		return kind != "" && kind == locType
	}
}

func (e *PulseToolExecutor) commandTargetForResolvedResource(resource ResolvedResourceInfo) (string, error) {
	if resource == nil {
		return "", fmt.Errorf("%s is unavailable because no resource context is attached", currentResourceHandle)
	}

	candidates := append([]string(nil), resource.GetAliases()...)
	candidates = append(candidates,
		resource.GetProviderUID(),
		resource.GetResourceID(),
		resource.GetTargetHost(),
	)
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		key := strings.ToLower(candidate)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		loc := e.resolveResourceLocation(candidate)
		if loc.Found && resolvedResourceKindMatchesLocation(resource, loc.ResourceType) {
			return candidate, nil
		}
	}

	switch canonicalQueryTypeForResolvedResource(resource) {
	case "vm", "system-container":
		if providerUID := strings.TrimSpace(resource.GetProviderUID()); providerUID != "" {
			return providerUID, nil
		}
	}
	if targetHost := strings.TrimSpace(resource.GetTargetHost()); targetHost != "" {
		return targetHost, nil
	}
	return "", fmt.Errorf("%s does not expose a command-capable target", currentResourceHandle)
}

func (e *PulseToolExecutor) resolveCurrentResourceCommandTarget(value string) (string, error) {
	if !isCurrentResourceReference(value) {
		return strings.TrimSpace(value), nil
	}
	resource, err := e.resolveCurrentResource()
	if err != nil {
		return "", err
	}
	return e.commandTargetForResolvedResource(resource)
}
