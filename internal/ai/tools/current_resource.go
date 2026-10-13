package tools

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const currentResourceHandle = agentcapabilities.CurrentResourceHandle

const currentResourceSelectionWindow = 365 * 24 * time.Hour

type sortedExplicitResourceProvider interface {
	GetRecentlyAccessedResourcesSorted(window time.Duration, max int) []string
}

// IsCurrentResourceReference reports whether a tool argument is one of the
// session-scoped placeholders that must resolve through attached resource
// context before execution.
func IsCurrentResourceReference(value string) bool {
	return agentcapabilities.IsCurrentResourceReference(value)
}

// ValidateCurrentResourceAvailable checks whether the session currently has a
// single attached resource that can satisfy current_resource placeholders.
func (e *PulseToolExecutor) ValidateCurrentResourceAvailable() error {
	_, err := e.resolveCurrentResource()
	return err
}

func (e *PulseToolExecutor) resolveCurrentResource() (ResolvedResourceInfo, error) {
	if e == nil || e.resolvedContext == nil {
		return nil, fmt.Errorf("%s is unavailable because no Pulse resource context is attached to this chat turn", currentResourceHandle)
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
		return nil, fmt.Errorf("%s is unavailable because no single attached resource is selected in this chat turn", currentResourceHandle)
	case 1:
		return resolved[0], nil
	default:
		return nil, fmt.Errorf("%s is ambiguous because multiple resources were recently selected; ask which host, VM, container, app, or storage resource the user means", currentResourceHandle)
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
		resource.GetResourceID(),
		resource.GetProviderUID(),
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
		return "", fmt.Errorf("%s is unavailable because no Pulse resource context is attached to this chat turn", currentResourceHandle)
	}

	if id := strings.TrimSpace(resource.GetResourceID()); id != "" {
		rs, err := e.readStateForControl()
		if err != nil {
			return "", err
		}
		loc := unifiedresources.ResolveResourceReferenceContext(rs, id).Location
		if loc.Found && resolvedResourceKindMatchesLocation(resource, loc.ResourceType) {
			return id, nil
		}
		return "", fmt.Errorf("%s no longer resolves to its registered resource", currentResourceHandle)
	}
	for _, candidate := range resource.GetAliases() {
		loc := e.resolveResourceLocation(candidate)
		if loc.Found && !loc.Ambiguous && resolvedResourceKindMatchesLocation(resource, loc.ResourceType) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("%s does not expose a command-capable target", currentResourceHandle)
}

func (e *PulseToolExecutor) resolveCurrentResourceCommandTarget(value string) (string, error) {
	if !IsCurrentResourceReference(value) {
		return strings.TrimSpace(value), nil
	}
	resource, err := e.resolveCurrentResource()
	if err != nil {
		return "", err
	}
	return e.commandTargetForResolvedResource(resource)
}
