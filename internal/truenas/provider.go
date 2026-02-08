package truenas

import (
	"os"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const (
	// FeatureTrueNAS gates fixture-first TrueNAS ingestion.
	FeatureTrueNAS = "PULSE_ENABLE_TRUENAS"
)

var featureTrueNASEnabled = parseBool(os.Getenv(FeatureTrueNAS))

// IsFeatureEnabled returns whether fixture-driven TrueNAS ingestion is enabled.
func IsFeatureEnabled() bool {
	return featureTrueNASEnabled
}

// SetFeatureEnabled allows tests to control the feature flag.
func SetFeatureEnabled(enabled bool) {
	featureTrueNASEnabled = enabled
}

// Provider converts TrueNAS fixture data into unified resources.
type Provider struct {
	fixtures FixtureSnapshot
	now      func() time.Time
}

// NewProvider returns a fixture-backed provider.
func NewProvider(fixtures FixtureSnapshot) *Provider {
	if fixtures.CollectedAt.IsZero() {
		fixtures.CollectedAt = time.Now().UTC()
	}
	return &Provider{
		fixtures: fixtures,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

// NewDefaultProvider returns a provider loaded with the default fixtures.
func NewDefaultProvider() *Provider {
	return NewProvider(DefaultFixtures())
}

// Records returns unified records if the feature flag is enabled.
func (p *Provider) Records() []unifiedresources.IngestRecord {
	if p == nil || !IsFeatureEnabled() {
		return nil
	}

	collectedAt := p.fixtures.CollectedAt
	if collectedAt.IsZero() {
		collectedAt = p.now()
	}
	systemSourceID := systemSourceID(p.fixtures.System.Hostname)
	records := make([]unifiedresources.IngestRecord, 0, 1+len(p.fixtures.Pools)+len(p.fixtures.Datasets))

	totalCapacity, totalUsed := aggregatePoolUsage(p.fixtures.Pools)
	records = append(records, unifiedresources.IngestRecord{
		SourceID: systemSourceID,
		Resource: unifiedresources.Resource{
			Type:      unifiedresources.ResourceTypeHost,
			Name:      strings.TrimSpace(p.fixtures.System.Hostname),
			Status:    statusFromSystem(p.fixtures.System),
			LastSeen:  collectedAt,
			UpdatedAt: collectedAt,
			Metrics: &unifiedresources.ResourceMetrics{
				Disk: diskMetric(totalCapacity, totalUsed),
			},
			Tags: []string{
				"truenas",
				p.fixtures.System.Version,
			},
		},
		Identity: unifiedresources.ResourceIdentity{
			MachineID: p.fixtures.System.MachineID,
			Hostnames: []string{p.fixtures.System.Hostname},
		},
	})

	for _, pool := range p.fixtures.Pools {
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
				Tags: []string{
					"truenas",
					"pool",
				},
			},
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: []string{
					p.fixtures.System.Hostname,
					pool.Name,
				},
			},
		})
	}

	for _, dataset := range p.fixtures.Datasets {
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
				Tags: []string{
					"truenas",
					"dataset",
				},
			},
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: []string{
					p.fixtures.System.Hostname,
					dataset.Name,
				},
			},
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
	switch strings.ToLower(strings.TrimSpace(pool.Status)) {
	case "online", "healthy":
		return unifiedresources.StatusOnline
	case "degraded", "warn", "warning":
		return unifiedresources.StatusWarning
	case "faulted", "offline", "unavailable":
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
