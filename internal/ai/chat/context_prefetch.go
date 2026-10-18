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
	Mentions []ResourceMention
	Summary  string // Formatted summary for AI consumption
}

// ContextPrefetcher proactively gathers context based on user message content
type ContextPrefetcher struct {
	readState unifiedresources.ReadState
}

// NewContextPrefetcher creates a new context prefetcher
func NewContextPrefetcher(readState unifiedresources.ReadState) *ContextPrefetcher {
	return &ContextPrefetcher{readState: readState}
}

func resourceMentionGovernance(resource *unifiedresources.Resource) (*unifiedresources.ResourcePolicy, string) {
	return unifiedresources.CanonicalGovernanceMetadata(resource)
}

func resourceSupportsControl(resourceType string, resource *unifiedresources.Resource) bool {
	switch tools.CanonicalToolResourceType(resourceType) {
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
	switch tools.CanonicalToolResourceType(resourceType) {
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

	return &PrefetchedContext{
		Mentions: mentions,
		Summary:  p.formatContextSummary(mentions),
	}
}

// mentionsIncludingHandoff merges the user's explicit @-mentions with resources
// handed off from a product surface (e.g. the drawer "Ask Assistant" action) so
// both flow through the same prefetch path. A drawer handoff anchors the turn on
// a resource but, without this, never delivered that resource's cloud-safe
// operational context (access command, config/data/log paths, ports) to the
// model — that context previously reached the model only via the @-mention path.
// HandoffResource and StructuredMention are field-identical; a handoff entry that
// duplicates an explicit mention (same type/id/name/node) is dropped.
func mentionsIncludingHandoff(mentions []StructuredMention, handoff []HandoffResource) []StructuredMention {
	if len(handoff) == 0 {
		return mentions
	}
	key := func(typ, id, name, node string) string {
		return strings.ToLower(strings.TrimSpace(typ) + "\x00" + strings.TrimSpace(id) + "\x00" + strings.TrimSpace(name) + "\x00" + strings.TrimSpace(node))
	}
	merged := make([]StructuredMention, 0, len(mentions)+len(handoff))
	seen := make(map[string]struct{}, len(mentions)+len(handoff))
	for _, m := range mentions {
		merged = append(merged, m)
		seen[key(m.Type, m.ID, m.Name, m.Node)] = struct{}{}
	}
	for _, r := range handoff {
		id := strings.TrimSpace(r.ID)
		name := strings.TrimSpace(r.Name)
		if id == "" && name == "" {
			continue
		}
		k := key(r.Type, r.ID, r.Name, r.Node)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		merged = append(merged, StructuredMention{
			ID:   id,
			Name: name,
			Type: strings.TrimSpace(r.Type),
			Node: strings.TrimSpace(r.Node),
		})
	}
	return merged
}

// resolveStructuredMentions converts frontend StructuredMention objects into ResourceMention
// objects with full routing info.
func (p *ContextPrefetcher) resolveStructuredMentions(structured []StructuredMention) []ResourceMention {
	var mentions []ResourceMention
	rs := p.readState

	for _, sm := range structured {
		if unifiedresources.IsUnsupportedLegacyResourceTypeAlias(sm.Type) {
			continue
		}
		var resolved unifiedresources.ResolvedResourceContext
		if strings.TrimSpace(sm.ID) != "" {
			resolved = unifiedresources.ResolveResourceReferenceContext(rs, sm.ID)
		} else {
			resolved = unifiedresources.ResolveResourceContext(rs, sm.Name)
		}
		resource := resolved.Resource
		if resource == nil || !resolved.Location.Found {
			continue
		}
		loc := resolved.Location
		resourceType := tools.CanonicalToolResourceType(loc.ResourceType)
		policy, summary := resourceMentionGovernance(resource)
		mention := ResourceMention{
			Name: loc.Name, ResourceType: resourceType,
			ResourceID: mentionResourceIDFromResolved(resourceType, resource),
			TargetID:   mentionTargetIDFromResolved(resourceType, resource),
			Adapter:    mentionAdapterFromResolved(resource), MatchedText: sm.Name,
			Policy: policy, AISafeSummary: summary, UnifiedResourceID: resource.ID,
			TargetHost: loc.TargetHost, SupportsControl: resourceSupportsControl(resourceType, resource),
		}
		switch resourceType {
		case "app-container":
			mention.TargetID, mention.ResourceID = resolvedAppContainerMentionCoordinates(resource)
			mention.DockerHostName, mention.DockerHostType = loc.DockerHostName, loc.DockerHostType
			mention.DockerHostVMID, mention.NodeName = loc.DockerHostVMID, loc.Node
			for _, container := range rs.DockerContainers() {
				if container == nil || container.ID() != resource.ID {
					continue
				}
				for _, mount := range container.Mounts() {
					if mount.Source != "" && mount.Destination != "" {
						mention.BindMounts = append(mention.BindMounts, MountInfo{Source: mount.Source, Destination: mount.Destination})
					}
				}
				break
			}
		case "storage":
			mention.ResourceID = resource.ID
			mention.TargetID = firstNonEmptyTrimmed(mention.TargetID, loc.TargetID, loc.TargetHost)
		case "agent":
			mention.ResourceID = firstNonEmptyTrimmed(mention.ResourceID, loc.TargetID, resource.ID)
			mention.TargetID = firstNonEmptyTrimmed(mention.TargetID, loc.TargetID)
		case "node":
			mention.ResourceID, mention.TargetID = loc.Name, loc.Node
		}
		mentions = append(mentions, mention)
	}

	return mentions
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
	if resource.Docker != nil && strings.TrimSpace(resource.Docker.HostSourceID) != "" {
		hostID = strings.TrimSpace(resource.Docker.HostSourceID)
	}
	containerID = appContainerProviderUID(*resource, hostID)
	return strings.TrimSpace(hostID), strings.TrimSpace(containerID)
}

func mentionResourceIDFromResolved(resourceType string, resource *unifiedresources.Resource) string {
	if resource == nil {
		return ""
	}

	switch tools.CanonicalToolResourceType(resourceType) {
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

	switch tools.CanonicalToolResourceType(resourceType) {
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

// formatContextSummary creates a formatted summary of the resolved mentions.
func (p *ContextPrefetcher) formatContextSummary(mentions []ResourceMention) string {
	if len(mentions) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("=== PULSE MONITORING DATA (AUTHORITATIVE) ===\n")
	sb.WriteString("This is verified data from Pulse monitoring sources. Canonical resource policy is enforced below.\n")
	sb.WriteString(unifiedresources.ResourcePolicyGovernedSummaryPreamble())
	sb.WriteString("\n\n")

	for _, mention := range mentions {
		if unifiedresources.ResourcePolicyRequiresGovernedSummary(mention.Policy) {
			// local_only never reaches here; it is withheld upstream.
			sb.WriteString(unifiedresources.FormatResourcePolicyGovernedSummary(mention.AISafeSummary, mention.Policy))
			continue
		}

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

			sb.WriteString(fmt.Sprintf("target_host: \"%s\"\n", firstNonEmptyTrimmed(mention.UnifiedResourceID, mention.TargetHost)))

			// Bind mounts - clarify these are on the LXC/VM filesystem
			if len(mention.BindMounts) > 0 {
				sb.WriteString(fmt.Sprintf("Bind mounts (paths on %s filesystem, NOT inside container):\n", mention.TargetHost))
				for _, m := range mention.BindMounts {
					sb.WriteString(fmt.Sprintf("  %s → %s\n", m.Source, m.Destination))
				}
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
				sb.WriteString(fmt.Sprintf("Shared guest control actions use pulse_control with type=\"resource\", resource_id=\"%s\" and action=\"start|stop|shutdown|restart\".\n", firstNonEmptyTrimmed(mention.UnifiedResourceID, mention.ResourceID)))
			} else {
				sb.WriteString("This guest is read-only in Pulse; shared write/control actions are unavailable for this resource.\n")
			}
		}

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
				sb.WriteString("Capability context: this resource is read-only in Pulse; shared control routing is not valid for it.\n")
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}
