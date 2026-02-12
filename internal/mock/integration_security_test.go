package mock

import (
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestSetMockConfigNormalizesInvalidAndOversizedValues(t *testing.T) {
	prevConfig := GetConfig()
	prevEnabled := IsMockEnabled()
	t.Cleanup(func() {
		SetMockConfig(prevConfig)
		SetEnabled(prevEnabled)
	})

	SetEnabled(false)

	overlong := strings.Repeat("a", maxMockHighLoadNodeChars+1)
	SetMockConfig(MockConfig{
		NodeCount:                0,
		VMsPerNode:               -10,
		LXCsPerNode:              maxMockLXCsPerNode + 5,
		DockerHostCount:          maxMockDockerHostCount + 1,
		DockerContainersPerHost:  maxMockDockerContainersPerHost + 10,
		GenericHostCount:         maxMockGenericHostCount + 3,
		K8sClusterCount:          maxMockK8sClusterCount + 1,
		K8sNodesPerCluster:       maxMockK8sNodesPerCluster + 1,
		K8sPodsPerCluster:        maxMockK8sPodsPerCluster + 1,
		K8sDeploymentsPerCluster: maxMockK8sDeploymentsPerCluster + 1,
		StoppedPercent:           10,
		HighLoadNodes:            []string{" pve1 ", "pve1", "", "pve2\n", overlong, "standalone1"},
	})

	cfg := GetConfig()
	if cfg.NodeCount != minMockNodeCount {
		t.Fatalf("expected node count to normalize to %d, got %d", minMockNodeCount, cfg.NodeCount)
	}
	if cfg.VMsPerNode != 0 {
		t.Fatalf("expected VMs per node to normalize to 0, got %d", cfg.VMsPerNode)
	}
	if cfg.LXCsPerNode != maxMockLXCsPerNode {
		t.Fatalf("expected LXCs per node to normalize to %d, got %d", maxMockLXCsPerNode, cfg.LXCsPerNode)
	}
	if cfg.DockerHostCount != maxMockDockerHostCount {
		t.Fatalf("expected docker host count to normalize to %d, got %d", maxMockDockerHostCount, cfg.DockerHostCount)
	}
	if cfg.DockerContainersPerHost != maxMockDockerContainersPerHost {
		t.Fatalf(
			"expected docker containers per host to normalize to %d, got %d",
			maxMockDockerContainersPerHost,
			cfg.DockerContainersPerHost,
		)
	}
	if cfg.GenericHostCount != maxMockGenericHostCount {
		t.Fatalf("expected generic host count to normalize to %d, got %d", maxMockGenericHostCount, cfg.GenericHostCount)
	}
	if cfg.K8sClusterCount != maxMockK8sClusterCount {
		t.Fatalf("expected k8s cluster count to normalize to %d, got %d", maxMockK8sClusterCount, cfg.K8sClusterCount)
	}
	if cfg.K8sNodesPerCluster != maxMockK8sNodesPerCluster {
		t.Fatalf(
			"expected k8s nodes per cluster to normalize to %d, got %d",
			maxMockK8sNodesPerCluster,
			cfg.K8sNodesPerCluster,
		)
	}
	if cfg.K8sPodsPerCluster != maxMockK8sPodsPerCluster {
		t.Fatalf(
			"expected k8s pods per cluster to normalize to %d, got %d",
			maxMockK8sPodsPerCluster,
			cfg.K8sPodsPerCluster,
		)
	}
	if cfg.K8sDeploymentsPerCluster != maxMockK8sDeploymentsPerCluster {
		t.Fatalf(
			"expected k8s deployments per cluster to normalize to %d, got %d",
			maxMockK8sDeploymentsPerCluster,
			cfg.K8sDeploymentsPerCluster,
		)
	}
	if cfg.StoppedPercent != 1 {
		t.Fatalf("expected stopped percent to normalize to 1, got %f", cfg.StoppedPercent)
	}

	expectedHighLoadNodes := []string{"pve1", "pve2", "standalone1"}
	if !reflect.DeepEqual(cfg.HighLoadNodes, expectedHighLoadNodes) {
		t.Fatalf("expected sanitized high load nodes %v, got %v", expectedHighLoadNodes, cfg.HighLoadNodes)
	}
}

func TestLoadMockConfigEnforcesUpperBoundsFromEnvironment(t *testing.T) {
	t.Setenv("PULSE_MOCK_NODES", strconv.Itoa(maxMockNodeCount+500))
	t.Setenv("PULSE_MOCK_VMS_PER_NODE", strconv.Itoa(maxMockVMsPerNode+500))
	t.Setenv("PULSE_MOCK_LXCS_PER_NODE", strconv.Itoa(maxMockLXCsPerNode+500))
	t.Setenv("PULSE_MOCK_DOCKER_HOSTS", strconv.Itoa(maxMockDockerHostCount+500))
	t.Setenv("PULSE_MOCK_DOCKER_CONTAINERS", strconv.Itoa(maxMockDockerContainersPerHost+500))
	t.Setenv("PULSE_MOCK_GENERIC_HOSTS", strconv.Itoa(maxMockGenericHostCount+500))
	t.Setenv("PULSE_MOCK_K8S_CLUSTERS", strconv.Itoa(maxMockK8sClusterCount+500))
	t.Setenv("PULSE_MOCK_K8S_NODES", strconv.Itoa(maxMockK8sNodesPerCluster+500))
	t.Setenv("PULSE_MOCK_K8S_PODS", strconv.Itoa(maxMockK8sPodsPerCluster+500))
	t.Setenv("PULSE_MOCK_K8S_DEPLOYMENTS", strconv.Itoa(maxMockK8sDeploymentsPerCluster+500))
	t.Setenv("PULSE_MOCK_STOPPED_PERCENT", "250")

	cfg := LoadMockConfig()
	if cfg.NodeCount != maxMockNodeCount {
		t.Fatalf("expected node count %d, got %d", maxMockNodeCount, cfg.NodeCount)
	}
	if cfg.VMsPerNode != maxMockVMsPerNode {
		t.Fatalf("expected VMs per node %d, got %d", maxMockVMsPerNode, cfg.VMsPerNode)
	}
	if cfg.LXCsPerNode != maxMockLXCsPerNode {
		t.Fatalf("expected LXCs per node %d, got %d", maxMockLXCsPerNode, cfg.LXCsPerNode)
	}
	if cfg.DockerHostCount != maxMockDockerHostCount {
		t.Fatalf("expected docker host count %d, got %d", maxMockDockerHostCount, cfg.DockerHostCount)
	}
	if cfg.DockerContainersPerHost != maxMockDockerContainersPerHost {
		t.Fatalf("expected docker containers per host %d, got %d", maxMockDockerContainersPerHost, cfg.DockerContainersPerHost)
	}
	if cfg.GenericHostCount != maxMockGenericHostCount {
		t.Fatalf("expected generic host count %d, got %d", maxMockGenericHostCount, cfg.GenericHostCount)
	}
	if cfg.K8sClusterCount != maxMockK8sClusterCount {
		t.Fatalf("expected k8s cluster count %d, got %d", maxMockK8sClusterCount, cfg.K8sClusterCount)
	}
	if cfg.K8sNodesPerCluster != maxMockK8sNodesPerCluster {
		t.Fatalf("expected k8s nodes per cluster %d, got %d", maxMockK8sNodesPerCluster, cfg.K8sNodesPerCluster)
	}
	if cfg.K8sPodsPerCluster != maxMockK8sPodsPerCluster {
		t.Fatalf("expected k8s pods per cluster %d, got %d", maxMockK8sPodsPerCluster, cfg.K8sPodsPerCluster)
	}
	if cfg.K8sDeploymentsPerCluster != maxMockK8sDeploymentsPerCluster {
		t.Fatalf(
			"expected k8s deployments per cluster %d, got %d",
			maxMockK8sDeploymentsPerCluster,
			cfg.K8sDeploymentsPerCluster,
		)
	}
	if cfg.StoppedPercent != 1 {
		t.Fatalf("expected stopped percent 1, got %f", cfg.StoppedPercent)
	}
}

func TestLoadMockConfigInvalidStoppedPercentFallsBackToDefault(t *testing.T) {
	t.Setenv("PULSE_MOCK_STOPPED_PERCENT", "NaN")

	cfg := LoadMockConfig()
	if math.IsNaN(cfg.StoppedPercent) {
		t.Fatal("expected stopped percent to be normalized to a finite value")
	}
	if cfg.StoppedPercent != DefaultConfig.StoppedPercent {
		t.Fatalf("expected stopped percent %f, got %f", DefaultConfig.StoppedPercent, cfg.StoppedPercent)
	}
}
