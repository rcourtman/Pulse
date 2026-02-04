package monitoring

import (
	"math"
	"testing"
	"time"
)

func TestLTTB_PassthroughSmallData(t *testing.T) {
	// Data smaller than target should be returned unchanged.
	data := makeLinear(5, time.Now(), time.Second)
	result := lttb(data, 10)
	if len(result) != 5 {
		t.Fatalf("expected 5 points, got %d", len(result))
	}
}

func TestLTTB_PassthroughTargetLessThan3(t *testing.T) {
	data := makeLinear(100, time.Now(), time.Second)
	result := lttb(data, 2)
	if len(result) != 100 {
		t.Fatalf("expected passthrough for target<3, got %d", len(result))
	}
}

func TestLTTB_ExactTarget(t *testing.T) {
	data := makeLinear(50, time.Now(), time.Second)
	result := lttb(data, 50)
	if len(result) != 50 {
		t.Fatalf("expected 50 points, got %d", len(result))
	}
}

func TestLTTB_KeepsFirstAndLast(t *testing.T) {
	data := makeLinear(100, time.Now(), time.Second)
	result := lttb(data, 10)
	if result[0] != data[0] {
		t.Fatal("first point not preserved")
	}
	if result[len(result)-1] != data[len(data)-1] {
		t.Fatal("last point not preserved")
	}
}

func TestLTTB_OutputLength(t *testing.T) {
	data := makeLinear(1000, time.Now(), time.Second)
	for _, target := range []int{3, 10, 50, 100, 200, 500} {
		result := lttb(data, target)
		if len(result) != target {
			t.Errorf("target %d: got %d points", target, len(result))
		}
	}
}

func TestLTTB_PreservesPeak(t *testing.T) {
	// Create data with a clear spike — LTTB should keep the peak.
	start := time.Now()
	data := make([]MetricPoint, 200)
	for i := range data {
		data[i] = MetricPoint{
			Value:     0,
			Timestamp: start.Add(time.Duration(i) * time.Second),
		}
	}
	// Insert a spike at position 100.
	data[100].Value = 100

	result := lttb(data, 20)

	// The spike should be preserved.
	maxVal := float64(0)
	for _, p := range result {
		if p.Value > maxVal {
			maxVal = p.Value
		}
	}
	if maxVal != 100 {
		t.Errorf("peak not preserved: max value in result = %f", maxVal)
	}
}

func TestLTTB_PreservesValley(t *testing.T) {
	start := time.Now()
	data := make([]MetricPoint, 200)
	for i := range data {
		data[i] = MetricPoint{
			Value:     50,
			Timestamp: start.Add(time.Duration(i) * time.Second),
		}
	}
	data[100].Value = 0

	result := lttb(data, 20)

	minVal := math.MaxFloat64
	for _, p := range result {
		if p.Value < minVal {
			minVal = p.Value
		}
	}
	if minVal != 0 {
		t.Errorf("valley not preserved: min value in result = %f", minVal)
	}
}

func TestLTTB_MonotonicTimestamps(t *testing.T) {
	data := makeLinear(500, time.Now(), time.Second)
	result := lttb(data, 50)
	for i := 1; i < len(result); i++ {
		if !result[i].Timestamp.After(result[i-1].Timestamp) {
			t.Fatalf("timestamps not monotonic at index %d", i)
		}
	}
}

// makeLinear creates n linearly increasing MetricPoints.
func makeLinear(n int, start time.Time, interval time.Duration) []MetricPoint {
	data := make([]MetricPoint, n)
	for i := range data {
		data[i] = MetricPoint{
			Value:     float64(i),
			Timestamp: start.Add(time.Duration(i) * interval),
		}
	}
	return data
}
