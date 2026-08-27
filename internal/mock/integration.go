package mock

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/mockruntime"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

var (
	dataMu          sync.RWMutex
	setEnabledMu    sync.Mutex
	updateLoopMu    sync.Mutex
	mockGraph       = emptyFixtureGraph()
	mockConfig      = DefaultConfig
	enabled         atomic.Bool
	fixtureRevision atomic.Uint64
	// fixtureDataVersion advances on EVERY observable mock-graph change:
	// metric ticks as well as the structural changes that bump
	// fixtureRevision. Caches of snapshots derived from the graph key on
	// this; fixtureRevision stays structural-only so the expensive seeded
	// trend history can be reused across monitor restarts within a process.
	fixtureDataVersion atomic.Uint64
	metricCohort       atomic.Uint64
	updateEveryNS      atomic.Int64
	updateTicker       *time.Ticker
	stopUpdatesCh      chan struct{}
	updateLoopWg       sync.WaitGroup
)

// Ten two-second cohorts keep every PVE node and its guests fresh within twenty
// seconds while bounding each realtime delta to roughly one tenth of a large
// demo estate. The WebSocket publisher may coalesce several sampler ticks into
// one broadcast, so this leaves enough headroom for that union to remain small.
const mockMetricCohortCount = 10

func init() {
	loadedConfig := normalizeMockConfig(LoadMockConfig())
	setMockUpdateInterval(loadedConfig.UpdateInterval)
	dataMu.Lock()
	mockConfig = loadedConfig
	dataMu.Unlock()

	initialEnabled := mockruntime.IsEnabled()
	if initialEnabled {
		log.Info().Msg("mock mode enabled at startup")
	}
	if err := setEnabled(initialEnabled, true); err != nil {
		log.Warn().Err(err).Msg("failed to enable mock mode at startup")
	}
}

func setMockUpdateInterval(interval time.Duration) {
	updateEveryNS.Store(int64(normalizeMockUpdateInterval(interval)))
}

func currentMockUpdateInterval() time.Duration {
	if interval := time.Duration(updateEveryNS.Load()); interval > 0 {
		return interval
	}
	return defaultMockUpdateInterval
}

func currentMockUpdateStepSeconds() float64 {
	seconds := currentMockUpdateInterval().Seconds()
	if seconds <= 0 {
		return defaultMockUpdateInterval.Seconds()
	}
	return seconds
}

func currentMockUpdateStepInt64() int64 {
	step := int64(currentMockUpdateStepSeconds())
	if step <= 0 {
		return int64(defaultMockUpdateInterval.Seconds())
	}
	return step
}

// IsMockEnabled returns whether mock mode is enabled.
func IsMockEnabled() bool {
	return enabled.Load()
}

// ErrReleaseFixturesUnauthorized is returned when a release build attempts to
// enable mock fixtures without an explicit entitled runtime authorization.
var ErrReleaseFixturesUnauthorized = mockruntime.ErrReleaseFixturesUnauthorized

// SetReleaseFixturesAuthorized sets whether the current runtime may enable mock
// fixtures in a release build.
func SetReleaseFixturesAuthorized(authorized bool) {
	mockruntime.SetReleaseFixturesAuthorized(authorized)
}

// SetEnabled enables or disables mock mode.
func SetEnabled(enable bool) error {
	return setEnabled(enable, false)
}

func setEnabled(enable bool, fromInit bool) error {
	setEnabledMu.Lock()
	defer setEnabledMu.Unlock()

	current := enabled.Load()
	if current == enable {
		// Still update env so other processes see the latest value when not invoked from init.
		if !fromInit && shouldSyncEnvFlag() {
			setEnvFlag(enable)
		}
		mockruntime.SetEnabled(enable)
		return nil
	}

	if err := mockruntime.ValidateEnablement(enable); err != nil {
		return err
	}

	dataMu.RLock()
	config := mockConfig
	dataMu.RUnlock()

	if enable {
		enableMockMode(config, fromInit)
	} else {
		disableMockMode()
	}

	mockruntime.SetEnabled(enable)

	if !fromInit && shouldSyncEnvFlag() {
		setEnvFlag(enable)
	}

	return nil
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

func enableMockMode(config MockConfig, fromInit bool) {
	config = normalizeMockConfig(config)
	now := time.Now()
	setMockUpdateInterval(config.UpdateInterval)

	dataMu.Lock()
	mockConfig = config
	mockGraph = buildFixtureGraph(config, now)
	fixtureRevision.Add(1)
	fixtureDataVersion.Add(1)
	metricCohort.Store(0)
	enabled.Store(true)
	dataMu.Unlock()
	startUpdateLoop()

	log.Info().
		Int("nodes", config.NodeCount).
		Int("vms_per_node", config.VMsPerNode).
		Int("lxcs_per_node", config.LXCsPerNode).
		Int("agent_hosts", config.GenericHostCount).
		Int("docker_hosts", config.DockerHostCount).
		Int("docker_containers_per_host", config.DockerContainersPerHost).
		Int("k8s_clusters", config.K8sClusterCount).
		Int("k8s_nodes_per_cluster", config.K8sNodesPerCluster).
		Int("k8s_pods_per_cluster", config.K8sPodsPerCluster).
		Int("k8s_deployments_per_cluster", config.K8sDeploymentsPerCluster).
		Bool("random_metrics", config.RandomMetrics).
		Float64("stopped_percent", config.StoppedPercent).
		Str("update_interval", config.UpdateInterval.String()).
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
	stopUpdateLoop()

	dataMu.Lock()
	mockGraph = emptyFixtureGraph()
	fixtureRevision.Add(1)
	fixtureDataVersion.Add(1)
	dataMu.Unlock()

	log.Info().Msg("mock mode disabled")
}

func startUpdateLoop() {
	updateLoopMu.Lock()
	defer updateLoopMu.Unlock()

	stopUpdateLoopLocked()
	stopCh := make(chan struct{})
	ticker := time.NewTicker(currentMockUpdateInterval())
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

	cohort := int(metricCohort.Add(1)-1) % mockMetricCohortCount
	mockGraph.UpdateMetricCohort(cfg, time.Now(), cohort, mockMetricCohortCount)
	fixtureDataVersion.Add(1)
}

// FixtureDataVersion returns a token that advances on every observable
// mock-graph change (metric ticks and structural changes alike). Derived
// snapshots cached against this token are current for as long as it is
// unchanged.
func FixtureDataVersion() uint64 {
	return fixtureDataVersion.Load()
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
		} else if math.IsNaN(percent) || math.IsInf(percent, 0) || percent < 0 {
			log.Warn().Str("env", "PULSE_MOCK_STOPPED_PERCENT").Str("value", raw).Msg("Invalid mock config value, expected percentage in range 0..100")
		} else {
			// Convert percentage to ratio; normalizeStoppedPercent will clamp >1 to 1.
			config.StoppedPercent = percent / 100.0
		}
	}

	if raw, ok := readMockEnv("PULSE_MOCK_UPDATE_INTERVAL"); ok {
		interval, err := time.ParseDuration(raw)
		if err != nil {
			log.Warn().Err(err).Str("env", "PULSE_MOCK_UPDATE_INTERVAL").Str("value", raw).Msg("Invalid mock config value, using default")
		} else if interval <= 0 {
			log.Warn().Str("env", "PULSE_MOCK_UPDATE_INTERVAL").Str("value", raw).Msg("Invalid mock config value, expected duration > 0")
		} else {
			config.UpdateInterval = interval
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
	if normalized.UpdateInterval != cfg.UpdateInterval {
		log.Warn().Str("provided", cfg.UpdateInterval.String()).Str("applied", normalized.UpdateInterval.String()).Msg("Normalized invalid mock UpdateInterval")
	}

	var configChanged bool
	var restartTicker bool
	dataMu.Lock()
	configChanged = !mockConfigsEqual(mockConfig, normalized)
	restartTicker = enabled.Load() && mockConfig.UpdateInterval != normalized.UpdateInterval
	mockConfig = normalized
	setMockUpdateInterval(normalized.UpdateInterval)
	if configChanged && enabled.Load() {
		mockGraph = buildFixtureGraph(normalized, time.Now())
		fixtureRevision.Add(1)
		fixtureDataVersion.Add(1)
	}
	dataMu.Unlock()

	if !configChanged {
		log.Debug().Msg("Mock configuration unchanged")
		return
	}

	log.Info().
		Int("nodes", normalized.NodeCount).
		Int("vms_per_node", normalized.VMsPerNode).
		Int("lxcs_per_node", normalized.LXCsPerNode).
		Int("agent_hosts", normalized.GenericHostCount).
		Int("docker_hosts", normalized.DockerHostCount).
		Int("docker_containers_per_host", normalized.DockerContainersPerHost).
		Int("k8s_clusters", normalized.K8sClusterCount).
		Int("k8s_nodes_per_cluster", normalized.K8sNodesPerCluster).
		Int("k8s_pods_per_cluster", normalized.K8sPodsPerCluster).
		Int("k8s_deployments_per_cluster", normalized.K8sDeploymentsPerCluster).
		Bool("random_metrics", normalized.RandomMetrics).
		Float64("stopped_percent", normalized.StoppedPercent).
		Str("update_interval", normalized.UpdateInterval.String()).
		Msg("Mock configuration updated")

	if restartTicker {
		startUpdateLoop()
	}
}

// UpdateAlertSnapshots replaces the active and recently resolved alert lists used for mock mode.
// This lets other components read alert data without querying the live alert manager, which can
// be locked while alerts are being generated. Keeping a snapshot here prevents any blocking when
// the API serves /api/state or WebSocket clients request the initial state.
func UpdateAlertSnapshots(active []alerts.Alert, resolved []models.ResolvedAlert) {
	dataMu.Lock()
	defer dataMu.Unlock()

	mockGraph.UpdateAlertSnapshots(active, resolved)
}

// GetMockAlertHistory returns mock alert history.
func GetMockAlertHistory(limit int) []models.Alert {
	if !IsMockEnabled() {
		return []models.Alert{}
	}

	dataMu.RLock()
	defer dataMu.RUnlock()

	if limit > 0 && limit < len(mockGraph.AlertHistory) {
		return cloneMockAlerts(mockGraph.AlertHistory[:limit])
	}
	return cloneMockAlerts(mockGraph.AlertHistory)
}

// GetMockAlertIncidentTimeline returns the occurrence-qualified incident
// fixture for a mock alert. Callers receive a defensive copy so UI filtering
// and note rendering cannot mutate the canonical fixture graph.
func GetMockAlertIncidentTimeline(alertIdentifier string, startedAt time.Time) *memory.Incident {
	if !IsMockEnabled() || strings.TrimSpace(alertIdentifier) == "" {
		return nil
	}

	dataMu.RLock()
	defer dataMu.RUnlock()
	for _, incident := range mockGraph.AlertIncidents {
		if incident == nil || incident.AlertIdentifier != alertIdentifier {
			continue
		}
		if !startedAt.IsZero() {
			delta := incident.OpenedAt.Sub(startedAt)
			if delta < 0 {
				delta = -delta
			}
			if delta > mockIncidentStartTolerance {
				continue
			}
		}
		return cloneMockIncident(incident)
	}
	return nil
}

// GetMockAlertIncidentsForResource returns newest-first incident fixtures for
// the resource-level timeline panel.
func GetMockAlertIncidentsForResource(resourceID string, limit int) []*memory.Incident {
	if !IsMockEnabled() || strings.TrimSpace(resourceID) == "" {
		return []*memory.Incident{}
	}

	dataMu.RLock()
	matches := make([]*memory.Incident, 0)
	for _, incident := range mockGraph.AlertIncidents {
		if incident != nil && incident.ResourceID == resourceID {
			matches = append(matches, cloneMockIncident(incident))
		}
	}
	dataMu.RUnlock()
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].OpenedAt.After(matches[j].OpenedAt)
	})
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

// AddMockAlertIncidentNote persists a note for the lifetime of the current
// mock fixture graph so the demo note workflow behaves like the real timeline.
func AddMockAlertIncidentNote(alertIdentifier, incidentID, note, user string) bool {
	if !IsMockEnabled() || strings.TrimSpace(note) == "" {
		return false
	}

	dataMu.Lock()
	defer dataMu.Unlock()
	for _, incident := range mockGraph.AlertIncidents {
		if incident == nil {
			continue
		}
		if incidentID != "" && incident.ID != incidentID {
			continue
		}
		if incidentID == "" && incident.AlertIdentifier != alertIdentifier {
			continue
		}
		noteAt := time.Now().UTC()
		if noteAt.Before(incident.OpenedAt) {
			noteAt = incident.OpenedAt
		}
		summary := "Note added"
		if strings.TrimSpace(user) != "" {
			summary = "Note added by " + strings.TrimSpace(user)
		}
		incident.Events = append(incident.Events, memory.IncidentEvent{
			ID:        fmt.Sprintf("mock-event-note-%s-%d", incident.AlertIdentifier, len(incident.Events)+1),
			Type:      memory.IncidentEventNote,
			Timestamp: noteAt,
			Summary:   summary,
			Details: map[string]interface{}{
				"note": strings.TrimSpace(note),
				"user": strings.TrimSpace(user),
			},
		})
		sort.SliceStable(incident.Events, func(i, j int) bool {
			return incident.Events[i].Timestamp.Before(incident.Events[j].Timestamp)
		})
		fixtureDataVersion.Add(1)
		return true
	}
	return false
}

func cloneState(state models.StateSnapshot) models.StateSnapshot {
	return state.Clone()
}
