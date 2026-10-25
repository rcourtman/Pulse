package monitoring

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
)

func TestMockChartTimestampGridKeepsOriginalBucketReadings(t *testing.T) {
	// Include dates before the Unix epoch, several readings in one bucket,
	// and a terminal reading in the same bucket as its predecessor.
	offsets := []time.Duration{-3 * time.Second, -2800 * time.Millisecond, -1100 * time.Millisecond,
		-time.Second, -500 * time.Millisecond, 0, 500 * time.Millisecond,
		time.Second, 1100 * time.Millisecond, 1500 * time.Millisecond}
	points := make([]time.Time, len(offsets))
	for i, offset := range offsets {
		points[i] = time.Unix(0, 0).Add(offset)
	}
	want := []time.Time{points[0], points[2], points[3], points[5], points[7], points[9]}
	got := sampleOnMockChartGrid(points, func(at time.Time) time.Time { return at }, time.Second)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("grid changed original bucket readings: got %v, want %v", got, want)
	}
	for _, interval := range []time.Duration{0, -time.Second} {
		cloned := sampleOnMockChartGrid(points, func(at time.Time) time.Time { return at }, interval)
		if !reflect.DeepEqual(cloned, points) {
			t.Fatalf("interval %s changed the unsampled input", interval)
		}
		cloned[0] = time.Time{}
		if points[0].IsZero() {
			t.Fatal("the returned series aliases its input")
		}
	}
}

func TestMockChartTimestampsBoundSynthesisAndPreserveTheDenseHistoryGrid(t *testing.T) {
	for _, duration := range []time.Duration{5 * time.Minute, time.Hour, 4 * time.Hour,
		24 * time.Hour, 7 * 24 * time.Hour, 30 * 24 * time.Hour} {
		t.Run(duration.String(), func(t *testing.T) {
			got := mockChartTimestamps(duration)
			if len(got) < 2 || len(got) > mockChartPointTarget(duration) {
				t.Fatalf("synthesizing %d values per series, want 2..%d", len(got), mockChartPointTarget(duration))
			}
			// Retain the former dense-then-thin calculation as the parity
			// oracle. Selecting timestamps first must preserve every reading
			// the chart previously returned, including the fresh terminal one.
			dense := buildTieredTimestamps(got[len(got)-1], duration)
			readings := make([]MetricPoint, len(dense))
			for i, at := range dense {
				readings[i] = MetricPoint{Timestamp: at, Value: float64(i)}
			}
			want := sampleMetricSeriesOnMockChartGrid(readings, mockChartGridInterval(duration))
			if len(got) != len(want) {
				t.Fatalf("timestamp selection changed: got %d readings, want %d", len(got), len(want))
			}
			for i, at := range got {
				if !at.Equal(want[i].Timestamp) {
					t.Fatalf("reading %d moved: got %s, want %s", i, at, want[i].Timestamp)
				}
			}
			if !got[0].Equal(dense[0]) || !got[len(got)-1].Equal(dense[len(dense)-1]) {
				t.Fatal("the full history window or fresh terminal reading was lost")
			}
		})
	}
}

func TestMockChartSynthesisKeepsCanonicalValuesAtRetainedTimestamps(t *testing.T) {
	for _, resourceType := range []string{"vm", "container", "dockerContainer", "k8s", "agent", "dockerHost", "node", "disk"} {
		t.Run(resourceType, func(t *testing.T) {
			const resourceID = "mock-chart-parity"
			const duration = 7 * 24 * time.Hour
			var series map[string][]MetricPoint
			switch resourceType {
			case "node":
				series = mockNodeMetricsForChart(resourceID, nil, duration)
			case "disk":
				series = mockDiskMetricsForChart(resourceID, nil, duration)
			default:
				series = mockGuestMetricsForChart(resourceType, resourceID, duration)
			}
			if len(series) == 0 {
				t.Fatal("synthetic history is empty")
			}
			sampler := mock.CurrentMetricSampler()
			for metric, points := range series {
				if metric == "memoryused" {
					continue // Derived bytes have their own capacity parity proof.
				}
				if len(points) < 2 || len(points) > mockChartPointTarget(duration) {
					t.Fatalf("%s synthesized %d points, want a bounded full-window series", metric, len(points))
				}
				for _, point := range points {
					want := sampler.SampleMetric(resourceType, resourceID, metric, point.Timestamp)
					if point.Value != want {
						t.Fatalf("%s at %s changed its canonical value: got %g, want %g", metric, point.Timestamp, point.Value, want)
					}
				}
			}
		})
	}
}
