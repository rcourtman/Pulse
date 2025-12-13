package ai

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/resources"
	"github.com/rs/zerolog/log"
)

// ResourceProvider provides access to the unified resource store.
type ResourceProvider interface {
	GetAll() []resources.Resource
	GetInfrastructure() []resources.Resource
	GetWorkloads() []resources.Resource
	GetByType(t resources.ResourceType) []resources.Resource
	GetStats() resources.StoreStats
	
	// Cross-platform query methods
	GetTopByCPU(limit int, types []resources.ResourceType) []resources.Resource
	GetTopByMemory(limit int, types []resources.ResourceType) []resources.Resource
	GetTopByDisk(limit int, types []resources.ResourceType) []resources.Resource
	GetRelated(resourceID string) map[string][]resources.Resource
	GetResourceSummary() resources.ResourceSummary
	
	// AI Routing support
	FindContainerHost(containerNameOrID string) string
}

// SetResourceProvider sets the resource provider for unified infrastructure context.
func (s *Service) SetResourceProvider(rp ResourceProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resourceProvider = rp
}

// buildUnifiedResourceContext creates AI context from the unified resource model.
// This provides a cleaner, deduplicated view of infrastructure.
func (s *Service) buildUnifiedResourceContext() string {
	s.mu.RLock()
	rp := s.resourceProvider
	s.mu.RUnlock()

	if rp == nil {
		return ""
	}

	var sections []string
	stats := rp.GetStats()

	// Header with summary
	sections = append(sections, "## Unified Infrastructure View")
	sections = append(sections, fmt.Sprintf("Total resources: %d (Infrastructure: %d, Workloads: %d)",
		stats.TotalResources, stats.ByType[resources.ResourceTypeNode]+stats.ByType[resources.ResourceTypeHost]+stats.ByType[resources.ResourceTypeDockerHost],
		stats.ByType[resources.ResourceTypeVM]+stats.ByType[resources.ResourceTypeContainer]+stats.ByType[resources.ResourceTypeDockerContainer]))

	// Build agent lookup
	agentsByHostname := make(map[string]bool)
	if s.agentServer != nil {
		for _, agent := range s.agentServer.GetConnectedAgents() {
			agentsByHostname[strings.ToLower(agent.Hostname)] = true
		}
	}

	// Infrastructure resources (nodes, hosts, docker hosts)
	infrastructure := rp.GetInfrastructure()
	if len(infrastructure) > 0 {
		sections = append(sections, "\n### Infrastructure (Nodes & Hosts)")
		sections = append(sections, "These are the physical/virtual machines that host workloads.")

		// Group by platform
		byPlatform := make(map[resources.PlatformType][]resources.Resource)
		for _, r := range infrastructure {
			byPlatform[r.PlatformType] = append(byPlatform[r.PlatformType], r)
		}

		// Proxmox nodes
		if nodes, ok := byPlatform[resources.PlatformProxmoxPVE]; ok && len(nodes) > 0 {
			sections = append(sections, "\n**Proxmox VE Nodes:**")
			for _, node := range nodes {
				hasAgent := agentsByHostname[strings.ToLower(node.Name)]
				agentStatus := "NO AGENT"
				if hasAgent {
					agentStatus = "HAS AGENT ✓"
				}

				// Build cluster info
				clusterInfo := ""
				if node.ClusterID != "" {
					clusterInfo = fmt.Sprintf(" [cluster: %s]", node.ClusterID)
				}

				// Get CPU/Memory metrics
				metrics := ""
				if node.CPU != nil && node.Memory != nil {
					metrics = fmt.Sprintf(" - CPU: %.1f%%, Mem: %.1f%%", node.CPUPercent(), node.MemoryPercent())
				}

				sections = append(sections, fmt.Sprintf("- **%s** (%s)%s%s [%s]",
					node.EffectiveDisplayName(), agentStatus, clusterInfo, metrics, node.Status))
			}
		}

		// Standalone hosts
		if hosts, ok := byPlatform[resources.PlatformHostAgent]; ok && len(hosts) > 0 {
			sections = append(sections, "\n**Standalone Hosts (via Host Agent):**")
			for _, host := range hosts {
				// Get IPs from identity
				ips := ""
				if host.Identity != nil && len(host.Identity.IPs) > 0 {
					ips = " - " + strings.Join(host.Identity.IPs, ", ")
				}

				// Get metrics
				metrics := ""
				if host.CPU != nil && host.Memory != nil {
					metrics = fmt.Sprintf(", CPU: %.1f%%, Mem: %.1f%%", host.CPUPercent(), host.MemoryPercent())
				}

				sections = append(sections, fmt.Sprintf("- **%s**%s%s [%s]",
					host.EffectiveDisplayName(), ips, metrics, host.Status))
			}
		}

		// Docker hosts
		if dhosts, ok := byPlatform[resources.PlatformDocker]; ok && len(dhosts) > 0 {
			sections = append(sections, "\n**Docker/Podman Hosts:**")
			for _, dh := range dhosts {
				if dh.Type != resources.ResourceTypeDockerHost {
					continue
				}

				// Count containers for this host
				allWorkloads := rp.GetWorkloads()
				containerCount := 0
				runningCount := 0
				for _, w := range allWorkloads {
					if w.ParentID == dh.ID {
						containerCount++
						if w.Status == resources.StatusRunning {
							runningCount++
						}
					}
				}

				sections = append(sections, fmt.Sprintf("- **%s** (%d/%d containers running) [%s]",
					dh.EffectiveDisplayName(), runningCount, containerCount, dh.Status))
			}
		}
	}

	// Workloads (VMs, containers)
	workloads := rp.GetWorkloads()
	if len(workloads) > 0 {
		sections = append(sections, "\n### Workloads (VMs & Containers)")

		// Group by parent for better organization
		byParent := make(map[string][]resources.Resource)
		noParent := []resources.Resource{}
		for _, w := range workloads {
			if w.ParentID != "" {
				byParent[w.ParentID] = append(byParent[w.ParentID], w)
			} else {
				noParent = append(noParent, w)
			}
		}

		// Get all infrastructure to map parent IDs to names
		infraMap := make(map[string]resources.Resource)
		for _, r := range infrastructure {
			infraMap[r.ID] = r
		}

		// Show workloads grouped by parent
		for parentID, children := range byParent {
			parentName := parentID
			if parent, ok := infraMap[parentID]; ok {
				parentName = parent.EffectiveDisplayName()
			}

			sections = append(sections, fmt.Sprintf("\n**On %s:**", parentName))
			for _, w := range children {
				typeLabel := string(w.Type)
				if w.Type == resources.ResourceTypeVM {
					typeLabel = "VM"
				} else if w.Type == resources.ResourceTypeContainer {
					typeLabel = "LXC"
				} else if w.Type == resources.ResourceTypeDockerContainer {
					typeLabel = "Docker"
				}

				// Get VMID from platform data if available
				vmidInfo := ""
				if w.PlatformID != "" && w.Type != resources.ResourceTypeDockerContainer {
					vmidInfo = fmt.Sprintf(" %s", w.PlatformID)
				}

				// Get IPs
				ips := ""
				if w.Identity != nil && len(w.Identity.IPs) > 0 {
					ips = " - " + strings.Join(w.Identity.IPs[:min(2, len(w.Identity.IPs))], ", ")
				}

				sections = append(sections, fmt.Sprintf("  - **%s** (%s%s)%s [%s]",
					w.EffectiveDisplayName(), typeLabel, vmidInfo, ips, w.Status))
			}
		}

		// Show orphaned workloads
		if len(noParent) > 0 {
			sections = append(sections, "\n**Other workloads:**")
			for _, w := range noParent {
				sections = append(sections, fmt.Sprintf("  - **%s** (%s) [%s]",
					w.EffectiveDisplayName(), w.Type, w.Status))
			}
		}
	}

	// Resources with alerts
	allResources := rp.GetAll()
	var alertResources []resources.Resource
	for _, r := range allResources {
		if len(r.Alerts) > 0 {
			alertResources = append(alertResources, r)
		}
	}
	if len(alertResources) > 0 {
		sections = append(sections, "\n### Resources with Active Alerts")
		for _, r := range alertResources {
			for _, alert := range r.Alerts {
				sections = append(sections, fmt.Sprintf("- **%s**: %s (%s)",
					r.EffectiveDisplayName(), alert.Message, alert.Level))
			}
		}
	}

	// Add resource summary for cross-platform analysis
	summary := rp.GetResourceSummary()
	if summary.TotalResources > 0 {
		sections = append(sections, "\n### Infrastructure Summary")
		sections = append(sections, fmt.Sprintf("- Status: %d healthy, %d degraded, %d offline",
			summary.Healthy, summary.Degraded, summary.Offline))
		if summary.WithAlerts > 0 {
			sections = append(sections, fmt.Sprintf("- Resources with alerts: %d", summary.WithAlerts))
		}
		
		// Show average resource usage by type
		if len(summary.ByType) > 0 {
			sections = append(sections, "- Average utilization by type:")
			for t, ts := range summary.ByType {
				if ts.Count > 0 && (ts.AvgCPUPercent > 0 || ts.AvgMemoryPercent > 0) {
					sections = append(sections, fmt.Sprintf("  - %s (%d): CPU %.1f%%, Memory %.1f%%",
						t, ts.Count, ts.AvgCPUPercent, ts.AvgMemoryPercent))
				}
			}
		}
	}

	// Top resource consumers (helps answer "what's using the most CPU/memory")
	topCPU := rp.GetTopByCPU(3, nil)
	if len(topCPU) > 0 {
		sections = append(sections, "\n### Top CPU Consumers")
		for i, r := range topCPU {
			sections = append(sections, fmt.Sprintf("%d. **%s** (%s): %.1f%%",
				i+1, r.EffectiveDisplayName(), r.Type, r.CPUPercent()))
		}
	}

	topMem := rp.GetTopByMemory(3, nil)
	if len(topMem) > 0 {
		sections = append(sections, "\n### Top Memory Consumers")
		for i, r := range topMem {
			sections = append(sections, fmt.Sprintf("%d. **%s** (%s): %.1f%%",
				i+1, r.EffectiveDisplayName(), r.Type, r.MemoryPercent()))
		}
	}

	topDisk := rp.GetTopByDisk(3, nil)
	if len(topDisk) > 0 {
		sections = append(sections, "\n### Top Disk Usage")
		for i, r := range topDisk {
			sections = append(sections, fmt.Sprintf("%d. **%s** (%s): %.1f%%",
				i+1, r.EffectiveDisplayName(), r.Type, r.DiskPercent()))
		}
	}

	if len(sections) == 0 {
		return ""
	}

	result := "\n\n" + strings.Join(sections, "\n")

	// Limit context size
	const maxContextSize = 50000
	if len(result) > maxContextSize {
		log.Warn().
			Int("original_size", len(result)).
			Int("max_size", maxContextSize).
			Msg("Unified resource context truncated")
		result = result[:maxContextSize] + "\n\n[... Context truncated ...]"
	}

	log.Debug().Int("unified_resource_context_size", len(result)).Msg("Built unified resource context")
	return result
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
