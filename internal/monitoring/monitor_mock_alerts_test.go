package monitoring

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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
	// memory alert to the agent on the first tick, as hostname
	// deduplication does for live agents.
	linkedNodes := 0
	for _, node := range state.Nodes {
		host, ok := onlineHosts[strings.TrimSpace(node.LinkedAgentID)]
		if !ok || !strings.EqualFold(host.Hostname, node.Name) {
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

func TestLeavingMockModeReleasesFixtureAgentHostnames(t *testing.T) {
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

	registeredPVE1 := false
	for _, host := range mock.CurrentFixtureGraph().State.Hosts {
		if host.Hostname == "pve1" && !strings.EqualFold(host.Status, "offline") {
			registeredPVE1 = true
		}
	}
	if !registeredPVE1 {
		t.Fatal("mock fixture has no online agent named pve1")
	}
	mustSetMonitorMockMode(t, monitor, false)

	// A real node named like a fixture agent, with no agent of its own,
	// must own its metric alerts once the monitor shows live data again.
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
	t.Fatal("real node pve1 raised no memory alert: the fixture agent's hostname deduplication outlived mock mode")
}

func TestMockHostAgentPassAfterLeavingMockModeRegistersNothing(t *testing.T) {
	manager := newMockHostAlertTestManager(t)
	monitor := &Monitor{alertManager: manager}
	mustSetMockEnabled(t, false)

	// A pass whose snapshot predates leaving mock mode reaches the agent step
	// after forgetMockHostAgents ran.
	stale := models.Host{
		ID:       "host-node-mock-cluster-1-pve1",
		Hostname: "pve1",
		Status:   "online",
		Memory:   models.Memory{Total: 32 << 30, Used: 30 << 30, Free: 2 << 30, Usage: 93.75},
	}
	monitor.evaluateMockHostAgents([]models.Host{stale}, nil)

	if active := manager.GetActiveAlerts(); len(active) != 0 {
		t.Fatalf("stale mock pass opened %d alerts in live mode", len(active))
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
	t.Fatal("stale mock pass registered fixture hostname pve1 and suppressed the real node's memory alert")
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

	monitor.checkMockPhysicalDiskAlerts(disks, nodes, hosts)
	got := diskAlertKeys(manager)
	if len(got) != 2 || !got["disk-health west/pve1 /dev/sda"] || !got["disk-wearout west/pve1 /dev/nvme0n1"] {
		t.Fatalf("disk alerts = %v, want only FAILED health on west/pve1 /dev/sda and wearout on west/pve1 /dev/nvme0n1", got)
	}

	// The linked agent's --disk-exclude forces matched devices healthy, so
	// excluding a disk resolves the alerts it already raised. Endurance
	// recovery needs three healthy-looking passes.
	hosts[0].DiskExclude = []string{"/dev/sda", "nvme0n1"}
	for i := 0; i < 3; i++ {
		monitor.checkMockPhysicalDiskAlerts(disks, nodes, hosts)
	}
	if got := diskAlertKeys(manager); len(got) != 0 {
		t.Fatalf("excluded devices kept disk alerts %v", got)
	}

	// A node that leaves the estate takes its disk alerts with it through the
	// node cleanup the mock pass runs first.
	hosts[0].DiskExclude = nil
	monitor.checkMockPhysicalDiskAlerts(disks, nodes, hosts)
	if got := diskAlertKeys(manager); len(got) != 2 {
		t.Fatalf("disk alerts after removing the exclusion = %v, want both west/pve1 alerts back", got)
	}
	manager.CleanupAlertsForNodes(map[string]bool{"pve2": true})
	if got := diskAlertKeys(manager); len(got) != 0 {
		t.Fatalf("disk alerts outlived their departed node: %v", got)
	}
}
