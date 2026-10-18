package monitoring

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rs/zerolog/log"
)

const (
	metricWindowPersistentCacheTTL = 30 * time.Second
	metricWindowCacheMaxEntries    = 1024
	metricWindowCacheMaxBytes      = 16 << 20
	// Charge conservatively for the key, map/entry bookkeeping and each
	// copied timestamp/value. This is a cache working-set budget, not RSS.
	metricWindowCacheEntryBytes = 256
	metricWindowCachePointBytes = 64
)

type metricWindowCacheEntry struct {
	points    []alerts.MetricWindowPoint
	expiresAt time.Time
	bytes     int
}

func metricHistoryName(metric string) string {
	switch strings.ToLower(strings.TrimSpace(metric)) {
	case "diskread":
		return "diskread"
	case "diskwrite":
		return "diskwrite"
	case "networkin":
		return "netin"
	case "networkout":
		return "netout"
	default:
		return strings.ToLower(strings.TrimSpace(metric))
	}
}

func (m *Monitor) metricWindowPoints(request alerts.MetricWindowRequest) ([]alerts.MetricWindowPoint, error) {
	if m == nil {
		return nil, fmt.Errorf("monitor unavailable")
	}
	m.mu.RLock()
	history := m.metricsHistory
	m.mu.RUnlock()
	resourceType := strings.TrimSpace(request.ResourceType)
	resourceID := strings.TrimSpace(request.ResourceID)
	if target := m.MetricsTargetForResource(request.ResourceID); target != nil {
		if strings.TrimSpace(target.ResourceType) != "" {
			resourceType = strings.TrimSpace(target.ResourceType)
		}
		if strings.TrimSpace(target.ResourceID) != "" {
			resourceID = strings.TrimSpace(target.ResourceID)
		}
	}
	if resourceType == "" || resourceID == "" {
		return nil, fmt.Errorf("metric target unavailable for %q", request.ResourceID)
	}

	metric := metricHistoryName(request.Metric)
	duration := request.End.Sub(request.Start)
	if duration <= 0 {
		return nil, fmt.Errorf("invalid metric window")
	}
	points := m.inMemoryMetricWindow(history, resourceType, resourceID, metric, duration)
	if metricWindowCoverage(points) < duration*8/10 {
		stored := m.persistentMetricWindow(history, resourceType, resourceID, metric, request.Start, request.End)
		// Append the fresh in-memory tail last so it remains authoritative when
		// SQLite contains an older value at the same timestamp.
		points = mergeMetricWindowPoints(stored, points, request.Start, request.End)
	}

	result := make([]alerts.MetricWindowPoint, 0, len(points))
	for _, point := range points {
		result = append(result, alerts.MetricWindowPoint{Timestamp: point.Timestamp, Value: point.Value})
	}
	return result, nil
}

func (m *Monitor) inMemoryMetricWindow(history *MetricsHistory, resourceType, resourceID, metric string, duration time.Duration) []MetricPoint {
	if history == nil {
		return nil
	}
	switch strings.ToLower(resourceType) {
	case "node":
		return history.GetNodeMetrics(resourceID, metric, duration)
	case "storage":
		return history.GetAllStorageMetrics(resourceID, duration)[metric]
	case "disk":
		return history.GetDiskMetrics(resourceID, metric, duration)
	default:
		return history.GetGuestMetrics(resourceID, metric, duration)
	}
}

func (m *Monitor) persistentMetricWindow(history *MetricsHistory, resourceType, resourceID, metric string, start, end time.Time) []MetricPoint {
	if m.metricsStore == nil {
		return nil
	}
	cacheKey := strings.Join([]string{resourceType, resourceID, metric, fmt.Sprint(end.Sub(start).Seconds())}, "\x00")
	now := time.Now()
	if history != nil {
		if points, ok := history.cachedMetricWindow(cacheKey, now); ok {
			return points
		}
	}

	var best []MetricPoint
	for _, candidate := range monitorStoreResourceTypeCandidates(resourceType) {
		stored, err := m.metricsStore.Query(candidate, resourceID, metric, start, end, 0)
		if err != nil {
			log.Debug().Err(err).Str("resource", resourceID).Str("metric", metric).Msg("Rolling alert history unavailable")
			continue
		}
		converted := make([]MetricPoint, len(stored))
		for i, point := range stored {
			converted[i] = MetricPoint{Timestamp: point.Timestamp, Value: point.Value}
		}
		if metricWindowCoverage(converted) > metricWindowCoverage(best) {
			best = converted
		}
	}
	if history != nil {
		history.cacheMetricWindow(cacheKey, best, now)
	}
	return best
}

func (mh *MetricsHistory) cachedMetricWindow(key string, now time.Time) ([]MetricPoint, bool) {
	mh.metricWindowMu.Lock()
	defer mh.metricWindowMu.Unlock()
	if cached, ok := mh.metricWindowCache[key]; ok {
		if !now.Before(cached.expiresAt) {
			mh.dropMetricWindowLocked(key)
			return nil, false
		}
		points := make([]MetricPoint, len(cached.points))
		for i, point := range cached.points {
			points[i] = MetricPoint{Timestamp: point.Timestamp, Value: point.Value}
		}
		return points, true
	}
	return nil, false
}

// Overflow windows still return the complete query result through the caller;
// only the optional private copy is refused. Neither persistent history nor
// alert coverage is truncated. Admit before copying a potentially large window.
func (mh *MetricsHistory) cacheMetricWindow(key string, points []MetricPoint, now time.Time) {
	mh.metricWindowMu.Lock()
	defer mh.metricWindowMu.Unlock()
	mh.dropMetricWindowLocked(key)
	// Sweep when the earliest retained entry can have expired, not on every
	// miss at capacity. A wide live fleet otherwise scans 1024 keys per miss.
	if !mh.metricWindowSweepAt.IsZero() && !now.Before(mh.metricWindowSweepAt) {
		var next time.Time
		for cachedKey, entry := range mh.metricWindowCache {
			if !now.Before(entry.expiresAt) {
				mh.dropMetricWindowLocked(cachedKey)
			} else if next.IsZero() || entry.expiresAt.Before(next) {
				next = entry.expiresAt
			}
		}
		mh.metricWindowSweepAt = next
	}
	// Check division before multiplication so an oversized key/window cannot
	// overflow the charge or allocate an uncapped second copy.
	if len(key) > metricWindowCacheMaxBytes-metricWindowCacheEntryBytes ||
		len(points) > (metricWindowCacheMaxBytes-metricWindowCacheEntryBytes-len(key))/metricWindowCachePointBytes {
		return
	}
	charge := metricWindowCacheEntryBytes + len(key) + len(points)*metricWindowCachePointBytes
	if len(mh.metricWindowCache) >= metricWindowCacheMaxEntries || charge > metricWindowCacheMaxBytes-mh.metricWindowBytes {
		return
	}
	cached := make([]alerts.MetricWindowPoint, len(points))
	for i, point := range points {
		cached[i] = alerts.MetricWindowPoint{Timestamp: point.Timestamp, Value: point.Value}
	}
	if mh.metricWindowCache == nil {
		mh.metricWindowCache = make(map[string]metricWindowCacheEntry)
	}
	expiresAt := now.Add(metricWindowPersistentCacheTTL)
	mh.metricWindowCache[key] = metricWindowCacheEntry{points: cached, expiresAt: expiresAt, bytes: charge}
	mh.metricWindowBytes += charge
	if mh.metricWindowSweepAt.IsZero() || expiresAt.Before(mh.metricWindowSweepAt) {
		mh.metricWindowSweepAt = expiresAt
	}
}

func (mh *MetricsHistory) dropMetricWindowLocked(key string) {
	if entry, ok := mh.metricWindowCache[key]; ok {
		delete(mh.metricWindowCache, key)
		mh.metricWindowBytes -= entry.bytes
		if len(mh.metricWindowCache) == 0 {
			mh.metricWindowCache = nil
			mh.metricWindowBytes = 0
			mh.metricWindowSweepAt = time.Time{}
		}
	}
}

func metricWindowCoverage(points []MetricPoint) time.Duration {
	if len(points) < 2 {
		return 0
	}
	return points[len(points)-1].Timestamp.Sub(points[0].Timestamp)
}

func mergeMetricWindowPoints(left, right []MetricPoint, start, end time.Time) []MetricPoint {
	combined := append(append(make([]MetricPoint, 0, len(left)+len(right)), left...), right...)
	sort.SliceStable(combined, func(i, j int) bool { return combined[i].Timestamp.Before(combined[j].Timestamp) })
	result := combined[:0]
	for _, point := range combined {
		if point.Timestamp.Before(start) || point.Timestamp.After(end) {
			continue
		}
		if len(result) > 0 && result[len(result)-1].Timestamp.Equal(point.Timestamp) {
			result[len(result)-1] = point
			continue
		}
		result = append(result, point)
	}
	return result
}
