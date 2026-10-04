package monitoring

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
)

// Preserve the original reader as a test-only reference. Paired benchmarks use
// the same executable, inputs and cutoff; no SQL or collector is exercised.
func legacyHistoryReadFilter(data []MetricPoint, cutoff time.Time) []MetricPoint {
	filtered := make([]MetricPoint, 0)
	for _, point := range data {
		if point.Timestamp.After(cutoff) {
			filtered = append(filtered, point)
		}
	}
	return filtered
}

func historyReadFixture(size, retained int, interleaved bool) ([]MetricPoint, time.Time) {
	cutoff := time.Unix(1700000000, 0).UTC()
	data := make([]MetricPoint, size)
	for i := range data {
		offset := i - (size - retained) + 1
		if interleaved {
			offset = i + 1
			if i%2 == 0 {
				offset = -offset
			}
		}
		data[i] = MetricPoint{Timestamp: cutoff.Add(time.Duration(offset) * time.Second), Value: float64(i)}
	}
	return data, cutoff
}

func TestHistoryReadFilterPreservesSemantics(t *testing.T) {
	cutoff := time.Unix(1700000000, 0).UTC()
	before := MetricPoint{Timestamp: cutoff.Add(-time.Second), Value: -1}
	equal := MetricPoint{Timestamp: cutoff.In(time.FixedZone("other", 3600)), Value: 0}
	after := MetricPoint{Timestamp: cutoff.Add(time.Second), Value: 2}
	later := MetricPoint{Timestamp: cutoff.Add(2 * time.Second), Value: 3}
	for name, data := range map[string][]MetricPoint{
		"nil": nil, "empty": {}, "expired": {before, equal},
		"all": {after, later}, "prefix": {before, equal, after, later},
		"unordered": {later, before, after, equal}, "duplicates": {before, after, after},
	} {
		t.Run(name, func(t *testing.T) {
			original := append([]MetricPoint(nil), data...)
			want := legacyHistoryReadFilter(data, cutoff)
			got := filterMetricsByTime(data, cutoff)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("filtered points = %#v, want %#v", got, want)
			}
			if got == nil {
				t.Fatal("empty History arrays must remain [] rather than null")
			}
			if len(got) > 0 {
				got[0].Value = -999
				got[0].Timestamp = time.Time{}
				if !reflect.DeepEqual(data, original) {
					t.Fatal("caller mutation changed retained History")
				}
			}
		})
	}
}

func TestHistoryReadFilterAllocationBudget(t *testing.T) {
	for _, tc := range []struct {
		name           string
		size, retained int
		interleaved    bool
	}{
		{"empty", 0, 0, false}, {"expired", 2880, 0, false},
		{"one", 2880, 1, false}, {"hour", 2880, 60, false},
		{"day", 2880, 1440, false}, {"all", 2880, 2880, false},
		{"interleaved", 2880, 1440, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, cutoff := historyReadFixture(tc.size, tc.retained, tc.interleaved)
			var snapshot []MetricPoint
			allocs := testing.AllocsPerRun(25, func() { snapshot = filterMetricsByTime(data, cutoff) })
			budget := float64(1)
			if tc.retained == 0 {
				budget = 0
			}
			t.Logf("retained=%d input=%d allocations=%.0f capacity=%d", len(snapshot), len(data), allocs, cap(snapshot))
			if allocs > budget {
				t.Errorf("snapshot allocated %.0f times, want at most %.0f", allocs, budget)
			}
			if cap(snapshot) != len(snapshot) {
				t.Errorf("snapshot capacity=%d, want exactly its %d retained points", cap(snapshot), len(snapshot))
			}
		})
	}
}

func TestHistoryReadPublicAllocationBudget(t *testing.T) {
	history := NewMetricsHistory(1024, 4*time.Hour)
	now := time.Now()
	for i := 0; i < 512; i++ {
		stamp := now.Add(time.Duration(i-512) * time.Second)
		history.AddGuestMetric("dense", "cpu", float64(i), stamp)
		history.AddGuestMetric("dense", "memoryused", float64(i), stamp)
		history.AddNodeMetric("dense", "cpu", float64(i), stamp)
		history.AddDiskMetric("dense", "smart_temp", float64(i), stamp)
		history.AddStorageMetric("dense", "usage", float64(i), stamp)
		history.AddStorageMetric("dense", "used", float64(i), stamp)
	}
	// These entries have the same map/empty-series shape but no sample arrays.
	history.AddGuestMetric("empty", "unknown", 0, now)
	history.AddStorageMetric("empty", "unknown", 0, now)
	for name, read := range map[string]func() []MetricPoint{
		"guest": func() []MetricPoint { return history.GetGuestMetrics("dense", "cpu", time.Hour) },
		"node":  func() []MetricPoint { return history.GetNodeMetrics("dense", "cpu", time.Hour) },
		"disk":  func() []MetricPoint { return history.GetDiskMetrics("dense", "smart_temp", time.Hour) },
	} {
		t.Run(name, func(t *testing.T) {
			var snapshot []MetricPoint
			allocs := testing.AllocsPerRun(25, func() { snapshot = read() })
			t.Logf("points=%d allocations=%.0f", len(snapshot), allocs)
			if len(snapshot) != 512 || allocs > 1 {
				t.Fatalf("dense read: points=%d allocations=%.0f, want 512/at most 1", len(snapshot), allocs)
			}
		})
	}
	for name, read := range map[string]func(string, time.Duration) map[string][]MetricPoint{
		"guest-all": history.GetAllGuestMetrics, "storage-all": history.GetAllStorageMetrics,
	} {
		t.Run(name, func(t *testing.T) {
			var snapshot map[string][]MetricPoint
			emptyAllocs := testing.AllocsPerRun(25, func() { snapshot = read("empty", time.Hour) })
			denseAllocs := testing.AllocsPerRun(25, func() { snapshot = read("dense", time.Hour) })
			count := 0
			for _, points := range snapshot {
				count += len(points)
			}
			t.Logf("points=%d empty allocations=%.0f dense allocations=%.0f", count, emptyAllocs, denseAllocs)
			if count != 1024 || denseAllocs-emptyAllocs > 2 {
				t.Fatalf("two populated series allocated %.0f extra times, want at most 2; points=%d", denseAllocs-emptyAllocs, count)
			}
		})
	}
}

func TestHistoryReadSnapshotsEverySeries(t *testing.T) {
	for scope, metrics := range map[string][]string{
		"guest": {"cpu", "memory", "memoryused", "disk", "gpu", "gpu_memory", "gpu_temperature", "diskread", "diskwrite", "netin", "netout", "temperature"},
		"node":  {"cpu", "memory", "memoryused", "disk", "netin", "netout", "temperature"},
		"disk":  {"smart_temp", "disk", "diskread", "diskwrite"}, "storage": {"usage", "used", "total", "avail"},
	} {
		for _, metric := range metrics {
			t.Run(scope+"/"+metric, func(t *testing.T) {
				history := NewMetricsHistory(32, 4*time.Hour)
				add, read := history.AddGuestMetric, history.GetGuestMetrics
				switch scope {
				case "node":
					add, read = history.AddNodeMetric, history.GetNodeMetrics
				case "disk":
					add, read = history.AddDiskMetric, history.GetDiskMetrics
				case "storage":
					add = history.AddStorageMetric
					read = func(id, metric string, duration time.Duration) []MetricPoint {
						return history.GetAllStorageMetrics(id, duration)[metric]
					}
				}
				now := time.Now()
				add("resource", metric, 10, now.Add(-2*time.Hour))
				add("resource", metric, 20, now.Add(-10*time.Minute))
				add("resource", metric, 30, now.Add(-time.Minute))
				want := []MetricPoint{{Timestamp: now.Add(-10 * time.Minute), Value: 20}, {Timestamp: now.Add(-time.Minute), Value: 30}}
				got := read("resource", metric, time.Hour)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("snapshot = %+v, want %+v", got, want)
				}
				got[0] = MetricPoint{}
				got = append(got, MetricPoint{Value: -999})
				if !reflect.DeepEqual(read("resource", metric, time.Hour), want) {
					t.Fatal("caller mutation changed History")
				}
				if scope == "guest" && !reflect.DeepEqual(history.GetAllGuestMetrics("resource", time.Hour)[metric], want) {
					t.Fatal("all-series guest read diverged")
				}
			})
		}
	}
}

func TestHistoryReadChartAndWindowSnapshots(t *testing.T) {
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	defer mustSetMockEnabled(t, previous)
	now := time.Now()
	history := NewMetricsHistory(32, 4*time.Hour)
	monitor := &Monitor{metricsHistory: history}
	const id = "instance:node:105"
	for i, offset := range []time.Duration{-2 * time.Hour, -10 * time.Minute, -time.Minute} {
		monitor.recordGuestMetric("vm", id, float64(10+i), 60, float64(4096+i), -1, -1, -1, -1, -1, now.Add(offset))
	}
	want := []MetricPoint{{Timestamp: now.Add(-10 * time.Minute), Value: 11}, {Timestamp: now.Add(-time.Minute), Value: 12}}
	chart := monitor.GetGuestMetricsForChart(id, "vm", id, time.Hour)
	if !reflect.DeepEqual(chart["cpu"], want) {
		t.Fatalf("chart = %+v, want %+v", chart["cpu"], want)
	}
	emptyJSON, err := json.Marshal(chart["disk"])
	if err != nil || string(emptyJSON) != "[]" {
		t.Fatalf("empty chart JSON = %s, err=%v", emptyJSON, err)
	}
	chart["cpu"][0] = MetricPoint{}
	batch := monitor.GetGuestMetricsForChartBatch("vm", []GuestChartRequest{{InMemoryKey: id, SQLResourceID: id}}, time.Hour, "cpu")
	if !reflect.DeepEqual(batch[id]["cpu"], want) {
		t.Fatalf("batch = %+v, want %+v", batch, want)
	}
	batch[id]["cpu"][0] = MetricPoint{}
	points, err := monitor.metricWindowPoints(alerts.MetricWindowRequest{ResourceID: id, ResourceType: "vm", Metric: "cpu", Start: now.Add(-time.Hour), End: now})
	if err != nil || len(points) != 2 || points[0].Value != 11 || points[1].Value != 12 {
		t.Fatalf("metric window = %+v, err=%v", points, err)
	}
	if !reflect.DeepEqual(monitor.GetGuestMetrics(id, time.Hour)["cpu"], want) {
		t.Fatal("chart/window mutation reached poll History")
	}
}

var historyReadBenchmarkSink []MetricPoint

func BenchmarkHistoryReadFilter(b *testing.B) {
	for _, tc := range []struct {
		name           string
		size, retained int
		interleaved    bool
	}{
		{"empty", 0, 0, false}, {"expired", 2880, 0, false},
		{"one", 2880, 1, false}, {"hour", 2880, 60, false},
		{"day", 2880, 1440, false}, {"all", 2880, 2880, false},
		{"interleaved", 2880, 1440, true},
	} {
		data, cutoff := historyReadFixture(tc.size, tc.retained, tc.interleaved)
		for _, variant := range []struct {
			name   string
			filter func([]MetricPoint, time.Time) []MetricPoint
		}{
			{"legacy", legacyHistoryReadFilter}, {"current", filterMetricsByTime},
		} {
			b.Run(fmt.Sprintf("%s/%s", tc.name, variant.name), func(b *testing.B) {
				if !reflect.DeepEqual(variant.filter(data, cutoff), legacyHistoryReadFilter(data, cutoff)) {
					b.Fatal("benchmark changed samples")
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					historyReadBenchmarkSink = variant.filter(data, cutoff)
				}
			})
		}
	}
}
