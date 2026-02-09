package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestBuildUnifiedResourceContext_NilProvider(t *testing.T) {
	s := &Service{}
	if got := s.buildUnifiedResourceContext(); got != "" {
		t.Errorf("expected empty context, got %q", got)
	}
}

func TestBuildUnifiedResourceContext_FullContext(t *testing.T) {
	nodeWithAgent := unifiedresources.Resource{
		ID:     "node-1",
		Name:   "delly",
		Type:   unifiedresources.ResourceTypeHost,
		Status: unifiedresources.StatusOnline,
		Identity: unifiedresources.ResourceIdentity{
			ClusterName: "cluster-a",
		},
		Metrics: &unifiedresources.ResourceMetrics{
			CPU:    &unifiedresources.MetricValue{Percent: 12.3},
			Memory: &unifiedresources.MetricValue{Percent: 45.6},
		},
		Proxmox: &unifiedresources.ProxmoxData{
			NodeName:    "delly",
			ClusterName: "cluster-a",
		},
	}
	nodeNoAgent := unifiedresources.Resource{
		ID:      "node-2",
		Name:    "minipc",
		Type:    unifiedresources.ResourceTypeHost,
		Status:  unifiedresources.StatusWarning,
		Proxmox: &unifiedresources.ProxmoxData{NodeName: "minipc"},
	}
	dockerNode := unifiedresources.Resource{
		ID:     "dock-node",
		Name:   "dock-node",
		Type:   unifiedresources.ResourceTypeHost,
		Status: unifiedresources.StatusOnline,
		Docker: &unifiedresources.DockerData{Hostname: "dock-node"},
	}
	host := unifiedresources.Resource{
		ID:     "host-1",
		Name:   "barehost",
		Type:   unifiedresources.ResourceTypeHost,
		Status: unifiedresources.StatusOnline,
		Identity: unifiedresources.ResourceIdentity{
			IPAddresses: []string{"192.168.1.10"},
		},
		Metrics: &unifiedresources.ResourceMetrics{
			CPU:    &unifiedresources.MetricValue{Percent: 5},
			Memory: &unifiedresources.MetricValue{Percent: 10},
		},
		Agent: &unifiedresources.AgentData{
			Hostname: "barehost",
		},
	}
	dockerHost := unifiedresources.Resource{
		ID:     "docker-1",
		Name:   "dockhost",
		Type:   unifiedresources.ResourceTypeHost,
		Status: unifiedresources.StatusOnline,
		Docker: &unifiedresources.DockerData{Hostname: "dockhost"},
	}

	nodeWithAgentID := nodeWithAgent.ID
	dockerHostID := dockerHost.ID
	unknownParentID := "unknown-parent"
	vm := unifiedresources.Resource{
		ID:       "vm-100",
		Name:     "web-vm",
		Type:     unifiedresources.ResourceTypeVM,
		ParentID: &nodeWithAgentID,
		Status:   unifiedresources.StatusOnline,
		Identity: unifiedresources.ResourceIdentity{
			IPAddresses: []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
		},
		Metrics: &unifiedresources.ResourceMetrics{
			CPU:    &unifiedresources.MetricValue{Percent: 65.4},
			Memory: &unifiedresources.MetricValue{Percent: 70.2},
		},
		Proxmox: &unifiedresources.ProxmoxData{
			VMID: 100,
		},
	}
	ct := unifiedresources.Resource{
		ID:       "ct-200",
		Name:     "db-ct",
		Type:     unifiedresources.ResourceTypeLXC,
		ParentID: &nodeWithAgentID,
		Status:   unifiedresources.StatusOffline,
		Proxmox: &unifiedresources.ProxmoxData{
			VMID: 200,
		},
	}
	dockerContainer := unifiedresources.Resource{
		ID:       "dock-300",
		Name:     "redis",
		Type:     unifiedresources.ResourceTypeContainer,
		ParentID: &dockerHostID,
		Status:   unifiedresources.StatusOnline,
		Metrics: &unifiedresources.ResourceMetrics{
			Disk: &unifiedresources.MetricValue{Percent: 70},
		},
	}
	dockerStopped := unifiedresources.Resource{
		ID:       "dock-301",
		Name:     "cache",
		Type:     unifiedresources.ResourceTypeContainer,
		ParentID: &dockerHostID,
		Status:   unifiedresources.StatusOffline,
	}
	unknownParent := unifiedresources.Resource{
		ID:       "vm-999",
		Name:     "mystery",
		Type:     unifiedresources.ResourceTypeVM,
		ParentID: &unknownParentID,
		Status:   unifiedresources.StatusOnline,
		Proxmox: &unifiedresources.ProxmoxData{
			VMID: 999,
		},
	}
	orphan := unifiedresources.Resource{
		ID:     "orphan-1",
		Name:   "orphan",
		Type:   unifiedresources.ResourceTypeLXC,
		Status: unifiedresources.StatusOnline,
		Identity: unifiedresources.ResourceIdentity{
			IPAddresses: []string{"172.16.0.5"},
		},
	}

	infrastructure := []unifiedresources.Resource{nodeWithAgent, nodeNoAgent, host, dockerHost, dockerNode}
	workloads := []unifiedresources.Resource{vm, ct, dockerContainer, dockerStopped, unknownParent, orphan}
	all := append(append([]unifiedresources.Resource{}, infrastructure...), workloads...)

	stats := unifiedresources.ResourceStats{
		Total: len(all),
		ByType: map[unifiedresources.ResourceType]int{
			unifiedresources.ResourceTypeHost:      5,
			unifiedresources.ResourceTypeVM:        2,
			unifiedresources.ResourceTypeLXC:       2,
			unifiedresources.ResourceTypeContainer: 2,
		},
		ByStatus: map[unifiedresources.ResourceStatus]int{
			unifiedresources.StatusOnline:  7,
			unifiedresources.StatusWarning: 1,
			unifiedresources.StatusOffline: 3,
		},
		BySource: map[unifiedresources.DataSource]int{
			unifiedresources.SourceProxmox: 4,
			unifiedresources.SourceAgent:   1,
			unifiedresources.SourceDocker:  2,
		},
	}

	mockURP := &mockUnifiedResourceProvider{
		getStatsFunc: func() unifiedresources.ResourceStats {
			return stats
		},
		getInfrastructureFunc: func() []unifiedresources.Resource {
			return infrastructure
		},
		getWorkloadsFunc: func() []unifiedresources.Resource {
			return workloads
		},
		getAllFunc: func() []unifiedresources.Resource {
			return all
		},
		getTopCPUFunc: func(limit int, types []unifiedresources.ResourceType) []unifiedresources.Resource {
			return []unifiedresources.Resource{vm}
		},
		getTopMemoryFunc: func(limit int, types []unifiedresources.ResourceType) []unifiedresources.Resource {
			return []unifiedresources.Resource{host}
		},
		getTopDiskFunc: func(limit int, types []unifiedresources.ResourceType) []unifiedresources.Resource {
			return []unifiedresources.Resource{dockerContainer}
		},
	}

	s := &Service{
		unifiedResourceProvider: mockURP,
		alertProvider: &resourceContextAlertProvider{
			active: []AlertInfo{
				{
					ResourceID:   vm.ID,
					ResourceName: vm.Name,
					Message:      "CPU high",
					Level:        "critical",
				},
			},
		},
	}
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

func TestBuildUnifiedResourceContext_UnifiedPath(t *testing.T) {
	clusterID := "k8s-cluster-1"
	k8sCluster := unifiedresources.Resource{
		ID:     clusterID,
		Name:   "prod-cluster",
		Type:   unifiedresources.ResourceTypeK8sCluster,
		Status: unifiedresources.StatusOnline,
		Identity: unifiedresources.ResourceIdentity{
			ClusterName: "prod-cluster",
		},
		Kubernetes: &unifiedresources.K8sData{
			ClusterID:   "prod-cluster",
			ClusterName: "prod-cluster",
		},
	}
	k8sNode := unifiedresources.Resource{
		ID:       "k8s-node-1",
		Name:     "worker-1",
		Type:     unifiedresources.ResourceTypeK8sNode,
		Status:   unifiedresources.StatusWarning,
		ParentID: &clusterID,
		Identity: unifiedresources.ResourceIdentity{
			ClusterName: "prod-cluster",
		},
		Metrics: &unifiedresources.ResourceMetrics{
			CPU:    &unifiedresources.MetricValue{Percent: 91},
			Memory: &unifiedresources.MetricValue{Percent: 77},
		},
		Kubernetes: &unifiedresources.K8sData{
			ClusterID:   "prod-cluster",
			ClusterName: "prod-cluster",
		},
	}

	all := []unifiedresources.Resource{k8sCluster, k8sNode}
	stats := unifiedresources.ResourceStats{
		Total: len(all),
		ByType: map[unifiedresources.ResourceType]int{
			unifiedresources.ResourceTypeK8sCluster: 1,
			unifiedresources.ResourceTypeK8sNode:    1,
		},
		ByStatus: map[unifiedresources.ResourceStatus]int{
			unifiedresources.StatusOnline:  1,
			unifiedresources.StatusWarning: 1,
		},
		BySource: map[unifiedresources.DataSource]int{
			unifiedresources.SourceK8s: 2,
		},
	}

	unifiedProvider := &mockUnifiedResourceProvider{
		getStatsFunc: func() unifiedresources.ResourceStats { return stats },
		getInfrastructureFunc: func() []unifiedresources.Resource {
			return all
		},
		getAllFunc: func() []unifiedresources.Resource {
			return all
		},
		getTopCPUFunc: func(limit int, types []unifiedresources.ResourceType) []unifiedresources.Resource {
			return []unifiedresources.Resource{k8sNode}
		},
	}

	s := &Service{
		unifiedResourceProvider: unifiedProvider,
		alertProvider: &resourceContextAlertProvider{
			active: []AlertInfo{
				{
					ResourceID:   k8sNode.ID,
					ResourceName: "wrong-fallback-name",
					Message:      "Node not ready",
					Level:        "warning",
				},
			},
		},
	}

	got := s.buildUnifiedResourceContext()
	if got == "" {
		t.Fatal("expected non-empty unified context")
	}

	assertContains := func(substr string) {
		t.Helper()
		if !strings.Contains(got, substr) {
			t.Fatalf("expected context to contain %q", substr)
		}
	}

	assertContains("Total resources: 2 (Infrastructure: 2, Workloads: 0)")
	assertContains("Kubernetes")
	assertContains("prod-cluster")
	assertContains("worker-1")
	assertContains("Resources with Active Alerts")
	assertContains("Node not ready")
}

func TestBuildUnifiedResourceContextIncludesTrueNASResources(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() {
		truenas.SetFeatureEnabled(previous)
	})

	registry := unifiedresources.NewRegistry(unifiedresources.NewMemoryStore())
	records := truenas.NewDefaultProvider().Records()
	if len(records) == 0 {
		t.Fatal("expected truenas fixture records")
	}
	registry.IngestRecords(unifiedresources.SourceTrueNAS, records)

	adapter := unifiedresources.NewUnifiedAIAdapter(registry)
	s := &Service{}
	s.SetUnifiedResourceProvider(adapter)

	got := s.buildUnifiedResourceContext()
	if got == "" {
		t.Fatal("expected non-empty context")
	}
	if !strings.Contains(got, "TrueNAS Systems") {
		t.Fatalf("expected context to include TrueNAS section, got %q", got)
	}
	if !strings.Contains(got, "truenas-main") {
		t.Fatalf("expected context to include truenas host, got %q", got)
	}
}

func TestBuildUnifiedResourceContext_TruncatesLargeContext(t *testing.T) {
	largeName := strings.Repeat("a", 60000)

	node := unifiedresources.Resource{
		ID:     "node-1",
		Name:   largeName,
		Type:   unifiedresources.ResourceTypeHost,
		Status: unifiedresources.StatusOnline,
	}

	stats := unifiedresources.ResourceStats{
		Total: 1,
		ByType: map[unifiedresources.ResourceType]int{
			unifiedresources.ResourceTypeHost: 1,
		},
	}

	mockURP := &mockUnifiedResourceProvider{
		getStatsFunc: func() unifiedresources.ResourceStats {
			return stats
		},
		getInfrastructureFunc: func() []unifiedresources.Resource {
			return []unifiedresources.Resource{node}
		},
		getWorkloadsFunc: func() []unifiedresources.Resource {
			return nil
		},
		getAllFunc: func() []unifiedresources.Resource {
			return []unifiedresources.Resource{node}
		},
	}

	s := &Service{unifiedResourceProvider: mockURP}
	got := s.buildUnifiedResourceContext()
	if !strings.Contains(got, "[... Context truncated ...]") {
		t.Fatal("expected context to be truncated")
	}
	if len(got) <= 50000 {
		t.Fatalf("expected truncated context length > 50000, got %d", len(got))
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
	registry := unifiedresources.NewRegistry(nil)
	now := time.Now().UTC()

	registry.IngestRecords(unifiedresources.SourceAgent, []unifiedresources.IngestRecord{
		{
			SourceID: "host-1",
			Resource: unifiedresources.Resource{
				Type:     unifiedresources.ResourceTypeHost,
				Name:     "host-1",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
			},
		},
	})
	registry.IngestRecords(unifiedresources.SourceProxmox, []unifiedresources.IngestRecord{
		{
			SourceID: "vm-1",
			Resource: unifiedresources.Resource{
				Type:     unifiedresources.ResourceTypeVM,
				Name:     "vm-1",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
			},
		},
		{
			SourceID: "ct-1",
			Resource: unifiedresources.Resource{
				Type:     unifiedresources.ResourceTypeLXC,
				Name:     "ct-1",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
			},
		},
	})
	unified := unifiedresources.NewUnifiedAIAdapter(registry)

	if got, want := len(unified.GetAll()), 3; got != want {
		t.Fatalf("GetAll() count mismatch: got=%d want=%d", got, want)
	}
}

func TestResourceContextUnifiedProvider_InfrastructureWorkloadSplit(t *testing.T) {
	registry := unifiedresources.NewRegistry(nil)
	now := time.Now().UTC()

	registry.IngestRecords(unifiedresources.SourceAgent, []unifiedresources.IngestRecord{
		{
			SourceID: "host-1",
			Resource: unifiedresources.Resource{
				Type:     unifiedresources.ResourceTypeHost,
				Name:     "host-1",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
			},
		},
	})
	registry.IngestRecords(unifiedresources.SourceProxmox, []unifiedresources.IngestRecord{
		{
			SourceID: "vm-1",
			Resource: unifiedresources.Resource{
				Type:     unifiedresources.ResourceTypeVM,
				Name:     "vm-1",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
			},
		},
		{
			SourceID: "ct-1",
			Resource: unifiedresources.Resource{
				Type:     unifiedresources.ResourceTypeLXC,
				Name:     "ct-1",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
			},
		},
	})
	unified := unifiedresources.NewUnifiedAIAdapter(registry)

	if got, want := len(unified.GetInfrastructure()), 1; got != want {
		t.Fatalf("GetInfrastructure() count mismatch: got=%d want=%d", got, want)
	}
	if got, want := len(unified.GetWorkloads()), 2; got != want {
		t.Fatalf("GetWorkloads() count mismatch: got=%d want=%d", got, want)
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

type resourceContextAlertProvider struct {
	active  []AlertInfo
	history []ResolvedAlertInfo
}

func (m *resourceContextAlertProvider) GetActiveAlerts() []AlertInfo {
	return m.active
}

func (m *resourceContextAlertProvider) GetRecentlyResolved(minutes int) []ResolvedAlertInfo {
	return m.history
}

func (m *resourceContextAlertProvider) GetAlertsByResource(resourceID string) []AlertInfo {
	out := make([]AlertInfo, 0)
	for _, alert := range m.active {
		if alert.ResourceID == resourceID {
			out = append(out, alert)
		}
	}
	return out
}

func (m *resourceContextAlertProvider) GetAlertHistory(resourceID string, limit int) []ResolvedAlertInfo {
	out := make([]ResolvedAlertInfo, 0)
	for _, alert := range m.history {
		if alert.ResourceID == resourceID {
			out = append(out, alert)
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}
