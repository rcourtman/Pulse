package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type targetedChartHistoryFixture struct {
	stubGuestMetricHistoryProvider
	reads, bulkReads int
	id, org, kind    string
}

func (s *targetedChartHistoryFixture) GuestMetricHistory(*Monitor, string, string, time.Duration) map[string]map[string][]MetricPoint {
	s.bulkReads++
	return nil
}

func (s *targetedChartHistoryFixture) GuestMetricHistoryForResource(_ *Monitor, org, kind, id string, _ time.Duration) map[string][]MetricPoint {
	s.reads++
	s.id, s.org, s.kind = id, org, kind
	return s.stubGuestMetricHistoryProvider.GuestMetricHistory(nil, org, kind, time.Hour)[id]
}

func TestGuestChartRequestedCoverageIsPerSeries(t *testing.T) {
	t.Parallel()
	for _, duration := range []time.Duration{time.Hour, 24 * time.Hour} {
		t.Run(duration.String(), func(t *testing.T) {
			monitor := newChartFallbackTestMonitor(t)
			monitor.SetOrgID("org-targeted")
			now := time.Now().UTC().Truncate(time.Second)
			points := []MetricPoint{{Timestamp: now.Add(-duration + time.Minute), Value: 100}, {Timestamp: now.Add(-time.Minute), Value: 0}}
			provider := &targetedChartHistoryFixture{stubGuestMetricHistoryProvider: stubGuestMetricHistoryProvider{history: map[string]map[string][]MetricPoint{
				"host-target": {"diskread": points, "diskwrite": points, "cpu": {{Timestamp: now, Value: 999}}},
			}}}
			monitor.supplementalProviders = map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: provider}
			for _, point := range points {
				monitor.metricsHistory.AddGuestMetric("agent:host-target", "cpu", 12, point.Timestamp)
			}
			result := monitor.GetGuestMetricsForChart("agent:host-target", "agent", "host-target", duration, "diskread", "diskwrite")
			if len(result["diskread"]) != 2 || len(result["diskwrite"]) != 2 || result["diskread"][1].Value != 0 || len(result["cpu"]) != 0 || provider.reads != 1 || provider.bulkReads != 0 {
				t.Fatalf("covered CPU hid requested IO or target escaped: %+v reads %d/%d", result, provider.reads, provider.bulkReads)
			}
			if provider.id != "host-target" || provider.org != "org-targeted" || provider.kind != "agent" {
				t.Fatalf("target binding changed: %+v", provider)
			}
			writeRawMetricBatch(t, monitor.metricsStore, "agent", "host-target", "diskread", points)
			writeRawMetricBatch(t, monitor.metricsStore, "agent", "host-target", "diskwrite", points)
			monitor.GetGuestMetricsForChart("agent:host-target", "agent", "host-target", duration, "diskread", "diskwrite")
			if provider.reads != 1 {
				t.Fatal("covered requested store series queried native provider")
			}
		})
	}
}
