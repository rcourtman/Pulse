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
	// FeatureTrueNAS gates fixture-first TrueNAS ingestion.
	FeatureTrueNAS = "PULSE_ENABLE_TRUENAS"
)

var featureTrueNASEnabled atomic.Bool

func init() {
	featureTrueNASEnabled.Store(parseBool(os.Getenv(FeatureTrueNAS)))
}

var errNilSnapshot = errors.New("truenas provider fetcher returned nil snapshot")

// IsFeatureEnabled returns whether fixture-driven TrueNAS ingestion is enabled.
func IsFeatureEnabled() bool {
	return featureTrueNASEnabled.Load()
}

// SetFeatureEnabled allows tests to control the feature flag.
func SetFeatureEnabled(enabled bool) {
	featureTrueNASEnabled.Store(enabled)
}

// Fetcher loads a TrueNAS snapshot from a concrete source.
type Fetcher interface {
	Fetch(ctx context.Context) (*FixtureSnapshot, error)
}

type fetcherCloser interface {
	Close()
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
	p.lastSnapshot = copyFixtureSnapshot(snapshot)
	p.mu.Unlock()
	return nil
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

// Records returns unified records if the feature flag is enabled.
func (p *Provider) Records() []unifiedresources.IngestRecord {
	if p == nil || !IsFeatureEnabled() {
		return nil
	}

	snapshot := p.Snapshot()
	if snapshot == nil {
		return nil
	}

	collectedAt := snapshot.CollectedAt
	if collectedAt.IsZero() {
		collectedAt = p.now()
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
			Metrics: &unifiedresources.ResourceMetrics{
				Disk: diskMetric(totalCapacity, totalUsed),
			},
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
				Type:       unifiedresources.ResourceTypeAppContainer,
				Technology: "docker",
				Name:       appDisplayName(app),
				Status:     statusFromApp(app),
				LastSeen:   collectedAt,
				UpdatedAt:  collectedAt,
				Docker:     dockerMeta,
				Tags:       appTags(app),
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
					DevPath:     "/dev/" + disk.Name,
					Model:       disk.Model,
					Serial:      disk.Serial,
					DiskType:    disk.Transport,
					SizeBytes:   disk.SizeBytes,
					Health:      healthFromDisk(disk),
					Temperature: disk.Temperature,
					Wearout:     -1,
					RPM:         rpmFromDisk(disk),
					Risk:        unifiedresources.PhysicalDiskRiskFromAssessment(assessment),
				},
				Tags:      []string{"truenas", "disk", disk.Transport},
				Incidents: incidents,
			},
			Identity: diskIdentity,
		})
	}

	return records
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

func parentPoolFromDataset(datasetName string) string {
	parts := strings.SplitN(strings.TrimSpace(datasetName), "/", 2)
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
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
	copied.Pools = append([]Pool(nil), snapshot.Pools...)
	copied.Datasets = append([]Dataset(nil), snapshot.Datasets...)
	copied.Disks = append([]Disk(nil), snapshot.Disks...)
	copied.Alerts = append([]Alert(nil), snapshot.Alerts...)
	copied.Apps = cloneApps(snapshot.Apps)
	copied.ZFSSnapshots = append([]ZFSSnapshot(nil), snapshot.ZFSSnapshots...)
	copied.ReplicationTasks = append([]ReplicationTask(nil), snapshot.ReplicationTasks...)
	return &copied
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
