package mock

import "testing"

func TestLoadMockConfigInvalidValuesFallbackToDefaults(t *testing.T) {
	t.Setenv("PULSE_MOCK_NODES", "-1")
	t.Setenv("PULSE_MOCK_VMS_PER_NODE", "not-a-number")
	t.Setenv("PULSE_MOCK_LXCS_PER_NODE", "-3")
	t.Setenv("PULSE_MOCK_DOCKER_HOSTS", "oops")
	t.Setenv("PULSE_MOCK_DOCKER_CONTAINERS", "-2")
	t.Setenv("PULSE_MOCK_GENERIC_HOSTS", "bad")
	t.Setenv("PULSE_MOCK_K8S_CLUSTERS", "-1")
	t.Setenv("PULSE_MOCK_K8S_NODES", "x")
	t.Setenv("PULSE_MOCK_K8S_PODS", "-1")
	t.Setenv("PULSE_MOCK_K8S_DEPLOYMENTS", "nope")
	t.Setenv("PULSE_MOCK_RANDOM_METRICS", "definitely")
	t.Setenv("PULSE_MOCK_STOPPED_PERCENT", "nan%")

	cfg := LoadMockConfig()

	if cfg.NodeCount != DefaultConfig.NodeCount {
		t.Fatalf("NodeCount = %d, want default %d", cfg.NodeCount, DefaultConfig.NodeCount)
	}
	if cfg.VMsPerNode != DefaultConfig.VMsPerNode {
		t.Fatalf("VMsPerNode = %d, want default %d", cfg.VMsPerNode, DefaultConfig.VMsPerNode)
	}
	if cfg.LXCsPerNode != DefaultConfig.LXCsPerNode {
		t.Fatalf("LXCsPerNode = %d, want default %d", cfg.LXCsPerNode, DefaultConfig.LXCsPerNode)
	}
	if cfg.DockerHostCount != DefaultConfig.DockerHostCount {
		t.Fatalf("DockerHostCount = %d, want default %d", cfg.DockerHostCount, DefaultConfig.DockerHostCount)
	}
	if cfg.DockerContainersPerHost != DefaultConfig.DockerContainersPerHost {
		t.Fatalf("DockerContainersPerHost = %d, want default %d", cfg.DockerContainersPerHost, DefaultConfig.DockerContainersPerHost)
	}
	if cfg.GenericHostCount != DefaultConfig.GenericHostCount {
		t.Fatalf("GenericHostCount = %d, want default %d", cfg.GenericHostCount, DefaultConfig.GenericHostCount)
	}
	if cfg.K8sClusterCount != DefaultConfig.K8sClusterCount {
		t.Fatalf("K8sClusterCount = %d, want default %d", cfg.K8sClusterCount, DefaultConfig.K8sClusterCount)
	}
	if cfg.K8sNodesPerCluster != DefaultConfig.K8sNodesPerCluster {
		t.Fatalf("K8sNodesPerCluster = %d, want default %d", cfg.K8sNodesPerCluster, DefaultConfig.K8sNodesPerCluster)
	}
	if cfg.K8sPodsPerCluster != DefaultConfig.K8sPodsPerCluster {
		t.Fatalf("K8sPodsPerCluster = %d, want default %d", cfg.K8sPodsPerCluster, DefaultConfig.K8sPodsPerCluster)
	}
	if cfg.K8sDeploymentsPerCluster != DefaultConfig.K8sDeploymentsPerCluster {
		t.Fatalf("K8sDeploymentsPerCluster = %d, want default %d", cfg.K8sDeploymentsPerCluster, DefaultConfig.K8sDeploymentsPerCluster)
	}
	if cfg.RandomMetrics != DefaultConfig.RandomMetrics {
		t.Fatalf("RandomMetrics = %t, want default %t", cfg.RandomMetrics, DefaultConfig.RandomMetrics)
	}
	if cfg.StoppedPercent != DefaultConfig.StoppedPercent {
		t.Fatalf("StoppedPercent = %f, want default %f", cfg.StoppedPercent, DefaultConfig.StoppedPercent)
	}
}

func TestLoadMockConfigValidValuesOverrideDefaults(t *testing.T) {
	t.Setenv("PULSE_MOCK_NODES", "9")
	t.Setenv("PULSE_MOCK_VMS_PER_NODE", "7")
	t.Setenv("PULSE_MOCK_LXCS_PER_NODE", "4")
	t.Setenv("PULSE_MOCK_DOCKER_HOSTS", "5")
	t.Setenv("PULSE_MOCK_DOCKER_CONTAINERS", "11")
	t.Setenv("PULSE_MOCK_GENERIC_HOSTS", "3")
	t.Setenv("PULSE_MOCK_K8S_CLUSTERS", "2")
	t.Setenv("PULSE_MOCK_K8S_NODES", "8")
	t.Setenv("PULSE_MOCK_K8S_PODS", "26")
	t.Setenv("PULSE_MOCK_K8S_DEPLOYMENTS", "6")
	t.Setenv("PULSE_MOCK_RANDOM_METRICS", "false")
	t.Setenv("PULSE_MOCK_STOPPED_PERCENT", "35")

	cfg := LoadMockConfig()

	if cfg.NodeCount != 9 {
		t.Fatalf("NodeCount = %d, want 9", cfg.NodeCount)
	}
	if cfg.VMsPerNode != 7 {
		t.Fatalf("VMsPerNode = %d, want 7", cfg.VMsPerNode)
	}
	if cfg.LXCsPerNode != 4 {
		t.Fatalf("LXCsPerNode = %d, want 4", cfg.LXCsPerNode)
	}
	if cfg.DockerHostCount != 5 {
		t.Fatalf("DockerHostCount = %d, want 5", cfg.DockerHostCount)
	}
	if cfg.DockerContainersPerHost != 11 {
		t.Fatalf("DockerContainersPerHost = %d, want 11", cfg.DockerContainersPerHost)
	}
	if cfg.GenericHostCount != 3 {
		t.Fatalf("GenericHostCount = %d, want 3", cfg.GenericHostCount)
	}
	if cfg.K8sClusterCount != 2 {
		t.Fatalf("K8sClusterCount = %d, want 2", cfg.K8sClusterCount)
	}
	if cfg.K8sNodesPerCluster != 8 {
		t.Fatalf("K8sNodesPerCluster = %d, want 8", cfg.K8sNodesPerCluster)
	}
	if cfg.K8sPodsPerCluster != 26 {
		t.Fatalf("K8sPodsPerCluster = %d, want 26", cfg.K8sPodsPerCluster)
	}
	if cfg.K8sDeploymentsPerCluster != 6 {
		t.Fatalf("K8sDeploymentsPerCluster = %d, want 6", cfg.K8sDeploymentsPerCluster)
	}
	if cfg.RandomMetrics {
		t.Fatalf("RandomMetrics = %t, want false", cfg.RandomMetrics)
	}
	if cfg.StoppedPercent != 0.35 {
		t.Fatalf("StoppedPercent = %f, want 0.35", cfg.StoppedPercent)
	}
}
