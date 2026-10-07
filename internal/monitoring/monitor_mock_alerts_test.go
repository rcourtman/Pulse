package monitoring

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestCheckMockDockerHostAlertsUsesHostLifecycleForOfflineFixtures(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	config := manager.GetConfig()
	config.Enabled = true
	config.ActivationState = alerts.ActivationPending
	config.TimeThresholds = map[string]int{}
	config.DockerDefaults.StateDisableConnectivity = false
	manager.UpdateConfig(config)

	host := models.DockerHost{
		ID:          "branch-edge",
		Hostname:    "branch-edge-01",
		DisplayName: "Branch Edge",
		Status:      "offline",
		Containers: []models.DockerContainer{
			{ID: "portal-1", Name: "branch-portal", State: "exited", Status: "Exited (137)"},
			{ID: "sync-1", Name: "branch-syncthing", State: "exited", Status: "Exited (137)"},
			{ID: "vpn-1", Name: "branch-vpn", State: "exited", Status: "Exited (137)"},
		},
	}

	// Prove the fixture would create the child storm if it were incorrectly
	// treated as fresh online telemetry.
	manager.CheckDockerHost(host)
	manager.CheckDockerHost(host)
	if got := manager.GetActiveAlerts(); len(got) != len(host.Containers) {
		t.Fatalf("fresh container evaluation created %d alerts, want %d", len(got), len(host.Containers))
	}

	monitor := &Monitor{alertManager: manager}
	monitor.checkMockDockerHostAlerts(host)
	monitor.checkMockDockerHostAlerts(host)
	monitor.checkMockDockerHostAlerts(host)

	active := manager.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("offline mock host produced %d active alerts, want one host incident: %+v", len(active), active)
	}
	if active[0].Type != "docker-host-offline" || active[0].ResourceID != "docker:branch-edge" {
		t.Fatalf("offline mock host alert = %+v, want canonical host connectivity incident", active[0])
	}
}

func newMockHostAlertTestManager(t *testing.T) *alerts.Manager {
	t.Helper()
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	config := manager.GetConfig()
	config.Enabled = true
	config.ActivationState = alerts.ActivationPending
	config.TimeThresholds = map[string]int{}
	config.MetricTimeThresholds = nil
	config.BackupDefaults.Enabled = false
	config.DockerDefaults.StateDisableConnectivity = false
	config.AgentDefaults.Memory = &alerts.HysteresisThreshold{Trigger: 1, Clear: 0.5}
	config.NodeDefaults.Memory = &alerts.HysteresisThreshold{Trigger: 1, Clear: 0.5}
	manager.UpdateConfig(config)
	return manager
}

func TestCheckMockAlertsEvaluatesHostAgentsBeforeLinkedNodes(t *testing.T) {
	mustSetMockEnabled(t, true)
	t.Cleanup(func() { mustSetMockEnabled(t, false) })
	manager := newMockHostAlertTestManager(t)

	monitor := &Monitor{alertManager: manager}
	monitor.checkMockAlerts()

	// Agents reading well above the 1% trigger must alert. A NotReady
	// Kubernetes node's agent validly reads 0%, and a later fixture read may
	// drift slightly from the pass's own snapshot, so the bound leaves margin.
	state := mock.CurrentFixtureGraph().State
	onlineHosts := make(map[string]models.Host, len(state.Hosts))
	for _, host := range state.Hosts {
		if !strings.EqualFold(strings.TrimSpace(host.Status), "offline") && host.Memory.HasKnownUsage() && host.Memory.Usage >= 5 {
			onlineHosts[host.ID] = host
		}
	}
	if len(onlineHosts) == 0 {
		t.Fatal("mock fixture has no online host agents reading above 5% memory")
	}

	active := manager.GetActiveAlerts()
	agentMemoryAlerts := 0
	alertByResourceMetric := make(map[string]bool, len(active))
	for _, alert := range active {
		alertByResourceMetric[alert.ResourceID+"/"+alert.Type] = true
		if alert.Type != "memory" || !strings.HasPrefix(alert.ResourceID, "agent:") {
			continue
		}
		if _, ok := onlineHosts[strings.TrimPrefix(alert.ResourceID, "agent:")]; ok {
			agentMemoryAlerts++
		}
	}
	if agentMemoryAlerts != len(onlineHosts) {
		t.Fatalf("mock pass opened %d agent memory alerts at a 1%% trigger, want one per online agent above 5%% (%d)", agentMemoryAlerts, len(onlineHosts))
	}

	// A node whose linked agent reported on the same pass must hand its
	// memory alert to the agent on the first tick, as node-link
	// deduplication does for live agents.
	linkedNodes := 0
	for _, node := range state.Nodes {
		host, ok := onlineHosts[strings.TrimSpace(node.LinkedAgentID)]
		if !ok || strings.TrimSpace(host.LinkedNodeID) != node.ID {
			continue
		}
		linkedNodes++
		if alertByResourceMetric[node.ID+"/memory"] {
			t.Fatalf("node %s raised its own memory alert although linked agent %s reported on the same pass", node.Name, host.ID)
		}
	}
	if linkedNodes == 0 {
		t.Fatal("mock fixture has no online node with a linked agent")
	}
}

func TestCheckMockHostAlertsUsesHostLifecycleForOfflineFixtures(t *testing.T) {
	manager := newMockHostAlertTestManager(t)
	monitor := &Monitor{alertManager: manager}

	host := models.Host{
		ID:          "host-linux-9",
		Hostname:    "branch-gateway",
		DisplayName: "Branch Gateway",
		Platform:    "linux",
		Status:      "offline",
		Memory:      models.Memory{Total: 16 << 30, Used: 15 << 30, Usage: 93.75},
	}

	// Three offline observations confirm the connectivity incident. CheckHost
	// would treat the stale fixture as a fresh report: it marks the host
	// online and raises the stale 94% memory reading instead.
	monitor.checkMockHostAlerts(host, nil)
	monitor.checkMockHostAlerts(host, nil)
	monitor.checkMockHostAlerts(host, nil)

	active := manager.GetActiveAlerts()
	if len(active) != 1 || active[0].Type != alerts.HostOfflineAlertType || active[0].ResourceID != "agent:host-linux-9" {
		got := make([]string, 0, len(active))
		for _, alert := range active {
			got = append(got, alert.ResourceID+" "+alert.Type)
		}
		t.Fatalf("offline mock agent alerts = %v, want only the host-offline incident for agent:host-linux-9", got)
	}

	host.Status = "online"
	monitor.checkMockHostAlerts(host, nil)
	for _, alert := range manager.GetActiveAlerts() {
		if alert.Type == alerts.HostOfflineAlertType {
			t.Fatalf("recovered mock agent kept its offline incident %s", alert.ID)
		}
	}
}

func TestLeavingMockModeReleasesFixtureAgentNodeLinks(t *testing.T) {
	setMockSamplerTestEnv(t, time.Hour, 5*time.Minute)
	manager := newMockHostAlertTestManager(t)
	monitor := &Monitor{
		state:          models.NewState(),
		alertManager:   manager,
		metricsHistory: NewMetricsHistory(10, time.Hour),
	}
	mustSetMonitorMockMode(t, monitor, true)
	t.Cleanup(func() { mustSetMockEnabled(t, false) })
	monitor.checkMockAlerts()

	linkedNodeID := ""
	for _, host := range mock.CurrentFixtureGraph().State.Hosts {
		if host.LinkedNodeID != "" && !strings.EqualFold(host.Status, "offline") {
			linkedNodeID = host.LinkedNodeID
			break
		}
	}
	if linkedNodeID == "" {
		t.Fatal("mock fixture has no online agent linked to a node")
	}
	mustSetMonitorMockMode(t, monitor, false)

	// A node carrying the linked fixture node's ID, with no agent of its own,
	// must own its metric alerts once the monitor shows live data again.
	node := models.Node{
		ID:       linkedNodeID,
		Name:     "pve1",
		Instance: "real",
		Status:   "online",
		Memory:   models.Memory{Total: 64 << 30, Used: 60 << 30, Free: 4 << 30, Usage: 93.75},
	}
	manager.CheckNode(node)
	for _, alert := range manager.GetActiveAlerts() {
		if alert.ResourceID == node.ID && alert.Type == "memory" {
			return
		}
	}
	t.Fatalf("node %s raised no memory alert: a fixture agent's node link outlived mock mode", node.ID)
}

func TestMockHostAgentPassAfterLeavingMockModeRegistersNothing(t *testing.T) {
	manager := newMockHostAlertTestManager(t)
	monitor := &Monitor{alertManager: manager}

	// A pass whose snapshot predates leaving mock mode reaches the agent step
	// after SetMockMode ended its epoch and forgetMockFixtureHosts ran.
	scope := monitor.mockModeFence.begin()
	monitor.mockModeFence.advance()
	stale := models.Host{
		ID:           "host-node-mock-cluster-1-pve1",
		Hostname:     "pve1",
		LinkedNodeID: "real-pve1",
		Status:       "online",
		Memory:       models.Memory{Total: 32 << 30, Used: 30 << 30, Free: 2 << 30, Usage: 93.75},
	}
	monitor.evaluateMockHostAgents(scope, []models.Host{stale}, nil, 1)

	if active := manager.GetActiveAlerts(); len(active) != 0 {
		t.Fatalf("stale mock pass opened %d alerts in live mode", len(active))
	}
	if len(monitor.mockHostAgents) != 0 {
		t.Fatalf("stale mock pass registered fixture agents %v after they were forgotten", monitor.mockHostAgents)
	}
	node := models.Node{
		ID:       "real-pve1",
		Name:     "pve1",
		Instance: "real",
		Status:   "online",
		Memory:   models.Memory{Total: 64 << 30, Used: 60 << 30, Free: 4 << 30, Usage: 93.75},
	}
	manager.CheckNode(node)
	for _, alert := range manager.GetActiveAlerts() {
		if alert.ResourceID == node.ID && alert.Type == "memory" {
			return
		}
	}
	t.Fatal("stale mock pass linked a fixture agent to real-pve1 and suppressed its memory alert")
}

// newMockDockerHostWithExitedContainers returns a reporting Docker host whose
// containers exited, so two evaluation passes open one alert per container.
func newMockDockerHostWithExitedContainers(id, hostname string) models.DockerHost {
	return models.DockerHost{
		ID:          id,
		Hostname:    hostname,
		DisplayName: hostname,
		Status:      "online",
		Containers: []models.DockerContainer{
			{ID: id + "-web", Name: "web", State: "exited", Status: "Exited (137)"},
			{ID: id + "-db", Name: "db", State: "exited", Status: "Exited (137)"},
		},
	}
}

func dockerAlertIDsForHost(manager *alerts.Manager, hostID string) []string {
	var ids []string
	for _, alert := range manager.GetActiveAlerts() {
		if alert.ResourceID == "docker:"+hostID || strings.HasPrefix(alert.ResourceID, "docker:"+hostID+"/") {
			ids = append(ids, alert.ID)
		}
	}
	return ids
}

func TestMockDockerPassAfterLeavingMockModeRaisesNothing(t *testing.T) {
	manager := newMockHostAlertTestManager(t)
	monitor := &Monitor{alertManager: manager}
	mustSetMockEnabled(t, false)

	// A pass whose snapshot predates leaving mock mode reaches the Docker
	// step after forgetMockFixtureHosts ran.
	host := newMockDockerHostWithExitedContainers("nebula-1-mock", "nebula-1")
	monitor.evaluateMockDockerHosts(monitor.mockModeFence.begin(), []models.DockerHost{host}, 1)
	monitor.evaluateMockDockerHosts(monitor.mockModeFence.begin(), []models.DockerHost{host}, 1)

	if ids := dockerAlertIDsForHost(manager, host.ID); len(ids) != 0 {
		t.Fatalf("stale mock pass opened Docker alerts in live mode: %v", ids)
	}
}

func TestLeavingMockModeRemovesFixtureDockerHostAlerts(t *testing.T) {
	mustSetMockEnabled(t, true)
	t.Cleanup(func() { mustSetMockEnabled(t, false) })
	manager := newMockHostAlertTestManager(t)
	monitor := &Monitor{alertManager: manager}

	host := newMockDockerHostWithExitedContainers("nebula-1-mock", "nebula-1")
	monitor.evaluateMockDockerHosts(monitor.mockModeFence.begin(), []models.DockerHost{host}, 1)
	monitor.evaluateMockDockerHosts(monitor.mockModeFence.begin(), []models.DockerHost{host}, 1)
	if ids := dockerAlertIDsForHost(manager, host.ID); len(ids) != len(host.Containers) {
		t.Fatalf("fixture Docker host opened %v, want one alert per exited container", ids)
	}

	// The alerts stand in for ones a pass in flight recreated after
	// SetMockMode(false) cleared the manager.
	mustSetMockEnabled(t, false)
	monitor.forgetMockFixtureHosts()

	if ids := dockerAlertIDsForHost(manager, host.ID); len(ids) != 0 {
		t.Fatalf("fixture Docker host alerts outlived mock mode: %v", ids)
	}
}

// Upstream mode epochs and the reviewed structural revisions protect different
// boundaries. A current revision cannot make an ended epoch eligible again.
func TestMockFixtureCurrentRevisionCannotReviveAnEndedEpoch(t *testing.T) {
	mustSetMockEnabled(t, true)
	t.Cleanup(func() { mustSetMockEnabled(t, false) })
	manager := newMockHostAlertTestManager(t)
	monitor := &Monitor{alertManager: manager}
	stale := monitor.mockModeFence.begin()
	monitor.mockModeFence.advance()
	_, revision := mock.CurrentFixtureGraphWithRevision()
	agent := models.Host{ID: "epoch-agent", Hostname: "epoch-agent", Status: "online"}
	docker := newMockDockerHostWithExitedContainers("epoch-docker", "epoch-docker")

	monitor.evaluateMockHostAgents(stale, []models.Host{agent}, nil, revision)
	for range 2 {
		monitor.evaluateMockDockerHosts(stale, []models.DockerHost{docker}, revision)
	}
	if len(monitor.mockHostAgents) != 0 || len(monitor.mockDockerHosts) != 0 || len(manager.GetActiveAlerts()) != 0 {
		t.Fatal("a current fixture revision revived an ended mode epoch")
	}

	current := monitor.mockModeFence.begin()
	monitor.evaluateMockHostAgents(current, []models.Host{agent}, nil, revision)
	for range 2 {
		monitor.evaluateMockDockerHosts(current, []models.DockerHost{docker}, revision)
	}
	if len(monitor.mockHostAgents) != 1 || len(monitor.mockDockerHosts) != 1 || len(dockerAlertIDsForHost(manager, docker.ID)) != len(docker.Containers) {
		t.Fatal("the current epoch lost normal fixture ownership or Docker evaluation")
	}
}

func TestMockFixturePassFromBeforeAnEstateRebuildIsRejected(t *testing.T) {
	mustSetMockEnabled(t, true)
	t.Cleanup(func() { mustSetMockEnabled(t, false) })
	manager := newMockHostAlertTestManager(t)
	monitor := &Monitor{alertManager: manager}

	retired := newMockDockerHostWithExitedContainers("proxmox-lxc-docker:Production West:pve1:108", "pve1-ct108")
	current := newMockDockerHostWithExitedContainers("proxmox-lxc-docker:Production West:pve1:105", "pve1-ct105")
	monitor.evaluateMockDockerHosts(monitor.mockModeFence.begin(), []models.DockerHost{current}, 2)
	monitor.evaluateMockDockerHosts(monitor.mockModeFence.begin(), []models.DockerHost{current}, 2)

	// A slow pass whose snapshot predates the rebuild reaches the lock after
	// the newer passes. Applying it would remove the current host and
	// re-evaluate the retired one.
	monitor.evaluateMockDockerHosts(monitor.mockModeFence.begin(), []models.DockerHost{retired}, 1)
	monitor.evaluateMockDockerHosts(monitor.mockModeFence.begin(), []models.DockerHost{retired}, 1)

	if ids := dockerAlertIDsForHost(manager, current.ID); len(ids) != len(current.Containers) {
		t.Fatalf("superseded pass disturbed the current host's alerts: %v", ids)
	}
	if ids := dockerAlertIDsForHost(manager, retired.ID); len(ids) != 0 {
		t.Fatalf("superseded pass re-evaluated a retired host: %v", ids)
	}
}

func TestMockFixturePassPausedAcrossDisableAndReenableIsRejected(t *testing.T) {
	mustSetMockEnabled(t, true)
	t.Cleanup(func() { mustSetMockEnabled(t, false) })
	manager := newMockHostAlertTestManager(t)
	monitor := &Monitor{alertManager: manager}

	retired := newMockDockerHostWithExitedContainers("proxmox-lxc-docker:Production West:pve1:108", "pve1-ct108")
	_, staleRevision := mock.CurrentFixtureGraphWithRevision()
	monitor.evaluateMockDockerHosts(monitor.mockModeFence.begin(), []models.DockerHost{retired}, staleRevision)

	// Mock mode goes off and back on while a pass holding the old snapshot
	// is paused; it resumes before any pass of the rebuilt estate applies.
	mustSetMockEnabled(t, false)
	monitor.forgetMockFixtureHosts()
	mustSetMockEnabled(t, true)
	monitor.evaluateMockDockerHosts(monitor.mockModeFence.begin(), []models.DockerHost{retired}, staleRevision)
	monitor.evaluateMockDockerHosts(monitor.mockModeFence.begin(), []models.DockerHost{retired}, staleRevision)

	if ids := dockerAlertIDsForHost(manager, retired.ID); len(ids) != 0 {
		t.Fatalf("pass from before the disable re-evaluated the old estate: %v", ids)
	}
}

// pinDefaultMockEstate enables mock mode on the default estate, whatever
// PULSE_MOCK_* the shell exports, and restores the previous mode and config.
func pinDefaultMockEstate(t *testing.T) {
	t.Helper()
	previousEnabled, previousConfig := mock.IsMockEnabled(), mock.GetConfig()
	t.Cleanup(func() {
		mustSetMockEnabled(t, false)
		mock.SetMockConfig(previousConfig)
		mustSetMockEnabled(t, previousEnabled)
	})
	mustSetMockEnabled(t, false)
	cfg := mock.DefaultConfig
	cfg.UpdateInterval = 5 * time.Minute
	mock.SetMockConfig(cfg)
}

// signalFirstFiring closes the returned channel when manager fires its first
// alert. Lifecycle callbacks run inside the evaluation that fired, so the
// pass that fired is past its fixture read when the channel closes.
func signalFirstFiring(t *testing.T, manager *alerts.Manager) <-chan struct{} {
	t.Helper()
	fired := make(chan struct{})
	var once sync.Once
	unsubscribe := manager.SubscribeLifecycleCallback(func(event alerts.LifecycleEvent) {
		if event.Type == eventlog.TypeFired {
			once.Do(func() { close(fired) })
		}
	})
	t.Cleanup(unsubscribe)
	return fired
}

func describeAlerts(active []alerts.Alert) string {
	keys := make([]string, 0, len(active))
	for _, alert := range active {
		keys = append(keys, alert.Type+" "+alert.ResourceID)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

// switchMockModeAsync runs SetMockMode on its own goroutine, so a test that is
// holding an evaluation parked can still bound how long the switch takes.
func switchMockModeAsync(t *testing.T, monitor *Monitor, enable bool) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := monitor.SetMockMode(enable); err != nil {
			t.Errorf("SetMockMode(%v): %v", enable, err)
		}
	}()
	return done
}

// goroutineIn reports whether one goroutine's stack holds every frame.
func goroutineIn(frames ...string) bool {
	buf := make([]byte, 4<<20)
	n := runtime.Stack(buf, true)
	for _, goroutine := range bytes.Split(buf[:n], []byte("\n\n")) {
		all := true
		for _, frame := range frames {
			all = all && bytes.Contains(goroutine, []byte(frame))
		}
		if all {
			return true
		}
	}
	return false
}

// waitForGoroutineIn waits until one goroutine's stack holds every frame. A
// test holding a lock uses it to see the goroutine it started parked on that
// lock at a known point.
func waitForGoroutineIn(t *testing.T, frames ...string) {
	t.Helper()
	waitForCondition(t, 30*time.Second, func() bool { return goroutineIn(frames...) },
		fmt.Sprintf("no goroutine reached %s", strings.Join(frames, " via ")))
}

func TestLeavingMockModeRefusesTheRestOfAnInFlightPass(t *testing.T) {
	setMockSamplerTestEnv(t, time.Hour, 5*time.Minute)
	pinDefaultMockEstate(t)
	manager := newMockHostAlertTestManager(t)
	monitor := &Monitor{
		state:          models.NewState(),
		alertManager:   manager,
		metricsHistory: NewMetricsHistory(10, time.Hour),
	}
	mustSetMonitorMockMode(t, monitor, true)

	// Park a real pass at its first storage capacity read: it has read the
	// fixture and evaluated guests, agents and nodes; storage, disks, PBS,
	// PMG and Docker are still ahead of it.
	monitor.metricsHistory.capacityForecastMu.Lock()
	parked := true
	defer func() {
		if parked {
			monitor.metricsHistory.capacityForecastMu.Unlock()
		}
	}()
	passDone := make(chan struct{})
	go func() {
		defer close(passDone)
		monitor.checkMockAlerts()
	}()
	waitForGoroutineIn(t, "monitoring.(*Monitor).storageCapacityTrendFor", "monitoring.(*Monitor).checkMockAlerts")

	waitForSignal(t, switchMockModeAsync(t, monitor, false), 30*time.Second, "leaving mock mode waited on a pass parked outside any evaluation")
	monitor.metricsHistory.capacityForecastMu.Unlock()
	parked = false
	waitForSignal(t, passDone, 30*time.Second, "mock pass did not finish after it was released")

	if active := manager.GetActiveAlerts(); len(active) != 0 {
		t.Fatalf("a mock pass in flight when mock mode ended left %d fixture alerts in live mode: %s",
			len(active), describeAlerts(active))
	}
}

func TestLeavingMockModeWaitsForAnInFlightEvaluation(t *testing.T) {
	setMockSamplerTestEnv(t, time.Hour, 5*time.Minute)
	pinDefaultMockEstate(t)
	manager := newMockHostAlertTestManager(t)
	monitor := &Monitor{
		state:          models.NewState(),
		alertManager:   manager,
		metricsHistory: NewMetricsHistory(10, time.Hour),
	}
	mustSetMonitorMockMode(t, monitor, true)

	// Park a real pass inside its first guest evaluation, in the CPU window
	// read the alert manager makes outside its own lock, so nothing but the
	// fence holds the switch back.
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	manager.SetMetricWindowProvider(func(alerts.MetricWindowRequest) ([]alerts.MetricWindowPoint, error) {
		once.Do(func() {
			close(entered)
			<-release
		})
		return nil, nil
	})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	passDone := make(chan struct{})
	go func() {
		defer close(passDone)
		monitor.checkMockAlerts()
	}()
	waitForSignal(t, entered, 30*time.Second, "mock pass never evaluated a windowed metric")

	// The switch must stop in advance until the evaluation returns. One
	// that finishes first cleared alerts the evaluation can still raise.
	leftMockMode := switchMockModeAsync(t, monitor, false)
	waitForCondition(t, 30*time.Second, func() bool {
		select {
		case <-leftMockMode:
			return true
		default:
		}
		return goroutineIn("monitoring.(*mockModeFence).advance", "monitoring.(*Monitor).SetMockMode")
	}, "leaving mock mode neither finished nor waited for the running evaluation")
	select {
	case <-leftMockMode:
		t.Fatal("leaving mock mode cleared alerts while a fixture evaluation was still running")
	default:
	}

	close(release)
	released = true
	waitForSignal(t, leftMockMode, 30*time.Second, "leaving mock mode did not finish once the evaluation returned")
	waitForSignal(t, passDone, 30*time.Second, "mock pass did not finish after it was released")
	if active := manager.GetActiveAlerts(); len(active) != 0 {
		t.Fatalf("the evaluation running when mock mode ended left %d fixture alerts in live mode: %s",
			len(active), describeAlerts(active))
	}
}

// blockingFixtureRecords serves one source's fixture records the way the
// router's mock adapter does, reading them on each call, and holds its first
// call until released. Later calls are not held, not even while the first is.
type blockingFixtureRecords struct {
	source  unifiedresources.DataSource
	entered chan struct{}
	release chan struct{}
	held    atomic.Bool
}

func newBlockingFixtureRecords(source unifiedresources.DataSource) *blockingFixtureRecords {
	return &blockingFixtureRecords{source: source, entered: make(chan struct{}), release: make(chan struct{})}
}

func (p *blockingFixtureRecords) SupplementalRecords(_ *Monitor, _ string) []unifiedresources.IngestRecord {
	records := mock.SupplementalRecords(p.source)
	if p.held.CompareAndSwap(false, true) {
		close(p.entered)
		<-p.release
	}
	return records
}

func TestLeavingMockModeDiscardsAnInFlightFrontendRefresh(t *testing.T) {
	setMockSamplerTestEnv(t, time.Hour, 5*time.Minute)
	pinDefaultMockEstate(t)
	manager := newMockHostAlertTestManager(t)
	provider := newBlockingFixtureRecords(unifiedresources.SourceVMware)
	monitor := &Monitor{
		state:          models.NewState(),
		alertManager:   manager,
		metricsHistory: NewMetricsHistory(10, time.Hour),
		resourceStore:  unifiedresources.NewMonitorAdapter(nil),
		supplementalProviders: map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{
			unifiedresources.SourceVMware: provider,
		},
	}
	mustSetMonitorMockMode(t, monitor, true)

	// Hold a frontend read in its registry rebuild: it has the fixture
	// snapshot and the vSphere fixture records, whose datastore, host and
	// network incidents its unified alert sync would raise.
	released := false
	defer func() {
		if !released {
			close(provider.release)
		}
	}()
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		monitor.BuildFrontendState()
	}()
	waitForSignal(t, provider.entered, 30*time.Second, "frontend read never reached the supplemental records")

	waitForSignal(t, switchMockModeAsync(t, monitor, false), 30*time.Second, "leaving mock mode waited on a registry rebuild")
	close(provider.release)
	released = true
	waitForSignal(t, readDone, 30*time.Second, "frontend read did not finish after it was released")

	if active := manager.GetActiveAlerts(); len(active) != 0 {
		t.Fatalf("a frontend read in flight when mock mode ended left %d fixture alerts in live mode: %s",
			len(active), describeAlerts(active))
	}
}

// gatedListStore holds the first registry listing made after open is called,
// so a test can publish another generation between a refresh's own publish
// and its read back.
type gatedListStore struct {
	*unifiedresources.MonitorAdapter
	mu      sync.Mutex
	armed   bool
	entered chan struct{}
	release chan struct{}
}

func (s *gatedListStore) arm() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed = true
}

func (s *gatedListStore) GetAll() []unifiedresources.Resource {
	s.mu.Lock()
	hold := s.armed
	s.armed = false
	s.mu.Unlock()
	if hold {
		close(s.entered)
		<-s.release
	}
	return s.MonitorAdapter.GetAll()
}

// fixedRecords stands in for a live provider: it serves the same records in
// either mode.
type fixedRecords []unifiedresources.IngestRecord

func (r fixedRecords) SupplementalRecords(_ *Monitor, _ string) []unifiedresources.IngestRecord {
	return r
}

func alertTypes(active []alerts.Alert) map[string]int {
	types := make(map[string]int)
	for _, alert := range active {
		types[alert.Type]++
	}
	return types
}

func TestLiveRefreshIgnoresARegistryRepublishedFromMockMode(t *testing.T) {
	setMockSamplerTestEnv(t, time.Hour, 5*time.Minute)
	pinDefaultMockEstate(t)
	manager := newMockHostAlertTestManager(t)
	mustSetMockEnabled(t, true)
	// TrueNAS records stand in for a live provider's data; the vSphere
	// fixture source serves nothing once mock mode is off.
	liveTrueNAS := fixedRecords(mock.SupplementalRecords(unifiedresources.SourceTrueNAS))
	mustSetMockEnabled(t, false)
	if len(liveTrueNAS) == 0 {
		t.Fatal("mock fixture has no TrueNAS records")
	}
	provider := newBlockingFixtureRecords(unifiedresources.SourceVMware)
	store := &gatedListStore{
		MonitorAdapter: unifiedresources.NewMonitorAdapter(nil),
		entered:        make(chan struct{}),
		release:        make(chan struct{}),
	}
	monitor := &Monitor{
		state:          models.NewState(),
		alertManager:   manager,
		metricsHistory: NewMetricsHistory(10, time.Hour),
		resourceStore:  store,
		supplementalProviders: map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{
			unifiedresources.SourceVMware:  provider,
			unifiedresources.SourceTrueNAS: liveTrueNAS,
		},
	}
	mustSetMonitorMockMode(t, monitor, true)
	releasedProvider, releasedList := false, false
	defer func() {
		if !releasedProvider {
			close(provider.release)
		}
		if !releasedList {
			close(store.release)
		}
	}()

	// A mock-mode refresh holds the fixture snapshot and vSphere records and
	// has not published them yet.
	staleDone := make(chan struct{})
	go func() {
		defer close(staleDone)
		monitor.updateResourceStore(monitor.currentStateWithScope())
	}()
	waitForSignal(t, provider.entered, 30*time.Second, "mock-mode refresh never reached the supplemental records")
	waitForSignal(t, switchMockModeAsync(t, monitor, false), 30*time.Second, "leaving mock mode waited on a registry rebuild")

	// A live refresh publishes the live TrueNAS records alone and is held
	// before reading the registry back.
	store.arm()
	liveDone := make(chan struct{})
	go func() {
		defer close(liveDone)
		monitor.updateResourceStore(monitor.currentStateWithScope())
	}()
	waitForSignal(t, store.entered, 30*time.Second, "live refresh never read the registry back")
	for _, resource := range store.MonitorAdapter.GetAll() {
		if slices.Contains(resource.Sources, unifiedresources.SourceVMware) {
			t.Fatalf("the live refresh published vSphere fixture resource %s", resource.ID)
		}
	}

	// The mock-mode refresh now publishes the fixture estate over it.
	close(provider.release)
	releasedProvider = true
	waitForSignal(t, staleDone, 30*time.Second, "mock-mode refresh did not finish after it was released")
	close(store.release)
	releasedList = true
	waitForSignal(t, liveDone, 30*time.Second, "live refresh did not finish after it was released")

	if active := manager.GetActiveAlerts(); len(active) != 0 {
		t.Fatalf("a live refresh evaluated a registry the mock-mode refresh republished, leaving %d fixture alerts: %s",
			len(active), describeAlerts(active))
	}
	if monitor.mockModeFence.registryCurrent() {
		t.Fatal("the registry the mock-mode refresh republished counted as current")
	}

	// The next live refresh is undisturbed: it replaces the registry and
	// evaluates the live records.
	monitor.updateResourceStore(monitor.currentStateWithScope())
	if !monitor.mockModeFence.registryCurrent() {
		t.Fatal("an undisturbed live rebuild did not make the registry current")
	}
	types := alertTypes(manager.GetActiveAlerts())
	if types["zfs-pool-state"] == 0 {
		t.Fatalf("the undisturbed live refresh raised no alert from the live TrueNAS records: %v", types)
	}
	for _, fixtureType := range []string{"resource-incident", "storage-incident"} {
		if types[fixtureType] != 0 {
			t.Fatalf("vSphere fixture incidents reached live mode: %v", types)
		}
	}
}

func TestDockerPruneIgnoresTheRegistryMockModeLeft(t *testing.T) {
	setMockSamplerTestEnv(t, time.Hour, 5*time.Minute)
	pinDefaultMockEstate(t)
	manager := newMockHostAlertTestManager(t)
	monitor := &Monitor{
		state:          models.NewState(),
		alertManager:   manager,
		metricsHistory: NewMetricsHistory(10, time.Hour),
		resourceStore:  unifiedresources.NewMonitorAdapter(nil),
	}
	mustSetMonitorMockMode(t, monitor, true)
	monitor.updateResourceStore(monitor.currentStateWithScope())
	mustSetMonitorMockMode(t, monitor, false)

	// Before any live rebuild replaces the fixture registry, a live Docker
	// host reports and then goes offline.
	host := models.DockerHost{ID: "live-docker-1", Hostname: "live-docker-1", DisplayName: "live-docker-1", Status: "offline"}
	monitor.state.UpsertDockerHost(host)
	for range 3 {
		manager.HandleDockerHostOffline(host)
	}
	if types := alertTypes(manager.GetActiveAlerts()); types["docker-host-offline"] != 1 {
		t.Fatalf("live Docker host raised no offline alert: %v", types)
	}

	monitor.SyncAlertState()
	if types := alertTypes(manager.GetActiveAlerts()); types["docker-host-offline"] != 1 {
		t.Fatalf("Docker pruning removed a live host's alert against the fixture registry: %v", types)
	}
}

func TestPreviousStateCarryIgnoresTheRegistryMockModeLeft(t *testing.T) {
	setMockSamplerTestEnv(t, time.Hour, 5*time.Minute)
	pinDefaultMockEstate(t)
	monitor := &Monitor{
		state:          models.NewState(),
		alertManager:   newMockHostAlertTestManager(t),
		metricsHistory: NewMetricsHistory(10, time.Hour),
		resourceStore:  unifiedresources.NewMonitorAdapter(nil),
	}
	mustSetMonitorMockMode(t, monitor, true)
	monitor.updateResourceStore(monitor.currentStateWithScope())
	fixture := mock.CurrentFixtureGraph().State
	if len(fixture.Nodes) == 0 || len(fixture.VMs) == 0 {
		t.Fatal("mock fixture has no nodes or VMs")
	}
	instance := fixture.VMs[0].Instance
	mustSetMonitorMockMode(t, monitor, false)

	// A live instance named like a fixture one polls before any live rebuild
	// replaces the fixture registry. Its previous-state carry, which writes
	// back into live state, must not see the fixture nodes and guests.
	if nodes := monitor.previousNodesForInstance(instance); len(nodes) != 0 {
		t.Fatalf("previous-node carry for %q returned %d fixture nodes after leaving mock mode", instance, len(nodes))
	}
	if guests := monitor.previousGuestContextForInstance(instance); len(guests.vms) != 0 || len(guests.containers) != 0 {
		t.Fatalf("previous-guest carry for %q returned %d fixture VMs and %d containers after leaving mock mode",
			instance, len(guests.vms), len(guests.containers))
	}
}

func TestLeavingMockModeDiscardsAnInFlightBackupEvaluation(t *testing.T) {
	setMockSamplerTestEnv(t, time.Hour, 5*time.Minute)
	pinDefaultMockEstate(t)
	manager := newMockHostAlertTestManager(t)
	config := manager.GetConfig()
	config.BackupDefaults.Enabled = true
	config.BackupDefaults.WarningDays = 1
	config.BackupDefaults.CriticalDays = 2
	manager.UpdateConfig(config)
	monitor := &Monitor{
		state:          models.NewState(),
		alertManager:   manager,
		metricsHistory: NewMetricsHistory(10, time.Hour),
	}
	mustSetMonitorMockMode(t, monitor, true)
	// Cache the fixture read view the guest lookup will be served from.
	monitor.GetUnifiedReadStateOrSnapshot()

	// Park a real evaluation at its guest lookup, after it read the fixture
	// recovery rollups whose stale backups raise backup-age alerts.
	monitor.mockUnifiedViewMu.Lock()
	parked := true
	defer func() {
		if parked {
			monitor.mockUnifiedViewMu.Unlock()
		}
	}()
	evalDone := make(chan struct{})
	go func() {
		defer close(evalDone)
		monitor.checkBackupAlerts(context.Background())
	}()
	waitForGoroutineIn(t, "monitoring.(*Monitor).currentUnifiedStateView", "monitoring.(*Monitor).checkBackupAlerts")

	waitForSignal(t, switchMockModeAsync(t, monitor, false), 30*time.Second, "leaving mock mode waited on a backup evaluation parked outside the alert manager")
	monitor.mockUnifiedViewMu.Unlock()
	parked = false
	waitForSignal(t, evalDone, 30*time.Second, "backup evaluation did not finish after it was released")

	if active := manager.GetActiveAlerts(); len(active) != 0 {
		t.Fatalf("a backup evaluation in flight when mock mode ended left %d fixture alerts in live mode: %.500s",
			len(active), describeAlerts(active))
	}
}

func TestLeavingMockModeDiscardsAnInFlightConnectionCheck(t *testing.T) {
	pinDefaultMockEstate(t)
	manager := newMockHostAlertTestManager(t)
	monitor := &Monitor{
		state:          models.NewState(),
		alertManager:   manager,
		metricsHistory: NewMetricsHistory(10, time.Hour),
	}
	mustSetMonitorMockMode(t, monitor, true)

	// A hybrid mock ledger lists real connections beside fixture ones, so the
	// same connection can be read degraded in mock mode and again live.
	degraded := alerts.ConnectionSnapshot{
		ID:      "pve-cluster-a",
		Name:    "cluster-a",
		Type:    alerts.ConnectionTypePVE,
		State:   alerts.ConnectionStateUnreachable,
		Enabled: true,
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	monitor.SetConnectionsSnapshotLister(func() []alerts.ConnectionSnapshot {
		once.Do(func() {
			close(entered)
			<-release
		})
		return []alerts.ConnectionSnapshot{degraded}
	})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	checkDone := make(chan struct{})
	go func() {
		defer close(checkDone)
		monitor.checkConnectionAlerts()
	}()
	waitForSignal(t, entered, 30*time.Second, "connection check never read the ledger")

	waitForSignal(t, switchMockModeAsync(t, monitor, false), 30*time.Second, "leaving mock mode waited on a connection check parked in its ledger read")
	close(release)
	released = true
	waitForSignal(t, checkDone, 30*time.Second, "connection check did not finish after it was released")

	// The connection-degraded alert confirms on its third consecutive
	// observation; the one read in mock mode must not be one of them.
	monitor.checkConnectionAlerts()
	monitor.checkConnectionAlerts()
	if active := manager.GetActiveAlerts(); len(active) != 0 {
		t.Fatalf("a connection observation read in mock mode counted toward live confirmation: %s", describeAlerts(active))
	}
	monitor.checkConnectionAlerts()
	if active := manager.GetActiveAlerts(); len(active) != 1 {
		t.Fatalf("three live degraded observations raised %d alerts, want 1: %s", len(active), describeAlerts(active))
	}
}

// diskAlertKeys lists the active disk-health and disk-wearout alerts as
// "type instance/node device" so assertions name the disk, not the alert ID.
func diskAlertKeys(manager *alerts.Manager) map[string]bool {
	keys := make(map[string]bool)
	for _, alert := range manager.GetActiveAlerts() {
		if alert.Type != "disk-health" && alert.Type != "disk-wearout" {
			continue
		}
		devPath, _ := alert.Metadata["disk_path"].(string)
		keys[alert.Type+" "+alert.Instance+"/"+alert.Node+" "+devPath] = true
	}
	return keys
}

func TestCheckMockAlertsEvaluatesPhysicalDisks(t *testing.T) {
	// Pin the default estate: a PULSE_MOCK_* environment can select one with
	// no FAILED disk (the public demo's eight nodes have none).
	previousEnabled, previousConfig := mock.IsMockEnabled(), mock.GetConfig()
	t.Cleanup(func() {
		mustSetMockEnabled(t, false)
		mock.SetMockConfig(previousConfig)
		mustSetMockEnabled(t, previousEnabled)
	})
	mustSetMockEnabled(t, false)
	cfg := mock.DefaultConfig
	cfg.UpdateInterval = 5 * time.Minute
	mock.SetMockConfig(cfg)
	mustSetMockEnabled(t, true)
	manager := newMockHostAlertTestManager(t)

	monitor := &Monitor{alertManager: manager}
	monitor.checkMockAlerts()

	// The fixture keeps a FAILED cohort and worn SSDs stable across restarts
	// so the disk alert lifecycle has evidence to act on. Each must raise the
	// alert the physical disk poller raises for a live disk, and a disk on a
	// node the poller would not reach raises nothing.
	state := mock.CurrentFixtureGraph().State
	onlineNodes := make(map[string]bool, len(state.Nodes))
	for _, node := range state.Nodes {
		onlineNodes[node.Instance+"/"+node.Name] = node.Status == "online"
	}
	want := make(map[string]bool)
	failed, worn := 0, 0
	for _, disk := range state.PhysicalDisks {
		if !onlineNodes[disk.Instance+"/"+disk.Node] {
			continue
		}
		if strings.EqualFold(disk.Health, "FAILED") {
			want["disk-health "+disk.Instance+"/"+disk.Node+" "+disk.DevPath] = true
			failed++
		}
		if disk.Wearout >= 0 && disk.Wearout < 10 {
			want["disk-wearout "+disk.Instance+"/"+disk.Node+" "+disk.DevPath] = true
			worn++
		}
	}
	if failed == 0 || worn == 0 {
		t.Fatalf("mock fixture has %d FAILED and %d worn disks on online nodes, want at least one of each", failed, worn)
	}

	got := diskAlertKeys(manager)
	for key := range got {
		if !want[key] {
			t.Errorf("mock pass raised unexpected disk alert %q", key)
		}
	}
	for key := range want {
		if !got[key] {
			t.Errorf("mock pass raised no alert %q", key)
		}
	}
}

func TestCheckMockPhysicalDiskAlertsFollowsPollerBoundary(t *testing.T) {
	manager := newMockHostAlertTestManager(t)
	monitor := &Monitor{alertManager: manager}

	nodes := []models.Node{
		{ID: "west-pve1", Name: "pve1", Instance: "west", Status: "online", LinkedAgentID: "host-west-pve1"},
		{ID: "east-pve1", Name: "pve1", Instance: "east", Status: "offline"},
	}
	hosts := []models.Host{{ID: "host-west-pve1", Hostname: "pve1", Status: "online"}}
	disks := []models.PhysicalDisk{
		{Instance: "west", Node: "pve1", DevPath: "/dev/sda", Model: "Crucial MX500 2TB", Type: "sata", Health: "FAILED", Wearout: -1},
		{Instance: "west", Node: "pve1", DevPath: "/dev/nvme0n1", Model: "Samsung 980 PRO 2TB", Type: "nvme", Health: "PASSED", Wearout: 6},
		// Same node name on another instance: that node is offline, so the
		// poller would not reach it and its disk must not alert.
		{Instance: "east", Node: "pve1", DevPath: "/dev/sdb", Model: "WD Red Pro 8TB", Type: "sata", Health: "FAILED", Wearout: -1},
	}

	monitor.checkMockPhysicalDiskAlerts(monitor.mockModeFence.begin(), disks, nodes, hosts)
	got := diskAlertKeys(manager)
	if len(got) != 2 || !got["disk-health west/pve1 /dev/sda"] || !got["disk-wearout west/pve1 /dev/nvme0n1"] {
		t.Fatalf("disk alerts = %v, want only FAILED health on west/pve1 /dev/sda and wearout on west/pve1 /dev/nvme0n1", got)
	}

	// The linked agent's --disk-exclude forces matched devices healthy, so
	// excluding a disk resolves the alerts it already raised. Endurance
	// recovery needs three healthy-looking passes.
	hosts[0].DiskExclude = []string{"/dev/sda", "nvme0n1"}
	for i := 0; i < 3; i++ {
		monitor.checkMockPhysicalDiskAlerts(monitor.mockModeFence.begin(), disks, nodes, hosts)
	}
	if got := diskAlertKeys(manager); len(got) != 0 {
		t.Fatalf("excluded devices kept disk alerts %v", got)
	}

	// A node that leaves the estate takes its disk alerts with it through the
	// node cleanup the mock pass runs first.
	hosts[0].DiskExclude = nil
	monitor.checkMockPhysicalDiskAlerts(monitor.mockModeFence.begin(), disks, nodes, hosts)
	if got := diskAlertKeys(manager); len(got) != 2 {
		t.Fatalf("disk alerts after removing the exclusion = %v, want both west/pve1 alerts back", got)
	}
	manager.CleanupAlertsForNodes(map[string]bool{"pve2": true})
	if got := diskAlertKeys(manager); len(got) != 0 {
		t.Fatalf("disk alerts outlived their departed node: %v", got)
	}
}
