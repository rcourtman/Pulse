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

type transportStatusFetcher interface {
	TransportStatus() TransportStatus
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

// TransportStatus returns the underlying client's non-secret connection-local
// transport diagnostics.
func (f *APIFetcher) TransportStatus() TransportStatus {
	if f == nil || f.Client == nil {
		return TransportStatus{Mode: TransportUnknown}
	}
	return f.Client.TransportStatus()
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
	connectionID string
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

// NewLiveProviderForConnection returns a live provider whose resources are
// keyed by the configured connection rather than the hostname the TrueNAS API
// reports. Two systems that report the same hostname (a DR box restored from
// the primary's config, the default "truenas" name) must stay distinct
// resources (#1573, #1575).
func NewLiveProviderForConnection(fetcher Fetcher, connectionID string) *Provider {
	provider := NewLiveProvider(fetcher)
	provider.connectionID = strings.TrimSpace(connectionID)
	return provider
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

// TransportStatus returns non-secret diagnostics when the provider is backed
// by the live API client. Fixture providers remain in the negotiating state.
func (p *Provider) TransportStatus() TransportStatus {
	if p == nil {
		return TransportStatus{Mode: TransportUnknown}
	}
	if source, ok := p.fetcher.(transportStatusFetcher); ok {
		return source.TransportStatus()
	}
	return TransportStatus{Mode: TransportUnknown}
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
	resourceID := trueNASSystemMetricResourceID(p.connectionID, snapshot.System)
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
// Fixture snapshots carry no connection, so they keep the hostname-scoped
// source IDs that mock mode and the mock metric identities derive.
func FixtureRecords(snapshot FixtureSnapshot) []unifiedresources.IngestRecord {
	return truenasRecordsFromSnapshot(&snapshot, "", nil)
}

// Records returns unified records if the feature flag is enabled.
func (p *Provider) Records() []unifiedresources.IngestRecord {
	if p == nil || !IsFeatureEnabled() {
		return nil
	}

	return truenasRecordsFromSnapshot(p.Snapshot(), p.connectionID, p.now)
}

// RecordsFromSnapshot projects an already-classified defensive snapshot using
// the provider's connection-scoped identity.
func (p *Provider) RecordsFromSnapshot(snapshot *FixtureSnapshot) []unifiedresources.IngestRecord {
	if p == nil || !IsFeatureEnabled() {
		return nil
	}
	return truenasRecordsFromSnapshot(snapshot, p.connectionID, p.now)
}

func truenasRecordsFromSnapshot(snapshot *FixtureSnapshot, connectionID string, now func() time.Time) []unifiedresources.IngestRecord {
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
	systemSourceID := systemSourceID(connectionID, snapshot.System.Hostname)
	// legacySystemSourceID is the retired hostname-keyed derivation. It is
	// only used to compute superseded canonical IDs so operator-owned rows
	// (never-auto-remediate, maintenance windows, action audits) written
	// under the old keys follow the resource onto its connection-scoped ID.
	legacySystemSourceID := ""
	if hostname := strings.TrimSpace(snapshot.System.Hostname); hostname != "" {
		if legacy := "system:" + hostname; legacy != systemSourceID {
			legacySystemSourceID = legacy
		}
	}
	supersededChildIDs := func(resourceType unifiedresources.ResourceType, legacyChildSourceID string) []string {
		if legacySystemSourceID == "" {
			return nil
		}
		return []string{unifiedresources.SourceSpecificID(resourceType, unifiedresources.SourceTrueNAS, legacyChildSourceID)}
	}
	var systemSupersededIDs []string
	if legacySystemSourceID != "" {
		// The retired client fell back to the reported hostname when the DMI
		// serial was empty, and the registry minted the system's canonical ID
		// from that machine key. Reproduce that ladder to name the old ID.
		legacyMachineID := strings.TrimSpace(snapshot.System.MachineID)
		if legacyMachineID == "" {
			legacyMachineID = strings.TrimSpace(snapshot.System.Hostname)
		}
		systemSupersededIDs = []string{
			unifiedresources.MachineIdentityCanonicalID(unifiedresources.ResourceTypeAgent, legacyMachineID),
			unifiedresources.SourceSpecificID(unifiedresources.ResourceTypeAgent, unifiedresources.SourceTrueNAS, legacySystemSourceID),
		}
	}
	systemAssessment := assessSystemStorage(snapshot)
	systemRisk := unifiedresources.StorageRiskFromAssessment(systemAssessment)
	_, protectionReduced, rebuildInProgress, protectionSummary, rebuildSummary := unifiedresources.StorageRiskSemantics(systemRisk)
	incidentAssignments := buildIncidentAssignments(snapshot, collectedAt)
	records := make([]unifiedresources.IngestRecord, 0, 1+len(snapshot.Pools)+len(snapshot.Datasets)+len(snapshot.Apps)+len(snapshot.VMs)+len(snapshot.Shares)+len(snapshot.Disks))

	totalCapacity, totalUsed := aggregatePoolUsage(snapshot.Pools)
	systemAgent := agentDataFromTrueNASSystem(connectionID, snapshot.System, snapshot.Disks, systemRisk, protectionReduced, protectionSummary, rebuildInProgress, rebuildSummary)
	records = append(records, unifiedresources.IngestRecord{
		SourceID:               systemSourceID,
		SupersededCanonicalIDs: systemSupersededIDs,
		Resource: unifiedresources.Resource{
			Type:        unifiedresources.ResourceTypeAgent,
			Name:        strings.TrimSpace(snapshot.System.Hostname),
			Status:      systemStatus(snapshot.System, systemRisk, incidentAssignments.System),
			LastSeen:    collectedAt,
			UpdatedAt:   collectedAt,
			Metrics:     metricsFromTrueNASSystem(snapshot.System, totalCapacity, totalUsed),
			Uptime:      snapshot.System.UptimeSeconds,
			Temperature: systemAgent.Temperature,
			Agent:       systemAgent,
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
				Services:              trueNASServicesFromServices(snapshot.Services),
			},
			Tags: []string{
				"truenas",
				snapshot.System.Version,
				"zfs",
			},
			Incidents: incidentAssignments.System,
		},
		// The system's identity deliberately carries no machine key: the
		// TrueNAS DMI serial is shared by DR clones and can be vendor
		// placeholder garbage, so the identity matcher would re-merge the
		// systems this record's connection-scoped source ID keeps apart
		// (#1573, #1575). The configured connection is the identity.
		Identity: unifiedresources.ResourceIdentity{
			Hostnames: []string{snapshot.System.Hostname},
		},
	})

	for _, pool := range snapshot.Pools {
		assessment := assessPool(pool)
		risk := unifiedresources.StorageRiskFromAssessment(assessment)
		incidents := incidentAssignments.Pools[strings.TrimSpace(pool.Name)]
		zfsPool := zfsPoolFromPool(pool)
		poolSourceID := scopedPoolSourceID(systemSourceID, pool.Name)
		records = append(records, unifiedresources.IngestRecord{
			SourceID:               poolSourceID,
			ParentSourceID:         systemSourceID,
			SupersededCanonicalIDs: supersededChildIDs(unifiedresources.ResourceTypeStorage, scopedPoolSourceID(legacySystemSourceID, pool.Name)),
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
					Type:              "zfs-pool",
					IsZFS:             true,
					Platform:          "truenas",
					Topology:          poolTopologyLabel(pool),
					Protection:        "zfs",
					Risk:              risk,
					RiskSummary:       unifiedresources.StorageRiskSummary(risk),
					PoolHealth:        poolHealthFromTrueNASPool(pool, assessment, collectedAt),
					ZFSPool:           &zfsPool,
					ZFSPoolState:      strings.ToUpper(strings.TrimSpace(pool.Status)),
					ZFSReadErrors:     pool.ReadErrors,
					ZFSWriteErrors:    pool.WriteErrors,
					ZFSChecksumErrors: pool.ChecksumErrors,
				},
				Tags:      poolTags(pool),
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
		incidents := incidentAssignments.Datasets[strings.TrimSpace(dataset.Name)]
		records = append(records, unifiedresources.IngestRecord{
			SourceID:               scopedDatasetSourceID(systemSourceID, dataset.Name),
			ParentSourceID:         scopedPoolSourceID(systemSourceID, parentPool),
			SupersededCanonicalIDs: supersededChildIDs(unifiedresources.ResourceTypeStorage, scopedDatasetSourceID(legacySystemSourceID, dataset.Name)),
			Resource: unifiedresources.Resource{
				Type:      unifiedresources.ResourceTypeStorage,
				Name:      dataset.Name,
				Status:    unifiedresources.IncidentsStatus(statusFromDataset(dataset), incidents),
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
				Incidents: incidents,
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
		metrics := metricsFromTrueNASApp(app, snapshot.System.MemoryTotalBytes)
		incidents := incidentAssignments.Apps[appIncidentKey(app)]
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
			SourceID:               scopedAppSourceID(systemSourceID, app),
			ParentSourceID:         systemSourceID,
			SupersededCanonicalIDs: supersededChildIDs(unifiedresources.ResourceTypeAppContainer, scopedAppSourceID(legacySystemSourceID, app)),
			Resource: unifiedresources.Resource{
				Type:       unifiedresources.ResourceTypeAppContainer,
				Technology: "docker",
				Name:       appDisplayName(app),
				Status:     unifiedresources.IncidentsStatus(statusFromApp(app), incidents),
				LastSeen:   collectedAt,
				UpdatedAt:  collectedAt,
				Metrics:    metrics,
				Docker:     dockerMeta,
				TrueNAS: &unifiedresources.TrueNASData{
					Hostname: strings.TrimSpace(snapshot.System.Hostname),
					App:      trueNASAppDataFromApp(app),
				},
				Capabilities: truenasAppCapabilities(),
				Tags:         appTags(app),
				Incidents:    incidents,
			},
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: dedupeStrings([]string{appDisplayName(app)}),
			},
		})
	}

	for _, vm := range snapshot.VMs {
		records = append(records, unifiedresources.IngestRecord{
			SourceID:               scopedVirtualMachineSourceID(systemSourceID, vm),
			ParentSourceID:         systemSourceID,
			SupersededCanonicalIDs: supersededChildIDs(unifiedresources.ResourceTypeVM, scopedVirtualMachineSourceID(legacySystemSourceID, vm)),
			Resource: unifiedresources.Resource{
				Type:      unifiedresources.ResourceTypeVM,
				Name:      virtualMachineDisplayName(vm),
				Status:    statusFromVirtualMachine(vm),
				LastSeen:  collectedAt,
				UpdatedAt: collectedAt,
				TrueNAS: &unifiedresources.TrueNASData{
					Hostname: strings.TrimSpace(snapshot.System.Hostname),
					VM:       trueNASVMDataFromVirtualMachine(vm),
				},
				Tags: virtualMachineTags(vm),
			},
			Identity: unifiedresources.ResourceIdentity{
				MachineID: strings.TrimSpace(vm.UUID),
				Hostnames: dedupeStrings([]string{
					strings.TrimSpace(snapshot.System.Hostname),
					virtualMachineDisplayName(vm),
				}),
			},
		})
	}

	for _, share := range snapshot.Shares {
		parentSourceID := systemSourceID
		if dataset := strings.TrimSpace(share.Dataset); dataset != "" {
			parentSourceID = scopedDatasetSourceID(systemSourceID, dataset)
		} else if pool := poolFromSharePath(share.Path); pool != "" {
			parentSourceID = scopedPoolSourceID(systemSourceID, pool)
		}
		records = append(records, unifiedresources.IngestRecord{
			SourceID:               scopedNetworkShareSourceID(systemSourceID, share),
			ParentSourceID:         parentSourceID,
			SupersededCanonicalIDs: supersededChildIDs(unifiedresources.ResourceTypeNetworkShare, scopedNetworkShareSourceID(legacySystemSourceID, share)),
			Resource: unifiedresources.Resource{
				Type:      unifiedresources.ResourceTypeNetworkShare,
				Name:      networkShareDisplayName(share),
				Status:    statusFromNetworkShare(share),
				LastSeen:  collectedAt,
				UpdatedAt: collectedAt,
				TrueNAS: &unifiedresources.TrueNASData{
					Hostname: strings.TrimSpace(snapshot.System.Hostname),
					Share:    trueNASShareDataFromNetworkShare(share),
				},
				Tags: networkShareTags(share),
			},
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: dedupeStrings([]string{
					strings.TrimSpace(snapshot.System.Hostname),
					networkShareDisplayName(share),
				}),
			},
		})
	}

	for _, disk := range snapshot.Disks {
		assessment := assessDisk(disk)
		incidents := incidentAssignments.Disks[strings.TrimSpace(disk.Name)]
		diskIdentity := unifiedresources.ResourceIdentity{
			Hostnames: []string{snapshot.System.Hostname},
		}
		if disk.Serial != "" {
			diskIdentity.MachineID = disk.Serial
		}
		parentSourceID := systemSourceID
		if pool := strings.TrimSpace(disk.Pool); pool != "" {
			parentSourceID = scopedPoolSourceID(systemSourceID, pool)
		}
		// Disks with a serial mint identity-keyed canonical IDs that do not
		// depend on the source ID, so only serial-less disks re-key when the
		// system scope moves to the connection.
		var diskSupersededIDs []string
		if disk.Serial == "" {
			diskSupersededIDs = supersededChildIDs(unifiedresources.ResourceTypePhysicalDisk, scopedDiskSourceID(legacySystemSourceID, disk.Name))
		}
		records = append(records, unifiedresources.IngestRecord{
			SourceID:               scopedDiskSourceID(systemSourceID, disk.Name),
			ParentSourceID:         parentSourceID,
			SupersededCanonicalIDs: diskSupersededIDs,
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
					StorageGroup:         strings.TrimSpace(disk.Pool),
					StorageState:         normalizedDiskStatus(disk),
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

func metricsFromTrueNASApp(app App, hostMemoryTotalBytes int64) *unifiedresources.ResourceMetrics {
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
	if hostMemoryTotalBytes > 0 {
		total := hostMemoryTotalBytes
		metrics.Memory.Total = &total
		metrics.Memory.Percent = (float64(memoryUsed) / float64(hostMemoryTotalBytes)) * 100
		metrics.Memory.Value = metrics.Memory.Percent
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

func agentDataFromTrueNASSystem(connectionID string, system SystemInfo, disks []Disk, storageRisk *unifiedresources.StorageRisk, protectionReduced bool, protectionSummary string, rebuildInProgress bool, rebuildSummary string) *unifiedresources.AgentData {
	agent := &unifiedresources.AgentData{
		AgentID:               trueNASSystemMetricResourceID(connectionID, system),
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
	if sensors := sensorMetaFromTrueNASSystem(system, disks); sensors != nil {
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

func sensorMetaFromTrueNASSystem(system SystemInfo, disks []Disk) *unifiedresources.HostSensorMeta {
	sensors := &unifiedresources.HostSensorMeta{}

	if len(system.TemperatureCelsius) > 0 {
		sensors.TemperatureCelsius = make(map[string]float64, len(system.TemperatureCelsius))
		for key, value := range system.TemperatureCelsius {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			sensors.TemperatureCelsius[key] = value
		}
		if len(sensors.TemperatureCelsius) == 0 {
			sensors.TemperatureCelsius = nil
		}
	}

	// Surface API-reported disk temperatures as SMART sensor entries so the
	// host Thermals card lists disks next to the CPU sensors — API-backed
	// TrueNAS systems have no pulse-agent SMART sweep to supply them (#1573).
	// Disks without a reading are skipped rather than rendered as 0°C.
	for _, disk := range disks {
		if disk.Temperature <= 0 {
			continue
		}
		device := strings.TrimSpace(disk.Name)
		if device == "" {
			device = strings.TrimSpace(disk.ID)
		}
		if device == "" {
			continue
		}
		sensors.SMART = append(sensors.SMART, unifiedresources.HostSMARTMeta{
			Device:      device,
			Model:       strings.TrimSpace(disk.Model),
			Serial:      strings.TrimSpace(disk.Serial),
			Type:        strings.TrimSpace(disk.Transport),
			SizeBytes:   disk.SizeBytes,
			Temperature: disk.Temperature,
			Health:      healthFromDisk(disk),
			Pool:        strings.TrimSpace(disk.Pool),
		})
	}

	if len(sensors.TemperatureCelsius) == 0 && len(sensors.SMART) == 0 {
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

type trueNASIncidentAssignments struct {
	System   []unifiedresources.ResourceIncident
	Pools    map[string][]unifiedresources.ResourceIncident
	Datasets map[string][]unifiedresources.ResourceIncident
	Disks    map[string][]unifiedresources.ResourceIncident
	Apps     map[string][]unifiedresources.ResourceIncident
}

type poolIncidentProjection struct {
	Incident unifiedresources.ResourceIncident
	Disk     string
}

func buildIncidentAssignments(snapshot *FixtureSnapshot, observedAt time.Time) trueNASIncidentAssignments {
	assignments := trueNASIncidentAssignments{
		Pools:    make(map[string][]unifiedresources.ResourceIncident),
		Datasets: make(map[string][]unifiedresources.ResourceIncident),
		Disks:    make(map[string][]unifiedresources.ResourceIncident),
		Apps:     make(map[string][]unifiedresources.ResourceIncident),
	}
	if snapshot == nil {
		return assignments
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

	nativePoolSignals := make(map[string]map[string]struct{}, len(snapshot.Alerts))
	for _, alert := range snapshot.Alerts {
		if alert.Dismissed {
			continue
		}
		incident, ok := incidentFromAlert(alert)
		if !ok {
			continue
		}
		assignments.System = append(assignments.System, incident)

		if poolName := poolNameFromAlert(alert); poolName != "" {
			assignments.Pools[poolName] = append(assignments.Pools[poolName], incident)
			if nativePoolSignals[poolName] == nil {
				nativePoolSignals[poolName] = make(map[string]struct{})
			}
			for _, code := range canonicalSignalsCoveredByNativeAlert(incident.Code) {
				nativePoolSignals[poolName][code] = struct{}{}
			}
		}

		if diskName := diskNameFromAlert(alert); diskName != "" {
			assignments.Disks[diskName] = append(assignments.Disks[diskName], incident)
			if poolName := diskPools[diskName]; poolName != "" {
				assignments.Pools[poolName] = append(assignments.Pools[poolName], incident)
			}
		}
	}

	for _, pool := range snapshot.Pools {
		poolName := strings.TrimSpace(pool.Name)
		if poolName == "" {
			continue
		}
		for _, projection := range incidentsFromPoolHealth(pool, observedAt) {
			if _, covered := nativePoolSignals[poolName][projection.Incident.Code]; covered {
				continue
			}
			assignments.System = append(assignments.System, projection.Incident)
			assignments.Pools[poolName] = append(assignments.Pools[poolName], projection.Incident)
			if projection.Disk != "" {
				assignments.Disks[projection.Disk] = append(assignments.Disks[projection.Disk], projection.Incident)
			}
		}
	}

	for _, dataset := range snapshot.Datasets {
		if incident, ok := incidentFromDatasetState(dataset, observedAt); ok {
			assignments.Datasets[strings.TrimSpace(dataset.Name)] = append(assignments.Datasets[strings.TrimSpace(dataset.Name)], incident)
		}
	}
	for _, app := range snapshot.Apps {
		assignments.Apps[appIncidentKey(app)] = append(assignments.Apps[appIncidentKey(app)], incidentsFromAppState(app, observedAt)...)
	}

	return assignments
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
		Provider:                      "truenas",
		NativeID:                      strings.TrimSpace(alert.ID),
		Code:                          incidentCodeFromAlert(alert),
		Severity:                      severity,
		Source:                        strings.TrimSpace(alert.Source),
		Summary:                       strings.TrimSpace(alert.Message),
		StartedAt:                     alert.Datetime,
		RecoveryConfirmationsRequired: 2,
	}, true
}

func incidentFromPoolStatus(pool Pool, observedAt time.Time) (unifiedresources.ResourceIncident, bool) {
	for _, projection := range incidentsFromPoolHealth(pool, observedAt) {
		if projection.Incident.Code == "zfs_pool_state" {
			return projection.Incident, true
		}
	}
	return unifiedresources.ResourceIncident{}, false
}

func incidentsFromPoolHealth(pool Pool, observedAt time.Time) []poolIncidentProjection {
	poolName := strings.TrimSpace(pool.Name)
	if poolName == "" {
		return nil
	}
	poolIdentity := firstNonEmptyString(pool.GUID, pool.ID, poolName)
	source := "pool.query"
	if pool.IsBoot {
		source = "boot.get_state"
	}
	makeIncident := func(nativeID, code string, severity storagehealth.RiskLevel, summary string) unifiedresources.ResourceIncident {
		return unifiedresources.ResourceIncident{
			Provider:                      "truenas",
			NativeID:                      nativeID,
			Code:                          code,
			Severity:                      severity,
			Source:                        source,
			Summary:                       summary,
			StartedAt:                     observedAt,
			ConfirmationsRequired:         2,
			RecoveryConfirmationsRequired: 2,
		}
	}

	var out []poolIncidentProjection
	state := strings.ToUpper(strings.TrimSpace(pool.Status))
	switch state {
	case "DEGRADED":
		out = append(out, poolIncidentProjection{Incident: makeIncident(
			"pool:"+poolName+":state",
			"zfs_pool_state",
			storagehealth.RiskWarning,
			fmt.Sprintf("ZFS pool %s is DEGRADED", poolName),
		)})
	case "FAULTED", "OFFLINE", "REMOVED", "UNAVAIL", "SUSPENDED":
		out = append(out, poolIncidentProjection{Incident: makeIncident(
			"pool:"+poolName+":state",
			"zfs_pool_state",
			storagehealth.RiskCritical,
			fmt.Sprintf("ZFS pool %s is %s", poolName, state),
		)})
	}

	hasVDevErrors := false
	for _, vdev := range pool.VDevs {
		if vdev.ReadErrors > 0 || vdev.WriteErrors > 0 || vdev.ChecksumErrors > 0 {
			hasVDevErrors = true
			break
		}
	}
	if !hasVDevErrors && (pool.ReadErrors > 0 || pool.WriteErrors > 0 || pool.ChecksumErrors > 0) {
		out = append(out, poolIncidentProjection{Incident: makeIncident(
			"pool:"+poolIdentity,
			"zfs_pool_errors",
			storagehealth.RiskWarning,
			fmt.Sprintf("ZFS pool %s reports read=%d write=%d checksum=%d errors", poolName, pool.ReadErrors, pool.WriteErrors, pool.ChecksumErrors),
		)})
	}

	if pool.Scan != nil {
		function := strings.ToUpper(strings.TrimSpace(pool.Scan.Function))
		scanState := strings.ToUpper(strings.TrimSpace(pool.Scan.State))
		switch {
		case pool.Scan.Errors > 0:
			out = append(out, poolIncidentProjection{Incident: makeIncident(
				"pool:"+poolIdentity+":scan",
				"zfs_scan_errors",
				storagehealth.RiskCritical,
				fmt.Sprintf("ZFS pool %s %s reports %d error(s)", poolName, strings.ToLower(firstNonEmptyString(function, "scan")), pool.Scan.Errors),
			)})
		case scanState == "FAILED":
			out = append(out, poolIncidentProjection{Incident: makeIncident(
				"pool:"+poolIdentity+":scan",
				"zfs_scan_failed",
				storagehealth.RiskCritical,
				fmt.Sprintf("ZFS pool %s %s failed", poolName, strings.ToLower(firstNonEmptyString(function, "scan"))),
			)})
		case function == "RESILVER" && poolScanIsActive(scanState):
			out = append(out, poolIncidentProjection{Incident: makeIncident(
				"pool:"+poolIdentity+":scan",
				"zfs_resilver_active",
				storagehealth.RiskWarning,
				poolScanSummary(pool),
			)})
		case function == "SCRUB" && poolScanIsActive(scanState):
			out = append(out, poolIncidentProjection{Incident: makeIncident(
				"pool:"+poolIdentity+":scan",
				"zfs_scrub_active",
				storagehealth.RiskMonitor,
				poolScanSummary(pool),
			)})
		}
	}

	for _, vdev := range pool.VDevs {
		state := strings.ToUpper(strings.TrimSpace(vdev.Status))
		deviceIdentity := firstNonEmptyString(vdev.GUID, vdev.Disk, vdev.Device, vdev.Path, vdev.ID, vdev.Name)
		if deviceIdentity == "" {
			continue
		}
		displayName := firstNonEmptyString(vdev.Disk, vdev.Device, vdev.Path, vdev.Name, deviceIdentity)
		nativeID := "pool:" + poolIdentity + ":vdev:" + deviceIdentity
		diskName := strings.TrimSpace(vdev.Disk)
		if vdev.Missing {
			out = append(out, poolIncidentProjection{
				Incident: makeIncident(nativeID, "zfs_device_missing", storagehealth.RiskCritical, fmt.Sprintf("ZFS device %s is reported missing by pool %s topology", displayName, poolName)),
				Disk:     diskName,
			})
		} else {
			switch state {
			case "DEGRADED":
				out = append(out, poolIncidentProjection{
					Incident: makeIncident(nativeID, "zfs_device_state", storagehealth.RiskWarning, fmt.Sprintf("ZFS device %s in pool %s is DEGRADED", displayName, poolName)),
					Disk:     diskName,
				})
			case "FAULTED", "FAILED", "OFFLINE", "REMOVED", "UNAVAIL":
				out = append(out, poolIncidentProjection{
					Incident: makeIncident(nativeID, "zfs_device_state", storagehealth.RiskCritical, fmt.Sprintf("ZFS device %s in pool %s is %s", displayName, poolName, state)),
					Disk:     diskName,
				})
			}
		}
		if vdev.ReadErrors > 0 || vdev.WriteErrors > 0 || vdev.ChecksumErrors > 0 {
			out = append(out, poolIncidentProjection{
				Incident: makeIncident(nativeID, "zfs_device_errors", storagehealth.RiskWarning, fmt.Sprintf("ZFS device %s reports read=%d write=%d checksum=%d errors", displayName, vdev.ReadErrors, vdev.WriteErrors, vdev.ChecksumErrors)),
				Disk:     diskName,
			})
		}
	}
	return out
}

func canonicalSignalsCoveredByNativeAlert(code string) []string {
	switch strings.TrimSpace(code) {
	case "truenas_volume_status":
		return []string{"zfs_pool_state", "zfs_device_state", "zfs_device_missing"}
	case "truenas_scrub":
		return []string{"zfs_scan_errors", "zfs_scan_failed"}
	default:
		return nil
	}
}

func incidentFromDatasetState(dataset Dataset, observedAt time.Time) (unifiedresources.ResourceIncident, bool) {
	name := strings.TrimSpace(dataset.Name)
	if name == "" {
		return unifiedresources.ResourceIncident{}, false
	}
	code := ""
	summary := ""
	switch {
	case dataset.Locked:
		code = "zfs_dataset_locked"
		summary = fmt.Sprintf("ZFS dataset %s is locked and unavailable", name)
	case !dataset.Mounted:
		code = "zfs_dataset_unmounted"
		summary = fmt.Sprintf("ZFS dataset %s is not mounted", name)
	default:
		return unifiedresources.ResourceIncident{}, false
	}
	return unifiedresources.ResourceIncident{
		Provider:                      "truenas",
		NativeID:                      "dataset:" + firstNonEmptyString(dataset.ID, name),
		Code:                          code,
		Severity:                      storagehealth.RiskWarning,
		Source:                        "pool.dataset.query",
		Summary:                       summary,
		StartedAt:                     observedAt,
		ConfirmationsRequired:         2,
		RecoveryConfirmationsRequired: 2,
	}, true
}

func incidentsFromAppState(app App, observedAt time.Time) []unifiedresources.ResourceIncident {
	appID := appIncidentKey(app)
	if appID == "" {
		return nil
	}
	makeIncident := func(nativeID, code string, severity storagehealth.RiskLevel, summary string) unifiedresources.ResourceIncident {
		return unifiedresources.ResourceIncident{
			Provider:                      "truenas",
			NativeID:                      nativeID,
			Code:                          code,
			Severity:                      severity,
			Source:                        "app.query",
			Summary:                       summary,
			StartedAt:                     observedAt,
			ConfirmationsRequired:         2,
			RecoveryConfirmationsRequired: 2,
		}
	}
	name := appDisplayName(app)
	var out []unifiedresources.ResourceIncident
	switch strings.ToUpper(strings.TrimSpace(app.State)) {
	case "CRASHED":
		out = append(out, makeIncident("app:"+appID, "truenas_app_crashed", storagehealth.RiskCritical, fmt.Sprintf("TrueNAS app %s is crashed", name)))
	case "STOPPED":
		out = append(out, makeIncident("app:"+appID, "truenas_app_stopped", storagehealth.RiskWarning, fmt.Sprintf("TrueNAS app %s is stopped", name)))
	}
	if !strings.EqualFold(strings.TrimSpace(app.State), "RUNNING") {
		return out
	}
	for _, container := range app.Containers {
		state := strings.ToUpper(strings.TrimSpace(container.State))
		if state != "CRASHED" && state != "EXITED" {
			continue
		}
		containerID := firstNonEmptyString(container.ID, container.ServiceName, container.Image)
		if containerID == "" {
			continue
		}
		containerName := firstNonEmptyString(container.ServiceName, container.ID, container.Image)
		out = append(out, makeIncident(
			"app:"+appID+":container:"+containerID,
			"truenas_app_container_failed",
			storagehealth.RiskCritical,
			fmt.Sprintf("Container %s in TrueNAS app %s is %s", containerName, name, state),
		))
	}
	return out
}

func appIncidentKey(app App) string {
	return firstNonEmptyString(strings.TrimSpace(app.ID), strings.TrimSpace(app.Name))
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
	return storagehealth.AssessZFSPool(zfsPoolFromPool(pool))
}

func zfsPoolFromPool(pool Pool) models.ZFSPool {
	devices := make([]models.ZFSDevice, 0, len(pool.VDevs))
	for _, vdev := range pool.VDevs {
		devices = append(devices, models.ZFSDevice{
			Name:           firstNonEmptyString(vdev.Name, vdev.Disk, vdev.Device, vdev.Path, vdev.ID),
			Type:           strings.ToLower(strings.TrimSpace(vdev.Type)),
			Role:           strings.ToLower(strings.TrimSpace(vdev.Role)),
			Parent:         strings.TrimSpace(vdev.ParentID),
			GUID:           strings.TrimSpace(vdev.GUID),
			Disk:           strings.TrimSpace(vdev.Disk),
			Path:           firstNonEmptyString(strings.TrimSpace(vdev.Path), devicePath(vdev.Device)),
			State:          strings.ToUpper(strings.TrimSpace(vdev.Status)),
			ReadErrors:     vdev.ReadErrors,
			WriteErrors:    vdev.WriteErrors,
			ChecksumErrors: vdev.ChecksumErrors,
			Missing:        vdev.Missing,
			Message:        strings.TrimSpace(vdev.Message),
		})
	}

	var scanDetails *models.ZFSScan
	scanSummary := ""
	if pool.Scan != nil {
		scanDetails = &models.ZFSScan{
			Function:              strings.ToUpper(strings.TrimSpace(pool.Scan.Function)),
			State:                 strings.ToUpper(strings.TrimSpace(pool.Scan.State)),
			Percentage:            pool.Scan.Percentage,
			Errors:                pool.Scan.Errors,
			BytesExamined:         pool.Scan.BytesExamined,
			BytesToProcess:        pool.Scan.BytesToProcess,
			TotalSecondsRemaining: pool.Scan.TotalSecondsRemaining,
			StartedAt:             pool.Scan.StartedAt,
			EndedAt:               pool.Scan.EndedAt,
		}
		scanSummary = poolScanSummary(pool)
	}

	return models.ZFSPool{
		Name:           strings.TrimSpace(pool.Name),
		State:          strings.ToUpper(strings.TrimSpace(pool.Status)),
		Status:         firstNonEmptyString(strings.TrimSpace(pool.StatusDetail), strings.TrimSpace(pool.StatusCode)),
		Scan:           scanSummary,
		ScanDetails:    scanDetails,
		ReadErrors:     pool.ReadErrors,
		WriteErrors:    pool.WriteErrors,
		ChecksumErrors: pool.ChecksumErrors,
		Devices:        devices,
	}
}

func devicePath(device string) string {
	device = strings.TrimSpace(device)
	if device == "" {
		return ""
	}
	if strings.HasPrefix(device, "/") {
		return device
	}
	return "/dev/" + device
}

func poolScanSummary(pool Pool) string {
	if pool.Scan == nil {
		return ""
	}
	function := strings.ToLower(strings.TrimSpace(pool.Scan.Function))
	state := strings.ToLower(strings.TrimSpace(pool.Scan.State))
	if function == "" {
		function = "scan"
	}
	summary := strings.TrimSpace(function + " " + state)
	if pool.Scan.Percentage > 0 && poolScanIsActive(state) {
		summary = fmt.Sprintf("%s (%.1f%%)", summary, pool.Scan.Percentage)
	}
	if pool.Scan.Errors > 0 {
		summary = fmt.Sprintf("%s, %d error(s)", summary, pool.Scan.Errors)
	}
	return summary
}

func poolScanIsActive(state string) bool {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "SCANNING", "RUNNING", "IN_PROGRESS", "INPROGRESS":
		return true
	default:
		return false
	}
}

func poolTopologyLabel(pool Pool) string {
	var dataTypes []string
	for _, vdev := range pool.VDevs {
		if vdev.ParentID != "" || !strings.EqualFold(strings.TrimSpace(vdev.Role), "data") {
			continue
		}
		vdevType := strings.ToLower(strings.TrimSpace(vdev.Type))
		if vdevType == "" || containsExactString(dataTypes, vdevType) {
			continue
		}
		dataTypes = append(dataTypes, vdevType)
	}
	if len(dataTypes) == 0 {
		return "pool"
	}
	if len(dataTypes) == 1 && dataTypes[0] == "disk" && len(pool.DiskMembers) > 1 {
		return "stripe"
	}
	return strings.Join(dataTypes, "+")
}

func poolHealthFromTrueNASPool(pool Pool, assessment storagehealth.Assessment, observedAt time.Time) *unifiedresources.PoolHealth {
	canonicalState := canonicalPoolState(pool.Status)
	severity := assessment.Level
	if severity == "" {
		severity = storagehealth.RiskHealthy
	}
	summary := ""
	if len(assessment.Reasons) > 0 {
		summary = strings.TrimSpace(assessment.Reasons[0].Summary)
	} else if canonicalState != "UNKNOWN" {
		summary = fmt.Sprintf("ZFS pool %s is %s", strings.TrimSpace(pool.Name), canonicalState)
	}
	evidenceCodes := make([]string, 0, len(assessment.Reasons))
	for _, reason := range assessment.Reasons {
		if code := strings.TrimSpace(reason.Code); code != "" && !containsExactString(evidenceCodes, code) {
			evidenceCodes = append(evidenceCodes, code)
		}
	}
	source := "pool.query"
	if pool.IsBoot {
		source = "boot.get_state"
	}
	return &unifiedresources.PoolHealth{
		Scope:          "pool",
		Provider:       "truenas",
		NativeID:       firstNonEmptyString(pool.GUID, pool.ID, pool.Name),
		CanonicalState: canonicalState,
		NativeState:    strings.ToUpper(strings.TrimSpace(pool.Status)),
		Severity:       severity,
		Summary:        summary,
		Recommendation: zfsPoolRecommendation(assessment),
		Source:         source,
		EvidenceCodes:  evidenceCodes,
		ObservedAt:     observedAt,
	}
}

func containsExactString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func canonicalPoolState(state string) string {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "ONLINE":
		return "ONLINE"
	case "DEGRADED":
		return "DEGRADED"
	case "FAULTED":
		return "FAULTED"
	case "OFFLINE":
		return "OFFLINE"
	case "UNAVAIL":
		return "UNAVAIL"
	case "REMOVED":
		return "UNAVAIL"
	case "SUSPENDED":
		return "FAULTED"
	default:
		return "UNKNOWN"
	}
}

func zfsPoolRecommendation(assessment storagehealth.Assessment) string {
	hasCode := func(codes ...string) bool {
		for _, reason := range assessment.Reasons {
			for _, code := range codes {
				if strings.TrimSpace(reason.Code) == code {
					return true
				}
			}
		}
		return false
	}
	switch {
	case hasCode("zfs_device_missing", "zfs_device_state"):
		return "Identify the affected vdev member, replace or reconnect it if native evidence confirms failure, then verify redundancy and resilver completion."
	case hasCode("zfs_scan_errors", "zfs_scan_failed"):
		return "Review the native scrub or resilver result, inspect affected devices and cabling, and restore a clean scan before making further storage changes."
	case hasCode("zfs_pool_state"):
		return "Inspect native pool and vdev status, preserve the remaining redundancy, and restore the pool to ONLINE."
	case hasCode("zfs_pool_errors", "zfs_device_errors"):
		return "Inspect the affected device and path, review SMART and cabling evidence, then run a scrub; replace hardware only when native evidence supports it."
	case hasCode("zfs_resilver_active"):
		return "Monitor resilver progress and avoid avoidable pool changes until protection returns to ONLINE."
	case hasCode("zfs_scrub_active"):
		return "Monitor the scrub to completion and review any reported errors."
	default:
		return ""
	}
}

func assessDisk(disk Disk) storagehealth.Assessment {
	sampleAssessment := storagehealth.AssessSample(storagehealth.Sample{
		Model:       strings.TrimSpace(disk.Model),
		Health:      healthForAssessment(disk),
		Temperature: disk.Temperature,
		Wearout:     -1,
	})

	stateUpper := normalizedDiskStatus(disk)
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
	case "FAULTED", "FAILED", "OFFLINE", "REMOVED", "UNAVAIL":
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

func poolTags(pool Pool) []string {
	tags := []string{
		"truenas",
		"pool",
		"zfs",
		"health:" + strings.ToLower(strings.TrimSpace(pool.Status)),
	}
	if pool.IsBoot {
		tags = append(tags, "boot-pool")
	}
	return tags
}

func statusFromDataset(dataset Dataset) unifiedresources.ResourceStatus {
	if dataset.Locked {
		return unifiedresources.StatusOffline
	}
	if !dataset.Mounted {
		return unifiedresources.StatusOffline
	}
	if dataset.ReadOnly && dataset.ReadOnlyReason != DatasetReadOnlyReplicationTarget {
		return unifiedresources.StatusWarning
	}
	return unifiedresources.StatusOnline
}

func datasetStateTag(dataset Dataset) string {
	if dataset.Locked {
		return "state:locked"
	}
	if !dataset.Mounted {
		return "state:unmounted"
	}
	if dataset.ReadOnly && dataset.ReadOnlyReason == DatasetReadOnlyReplicationTarget {
		return "state:replication-readonly"
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

// systemSourceID keys an API-added TrueNAS system by the configured
// connection. The reported hostname is not an identity: two systems that
// report the same hostname (a DR clone, the default "truenas" name) must not
// collapse into one resource and flap between each other's data every poll
// (#1573, #1575). The hostname arm survives only for fixture snapshots,
// which carry no connection.
func systemSourceID(connectionID, hostname string) string {
	if id := strings.TrimSpace(connectionID); id != "" {
		return "system:" + id
	}
	return "system:" + strings.TrimSpace(hostname)
}

func poolSourceID(pool string) string {
	return "pool:" + strings.TrimSpace(pool)
}

func scopedPoolSourceID(systemSourceID, pool string) string {
	return scopedTrueNASSourceID(systemSourceID, poolSourceID(pool))
}

func datasetSourceID(dataset string) string {
	return "dataset:" + strings.TrimSpace(dataset)
}

func scopedDatasetSourceID(systemSourceID, dataset string) string {
	return scopedTrueNASSourceID(systemSourceID, datasetSourceID(dataset))
}

func appSourceID(app App) string {
	id := strings.TrimSpace(app.ID)
	if id == "" {
		id = appDisplayName(app)
	}
	return "app:" + id
}

func scopedAppSourceID(systemSourceID string, app App) string {
	return scopedTrueNASSourceID(systemSourceID, appSourceID(app))
}

func virtualMachineSourceID(vm VirtualMachine) string {
	if id := strings.TrimSpace(vm.ID); id != "" {
		return "vm:" + id
	}
	return "vm:" + virtualMachineDisplayName(vm)
}

func scopedVirtualMachineSourceID(systemSourceID string, vm VirtualMachine) string {
	return scopedTrueNASSourceID(systemSourceID, virtualMachineSourceID(vm))
}

func networkShareSourceID(share NetworkShare) string {
	protocol := strings.ToLower(strings.TrimSpace(share.Protocol))
	if protocol == "" {
		protocol = "share"
	}
	stable := strings.TrimSpace(share.ID)
	if stable == "" {
		stable = networkShareDisplayName(share)
	}
	if stable == "" {
		stable = strings.TrimSpace(share.Path)
	}
	return "share:" + protocol + ":" + stable
}

func scopedNetworkShareSourceID(systemSourceID string, share NetworkShare) string {
	return scopedTrueNASSourceID(systemSourceID, networkShareSourceID(share))
}

func diskSourceID(name string) string {
	return "disk:" + strings.TrimSpace(name)
}

func scopedDiskSourceID(systemSourceID, name string) string {
	return scopedTrueNASSourceID(systemSourceID, diskSourceID(name))
}

// TrueNAS resource names are appliance-local. Scope child source IDs under the
// system source key so common names such as "tank" and "nextcloud" do not merge
// across configured TrueNAS appliances.
func scopedTrueNASSourceID(systemSourceID, sourceID string) string {
	systemSourceID = strings.TrimSpace(systemSourceID)
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return systemSourceID
	}
	if systemSourceID == "" {
		return sourceID
	}
	if strings.HasPrefix(sourceID, systemSourceID+"/") {
		return sourceID
	}
	return systemSourceID + "/" + sourceID
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

func virtualMachineDisplayName(vm VirtualMachine) string {
	if name := strings.TrimSpace(vm.Name); name != "" {
		return name
	}
	return strings.TrimSpace(vm.ID)
}

func networkShareDisplayName(share NetworkShare) string {
	if name := strings.TrimSpace(share.Name); name != "" {
		return name
	}
	if len(share.Aliases) > 0 {
		if alias := strings.TrimSpace(share.Aliases[0]); alias != "" {
			return alias
		}
	}
	if dataset := strings.TrimSpace(share.Dataset); dataset != "" {
		return dataset
	}
	path := strings.Trim(strings.TrimSpace(share.Path), "/")
	if path == "" {
		return strings.TrimSpace(share.ID)
	}
	parts := strings.Split(path, "/")
	return strings.TrimSpace(parts[len(parts)-1])
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

func trueNASServicesFromServices(services []Service) []unifiedresources.TrueNASService {
	if len(services) == 0 {
		return nil
	}
	out := make([]unifiedresources.TrueNASService, 0, len(services))
	for _, service := range services {
		name := strings.TrimSpace(service.Service)
		id := strings.TrimSpace(service.ID)
		if id == "" {
			id = name
		}
		out = append(out, unifiedresources.TrueNASService{
			ID:      id,
			Service: name,
			Enabled: service.Enabled,
			State:   strings.TrimSpace(service.State),
			PIDs:    append([]int(nil), service.PIDs...),
		})
	}
	return out
}

func trueNASAppDataFromApp(app App) *unifiedresources.TrueNASApp {
	containerCount := app.ContainerCount
	if containerCount == 0 && len(app.Containers) > 0 {
		containerCount = len(app.Containers)
	}
	data := &unifiedresources.TrueNASApp{
		ID:                    strings.TrimSpace(app.ID),
		Name:                  strings.TrimSpace(app.Name),
		State:                 strings.TrimSpace(app.State),
		Version:               strings.TrimSpace(app.Version),
		HumanVersion:          strings.TrimSpace(app.HumanVersion),
		CustomApp:             app.CustomApp,
		UpgradeAvailable:      app.UpgradeAvailable,
		ImageUpdatesAvailable: app.ImageUpdatesAvailable,
		Notes:                 strings.TrimSpace(app.Notes),
		ContainerCount:        containerCount,
		UsedHostIPs:           dedupeStrings(app.UsedHostIPs),
		UsedPorts:             trueNASAppPortsFromAppPorts(app.UsedPorts),
		Containers:            trueNASAppContainersFromAppContainers(app.Containers),
		Volumes:               trueNASAppVolumesFromAppVolumes(app.Volumes),
		Images:                dedupeStrings(app.Images),
		Networks:              trueNASAppNetworksFromAppNetworks(app.Networks),
	}
	if app.Stats != nil {
		data.Stats = &unifiedresources.TrueNASAppStats{
			IntervalSeconds: app.Stats.IntervalSeconds,
			CollectedAt:     app.Stats.CollectedAt,
		}
	}
	return data
}

func trueNASAppPortsFromAppPorts(ports []AppPort) []unifiedresources.TrueNASAppPort {
	if len(ports) == 0 {
		return nil
	}
	out := make([]unifiedresources.TrueNASAppPort, 0, len(ports))
	for _, port := range ports {
		if port.ContainerPort == 0 && len(port.HostPorts) == 0 {
			continue
		}
		out = append(out, unifiedresources.TrueNASAppPort{
			ContainerPort: port.ContainerPort,
			Protocol:      strings.ToLower(strings.TrimSpace(port.Protocol)),
			HostPorts:     trueNASAppHostPortsFromAppHostPorts(port.HostPorts),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func trueNASAppHostPortsFromAppHostPorts(hostPorts []AppHostPort) []unifiedresources.TrueNASAppHostPort {
	if len(hostPorts) == 0 {
		return nil
	}
	out := make([]unifiedresources.TrueNASAppHostPort, 0, len(hostPorts))
	for _, hostPort := range hostPorts {
		if hostPort.HostPort == 0 && strings.TrimSpace(hostPort.HostIP) == "" {
			continue
		}
		out = append(out, unifiedresources.TrueNASAppHostPort{
			HostPort: hostPort.HostPort,
			HostIP:   strings.TrimSpace(hostPort.HostIP),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func trueNASAppContainersFromAppContainers(containers []AppContainer) []unifiedresources.TrueNASAppContainer {
	if len(containers) == 0 {
		return nil
	}
	out := make([]unifiedresources.TrueNASAppContainer, 0, len(containers))
	for _, container := range containers {
		if strings.TrimSpace(container.ID) == "" && strings.TrimSpace(container.ServiceName) == "" {
			continue
		}
		out = append(out, unifiedresources.TrueNASAppContainer{
			ID:           strings.TrimSpace(container.ID),
			ServiceName:  strings.TrimSpace(container.ServiceName),
			Image:        strings.TrimSpace(container.Image),
			State:        strings.TrimSpace(container.State),
			PortConfig:   trueNASAppPortsFromAppPorts(container.PortConfig),
			VolumeMounts: trueNASAppVolumesFromAppVolumes(container.VolumeMounts),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func trueNASAppVolumesFromAppVolumes(volumes []AppVolume) []unifiedresources.TrueNASAppVolume {
	if len(volumes) == 0 {
		return nil
	}
	out := make([]unifiedresources.TrueNASAppVolume, 0, len(volumes))
	for _, volume := range volumes {
		source := strings.TrimSpace(volume.Source)
		destination := strings.TrimSpace(volume.Destination)
		if source == "" && destination == "" {
			continue
		}
		out = append(out, unifiedresources.TrueNASAppVolume{
			Source:      source,
			Destination: destination,
			Mode:        strings.TrimSpace(volume.Mode),
			Type:        strings.TrimSpace(volume.Type),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func trueNASAppNetworksFromAppNetworks(networks []AppNetwork) []unifiedresources.TrueNASAppNetwork {
	if len(networks) == 0 {
		return nil
	}
	out := make([]unifiedresources.TrueNASAppNetwork, 0, len(networks))
	for _, network := range networks {
		id := strings.TrimSpace(network.ID)
		name := strings.TrimSpace(network.Name)
		if id == "" && name == "" {
			continue
		}
		out = append(out, unifiedresources.TrueNASAppNetwork{
			ID:     id,
			Name:   name,
			Labels: copyStringLabels(network.Labels),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func trueNASVMDataFromVirtualMachine(vm VirtualMachine) *unifiedresources.TrueNASVM {
	return &unifiedresources.TrueNASVM{
		ID:                    strings.TrimSpace(vm.ID),
		Name:                  strings.TrimSpace(vm.Name),
		Description:           strings.TrimSpace(vm.Description),
		State:                 strings.TrimSpace(vm.State),
		DomainState:           strings.TrimSpace(vm.DomainState),
		PID:                   vm.PID,
		VCPUs:                 vm.VCPUs,
		Cores:                 vm.Cores,
		Threads:               vm.Threads,
		MemoryBytes:           vm.MemoryBytes,
		MinMemoryBytes:        vm.MinMemoryBytes,
		CPUMode:               strings.TrimSpace(vm.CPUMode),
		CPUModel:              strings.TrimSpace(vm.CPUModel),
		Bootloader:            strings.TrimSpace(vm.Bootloader),
		Autostart:             vm.Autostart,
		SuspendOnSnapshot:     vm.SuspendOnSnapshot,
		TrustedPlatformModule: vm.TrustedPlatformModule,
		SecureBoot:            vm.SecureBoot,
		Time:                  strings.TrimSpace(vm.Time),
		ArchType:              strings.TrimSpace(vm.ArchType),
		MachineType:           strings.TrimSpace(vm.MachineType),
		UUID:                  strings.TrimSpace(vm.UUID),
		DisplayAvailable:      vm.DisplayAvailable,
		DeviceCount:           vm.DeviceCount,
		DiskCount:             vm.DiskCount,
		NICCount:              vm.NICCount,
		DisplayCount:          vm.DisplayCount,
		CDROMCount:            vm.CDROMCount,
		USBCount:              vm.USBCount,
		PCICount:              vm.PCICount,
	}
}

func trueNASShareDataFromNetworkShare(share NetworkShare) *unifiedresources.TrueNASShare {
	return &unifiedresources.TrueNASShare{
		ID:                     strings.TrimSpace(share.ID),
		Name:                   networkShareDisplayName(share),
		Protocol:               strings.ToUpper(strings.TrimSpace(share.Protocol)),
		Path:                   strings.TrimSpace(share.Path),
		Dataset:                strings.TrimSpace(share.Dataset),
		RelativePath:           strings.TrimSpace(share.RelativePath),
		Comment:                strings.TrimSpace(share.Comment),
		Enabled:                share.Enabled,
		ReadOnly:               share.ReadOnly,
		Browsable:              share.Browsable,
		Locked:                 share.Locked,
		AccessBasedEnumeration: share.AccessBasedEnumeration,
		AuditEnabled:           share.AuditEnabled,
		ExposeSnapshots:        share.ExposeSnapshots,
		Aliases:                dedupeStrings(share.Aliases),
		Hosts:                  dedupeStrings(share.Hosts),
		Networks:               dedupeStrings(share.Networks),
		Security:               dedupeStrings(share.Security),
		MapRootUser:            strings.TrimSpace(share.MapRootUser),
		MapRootGroup:           strings.TrimSpace(share.MapRootGroup),
		MapAllUser:             strings.TrimSpace(share.MapAllUser),
		MapAllGroup:            strings.TrimSpace(share.MapAllGroup),
	}
}

func copyStringLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	out := make(map[string]string, len(labels))
	for key, value := range labels {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
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

func statusFromVirtualMachine(vm VirtualMachine) unifiedresources.ResourceStatus {
	state := strings.ToUpper(strings.TrimSpace(vm.State))
	if state == "" {
		state = strings.ToUpper(strings.TrimSpace(vm.DomainState))
	}
	switch state {
	case "RUNNING", "ACTIVE":
		return unifiedresources.StatusOnline
	case "STOPPED", "SHUTOFF", "SHUTDOWN", "POWEROFF":
		return unifiedresources.StatusOffline
	case "PAUSED", "SUSPENDED", "ERROR", "CRASHED", "PANICKED":
		return unifiedresources.StatusWarning
	default:
		return unifiedresources.StatusUnknown
	}
}

func statusFromNetworkShare(share NetworkShare) unifiedresources.ResourceStatus {
	if !share.Enabled {
		return unifiedresources.StatusOffline
	}
	if share.Locked {
		return unifiedresources.StatusWarning
	}
	return unifiedresources.StatusOnline
}

func virtualMachineTags(vm VirtualMachine) []string {
	tags := []string{"truenas", "vm"}
	if state := strings.ToLower(strings.TrimSpace(vm.State)); state != "" {
		tags = append(tags, "state:"+state)
	}
	if bootloader := strings.ToLower(strings.TrimSpace(vm.Bootloader)); bootloader != "" {
		tags = append(tags, "boot:"+bootloader)
	}
	if vm.Autostart {
		tags = append(tags, "autostart")
	}
	if vm.TrustedPlatformModule {
		tags = append(tags, "tpm")
	}
	if vm.SecureBoot {
		tags = append(tags, "secure-boot")
	}
	return dedupeStrings(tags)
}

func networkShareTags(share NetworkShare) []string {
	tags := []string{"truenas", "share"}
	if protocol := strings.ToLower(strings.TrimSpace(share.Protocol)); protocol != "" {
		tags = append(tags, protocol)
	}
	if share.ReadOnly {
		tags = append(tags, "readonly")
	}
	if share.Locked {
		tags = append(tags, "locked")
	}
	if dataset := strings.TrimSpace(share.Dataset); dataset != "" {
		tags = append(tags, "dataset:"+dataset)
	}
	return dedupeStrings(tags)
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
	switch normalizedDiskStatus(disk) {
	case "ONLINE", "OK", "PASSED", "HEALTHY":
		return unifiedresources.StatusOnline
	case "DEGRADED":
		return unifiedresources.StatusWarning
	case "FAULTED", "FAILED", "OFFLINE", "REMOVED", "UNAVAIL":
		return unifiedresources.StatusOffline
	default:
		return unifiedresources.StatusUnknown
	}
}

func healthFromDisk(disk Disk) string {
	if health, ok := explicitDiskHealth(disk); ok {
		return health
	}
	switch normalizedDiskStatus(disk) {
	case "ONLINE", "OK", "PASSED", "HEALTHY":
		return "PASSED"
	case "DEGRADED":
		return "DEGRADED"
	case "FAULTED", "FAILED", "OFFLINE", "REMOVED", "UNAVAIL":
		return "FAILED"
	default:
		// TrueNAS can omit disk health/status when SMART data is unavailable.
		// Missing or provider-unknown telemetry is not a disk failure signal.
		return "UNKNOWN"
	}
}

func healthForAssessment(disk Disk) string {
	if health, ok := explicitDiskHealth(disk); ok {
		return health
	}
	switch normalizedDiskStatus(disk) {
	case "DEGRADED":
		return ""
	default:
		return healthFromDisk(disk)
	}
}

func normalizedDiskStatus(disk Disk) string {
	return strings.ToUpper(strings.TrimSpace(disk.Status))
}

func explicitDiskHealth(disk Disk) (string, bool) {
	if !disk.HealthStatusPresent && strings.TrimSpace(disk.Health) == "" {
		return "", false
	}
	return normalizeExplicitDiskHealth(disk.Health), true
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

// trueNASSystemMetricResourceID must stay systemSourceID minus its "system:"
// prefix: BuildMetricsTarget derives the agent metric target by stripping
// that prefix from the TrueNAS source ID (canonicalAgentMetricID), and the
// records' Agent.AgentID plus the native history keys returned by
// SystemMetricHistory have to resolve to the same series.
func trueNASSystemMetricResourceID(connectionID string, system SystemInfo) string {
	if id := strings.TrimSpace(connectionID); id != "" {
		return id
	}
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
	copied.Pools = clonePools(snapshot.Pools)
	copied.Datasets = append([]Dataset(nil), snapshot.Datasets...)
	copied.Disks = append([]Disk(nil), snapshot.Disks...)
	copied.Alerts = append([]Alert(nil), snapshot.Alerts...)
	copied.Services = cloneServices(snapshot.Services)
	copied.Apps = cloneApps(snapshot.Apps)
	copied.VMs = append([]VirtualMachine(nil), snapshot.VMs...)
	copied.Shares = cloneNetworkShares(snapshot.Shares)
	copied.ZFSSnapshots = append([]ZFSSnapshot(nil), snapshot.ZFSSnapshots...)
	copied.ReplicationTasks = append([]ReplicationTask(nil), snapshot.ReplicationTasks...)
	return &copied
}

func clonePools(pools []Pool) []Pool {
	if len(pools) == 0 {
		return nil
	}
	out := make([]Pool, len(pools))
	for i := range pools {
		out[i] = pools[i]
		if pools[i].Scan != nil {
			scan := *pools[i].Scan
			out[i].Scan = &scan
		}
		out[i].VDevs = append([]PoolVDev(nil), pools[i].VDevs...)
		out[i].DiskMembers = append([]PoolDiskMember(nil), pools[i].DiskMembers...)
	}
	return out
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

func cloneServices(services []Service) []Service {
	if len(services) == 0 {
		return nil
	}
	out := make([]Service, len(services))
	for i := range services {
		out[i] = services[i]
		out[i].PIDs = append([]int(nil), services[i].PIDs...)
	}
	return out
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

func cloneNetworkShares(shares []NetworkShare) []NetworkShare {
	if len(shares) == 0 {
		return nil
	}
	out := make([]NetworkShare, len(shares))
	for i := range shares {
		out[i] = shares[i]
		out[i].Aliases = append([]string(nil), shares[i].Aliases...)
		out[i].Hosts = append([]string(nil), shares[i].Hosts...)
		out[i].Networks = append([]string(nil), shares[i].Networks...)
		out[i].Security = append([]string(nil), shares[i].Security...)
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
