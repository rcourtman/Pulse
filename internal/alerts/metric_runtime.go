package alerts

import "strings"

type metricStabilityPolicy struct {
	CriticalBypassesDelay bool
	RecoveryDelaySeconds  int
}

const (
	defaultLegacyMetricDelaySeconds   = 5
	defaultNoisyGaugeStabilitySeconds = 5 * 60
)

func defaultMetricStabilityDelay(metricType string) int {
	switch strings.ToLower(strings.TrimSpace(metricType)) {
	case "memory", "temperature", "disktemperature", "disk_temperature":
		return defaultNoisyGaugeStabilitySeconds
	default:
		return 0
	}
}

// metricStabilityFor keeps noisy gauges trustworthy without adding a second
// operator-facing policy surface. The configured alert delay is the one
// stability window on both lifecycle edges; critical evidence bypasses only
// the legacy/default activation delay, never an explicit intent policy.
func metricStabilityFor(metricType string, delaySeconds int) metricStabilityPolicy {
	if delaySeconds <= 0 {
		return metricStabilityPolicy{}
	}
	switch strings.ToLower(strings.TrimSpace(metricType)) {
	case "memory", "temperature", "disktemperature", "disk_temperature":
		return metricStabilityPolicy{
			CriticalBypassesDelay: true,
			RecoveryDelaySeconds:  delaySeconds,
		}
	default:
		return metricStabilityPolicy{}
	}
}

func isMetricThresholdAlertType(metricType string) bool {
	switch metricType {
	case "cpu", "memory", "disk", "diskRead", "diskWrite", "networkIn", "networkOut", "temperature", "diskTemperature", "usage":
		return true
	default:
		return false
	}
}

// getThresholdForMetric returns the threshold for a specific metric type from a ThresholdConfig.
func getThresholdForMetric(config ThresholdConfig, metricType string) *HysteresisThreshold {
	switch metricType {
	case "cpu":
		return config.CPU
	case "memory":
		return config.Memory
	case "disk":
		return config.Disk
	case "diskRead":
		return config.DiskRead
	case "diskWrite":
		return config.DiskWrite
	case "networkIn":
		return config.NetworkIn
	case "networkOut":
		return config.NetworkOut
	case "temperature":
		return config.Temperature
	case "diskTemperature":
		return config.DiskTemperature
	case "usage":
		return config.Usage
	default:
		return nil
	}
}

// getTimeThreshold determines the delay to apply for a metric/resource combination.
func (m *Manager) getTimeThreshold(resourceID string, resourceType, metricType string) int {
	effective := m.resolveEffectiveIntentPolicyNoLock(resourceID, resourceType, MetricAlertIntentSignal(metricType))
	return effective.GraceSeconds
}

func (m *Manager) getLegacyTimeThresholdWithSource(resourceType, metricType string) (int, string, bool) {
	if delay, ok := m.getMetricTimeThreshold(resourceType, metricType); ok {
		return delay, "legacy.metricTimeThresholds." + strings.ToLower(strings.TrimSpace(resourceType)) + "." + strings.ToLower(strings.TrimSpace(metricType)), true
	}
	if delay, ok := m.getGlobalExactMetricTimeThreshold(metricType); ok {
		return delay, "legacy.metricTimeThresholds.all." + strings.ToLower(strings.TrimSpace(metricType)), true
	}
	base, hasTypeSpecific := m.getBaseTimeThreshold(resourceType)
	if hasTypeSpecific {
		// The legacy factory delay was five seconds for every metric. Upgrade
		// only that unchanged default for noisy gauges; a deliberate type-level
		// value remains authoritative, as does any metric-specific override above.
		if base == defaultLegacyMetricDelaySeconds {
			if delay := defaultMetricStabilityDelay(metricType); delay > 0 {
				return delay, "factory.metricStability." + strings.ToLower(strings.TrimSpace(metricType)), true
			}
		}
		return base, "legacy.timeThresholds." + strings.ToLower(strings.TrimSpace(resourceType)), true
	}
	if delay, ok := m.getGlobalMetricTimeThreshold(metricType); ok {
		return delay, "legacy.metricTimeThresholds.all." + strings.ToLower(strings.TrimSpace(metricType)), true
	}
	if base != 0 {
		if base == defaultLegacyMetricDelaySeconds {
			if delay := defaultMetricStabilityDelay(metricType); delay > 0 {
				return delay, "factory.metricStability." + strings.ToLower(strings.TrimSpace(metricType)), true
			}
		}
		return base, "legacy.timeThresholds.all", true
	}
	return 0, "", false
}

// getGlobalExactMetricTimeThreshold returns only an explicit all.<metric>
// override. Broad all.default/all.* fallbacks retain their legacy position
// after resource-type delays and therefore cannot accidentally erase those
// more specific policies.
func (m *Manager) getGlobalExactMetricTimeThreshold(metricType string) (int, bool) {
	if len(m.config.MetricTimeThresholds) == 0 {
		return 0, false
	}

	perType, ok := m.config.MetricTimeThresholds["all"]
	if !ok || len(perType) == 0 {
		return 0, false
	}

	metricKey := strings.ToLower(strings.TrimSpace(metricType))
	if metricKey == "" {
		return 0, false
	}

	delay, ok := perType[metricKey]
	return delay, ok
}

// getMetricTimeThreshold returns a metric-specific delay if configured at the resource-type level.
func (m *Manager) getMetricTimeThreshold(resourceType, metricType string) (int, bool) {
	if len(m.config.MetricTimeThresholds) == 0 {
		return 0, false
	}

	metricKey := strings.ToLower(strings.TrimSpace(metricType))
	if metricKey == "" {
		return 0, false
	}

	for _, typeKey := range CanonicalResourceTypeKeys(resourceType) {
		perType, ok := m.config.MetricTimeThresholds[typeKey]
		if !ok || len(perType) == 0 {
			continue
		}

		if delay, ok := perType[metricKey]; ok {
			return delay, true
		}
		if delay, ok := perType["default"]; ok {
			return delay, true
		}
		if delay, ok := perType["_default"]; ok {
			return delay, true
		}
		if delay, ok := perType["*"]; ok {
			return delay, true
		}
	}

	return 0, false
}

// getBaseTimeThreshold returns the resource-type level delay.
func (m *Manager) getBaseTimeThreshold(resourceType string) (int, bool) {
	if m.config.TimeThresholds != nil {
		for _, key := range CanonicalResourceTypeKeys(resourceType) {
			if delay, ok := m.config.TimeThresholds[key]; ok {
				return delay, true
			}
		}
		if delay, ok := m.config.TimeThresholds["all"]; ok {
			return delay, false
		}
	}

	return 0, false
}

func (m *Manager) getGlobalMetricTimeThreshold(metricType string) (int, bool) {
	if len(m.config.MetricTimeThresholds) == 0 {
		return 0, false
	}

	perType, ok := m.config.MetricTimeThresholds["all"]
	if !ok || len(perType) == 0 {
		return 0, false
	}

	metricKey := strings.ToLower(strings.TrimSpace(metricType))
	if metricKey == "" {
		return 0, false
	}

	if delay, ok := perType[metricKey]; ok {
		return delay, true
	}
	if delay, ok := perType["default"]; ok {
		return delay, true
	}
	if delay, ok := perType["_default"]; ok {
		return delay, true
	}
	if delay, ok := perType["*"]; ok {
		return delay, true
	}

	return 0, false
}

// metricOptions carries per-evaluation overrides for metric threshold checks.
type metricOptions struct {
	Metadata       map[string]interface{}
	RemoveMetadata []string
	Message        string
	// MonitorOnly suppresses external notifications while still tracking the alert.
	MonitorOnly bool
}

func sanitizeAlertKey(label string) string {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return ""
	}

	if trimmed == "/" {
		return "root"
	}

	trimmed = strings.Trim(trimmed, "/\\ ")
	if trimmed == "" {
		trimmed = "root"
	}

	lower := strings.ToLower(trimmed)
	var builder strings.Builder
	builder.Grow(len(lower))
	prevDash := false
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			prevDash = false
			continue
		}
		if r == '.' {
			builder.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			builder.WriteRune('-')
			prevDash = true
		}
	}

	sanitized := strings.Trim(builder.String(), "-.")
	if sanitized == "" {
		sanitized = "disk"
	}

	return sanitized
}

// abs returns the absolute value of a float64.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
