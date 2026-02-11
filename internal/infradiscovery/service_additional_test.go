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

	analyzer := &blockingAIAnalyzer{
		response:  `{"service_type": "postgres", "service_name": "PostgreSQL", "category": "database", "cli_command": "", "confidence": 0.95, "reasoning": "PostgreSQL"}`,
		startedCh: make(chan struct{}, 1),
		releaseCh: make(chan struct{}),
	}

	service := NewService(provider, nil, DefaultConfig())
	service.SetAIAnalyzer(analyzer)

	firstDone := make(chan []DiscoveredApp, 1)
	go func() {
		firstDone <- service.RunDiscovery(context.Background())
	}()

	select {
	case <-analyzer.startedCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first discovery run to start")
	}

	skipped := service.RunDiscovery(context.Background())
	if skipped != nil {
		t.Fatalf("expected overlapping discovery run to be skipped with nil result, got %v", skipped)
	}
	if got := atomic.LoadInt32(&analyzer.callCount); got != 1 {
		t.Fatalf("expected analyzer to run once while overlap is skipped, got %d calls", got)
	}

	close(analyzer.releaseCh)

	select {
	case first := <-firstDone:
		if len(first) != 1 {
			t.Fatalf("expected first run to discover 1 app, got %d", len(first))
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first discovery run to finish")
	}
}

func TestBuildContainerInfoCopiesLabels(t *testing.T) {
	service := &Service{}
	labels := map[string]string{"app": "database"}
	container := models.DockerContainer{
		Name:   "db",
		Image:  "postgres:14",
		Labels: labels,
	}

	info := service.buildContainerInfo(container)

	labels["app"] = "changed"
	if info.Labels["app"] != "database" {
		t.Fatalf("expected copied labels to remain unchanged, got %q", info.Labels["app"])
	}

	info.Labels["role"] = "primary"
	if _, exists := labels["role"]; exists {
		t.Fatal("expected label map copy, but original map was modified")
	}
}
