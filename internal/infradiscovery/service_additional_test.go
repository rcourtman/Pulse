package infradiscovery

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type contextCheckingAnalyzer struct {
	called bool
	sawNil bool
}

func (a *contextCheckingAnalyzer) AnalyzeForDiscovery(ctx context.Context, prompt string) (string, error) {
	a.called = true
	a.sawNil = ctx == nil
	return `{"service_type":"postgres","service_name":"PostgreSQL","category":"database","cli_command":"docker exec {container} psql -U postgres","confidence":0.95,"reasoning":"postgres image"}`, nil
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

func TestNewServiceNormalizesInvalidConfig(t *testing.T) {
	service := NewService(&mockStateProvider{}, nil, Config{
		Interval:    -1 * time.Second,
		CacheExpiry: 0,
	})

	if service.interval != defaultDiscoveryInterval {
		t.Fatalf("interval = %v, want %v", service.interval, defaultDiscoveryInterval)
	}
	if service.cacheExpiry != defaultCacheExpiry {
		t.Fatalf("cacheExpiry = %v, want %v", service.cacheExpiry, defaultCacheExpiry)
	}
}

func TestNewServiceNilStateProviderUsesEmptyState(t *testing.T) {
	service := NewService(nil, nil, DefaultConfig())
	service.SetAIAnalyzer(&mockAIAnalyzer{})

	apps := service.RunDiscovery(context.Background())
	if len(apps) != 0 {
		t.Fatalf("expected 0 apps from empty state provider, got %d", len(apps))
	}
	if service.GetLastRun().IsZero() {
		t.Fatal("expected lastRun to be updated for empty-state discovery run")
	}
}

func TestRunDiscoveryNormalizesNilContext(t *testing.T) {
	provider := &mockStateProvider{
		state: models.StateSnapshot{
			DockerHosts: []models.DockerHost{
				{
					AgentID:  "agent-1",
					Hostname: "host1",
					Containers: []models.DockerContainer{
						{ID: "1", Name: "db", Image: "postgres:14"},
					},
				},
			},
		},
	}
	analyzer := &contextCheckingAnalyzer{}

	service := NewService(provider, nil, DefaultConfig())
	service.SetAIAnalyzer(analyzer)

	apps := service.RunDiscovery(nil)
	if !analyzer.called {
		t.Fatal("expected analyzer to be called")
	}
	if analyzer.sawNil {
		t.Fatal("expected RunDiscovery to pass non-nil context to analyzer")
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 discovered app, got %d", len(apps))
	}
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
	status := service.GetStatus()
	if running, ok := status["running"].(bool); !ok || !running {
		t.Fatalf("expected running status true, got %v", status["running"])
	}

	waitFor(t, 500*time.Millisecond, func() bool {
		return !service.GetLastRun().IsZero()
	})

	service.Stop()
	status = service.GetStatus()
	if running, ok := status["running"].(bool); !ok || running {
		t.Fatalf("expected running status false, got %v", status["running"])
	}
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
