package tools

import (
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// StateGetter provides access to the current infrastructure state
type StateGetter interface {
	GetState() models.StateSnapshot
}

// AlertManagerMCPAdapter adapts alerts.Manager to MCP AlertProvider interface
type AlertManagerMCPAdapter struct {
	manager AlertManager
}

// AlertManager interface matches what alerts.Manager provides
type AlertManager interface {
	GetActiveAlerts() []alerts.Alert
}

// NewAlertManagerMCPAdapter creates a new adapter for alert manager
func NewAlertManagerMCPAdapter(manager AlertManager) *AlertManagerMCPAdapter {
	if manager == nil {
		return nil
	}
	return &AlertManagerMCPAdapter{manager: manager}
}

// GetActiveAlerts implements mcp.AlertProvider
func (a *AlertManagerMCPAdapter) GetActiveAlerts() []ActiveAlert {
	if a.manager == nil {
		return nil
	}

	activeAlerts := a.manager.GetActiveAlerts()
	result := make([]ActiveAlert, 0, len(activeAlerts))

	for _, alert := range activeAlerts {
		result = append(result, ActiveAlert{
			ID:           alert.ID,
			ResourceID:   alert.ResourceID,
			ResourceName: alert.ResourceName,
			Type:         alert.Type,
			Severity:     string(alert.Level),
			Value:        alert.Value,
			Threshold:    alert.Threshold,
			StartTime:    alert.StartTime,
			Message:      alert.Message,
		})
	}

	return result
}

// StorageMCPAdapter adapts the monitor state to MCP StorageProvider interface
type StorageMCPAdapter struct {
	stateGetter StateGetter
}

// NewStorageMCPAdapter creates a new adapter for storage data
func NewStorageMCPAdapter(stateGetter StateGetter) *StorageMCPAdapter {
	if stateGetter == nil {
		return nil
	}
	return &StorageMCPAdapter{stateGetter: stateGetter}
}

// GetStorage implements mcp.StorageProvider
func (a *StorageMCPAdapter) GetStorage() []models.Storage {
	if a.stateGetter == nil {
		return nil
	}
	state := a.stateGetter.GetState()
	return state.Storage
}

// GetCephClusters implements mcp.StorageProvider
func (a *StorageMCPAdapter) GetCephClusters() []models.CephCluster {
	if a.stateGetter == nil {
		return nil
	}
	state := a.stateGetter.GetState()
	return state.CephClusters
}

// BackupMCPAdapter adapts the monitor state to MCP BackupProvider interface
type BackupMCPAdapter struct {
	stateGetter StateGetter
}

// NewBackupMCPAdapter creates a new adapter for backup data
func NewBackupMCPAdapter(stateGetter StateGetter) *BackupMCPAdapter {
	if stateGetter == nil {
		return nil
	}
	return &BackupMCPAdapter{stateGetter: stateGetter}
}

// GetBackups implements mcp.BackupProvider
func (a *BackupMCPAdapter) GetBackups() models.Backups {
	if a.stateGetter == nil {
		return models.Backups{}
	}
	state := a.stateGetter.GetState()
	return state.Backups
}

// GetPBSInstances implements mcp.BackupProvider
func (a *BackupMCPAdapter) GetPBSInstances() []models.PBSInstance {
	if a.stateGetter == nil {
		return nil
	}
	state := a.stateGetter.GetState()
	return state.PBSInstances
}

// DiskHealthMCPAdapter adapts the monitor state to MCP DiskHealthProvider interface
type DiskHealthMCPAdapter struct {
	stateGetter StateGetter
}

// NewDiskHealthMCPAdapter creates a new adapter for disk health data
func NewDiskHealthMCPAdapter(stateGetter StateGetter) *DiskHealthMCPAdapter {
	if stateGetter == nil {
		return nil
	}
	return &DiskHealthMCPAdapter{stateGetter: stateGetter}
}

// GetHosts implements mcp.DiskHealthProvider
func (a *DiskHealthMCPAdapter) GetHosts() []models.Host {
	if a.stateGetter == nil {
		return nil
	}
	state := a.stateGetter.GetState()
	return state.Hosts
}

// RawMetricPoint represents a single metric value at a point in time
type RawMetricPoint struct {
	Value     float64
	Timestamp time.Time
}

// MetricsSource provides access to historical metrics data
type MetricsSource interface {
	GetGuestMetrics(guestID string, metricType string, duration time.Duration) []RawMetricPoint
	GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []RawMetricPoint
	GetAllGuestMetrics(guestID string, duration time.Duration) map[string][]RawMetricPoint
}

// MetricsHistoryMCPAdapter adapts the metrics history to MCP MetricsHistoryProvider interface
type MetricsHistoryMCPAdapter struct {
	stateGetter   StateGetter
	metricsSource MetricsSource
}

// NewMetricsHistoryMCPAdapter creates a new adapter for metrics history
func NewMetricsHistoryMCPAdapter(stateGetter StateGetter, metricsSource MetricsSource) *MetricsHistoryMCPAdapter {
	if stateGetter == nil || metricsSource == nil {
		return nil
	}
	return &MetricsHistoryMCPAdapter{
		stateGetter:   stateGetter,
		metricsSource: metricsSource,
	}
}

// GetResourceMetrics implements mcp.MetricsHistoryProvider
func (a *MetricsHistoryMCPAdapter) GetResourceMetrics(resourceID string, period time.Duration) ([]MetricPoint, error) {
	if a.metricsSource == nil {
		return nil, nil
	}

	// Try guest metrics first (VMs and containers)
	allMetrics := a.metricsSource.GetAllGuestMetrics(resourceID, period)
	if len(allMetrics) == 0 {
		// Try node metrics
		cpuPoints := a.metricsSource.GetNodeMetrics(resourceID, "cpu", period)
		memPoints := a.metricsSource.GetNodeMetrics(resourceID, "memory", period)
		if len(cpuPoints) > 0 || len(memPoints) > 0 {
			allMetrics = map[string][]RawMetricPoint{
				"cpu":    cpuPoints,
				"memory": memPoints,
			}
		}
	}

	if len(allMetrics) == 0 {
		return nil, nil
	}

	// Merge metrics by timestamp
	return mergeMetricsByTimestamp(allMetrics), nil
}

// GetAllMetricsSummary implements mcp.MetricsHistoryProvider
func (a *MetricsHistoryMCPAdapter) GetAllMetricsSummary(period time.Duration) (map[string]ResourceMetricsSummary, error) {
	if a.stateGetter == nil || a.metricsSource == nil {
		return nil, nil
	}

	state := a.stateGetter.GetState()
	result := make(map[string]ResourceMetricsSummary)

	// Process VMs
	for _, vm := range state.VMs {
		vmID := fmt.Sprintf("%d", vm.VMID)
		if summary := a.computeSummary(vmID, vm.Name, "vm", period); summary != nil {
			result[vmID] = *summary
		}
	}

	// Process containers
	for _, ct := range state.Containers {
		ctID := fmt.Sprintf("%d", ct.VMID)
		if summary := a.computeSummary(ctID, ct.Name, "container", period); summary != nil {
			result[ctID] = *summary
		}
	}

	// Process nodes
	for _, node := range state.Nodes {
		if summary := a.computeSummary(node.ID, node.Name, "node", period); summary != nil {
			result[node.ID] = *summary
		}
	}

	return result, nil
}

func (a *MetricsHistoryMCPAdapter) computeSummary(resourceID, resourceName, resourceType string, period time.Duration) *ResourceMetricsSummary {
	var cpuPoints, memPoints []RawMetricPoint

	if resourceType == "node" {
		cpuPoints = a.metricsSource.GetNodeMetrics(resourceID, "cpu", period)
		memPoints = a.metricsSource.GetNodeMetrics(resourceID, "memory", period)
	} else {
		cpuPoints = a.metricsSource.GetGuestMetrics(resourceID, "cpu", period)
		memPoints = a.metricsSource.GetGuestMetrics(resourceID, "memory", period)
	}

	if len(cpuPoints) == 0 && len(memPoints) == 0 {
		return nil
	}

	summary := &ResourceMetricsSummary{
		ResourceID:   resourceID,
		ResourceName: resourceName,
		ResourceType: resourceType,
		Trend:        "stable",
	}

	if len(cpuPoints) > 0 {
		summary.AvgCPU, summary.MaxCPU = computeStats(cpuPoints)
		summary.Trend = computeTrend(cpuPoints)
	}
	if len(memPoints) > 0 {
		summary.AvgMemory, summary.MaxMemory = computeStats(memPoints)
	}

	return summary
}

func mergeMetricsByTimestamp(allMetrics map[string][]RawMetricPoint) []MetricPoint {
	// Build a map of timestamp -> MetricPoint
	pointsByTime := make(map[int64]*MetricPoint)

	for metricType, points := range allMetrics {
		for _, p := range points {
			ts := p.Timestamp.Unix()
			if _, exists := pointsByTime[ts]; !exists {
				pointsByTime[ts] = &MetricPoint{Timestamp: p.Timestamp}
			}
			switch metricType {
			case "cpu":
				pointsByTime[ts].CPU = p.Value
			case "memory":
				pointsByTime[ts].Memory = p.Value
			case "disk":
				pointsByTime[ts].Disk = p.Value
			}
		}
	}

	// Convert to slice and sort by timestamp
	result := make([]MetricPoint, 0, len(pointsByTime))
	for _, p := range pointsByTime {
		result = append(result, *p)
	}

	return result
}

func computeStats(points []RawMetricPoint) (avg, max float64) {
	if len(points) == 0 {
		return 0, 0
	}

	var sum float64
	for _, p := range points {
		sum += p.Value
		if p.Value > max {
			max = p.Value
		}
	}
	avg = sum / float64(len(points))
	return avg, max
}

func computeTrend(points []RawMetricPoint) string {
	if len(points) < 2 {
		return "stable"
	}

	// Compare first quarter average to last quarter average
	quarter := len(points) / 4
	if quarter < 1 {
		quarter = 1
	}

	var firstSum, lastSum float64
	for i := 0; i < quarter; i++ {
		firstSum += points[i].Value
	}
	for i := len(points) - quarter; i < len(points); i++ {
		lastSum += points[i].Value
	}

	firstAvg := firstSum / float64(quarter)
	lastAvg := lastSum / float64(quarter)

	diff := lastAvg - firstAvg
	threshold := 5.0 // 5% change threshold

	if diff > threshold {
		return "growing"
	} else if diff < -threshold {
		return "declining"
	}
	return "stable"
}

// ========== Baseline Provider Adapter ==========

// BaselineSource provides access to baseline data
type BaselineSource interface {
	GetBaseline(resourceID, metric string) (mean, stddev float64, sampleCount int, ok bool)
	GetAllBaselines() map[string]map[string]BaselineData
}

// BaselineData represents baseline data from the source
type BaselineData struct {
	Mean        float64
	StdDev      float64
	SampleCount int
}

// BaselineMCPAdapter adapts baseline.Store to MCP BaselineProvider interface
type BaselineMCPAdapter struct {
	source BaselineSource
}

// NewBaselineMCPAdapter creates a new adapter for baseline data
func NewBaselineMCPAdapter(source BaselineSource) *BaselineMCPAdapter {
	if source == nil {
		return nil
	}
	return &BaselineMCPAdapter{source: source}
}

// GetBaseline implements mcp.BaselineProvider
func (a *BaselineMCPAdapter) GetBaseline(resourceID, metric string) *MetricBaseline {
	if a.source == nil {
		return nil
	}

	mean, stddev, _, ok := a.source.GetBaseline(resourceID, metric)
	if !ok {
		return nil
	}

	return &MetricBaseline{
		Mean:   mean,
		StdDev: stddev,
		Min:    mean - 2*stddev, // Approximate
		Max:    mean + 2*stddev, // Approximate
	}
}

// GetAllBaselines implements mcp.BaselineProvider
func (a *BaselineMCPAdapter) GetAllBaselines() map[string]map[string]*MetricBaseline {
	if a.source == nil {
		return nil
	}

	sourceData := a.source.GetAllBaselines()
	if sourceData == nil {
		return nil
	}

	result := make(map[string]map[string]*MetricBaseline)
	for resourceID, metrics := range sourceData {
		result[resourceID] = make(map[string]*MetricBaseline)
		for metric, data := range metrics {
			result[resourceID][metric] = &MetricBaseline{
				Mean:   data.Mean,
				StdDev: data.StdDev,
				Min:    data.Mean - 2*data.StdDev,
				Max:    data.Mean + 2*data.StdDev,
			}
		}
	}

	return result
}

// ========== Pattern Provider Adapter ==========

// PatternSource provides access to pattern and prediction data
type PatternSource interface {
	GetPatterns() []PatternData
	GetPredictions() []PredictionData
}

// PatternData represents pattern data from the source
type PatternData struct {
	ResourceID  string
	PatternType string
	Description string
	Confidence  float64
	LastSeen    time.Time
}

// PredictionData represents prediction data from the source
type PredictionData struct {
	ResourceID     string
	IssueType      string
	PredictedTime  time.Time
	Confidence     float64
	Recommendation string
}

// PatternMCPAdapter adapts patterns.Detector to MCP PatternProvider interface
type PatternMCPAdapter struct {
	source      PatternSource
	stateGetter StateGetter
}

// NewPatternMCPAdapter creates a new adapter for pattern data
func NewPatternMCPAdapter(source PatternSource, stateGetter StateGetter) *PatternMCPAdapter {
	if source == nil {
		return nil
	}
	return &PatternMCPAdapter{source: source, stateGetter: stateGetter}
}

// GetPatterns implements mcp.PatternProvider
func (a *PatternMCPAdapter) GetPatterns() []Pattern {
	if a.source == nil {
		return nil
	}

	sourcePatterns := a.source.GetPatterns()
	if sourcePatterns == nil {
		return nil
	}

	result := make([]Pattern, 0, len(sourcePatterns))
	for _, p := range sourcePatterns {
		result = append(result, Pattern{
			ResourceID:   p.ResourceID,
			ResourceName: a.getResourceName(p.ResourceID),
			PatternType:  p.PatternType,
			Description:  p.Description,
			Confidence:   p.Confidence,
			LastSeen:     p.LastSeen,
		})
	}

	return result
}

// GetPredictions implements mcp.PatternProvider
func (a *PatternMCPAdapter) GetPredictions() []Prediction {
	if a.source == nil {
		return nil
	}

	sourcePredictions := a.source.GetPredictions()
	if sourcePredictions == nil {
		return nil
	}

	result := make([]Prediction, 0, len(sourcePredictions))
	for _, p := range sourcePredictions {
		result = append(result, Prediction{
			ResourceID:     p.ResourceID,
			ResourceName:   a.getResourceName(p.ResourceID),
			IssueType:      p.IssueType,
			PredictedTime:  p.PredictedTime,
			Confidence:     p.Confidence,
			Recommendation: p.Recommendation,
		})
	}

	return result
}

func (a *PatternMCPAdapter) getResourceName(resourceID string) string {
	if a.stateGetter == nil {
		return resourceID
	}
	state := a.stateGetter.GetState()
	for _, vm := range state.VMs {
		if fmt.Sprintf("%d", vm.VMID) == resourceID {
			return vm.Name
		}
	}
	for _, ct := range state.Containers {
		if fmt.Sprintf("%d", ct.VMID) == resourceID {
			return ct.Name
		}
	}
	for _, node := range state.Nodes {
		if node.ID == resourceID {
			return node.Name
		}
	}
	return resourceID
}

// ========== Findings Manager Adapter ==========

// FindingsManagerSource provides findings management operations
type FindingsManagerSource interface {
	ResolveFinding(findingID, note string) error
	DismissFinding(findingID, reason, note string) error
}

// FindingsManagerMCPAdapter adapts PatrolService to MCP FindingsManager interface
type FindingsManagerMCPAdapter struct {
	source FindingsManagerSource
}

// NewFindingsManagerMCPAdapter creates a new adapter for findings management
func NewFindingsManagerMCPAdapter(source FindingsManagerSource) *FindingsManagerMCPAdapter {
	if source == nil {
		return nil
	}
	return &FindingsManagerMCPAdapter{source: source}
}

// ResolveFinding implements mcp.FindingsManager
func (a *FindingsManagerMCPAdapter) ResolveFinding(findingID, note string) error {
	if a.source == nil {
		return fmt.Errorf("findings manager not available")
	}
	return a.source.ResolveFinding(findingID, note)
}

// DismissFinding implements mcp.FindingsManager
func (a *FindingsManagerMCPAdapter) DismissFinding(findingID, reason, note string) error {
	if a.source == nil {
		return fmt.Errorf("findings manager not available")
	}
	return a.source.DismissFinding(findingID, reason, note)
}

// ========== Metadata Updater Adapter ==========

// MetadataUpdaterSource provides metadata update operations
type MetadataUpdaterSource interface {
	SetResourceURL(resourceType, resourceID, url string) error
}

// MetadataUpdaterMCPAdapter adapts ai.Service to MCP MetadataUpdater interface
type MetadataUpdaterMCPAdapter struct {
	source MetadataUpdaterSource
}

// NewMetadataUpdaterMCPAdapter creates a new adapter for metadata updates
func NewMetadataUpdaterMCPAdapter(source MetadataUpdaterSource) *MetadataUpdaterMCPAdapter {
	if source == nil {
		return nil
	}
	return &MetadataUpdaterMCPAdapter{source: source}
}

// SetResourceURL implements mcp.MetadataUpdater
func (a *MetadataUpdaterMCPAdapter) SetResourceURL(resourceType, resourceID, url string) error {
	if a.source == nil {
		return fmt.Errorf("metadata updater not available")
	}
	return a.source.SetResourceURL(resourceType, resourceID, url)
}

// ========== Updates Provider Adapter ==========

// UpdatesMonitor is the subset of Monitor methods needed for update operations
type UpdatesMonitor interface {
	GetState() models.StateSnapshot
	QueueDockerCheckUpdatesCommand(hostID string) (models.DockerHostCommandStatus, error)
	QueueDockerContainerUpdateCommand(hostID, containerID, containerName string) (models.DockerHostCommandStatus, error)
}

// UpdatesConfig provides configuration for update operations
type UpdatesConfig interface {
	IsDockerUpdateActionsEnabled() bool
}

// UpdatesMCPAdapter adapts Monitor to MCP UpdatesProvider interface
type UpdatesMCPAdapter struct {
	monitor UpdatesMonitor
	config  UpdatesConfig
}

// NewUpdatesMCPAdapter creates a new adapter for update operations
func NewUpdatesMCPAdapter(monitor UpdatesMonitor, config UpdatesConfig) *UpdatesMCPAdapter {
	if monitor == nil {
		return nil
	}
	return &UpdatesMCPAdapter{monitor: monitor, config: config}
}

// GetPendingUpdates implements mcp.UpdatesProvider
func (a *UpdatesMCPAdapter) GetPendingUpdates(hostID string) []ContainerUpdateInfo {
	if a.monitor == nil {
		return nil
	}

	state := a.monitor.GetState()
	var updates []ContainerUpdateInfo

	for _, host := range state.DockerHosts {
		// Filter by host ID if specified
		if hostID != "" && host.ID != hostID {
			continue
		}

		for _, container := range host.Containers {
			if container.UpdateStatus == nil {
				continue
			}

			// Only include containers with updates available or errors
			if !container.UpdateStatus.UpdateAvailable && container.UpdateStatus.Error == "" {
				continue
			}

			update := ContainerUpdateInfo{
				HostID:          host.ID,
				HostName:        host.DisplayName,
				ContainerID:     container.ID,
				ContainerName:   trimContainerName(container.Name),
				Image:           container.Image,
				UpdateAvailable: container.UpdateStatus.UpdateAvailable,
			}

			if container.UpdateStatus.CurrentDigest != "" {
				update.CurrentDigest = container.UpdateStatus.CurrentDigest
			}
			if container.UpdateStatus.LatestDigest != "" {
				update.LatestDigest = container.UpdateStatus.LatestDigest
			}
			if !container.UpdateStatus.LastChecked.IsZero() {
				update.LastChecked = container.UpdateStatus.LastChecked.Unix()
			}
			if container.UpdateStatus.Error != "" {
				update.Error = container.UpdateStatus.Error
			}

			updates = append(updates, update)
		}
	}

	return updates
}

// TriggerUpdateCheck implements mcp.UpdatesProvider
func (a *UpdatesMCPAdapter) TriggerUpdateCheck(hostID string) (DockerCommandStatus, error) {
	if a.monitor == nil {
		return DockerCommandStatus{}, fmt.Errorf("monitor not available")
	}

	cmdStatus, err := a.monitor.QueueDockerCheckUpdatesCommand(hostID)
	if err != nil {
		return DockerCommandStatus{}, err
	}

	return DockerCommandStatus{
		ID:      cmdStatus.ID,
		Type:    cmdStatus.Type,
		Status:  cmdStatus.Status,
		Message: cmdStatus.Message,
	}, nil
}

// UpdateContainer implements mcp.UpdatesProvider
func (a *UpdatesMCPAdapter) UpdateContainer(hostID, containerID, containerName string) (DockerCommandStatus, error) {
	if a.monitor == nil {
		return DockerCommandStatus{}, fmt.Errorf("monitor not available")
	}

	cmdStatus, err := a.monitor.QueueDockerContainerUpdateCommand(hostID, containerID, containerName)
	if err != nil {
		return DockerCommandStatus{}, err
	}

	return DockerCommandStatus{
		ID:      cmdStatus.ID,
		Type:    cmdStatus.Type,
		Status:  cmdStatus.Status,
		Message: cmdStatus.Message,
	}, nil
}

// IsUpdateActionsEnabled implements mcp.UpdatesProvider
func (a *UpdatesMCPAdapter) IsUpdateActionsEnabled() bool {
	if a.config == nil {
		return true // Default to enabled if no config
	}
	return a.config.IsDockerUpdateActionsEnabled()
}

// trimContainerName removes the leading slash from container names
func trimContainerName(name string) string {
	if len(name) > 0 && name[0] == '/' {
		return name[1:]
	}
	return name
}
