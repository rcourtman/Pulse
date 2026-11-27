package monitoring

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
)

func TestRecordGuestMetricBatchPreservesSeries(t *testing.T) {
	previousMockMode := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, previousMockMode) })

	history := NewMetricsHistory(1000, 24*time.Hour)
	m := &Monitor{metricsHistory: history}
	now := time.Now()
	first, second := now.Add(-2*time.Minute), now.Add(-time.Minute)
	const guest = "test-node:101"
	m.recordGuestMetric("vm", guest, 10, 20, 30, 40, 50, 60, 70, 80, first)
	m.recordGuestMetric("vm", guest, 1, -1, 0, math.NaN(), -1, 0, 0, -1, second)
	m.recordGuestMetric("vm", guest, 2, -1, 0, math.NaN(), -1, 0, 0, -1, second)

	for _, tc := range []struct {
		kind string
		want []MetricPoint
	}{
		{"cpu", []MetricPoint{{Timestamp: first, Value: 10}, {Timestamp: second, Value: 2}}},
		{"memory", []MetricPoint{{Timestamp: first, Value: 20}}},
		{"memoryused", []MetricPoint{{Timestamp: first, Value: 30}, {Timestamp: second, Value: 0}}},
		{"disk", []MetricPoint{{Timestamp: first, Value: 40}}},
		{"diskread", []MetricPoint{{Timestamp: first, Value: 50}}},
		{"diskwrite", []MetricPoint{{Timestamp: first, Value: 60}, {Timestamp: second, Value: 0}}},
		{"netin", []MetricPoint{{Timestamp: first, Value: 70}, {Timestamp: second, Value: 0}}},
		{"netout", []MetricPoint{{Timestamp: first, Value: 80}}},
	} {
		if got := history.GetGuestMetrics(guest, tc.kind, time.Hour); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s history = %v; want %v", tc.kind, got, tc.want)
		}
	}
}

func BenchmarkRecordGuestMetricHistory(b *testing.B) {
	for _, tc := range []struct {
		name string
		disk int64
	}{
		{"all-series", 50},
		{"cpu-only", -1},
	} {
		b.Run(tc.name, func(b *testing.B) {
			if mock.IsMockEnabled() {
				b.Fatal("benchmark requires live history writes")
			}
			history := NewMetricsHistory(1000, 24*time.Hour)
			m := &Monitor{metricsHistory: history}
			now := time.Now().Add(-time.Minute)
			record := func(at time.Time) {
				if tc.disk < 0 {
					m.recordGuestMetric("vm", "test-node:101", 10, -1, -1, -1, -1, -1, -1, -1, at)
				} else {
					m.recordGuestMetric("vm", "test-node:101", 10, 20, 30, 40, tc.disk, 60, 70, 80, at)
				}
			}
			for i := range 1000 {
				record(now.Add(time.Duration(i) * time.Nanosecond))
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				record(now.Add(time.Duration(1000+i) * time.Nanosecond))
			}
		})
	}
}
