package alerts

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const minimumMetricWindowSamples = 3

// MetricWindowPoint is one trustworthy observation in a metric series.
type MetricWindowPoint struct {
	Timestamp time.Time
	Value     float64
}

// MetricWindowRequest identifies the canonical series required to evaluate a
// rolling alert rule. Providers may resolve resourceID to a different durable
// metrics identity, but must not substitute a different resource.
type MetricWindowRequest struct {
	ResourceID   string
	ResourceType string
	Metric       string
	Start        time.Time
	End          time.Time
}

// MetricWindowProvider bridges alert policy to monitoring-owned history.
type MetricWindowProvider func(MetricWindowRequest) ([]MetricWindowPoint, error)

type metricWindowObservation struct {
	Value           float64
	CurrentValue    float64
	WindowSeconds   int
	CoverageSeconds int
	SampleCount     int
	Ready           bool
}

// SetMetricWindowProvider installs the monitoring-owned history bridge.
func (m *Manager) SetMetricWindowProvider(provider MetricWindowProvider) {
	m.mu.Lock()
	m.metricWindowProvider = provider
	m.mu.Unlock()
}

func (m *Manager) metricEvaluationWindowNoLock(resourceType, metricType string) int {
	metricKey := strings.ToLower(strings.TrimSpace(metricType))
	for _, typeKey := range CanonicalResourceTypeKeys(resourceType) {
		if perType := m.config.MetricEvaluationWindows[typeKey]; perType != nil {
			if window, ok := perType[metricKey]; ok {
				return window
			}
		}
	}
	if perType := m.config.MetricEvaluationWindows["all"]; perType != nil {
		return perType[metricKey]
	}
	return 0
}

func (m *Manager) evaluateMetricWindow(resourceID, resourceType, metricType string, current float64, observedAt time.Time) metricWindowObservation {
	m.mu.RLock()
	windowSeconds := m.metricEvaluationWindowNoLock(resourceType, metricType)
	provider := m.metricWindowProvider
	m.mu.RUnlock()

	instant := metricWindowObservation{Value: current, CurrentValue: current, Ready: true}
	if windowSeconds <= 0 {
		return instant
	}
	instant.WindowSeconds = windowSeconds
	// Unit tests and embedders that construct an alert manager without the
	// monitoring runtime retain current-value behavior. Production always wires
	// a provider; a wired provider returning insufficient evidence is unknown.
	if provider == nil {
		return instant
	}

	start := observedAt.Add(-time.Duration(windowSeconds) * time.Second)
	points, err := provider(MetricWindowRequest{
		ResourceID: resourceID, ResourceType: resourceType, Metric: metricType,
		Start: start, End: observedAt,
	})
	if err != nil {
		return metricWindowObservation{CurrentValue: current, WindowSeconds: windowSeconds}
	}
	points = append(points, MetricWindowPoint{Timestamp: observedAt, Value: current})
	return calculateMetricWindow(points, start, observedAt, current, windowSeconds)
}

func calculateMetricWindow(points []MetricWindowPoint, start, end time.Time, current float64, windowSeconds int) metricWindowObservation {
	result := metricWindowObservation{CurrentValue: current, WindowSeconds: windowSeconds}
	filtered := make([]MetricWindowPoint, 0, len(points))
	for _, point := range points {
		if point.Timestamp.Before(start) || point.Timestamp.After(end) {
			continue
		}
		filtered = append(filtered, point)
	}
	sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].Timestamp.Before(filtered[j].Timestamp) })

	// Collapse duplicate timestamps, preferring the last value (the current
	// observation appended by the caller wins over an already-persisted point).
	unique := filtered[:0]
	for _, point := range filtered {
		if len(unique) > 0 && unique[len(unique)-1].Timestamp.Equal(point.Timestamp) {
			unique[len(unique)-1] = point
			continue
		}
		unique = append(unique, point)
	}
	result.SampleCount = len(unique)
	if len(unique) < minimumMetricWindowSamples {
		return result
	}

	coverage := unique[len(unique)-1].Timestamp.Sub(unique[0].Timestamp)
	result.CoverageSeconds = int(coverage.Seconds())
	requiredCoverage := time.Duration(windowSeconds) * time.Second * 8 / 10
	maxGap := 2 * time.Minute
	if windowDuration := time.Duration(windowSeconds) * time.Second; windowDuration < maxGap {
		maxGap = windowDuration
	}
	if coverage < requiredCoverage || end.Sub(unique[len(unique)-1].Timestamp) > maxGap {
		return result
	}

	weightedTotal := 0.0
	weightedDuration := time.Duration(0)
	for i := 1; i < len(unique); i++ {
		segment := unique[i].Timestamp.Sub(unique[i-1].Timestamp)
		if segment <= 0 || segment > maxGap {
			return result
		}
		// Trapezoidal integration avoids bias from variable polling cadence.
		weightedTotal += ((unique[i-1].Value + unique[i].Value) / 2) * segment.Seconds()
		weightedDuration += segment
	}
	if weightedDuration <= 0 {
		return result
	}
	result.Value = weightedTotal / weightedDuration.Seconds()
	result.Ready = true
	return result
}

func metricWindowOptions(opts *metricOptions, metricType, resourceType string, observation metricWindowObservation) *metricOptions {
	if observation.WindowSeconds <= 0 {
		return opts
	}
	merged := &metricOptions{}
	if opts != nil {
		*merged = *opts
		merged.Metadata = cloneMetricMetadata(opts.Metadata)
		merged.RemoveMetadata = append([]string(nil), opts.RemoveMetadata...)
	} else {
		merged.Metadata = make(map[string]interface{})
	}
	if merged.Metadata == nil {
		merged.Metadata = make(map[string]interface{})
	}
	merged.Metadata["evaluationMode"] = "rolling_average"
	merged.Metadata["evaluationWindowSeconds"] = observation.WindowSeconds
	merged.Metadata["evaluationCoverageSeconds"] = observation.CoverageSeconds
	merged.Metadata["evaluationSampleCount"] = observation.SampleCount
	merged.Metadata["currentValue"] = observation.CurrentValue
	merged.Metadata["evaluatedValue"] = observation.Value
	if merged.Message == "" {
		label := resourceTypeLabel(resourceType)
		unit := "%"
		if metricType == "diskRead" || metricType == "diskWrite" || metricType == "networkIn" || metricType == "networkOut" {
			unit = " MB/s"
		}
		merged.Message = fmt.Sprintf("%s %s %s average at %.1f%s (current %.1f%s)", label, metricType, metricWindowLabel(observation.WindowSeconds), observation.Value, unit, observation.CurrentValue, unit)
	}
	return merged
}

func cloneMetricMetadata(input map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(input)+6)
	for key, value := range input {
		result[key] = value
	}
	return result
}

func metricWindowLabel(seconds int) string {
	if seconds%3600 == 0 {
		return fmt.Sprintf("%d-hour", seconds/3600)
	}
	if seconds%60 == 0 {
		return fmt.Sprintf("%d-minute", seconds/60)
	}
	return fmt.Sprintf("%d-second", seconds)
}
