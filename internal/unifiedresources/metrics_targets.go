package unifiedresources

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// BuildMetricsTarget resolves the history/metrics query target for a canonical
// resource using its source-specific IDs.
func BuildMetricsTarget(resource Resource, sourceTargets []SourceTarget) *MetricsTarget {
	if len(sourceTargets) == 0 {
		return nil
	}

	bySource := make(map[DataSource]SourceTarget, len(sourceTargets))
	for _, target := range sourceTargets {
		target.SourceID = strings.TrimSpace(target.SourceID)
		if target.SourceID == "" {
			continue
		}
		bySource[target.Source] = target
	}

	switch CanonicalResourceType(resource.Type) {
	case ResourceTypeAgent:
		// The agent source must win over platform sources (Proxmox/VMware):
		// real-mode agent metrics are written under the host agent ID
		// (monitor_agents.go writes "agent"/host.ID, the same ID ingestHost
		// registers for SourceAgent), so a merged node+agent resource must
		// advertise that key. Preferring the platform source ID here would
		// point store readers (reports, drawer history) at an "agent"-typed
		// key nothing ever writes.
		if st, ok := bySource[SourceAgent]; ok {
			return &MetricsTarget{ResourceType: "agent", ResourceID: st.SourceID}
		}
		if st, ok := bySource[SourceProxmox]; ok {
			// Resource identity is agent-shaped, but PVE history is written
			// under the node store family. Keep storage coordinates explicit.
			return &MetricsTarget{ResourceType: "node", ResourceID: st.SourceID}
		}
		if st, ok := bySource[SourceVMware]; ok {
			return &MetricsTarget{ResourceType: "agent", ResourceID: st.SourceID}
		}
		if st, ok := bySource[SourceTrueNAS]; ok {
			if resourceID := canonicalAgentMetricID(st.SourceID); resourceID != "" {
				return &MetricsTarget{ResourceType: "agent", ResourceID: resourceID}
			}
		}
		if st, ok := bySource[SourceDocker]; ok {
			return &MetricsTarget{ResourceType: "docker-host", ResourceID: st.SourceID}
		}
	case ResourceTypeVM:
		if st, ok := bySource[SourceProxmox]; ok {
			return &MetricsTarget{ResourceType: "vm", ResourceID: st.SourceID}
		}
		if st, ok := bySource[SourceVMware]; ok {
			return &MetricsTarget{ResourceType: "vm", ResourceID: st.SourceID}
		}
		if st, ok := bySource[SourceTrueNAS]; ok {
			if resourceID := canonicalTrueNASVMMetricID(st.SourceID); resourceID != "" {
				return &MetricsTarget{ResourceType: "vm", ResourceID: resourceID}
			}
		}
		if strings.EqualFold(strings.TrimSpace(resource.Technology), "libvirt") {
			if st, ok := bySource[SourceAgent]; ok {
				return &MetricsTarget{ResourceType: "vm", ResourceID: st.SourceID}
			}
		}
	case ResourceTypeSystemContainer:
		if st, ok := bySource[SourceProxmox]; ok {
			return &MetricsTarget{ResourceType: "system-container", ResourceID: st.SourceID}
		}
	case ResourceTypeAppContainer:
		if st, ok := bySource[SourceDocker]; ok {
			// Container history is written per Docker host
			// (DockerContainerMetricID), under the same host-scoped key
			// the registry ingests the container by. A container ID is
			// only what a host's report says it is, so the bare ID must
			// not address a series: two hosts reporting one ID would
			// share it.
			return &MetricsTarget{ResourceType: "app-container", ResourceID: st.SourceID}
		}
		if st, ok := bySource[SourceTrueNAS]; ok {
			if resourceID := canonicalAppContainerMetricID(st.SourceID); resourceID != "" {
				return &MetricsTarget{ResourceType: "app-container", ResourceID: resourceID}
			}
		}
	case ResourceTypeStorage, ResourceTypeCeph:
		if st, ok := bySource[SourceProxmox]; ok {
			return &MetricsTarget{ResourceType: "storage", ResourceID: st.SourceID}
		}
		if st, ok := bySource[SourceVMware]; ok {
			return &MetricsTarget{ResourceType: "storage", ResourceID: st.SourceID}
		}
		if st, ok := bySource[SourceTrueNAS]; ok {
			return &MetricsTarget{ResourceType: "storage", ResourceID: st.SourceID}
		}
		if st, ok := bySource[SourcePBS]; ok {
			return &MetricsTarget{ResourceType: "storage", ResourceID: st.SourceID}
		}
		if st, ok := bySource[SourceAgent]; ok {
			return &MetricsTarget{ResourceType: "storage", ResourceID: st.SourceID}
		}
	case ResourceTypePhysicalDisk:
		// The agent source must win over platform sources for physical
		// disks: SMART/temperature metrics are written by the host agent
		// (HostSMARTDiskMetricID in monitor_agents.go). Proxmox API does
		// not expose detailed SMART data, so preferring its source ID here
		// would point store readers at a key nothing writes and disk
		// metrics would appear empty (GitHub issue #1487). The agent's
		// source ID is host-scoped (HostSMARTDiskSourceID), while its
		// metrics key is the disk's serial or WWN, so a disk with hardware
		// identity resolves to that. Only an identity-less disk falls back
		// to its source ID, which keeps the shape the writer uses.
		if st, ok := bySource[SourceAgent]; ok {
			if resourceID := PhysicalDiskMetaMetricID(resource.PhysicalDisk, st.SourceID); resourceID != "" {
				return &MetricsTarget{ResourceType: "disk", ResourceID: resourceID}
			}
		}
		if st, ok := bySource[SourceProxmox]; ok {
			if resourceID := PhysicalDiskMetaMetricID(resource.PhysicalDisk, st.SourceID); resourceID != "" {
				return &MetricsTarget{ResourceType: "disk", ResourceID: resourceID}
			}
		}
		if st, ok := bySource[SourceTrueNAS]; ok {
			if resourceID := PhysicalDiskMetaMetricID(resource.PhysicalDisk, st.SourceID); resourceID != "" {
				return &MetricsTarget{ResourceType: "disk", ResourceID: resourceID}
			}
		}
	case ResourceTypePod:
		if st, ok := bySource[SourceK8s]; ok {
			if resourceID := CanonicalKubernetesPodMetricID(st.SourceID); resourceID != "" {
				return &MetricsTarget{ResourceType: string(ResourceTypePod), ResourceID: resourceID}
			}
		}
	case ResourceTypeK8sCluster:
		if st, ok := bySource[SourceK8s]; ok {
			return &MetricsTarget{ResourceType: string(ResourceTypeK8sCluster), ResourceID: st.SourceID}
		}
	case ResourceTypeK8sNode:
		if st, ok := bySource[SourceK8s]; ok {
			return &MetricsTarget{ResourceType: string(ResourceTypeK8sNode), ResourceID: st.SourceID}
		}
	case ResourceTypeK8sDeployment:
		if st, ok := bySource[SourceK8s]; ok {
			return &MetricsTarget{ResourceType: string(ResourceTypeK8sDeployment), ResourceID: st.SourceID}
		}
	case ResourceTypePBS:
		if st, ok := bySource[SourcePBS]; ok {
			return &MetricsTarget{ResourceType: "agent", ResourceID: st.SourceID}
		}
	case ResourceTypePMG:
		if st, ok := bySource[SourcePMG]; ok {
			return &MetricsTarget{ResourceType: "agent", ResourceID: st.SourceID}
		}
	}

	return nil
}

// DockerContainerMetricID returns the key a Docker container's metric history
// is recorded and read under: the host-scoped source ID the registry ingests
// the container by (dockerContainerSourceID), which is also its metrics
// target. Container IDs come from each host's own report, so history is
// scoped to the reporting host like the container's resource and alerts are.
func DockerContainerMetricID(hostID, containerID string) string {
	return dockerContainerSourceID(models.DockerHost{ID: hostID}, models.DockerContainer{ID: containerID})
}

func CanonicalKubernetesPodMetricID(sourceID string) string {
	trimmed := strings.TrimSpace(sourceID)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "k8s:") {
		return trimmed
	}
	return "k8s:" + trimmed
}

func canonicalAppContainerMetricID(sourceID string) string {
	trimmed := strings.TrimSpace(sourceID)
	trimmed = strings.TrimPrefix(trimmed, "app:")
	return strings.TrimSpace(trimmed)
}

func canonicalTrueNASVMMetricID(sourceID string) string {
	trimmed := strings.TrimSpace(sourceID)
	trimmed = strings.TrimPrefix(trimmed, "vm:")
	return strings.TrimSpace(trimmed)
}

func canonicalAgentMetricID(sourceID string) string {
	trimmed := strings.TrimSpace(sourceID)
	trimmed = strings.TrimPrefix(trimmed, "system:")
	trimmed = strings.TrimPrefix(trimmed, "agent:")
	return strings.TrimSpace(trimmed)
}
