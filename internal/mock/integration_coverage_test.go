package mock

import (
	"os"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

var mockEnvKeys = []string{
	"PULSE_MOCK_MODE",
	"PULSE_MOCK_NODES",
	"PULSE_MOCK_VMS_PER_NODE",
	"PULSE_MOCK_LXCS_PER_NODE",
	"PULSE_MOCK_DOCKER_HOSTS",
	"PULSE_MOCK_DOCKER_CONTAINERS",
	"PULSE_MOCK_GENERIC_HOSTS",
	"PULSE_MOCK_K8S_CLUSTERS",
	"PULSE_MOCK_K8S_NODES",
	"PULSE_MOCK_K8S_PODS",
	"PULSE_MOCK_K8S_DEPLOYMENTS",
	"PULSE_MOCK_RANDOM_METRICS",
	"PULSE_MOCK_STOPPED_PERCENT",
}

func resetMockIntegrationState(t *testing.T) {
	t.Helper()

	stopUpdateLoopLocked()
	dataMu.Lock()
	mockData = models.StateSnapshot{}
	mockAlerts = nil
	mockConfig = DefaultConfig
	dataMu.Unlock()
	enabled.Store(false)

	for _, key := range mockEnvKeys {
		_ = os.Unsetenv(key)
	}

	t.Cleanup(func() {
		stopUpdateLoopLocked()
		dataMu.Lock()
		mockData = models.StateSnapshot{}
		mockAlerts = nil
		mockConfig = DefaultConfig
		dataMu.Unlock()
		enabled.Store(false)
		for _, key := range mockEnvKeys {
			_ = os.Unsetenv(key)
		}
	})
}

func TestLoadMockConfigAppliesValidEnvironmentOverrides(t *testing.T) {
	resetMockIntegrationState(t)

	t.Setenv("PULSE_MOCK_NODES", "12")
	t.Setenv("PULSE_MOCK_VMS_PER_NODE", "3")
	t.Setenv("PULSE_MOCK_LXCS_PER_NODE", "4")
	t.Setenv("PULSE_MOCK_DOCKER_HOSTS", "5")
	t.Setenv("PULSE_MOCK_DOCKER_CONTAINERS", "6")
	t.Setenv("PULSE_MOCK_GENERIC_HOSTS", "7")
	t.Setenv("PULSE_MOCK_K8S_CLUSTERS", "2")
	t.Setenv("PULSE_MOCK_K8S_NODES", "8")
	t.Setenv("PULSE_MOCK_K8S_PODS", "30")
	t.Setenv("PULSE_MOCK_K8S_DEPLOYMENTS", "9")
	t.Setenv("PULSE_MOCK_RANDOM_METRICS", "false")
	t.Setenv("PULSE_MOCK_STOPPED_PERCENT", "35")

	cfg := LoadMockConfig()

	if cfg.NodeCount != 12 {
		t.Fatalf("expected node count override, got %d", cfg.NodeCount)
	}
	if cfg.VMsPerNode != 3 {
		t.Fatalf("expected vm count override, got %d", cfg.VMsPerNode)
	}
	if cfg.LXCsPerNode != 4 {
		t.Fatalf("expected lxc count override, got %d", cfg.LXCsPerNode)
	}
	if cfg.DockerHostCount != 5 {
		t.Fatalf("expected docker host override, got %d", cfg.DockerHostCount)
	}
	if cfg.DockerContainersPerHost != 6 {
		t.Fatalf("expected docker container override, got %d", cfg.DockerContainersPerHost)
	}
	if cfg.GenericHostCount != 7 {
		t.Fatalf("expected generic host override, got %d", cfg.GenericHostCount)
	}
	if cfg.K8sClusterCount != 2 {
		t.Fatalf("expected k8s cluster override, got %d", cfg.K8sClusterCount)
	}
	if cfg.K8sNodesPerCluster != 8 {
		t.Fatalf("expected k8s node override, got %d", cfg.K8sNodesPerCluster)
	}
	if cfg.K8sPodsPerCluster != 30 {
		t.Fatalf("expected k8s pod override, got %d", cfg.K8sPodsPerCluster)
	}
	if cfg.K8sDeploymentsPerCluster != 9 {
		t.Fatalf("expected k8s deployment override, got %d", cfg.K8sDeploymentsPerCluster)
	}
	if cfg.RandomMetrics {
		t.Fatalf("expected random metrics to be false")
	}
	if cfg.StoppedPercent != 0.35 {
		t.Fatalf("expected stopped percent 0.35, got %f", cfg.StoppedPercent)
	}
}

func TestLoadMockConfigIgnoresInvalidOrOutOfRangeValues(t *testing.T) {
	resetMockIntegrationState(t)

	t.Setenv("PULSE_MOCK_NODES", "0")
	t.Setenv("PULSE_MOCK_VMS_PER_NODE", "-1")
	t.Setenv("PULSE_MOCK_LXCS_PER_NODE", "invalid")
	t.Setenv("PULSE_MOCK_DOCKER_HOSTS", "-2")
	t.Setenv("PULSE_MOCK_DOCKER_CONTAINERS", "bad")
	t.Setenv("PULSE_MOCK_GENERIC_HOSTS", "-3")
	t.Setenv("PULSE_MOCK_K8S_CLUSTERS", "bad")
	t.Setenv("PULSE_MOCK_K8S_NODES", "-4")
	t.Setenv("PULSE_MOCK_K8S_PODS", "bad")
	t.Setenv("PULSE_MOCK_K8S_DEPLOYMENTS", "-5")
	t.Setenv("PULSE_MOCK_STOPPED_PERCENT", "not-a-number")

	cfg := LoadMockConfig()

	if cfg.NodeCount != DefaultConfig.NodeCount {
		t.Fatalf("expected default node count, got %d", cfg.NodeCount)
	}
	if cfg.VMsPerNode != DefaultConfig.VMsPerNode {
		t.Fatalf("expected default vm count, got %d", cfg.VMsPerNode)
	}
	if cfg.LXCsPerNode != DefaultConfig.LXCsPerNode {
		t.Fatalf("expected default lxc count, got %d", cfg.LXCsPerNode)
	}
	if cfg.DockerHostCount != DefaultConfig.DockerHostCount {
		t.Fatalf("expected default docker host count, got %d", cfg.DockerHostCount)
	}
	if cfg.DockerContainersPerHost != DefaultConfig.DockerContainersPerHost {
		t.Fatalf("expected default docker container count, got %d", cfg.DockerContainersPerHost)
	}
	if cfg.GenericHostCount != DefaultConfig.GenericHostCount {
		t.Fatalf("expected default generic host count, got %d", cfg.GenericHostCount)
	}
	if cfg.K8sClusterCount != DefaultConfig.K8sClusterCount {
		t.Fatalf("expected default k8s cluster count, got %d", cfg.K8sClusterCount)
	}
	if cfg.K8sNodesPerCluster != DefaultConfig.K8sNodesPerCluster {
		t.Fatalf("expected default k8s nodes count, got %d", cfg.K8sNodesPerCluster)
	}
	if cfg.K8sPodsPerCluster != DefaultConfig.K8sPodsPerCluster {
		t.Fatalf("expected default k8s pods count, got %d", cfg.K8sPodsPerCluster)
	}
	if cfg.K8sDeploymentsPerCluster != DefaultConfig.K8sDeploymentsPerCluster {
		t.Fatalf("expected default k8s deployments count, got %d", cfg.K8sDeploymentsPerCluster)
	}
	if cfg.StoppedPercent != DefaultConfig.StoppedPercent {
		t.Fatalf("expected default stopped percent, got %f", cfg.StoppedPercent)
	}
}

func TestSetEnabledNoOpBehaviorForInitAndRuntime(t *testing.T) {
	resetMockIntegrationState(t)

	if err := os.Setenv("PULSE_MOCK_MODE", "preserve"); err != nil {
		t.Fatalf("failed to seed env: %v", err)
	}

	setEnabled(false, true)
	if got := os.Getenv("PULSE_MOCK_MODE"); got != "preserve" {
		t.Fatalf("expected init no-op to preserve env, got %q", got)
	}

	setEnabled(false, false)
	if got := os.Getenv("PULSE_MOCK_MODE"); got != "false" {
		t.Fatalf("expected runtime no-op to write false env, got %q", got)
	}
}

func TestSetMockConfigOnlyRegeneratesWhenEnabled(t *testing.T) {
	resetMockIntegrationState(t)

	dataMu.Lock()
	mockData = models.StateSnapshot{Nodes: []models.Node{{ID: "node-existing", Name: "existing"}}}
	dataMu.Unlock()

	cfg := DefaultConfig
	cfg.NodeCount = 2
	cfg.VMsPerNode = 1
	cfg.LXCsPerNode = 2

	enabled.Store(false)
	SetMockConfig(cfg)

	if got := GetConfig(); got.NodeCount != 2 || got.VMsPerNode != 1 || got.LXCsPerNode != 2 {
		t.Fatalf("config not updated while disabled: %+v", got)
	}

	dataMu.RLock()
	nodesWhenDisabled := len(mockData.Nodes)
	dataMu.RUnlock()
	if nodesWhenDisabled != 1 {
		t.Fatalf("expected data unchanged while disabled, got %d nodes", nodesWhenDisabled)
	}

	enabled.Store(true)
	cfg.NodeCount = 3
	SetMockConfig(cfg)

	dataMu.RLock()
	defer dataMu.RUnlock()

	if len(mockData.Nodes) != 3 {
		t.Fatalf("expected regenerated node count, got %d", len(mockData.Nodes))
	}
	if mockData.LastUpdate.IsZero() {
		t.Fatalf("expected regenerated state to update last-update timestamp")
	}
	if len(mockAlerts) == 0 {
		t.Fatalf("expected regenerated alert history when enabled")
	}
}

func TestUpdateMetricsGuardAndTimestamp(t *testing.T) {
	resetMockIntegrationState(t)

	cfg := DefaultConfig
	cfg.NodeCount = 1
	cfg.VMsPerNode = 0
	cfg.LXCsPerNode = 0
	cfg.DockerHostCount = 0
	cfg.GenericHostCount = 0
	cfg.K8sClusterCount = 0
	cfg.RandomMetrics = false

	dataMu.Lock()
	mockData = GenerateMockData(cfg)
	mockData.LastUpdate = time.Time{}
	dataMu.Unlock()

	enabled.Store(false)
	updateMetrics(cfg)

	dataMu.RLock()
	lastWhenDisabled := mockData.LastUpdate
	dataMu.RUnlock()
	if !lastWhenDisabled.IsZero() {
		t.Fatalf("expected no update while disabled")
	}

	enabled.Store(true)
	updateMetrics(cfg)

	dataMu.RLock()
	lastWhenEnabled := mockData.LastUpdate
	dataMu.RUnlock()
	if lastWhenEnabled.IsZero() {
		t.Fatalf("expected update timestamp while enabled")
	}
}

func TestUpdateLoopStartStopLifecycle(t *testing.T) {
	resetMockIntegrationState(t)

	startUpdateLoopLocked()
	firstTicker := updateTicker
	startUpdateLoopLocked()
	secondTicker := updateTicker
	stopUpdateLoopLocked()
	finalTicker := updateTicker
	finalStopCh := stopUpdatesCh

	if firstTicker == nil {
		t.Fatalf("expected ticker after first start")
	}
	if secondTicker == nil {
		t.Fatalf("expected ticker after restart")
	}
	if firstTicker == secondTicker {
		t.Fatalf("expected restart to replace ticker instance")
	}
	if finalTicker != nil || finalStopCh != nil {
		t.Fatalf("expected update loop globals to be cleared after stop")
	}
}

func TestUpdateAlertSnapshotsAndHistoryAccessors(t *testing.T) {
	resetMockIntegrationState(t)

	start := time.Now().Add(-2 * time.Minute).Truncate(time.Second)
	resolvedTime := time.Now().Add(-1 * time.Minute).Truncate(time.Second)
	active := []alerts.Alert{
		{
			ID:           "a-1",
			Type:         "threshold",
			Level:        alerts.AlertLevelCritical,
			ResourceID:   "vm-100",
			ResourceName: "VM 100",
			Node:         "node-a",
			Instance:     "pve-a",
			Message:      "CPU critical",
			Value:        97,
			Threshold:    90,
			StartTime:    start,
			Acknowledged: true,
		},
	}
	resolved := []models.ResolvedAlert{
		{
			Alert: models.Alert{ID: "r-1", Message: "Recovered"},
			ResolvedTime: resolvedTime,
		},
	}

	UpdateAlertSnapshots(active, resolved)

	dataMu.RLock()
	if len(mockData.ActiveAlerts) != 1 {
		dataMu.RUnlock()
		t.Fatalf("expected one active alert snapshot, got %d", len(mockData.ActiveAlerts))
	}
	snapshot := mockData.ActiveAlerts[0]
	if snapshot.Level != "critical" {
		dataMu.RUnlock()
		t.Fatalf("expected level conversion to string, got %q", snapshot.Level)
	}
	if snapshot.ResourceID != "vm-100" || snapshot.Instance != "pve-a" || !snapshot.Acknowledged {
		dataMu.RUnlock()
		t.Fatalf("unexpected converted active alert snapshot: %+v", snapshot)
	}
	if len(mockData.RecentlyResolved) != 1 || mockData.RecentlyResolved[0].ID != "r-1" {
		dataMu.RUnlock()
		t.Fatalf("unexpected resolved snapshot data: %+v", mockData.RecentlyResolved)
	}
	dataMu.RUnlock()

	active[0].Message = "mutated"
	resolved[0].ID = "mutated"

	dataMu.RLock()
	if mockData.ActiveAlerts[0].Message != "CPU critical" {
		dataMu.RUnlock()
		t.Fatalf("expected active alert snapshot to be independent from source slice")
	}
	if mockData.RecentlyResolved[0].ID != "r-1" {
		dataMu.RUnlock()
		t.Fatalf("expected resolved alert snapshot to be copied")
	}
	dataMu.RUnlock()

	dataMu.Lock()
	mockAlerts = []models.Alert{{ID: "h-1"}, {ID: "h-2"}, {ID: "h-3"}}
	dataMu.Unlock()

	enabled.Store(false)
	if got := GetMockAlertHistory(2); len(got) != 0 {
		t.Fatalf("expected disabled history call to return empty slice, got %d", len(got))
	}

	enabled.Store(true)
	if got := GetMockAlertHistory(2); len(got) != 2 || got[0].ID != "h-1" || got[1].ID != "h-2" {
		t.Fatalf("unexpected limited history result: %+v", got)
	}
	all := GetMockAlertHistory(0)
	if len(all) != 3 {
		t.Fatalf("expected full history result, got %d", len(all))
	}
	all[0].ID = "changed"

	dataMu.RLock()
	defer dataMu.RUnlock()
	if mockAlerts[0].ID != "h-1" {
		t.Fatalf("expected history accessor to return defensive copies")
	}
}
