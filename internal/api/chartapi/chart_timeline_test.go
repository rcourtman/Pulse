package chartapi

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// Keep the pre-reuse conversion as a wire/ownership oracle. Sampling itself
// remains covered by referenceChartCap and TestChartPointSelectionMatchesOriginalCap.
func referenceConvertMetricsForChart(metrics map[string][]monitoring.MetricPoint, oldestTimestamp *int64, maxPoints int) VMChartData {
	converted := make(VMChartData, len(metrics))
	for metricType, metricPoints := range metrics {
		if !sparklineMetrics[metricType] {
			continue
		}
		points := make([]MetricPoint, len(metricPoints))
		for i, point := range metricPoints {
			ts := point.Timestamp.UnixMilli()
			if ts < *oldestTimestamp {
				*oldestTimestamp = ts
			}
			points[i] = MetricPoint{Timestamp: ts, Value: point.Value}
		}
		converted[metricType] = referenceChartCap(points, maxPoints)
	}
	return converted
}

func timelineFixture(count int, shared bool) map[string][]monitoring.MetricPoint {
	result := make(map[string][]monitoring.MetricPoint)
	for m, name := range []string{"cpu", "memory", "memoryused", "disk", "netin", "netout", "diskread", "diskwrite"} {
		points := make([]monitoring.MetricPoint, count)
		for i := range points {
			offset := int64(i) * 60000
			if !shared {
				offset += int64((i % 7) * m)
			}
			points[i] = monitoring.MetricPoint{Timestamp: time.UnixMilli(1792930000000 + offset), Value: float64((i + 3*m) % 100)}
		}
		result[name] = points
	}
	return result
}

func TestConvertMetricsForChartSharedAndDivergentTimelines(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(20261025))
	for scenario := 0; scenario < 100; scenario++ {
		n := []int{0, 1, 2, 17, 121, 240, 500, 777}[scenario%8]
		source := timelineFixture(n, scenario%2 == 0)
		for metric, points := range source {
			// Same first/last/count with different interiors must never reuse the
			// wrong representatives. Values, sub-ms timestamps and zones are not keys.
			for i := range points {
				switch scenario % 7 {
				case 0:
					points[i].Timestamp = time.UnixMilli(-1000 + int64(i%9))
				case 1:
					if metric == "memory" && i > 0 && i < n-1 {
						points[i].Timestamp = points[i].Timestamp.Add(time.Duration(rng.Intn(120000)-60000) * time.Millisecond)
					}
				case 2:
					if metric == "disk" {
						points[i].Timestamp = time.UnixMilli(100)
					}
				case 3:
					points[i].Timestamp = points[i].Timestamp.In(time.FixedZone("fixture", 3600)).Add(time.Duration(rng.Intn(999999)) * time.Nanosecond)
				case 4:
					if i > n/2 {
						points[i].Timestamp = points[n/2].Timestamp.Add(time.Duration(i-n/2) * time.Millisecond)
					}
				case 5:
					if metric == "cpu" {
						points[i].Value = math.Copysign(0, -1)
					}
				}
			}
			if scenario%7 == 6 && len(points) > 2 {
				points[0], points[n-1] = points[n-1], points[0]
			}
		}
		if scenario%3 == 0 {
			source["cpu"] = nil
		}
		source["not-a-sparkline"] = []monitoring.MetricPoint{{Timestamp: time.UnixMilli(math.MinInt64), Value: -999}}
		for _, limit := range []int{-1, 0, 1, 2, 3, 30, 120, 500} {
			t.Run(fmt.Sprintf("%d/cap=%d", scenario, limit), func(t *testing.T) {
				wantOldest, gotOldest := int64(math.MaxInt64), int64(math.MaxInt64)
				want := referenceConvertMetricsForChart(source, &wantOldest, limit)
				got := convertMetricsForChart(source, &gotOldest, limit)
				if !reflect.DeepEqual(got, want) || gotOldest != wantOldest {
					t.Fatalf("timeline reuse changed original observations or oldest: got=%d want=%d", gotOldest, wantOldest)
				}
				gotJSON, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				wantJSON, err := json.Marshal(want)
				if err != nil {
					t.Fatal(err)
				}
				if string(gotJSON) != string(wantJSON) {
					t.Fatal("wire bytes changed")
				}
				for metric, points := range got {
					if len(points) > 0 {
						first := source[metric][0]
						points[0].Value = 99999
						points[0].Timestamp = 0
						if source[metric][0] != first {
							t.Fatal("output aliases input")
						}
					}
				}
			})
		}
	}
}

var chartTimelineBenchmarkSink VMChartData

func TestConvertMetricsForChartSharedTimelineAllocations(t *testing.T) {
	source := timelineFixture(240, true)
	// Compare against the exact immediate-parent capped path, not the older
	// full-wire oracle above, which intentionally converts before capping.
	parent := func() {
		oldest := int64(math.MaxInt64)
		chartTimelineBenchmarkSink = parentConvertMetricsForChart(source, &oldest, 120)
	}
	production := func() {
		oldest := int64(math.MaxInt64)
		chartTimelineBenchmarkSink = convertMetricsForChart(source, &oldest, 120)
	}
	want := testing.AllocsPerRun(20, parent)
	got := testing.AllocsPerRun(20, production)
	t.Logf("shared timeline allocations: parent=%.0f production=%.0f", want, got)
	if got >= want {
		t.Fatalf("shared timeline must avoid repeated sampling allocations: parent=%.0f production=%.0f", want, got)
	}
}

// Frozen immediate-parent path for comparable allocation measurements.
func parentConvertMetricsForChart(metrics map[string][]monitoring.MetricPoint, oldestTimestamp *int64, maxPoints int) VMChartData {
	converted := make(VMChartData, len(metrics))
	for metricType, metricPoints := range metrics {
		if !sparklineMetrics[metricType] {
			continue
		}
		if len(metricPoints) <= maxPoints || maxPoints <= 0 {
			points := make([]MetricPoint, len(metricPoints))
			for i, point := range metricPoints {
				ts := point.Timestamp.UnixMilli()
				if ts < *oldestTimestamp {
					*oldestTimestamp = ts
				}
				points[i] = MetricPoint{Timestamp: ts, Value: point.Value}
			}
			converted[metricType] = points
			continue
		}
		for _, point := range metricPoints {
			if ts := point.Timestamp.UnixMilli(); ts < *oldestTimestamp {
				*oldestTimestamp = ts
			}
		}
		indices := chartPointIndices(len(metricPoints), maxPoints, func(i int) int64 { return metricPoints[i].Timestamp.UnixMilli() })
		points := make([]MetricPoint, len(indices))
		for i, index := range indices {
			point := metricPoints[index]
			points[i] = MetricPoint{Timestamp: point.Timestamp.UnixMilli(), Value: point.Value}
		}
		converted[metricType] = points
	}
	return converted
}

func BenchmarkConvertMetricsForChartTimelines(b *testing.B) {
	for _, shared := range []bool{true, false} {
		source := timelineFixture(240, shared)
		b.Run(fmt.Sprintf("shared=%t", shared), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				oldest := int64(math.MaxInt64)
				chartTimelineBenchmarkSink = convertMetricsForChart(source, &oldest, 120)
			}
		})
	}
}
