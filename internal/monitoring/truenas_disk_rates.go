package monitoring

import (
	"math"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

// CORE reporting supplies rates, not kernel counters. Record the canonical
// selection directly; never fabricate counters or divide a rate a second time.
// Called only after the physical-disk sync's native-writer/mock/identity gates.
func (m *Monitor) writeTrueNASDiskRates(resource unifiedresources.Resource, targetID string, now time.Time) {
	if resource.Metrics == nil || m.metricsStore == nil {
		return
	}
	writes := make([]metrics.WriteMetric, 0, 2)
	for _, sample := range []struct {
		kind   string
		metric *unifiedresources.MetricValue
	}{
		{"diskread", resource.Metrics.DiskRead}, {"diskwrite", resource.Metrics.DiskWrite},
	} {
		metric := sample.metric
		if metric == nil || metric.Source != unifiedresources.SourceTrueNAS || metric.Unit != "bytes/s" ||
			metric.ObservedAt.IsZero() || metric.ObservedAt.After(now) || metric.Value < 0 || math.IsNaN(metric.Value) || math.IsInf(metric.Value, 0) {
			continue
		}
		writes = append(writes, metrics.WriteMetric{ResourceType: "disk", ResourceID: targetID, MetricType: sample.kind, Value: metric.Value, Timestamp: metric.ObservedAt, Tier: metrics.TierRaw})
	}
	writes = m.dedupeUnifiedMetricWrites(writes)
	for _, write := range writes {
		if m.metricsHistory != nil {
			m.metricsHistory.AddDiskMetric(targetID, write.MetricType, write.Value, write.Timestamp)
		}
	}
	if len(writes) > 0 {
		m.metricsStore.WriteBatchBounded(writes)
	}
}
