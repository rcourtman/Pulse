package chat

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// ResourceMention represents a detected resource mention in a user message
type ResourceMention struct {
	Name              string
	ResourceType      string // "vm", "system-container", "app-container", "agent", "node"
	ResourceID        string
	TargetID          string
	Adapter           string
	MatchedText       string // The actual text that matched
	Policy            *unifiedresources.ResourcePolicy
	AISafeSummary     string
	UnifiedResourceID string
	SupportsControl   bool
	// Docker-specific: bind mounts (source -> destination)
	BindMounts []MountInfo
	// Full routing chain (for Docker containers)
	DockerHostName string // Name of system-container/VM/host running Docker (e.g., "homepage-docker")
	DockerHostType string // "system-container", "vm", or "standalone" (from ResolveResource)
	DockerHostVMID int    // Guest ID (VMID) if DockerHost is a system container or VM
	NodeName       string // Hypervisor node name (e.g., "pve-node")
	TargetHost     string // The correct target_host to use for commands
}

// MountInfo represents a bind mount mapping
type MountInfo struct {
	Source      string // Host path (where to actually edit files)
	Destination string // Container path (what the service sees)
}

// PrefetchedContext contains proactively gathered context for a user message
type PrefetchedContext struct {
	Mentions    []ResourceMention
	Discoveries []*tools.ResourceDiscoveryInfo
	Summary     string // Formatted summary for AI consumption
}

// ContextPrefetcher proactively gathers context based on user message content
type ContextPrefetcher struct {
	readState         unifiedresources.ReadState
	discoveryProvider tools.DiscoveryProvider
}

// NewContextPrefetcher creates a new context prefetcher
func NewContextPrefetcher(readState unifiedresources.ReadState, discoveryProvider tools.DiscoveryProvider) *ContextPrefetcher {
	return &ContextPrefetcher{
		readState:         readState,
		discoveryProvider: discoveryProvider,
	}
}

func resourceMentionGovernance(resource *unifiedresources.Resource) (*unifiedresources.ResourcePolicy, string) {
	return unifiedresources.CanonicalGovernanceMetadata(resource)
}

func resourceSupportsControl(resourceType string, resource *unifiedresources.Resource) bool {
	switch tools.CanonicalDiscoveryResourceType(resourceType) {
	case "vm", "system-container":
		return resource != nil && resource.Proxmox != nil
	case "app-container":
		if resource == nil {
			return false
		}
		return resource.Docker != nil || resource.TrueNAS != nil
	case "agent":
		return resource != nil && resource.Agent != nil && resource.Agent.CommandsEnabled
	default:
		return false
	}
}

func resourceRequiresReadOnlyGuidance(resourceType string, supportsControl bool) bool {
	switch tools.CanonicalDiscoveryResourceType(resourceType) {
	case "vm", "system-container", "agent", "storage":
		return !supportsControl
	default:
		return false
	}
}

// Prefetch gathers context only for explicit structured mentions selected by
// the user. Plain chat text is left untouched so the selected model decides
// whether it needs tools or more context.
func (p *ContextPrefetcher) Prefetch(ctx context.Context, message string, structuredMentions []StructuredMention) *PrefetchedContext {
	log.Info().
		Bool("hasReadState", p.readState != nil).
		Bool("hasDiscoveryProvider", p.discoveryProvider != nil).
		Int("structured_mentions", len(structuredMentions)).
		Msg("[ContextPrefetch] Starting prefetch")

	if len(structuredMentions) == 0 {
		log.Debug().Msg("[ContextPrefetch] No structured mentions; skipping pre-model context lookup")
		return nil
	}

	if p.readState == nil {
		log.Warn().Msg("[ContextPrefetch] No ReadState, cannot prefetch")
		return nil
	}

	vmsCount := len(p.readState.VMs())
	containersCount := len(p.readState.Containers())
	nodesCount := len(p.readState.Nodes())
	dockerHostsCount := len(p.readState.DockerHosts())
	log.Info().
		Int("vms", vmsCount).
		Int("containers", containersCount).
		Int("nodes", nodesCount).
		Int("dockerHosts", dockerHostsCount).
		Msg("[ContextPrefetch] Got state for matching")

	mentions := p.resolveStructuredMentions(structuredMentions)
	log.Info().
		Int("structured_input", len(structuredMentions)).
		Int("resolved", len(mentions)).
		Msg("[ContextPrefetch] Resolved structured mentions")

	if len(mentions) == 0 {
		log.Info().Str("message", message[:min(50, len(message))]).Msg("[ContextPrefetch] No structured mentions resolved")
		return nil
	}

	log.Info().
		Int("mentions_found", len(mentions)).
		Msg("[ContextPrefetch] Found resource mentions in message")

	// Gather discovery data for each mention
	var discoveries []*tools.ResourceDiscoveryInfo
	if p.discoveryProvider != nil {
		for _, mention := range mentions {
			if unifiedresources.ResourcePolicyRequiresGovernedSummary(mention.Policy) {
				continue
			}
			discovery, err := p.getOrTriggerDiscovery(ctx, mention)
			if err != nil {
				log.Debug().
					Err(err).
					Str("resource", mention.Name).
					Msg("[ContextPrefetch] Failed to get discovery")
				continue
			}
			if discovery != nil {
				discoveries = append(discoveries, discovery)
			}
		}
	}

	// Format the context summary
	summary := p.formatContextSummary(mentions, discoveries)

	return &PrefetchedContext{
		Mentions:    mentions,
		Discoveries: discoveries,
		Summary:     summary,
	}
}

// resolveStructuredMentions converts frontend StructuredMention objects into ResourceMention
// objects with full routing info.
func (p *ContextPrefetcher) resolveStructuredMentions(structured []StructuredMention) []ResourceMention {
	var mentions []ResourceMention
	rs := p.readState

	for _, sm := range structured {
		// Parse the structured ID to extract resource details.
		// Frontend ID formats: "vm:node:vmid", "system-container:node:vmid",
		// "app-container:host:providerUid" (canonical), legacy "docker:hostId:containerId",
		// "agent:id", "node:instance:name"
		parts := strings.Split(sm.ID, ":")

		// Enforce canonical v6 frontend mention types only.
		legacyMentionType := strings.ToLower(strings.TrimSpace(sm.Type))
		if legacyMentionType == "container" || legacyMentionType == "lxc" || legacyMentionType == "docker" || legacyMentionType == "docker-container" {
			log.Warn().
				Str("name", sm.Name).
				Str("id", sm.ID).
				Msg("[ContextPrefetch] Ignoring unsupported legacy structured mention type")
			continue
		}
		resourceType := tools.CanonicalDiscoveryResourceType(sm.Type)

		// Use the canonical unified-resource resolver for routing plus policy metadata.
		resolved := unifiedresources.ResolveResourceContext(rs, sm.Name)
		policy, aiSafeSummary := resourceMentionGovernance(resolved.Resource)
		loc := resolved.Location

		switch resourceType {
		case "vm":
			vmID := ""
			node := sm.Node
			if len(parts) >= 3 {
				node = parts[1]
				vmID = strings.Join(parts[2:], ":")
			}
			vmID = firstNonEmptyTrimmed(vmID, mentionResourceIDFromResolved("vm", resolved.Resource))
			node = firstNonEmptyTrimmed(node, mentionTargetIDFromResolved("vm", resolved.Resource))
			mentions = append(mentions, ResourceMention{
				Name:              sm.Name,
				ResourceType:      "vm",
				ResourceID:        vmID,
				TargetID:          node,
				Adapter:           mentionAdapterFromResolved(resolved.Resource),
				MatchedText:       sm.Name,
				Policy:            policy,
				AISafeSummary:     aiSafeSummary,
				UnifiedResourceID: resolvedStructuredUnifiedResourceID(sm.ID, resolved.Resource),
				TargetHost:        loc.TargetHost,
				SupportsControl:   resourceSupportsControl("vm", resolved.Resource),
			})

		case "system-container":
			vmID := ""
			node := sm.Node
			if len(parts) >= 3 {
				node = parts[1]
				vmID = strings.Join(parts[2:], ":")
			}
			vmID = firstNonEmptyTrimmed(vmID, mentionResourceIDFromResolved("system-container", resolved.Resource))
			node = firstNonEmptyTrimmed(node, mentionTargetIDFromResolved("system-container", resolved.Resource))
			mentions = append(mentions, ResourceMention{
				Name:              sm.Name,
				ResourceType:      "system-container",
				ResourceID:        vmID,
				TargetID:          node,
				Adapter:           mentionAdapterFromResolved(resolved.Resource),
				MatchedText:       sm.Name,
				Policy:            policy,
				AISafeSummary:     aiSafeSummary,
				UnifiedResourceID: resolvedStructuredUnifiedResourceID(sm.ID, resolved.Resource),
				TargetHost:        loc.TargetHost,
				SupportsControl:   resourceSupportsControl("system-container", resolved.Resource),
			})

		case "app-container":
			hostID, containerID := parseStructuredAppContainerMentionID(sm.ID, rs)
			if resolved.Resource != nil {
				resolvedHost, resolvedContainerID := resolvedAppContainerMentionCoordinates(resolved.Resource)
				hostID = firstNonEmptyTrimmed(hostID, resolvedHost)
				containerID = firstNonEmptyTrimmed(containerID, resolvedContainerID)
			}

			// Gather bind mounts via ReadState
			var mounts []MountInfo
			if rs != nil {
				for _, container := range rs.DockerContainers() {
					if container == nil {
						continue
					}
					if container.Name() == sm.Name || container.ContainerID() == containerID {
						for _, m := range container.Mounts() {
							if m.Source != "" && m.Destination != "" {
								mounts = append(mounts, MountInfo{
									Source:      m.Source,
									Destination: m.Destination,
								})
							}
						}
						break
					}
				}
			}

			mentions = append(mentions, ResourceMention{
				Name:              sm.Name,
				ResourceType:      "app-container",
				ResourceID:        containerID,
				TargetID:          hostID,
				Adapter:           mentionAdapterFromResolved(resolved.Resource),
				MatchedText:       sm.Name,
				Policy:            policy,
				AISafeSummary:     aiSafeSummary,
				UnifiedResourceID: resolvedStructuredUnifiedResourceID(sm.ID, resolved.Resource),
				BindMounts:        mounts,
				DockerHostName:    loc.DockerHostName,
				DockerHostType:    loc.DockerHostType,
				DockerHostVMID:    loc.DockerHostVMID,
				NodeName:          loc.Node,
				TargetHost:        loc.TargetHost,
			})

		case "storage":
			resourceID := firstNonEmptyTrimmed(sm.ID, resolvedUnifiedResourceID(resolved.Resource), mentionResourceIDFromResolved("storage", resolved.Resource))
			targetID := firstNonEmptyTrimmed(sm.Node, mentionTargetIDFromResolved("storage", resolved.Resource), loc.TargetHost)
			mentions = append(mentions, ResourceMention{
				Name:              sm.Name,
				ResourceType:      "storage",
				ResourceID:        resourceID,
				TargetID:          targetID,
				Adapter:           mentionAdapterFromResolved(resolved.Resource),
				MatchedText:       sm.Name,
				Policy:            policy,
				AISafeSummary:     aiSafeSummary,
				UnifiedResourceID: resolvedStructuredUnifiedResourceID(sm.ID, resolved.Resource),
				SupportsControl:   resourceSupportsControl("storage", resolved.Resource),
				TargetHost:        firstNonEmptyTrimmed(loc.TargetHost, targetID),
			})

		case "node":
			mentions = append(mentions, ResourceMention{
				Name:              sm.Name,
				ResourceType:      "node",
				ResourceID:        sm.Name,
				TargetID:          sm.Name,
				MatchedText:       sm.Name,
				Policy:            policy,
				AISafeSummary:     aiSafeSummary,
				UnifiedResourceID: resolvedStructuredUnifiedResourceID(sm.ID, resolved.Resource),
				TargetHost:        loc.TargetHost,
			})

		case "agent":
			hostID := ""
			if len(parts) >= 2 {
				hostID = strings.Join(parts[1:], ":")
			}
			mentions = append(mentions, ResourceMention{
				Name:              sm.Name,
				ResourceType:      "agent",
				ResourceID:        hostID,
				TargetID:          hostID,
				Adapter:           mentionAdapterFromResolved(resolved.Resource),
				MatchedText:       sm.Name,
				Policy:            policy,
				AISafeSummary:     aiSafeSummary,
				UnifiedResourceID: resolvedStructuredUnifiedResourceID(sm.ID, resolved.Resource),
				SupportsControl:   resourceSupportsControl("agent", resolved.Resource),
				TargetHost:        loc.TargetHost,
			})

		default:
			if unifiedresources.IsUnsupportedLegacyResourceTypeAlias(sm.Type) {
				log.Warn().
					Str("name", sm.Name).
					Str("id", sm.ID).
					Msg("[ContextPrefetch] Ignoring unsupported structured mention type")
				continue
			}
			log.Warn().
				Str("name", sm.Name).
				Str("type", sm.Type).
				Msg("[ContextPrefetch] Unknown structured mention type, falling back to ResolveResource")
			mentions = append(mentions, ResourceMention{
				Name:              sm.Name,
				ResourceType:      resourceType,
				ResourceID:        firstNonEmptyTrimmed(sm.ID, mentionResourceIDFromResolved(resourceType, resolved.Resource)),
				TargetID:          firstNonEmptyTrimmed(sm.Node, mentionTargetIDFromResolved(resourceType, resolved.Resource)),
				MatchedText:       sm.Name,
				Policy:            policy,
				AISafeSummary:     aiSafeSummary,
				UnifiedResourceID: resolvedStructuredUnifiedResourceID(sm.ID, resolved.Resource),
				TargetHost:        loc.TargetHost,
			})
		}
	}

	return mentions
}

type appContainerResourceGetter interface {
	GetByType(unifiedresources.ResourceType) []unifiedresources.Resource
}

type appContainerResourceLister interface {
	ListByType(unifiedresources.ResourceType) []unifiedresources.Resource
}

func appContainerResourcesFromReadState(rs unifiedresources.ReadState) []unifiedresources.Resource {
	if rs == nil {
		return nil
	}
	if getter, ok := any(rs).(appContainerResourceGetter); ok {
		return getter.GetByType(unifiedresources.ResourceTypeAppContainer)
	}
	if lister, ok := any(rs).(appContainerResourceLister); ok {
		return lister.ListByType(unifiedresources.ResourceTypeAppContainer)
	}
	return nil
}

func appContainerResourceHost(resource unifiedresources.Resource) string {
	if host := strings.TrimSpace(resource.ParentName); host != "" {
		return host
	}
	if resource.Docker != nil {
		if host := strings.TrimSpace(resource.Docker.Hostname); host != "" {
			return host
		}
	}
	if resource.TrueNAS != nil {
		if host := strings.TrimSpace(resource.TrueNAS.Hostname); host != "" {
			return host
		}
	}
	for _, host := range resource.Identity.Hostnames {
		if host = strings.TrimSpace(host); host != "" {
			return host
		}
	}
	return ""
}

func parseStructuredAppContainerMentionID(mentionID string, rs unifiedresources.ReadState) (hostID string, containerID string) {
	const canonicalPrefix = "app-container:"
	if strings.HasPrefix(mentionID, canonicalPrefix) {
		if rs != nil {
			for _, resource := range appContainerResourcesFromReadState(rs) {
				if !strings.EqualFold(strings.TrimSpace(resource.ID), mentionID) {
					continue
				}
				host := appContainerResourceHost(resource)
				if host == "" {
					continue
				}
				if providerUID := appContainerProviderUID(resource, host); providerUID != "" {
					return host, providerUID
				}
			}
		}

		raw := strings.TrimPrefix(mentionID, canonicalPrefix)
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
			return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		}
		return "", ""
	}

	return parseStructuredDockerMentionID(mentionID, rs)
}

func appContainerProviderUID(resource unifiedresources.Resource, host string) string {
	prefix := "app-container:" + strings.TrimSpace(host) + ":"
	if strings.HasPrefix(resource.ID, prefix) {
		if providerUID := strings.TrimSpace(strings.TrimPrefix(resource.ID, prefix)); providerUID != "" {
			return providerUID
		}
	}
	if resource.Docker != nil {
		if containerID := strings.TrimSpace(resource.Docker.ContainerID); containerID != "" {
			return containerID
		}
	}
	if resource.Canonical != nil {
		if primaryID := strings.TrimSpace(resource.Canonical.PrimaryID); primaryID != "" {
			return primaryID
		}
	}
	if name := strings.TrimSpace(resource.Name); name != "" {
		return name
	}
	return ""
}

func resolvedAppContainerMentionCoordinates(resource *unifiedresources.Resource) (hostID string, containerID string) {
	if resource == nil {
		return "", ""
	}
	hostID = appContainerResourceHost(*resource)
	containerID = appContainerProviderUID(*resource, hostID)
	return strings.TrimSpace(hostID), strings.TrimSpace(containerID)
}

func resolvedUnifiedResourceID(resource *unifiedresources.Resource) string {
	if resource == nil {
		return ""
	}
	return strings.TrimSpace(resource.ID)
}

func resolvedStructuredUnifiedResourceID(structuredID string, resource *unifiedresources.Resource) string {
	return firstNonEmptyTrimmed(resolvedUnifiedResourceID(resource), structuredID)
}

func mentionResourceIDFromResolved(resourceType string, resource *unifiedresources.Resource) string {
	if resource == nil {
		return ""
	}

	switch tools.CanonicalDiscoveryResourceType(resourceType) {
	case "vm", "system-container":
		if resource.Proxmox != nil && resource.Proxmox.VMID > 0 {
			return strconv.Itoa(resource.Proxmox.VMID)
		}
		if resource.VMware != nil {
			return strings.TrimSpace(resource.VMware.ManagedObjectID)
		}
	case "storage":
		if resource.Proxmox != nil {
			return strings.TrimSpace(resource.Proxmox.SourceID)
		}
		if resource.VMware != nil {
			return strings.TrimSpace(resource.VMware.ManagedObjectID)
		}
	case "agent":
		if resource.Agent != nil {
			return strings.TrimSpace(resource.Agent.AgentID)
		}
	}

	return ""
}

func mentionTargetIDFromResolved(resourceType string, resource *unifiedresources.Resource) string {
	if resource == nil {
		return ""
	}

	switch tools.CanonicalDiscoveryResourceType(resourceType) {
	case "vm", "system-container":
		if resource.Proxmox != nil {
			return strings.TrimSpace(resource.Proxmox.NodeName)
		}
		if resource.VMware != nil {
			return firstNonEmptyTrimmed(
				resource.VMware.RuntimeHostName,
				resource.VMware.RuntimeHostID,
				resource.ParentName,
			)
		}
	case "storage":
		if resource.Proxmox != nil {
			return firstNonEmptyTrimmed(resource.Proxmox.NodeName, resource.Proxmox.Instance)
		}
		if resource.VMware != nil {
			return firstNonEmptyTrimmed(
				resource.ParentName,
				resource.VMware.ConnectionName,
				resource.VMware.RuntimeHostName,
			)
		}
	case "agent":
		if resource.Agent != nil {
			return strings.TrimSpace(resource.Agent.AgentID)
		}
	}

	return ""
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseStructuredDockerMentionID(mentionID string, rs unifiedresources.ReadState) (hostID string, containerID string) {
	const prefix = "docker:"
	if !strings.HasPrefix(mentionID, prefix) {
		return "", ""
	}

	raw := strings.TrimPrefix(mentionID, prefix)
	if raw == "" {
		return "", ""
	}

	// Prefer matching known docker host IDs via ReadState so host IDs containing
	// colons remain intact (V6 unified IDs can include colon separators).
	bestHostID := ""
	bestContainerID := ""
	if rs != nil {
		for _, dockerHost := range rs.DockerHosts() {
			if dockerHost == nil {
				continue
			}
			id := strings.TrimSpace(dockerHost.HostSourceID())
			if id == "" {
				continue
			}
			hostPrefix := id + ":"
			if !strings.HasPrefix(raw, hostPrefix) {
				continue
			}
			candidateContainerID := strings.TrimPrefix(raw, hostPrefix)
			if candidateContainerID == "" {
				continue
			}
			if len(id) > len(bestHostID) {
				bestHostID = id
				bestContainerID = candidateContainerID
			}
		}
	}
	if bestHostID != "" {
		return bestHostID, bestContainerID
	}

	// Legacy fallback: split once after the docker prefix.
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) < 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// getOrTriggerDiscovery gets existing discovery or triggers a new one
func (p *ContextPrefetcher) getOrTriggerDiscovery(ctx context.Context, mention ResourceMention) (*tools.ResourceDiscoveryInfo, error) {
	discoveryType := tools.DiscoveryProviderResourceType(mention.ResourceType)
	canonicalType := tools.CanonicalDiscoveryResourceType(mention.ResourceType)

	// First try to get existing discovery
	discovery, err := p.discoveryProvider.GetDiscoveryByResource(discoveryType, mention.TargetID, mention.ResourceID)
	if err == nil && discovery != nil {
		log.Debug().
			Str("resource", mention.Name).
			Msg("[ContextPrefetch] Using cached discovery")
		return discovery, nil
	}

	// Trigger discovery if not found (for VMs, system containers, and Docker containers)
	if canonicalType == "vm" || canonicalType == "system-container" || canonicalType == "app-container" {
		log.Debug().
			Str("resource", mention.Name).
			Str("type", canonicalType).
			Msg("[ContextPrefetch] Triggering discovery")

		discovery, err = p.discoveryProvider.TriggerDiscovery(ctx, discoveryType, mention.TargetID, mention.ResourceID, false)
		if err != nil {
			return nil, err
		}
		return discovery, nil
	}

	return nil, nil
}

// formatContextSummary creates a formatted summary of the gathered context
func (p *ContextPrefetcher) formatContextSummary(mentions []ResourceMention, discoveries []*tools.ResourceDiscoveryInfo) string {
	if len(mentions) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("=== PULSE MONITORING DATA (AUTHORITATIVE) ===\n")
	sb.WriteString("This is verified data from Pulse monitoring sources. Canonical resource policy is enforced below.\n")
	sb.WriteString(unifiedresources.ResourcePolicyGovernedSummaryPreamble())
	sb.WriteString("\n\n")

	// Create a map for quick discovery lookup
	discoveryMap := make(map[string]*tools.ResourceDiscoveryInfo)
	for _, d := range discoveries {
		discoveryTargetID := tools.CanonicalDiscoveryTargetID(d, "")
		key := fmt.Sprintf("%s:%s:%s", tools.CanonicalDiscoveryResourceType(d.ResourceType), discoveryTargetID, d.ResourceID)
		discoveryMap[key] = d
	}

	for _, mention := range mentions {
		if unifiedresources.ResourcePolicyRequiresGovernedSummary(mention.Policy) {
			sb.WriteString(unifiedresources.FormatResourcePolicyGovernedSummary(mention.AISafeSummary, mention.Policy))
			continue
		}

		key := fmt.Sprintf("%s:%s:%s", tools.CanonicalDiscoveryResourceType(mention.ResourceType), mention.TargetID, mention.ResourceID)
		discovery, hasDiscovery := discoveryMap[key]

		hint := readRoutingHintForMention(mention)

		// Docker containers get special treatment - show the full routing chain
		if mentionUsesDockerRouting(mention) {
			sb.WriteString(fmt.Sprintf("## %s (Docker container)\n", mention.Name))

			// Show the full routing chain unambiguously.
			if mention.DockerHostType == "system-container" {
				sb.WriteString(fmt.Sprintf("Location: Docker on \"%s\" (container %d) on node \"%s\"\n",
					mention.DockerHostName, mention.DockerHostVMID, mention.NodeName))
			} else if mention.DockerHostType == "vm" {
				sb.WriteString(fmt.Sprintf("Location: Docker on \"%s\" (VM %d) on node \"%s\"\n",
					mention.DockerHostName, mention.DockerHostVMID, mention.NodeName))
			} else {
				sb.WriteString(fmt.Sprintf("Location: Docker on host \"%s\"\n", mention.DockerHostName))
			}

			sb.WriteString(fmt.Sprintf("target_host: \"%s\"\n", mention.TargetHost))

			// Bind mounts - clarify these are on the LXC/VM filesystem
			if len(mention.BindMounts) > 0 {
				sb.WriteString(fmt.Sprintf("Bind mounts (paths on %s filesystem, NOT inside container):\n", mention.TargetHost))
				for _, m := range mention.BindMounts {
					sb.WriteString(fmt.Sprintf("  %s → %s\n", m.Source, m.Destination))
				}
			}

			// Add discovery info if available
			if hasDiscovery {
				if discovery.ServiceVersion != "" {
					sb.WriteString(fmt.Sprintf("Service version: %s\n", discovery.ServiceVersion))
				}
				if discovery.SuggestedURL != "" {
					sb.WriteString(fmt.Sprintf("Suggested web URL: %s\n", discovery.SuggestedURL))
				}
				if len(discovery.ConfigPaths) > 0 {
					sb.WriteString(fmt.Sprintf("Config paths: %v\n", discovery.ConfigPaths))
				}
				if len(discovery.LogPaths) > 0 {
					var filePaths []string
					for _, lp := range discovery.LogPaths {
						if strings.HasPrefix(lp, "/") {
							filePaths = append(filePaths, lp)
						}
					}
					if len(filePaths) > 0 {
						sb.WriteString(fmt.Sprintf("Log files: %v\n", filePaths))
					}
				}
			} else {
				sb.WriteString("Resource location and target_host are already resolved; discovery is not required to identify this resource.\n")
			}

			sb.WriteString("\n")
			continue
		}

		// Non-Docker resources (LXC, VM, host, node)
		sb.WriteString(fmt.Sprintf("## %s\n", mention.Name))
		sb.WriteString(fmt.Sprintf("Type: %s | Target: %s\n", mention.ResourceType, mention.TargetID))

		// Include guest identifier, but only emit control instructions for platforms
		// that are actually on the shared control surface.
		if (mention.ResourceType == "system-container" || mention.ResourceType == "vm") && mention.ResourceID != "" {
			sb.WriteString(fmt.Sprintf("VMID: %s\n", mention.ResourceID))
			if mention.SupportsControl {
				sb.WriteString(fmt.Sprintf("Shared guest control actions are available with guest_id=\"%s\" and action=\"start|stop|shutdown|restart\".\n", mention.ResourceID))
			} else {
				sb.WriteString("This guest is read-only in Pulse; shared write/control actions are unavailable for this resource.\n")
			}
		}

		if hasDiscovery && hint.mode == readRoutingTargetHost {
			// Command routing
			if discovery.Hostname != "" {
				sb.WriteString(fmt.Sprintf("target_host: \"%s\"\n", discovery.Hostname))
			} else {
				sb.WriteString(fmt.Sprintf("target_host: \"%s\"\n", mention.Name))
			}

			// Service info
			if discovery.ServiceType != "" {
				sb.WriteString(fmt.Sprintf("Service: %s", discovery.ServiceType))
				if discovery.ServiceName != "" {
					sb.WriteString(fmt.Sprintf(" (%s)", discovery.ServiceName))
				}
				sb.WriteString("\n")
			}
			if discovery.ServiceVersion != "" {
				sb.WriteString(fmt.Sprintf("Service version: %s\n", discovery.ServiceVersion))
			}
			if discovery.SuggestedURL != "" {
				sb.WriteString(fmt.Sprintf("Suggested web URL: %s\n", discovery.SuggestedURL))
			}

			// File paths - these are the verified paths to use
			if len(discovery.LogPaths) > 0 {
				// Separate file paths from commands for clarity
				var filePaths []string
				var commands []string
				for _, lp := range discovery.LogPaths {
					if strings.HasPrefix(lp, "/") {
						filePaths = append(filePaths, lp)
					} else {
						commands = append(commands, lp)
					}
				}
				if len(filePaths) > 0 {
					sb.WriteString(fmt.Sprintf("Log files (check these first): %v\n", filePaths))
				}
				if len(commands) > 0 {
					sb.WriteString(fmt.Sprintf("Log commands (alternative): %v\n", commands))
				}
			}
			if len(discovery.ConfigPaths) > 0 {
				sb.WriteString(fmt.Sprintf("Config paths: %v\n", discovery.ConfigPaths))
			}
			if len(discovery.DataPaths) > 0 {
				sb.WriteString(fmt.Sprintf("Data paths: %v\n", discovery.DataPaths))
			}

			// Ports
			if len(discovery.Ports) > 0 {
				var portStrs []string
				for _, p := range discovery.Ports {
					portStrs = append(portStrs, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
				}
				sb.WriteString(fmt.Sprintf("Ports: %v\n", portStrs))
			}

			// Docker bind mounts for containers/VMs running Docker
			if len(discovery.BindMounts) > 0 {
				sb.WriteString("Docker containers on this host:\n")
				containerMounts := make(map[string][]tools.DiscoveryMount)
				for _, m := range discovery.BindMounts {
					name := m.ContainerName
					if name == "" {
						name = "(container)"
					}
					containerMounts[name] = append(containerMounts[name], m)
				}
				for containerName, mounts := range containerMounts {
					sb.WriteString(fmt.Sprintf("  %s:\n", containerName))
					for _, m := range mounts {
						sb.WriteString(fmt.Sprintf("    %s → %s\n", m.Source, m.Destination))
					}
				}
				sb.WriteString("  To edit container files: use the left path (host path)\n")
			}
		} else {
			// No discovery - provide basic routing without suggesting discovery calls
			switch hint.mode {
			case readRoutingNativeResource, readRoutingQueryOnly:
				if hint.ref != "" {
					sb.WriteString(fmt.Sprintf("resource_id: \"%s\"\n", hint.ref))
				}
				if contextLine := hint.prefetchContext(); contextLine != "" {
					sb.WriteString(contextLine + "\n")
				}
			default:
				targetHost := firstNonEmptyTrimmed(hint.ref, mention.Name)
				if targetHost != "" {
					sb.WriteString(fmt.Sprintf("target_host: \"%s\"\n", targetHost))
				}
				if resourceRequiresReadOnlyGuidance(mention.ResourceType, mention.SupportsControl) {
					sb.WriteString("Capability context: this resource is read-only in Pulse; shared control and discovery routing are not valid for it.\n")
				} else {
					sb.WriteString("Resource location is already resolved; discovery is not required to identify this resource.\n")
				}
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}
