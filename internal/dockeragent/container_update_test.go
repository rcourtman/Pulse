package dockeragent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/opencontainers/image-spec/specs-go/v1"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

func baseInspect() containertypes.InspectResponse {
	state := &containertypes.State{Running: true}
	hostConfig := &containertypes.HostConfig{}

	return containertypes.InspectResponse{
		ContainerJSONBase: &containertypes.ContainerJSONBase{
			Name:         "/app",
			Image:        "sha256:old0000000000",
			State:        state,
			RestartCount: 1,
			HostConfig:   hostConfig,
		},
		Config: &containertypes.Config{
			Image: "nginx:latest",
		},
		NetworkSettings: &containertypes.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{
				"net1": {Aliases: []string{"app"}},
				"net2": {Aliases: []string{"app2"}},
			},
		},
	}
}

func TestUpdateContainer_Errors(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("inspect error", func(t *testing.T) {
		agent := &Agent{
			docker: &fakeDockerClient{
				containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
					return containertypes.InspectResponse{}, errors.New("inspect failed")
				},
			},
			logger: logger,
		}

		result := agent.updateContainerWithProgress(context.Background(), "container1", nil)
		if result.Error == "" {
			t.Fatal("expected error for inspect failure")
		}
	})

	t.Run("pull error", func(t *testing.T) {
		agent := &Agent{
			docker: &fakeDockerClient{
				containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
					return baseInspect(), nil
				},
				imagePullFn: func(context.Context, string, image.PullOptions) (io.ReadCloser, error) {
					return nil, errors.New("pull failed")
				},
			},
			logger: logger,
		}

		result := agent.updateContainerWithProgress(context.Background(), "container1", nil)
		if result.Error == "" {
			t.Fatal("expected error for pull failure")
		}
	})

	t.Run("stop error", func(t *testing.T) {
		agent := &Agent{
			docker: &fakeDockerClient{
				containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
					return baseInspect(), nil
				},
				imagePullFn: func(context.Context, string, image.PullOptions) (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader("{}")), nil
				},
				containerStopFn: func(context.Context, string, containertypes.StopOptions) error {
					return errors.New("stop failed")
				},
			},
			logger: logger,
		}

		result := agent.updateContainerWithProgress(context.Background(), "container1", nil)
		if result.Error == "" {
			t.Fatal("expected error for stop failure")
		}
	})

	t.Run("rename error", func(t *testing.T) {
		startCalled := false
		agent := &Agent{
			docker: &fakeDockerClient{
				containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
					return baseInspect(), nil
				},
				imagePullFn: func(context.Context, string, image.PullOptions) (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader("{}")), nil
				},
				containerStopFn: func(context.Context, string, containertypes.StopOptions) error {
					return nil
				},
				containerRenameFn: func(context.Context, string, string) error {
					return errors.New("rename failed")
				},
				containerStartFn: func(context.Context, string, containertypes.StartOptions) error {
					startCalled = true
					return nil
				},
			},
			logger: logger,
		}

		result := agent.updateContainerWithProgress(context.Background(), "container1", nil)
		if result.Error == "" {
			t.Fatal("expected error for rename failure")
		}
		if !startCalled {
			t.Fatal("expected original container to be restarted")
		}
	})

	t.Run("create error", func(t *testing.T) {
		renameCalled := false
		startCalled := false
		agent := &Agent{
			docker: &fakeDockerClient{
				containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
					return baseInspect(), nil
				},
				imagePullFn: func(context.Context, string, image.PullOptions) (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader("{}")), nil
				},
				containerStopFn: func(context.Context, string, containertypes.StopOptions) error {
					return nil
				},
				containerRenameFn: func(context.Context, string, string) error {
					renameCalled = true
					return nil
				},
				containerCreateFn: func(context.Context, *containertypes.Config, *containertypes.HostConfig, *network.NetworkingConfig, *v1.Platform, string) (containertypes.CreateResponse, error) {
					return containertypes.CreateResponse{}, errors.New("create failed")
				},
				containerStartFn: func(context.Context, string, containertypes.StartOptions) error {
					startCalled = true
					return nil
				},
			},
			logger: logger,
		}

		result := agent.updateContainerWithProgress(context.Background(), "container1", nil)
		if result.Error == "" {
			t.Fatal("expected error for create failure")
		}
		if !renameCalled || !startCalled {
			t.Fatal("expected rollback to rename and restart")
		}
	})

	t.Run("start error", func(t *testing.T) {
		removed := false
		renamed := false
		restarted := false
		agent := &Agent{
			docker: &fakeDockerClient{
				containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
					return baseInspect(), nil
				},
				imagePullFn: func(context.Context, string, image.PullOptions) (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader("{}")), nil
				},
				containerStopFn: func(context.Context, string, containertypes.StopOptions) error {
					return nil
				},
				containerCreateFn: func(context.Context, *containertypes.Config, *containertypes.HostConfig, *network.NetworkingConfig, *v1.Platform, string) (containertypes.CreateResponse, error) {
					return containertypes.CreateResponse{ID: "new123"}, nil
				},
				containerStartFn: func(_ context.Context, id string, _ containertypes.StartOptions) error {
					if id == "new123" {
						return errors.New("start failed")
					}
					restarted = true
					return nil
				},
				containerRemoveFn: func(context.Context, string, containertypes.RemoveOptions) error {
					removed = true
					return nil
				},
				containerRenameFn: func(context.Context, string, string) error {
					renamed = true
					return nil
				},
			},
			logger: logger,
		}

		result := agent.updateContainerWithProgress(context.Background(), "container1", nil)
		if result.Error == "" {
			t.Fatal("expected error for start failure")
		}
		if !removed || !renamed || !restarted {
			t.Fatal("expected rollback cleanup")
		}
	})
}

func TestUpdateContainer_Success(t *testing.T) {
	logger := zerolog.Nop()
	swap(t, &sleepFn, func(time.Duration) {})
	swap(t, &nowFn, func() time.Time {
		return time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)
	})

	var (
		mu           sync.Mutex
		cleanupCalls int
		cleanupErr   error
		cleanupCh    = make(chan struct{})
	)

	agent := &Agent{
		docker: &fakeDockerClient{
			containerInspectFn: func(_ context.Context, id string) (containertypes.InspectResponse, error) {
				if id == "new123" {
					inspect := baseInspect()
					inspect.ContainerJSONBase.Image = "sha256:new0000000000"
					return inspect, nil
				}
				return baseInspect(), nil
			},
			imagePullFn: func(context.Context, string, image.PullOptions) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("{}")), nil
			},
			containerStopFn: func(context.Context, string, containertypes.StopOptions) error {
				return nil
			},
			containerRenameFn: func(context.Context, string, string) error {
				return nil
			},
			containerCreateFn: func(context.Context, *containertypes.Config, *containertypes.HostConfig, *network.NetworkingConfig, *v1.Platform, string) (containertypes.CreateResponse, error) {
				return containertypes.CreateResponse{ID: "new123"}, nil
			},
			networkConnectFn: func(context.Context, string, string, *network.EndpointSettings) error {
				return errors.New("network connect failed")
			},
			containerStartFn: func(context.Context, string, containertypes.StartOptions) error {
				return nil
			},
			containerRemoveFn: func(context.Context, string, containertypes.RemoveOptions) error {
				mu.Lock()
				cleanupCalls++
				err := cleanupErr
				mu.Unlock()
				close(cleanupCh)
				return err
			},
		},
		logger: logger,
	}

	result := agent.updateContainerWithProgress(context.Background(), "container1", nil)
	if !result.Success {
		t.Fatalf("expected success, got error %q", result.Error)
	}
	if !result.BackupCreated || result.BackupContainer == "" {
		t.Fatalf("expected backup to be created")
	}
	if result.NewImageDigest == "" {
		t.Fatalf("expected new image digest")
	}

	<-cleanupCh

	mu.Lock()
	if cleanupCalls != 1 {
		t.Fatalf("expected cleanup to be called once, got %d", cleanupCalls)
	}
	mu.Unlock()
}

func TestUpdateContainer_CleanupError(t *testing.T) {
	logger := zerolog.Nop()
	swap(t, &sleepFn, func(time.Duration) {})
	swap(t, &nowFn, func() time.Time {
		return time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)
	})

	cleanupErr := errors.New("cleanup failed")
	done := make(chan struct{})

	agent := &Agent{
		docker: &fakeDockerClient{
			containerInspectFn: func(_ context.Context, id string) (containertypes.InspectResponse, error) {
				if id == "new123" {
					inspect := baseInspect()
					inspect.ContainerJSONBase.Image = "sha256:new0000000000"
					return inspect, nil
				}
				return baseInspect(), nil
			},
			imagePullFn: func(context.Context, string, image.PullOptions) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("{}")), nil
			},
			containerStopFn: func(context.Context, string, containertypes.StopOptions) error {
				return nil
			},
			containerRenameFn: func(context.Context, string, string) error {
				return nil
			},
			containerCreateFn: func(context.Context, *containertypes.Config, *containertypes.HostConfig, *network.NetworkingConfig, *v1.Platform, string) (containertypes.CreateResponse, error) {
				return containertypes.CreateResponse{ID: "new123"}, nil
			},
			containerStartFn: func(context.Context, string, containertypes.StartOptions) error {
				return nil
			},
			containerRemoveFn: func(context.Context, string, containertypes.RemoveOptions) error {
				close(done)
				return cleanupErr
			},
		},
		logger: logger,
	}

	result := agent.updateContainerWithProgress(context.Background(), "container1", nil)
	if !result.Success {
		t.Fatalf("expected success, got error %q", result.Error)
	}

	<-done
}

func TestHandleUpdateContainerCommand(t *testing.T) {
	logger := zerolog.Nop()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	agent := &Agent{
		logger: logger,
		hostID: "host1",
		httpClients: map[bool]*http.Client{
			false: server.Client(),
		},
		docker: &fakeDockerClient{
			containerInspectFn: func(_ context.Context, id string) (containertypes.InspectResponse, error) {
				inspect := baseInspect()
				if id == "new123" {
					inspect.ContainerJSONBase.Image = "sha256:new0000000000"
				}
				return inspect, nil
			},
			imagePullFn: func(context.Context, string, image.PullOptions) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("{}")), nil
			},
			containerStopFn: func(context.Context, string, containertypes.StopOptions) error {
				return nil
			},
			containerRenameFn: func(context.Context, string, string) error {
				return nil
			},
			containerCreateFn: func(context.Context, *containertypes.Config, *containertypes.HostConfig, *network.NetworkingConfig, *v1.Platform, string) (containertypes.CreateResponse, error) {
				return containertypes.CreateResponse{ID: "new123"}, nil
			},
			containerStartFn: func(context.Context, string, containertypes.StartOptions) error {
				return nil
			},
		},
	}

	command := agentsdocker.Command{
		ID:   "cmd1",
		Type: agentsdocker.CommandTypeUpdateContainer,
		Payload: map[string]any{
			"containerId": "container1",
		},
	}

	if err := agent.handleUpdateContainerCommand(context.Background(), TargetConfig{URL: server.URL}, command); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("missing container id", func(t *testing.T) {
		if err := agent.handleUpdateContainerCommand(context.Background(), TargetConfig{URL: server.URL}, agentsdocker.Command{ID: "cmd2"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing container id ack failure", func(t *testing.T) {
		if err := agent.handleUpdateContainerCommand(context.Background(), TargetConfig{URL: "http://example.com/\x7f"}, agentsdocker.Command{ID: "cmd2c"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("container id wrong type", func(t *testing.T) {
		cmd := agentsdocker.Command{
			ID:   "cmd2b",
			Type: agentsdocker.CommandTypeUpdateContainer,
			Payload: map[string]any{
				"containerId": 123,
			},
		}
		if err := agent.handleUpdateContainerCommand(context.Background(), TargetConfig{URL: server.URL}, cmd); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("container id unmarshalable payload", func(t *testing.T) {
		cmd := agentsdocker.Command{
			ID:   "cmd2d",
			Type: agentsdocker.CommandTypeUpdateContainer,
			Payload: map[string]any{
				"containerId": make(chan int),
			},
		}
		if err := agent.handleUpdateContainerCommand(context.Background(), TargetConfig{URL: server.URL}, cmd); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("ack error stops early", func(t *testing.T) {
		badTarget := TargetConfig{URL: "http://example.com/\x7f"}
		agent.hostID = "host1"

		cmd := agentsdocker.Command{
			ID:   "cmd3",
			Type: agentsdocker.CommandTypeUpdateContainer,
			Payload: map[string]any{
				"containerId": "container1",
			},
		}

		if err := agent.handleUpdateContainerCommand(context.Background(), badTarget, cmd); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("update failure sends failed ack", func(t *testing.T) {
		var ack agentsdocker.CommandAck
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &ack)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		agent := &Agent{
			logger: zerolog.Nop(),
			hostID: "host1",
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
			docker: &fakeDockerClient{
				containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
					return containertypes.InspectResponse{}, errors.New("inspect failed")
				},
			},
		}

		cmd := agentsdocker.Command{
			ID:   "cmd4",
			Type: agentsdocker.CommandTypeUpdateContainer,
			Payload: map[string]any{
				"containerId": "container1",
			},
		}

		if err := agent.handleUpdateContainerCommand(context.Background(), TargetConfig{URL: server.URL, Token: "token"}, cmd); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ack.Status != agentsdocker.CommandStatusFailed {
			t.Fatalf("expected failed status, got %q", ack.Status)
		}
	})

	t.Run("completion ack error", func(t *testing.T) {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if calls == 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		agent := &Agent{
			logger: zerolog.Nop(),
			hostID: "host1",
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
			docker: &fakeDockerClient{
				containerInspectFn: func(_ context.Context, id string) (containertypes.InspectResponse, error) {
					inspect := baseInspect()
					if id == "new123" {
						inspect.ContainerJSONBase.Image = "sha256:new0000000000"
					}
					return inspect, nil
				},
				imagePullFn: func(context.Context, string, image.PullOptions) (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader("{}")), nil
				},
				containerStopFn: func(context.Context, string, containertypes.StopOptions) error {
					return nil
				},
				containerRenameFn: func(context.Context, string, string) error {
					return nil
				},
				containerCreateFn: func(context.Context, *containertypes.Config, *containertypes.HostConfig, *network.NetworkingConfig, *v1.Platform, string) (containertypes.CreateResponse, error) {
					return containertypes.CreateResponse{ID: "new123"}, nil
				},
				containerStartFn: func(context.Context, string, containertypes.StartOptions) error {
					return nil
				},
			},
		}

		cmd := agentsdocker.Command{
			ID:   "cmd5",
			Type: agentsdocker.CommandTypeUpdateContainer,
			Payload: map[string]any{
				"containerId": "container1",
			},
		}

		if err := agent.handleUpdateContainerCommand(context.Background(), TargetConfig{URL: server.URL, Token: "token"}, cmd); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
