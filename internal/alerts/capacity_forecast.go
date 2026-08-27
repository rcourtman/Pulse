package alerts

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

const (
	capacityForecastLookback       = 7 * 24 * time.Hour
	capacityForecastBucketWidth    = time.Hour
	capacityForecastMinimumSpan    = 24 * time.Hour
	capacityForecastFreshness      = 2 * time.Hour
	capacityForecastMinimumBuckets = 12
	capacityForecastMinimumRate    = 0.1
	capacityForecastMinConfidence  = 0.80
	capacityForecastWarningHorizon = 7 * 24 * time.Hour
	capacityForecastCriticalWindow = 24 * time.Hour
	capacityForecastRecoveryWindow = 14 * 24 * time.Hour
)

// CapacityMetricPoint is one percentage-utilization observation used to
// estimate when a capacity-backed resource will fill. Callers may provide raw
// samples at any cadence; the estimator normalizes them into hourly medians so
// a fast poller cannot manufacture confidence by repeating nearly identical
// observations.
type CapacityMetricPoint struct {
	Timestamp time.Time
	Value     float64
}

// CapacityTrendObservation is detector evidence, not an alert decision.
// Ready means the evidence coverage is sufficient to say whether a trend is
// actionable. A non-ready observation must not be interpreted as recovery.
type CapacityTrendObservation struct {
	Ready        bool
	Reason       string
	ObservedAt   time.Time
	DailyChange  float64
	Confidence   float64
	SampleCount  int
	BucketCount  int
	CoverageSpan time.Duration
}

type capacityBucketPoint struct {
	timestamp time.Time
	value     float64
}

// EstimateCapacityTrend produces a conservative, time-aware capacity trend.
// Alert policy (warning/critical horizons, threshold coexistence, and
// hysteresis) remains in Manager; this function only establishes trustworthy
// trend evidence.
func EstimateCapacityTrend(points []CapacityMetricPoint, now time.Time) CapacityTrendObservation {
	if now.IsZero() {
		now = time.Now()
	}
	result := CapacityTrendObservation{Reason: "insufficient-history"}
	if len(points) == 0 {
		return result
	}

	cutoff := now.Add(-capacityForecastLookback)
	buckets := make(map[int64][]float64)
	validSamples := 0
	latest := time.Time{}
	for _, point := range points {
		if point.Timestamp.IsZero() || point.Timestamp.Before(cutoff) || point.Timestamp.After(now.Add(5*time.Minute)) {
			continue
		}
		if math.IsNaN(point.Value) || math.IsInf(point.Value, 0) || point.Value < 0 || point.Value > 100 {
			continue
		}
		bucket := point.Timestamp.Unix() / int64(capacityForecastBucketWidth/time.Second)
		buckets[bucket] = append(buckets[bucket], point.Value)
		validSamples++
		if point.Timestamp.After(latest) {
			latest = point.Timestamp
		}
	}
	result.SampleCount = validSamples
	if validSamples == 0 || latest.IsZero() {
		result.Reason = "no-valid-samples"
		return result
	}
	result.ObservedAt = latest
	if now.Sub(latest) > capacityForecastFreshness {
		result.Reason = "stale-history"
		return result
	}

	normalized := make([]capacityBucketPoint, 0, len(buckets))
	for bucket, values := range buckets {
		sort.Float64s(values)
		median := values[len(values)/2]
		if len(values)%2 == 0 {
			median = (values[len(values)/2-1] + values[len(values)/2]) / 2
		}
		normalized = append(normalized, capacityBucketPoint{
			timestamp: time.Unix(bucket*int64(capacityForecastBucketWidth/time.Second), 0),
			value:     median,
		})
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].timestamp.Before(normalized[j].timestamp)
	})
	result.BucketCount = len(normalized)
	if len(normalized) < capacityForecastMinimumBuckets {
		result.Reason = "insufficient-hourly-coverage"
		return result
	}
	result.CoverageSpan = normalized[len(normalized)-1].timestamp.Sub(normalized[0].timestamp)
	if result.CoverageSpan < capacityForecastMinimumSpan {
		result.Reason = "history-window-too-short"
		return result
	}

	overallSlope, overallR2 := capacityLinearRegression(normalized)
	recentStart := len(normalized) / 2
	if len(normalized)-recentStart < 8 {
		recentStart = len(normalized) - 8
	}
	recentSlope, _ := capacityLinearRegression(normalized[recentStart:])
	overallDaily := overallSlope * 24
	recentDaily := recentSlope * 24
	result.DailyChange = overallDaily

	// A historic rise that has flattened or reversed is not an impending
	// exhaustion signal. Requiring both windows to rise also rejects one-off
	// capacity jumps caused by a resize or a telemetry discontinuity.
	if overallDaily <= capacityForecastMinimumRate || recentDaily <= capacityForecastMinimumRate {
		result.Ready = true
		result.Reason = "not-increasing"
		return result
	}

	spanFactor := math.Min(1, result.CoverageSpan.Hours()/48)
	agreement := math.Min(overallDaily, recentDaily) / math.Max(overallDaily, recentDaily)
	result.Confidence = clampCapacityConfidence(overallR2 * spanFactor * agreement)
	result.Ready = true
	if result.Confidence < capacityForecastMinConfidence {
		result.Reason = "low-confidence"
		return result
	}
	result.Reason = "increasing"
	return result
}

func capacityLinearRegression(points []capacityBucketPoint) (slopePerHour, rSquared float64) {
	if len(points) < 2 {
		return 0, 0
	}
	start := points[0].timestamp
	n := float64(len(points))
	var sumX, sumY, sumXY, sumX2 float64
	for _, point := range points {
		x := point.timestamp.Sub(start).Hours()
		sumX += x
		sumY += point.value
		sumXY += x * point.value
		sumX2 += x * x
	}
	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0, 0
	}
	slopePerHour = (n*sumXY - sumX*sumY) / denominator
	intercept := (sumY - slopePerHour*sumX) / n
	meanY := sumY / n
	var residual, total float64
	for _, point := range points {
		x := point.timestamp.Sub(start).Hours()
		predicted := intercept + slopePerHour*x
		residual += math.Pow(point.value-predicted, 2)
		total += math.Pow(point.value-meanY, 2)
	}
	if total == 0 {
		return slopePerHour, 0
	}
	return slopePerHour, clampCapacityConfidence(1 - residual/total)
}

func clampCapacityConfidence(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

const (
	capacityAlertOriginKey       = "capacityAlertOrigin"
	capacityAlertOriginThreshold = "threshold"
	capacityAlertOriginForecast  = "forecast"
)

var capacityForecastMetadataKeys = []string{
	"forecastConfidence",
	"forecastDailyChangePct",
	"forecastDaysToFull",
	"forecastObservedAt",
	"forecastSampleCount",
	"forecastBucketCount",
	"forecastCoverageSeconds",
}

func (m *Manager) evaluateStorageCapacity(storage models.Storage, thresholds ThresholdConfig, trend CapacityTrendObservation) {
	input := &UnifiedResourceInput{
		ID:       storage.ID,
		Type:     "storage",
		Name:     storage.Name,
		Node:     storage.Node,
		Instance: storage.Instance,
		Disk:     &UnifiedResourceMetric{Percent: storage.Usage},
	}
	m.evaluateUnifiedCapacity(input, thresholds, trend, func() bool {
		if !m.config.Enabled {
			return true
		}
		allDisabled, _ := m.alertPolicyTypeSwitchesNoLock("storage")
		return allDisabled || m.resolveStorageThresholdsNoLock(storage).Disabled
	})
}

func (m *Manager) evaluateUnifiedCapacity(input *UnifiedResourceInput, thresholds ThresholdConfig, trend CapacityTrendObservation, policyDisabledNoLock func() bool) {
	if input == nil || input.Disk == nil {
		return
	}
	threshold := thresholds.Usage
	if threshold == nil || threshold.Trigger <= 0 {
		m.evaluateUnifiedMetrics(input, thresholds, nil)
		return
	}

	current := input.DiskValue()
	active, origin := m.activeStorageCapacityOrigin(input.ID)
	staticTriggered := current >= threshold.Trigger
	staticLatched := active && origin == capacityAlertOriginThreshold && threshold.Clear > 0 && current >= threshold.Clear
	if staticTriggered || staticLatched {
		m.evaluateUnifiedMetrics(input, thresholds, &metricOptions{
			Metadata: map[string]interface{}{
				capacityAlertOriginKey: capacityAlertOriginThreshold,
			},
			RemoveMetadata: capacityForecastMetadataKeys,
		})
		return
	}

	// Insufficient or low-confidence history is unknown, not recovery. Keep an
	// already-firing forecast occurrence untouched until trustworthy evidence
	// says the risk has receded; normal stale-alert cleanup remains the final
	// bound if telemetry never becomes usable again.
	recoveryFloor := threshold.Clear
	if recoveryFloor <= 0 {
		recoveryFloor = threshold.Trigger
	}
	if active && origin == capacityAlertOriginForecast && current >= recoveryFloor && (!trend.Ready || trend.Reason == "low-confidence") {
		return
	}

	eta, hasETA := capacityTimeToFull(current, trend.DailyChange)
	forecastTrusted := trend.Ready && trend.Confidence >= capacityForecastMinConfidence && hasETA
	forecastTriggered := forecastTrusted && eta <= capacityForecastWarningHorizon
	forecastLatched := active && origin == capacityAlertOriginForecast && forecastTrusted && eta <= capacityForecastRecoveryWindow
	if forecastTriggered || forecastLatched {
		m.evaluateCapacityForecast(input, thresholds, trend, eta, policyDisabledNoLock)
		return
	}

	// No predictive risk remains. Run the ordinary metric evaluator so a
	// forecast recovery and every existing static hysteresis rule retain the
	// same canonical state, history, and notification behavior.
	m.evaluateUnifiedMetrics(input, thresholds, &metricOptions{
		Metadata: map[string]interface{}{
			capacityAlertOriginKey: capacityAlertOriginThreshold,
		},
		RemoveMetadata: capacityForecastMetadataKeys,
	})
}

func (m *Manager) activeStorageCapacityOrigin(resourceID string) (bool, string) {
	trackingKey := canonicalMetricStateID(resourceID, "usage")
	m.mu.RLock()
	defer m.mu.RUnlock()
	alert, exists := m.getActiveAlertNoLock(trackingKey)
	if !exists || alert == nil {
		return false, ""
	}
	if alert.Metadata == nil {
		return true, capacityAlertOriginThreshold
	}
	origin, _ := alert.Metadata[capacityAlertOriginKey].(string)
	if origin == "" {
		origin = capacityAlertOriginThreshold
	}
	return true, origin
}

func capacityTimeToFull(current, dailyChange float64) (time.Duration, bool) {
	if current < 0 || current >= 100 || dailyChange <= capacityForecastMinimumRate || math.IsNaN(dailyChange) || math.IsInf(dailyChange, 0) {
		return 0, false
	}
	days := (100 - current) / dailyChange
	if days <= 0 || math.IsNaN(days) || math.IsInf(days, 0) {
		return 0, false
	}
	return time.Duration(days * float64(24*time.Hour)), true
}

func (m *Manager) evaluateCapacityForecast(input *UnifiedResourceInput, thresholds ThresholdConfig, trend CapacityTrendObservation, eta time.Duration, policyDisabledNoLock func() bool) {
	resourceID := input.ID
	specID := canonicalMetricSpecID(resourceID, "usage")
	resourceType, ok := unifiedMetricResourceType(input.Type)
	if !ok {
		return
	}
	spec, err := buildCanonicalSeverityThresholdSpec(
		specID,
		resourceID,
		input.Name,
		resourceType,
		"capacity-risk",
		1,
		2,
		false,
	)
	if err != nil {
		log.Warn().Err(err).Str("resourceID", input.ID).Msg("Skipping invalid storage capacity forecast spec")
		return
	}
	spec.ConfirmationsRequired = 2
	if err := spec.Validate(); err != nil {
		log.Warn().Err(err).Str("resourceID", input.ID).Msg("Skipping invalid confirmed storage capacity forecast spec")
		return
	}

	riskScore := 1.0
	if eta <= capacityForecastCriticalWindow {
		riskScore = 2
	}
	observedAt := m.policyNow()
	daysToFull := eta.Hours() / 24
	message := fmt.Sprintf(
		"%s projected to fill in %s (%.1f%% used, +%.2f%%/day)",
		unifiedAlertType(input.Type),
		formatCapacityETA(eta),
		input.DiskValue(),
		trend.DailyChange,
	)
	attributes := map[string]string{
		"capacity_alert_origin":    capacityAlertOriginForecast,
		"confidence":               strconv.FormatFloat(trend.Confidence, 'f', 3, 64),
		"current_usage_percent":    strconv.FormatFloat(input.DiskValue(), 'f', 2, 64),
		"daily_change_percent":     strconv.FormatFloat(trend.DailyChange, 'f', 3, 64),
		"forecast_days_to_full":    strconv.FormatFloat(daysToFull, 'f', 2, 64),
		"history_bucket_count":     strconv.Itoa(trend.BucketCount),
		"history_coverage_seconds": strconv.FormatInt(int64(trend.CoverageSpan/time.Second), 10),
	}
	metadata := map[string]interface{}{
		"resourceType":            input.Type,
		"clearThreshold":          thresholds.Usage.Clear,
		capacityAlertOriginKey:    capacityAlertOriginForecast,
		"forecastConfidence":      trend.Confidence,
		"forecastDailyChangePct":  trend.DailyChange,
		"forecastDaysToFull":      daysToFull,
		"forecastObservedAt":      trend.ObservedAt,
		"forecastSampleCount":     trend.SampleCount,
		"forecastBucketCount":     trend.BucketCount,
		"forecastCoverageSeconds": int64(trend.CoverageSpan / time.Second),
	}
	_, _ = m.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt: observedAt,
			Summary:    message,
			Attributes: attributes,
			SeverityThreshold: &alertspecs.SeverityThresholdEvidence{
				Metric:    "capacity-risk",
				Direction: alertspecs.ThresholdDirectionAbove,
				Observed:  riskScore,
			},
		},
		IntentSignal:                 MetricAlertIntentSignal("usage"),
		PolicyDisabledNoLock:         policyDisabledNoLock,
		AlertID:                      canonicalMetricStateID(resourceID, "usage"),
		AlertType:                    "usage",
		ResourceID:                   resourceID,
		ResourceName:                 input.Name,
		Node:                         input.Node,
		Instance:                     input.Instance,
		Message:                      message,
		Value:                        input.DiskValue(),
		Threshold:                    100,
		Metadata:                     metadata,
		AddToRecent:                  true,
		AddToHistory:                 true,
		RateLimit:                    true,
		NotifyOnSeverityChange:       true,
		AddToHistoryOnSeverityChange: true,
	})
}

func formatCapacityETA(eta time.Duration) string {
	if eta < 2*time.Hour {
		return "about 1 hour"
	}
	if eta < 24*time.Hour {
		return fmt.Sprintf("about %.0f hours", math.Ceil(eta.Hours()))
	}
	days := int(math.Ceil(eta.Hours() / 24))
	if days == 1 {
		return "about 1 day"
	}
	return fmt.Sprintf("about %d days", days)
}
