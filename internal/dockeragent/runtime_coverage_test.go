package dockeragent

import (
	"context"
	"errors"
	"strings"
	"testing"

	systemtypes "github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog"
)

func TestTryRuntimeCandidate(t *testing.T) {
	t.Run("new client error", func(t *testing.T) {
		swap(t, &newDockerClientFn, func(_ ...client.Opt) (dockerClient, error) {
			return nil, errors.New("dial failed")
		})

		if _, _, err := tryRuntimeCandidate(nil); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("info error closes client", func(t *testing.T) {
		closed := false
		fake := &fakeDockerClient{
			infoFunc: func(_ context.Context) (systemtypes.Info, error) {
				return systemtypes.Info{}, errors.New("info failed")
			},
			closeFn: func() error {
				closed = true
				return nil
			},
		}
		swap(t, &newDockerClientFn, func(_ ...client.Opt) (dockerClient, error) {
			return fake, nil
		})

		if _, _, err := tryRuntimeCandidate(nil); err == nil {
			t.Fatal("expected error")
		}
		if !closed {
			t.Fatal("expected Close to be called on error")
		}
	})

	t.Run("success", func(t *testing.T) {
		fake := &fakeDockerClient{
			infoFunc: func(_ context.Context) (systemtypes.Info, error) {
				return systemtypes.Info{ServerVersion: "24.0.0"}, nil
			},
		}
		swap(t, &newDockerClientFn, func(_ ...client.Opt) (dockerClient, error) {
			return fake, nil
		})

		gotClient, info, err := tryRuntimeCandidate(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotClient != fake {
			t.Fatal("expected returned client to match fake")
		}
		if info.ServerVersion != "24.0.0" {
			t.Fatalf("expected info to be returned")
		}
	})
}

func TestConnectRuntime(t *testing.T) {
	t.Run("no candidates", func(t *testing.T) {
		swap(t, &buildRuntimeCandidatesFn, func(_ RuntimeKind) []runtimeCandidate {
			return nil
		})

		if _, _, _, err := connectRuntime(RuntimeAuto, nil); err == nil {
			t.Fatal("expected error with no candidates")
		}
	})

	t.Run("candidate failure accumulates attempts", func(t *testing.T) {
		swap(t, &buildRuntimeCandidatesFn, func(_ RuntimeKind) []runtimeCandidate {
			return []runtimeCandidate{{label: "first"}}
		})
		swap(t, &tryRuntimeCandidateFn, func(_ []client.Opt) (dockerClient, systemtypes.Info, error) {
			return nil, systemtypes.Info{}, errors.New("no socket")
		})

		if _, _, _, err := connectRuntime(RuntimeAuto, nil); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("preference mismatch returns error", func(t *testing.T) {
		fake := &fakeDockerClient{daemonHost: "unix:///run/podman/podman.sock"}
		swap(t, &buildRuntimeCandidatesFn, func(_ RuntimeKind) []runtimeCandidate {
			return []runtimeCandidate{{label: "podman"}}
		})
		swap(t, &tryRuntimeCandidateFn, func(_ []client.Opt) (dockerClient, systemtypes.Info, error) {
			return fake, systemtypes.Info{ServerVersion: "4.6.1"}, nil
		})

		_, _, _, err := connectRuntime(RuntimeDocker, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "detected podman runtime") {
			t.Fatalf("expected mismatch error, got %v", err)
		}
	})

	t.Run("success with logger", func(t *testing.T) {
		fake := &fakeDockerClient{daemonHost: "unix:///var/run/docker.sock"}
		swap(t, &buildRuntimeCandidatesFn, func(_ RuntimeKind) []runtimeCandidate {
			return []runtimeCandidate{{label: "docker"}}
		})
		swap(t, &tryRuntimeCandidateFn, func(_ []client.Opt) (dockerClient, systemtypes.Info, error) {
			return fake, systemtypes.Info{ServerVersion: "24.0.0"}, nil
		})

		logger := zerolog.Nop()
		cli, info, runtimeKind, err := connectRuntime(RuntimeAuto, &logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cli != fake {
			t.Fatalf("expected client to be returned")
		}
		if info.ServerVersion != "24.0.0" {
			t.Fatalf("expected info to be returned")
		}
		if runtimeKind != RuntimeDocker {
			t.Fatalf("expected runtime docker, got %v", runtimeKind)
		}
	})
}
