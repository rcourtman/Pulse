package monitoring

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
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
	return map[unifiedresources.DataSource]time.Duration{
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
