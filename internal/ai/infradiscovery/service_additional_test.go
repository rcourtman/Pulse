package infradiscovery

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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
	provider := &mockStateProvider{state: models.StateSnapshot{}}
	service := NewService(provider, nil, Config{
		Interval:    10 * time.Millisecond,
		CacheExpiry: time.Millisecond,
	})
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

	provider := &mockStateProvider{state: models.StateSnapshot{}}
	service := NewService(provider, nil, Config{
		Interval:    10 * time.Millisecond,
		CacheExpiry: time.Millisecond,
	})
	service.SetAIAnalyzer(&mockAIAnalyzer{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service.Start(ctx)
	waitFor(t, 500*time.Millisecond, func() bool {
		return !service.GetLastRun().IsZero()
	})
	service.Stop()

	service.Start(ctx)
	waitFor(t, 500*time.Millisecond, func() bool {
		return service.GetStatus()["running"] == true
	})
	service.Stop()
}

func TestForceRefreshUpdatesLastRun(t *testing.T) {
	provider := &mockStateProvider{state: models.StateSnapshot{}}
	service := NewService(provider, nil, DefaultConfig())
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
	service := NewService(&mockStateProvider{}, store, DefaultConfig())

	apps := []DiscoveredApp{
		{
			ID:        "docker:host1:pg",
			Name:      "PostgreSQL",
			RunsIn:    "docker",
			HostID:    "host-1",
			Hostname:  "host1",
			CLIAccess: "docker exec pg psql",
		},
		{
			ID:        "docker:host1:redis",
			Name:      "Redis",
			RunsIn:    "docker",
			HostID:    "host-1",
			Hostname:  "host1",
			CLIAccess: "",
		},
	}

	service.saveDiscoveries(apps)

	knowledgeData, err := store.GetKnowledge("host-1")
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
	provider := &mockStateProvider{
		state: models.StateSnapshot{
			DockerHosts: []models.DockerHost{
				{
					AgentID:  "agent-1",
					Hostname: "host1",
					Containers: []models.DockerContainer{
						{ID: "1", Name: "web", Image: "nginx:latest"},
					},
				},
			},
		},
	}

	analyzer := &mockAIAnalyzer{
		responses: map[string]string{
			"nginx:latest": `{"service_type": "nginx", "service_name": "Nginx", "category": "web", "cli_command": "", "confidence": 0.9, "reasoning": "Web server"}`,
		},
	}

	service := NewService(provider, nil, DefaultConfig())
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
	service := NewService(&mockStateProvider{}, nil, Config{
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

	app := service.analyzeContainer(context.Background(), analyzer, models.DockerContainer{
		ID:    "1",
		Name:  "db",
		Image: "postgres:16",
	}, models.DockerHost{
		AgentID:  "agent-1",
		Hostname: "docker-host",
	})
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
	provider := &mockStateProvider{
		state: models.StateSnapshot{
			DockerHosts: []models.DockerHost{
				{
					AgentID:  "agent-1",
					Hostname: "docker-host",
					Containers: []models.DockerContainer{
						{ID: "1", Name: "slow-service", Image: "slow:latest"},
					},
				},
			},
		},
	}

	service := NewService(provider, nil, Config{
		Interval:          time.Minute,
		CacheExpiry:       time.Hour,
		AIAnalysisTimeout: 20 * time.Millisecond,
	})
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
