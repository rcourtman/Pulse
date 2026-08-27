package alerts

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestEstimateCapacityTrendRequiresCoverageAndRecentAgreement(t *testing.T) {
	now := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC)

	t.Run("trusts a clean multi-day fill trend", func(t *testing.T) {
		points := make([]CapacityMetricPoint, 0, 73)
		for hour := 72; hour >= 0; hour-- {
			age := time.Duration(hour) * time.Hour
			points = append(points, CapacityMetricPoint{
				Timestamp: now.Add(-age),
				Value:     55 + float64(72-hour)*0.08,
			})
		}
		trend := EstimateCapacityTrend(points, now)
		if !trend.Ready || trend.Reason != "increasing" {
			t.Fatalf("trend = %+v, want trusted increasing evidence", trend)
		}
		if math.Abs(trend.DailyChange-1.92) > 0.02 {
			t.Fatalf("DailyChange = %.3f, want about 1.92", trend.DailyChange)
		}
		if trend.Confidence < capacityForecastMinConfidence {
			t.Fatalf("Confidence = %.3f, want >= %.2f", trend.Confidence, capacityForecastMinConfidence)
		}
	})

	t.Run("rejects dense samples without a full-day span", func(t *testing.T) {
		points := make([]CapacityMetricPoint, 0, 300)
		for i := 0; i < 300; i++ {
			points = append(points, CapacityMetricPoint{
				Timestamp: now.Add(-time.Duration(300-i) * time.Minute),
				Value:     50 + float64(i)*0.02,
			})
		}
		trend := EstimateCapacityTrend(points, now)
		if trend.Ready || trend.Reason != "insufficient-hourly-coverage" {
			t.Fatalf("trend = %+v, want insufficient hourly coverage", trend)
		}
	})

	t.Run("does not extrapolate a historic jump after growth stops", func(t *testing.T) {
		points := make([]CapacityMetricPoint, 0, 73)
		for hour := 72; hour >= 0; hour-- {
			elapsed := 72 - hour
			value := 50.0
			if elapsed >= 24 {
				value = 70
			}
			points = append(points, CapacityMetricPoint{Timestamp: now.Add(-time.Duration(hour) * time.Hour), Value: value})
		}
		trend := EstimateCapacityTrend(points, now)
		if !trend.Ready || trend.Reason != "not-increasing" {
			t.Fatalf("trend = %+v, want a ready non-increasing decision", trend)
		}
	})
}

func TestStorageForecastSharesLifecycleWithStaticUsageAlert(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.SuppressionWindow = 0
	m.config.MinimumDelta = 0
	m.config.ActivationState = ActivationActive
	m.config.StorageDefault = HysteresisThreshold{Trigger: 80, Clear: 70}
	m.mu.Unlock()

	deliveries := 0
	resolved := 0
	m.SetAlertCallback(func(*Alert) { deliveries++ })
	m.SetResolvedCallback(func(string) { resolved++ })

	storage := models.Storage{
		ID:       "storage-forecast-1",
		Name:     "archive",
		Node:     "pve1",
		Instance: "lab",
		Status:   "active",
		Usage:    72,
	}
	trend := CapacityTrendObservation{
		Ready:        true,
		Reason:       "increasing",
		ObservedAt:   time.Now(),
		DailyChange:  5,
		Confidence:   0.98,
		SampleCount:  400,
		BucketCount:  72,
		CoverageSpan: 72 * time.Hour,
	}

	// Forecast alerts require two independent evaluation cycles.
	m.CheckStorageWithCapacityTrend(storage, trend)
	if testHasActiveAlert(t, m, canonicalMetricStateID(storage.ID, "usage")) {
		t.Fatal("forecast activated before its confirmation floor")
	}
	m.CheckStorageWithCapacityTrend(storage, trend)
	alert := testRequireActiveAlert(t, m, canonicalMetricStateID(storage.ID, "usage"))
	if alert.Type != "usage" || alert.Level != AlertLevelWarning {
		t.Fatalf("forecast alert = type %q level %q, want usage warning", alert.Type, alert.Level)
	}
	if got := alert.Metadata[capacityAlertOriginKey]; got != capacityAlertOriginForecast {
		t.Fatalf("capacity origin = %v, want forecast", got)
	}
	if !strings.Contains(alert.Message, "projected to fill") {
		t.Fatalf("message = %q, want predictive explanation", alert.Message)
	}
	if deliveries != 1 {
		t.Fatalf("deliveries = %d, want one forecast activation", deliveries)
	}
	start := alert.StartTime
	if err := m.AcknowledgeAlert(alert.ID, "operator"); err != nil {
		t.Fatalf("acknowledge forecast: %v", err)
	}

	// A real threshold breach upgrades the same occurrence. It must not emit a
	// forecast recovery or a second unacknowledged activation.
	storage.Usage = 91
	m.CheckStorageWithCapacityTrend(storage, trend)
	upgraded := testRequireActiveAlert(t, m, canonicalMetricStateID(storage.ID, "usage"))
	if upgraded.StartTime != start {
		t.Fatalf("occurrence start changed from %s to %s", start, upgraded.StartTime)
	}
	if !upgraded.Acknowledged || upgraded.AckUser != "operator" {
		t.Fatalf("acknowledgement was not preserved: %+v", upgraded)
	}
	if upgraded.Level != AlertLevelCritical {
		t.Fatalf("upgraded level = %q, want critical", upgraded.Level)
	}
	if got := upgraded.Metadata[capacityAlertOriginKey]; got != capacityAlertOriginThreshold {
		t.Fatalf("capacity origin = %v, want threshold", got)
	}
	if _, exists := upgraded.Metadata["forecastDaysToFull"]; exists {
		t.Fatal("stale forecast metadata survived the static threshold transition")
	}
	if deliveries != 1 {
		t.Fatalf("deliveries = %d, want acknowledged transition not to page again", deliveries)
	}
	if resolved != 0 {
		t.Fatalf("resolved callbacks = %d, forecast-to-threshold transition must stay one incident", resolved)
	}
	if got := len(m.GetActiveAlerts()); got != 1 {
		t.Fatalf("active alerts = %d, want one canonical capacity incident", got)
	}
}

func TestStorageForecastTreatsUncertainEvidenceAsUnknown(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.StorageDefault = HysteresisThreshold{Trigger: 80, Clear: 70}
	m.mu.Unlock()

	storage := models.Storage{ID: "storage-forecast-unknown", Name: "media", Status: "active", Usage: 70}
	trusted := CapacityTrendObservation{
		Ready: true, Reason: "increasing", ObservedAt: time.Now(), DailyChange: 6,
		Confidence: 0.99, SampleCount: 300, BucketCount: 48, CoverageSpan: 48 * time.Hour,
	}
	m.CheckStorageWithCapacityTrend(storage, trusted)
	m.CheckStorageWithCapacityTrend(storage, trusted)
	before := testRequireActiveAlert(t, m, canonicalMetricStateID(storage.ID, "usage"))

	uncertain := trusted
	uncertain.Confidence = 0.3
	uncertain.Reason = "low-confidence"
	m.CheckStorageWithCapacityTrend(storage, uncertain)
	after := testRequireActiveAlert(t, m, canonicalMetricStateID(storage.ID, "usage"))
	if after.StartTime != before.StartTime {
		t.Fatal("uncertain evidence replaced the active forecast occurrence")
	}

	recovered := trusted
	recovered.DailyChange = 0
	recovered.Confidence = 0
	recovered.Reason = "not-increasing"
	m.CheckStorageWithCapacityTrend(storage, recovered)
	if testHasActiveAlert(t, m, canonicalMetricStateID(storage.ID, "usage")) {
		t.Fatal("positive non-increasing evidence did not recover the forecast")
	}
}

func TestUnifiedStorageForecastUsesPlatformPolicyAndCanonicalIdentity(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()
	usage := &HysteresisThreshold{Trigger: 85, Clear: 75}
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.TrueNASDefaults.Usage = usage
	m.mu.Unlock()

	input := &UnifiedResourceInput{
		ID:       "truenas:atlas/pool:tank",
		Type:     "truenas-pool",
		Name:     "tank",
		Node:     "atlas",
		Instance: "TrueNAS",
		Disk:     &UnifiedResourceMetric{Percent: 70},
	}
	trend := CapacityTrendObservation{
		Ready: true, Reason: "increasing", ObservedAt: time.Now(), DailyChange: 7,
		Confidence: 0.99, SampleCount: 300, BucketCount: 48, CoverageSpan: 48 * time.Hour,
	}
	m.CheckUnifiedResourceWithCapacityTrend(input, trend)
	m.CheckUnifiedResourceWithCapacityTrend(input, trend)

	alert := testRequireActiveAlert(t, m, canonicalMetricStateID(input.ID, "usage"))
	if alert.ResourceID != input.ID || alert.Type != "usage" {
		t.Fatalf("alert identity = resource %q type %q", alert.ResourceID, alert.Type)
	}
	if got := alert.Metadata["resourceType"]; got != "truenas-pool" {
		t.Fatalf("resourceType metadata = %v, want truenas-pool", got)
	}
	if got := alert.Metadata[capacityAlertOriginKey]; got != capacityAlertOriginForecast {
		t.Fatalf("capacity origin = %v, want forecast", got)
	}
}
