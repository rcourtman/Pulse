package mock

import (
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

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

// ToggleMockMode enables or disables mock mode at runtime (backwards-compatible helper).
func ToggleMockMode(enable bool) {
	SetEnabled(enable)
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
	stopUpdatesCh = make(chan struct{})
	updateTicker = time.NewTicker(updateInterval)

	go func() {
		for {
			select {
			case <-updateTicker.C:
				cfg := GetConfig()
				updateMetrics(cfg)
			case <-stopUpdatesCh:
				return
			}
		}
	}()
}

func stopUpdateLoopLocked() {
	if updateTicker != nil {
		updateTicker.Stop()
		updateTicker = nil
	}
	if stopUpdatesCh != nil {
		close(stopUpdatesCh)
		stopUpdatesCh = nil
	}
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

	if val := os.Getenv("PULSE_MOCK_RANDOM_METRICS"); val != "" {
		config.RandomMetrics = val == "true"
	}

	if val := os.Getenv("PULSE_MOCK_STOPPED_PERCENT"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			config.StoppedPercent = f / 100.0
		}
	}

	return config
}

// SetMockConfig updates the mock configuration dynamically and regenerates data when enabled.
func SetMockConfig(cfg MockConfig) {
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
	copyState := models.StateSnapshot{
		Nodes:            append([]models.Node(nil), state.Nodes...),
		VMs:              append([]models.VM(nil), state.VMs...),
		Containers:       append([]models.Container(nil), state.Containers...),
		Storage:          append([]models.Storage(nil), state.Storage...),
		PhysicalDisks:    append([]models.PhysicalDisk(nil), state.PhysicalDisks...),
		PBSInstances:     append([]models.PBSInstance(nil), state.PBSInstances...),
		PBSBackups:       append([]models.PBSBackup(nil), state.PBSBackups...),
		Metrics:          append([]models.Metric(nil), state.Metrics...),
		Performance:      state.Performance,
		Stats:            state.Stats,
		ActiveAlerts:     append([]models.Alert(nil), state.ActiveAlerts...),
		RecentlyResolved: append([]models.ResolvedAlert(nil), state.RecentlyResolved...),
		LastUpdate:       state.LastUpdate,
		ConnectionHealth: make(map[string]bool, len(state.ConnectionHealth)),
	}

	copyState.PVEBackups = models.PVEBackups{
		BackupTasks:    append([]models.BackupTask(nil), state.PVEBackups.BackupTasks...),
		StorageBackups: append([]models.StorageBackup(nil), state.PVEBackups.StorageBackups...),
		GuestSnapshots: append([]models.GuestSnapshot(nil), state.PVEBackups.GuestSnapshots...),
	}

	for k, v := range state.ConnectionHealth {
		copyState.ConnectionHealth[k] = v
	}

	return copyState
}
