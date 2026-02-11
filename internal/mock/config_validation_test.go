package mock

import "testing"

func TestLoadMockConfigRejectsInvalidValues(t *testing.T) {
	t.Setenv("PULSE_MOCK_NODES", "0")
	t.Setenv("PULSE_MOCK_VMS_PER_NODE", "-4")
	t.Setenv("PULSE_MOCK_RANDOM_METRICS", "definitely")
	t.Setenv("PULSE_MOCK_STOPPED_PERCENT", "150")
	t.Setenv("PULSE_MOCK_DOCKER_HOSTS", "0")

	cfg := LoadMockConfig()

	if cfg.NodeCount != DefaultConfig.NodeCount {
		t.Fatalf("expected default node count %d, got %d", DefaultConfig.NodeCount, cfg.NodeCount)
	}
	if cfg.VMsPerNode != DefaultConfig.VMsPerNode {
		t.Fatalf("expected default VMsPerNode %d, got %d", DefaultConfig.VMsPerNode, cfg.VMsPerNode)
	}
	if cfg.RandomMetrics != DefaultConfig.RandomMetrics {
		t.Fatalf("expected default RandomMetrics=%t, got %t", DefaultConfig.RandomMetrics, cfg.RandomMetrics)
	}
	if cfg.StoppedPercent != DefaultConfig.StoppedPercent {
		t.Fatalf("expected default StoppedPercent=%f, got %f", DefaultConfig.StoppedPercent, cfg.StoppedPercent)
	}
	if cfg.DockerHostCount != 0 {
		t.Fatalf("expected DockerHostCount=0 from valid env override, got %d", cfg.DockerHostCount)
	}
}

func TestSetMockConfigNormalizesInvalidValues(t *testing.T) {
	SetEnabled(false)
	t.Cleanup(func() {
		SetEnabled(false)
		SetMockConfig(DefaultConfig)
	})

	SetMockConfig(MockConfig{
		NodeCount:                -5,
		VMsPerNode:               -1,
		LXCsPerNode:              -2,
		DockerHostCount:          -3,
		DockerContainersPerHost:  -4,
		GenericHostCount:         -5,
		K8sClusterCount:          -6,
		K8sNodesPerCluster:       -7,
		K8sPodsPerCluster:        -8,
		K8sDeploymentsPerCluster: -9,
		StoppedPercent:           1.5,
	})

	cfg := GetConfig()
	if cfg.NodeCount != DefaultConfig.NodeCount {
		t.Fatalf("expected default NodeCount=%d, got %d", DefaultConfig.NodeCount, cfg.NodeCount)
	}
	if cfg.VMsPerNode != DefaultConfig.VMsPerNode {
		t.Fatalf("expected default VMsPerNode=%d, got %d", DefaultConfig.VMsPerNode, cfg.VMsPerNode)
	}
	if cfg.LXCsPerNode != DefaultConfig.LXCsPerNode {
		t.Fatalf("expected default LXCsPerNode=%d, got %d", DefaultConfig.LXCsPerNode, cfg.LXCsPerNode)
	}
	if cfg.StoppedPercent != DefaultConfig.StoppedPercent {
		t.Fatalf("expected default StoppedPercent=%f, got %f", DefaultConfig.StoppedPercent, cfg.StoppedPercent)
	}
}

func TestGenerateMockDataAllowsZeroGuestConfig(t *testing.T) {
	cfg := DefaultConfig
	cfg.NodeCount = 3
	cfg.VMsPerNode = 0
	cfg.LXCsPerNode = 0

	data := GenerateMockData(cfg)

	if len(data.Nodes) != cfg.NodeCount {
		t.Fatalf("expected %d nodes, got %d", cfg.NodeCount, len(data.Nodes))
	}
}
