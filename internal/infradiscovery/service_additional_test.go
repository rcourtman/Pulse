package infradiscovery

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type blockingAIAnalyzer struct {
	response  string
	startedCh chan struct{}
	releaseCh chan struct{}
	callCount int32
}

func (b *blockingAIAnalyzer) AnalyzeForDiscovery(ctx context.Context, prompt string) (string, error) {
	atomic.AddInt32(&b.callCount, 1)
	select {
	case b.startedCh <- struct{}{}:
	default:
	}

	select {
	case <-b.releaseCh:
	case <-ctx.Done():
		return "", ctx.Err()
	}

	return b.response, nil
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

func TestStartStopRestartStopIsSafe(t *testing.T) {
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

	firstRun := service.GetLastRun()
	if firstRun.IsZero() {
		t.Fatal("expected first discovery run timestamp")
	}

	service.Start(ctx)
	waitFor(t, 500*time.Millisecond, func() bool {
		return service.GetLastRun().After(firstRun)
	})
	service.Stop()

	status := service.GetStatus()
	if running, ok := status["running"].(bool); !ok || running {
		t.Fatalf("expected running status false after second stop, got %v", status["running"])
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

func TestSaveDiscoveriesHandlesKnowledgeWriteErrors(t *testing.T) {
	root := t.TempDir()
	store, err := knowledge.NewStore(root)
	if err != nil {
		t.Fatalf("create knowledge store: %v", err)
	}

	knowledgeDir := filepath.Join(root, "knowledge")
	if err := os.Chmod(knowledgeDir, 0500); err != nil {
		t.Fatalf("chmod knowledge dir: %v", err)
	}
	defer func() {
		_ = os.Chmod(knowledgeDir, 0700)
	}()

	service := NewService(&mockStateProvider{}, store, DefaultConfig())
	service.saveDiscoveries([]DiscoveredApp{
		{
			ID:        "docker:host1:pg",
			Name:      "PostgreSQL",
			RunsIn:    "docker",
			HostID:    "host-1",
			Hostname:  "host1",
			CLIAccess: "docker exec pg psql",
		},
	})
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

func TestRunDiscoverySkipsOverlappingRuns(t *testing.T) {
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

	tests := []struct {
		name     string
		analyzer AIAnalyzer
	}{
		{
			name: "analyzer error",
			analyzer: analyzerFunc(func(ctx context.Context, prompt string) (string, error) {
				return "", errors.New("ai unavailable")
			}),
		},
		{
			name: "invalid ai response",
			analyzer: analyzerFunc(func(ctx context.Context, prompt string) (string, error) {
				return "this is not json", nil
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(provider, nil, DefaultConfig())
			service.SetAIAnalyzer(tt.analyzer)

			apps := service.RunDiscovery(context.Background())
			if len(apps) != 0 {
				t.Fatalf("expected 0 discovered apps, got %d", len(apps))
			}
		})
	}
}

func TestRunDiscoveryExtractsPublicAndPrivatePorts(t *testing.T) {
	provider := &mockStateProvider{
		state: models.StateSnapshot{
			DockerHosts: []models.DockerHost{
				{
					AgentID:  "agent-1",
					Hostname: "host1",
					Containers: []models.DockerContainer{
						{
							ID:    "1",
							Name:  "svc",
							Image: "svc:latest",
							Ports: []models.DockerContainerPort{
								{PublicPort: 8080, PrivatePort: 80, Protocol: "tcp"},
								{PublicPort: 0, PrivatePort: 5432, Protocol: "tcp"},
								{PublicPort: 0, PrivatePort: 0, Protocol: "tcp"},
							},
						},
					},
				},
			},
		},
	}

	service := NewService(provider, nil, DefaultConfig())
	service.SetAIAnalyzer(analyzerFunc(func(ctx context.Context, prompt string) (string, error) {
		return `{"service_type":"postgres","service_name":"PostgreSQL","category":"database","cli_command":"","confidence":0.9,"reasoning":"known image"}`, nil
	}))

	apps := service.RunDiscovery(context.Background())
	if len(apps) != 1 {
		t.Fatalf("expected 1 discovered app, got %d", len(apps))
	}

	if len(apps[0].Ports) != 2 {
		t.Fatalf("expected 2 ports, got %d (%v)", len(apps[0].Ports), apps[0].Ports)
	}
	if apps[0].Ports[0] != 8080 || apps[0].Ports[1] != 5432 {
		t.Fatalf("unexpected ports: %v", apps[0].Ports)
	}
}
