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
	records := make([]unifiedresources.IngestRecord, 0, 1+len(snapshot.Pools)+len(snapshot.Datasets)+len(snapshot.Disks))

	totalCapacity, totalUsed := aggregatePoolUsage(snapshot.Pools)
	records = append(records, unifiedresources.IngestRecord{
		SourceID: systemSourceID,
		Resource: unifiedresources.Resource{
			Type:      unifiedresources.ResourceTypeHost,
			Name:      strings.TrimSpace(snapshot.System.Hostname),
			Status:    statusFromSystem(snapshot.System),
			LastSeen:  collectedAt,
			UpdatedAt: collectedAt,
			Metrics: &unifiedresources.ResourceMetrics{
				Disk: diskMetric(totalCapacity, totalUsed),
			},
			TrueNAS: &unifiedresources.TrueNASData{
				Hostname:      strings.TrimSpace(snapshot.System.Hostname),
				Version:       snapshot.System.Version,
				UptimeSeconds: snapshot.System.UptimeSeconds,
			},
			Tags: []string{
				"truenas",
				snapshot.System.Version,
				"zfs",
			},
		},
		Identity: unifiedresources.ResourceIdentity{
			MachineID: snapshot.System.MachineID,
			Hostnames: []string{snapshot.System.Hostname},
		},
	})

	for _, pool := range snapshot.Pools {
		records = append(records, unifiedresources.IngestRecord{
			SourceID:       poolSourceID(pool.Name),
			ParentSourceID: systemSourceID,
			Resource: unifiedresources.Resource{
				Type:      unifiedresources.ResourceTypeStorage,
				Name:      pool.Name,
				Status:    statusFromPool(pool),
				LastSeen:  collectedAt,
				UpdatedAt: collectedAt,
				Metrics: &unifiedresources.ResourceMetrics{
					Disk: diskMetric(pool.TotalBytes, pool.UsedBytes),
				},
				Storage: &unifiedresources.StorageMeta{
					Type:         "zfs-pool",
					IsZFS:        true,
					ZFSPoolState: strings.ToUpper(strings.TrimSpace(pool.Status)),
				},
				Tags: []string{
					"truenas",
					"pool",
					"zfs",
					"health:" + strings.ToLower(strings.TrimSpace(pool.Status)),
				},
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
					Type:  "zfs-dataset",
					IsZFS: true,
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

	for _, disk := range snapshot.Disks {
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
				Status:    statusFromDisk(disk),
				LastSeen:  collectedAt,
				UpdatedAt: collectedAt,
				PhysicalDisk: &unifiedresources.PhysicalDiskMeta{
					DevPath:   "/dev/" + disk.Name,
					Model:     disk.Model,
					Serial:    disk.Serial,
					DiskType:  disk.Transport,
					SizeBytes: disk.SizeBytes,
					Health:    healthFromDisk(disk),
					Wearout:   -1,
					RPM:       rpmFromDisk(disk),
				},
				Tags: []string{"truenas", "disk", disk.Transport},
			},
			Identity: diskIdentity,
		})
	}

	return records
}

func statusFromSystem(system SystemInfo) unifiedresources.ResourceStatus {
	if system.Healthy {
		return unifiedresources.StatusOnline
	}
	return unifiedresources.StatusWarning
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

func diskSourceID(name string) string {
	return "disk:" + strings.TrimSpace(name)
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
	copied.ZFSSnapshots = append([]ZFSSnapshot(nil), snapshot.ZFSSnapshots...)
	copied.ReplicationTasks = append([]ReplicationTask(nil), snapshot.ReplicationTasks...)
	return &copied
}
