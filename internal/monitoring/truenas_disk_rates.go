package monitoring

import (
	"math"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

// CORE polls commonly advance the native tail once a minute. In a dense raw
// retention window, adding another point on every poll slows day reads.
// Admit at most one earlier observation per two-minute tail window.
const coreDiskHistoryTailWindowSeconds = int64(120)

func validTrueNASDiskRate(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

// Keep the native tail and, on an advancing poll, at most one earlier reading
// from the same inventory. The canonical target/mock/native-writer gates run
// before this collector. These readings are rates, not counters.
func (m *Monitor) collectTrueNASDiskRates(writes []metrics.WriteMetric, resource unifiedresources.Resource, targetID string, now time.Time, history truenas.COREHistoryDisk) []metrics.WriteMetric {
	if resource.Metrics == nil || (resource.Metrics.DiskRead == nil && resource.Metrics.DiskWrite == nil) {
		return writes
	}
	if history.TargetID != targetID || !history.InventoryAt.Equal(resource.SourceStatus[unifiedresources.SourceTrueNAS].LastSeen) {
		history = truenas.COREHistoryDisk{}
	}
	m.unifiedMetricSyncMu.Lock()
	defer m.unifiedMetricSyncMu.Unlock()
	if m.unifiedMetricSyncLast == nil || len(m.unifiedMetricSyncLast) > unifiedMetricSyncMaxSeries {
		m.unifiedMetricSyncLast = make(map[unifiedMetricSampleKey]unifiedMetricSample)
	}
	var tails, earlierWrites [2]metrics.WriteMetric
	for i, sample := range [...]struct {
		kind    string
		metric  *unifiedresources.MetricValue
		history truenas.COREHistoryDirection
	}{
		{"diskread", resource.Metrics.DiskRead, history.Read},
		{"diskwrite", resource.Metrics.DiskWrite, history.Write},
	} {
		metric := sample.metric
		if metric == nil || metric.Source != unifiedresources.SourceTrueNAS || metric.Unit != "bytes/s" ||
			metric.ObservedAt.IsZero() || metric.ObservedAt.After(now) || !validTrueNASDiskRate(metric.Value) {
			continue
		}
		key := unifiedMetricSampleKey{resourceType: "disk", resourceID: targetID, metricType: sample.kind}
		last, known := m.unifiedMetricSyncLast[key]
		tail := unifiedMetricSample{timestampUnix: metric.ObservedAt.Unix(), value: metric.Value}
		if known && (tail.timestampUnix < last.timestampUnix || tail == last) {
			continue
		}
		earlier := sample.history.Earlier
		// Same-instant scalar corrections still work, but never replay an
		// earlier observation. On a cold start only the tail is admitted.
		if known && last.timestampUnix/coreDiskHistoryTailWindowSeconds < tail.timestampUnix/coreDiskHistoryTailWindowSeconds &&
			sample.history.Tail.Timestamp.Equal(metric.ObservedAt) && sample.history.Tail.Value == metric.Value &&
			earlier.Timestamp.Unix() > last.timestampUnix && earlier.Timestamp.Unix() < tail.timestampUnix &&
			!earlier.Timestamp.Before(now.Add(-5*time.Minute)) && validTrueNASDiskRate(earlier.Value) {
			earlierWrites[i] = metrics.WriteMetric{ResourceType: "disk", ResourceID: targetID, MetricType: sample.kind, Value: earlier.Value, Timestamp: earlier.Timestamp, Tier: metrics.TierRaw}
		}
		tails[i] = metrics.WriteMetric{ResourceType: "disk", ResourceID: targetID, MetricType: sample.kind, Value: metric.Value, Timestamp: metric.ObservedAt, Tier: metrics.TierRaw}
		m.unifiedMetricSyncLast[key] = tail
	}
	// The provider selected a single native frame. If either independently
	// guarded direction cannot admit an observed companion, leave the entire
	// optional point out instead of recording a false one-direction gap.
	paired := !history.Read.Earlier.Timestamp.IsZero() && !history.Write.Earlier.Timestamp.IsZero()
	if paired && (earlierWrites[0].Timestamp.IsZero() || earlierWrites[1].Timestamp.IsZero() ||
		!earlierWrites[0].Timestamp.Equal(earlierWrites[1].Timestamp)) {
		earlierWrites = [2]metrics.WriteMetric{}
	}
	firstWrite := len(writes)
	for i, tail := range tails {
		if !earlierWrites[i].Timestamp.IsZero() {
			writes = append(writes, earlierWrites[i])
		}
		if !tail.Timestamp.IsZero() {
			writes = append(writes, tail)
		}
	}
	if m.metricsHistory != nil {
		for _, write := range writes[firstWrite:] {
			m.metricsHistory.AddDiskMetric(targetID, write.MetricType, write.Value, write.Timestamp)
		}
	}
	return writes
}
