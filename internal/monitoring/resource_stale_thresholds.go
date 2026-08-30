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
