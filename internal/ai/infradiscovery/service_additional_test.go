package infradiscovery

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type blockingAIAnalyzer struct{}

func (blockingAIAnalyzer) AnalyzeForDiscovery(ctx context.Context, prompt string) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func waitFor(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func TestStartStopUpdatesStatus(t *testing.T) {
	service := NewService(nil, Config{
		Interval:    10 * time.Millisecond,
		CacheExpiry: time.Millisecond,
	})
	service.SetReadState(&mockReadState{})
	service.SetAIAnalyzer(&mockAIAnalyzer{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service.Start(ctx)
	status := service.GetStatusSnapshot()
	if !status.Running {
		t.Fatalf("expected running status true, got %v", status.Running)
	}

	waitFor(t, 500*time.Millisecond, func() bool {
		return !service.GetLastRun().IsZero()
	})

	service.Stop()
	status = service.GetStatusSnapshot()
	if status.Running {
		t.Fatalf("expected running status false, got %v", status.Running)
	}
}

func TestStartStopCanRestartSafely(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected restart to be panic-free, got panic: %v", r)
		}
	}()

	service := NewService(nil, Config{
		Interval:    10 * time.Millisecond,
		CacheExpiry: time.Millisecond,
	})
	service.SetAIAnalyzer(&mockAIAnalyzer{})
	service.SetReadState(&mockReadState{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service.Start(ctx)
	waitFor(t, 500*time.Millisecond, func() bool {
		return !service.GetLastRun().IsZero()
	})
	service.Stop()

	service.Start(ctx)
	waitFor(t, 500*time.Millisecond, func() bool {
		return service.GetStatusSnapshot().Running
	})
	service.Stop()
}

func TestForceRefreshUpdatesLastRun(t *testing.T) {
	service := NewService(nil, DefaultConfig())
	service.SetReadState(&mockReadState{})
	service.SetAIAnalyzer(&mockAIAnalyzer{})

	service.ForceRefresh(context.Background())

	waitFor(t, 500*time.Millisecond, func() bool {
		return !service.GetLastRun().IsZero()
	})
}

func TestSaveDiscoveriesWritesKnowledge(t *testing.T) {
	store, err := knowledge.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("create knowledge store: %v", err)
	}
	service := NewService(store, DefaultConfig())

	apps := []DiscoveredApp{
		{
			ID:        "docker:host1:pg",
			Name:      "PostgreSQL",
			RunsIn:    "docker",
			TargetID:  "agent-1",
			Hostname:  "host1",
			CLIAccess: "docker exec pg psql",
		},
		{
			ID:        "docker:host1:redis",
			Name:      "Redis",
			RunsIn:    "docker",
			TargetID:  "agent-1",
			Hostname:  "host1",
			CLIAccess: "",
		},
	}

	service.saveDiscoveries(apps)

	knowledgeData, err := store.GetKnowledge("agent-1")
	if err != nil {
		t.Fatalf("load knowledge: %v", err)
	}
	if knowledgeData == nil || len(knowledgeData.Notes) != 2 {
		t.Fatalf("expected 2 notes, got %+v", knowledgeData)
	}

	var sawCLI, sawNoCLI bool
	for _, note := range knowledgeData.Notes {
		if strings.Contains(note.Content, "CLI access:") {
			sawCLI = true
		}
		if strings.Contains(note.Content, "No CLI access available.") {
			sawNoCLI = true
		}
	}
	if !sawCLI || !sawNoCLI {
		t.Fatalf("expected notes with and without CLI access, got %+v", knowledgeData.Notes)
	}
}

func TestGetDiscoveriesReturnsCopy(t *testing.T) {
	rs := readStateFromDockerHosts([]models.DockerHost{
		{
			ID:       "agent-1",
			AgentID:  "agent-1",
			Hostname: "host1",
			Containers: []models.DockerContainer{
				{ID: "1", Name: "web", Image: "nginx:latest"},
			},
		},
	})

	analyzer := &mockAIAnalyzer{
		responses: map[string]string{
			"nginx:latest": `{"service_type": "nginx", "service_name": "Nginx", "category": "web", "cli_command": "", "confidence": 0.9, "reasoning": "Web server"}`,
		},
	}

	service := NewService(nil, DefaultConfig())
	service.SetReadState(rs)
	service.SetAIAnalyzer(analyzer)
	service.RunDiscovery(context.Background())

	first := service.GetDiscoveries()
	if len(first) != 1 {
		t.Fatalf("expected 1 discovery, got %d", len(first))
	}
	first[0].Name = "changed"

	second := service.GetDiscoveries()
	if len(second) != 1 {
		t.Fatalf("expected 1 discovery, got %d", len(second))
	}
	if second[0].Name == "changed" {
		t.Fatalf("expected discoveries to be immutable copy, got %v", second[0].Name)
	}
}

func TestAnalyzeContainer_StaleCacheEntryExpiresPerImage(t *testing.T) {
	service := NewService(nil, Config{
		Interval:          time.Minute,
		CacheExpiry:       time.Minute,
		AIAnalysisTimeout: time.Second,
	})

	analyzer := &mockAIAnalyzer{
		responses: map[string]string{
			"postgres:16": `{"service_type":"postgres","service_name":"PostgreSQL","category":"database","cli_command":"docker exec {container} psql","confidence":0.95,"reasoning":"fresh result"}`,
		},
	}
	service.SetAIAnalyzer(analyzer)

	service.analysisCache["postgres:16"] = &analysisCacheEntry{
		result: &DiscoveryResult{
			ServiceType: "redis",
			ServiceName: "Redis",
			Category:    "cache",
			Confidence:  0.9,
			Reasoning:   "stale cache",
		},
		cachedAt: time.Now().Add(-2 * time.Minute),
	}
	service.analysisCache["nginx:latest"] = &analysisCacheEntry{
		result: &DiscoveryResult{
			ServiceType: "nginx",
			ServiceName: "Nginx",
			Category:    "web",
			Confidence:  0.9,
			Reasoning:   "fresh cache",
		},
		cachedAt: time.Now(),
	}

	app := service.analyzeContainer(context.Background(), analyzer,
		dockerContainerViewFromModel(models.DockerContainer{
			ID:    "1",
			Name:  "db",
			Image: "postgres:16",
		}),
		dockerHostViewFromModel(models.DockerHost{
			AgentID:  "agent-1",
			Hostname: "docker-host",
		}),
	)
	if app == nil {
		t.Fatalf("expected discovery from refreshed AI analysis")
	}
	if app.Type != "postgres" {
		t.Fatalf("expected refreshed service type postgres, got %q", app.Type)
	}
	if analyzer.callCount != 1 {
		t.Fatalf("expected analyzer call for stale entry, got %d", analyzer.callCount)
	}
}

func TestRunDiscovery_AIAnalysisTimeout(t *testing.T) {
	rs := readStateFromDockerHosts([]models.DockerHost{
		{
			ID:       "agent-1",
			AgentID:  "agent-1",
			Hostname: "docker-host",
			Containers: []models.DockerContainer{
				{ID: "1", Name: "slow-service", Image: "slow:latest"},
			},
		},
	})

	service := NewService(nil, Config{
		Interval:          time.Minute,
		CacheExpiry:       time.Hour,
		AIAnalysisTimeout: 20 * time.Millisecond,
	})
	service.SetReadState(rs)
	service.SetAIAnalyzer(blockingAIAnalyzer{})

	start := time.Now()
	apps := service.RunDiscovery(context.Background())
	elapsed := time.Since(start)

	if len(apps) != 0 {
		t.Fatalf("expected timeout to skip discovery, got %d apps", len(apps))
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("expected run to be bounded by AI timeout, took %v", elapsed)
	}
}

// mockReadState implements unifiedresources.ReadState for testing the ReadState path.
type mockReadState struct {
	dockerHosts      []*unifiedresources.DockerHostView
	dockerContainers []*unifiedresources.DockerContainerView
}

func (m *mockReadState) VMs() []*unifiedresources.VMView                 { return nil }
func (m *mockReadState) Containers() []*unifiedresources.ContainerView   { return nil }
func (m *mockReadState) Nodes() []*unifiedresources.NodeView             { return nil }
func (m *mockReadState) Hosts() []*unifiedresources.HostView             { return nil }
func (m *mockReadState) DockerHosts() []*unifiedresources.DockerHostView { return m.dockerHosts }
func (m *mockReadState) DockerContainers() []*unifiedresources.DockerContainerView {
	return m.dockerContainers
}
func (m *mockReadState) StoragePools() []*unifiedresources.StoragePoolView     { return nil }
func (m *mockReadState) PBSInstances() []*unifiedresources.PBSInstanceView     { return nil }
func (m *mockReadState) PMGInstances() []*unifiedresources.PMGInstanceView     { return nil }
func (m *mockReadState) K8sClusters() []*unifiedresources.K8sClusterView       { return nil }
func (m *mockReadState) K8sNodes() []*unifiedresources.K8sNodeView             { return nil }
func (m *mockReadState) Pods() []*unifiedresources.PodView                     { return nil }
func (m *mockReadState) K8sDeployments() []*unifiedresources.K8sDeploymentView { return nil }
func (m *mockReadState) Workloads() []*unifiedresources.WorkloadView           { return nil }
func (m *mockReadState) Infrastructure() []*unifiedresources.InfrastructureView {
	return nil
}

func TestRunDiscovery_ReadStatePath(t *testing.T) {
	hostView := dockerHostViewFromModel(models.DockerHost{
		ID:       "docker-host-1",
		AgentID:  "agent-1",
		Hostname: "docker-host",
	})
	// Build a container view with HostSourceID wired to the host.
	ctResource := &unifiedresources.Resource{
		ID:   "ct-1",
		Name: "mydb",
		Type: unifiedresources.ResourceTypeAppContainer,
		Docker: &unifiedresources.DockerData{
			ContainerID:    "ct-1",
			HostSourceID:   "docker-host-1",
			Image:          "postgres:14",
			ContainerState: "running",
		},
	}
	ctViewVal := unifiedresources.NewDockerContainerView(ctResource)
	ctView := &ctViewVal

	rs := &mockReadState{
		dockerHosts:      []*unifiedresources.DockerHostView{hostView},
		dockerContainers: []*unifiedresources.DockerContainerView{ctView},
	}

	analyzer := &mockAIAnalyzer{
		responses: map[string]string{
			"postgres:14": `{"service_type":"postgres","service_name":"PostgreSQL","category":"database","cli_command":"docker exec {container} psql","confidence":0.95,"reasoning":"PostgreSQL"}`,
		},
	}

	// No StateProvider — ReadState only.
	service := NewService(nil, DefaultConfig())
	service.SetReadState(rs)
	service.SetAIAnalyzer(analyzer)

	apps := service.RunDiscovery(context.Background())
	if len(apps) != 1 {
		t.Fatalf("RunDiscovery() via ReadState returned %d apps, want 1", len(apps))
	}
	if apps[0].Type != "postgres" {
		t.Fatalf("Type = %q, want postgres", apps[0].Type)
	}
	if apps[0].Hostname != "docker-host" {
		t.Fatalf("Hostname = %q, want docker-host", apps[0].Hostname)
	}
}

func TestRunDiscovery_ReadStateSkipsNilEntries(t *testing.T) {
	hostView := dockerHostViewFromModel(models.DockerHost{
		ID:       "docker-host-1",
		AgentID:  "agent-1",
		Hostname: "docker-host",
	})
	ctResource := &unifiedresources.Resource{
		ID:   "ct-1",
		Name: "mydb",
		Type: unifiedresources.ResourceTypeAppContainer,
		Docker: &unifiedresources.DockerData{
			ContainerID:    "ct-1",
			HostSourceID:   "docker-host-1",
			Image:          "postgres:14",
			ContainerState: "running",
		},
	}
	ctViewVal := unifiedresources.NewDockerContainerView(ctResource)
	ctView := &ctViewVal

	// Include nil entries alongside valid ones.
	rs := &mockReadState{
		dockerHosts:      []*unifiedresources.DockerHostView{nil, hostView, nil},
		dockerContainers: []*unifiedresources.DockerContainerView{nil, ctView, nil},
	}

	analyzer := &mockAIAnalyzer{
		responses: map[string]string{
			"postgres:14": `{"service_type":"postgres","service_name":"PostgreSQL","category":"database","cli_command":"","confidence":0.95,"reasoning":"PostgreSQL"}`,
		},
	}

	service := NewService(nil, DefaultConfig())
	service.SetReadState(rs)
	service.SetAIAnalyzer(analyzer)

	apps := service.RunDiscovery(context.Background())
	if len(apps) != 1 {
		t.Fatalf("RunDiscovery() with nil entries returned %d apps, want 1", len(apps))
	}
}

func TestRunDiscovery_ReadStateSkipsUnresolvedHost(t *testing.T) {
	// Container references a host that doesn't exist in the hosts list.
	ctResource := &unifiedresources.Resource{
		ID:   "ct-orphan",
		Name: "orphan",
		Type: unifiedresources.ResourceTypeAppContainer,
		Docker: &unifiedresources.DockerData{
			ContainerID:    "ct-orphan",
			HostSourceID:   "nonexistent-host",
			Image:          "redis:7",
			ContainerState: "running",
		},
	}
	ctViewVal := unifiedresources.NewDockerContainerView(ctResource)
	ctView := &ctViewVal

	rs := &mockReadState{
		dockerHosts:      nil, // no hosts at all
		dockerContainers: []*unifiedresources.DockerContainerView{ctView},
	}

	analyzer := &mockAIAnalyzer{
		responses: map[string]string{
			"redis:7": `{"service_type":"redis","service_name":"Redis","category":"cache","cli_command":"","confidence":0.9,"reasoning":"Redis"}`,
		},
	}

	service := NewService(nil, DefaultConfig())
	service.SetReadState(rs)
	service.SetAIAnalyzer(analyzer)

	apps := service.RunDiscovery(context.Background())
	if len(apps) != 0 {
		t.Fatalf("RunDiscovery() with unresolved host returned %d apps, want 0", len(apps))
	}
	if analyzer.callCount != 0 {
		t.Fatalf("analyzer should not be called for orphaned containers, got %d calls", analyzer.callCount)
	}
}

func TestRunDiscovery_ReadStateSkipsEmptyHostSourceID(t *testing.T) {
	// Host with empty HostSourceID should not pollute the lookup map.
	emptyHost := dockerHostViewFromModel(models.DockerHost{
		ID:       "",
		AgentID:  "agent-empty",
		Hostname: "empty-host",
	})
	realHost := dockerHostViewFromModel(models.DockerHost{
		ID:       "real-host-1",
		AgentID:  "agent-1",
		Hostname: "real-host",
	})

	// Container with empty HostSourceID should be skipped (unresolved),
	// not matched to the empty-ID host.
	ctEmpty := &unifiedresources.Resource{
		ID:   "ct-empty",
		Name: "mystery",
		Type: unifiedresources.ResourceTypeAppContainer,
		Docker: &unifiedresources.DockerData{
			ContainerID:    "ct-empty",
			HostSourceID:   "",
			Image:          "nginx:latest",
			ContainerState: "running",
		},
	}
	ctEmptyView := unifiedresources.NewDockerContainerView(ctEmpty)

	// Container with valid HostSourceID should be processed normally.
	ctValid := &unifiedresources.Resource{
		ID:   "ct-valid",
		Name: "mydb",
		Type: unifiedresources.ResourceTypeAppContainer,
		Docker: &unifiedresources.DockerData{
			ContainerID:    "ct-valid",
			HostSourceID:   "real-host-1",
			Image:          "postgres:14",
			ContainerState: "running",
		},
	}
	ctValidView := unifiedresources.NewDockerContainerView(ctValid)

	rs := &mockReadState{
		dockerHosts:      []*unifiedresources.DockerHostView{emptyHost, realHost},
		dockerContainers: []*unifiedresources.DockerContainerView{&ctEmptyView, &ctValidView},
	}

	analyzer := &mockAIAnalyzer{
		responses: map[string]string{
			"nginx:latest": `{"service_type":"nginx","service_name":"Nginx","category":"web","cli_command":"","confidence":0.9,"reasoning":"Nginx"}`,
			"postgres:14":  `{"service_type":"postgres","service_name":"PostgreSQL","category":"database","cli_command":"","confidence":0.95,"reasoning":"PostgreSQL"}`,
		},
	}

	service := NewService(nil, DefaultConfig())
	service.SetReadState(rs)
	service.SetAIAnalyzer(analyzer)

	apps := service.RunDiscovery(context.Background())
	// Only the valid container (postgres on real-host) should be discovered.
	// The empty-HostSourceID container should be skipped.
	if len(apps) != 1 {
		t.Fatalf("RunDiscovery() returned %d apps, want 1 (only valid host container)", len(apps))
	}
	if apps[0].Type != "postgres" {
		t.Fatalf("Type = %q, want postgres", apps[0].Type)
	}
}
