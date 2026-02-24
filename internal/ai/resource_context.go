package ai

import (
	"fmt"
	"sort"
	"strings"

	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// UnifiedResourceProvider exposes unified-resource-native context APIs.
type UnifiedResourceProvider interface {
	GetAll() []unifiedresources.Resource
	GetInfrastructure() []unifiedresources.Resource
	GetWorkloads() []unifiedresources.Resource
	GetByType(t unifiedresources.ResourceType) []unifiedresources.Resource
	GetStats() unifiedresources.ResourceStats
	GetTopByCPU(limit int, types []unifiedresources.ResourceType) []unifiedresources.Resource
	GetTopByMemory(limit int, types []unifiedresources.ResourceType) []unifiedresources.Resource
	GetTopByDisk(limit int, types []unifiedresources.ResourceType) []unifiedresources.Resource
	GetRelated(resourceID string) map[string][]unifiedresources.Resource
	FindContainerHost(containerNameOrID string) string
}

// SetUnifiedResourceProvider sets the unified-resource-native provider.
// It also forwards the provider to the patrol service so seed context
// can read from the unified resource registry.
func (s *Service) SetUnifiedResourceProvider(urp UnifiedResourceProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unifiedResourceProvider = urp

	if s.patrolService != nil {
		s.patrolService.SetUnifiedResourceProvider(urp)
	}
}

// buildUnifiedResourceContext creates AI context from the unified resource model.
// This provides a cleaner, deduplicated view of infrastructure.
func (s *Service) buildUnifiedResourceContext() string {
	s.mu.RLock()
	urp := s.unifiedResourceProvider
	ap := s.alertProvider
	agentServer := s.agentServer
	s.mu.RUnlock()

	if urp != nil {
		var sections []string
		stats := urp.GetStats()

		infraCount := stats.ByType[unifiedresources.ResourceTypeHost] +
			stats.ByType[unifiedresources.ResourceTypeK8sCluster] +
			stats.ByType[unifiedresources.ResourceTypeK8sNode]
		workloadCount := stats.ByType[unifiedresources.ResourceTypeVM] +
			stats.ByType[unifiedresources.ResourceTypeSystemContainer] +
			stats.ByType[unifiedresources.ResourceTypeAppContainer] +
			stats.ByType[unifiedresources.ResourceTypePod] +
			stats.ByType[unifiedresources.ResourceTypeK8sDeployment]

		sections = append(sections, "## Unified Infrastructure View")
		sections = append(sections, fmt.Sprintf("Total resources: %d (Infrastructure: %d, Workloads: %d)",
			stats.Total, infraCount, workloadCount))

		agentsByHostname := make(map[string]bool)
		if agentServer != nil {
			for _, agent := range agentServer.GetConnectedAgents() {
				hostname := strings.ToLower(strings.TrimSpace(agent.Hostname))
				if hostname != "" {
					agentsByHostname[hostname] = true
				}
			}
		}

		hasConnectedAgent := func(r unifiedresources.Resource) bool {
			candidates := []string{r.Name}
			if r.Agent != nil {
				candidates = append(candidates, r.Agent.Hostname)
			}
			candidates = append(candidates, r.Identity.Hostnames...)
			for _, candidate := range candidates {
				hostname := strings.ToLower(strings.TrimSpace(candidate))
				if hostname == "" {
					continue
				}
				if agentsByHostname[hostname] {
					return true
				}
			}
			return false
		}

		clusterName := func(r unifiedresources.Resource) string {
			if strings.TrimSpace(r.Identity.ClusterName) != "" {
				return strings.TrimSpace(r.Identity.ClusterName)
			}
			if r.Proxmox != nil && strings.TrimSpace(r.Proxmox.ClusterName) != "" {
				return strings.TrimSpace(r.Proxmox.ClusterName)
			}
			if r.Kubernetes != nil && strings.TrimSpace(r.Kubernetes.ClusterName) != "" {
				return strings.TrimSpace(r.Kubernetes.ClusterName)
			}
			return ""
		}

		infrastructure := urp.GetInfrastructure()
		workloads := urp.GetWorkloads()
		allResources := urp.GetAll()
		byResourceID := make(map[string]unifiedresources.Resource, len(allResources))
		for _, resource := range allResources {
			byResourceID[resource.ID] = resource
		}

		if len(infrastructure) > 0 {
			sections = append(sections, "\n### Infrastructure (Nodes & Hosts)")
			sections = append(sections, "These are the physical/virtual machines that host workloads.")

			proxmoxNodes := make([]unifiedresources.Resource, 0)
			standaloneHosts := make([]unifiedresources.Resource, 0)
			trueNASHosts := make([]unifiedresources.Resource, 0)
			dockerHosts := make([]unifiedresources.Resource, 0)
			k8sClusters := make([]unifiedresources.Resource, 0)
			k8sNodes := make([]unifiedresources.Resource, 0)

			for _, resource := range infrastructure {
				switch {
				case resource.Type == unifiedresources.ResourceTypeK8sCluster:
					k8sClusters = append(k8sClusters, resource)
				case resource.Type == unifiedresources.ResourceTypeK8sNode:
					k8sNodes = append(k8sNodes, resource)
				case resource.Proxmox != nil:
					proxmoxNodes = append(proxmoxNodes, resource)
				case resource.Docker != nil:
					dockerHosts = append(dockerHosts, resource)
				case hasResourceTag(resource, "truenas"):
					trueNASHosts = append(trueNASHosts, resource)
				case resource.Agent != nil:
					standaloneHosts = append(standaloneHosts, resource)
				default:
					standaloneHosts = append(standaloneHosts, resource)
				}
			}

			sortResources := func(resources []unifiedresources.Resource) {
				sort.Slice(resources, func(i, j int) bool {
					return strings.ToLower(unifiedResourceDisplayName(resources[i])) < strings.ToLower(unifiedResourceDisplayName(resources[j]))
				})
			}

			sortResources(proxmoxNodes)
			sortResources(standaloneHosts)
			sortResources(trueNASHosts)
			sortResources(dockerHosts)
			sortResources(k8sClusters)
			sortResources(k8sNodes)

			if len(proxmoxNodes) > 0 {
				sections = append(sections, "\n**Proxmox VE Nodes:**")
				for _, node := range proxmoxNodes {
					agentStatus := "NO AGENT"
					if hasConnectedAgent(node) {
						agentStatus = "HAS AGENT ✓"
					}

					clusterInfo := ""
					if name := clusterName(node); name != "" {
						clusterInfo = fmt.Sprintf(" [cluster: %s]", name)
					}

					metrics := ""
					cpuPercent := 0.0
					memPercent := 0.0
					if node.Metrics != nil {
						cpuPercent = unifiedMetricPercent(node.Metrics.CPU)
						memPercent = unifiedMetricPercent(node.Metrics.Memory)
					}
					if cpuPercent > 0 || memPercent > 0 {
						metrics = fmt.Sprintf(" - CPU: %.1f%%, Mem: %.1f%%", cpuPercent, memPercent)
					}

					sections = append(sections, fmt.Sprintf("- **%s** (%s)%s%s [%s]",
						unifiedResourceDisplayName(node), agentStatus, clusterInfo, metrics, node.Status))
				}
			}

			if len(standaloneHosts) > 0 {
				sections = append(sections, "\n**Standalone Hosts (via Host Agent):**")
				for _, host := range standaloneHosts {
					ips := ""
					if len(host.Identity.IPAddresses) > 0 {
						ips = " - " + strings.Join(host.Identity.IPAddresses, ", ")
					}

					metrics := ""
					cpuPercent := 0.0
					memPercent := 0.0
					if host.Metrics != nil {
						cpuPercent = unifiedMetricPercent(host.Metrics.CPU)
						memPercent = unifiedMetricPercent(host.Metrics.Memory)
					}
					if cpuPercent > 0 || memPercent > 0 {
						metrics = fmt.Sprintf(", CPU: %.1f%%, Mem: %.1f%%", cpuPercent, memPercent)
					}

					sections = append(sections, fmt.Sprintf("- **%s**%s%s [%s]",
						unifiedResourceDisplayName(host), ips, metrics, host.Status))
				}
			}

			if len(trueNASHosts) > 0 {
				sections = append(sections, "\n**TrueNAS Systems:**")
				for _, host := range trueNASHosts {
					metrics := ""
					diskPercent := 0.0
					if host.Metrics != nil {
						diskPercent = unifiedMetricPercent(host.Metrics.Disk)
					}
					if diskPercent > 0 {
						metrics = fmt.Sprintf(", Disk: %.1f%%", diskPercent)
					}

					sections = append(sections, fmt.Sprintf("- **%s**%s [%s]",
						unifiedResourceDisplayName(host), metrics, host.Status))
				}
			}

			if len(dockerHosts) > 0 {
				sections = append(sections, "\n**Docker/Podman Hosts:**")
				for _, host := range dockerHosts {
					containerCount := 0
					runningCount := 0
					for _, workload := range workloads {
						if workload.ParentID == nil || *workload.ParentID != host.ID {
							continue
						}
						if workload.Type != unifiedresources.ResourceTypeAppContainer {
							continue
						}
						containerCount++
						if workload.Status == unifiedresources.StatusOnline {
							runningCount++
						}
					}

					sections = append(sections, fmt.Sprintf("- **%s** (%d/%d containers running) [%s]",
						unifiedResourceDisplayName(host), runningCount, containerCount, host.Status))
				}
			}

			if len(k8sClusters) > 0 || len(k8sNodes) > 0 {
				sections = append(sections, "\n**Kubernetes:**")
				for _, cluster := range k8sClusters {
					nodeCount := 0
					for _, node := range k8sNodes {
						if node.ParentID != nil && *node.ParentID == cluster.ID {
							nodeCount++
						}
					}

					clusterInfo := ""
					if name := clusterName(cluster); name != "" {
						clusterInfo = fmt.Sprintf(" [cluster: %s]", name)
					}
					sections = append(sections, fmt.Sprintf("- **%s** (Cluster%s, %d nodes) [%s]",
						unifiedResourceDisplayName(cluster), clusterInfo, nodeCount, cluster.Status))
				}

				for _, node := range k8sNodes {
					agentStatus := "NO AGENT"
					if hasConnectedAgent(node) {
						agentStatus = "HAS AGENT ✓"
					}

					clusterInfo := ""
					if name := clusterName(node); name != "" {
						clusterInfo = fmt.Sprintf(" [cluster: %s]", name)
					}

					metrics := ""
					cpuPercent := 0.0
					memPercent := 0.0
					if node.Metrics != nil {
						cpuPercent = unifiedMetricPercent(node.Metrics.CPU)
						memPercent = unifiedMetricPercent(node.Metrics.Memory)
					}
					if cpuPercent > 0 || memPercent > 0 {
						metrics = fmt.Sprintf(" - CPU: %.1f%%, Mem: %.1f%%", cpuPercent, memPercent)
					}

					sections = append(sections, fmt.Sprintf("- **%s** (Node, %s)%s%s [%s]",
						unifiedResourceDisplayName(node), agentStatus, clusterInfo, metrics, node.Status))
				}
			}
		}

		if len(workloads) > 0 {
			sections = append(sections, "\n### Workloads (VMs & Containers)")

			byParent := make(map[string][]unifiedresources.Resource)
			noParent := make([]unifiedresources.Resource, 0)
			for _, workload := range workloads {
				if workload.ParentID != nil && strings.TrimSpace(*workload.ParentID) != "" {
					byParent[*workload.ParentID] = append(byParent[*workload.ParentID], workload)
					continue
				}
				noParent = append(noParent, workload)
			}

			infraMap := make(map[string]unifiedresources.Resource, len(infrastructure))
			for _, resource := range infrastructure {
				infraMap[resource.ID] = resource
			}

			parentIDs := make([]string, 0, len(byParent))
			for parentID := range byParent {
				parentIDs = append(parentIDs, parentID)
			}
			sort.Strings(parentIDs)

			for _, parentID := range parentIDs {
				parentName := parentID
				if parent, ok := infraMap[parentID]; ok {
					parentName = unifiedResourceDisplayName(parent)
				}

				sections = append(sections, fmt.Sprintf("\n**On %s:**", parentName))
				children := byParent[parentID]
				sort.Slice(children, func(i, j int) bool {
					return strings.ToLower(unifiedResourceDisplayName(children[i])) < strings.ToLower(unifiedResourceDisplayName(children[j]))
				})

				for _, workload := range children {
					typeLabel := string(workload.Type)
					switch workload.Type {
					case unifiedresources.ResourceTypeVM:
						typeLabel = "VM"
					case unifiedresources.ResourceTypeSystemContainer:
						typeLabel = "Container"
					case unifiedresources.ResourceTypeAppContainer:
						typeLabel = "Docker"
					}

					vmidInfo := ""
					if workload.Proxmox != nil && workload.Proxmox.VMID > 0 && workload.Type != unifiedresources.ResourceTypeAppContainer {
						vmidInfo = fmt.Sprintf(" %d", workload.Proxmox.VMID)
					}

					ips := ""
					if len(workload.Identity.IPAddresses) > 0 {
						ips = " - " + strings.Join(workload.Identity.IPAddresses[:min(2, len(workload.Identity.IPAddresses))], ", ")
					}

					sections = append(sections, fmt.Sprintf("  - **%s** (%s%s)%s [%s]",
						unifiedResourceDisplayName(workload), typeLabel, vmidInfo, ips, workload.Status))
				}
			}

			if len(noParent) > 0 {
				sort.Slice(noParent, func(i, j int) bool {
					return strings.ToLower(unifiedResourceDisplayName(noParent[i])) < strings.ToLower(unifiedResourceDisplayName(noParent[j]))
				})
				sections = append(sections, "\n**Other workloads:**")
				for _, workload := range noParent {
					sections = append(sections, fmt.Sprintf("  - **%s** (%s) [%s]",
						unifiedResourceDisplayName(workload), workload.Type, workload.Status))
				}
			}
		}

		activeAlerts := make([]AlertInfo, 0)
		if ap != nil {
			activeAlerts = ap.GetActiveAlerts()
		}

		if len(activeAlerts) > 0 {
			sections = append(sections, "\n### Resources with Active Alerts")
			for _, alert := range activeAlerts {
				displayName := strings.TrimSpace(alert.ResourceName)
				if resourceID := strings.TrimSpace(alert.ResourceID); resourceID != "" {
					if resource, ok := byResourceID[resourceID]; ok {
						displayName = unifiedResourceDisplayName(resource)
					} else if displayName == "" {
						displayName = resourceID
					}
				}
				if displayName == "" {
					displayName = "unknown-resource"
				}

				sections = append(sections, fmt.Sprintf("- **%s**: %s (%s)",
					displayName, alert.Message, alert.Level))
			}
		}

		if stats.Total > 0 {
			sections = append(sections, "\n### Infrastructure Summary")

			healthy := stats.ByStatus[unifiedresources.StatusOnline]
			degraded := stats.ByStatus[unifiedresources.StatusWarning]
			offline := stats.ByStatus[unifiedresources.StatusOffline] + stats.ByStatus[unifiedresources.StatusUnknown]
			sections = append(sections, fmt.Sprintf("- Status: %d healthy, %d degraded, %d offline",
				healthy, degraded, offline))

			withAlerts := make(map[string]struct{})
			for _, alert := range activeAlerts {
				if resourceID := strings.TrimSpace(alert.ResourceID); resourceID != "" {
					withAlerts[resourceID] = struct{}{}
				}
			}
			if len(withAlerts) > 0 {
				sections = append(sections, fmt.Sprintf("- Resources with alerts: %d", len(withAlerts)))
			}

			type metricSummary struct {
				count     int
				cpuSum    float64
				memorySum float64
			}

			byTypeSummary := make(map[unifiedresources.ResourceType]metricSummary)
			for _, resource := range allResources {
				summary := byTypeSummary[resource.Type]
				summary.count++
				if resource.Metrics != nil {
					summary.cpuSum += unifiedMetricPercent(resource.Metrics.CPU)
					summary.memorySum += unifiedMetricPercent(resource.Metrics.Memory)
				}
				byTypeSummary[resource.Type] = summary
			}

			if len(byTypeSummary) > 0 {
				sections = append(sections, "- Average utilization by type:")
				typeKeys := make([]unifiedresources.ResourceType, 0, len(byTypeSummary))
				for t := range byTypeSummary {
					typeKeys = append(typeKeys, t)
				}
				sort.Slice(typeKeys, func(i, j int) bool {
					return typeKeys[i] < typeKeys[j]
				})

				for _, t := range typeKeys {
					summary := byTypeSummary[t]
					if summary.count == 0 {
						continue
					}
					avgCPU := summary.cpuSum / float64(summary.count)
					avgMemory := summary.memorySum / float64(summary.count)
					if avgCPU > 0 || avgMemory > 0 {
						sections = append(sections, fmt.Sprintf("  - %s (%d): CPU %.1f%%, Memory %.1f%%",
							t, summary.count, avgCPU, avgMemory))
					}
				}
			}
		}

		topCPU := urp.GetTopByCPU(3, nil)
		if len(topCPU) > 0 {
			sections = append(sections, "\n### Top CPU Consumers")
			for i, resource := range topCPU {
				cpuPercent := 0.0
				if resource.Metrics != nil {
					cpuPercent = unifiedMetricPercent(resource.Metrics.CPU)
				}
				sections = append(sections, fmt.Sprintf("%d. **%s** (%s): %.1f%%",
					i+1, unifiedResourceDisplayName(resource), resource.Type, cpuPercent))
			}
		}

		topMem := urp.GetTopByMemory(3, nil)
		if len(topMem) > 0 {
			sections = append(sections, "\n### Top Memory Consumers")
			for i, resource := range topMem {
				memPercent := 0.0
				if resource.Metrics != nil {
					memPercent = unifiedMetricPercent(resource.Metrics.Memory)
				}
				sections = append(sections, fmt.Sprintf("%d. **%s** (%s): %.1f%%",
					i+1, unifiedResourceDisplayName(resource), resource.Type, memPercent))
			}
		}

		topDisk := urp.GetTopByDisk(3, nil)
		if len(topDisk) > 0 {
			sections = append(sections, "\n### Top Disk Usage")
			for i, resource := range topDisk {
				diskPercent := 0.0
				if resource.Metrics != nil {
					diskPercent = unifiedMetricPercent(resource.Metrics.Disk)
				}
				sections = append(sections, fmt.Sprintf("%d. **%s** (%s): %.1f%%",
					i+1, unifiedResourceDisplayName(resource), resource.Type, diskPercent))
			}
		}

		result := "\n\n" + strings.Join(sections, "\n")

		const maxContextSize = 50000
		if len(result) > maxContextSize {
			log.Warn().
				Int("original_size", len(result)).
				Int("max_size", maxContextSize).
				Msg("Unified resource context truncated")
			result = result[:maxContextSize] + "\n\n[... Context truncated ...]"
		}

		log.Debug().Int("unified_resource_context_size", len(result)).Msg("built unified resource context")
		return result
	}

	return ""
}

func unifiedResourceDisplayName(r unifiedresources.Resource) string {
	if strings.TrimSpace(r.Name) != "" {
		return strings.TrimSpace(r.Name)
	}
	return strings.TrimSpace(r.ID)
}

func unifiedMetricPercent(m *unifiedresources.MetricValue) float64 {
	if m == nil {
		return 0
	}
	if m.Percent > 0 {
		return m.Percent
	}
	if m.Value > 0 {
		return m.Value
	}
	if m.Used != nil && m.Total != nil && *m.Total > 0 {
		return (float64(*m.Used) / float64(*m.Total)) * 100
	}
	return 0
}

func hasResourceTag(resource unifiedresources.Resource, tag string) bool {
	for _, t := range resource.Tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
