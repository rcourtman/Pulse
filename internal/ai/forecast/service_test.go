package forecast

import (
	"fmt"
	"testing"
	"time"
)

// Mock data provider
type mockDataProvider struct {
	data map[string][]MetricDataPoint
}

func (m *mockDataProvider) GetMetricHistory(resourceID, metric string, from, to time.Time) ([]MetricDataPoint, error) {
	key := resourceID + ":" + metric
	if data, ok := m.data[key]; ok {
		// Filter by time range
		var result []MetricDataPoint
		for _, dp := range data {
			if (dp.Timestamp.Equal(from) || dp.Timestamp.After(from)) &&
				(dp.Timestamp.Equal(to) || dp.Timestamp.Before(to)) {
				result = append(result, dp)
			}
		}
		return result, nil
	}
	return nil, fmt.Errorf("no data")
}

func TestService_Forecast_Increasing(t *testing.T) {
	cfg := DefaultForecastConfig()
	cfg.VolatileThreshold = 50.0 // Higher threshold to allow more variance
	svc := NewService(cfg)

	// Create increasing data with small noise
	now := time.Now()
	data := make([]MetricDataPoint, 200)
	for i := 0; i < 200; i++ {
		data[i] = MetricDataPoint{
			Timestamp: now.Add(-time.Duration(200-i) * time.Hour),
			Value:     50.0 + float64(i)*0.1, // Gentle increasing trend: 50 -> 70
		}
	}

	svc.SetDataProvider(&mockDataProvider{
		data: map[string][]MetricDataPoint{
			"vm-101:cpu": data,
		},
	})

	forecast, err := svc.Forecast("vm-101", "web-server", "cpu", 24*time.Hour, 90)
	if err != nil {
		t.Fatalf("Forecast failed: %v", err)
	}

	// With a positive rate, predicted should be higher
	if forecast.PredictedValue <= forecast.CurrentValue {
		t.Errorf("Expected predicted value > current value for increasing data")
	}

	// Rate per day should be positive
	if forecast.Trend.RatePerDay <= 0 {
		t.Errorf("Expected positive rate per day, got %.2f", forecast.Trend.RatePerDay)
	}
}

func TestService_Forecast_Decreasing(t *testing.T) {
	cfg := DefaultForecastConfig()
	cfg.VolatileThreshold = 50.0 // Higher threshold to allow more variance
	svc := NewService(cfg)

	now := time.Now()
	data := make([]MetricDataPoint, 200)
	for i := 0; i < 200; i++ {
		data[i] = MetricDataPoint{
			Timestamp: now.Add(-time.Duration(200-i) * time.Hour),
			Value:     80.0 - float64(i)*0.1, // Gentle decreasing trend: 80 -> 60
		}
	}

	svc.SetDataProvider(&mockDataProvider{
		data: map[string][]MetricDataPoint{
			"vm-101:cpu": data,
		},
	})

	forecast, err := svc.Forecast("vm-101", "web-server", "cpu", 24*time.Hour, 0)
	if err != nil {
		t.Fatalf("Forecast failed: %v", err)
	}

	// With a negative rate, predicted should be lower
	if forecast.PredictedValue >= forecast.CurrentValue {
		t.Errorf("Expected predicted value < current value for decreasing data")
	}

	// Rate per day should be negative
	if forecast.Trend.RatePerDay >= 0 {
		t.Errorf("Expected negative rate per day, got %.2f", forecast.Trend.RatePerDay)
	}
}

func TestService_Forecast_Stable(t *testing.T) {
	svc := NewService(DefaultForecastConfig())

	now := time.Now()
	data := make([]MetricDataPoint, 100)
	for i := 0; i < 100; i++ {
		data[i] = MetricDataPoint{
			Timestamp: now.Add(-time.Duration(100-i) * time.Hour),
			Value:     50.0, // Constant
		}
	}

	svc.SetDataProvider(&mockDataProvider{
		data: map[string][]MetricDataPoint{
			"vm-101:cpu": data,
		},
	})

	forecast, err := svc.Forecast("vm-101", "", "cpu", 24*time.Hour, 0)
	if err != nil {
		t.Fatalf("Forecast failed: %v", err)
	}

	if forecast.Trend.Direction != TrendStable {
		t.Errorf("Expected stable trend, got %s", forecast.Trend.Direction)
	}
}

func TestService_Forecast_TimeToThreshold(t *testing.T) {
	svc := NewService(DefaultForecastConfig())

	now := time.Now()
	data := make([]MetricDataPoint, 200)
	for i := 0; i < 200; i++ {
		data[i] = MetricDataPoint{
			Timestamp: now.Add(-time.Duration(200-i) * time.Hour),
			Value:     50.0 + float64(i)*0.2, // Slowly increasing
		}
	}

	svc.SetDataProvider(&mockDataProvider{
		data: map[string][]MetricDataPoint{
			"storage-1:disk": data,
		},
	})

	forecast, err := svc.Forecast("storage-1", "local-zfs", "disk", 24*time.Hour, 95)
	if err != nil {
		t.Fatalf("Forecast failed: %v", err)
	}

	if forecast.TimeToThreshold == nil {
		t.Error("Expected time to threshold to be calculated")
	}
}

func TestService_Forecast_NoProvider(t *testing.T) {
	svc := NewService(DefaultForecastConfig())

	_, err := svc.Forecast("vm-101", "", "cpu", 24*time.Hour, 0)
	if err == nil {
		t.Error("Expected error when no provider configured")
	}
}

func TestService_Forecast_InsufficientData(t *testing.T) {
	svc := NewService(DefaultForecastConfig())

	svc.SetDataProvider(&mockDataProvider{
		data: map[string][]MetricDataPoint{
			"vm-101:cpu": {{Timestamp: time.Now(), Value: 50}}, // Only 1 point
		},
	})

	_, err := svc.Forecast("vm-101", "", "cpu", 24*time.Hour, 0)
	if err == nil {
		t.Error("Expected error with insufficient data")
	}
}

func TestService_Forecast_Confidence(t *testing.T) {
	svc := NewService(DefaultForecastConfig())

	now := time.Now()
	// Lots of stable data = high confidence
	data := make([]MetricDataPoint, 500)
	for i := 0; i < 500; i++ {
		data[i] = MetricDataPoint{
			Timestamp: now.Add(-time.Duration(500-i) * time.Hour),
			Value:     50.0,
		}
	}

	svc.SetDataProvider(&mockDataProvider{
		data: map[string][]MetricDataPoint{
			"vm-101:cpu": data,
		},
	})

	forecast, _ := svc.Forecast("vm-101", "", "cpu", 24*time.Hour, 0)

	if forecast.Confidence < 0.5 {
		t.Errorf("Expected high confidence with stable data, got %.2f", forecast.Confidence)
	}
}

func TestService_FormatForContext(t *testing.T) {
	svc := NewService(DefaultForecastConfig())

	forecasts := []*Forecast{
		{
			ResourceID:  "vm-101",
			Metric:      "cpu",
			Trend:       Trend{Direction: TrendIncreasing},
			Description: "CPU is increasing (5%/day)",
			Confidence:  0.8,
		},
		{
			ResourceID:  "storage-1",
			Metric:      "disk",
			Trend:       Trend{Direction: TrendStable},
			Description: "Disk usage is stable",
			Confidence:  0.9,
		},
	}

	context := svc.FormatForContext(forecasts)

	if context == "" {
		t.Error("Expected non-empty context")
	}

	if !containsStr(context, "Forecasts") {
		t.Error("Expected 'Forecasts' in context")
	}

	if !containsStr(context, "increasing") {
		t.Error("Expected trend info in context")
	}
}

func TestService_FormatForContext_Empty(t *testing.T) {
	svc := NewService(DefaultForecastConfig())

	context := svc.FormatForContext(nil)
	if context != "" {
		t.Error("Expected empty context for nil forecasts")
	}

	context = svc.FormatForContext([]*Forecast{})
	if context != "" {
		t.Error("Expected empty context for empty forecasts")
	}
}

func TestLinearRegression(t *testing.T) {
	// Test with known data
	now := time.Now()
	data := []MetricDataPoint{
		{Timestamp: now, Value: 10},
		{Timestamp: now.Add(time.Hour), Value: 20},
		{Timestamp: now.Add(2 * time.Hour), Value: 30},
	}

	slope, _ := linearRegression(data)

	// Slope should be 10 per hour = 10/3600 per second
	expectedSlope := 10.0 / 3600.0
	tolerance := 0.001
	if slope < expectedSlope-tolerance || slope > expectedSlope+tolerance {
		t.Errorf("Expected slope ~%.6f, got %.6f", expectedSlope, slope)
	}
}

func TestMean(t *testing.T) {
	data := []MetricDataPoint{
		{Value: 10},
		{Value: 20},
		{Value: 30},
	}

	result := mean(data)
	if result != 20.0 {
		t.Errorf("Expected mean 20, got %.1f", result)
	}
}

func TestMean_Empty(t *testing.T) {
	result := mean([]MetricDataPoint{})
	if result != 0 {
		t.Errorf("Expected 0 for empty slice, got %.1f", result)
	}
}

func TestStandardDeviation(t *testing.T) {
	data := []MetricDataPoint{
		{Value: 10},
		{Value: 20},
		{Value: 30},
	}

	result := standardDeviation(data)
	if result <= 0 {
		t.Errorf("Expected positive std dev, got %.1f", result)
	}
}

func TestFilterByWindow(t *testing.T) {
	now := time.Now()
	data := []MetricDataPoint{
		{Timestamp: now.Add(-2 * time.Hour), Value: 1},
		{Timestamp: now.Add(-1 * time.Hour), Value: 2},
		{Timestamp: now, Value: 3},
	}

	filtered := filterByWindow(data, now.Add(-90*time.Minute), now)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 points in window, got %d", len(filtered))
	}
}

// Helper
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
