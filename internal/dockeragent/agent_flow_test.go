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
	"github.com/rcourtman/pulse-go-rewrite/internal/buffer"
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
			PulseURL:    "https://pulse.example.com/",
			APIToken:    "token",
			LogLevel:    zerolog.InfoLevel,
			AgentType:   "unified",
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
}

func TestStopTimer(t *testing.T) {
	t.Run("timer not fired", func(t *testing.T) {
		timer := time.NewTimer(time.Hour)
		stopTimer(timer)
	})

	t.Run("timer fired and drained", func(t *testing.T) {
		timer := time.NewTimer(0)
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
			logger:       logger,
			targets:      []TargetConfig{{URL: "http://invalid", Token: "token"}},
			httpClients:  map[bool]*http.Client{false: {Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("send failed")
			})}},
			reportBuffer: buffer.New[agentsdocker.Report](10),
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
			reportBuffer: buffer.New[agentsdocker.Report](10),
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
			reportBuffer: buffer.New[agentsdocker.Report](10),
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
