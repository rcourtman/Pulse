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

// adaptiveResourcePollIntervals are the cadences freshness is derived from when
// an adaptive scheduler plans polling. It selects its own intervals and never
// reads the per-platform ones; a healthy instance's staleness score is near
// zero after every success, so its cadence stretches toward the scheduler's
// maximum interval, which bounds it. Freshness must cover that gap, or
// healthy Proxmox, PBS and PMG rows read stale, and Proxmox nodes lose their
// offline grace, for much of every cycle.
func adaptiveResourcePollIntervals(maxInterval time.Duration) resourcePollIntervals {
	return resourcePollIntervals{pve: maxInterval, pbs: maxInterval, pmg: maxInterval}
}

func resourcePollIntervalsForConfig(cfg *config.Config) resourcePollIntervals {
	if cfg != nil && cfg.AdaptivePollingEnabled {
		return adaptiveResourcePollIntervals(normalizedSchedulerConfig(SchedulerConfig{
			BaseInterval: cfg.AdaptivePollingBaseInterval,
			MinInterval:  cfg.AdaptivePollingMinInterval,
			MaxInterval:  cfg.AdaptivePollingMaxInterval,
		}).MaxInterval)
	}
	return resourcePollIntervals{
		pve: effectivePVEPollingIntervalForConfig(cfg),
		pbs: effectivePlatformPollingIntervalForConfig(cfg, "pbs"),
		pmg: effectivePlatformPollingIntervalForConfig(cfg, "pmg"),
	}
}

// ResourceStaleThresholdsForConfig derives canonical resource freshness from
// configured polling cadence: the per-platform intervals, or the adaptive
// scheduler's maximum interval when adaptive polling is enabled. A source
// should not be considered stale until it has missed at least one expected
// poll cycle plus the normal interval. A running monitor judges by
// Monitor.resourceStaleThresholds instead, which honours the runtime polling
// overrides its scheduler reads.
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

// resourcePollIntervals reads the cadences the monitor's scheduler polls at.
// A fixed-cadence scheduler polls each platform at its base interval,
// honouring the runtime overrides: non-default tenant monitors poll against a
// detached config copy (#1619), so the settings API pushes saved PBS and PMG
// intervals into them as overrides, and freshness derived from m.config alone
// would judge those sources by an interval nothing polls at. A PVE interval
// change reloads every monitor from saved config instead, so the config value
// is already the live one. An adaptive scheduler never reads the per-platform
// intervals or their overrides, so they cannot move freshness there; it is
// judged by the longest interval that scheduler selects.
func (m *Monitor) resourcePollIntervals() resourcePollIntervals {
	if m.scheduler != nil {
		return adaptiveResourcePollIntervals(m.scheduler.MaxInterval())
	}
	return resourcePollIntervals{
		pve: m.BasePollInterval(InstanceTypePVE),
		pbs: m.BasePollInterval(InstanceTypePBS),
		pmg: m.BasePollInterval(InstanceTypePMG),
	}
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
