package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// ResourceMention represents a detected resource mention in a user message
type ResourceMention struct {
	Name         string
	ResourceType string // "vm", "system-container", "app-container", "agent", "node"
	ResourceID   string
	TargetID     string
	MatchedText  string // The actual text that matched
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

// Prefetch analyzes a user message and proactively gathers relevant context.
// When structuredMentions are provided (from the frontend @ autocomplete), they are used
// directly instead of fuzzy-matching resource names from the message text.
func (p *ContextPrefetcher) Prefetch(ctx context.Context, message string, structuredMentions []StructuredMention) *PrefetchedContext {
	log.Info().
		Bool("hasReadState", p.readState != nil).
		Bool("hasDiscoveryProvider", p.discoveryProvider != nil).
		Int("structured_mentions", len(structuredMentions)).
		Msg("[ContextPrefetch] Starting prefetch")

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

	var mentions []ResourceMention

	if len(structuredMentions) > 0 {
		// Structured path: frontend already resolved the resources via autocomplete.
		// Convert to ResourceMention using ResolveResource for full routing info.
		mentions = p.resolveStructuredMentions(structuredMentions)
		log.Info().
			Int("structured_input", len(structuredMentions)).
			Int("resolved", len(mentions)).
			Msg("[ContextPrefetch] Resolved structured mentions")
	} else {
		// Fallback: fuzzy-match resource names from the message text.
		// Used when the user types @name manually without selecting from autocomplete.
		mentions = p.extractResourceMentions(message)
	}

	if len(mentions) == 0 {
		log.Info().Str("message", message[:min(50, len(message))]).Msg("[ContextPrefetch] No resource mentions found")

		// Check for explicit @ mentions that didn't resolve to any resource.
		// If the user tagged something with @ but we couldn't find it, tell the AI
		// so it doesn't waste tool calls searching for it.
		unresolvedMentions := extractExplicitAtMentions(message)
		if len(unresolvedMentions) > 0 {
			log.Info().
				Strs("unresolved", unresolvedMentions).
				Msg("[ContextPrefetch] Found unresolved @ mentions")

			var sb strings.Builder
			sb.WriteString("=== RESOURCE LOOKUP RESULT ===\n")
			for _, name := range unresolvedMentions {
				sb.WriteString(fmt.Sprintf("'%s' was NOT found in Pulse monitoring. It is not a tracked VM, container, Docker container, or host.\n", name))
			}
			sb.WriteString("Do NOT use pulse_discovery to search for these resources — they are not in the system.\n")
			sb.WriteString("Instead: use pulse_control directly if you know the host where the service runs, or ask the user for the location.\n")

			return &PrefetchedContext{
				Summary: sb.String(),
			}
		}

		return nil
	}

	log.Info().
		Int("mentions_found", len(mentions)).
		Msg("[ContextPrefetch] Found resource mentions in message")

	// Gather discovery data for each mention
	var discoveries []*tools.ResourceDiscoveryInfo
	if p.discoveryProvider != nil {
		for _, mention := range mentions {
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

// extractResourceMentions finds resource names mentioned in the message
// It supports two modes:
// 1. Explicit @ mentions: @homepage, @influxdb (high confidence, exact match)
// 2. Fuzzy name matching: "homepage" matches "homepage-docker" (fallback)
func (p *ContextPrefetcher) extractResourceMentions(message string) []ResourceMention {
	rs := p.readState
	messageLower := strings.ToLower(message)
	var mentions []ResourceMention
	seen := make(map[string]bool) // Deduplicate by name

	// Extract words from message (3+ chars) for partial matching
	messageWords := extractWords(messageLower)

	// Check VMs (via ReadState)
	if rs != nil {
		for _, vm := range rs.VMs() {
			if vm == nil {
				continue
			}
			name := vm.Name()
			nameLower := strings.ToLower(name)
			if nameLower != "" && len(nameLower) >= 3 && matchesResource(messageLower, messageWords, nameLower) {
				if !seen[nameLower] {
					seen[nameLower] = true
					mentions = append(mentions, ResourceMention{
						Name:         name,
						ResourceType: "vm",
						ResourceID:   fmt.Sprintf("%d", vm.VMID()),
						TargetID:     vm.Node(),
						MatchedText:  name,
					})
				}
			}
		}
	}

	// Check system containers (LXC via ReadState)
	if rs != nil {
		for _, ct := range rs.Containers() {
			if ct == nil {
				continue
			}
			name := ct.Name()
			nameLower := strings.ToLower(name)
			if nameLower != "" && len(nameLower) >= 3 && matchesResource(messageLower, messageWords, nameLower) {
				if !seen[nameLower] {
					seen[nameLower] = true
					mentions = append(mentions, ResourceMention{
						Name:         name,
						ResourceType: "system-container",
						ResourceID:   fmt.Sprintf("%d", ct.VMID()),
						TargetID:     ct.Node(),
						MatchedText:  name,
					})
				}
			}
		}
	}

	// Check Docker containers (via ReadState) - use ResolveResource for authoritative location
	if rs != nil {
		// Build a map from unified resource ID → source host ID so container
		// ParentID (unified) can be resolved to the original models.DockerHost.ID
		// that discovery and other subsystems expect.
		dockerHostSourceIDs := make(map[string]string)
		for _, dh := range rs.DockerHosts() {
			if dh == nil {
				continue
			}
			dockerHostSourceIDs[dh.ID()] = dh.HostSourceID()
		}

		for _, container := range rs.DockerContainers() {
			if container == nil {
				continue
			}
			name := container.Name()
			nameLower := strings.ToLower(name)
			if nameLower != "" && len(nameLower) >= 3 && matchesResource(messageLower, messageWords, nameLower) {
				if !seen[nameLower] {
					seen[nameLower] = true

					// Capture bind mounts
					var mounts []MountInfo
					for _, m := range container.Mounts() {
						if m.Source != "" && m.Destination != "" {
							mounts = append(mounts, MountInfo{
								Source:      m.Source,
								Destination: m.Destination,
							})
						}
					}

					// Resolve parent ID to original source host ID
					hostID := dockerHostSourceIDs[container.ParentID()]
					if hostID == "" {
						hostID = container.ParentID()
					}

					// Use the authoritative ResolveResource function
					loc := unifiedresources.ResolveResource(rs, name)

					mentions = append(mentions, ResourceMention{
						Name:           name,
						ResourceType:   "app-container",
						ResourceID:     container.ContainerID(),
						TargetID:       hostID,
						MatchedText:    name,
						BindMounts:     mounts,
						DockerHostName: loc.DockerHostName,
						DockerHostType: loc.DockerHostType,
						DockerHostVMID: loc.DockerHostVMID,
						NodeName:       loc.Node,
						TargetHost:     loc.TargetHost,
					})
				}
			}
		}
	}

	// Check Proxmox nodes (via ReadState)
	if rs != nil {
		for _, node := range rs.Nodes() {
			if node == nil {
				continue
			}
			name := node.Name()
			nameLower := strings.ToLower(name)
			if nameLower != "" && len(nameLower) >= 3 && matchesResource(messageLower, messageWords, nameLower) {
				if !seen[nameLower] {
					seen[nameLower] = true
					loc := unifiedresources.ResolveResource(rs, name)
					mentions = append(mentions, ResourceMention{
						Name:         name,
						ResourceType: "node",
						ResourceID:   name,
						TargetID:     name,
						MatchedText:  name,
						TargetHost:   loc.TargetHost,
					})
				}
			}
		}
	}

	// Check generic Hosts (Windows/Linux via Pulse Unified Agent, via ReadState)
	if rs != nil {
		for _, host := range rs.Hosts() {
			if host == nil {
				continue
			}
			hostname := host.Hostname()
			nameLower := strings.ToLower(hostname)
			if nameLower != "" && len(nameLower) >= 3 && matchesResource(messageLower, messageWords, nameLower) {
				if !seen[nameLower] {
					seen[nameLower] = true
					loc := unifiedresources.ResolveResource(rs, hostname)
					// Use AgentID which maps to the original models.Host.ID
					hostID := host.AgentID()
					mentions = append(mentions, ResourceMention{
						Name:         hostname,
						ResourceType: "agent",
						ResourceID:   hostID,
						TargetID:     hostID,
						MatchedText:  hostname,
						TargetHost:   loc.TargetHost,
					})
				}
			}
		}
	}

	// Check Kubernetes clusters, pods, and deployments (via ReadState)
	if rs != nil {
		// Build map from unified resource ID → source cluster ID so pods/deployments
		// can resolve TargetID to the original models.KubernetesCluster.ID.
		k8sClusterSourceIDs := make(map[string]string)
		for _, cluster := range rs.K8sClusters() {
			if cluster == nil {
				continue
			}
			k8sClusterSourceIDs[cluster.ID()] = cluster.ClusterID()

			clusterName := cluster.Name()
			clusterLower := strings.ToLower(clusterName)
			if clusterLower != "" && len(clusterLower) >= 3 && matchesResource(messageLower, messageWords, clusterLower) {
				if !seen[clusterLower] {
					seen[clusterLower] = true
					loc := unifiedresources.ResolveResource(rs, clusterName)
					clusterSourceID := cluster.ClusterID()
					mentions = append(mentions, ResourceMention{
						Name:         clusterName,
						ResourceType: "k8s-cluster",
						ResourceID:   clusterSourceID,
						TargetID:     clusterSourceID,
						MatchedText:  clusterName,
						TargetHost:   loc.TargetHost,
					})
				}
			}
		}

		// Check pods (flat list, linked to cluster via ParentID)
		for _, pod := range rs.Pods() {
			if pod == nil {
				continue
			}
			podName := pod.Name()
			podLower := strings.ToLower(podName)
			if podLower != "" && len(podLower) >= 3 && matchesResource(messageLower, messageWords, podLower) {
				if !seen[podLower] {
					seen[podLower] = true
					loc := unifiedresources.ResolveResource(rs, podName)
					hostID := k8sClusterSourceIDs[pod.ParentID()]
					if hostID == "" {
						hostID = pod.ParentID()
					}
					mentions = append(mentions, ResourceMention{
						Name:         podName,
						ResourceType: "k8s-pod",
						ResourceID:   podName,
						TargetID:     hostID,
						MatchedText:  podName,
						TargetHost:   loc.TargetHost,
					})
				}
			}
		}

		// Check deployments (flat list, linked to cluster via ParentID)
		for _, deploy := range rs.K8sDeployments() {
			if deploy == nil {
				continue
			}
			deployName := deploy.Name()
			deployLower := strings.ToLower(deployName)
			if deployLower != "" && len(deployLower) >= 3 && matchesResource(messageLower, messageWords, deployLower) {
				if !seen[deployLower] {
					seen[deployLower] = true
					loc := unifiedresources.ResolveResource(rs, deployName)
					hostID := k8sClusterSourceIDs[deploy.ParentID()]
					if hostID == "" {
						hostID = deploy.ParentID()
					}
					mentions = append(mentions, ResourceMention{
						Name:         deployName,
						ResourceType: "k8s-deployment",
						ResourceID:   deployName,
						TargetID:     hostID,
						MatchedText:  deployName,
						TargetHost:   loc.TargetHost,
					})
				}
			}
		}
	}

	return mentions
}

// resolveStructuredMentions converts frontend StructuredMention objects into ResourceMention
// objects with full routing info. This is the preferred path — no fuzzy matching needed.
func (p *ContextPrefetcher) resolveStructuredMentions(structured []StructuredMention) []ResourceMention {
	var mentions []ResourceMention
	rs := p.readState

	for _, sm := range structured {
		// Parse the structured ID to extract resource details.
		// Frontend ID formats: "vm:node:vmid", "system-container:node:vmid",
		// "docker:hostId:containerId", "agent:id", "node:instance:name"
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
		resourceType := canonicalMentionResourceType(sm.Type)

		// Use ResolveResource for full routing info (target_host, Docker chain, etc.)
		loc := unifiedresources.ResolveResource(rs, sm.Name)

		switch resourceType {
		case "vm":
			vmID := ""
			node := sm.Node
			if len(parts) >= 3 {
				node = parts[1]
				vmID = parts[2]
			}
			mentions = append(mentions, ResourceMention{
				Name:         sm.Name,
				ResourceType: "vm",
				ResourceID:   vmID,
				TargetID:     node,
				MatchedText:  sm.Name,
				TargetHost:   loc.TargetHost,
			})

		case "system-container":
			vmID := ""
			node := sm.Node
			if len(parts) >= 3 {
				node = parts[1]
				vmID = parts[2]
			}
			mentions = append(mentions, ResourceMention{
				Name:         sm.Name,
				ResourceType: "system-container",
				ResourceID:   vmID,
				TargetID:     node,
				MatchedText:  sm.Name,
				TargetHost:   loc.TargetHost,
			})

		case "app-container":
			hostID, containerID := parseStructuredDockerMentionID(sm.ID, rs)

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
				Name:           sm.Name,
				ResourceType:   "app-container",
				ResourceID:     containerID,
				TargetID:       hostID,
				MatchedText:    sm.Name,
				BindMounts:     mounts,
				DockerHostName: loc.DockerHostName,
				DockerHostType: loc.DockerHostType,
				DockerHostVMID: loc.DockerHostVMID,
				NodeName:       loc.Node,
				TargetHost:     loc.TargetHost,
			})

		case "node":
			mentions = append(mentions, ResourceMention{
				Name:         sm.Name,
				ResourceType: "node",
				ResourceID:   sm.Name,
				TargetID:     sm.Name,
				MatchedText:  sm.Name,
				TargetHost:   loc.TargetHost,
			})

		case "agent":
			hostID := ""
			if len(parts) >= 2 {
				hostID = strings.Join(parts[1:], ":")
			}
			mentions = append(mentions, ResourceMention{
				Name:         sm.Name,
				ResourceType: "agent",
				ResourceID:   hostID,
				TargetID:     hostID,
				MatchedText:  sm.Name,
				TargetHost:   loc.TargetHost,
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
				Name:         sm.Name,
				ResourceType: resourceType,
				ResourceID:   sm.ID,
				TargetID:     sm.Node,
				MatchedText:  sm.Name,
				TargetHost:   loc.TargetHost,
			})
		}
	}

	return mentions
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

func canonicalMentionResourceType(raw string) string {
	resourceType := strings.ToLower(strings.TrimSpace(raw))
	switch resourceType {
	case "docker":
		return "app-container"
	case "docker-host":
		return "docker-host"
	case "k8s-cluster":
		return "k8s-cluster"
	case "k8s-pod":
		return "k8s-pod"
	case "k8s-deployment":
		return "k8s-deployment"
	default:
		return resourceType
	}
}

func discoveryResourceType(resourceType string) string {
	switch canonicalMentionResourceType(resourceType) {
	case "app-container":
		// Discovery backend still stores Docker containers under "docker".
		return "docker"
	default:
		return canonicalMentionResourceType(resourceType)
	}
}

func canonicalDiscoveryTargetID(discovery *tools.ResourceDiscoveryInfo) string {
	if discovery == nil {
		return ""
	}
	targetID := strings.TrimSpace(discovery.TargetID)
	return targetID
}

// getOrTriggerDiscovery gets existing discovery or triggers a new one
func (p *ContextPrefetcher) getOrTriggerDiscovery(ctx context.Context, mention ResourceMention) (*tools.ResourceDiscoveryInfo, error) {
	discoveryType := discoveryResourceType(mention.ResourceType)
	canonicalType := canonicalMentionResourceType(mention.ResourceType)

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

		discovery, err = p.discoveryProvider.TriggerDiscovery(ctx, discoveryType, mention.TargetID, mention.ResourceID)
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
	sb.WriteString("This is verified data from Pulse agents. Use these exact paths and hostnames.\n\n")

	// Create a map for quick discovery lookup
	discoveryMap := make(map[string]*tools.ResourceDiscoveryInfo)
	for _, d := range discoveries {
		discoveryTargetID := canonicalDiscoveryTargetID(d)
		key := fmt.Sprintf("%s:%s:%s", canonicalMentionResourceType(d.ResourceType), discoveryTargetID, d.ResourceID)
		discoveryMap[key] = d
	}

	for _, mention := range mentions {
		key := fmt.Sprintf("%s:%s:%s", canonicalMentionResourceType(mention.ResourceType), mention.TargetID, mention.ResourceID)
		discovery, hasDiscovery := discoveryMap[key]

		// Docker containers get special treatment - show the full routing chain
		if canonicalMentionResourceType(mention.ResourceType) == "app-container" {
			sb.WriteString(fmt.Sprintf("## %s (Docker container)\n", mention.Name))

			// Show the full routing chain unambiguously
			if mention.DockerHostType == "system-container" {
				sb.WriteString(fmt.Sprintf("Location: Docker on \"%s\" (container %d) on node \"%s\"\n",
					mention.DockerHostName, mention.DockerHostVMID, mention.NodeName))
			} else if mention.DockerHostType == "vm" {
				sb.WriteString(fmt.Sprintf("Location: Docker on \"%s\" (VM %d) on node \"%s\"\n",
					mention.DockerHostName, mention.DockerHostVMID, mention.NodeName))
			} else {
				sb.WriteString(fmt.Sprintf("Location: Docker on host \"%s\"\n", mention.DockerHostName))
			}

			// THE target_host - this is the critical routing info
			sb.WriteString(fmt.Sprintf(">>> target_host: \"%s\" <<<\n", mention.TargetHost))

			// Bind mounts - clarify these are on the LXC/VM filesystem
			if len(mention.BindMounts) > 0 {
				sb.WriteString(fmt.Sprintf("Bind mounts (paths on %s filesystem, NOT inside container):\n", mention.TargetHost))
				for _, m := range mention.BindMounts {
					sb.WriteString(fmt.Sprintf("  %s → %s\n", m.Source, m.Destination))
				}
			}

			// Add discovery info if available
			if hasDiscovery {
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
				sb.WriteString("You have the resource location and target_host. Proceed directly with pulse_docker or pulse_control — do NOT call pulse_discovery.\n")
			}

			sb.WriteString("\n")
			continue
		}

		// Non-Docker resources (LXC, VM, host, node)
		sb.WriteString(fmt.Sprintf("## %s\n", mention.Name))
		sb.WriteString(fmt.Sprintf("Type: %s | Target: %s\n", mention.ResourceType, mention.TargetID))

		// Include VMID for VMs and system containers — the AI needs this for pulse_control guest operations
		if (mention.ResourceType == "system-container" || mention.ResourceType == "vm") && mention.ResourceID != "" {
			sb.WriteString(fmt.Sprintf("VMID: %s\n", mention.ResourceID))
			sb.WriteString(fmt.Sprintf("To control this guest, use: pulse_control type=\"guest\", guest_id=\"%s\", action=\"start|stop|shutdown|restart\"\n", mention.ResourceID))
		}

		if hasDiscovery {
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
			sb.WriteString(fmt.Sprintf("target_host: \"%s\"\n", mention.Name))
			if mention.ResourceType == "system-container" || mention.ResourceType == "vm" {
				sb.WriteString("Proceed directly with pulse_control — do NOT call pulse_discovery.\n")
			} else {
				sb.WriteString("Proceed directly with pulse_control — do NOT call pulse_discovery.\n")
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// extractWords extracts words (3+ characters) from a message for matching
func extractWords(message string) []string {
	// Split on common delimiters and filter short words
	var words []string
	current := ""
	for _, r := range message {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			current += string(r)
		} else {
			if len(current) >= 3 {
				words = append(words, current)
			}
			current = ""
		}
	}
	if len(current) >= 3 {
		words = append(words, current)
	}
	return words
}

// extractExplicitAtMentions finds @name patterns in a message that look like
// explicit resource mentions typed by the user. Returns the names without the @ prefix.
func extractExplicitAtMentions(message string) []string {
	var mentions []string
	seen := make(map[string]bool)

	for i := 0; i < len(message); i++ {
		if message[i] != '@' {
			continue
		}
		// @ must be at start or preceded by whitespace
		if i > 0 && message[i-1] != ' ' && message[i-1] != '\t' && message[i-1] != '\n' {
			continue
		}
		// Extract the word after @
		start := i + 1
		end := start
		for end < len(message) && message[end] != ' ' && message[end] != '\t' && message[end] != '\n' {
			end++
		}
		if end > start {
			name := message[start:end]
			nameLower := strings.ToLower(name)
			if len(nameLower) >= 2 && !seen[nameLower] {
				seen[nameLower] = true
				mentions = append(mentions, name)
			}
		}
	}
	return mentions
}

// matchesResource checks if a resource name matches the message using fuzzy matching
// Handles cases like "homepage" matching "homepage-docker"
func matchesResource(messageLower string, messageWords []string, resourceName string) bool {
	// Direct containment: message contains full resource name
	if strings.Contains(messageLower, resourceName) {
		return true
	}

	// Check if any message word is a significant prefix of the resource name
	// e.g., "homepage" matches "homepage-docker"
	for _, word := range messageWords {
		// Word must be at least 4 chars to avoid false positives
		if len(word) >= 4 {
			// Check if resource name starts with this word
			if strings.HasPrefix(resourceName, word) {
				return true
			}
			// Check if any hyphenated part matches
			parts := strings.Split(resourceName, "-")
			for _, part := range parts {
				if part == word {
					return true
				}
			}
		}
	}

	return false
}
