package dockeragent

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/rs/zerolog"
)

func TestCollectContainer(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("success with running container", func(t *testing.T) {
		stats := containertypes.StatsResponse{
			Read: time.Now(),
			CPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 200000000},
				SystemUsage: 2000000000,
				OnlineCPUs:  2,
			},
			PreCPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 100000000},
				SystemUsage: 1000000000,
			},
			MemoryStats: containertypes.MemoryStats{
				Usage: 1000000,
				Limit: 4000000,
				Stats: map[string]uint64{"cache": 200000},
			},
			BlkioStats: containertypes.BlkioStats{
				IoServiceBytesRecursive: []containertypes.BlkioStatEntry{
					{Op: "Read", Value: 100},
					{Op: "Write", Value: 200},
				},
			},
		}

		sizeRw := int64(1234)
		sizeRoot := int64(5678)
		inspect := baseInspect()
		inspect.ContainerJSONBase.SizeRw = &sizeRw
		inspect.ContainerJSONBase.SizeRootFs = &sizeRoot
		inspect.ContainerJSONBase.State = &containertypes.State{
			Running:   true,
			StartedAt: time.Now().Add(-time.Minute).Format(time.RFC3339Nano),
			Health:    &containertypes.Health{Status: "healthy"},
		}
		inspect.Config.Env = []string{"PASSWORD=secret", "PATH=/bin"}
		inspect.NetworkSettings.Networks["net1"].IPAddress = "10.0.0.2"
		inspect.Mounts = []containertypes.MountPoint{
			{Type: "bind", Source: "/data", Destination: "/data", RW: true},
		}

		agent := &Agent{
			cfg: Config{
				CollectDiskMetrics: true,
			},
			runtime:          RuntimePodman,
			logger:           logger,
			prevContainerCPU: make(map[string]cpuSample),
			registryChecker:  NewRegistryChecker(logger),
			docker: &fakeDockerClient{
				containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
					return inspect, nil, nil
				},
				containerStatsOneShotFn: func(context.Context, string) (containertypes.StatsResponseReader, error) {
					return statsReader(t, stats), nil
				},
			},
		}

		summary := containertypes.Summary{
			ID:      "container-123456",
			Names:   []string{"/app"},
			Image:   "nginx@sha256:abc123",
			ImageID: "sha256:local",
			Created: time.Now().Add(-time.Hour).Unix(),
			State:   "running",
			Status:  "Up",
			Ports: []containertypes.Port{
				{PrivatePort: 80, PublicPort: 8080, Type: "tcp", IP: "0.0.0.0"},
			},
			Labels: map[string]string{
				"io.podman.annotations.pod.name": "mypod",
			},
		}

		container, err := agent.collectContainer(context.Background(), summary)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if container.Name != "app" {
			t.Fatalf("expected name app, got %q", container.Name)
		}
		if container.Health != "healthy" {
			t.Fatalf("expected health status, got %q", container.Health)
		}
		if container.BlockIO == nil {
			t.Fatalf("expected block IO to be populated")
		}
		if container.Podman == nil || container.Podman.PodName != "mypod" {
			t.Fatalf("expected podman metadata")
		}
		if container.UpdateStatus == nil || container.UpdateStatus.Error == "" {
			t.Fatalf("expected update status for digest-pinned image")
		}
		if len(container.Networks) == 0 {
			t.Fatalf("expected networks to be populated")
		}
	})

	t.Run("stopped container clears sample", func(t *testing.T) {
		agent := &Agent{
			logger: logger,
			prevContainerCPU: map[string]cpuSample{
				"container-123456": {totalUsage: 1},
			},
			docker: &fakeDockerClient{
				containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
					inspect := baseInspect()
					inspect.ContainerJSONBase.State = &containertypes.State{Running: false}
					return inspect, nil, nil
				},
			},
		}

		summary := containertypes.Summary{ID: "container-123456", Names: []string{"/app"}, State: "exited"}
		if _, err := agent.collectContainer(context.Background(), summary); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := agent.prevContainerCPU["container1"]; ok {
			t.Fatalf("expected sample to be removed")
		}
	})

	t.Run("inspect error", func(t *testing.T) {
		agent := &Agent{
			docker: &fakeDockerClient{
				containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
					return containertypes.InspectResponse{}, nil, errors.New("inspect failed")
				},
			},
			logger: logger,
		}

		if _, err := agent.collectContainer(context.Background(), containertypes.Summary{ID: "container-123456"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("stats error", func(t *testing.T) {
		agent := &Agent{
			docker: &fakeDockerClient{
				containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
					inspect := baseInspect()
					inspect.ContainerJSONBase.State = &containertypes.State{Running: true}
					return inspect, nil, nil
				},
				containerStatsOneShotFn: func(context.Context, string) (containertypes.StatsResponseReader, error) {
					return containertypes.StatsResponseReader{}, errors.New("stats failed")
				},
			},
			logger: logger,
		}

		if _, err := agent.collectContainer(context.Background(), containertypes.Summary{ID: "container-123456"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("stats decode error", func(t *testing.T) {
		agent := &Agent{
			docker: &fakeDockerClient{
				containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
					inspect := baseInspect()
					inspect.ContainerJSONBase.State = &containertypes.State{Running: true}
					return inspect, nil, nil
				},
				containerStatsOneShotFn: func(context.Context, string) (containertypes.StatsResponseReader, error) {
					return containertypes.StatsResponseReader{Body: io.NopCloser(strings.NewReader("{"))}, nil
				},
			},
			logger: logger,
		}

		if _, err := agent.collectContainer(context.Background(), containertypes.Summary{ID: "container-123456"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("uptime negative clamped", func(t *testing.T) {
		future := time.Now().Add(5 * time.Minute).Format(time.RFC3339Nano)
		inspect := baseInspect()
		inspect.ContainerJSONBase.State = &containertypes.State{
			Running:   true,
			StartedAt: future,
		}

		stats := containertypes.StatsResponse{
			Read: time.Now(),
			CPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 100},
				SystemUsage: 1000,
				OnlineCPUs:  1,
			},
			PreCPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 100},
				SystemUsage: 1000,
			},
		}

		agent := &Agent{
			logger:           logger,
			prevContainerCPU: make(map[string]cpuSample),
			docker: &fakeDockerClient{
				containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
					return inspect, nil, nil
				},
				containerStatsOneShotFn: func(context.Context, string) (containertypes.StatsResponseReader, error) {
					return statsReader(t, stats), nil
				},
			},
		}

		container, err := agent.collectContainer(context.Background(), containertypes.Summary{ID: "container-123456", Names: []string{"/app"}, State: "running"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if container.UptimeSeconds != 0 {
			t.Fatalf("expected uptime to be clamped to 0, got %d", container.UptimeSeconds)
		}
	})
}

func TestCollectContainers(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("list error", func(t *testing.T) {
		agent := &Agent{
			docker: &fakeDockerClient{
				containerListFunc: func(context.Context, containertypes.ListOptions) ([]containertypes.Summary, error) {
					return nil, errors.New("list failed")
				},
			},
			logger: logger,
		}

		if _, err := agent.collectContainers(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("filters and prune", func(t *testing.T) {
		agent := &Agent{
			logger:       logger,
			stateFilters: []string{"running"},
			allowedStates: map[string]struct{}{
				"running": {},
			},
			prevContainerCPU: map[string]cpuSample{
				"stale": {totalUsage: 1},
			},
			docker: &fakeDockerClient{
				containerListFunc: func(_ context.Context, opts containertypes.ListOptions) ([]containertypes.Summary, error) {
					if opts.Filters.Len() == 0 {
						t.Fatal("expected filters to be set")
					}
					return []containertypes.Summary{
						{ID: "running1", Names: []string{"/run"}, State: "running"},
						{ID: "exited1", Names: []string{"/exit"}, State: "exited"},
					}, nil
				},
				containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
					return containertypes.InspectResponse{}, nil, errors.New("inspect failed")
				},
			},
		}

		containers, err := agent.collectContainers(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(containers) != 0 {
			t.Fatalf("expected no containers, got %d", len(containers))
		}
		if _, ok := agent.prevContainerCPU["stale"]; ok {
			t.Fatalf("expected stale sample to be pruned")
		}
	})
}

func TestPrimaryTargetAndHTTPClient(t *testing.T) {
	t.Run("primary target empty", func(t *testing.T) {
		agent := &Agent{}
		if target := agent.primaryTarget(); target.URL != "" {
			t.Fatal("expected empty target")
		}
	})

	t.Run("http client selection", func(t *testing.T) {
		secure := &http.Client{}
		insecure := &http.Client{}
		agent := &Agent{
			httpClients: map[bool]*http.Client{
				false: secure,
				true:  insecure,
			},
		}
		if got := agent.httpClientFor(TargetConfig{InsecureSkipVerify: true}); got != insecure {
			t.Fatal("expected insecure client")
		}
		if got := agent.httpClientFor(TargetConfig{InsecureSkipVerify: false}); got != secure {
			t.Fatal("expected secure client")
		}
	})

	t.Run("http client fallback", func(t *testing.T) {
		agent := &Agent{
			httpClients: map[bool]*http.Client{},
		}
		got := agent.httpClientFor(TargetConfig{InsecureSkipVerify: true})
		if got == nil {
			t.Fatal("expected fallback client")
		}
	})

	t.Run("http client fallback prefers secure", func(t *testing.T) {
		secure := &http.Client{}
		agent := &Agent{
			httpClients: map[bool]*http.Client{
				false: secure,
			},
		}
		got := agent.httpClientFor(TargetConfig{InsecureSkipVerify: true})
		if got != secure {
			t.Fatal("expected secure fallback client")
		}
	})

	t.Run("http client fallback prefers insecure", func(t *testing.T) {
		insecure := &http.Client{}
		agent := &Agent{
			httpClients: map[bool]*http.Client{
				true: insecure,
			},
		}
		got := agent.httpClientFor(TargetConfig{InsecureSkipVerify: false})
		if got != insecure {
			t.Fatal("expected insecure fallback client")
		}
	})
}

func TestNewHTTPClient(t *testing.T) {
	client := newHTTPClient(true)
	if client.Transport == nil {
		t.Fatal("expected transport")
	}
	transport := client.Transport.(*http.Transport)
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected insecure skip verify true")
	}
}

func TestAgentClose(t *testing.T) {
	closed := false
	agent := &Agent{
		docker: &fakeDockerClient{
			closeFn: func() error {
				closed = true
				return nil
			},
		},
	}

	if err := agent.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !closed {
		t.Fatal("expected docker Close to be called")
	}
}
