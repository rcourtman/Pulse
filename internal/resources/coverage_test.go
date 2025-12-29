package resources

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestResourcePlatformDataAndMetrics(t *testing.T) {
	var out struct {
		Value string `json:"value"`
	}

	r := Resource{Name: "name"}
	if err := r.GetPlatformData(&out); err != nil {
		t.Fatalf("expected nil error for empty platform data, got %v", err)
	}

	if err := r.SetPlatformData(make(chan int)); err == nil {
		t.Fatal("expected error for unserializable platform data")
	}

	if err := r.SetPlatformData(struct {
		Value string `json:"value"`
	}{Value: "ok"}); err != nil {
		t.Fatalf("unexpected error setting platform data: %v", err)
	}
	if err := r.GetPlatformData(&out); err != nil {
		t.Fatalf("unexpected error getting platform data: %v", err)
	}
	if out.Value != "ok" {
		t.Fatalf("expected platform data value to be ok, got %q", out.Value)
	}

	r.PlatformData = []byte("{")
	if err := r.GetPlatformData(&out); err == nil {
		t.Fatal("expected error for invalid platform data")
	}

	r.DisplayName = "display"
	if r.EffectiveDisplayName() != "display" {
		t.Fatalf("expected display name to be used")
	}
	r.DisplayName = ""
	if r.EffectiveDisplayName() != "name" {
		t.Fatalf("expected name fallback")
	}

	if r.CPUPercent() != 0 {
		t.Fatalf("expected CPUPercent 0 for nil CPU")
	}
	r.CPU = &MetricValue{Current: 12.5}
	if r.CPUPercent() != 12.5 {
		t.Fatalf("expected CPUPercent 12.5")
	}

	if r.MemoryPercent() != 0 {
		t.Fatalf("expected MemoryPercent 0 for nil memory")
	}
	total := int64(100)
	used := int64(25)
	r.Memory = &MetricValue{Total: &total, Used: &used}
	if r.MemoryPercent() != 25 {
		t.Fatalf("expected MemoryPercent 25")
	}
	r.Memory = &MetricValue{Current: 40}
	if r.MemoryPercent() != 40 {
		t.Fatalf("expected MemoryPercent 40")
	}

	if r.DiskPercent() != 0 {
		t.Fatalf("expected DiskPercent 0 for nil disk")
	}
	totalDisk := int64(200)
	usedDisk := int64(50)
	r.Disk = &MetricValue{Total: &totalDisk, Used: &usedDisk}
	if r.DiskPercent() != 25 {
		t.Fatalf("expected DiskPercent 25")
	}
	r.Disk = &MetricValue{Current: 33}
	if r.DiskPercent() != 33 {
		t.Fatalf("expected DiskPercent 33")
	}

	r.Type = ResourceTypeNode
	if !r.IsInfrastructure() {
		t.Fatalf("expected node to be infrastructure")
	}
	if r.IsWorkload() {
		t.Fatalf("expected node to not be workload")
	}
	r.Type = ResourceTypeVM
	if r.IsInfrastructure() {
		t.Fatalf("expected vm to not be infrastructure")
	}
	if !r.IsWorkload() {
		t.Fatalf("expected vm to be workload")
	}
}

func TestStorePreferredAndSuppressed(t *testing.T) {
	store := NewStore()
	now := time.Now()

	existing := Resource{
		ID:         "agent-1",
		Type:       ResourceTypeHost,
		SourceType: SourceAgent,
		LastSeen:   now,
		Identity: &ResourceIdentity{
			Hostname: "host1",
		},
	}
	store.Upsert(existing)

	incoming := Resource{
		ID:         "api-1",
		Type:       ResourceTypeHost,
		SourceType: SourceAPI,
		LastSeen:   now.Add(1 * time.Minute),
		Identity: &ResourceIdentity{
			Hostname: "host1",
		},
	}
	preferred := store.Upsert(incoming)
	if preferred != existing.ID {
		t.Fatalf("expected preferred ID to be %s, got %s", existing.ID, preferred)
	}
	if !store.IsSuppressed(incoming.ID) {
		t.Fatalf("expected incoming to be suppressed")
	}
	if store.GetPreferredID(incoming.ID) != existing.ID {
		t.Fatalf("expected preferred ID to map to %s", existing.ID)
	}
	if store.GetPreferredID(existing.ID) != existing.ID {
		t.Fatalf("expected preferred ID to return itself")
	}

	got, ok := store.Get(incoming.ID)
	if !ok || got.ID != existing.ID {
		t.Fatalf("expected Get to return preferred resource")
	}

	if store.GetPreferredResourceFor(incoming.ID) == nil {
		t.Fatalf("expected preferred resource for suppressed ID")
	}
	if store.GetPreferredResourceFor(existing.ID) == nil {
		t.Fatalf("expected preferred resource for existing ID")
	}
	if store.GetPreferredResourceFor("missing") != nil {
		t.Fatalf("expected nil for missing resource")
	}

	if !store.IsSamePhysicalMachine(existing.ID, incoming.ID) {
		t.Fatalf("expected IDs to be same physical machine")
	}
	if !store.IsSamePhysicalMachine(incoming.ID, existing.ID) {
		t.Fatalf("expected merged ID to match preferred")
	}
	if store.IsSamePhysicalMachine(existing.ID, "other") {
		t.Fatalf("expected different IDs to not match")
	}
	if !store.IsSamePhysicalMachine(existing.ID, existing.ID) {
		t.Fatalf("expected same ID to match")
	}

	store.Remove(existing.ID)
	if store.IsSuppressed(incoming.ID) {
		t.Fatalf("expected suppression to be cleared after removal")
	}
	if _, ok := store.Get(incoming.ID); ok {
		t.Fatalf("expected Get to fail for removed preferred resource")
	}
}

func TestStoreStatsAndHelpers(t *testing.T) {
	store := NewStore()
	now := time.Now()

	store.Upsert(Resource{
		ID:         "host-1",
		Type:       ResourceTypeHost,
		Status:     StatusOffline,
		SourceType: SourceAgent,
		LastSeen:   now,
		Alerts:     []ResourceAlert{{ID: "a1"}},
		Identity: &ResourceIdentity{
			Hostname: "host-1",
		},
	})
	store.Upsert(Resource{
		ID:         "host-2",
		Type:       ResourceTypeHost,
		Status:     StatusOnline,
		SourceType: SourceAPI,
		LastSeen:   now.Add(time.Minute),
		Identity: &ResourceIdentity{
			Hostname: "host-1",
		},
	})

	stats := store.GetStats()
	if stats.SuppressedResources != 1 {
		t.Fatalf("expected 1 suppressed resource")
	}
	if stats.WithAlerts != 1 {
		t.Fatalf("expected 1 resource with alerts")
	}

	if store.sourceScore(SourceType("other")) != 0 {
		t.Fatalf("expected default source score")
	}

	a := &Resource{ID: "a", SourceType: SourceAPI, LastSeen: now}
	b := &Resource{ID: "b", SourceType: SourceAgent, LastSeen: now.Add(time.Second)}
	if store.preferredResource(a, b) != b {
		t.Fatalf("expected agent resource to be preferred")
	}
	c := &Resource{ID: "c", SourceType: SourceAPI, LastSeen: now.Add(time.Second)}
	if store.preferredResource(a, c) != c {
		t.Fatalf("expected newer resource to be preferred")
	}
	d := &Resource{ID: "d", SourceType: SourceAgent, LastSeen: now}
	e := &Resource{ID: "e", SourceType: SourceAPI, LastSeen: now}
	if store.preferredResource(d, e) != d {
		t.Fatalf("expected higher score resource to be preferred")
	}
	f := &Resource{ID: "f", SourceType: SourceAPI, LastSeen: now.Add(2 * time.Second)}
	g := &Resource{ID: "g", SourceType: SourceAPI, LastSeen: now.Add(1 * time.Second)}
	if store.preferredResource(f, g) != f {
		t.Fatalf("expected newer resource to be preferred when scores equal")
	}

	if store.findDuplicate(&Resource{}) != "" {
		t.Fatalf("expected no duplicate for nil identity")
	}
	if store.findDuplicate(&Resource{Type: ResourceTypeVM, Identity: &ResourceIdentity{Hostname: "host-1"}}) != "" {
		t.Fatalf("expected no duplicate for workload type")
	}
}

func TestStorePreferredSourceAndPolling(t *testing.T) {
	store := NewStore()
	now := time.Now()

	store.Upsert(Resource{
		ID:         "api-1",
		Type:       ResourceTypeHost,
		SourceType: SourceAPI,
		LastSeen:   now,
		Identity: &ResourceIdentity{
			Hostname: "host1",
		},
	})
	if store.HasPreferredSourceForHostname("host1") {
		t.Fatalf("expected no preferred source for API-only hostname")
	}

	store.Upsert(Resource{
		ID:         "hybrid-1",
		Type:       ResourceTypeHost,
		SourceType: SourceHybrid,
		LastSeen:   now.Add(time.Second),
		Identity: &ResourceIdentity{
			Hostname: "host1",
		},
	})
	if !store.HasPreferredSourceForHostname("HOST1") {
		t.Fatalf("expected preferred source for hostname")
	}
	if store.HasPreferredSourceForHostname("") {
		t.Fatalf("expected empty hostname to be false")
	}
	if !store.ShouldSkipAPIPolling("host1") {
		t.Fatalf("expected skip polling for preferred source")
	}
	if store.HasPreferredSourceForHostname("missing") {
		t.Fatalf("expected missing hostname to be false")
	}
}

func TestStoreAgentHostnamesAndRecommendations(t *testing.T) {
	store := NewStore()
	now := time.Now()

	store.Upsert(Resource{
		ID:         "agent-1",
		Type:       ResourceTypeHost,
		SourceType: SourceAgent,
		LastSeen:   now,
		Identity: &ResourceIdentity{
			Hostname: "host1",
		},
	})
	store.Upsert(Resource{
		ID:         "hybrid-1",
		Type:       ResourceTypeHost,
		SourceType: SourceHybrid,
		LastSeen:   now,
		Identity: &ResourceIdentity{
			Hostname: "HOST2",
		},
	})
	store.Upsert(Resource{
		ID:         "api-1",
		Type:       ResourceTypeHost,
		SourceType: SourceAPI,
		LastSeen:   now,
		Identity: &ResourceIdentity{
			Hostname: "host3",
		},
	})
	store.Upsert(Resource{
		ID:         "no-identity",
		Type:       ResourceTypeHost,
		SourceType: SourceAgent,
		LastSeen:   now,
	})
	store.Upsert(Resource{
		ID:         "node-agent",
		Type:       ResourceTypeNode,
		SourceType: SourceAgent,
		LastSeen:   now,
		Identity: &ResourceIdentity{
			Hostname: "host1",
		},
	})

	hostnames := store.GetAgentMonitoredHostnames()
	seen := make(map[string]bool)
	for _, h := range hostnames {
		seen[strings.ToLower(h)] = true
	}
	if !seen["host1"] || !seen["host2"] {
		t.Fatalf("expected host1 and host2 to be monitored, got %v", hostnames)
	}
	if len(seen) != 2 {
		t.Fatalf("expected two unique hostnames")
	}

	recs := store.GetPollingRecommendations()
	if recs["host1"] != 0 {
		t.Fatalf("expected host1 recommendation 0, got %v", recs["host1"])
	}
	if recs["host2"] != 0.5 {
		t.Fatalf("expected host2 recommendation 0.5, got %v", recs["host2"])
	}
	if _, ok := recs["host3"]; ok {
		t.Fatalf("did not expect API-only host to have recommendation")
	}
}

func TestStoreFindContainerHost(t *testing.T) {
	store := NewStore()
	now := time.Now()

	host := Resource{
		ID:         "docker-host-1",
		Type:       ResourceTypeDockerHost,
		Name:       "docker-host",
		SourceType: SourceAgent,
		LastSeen:   now,
		Identity: &ResourceIdentity{
			Hostname: "docker1",
		},
	}
	store.Upsert(host)
	store.Upsert(Resource{
		ID:         "docker-host-1/container-1",
		Type:       ResourceTypeDockerContainer,
		Name:       "web-app",
		ParentID:   "docker-host-1",
		SourceType: SourceAgent,
		LastSeen:   now,
	})

	if store.FindContainerHost("") != "" {
		t.Fatalf("expected empty search to return empty")
	}
	if store.FindContainerHost("missing") != "" {
		t.Fatalf("expected missing container to return empty")
	}
	if store.FindContainerHost("web-app") != "docker1" {
		t.Fatalf("expected host name from identity")
	}
	if store.FindContainerHost("CONTAINER-1") != "docker1" {
		t.Fatalf("expected match by ID")
	}
	if store.FindContainerHost("web") != "docker1" {
		t.Fatalf("expected match by substring")
	}

	store2 := NewStore()
	store2.Upsert(Resource{
		ID:         "container-2",
		Type:       ResourceTypeDockerContainer,
		Name:       "db",
		ParentID:   "missing-host",
		SourceType: SourceAgent,
		LastSeen:   now,
	})
	if store2.FindContainerHost("db") != "" {
		t.Fatalf("expected missing parent to return empty")
	}

	store3 := NewStore()
	store3.Upsert(Resource{
		ID:         "host-no-identity",
		Type:       ResourceTypeDockerHost,
		Name:       "host-name",
		SourceType: SourceAgent,
		LastSeen:   now,
	})
	store3.Upsert(Resource{
		ID:         "host-no-identity/container-3",
		Type:       ResourceTypeDockerContainer,
		Name:       "cache",
		ParentID:   "host-no-identity",
		SourceType: SourceAgent,
		LastSeen:   now,
	})
	if store3.FindContainerHost("cache") != "host-name" {
		t.Fatalf("expected fallback to host name")
	}
}

func TestResourceQueryFiltersAndSorting(t *testing.T) {
	store := NewStore()
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	store.Upsert(Resource{
		ID:         "r1",
		Type:       ResourceTypeVM,
		Name:       "b",
		Status:     StatusRunning,
		ClusterID:  "c1",
		CPU:        &MetricValue{Current: 10},
		Memory:     &MetricValue{Current: 60},
		Disk:       &MetricValue{Current: 20},
		LastSeen:   base.Add(1 * time.Hour),
		Alerts:     []ResourceAlert{{ID: "a1"}},
		SourceType: SourceAPI,
	})
	store.Upsert(Resource{
		ID:         "r2",
		Type:       ResourceTypeNode,
		Name:       "a",
		Status:     StatusOnline,
		ClusterID:  "c1",
		CPU:        &MetricValue{Current: 50},
		Memory:     &MetricValue{Current: 10},
		Disk:       &MetricValue{Current: 90},
		LastSeen:   base.Add(2 * time.Hour),
		SourceType: SourceAPI,
	})
	store.Upsert(Resource{
		ID:         "r3",
		Type:       ResourceTypeVM,
		Name:       "c",
		Status:     StatusOffline,
		ClusterID:  "c2",
		CPU:        &MetricValue{Current: 5},
		Memory:     &MetricValue{Current: 30},
		Disk:       &MetricValue{Current: 10},
		LastSeen:   base.Add(3 * time.Hour),
		SourceType: SourceAPI,
	})

	clustered := store.Query().InCluster("c1").Execute()
	if len(clustered) != 2 {
		t.Fatalf("expected 2 clustered resources, got %d", len(clustered))
	}

	withAlerts := store.Query().WithAlerts().Execute()
	if len(withAlerts) != 1 || withAlerts[0].ID != "r1" {
		t.Fatalf("expected only r1 with alerts")
	}

	sorted := store.Query().SortBy("name", false).Execute()
	if len(sorted) < 2 || sorted[0].Name != "a" {
		t.Fatalf("expected sorted results by name")
	}

	limited := store.Query().SortBy("cpu", true).Offset(1).Limit(1).Execute()
	if len(limited) != 1 {
		t.Fatalf("expected limited results")
	}

	empty := store.Query().Offset(10).Execute()
	if len(empty) != 0 {
		t.Fatalf("expected empty results for large offset")
	}
}

func TestSortResourcesFields(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	resources := []Resource{
		{
			ID:       "r1",
			Type:     ResourceTypeVM,
			Name:     "b",
			Status:   StatusRunning,
			CPU:      &MetricValue{Current: 20},
			Memory:   &MetricValue{Current: 40},
			Disk:     &MetricValue{Current: 30},
			LastSeen: base.Add(1 * time.Hour),
		},
		{
			ID:       "r2",
			Type:     ResourceTypeNode,
			Name:     "a",
			Status:   StatusOffline,
			CPU:      &MetricValue{Current: 50},
			Memory:   &MetricValue{Current: 10},
			Disk:     &MetricValue{Current: 90},
			LastSeen: base.Add(2 * time.Hour),
		},
		{
			ID:       "r3",
			Type:     ResourceTypeContainer,
			Name:     "c",
			Status:   StatusDegraded,
			CPU:      &MetricValue{Current: 5},
			Memory:   &MetricValue{Current: 80},
			Disk:     &MetricValue{Current: 10},
			LastSeen: base.Add(3 * time.Hour),
		},
	}

	single := []Resource{{ID: "only"}}
	sortResources(single, "name", false)

	cases := []struct {
		field string
		desc  bool
		want  string
	}{
		{"name", false, "r2"},
		{"name", true, "r3"},
		{"type", false, "r3"},
		{"type", true, "r1"},
		{"status", false, "r3"},
		{"status", true, "r1"},
		{"cpu", true, "r2"},
		{"cpu", false, "r3"},
		{"memory", true, "r3"},
		{"memory", false, "r2"},
		{"disk", true, "r2"},
		{"disk", false, "r3"},
		{"last_seen", true, "r3"},
		{"last_seen", false, "r1"},
		{"lastseen", true, "r3"},
		{"mem", true, "r3"},
	}

	for _, tc := range cases {
		sorted := append([]Resource(nil), resources...)
		sortResources(sorted, tc.field, tc.desc)
		if sorted[0].ID != tc.want {
			t.Fatalf("sort %s desc=%v expected %s, got %s", tc.field, tc.desc, tc.want, sorted[0].ID)
		}
	}
}

func TestGetTopByMemoryAndDisk(t *testing.T) {
	store := NewStore()
	now := time.Now()

	store.Upsert(Resource{
		ID:       "vm1",
		Type:     ResourceTypeVM,
		Memory:   &MetricValue{Current: 80},
		Disk:     &MetricValue{Current: 30},
		LastSeen: now,
	})
	store.Upsert(Resource{
		ID:       "vm2",
		Type:     ResourceTypeVM,
		Memory:   &MetricValue{Current: 20},
		Disk:     &MetricValue{Current: 90},
		LastSeen: now,
	})
	store.Upsert(Resource{
		ID:       "node1",
		Type:     ResourceTypeNode,
		Memory:   &MetricValue{Current: 60},
		Disk:     &MetricValue{Current: 10},
		LastSeen: now,
	})
	store.Upsert(Resource{
		ID:       "skip-memory",
		Type:     ResourceTypeVM,
		LastSeen: now,
	})
	store.Upsert(Resource{
		ID:       "skip-disk",
		Type:     ResourceTypeVM,
		Disk:     &MetricValue{Current: 0},
		LastSeen: now,
	})

	topMem := store.GetTopByMemory(1, nil)
	if len(topMem) != 1 || topMem[0].ID != "vm1" {
		t.Fatalf("expected vm1 to be top memory")
	}
	topMemVMs := store.GetTopByMemory(10, []ResourceType{ResourceTypeVM})
	if len(topMemVMs) != 2 {
		t.Fatalf("expected 2 VM memory results, got %d", len(topMemVMs))
	}

	topDisk := store.GetTopByDisk(1, nil)
	if len(topDisk) != 1 || topDisk[0].ID != "vm2" {
		t.Fatalf("expected vm2 to be top disk")
	}
	topDiskNodes := store.GetTopByDisk(10, []ResourceType{ResourceTypeNode})
	if len(topDiskNodes) != 1 || topDiskNodes[0].ID != "node1" {
		t.Fatalf("expected node1 to be top disk node")
	}
}

func TestGetTopByCPU_SkipsZero(t *testing.T) {
	store := NewStore()
	now := time.Now()

	store.Upsert(Resource{
		ID:       "skip-cpu-1",
		Type:     ResourceTypeVM,
		LastSeen: now,
	})
	store.Upsert(Resource{
		ID:       "skip-cpu-2",
		Type:     ResourceTypeVM,
		CPU:      &MetricValue{Current: 0},
		LastSeen: now,
	})
	store.Upsert(Resource{
		ID:       "cpu-1",
		Type:     ResourceTypeVM,
		CPU:      &MetricValue{Current: 10},
		LastSeen: now,
	})

	top := store.GetTopByCPU(10, nil)
	if len(top) != 1 || top[0].ID != "cpu-1" {
		t.Fatalf("expected only cpu-1 to be returned")
	}
}

func TestGetRelatedWithChildren(t *testing.T) {
	store := NewStore()
	now := time.Now()

	store.Upsert(Resource{
		ID:       "parent",
		Type:     ResourceTypeNode,
		Name:     "parent",
		LastSeen: now,
	})
	store.Upsert(Resource{
		ID:       "child-1",
		Type:     ResourceTypeVM,
		Name:     "child-1",
		ParentID: "parent",
		LastSeen: now,
	})

	related := store.GetRelated("parent")
	if children, ok := related["children"]; !ok || len(children) != 1 {
		t.Fatalf("expected children for parent")
	}

	if len(store.GetRelated("missing")) != 0 {
		t.Fatalf("expected no related resources for missing ID")
	}
}

func TestResourceSummaryWithDegradedAndAlerts(t *testing.T) {
	store := NewStore()
	now := time.Now()

	store.Upsert(Resource{
		ID:       "healthy",
		Type:     ResourceTypeNode,
		Status:   StatusOnline,
		LastSeen: now,
		CPU:      &MetricValue{Current: 20},
		Memory:   &MetricValue{Current: 50},
	})
	store.Upsert(Resource{
		ID:       "degraded",
		Type:     ResourceTypeVM,
		Status:   StatusDegraded,
		LastSeen: now,
		Alerts:   []ResourceAlert{{ID: "a1"}},
	})
	store.Upsert(Resource{
		ID:       "unknown",
		Type:     ResourceTypeVM,
		Status:   StatusUnknown,
		LastSeen: now,
	})

	summary := store.GetResourceSummary()
	if summary.Degraded != 1 {
		t.Fatalf("expected degraded count 1")
	}
	if summary.Offline != 1 {
		t.Fatalf("expected offline count 1")
	}
	if summary.WithAlerts != 1 {
		t.Fatalf("expected alerts count 1")
	}
}

func TestPopulateFromSnapshotFull(t *testing.T) {
	store := NewStore()
	store.Upsert(Resource{ID: "old-resource", Type: ResourceTypeNode, LastSeen: time.Now()})

	now := time.Now()
	cluster := models.KubernetesCluster{
		ID:       "cluster-1",
		AgentID:  "agent-1",
		Status:   "online",
		LastSeen: now,
		Nodes: []models.KubernetesNode{
			{Name: "node-1", Ready: true},
		},
		Pods: []models.KubernetesPod{
			{Name: "pod-1", Namespace: "default", Phase: "Running", NodeName: "node-1"},
		},
		Deployments: []models.KubernetesDeployment{
			{Name: "dep-1", Namespace: "default", DesiredReplicas: 1, AvailableReplicas: 1},
		},
	}
	dockerHost := models.DockerHost{
		ID:       "docker-1",
		AgentID:  "agent-docker",
		Hostname: "docker-host",
		Status:   "online",
		Memory: models.Memory{
			Total: 100,
			Used:  50,
			Free:  50,
			Usage: 50,
		},
		Disks: []models.Disk{
			{Total: 100, Used: 60, Free: 40, Usage: 60},
		},
		NetworkInterfaces: []models.HostNetworkInterface{
			{Addresses: []string{"10.0.0.1"}, RXBytes: 1, TXBytes: 2},
		},
		Containers: []models.DockerContainer{
			{
				ID:            "container-1",
				Name:          "web",
				State:         "running",
				Status:        "Up",
				CPUPercent:    5,
				MemoryUsage:   50,
				MemoryLimit:   100,
				MemoryPercent: 50,
			},
		},
		LastSeen: now,
	}
	snapshot := models.StateSnapshot{
		DockerHosts: []models.DockerHost{dockerHost},
		KubernetesClusters: []models.KubernetesCluster{
			cluster,
		},
		PBSInstances: []models.PBSInstance{
			{
				ID:               "pbs-1",
				Name:             "pbs",
				Host:             "pbs.local",
				Status:           "online",
				ConnectionHealth: "healthy",
				CPU:              20,
				Memory:           50,
				MemoryTotal:      100,
				MemoryUsed:       50,
				Uptime:           10,
				LastSeen:         now,
			},
		},
		Storage: []models.Storage{
			{
				ID:       "storage-1",
				Name:     "local",
				Instance: "pve1",
				Node:     "node1",
				Total:    100,
				Used:     50,
				Free:     50,
				Usage:    50,
				Active:   true,
				Enabled:  true,
			},
		},
	}

	store.PopulateFromSnapshot(snapshot)

	if _, ok := store.Get("old-resource"); ok {
		t.Fatalf("expected old resource to be removed")
	}

	if len(store.Query().OfType(ResourceTypeDockerHost).Execute()) != 1 {
		t.Fatalf("expected 1 docker host")
	}
	if len(store.Query().OfType(ResourceTypeDockerContainer).Execute()) != 1 {
		t.Fatalf("expected 1 docker container")
	}
	if len(store.Query().OfType(ResourceTypeK8sCluster).Execute()) != 1 {
		t.Fatalf("expected 1 k8s cluster")
	}
	if len(store.Query().OfType(ResourceTypeK8sNode).Execute()) != 1 {
		t.Fatalf("expected 1 k8s node")
	}
	if len(store.Query().OfType(ResourceTypePod).Execute()) != 1 {
		t.Fatalf("expected 1 k8s pod")
	}
	if len(store.Query().OfType(ResourceTypeK8sDeployment).Execute()) != 1 {
		t.Fatalf("expected 1 k8s deployment")
	}
	if len(store.Query().OfType(ResourceTypePBS).Execute()) != 1 {
		t.Fatalf("expected 1 PBS instance")
	}
	if len(store.Query().OfType(ResourceTypeStorage).Execute()) != 1 {
		t.Fatalf("expected 1 storage resource")
	}
}
