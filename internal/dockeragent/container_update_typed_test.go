package dockeragent

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/rs/zerolog"
)

func TestTypedContainerUpdatePreflightRefusesRuntimeAndDigestDrift(t *testing.T) {
	t.Run("runtime mismatch", func(t *testing.T) {
		inspectCalled := false
		agent := &Agent{
			runtime: RuntimeDocker,
			docker: &fakeDockerClient{
				containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
					inspectCalled = true
					return baseInspect(), nil
				},
			},
		}
		if _, err := agent.TypedContainerUpdate(context.Background(), "podman", "container-id", "", nil); err == nil ||
			!strings.Contains(err.Error(), "runtime mismatch") {
			t.Fatalf("runtime mismatch error = %v", err)
		}
		if inspectCalled {
			t.Fatal("runtime mismatch reached the daemon")
		}
	})

	t.Run("digest drift", func(t *testing.T) {
		pullCalled := false
		agent := &Agent{
			runtime: RuntimeDocker,
			docker: &fakeDockerClient{
				containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
					return baseInspect(), nil
				},
				imagePullFn: func(context.Context, string, dockerImagePullOptions) (io.ReadCloser, error) {
					pullCalled = true
					return io.NopCloser(strings.NewReader("{}")), nil
				},
			},
		}
		if _, err := agent.TypedContainerUpdate(context.Background(), "docker", "container-id", "sha256:planned", nil); err == nil ||
			!strings.Contains(err.Error(), "digest no longer matches") {
			t.Fatalf("digest drift error = %v", err)
		}
		if pullCalled {
			t.Fatal("digest drift crossed the mutation preflight")
		}
	})
}

func TestTypedContainerUpdateDelegatesToProductionRecreatePath(t *testing.T) {
	swap(t, &sleepFn, func(time.Duration) {})
	swap(t, &newTimerFn, func(time.Duration) *time.Timer { return time.NewTimer(0) })

	cleanupDone := make(chan struct{})
	agent := &Agent{
		runtime: RuntimeDocker,
		docker: &fakeDockerClient{
			containerInspectFn: func(_ context.Context, id string) (containertypes.InspectResponse, error) {
				inspect := baseInspect()
				if id == "replacement-id" {
					inspect.Image = "sha256:new"
				}
				return inspect, nil
			},
			imagePullFn: func(context.Context, string, dockerImagePullOptions) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("{}")), nil
			},
			containerStopFn: func(context.Context, string, dockerContainerStopOptions) error {
				return nil
			},
			containerRenameFn: func(context.Context, string, string) error {
				return nil
			},
			containerCreateFn: func(context.Context, *containertypes.Config, *containertypes.HostConfig, *network.NetworkingConfig, *v1.Platform, string) (containertypes.CreateResponse, error) {
				return containertypes.CreateResponse{ID: "replacement-id"}, nil
			},
			containerStartFn: func(context.Context, string, dockerContainerStartOptions) error {
				return nil
			},
			containerRemoveFn: func(context.Context, string, dockerContainerRemoveOptions) error {
				close(cleanupDone)
				return nil
			},
		},
		logger: zerolog.Nop(),
	}

	var progress []string
	outcome, err := agent.TypedContainerUpdate(
		context.Background(),
		"docker",
		"container-id",
		"sha256:old0000000000",
		func(step string) { progress = append(progress, step) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Success || outcome.OldContainerID != "container-id" ||
		outcome.NewContainerID != "replacement-id" || !outcome.BackupCreated ||
		outcome.NewImageDigest != "sha256:new" {
		t.Fatalf("typed outcome = %+v", outcome)
	}
	if !containsStringFragment(progress, "Pulling image") ||
		!containsStringFragment(progress, "Creating new container") ||
		!containsStringFragment(progress, "Verifying container stability") {
		t.Fatalf("typed production-path progress = %v", progress)
	}
	<-cleanupDone
}
