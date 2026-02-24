package unifiedresources

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// SnapshotWithoutSources returns a copy of snapshot with source-owned slices
// removed from legacy snapshot ingestion.
func SnapshotWithoutSources(snapshot models.StateSnapshot, excludedSources []DataSource) models.StateSnapshot {
	if len(excludedSources) == 0 {
		return snapshot
	}

	out := snapshot
	for _, source := range excludedSources {
		switch normalizeDataSource(source) {
		case SourceProxmox:
			out.Nodes = nil
			out.VMs = nil
			out.Containers = nil
			out.Storage = nil
			out.CephClusters = nil
			out.PhysicalDisks = nil
		case SourceAgent:
			out.Hosts = nil
		case SourceDocker:
			out.DockerHosts = nil
		case SourcePBS:
			out.PBSInstances = nil
		case SourcePMG:
			out.PMGInstances = nil
		case SourceK8s:
			out.KubernetesClusters = nil
		}
	}

	return out
}

func normalizeDataSource(source DataSource) DataSource {
	normalized := strings.ToLower(strings.TrimSpace(string(source)))
	switch normalized {
	case "pve":
		return SourceProxmox
	case "k8s":
		return SourceK8s
	default:
		return DataSource(normalized)
	}
}
