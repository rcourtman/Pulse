package truenas

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const (
	// FeatureTrueNAS allows explicit opt-out of the default-on TrueNAS API-backed
	// platform integration.
	FeatureTrueNAS = "PULSE_ENABLE_TRUENAS"
)

var featureTrueNASEnabled atomic.Bool

func init() {
	featureTrueNASEnabled.Store(parseFeatureEnabled(os.Getenv(FeatureTrueNAS)))
}

var errNilSnapshot = errors.New("truenas provider fetcher returned nil snapshot")

// IsFeatureEnabled returns whether the TrueNAS API-backed platform integration
// is enabled.
func IsFeatureEnabled() bool {
	return featureTrueNASEnabled.Load()
}

// SetFeatureEnabled allows tests to control the feature flag.
func SetFeatureEnabled(enabled bool) {
	featureTrueNASEnabled.Store(enabled)
}

// ResetFeatureEnabledFromEnv restores the feature flag from the current
// environment configuration.
func ResetFeatureEnabledFromEnv() {
	featureTrueNASEnabled.Store(parseFeatureEnabled(os.Getenv(FeatureTrueNAS)))
}

// Fetcher loads a TrueNAS snapshot from a concrete source.
type Fetcher interface {
	Fetch(ctx context.Context) (*FixtureSnapshot, error)
}

type fetcherCloser interface {
	Close()
}

type appActionFetcher interface {
	StartApp(ctx context.Context, appID string) error
	StopApp(ctx context.Context, appID string) error
}

type appReadFetcher interface {
	ReadAppLogs(ctx context.Context, appName, containerID string, tailLines int) ([]AppLogLine, error)
}

type physicalDiskHistoryFetcher interface {
	DiskTemperatureHistory(ctx context.Context, identifiers []string, duration time.Duration) (map[string][]TimeSeriesPoint, error)
}

type systemMetricHistoryFetcher interface {
	SystemMetricHistory(ctx context.Context, duration time.Duration) (*SystemMetricHistory, error)
}

// APIFetcher loads snapshots from the live TrueNAS API client.
type APIFetcher struct {
	Client *Client
}

// Fetch implements Fetcher.
func (f *APIFetcher) Fetch(ctx context.Context) (*FixtureSnapshot, error) {
	if f == nil || f.Client == nil {
		return nil, fmt.Errorf("truenas api fetcher client is nil")
	}
	return f.Client.FetchSnapshot(ctx)
}

// Close releases idle resources held by the underlying API client.
func (f *APIFetcher) Close() {
	if f == nil || f.Client == nil {
		return
	}
	f.Client.Close()
}

func (f *APIFetcher) StartApp(ctx context.Context, appID string) error {
	if f == nil || f.Client == nil {
		return fmt.Errorf("truenas api fetcher client is nil")
	}
	return f.Client.StartApp(ctx, appID)
}

func (f *APIFetcher) StopApp(ctx context.Context, appID string) error {
	if f == nil || f.Client == nil {
		return fmt.Errorf("truenas api fetcher client is nil")
	}
	return f.Client.StopApp(ctx, appID)
}

func (f *APIFetcher) ReadAppLogs(ctx context.Context, appName, containerID string, tailLines int) ([]AppLogLine, error) {
	if f == nil || f.Client == nil {
		return nil, fmt.Errorf("truenas api fetcher client is nil")
	}
	return f.Client.GetAppLogs(ctx, appName, containerID, tailLines)
}

func (f *APIFetcher) DiskTemperatureHistory(ctx context.Context, identifiers []string, duration time.Duration) (map[string][]TimeSeriesPoint, error) {
	if f == nil || f.Client == nil {
		return nil, fmt.Errorf("truenas api fetcher client is nil")
	}
	return f.Client.GetDiskTemperatureHistory(ctx, identifiers, duration)
}

func (f *APIFetcher) SystemMetricHistory(ctx context.Context, duration time.Duration) (*SystemMetricHistory, error) {
	if f == nil || f.Client == nil {
		return nil, fmt.Errorf("truenas api fetcher client is nil")
	}
	return f.Client.GetSystemMetricHistory(ctx, duration)
}

// FixtureFetcher loads snapshots from static fixture data.
type FixtureFetcher struct {
	Snapshot FixtureSnapshot
}

// Fetch implements Fetcher.
func (f *FixtureFetcher) Fetch(context.Context) (*FixtureSnapshot, error) {
	if f == nil {
		return nil, nil
	}
	return copyFixtureSnapshot(&f.Snapshot), nil
}

// Provider converts TrueNAS snapshot data into unified resources.
type Provider struct {
	fetcher      Fetcher
	lastSnapshot *FixtureSnapshot
	mu           sync.Mutex
	now          func() time.Time
}

// NewLiveProvider returns a provider backed by any fetcher implementation.
func NewLiveProvider(fetcher Fetcher) *Provider {
	return &Provider{
		fetcher: fetcher,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

// NewProvider returns a fixture-backed provider.
func NewProvider(fixtures FixtureSnapshot) *Provider {
	if fixtures.CollectedAt.IsZero() {
		fixtures.CollectedAt = time.Now().UTC()
	}
	provider := NewLiveProvider(&FixtureFetcher{Snapshot: fixtures})
	provider.lastSnapshot = copyFixtureSnapshot(&fixtures)
	return provider
}

// NewDefaultProvider returns a provider loaded with the default fixtures.
func NewDefaultProvider() *Provider {
	return NewProvider(DefaultFixtures())
}

// Refresh fetches and caches the latest snapshot.
func (p *Provider) Refresh(ctx context.Context) error {
	if p == nil {
		return fmt.Errorf("truenas provider is nil")
	}
	if p.fetcher == nil {
		return fmt.Errorf("truenas provider fetcher is nil")
	}

	snapshot, err := p.fetcher.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("refresh truenas snapshot: %w", err)
	}
	if snapshot == nil {
		return errNilSnapshot
	}

	p.mu.Lock()
	enrichAppStatsFromPreviousSnapshot(snapshot, p.lastSnapshot)
	p.lastSnapshot = copyFixtureSnapshot(snapshot)
	p.mu.Unlock()
	return nil
}

// ControlApp executes a native start/stop action against a TrueNAS app and
// refreshes the cached snapshot so downstream readers observe canonical state.
func (p *Provider) ControlApp(ctx context.Context, appID, action string) (*FixtureSnapshot, error) {
	if p == nil {
		return nil, fmt.Errorf("truenas provider is nil")
	}
	executor, ok := p.fetcher.(appActionFetcher)
	if !ok {
		return nil, fmt.Errorf("truenas provider fetcher does not support app control")
	}

	switch strings.ToLower(strings.TrimSpace(action)) {
	case "start":
		if err := executor.StartApp(ctx, appID); err != nil {
			return nil, err
		}
	case "stop":
		if err := executor.StopApp(ctx, appID); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported truenas app action %q", action)
	}

	if err := p.Refresh(ctx); err != nil {
		return nil, err
	}
	return p.Snapshot(), nil
}

// ReadAppLogs returns a bounded recent log tail for one TrueNAS app container.
func (p *Provider) ReadAppLogs(ctx context.Context, appID, containerRef string, tailLines int) (*AppLogResult, error) {
	if p == nil {
		return nil, fmt.Errorf("truenas provider is nil")
	}
	reader, ok := p.fetcher.(appReadFetcher)
	if !ok {
		return nil, fmt.Errorf("truenas provider fetcher does not support app log reads")
	}

	snapshot := p.Snapshot()
	app, err := findAppInSnapshot(snapshot, appID)
	if err != nil {
		return nil, err
	}
	container, err := selectAppLogContainer(*app, containerRef)
	if err != nil {
		return nil, err
	}

	lines, err := reader.ReadAppLogs(ctx, appCanonicalID(*app), container.ID, tailLines)
	if err != nil {
		return nil, err
	}

	result := &AppLogResult{
		App:       *app,
		Container: container,
		Lines:     trimAppLogResultLines(lines, tailLines),
		TailLines: tailLines,
	}
	if snapshot != nil {
		result.Host = strings.TrimSpace(snapshot.System.Hostname)
	}
	return result, nil
}

// GetAppConfig returns the current application configuration/runtime shape for
// one TrueNAS app from the provider snapshot.
func (p *Provider) GetAppConfig(_ context.Context, appID string) (*AppConfigResult, error) {
	if p == nil {
		return nil, fmt.Errorf("truenas provider is nil")
	}

	snapshot := p.Snapshot()
	app, err := findAppInSnapshot(snapshot, appID)
	if err != nil {
		return nil, err
	}

	result := &AppConfigResult{App: *app}
	if snapshot != nil {
		result.Host = strings.TrimSpace(snapshot.System.Hostname)
	}
	return result, nil
}

// SystemMetricHistory returns canonical host-chart metrics for the TrueNAS
// appliance keyed by the shared agent metrics resource ID.
func (p *Provider) SystemMetricHistory(ctx context.Context, duration time.Duration) (string, map[string][]TimeSeriesPoint, error) {
	if p == nil {
		return "", nil, fmt.Errorf("truenas provider is nil")
	}
	historyFetcher, ok := p.fetcher.(systemMetricHistoryFetcher)
	if !ok {
		return "", nil, fmt.Errorf("truenas provider fetcher does not support system metric history")
	}

	snapshot := p.Snapshot()
	if snapshot == nil {
		return "", nil, fmt.Errorf("truenas provider has no cached snapshot")
	}
	resourceID := trueNASSystemMetricResourceID(snapshot.System)
	if resourceID == "" {
		return "", nil, fmt.Errorf("truenas provider snapshot is missing system metric resource id")
	}

	nativeHistory, err := historyFetcher.SystemMetricHistory(ctx, duration)
	if err != nil {
		return "", nil, err
	}
	if nativeHistory == nil {
		return resourceID, nil, nil
	}

	metricMap := make(map[string][]TimeSeriesPoint)
	if len(nativeHistory.CPUPercent) > 0 {
		metricMap["cpu"] = cloneTimeSeriesPoints(nativeHistory.CPUPercent)
	}
	if memoryPercent := systemMemoryPercentHistory(nativeHistory, snapshot.System.MemoryTotalBytes); len(memoryPercent) > 0 {
		metricMap["memory"] = memoryPercent
	}
	if len(nativeHistory.NetInRate) > 0 {
		metricMap["netin"] = cloneTimeSeriesPoints(nativeHistory.NetInRate)
	}
	if len(nativeHistory.NetOutRate) > 0 {
		metricMap["netout"] = cloneTimeSeriesPoints(nativeHistory.NetOutRate)
	}
	if len(nativeHistory.DiskReadRate) > 0 {
		metricMap["diskread"] = cloneTimeSeriesPoints(nativeHistory.DiskReadRate)
	}
	if len(nativeHistory.DiskWriteRate) > 0 {
		metricMap["diskwrite"] = cloneTimeSeriesPoints(nativeHistory.DiskWriteRate)
	}
	if len(metricMap) == 0 {
		return resourceID, nil, nil
	}
	return resourceID, metricMap, nil
}

// PhysicalDiskTemperatureHistory returns canonical physical-disk temperature
// series keyed by the shared physical-disk metrics resource IDs.
func (p *Provider) PhysicalDiskTemperatureHistory(ctx context.Context, duration time.Duration) (map[string][]TimeSeriesPoint, error) {
	if p == nil {
		return nil, fmt.Errorf("truenas provider is nil")
	}
	historyFetcher, ok := p.fetcher.(physicalDiskHistoryFetcher)
	if !ok {
		return nil, fmt.Errorf("truenas provider fetcher does not support physical disk history")
	}

	snapshot := p.Snapshot()
	if snapshot == nil {
		return nil, fmt.Errorf("truenas provider has no cached snapshot")
	}

	identifiers := make([]string, 0, len(snapshot.Disks))
	metricIDsByIdentifier := make(map[string]string, len(snapshot.Disks)*3)
	for _, disk := range snapshot.Disks {
		metricID := trueNASDiskMetricResourceID(disk)
		if metricID == "" {
			continue
		}
		if name := strings.TrimSpace(disk.Name); name != "" {
			identifiers = append(identifiers, name)
		}
		for _, key := range trueNASDiskHistoryLookupKeys(disk) {
			if _, exists := metricIDsByIdentifier[key]; !exists {
				metricIDsByIdentifier[key] = metricID
			}
		}
	}
	identifiers = dedupeStrings(identifiers)
	if len(identifiers) == 0 {
		return nil, nil
	}

	nativeHistory, err := historyFetcher.DiskTemperatureHistory(ctx, identifiers, duration)
	if err != nil {
		return nil, err
	}
	if len(nativeHistory) == 0 {
		return nil, nil
	}

	historyByMetricID := make(map[string][]TimeSeriesPoint, len(nativeHistory))
	for identifier, points := range nativeHistory {
		metricID := metricIDsByIdentifier[strings.TrimSpace(identifier)]
		if metricID == "" || len(points) == 0 {
			continue
		}
		copied := make([]TimeSeriesPoint, len(points))
		copy(copied, points)
		historyByMetricID[metricID] = copied
	}
	if len(historyByMetricID) == 0 {
		return nil, nil
	}
	return historyByMetricID, nil
}

// Close releases resources held by the active fetcher, if supported.
func (p *Provider) Close() {
	if p == nil || p.fetcher == nil {
		return
	}
	if closer, ok := p.fetcher.(fetcherCloser); ok {
		closer.Close()
	}
}

// Snapshot returns a defensive copy of the most recent cached snapshot.
// Callers should treat the returned snapshot as immutable.
func (p *Provider) Snapshot() *FixtureSnapshot {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	snapshot := copyFixtureSnapshot(p.lastSnapshot)
	p.mu.Unlock()
	return snapshot
}

// FixtureRecords projects a TrueNAS fixture snapshot into canonical unified
// resource ingest records without consulting the runtime feature flag.
func FixtureRecords(snapshot FixtureSnapshot) []unifiedresources.IngestRecord {
	return truenasRecordsFromSnapshot(&snapshot, nil)
}

// Records returns unified records if the feature flag is enabled.
func (p *Provider) Records() []unifiedresources.IngestRecord {
	if p == nil || !IsFeatureEnabled() {
		return nil
	}

	return truenasRecordsFromSnapshot(p.Snapshot(), p.now)
}

func truenasRecordsFromSnapshot(snapshot *FixtureSnapshot, now func() time.Time) []unifiedresources.IngestRecord {
	if snapshot == nil {
		return nil
	}

	collectedAt := snapshot.CollectedAt
	if collectedAt.IsZero() {
		collectedAt = snapshot.System.CollectedAt
	}
	if collectedAt.IsZero() {
		if now != nil {
			collectedAt = now().UTC()
		} else {
			collectedAt = time.Now().UTC()
		}
	}
	systemSourceID := systemSourceID(snapshot.System.Hostname)
	systemAssessment := assessSystemStorage(snapshot)
	systemRisk := unifiedresources.StorageRiskFromAssessment(systemAssessment)
	_, protectionReduced, rebuildInProgress, protectionSummary, rebuildSummary := unifiedresources.StorageRiskSemantics(systemRisk)
	systemIncidents, poolIncidents, diskIncidents := buildIncidentAssignments(snapshot)
	records := make([]unifiedresources.IngestRecord, 0, 1+len(snapshot.Pools)+len(snapshot.Datasets)+len(snapshot.Apps)+len(snapshot.Disks))

	totalCapacity, totalUsed := aggregatePoolUsage(snapshot.Pools)
	records = append(records, unifiedresources.IngestRecord{
		SourceID: systemSourceID,
		Resource: unifiedresources.Resource{
			Type:      unifiedresources.ResourceTypeAgent,
			Name:      strings.TrimSpace(snapshot.System.Hostname),
			Status:    systemStatus(snapshot.System, systemRisk, systemIncidents),
			LastSeen:  collectedAt,
			UpdatedAt: collectedAt,
			Metrics:   metricsFromTrueNASSystem(snapshot.System, totalCapacity, totalUsed),
			Agent:     agentDataFromTrueNASSystem(snapshot.System, systemRisk, protectionReduced, protectionSummary, rebuildInProgress, rebuildSummary),
			TrueNAS: &unifiedresources.TrueNASData{
				Hostname:              strings.TrimSpace(snapshot.System.Hostname),
				Version:               snapshot.System.Version,
				UptimeSeconds:         snapshot.System.UptimeSeconds,
				StorageRisk:           systemRisk,
				StorageRiskSummary:    unifiedresources.StorageRiskSummary(systemRisk),
				StoragePostureSummary: unifiedresources.StorageRiskSummary(systemRisk),
				ProtectionReduced:     protectionReduced,
				ProtectionSummary:     protectionSummary,
				RebuildInProgress:     rebuildInProgress,
				RebuildSummary:        rebuildSummary,
			},
			Tags: []string{
				"truenas",
				snapshot.System.Version,
				"zfs",
			},
			Incidents: systemIncidents,
		},
		Identity: unifiedresources.ResourceIdentity{
			MachineID: snapshot.System.MachineID,
			Hostnames: []string{snapshot.System.Hostname},
		},
	})

	for _, pool := range snapshot.Pools {
		assessment := assessPool(pool)
		risk := unifiedresources.StorageRiskFromAssessment(assessment)
		incidents := poolIncidents[strings.TrimSpace(pool.Name)]
		records = append(records, unifiedresources.IngestRecord{
			SourceID:       poolSourceID(pool.Name),
			ParentSourceID: systemSourceID,
			Resource: unifiedresources.Resource{
				Type:      unifiedresources.ResourceTypeStorage,
				Name:      pool.Name,
				Status:    unifiedresources.IncidentsStatus(statusFromPool(pool), incidents),
				LastSeen:  collectedAt,
				UpdatedAt: collectedAt,
				Metrics: &unifiedresources.ResourceMetrics{
					Disk: diskMetric(pool.TotalBytes, pool.UsedBytes),
				},
				Storage: &unifiedresources.StorageMeta{
					Type:         "zfs-pool",
					IsZFS:        true,
					Platform:     "truenas",
					Topology:     "pool",
					Protection:   "zfs",
					Risk:         risk,
					ZFSPoolState: strings.ToUpper(strings.TrimSpace(pool.Status)),
				},
				Tags: []string{
					"truenas",
					"pool",
					"zfs",
					"health:" + strings.ToLower(strings.TrimSpace(pool.Status)),
				},
				Incidents: incidents,
			},
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: []string{
					snapshot.System.Hostname,
					pool.Name,
				},
			},
		})
	}

	for _, dataset := range snapshot.Datasets {
		parentPool := strings.TrimSpace(dataset.Pool)
		if parentPool == "" {
			parentPool = parentPoolFromDataset(dataset.Name)
		}
		totalBytes := dataset.UsedBytes + dataset.AvailBytes
		records = append(records, unifiedresources.IngestRecord{
			SourceID:       datasetSourceID(dataset.Name),
			ParentSourceID: poolSourceID(parentPool),
			Resource: unifiedresources.Resource{
				Type:      unifiedresources.ResourceTypeStorage,
				Name:      dataset.Name,
				Status:    statusFromDataset(dataset),
				LastSeen:  collectedAt,
				UpdatedAt: collectedAt,
				Metrics: &unifiedresources.ResourceMetrics{
					Disk: diskMetric(totalBytes, dataset.UsedBytes),
				},
				Storage: &unifiedresources.StorageMeta{
					Type:       "zfs-dataset",
					IsZFS:      true,
					Platform:   "truenas",
					Topology:   "dataset",
					Protection: "zfs",
				},
				Tags: []string{
					"truenas",
					"dataset",
					"zfs",
					datasetStateTag(dataset),
				},
			},
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: []string{
					snapshot.System.Hostname,
					dataset.Name,
				},
			},
		})
	}

	for _, app := range snapshot.Apps {
		metrics := metricsFromTrueNASApp(app)
		dockerMeta := &unifiedresources.DockerData{
			ContainerID:    appCanonicalID(app),
			Hostname:       strings.TrimSpace(snapshot.System.Hostname),
			DisplayName:    appDisplayName(app),
			Image:          primaryAppImage(app),
			Runtime:        "docker",
			ContainerState: appContainerState(app),
			Ports:          dockerPortsFromTrueNASApp(app),
			Mounts:         dockerMountsFromTrueNASApp(app),
			Networks:       dockerNetworksFromTrueNASApp(app),
			Labels: map[string]string{
				"truenas.app_id": strings.TrimSpace(app.ID),
			},
		}
		if app.Stats != nil {
			dockerMeta.NetInRate = app.Stats.NetInRate
			dockerMeta.NetOutRate = app.Stats.NetOutRate
			dockerMeta.DiskReadRate = app.Stats.DiskReadRate
			dockerMeta.DiskWriteRate = app.Stats.DiskWriteRate
		}
		if strings.TrimSpace(app.Version) != "" {
			dockerMeta.Labels["truenas.version"] = strings.TrimSpace(app.Version)
		}
		if strings.TrimSpace(app.HumanVersion) != "" {
			dockerMeta.Labels["truenas.human_version"] = strings.TrimSpace(app.HumanVersion)
		}
		if app.CustomApp {
			dockerMeta.Labels["truenas.custom_app"] = "true"
		}

		records = append(records, unifiedresources.IngestRecord{
			SourceID:       appSourceID(app.ID),
			ParentSourceID: systemSourceID,
			Resource: unifiedresources.Resource{
				Type:         unifiedresources.ResourceTypeAppContainer,
				Technology:   "docker",
				Name:         appDisplayName(app),
				Status:       statusFromApp(app),
				LastSeen:     collectedAt,
				UpdatedAt:    collectedAt,
				Metrics:      metrics,
				Docker:       dockerMeta,
				Capabilities: truenasAppCapabilities(),
				Tags:         appTags(app),
			},
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: dedupeStrings([]string{appDisplayName(app)}),
			},
		})
	}

	for _, disk := range snapshot.Disks {
		assessment := assessDisk(disk)
		incidents := diskIncidents[strings.TrimSpace(disk.Name)]
		diskIdentity := unifiedresources.ResourceIdentity{
			Hostnames: []string{snapshot.System.Hostname},
		}
		if disk.Serial != "" {
			diskIdentity.MachineID = disk.Serial
		}
		records = append(records, unifiedresources.IngestRecord{
			SourceID:       diskSourceID(disk.Name),
			ParentSourceID: poolSourceID(disk.Pool),
			Resource: unifiedresources.Resource{
				Type:      unifiedresources.ResourceTypePhysicalDisk,
				Name:      disk.Name,
				Status:    unifiedresources.IncidentsStatus(unifiedresources.PhysicalDiskStatus(disk.Model, healthFromDisk(disk), assessment), incidents),
				LastSeen:  collectedAt,
				UpdatedAt: collectedAt,
				PhysicalDisk: &unifiedresources.PhysicalDiskMeta{
					DevPath:              "/dev/" + disk.Name,
					Model:                disk.Model,
					Serial:               disk.Serial,
					DiskType:             disk.Transport,
					SizeBytes:            disk.SizeBytes,
					Health:               healthFromDisk(disk),
					Temperature:          disk.Temperature,
					TemperatureAggregate: temperatureAggregateMetaFromTrueNASDisk(disk),
					Wearout:              -1,
					RPM:                  rpmFromDisk(disk),
					Risk:                 unifiedresources.PhysicalDiskRiskFromAssessmentAndIncidents(assessment, incidents),
				},
				Tags:      []string{"truenas", "disk", disk.Transport},
				Incidents: incidents,
			},
			Identity: diskIdentity,
		})
	}

	return records
}

func metricsFromTrueNASApp(app App) *unifiedresources.ResourceMetrics {
	if app.Stats == nil {
		return nil
	}

	stats := app.Stats
	metrics := &unifiedresources.ResourceMetrics{
		CPU: &unifiedresources.MetricValue{
			Value:   stats.CPUPercent,
			Percent: stats.CPUPercent,
			Unit:    "percent",
			Source:  unifiedresources.SourceTrueNAS,
		},
		NetIn: &unifiedresources.MetricValue{
			Value:  stats.NetInRate,
			Unit:   "bytes/s",
			Source: unifiedresources.SourceTrueNAS,
		},
		NetOut: &unifiedresources.MetricValue{
			Value:  stats.NetOutRate,
			Unit:   "bytes/s",
			Source: unifiedresources.SourceTrueNAS,
		},
		DiskRead: &unifiedresources.MetricValue{
			Value:  stats.DiskReadRate,
			Unit:   "bytes/s",
			Source: unifiedresources.SourceTrueNAS,
		},
		DiskWrite: &unifiedresources.MetricValue{
			Value:  stats.DiskWriteRate,
			Unit:   "bytes/s",
			Source: unifiedresources.SourceTrueNAS,
		},
	}

	memoryUsed := stats.MemoryBytes
	metrics.Memory = &unifiedresources.MetricValue{
		Used:   &memoryUsed,
		Unit:   "bytes",
		Source: unifiedresources.SourceTrueNAS,
	}
	return metrics
}

func metricsFromTrueNASSystem(system SystemInfo, totalCapacity, totalUsed int64) *unifiedresources.ResourceMetrics {
	hasRealtimeTelemetry := !system.CollectedAt.IsZero() || system.IntervalSeconds > 0
	metrics := &unifiedresources.ResourceMetrics{
		Disk: diskMetric(totalCapacity, totalUsed),
	}

	if hasRealtimeTelemetry {
		metrics.CPU = &unifiedresources.MetricValue{
			Value:   system.CPUPercent,
			Percent: system.CPUPercent,
			Unit:    "percent",
			Source:  unifiedresources.SourceTrueNAS,
		}
	}

	if system.MemoryTotalBytes > 0 {
		used := system.MemoryTotalBytes - system.MemoryAvailableBytes
		if used < 0 {
			used = 0
		}
		memory := &unifiedresources.MetricValue{
			Used:   &used,
			Total:  &system.MemoryTotalBytes,
			Unit:   "bytes",
			Source: unifiedresources.SourceTrueNAS,
		}
		if system.MemoryTotalBytes > 0 {
			memory.Percent = (float64(used) / float64(system.MemoryTotalBytes)) * 100
			memory.Value = memory.Percent
		}
		metrics.Memory = memory
	}

	if hasRealtimeTelemetry {
		metrics.NetIn = &unifiedresources.MetricValue{
			Value:  system.NetInRate,
			Unit:   "bytes/s",
			Source: unifiedresources.SourceTrueNAS,
		}
		metrics.NetOut = &unifiedresources.MetricValue{
			Value:  system.NetOutRate,
			Unit:   "bytes/s",
			Source: unifiedresources.SourceTrueNAS,
		}
		metrics.DiskRead = &unifiedresources.MetricValue{
			Value:  system.DiskReadRate,
			Unit:   "bytes/s",
			Source: unifiedresources.SourceTrueNAS,
		}
		metrics.DiskWrite = &unifiedresources.MetricValue{
			Value:  system.DiskWriteRate,
			Unit:   "bytes/s",
			Source: unifiedresources.SourceTrueNAS,
		}
	}

	return metrics
}

func agentDataFromTrueNASSystem(system SystemInfo, storageRisk *unifiedresources.StorageRisk, protectionReduced bool, protectionSummary string, rebuildInProgress bool, rebuildSummary string) *unifiedresources.AgentData {
	agent := &unifiedresources.AgentData{
		Hostname:              strings.TrimSpace(system.Hostname),
		MachineID:             strings.TrimSpace(system.MachineID),
		Platform:              "truenas",
		OSName:                "TrueNAS",
		OSVersion:             strings.TrimSpace(system.Version),
		CPUCount:              system.CPUCount,
		UptimeSeconds:         system.UptimeSeconds,
		IntervalSeconds:       system.IntervalSeconds,
		StorageRisk:           storageRisk,
		StorageRiskSummary:    unifiedresources.StorageRiskSummary(storageRisk),
		StoragePostureSummary: unifiedresources.StorageRiskSummary(storageRisk),
		ProtectionReduced:     protectionReduced,
		ProtectionSummary:     protectionSummary,
		RebuildInProgress:     rebuildInProgress,
		RebuildSummary:        rebuildSummary,
		NetInRate:             system.NetInRate,
		NetOutRate:            system.NetOutRate,
		DiskReadRate:          system.DiskReadRate,
		DiskWriteRate:         system.DiskWriteRate,
	}

	if system.MemoryTotalBytes > 0 {
		used := system.MemoryTotalBytes - system.MemoryAvailableBytes
		if used < 0 {
			used = 0
		}
		free := system.MemoryAvailableBytes
		if free < 0 {
			free = 0
		}
		agent.Memory = &unifiedresources.AgentMemoryMeta{
			Total: system.MemoryTotalBytes,
			Used:  used,
			Free:  free,
		}
	}
	if temperature := maxTrueNASSystemTemperature(system); temperature != nil {
		agent.Temperature = temperature
	}
	if sensors := sensorMetaFromTrueNASSystem(system); sensors != nil {
		agent.Sensors = sensors
	}

	return agent
}

func maxTrueNASSystemTemperature(system SystemInfo) *float64 {
	if len(system.TemperatureCelsius) == 0 {
		return nil
	}
	if value, ok := system.TemperatureCelsius["cpu_package"]; ok && value > 0 {
		canonical := value
		return &canonical
	}

	var best float64
	found := false
	for key, value := range system.TemperatureCelsius {
		key = strings.TrimSpace(strings.ToLower(key))
		if value <= 0 || !strings.HasPrefix(key, "cpu") {
			continue
		}
		if !found || value > best {
			best = value
			found = true
		}
	}
	if !found {
		return nil
	}
	return &best
}

func sensorMetaFromTrueNASSystem(system SystemInfo) *unifiedresources.HostSensorMeta {
	if len(system.TemperatureCelsius) == 0 {
		return nil
	}

	sensors := &unifiedresources.HostSensorMeta{
		TemperatureCelsius: make(map[string]float64, len(system.TemperatureCelsius)),
	}
	for key, value := range system.TemperatureCelsius {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		sensors.TemperatureCelsius[key] = value
	}
	if len(sensors.TemperatureCelsius) == 0 {
		return nil
	}
	return sensors
}

func enrichAppStatsFromPreviousSnapshot(current *FixtureSnapshot, previous *FixtureSnapshot) {
	if current == nil || previous == nil || len(current.Apps) == 0 || len(previous.Apps) == 0 {
		return
	}

	previousByApp := make(map[string]*AppStats, len(previous.Apps))
	for i := range previous.Apps {
		if previous.Apps[i].Stats == nil {
			continue
		}
		key := normalizeAppStatsKey(previous.Apps[i].ID)
		if key == "" {
			key = normalizeAppStatsKey(previous.Apps[i].Name)
		}
		if key == "" {
			continue
		}
		previousByApp[key] = previous.Apps[i].Stats
	}

	for i := range current.Apps {
		stats := current.Apps[i].Stats
		if stats == nil {
			continue
		}
		key := normalizeAppStatsKey(current.Apps[i].ID)
		if key == "" {
			key = normalizeAppStatsKey(current.Apps[i].Name)
		}
		if key == "" {
			continue
		}
		previousStats, ok := previousByApp[key]
		if !ok || previousStats == nil {
			continue
		}
		deltaSeconds := stats.CollectedAt.Sub(previousStats.CollectedAt).Seconds()
		if deltaSeconds <= 0 {
			continue
		}
		if stats.BlockReadBytes >= previousStats.BlockReadBytes {
			stats.DiskReadRate = float64(stats.BlockReadBytes-previousStats.BlockReadBytes) / deltaSeconds
		}
		if stats.BlockWriteBytes >= previousStats.BlockWriteBytes {
			stats.DiskWriteRate = float64(stats.BlockWriteBytes-previousStats.BlockWriteBytes) / deltaSeconds
		}
	}
}

func buildIncidentAssignments(snapshot *FixtureSnapshot) ([]unifiedresources.ResourceIncident, map[string][]unifiedresources.ResourceIncident, map[string][]unifiedresources.ResourceIncident) {
	systemIncidents := make([]unifiedresources.ResourceIncident, 0)
	poolIncidents := make(map[string][]unifiedresources.ResourceIncident)
	diskIncidents := make(map[string][]unifiedresources.ResourceIncident)
	if snapshot == nil {
		return systemIncidents, poolIncidents, diskIncidents
	}

	diskPools := make(map[string]string, len(snapshot.Disks))
	for _, disk := range snapshot.Disks {
		diskName := strings.TrimSpace(disk.Name)
		poolName := strings.TrimSpace(disk.Pool)
		if diskName == "" || poolName == "" {
			continue
		}
		diskPools[diskName] = poolName
	}

	for _, alert := range snapshot.Alerts {
		if alert.Dismissed {
			continue
		}
		incident, ok := incidentFromAlert(alert)
		if !ok {
			continue
		}
		systemIncidents = append(systemIncidents, incident)

		if poolName := poolNameFromAlert(alert); poolName != "" {
			poolIncidents[poolName] = append(poolIncidents[poolName], incident)
		}

		if diskName := diskNameFromAlert(alert); diskName != "" {
			diskIncidents[diskName] = append(diskIncidents[diskName], incident)
			if poolName := diskPools[diskName]; poolName != "" {
				poolIncidents[poolName] = append(poolIncidents[poolName], incident)
			}
		}
	}

	return systemIncidents, poolIncidents, diskIncidents
}

func assessSystemStorage(snapshot *FixtureSnapshot) storagehealth.Assessment {
	if snapshot == nil {
		return storagehealth.Assessment{Level: storagehealth.RiskHealthy}
	}

	assessments := make([]storagehealth.Assessment, 0, len(snapshot.Pools)+len(snapshot.Disks))
	for _, pool := range snapshot.Pools {
		assessments = append(assessments, assessPool(pool))
	}
	for _, disk := range snapshot.Disks {
		assessments = append(assessments, assessDisk(disk))
	}
	return storagehealth.SummarizeAssessments(assessments...)
}

func incidentFromAlert(alert Alert) (unifiedresources.ResourceIncident, bool) {
	severity, ok := severityFromAlertLevel(alert.Level)
	if !ok {
		return unifiedresources.ResourceIncident{}, false
	}
	return unifiedresources.ResourceIncident{
		Provider:  "truenas",
		NativeID:  strings.TrimSpace(alert.ID),
		Code:      incidentCodeFromAlert(alert),
		Severity:  severity,
		Source:    strings.TrimSpace(alert.Source),
		Summary:   strings.TrimSpace(alert.Message),
		StartedAt: alert.Datetime,
	}, true
}

func severityFromAlertLevel(level string) (storagehealth.RiskLevel, bool) {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "CRITICAL", "ERROR", "ALERT":
		return storagehealth.RiskCritical, true
	case "WARNING", "WARN":
		return storagehealth.RiskWarning, true
	case "INFO", "NOTICE":
		return storagehealth.RiskMonitor, true
	default:
		return storagehealth.RiskHealthy, false
	}
}

func incidentCodeFromAlert(alert Alert) string {
	source := strings.ToLower(strings.TrimSpace(alert.Source))
	switch source {
	case "volumestatus":
		return "truenas_volume_status"
	case "smart":
		return "truenas_smart"
	case "scrub":
		return "truenas_scrub"
	default:
		if source == "" {
			return "truenas_alert"
		}
		return "truenas_" + strings.ReplaceAll(source, " ", "_")
	}
}

func poolNameFromAlert(alert Alert) string {
	message := strings.TrimSpace(alert.Message)
	if message == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(message), "pool ") {
		return ""
	}
	rest := strings.TrimSpace(message[len("Pool "):])
	if rest == "" {
		return ""
	}
	end := strings.IndexAny(rest, " :")
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}

func diskNameFromAlert(alert Alert) string {
	message := strings.TrimSpace(alert.Message)
	if message == "" {
		return ""
	}
	marker := "Device /dev/"
	idx := strings.Index(message, marker)
	if idx < 0 {
		return ""
	}
	rest := message[idx+len(marker):]
	if rest == "" {
		return ""
	}
	end := strings.IndexAny(rest, " :.,")
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}

func assessPool(pool Pool) storagehealth.Assessment {
	return storagehealth.AssessZFSPool(models.ZFSPool{
		Name:  strings.TrimSpace(pool.Name),
		State: strings.ToUpper(strings.TrimSpace(pool.Status)),
	})
}

func assessDisk(disk Disk) storagehealth.Assessment {
	sampleAssessment := storagehealth.AssessSample(storagehealth.Sample{
		Model:       strings.TrimSpace(disk.Model),
		Health:      healthFromDisk(disk),
		Temperature: disk.Temperature,
		Wearout:     -1,
	})

	stateUpper := strings.ToUpper(strings.TrimSpace(disk.Status))
	stateAssessment := storagehealth.Assessment{Level: storagehealth.RiskHealthy}
	switch stateUpper {
	case "DEGRADED":
		stateAssessment = storagehealth.Assessment{
			Level: storagehealth.RiskWarning,
			Reasons: []storagehealth.Reason{{
				Code:     "truenas_disk_state",
				Severity: storagehealth.RiskWarning,
				Summary:  fmt.Sprintf("TrueNAS disk %s is DEGRADED", strings.TrimSpace(disk.Name)),
			}},
		}
	case "FAULTED", "OFFLINE", "REMOVED", "UNAVAIL":
		stateAssessment = storagehealth.Assessment{
			Level: storagehealth.RiskCritical,
			Reasons: []storagehealth.Reason{{
				Code:     "truenas_disk_state",
				Severity: storagehealth.RiskCritical,
				Summary:  fmt.Sprintf("TrueNAS disk %s is %s", strings.TrimSpace(disk.Name), stateUpper),
			}},
		}
	}

	return storagehealth.SummarizeAssessments(sampleAssessment, stateAssessment)
}

func statusFromSystem(system SystemInfo) unifiedresources.ResourceStatus {
	if system.Healthy {
		return unifiedresources.StatusOnline
	}
	return unifiedresources.StatusWarning
}

func systemStatus(system SystemInfo, storageRisk *unifiedresources.StorageRisk, incidents []unifiedresources.ResourceIncident) unifiedresources.ResourceStatus {
	status := statusFromSystem(system)
	if storageRisk == nil {
		return unifiedresources.IncidentsStatus(status, incidents)
	}
	return unifiedresources.IncidentsStatus(unifiedresources.StorageStatus(status, storageRisk), incidents)
}

func statusFromPool(pool Pool) unifiedresources.ResourceStatus {
	switch strings.ToUpper(strings.TrimSpace(pool.Status)) {
	case "ONLINE", "HEALTHY":
		return unifiedresources.StatusOnline
	case "DEGRADED":
		return unifiedresources.StatusWarning
	case "FAULTED", "OFFLINE", "REMOVED", "UNAVAIL":
		return unifiedresources.StatusOffline
	default:
		return unifiedresources.StatusUnknown
	}
}

func statusFromDataset(dataset Dataset) unifiedresources.ResourceStatus {
	if !dataset.Mounted {
		return unifiedresources.StatusOffline
	}
	if dataset.ReadOnly {
		return unifiedresources.StatusWarning
	}
	return unifiedresources.StatusOnline
}

func datasetStateTag(dataset Dataset) string {
	if !dataset.Mounted {
		return "state:unmounted"
	}
	if dataset.ReadOnly {
		return "state:readonly"
	}
	return "state:mounted"
}

func diskMetric(total, used int64) *unifiedresources.MetricValue {
	if total <= 0 {
		return nil
	}
	totalCopy := total
	usedCopy := used
	percent := (float64(used) / float64(total)) * 100
	return &unifiedresources.MetricValue{
		Total:   &totalCopy,
		Used:    &usedCopy,
		Value:   percent,
		Percent: percent,
		Unit:    "bytes",
	}
}

func aggregatePoolUsage(pools []Pool) (int64, int64) {
	var total int64
	var used int64
	for _, pool := range pools {
		total += pool.TotalBytes
		used += pool.UsedBytes
	}
	return total, used
}

func systemSourceID(hostname string) string {
	return "system:" + strings.TrimSpace(hostname)
}

func poolSourceID(pool string) string {
	return "pool:" + strings.TrimSpace(pool)
}

func datasetSourceID(dataset string) string {
	return "dataset:" + strings.TrimSpace(dataset)
}

func appSourceID(id string) string {
	return "app:" + strings.TrimSpace(id)
}

func diskSourceID(name string) string {
	return "disk:" + strings.TrimSpace(name)
}

func appDisplayName(app App) string {
	if name := strings.TrimSpace(app.Name); name != "" {
		return name
	}
	return strings.TrimSpace(app.ID)
}

func appCanonicalID(app App) string {
	if id := strings.TrimSpace(app.ID); id != "" {
		return id
	}
	return appDisplayName(app)
}

func findAppInSnapshot(snapshot *FixtureSnapshot, appID string) (*App, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("truenas snapshot is unavailable")
	}
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("truenas app id is required")
	}
	for i := range snapshot.Apps {
		app := &snapshot.Apps[i]
		if strings.EqualFold(appCanonicalID(*app), appID) || strings.EqualFold(strings.TrimSpace(app.Name), appID) {
			return app, nil
		}
	}
	return nil, fmt.Errorf("truenas app %q was not found", appID)
}

func selectAppLogContainer(app App, containerRef string) (AppContainer, error) {
	if len(app.Containers) == 0 {
		return AppContainer{}, fmt.Errorf("truenas app %q does not expose any runtime containers for log reads", appDisplayName(app))
	}
	containerRef = strings.TrimSpace(containerRef)
	if containerRef != "" {
		for _, container := range app.Containers {
			if strings.EqualFold(strings.TrimSpace(container.ID), containerRef) ||
				strings.EqualFold(strings.TrimSpace(container.ServiceName), containerRef) {
				return container, nil
			}
		}
		return AppContainer{}, fmt.Errorf("truenas app %q does not have container %q. Available containers: %s", appDisplayName(app), containerRef, availableAppLogContainers(app.Containers))
	}
	if len(app.Containers) == 1 {
		return app.Containers[0], nil
	}

	canonicalAppID := normalizeAppStatsKey(appCanonicalID(app))
	for _, container := range app.Containers {
		if normalizeAppStatsKey(container.ServiceName) == canonicalAppID {
			return container, nil
		}
	}
	for _, container := range app.Containers {
		if strings.EqualFold(strings.TrimSpace(container.State), "running") {
			return container, nil
		}
	}
	return AppContainer{}, fmt.Errorf("truenas app %q has multiple containers. Specify container using one of: %s", appDisplayName(app), availableAppLogContainers(app.Containers))
}

func availableAppLogContainers(containers []AppContainer) string {
	if len(containers) == 0 {
		return ""
	}
	options := make([]string, 0, len(containers))
	for _, container := range containers {
		label := strings.TrimSpace(container.ServiceName)
		if label == "" {
			label = strings.TrimSpace(container.ID)
		}
		if label == "" {
			continue
		}
		if id := strings.TrimSpace(container.ID); id != "" && !strings.EqualFold(id, label) {
			label = fmt.Sprintf("%s (%s)", label, id)
		}
		options = append(options, label)
	}
	if len(options) == 0 {
		return ""
	}
	return strings.Join(options, ", ")
}

func trimAppLogResultLines(lines []AppLogLine, tailLines int) []AppLogLine {
	if len(lines) == 0 {
		return nil
	}
	if tailLines > 0 && len(lines) > tailLines {
		lines = lines[len(lines)-tailLines:]
	}
	out := make([]AppLogLine, len(lines))
	copy(out, lines)
	return out
}

func primaryAppImage(app App) string {
	if len(app.Images) > 0 {
		if image := strings.TrimSpace(app.Images[0]); image != "" {
			return image
		}
	}
	for _, container := range app.Containers {
		if image := strings.TrimSpace(container.Image); image != "" {
			return image
		}
	}
	return ""
}

func appContainerState(app App) string {
	if len(app.Containers) > 0 {
		state := strings.ToLower(strings.TrimSpace(app.Containers[0].State))
		if state != "" {
			return state
		}
	}
	return strings.ToLower(strings.TrimSpace(app.State))
}

func statusFromApp(app App) unifiedresources.ResourceStatus {
	switch strings.ToUpper(strings.TrimSpace(app.State)) {
	case "RUNNING":
		return unifiedresources.StatusOnline
	case "DEPLOYING", "STOPPING", "CRASHED":
		return unifiedresources.StatusWarning
	case "STOPPED":
		return unifiedresources.StatusOffline
	default:
		return unifiedresources.StatusUnknown
	}
}

func truenasAppCapabilities() []unifiedresources.ResourceCapability {
	return []unifiedresources.ResourceCapability{
		{
			Name:                 "start",
			Type:                 unifiedresources.CapabilityTypeCommon,
			Description:          "Start the TrueNAS-managed application.",
			MinimumApprovalLevel: unifiedresources.ApprovalAdmin,
			Platform:             "truenas",
			InternalHandler:      "app.start",
		},
		{
			Name:                 "stop",
			Type:                 unifiedresources.CapabilityTypeCommon,
			Description:          "Stop the TrueNAS-managed application.",
			MinimumApprovalLevel: unifiedresources.ApprovalAdmin,
			Platform:             "truenas",
			InternalHandler:      "app.stop",
		},
		{
			Name:                 "restart",
			Type:                 unifiedresources.CapabilityTypeCommon,
			Description:          "Restart the TrueNAS-managed application using canonical stop/start semantics.",
			MinimumApprovalLevel: unifiedresources.ApprovalAdmin,
			Platform:             "truenas",
			InternalHandler:      "app.restart",
		},
	}
}

func appTags(app App) []string {
	tags := []string{"truenas", "app"}
	if state := strings.ToLower(strings.TrimSpace(app.State)); state != "" {
		tags = append(tags, "state:"+state)
	}
	if app.CustomApp {
		tags = append(tags, "custom-app")
	}
	return dedupeStrings(tags)
}

func dockerPortsFromTrueNASApp(app App) []unifiedresources.DockerPortMeta {
	if len(app.UsedPorts) == 0 {
		return nil
	}
	ports := make([]unifiedresources.DockerPortMeta, 0, len(app.UsedPorts))
	for _, port := range app.UsedPorts {
		if port.ContainerPort == 0 && len(port.HostPorts) == 0 {
			continue
		}
		if len(port.HostPorts) == 0 {
			ports = append(ports, unifiedresources.DockerPortMeta{
				PrivatePort: port.ContainerPort,
				Protocol:    strings.ToLower(strings.TrimSpace(port.Protocol)),
			})
			continue
		}
		for _, hostPort := range port.HostPorts {
			ports = append(ports, unifiedresources.DockerPortMeta{
				PrivatePort: port.ContainerPort,
				PublicPort:  hostPort.HostPort,
				Protocol:    strings.ToLower(strings.TrimSpace(port.Protocol)),
				IP:          strings.TrimSpace(hostPort.HostIP),
			})
		}
	}
	if len(ports) == 0 {
		return nil
	}
	return ports
}

func dockerMountsFromTrueNASApp(app App) []unifiedresources.DockerMountMeta {
	if len(app.Volumes) == 0 {
		return nil
	}
	mounts := make([]unifiedresources.DockerMountMeta, 0, len(app.Volumes))
	for _, volume := range app.Volumes {
		destination := strings.TrimSpace(volume.Destination)
		source := strings.TrimSpace(volume.Source)
		if destination == "" && source == "" {
			continue
		}
		mounts = append(mounts, unifiedresources.DockerMountMeta{
			Type:        strings.TrimSpace(volume.Type),
			Source:      source,
			Destination: destination,
			Mode:        strings.TrimSpace(volume.Mode),
			RW:          !strings.Contains(strings.ToLower(strings.TrimSpace(volume.Mode)), "ro"),
		})
	}
	if len(mounts) == 0 {
		return nil
	}
	return mounts
}

func dockerNetworksFromTrueNASApp(app App) []unifiedresources.DockerNetworkMeta {
	if len(app.Networks) == 0 {
		return nil
	}
	networks := make([]unifiedresources.DockerNetworkMeta, 0, len(app.Networks))
	for _, network := range app.Networks {
		name := strings.TrimSpace(network.Name)
		if name == "" {
			name = strings.TrimSpace(network.ID)
		}
		if name == "" {
			continue
		}
		networks = append(networks, unifiedresources.DockerNetworkMeta{Name: name})
	}
	if len(networks) == 0 {
		return nil
	}
	return networks
}

func statusFromDisk(disk Disk) unifiedresources.ResourceStatus {
	switch strings.ToUpper(strings.TrimSpace(disk.Status)) {
	case "ONLINE":
		return unifiedresources.StatusOnline
	case "DEGRADED":
		return unifiedresources.StatusWarning
	case "FAULTED", "OFFLINE", "REMOVED", "UNAVAIL":
		return unifiedresources.StatusOffline
	default:
		return unifiedresources.StatusUnknown
	}
}

func healthFromDisk(disk Disk) string {
	switch strings.ToUpper(strings.TrimSpace(disk.Status)) {
	case "ONLINE":
		return "PASSED"
	case "DEGRADED":
		return "UNKNOWN"
	default:
		return "FAILED"
	}
}

func rpmFromDisk(disk Disk) int {
	if disk.Rotational {
		return 7200
	}
	return 0
}

func temperatureAggregateMetaFromTrueNASDisk(disk Disk) *unifiedresources.TemperatureAggregateMeta {
	aggregate := disk.TemperatureAggregate
	if aggregate.WindowDays <= 0 && aggregate.MinCelsius <= 0 && aggregate.AvgCelsius <= 0 && aggregate.MaxCelsius <= 0 {
		return nil
	}
	return &unifiedresources.TemperatureAggregateMeta{
		WindowDays: aggregate.WindowDays,
		MinCelsius: aggregate.MinCelsius,
		AvgCelsius: aggregate.AvgCelsius,
		MaxCelsius: aggregate.MaxCelsius,
	}
}

func cloneTimeSeriesPoints(points []TimeSeriesPoint) []TimeSeriesPoint {
	if len(points) == 0 {
		return nil
	}
	cloned := make([]TimeSeriesPoint, len(points))
	copy(cloned, points)
	return cloned
}

func systemMemoryPercentHistory(history *SystemMetricHistory, fallbackTotalBytes int64) []TimeSeriesPoint {
	if history == nil {
		return nil
	}
	if len(history.MemoryPercent) > 0 {
		return cloneTimeSeriesPoints(history.MemoryPercent)
	}

	totalBytes := float64(fallbackTotalBytes)
	if totalBytes <= 0 {
		if len(history.MemoryTotalBytes) > 0 {
			totalBytes = history.MemoryTotalBytes[len(history.MemoryTotalBytes)-1].Value
		}
	}
	if totalBytes <= 0 {
		return nil
	}

	if len(history.MemoryUsedBytes) > 0 {
		points := make([]TimeSeriesPoint, 0, len(history.MemoryUsedBytes))
		for _, point := range history.MemoryUsedBytes {
			points = append(points, TimeSeriesPoint{
				Timestamp: point.Timestamp,
				Value:     (point.Value / totalBytes) * 100,
			})
		}
		return points
	}
	if len(history.MemoryAvailableBytes) > 0 {
		points := make([]TimeSeriesPoint, 0, len(history.MemoryAvailableBytes))
		for _, point := range history.MemoryAvailableBytes {
			points = append(points, TimeSeriesPoint{
				Timestamp: point.Timestamp,
				Value:     ((totalBytes - point.Value) / totalBytes) * 100,
			})
		}
		return points
	}
	return nil
}

func trueNASSystemMetricResourceID(system SystemInfo) string {
	return strings.TrimSpace(system.Hostname)
}

func trueNASDiskHistoryLookupKeys(disk Disk) []string {
	return dedupeStrings([]string{
		strings.TrimSpace(disk.Name),
		strings.TrimSpace(disk.ID),
		strings.TrimSpace(disk.Serial),
	})
}

func trueNASDiskMetricResourceID(disk Disk) string {
	devPath := ""
	if name := strings.TrimSpace(disk.Name); name != "" {
		devPath = "/dev/" + name
	}
	meta := &unifiedresources.PhysicalDiskMeta{
		DevPath:   devPath,
		Serial:    strings.TrimSpace(disk.Serial),
		DiskType:  strings.TrimSpace(disk.Transport),
		SizeBytes: disk.SizeBytes,
	}
	fallback := strings.TrimSpace(disk.ID)
	if fallback == "" {
		fallback = strings.TrimSpace(disk.Name)
	}
	return unifiedresources.PhysicalDiskMetaMetricID(meta, fallback)
}

func parentPoolFromDataset(datasetName string) string {
	parts := strings.SplitN(strings.TrimSpace(datasetName), "/", 2)
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func parseFeatureEnabled(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return true
	}
	return parseBool(raw)
}

func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func copyFixtureSnapshot(snapshot *FixtureSnapshot) *FixtureSnapshot {
	if snapshot == nil {
		return nil
	}

	copied := *snapshot
	copied.System = cloneSystemInfo(snapshot.System)
	copied.Pools = append([]Pool(nil), snapshot.Pools...)
	copied.Datasets = append([]Dataset(nil), snapshot.Datasets...)
	copied.Disks = append([]Disk(nil), snapshot.Disks...)
	copied.Alerts = append([]Alert(nil), snapshot.Alerts...)
	copied.Apps = cloneApps(snapshot.Apps)
	copied.ZFSSnapshots = append([]ZFSSnapshot(nil), snapshot.ZFSSnapshots...)
	copied.ReplicationTasks = append([]ReplicationTask(nil), snapshot.ReplicationTasks...)
	return &copied
}

func cloneSystemInfo(system SystemInfo) SystemInfo {
	cloned := system
	if len(system.TemperatureCelsius) > 0 {
		cloned.TemperatureCelsius = make(map[string]float64, len(system.TemperatureCelsius))
		for key, value := range system.TemperatureCelsius {
			cloned.TemperatureCelsius[key] = value
		}
	}
	return cloned
}

func cloneApps(apps []App) []App {
	if len(apps) == 0 {
		return nil
	}
	out := make([]App, len(apps))
	for i := range apps {
		out[i] = apps[i]
		out[i].UsedHostIPs = append([]string(nil), apps[i].UsedHostIPs...)
		out[i].UsedPorts = cloneAppPorts(apps[i].UsedPorts)
		out[i].Containers = cloneAppContainers(apps[i].Containers)
		out[i].Volumes = append([]AppVolume(nil), apps[i].Volumes...)
		out[i].Images = append([]string(nil), apps[i].Images...)
		out[i].Networks = cloneAppNetworks(apps[i].Networks)
		if apps[i].Stats != nil {
			stats := *apps[i].Stats
			stats.Interfaces = append([]AppInterfaceStats(nil), apps[i].Stats.Interfaces...)
			out[i].Stats = &stats
		}
	}
	return out
}

func cloneAppPorts(ports []AppPort) []AppPort {
	if len(ports) == 0 {
		return nil
	}
	out := make([]AppPort, len(ports))
	for i := range ports {
		out[i] = ports[i]
		out[i].HostPorts = append([]AppHostPort(nil), ports[i].HostPorts...)
	}
	return out
}

func cloneAppContainers(containers []AppContainer) []AppContainer {
	if len(containers) == 0 {
		return nil
	}
	out := make([]AppContainer, len(containers))
	for i := range containers {
		out[i] = containers[i]
		out[i].PortConfig = cloneAppPorts(containers[i].PortConfig)
		out[i].VolumeMounts = append([]AppVolume(nil), containers[i].VolumeMounts...)
	}
	return out
}

func cloneAppNetworks(networks []AppNetwork) []AppNetwork {
	if len(networks) == 0 {
		return nil
	}
	out := make([]AppNetwork, len(networks))
	for i := range networks {
		out[i] = networks[i]
		if len(networks[i].Labels) > 0 {
			out[i].Labels = make(map[string]string, len(networks[i].Labels))
			for key, value := range networks[i].Labels {
				out[i].Labels[key] = value
			}
		}
	}
	return out
}
