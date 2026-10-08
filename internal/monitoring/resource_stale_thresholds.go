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

// resourcePollIntervals are the poll cadences canonical resource freshness is
// derived from.
type resourcePollIntervals struct {
	pve time.Duration
	pbs time.Duration
	pmg time.Duration
}

func resourcePollIntervalsForConfig(cfg *config.Config) resourcePollIntervals {
	return resourcePollIntervals{
		pve: effectivePVEPollingIntervalForConfig(cfg),
		pbs: effectivePlatformPollingIntervalForConfig(cfg, "pbs"),
		pmg: effectivePlatformPollingIntervalForConfig(cfg, "pmg"),
	}
}

// ResourceStaleThresholdsForConfig derives canonical resource freshness from
// configured polling cadence. A source should not be considered stale until it
// has missed at least one expected poll cycle plus the normal interval. A
// running monitor judges by Monitor.resourceStaleThresholds instead, which
// honours the runtime polling overrides its scheduler reads.
func ResourceStaleThresholdsForConfig(cfg *config.Config) map[unifiedresources.DataSource]time.Duration {
	return resourceStaleThresholdsForConfig(cfg, mock.IsMockEnabled(), mock.SupplementalRefreshInterval)
}

func resourceStaleThresholdsForConfig(
	cfg *config.Config,
	mockEnabled bool,
	mockSupplementalCadence func() time.Duration,
) map[unifiedresources.DataSource]time.Duration {
	return resourceStaleThresholdsForIntervals(resourcePollIntervalsForConfig(cfg), mockEnabled, mockSupplementalCadence)
}

func resourceStaleThresholdsForIntervals(
	intervals resourcePollIntervals,
	mockEnabled bool,
	mockSupplementalCadence func() time.Duration,
) map[unifiedresources.DataSource]time.Duration {
	thresholds := map[unifiedresources.DataSource]time.Duration{
		unifiedresources.SourceProxmox: resourceStaleThresholdForPollInterval(
			intervals.pve,
			defaultProxmoxResourceStaleThreshold,
		),
		unifiedresources.SourcePBS: resourceStaleThresholdForPollInterval(
			intervals.pbs,
			defaultPlatformResourceStaleThreshold,
		),
		unifiedresources.SourcePMG: resourceStaleThresholdForPollInterval(
			intervals.pmg,
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
	if m == nil || m.config == nil {
		return ResourceStaleThresholdsForConfig(nil)
	}
	return resourceStaleThresholdsForIntervals(
		m.resourcePollIntervals(),
		mock.IsMockEnabled(),
		mock.SupplementalRefreshInterval,
	)
}

// resourcePollIntervals reads the per-platform base cadences, honouring the
// runtime overrides a fixed-cadence scheduler polls at. Non-default tenant
// monitors poll against a detached config copy (#1619), so the settings API
// pushes saved PBS and PMG intervals into them as runtime overrides; freshness
// derived from m.config alone would judge those sources by an interval nothing
// polls at. A PVE interval change reloads every monitor from saved config
// instead, so the config value is already the live one. An adaptive scheduler
// selects its own intervals and never reads the per-platform overrides, so
// they must not move freshness there either.
func (m *Monitor) resourcePollIntervals() resourcePollIntervals {
	intervals := resourcePollIntervalsForConfig(m.config)
	if m.scheduler != nil {
		return intervals
	}
	intervals.pbs = clampInterval(m.pbsPollingIntervalSetting(), 10*time.Second, time.Hour)
	intervals.pmg = clampInterval(m.pmgPollingIntervalSetting(), 10*time.Second, time.Hour)
	return intervals
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
