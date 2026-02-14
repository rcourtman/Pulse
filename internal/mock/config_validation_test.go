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
	// 150% parses successfully as 150/100=1.5, then normalizeStoppedPercent clamps to 1.0.
	if cfg.StoppedPercent != 1.0 {
		t.Fatalf("expected clamped StoppedPercent=1.0, got %f", cfg.StoppedPercent)
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
	if cfg.NodeCount != 1 {
		t.Fatalf("expected clamped NodeCount=1, got %d", cfg.NodeCount)
	}
	if cfg.VMsPerNode != 0 {
		t.Fatalf("expected clamped VMsPerNode=0, got %d", cfg.VMsPerNode)
	}
	if cfg.LXCsPerNode != 0 {
		t.Fatalf("expected clamped LXCsPerNode=0, got %d", cfg.LXCsPerNode)
	}
	if cfg.StoppedPercent != 1.0 {
		t.Fatalf("expected clamped StoppedPercent=1.0, got %f", cfg.StoppedPercent)
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
