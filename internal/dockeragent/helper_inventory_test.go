package dockeragent

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	systemtypes "github.com/moby/moby/api/types/system"
	"github.com/rcourtman/pulse-go-rewrite/internal/agenthelper"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

type helperInventoryStub struct {
	result agenthelper.ContainerInventoryResult
	err    error
	calls  int
}

type helperOperationStatusEvent struct {
	operation string
	err       error
}

type helperOperationStatusRecorderStub struct {
	events []helperOperationStatusEvent
}

func (s *helperOperationStatusRecorderStub) Record(operation string, err error) {
	s.events = append(s.events, helperOperationStatusEvent{operation: operation, err: err})
}

func (s *helperOperationStatusRecorderStub) ModuleStatus() agentshost.ModuleStatus {
	status := agentshost.ModuleStatus{
		Name: agentshost.ModuleNameTypedPrivilegeHelper, Enabled: true, State: "running",
		UpdatedAt: time.Now().UTC(),
	}
	if len(s.events) > 0 && s.events[len(s.events)-1].err != nil {
		status.State = "degraded"
		status.LastError = "container.inventory: helper operation failed"
	}
	return status
}

func TestCollectHelperFailureDeliversIncompleteDegradationStatus(t *testing.T) {
	reports := make(chan agentsdocker.Report, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reader, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer reader.Close()
		var report agentsdocker.Report
		if err := json.NewDecoder(reader).Decode(&report); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		reports <- report
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	status := &helperOperationStatusRecorderStub{}
	helper := &helperInventoryStub{err: &agenthelper.RemoteError{
		Code: agenthelper.ErrorProviderUnavailable, Message: "token=must-not-leak", RequestID: "secret-id",
	}}
	agent := &Agent{
		cfg: Config{
			AgentID: "docker-only-agent", AgentType: "unified", Interval: 30 * time.Second,
			HelperOperationStatus: status,
		},
		helperInventory: helper,
		runtimePref:     RuntimeDocker,
		runtime:         RuntimeDocker,
		agentVersion:    "6.4.2",
		hostName:        "docker-only",
		machineID:       "machine-docker-only",
		targets: []TargetConfig{{
			Name: "primary", URL: server.URL, Token: "token", Authoritative: true,
		}},
		httpClients:    map[bool]*http.Client{false: server.Client()},
		logger:         zerolog.Nop(),
		reportStreamID: "docker-status-test-stream",
	}

	returned, err := agent.collectOnceWithReport(context.Background())
	if err == nil {
		t.Fatal("helper inventory failure was hidden")
	}
	if returned.InventoryComplete == nil || *returned.InventoryComplete {
		t.Fatalf("returned failure report inventoryComplete = %v", returned.InventoryComplete)
	}
	select {
	case report := <-reports:
		if report.InventoryComplete == nil || *report.InventoryComplete {
			t.Fatalf("delivered failure report inventoryComplete = %v", report.InventoryComplete)
		}
		if len(report.Containers) != 0 || len(report.Agent.Modules) != 1 || report.Agent.Modules[0].State != "degraded" {
			t.Fatalf("delivered helper failure report = %+v", report)
		}
		if stream, sequence, ok := agentshost.ParseReportSequenceID(report.SequenceID); !ok || stream != "docker-status-test-stream" || sequence != 1 {
			t.Fatalf("delivered helper status sequence = %q (%q, %d, %t)", report.SequenceID, stream, sequence, ok)
		}
		serialized, marshalErr := json.Marshal(report.Agent.Modules)
		if marshalErr != nil {
			t.Fatalf("marshal delivered helper modules: %v", marshalErr)
		}
		for _, secret := range []string{"must-not-leak", "secret-id"} {
			if strings.Contains(string(serialized), secret) {
				t.Fatalf("raw helper detail %q reached Docker status report: %s", secret, serialized)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("helper degradation status report was not delivered")
	}
}

func TestIncompleteHelperStatusIsNeverBufferedForStaleReplay(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "offline", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	incomplete := false
	agent := &Agent{
		targets: []TargetConfig{{
			Name: "primary", URL: server.URL, Token: "token", Authoritative: true,
		}},
		httpClients: map[bool]*http.Client{false: server.Client()},
		logger:      zerolog.Nop(),
	}
	if err := agent.deliverReport(context.Background(), agentsdocker.Report{InventoryComplete: &incomplete}); err != nil {
		t.Fatalf("deliver incomplete helper status: %v", err)
	}
	if got := agent.reportBuffers["primary"].Len(); got != 0 {
		t.Fatalf("incomplete helper status buffered for stale replay: %d reports", got)
	}
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

func TestNewRecordsTypedHelperProbeFailure(t *testing.T) {
	originalConnect := connectCollectorRuntimeFn
	t.Cleanup(func() { connectCollectorRuntimeFn = originalConnect })
	connectCollectorRuntimeFn = func(RuntimeKind, *zerolog.Logger) (dockerClient, systemtypes.Info, RuntimeKind, error) {
		return nil, systemtypes.Info{}, RuntimeAuto, errors.New("permission denied opening daemon socket")
	}
	helperErr := &agenthelper.RemoteError{
		Code: agenthelper.ErrorProviderUnavailable, Message: "token=must-not-persist", RequestID: "secret-id",
	}
	helper := &helperInventoryStub{err: helperErr}
	status := &helperOperationStatusRecorderStub{}
	logger := zerolog.Nop()
	agent, err := New(Config{
		PulseURL: "http://127.0.0.1:7655", APIToken: "token", Runtime: "auto",
		HelperInventory: helper, HelperOperationStatus: status, Logger: &logger,
	})
	if agent != nil || err == nil {
		t.Fatalf("failed helper probe = agent:%T err:%v", agent, err)
	}
	if len(status.events) != 1 || status.events[0].operation != agenthelper.OperationContainerInventory || !errors.Is(status.events[0].err, helperErr) {
		t.Fatalf("failed helper probe status = %+v", status.events)
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
	status := &helperOperationStatusRecorderStub{}
	logger := zerolog.Nop()
	agent, err := New(Config{
		PulseURL: "http://127.0.0.1:7655", APIToken: "token", Runtime: "auto",
		HelperInventory: helper, HelperOperationStatus: status, Logger: &logger,
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
	if len(status.events) != 1 || status.events[0].operation != agenthelper.OperationContainerInventory || status.events[0].err != nil {
		t.Fatalf("rootful rejection/helper recovery status = %+v", status.events)
	}
}

func TestHelperInventoryStatusTracksFailureAndCompleteRecovery(t *testing.T) {
	originalMetrics := hostmetricsCollectWithDiskFilters
	t.Cleanup(func() { hostmetricsCollectWithDiskFilters = originalMetrics })
	hostmetricsCollectWithDiskFilters = func(context.Context, []string, []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{}, nil
	}
	status := &helperOperationStatusRecorderStub{}
	helper := &helperInventoryStub{result: agenthelper.ContainerInventoryResult{
		Runtimes: []agenthelper.ContainerRuntimeSnapshot{{Runtime: "docker", Available: true}},
	}}
	agent := &Agent{
		cfg:             Config{HelperOperationStatus: status},
		helperInventory: helper,
		runtimePref:     RuntimeDocker,
		runtime:         RuntimeDocker,
		logger:          zerolog.Nop(),
	}

	if _, err := agent.buildHelperInventoryReport(context.Background()); err != nil {
		t.Fatalf("complete helper inventory: %v", err)
	}
	helper.err = errors.New("helper transport token=must-not-persist")
	if _, err := agent.buildHelperInventoryReport(context.Background()); err == nil {
		t.Fatal("helper transport failure was accepted")
	}
	helper.err = nil
	helper.result = agenthelper.ContainerInventoryResult{
		Runtimes: []agenthelper.ContainerRuntimeSnapshot{{Runtime: "docker", Available: false}},
	}
	if _, err := agent.buildHelperInventoryReport(context.Background()); err == nil {
		t.Fatal("unavailable helper runtime was accepted")
	}
	helper.result.Runtimes[0].Available = true
	if _, err := agent.buildHelperInventoryReport(context.Background()); err != nil {
		t.Fatalf("recovered helper inventory: %v", err)
	}

	if len(status.events) != 4 {
		t.Fatalf("status events = %+v, want success/failure/failure/success", status.events)
	}
	for _, event := range status.events {
		if event.operation != agenthelper.OperationContainerInventory {
			t.Fatalf("status operation = %q", event.operation)
		}
	}
	if status.events[0].err != nil || status.events[1].err == nil || status.events[2].err == nil || status.events[3].err != nil {
		t.Fatalf("status outcomes = %+v", status.events)
	}
}

func TestDirectRootlessLossFallsBackToCompleteHelperInventory(t *testing.T) {
	originalMetrics := hostmetricsCollectWithDiskFilters
	t.Cleanup(func() { hostmetricsCollectWithDiskFilters = originalMetrics })
	hostmetricsCollectWithDiskFilters = func(context.Context, []string, []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{}, nil
	}

	closed := false
	helper := &helperInventoryStub{result: agenthelper.ContainerInventoryResult{
		Runtimes: []agenthelper.ContainerRuntimeSnapshot{{
			Runtime: "docker", Available: true,
			Containers: []agenthelper.ContainerSummary{{ID: "container-1", Names: []string{"app"}, State: "running"}},
		}},
	}}
	agent := &Agent{
		cfg: Config{
			HelperInventory: helper, IncludeServices: true, IncludeTasks: true,
		},
		docker:            newSwappableDockerClient(&fakeDockerClient{closeFn: func() error { closed = true; return nil }}),
		runtime:           RuntimeDocker,
		runtimePref:       RuntimeDocker,
		runtimeGoneStreak: runtimeReconnectFailureThreshold,
		registryChecker:   newRegistryCheckerWithConfig(zerolog.Nop(), true),
		allowedStates:     map[string]struct{}{"running": {}},
		logger:            zerolog.Nop(),
	}

	if !agent.maybeFallbackToHelperInventory(context.Background()) {
		t.Fatal("complete helper inventory did not become the recovery source")
	}
	if !closed || agent.docker != nil || agent.helperInventory != helper {
		t.Fatalf("fallback state = closed:%t docker:%T helper:%T", closed, agent.docker, agent.helperInventory)
	}
	if agent.cfg.IncludeServices || agent.cfg.IncludeTasks || !agent.cfg.DisableUpdateChecks || agent.registryChecker.Enabled() {
		t.Fatalf("helper fallback retained direct-only configuration: %+v", agent.cfg)
	}
	report, err := agent.buildHelperInventoryReport(context.Background())
	if err != nil {
		t.Fatalf("build recovered helper report: %v", err)
	}
	if report.InventoryComplete == nil || !*report.InventoryComplete || report.Host.CollectionMode != agentsdocker.CollectionModeTypedHelperSummary || len(report.Containers) != 1 {
		t.Fatalf("recovered helper report = %+v", report)
	}
}

func TestDirectRootlessLossDefersHelperFallbackWhileBackgroundTaskRuns(t *testing.T) {
	helper := &helperInventoryStub{result: agenthelper.ContainerInventoryResult{
		Runtimes: []agenthelper.ContainerRuntimeSnapshot{{Runtime: "docker", Available: true}},
	}}
	agent := &Agent{
		cfg:                Config{HelperInventory: helper},
		docker:             &fakeDockerClient{},
		runtimeGoneStreak:  runtimeReconnectFailureThreshold,
		cleanupTaskRunning: true,
		logger:             zerolog.Nop(),
	}
	if agent.maybeFallbackToHelperInventory(context.Background()) {
		t.Fatal("helper fallback raced an active direct-runtime cleanup")
	}
	if helper.calls != 0 || agent.helperInventory != nil || agent.docker == nil {
		t.Fatalf("busy fallback mutated state: calls=%d helper=%T docker=%T", helper.calls, agent.helperInventory, agent.docker)
	}
}

func TestMonitoringOnlyCollectorNeverSchedulesLegacyMutationTasks(t *testing.T) {
	helper := &helperInventoryStub{}
	agent := &Agent{
		cfg:    Config{HelperInventory: helper},
		docker: &fakeDockerClient{},
		logger: zerolog.Nop(),
	}
	agent.scheduleDirectCleanup()
	agent.scheduleDirectUpdateCheck()
	if agent.cleanupTaskRunning || agent.updateCheckRunning {
		t.Fatalf("monitoring-only collector scheduled legacy tasks: cleanup=%t update=%t", agent.cleanupTaskRunning, agent.updateCheckRunning)
	}
	agent.cleanupOrphanedBackups(context.Background())
	agent.checkForUpdates(context.Background())
	if agent.cleanupTaskRunning || agent.updateCheckRunning {
		t.Fatalf("direct legacy task entrypoints bypassed profile gate: cleanup=%t update=%t", agent.cleanupTaskRunning, agent.updateCheckRunning)
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
