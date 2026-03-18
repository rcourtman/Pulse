package unifiedresources

import "github.com/rcourtman/pulse-go-rewrite/internal/models"

func kubernetesMetricCapabilities(cluster models.KubernetesCluster, linkedHosts []*models.Host) *K8sMetricCapabilities {
	nodeTelemetry := len(linkedHosts) > 0
	nodeCPUMemory := nodeTelemetry
	for _, node := range cluster.Nodes {
		if node.UsageCPUMilliCores > 0 || node.UsageCPUPercent > 0 || node.UsageMemoryBytes > 0 || node.UsageMemoryPercent > 0 {
			nodeCPUMemory = true
			break
		}
	}

	podCPUMemory := false
	podNetwork := false
	podEphemeralDisk := false
	for _, pod := range cluster.Pods {
		if !podCPUMemory && (pod.UsageCPUMilliCores > 0 || pod.UsageCPUPercent > 0 || pod.UsageMemoryBytes > 0 || pod.UsageMemoryPercent > 0) {
			podCPUMemory = true
		}
		if !podNetwork && (pod.NetworkRxBytes > 0 || pod.NetworkTxBytes > 0 || pod.NetInRate > 0 || pod.NetOutRate > 0) {
			podNetwork = true
		}
		if !podEphemeralDisk && (pod.EphemeralStorageCapacityBytes > 0 || pod.EphemeralStorageUsedBytes > 0 || pod.DiskUsagePercent > 0) {
			podEphemeralDisk = true
		}
		if podCPUMemory && podNetwork && podEphemeralDisk {
			break
		}
	}

	capabilities := &K8sMetricCapabilities{
		NodeCPUMemory:    nodeCPUMemory,
		NodeTelemetry:    nodeTelemetry,
		PodCPUMemory:     podCPUMemory,
		PodNetwork:       podNetwork,
		PodEphemeralDisk: podEphemeralDisk,
		PodDiskIO:        false, // Not currently collected for Kubernetes pods.
	}
	return capabilities
}

func cloneKubernetesMetricCapabilities(in *K8sMetricCapabilities) *K8sMetricCapabilities {
	if in == nil {
		return nil
	}
	clone := *in
	return &clone
}
