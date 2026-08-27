package dockeragent

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	imagetypes "github.com/moby/moby/api/types/image"
	swarmtypes "github.com/moby/moby/api/types/swarm"
	systemtypes "github.com/moby/moby/api/types/system"
	dockerclient "github.com/moby/moby/client"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	"github.com/rs/zerolog"
)

func TestBuildReportSynologySizedInventoryBoundsStorageComputations(t *testing.T) {
	swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{}, nil
	})

	const (
		containerCount = 47
		runningCount   = 4
	)
	containers := make([]containertypes.Summary, 0, containerCount)
	running := make(map[string]bool, runningCount)
	for i := 0; i < containerCount; i++ {
		id := fmt.Sprintf("container-%02d", i)
		state := containertypes.ContainerState("exited")
		if i < runningCount {
			state = "running"
			running[id] = true
		}
		containers = append(containers, containertypes.Summary{
			ID:    id,
			Names: []string{"/" + id},
			State: state,
		})
	}

	var diskUsageCalls, imageListCalls, sharedSizeRequests int
	agent := &Agent{
		cfg: Config{
			Interval:          30 * time.Second,
			IncludeContainers: true,
		},
		runtime:          RuntimeDocker,
		logger:           zerolog.Nop(),
		prevContainerCPU: make(map[string]cpuSample),
		docker: &fakeDockerClient{
			infoFunc: func(context.Context) (systemtypes.Info, error) {
				return systemtypes.Info{ID: "synology-daemon", Name: "synology", ServerVersion: "24.0.0"}, nil
			},
			containerListFunc: func(context.Context, dockerContainerListOptions) ([]containertypes.Summary, error) {
				return containers, nil
			},
			containerInspectWithRawFn: func(_ context.Context, id string, size bool) (containertypes.InspectResponse, []byte, error) {
				if size {
					t.Fatal("normal unified-agent reports must not request per-container size walks")
				}
				return containertypes.InspectResponse{
					State:  &containertypes.State{Running: running[id]},
					Config: &containertypes.Config{},
				}, nil, nil
			},
			containerStatsOneShotFn: func(context.Context, string) (dockerStatsResponseReader, error) {
				return statsReader(t, containertypes.StatsResponse{}), nil
			},
			diskUsageFn: func(context.Context, dockerDiskUsageOptions) (dockerclient.DiskUsageResult, error) {
				diskUsageCalls++
				return dockerclient.DiskUsageResult{}, nil
			},
			imageListFn: func(_ context.Context, opts dockerImageListOptions) ([]imagetypes.Summary, error) {
				imageListCalls++
				if opts.SharedSize {
					sharedSizeRequests++
				}
				return nil, nil
			},
		},
	}

	for cycle := 1; cycle <= 2; cycle++ {
		report, err := agent.buildReport(context.Background())
		if err != nil {
			t.Fatalf("build report cycle %d: %v", cycle, err)
		}
		if len(report.Containers) != containerCount {
			t.Fatalf("cycle %d containers = %d, want %d", cycle, len(report.Containers), containerCount)
		}
	}

	if diskUsageCalls != 1 {
		t.Fatalf("full daemon storage walks = %d, want one across two live telemetry cycles", diskUsageCalls)
	}
	if imageListCalls != 2 {
		t.Fatalf("fresh image inventory calls = %d, want one per live telemetry cycle", imageListCalls)
	}
	if sharedSizeRequests != 0 {
		t.Fatalf("live image inventory requested %d shared-size computations, want none", sharedSizeRequests)
	}
}

func TestBuildReport_RuntimeChangePodman(t *testing.T) {
	swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{
			CPUUsagePercent: 10,
			LoadAverage:     []float64{1.0, 0.5, 0.2},
		}, nil
	})

	inspect := containertypes.InspectResponse{
		State:  &containertypes.State{Running: false},
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
			containerListFunc: func(context.Context, dockerContainerListOptions) ([]containertypes.Summary, error) {
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
		State:  &containertypes.State{Running: false},
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
			containerListFunc: func(context.Context, dockerContainerListOptions) ([]containertypes.Summary, error) {
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

func TestBuildReport_ReportsAuthorizationPlugins(t *testing.T) {
	swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{}, nil
	})

	agent := &Agent{
		cfg: Config{
			IncludeContainers: false,
		},
		runtime: RuntimeDocker,
		logger:  zerolog.Nop(),
		docker: &fakeDockerClient{
			infoFunc: func(context.Context) (systemtypes.Info, error) {
				return systemtypes.Info{
					Name:            "docker-host",
					ServerVersion:   "24.0.0",
					OperatingSystem: "linux",
					Plugins: systemtypes.PluginsInfo{
						Authorization: []string{"opa", " audit "},
					},
				}, nil
			},
		},
	}

	report, err := agent.buildReport(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Host.Security == nil {
		t.Fatalf("expected security posture in report")
	}
	if got := report.Host.Security.AuthorizationPlugins; len(got) != 2 || got[0] != "opa" || got[1] != "audit" {
		t.Fatalf("expected normalized authorization plugins, got %#v", got)
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
			containerListFunc: func(context.Context, dockerContainerListOptions) ([]containertypes.Summary, error) {
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
		State:  &containertypes.State{Running: false},
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
			containerListFunc: func(context.Context, dockerContainerListOptions) ([]containertypes.Summary, error) {
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
		State: &containertypes.State{
			Running:    false,
			FinishedAt: finishedAt,
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
