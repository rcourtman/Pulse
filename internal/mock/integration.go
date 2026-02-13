package mock

import (
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

var (
	dataMu        sync.RWMutex
	modeMu        sync.Mutex
	updateLoopMu  sync.Mutex
	mockData      models.StateSnapshot
	mockAlerts    []models.Alert
	mockConfig    = DefaultConfig
	enabled       atomic.Bool
	updateTicker  *time.Ticker
	stopUpdatesCh chan struct{}
	updateLoopWg  sync.WaitGroup
)

const updateInterval = 2 * time.Second

func init() {
	initialEnabled := os.Getenv("PULSE_MOCK_MODE") == "true"
	if initialEnabled {
		log.Info().Msg("mock mode enabled at startup")
	}
	setEnabled(initialEnabled, true)
}

// IsMockEnabled returns whether mock mode is enabled.
func IsMockEnabled() bool {
	return enabled.Load()
}

// SetEnabled enables or disables mock mode.
func SetEnabled(enable bool) {
	setEnabled(enable, false)
}

func setEnabled(enable bool, fromInit bool) {
	setEnabledMu.Lock()
	defer setEnabledMu.Unlock()

	current := enabled.Load()
	if current == enable {
		// Still update env so other processes see the latest value when not invoked from init.
		if !fromInit {
			setEnvFlag(enable)
		}
		return
	}

	if enable {
		enableMockMode(fromInit)
	} else {
		disableMockMode()
	}

	if !fromInit {
		setEnvFlag(enable)
	}
}

func setEnvFlag(enable bool) {
	value := "false"
	if enable {
		value = "true"
	}

	if err := os.Setenv("PULSE_MOCK_MODE", value); err != nil {
		log.Warn().
			Err(err).
			Str("env_var", "PULSE_MOCK_MODE").
			Str("value", value).
			Msg("Failed to synchronize mock mode environment flag")
	}
}

func readMockEnv(name string) (string, bool) {
	raw, ok := os.LookupEnv(name)
	if !ok {
		return "", false
	}

	value := strings.TrimSpace(raw)
	if value == "" {
		log.Warn().
			Str("env", name).
			Msg("Ignoring empty mock configuration value")
		return "", false
	}

	return value, true
}

func enableMockMode(fromInit bool) {
	config := LoadMockConfig()

	dataMu.Lock()
	mockConfig = config
	mockData = GenerateMockData(config)
	mockAlerts = GenerateAlertHistory(mockData.Nodes, mockData.VMs, mockData.Containers)
	mockData.LastUpdate = time.Now()
	enabled.Store(true)
	dataMu.Unlock()
	startUpdateLoop()

	log.Info().
		Int("nodes", config.NodeCount).
		Int("vms_per_node", config.VMsPerNode).
		Int("lxcs_per_node", config.LXCsPerNode).
		Int("host_agents", config.GenericHostCount).
		Int("docker_hosts", config.DockerHostCount).
		Int("docker_containers_per_host", config.DockerContainersPerHost).
		Int("k8s_clusters", config.K8sClusterCount).
		Int("k8s_nodes_per_cluster", config.K8sNodesPerCluster).
		Int("k8s_pods_per_cluster", config.K8sPodsPerCluster).
		Int("k8s_deployments_per_cluster", config.K8sDeploymentsPerCluster).
		Bool("random_metrics", config.RandomMetrics).
		Float64("stopped_percent", config.StoppedPercent).
		Msg("mock mode enabled")

	if !fromInit {
		log.Info().Msg("mock data generator started")
	}
}

func disableMockMode() {
	if !enabled.Load() {
		return
	}
	enabled.Store(false)
	stopUpdateLoopSignalLocked()
	dataMu.Unlock()

	waitForUpdateLoopStop()

	dataMu.Lock()
	mockData = models.StateSnapshot{}
	mockAlerts = nil
	dataMu.Unlock()

	log.Info().Msg("mock mode disabled")
}

func startUpdateLoop() {
	updateLoopMu.Lock()
	defer updateLoopMu.Unlock()

	stopUpdateLoopLocked()
	stopCh := make(chan struct{})
	ticker := time.NewTicker(updateInterval)
	stopUpdatesCh = stopCh
	updateTicker = ticker

	updateLoopWg.Add(1)
	go func(stop <-chan struct{}, tick *time.Ticker) {
		defer updateLoopWg.Done()
		for {
			select {
			case <-tick.C:
				cfg := GetConfig()
				updateMetrics(cfg)
			case <-stop:
				return
			}
		}
	}(stopCh, ticker)
}

func stopUpdateLoop() {
	updateLoopMu.Lock()
	defer updateLoopMu.Unlock()
	stopUpdateLoopLocked()
}

func stopUpdateLoopLocked() {
	stopUpdateLoopSignalLocked()
	waitForUpdateLoopStop()
}

func stopUpdateLoopSignalLocked() {
	if ch := stopUpdatesCh; ch != nil {
		close(ch)
		stopUpdatesCh = nil
	}
	if ticker := updateTicker; ticker != nil {
		ticker.Stop()
		updateTicker = nil
	}
}

func waitForUpdateLoopStop() {
	updateLoopWg.Wait()
}

func updateMetrics(cfg MockConfig) {
	if !IsMockEnabled() {
		return
	}

	dataMu.Lock()
	defer dataMu.Unlock()

	UpdateMetrics(&mockData, cfg)
	mockData.LastUpdate = time.Now()
}

// GetConfig returns the current mock configuration.
func GetConfig() MockConfig {
	dataMu.RLock()
	defer dataMu.RUnlock()
	return mockConfig
}

// LoadMockConfig loads mock configuration from environment variables.
func LoadMockConfig() MockConfig {
	config := DefaultConfig

	if raw, ok := readMockEnv("PULSE_MOCK_NODES"); ok {
		n, err := strconv.Atoi(raw)
		if err != nil {
			log.Warn().Err(err).Str("env", "PULSE_MOCK_NODES").Str("value", raw).Msg("Invalid mock config value, using default")
		} else if n <= 0 {
			log.Warn().Str("env", "PULSE_MOCK_NODES").Str("value", raw).Msg("Invalid mock config value, expected integer > 0")
		} else {
			config.NodeCount = n
		}
	}

	if raw, ok := readMockEnv("PULSE_MOCK_VMS_PER_NODE"); ok {
		n, err := strconv.Atoi(raw)
		if err != nil {
			log.Warn().Err(err).Str("env", "PULSE_MOCK_VMS_PER_NODE").Str("value", raw).Msg("Invalid mock config value, using default")
		} else if n < 0 {
			log.Warn().Str("env", "PULSE_MOCK_VMS_PER_NODE").Str("value", raw).Msg("Invalid mock config value, expected integer >= 0")
		} else {
			config.VMsPerNode = n
		}
	}

	if raw, ok := readMockEnv("PULSE_MOCK_LXCS_PER_NODE"); ok {
		n, err := strconv.Atoi(raw)
		if err != nil {
			log.Warn().Err(err).Str("env", "PULSE_MOCK_LXCS_PER_NODE").Str("value", raw).Msg("Invalid mock config value, using default")
		} else if n < 0 {
			log.Warn().Str("env", "PULSE_MOCK_LXCS_PER_NODE").Str("value", raw).Msg("Invalid mock config value, expected integer >= 0")
		} else {
			config.LXCsPerNode = n
		}
	}

	if raw, ok := readMockEnv("PULSE_MOCK_DOCKER_HOSTS"); ok {
		n, err := strconv.Atoi(raw)
		if err != nil {
			log.Warn().Err(err).Str("env", "PULSE_MOCK_DOCKER_HOSTS").Str("value", raw).Msg("Invalid mock config value, using default")
		} else if n < 0 {
			log.Warn().Str("env", "PULSE_MOCK_DOCKER_HOSTS").Str("value", raw).Msg("Invalid mock config value, expected integer >= 0")
		} else {
			config.DockerHostCount = n
		}
	}

	if raw, ok := readMockEnv("PULSE_MOCK_DOCKER_CONTAINERS"); ok {
		n, err := strconv.Atoi(raw)
		if err != nil {
			log.Warn().Err(err).Str("env", "PULSE_MOCK_DOCKER_CONTAINERS").Str("value", raw).Msg("Invalid mock config value, using default")
		} else if n < 0 {
			log.Warn().Str("env", "PULSE_MOCK_DOCKER_CONTAINERS").Str("value", raw).Msg("Invalid mock config value, expected integer >= 0")
		} else {
			config.DockerContainersPerHost = n
		}
	}

	if raw, ok := readMockEnv("PULSE_MOCK_GENERIC_HOSTS"); ok {
		n, err := strconv.Atoi(raw)
		if err != nil {
			log.Warn().Err(err).Str("env", "PULSE_MOCK_GENERIC_HOSTS").Str("value", raw).Msg("Invalid mock config value, using default")
		} else if n < 0 {
			log.Warn().Str("env", "PULSE_MOCK_GENERIC_HOSTS").Str("value", raw).Msg("Invalid mock config value, expected integer >= 0")
		} else {
			config.GenericHostCount = n
		}
	}

	if raw, ok := readMockEnv("PULSE_MOCK_K8S_CLUSTERS"); ok {
		n, err := strconv.Atoi(raw)
		if err != nil {
			log.Warn().Err(err).Str("env", "PULSE_MOCK_K8S_CLUSTERS").Str("value", raw).Msg("Invalid mock config value, using default")
		} else if n < 0 {
			log.Warn().Str("env", "PULSE_MOCK_K8S_CLUSTERS").Str("value", raw).Msg("Invalid mock config value, expected integer >= 0")
		} else {
			config.K8sClusterCount = n
		}
	}

	if raw, ok := readMockEnv("PULSE_MOCK_K8S_NODES"); ok {
		n, err := strconv.Atoi(raw)
		if err != nil {
			log.Warn().Err(err).Str("env", "PULSE_MOCK_K8S_NODES").Str("value", raw).Msg("Invalid mock config value, using default")
		} else if n < 0 {
			log.Warn().Str("env", "PULSE_MOCK_K8S_NODES").Str("value", raw).Msg("Invalid mock config value, expected integer >= 0")
		} else {
			config.K8sNodesPerCluster = n
		}
	}

	if raw, ok := readMockEnv("PULSE_MOCK_K8S_PODS"); ok {
		n, err := strconv.Atoi(raw)
		if err != nil {
			log.Warn().Err(err).Str("env", "PULSE_MOCK_K8S_PODS").Str("value", raw).Msg("Invalid mock config value, using default")
		} else if n < 0 {
			log.Warn().Str("env", "PULSE_MOCK_K8S_PODS").Str("value", raw).Msg("Invalid mock config value, expected integer >= 0")
		} else {
			config.K8sPodsPerCluster = n
		}
	}

	if raw, ok := readMockEnv("PULSE_MOCK_K8S_DEPLOYMENTS"); ok {
		n, err := strconv.Atoi(raw)
		if err != nil {
			log.Warn().Err(err).Str("env", "PULSE_MOCK_K8S_DEPLOYMENTS").Str("value", raw).Msg("Invalid mock config value, using default")
		} else if n < 0 {
			log.Warn().Str("env", "PULSE_MOCK_K8S_DEPLOYMENTS").Str("value", raw).Msg("Invalid mock config value, expected integer >= 0")
		} else {
			config.K8sDeploymentsPerCluster = n
		}
	}

	if raw, ok := readMockEnv("PULSE_MOCK_RANDOM_METRICS"); ok {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			log.Warn().Err(err).Str("env", "PULSE_MOCK_RANDOM_METRICS").Str("value", raw).Msg("Invalid mock config value, using default")
		} else {
			config.RandomMetrics = enabled
		}
	}

	if raw, ok := readMockEnv("PULSE_MOCK_STOPPED_PERCENT"); ok {
		percent, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			log.Warn().Err(err).Str("env", "PULSE_MOCK_STOPPED_PERCENT").Str("value", raw).Msg("Invalid mock config value, using default")
		} else if math.IsNaN(percent) || math.IsInf(percent, 0) || percent < 0 || percent > 100 {
			log.Warn().Str("env", "PULSE_MOCK_STOPPED_PERCENT").Str("value", raw).Msg("Invalid mock config value, expected percentage in range 0..100")
		} else {
			config.StoppedPercent = percent / 100.0
		}
	}

	return normalizeMockConfig(config)
}

// SetMockConfig updates the mock configuration dynamically and regenerates data when enabled.
func SetMockConfig(cfg MockConfig) {
	normalized := normalizeMockConfig(cfg)
	if normalized.NodeCount != cfg.NodeCount {
		log.Warn().Int("provided", cfg.NodeCount).Int("applied", normalized.NodeCount).Msg("Normalized invalid mock NodeCount")
	}
	if normalized.VMsPerNode != cfg.VMsPerNode {
		log.Warn().Int("provided", cfg.VMsPerNode).Int("applied", normalized.VMsPerNode).Msg("Normalized invalid mock VMsPerNode")
	}
	if normalized.LXCsPerNode != cfg.LXCsPerNode {
		log.Warn().Int("provided", cfg.LXCsPerNode).Int("applied", normalized.LXCsPerNode).Msg("Normalized invalid mock LXCsPerNode")
	}
	if normalized.DockerHostCount != cfg.DockerHostCount {
		log.Warn().Int("provided", cfg.DockerHostCount).Int("applied", normalized.DockerHostCount).Msg("Normalized invalid mock DockerHostCount")
	}
	if normalized.DockerContainersPerHost != cfg.DockerContainersPerHost {
		log.Warn().Int("provided", cfg.DockerContainersPerHost).Int("applied", normalized.DockerContainersPerHost).Msg("Normalized invalid mock DockerContainersPerHost")
	}
	if normalized.GenericHostCount != cfg.GenericHostCount {
		log.Warn().Int("provided", cfg.GenericHostCount).Int("applied", normalized.GenericHostCount).Msg("Normalized invalid mock GenericHostCount")
	}
	if normalized.K8sClusterCount != cfg.K8sClusterCount {
		log.Warn().Int("provided", cfg.K8sClusterCount).Int("applied", normalized.K8sClusterCount).Msg("Normalized invalid mock K8sClusterCount")
	}
	if normalized.K8sNodesPerCluster != cfg.K8sNodesPerCluster {
		log.Warn().Int("provided", cfg.K8sNodesPerCluster).Int("applied", normalized.K8sNodesPerCluster).Msg("Normalized invalid mock K8sNodesPerCluster")
	}
	if normalized.K8sPodsPerCluster != cfg.K8sPodsPerCluster {
		log.Warn().Int("provided", cfg.K8sPodsPerCluster).Int("applied", normalized.K8sPodsPerCluster).Msg("Normalized invalid mock K8sPodsPerCluster")
	}
	if normalized.K8sDeploymentsPerCluster != cfg.K8sDeploymentsPerCluster {
		log.Warn().Int("provided", cfg.K8sDeploymentsPerCluster).Int("applied", normalized.K8sDeploymentsPerCluster).Msg("Normalized invalid mock K8sDeploymentsPerCluster")
	}
	if normalized.StoppedPercent != cfg.StoppedPercent {
		log.Warn().Float64("provided", cfg.StoppedPercent).Float64("applied", normalized.StoppedPercent).Msg("Normalized invalid mock StoppedPercent")
	}

	dataMu.Lock()
	mockConfig = normalized
	if enabled.Load() {
		mockData = GenerateMockData(normalized)
		mockAlerts = GenerateAlertHistory(mockData.Nodes, mockData.VMs, mockData.Containers)
		mockData.LastUpdate = time.Now()
	}
	dataMu.Unlock()

	log.Info().
		Int("nodes", normalized.NodeCount).
		Int("vms_per_node", normalized.VMsPerNode).
		Int("lxcs_per_node", normalized.LXCsPerNode).
		Int("docker_hosts", normalized.DockerHostCount).
		Int("docker_containers_per_host", normalized.DockerContainersPerHost).
		Int("k8s_clusters", normalized.K8sClusterCount).
		Int("k8s_nodes_per_cluster", normalized.K8sNodesPerCluster).
		Int("k8s_pods_per_cluster", normalized.K8sPodsPerCluster).
		Int("k8s_deployments_per_cluster", normalized.K8sDeploymentsPerCluster).
		Bool("random_metrics", normalized.RandomMetrics).
		Float64("stopped_percent", normalized.StoppedPercent).
		Msg("Mock configuration updated")
}

// GetMockState returns the current mock state snapshot.
func GetMockState() models.StateSnapshot {
	if !IsMockEnabled() {
		return models.StateSnapshot{}
	}

	dataMu.RLock()
	defer dataMu.RUnlock()

	return cloneState(mockData)
}

// UpdateAlertSnapshots replaces the active and recently resolved alert lists used for mock mode.
// This lets other components read alert data without querying the live alert manager, which can
// be locked while alerts are being generated. Keeping a snapshot here prevents any blocking when
// the API serves /api/state or WebSocket clients request the initial state.
func UpdateAlertSnapshots(active []alerts.Alert, resolved []models.ResolvedAlert) {
	dataMu.Lock()
	defer dataMu.Unlock()

	converted := make([]models.Alert, 0, len(active))
	for _, alert := range active {
		converted = append(converted, models.Alert{
			ID:           alert.ID,
			Type:         alert.Type,
			Level:        string(alert.Level),
			ResourceID:   alert.ResourceID,
			ResourceName: alert.ResourceName,
			Node:         alert.Node,
			Instance:     alert.Instance,
			Message:      alert.Message,
			Value:        alert.Value,
			Threshold:    alert.Threshold,
			StartTime:    alert.StartTime,
			Acknowledged: alert.Acknowledged,
		})
	}
	mockData.ActiveAlerts = converted
	mockData.RecentlyResolved = append([]models.ResolvedAlert(nil), resolved...)
}

// GetMockAlertHistory returns mock alert history.
func GetMockAlertHistory(limit int) []models.Alert {
	if !IsMockEnabled() {
		return []models.Alert{}
	}

	dataMu.RLock()
	defer dataMu.RUnlock()

	if limit > 0 && limit < len(mockAlerts) {
		return append([]models.Alert(nil), mockAlerts[:limit]...)
	}
	return append([]models.Alert(nil), mockAlerts...)
}

func cloneState(state models.StateSnapshot) models.StateSnapshot {
	kubernetesClusters := make([]models.KubernetesCluster, len(state.KubernetesClusters))
	for i, cluster := range state.KubernetesClusters {
		clusterCopy := cluster

		clusterCopy.Nodes = append([]models.KubernetesNode(nil), cluster.Nodes...)

		clusterCopy.Pods = make([]models.KubernetesPod, len(cluster.Pods))
		for j, pod := range cluster.Pods {
			podCopy := pod

			if pod.Labels != nil {
				labelsCopy := make(map[string]string, len(pod.Labels))
				for k, v := range pod.Labels {
					labelsCopy[k] = v
				}
				podCopy.Labels = labelsCopy
			}

			podCopy.Containers = append([]models.KubernetesPodContainer(nil), pod.Containers...)
			clusterCopy.Pods[j] = podCopy
		}

		clusterCopy.Deployments = make([]models.KubernetesDeployment, len(cluster.Deployments))
		for j, dep := range cluster.Deployments {
			depCopy := dep
			if dep.Labels != nil {
				labelsCopy := make(map[string]string, len(dep.Labels))
				for k, v := range dep.Labels {
					labelsCopy[k] = v
				}
				depCopy.Labels = labelsCopy
			}
			clusterCopy.Deployments[j] = depCopy
		}

		kubernetesClusters[i] = clusterCopy
	}

	copyState := models.StateSnapshot{
		Nodes:                     append([]models.Node(nil), state.Nodes...),
		VMs:                       append([]models.VM(nil), state.VMs...),
		Containers:                append([]models.Container(nil), state.Containers...),
		DockerHosts:               append([]models.DockerHost(nil), state.DockerHosts...),
		KubernetesClusters:        kubernetesClusters,
		RemovedKubernetesClusters: append([]models.RemovedKubernetesCluster(nil), state.RemovedKubernetesClusters...),
		Hosts:                     append([]models.Host(nil), state.Hosts...),
		PMGInstances:              append([]models.PMGInstance(nil), state.PMGInstances...),
		Storage:                   append([]models.Storage(nil), state.Storage...),
		CephClusters:              append([]models.CephCluster(nil), state.CephClusters...),
		PhysicalDisks:             append([]models.PhysicalDisk(nil), state.PhysicalDisks...),
		PBSInstances:              append([]models.PBSInstance(nil), state.PBSInstances...),
		PBSBackups:                append([]models.PBSBackup(nil), state.PBSBackups...),
		PMGBackups:                append([]models.PMGBackup(nil), state.PMGBackups...),
		ReplicationJobs:           append([]models.ReplicationJob(nil), state.ReplicationJobs...),
		Metrics:                   append([]models.Metric(nil), state.Metrics...),
		Performance:               state.Performance,
		Stats:                     state.Stats,
		ActiveAlerts:              append([]models.Alert(nil), state.ActiveAlerts...),
		RecentlyResolved:          append([]models.ResolvedAlert(nil), state.RecentlyResolved...),
		LastUpdate:                state.LastUpdate,
		ConnectionHealth:          make(map[string]bool, len(state.ConnectionHealth)),
	}

	copyState.PVEBackups = models.PVEBackups{
		BackupTasks:    append([]models.BackupTask(nil), state.PVEBackups.BackupTasks...),
		StorageBackups: append([]models.StorageBackup(nil), state.PVEBackups.StorageBackups...),
		GuestSnapshots: append([]models.GuestSnapshot(nil), state.PVEBackups.GuestSnapshots...),
	}
	copyState.Backups = models.Backups{
		PVE: models.PVEBackups{
			BackupTasks:    append([]models.BackupTask(nil), state.Backups.PVE.BackupTasks...),
			StorageBackups: append([]models.StorageBackup(nil), state.Backups.PVE.StorageBackups...),
			GuestSnapshots: append([]models.GuestSnapshot(nil), state.Backups.PVE.GuestSnapshots...),
		},
		PBS: append([]models.PBSBackup(nil), state.Backups.PBS...),
		PMG: append([]models.PMGBackup(nil), state.Backups.PMG...),
	}

	for k, v := range state.ConnectionHealth {
		copyState.ConnectionHealth[k] = v
	}

	return copyState
}
