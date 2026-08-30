package dockeragent

import (
	"context"
	"errors"
	"testing"
	"time"

	systemtypes "github.com/moby/moby/api/types/system"
	"github.com/rcourtman/pulse-go-rewrite/internal/agenthelper"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

type helperInventoryStub struct {
	result agenthelper.ContainerInventoryResult
	err    error
	calls  int
}

func (s *helperInventoryStub) Inventory(context.Context) (agenthelper.ContainerInventoryResult, error) {
	s.calls++
	return s.result, s.err
}

func TestSelectHelperRuntimeHonorsPreferenceAndAvailability(t *testing.T) {
	result := agenthelper.ContainerInventoryResult{Runtimes: []agenthelper.ContainerRuntimeSnapshot{
		{Runtime: "docker", Available: true},
		{Runtime: "podman", Available: true},
	}}
	selected, err := selectHelperRuntime(result, RuntimeAuto)
	if err != nil || selected.Runtime != "docker" {
		t.Fatalf("auto selection = %+v, %v; want docker", selected, err)
	}
	selected, err = selectHelperRuntime(result, RuntimePodman)
	if err != nil || selected.Runtime != "podman" {
		t.Fatalf("podman selection = %+v, %v", selected, err)
	}
	if _, err := selectHelperRuntime(agenthelper.ContainerInventoryResult{}, RuntimeDocker); err == nil {
		t.Fatal("unavailable requested runtime was accepted")
	}
}

func TestNewFallsBackToTypedHelperWithoutActionAuthority(t *testing.T) {
	originalConnect := connectCollectorRuntimeFn
	t.Cleanup(func() { connectCollectorRuntimeFn = originalConnect })
	connectCollectorRuntimeFn = func(RuntimeKind, *zerolog.Logger) (dockerClient, systemtypes.Info, RuntimeKind, error) {
		return nil, systemtypes.Info{}, RuntimeAuto, errors.New("permission denied opening daemon socket")
	}
	helper := &helperInventoryStub{result: agenthelper.ContainerInventoryResult{
		Runtimes: []agenthelper.ContainerRuntimeSnapshot{{Runtime: "docker", Available: true}},
	}}
	logger := zerolog.Nop()
	agent, err := New(Config{
		PulseURL: "http://127.0.0.1:7655", APIToken: "token", Runtime: "auto",
		HelperInventory: helper, Logger: &logger,
	})
	if err != nil {
		t.Fatalf("New helper fallback: %v", err)
	}
	defer agent.Close()
	if agent.helperInventory != helper || agent.docker != nil {
		t.Fatalf("helper fallback wiring = helper:%T docker:%T", agent.helperInventory, agent.docker)
	}
	if agent.ContainerActionsAvailable() {
		t.Fatal("summary-only helper fallback exposed container action authority")
	}
	if helper.calls != 1 {
		t.Fatalf("helper probe calls = %d, want 1", helper.calls)
	}
}

func TestNewRejectsDirectRootfulSocketWhenTypedHelperIsConfigured(t *testing.T) {
	originalConnect := connectCollectorRuntimeFn
	t.Cleanup(func() { connectCollectorRuntimeFn = originalConnect })
	closed := false
	connectCollectorRuntimeFn = func(RuntimeKind, *zerolog.Logger) (dockerClient, systemtypes.Info, RuntimeKind, error) {
		return &fakeDockerClient{
			daemonHost: "unix:///var/run/docker.sock",
			closeFn:    func() error { closed = true; return nil },
		}, systemtypes.Info{ID: "rootful-daemon"}, RuntimeDocker, nil
	}
	helper := &helperInventoryStub{result: agenthelper.ContainerInventoryResult{
		Runtimes: []agenthelper.ContainerRuntimeSnapshot{{Runtime: "docker", Available: true}},
	}}
	logger := zerolog.Nop()
	agent, err := New(Config{
		PulseURL: "http://127.0.0.1:7655", APIToken: "token", Runtime: "auto",
		HelperInventory: helper, Logger: &logger,
	})
	if err != nil {
		t.Fatalf("New helper fallback: %v", err)
	}
	defer agent.Close()
	if !closed {
		t.Fatal("rejected rootful direct client was not closed")
	}
	if agent.helperInventory != helper || agent.ContainerActionsAvailable() {
		t.Fatalf("rootful endpoint bypassed helper boundary: helper=%T actions=%t", agent.helperInventory, agent.ContainerActionsAvailable())
	}
}

func TestBuildHelperInventoryReportIsExplicitlySummaryOnly(t *testing.T) {
	originalMetrics := hostmetricsCollectWithDiskFilters
	t.Cleanup(func() { hostmetricsCollectWithDiskFilters = originalMetrics })
	hostmetricsCollectWithDiskFilters = func(context.Context, []string, []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{
			CPUUsagePercent: 12.5,
			CPUCount:        4,
			Memory:          agentshost.MemoryMetric{TotalBytes: 4096, UsedBytes: 1024},
		}, nil
	}
	helper := &helperInventoryStub{result: agenthelper.ContainerInventoryResult{
		Runtimes: []agenthelper.ContainerRuntimeSnapshot{{
			Runtime: "docker", Available: true,
			Containers: []agenthelper.ContainerSummary{
				{ID: "1234567890abcdef", Names: []string{"/app"}, Image: "repo/app:1", State: "running", Status: "Up", Created: 1_700_000_000},
				{ID: "backup", Names: []string{"app_pulse_backup_20260830_120000"}, State: "exited"},
				{ID: "stopped", Names: []string{"stopped"}, State: "exited"},
			},
		}},
	}}
	agent := &Agent{
		cfg: Config{
			AgentID: "agent-1", AgentType: "unified", Interval: 15 * time.Second,
		},
		helperInventory: helper,
		runtimePref:     RuntimeAuto,
		runtime:         RuntimeDocker,
		agentVersion:    "6.0.0",
		hostName:        "node-1",
		machineID:       "machine-1",
		allowedStates:   map[string]struct{}{"running": {}},
		logger:          zerolog.Nop(),
	}
	report, err := agent.buildHelperInventoryReport(context.Background())
	if err != nil {
		t.Fatalf("buildHelperInventoryReport: %v", err)
	}
	if report.Host.CollectionMode != "typed-helper-summary" {
		t.Fatalf("collection mode = %q", report.Host.CollectionMode)
	}
	if len(report.Containers) != 1 {
		t.Fatalf("containers = %+v, want only running non-backup summary", report.Containers)
	}
	container := report.Containers[0]
	if container.ID != "1234567890abcdef" || container.Name != "app" || container.Image != "repo/app:1" {
		t.Fatalf("container summary = %+v", container)
	}
	if container.CPUPercent != 0 || container.MemoryUsageBytes != 0 || len(container.Env) != 0 || len(container.Mounts) != 0 {
		t.Fatalf("summary-only report fabricated privileged detail: %+v", container)
	}
	if report.Host.TotalMemoryBytes != 4096 || report.Host.TotalCPU != 4 || report.Agent.IntervalSeconds != 15 {
		t.Fatalf("host/agent summary = %+v / %+v", report.Host, report.Agent)
	}
}
