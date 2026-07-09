package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// AlertManagerToolAdapter adapts alerts.Manager to AlertProvider interface
type AlertManagerToolAdapter struct {
	manager AlertManager
}

// AlertManager interface matches what alerts.Manager provides
type AlertManager interface {
	GetActiveAlerts() []alerts.Alert
	GetRecentlyResolved() []models.ResolvedAlert
}

// NewAlertManagerToolAdapter creates a new adapter for alert manager
func NewAlertManagerToolAdapter(manager AlertManager) *AlertManagerToolAdapter {
	if manager == nil {
		return nil
	}
	return &AlertManagerToolAdapter{manager: manager}
}

// GetActiveAlerts implements AlertProvider
func (a *AlertManagerToolAdapter) GetActiveAlerts() []ActiveAlert {
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

// GetRecentlyResolved implements AlertProvider
func (a *AlertManagerToolAdapter) GetRecentlyResolved(minutes int) []models.ResolvedAlert {
	if a.manager == nil {
		return nil
	}
	return a.manager.GetRecentlyResolved()
}

// GuestConfigSource provides guest configuration data with context.
type GuestConfigSource interface {
	GetGuestConfig(ctx context.Context, guestType, instance, node string, vmID int) (map[string]interface{}, error)
}

// GuestConfigToolAdapter adapts monitoring config access to GuestConfigProvider interface.
type GuestConfigToolAdapter struct {
	source  GuestConfigSource
	timeout time.Duration
}

// NewGuestConfigToolAdapter creates a new adapter for guest config data.
func NewGuestConfigToolAdapter(source GuestConfigSource) *GuestConfigToolAdapter {
	if source == nil {
		return nil
	}
	return &GuestConfigToolAdapter{
		source:  source,
		timeout: 5 * time.Second,
	}
}

// GetGuestConfig implements GuestConfigProvider.
func (a *GuestConfigToolAdapter) GetGuestConfig(guestType, instance, node string, vmID int) (map[string]interface{}, error) {
	if a == nil || a.source == nil {
		return nil, fmt.Errorf("guest config source not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
	defer cancel()
	return a.source.GetGuestConfig(ctx, guestType, instance, node, vmID)
}

// BackupToolAdapter adapts backup data sources to BackupProvider interface.
// Uses functional getters so callers can wire ReadState or StateSnapshot sources.
type BackupToolAdapter struct {
	getBackups      func() models.Backups
	getPBSInstances func() []models.PBSInstance
}

// NewBackupToolAdapter creates a new adapter for backup data.
// Both getters must be non-nil; returns nil if either is nil.
func NewBackupToolAdapter(getBackups func() models.Backups, getPBSInstances func() []models.PBSInstance) *BackupToolAdapter {
	if getBackups == nil || getPBSInstances == nil {
		return nil
	}
	return &BackupToolAdapter{getBackups: getBackups, getPBSInstances: getPBSInstances}
}

// GetBackups implements BackupProvider
func (a *BackupToolAdapter) GetBackups() models.Backups {
	if a.getBackups == nil {
		return models.Backups{}
	}
	return a.getBackups()
}

// GetPBSInstances implements BackupProvider
func (a *BackupToolAdapter) GetPBSInstances() []models.PBSInstance {
	if a.getPBSInstances == nil {
		return nil
	}
	return a.getPBSInstances()
}

// ReplicationToolAdapter adapts replication data sources to ReplicationProvider.
// Uses a functional getter so callers can wire ReadState or StateSnapshot sources.
type ReplicationToolAdapter struct {
	getJobs func() []models.ReplicationJob
}

// NewReplicationToolAdapter creates a new adapter for replication data.
func NewReplicationToolAdapter(getJobs func() []models.ReplicationJob) *ReplicationToolAdapter {
	if getJobs == nil {
		return nil
	}
	return &ReplicationToolAdapter{getJobs: getJobs}
}

// GetReplicationJobs implements ReplicationProvider.
func (a *ReplicationToolAdapter) GetReplicationJobs() []models.ReplicationJob {
	if a.getJobs == nil {
		return nil
	}
	return a.getJobs()
}

// ConnectionHealthToolAdapter adapts connection health data sources to ConnectionHealthProvider.
// Uses a functional getter so callers can wire ReadState or StateSnapshot sources.
type ConnectionHealthToolAdapter struct {
	getHealth func() map[string]bool
}

// NewConnectionHealthToolAdapter creates a new adapter for connection health data.
func NewConnectionHealthToolAdapter(getHealth func() map[string]bool) *ConnectionHealthToolAdapter {
	if getHealth == nil {
		return nil
	}
	return &ConnectionHealthToolAdapter{getHealth: getHealth}
}

// GetConnectionHealth implements ConnectionHealthProvider.
func (a *ConnectionHealthToolAdapter) GetConnectionHealth() map[string]bool {
	if a.getHealth == nil {
		return nil
	}
	return a.getHealth()
}

// RecoveryPointsToolAdapter provides Assistant tools access to persisted recovery points.
// It is org-scoped to avoid leaking cross-tenant data.
type RecoveryPointsToolAdapter struct {
	manager *recoverymanager.Manager
	orgID   string
	timeout time.Duration
}

func NewRecoveryPointsToolAdapter(manager *recoverymanager.Manager, orgID string) *RecoveryPointsToolAdapter {
	if manager == nil {
		return nil
	}
	if orgID == "" {
		orgID = "default"
	}
	return &RecoveryPointsToolAdapter{
		manager: manager,
		orgID:   orgID,
		timeout: 5 * time.Second,
	}
}

func (a *RecoveryPointsToolAdapter) ListPoints(ctx context.Context, opts recovery.ListPointsOptions) ([]recovery.RecoveryPoint, int, error) {
	if a == nil || a.manager == nil {
		return nil, 0, fmt.Errorf("recovery points provider not available")
	}
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	store, err := a.manager.StoreForOrg(a.orgID)
	if err != nil {
		return nil, 0, err
	}
	return store.ListPoints(ctx, opts)
}

// DiskHealthToolAdapter adapts unified read-state hosts to DiskHealthProvider.
type DiskHealthToolAdapter struct {
	readState unifiedresources.ReadState
}

// NewDiskHealthToolAdapter creates a new adapter for disk health data
func NewDiskHealthToolAdapter(readState unifiedresources.ReadState) *DiskHealthToolAdapter {
	if readState == nil {
		return nil
	}
	return &DiskHealthToolAdapter{readState: readState}
}

// GetHosts implements DiskHealthProvider
func (a *DiskHealthToolAdapter) GetHosts() []*unifiedresources.HostView {
	if a.readState == nil {
		return nil
	}
	return a.readState.Hosts()
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

// MetricsHistoryToolAdapter adapts the metrics history to MetricsHistoryProvider interface
type MetricsHistoryToolAdapter struct {
	readState     unifiedresources.ReadState
	metricsSource MetricsSource
}

// NewMetricsHistoryToolAdapter creates a new adapter for metrics history.
// readState is required for iterating VMs/Containers/Nodes in GetAllMetricsSummary.
func NewMetricsHistoryToolAdapter(metricsSource MetricsSource, readState unifiedresources.ReadState) *MetricsHistoryToolAdapter {
	if metricsSource == nil || readState == nil {
		return nil
	}
	return &MetricsHistoryToolAdapter{
		readState:     readState,
		metricsSource: metricsSource,
	}
}

// GetResourceMetrics implements MetricsHistoryProvider
func (a *MetricsHistoryToolAdapter) GetResourceMetrics(resourceID string, period time.Duration) ([]MetricPoint, error) {
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

// GetAllMetricsSummary implements MetricsHistoryProvider.
// Uses ReadState to iterate VMs, Containers, and Nodes.
func (a *MetricsHistoryToolAdapter) GetAllMetricsSummary(period time.Duration) (map[string]ResourceMetricsSummary, error) {
	if a.metricsSource == nil || a.readState == nil {
		return nil, nil
	}

	result := make(map[string]ResourceMetricsSummary)

	for _, vm := range a.readState.VMs() {
		vmID := fmt.Sprintf("%d", vm.VMID())
		if summary := a.computeSummary(vmID, vm.Name(), "vm", period); summary != nil {
			result[vmID] = *summary
		}
	}
	for _, ct := range a.readState.Containers() {
		ctID := fmt.Sprintf("%d", ct.VMID())
		if summary := a.computeSummary(ctID, ct.Name(), "system-container", period); summary != nil {
			result[ctID] = *summary
		}
	}
	// Nodes: SourceID() returns the legacy node ID that MetricsSource indexes by.
	for _, node := range a.readState.Nodes() {
		nodeID := node.SourceID()
		if nodeID == "" {
			continue
		}
		if summary := a.computeSummary(nodeID, node.Name(), "node", period); summary != nil {
			result[nodeID] = *summary
		}
	}

	return result, nil
}

func (a *MetricsHistoryToolAdapter) computeSummary(resourceID, resourceName, resourceType string, period time.Duration) *ResourceMetricsSummary {
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

// BaselineToolAdapter adapts baseline.Store to BaselineProvider interface
type BaselineToolAdapter struct {
	source BaselineSource
}

// NewBaselineToolAdapter creates a new adapter for baseline data
func NewBaselineToolAdapter(source BaselineSource) *BaselineToolAdapter {
	if source == nil {
		return nil
	}
	return &BaselineToolAdapter{source: source}
}

// GetBaseline implements BaselineProvider
func (a *BaselineToolAdapter) GetBaseline(resourceID, metric string) *MetricBaseline {
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

// GetAllBaselines implements BaselineProvider
func (a *BaselineToolAdapter) GetAllBaselines() map[string]map[string]*MetricBaseline {
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

// PatternToolAdapter adapts patterns.Detector to PatternProvider interface
type PatternToolAdapter struct {
	source    PatternSource
	readState unifiedresources.ReadState
}

// NewPatternToolAdapter creates a new adapter for pattern data.
// readState is optional: when provided, resource name lookups resolve via
// ReadState views; when nil, raw resource IDs are returned as names.
func NewPatternToolAdapter(source PatternSource, readState unifiedresources.ReadState) *PatternToolAdapter {
	if source == nil {
		return nil
	}
	return &PatternToolAdapter{source: source, readState: readState}
}

// GetPatterns implements PatternProvider
func (a *PatternToolAdapter) GetPatterns() []Pattern {
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

// GetPredictions implements PatternProvider
func (a *PatternToolAdapter) GetPredictions() []Prediction {
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

func (a *PatternToolAdapter) getResourceName(resourceID string) string {
	if a.readState == nil {
		return resourceID
	}
	for _, vm := range a.readState.VMs() {
		if fmt.Sprintf("%d", vm.VMID()) == resourceID {
			return vm.Name()
		}
	}
	for _, ct := range a.readState.Containers() {
		if fmt.Sprintf("%d", ct.VMID()) == resourceID {
			return ct.Name()
		}
	}
	// Nodes: SourceID() returns the legacy node ID that patterns store.
	for _, node := range a.readState.Nodes() {
		if node.SourceID() == resourceID {
			return node.Name()
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

// FindingsManagerToolAdapter adapts PatrolService to the Assistant findings manager provider.
type FindingsManagerToolAdapter struct {
	source FindingsManagerSource
}

// NewFindingsManagerToolAdapter creates a new adapter for findings management
func NewFindingsManagerToolAdapter(source FindingsManagerSource) *FindingsManagerToolAdapter {
	if source == nil {
		return nil
	}
	return &FindingsManagerToolAdapter{source: source}
}

// ResolveFinding implements FindingsManager
func (a *FindingsManagerToolAdapter) ResolveFinding(findingID, note string) error {
	if a.source == nil {
		return fmt.Errorf("findings manager not available")
	}
	return a.source.ResolveFinding(findingID, note)
}

// DismissFinding implements FindingsManager
func (a *FindingsManagerToolAdapter) DismissFinding(findingID, reason, note string) error {
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

// MetadataUpdaterToolAdapter adapts ai.Service to the Assistant metadata updater provider.
type MetadataUpdaterToolAdapter struct {
	source MetadataUpdaterSource
}

// NewMetadataUpdaterToolAdapter creates a new adapter for metadata updates
func NewMetadataUpdaterToolAdapter(source MetadataUpdaterSource) *MetadataUpdaterToolAdapter {
	if source == nil {
		return nil
	}
	return &MetadataUpdaterToolAdapter{source: source}
}

// SetResourceURL implements MetadataUpdater
func (a *MetadataUpdaterToolAdapter) SetResourceURL(resourceType, resourceID, url string) error {
	if a.source == nil {
		return fmt.Errorf("metadata updater not available")
	}
	return a.source.SetResourceURL(resourceType, resourceID, url)
}

// ========== Updates Provider Adapter ==========

// UpdatesCommandRunner is the subset of Monitor methods needed for Docker update commands.
// It does not include GetState — Docker host iteration uses a functional getter instead.
type UpdatesCommandRunner interface {
	QueueDockerCheckUpdatesCommand(hostID string) (models.DockerHostCommandStatus, error)
	QueueDockerContainerUpdateCommand(hostID, containerID, containerName string) (models.DockerHostCommandStatus, error)
	GetDockerCommandStatus(commandID string) (models.DockerHostCommandStatus, bool)
}

// UpdatesConfig provides configuration for update operations
type UpdatesConfig interface {
	IsDockerUpdateActionsEnabled() bool
}

// UpdatesToolAdapter adapts Monitor to UpdatesProvider interface.
// Uses a functional getter for Docker host iteration (decoupled from GetState)
// and a command runner for Docker update operations.
type UpdatesToolAdapter struct {
	readState unifiedresources.ReadState
	commands  UpdatesCommandRunner
	config    UpdatesConfig
}

// NewUpdatesToolAdapter creates a new adapter for update operations.
// readState provides Docker host/container views; commands provides update command execution.
func NewUpdatesToolAdapter(readState unifiedresources.ReadState, commands UpdatesCommandRunner, config UpdatesConfig) *UpdatesToolAdapter {
	if readState == nil || commands == nil {
		return nil
	}
	return &UpdatesToolAdapter{readState: readState, commands: commands, config: config}
}

// GetPendingUpdates implements UpdatesProvider
func (a *UpdatesToolAdapter) GetPendingUpdates(hostID string) []ContainerUpdateInfo {
	if a.readState == nil {
		return nil
	}

	hostLabels := make(map[string]string)
	for _, host := range a.readState.DockerHosts() {
		if host == nil {
			continue
		}
		label := strings.TrimSpace(host.Name())
		if label == "" {
			label = strings.TrimSpace(host.Hostname())
		}
		if label == "" {
			label = strings.TrimSpace(host.ID())
		}
		if resourceID := strings.TrimSpace(host.ID()); resourceID != "" {
			hostLabels[resourceID] = label
		}
		if sourceID := strings.TrimSpace(host.HostSourceID()); sourceID != "" {
			hostLabels[sourceID] = label
		}
	}

	var updates []ContainerUpdateInfo

	for _, container := range a.readState.DockerContainers() {
		if container == nil {
			continue
		}
		targetID := strings.TrimSpace(container.HostSourceID())
		if targetID == "" {
			targetID = strings.TrimSpace(container.ParentID())
		}
		if hostID != "" && targetID != hostID && strings.TrimSpace(container.ParentID()) != hostID {
			continue
		}

		updateStatus := container.UpdateStatus()
		if updateStatus == nil {
			continue
		}

		// Only include containers with updates available or errors.
		if !updateStatus.UpdateAvailable && updateStatus.Error == "" {
			continue
		}

		containerID := strings.TrimSpace(container.ContainerID())
		if containerID == "" {
			containerID = strings.TrimSpace(container.ID())
		}

		update := ContainerUpdateInfo{
			TargetID:        targetID,
			HostName:        hostLabels[targetID],
			ContainerID:     containerID,
			ContainerName:   trimContainerName(container.Name()),
			Image:           container.Image(),
			UpdateAvailable: updateStatus.UpdateAvailable,
		}

		if updateStatus.CurrentDigest != "" {
			update.CurrentDigest = updateStatus.CurrentDigest
		}
		if updateStatus.LatestDigest != "" {
			update.LatestDigest = updateStatus.LatestDigest
		}
		if !updateStatus.LastChecked.IsZero() {
			update.LastChecked = updateStatus.LastChecked.Unix()
		}
		if updateStatus.Error != "" {
			update.Error = updateStatus.Error
		}
		if update.HostName == "" {
			update.HostName = targetID
		}

		updates = append(updates, update)
	}

	return updates
}

// TriggerUpdateCheck implements UpdatesProvider
func (a *UpdatesToolAdapter) TriggerUpdateCheck(hostID string) (DockerCommandStatus, error) {
	if a.commands == nil {
		return DockerCommandStatus{}, fmt.Errorf("monitor not available")
	}

	cmdStatus, err := a.commands.QueueDockerCheckUpdatesCommand(hostID)
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

// UpdateContainer implements UpdatesProvider
func (a *UpdatesToolAdapter) UpdateContainer(hostID, containerID, containerName string) (DockerCommandStatus, error) {
	if a.commands == nil {
		return DockerCommandStatus{}, fmt.Errorf("monitor not available")
	}

	cmdStatus, err := a.commands.QueueDockerContainerUpdateCommand(hostID, containerID, containerName)
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

// GetCommandStatus implements UpdatesProvider
func (a *UpdatesToolAdapter) GetCommandStatus(commandID string) (DockerCommandStatus, bool) {
	if a.commands == nil {
		return DockerCommandStatus{}, false
	}

	cmdStatus, ok := a.commands.GetDockerCommandStatus(commandID)
	if !ok {
		return DockerCommandStatus{}, false
	}

	return DockerCommandStatus{
		ID:            cmdStatus.ID,
		Type:          cmdStatus.Type,
		Status:        cmdStatus.Status,
		Message:       cmdStatus.Message,
		FailureReason: cmdStatus.FailureReason,
	}, true
}

// IsUpdateActionsEnabled implements UpdatesProvider
func (a *UpdatesToolAdapter) IsUpdateActionsEnabled() bool {
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

// ========== Discovery Provider Adapter ==========

// DiscoverySource provides access to AI-powered infrastructure discovery data
type DiscoverySource interface {
	GetDiscovery(id string) (DiscoverySourceData, error)
	GetDiscoveryByResource(resourceType, targetID, resourceID string) (DiscoverySourceData, error)
	ListDiscoveries() ([]DiscoverySourceData, error)
	ListDiscoveriesByType(resourceType string) ([]DiscoverySourceData, error)
	ListDiscoveriesByTarget(targetID string) ([]DiscoverySourceData, error)
	FormatForAIContext(discoveries []DiscoverySourceData) string
	// TriggerDiscovery initiates discovery for a resource, returning discovered data
	TriggerDiscovery(ctx context.Context, resourceType, targetID, resourceID string, force bool) (DiscoverySourceData, error)
}

// DiscoverySourceData represents discovery data from the source
type DiscoverySourceData struct {
	ID             string
	ResourceType   string
	ResourceID     string
	TargetID       string
	AgentID        string
	Hostname       string
	ServiceType    string
	ServiceName    string
	ServiceVersion string
	Category       string
	CLIAccess      string
	Facts          []DiscoverySourceFact
	ConfigPaths    []string
	DataPaths      []string
	LogPaths       []string
	Ports          []DiscoverySourcePort
	DockerMounts   []DiscoverySourceDockerMount // Docker bind mounts (for LXCs/VMs running Docker)
	SuggestedURL   string
	UserNotes      string
	Confidence     float64
	AIReasoning    string
	DiscoveredAt   time.Time
	UpdatedAt      time.Time
}

// DiscoverySourceDockerMount represents a Docker bind mount from the source
type DiscoverySourceDockerMount struct {
	ContainerName string // Docker container name
	Source        string // Host path (where to actually write files)
	Destination   string // Container path (what the service sees)
	Type          string // Mount type: bind, volume, tmpfs
	ReadOnly      bool   // Whether mount is read-only
}

// DiscoverySourcePort represents a port from the source
type DiscoverySourcePort struct {
	Port     int
	Protocol string
	Process  string
	Address  string
}

// DiscoverySourceFact represents a fact from the source
type DiscoverySourceFact struct {
	Category   string
	Key        string
	Value      string
	Source     string
	Confidence float64 // 0-1 confidence for this fact
}

// DiscoveryToolAdapter adapts servicediscovery.Service to DiscoveryProvider interface
type DiscoveryToolAdapter struct {
	source DiscoverySource
}

// NewDiscoveryToolAdapter creates a new adapter for discovery data
func NewDiscoveryToolAdapter(source DiscoverySource) *DiscoveryToolAdapter {
	if source == nil {
		return nil
	}
	return &DiscoveryToolAdapter{source: source}
}

// GetDiscovery implements tools.DiscoveryProvider
func (a *DiscoveryToolAdapter) GetDiscovery(id string) (*ResourceDiscoveryInfo, error) {
	if a.source == nil {
		return nil, fmt.Errorf("discovery source not available")
	}

	data, err := a.source.GetDiscovery(id)
	if err != nil {
		return nil, err
	}

	return a.convertToInfo(data), nil
}

// GetDiscoveryByResource implements tools.DiscoveryProvider
func (a *DiscoveryToolAdapter) GetDiscoveryByResource(resourceType, targetID, resourceID string) (*ResourceDiscoveryInfo, error) {
	if a.source == nil {
		return nil, fmt.Errorf("discovery source not available")
	}

	data, err := a.source.GetDiscoveryByResource(resourceType, targetID, resourceID)
	if err != nil {
		return nil, err
	}

	return a.convertToInfo(data), nil
}

// ListDiscoveries implements tools.DiscoveryProvider
func (a *DiscoveryToolAdapter) ListDiscoveries() ([]*ResourceDiscoveryInfo, error) {
	if a.source == nil {
		return nil, fmt.Errorf("discovery source not available")
	}

	dataList, err := a.source.ListDiscoveries()
	if err != nil {
		return nil, err
	}

	return a.convertList(dataList), nil
}

// ListDiscoveriesByType implements tools.DiscoveryProvider
func (a *DiscoveryToolAdapter) ListDiscoveriesByType(resourceType string) ([]*ResourceDiscoveryInfo, error) {
	if a.source == nil {
		return nil, fmt.Errorf("discovery source not available")
	}

	dataList, err := a.source.ListDiscoveriesByType(resourceType)
	if err != nil {
		return nil, err
	}

	return a.convertList(dataList), nil
}

// ListDiscoveriesByTarget implements tools.DiscoveryProvider
func (a *DiscoveryToolAdapter) ListDiscoveriesByTarget(targetID string) ([]*ResourceDiscoveryInfo, error) {
	if a.source == nil {
		return nil, fmt.Errorf("discovery source not available")
	}

	dataList, err := a.source.ListDiscoveriesByTarget(targetID)
	if err != nil {
		return nil, err
	}

	return a.convertList(dataList), nil
}

// FormatForAIContext implements tools.DiscoveryProvider
func (a *DiscoveryToolAdapter) FormatForAIContext(discoveries []*ResourceDiscoveryInfo) string {
	if a.source == nil {
		return ""
	}

	// Convert back to source data format
	sourceData := make([]DiscoverySourceData, 0, len(discoveries))
	for _, d := range discoveries {
		if d == nil {
			continue
		}
		facts := make([]DiscoverySourceFact, 0, len(d.Facts))
		for _, f := range d.Facts {
			facts = append(facts, DiscoverySourceFact{
				Category:   f.Category,
				Key:        f.Key,
				Value:      f.Value,
				Source:     f.Source,
				Confidence: f.Confidence,
			})
		}
		ports := make([]DiscoverySourcePort, 0, len(d.Ports))
		for _, p := range d.Ports {
			ports = append(ports, DiscoverySourcePort{
				Port:     p.Port,
				Protocol: p.Protocol,
				Process:  p.Process,
				Address:  p.Address,
			})
		}
		dockerMounts := make([]DiscoverySourceDockerMount, 0, len(d.BindMounts))
		for _, m := range d.BindMounts {
			dockerMounts = append(dockerMounts, DiscoverySourceDockerMount{
				ContainerName: m.ContainerName,
				Source:        m.Source,
				Destination:   m.Destination,
				Type:          m.Type,
				ReadOnly:      m.ReadOnly,
			})
		}
		targetID := strings.TrimSpace(d.TargetID)
		sourceData = append(sourceData, DiscoverySourceData{
			ID:             d.ID,
			ResourceType:   d.ResourceType,
			ResourceID:     d.ResourceID,
			TargetID:       targetID,
			AgentID:        d.AgentID,
			Hostname:       d.Hostname,
			ServiceType:    d.ServiceType,
			ServiceName:    d.ServiceName,
			ServiceVersion: d.ServiceVersion,
			Category:       d.Category,
			CLIAccess:      d.CLIAccess,
			Facts:          facts,
			ConfigPaths:    d.ConfigPaths,
			DataPaths:      d.DataPaths,
			Ports:          ports,
			DockerMounts:   dockerMounts,
			UserNotes:      d.UserNotes,
			Confidence:     d.Confidence,
			AIReasoning:    d.AIReasoning,
			DiscoveredAt:   d.DiscoveredAt,
			UpdatedAt:      d.UpdatedAt,
		})
	}

	return a.source.FormatForAIContext(sourceData)
}

// TriggerDiscovery implements tools.DiscoveryProvider
func (a *DiscoveryToolAdapter) TriggerDiscovery(ctx context.Context, resourceType, targetID, resourceID string, force bool) (*ResourceDiscoveryInfo, error) {
	if a.source == nil {
		return nil, fmt.Errorf("discovery source not available")
	}

	data, err := a.source.TriggerDiscovery(ctx, resourceType, targetID, resourceID, force)
	if err != nil {
		return nil, err
	}

	return a.convertToInfo(data), nil
}

func (a *DiscoveryToolAdapter) convertToInfo(data DiscoverySourceData) *ResourceDiscoveryInfo {
	if data.ID == "" {
		return nil
	}

	facts := make([]DiscoveryFact, 0, len(data.Facts))
	for _, f := range data.Facts {
		facts = append(facts, DiscoveryFact{
			Category:   f.Category,
			Key:        f.Key,
			Value:      f.Value,
			Source:     f.Source,
			Confidence: f.Confidence,
		})
	}

	ports := make([]DiscoveryPortInfo, 0, len(data.Ports))
	for _, p := range data.Ports {
		ports = append(ports, DiscoveryPortInfo{
			Port:     p.Port,
			Protocol: p.Protocol,
			Process:  p.Process,
			Address:  p.Address,
		})
	}

	// Convert DockerMounts to BindMounts
	bindMounts := make([]DiscoveryMount, 0, len(data.DockerMounts))
	for _, m := range data.DockerMounts {
		bindMounts = append(bindMounts, DiscoveryMount{
			ContainerName: m.ContainerName,
			Source:        m.Source,
			Destination:   m.Destination,
			Type:          m.Type,
			ReadOnly:      m.ReadOnly,
		})
	}

	targetID := strings.TrimSpace(data.TargetID)
	agentID := strings.TrimSpace(data.AgentID)
	if agentID == "" && data.ResourceType == "agent" {
		agentID = targetID
	}

	return &ResourceDiscoveryInfo{
		ID:             data.ID,
		ResourceType:   data.ResourceType,
		ResourceID:     data.ResourceID,
		TargetID:       targetID,
		AgentID:        agentID,
		Hostname:       data.Hostname,
		ServiceType:    data.ServiceType,
		ServiceName:    data.ServiceName,
		ServiceVersion: data.ServiceVersion,
		Category:       data.Category,
		CLIAccess:      data.CLIAccess,
		Facts:          facts,
		ConfigPaths:    data.ConfigPaths,
		DataPaths:      data.DataPaths,
		LogPaths:       data.LogPaths,
		Ports:          ports,
		BindMounts:     bindMounts,
		UserNotes:      data.UserNotes,
		Confidence:     data.Confidence,
		AIReasoning:    data.AIReasoning,
		DiscoveredAt:   data.DiscoveredAt,
		UpdatedAt:      data.UpdatedAt,
	}
}

func (a *DiscoveryToolAdapter) convertList(dataList []DiscoverySourceData) []*ResourceDiscoveryInfo {
	result := make([]*ResourceDiscoveryInfo, 0, len(dataList))
	for _, data := range dataList {
		if info := a.convertToInfo(data); info != nil {
			result = append(result, info)
		}
	}
	return result
}
