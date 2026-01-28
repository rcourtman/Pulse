package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// ResourceMention represents a detected resource mention in a user message
type ResourceMention struct {
	Name         string
	ResourceType string // "vm", "lxc", "docker", "host"
	ResourceID   string
	HostID       string
	MatchedText  string // The actual text that matched
	// Docker-specific: bind mounts (source -> destination)
	BindMounts []MountInfo
	// Full routing chain (for Docker containers)
	DockerHostName string // Name of LXC/VM/host running Docker (e.g., "homepage-docker")
	DockerHostType string // "lxc", "vm", or "host"
	DockerHostVMID int    // VMID if DockerHost is an LXC/VM
	ProxmoxNode    string // Proxmox node name (e.g., "delly")
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
	stateProvider     StateProvider
	discoveryProvider tools.DiscoveryProvider
}

// NewContextPrefetcher creates a new context prefetcher
func NewContextPrefetcher(stateProvider StateProvider, discoveryProvider tools.DiscoveryProvider) *ContextPrefetcher {
	return &ContextPrefetcher{
		stateProvider:     stateProvider,
		discoveryProvider: discoveryProvider,
	}
}

// Prefetch analyzes a user message and proactively gathers relevant context
func (p *ContextPrefetcher) Prefetch(ctx context.Context, message string) *PrefetchedContext {
	log.Info().
		Bool("hasStateProvider", p.stateProvider != nil).
		Bool("hasDiscoveryProvider", p.discoveryProvider != nil).
		Msg("[ContextPrefetch] Starting prefetch")

	if p.stateProvider == nil {
		log.Warn().Msg("[ContextPrefetch] No state provider, cannot prefetch")
		return nil
	}

	state := p.stateProvider.GetState()
	log.Info().
		Int("vms", len(state.VMs)).
		Int("containers", len(state.Containers)).
		Int("dockerHosts", len(state.DockerHosts)).
		Msg("[ContextPrefetch] Got state for matching")

	mentions := p.extractResourceMentions(message, state)

	if len(mentions) == 0 {
		log.Info().Str("message", message[:min(50, len(message))]).Msg("[ContextPrefetch] No resource mentions found")
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
func (p *ContextPrefetcher) extractResourceMentions(message string, state models.StateSnapshot) []ResourceMention {
	messageLower := strings.ToLower(message)
	var mentions []ResourceMention
	seen := make(map[string]bool) // Deduplicate by name

	// Extract words from message (3+ chars) for partial matching
	messageWords := extractWords(messageLower)

	// Check VMs
	for _, vm := range state.VMs {
		nameLower := strings.ToLower(vm.Name)
		if nameLower != "" && len(nameLower) >= 3 && matchesResource(messageLower, messageWords, nameLower) {
			if !seen[nameLower] {
				seen[nameLower] = true
				mentions = append(mentions, ResourceMention{
					Name:         vm.Name,
					ResourceType: "vm",
					ResourceID:   fmt.Sprintf("%d", vm.VMID),
					HostID:       vm.Node,
					MatchedText:  vm.Name,
				})
			}
		}
	}

	// Check LXC containers (in StateSnapshot, LXCs are in Containers with Type "lxc")
	for _, container := range state.Containers {
		if container.Type != "lxc" {
			continue
		}
		nameLower := strings.ToLower(container.Name)
		if nameLower != "" && len(nameLower) >= 3 && matchesResource(messageLower, messageWords, nameLower) {
			if !seen[nameLower] {
				seen[nameLower] = true
				mentions = append(mentions, ResourceMention{
					Name:         container.Name,
					ResourceType: "lxc",
					ResourceID:   fmt.Sprintf("%d", container.VMID),
					HostID:       container.Node,
					MatchedText:  container.Name,
				})
			}
		}
	}

	// Check Docker containers - use ResolveResource for authoritative location
	for _, dockerHost := range state.DockerHosts {
		for _, container := range dockerHost.Containers {
			nameLower := strings.ToLower(container.Name)
			if nameLower != "" && len(nameLower) >= 3 && matchesResource(messageLower, messageWords, nameLower) {
				if !seen[nameLower] {
					seen[nameLower] = true

					// Capture bind mounts
					var mounts []MountInfo
					for _, m := range container.Mounts {
						if m.Source != "" && m.Destination != "" {
							mounts = append(mounts, MountInfo{
								Source:      m.Source,
								Destination: m.Destination,
							})
						}
					}

					// Use the authoritative ResolveResource function
					loc := state.ResolveResource(container.Name)

					mentions = append(mentions, ResourceMention{
						Name:           container.Name,
						ResourceType:   "docker",
						ResourceID:     container.ID,
						HostID:         dockerHost.ID,
						MatchedText:    container.Name,
						BindMounts:     mounts,
						DockerHostName: loc.DockerHostName,
						DockerHostType: loc.DockerHostType,
						DockerHostVMID: loc.DockerHostVMID,
						ProxmoxNode:    loc.Node,
						TargetHost:     loc.TargetHost,
					})
				}
			}
		}
	}

	// Check Proxmox nodes
	for _, node := range state.Nodes {
		nameLower := strings.ToLower(node.Name)
		if nameLower != "" && len(nameLower) >= 3 && matchesResource(messageLower, messageWords, nameLower) {
			if !seen[nameLower] {
				seen[nameLower] = true
				loc := state.ResolveResource(node.Name)
				mentions = append(mentions, ResourceMention{
					Name:         node.Name,
					ResourceType: "node",
					ResourceID:   node.Name,
					HostID:       node.Name,
					MatchedText:  node.Name,
					TargetHost:   loc.TargetHost,
				})
			}
		}
	}

	// Check generic Hosts (Windows/Linux via Pulse Unified Agent)
	for _, host := range state.Hosts {
		nameLower := strings.ToLower(host.Hostname)
		if nameLower != "" && len(nameLower) >= 3 && matchesResource(messageLower, messageWords, nameLower) {
			if !seen[nameLower] {
				seen[nameLower] = true
				loc := state.ResolveResource(host.Hostname)
				mentions = append(mentions, ResourceMention{
					Name:         host.Hostname,
					ResourceType: "host",
					ResourceID:   host.ID,
					HostID:       host.ID,
					MatchedText:  host.Hostname,
					TargetHost:   loc.TargetHost,
				})
			}
		}
	}

	// Check Kubernetes clusters, pods, and deployments
	for _, cluster := range state.KubernetesClusters {
		clusterLower := strings.ToLower(cluster.Name)
		if clusterLower != "" && len(clusterLower) >= 3 && matchesResource(messageLower, messageWords, clusterLower) {
			if !seen[clusterLower] {
				seen[clusterLower] = true
				loc := state.ResolveResource(cluster.Name)
				mentions = append(mentions, ResourceMention{
					Name:         cluster.Name,
					ResourceType: "k8s_cluster",
					ResourceID:   cluster.ID,
					HostID:       cluster.ID,
					MatchedText:  cluster.Name,
					TargetHost:   loc.TargetHost,
				})
			}
		}

		// Check pods
		for _, pod := range cluster.Pods {
			podLower := strings.ToLower(pod.Name)
			if podLower != "" && len(podLower) >= 3 && matchesResource(messageLower, messageWords, podLower) {
				if !seen[podLower] {
					seen[podLower] = true
					loc := state.ResolveResource(pod.Name)
					mentions = append(mentions, ResourceMention{
						Name:         pod.Name,
						ResourceType: "k8s_pod",
						ResourceID:   pod.Name,
						HostID:       cluster.ID,
						MatchedText:  pod.Name,
						TargetHost:   loc.TargetHost,
					})
				}
			}
		}

		// Check deployments
		for _, deploy := range cluster.Deployments {
			deployLower := strings.ToLower(deploy.Name)
			if deployLower != "" && len(deployLower) >= 3 && matchesResource(messageLower, messageWords, deployLower) {
				if !seen[deployLower] {
					seen[deployLower] = true
					loc := state.ResolveResource(deploy.Name)
					mentions = append(mentions, ResourceMention{
						Name:         deploy.Name,
						ResourceType: "k8s_deployment",
						ResourceID:   deploy.Name,
						HostID:       cluster.ID,
						MatchedText:  deploy.Name,
						TargetHost:   loc.TargetHost,
					})
				}
			}
		}
	}

	return mentions
}

// getOrTriggerDiscovery gets existing discovery or triggers a new one
func (p *ContextPrefetcher) getOrTriggerDiscovery(ctx context.Context, mention ResourceMention) (*tools.ResourceDiscoveryInfo, error) {
	// First try to get existing discovery
	discovery, err := p.discoveryProvider.GetDiscoveryByResource(mention.ResourceType, mention.HostID, mention.ResourceID)
	if err == nil && discovery != nil {
		log.Debug().
			Str("resource", mention.Name).
			Msg("[ContextPrefetch] Using cached discovery")
		return discovery, nil
	}

	// Trigger discovery if not found (for VMs and LXCs)
	if mention.ResourceType == "vm" || mention.ResourceType == "lxc" || mention.ResourceType == "docker" {
		log.Debug().
			Str("resource", mention.Name).
			Str("type", mention.ResourceType).
			Msg("[ContextPrefetch] Triggering discovery")

		discovery, err = p.discoveryProvider.TriggerDiscovery(ctx, mention.ResourceType, mention.HostID, mention.ResourceID)
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
		key := fmt.Sprintf("%s:%s:%s", d.ResourceType, d.HostID, d.ResourceID)
		discoveryMap[key] = d
	}

	for _, mention := range mentions {
		key := fmt.Sprintf("%s:%s:%s", mention.ResourceType, mention.HostID, mention.ResourceID)
		discovery, hasDiscovery := discoveryMap[key]

		// Docker containers get special treatment - show the full routing chain
		if mention.ResourceType == "docker" {
			sb.WriteString(fmt.Sprintf("## %s (Docker container)\n", mention.Name))

			// Show the full routing chain unambiguously
			if mention.DockerHostType == "lxc" {
				sb.WriteString(fmt.Sprintf("Location: Docker on \"%s\" (LXC %d) on Proxmox node \"%s\"\n",
					mention.DockerHostName, mention.DockerHostVMID, mention.ProxmoxNode))
			} else if mention.DockerHostType == "vm" {
				sb.WriteString(fmt.Sprintf("Location: Docker on \"%s\" (VM %d) on Proxmox node \"%s\"\n",
					mention.DockerHostName, mention.DockerHostVMID, mention.ProxmoxNode))
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
			}

			sb.WriteString("\n")
			continue
		}

		// Non-Docker resources (LXC, VM, host)
		sb.WriteString(fmt.Sprintf("## %s\n", mention.Name))
		sb.WriteString(fmt.Sprintf("Type: %s | Host: %s\n", mention.ResourceType, mention.HostID))

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

			// Docker bind mounts for LXCs/VMs running Docker
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
			// No discovery - provide basic routing
			sb.WriteString(fmt.Sprintf("target_host: \"%s\"\n", mention.Name))
			sb.WriteString("Discovery data not available. Run pulse_discovery to get details.\n")
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
