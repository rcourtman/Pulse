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

type analyzerFunc func(ctx context.Context, prompt string) (string, error)

func (f analyzerFunc) AnalyzeForDiscovery(ctx context.Context, prompt string) (string, error) {
	return f(ctx, prompt)
}

type panickingStateProvider struct {
	callCount int32
}

func (p *panickingStateProvider) GetState() models.StateSnapshot {
	atomic.AddInt32(&p.callCount, 1)
	panic("infradiscovery test panic")
}

func (p *panickingStateProvider) Calls() int32 {
	return atomic.LoadInt32(&p.callCount)
}

func TestNewServiceZeroConfigUsesDefaults(t *testing.T) {
	service := NewService(&mockStateProvider{}, nil, Config{})
	if service.interval != 5*time.Minute {
		t.Fatalf("interval = %v, want %v", service.interval, 5*time.Minute)
	}
	if service.cacheExpiry != time.Hour {
		t.Fatalf("cacheExpiry = %v, want %v", service.cacheExpiry, time.Hour)
	}
}

func TestStartReturnsWhenAlreadyRunning(t *testing.T) {
	service := NewService(&mockStateProvider{}, nil, Config{
		Interval:    time.Hour,
		CacheExpiry: time.Hour,
	})
	service.SetAIAnalyzer(&mockAIAnalyzer{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service.Start(ctx)
	originalStopCh := service.stopCh

	service.Start(ctx)
	if service.stopCh != originalStopCh {
		t.Fatal("start should not replace stop channel when already running")
	}

	service.Stop()
}

func TestStartAndForceRefreshRecoverFromPanics(t *testing.T) {
	provider := &panickingStateProvider{}
	service := NewService(provider, nil, Config{
		Interval:    10 * time.Millisecond,
		CacheExpiry: time.Hour,
	})
	service.SetAIAnalyzer(&mockAIAnalyzer{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service.Start(ctx)
	waitFor(t, time.Second, func() bool {
		return provider.Calls() >= 2
	})

	service.ForceRefresh(context.Background())
	waitFor(t, time.Second, func() bool {
		return provider.Calls() >= 3
	})

	service.Stop()
}

func TestDiscoveryLoopExitsOnContextCancel(t *testing.T) {
	service := NewService(&mockStateProvider{}, nil, Config{
		Interval:    time.Hour,
		CacheExpiry: time.Hour,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		service.discoveryLoop(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("discoveryLoop did not stop after context cancellation")
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

func TestRunDiscoveryHandlesAnalysisFailures(t *testing.T) {
	provider := &mockStateProvider{
		state: models.StateSnapshot{
			DockerHosts: []models.DockerHost{
				{
					AgentID:  "agent-1",
					Hostname: "host1",
					Containers: []models.DockerContainer{
						{ID: "1", Name: "app", Image: "app:image"},
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
