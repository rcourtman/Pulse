package dockeragent

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	systemtypes "github.com/docker/docker/api/types/system"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

func TestNewAgent(t *testing.T) {
	t.Run("missing targets", func(t *testing.T) {
		if _, err := New(Config{}); err == nil {
			t.Fatal("expected error for missing target")
		}
	})

	t.Run("creates agent with defaults", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "machine-id")
		if err := os.WriteFile(path, []byte("machine-1"), 0600); err != nil {
			t.Fatalf("write machine-id: %v", err)
		}
		swap(t, &machineIDPaths, []string{path})

		fake := &fakeDockerClient{daemonHost: "unix:///var/run/docker.sock"}
		swap(t, &connectRuntimeFn, func(_ RuntimeKind, _ *zerolog.Logger) (dockerClient, systemtypes.Info, RuntimeKind, error) {
			return fake, systemtypes.Info{ID: "daemon1", ServerVersion: "24.0.0"}, RuntimeDocker, nil
		})

		cfg := Config{
			PulseURL:     "https://pulse.example.com/",
			APIToken:     "token",
			LogLevel:     zerolog.InfoLevel,
			AgentType:    "unified",
			AgentVersion: "",
		}

		agent, err := New(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if agent.cfg.PulseURL != "https://pulse.example.com" {
			t.Fatalf("expected trimmed URL, got %q", agent.cfg.PulseURL)
		}
		if agent.cfg.IncludeContainers && agent.cfg.IncludeServices && agent.cfg.IncludeTasks {
			// ok
		} else {
			t.Fatal("expected include flags to default to true")
		}
		if agent.machineID != "machine-1" {
			t.Fatalf("expected machine-id to be loaded, got %q", agent.machineID)
		}
		if agent.agentVersion != Version {
			t.Fatalf("expected agent version to default, got %q", agent.agentVersion)
		}
	})

	t.Run("podman disables swarm collections", func(t *testing.T) {
		fake := &fakeDockerClient{daemonHost: "unix:///run/podman/podman.sock"}
		swap(t, &connectRuntimeFn, func(_ RuntimeKind, _ *zerolog.Logger) (dockerClient, systemtypes.Info, RuntimeKind, error) {
			return fake, systemtypes.Info{ID: "podman", ServerVersion: "4.6.0"}, RuntimePodman, nil
		})

		cfg := Config{
			PulseURL:        "https://pulse.example.com",
			APIToken:        "token",
			IncludeServices: true,
			IncludeTasks:    true,
		}

		agent, err := New(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if agent.cfg.IncludeServices || agent.cfg.IncludeTasks {
			t.Fatal("expected swarm collection to be disabled for podman")
		}
	})

	t.Run("targets provided and logger override", func(t *testing.T) {
		fake := &fakeDockerClient{daemonHost: "unix:///var/run/docker.sock"}
		swap(t, &connectRuntimeFn, func(_ RuntimeKind, _ *zerolog.Logger) (dockerClient, systemtypes.Info, RuntimeKind, error) {
			return fake, systemtypes.Info{ID: "daemon1", ServerVersion: "24.0.0"}, RuntimeDocker, nil
		})

		logger := zerolog.Nop()
		cfg := Config{
			Targets: []TargetConfig{
				{URL: "https://pulse.example.com/", Token: "token", InsecureSkipVerify: false},
				{URL: "https://pulse-dr.example.com", Token: "token2", InsecureSkipVerify: true},
			},
			HostnameOverride:  "override",
			AgentVersion:      "1.2.3",
			IncludeContainers: true,
			ContainerStates:   []string{"running"},
			LogLevel:          zerolog.InfoLevel,
			Logger:            &logger,
		}

		prev := zerolog.GlobalLevel()
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		t.Cleanup(func() {
			zerolog.SetGlobalLevel(prev)
		})

		agent, err := New(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if agent.hostName != "override" {
			t.Fatalf("expected hostname override, got %q", agent.hostName)
		}
		if agent.agentVersion != "1.2.3" {
			t.Fatalf("expected agent version override, got %q", agent.agentVersion)
		}
		if len(agent.httpClients) != 2 {
			t.Fatalf("expected both secure and insecure clients")
		}
		if _, ok := agent.allowedStates["running"]; !ok {
			t.Fatalf("expected allowed state to be set")
		}
	})

	t.Run("invalid targets error", func(t *testing.T) {
		if _, err := New(Config{
			Targets: []TargetConfig{{URL: "", Token: "token"}},
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid container states error", func(t *testing.T) {
		if _, err := New(Config{
			PulseURL:        "https://pulse.example.com",
			APIToken:        "token",
			ContainerStates: []string{"bad"},
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid swarm scope error", func(t *testing.T) {
		if _, err := New(Config{
			PulseURL:   "https://pulse.example.com",
			APIToken:   "token",
			SwarmScope: "bad",
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid runtime error", func(t *testing.T) {
		if _, err := New(Config{
			PulseURL: "https://pulse.example.com",
			APIToken: "token",
			Runtime:  "bad",
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("connect runtime error", func(t *testing.T) {
		swap(t, &connectRuntimeFn, func(_ RuntimeKind, _ *zerolog.Logger) (dockerClient, systemtypes.Info, RuntimeKind, error) {
			return nil, systemtypes.Info{}, RuntimeDocker, errors.New("connect failed")
		})

		if _, err := New(Config{
			PulseURL: "https://pulse.example.com",
			APIToken: "token",
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("fallback normalize targets error", func(t *testing.T) {
		swap(t, &normalizeTargetsFn, func(targets []TargetConfig) ([]TargetConfig, error) {
			if len(targets) == 0 {
				return nil, nil
			}
			return nil, errors.New("normalize failed")
		})

		if _, err := New(Config{
			PulseURL: "https://pulse.example.com",
			APIToken: "token",
		}); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestStopTimer(t *testing.T) {
	t.Run("timer not fired", func(t *testing.T) {
		timer := time.NewTimer(time.Hour)
		stopTimer(timer)
	})

	t.Run("timer fired and drained", func(t *testing.T) {
		timer := time.NewTimer(1 * time.Millisecond)
		time.Sleep(10 * time.Millisecond)
		stopTimer(timer)
		select {
		case <-timer.C:
			t.Fatal("expected timer channel to be drained")
		default:
		}
	})
}

func TestCollectOnce(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("build report error", func(t *testing.T) {
		agent := &Agent{
			docker: &fakeDockerClient{
				infoFunc: func(context.Context) (systemtypes.Info, error) {
					return systemtypes.Info{}, errors.New("info failed")
				},
			},
			logger: logger,
		}

		if err := agent.collectOnce(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("send report error buffers", func(t *testing.T) {
		swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
			return hostmetrics.Snapshot{}, nil
		})

		agent := &Agent{
			cfg: Config{Interval: 30 * time.Second},
			docker: &fakeDockerClient{
				infoFunc: func(context.Context) (systemtypes.Info, error) {
					return systemtypes.Info{ID: "daemon", ServerVersion: "24.0.0"}, nil
				},
			},
			logger:  logger,
			targets: []TargetConfig{{URL: "http://invalid", Token: "token"}},
			httpClients: map[bool]*http.Client{false: {Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("send failed")
			})}},
			reportBuffer: utils.NewQueue[agentsdocker.Report](10),
		}

		if err := agent.collectOnce(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if agent.reportBuffer.Len() != 1 {
			t.Fatalf("expected report to be buffered")
		}
	})

	t.Run("send report stop requested", func(t *testing.T) {
		swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
			return hostmetrics.Snapshot{}, nil
		})

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"host was removed","code":"invalid_report"}`))
		}))
		defer server.Close()

		agent := &Agent{
			cfg: Config{Interval: 30 * time.Second},
			docker: &fakeDockerClient{
				infoFunc: func(context.Context) (systemtypes.Info, error) {
					return systemtypes.Info{ID: "daemon", ServerVersion: "24.0.0"}, nil
				},
			},
			logger:       logger,
			targets:      []TargetConfig{{URL: server.URL, Token: "token"}},
			httpClients:  map[bool]*http.Client{false: server.Client()},
			reportBuffer: utils.NewQueue[agentsdocker.Report](10),
		}

		if err := agent.collectOnce(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if agent.reportBuffer.Len() != 0 {
			t.Fatalf("expected no buffering on stop request")
		}
	})

	t.Run("flush buffer after success", func(t *testing.T) {
		swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
			return hostmetrics.Snapshot{}, nil
		})

		var buf bytes.Buffer
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}))
		defer server.Close()

		agent := &Agent{
			cfg: Config{Interval: 30 * time.Second},
			docker: &fakeDockerClient{
				infoFunc: func(context.Context) (systemtypes.Info, error) {
					return systemtypes.Info{ID: "daemon", ServerVersion: "24.0.0"}, nil
				},
			},
			logger:       zerolog.New(&buf),
			targets:      []TargetConfig{{URL: server.URL, Token: "token"}},
			httpClients:  map[bool]*http.Client{false: server.Client()},
			reportBuffer: utils.NewQueue[agentsdocker.Report](10),
		}

		agent.reportBuffer.Push(agentsdocker.Report{Agent: agentsdocker.AgentInfo{ID: "queued"}})
		agent.reportBuffer.Push(agentsdocker.Report{Agent: agentsdocker.AgentInfo{ID: "queued2"}})

		if err := agent.collectOnce(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if agent.reportBuffer.Len() != 0 {
			t.Fatalf("expected buffer to be flushed")
		}
	})
}

func TestRun(t *testing.T) {
	t.Run("stop requested on startup", func(t *testing.T) {
		swap(t, &connectRuntimeFn, func(_ RuntimeKind, _ *zerolog.Logger) (dockerClient, systemtypes.Info, RuntimeKind, error) {
			return &fakeDockerClient{
				infoFunc: func(context.Context) (systemtypes.Info, error) {
					return systemtypes.Info{}, ErrStopRequested
				},
			}, systemtypes.Info{}, RuntimeDocker, nil
		})

		agent := &Agent{
			cfg: Config{
				Interval: 10 * time.Millisecond,
			},
			docker: &fakeDockerClient{
				infoFunc: func(context.Context) (systemtypes.Info, error) {
					return systemtypes.Info{}, ErrStopRequested
				},
			},
			logger: zerolog.Nop(),
		}

		if err := agent.Run(context.Background()); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("interval default and initial error", func(t *testing.T) {
		agent := &Agent{
			cfg: Config{
				Interval: 0,
			},
			docker: &fakeDockerClient{
				infoFunc: func(context.Context) (systemtypes.Info, error) {
					return systemtypes.Info{}, errors.New("info failed")
				},
			},
			logger: zerolog.Nop(),
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			done <- agent.Run(ctx)
		}()

		time.Sleep(10 * time.Millisecond)
		cancel()

		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled, got %v", err)
		}
		if agent.cfg.Interval <= 0 {
			t.Fatalf("expected interval to be defaulted")
		}
	})

	t.Run("ticker and update timer", func(t *testing.T) {
		swap(t, &randomDurationFn, func(time.Duration) time.Duration {
			return -5 * time.Second
		})
		swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
			return hostmetrics.Snapshot{}, nil
		})

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		agent := &Agent{
			cfg: Config{
				Interval:          5 * time.Millisecond,
				DisableAutoUpdate: true,
			},
			docker: &fakeDockerClient{
				infoFunc: func(context.Context) (systemtypes.Info, error) {
					return systemtypes.Info{ID: "daemon", ServerVersion: "24.0.0"}, nil
				},
			},
			logger:       zerolog.Nop(),
			targets:      []TargetConfig{{URL: server.URL, Token: "token"}},
			httpClients:  map[bool]*http.Client{false: server.Client()},
			reportBuffer: utils.NewQueue[agentsdocker.Report](10),
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			done <- agent.Run(ctx)
		}()

		time.Sleep(20 * time.Millisecond)
		cancel()

		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled, got %v", err)
		}
	})

	t.Run("update timer reset when delay <= 0", func(t *testing.T) {
		swap(t, &randomDurationFn, func(time.Duration) time.Duration {
			return -25 * time.Hour
		})
		swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
			return hostmetrics.Snapshot{}, nil
		})

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		agent := &Agent{
			cfg: Config{
				Interval:          time.Hour,
				DisableAutoUpdate: true,
			},
			docker: &fakeDockerClient{
				infoFunc: func(context.Context) (systemtypes.Info, error) {
					return systemtypes.Info{ID: "daemon", ServerVersion: "24.0.0"}, nil
				},
			},
			logger:       zerolog.Nop(),
			targets:      []TargetConfig{{URL: server.URL, Token: "token"}},
			httpClients:  map[bool]*http.Client{false: server.Client()},
			reportBuffer: utils.NewQueue[agentsdocker.Report](10),
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			done <- agent.Run(ctx)
		}()

		time.Sleep(20 * time.Millisecond)
		cancel()

		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled, got %v", err)
		}
	})

	t.Run("stop requested on ticker", func(t *testing.T) {
		swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
			return hostmetrics.Snapshot{}, nil
		})
		swap(t, &randomDurationFn, func(time.Duration) time.Duration {
			return 0
		})

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		calls := 0
		agent := &Agent{
			cfg: Config{
				Interval:          5 * time.Millisecond,
				DisableAutoUpdate: true,
			},
			docker: &fakeDockerClient{
				infoFunc: func(context.Context) (systemtypes.Info, error) {
					calls++
					if calls > 1 {
						return systemtypes.Info{}, ErrStopRequested
					}
					return systemtypes.Info{ID: "daemon", ServerVersion: "24.0.0"}, nil
				},
			},
			logger:       zerolog.Nop(),
			targets:      []TargetConfig{{URL: server.URL, Token: "token"}},
			httpClients:  map[bool]*http.Client{false: server.Client()},
			reportBuffer: utils.NewQueue[agentsdocker.Report](10),
		}

		done := make(chan error, 1)
		go func() {
			done <- agent.Run(context.Background())
		}()

		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("expected nil, got %v", err)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatal("expected Run to stop")
		}
	})

	t.Run("ticker error logs", func(t *testing.T) {
		swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
			return hostmetrics.Snapshot{}, nil
		})
		swap(t, &randomDurationFn, func(time.Duration) time.Duration {
			return 0
		})

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		calls := 0
		agent := &Agent{
			cfg: Config{
				Interval:          5 * time.Millisecond,
				DisableAutoUpdate: true,
			},
			docker: &fakeDockerClient{
				infoFunc: func(context.Context) (systemtypes.Info, error) {
					calls++
					if calls > 1 {
						return systemtypes.Info{}, errors.New("info failed")
					}
					return systemtypes.Info{ID: "daemon", ServerVersion: "24.0.0"}, nil
				},
			},
			logger:       zerolog.Nop(),
			targets:      []TargetConfig{{URL: server.URL, Token: "token"}},
			httpClients:  map[bool]*http.Client{false: server.Client()},
			reportBuffer: utils.NewQueue[agentsdocker.Report](10),
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			done <- agent.Run(ctx)
		}()

		time.Sleep(20 * time.Millisecond)
		cancel()

		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled, got %v", err)
		}
	})
}
