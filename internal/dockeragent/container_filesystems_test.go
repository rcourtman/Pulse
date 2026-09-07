package dockeragent

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/rcourtman/pulse-go-rewrite/internal/filesystemprobe"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rcourtman/pulse-go-rewrite/pkg/agents/filesystem"
)

func TestCollectContainerFilesystemIdentityBoundary(t *testing.T) {
	for _, scenario := range []string{"measured", "remote daemon", "wrong inspect", "process restarted", "revalidation failed", "native unavailable", "layer sizing disabled"} {
		t.Run(scenario, func(t *testing.T) {
			id := strings.Repeat("a", 64)
			inspect := baseInspect()
			inspect.ID = id
			inspect.State.Pid = 42
			inspect.State.Running = true
			inspect.State.StartedAt = time.Now().Format(time.RFC3339Nano)
			client := &fakeDockerClient{daemonHost: "unix:///var/run/docker.sock"}
			client.containerInspectFn = func(context.Context, string) (containertypes.InspectResponse, error) {
				if scenario == "revalidation failed" {
					return containertypes.InspectResponse{}, errors.New("gone")
				}
				current := inspect
				state := *inspect.State
				current.State = &state
				if scenario == "process restarted" {
					current.State.Pid++
				}
				return current, nil
			}
			calls := 0
			a := &Agent{cfg: Config{CollectDiskMetrics: scenario != "layer sizing disabled"}, docker: client, runtime: RuntimeDocker,
				observeFilesystems: func(_ context.Context, req filesystemprobe.ContainerRequest) ([]filesystem.Observation, error) {
					calls++
					if req.ContainerID != id || req.PID != 42 || req.Runtime != "docker" || !reflect.DeepEqual(req.Mountpoints, []string{"/", "/cache"}) {
						t.Fatalf("wrong namespace coordinates: %+v", req)
					}
					if scenario == "native unavailable" {
						return nil, errors.New("namespace unavailable")
					}
					return []filesystem.Observation{{Mountpoint: "/cache", Source: filesystemprobe.Source, ObservedAt: time.Now(), Type: "tmpfs", Usage: &filesystem.Usage{CapacityBytes: 8 << 20}}}, nil
				},
			}
			if scenario == "remote daemon" {
				client.daemonHost = "tcp://remote.invalid:2376"
			}
			if scenario == "wrong inspect" {
				inspect.ID = strings.Repeat("b", 64)
			}
			got := a.collectContainerFilesystems(context.Background(), id, inspect, []agentsdocker.ContainerMount{{Destination: "/cache"}, {Destination: "/cache"}})
			if scenario == "measured" || scenario == "layer sizing disabled" {
				if calls != 1 || len(got) != 1 || got[0].Usage == nil || got[0].Usage.AvailableBytes != 0 {
					t.Fatalf("exhaustion lost: %+v", got)
				}
				return
			}
			if len(got) != 2 {
				t.Fatalf("missing failure scope: %+v", got)
			}
			for _, row := range got {
				if row.Usage != nil || row.Error == "" {
					t.Fatalf("unverified observation became capacity: %+v", row)
				}
			}
			if (scenario == "remote daemon" || scenario == "wrong inspect") && calls != 0 {
				t.Fatal("unattested target reached native probe")
			}
		})
	}
}
