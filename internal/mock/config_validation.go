package mock

import (
	"math"
	"strings"
	"unicode"
)

const (
	minMockNodeCount = 1

	maxMockNodeCount                = 64
	maxMockVMsPerNode               = 128
	maxMockLXCsPerNode              = 128
	maxMockDockerHostCount          = 64
	maxMockDockerContainersPerHost  = 256
	maxMockGenericHostCount         = 256
	maxMockK8sClusterCount          = 32
	maxMockK8sNodesPerCluster       = 128
	maxMockK8sPodsPerCluster        = 1000
	maxMockK8sDeploymentsPerCluster = 256

	maxMockHighLoadNodes     = 256
	maxMockHighLoadNodeChars = 128
)

func normalizeMockConfig(cfg MockConfig) MockConfig {
	cfg.NodeCount = clampInt(cfg.NodeCount, minMockNodeCount, maxMockNodeCount)
	cfg.VMsPerNode = clampInt(cfg.VMsPerNode, 0, maxMockVMsPerNode)
	cfg.LXCsPerNode = clampInt(cfg.LXCsPerNode, 0, maxMockLXCsPerNode)
	cfg.DockerHostCount = clampInt(cfg.DockerHostCount, 0, maxMockDockerHostCount)
	cfg.DockerContainersPerHost = clampInt(cfg.DockerContainersPerHost, 0, maxMockDockerContainersPerHost)
	cfg.GenericHostCount = clampInt(cfg.GenericHostCount, 0, maxMockGenericHostCount)
	cfg.K8sClusterCount = clampInt(cfg.K8sClusterCount, 0, maxMockK8sClusterCount)
	cfg.K8sNodesPerCluster = clampInt(cfg.K8sNodesPerCluster, 0, maxMockK8sNodesPerCluster)
	cfg.K8sPodsPerCluster = clampInt(cfg.K8sPodsPerCluster, 0, maxMockK8sPodsPerCluster)
	cfg.K8sDeploymentsPerCluster = clampInt(cfg.K8sDeploymentsPerCluster, 0, maxMockK8sDeploymentsPerCluster)
	cfg.StoppedPercent = normalizeStoppedPercent(cfg.StoppedPercent)
	cfg.HighLoadNodes = normalizeHighLoadNodes(cfg.HighLoadNodes)

	return cfg
}

func normalizeStoppedPercent(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return DefaultConfig.StoppedPercent
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func normalizeHighLoadNodes(nodes []string) []string {
	if len(nodes) == 0 {
		return nil
	}

	limit := len(nodes)
	if limit > maxMockHighLoadNodes {
		limit = maxMockHighLoadNodes
	}

	out := make([]string, 0, limit)
	seen := make(map[string]struct{}, limit)
	for _, raw := range nodes {
		if len(out) >= maxMockHighLoadNodes {
			break
		}

		name := strings.TrimSpace(raw)
		if name == "" || len(name) > maxMockHighLoadNodeChars || containsControlChars(name) {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}

		seen[name] = struct{}{}
		out = append(out, name)
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

func containsControlChars(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
