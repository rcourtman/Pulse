package monitoring

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestDiskDrawerNativeTemperatureHistory(t *testing.T) {
	if mock.IsMockEnabled() {
		t.Fatal("native History test requires non-mock mode")
	}
	now := time.Now().UTC().Truncate(time.Second)
	native := []MetricPoint{
		{Timestamp: now.Add(-55 * time.Minute), Value: 31.25},
		{Timestamp: now.Add(-30 * time.Minute), Value: 33.5},
		{Timestamp: now.Add(-time.Minute), Value: 32.75},
	}
	for _, count := range []int{0, 1} {
		t.Run(map[int]string{0: "empty", 1: "sparse"}[count], func(t *testing.T) {
			monitor := &Monitor{metricsHistory: NewMetricsHistory(20, time.Hour)}
			if count != 0 {
				monitor.metricsHistory.AddDiskMetric("disk-serial", "smart_temp", 39, now.Add(-time.Minute))
			}
			monitor.supplementalProviders = map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{
				unifiedresources.SourceTrueNAS: &stubDiskTemperatureHistoryProvider{history: map[string][]MetricPoint{"disk-serial": native}},
			}
			got := monitor.GetDiskMetricsForChart("disk-serial", "smart_temp", time.Hour)
			if !reflect.DeepEqual(got, native) {
				t.Fatalf("drawer lost native timestamps/Celsius samples: got %+v, want %+v", got, native)
			}
			got[0].Value = 999
			if next := monitor.GetDiskMetricsForChart("disk-serial", "smart_temp", time.Hour); !reflect.DeepEqual(next, native) {
				t.Fatal("drawer response aliases provider history")
			}
		})
	}
}

type countingDiskHistoryProvider struct {
	stubDiskTemperatureHistoryProvider
	calls int
	org   string
}

func (p *countingDiskHistoryProvider) PhysicalDiskTemperatureHistory(_ *Monitor, org string, _ time.Duration) map[string][]MetricPoint {
	p.calls++
	p.org = org
	return p.history
}

func TestDiskDrawerNativeHistoryBoundaries(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	provider := &countingDiskHistoryProvider{stubDiskTemperatureHistoryProvider: stubDiskTemperatureHistoryProvider{
		history: map[string][]MetricPoint{"known-disk": {{Timestamp: now.Add(-time.Minute), Value: 31}}},
	}}
	monitor := &Monitor{metricsHistory: NewMetricsHistory(20, time.Hour)}
	monitor.SetOrgID("org-a")
	monitor.supplementalProviders = map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: provider}
	if got := monitor.GetDiskMetricsForChart("known-disk", "diskread", time.Hour); len(got) != 0 || provider.calls != 0 {
		t.Fatalf("disk I/O must not ask for temperatures: %+v, calls %d", got, provider.calls)
	}
	if got := monitor.GetDiskMetricsForChart("foreign-disk", "smart_temp", time.Hour); len(got) != 0 || provider.org != "org-a" {
		t.Fatalf("foreign/native identity leaked: %+v, org %q", got, provider.org)
	}
	got := monitor.GetDiskMetricsForChart("known-disk", "smart_temp", time.Hour)
	if len(got) != 1 || got[0].Value != 31 || !got[0].Timestamp.Equal(now.Add(-time.Minute)) {
		t.Fatalf("one actual sample was fabricated/renewed: %+v", got)
	}
	calls := provider.calls
	for _, offset := range []time.Duration{-55 * time.Minute, -30 * time.Minute, -time.Minute} {
		monitor.metricsHistory.AddDiskMetric("known-disk", "smart_temp", 35, now.Add(offset))
	}
	if got := monitor.GetDiskMetricsForChart("known-disk", "smart_temp", time.Hour); len(got) != 3 || got[0].Value != 35 || provider.calls != calls {
		t.Fatalf("sufficient local coverage should not invoke native RPC: %+v, calls %d", got, provider.calls)
	}
	var nilMonitor *Monitor
	if got := nilMonitor.GetDiskMetricsForChart("known-disk", "smart_temp", time.Hour); len(got) != 0 {
		t.Fatalf("nil monitor has history: %+v", got)
	}
}
