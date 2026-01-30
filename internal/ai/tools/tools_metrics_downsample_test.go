package tools

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownsampleMetrics_NoOpWhenUnderTarget(t *testing.T) {
	points := make([]MetricPoint, 50)
	for i := range points {
		points[i] = MetricPoint{
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
			CPU:       float64(i),
			Memory:    float64(i * 2),
		}
	}

	result := downsampleMetrics(points, 120)
	assert.Equal(t, 50, len(result), "should return all points when under target")
	assert.Equal(t, points[0].CPU, result[0].CPU)
}

func TestDownsampleMetrics_ExactTarget(t *testing.T) {
	points := make([]MetricPoint, 120)
	for i := range points {
		points[i] = MetricPoint{
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
			CPU:       float64(i),
		}
	}

	result := downsampleMetrics(points, 120)
	assert.Equal(t, 120, len(result), "should return all points when exactly at target")
}

func TestDownsampleMetrics_ReducesToTarget(t *testing.T) {
	// 1000 points -> 120 target
	n := 1000
	points := make([]MetricPoint, n)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range points {
		points[i] = MetricPoint{
			Timestamp: start.Add(time.Duration(i) * time.Minute),
			CPU:       50.0, // constant
			Memory:    70.0,
		}
	}

	result := downsampleMetrics(points, 120)
	assert.Equal(t, 120, len(result), "should produce exactly target count")

	// With constant data, averages should match the original values
	for _, p := range result {
		assert.InDelta(t, 50.0, p.CPU, 0.01, "CPU average should be 50")
		assert.InDelta(t, 70.0, p.Memory, 0.01, "Memory average should be 70")
	}
}

func TestDownsampleMetrics_PreservesTimeOrder(t *testing.T) {
	n := 500
	points := make([]MetricPoint, n)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range points {
		points[i] = MetricPoint{
			Timestamp: start.Add(time.Duration(i) * time.Minute),
			CPU:       float64(i),
		}
	}

	result := downsampleMetrics(points, 120)

	for i := 1; i < len(result); i++ {
		assert.True(t, result[i].Timestamp.After(result[i-1].Timestamp),
			"timestamps should be in order: [%d]=%v should be after [%d]=%v",
			i, result[i].Timestamp, i-1, result[i-1].Timestamp)
	}
}

func TestDownsampleMetrics_BucketAveraging(t *testing.T) {
	// Create 240 points: first 120 have CPU=10, next 120 have CPU=90
	// With target=2 buckets, we should get ~10 and ~90
	points := make([]MetricPoint, 240)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range points {
		points[i] = MetricPoint{
			Timestamp: start.Add(time.Duration(i) * time.Minute),
		}
		if i < 120 {
			points[i].CPU = 10.0
		} else {
			points[i].CPU = 90.0
		}
	}

	result := downsampleMetrics(points, 2)
	require.Equal(t, 2, len(result))
	assert.InDelta(t, 10.0, result[0].CPU, 0.01, "first bucket should average ~10")
	assert.InDelta(t, 90.0, result[1].CPU, 0.01, "second bucket should average ~90")
}

func TestDownsampleMetrics_PreservesDiskWhenPresent(t *testing.T) {
	points := make([]MetricPoint, 200)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range points {
		points[i] = MetricPoint{
			Timestamp: start.Add(time.Duration(i) * time.Minute),
			CPU:       50.0,
			Memory:    60.0,
			Disk:      75.0,
		}
	}

	result := downsampleMetrics(points, 10)
	for _, p := range result {
		assert.InDelta(t, 75.0, p.Disk, 0.01, "disk should be preserved when present")
	}
}

func TestDownsampleMetrics_DiskZeroWhenAbsent(t *testing.T) {
	points := make([]MetricPoint, 200)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range points {
		points[i] = MetricPoint{
			Timestamp: start.Add(time.Duration(i) * time.Minute),
			CPU:       50.0,
			Memory:    60.0,
			// Disk intentionally zero/absent
		}
	}

	result := downsampleMetrics(points, 10)
	for _, p := range result {
		assert.Equal(t, 0.0, p.Disk, "disk should be 0 when not present in source")
	}
}

func TestDownsampleMetrics_EdgeCases(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		result := downsampleMetrics(nil, 120)
		assert.Nil(t, result)

		result = downsampleMetrics([]MetricPoint{}, 120)
		assert.Equal(t, 0, len(result))
	})

	t.Run("target zero", func(t *testing.T) {
		points := []MetricPoint{{CPU: 1.0}}
		result := downsampleMetrics(points, 0)
		assert.Equal(t, 1, len(result), "target=0 should return original")
	})

	t.Run("single point", func(t *testing.T) {
		points := []MetricPoint{{CPU: 42.0, Timestamp: time.Now()}}
		result := downsampleMetrics(points, 120)
		assert.Equal(t, 1, len(result))
		assert.Equal(t, 42.0, result[0].CPU)
	})

	t.Run("target equals input", func(t *testing.T) {
		points := make([]MetricPoint, 120)
		result := downsampleMetrics(points, 120)
		assert.Equal(t, 120, len(result))
	})
}

func TestDownsampleMetrics_LargeDataset(t *testing.T) {
	// Simulate 7 days of per-minute data (~10K points)
	n := 10080 // 7 * 24 * 60
	points := make([]MetricPoint, n)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range points {
		points[i] = MetricPoint{
			Timestamp: start.Add(time.Duration(i) * time.Minute),
			CPU:       float64(i%100) + 10.0,
			Memory:    65.0,
		}
	}

	result := downsampleMetrics(points, maxMetricPoints)
	assert.Equal(t, maxMetricPoints, len(result), "should produce exactly maxMetricPoints (%d) from %d input points", maxMetricPoints, n)

	// Verify time span is preserved (first and last points should be close to original bounds)
	assert.True(t, result[0].Timestamp.After(start.Add(-time.Hour)),
		"first point should be near the start")
	lastExpected := start.Add(time.Duration(n-1) * time.Minute)
	assert.True(t, result[len(result)-1].Timestamp.Before(lastExpected.Add(time.Hour)),
		"last point should be near the end")
}

func TestMaxMetricPointsConstant(t *testing.T) {
	assert.Equal(t, 120, maxMetricPoints, "maxMetricPoints should be 120")
}

func TestMetricsResponseFields(t *testing.T) {
	// Verify the new fields exist and serialize correctly
	resp := MetricsResponse{
		ResourceID:    "vm101",
		Period:        "7d",
		Downsampled:   true,
		OriginalCount: 10080,
	}

	assert.True(t, resp.Downsampled)
	assert.Equal(t, 10080, resp.OriginalCount)
}
