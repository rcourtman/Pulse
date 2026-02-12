package mock

import (
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

var (
	dataMu        sync.RWMutex
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
		log.Info().Msg("Mock mode enabled at startup")
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
	if enable {
		_ = os.Setenv("PULSE_MOCK_MODE", "true")
	} else {
		_ = os.Setenv("PULSE_MOCK_MODE", "false")
	}
}

func enableMockMode(fromInit bool) {
	config := LoadMockConfig()

	dataMu.Lock()
	mockConfig = config
	mockData = GenerateMockData(config)
	mockAlerts = GenerateAlertHistory(mockData.Nodes, mockData.VMs, mockData.Containers)
	mockData.LastUpdate = time.Now()
	enabled.Store(true)
	startUpdateLoopLocked()
	dataMu.Unlock()

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
		Msg("Mock mode enabled")

	if !fromInit {
		log.Info().Msg("Mock data generator started")
	}
}

func disableMockMode() {
	dataMu.Lock()
	if !enabled.Load() {
		dataMu.Unlock()
		return
	}
	enabled.Store(false)
	stopUpdateLoopLocked()
	mockData = models.StateSnapshot{}
	mockAlerts = nil
	dataMu.Unlock()

	log.Info().Msg("Mock mode disabled")
}

func startUpdateLoopLocked() {
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

func stopUpdateLoopLocked() {
	if ch := stopUpdatesCh; ch != nil {
		close(ch)
		stopUpdatesCh = nil
	}
	if ticker := updateTicker; ticker != nil {
		ticker.Stop()
		updateTicker = nil
	}
	// Wait for the update goroutine to exit
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

	if val := os.Getenv("PULSE_MOCK_NODES"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			config.NodeCount = n
		}
	}

	if val := os.Getenv("PULSE_MOCK_VMS_PER_NODE"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n >= 0 {
			config.VMsPerNode = n
		}
	}

	if val := os.Getenv("PULSE_MOCK_LXCS_PER_NODE"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n >= 0 {
			config.LXCsPerNode = n
		}
	}

	if val := os.Getenv("PULSE_MOCK_DOCKER_HOSTS"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n >= 0 {
			config.DockerHostCount = n
		}
	}

	if val := os.Getenv("PULSE_MOCK_DOCKER_CONTAINERS"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n >= 0 {
			config.DockerContainersPerHost = n
		}
	}

	if val := os.Getenv("PULSE_MOCK_GENERIC_HOSTS"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n >= 0 {
			config.GenericHostCount = n
		}
	}

	if val := os.Getenv("PULSE_MOCK_K8S_CLUSTERS"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n >= 0 {
			config.K8sClusterCount = n
		}
	}

	if val := os.Getenv("PULSE_MOCK_K8S_NODES"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n >= 0 {
			config.K8sNodesPerCluster = n
		}
	}

	if val := os.Getenv("PULSE_MOCK_K8S_PODS"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n >= 0 {
			config.K8sPodsPerCluster = n
		}
	}

	if val := os.Getenv("PULSE_MOCK_K8S_DEPLOYMENTS"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n >= 0 {
			config.K8sDeploymentsPerCluster = n
		}
	}

	if val := os.Getenv("PULSE_MOCK_RANDOM_METRICS"); val != "" {
		config.RandomMetrics = val == "true"
	}

	if val := os.Getenv("PULSE_MOCK_STOPPED_PERCENT"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			config.StoppedPercent = f / 100.0
		}
	}

	return normalizeMockConfig(config)
}

// SetMockConfig updates the mock configuration dynamically and regenerates data when enabled.
func SetMockConfig(cfg MockConfig) {
	cfg = normalizeMockConfig(cfg)

	dataMu.Lock()
	mockConfig = cfg
	if enabled.Load() {
		mockData = GenerateMockData(cfg)
		mockAlerts = GenerateAlertHistory(mockData.Nodes, mockData.VMs, mockData.Containers)
		mockData.LastUpdate = time.Now()
	}
	dataMu.Unlock()

	log.Info().
		Int("nodes", cfg.NodeCount).
		Int("vms_per_node", cfg.VMsPerNode).
		Int("lxcs_per_node", cfg.LXCsPerNode).
		Int("docker_hosts", cfg.DockerHostCount).
		Int("docker_containers_per_host", cfg.DockerContainersPerHost).
		Int("k8s_clusters", cfg.K8sClusterCount).
		Int("k8s_nodes_per_cluster", cfg.K8sNodesPerCluster).
		Int("k8s_pods_per_cluster", cfg.K8sPodsPerCluster).
		Int("k8s_deployments_per_cluster", cfg.K8sDeploymentsPerCluster).
		Bool("random_metrics", cfg.RandomMetrics).
		Float64("stopped_percent", cfg.StoppedPercent).
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
