package ai

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/resources"
)

func TestBuildUnifiedResourceContext_NilProvider(t *testing.T) {
	s := &Service{}
	if got := s.buildUnifiedResourceContext(); got != "" {
		t.Errorf("expected empty context, got %q", got)
	}
}

func TestBuildUnifiedResourceContext_FullContext(t *testing.T) {
	nodeWithAgent := resources.Resource{
		ID:           "node-1",
		Name:         "delly",
		Type:         resources.ResourceTypeNode,
		PlatformType: resources.PlatformProxmoxPVE,
		ClusterID:    "cluster-a",
		Status:       resources.StatusOnline,
		CPU:          metricValue(12.3),
		Memory:       metricValue(45.6),
	}
	nodeNoAgent := resources.Resource{
		ID:           "node-2",
		Name:         "minipc",
		Type:         resources.ResourceTypeNode,
		PlatformType: resources.PlatformProxmoxPVE,
		Status:       resources.StatusDegraded,
	}
	dockerNode := resources.Resource{
		ID:           "dock-node",
		Name:         "dock-node",
		Type:         resources.ResourceTypeNode,
		PlatformType: resources.PlatformDocker,
		Status:       resources.StatusOnline,
	}
	host := resources.Resource{
		ID:           "host-1",
		Name:         "barehost",
		Type:         resources.ResourceTypeHost,
		PlatformType: resources.PlatformHostAgent,
		Status:       resources.StatusOnline,
		Identity:     &resources.ResourceIdentity{IPs: []string{"192.168.1.10"}},
		CPU:          metricValue(5.0),
		Memory:       metricValue(10.0),
	}
	dockerHost := resources.Resource{
		ID:           "docker-1",
		Name:         "dockhost",
		Type:         resources.ResourceTypeDockerHost,
		PlatformType: resources.PlatformDocker,
		Status:       resources.StatusRunning,
	}

	vm := resources.Resource{
		ID:         "vm-100",
		Name:       "web-vm",
		Type:       resources.ResourceTypeVM,
		ParentID:   "node-1",
		PlatformID: "100",
		Status:     resources.StatusRunning,
		Identity:   &resources.ResourceIdentity{IPs: []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}},
		CPU:        metricValue(65.4),
		Memory:     metricValue(70.2),
	}
	vm.Alerts = []resources.ResourceAlert{
		{ID: "alert-1", Message: "CPU high", Level: "critical"},
	}

	ct := resources.Resource{
		ID:         "ct-200",
		Name:       "db-ct",
		Type:       resources.ResourceTypeContainer,
		ParentID:   "node-1",
		PlatformID: "200",
		Status:     resources.StatusStopped,
	}
	dockerContainer := resources.Resource{
		ID:       "dock-300",
		Name:     "redis",
		Type:     resources.ResourceTypeDockerContainer,
		ParentID: "docker-1",
		Status:   resources.StatusRunning,
		Disk:     metricValue(70.0),
	}
	dockerStopped := resources.Resource{
		ID:       "dock-301",
		Name:     "cache",
		Type:     resources.ResourceTypeDockerContainer,
		ParentID: "docker-1",
		Status:   resources.StatusStopped,
	}
	unknownParent := resources.Resource{
		ID:         "vm-999",
		Name:       "mystery",
		Type:       resources.ResourceTypeVM,
		ParentID:   "unknown-parent",
		PlatformID: "999",
		Status:     resources.StatusRunning,
	}
	orphan := resources.Resource{
		ID:       "orphan-1",
		Name:     "orphan",
		Type:     resources.ResourceTypeContainer,
		Status:   resources.StatusRunning,
		Identity: &resources.ResourceIdentity{IPs: []string{"172.16.0.5"}},
	}

	infrastructure := []resources.Resource{nodeWithAgent, nodeNoAgent, host, dockerHost, dockerNode}
	workloads := []resources.Resource{vm, ct, dockerContainer, dockerStopped, unknownParent, orphan}
	all := append(append([]resources.Resource{}, infrastructure...), workloads...)

	stats := resources.StoreStats{
		TotalResources: len(all),
		ByType: map[resources.ResourceType]int{
			resources.ResourceTypeNode:            3,
			resources.ResourceTypeHost:            1,
			resources.ResourceTypeDockerHost:      1,
			resources.ResourceTypeVM:              2,
			resources.ResourceTypeContainer:       2,
			resources.ResourceTypeDockerContainer: 2,
		},
	}

	summary := resources.ResourceSummary{
		TotalResources: len(all),
		Healthy:        6,
		Degraded:       1,
		Offline:        4,
		WithAlerts:     1,
		ByType: map[resources.ResourceType]resources.TypeSummary{
			resources.ResourceTypeVM:        {Count: 2, AvgCPUPercent: 40.0, AvgMemoryPercent: 55.0},
			resources.ResourceTypeContainer: {Count: 2},
		},
	}

	mockRP := &mockResourceProvider{
		getStatsFunc: func() resources.StoreStats {
			return stats
		},
		getInfrastructureFunc: func() []resources.Resource {
			return infrastructure
		},
		getWorkloadsFunc: func() []resources.Resource {
			return workloads
		},
		getAllFunc: func() []resources.Resource {
			return all
		},
		getSummaryFunc: func() resources.ResourceSummary {
			return summary
		},
		getTopCPUFunc: func(limit int, types []resources.ResourceType) []resources.Resource {
			return []resources.Resource{vm}
		},
		getTopMemoryFunc: func(limit int, types []resources.ResourceType) []resources.Resource {
			return []resources.Resource{host}
		},
		getTopDiskFunc: func(limit int, types []resources.ResourceType) []resources.Resource {
			return []resources.Resource{dockerContainer}
		},
	}

	s := &Service{resourceProvider: mockRP}
	s.agentServer = &mockAgentServer{
		agents: []agentexec.ConnectedAgent{
			{AgentID: "agent-1", Hostname: "delly"},
		},
	}

	got := s.buildUnifiedResourceContext()
	if got == "" {
		t.Fatal("expected non-empty context")
	}

	assertContains := func(substr string) {
		t.Helper()
		if !strings.Contains(got, substr) {
			t.Fatalf("expected context to contain %q", substr)
		}
	}

	assertContains("## Unified Infrastructure View")
	assertContains("Total resources: 11 (Infrastructure: 5, Workloads: 6)")
	assertContains("Proxmox VE Nodes")
	assertContains("HAS AGENT")
	assertContains("NO AGENT")
	assertContains("cluster: cluster-a")
	assertContains("Standalone Hosts")
	assertContains("192.168.1.10")
	assertContains("Docker/Podman Hosts")
	assertContains("1/2 containers running")
	assertContains("Workloads (VMs & Containers)")
	assertContains("On delly")
	assertContains("On unknown-parent")
	assertContains("Other workloads")
	assertContains("10.0.0.1, 10.0.0.2")
	assertContains("Resources with Active Alerts")
	assertContains("CPU high")
	assertContains("Infrastructure Summary")
	assertContains("Resources with alerts: 1")
	assertContains("Average utilization by type")
	assertContains("Top CPU Consumers")
	assertContains("Top Memory Consumers")
	assertContains("Top Disk Usage")
}

func TestBuildUnifiedResourceContext_TruncatesLargeContext(t *testing.T) {
	largeName := strings.Repeat("a", 60000)

	node := resources.Resource{
		ID:           "node-1",
		Name:         "node-1",
		DisplayName:  largeName,
		Type:         resources.ResourceTypeNode,
		PlatformType: resources.PlatformProxmoxPVE,
		Status:       resources.StatusOnline,
	}

	stats := resources.StoreStats{
		TotalResources: 1,
		ByType: map[resources.ResourceType]int{
			resources.ResourceTypeNode: 1,
		},
	}

	mockRP := &mockResourceProvider{
		getStatsFunc: func() resources.StoreStats {
			return stats
		},
		getInfrastructureFunc: func() []resources.Resource {
			return []resources.Resource{node}
		},
		getWorkloadsFunc: func() []resources.Resource {
			return nil
		},
		getAllFunc: func() []resources.Resource {
			return []resources.Resource{node}
		},
		getSummaryFunc: func() resources.ResourceSummary {
			return resources.ResourceSummary{TotalResources: 1}
		},
	}

	s := &Service{resourceProvider: mockRP}
	got := s.buildUnifiedResourceContext()
	if !strings.Contains(got, "[... Context truncated ...]") {
		t.Fatal("expected context to be truncated")
	}
	if len(got) <= 50000 {
		t.Fatalf("expected truncated context length > 50000, got %d", len(got))
	}
}

func metricValue(current float64) *resources.MetricValue {
	return &resources.MetricValue{Current: current}
}
