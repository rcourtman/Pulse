package dockeragent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

func TestTypedContainerLifecycleUsesConnectedDaemonAPI(t *testing.T) {
	containerID := strings.Repeat("a", 64)
	startedAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Nanosecond)
	restarts := 0
	agent := &Agent{
		runtime: RuntimeDocker,
		docker: &fakeDockerClient{
			containerInspectFn: func(_ context.Context, id string) (containertypes.InspectResponse, error) {
				if id != containerID {
					t.Fatalf("inspect id = %q", id)
				}
				return containertypes.InspectResponse{
					ID: containerID, RestartCount: 3,
					State: &containertypes.State{Status: containertypes.ContainerState("running"), Running: true, StartedAt: startedAt.Format(time.RFC3339Nano), Health: &containertypes.Health{Status: "healthy"}},
				}, nil
			},
			containerRestartFn: func(_ context.Context, id string, _ dockerContainerRestartOptions) error {
				if id != containerID {
					t.Fatalf("restart id = %q", id)
				}
				restarts++
				return nil
			},
		},
	}
	snapshot, err := agent.InspectDockerContainerLifecycle(context.Background(), "docker", containerID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ContainerID != containerID || snapshot.State != "running" || !snapshot.Running || snapshot.Health != agentexec.DockerContainerHealthHealthy || snapshot.RestartCount != 3 || !snapshot.StartedAt.Equal(startedAt) {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if err := agent.MutateDockerContainerLifecycle(context.Background(), "docker", "restart", containerID); err != nil {
		t.Fatal(err)
	}
	if restarts != 1 {
		t.Fatalf("restart calls = %d, want 1", restarts)
	}
}

func TestTypedContainerLifecycleMutationDoesNotRetryAmbiguousFailure(t *testing.T) {
	calls := 0
	agent := &Agent{
		runtime: RuntimeDocker,
		docker: &fakeDockerClient{containerRestartFn: func(context.Context, string, dockerContainerRestartOptions) error {
			calls++
			return errors.New("connection reset after request")
		}},
	}
	if err := agent.MutateDockerContainerLifecycle(context.Background(), "docker", "restart", strings.Repeat("b", 64)); err == nil {
		t.Fatal("ambiguous mutation failure was hidden")
	}
	if calls != 1 {
		t.Fatalf("mutation calls = %d, want exactly 1", calls)
	}
}
