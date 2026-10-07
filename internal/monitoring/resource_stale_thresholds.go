package monitoring

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const (
	defaultProxmoxResourceStaleThreshold  = 60 * time.Second
	defaultPlatformResourceStaleThreshold = 120 * time.Second
)

// ResourceStaleThresholdsForConfig derives canonical resource freshness from
// polling cadence. A source should not be considered stale until it has missed
// at least one expected poll cycle plus the normal interval.
func ResourceStaleThresholdsForConfig(cfg *config.Config) map[unifiedresources.DataSource]time.Duration {
	return resourceStaleThresholdsForConfig(cfg, mock.IsMockEnabled(), mock.SupplementalRefreshInterval)
}

func resourceStaleThresholdsForConfig(
	cfg *config.Config,
	mockEnabled bool,
	mockSupplementalCadence func() time.Duration,
) map[unifiedresources.DataSource]time.Duration {
	thresholds := map[unifiedresources.DataSource]time.Duration{
		unifiedresources.SourceProxmox: resourceStaleThresholdForPollInterval(
			effectivePVEPollingIntervalForConfig(cfg),
			defaultProxmoxResourceStaleThreshold,
		),
		unifiedresources.SourcePBS: resourceStaleThresholdForPollInterval(
			effectivePlatformPollingIntervalForConfig(cfg, "pbs"),
			defaultPlatformResourceStaleThreshold,
		),
		unifiedresources.SourcePMG: resourceStaleThresholdForPollInterval(
			effectivePlatformPollingIntervalForConfig(cfg, "pmg"),
			defaultPlatformResourceStaleThreshold,
		),
	}
	// Mock mode's provider-backed fixtures (TrueNAS, VMware, availability)
	// deliver on the mock update loop's supplemental cadence rather than a
	// real poll schedule. Derive their freshness from that cadence the same
	// way the entries above derive theirs, so a slow mock tick (e.g. a large
	// PULSE_MOCK_UPDATE_INTERVAL on the public demo) does not flag every
	// provider-owned row as a stale source between refreshes.
	if mockEnabled && mockSupplementalCadence != nil {
		supplemental := resourceStaleThresholdForPollInterval(
			mockSupplementalCadence(),
			defaultPlatformResourceStaleThreshold,
		)
		thresholds[unifiedresources.SourceTrueNAS] = supplemental
		thresholds[unifiedresources.SourceVMware] = supplemental
		thresholds[unifiedresources.SourceAvailability] = supplemental
	}
	return thresholds
}

// UnifiedResourceSnapshotWithStaleThresholds is UnifiedResourceSnapshot
// plus the stale thresholds the snapshot's registry judged its sightings by.
// A consumer that rebuilds a registry from the snapshot, as the resources API
// does, judges by the same thresholds, so its stale pass and presentation
// agree with the monitor's views.
func (m *Monitor) UnifiedResourceSnapshotWithStaleThresholds() ([]unifiedresources.Resource, time.Time, map[unifiedresources.DataSource]time.Duration) {
	view := m.currentUnifiedStateView()
	return m.applyPersistedMetadataToUnifiedResources(view.resources), view.freshness, view.staleThresholds
}

// unifiedResourceThresholdProjectionLister lists a read state with the stale
// thresholds the listed registry generation was judged by (MonitorAdapter,
// and any wrapper embedding one).
type unifiedResourceThresholdProjectionLister interface {
	GetAllWithMetricsTargetsAndStaleThresholds() ([]unifiedresources.Resource, map[string]unifiedresources.MetricsTarget, map[unifiedresources.DataSource]time.Duration)
}

// unifiedProjectionResourcesWithStaleThresholds lists a read state with the
// thresholds the listed generation was judged by: the resource store's
// configured ones, or nil (the registry defaults) for a read state that
// reports none, such as a view built from mock fixtures or a bare snapshot.
func unifiedProjectionResourcesWithStaleThresholds(
	lister unifiedResourceReadStateLister,
) ([]unifiedresources.Resource, MetricsTargetResourceStore, map[unifiedresources.DataSource]time.Duration) {
	if projection, ok := lister.(unifiedResourceThresholdProjectionLister); ok {
		resources, targets, thresholds := projection.GetAllWithMetricsTargetsAndStaleThresholds()
		return resources, projectionMetricsTargets(targets), thresholds
	}
	resources, targets := unifiedProjectionResources(lister)
	return resources, targets, nil
}

func (m *Monitor) resourceStaleThresholds() map[unifiedresources.DataSource]time.Duration {
	if m == nil {
		return ResourceStaleThresholdsForConfig(nil)
	}
	return ResourceStaleThresholdsForConfig(m.config)
}

func (m *Monitor) pveNodeOfflineGracePeriod() time.Duration {
	return m.resourceStaleThresholds()[unifiedresources.SourceProxmox]
}

func effectivePVEPollingIntervalForConfig(cfg *config.Config) time.Duration {
	const minInterval = 10 * time.Second
	const maxInterval = time.Hour

	interval := minInterval
	if cfg != nil && cfg.PVEPollingInterval > 0 {
		interval = cfg.PVEPollingInterval
	}
	return clampInterval(interval, minInterval, maxInterval)
}

func effectivePlatformPollingIntervalForConfig(cfg *config.Config, platform string) time.Duration {
	if cfg == nil {
		return 60 * time.Second
	}
	switch platform {
	case "pbs":
		return clampInterval(cfg.PBSPollingInterval, 10*time.Second, time.Hour)
	case "pmg":
		return clampInterval(cfg.PMGPollingInterval, 10*time.Second, time.Hour)
	default:
		return 60 * time.Second
	}
}

func resourceStaleThresholdForPollInterval(interval, minimum time.Duration) time.Duration {
	if minimum <= 0 {
		minimum = defaultPlatformResourceStaleThreshold
	}
	if interval <= 0 {
		return minimum
	}
	threshold := interval * 2
	if threshold < minimum {
		return minimum
	}
	return threshold
}
