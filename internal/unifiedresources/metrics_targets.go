package unifiedresources

// BuildMetricsTarget resolves the history/metrics query target for a canonical
// resource using its source-specific IDs.
func BuildMetricsTarget(resource Resource, sourceTargets []SourceTarget) *MetricsTarget {
	if len(sourceTargets) == 0 {
		return nil
	}

	bySource := make(map[DataSource]SourceTarget, len(sourceTargets))
	for _, target := range sourceTargets {
		bySource[target.Source] = target
	}

	switch CanonicalResourceType(resource.Type) {
	case ResourceTypeAgent:
		if st, ok := bySource[SourceProxmox]; ok {
			return &MetricsTarget{ResourceType: "node", ResourceID: st.SourceID}
		}
		if st, ok := bySource[SourceAgent]; ok {
			return &MetricsTarget{ResourceType: "agent", ResourceID: st.SourceID}
		}
		if st, ok := bySource[SourceDocker]; ok {
			return &MetricsTarget{ResourceType: "docker-host", ResourceID: st.SourceID}
		}
	case ResourceTypeVM:
		if st, ok := bySource[SourceProxmox]; ok {
			return &MetricsTarget{ResourceType: "vm", ResourceID: st.SourceID}
		}
	case ResourceTypeSystemContainer:
		if st, ok := bySource[SourceProxmox]; ok {
			return &MetricsTarget{ResourceType: "system-container", ResourceID: st.SourceID}
		}
	case ResourceTypeAppContainer:
		if st, ok := bySource[SourceDocker]; ok {
			return &MetricsTarget{ResourceType: "app-container", ResourceID: st.SourceID}
		}
	case ResourceTypeStorage, ResourceTypeCeph:
		if st, ok := bySource[SourceProxmox]; ok {
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
		if st, ok := bySource[SourceProxmox]; ok {
			if resourceID := PhysicalDiskMetaMetricID(resource.PhysicalDisk, st.SourceID); resourceID != "" {
				return &MetricsTarget{ResourceType: "disk", ResourceID: resourceID}
			}
		}
		if st, ok := bySource[SourceAgent]; ok {
			if resourceID := PhysicalDiskMetaMetricID(resource.PhysicalDisk, st.SourceID); resourceID != "" {
				return &MetricsTarget{ResourceType: "disk", ResourceID: resourceID}
			}
		}
	case ResourceTypePod:
		if st, ok := bySource[SourceK8s]; ok {
			return &MetricsTarget{ResourceType: string(ResourceTypePod), ResourceID: st.SourceID}
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
			return &MetricsTarget{ResourceType: "node", ResourceID: st.SourceID}
		}
	case ResourceTypePMG:
		if st, ok := bySource[SourcePMG]; ok {
			return &MetricsTarget{ResourceType: "node", ResourceID: st.SourceID}
		}
	}

	return nil
}
