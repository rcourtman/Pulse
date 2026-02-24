package unifiedresources

import (
	"sort"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func strPointer(v string) *string {
	return &v
}

func int64Pointer(v int64) *int64 {
	return &v
}

func testRegistry(resources ...Resource) *ResourceRegistry {
	registry := NewRegistry(nil)
	registry.mu.Lock()
	defer registry.mu.Unlock()

	registry.resources = make(map[string]*Resource, len(resources))
	for i := range resources {
		resource := resources[i]
		registry.resources[resource.ID] = &resource
	}
	return registry
}

func resourceIDs(resources []Resource) []string {
	ids := make([]string, len(resources))
	for i, resource := range resources {
		ids[i] = resource.ID
	}
	sort.Strings(ids)
	return ids
}

func TestMonitorAdapterGetPollingRecommendationsAndSkip(t *testing.T) {
	adapter := NewMonitorAdapter(testRegistry(
		Resource{
			ID:      "agent-1",
			Name:    "ignored",
			Sources: []DataSource{SourceAgent},
			Agent:   &AgentData{Hostname: " Agent-Host "},
		},
		Resource{
			ID:       "hybrid-1",
			Name:     "fallback-name-not-used",
			Sources:  []DataSource{SourceAgent, SourceProxmox},
			Identity: ResourceIdentity{Hostnames: []string{" hybrid-host "}},
		},
		Resource{
			ID:      "hybrid-2",
			Name:    "Agent-Host",
			Sources: []DataSource{SourceAgent, SourceProxmox},
		},
		Resource{
			ID:      "api-1",
			Name:    "api-only",
			Sources: []DataSource{SourceProxmox},
			Proxmox: &ProxmoxData{NodeName: "api-only"},
		},
		Resource{
			ID:      "agent-empty",
			Sources: []DataSource{SourceAgent},
		},
	))

	got := adapter.GetPollingRecommendations()
	if len(got) != 2 {
		t.Fatalf("expected 2 recommendations, got %d", len(got))
	}
	if got["agent-host"] != 0 {
		t.Fatalf("expected agent-host multiplier to be 0, got %.2f", got["agent-host"])
	}
	if got["hybrid-host"] != 0.5 {
		t.Fatalf("expected hybrid-host multiplier to be 0.5, got %.2f", got["hybrid-host"])
	}

	if !adapter.ShouldSkipAPIPolling(" AGENT-HOST ") {
		t.Fatalf("expected ShouldSkipAPIPolling to normalize hostname and skip")
	}
	if adapter.ShouldSkipAPIPolling("hybrid-host") {
		t.Fatalf("expected hybrid host to be reduced polling, not skipped")
	}
	if adapter.ShouldSkipAPIPolling("missing-host") {
		t.Fatalf("expected unknown host not to be skipped")
	}
	if adapter.ShouldSkipAPIPolling("  ") {
		t.Fatalf("expected blank hostname not to be skipped")
	}
}

func TestMonitorSourceType(t *testing.T) {
	tests := []struct {
		name    string
		sources []DataSource
		want    string
	}{
		{name: "none defaults to api", sources: nil, want: "api"},
		{name: "agent source", sources: []DataSource{SourceAgent}, want: "agent"},
		{name: "docker source", sources: []DataSource{SourceDocker}, want: "agent"},
		{name: "k8s source", sources: []DataSource{SourceK8s}, want: "agent"},
		{name: "proxmox source", sources: []DataSource{SourceProxmox}, want: "api"},
		{name: "multiple sources are hybrid", sources: []DataSource{SourceAgent, SourceProxmox}, want: "hybrid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := monitorSourceType(tt.sources); got != tt.want {
				t.Fatalf("monitorSourceType(%v) = %q, want %q", tt.sources, got, tt.want)
			}
		})
	}
}

func TestMonitorHostnamePriority(t *testing.T) {
	tests := []struct {
		name     string
		resource Resource
		want     string
	}{
		{
			name: "agent hostname preferred",
			resource: Resource{
				Agent:   &AgentData{Hostname: " agent-host "},
				Docker:  &DockerData{Hostname: "docker-host"},
				Proxmox: &ProxmoxData{NodeName: "node-host"},
				Identity: ResourceIdentity{
					Hostnames: []string{"identity-host"},
				},
			},
			want: "agent-host",
		},
		{
			name: "docker hostname fallback",
			resource: Resource{
				Agent:   &AgentData{Hostname: " "},
				Docker:  &DockerData{Hostname: " docker-host "},
				Proxmox: &ProxmoxData{NodeName: "node-host"},
			},
			want: "docker-host",
		},
		{
			name: "proxmox hostname fallback",
			resource: Resource{
				Agent:   &AgentData{Hostname: ""},
				Docker:  &DockerData{Hostname: " "},
				Proxmox: &ProxmoxData{NodeName: " node-host "},
			},
			want: "node-host",
		},
		{
			name: "identity hostname fallback",
			resource: Resource{
				Identity: ResourceIdentity{Hostnames: []string{" ", " identity-host ", "other"}},
			},
			want: "identity-host",
		},
		{
			name:     "empty resource",
			resource: Resource{},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := monitorHostname(tt.resource); got != tt.want {
				t.Fatalf("monitorHostname() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMonitorAdapterPopulateFromSnapshot(t *testing.T) {
	now := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)
	snapshot := models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:       "host-1",
				Hostname: "host-1.local",
				Status:   "online",
				LastSeen: now,
			},
		},
		ActiveAlerts: []models.Alert{
			{ID: "alert-1", ResourceID: "host-1"},
		},
	}

	adapter := NewMonitorAdapter(NewRegistry(nil))
	adapter.PopulateFromSnapshot(snapshot)

	if len(adapter.GetAll()) == 0 {
		t.Fatalf("expected PopulateFromSnapshot to ingest resources")
	}
	if len(adapter.activeAlerts) != 1 {
		t.Fatalf("expected 1 active alert, got %d", len(adapter.activeAlerts))
	}
	if adapter.activeAlerts[0].ID != "alert-1" {
		t.Fatalf("expected copied alert ID alert-1, got %q", adapter.activeAlerts[0].ID)
	}

	snapshot.ActiveAlerts[0].ID = "mutated"
	if adapter.activeAlerts[0].ID != "alert-1" {
		t.Fatalf("expected active alerts to be copied from snapshot")
	}

	nilRegistryAdapter := &MonitorAdapter{}
	nilRegistryAdapter.PopulateFromSnapshot(snapshot)
	if len(nilRegistryAdapter.activeAlerts) != 0 {
		t.Fatalf("expected nil-registry adapter to ignore snapshot")
	}
}

func TestMonitorAdapterPopulateSupplementalRecords(t *testing.T) {
	now := time.Date(2026, 2, 21, 10, 0, 0, 0, time.UTC)
	adapter := NewMonitorAdapter(NewRegistry(nil))

	customSource := DataSource("xcp")
	adapter.PopulateSupplementalRecords(customSource, []IngestRecord{
		{
			SourceID: "xcp-host-1",
			Resource: Resource{
				Type:     ResourceTypeHost,
				Name:     "xcp-host-1",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"xcp-host-1"}},
		},
	})

	resources := adapter.GetAll()
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource after supplemental ingest, got %d", len(resources))
	}
	if resources[0].Name != "xcp-host-1" {
		t.Fatalf("expected resource name xcp-host-1, got %q", resources[0].Name)
	}
	if len(resources[0].Sources) != 1 || resources[0].Sources[0] != customSource {
		t.Fatalf("expected source %q, got %#v", customSource, resources[0].Sources)
	}

	// Nil/empty inputs should be ignored safely.
	adapter.PopulateSupplementalRecords("", []IngestRecord{{SourceID: "ignored"}})
	adapter.PopulateSupplementalRecords(customSource, nil)
	nilRegistryAdapter := &MonitorAdapter{}
	nilRegistryAdapter.PopulateSupplementalRecords(customSource, []IngestRecord{{SourceID: "ignored"}})
}

func TestUnifiedAIAdapterClassificationAndStats(t *testing.T) {
	registry := testRegistry(
		Resource{ID: "host-1", Type: ResourceTypeHost, Status: StatusOnline, Sources: []DataSource{SourceAgent}},
		Resource{ID: "cluster-1", Type: ResourceTypeK8sCluster, Status: StatusOnline, Sources: []DataSource{SourceK8s}},
		Resource{ID: "node-1", Type: ResourceTypeK8sNode, Status: StatusWarning, Sources: []DataSource{SourceK8s}},
		Resource{ID: "vm-1", Type: ResourceTypeVM, Status: StatusOnline, Sources: []DataSource{SourceProxmox}},
		Resource{ID: "lxc-1", Type: ResourceTypeLXC, Status: StatusOnline, Sources: []DataSource{SourceProxmox}},
		Resource{ID: "ct-1", Type: ResourceTypeContainer, Status: StatusOnline, Sources: []DataSource{SourceDocker}},
		Resource{ID: "pod-1", Type: ResourceTypePod, Status: StatusOffline, Sources: []DataSource{SourceK8s}},
		Resource{ID: "dep-1", Type: ResourceTypeK8sDeployment, Status: StatusOnline, Sources: []DataSource{SourceK8s}},
		Resource{ID: "storage-1", Type: ResourceTypeStorage, Status: StatusOnline, Sources: []DataSource{SourceProxmox}},
	)
	adapter := NewUnifiedAIAdapter(registry)

	infrastructure := adapter.GetInfrastructure()
	if len(infrastructure) != 3 {
		t.Fatalf("expected 3 infrastructure resources, got %d", len(infrastructure))
	}
	workloads := adapter.GetWorkloads()
	if len(workloads) != 5 {
		t.Fatalf("expected 5 workload resources, got %d", len(workloads))
	}
	containers := adapter.GetByType(ResourceTypeContainer)
	if len(containers) != 1 || containers[0].ID != "ct-1" {
		t.Fatalf("expected container lookup to return ct-1, got %#v", containers)
	}

	stats := adapter.GetStats()
	if stats.Total != 9 {
		t.Fatalf("expected stats total 9, got %d", stats.Total)
	}
	if stats.ByType[ResourceTypeHost] != 1 || stats.ByType[ResourceTypeVM] != 1 {
		t.Fatalf("unexpected ByType stats: %#v", stats.ByType)
	}
	if stats.ByStatus[StatusOffline] != 1 {
		t.Fatalf("expected one offline resource, got %#v", stats.ByStatus)
	}

	var nilAdapter *UnifiedAIAdapter
	nilStats := nilAdapter.GetStats()
	if nilStats.Total != 0 || nilStats.ByType == nil || nilStats.ByStatus == nil || nilStats.BySource == nil {
		t.Fatalf("expected nil adapter stats to return initialized empty maps")
	}
}

func TestUnifiedAIAdapterGetTopByMetric(t *testing.T) {
	registry := testRegistry(
		Resource{
			ID:   "vm-z",
			Type: ResourceTypeVM,
			Name: "Zulu",
			Metrics: &ResourceMetrics{
				CPU: &MetricValue{Percent: 90},
				Memory: &MetricValue{
					Used:  int64Pointer(8),
					Total: int64Pointer(10),
				},
			},
		},
		Resource{
			ID:   "vm-a",
			Type: ResourceTypeVM,
			Name: "alpha",
			Metrics: &ResourceMetrics{
				CPU:    &MetricValue{Value: 90},
				Memory: &MetricValue{Percent: 60},
				Disk:   &MetricValue{Percent: 70},
			},
		},
		Resource{
			ID:   "vm-b",
			Type: ResourceTypeVM,
			Name: "beta",
			Metrics: &ResourceMetrics{
				CPU: &MetricValue{
					Used:  int64Pointer(30),
					Total: int64Pointer(60),
				},
				Memory: &MetricValue{Percent: 40},
			},
		},
		Resource{
			ID:   "host-1",
			Type: ResourceTypeHost,
			Name: "host",
			Metrics: &ResourceMetrics{
				CPU: &MetricValue{Percent: 99},
			},
		},
		Resource{
			ID:      "vm-zero",
			Type:    ResourceTypeVM,
			Name:    "zero",
			Metrics: &ResourceMetrics{CPU: &MetricValue{}},
		},
	)
	adapter := NewUnifiedAIAdapter(registry)

	topCPU := adapter.GetTopByCPU(0, []ResourceType{ResourceTypeVM})
	if len(topCPU) != 3 {
		t.Fatalf("expected 3 VMs with positive CPU metrics, got %d", len(topCPU))
	}
	if topCPU[0].ID != "vm-a" || topCPU[1].ID != "vm-z" || topCPU[2].ID != "vm-b" {
		t.Fatalf("unexpected CPU ranking order: %#v", resourceIDs(topCPU))
	}

	topCPULimit := adapter.GetTopByCPU(1, []ResourceType{ResourceTypeVM})
	if len(topCPULimit) != 1 || topCPULimit[0].ID != "vm-a" {
		t.Fatalf("expected limited CPU ranking to include vm-a, got %#v", topCPULimit)
	}

	topMemory := adapter.GetTopByMemory(0, nil)
	if len(topMemory) != 3 {
		t.Fatalf("expected 3 resources with positive memory metrics, got %d", len(topMemory))
	}
	if topMemory[0].ID != "vm-z" {
		t.Fatalf("expected vm-z to lead memory ranking, got %q", topMemory[0].ID)
	}

	topDisk := adapter.GetTopByDisk(0, []ResourceType{ResourceTypeVM})
	if len(topDisk) != 1 || topDisk[0].ID != "vm-a" {
		t.Fatalf("expected disk ranking to include vm-a only, got %#v", topDisk)
	}
}

func TestUnifiedAIAdapterGetRelated(t *testing.T) {
	registry := testRegistry(
		Resource{ID: "parent", Type: ResourceTypeHost, Name: "parent-host"},
		Resource{ID: "child", Type: ResourceTypeContainer, Name: "child", ParentID: strPointer("parent")},
		Resource{ID: "sibling", Type: ResourceTypeVM, Name: "sibling", ParentID: strPointer("parent")},
		Resource{ID: "grandchild", Type: ResourceTypeLXC, Name: "grandchild", ParentID: strPointer("child")},
		Resource{ID: "other", Type: ResourceTypeStorage, Name: "other"},
	)
	adapter := NewUnifiedAIAdapter(registry)

	childRelated := adapter.GetRelated("child")
	if len(childRelated["parent"]) != 1 || childRelated["parent"][0].ID != "parent" {
		t.Fatalf("expected parent relation for child, got %#v", childRelated["parent"])
	}
	if len(childRelated["siblings"]) != 1 || childRelated["siblings"][0].ID != "sibling" {
		t.Fatalf("expected one sibling for child, got %#v", childRelated["siblings"])
	}
	if len(childRelated["children"]) != 1 || childRelated["children"][0].ID != "grandchild" {
		t.Fatalf("expected one grandchild relation, got %#v", childRelated["children"])
	}

	parentRelated := adapter.GetRelated("parent")
	if len(parentRelated["parent"]) != 0 {
		t.Fatalf("expected parent resource to have no parent relation")
	}
	childIDs := resourceIDs(parentRelated["children"])
	if len(childIDs) != 2 || childIDs[0] != "child" || childIDs[1] != "sibling" {
		t.Fatalf("expected parent children [child sibling], got %#v", childIDs)
	}

	if got := adapter.GetRelated("missing"); len(got) != 0 {
		t.Fatalf("expected missing resource relations to be empty, got %#v", got)
	}
	if got := adapter.GetRelated("  "); len(got) != 0 {
		t.Fatalf("expected blank resource ID relations to be empty, got %#v", got)
	}
}

func TestUnifiedAIAdapterFindContainerHost(t *testing.T) {
	registry := testRegistry(
		Resource{
			ID:    "host-a",
			Type:  ResourceTypeHost,
			Name:  "host-a-name",
			Agent: &AgentData{Hostname: "agent-host-a"},
		},
		Resource{
			ID:       "ctr-a",
			Type:     ResourceTypeContainer,
			Name:     "web-app",
			ParentID: strPointer("host-a"),
			Docker:   &DockerData{ContainerID: "abc123"},
		},
		Resource{
			ID:       "host-b",
			Type:     ResourceTypeHost,
			Name:     "host-b-name",
			Identity: ResourceIdentity{Hostnames: []string{"identity-host-b"}},
		},
		Resource{
			ID:       "vm-b",
			Type:     ResourceTypeVM,
			Name:     "db-vm",
			ParentID: strPointer("host-b"),
		},
		Resource{
			ID:   "host-c",
			Type: ResourceTypeHost,
			Name: "parent-c-name",
		},
		Resource{
			ID:       "lxc-c",
			Type:     ResourceTypeLXC,
			Name:     "cache-lxc",
			ParentID: strPointer("host-c"),
		},
		Resource{
			ID:   "host-d",
			Type: ResourceTypeHost,
		},
		Resource{
			ID:       "ctr-d",
			Type:     ResourceTypeContainer,
			Name:     "orphan-ish",
			ParentID: strPointer("host-d"),
		},
		Resource{
			ID:       "ctr-missing",
			Type:     ResourceTypeContainer,
			Name:     "missing-parent",
			ParentID: strPointer("missing-parent-id"),
		},
		Resource{
			ID:       "pod-e",
			Type:     ResourceTypePod,
			Name:     "pod-e",
			ParentID: strPointer("host-a"),
		},
	)
	adapter := NewUnifiedAIAdapter(registry)

	tests := []struct {
		name  string
		query string
		want  string
	}{
		{name: "exact container name", query: "web-app", want: "agent-host-a"},
		{name: "exact container id", query: "abc123", want: "agent-host-a"},
		{name: "partial container id", query: "bc12", want: "agent-host-a"},
		{name: "vm fallback to identity hostname", query: "db-vm", want: "identity-host-b"},
		{name: "lxc fallback to parent name", query: "cache-lxc", want: "parent-c-name"},
		{name: "fallback to parent id", query: "orphan-ish", want: "host-d"},
		{name: "missing parent", query: "missing-parent", want: ""},
		{name: "non-workload type ignored", query: "pod-e", want: ""},
		{name: "unknown query", query: "does-not-exist", want: ""},
		{name: "blank query", query: "  ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adapter.FindContainerHost(tt.query); got != tt.want {
				t.Fatalf("FindContainerHost(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestMetricPercent(t *testing.T) {
	tests := []struct {
		name   string
		metric *MetricValue
		want   float64
	}{
		{name: "nil", metric: nil, want: 0},
		{name: "percent has priority", metric: &MetricValue{Percent: 75, Value: 10}, want: 75},
		{name: "value fallback", metric: &MetricValue{Value: 55}, want: 55},
		{name: "used total fallback", metric: &MetricValue{Used: int64Pointer(25), Total: int64Pointer(50)}, want: 50},
		{name: "invalid total", metric: &MetricValue{Used: int64Pointer(25), Total: int64Pointer(0)}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := metricPercent(tt.metric); got != tt.want {
				t.Fatalf("metricPercent(%#v) = %v, want %v", tt.metric, got, tt.want)
			}
		})
	}
}
