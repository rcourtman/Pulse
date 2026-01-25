package forecast

import (
	"testing"
	"time"
)

type mockStateProvider struct {
	state StateSnapshot
}

func (m mockStateProvider) GetState() StateSnapshot {
	return m.state
}

func buildLinearData(end time.Time, points int, step time.Duration, startValue, stepValue float64) []MetricDataPoint {
	data := make([]MetricDataPoint, points)
	start := end.Add(-time.Duration(points-1) * step)
	for i := 0; i < points; i++ {
		data[i] = MetricDataPoint{
			Timestamp: start.Add(time.Duration(i) * step),
			Value:     startValue + float64(i)*stepValue,
		}
	}
	return data
}

func TestNewService_DefaultsApplied(t *testing.T) {
	svc := NewService(ForecastConfig{})

	if svc.config.ShortTermWindow <= 0 {
		t.Error("expected ShortTermWindow default to be set")
	}
	if svc.config.MediumTermWindow <= 0 {
		t.Error("expected MediumTermWindow default to be set")
	}
	if svc.config.LongTermWindow <= 0 {
		t.Error("expected LongTermWindow default to be set")
	}
	if svc.config.DefaultHorizon <= 0 {
		t.Error("expected DefaultHorizon default to be set")
	}
	if svc.config.MaxHorizon <= 0 {
		t.Error("expected MaxHorizon default to be set")
	}
	if svc.config.StableThreshold <= 0 {
		t.Error("expected StableThreshold default to be set")
	}
	if svc.config.VolatileThreshold <= 0 {
		t.Error("expected VolatileThreshold default to be set")
	}
}

func TestIsPercentageMetric(t *testing.T) {
	cases := map[string]bool{
		"cpu":    true,
		"CPU":    true,
		"memory": true,
		"mem":    true,
		"disk":   true,
		"iops":   false,
	}

	for metric, expected := range cases {
		if got := isPercentageMetric(metric); got != expected {
			t.Errorf("metric %q expected %v, got %v", metric, expected, got)
		}
	}
}

func TestCalculateTrend_Volatile(t *testing.T) {
	svc := NewService(DefaultForecastConfig())
	now := time.Now()
	data := []MetricDataPoint{
		{Timestamp: now.Add(-5 * time.Hour), Value: 10},
		{Timestamp: now.Add(-4 * time.Hour), Value: 120},
		{Timestamp: now.Add(-3 * time.Hour), Value: 15},
		{Timestamp: now.Add(-2 * time.Hour), Value: 130},
		{Timestamp: now.Add(-1 * time.Hour), Value: 20},
		{Timestamp: now, Value: 140},
	}

	trend := svc.calculateTrend(data, DefaultForecastConfig())
	if trend.Direction != TrendVolatile {
		t.Fatalf("expected volatile trend, got %s", trend.Direction)
	}
}

func TestDetectSeasonality_InsufficientData(t *testing.T) {
	svc := NewService(DefaultForecastConfig())
	now := time.Now()
	data := buildLinearData(now, 24, time.Hour, 10, 0)

	if seasonality := svc.detectSeasonality(data); seasonality != nil {
		t.Fatalf("expected nil seasonality for insufficient data")
	}
}

func TestDetectSeasonality_DailyPeaks(t *testing.T) {
	svc := NewService(DefaultForecastConfig())
	base := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	var data []MetricDataPoint
	for day := 0; day < 3; day++ {
		for hour := 0; hour < 24; hour++ {
			value := 1.0
			if hour == 14 {
				value = 100.0
			}
			data = append(data, MetricDataPoint{
				Timestamp: base.Add(time.Duration(day*24+hour) * time.Hour),
				Value:     value,
			})
		}
	}

	seasonality := svc.detectSeasonality(data)
	if seasonality == nil || !seasonality.HasDaily {
		t.Fatalf("expected daily seasonality to be detected")
	}
	if seasonality.HasWeekly {
		t.Fatalf("expected weekly seasonality to be false")
	}
	foundPeak := false
	for _, hour := range seasonality.PeakHours {
		if hour == 14 {
			foundPeak = true
			break
		}
	}
	if !foundPeak {
		t.Fatalf("expected peak hour 14 to be detected")
	}
}

func TestGenerateDescription_WithThresholdAndAcceleration(t *testing.T) {
	svc := NewService(DefaultForecastConfig())
	threshold := 90.0
	timeToThreshold := 4 * time.Hour
	trend := Trend{
		Direction:    TrendIncreasing,
		RatePerDay:   12.5,
		Acceleration: 1.0,
	}

	desc := svc.generateDescription("cpu", 70, 80, trend, &timeToThreshold, threshold)
	if !containsStr(desc, "increasing") {
		t.Fatalf("expected increasing text in description")
	}
	if !containsStr(desc, "Will reach 90% in 4 hours") {
		t.Fatalf("expected time-to-threshold text in description: %s", desc)
	}
	if !containsStr(desc, "accelerating") {
		t.Fatalf("expected accelerating text in description")
	}
}

func TestGenerateDescription_WeeksAndDecelerating(t *testing.T) {
	svc := NewService(DefaultForecastConfig())
	timeToThreshold := 14 * 24 * time.Hour
	trend := Trend{
		Direction:    TrendDecreasing,
		RatePerDay:   -2.0,
		Acceleration: -1.2,
	}

	desc := svc.generateDescription("disk", 80, 60, trend, &timeToThreshold, 50)
	if !containsStr(desc, "decreasing") {
		t.Fatalf("expected decreasing text in description")
	}
	if !containsStr(desc, "weeks") {
		t.Fatalf("expected weeks time-to-threshold text")
	}
	if !containsStr(desc, "decelerating") {
		t.Fatalf("expected decelerating text in description")
	}
}

func TestCalculateConfidence_ClampHigh(t *testing.T) {
	svc := NewService(DefaultForecastConfig())
	now := time.Now()
	data := buildLinearData(now, 1000, time.Minute, 10, 0)
	trend := Trend{Direction: TrendStable}

	confidence := svc.calculateConfidence(data, trend)
	if confidence != 0.95 {
		t.Fatalf("expected confidence to be clamped at 0.95, got %.2f", confidence)
	}
}

func TestCalculateConfidence_VolatileAcceleration(t *testing.T) {
	svc := NewService(DefaultForecastConfig())
	now := time.Now()
	data := buildLinearData(now, 10, time.Minute, 10, 5)
	trend := Trend{Direction: TrendVolatile, Acceleration: 2.0}

	confidence := svc.calculateConfidence(data, trend)
	if confidence >= 0.5 {
		t.Fatalf("expected lower confidence for volatile acceleration, got %.2f", confidence)
	}
}

func TestFormatForContext_LowConfidenceNote(t *testing.T) {
	svc := NewService(DefaultForecastConfig())
	forecasts := []*Forecast{
		{
			ResourceID:  "vm-201",
			Metric:      "cpu",
			Trend:       Trend{Direction: TrendIncreasing},
			Description: "CPU is increasing",
			Confidence:  0.2,
		},
	}

	context := svc.FormatForContext(forecasts)
	if !containsStr(context, "low confidence") {
		t.Fatalf("expected low confidence note in context")
	}
}

func TestFormatKeyForecasts_NoProviders(t *testing.T) {
	svc := NewService(DefaultForecastConfig())

	if result := svc.FormatKeyForecasts(); result != "" {
		t.Fatalf("expected empty result when providers are missing")
	}
}

func TestFormatKeyForecasts_NoStateProvider(t *testing.T) {
	svc := NewService(DefaultForecastConfig())
	svc.SetDataProvider(&mockDataProvider{data: map[string][]MetricDataPoint{}})
	if result := svc.FormatKeyForecasts(); result != "" {
		t.Fatalf("expected empty result when state provider is missing")
	}
}

func TestFormatKeyForecasts_Concerns(t *testing.T) {
	cfg := DefaultForecastConfig()
	cfg.VolatileThreshold = 100.0
	svc := NewService(cfg)

	now := time.Now()
	data := buildLinearData(now, 24, time.Hour, 70, 1.0)

	svc.SetDataProvider(&mockDataProvider{
		data: map[string][]MetricDataPoint{
			"vm-1:cpu": data,
		},
	})
	svc.SetStateProvider(mockStateProvider{
		state: StateSnapshot{
			VMs: []VMInfo{{ID: "vm-1", Name: ""}},
		},
	})

	result := svc.FormatKeyForecasts()
	if result == "" {
		t.Fatalf("expected non-empty result for concerning trends")
	}
	if !containsStr(result, "vm-1") {
		t.Fatalf("expected vm-1 to be mentioned in concerns")
	}
	if !containsStr(result, "increasing") {
		t.Fatalf("expected increasing trend note in concerns")
	}
	if !containsStr(result, "critical") {
		t.Fatalf("expected critical note in concerns")
	}
}

func TestFormatKeyForecasts_AllResourceTypes(t *testing.T) {
	cfg := DefaultForecastConfig()
	cfg.VolatileThreshold = 100.0
	svc := NewService(cfg)

	now := time.Now()
	data := buildLinearData(now, 24, time.Hour, 70, 1.0)

	svc.SetDataProvider(&mockDataProvider{
		data: map[string][]MetricDataPoint{
			"vm-1:cpu":       data,
			"ct-1:cpu":       data,
			"node-1:cpu":     data,
			"storage-1:disk": data,
		},
	})
	svc.SetStateProvider(mockStateProvider{
		state: StateSnapshot{
			VMs:        []VMInfo{{ID: "vm-1", Name: "vm"}},
			Containers: []ContainerInfo{{ID: "ct-1", Name: "ct"}},
			Nodes:      []NodeInfo{{ID: "node-1", Name: "node"}},
			Storage:    []StorageInfo{{ID: "storage-1", Name: ""}},
		},
	})

	result := svc.FormatKeyForecasts()
	if result == "" {
		t.Fatalf("expected formatted concerns")
	}
	if !containsStr(result, "storage-1") {
		t.Fatalf("expected storage to be included")
	}
}

func TestForecastAll_ActionableSorted(t *testing.T) {
	cfg := DefaultForecastConfig()
	cfg.VolatileThreshold = 100.0
	svc := NewService(cfg)

	now := time.Now()
	rapid := buildLinearData(now, 80, time.Hour, 10, 1.0) // current near threshold
	slow := buildLinearData(now, 80, time.Hour, 20, 0.5)  // slower breach
	flat := buildLinearData(now, 80, time.Hour, 60, 0.0)  // non-increasing

	svc.SetDataProvider(&mockDataProvider{
		data: map[string][]MetricDataPoint{
			"vm-fast:disk": rapid,
			"vm-slow:disk": slow,
			"vm-flat:disk": flat,
		},
	})
	svc.SetStateProvider(mockStateProvider{
		state: StateSnapshot{
			VMs: []VMInfo{
				{ID: "vm-fast", Name: "fast"},
				{ID: "vm-slow", Name: "slow"},
				{ID: "vm-flat", Name: "flat"},
			},
		},
	})

	resp, err := svc.ForecastAll("disk", 24*time.Hour, 90)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Forecasts) != 2 {
		t.Fatalf("expected 2 actionable forecasts, got %d", len(resp.Forecasts))
	}
	if resp.Forecasts[0].ResourceID != "vm-fast" {
		t.Fatalf("expected vm-fast to be most urgent, got %s", resp.Forecasts[0].ResourceID)
	}
	if resp.Forecasts[1].ResourceID != "vm-slow" {
		t.Fatalf("expected vm-slow to be second, got %s", resp.Forecasts[1].ResourceID)
	}
}

func TestForecastAll_MissingProviders(t *testing.T) {
	svc := NewService(DefaultForecastConfig())
	if _, err := svc.ForecastAll("disk", time.Hour, 80); err == nil {
		t.Fatalf("expected error when data provider is missing")
	}

	svc.SetDataProvider(&mockDataProvider{data: map[string][]MetricDataPoint{}})
	if _, err := svc.ForecastAll("disk", time.Hour, 80); err == nil {
		t.Fatalf("expected error when state provider is missing")
	}
}

func TestForecastAll_FiltersNonActionable(t *testing.T) {
	cfg := DefaultForecastConfig()
	cfg.VolatileThreshold = 100.0
	svc := NewService(cfg)

	now := time.Now()
	aboveThreshold := buildLinearData(now, 60, time.Hour, 80, 0.3)
	lowConfidence := buildLinearData(now, 5, time.Hour, 10, 1.0)

	svc.SetDataProvider(&mockDataProvider{
		data: map[string][]MetricDataPoint{
			"vm-above:disk": aboveThreshold,
			"vm-low:disk":   lowConfidence,
		},
	})
	svc.SetStateProvider(mockStateProvider{
		state: StateSnapshot{
			VMs: []VMInfo{
				{ID: "vm-above", Name: "above"},
				{ID: "vm-low", Name: "low"},
			},
		},
	})

	resp, err := svc.ForecastAll("disk", 24*time.Hour, 90)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Forecasts) != 0 {
		t.Fatalf("expected no actionable forecasts, got %d", len(resp.Forecasts))
	}
}

func TestForecastAll_MultipleResourceTypes(t *testing.T) {
	cfg := DefaultForecastConfig()
	cfg.VolatileThreshold = 100.0
	svc := NewService(cfg)

	now := time.Now()
	vmData := buildLinearData(now, 60, time.Hour, 20, 0.8)
	ctData := buildLinearData(now, 60, time.Hour, 30, 0.6)
	nodeData := buildLinearData(now, 60, time.Hour, 40, 0.4)

	svc.SetDataProvider(&mockDataProvider{
		data: map[string][]MetricDataPoint{
			"vm-1:disk":   vmData,
			"ct-1:disk":   ctData,
			"node-1:disk": nodeData,
		},
	})
	svc.SetStateProvider(mockStateProvider{
		state: StateSnapshot{
			VMs:        []VMInfo{{ID: "vm-1", Name: "vm"}},
			Containers: []ContainerInfo{{ID: "ct-1", Name: "ct"}},
			Nodes:      []NodeInfo{{ID: "node-1", Name: "node"}},
		},
	})

	resp, err := svc.ForecastAll("disk", 24*time.Hour, 90)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Forecasts) != 3 {
		t.Fatalf("expected forecasts for vm, container, node, got %d", len(resp.Forecasts))
	}
}

func TestForecastAll_SkipsErroredResources(t *testing.T) {
	cfg := DefaultForecastConfig()
	cfg.VolatileThreshold = 100.0
	svc := NewService(cfg)

	now := time.Now()
	vmData := buildLinearData(now, 60, time.Hour, 20, 0.8)

	svc.SetDataProvider(&mockDataProvider{
		data: map[string][]MetricDataPoint{
			"vm-1:disk": vmData,
		},
	})
	svc.SetStateProvider(mockStateProvider{
		state: StateSnapshot{
			VMs:        []VMInfo{{ID: "vm-1", Name: "vm"}},
			Containers: []ContainerInfo{{ID: "ct-1", Name: "ct"}},
		},
	})

	resp, err := svc.ForecastAll("disk", 24*time.Hour, 90)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Forecasts) != 1 {
		t.Fatalf("expected only VM forecast, got %d", len(resp.Forecasts))
	}
}

func TestForecastToOverviewItem(t *testing.T) {
	ttl := 2 * time.Hour
	item := forecastToOverviewItem(&Forecast{
		ResourceID:      "vm-9",
		ResourceName:    "db",
		Metric:          "disk",
		CurrentValue:    70,
		PredictedValue:  85,
		TimeToThreshold: &ttl,
		Confidence:      0.6,
		Trend:           Trend{Direction: TrendIncreasing},
	}, "vm")

	if item.TimeToThreshold == nil || *item.TimeToThreshold == 0 {
		t.Fatalf("expected time to threshold to be converted to seconds")
	}
	if item.ResourceType != "vm" {
		t.Fatalf("expected resource type vm, got %s", item.ResourceType)
	}
}

func TestLinearRegression_Degenerate(t *testing.T) {
	now := time.Now()
	data := []MetricDataPoint{
		{Timestamp: now, Value: 10},
		{Timestamp: now, Value: 20},
		{Timestamp: now, Value: 30},
	}

	slope, intercept := linearRegression(data)
	if slope != 0 {
		t.Fatalf("expected zero slope for degenerate timestamps, got %.2f", slope)
	}
	if intercept != 20 {
		t.Fatalf("expected intercept to be mean (20), got %.2f", intercept)
	}
}

func TestFilterByWindow_InclusiveBounds(t *testing.T) {
	now := time.Now()
	data := []MetricDataPoint{
		{Timestamp: now.Add(-2 * time.Hour), Value: 1},
		{Timestamp: now.Add(-1 * time.Hour), Value: 2},
		{Timestamp: now, Value: 3},
	}

	filtered := filterByWindow(data, now.Add(-2*time.Hour), now)
	if len(filtered) != 3 {
		t.Fatalf("expected inclusive bounds to include all points, got %d", len(filtered))
	}
}
