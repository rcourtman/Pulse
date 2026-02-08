package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestBuildUnifiedResourceContext_NilProvider(t *testing.T) {
	s := &Service{}
	if got := s.buildUnifiedResourceContext(); got != "" {
		t.Errorf("expected empty context, got %q", got)
	}
}

func TestBuildUnifiedResourceContext_FullContext(t *testing.T) {
	nodeWithAgent := unifiedresources.LegacyResource{
		ID:           "node-1",
		Name:         "delly",
		Type:         unifiedresources.LegacyResourceTypeNode,
		PlatformType: unifiedresources.LegacyPlatformProxmoxPVE,
		ClusterID:    "cluster-a",
		Status:       unifiedresources.LegacyStatusOnline,
		CPU:          metricValue(12.3),
		Memory:       metricValue(45.6),
	}
	nodeNoAgent := unifiedresources.LegacyResource{
		ID:           "node-2",
		Name:         "minipc",
		Type:         unifiedresources.LegacyResourceTypeNode,
		PlatformType: unifiedresources.LegacyPlatformProxmoxPVE,
		Status:       unifiedresources.LegacyStatusDegraded,
	}
	dockerNode := unifiedresources.LegacyResource{
		ID:           "dock-node",
		Name:         "dock-node",
		Type:         unifiedresources.LegacyResourceTypeNode,
		PlatformType: unifiedresources.LegacyPlatformDocker,
		Status:       unifiedresources.LegacyStatusOnline,
	}
	host := unifiedresources.LegacyResource{
		ID:           "host-1",
		Name:         "barehost",
		Type:         unifiedresources.LegacyResourceTypeHost,
		PlatformType: unifiedresources.LegacyPlatformHostAgent,
		Status:       unifiedresources.LegacyStatusOnline,
		Identity:     &unifiedresources.LegacyIdentity{IPs: []string{"192.168.1.10"}},
		CPU:          metricValue(5.0),
		Memory:       metricValue(10.0),
	}
	dockerHost := unifiedresources.LegacyResource{
		ID:           "docker-1",
		Name:         "dockhost",
		Type:         unifiedresources.LegacyResourceTypeDockerHost,
		PlatformType: unifiedresources.LegacyPlatformDocker,
		Status:       unifiedresources.LegacyStatusRunning,
	}

	vm := unifiedresources.LegacyResource{
		ID:         "vm-100",
		Name:       "web-vm",
		Type:       unifiedresources.LegacyResourceTypeVM,
		ParentID:   "node-1",
		PlatformID: "100",
		Status:     unifiedresources.LegacyStatusRunning,
		Identity:   &unifiedresources.LegacyIdentity{IPs: []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}},
		CPU:        metricValue(65.4),
		Memory:     metricValue(70.2),
	}
	vm.Alerts = []unifiedresources.LegacyAlert{
		{ID: "alert-1", Message: "CPU high", Level: "critical"},
	}

	ct := unifiedresources.LegacyResource{
		ID:         "ct-200",
		Name:       "db-ct",
		Type:       unifiedresources.LegacyResourceTypeContainer,
		ParentID:   "node-1",
		PlatformID: "200",
		Status:     unifiedresources.LegacyStatusStopped,
	}
	dockerContainer := unifiedresources.LegacyResource{
		ID:       "dock-300",
		Name:     "redis",
		Type:     unifiedresources.LegacyResourceTypeDockerContainer,
		ParentID: "docker-1",
		Status:   unifiedresources.LegacyStatusRunning,
		Disk:     metricValue(70.0),
	}
	dockerStopped := unifiedresources.LegacyResource{
		ID:       "dock-301",
		Name:     "cache",
		Type:     unifiedresources.LegacyResourceTypeDockerContainer,
		ParentID: "docker-1",
		Status:   unifiedresources.LegacyStatusStopped,
	}
	unknownParent := unifiedresources.LegacyResource{
		ID:         "vm-999",
		Name:       "mystery",
		Type:       unifiedresources.LegacyResourceTypeVM,
		ParentID:   "unknown-parent",
		PlatformID: "999",
		Status:     unifiedresources.LegacyStatusRunning,
	}
	orphan := unifiedresources.LegacyResource{
		ID:       "orphan-1",
		Name:     "orphan",
		Type:     unifiedresources.LegacyResourceTypeContainer,
		Status:   unifiedresources.LegacyStatusRunning,
		Identity: &unifiedresources.LegacyIdentity{IPs: []string{"172.16.0.5"}},
	}

	infrastructure := []unifiedresources.LegacyResource{nodeWithAgent, nodeNoAgent, host, dockerHost, dockerNode}
	workloads := []unifiedresources.LegacyResource{vm, ct, dockerContainer, dockerStopped, unknownParent, orphan}
	all := append(append([]unifiedresources.LegacyResource{}, infrastructure...), workloads...)

	stats := unifiedresources.LegacyStoreStats{
		TotalResources: len(all),
		ByType: map[unifiedresources.LegacyResourceType]int{
			unifiedresources.LegacyResourceTypeNode:            3,
			unifiedresources.LegacyResourceTypeHost:            1,
			unifiedresources.LegacyResourceTypeDockerHost:      1,
			unifiedresources.LegacyResourceTypeVM:              2,
			unifiedresources.LegacyResourceTypeContainer:       2,
			unifiedresources.LegacyResourceTypeDockerContainer: 2,
		},
	}

	summary := unifiedresources.LegacyResourceSummary{
		TotalResources: len(all),
		Healthy:        6,
		Degraded:       1,
		Offline:        4,
		WithAlerts:     1,
		ByType: map[unifiedresources.LegacyResourceType]unifiedresources.LegacyTypeSummary{
			unifiedresources.LegacyResourceTypeVM:        {Count: 2, AvgCPUPercent: 40.0, AvgMemoryPercent: 55.0},
			unifiedresources.LegacyResourceTypeContainer: {Count: 2},
		},
	}

	mockRP := &mockResourceProvider{
		getStatsFunc: func() unifiedresources.LegacyStoreStats {
			return stats
		},
		getInfrastructureFunc: func() []unifiedresources.LegacyResource {
			return infrastructure
		},
		getWorkloadsFunc: func() []unifiedresources.LegacyResource {
			return workloads
		},
		getAllFunc: func() []unifiedresources.LegacyResource {
			return all
		},
		getSummaryFunc: func() unifiedresources.LegacyResourceSummary {
			return summary
		},
		getTopCPUFunc: func(limit int, types []unifiedresources.LegacyResourceType) []unifiedresources.LegacyResource {
			return []unifiedresources.LegacyResource{vm}
		},
		getTopMemoryFunc: func(limit int, types []unifiedresources.LegacyResourceType) []unifiedresources.LegacyResource {
			return []unifiedresources.LegacyResource{host}
		},
		getTopDiskFunc: func(limit int, types []unifiedresources.LegacyResourceType) []unifiedresources.LegacyResource {
			return []unifiedresources.LegacyResource{dockerContainer}
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

	node := unifiedresources.LegacyResource{
		ID:           "node-1",
		Name:         "node-1",
		DisplayName:  largeName,
		Type:         unifiedresources.LegacyResourceTypeNode,
		PlatformType: unifiedresources.LegacyPlatformProxmoxPVE,
		Status:       unifiedresources.LegacyStatusOnline,
	}

	stats := unifiedresources.LegacyStoreStats{
		TotalResources: 1,
		ByType: map[unifiedresources.LegacyResourceType]int{
			unifiedresources.LegacyResourceTypeNode: 1,
		},
	}

	mockRP := &mockResourceProvider{
		getStatsFunc: func() unifiedresources.LegacyStoreStats {
			return stats
		},
		getInfrastructureFunc: func() []unifiedresources.LegacyResource {
			return []unifiedresources.LegacyResource{node}
		},
		getWorkloadsFunc: func() []unifiedresources.LegacyResource {
			return nil
		},
		getAllFunc: func() []unifiedresources.LegacyResource {
			return []unifiedresources.LegacyResource{node}
		},
		getSummaryFunc: func() unifiedresources.LegacyResourceSummary {
			return unifiedresources.LegacyResourceSummary{TotalResources: 1}
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

func TestBuildUnifiedResourceContext_WithLegacyAdapterProvider(t *testing.T) {
	adapter := unifiedresources.NewLegacyAdapter(nil)
	adapter.PopulateFromSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "node-1", Status: "online"},
		},
		VMs: []models.VM{
			{ID: "vm-1", Name: "vm-1", Node: "node-1", Status: "running"},
		},
	})

	s := &Service{resourceProvider: adapter}
	got := s.buildUnifiedResourceContext()
	if got == "" {
		t.Fatal("expected non-empty context from adapter provider")
	}
	if !strings.Contains(got, "node-1") {
		t.Fatalf("expected context to include node from adapter, got %q", got)
	}
	if !strings.Contains(got, "vm-1") {
		t.Fatalf("expected context to include vm from adapter, got %q", got)
	}
}

func TestResourceContextUnifiedProvider_NilRegistry(t *testing.T) {
	adapter := unifiedresources.NewUnifiedAIAdapter(nil)

	if got := adapter.GetAll(); len(got) != 0 {
		t.Fatalf("GetAll() len = %d, want 0", len(got))
	}
	if got := adapter.GetInfrastructure(); len(got) != 0 {
		t.Fatalf("GetInfrastructure() len = %d, want 0", len(got))
	}
	if got := adapter.GetWorkloads(); len(got) != 0 {
		t.Fatalf("GetWorkloads() len = %d, want 0", len(got))
	}
	if got := adapter.GetByType(unifiedresources.ResourceTypeHost); len(got) != 0 {
		t.Fatalf("GetByType(host) len = %d, want 0", len(got))
	}
	if got := adapter.GetTopByCPU(3, nil); len(got) != 0 {
		t.Fatalf("GetTopByCPU() len = %d, want 0", len(got))
	}
	if got := adapter.GetRelated("missing"); len(got) != 0 {
		t.Fatalf("GetRelated(missing) len = %d, want 0", len(got))
	}
	if got := adapter.FindContainerHost("missing"); got != "" {
		t.Fatalf("FindContainerHost(missing) = %q, want empty", got)
	}
	if stats := adapter.GetStats(); stats.Total != 0 {
		t.Fatalf("GetStats().Total = %d, want 0", stats.Total)
	}
}

func TestResourceContextUnifiedProvider_ResourceCounts(t *testing.T) {
	legacy, unified := resourceContextTestAdaptersFromSnapshot(resourceContextTestSnapshot())

	if got, want := len(unified.GetAll()), len(legacy.GetAll()); got != want {
		t.Fatalf("GetAll() count mismatch: unified=%d legacy=%d", got, want)
	}
}

func TestResourceContextUnifiedProvider_InfrastructureWorkloadSplit(t *testing.T) {
	legacy, unified := resourceContextTestAdaptersFromSnapshot(resourceContextTestSnapshot())

	if got, want := len(unified.GetInfrastructure()), len(legacy.GetInfrastructure()); got != want {
		t.Fatalf("GetInfrastructure() count mismatch: unified=%d legacy=%d", got, want)
	}
	if got, want := len(unified.GetWorkloads()), len(legacy.GetWorkloads()); got != want {
		t.Fatalf("GetWorkloads() count mismatch: unified=%d legacy=%d", got, want)
	}
}

func TestResourceContextUnifiedProvider_TopCPU(t *testing.T) {
	registry := unifiedresources.NewRegistry(nil)
	now := time.Now().UTC()
	registry.IngestRecords(unifiedresources.SourceAgent, []unifiedresources.IngestRecord{
		{
			SourceID: "host-low",
			Resource: unifiedresources.Resource{
				Type:     unifiedresources.ResourceTypeHost,
				Name:     "host-low",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
				Metrics: &unifiedresources.ResourceMetrics{
					CPU: &unifiedresources.MetricValue{Percent: 25},
				},
			},
		},
	})
	registry.IngestRecords(unifiedresources.SourceProxmox, []unifiedresources.IngestRecord{
		{
			SourceID: "vm-high",
			Resource: unifiedresources.Resource{
				Type:     unifiedresources.ResourceTypeVM,
				Name:     "vm-high",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
				Metrics: &unifiedresources.ResourceMetrics{
					CPU: &unifiedresources.MetricValue{Percent: 92},
				},
			},
		},
	})
	registry.IngestRecords(unifiedresources.SourceDocker, []unifiedresources.IngestRecord{
		{
			SourceID: "ct-mid",
			Resource: unifiedresources.Resource{
				Type:     unifiedresources.ResourceTypeContainer,
				Name:     "ct-mid",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
				Metrics: &unifiedresources.ResourceMetrics{
					CPU: &unifiedresources.MetricValue{Percent: 61},
				},
			},
		},
	})

	unified := unifiedresources.NewUnifiedAIAdapter(registry)
	top := unified.GetTopByCPU(2, []unifiedresources.ResourceType{
		unifiedresources.ResourceTypeHost,
		unifiedresources.ResourceTypeVM,
		unifiedresources.ResourceTypeContainer,
	})
	if len(top) != 2 {
		t.Fatalf("GetTopByCPU() len = %d, want 2", len(top))
	}
	if top[0].Name != "vm-high" || top[1].Name != "ct-mid" {
		t.Fatalf("unexpected top CPU ordering: got [%s, %s]", top[0].Name, top[1].Name)
	}
}

func TestResourceContextUnifiedProvider_FindContainerHost(t *testing.T) {
	registry := unifiedresources.NewRegistry(nil)
	now := time.Now().UTC()
	registry.IngestRecords(unifiedresources.SourceDocker, []unifiedresources.IngestRecord{
		{
			SourceID: "docker-host-1",
			Resource: unifiedresources.Resource{
				Type:     unifiedresources.ResourceTypeHost,
				Name:     "docker-node-1",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
			},
		},
		{
			SourceID:       "container-1",
			ParentSourceID: "docker-host-1",
			Resource: unifiedresources.Resource{
				Type:     unifiedresources.ResourceTypeContainer,
				Name:     "web",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
				Docker:   &unifiedresources.DockerData{ContainerID: "abc123"},
			},
		},
	})

	unified := unifiedresources.NewUnifiedAIAdapter(registry)
	if got := unified.FindContainerHost("web"); got != "docker-node-1" {
		t.Fatalf("FindContainerHost(web) = %q, want docker-node-1", got)
	}
	if got := unified.FindContainerHost("abc123"); got != "docker-node-1" {
		t.Fatalf("FindContainerHost(abc123) = %q, want docker-node-1", got)
	}
}

func resourceContextTestAdaptersFromSnapshot(snapshot models.StateSnapshot) (*unifiedresources.AIAdapter, *unifiedresources.UnifiedAIAdapter) {
	registry := unifiedresources.NewRegistry(nil)
	legacy := unifiedresources.NewAIAdapter(registry)
	unified := unifiedresources.NewUnifiedAIAdapter(registry)
	legacy.PopulateFromSnapshot(snapshot)
	return legacy, unified
}

func resourceContextTestSnapshot() models.StateSnapshot {
	now := time.Now().UTC()
	return models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:       "node-1",
				Name:     "node-1",
				Instance: "pve-a",
				Status:   "online",
				CPU:      0.35,
				Memory:   models.Memory{Total: 100, Used: 35, Usage: 0.35},
				LastSeen: now,
			},
		},
		VMs: []models.VM{
			{
				ID:       "vm-101",
				VMID:     101,
				Name:     "vm-101",
				Node:     "node-1",
				Instance: "pve-a",
				Status:   "running",
				CPU:      0.75,
				Memory:   models.Memory{Total: 100, Used: 70, Usage: 0.70},
				LastSeen: now,
			},
		},
		Containers: []models.Container{
			{
				ID:       "ct-201",
				VMID:     201,
				Name:     "ct-201",
				Node:     "node-1",
				Instance: "pve-a",
				Status:   "running",
				CPU:      0.40,
				Memory:   models.Memory{Total: 100, Used: 45, Usage: 0.45},
				LastSeen: now,
			},
		},
		DockerHosts: []models.DockerHost{
			{
				ID:       "docker-host-1",
				Hostname: "docker-host-1",
				Status:   "online",
				CPUUsage: 0.20,
				Memory:   models.Memory{Total: 100, Used: 30, Usage: 0.30},
				LastSeen: now,
				Containers: []models.DockerContainer{
					{
						ID:            "docker-ctr-1",
						Name:          "docker-ctr-1",
						State:         "running",
						CPUPercent:    0.50,
						MemoryLimit:   100,
						MemoryUsage:   20,
						MemoryPercent: 0.20,
					},
				},
			},
		},
	}
}

func metricValue(current float64) *unifiedresources.LegacyMetricValue {
	return &unifiedresources.LegacyMetricValue{Current: current}
}
