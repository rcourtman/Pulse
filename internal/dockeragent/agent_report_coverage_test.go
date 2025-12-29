package dockeragent

import (
	"context"
	"errors"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	systemtypes "github.com/docker/docker/api/types/system"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	"github.com/rs/zerolog"
)

func TestBuildReport_RuntimeChangePodman(t *testing.T) {
	swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{
			CPUUsagePercent: 10,
			LoadAverage:     []float64{1.0, 0.5, 0.2},
		}, nil
	})

	inspect := containertypes.InspectResponse{
		ContainerJSONBase: &containertypes.ContainerJSONBase{
			State: &containertypes.State{Running: false},
		},
		Config: &containertypes.Config{},
	}

	agent := &Agent{
		cfg: Config{
			Interval:          0,
			IncludeServices:   true,
			IncludeTasks:      true,
			IncludeContainers: true,
		},
		runtime:    RuntimeDocker,
		daemonHost: "",
		daemonID:   "",
		machineID:  "machine-1",
		hostName:   "",
		logger:     zerolog.Nop(),
		docker: &fakeDockerClient{
			daemonHost: "unix:///run/podman/podman.sock",
			infoFunc: func(context.Context) (systemtypes.Info, error) {
				return systemtypes.Info{
					ID:              "daemon",
					Name:            "podman-host",
					ServerVersion:   "4.6.0",
					InitBinary:      "podman",
					NCPU:            4,
					Architecture:    "amd64",
					OperatingSystem: "linux",
				}, nil
			},
			containerListFunc: func(context.Context, containertypes.ListOptions) ([]containertypes.Summary, error) {
				return []containertypes.Summary{
					{ID: "container1", Names: []string{"/app"}, State: "exited"},
				}, nil
			},
			containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
				return inspect, nil, nil
			},
		},
	}

	report, err := agent.buildReport(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Agent.ID != "machine-1" {
		t.Fatalf("expected agent id to fall back to machine id")
	}
	if report.Host.Runtime != string(RuntimePodman) {
		t.Fatalf("expected runtime to be podman, got %q", report.Host.Runtime)
	}
	if report.Agent.IntervalSeconds == 0 {
		t.Fatalf("expected interval seconds to be defaulted")
	}
}

func TestBuildReport_CollectContainersForcedAndSwarmInfo(t *testing.T) {
	swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{}, nil
	})

	inspect := containertypes.InspectResponse{
		ContainerJSONBase: &containertypes.ContainerJSONBase{
			State: &containertypes.State{Running: false},
		},
		Config: &containertypes.Config{},
	}

	var listCalled bool
	agent := &Agent{
		cfg: Config{
			IncludeContainers: false,
			IncludeServices:   true,
			IncludeTasks:      false,
		},
		runtime:       RuntimeDocker,
		daemonHost:    "unix:///var/run/docker.sock",
		daemonID:      "",
		machineID:     "",
		hostName:      "override",
		supportsSwarm: true,
		logger:        zerolog.Nop(),
		docker: &fakeDockerClient{
			daemonHost: "unix:///var/run/docker.sock",
			infoFunc: func(context.Context) (systemtypes.Info, error) {
				return systemtypes.Info{
					ID:            "daemon",
					Name:          "docker-host",
					ServerVersion: "24.0.0",
					Swarm: swarmtypes.Info{
						NodeID:           "node1",
						ControlAvailable: false,
						LocalNodeState:   swarmtypes.LocalNodeStateActive,
					},
				}, nil
			},
			containerListFunc: func(context.Context, containertypes.ListOptions) ([]containertypes.Summary, error) {
				listCalled = true
				return []containertypes.Summary{
					{
						ID:    "container1",
						Names: []string{"/app"},
						State: "exited",
					},
				}, nil
			},
			containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
				return inspect, nil, nil
			},
		},
	}

	report, err := agent.buildReport(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Agent.ID != "override" {
		t.Fatalf("expected agent id to fall back to host override")
	}
	if report.Host.Swarm == nil {
		t.Fatalf("expected swarm info to be included")
	}
	if !listCalled {
		t.Fatalf("expected containers to be collected for fallback scope")
	}
	if len(report.Services) != 0 {
		t.Fatalf("expected no services when none derived")
	}
}

func TestBuildReport_HostMetricsError(t *testing.T) {
	swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{}, errors.New("metrics failed")
	})

	agent := &Agent{
		cfg: Config{
			IncludeContainers: true,
		},
		logger: zerolog.Nop(),
		docker: &fakeDockerClient{
			infoFunc: func(context.Context) (systemtypes.Info, error) {
				return systemtypes.Info{ServerVersion: "24.0.0"}, nil
			},
		},
	}

	if _, err := agent.buildReport(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildReport_CollectContainersError(t *testing.T) {
	swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{}, nil
	})

	agent := &Agent{
		cfg: Config{
			IncludeContainers: true,
		},
		logger: zerolog.Nop(),
		docker: &fakeDockerClient{
			infoFunc: func(context.Context) (systemtypes.Info, error) {
				return systemtypes.Info{ServerVersion: "24.0.0"}, nil
			},
			containerListFunc: func(context.Context, containertypes.ListOptions) ([]containertypes.Summary, error) {
				return nil, errors.New("list failed")
			},
		},
	}

	if _, err := agent.buildReport(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildReport_SwarmServicesTasks(t *testing.T) {
	swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{}, nil
	})

	inspect := containertypes.InspectResponse{
		ContainerJSONBase: &containertypes.ContainerJSONBase{
			State: &containertypes.State{Running: false},
		},
		Config: &containertypes.Config{},
	}

	agent := &Agent{
		cfg: Config{
			IncludeContainers: true,
			IncludeServices:   true,
			IncludeTasks:      true,
		},
		runtime:       RuntimeDocker,
		supportsSwarm: true,
		logger:        zerolog.Nop(),
		docker: &fakeDockerClient{
			infoFunc: func(context.Context) (systemtypes.Info, error) {
				return systemtypes.Info{
					ServerVersion: "24.0.0",
					Swarm: swarmtypes.Info{
						NodeID:           "node1",
						ControlAvailable: false,
						LocalNodeState:   swarmtypes.LocalNodeStateActive,
					},
				}, nil
			},
			containerListFunc: func(context.Context, containertypes.ListOptions) ([]containertypes.Summary, error) {
				return []containertypes.Summary{
					{
						ID:    "container1",
						Names: []string{"/web.1"},
						State: "running",
						Labels: map[string]string{
							"com.docker.swarm.service.id":   "svc1",
							"com.docker.swarm.service.name": "web",
							"com.docker.swarm.task.id":      "task1",
							"com.docker.swarm.task.slot":    "1",
						},
					},
				}, nil
			},
			containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
				return inspect, nil, nil
			},
		},
	}

	report, err := agent.buildReport(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Services) == 0 || len(report.Tasks) == 0 {
		t.Fatalf("expected services and tasks to be included")
	}
}

func TestPruneStaleCPUSamplesEmpty(t *testing.T) {
	agent := &Agent{
		prevContainerCPU: map[string]cpuSample{},
	}
	agent.pruneStaleCPUSamples(map[string]struct{}{"active": {}})
}

func TestCollectContainerFinishedAt(t *testing.T) {
	finishedAt := time.Now().Add(-time.Minute).Format(time.RFC3339Nano)

	inspect := containertypes.InspectResponse{
		ContainerJSONBase: &containertypes.ContainerJSONBase{
			State: &containertypes.State{
				Running:    false,
				FinishedAt: finishedAt,
			},
		},
		Config:          &containertypes.Config{},
		NetworkSettings: nil,
	}

	agent := &Agent{
		logger: zerolog.Nop(),
		docker: &fakeDockerClient{
			containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
				return inspect, nil, nil
			},
		},
	}

	container, err := agent.collectContainer(context.Background(), containertypes.Summary{ID: "container1", Names: []string{"/app"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if container.FinishedAt == nil {
		t.Fatalf("expected finished at to be set")
	}
	if len(container.Networks) != 0 {
		t.Fatalf("expected no networks when settings nil")
	}
}
