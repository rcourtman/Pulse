package context

import (
	"testing"
	"time"
)

func TestComputeTrend_Growing(t *testing.T) {
	// Create growing data (10% per day)
	now := time.Now()
	points := make([]MetricPoint, 24)
	for i := 0; i < 24; i++ {
		// 10% per day = ~0.417% per hour
		points[i] = MetricPoint{
			Value:     50 + float64(i)*0.417,
			Timestamp: now.Add(time.Duration(-24+i) * time.Hour),
		}
	}

	trend := ComputeTrend(points, "memory", 24*time.Hour)

	if trend.Direction != TrendGrowing {
		t.Errorf("Expected TrendGrowing, got %s", trend.Direction)
	}

	// Rate should be ~10% per day
	if trend.RatePerDay < 8 || trend.RatePerDay > 12 {
		t.Errorf("Expected rate ~10/day, got %.2f", trend.RatePerDay)
	}

	if trend.DataPoints != 24 {
		t.Errorf("Expected 24 data points, got %d", trend.DataPoints)
	}
}

func TestComputeTrend_Stable(t *testing.T) {
	// Create stable data with small fluctuations
	now := time.Now()
	points := make([]MetricPoint, 24)
	for i := 0; i < 24; i++ {
		// Small random-looking variation around 50%, but no trend
		offset := float64(i%3 - 1) * 0.2
		points[i] = MetricPoint{
			Value:     50 + offset,
			Timestamp: now.Add(time.Duration(-24+i) * time.Hour),
		}
	}

	trend := ComputeTrend(points, "cpu", 24*time.Hour)

	if trend.Direction != TrendStable {
		t.Errorf("Expected TrendStable, got %s (rate: %.4f/hr)", trend.Direction, trend.RatePerHour)
	}
}

func TestComputeTrend_Declining(t *testing.T) {
	// Create declining data
	now := time.Now()
	points := make([]MetricPoint, 24)
	for i := 0; i < 24; i++ {
		points[i] = MetricPoint{
			Value:     80 - float64(i)*0.5, // -12% per day
			Timestamp: now.Add(time.Duration(-24+i) * time.Hour),
		}
	}

	trend := ComputeTrend(points, "disk", 24*time.Hour)

	if trend.Direction != TrendDeclining {
		t.Errorf("Expected TrendDeclining, got %s", trend.Direction)
	}
}

func TestComputeTrend_Volatile(t *testing.T) {
	// Create volatile data with high variance
	now := time.Now()
	points := make([]MetricPoint, 24)
	for i := 0; i < 24; i++ {
		// Alternating high/low values
		value := 50.0
		if i%2 == 0 {
			value = 80.0
		} else {
			value = 20.0
		}
		points[i] = MetricPoint{
			Value:     value,
			Timestamp: now.Add(time.Duration(-24+i) * time.Hour),
		}
	}

	trend := ComputeTrend(points, "cpu", 24*time.Hour)

	if trend.Direction != TrendVolatile {
		t.Errorf("Expected TrendVolatile, got %s (stddev: %.2f, mean: %.2f)", 
			trend.Direction, trend.StdDev, trend.Average)
	}
}

func TestComputeTrend_InsufficientData(t *testing.T) {
	// Only one data point
	points := []MetricPoint{
		{Value: 50, Timestamp: time.Now()},
	}

	trend := ComputeTrend(points, "memory", 24*time.Hour)

	if trend.Confidence != 0 {
		t.Errorf("Expected 0 confidence with insufficient data, got %.2f", trend.Confidence)
	}
}

func TestLinearRegression_Perfect(t *testing.T) {
	// Perfect linear data: y = 2x + 10
	now := time.Now()
	points := make([]MetricPoint, 10)
	for i := 0; i < 10; i++ {
		points[i] = MetricPoint{
			Value:     10 + float64(i)*2,
			Timestamp: now.Add(time.Duration(i) * time.Second),
		}
	}

	result := linearRegression(points)

	// Slope should be 2 per second
	if result.Slope < 1.9 || result.Slope > 2.1 {
		t.Errorf("Expected slope ~2, got %.4f", result.Slope)
	}

	// R² should be 1 (perfect fit)
	if result.R2 < 0.99 {
		t.Errorf("Expected R² ~1, got %.4f", result.R2)
	}
}

func TestComputePercentiles(t *testing.T) {
	now := time.Now()
	// Create 100 points with values 1-100
	points := make([]MetricPoint, 100)
	for i := 0; i < 100; i++ {
		points[i] = MetricPoint{
			Value:     float64(i + 1),
			Timestamp: now.Add(time.Duration(i) * time.Second),
		}
	}

	percentiles := ComputePercentiles(points, 5, 50, 95)

	// P5 should be ~5
	if percentiles[5] < 4 || percentiles[5] > 6 {
		t.Errorf("Expected P5 ~5, got %.2f", percentiles[5])
	}

	// P50 should be ~50
	if percentiles[50] < 49 || percentiles[50] > 51 {
		t.Errorf("Expected P50 ~50, got %.2f", percentiles[50])
	}

	// P95 should be ~95
	if percentiles[95] < 94 || percentiles[95] > 96 {
		t.Errorf("Expected P95 ~95, got %.2f", percentiles[95])
	}
}

func TestTrendSummary(t *testing.T) {
	tests := []struct {
		name     string
		trend    Trend
		expected string
	}{
		{
			name: "growing fast",
			trend: Trend{
				Direction:   TrendGrowing,
				RatePerDay:  5.5,
				RatePerHour: 0.23,
				DataPoints:  24,
			},
			expected: "growing 5.5/day",
		},
		{
			name: "growing slow",
			trend: Trend{
				Direction:   TrendGrowing,
				RatePerDay:  0.5,
				RatePerHour: 0.02,
				DataPoints:  24,
			},
			expected: "growing 0.02/hr",
		},
		{
			name: "stable",
			trend: Trend{
				Direction:  TrendStable,
				DataPoints: 24,
			},
			expected: "stable",
		},
		{
			name: "volatile",
			trend: Trend{
				Direction:  TrendVolatile,
				DataPoints: 24,
			},
			expected: "volatile",
		},
		{
			name: "insufficient data",
			trend: Trend{
				DataPoints: 1,
			},
			expected: "insufficient data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TrendSummary(tt.trend)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestComputeStats(t *testing.T) {
	points := []MetricPoint{
		{Value: 10},
		{Value: 20},
		{Value: 30},
		{Value: 40},
		{Value: 50},
	}

	stats := computeStats(points)

	if stats.Count != 5 {
		t.Errorf("Expected count 5, got %d", stats.Count)
	}
	if stats.Min != 10 {
		t.Errorf("Expected min 10, got %.2f", stats.Min)
	}
	if stats.Max != 50 {
		t.Errorf("Expected max 50, got %.2f", stats.Max)
	}
	if stats.Mean != 30 {
		t.Errorf("Expected mean 30, got %.2f", stats.Mean)
	}
}

// TestComputeTrend_ShortTimeSpanBlip tests that a small fluctuation
// over a very short time span (like 1 minute) doesn't get extrapolated
// to an absurd daily rate like 700%/day
func TestComputeTrend_ShortTimeSpanBlip(t *testing.T) {
	// This simulates the exact bug: homepage-docker goes from 24.8% to 25.2%
	// over 1 minute (3 data points), but was being reported as 708%/day growth
	now := time.Now()
	points := []MetricPoint{
		{Value: 24.8, Timestamp: now.Add(-2 * time.Minute)},
		{Value: 25.0, Timestamp: now.Add(-1 * time.Minute)},
		{Value: 25.2, Timestamp: now}, // Only 0.4% change total
	}

	trend := ComputeTrend(points, "memory", 24*time.Hour)

	// With only 2 minutes of data, we should NOT extrapolate to crazy daily rates
	// The observed change is only 0.4%, so a 700% daily rate is nonsense
	if trend.RatePerDay > 50 {
		t.Errorf("Short time span blip should not extrapolate to %f%%/day (expected < 50)", trend.RatePerDay)
	}

	// Confidence should be low for such short time spans
	if trend.Confidence > 0.5 {
		t.Errorf("Expected low confidence for 2-minute span, got %.2f", trend.Confidence)
	}
}

// TestComputeTrend_PercentageCapping tests that percentage metrics (0-100)
// have their growth rates capped to physically possible limits
func TestComputeTrend_PercentageCapping(t *testing.T) {
	// Even with a long time span, if the raw rate comes out absurdly high
	// (which shouldn't happen with good data, but let's test the cap)
	now := time.Now()
	
	// Create data that would naively produce a >100%/day rate
	// 5 points over 2 hours with aggressive growth
	points := make([]MetricPoint, 5)
	for i := 0; i < 5; i++ {
		points[i] = MetricPoint{
			Value:     20 + float64(i)*10, // 20, 30, 40, 50, 60
			Timestamp: now.Add(time.Duration(-4+i) * 30 * time.Minute), // 30 min apart
		}
	}

	trend := ComputeTrend(points, "memory", 24*time.Hour)

	// For a percentage metric, rate should be capped at 100%/day max
	if trend.RatePerDay > 100 {
		t.Errorf("Percentage metric should be capped at 100%%/day, got %.2f", trend.RatePerDay)
	}
}

// TestComputeTrend_MediumTimeSpan tests that 10-60 minutes of data
// gets moderate rate capping but isn't completely zeroed out
func TestComputeTrend_MediumTimeSpan(t *testing.T) {
	now := time.Now()
	// 30 minutes of data with steady growth
	points := make([]MetricPoint, 7)
	for i := 0; i < 7; i++ {
		points[i] = MetricPoint{
			Value:     30 + float64(i)*1.5, // Growing ~10% over 30 min
			Timestamp: now.Add(time.Duration(-30+i*5) * time.Minute),
		}
	}

	trend := ComputeTrend(points, "cpu", 24*time.Hour)

	// Rate should be present (not zeroed) but reasonable
	if trend.RatePerHour == 0 {
		t.Errorf("Medium time span should have non-zero hourly rate")
	}
	
	// But daily extrapolation should be constrained
	observedChange := 1.5 * 6 // ~9% change
	if trend.RatePerDay > observedChange*15 {
		t.Errorf("Daily rate %.2f should not vastly exceed observed change %.2f", 
			trend.RatePerDay, observedChange)
	}
}

// TestComputeTrend_LongTimeSpanNoChange tests that with 24h of data
// and minimal change, we get stable (not growing) trend
func TestComputeTrend_LongTimeSpanNoChange(t *testing.T) {
	now := time.Now()
	// 24 hours of stable data at ~25%
	points := make([]MetricPoint, 24)
	for i := 0; i < 24; i++ {
		// Very small oscillation around 25%
		points[i] = MetricPoint{
			Value:     25.0 + float64(i%2)*0.2, // 25.0, 25.2, 25.0, 25.2...
			Timestamp: now.Add(time.Duration(-24+i) * time.Hour),
		}
	}

	trend := ComputeTrend(points, "memory", 24*time.Hour)

	if trend.Direction == TrendGrowing {
		t.Errorf("Stable oscillating data should not be classified as Growing")
	}
	
	// Rate should be tiny
	if trend.RatePerDay > 1 || trend.RatePerDay < -1 {
		t.Errorf("Stable data should have near-zero rate, got %.2f/day", trend.RatePerDay)
	}
}

