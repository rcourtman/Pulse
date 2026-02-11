package infradiscovery

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type blockingAIAnalyzer struct {
	started  chan struct{}
	returned chan struct{}
}

func (a *blockingAIAnalyzer) AnalyzeForDiscovery(ctx context.Context, prompt string) (string, error) {
	select {
	case a.started <- struct{}{}:
	default:
	}

	<-ctx.Done()

	select {
	case a.returned <- struct{}{}:
	default:
	}

	return "", ctx.Err()
}

type countingAIAnalyzer struct {
	callCount int32
}

func (a *countingAIAnalyzer) AnalyzeForDiscovery(ctx context.Context, prompt string) (string, error) {
	atomic.AddInt32(&a.callCount, 1)
	return `{"service_type": "nginx", "service_name": "Nginx", "category": "web", "cli_command": "", "confidence": 0.9, "reasoning": "Web server"}`, nil
}

func (a *countingAIAnalyzer) Calls() int32 {
	return atomic.LoadInt32(&a.callCount)
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

func TestStopCancelsInFlightDiscovery(t *testing.T) {
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

	analyzer := &blockingAIAnalyzer{
		started:  make(chan struct{}, 1),
		returned: make(chan struct{}, 1),
	}

	service := NewService(provider, nil, Config{
		Interval:    time.Hour,
		CacheExpiry: time.Hour,
	})
	service.SetAIAnalyzer(analyzer)
	service.Start(context.Background())

	select {
	case <-analyzer.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for discovery analysis to start")
	}

	stopDone := make(chan struct{})
	go func() {
		service.Stop()
		close(stopDone)
	}()

	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("service.Stop did not complete after canceling in-flight discovery")
	}

	select {
	case <-analyzer.returned:
	case <-time.After(2 * time.Second):
		t.Fatal("analyzer did not return after stop cancellation")
	}

	status := service.GetStatus()
	if running, ok := status["running"].(bool); !ok || running {
		t.Fatalf("expected running status false after stop, got %v", status["running"])
	}
}

func TestForceRefreshSkippedAfterStop(t *testing.T) {
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
	analyzer := &countingAIAnalyzer{}

	service := NewService(provider, nil, DefaultConfig())
	service.SetAIAnalyzer(analyzer)
	service.Stop()
	service.ForceRefresh(context.Background())

	time.Sleep(100 * time.Millisecond)

	if calls := analyzer.Calls(); calls != 0 {
		t.Fatalf("expected force refresh to be skipped after stop, analyzer called %d times", calls)
	}
	if !service.GetLastRun().IsZero() {
		t.Fatalf("expected last run to remain zero after skipped force refresh, got %v", service.GetLastRun())
	}
}

func TestContextCancellationUpdatesRunningStatus(t *testing.T) {
	provider := &mockStateProvider{state: models.StateSnapshot{}}
	service := NewService(provider, nil, Config{
		Interval:    time.Hour,
		CacheExpiry: time.Hour,
	})
	service.SetAIAnalyzer(&mockAIAnalyzer{})

	ctx, cancel := context.WithCancel(context.Background())
	service.Start(ctx)
	cancel()

	waitFor(t, 2*time.Second, func() bool {
		status := service.GetStatus()
		running, _ := status["running"].(bool)
		return !running
	})
}
