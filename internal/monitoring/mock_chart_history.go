package monitoring

import (
	"math"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

var (
	mockGuestChartMetricTypes = []string{"cpu", "memory", "disk", "diskread", "diskwrite", "netin", "netout"}
	mockHostChartMetricTypes  = []string{"cpu", "memory", "disk", "diskread", "diskwrite", "netin", "netout", "temperature"}
	mockNodeChartMetricTypes  = []string{"cpu", "memory", "disk", "netin", "netout", "temperature"}
	mockDiskChartMetricTypes  = []string{"disk", "diskread", "diskwrite", "smart_temp"}
)

func normalizeMockChartResourceType(resourceType string) string {
	switch strings.TrimSpace(resourceType) {
	case "system-container":
		return "container"
	case "app-container", "docker":
		return "dockerContainer"
	case "pod":
		return "k8s"
	default:
		return strings.TrimSpace(resourceType)
	}
}

func mockChartTimestamps(duration time.Duration) []time.Time {
	if duration <= 0 {
		duration = time.Hour
	}

	now := normalizeMockMetricTimestamp(time.Now().UTC(), defaultMockSampleInterval)
	timestamps := buildTieredTimestamps(now, duration)
	if len(timestamps) > 0 {
		return timestamps
	}

	return []time.Time{
		now.Add(-duration),
		now,
	}
}

func mockCanonicalMetricSeries(resourceType, resourceID, metricType string, timestamps []time.Time) []MetricPoint {
	resourceType = normalizeMockChartResourceType(resourceType)
	resourceID = strings.TrimSpace(resourceID)
	metricType = strings.TrimSpace(metricType)
	if resourceType == "" || resourceID == "" || metricType == "" || len(timestamps) == 0 {
		return nil
	}

	values := canonicalMetricSeries(resourceType, resourceID, metricType, timestamps)
	if len(values) == 0 {
		return nil
	}

	points := make([]MetricPoint, len(values))
	for i, value := range values {
		points[i] = MetricPoint{
			Timestamp: timestamps[i],
			Value:     value,
		}
	}

	return points
}

// mockChartGridInterval is the bucket width that keeps a mock chart within
// mockChartPointTarget points. A window that does not start on a bucket edge
// touches one more bucket than it spans, and the newest reading is kept on top
// of one reading per bucket, hence target-2.
func mockChartGridInterval(duration time.Duration) time.Duration {
	if duration <= 0 {
		duration = time.Hour
	}
	buckets := time.Duration(mockChartPointTarget(duration) - 2)
	if buckets < 1 {
		buckets = 1
	}
	interval := (duration + buckets - 1) / buckets
	return ((interval + time.Second - 1) / time.Second) * time.Second
}

// sampleMetricSeriesOnMockChartGrid keeps the first reading in each
// wall-clock bucket of the given width, plus the newest reading so the chart
// ends at the live value. Buckets are aligned to the Unix epoch and readings
// keep their own timestamps, so which readings survive depends only on when
// they were taken, never on their values: every metric of one resource
// sampled at the same times keeps the same timestamps, as the poller writes
// them. points must be in ascending timestamp order.
func sampleMetricSeriesOnMockChartGrid(points []MetricPoint, interval time.Duration) []MetricPoint {
	if len(points) == 0 {
		return nil
	}
	width := int64(interval)
	if width <= 0 {
		return cloneMetricSeries(points)
	}

	bucketOf := func(ts time.Time) int64 {
		nanos := ts.UnixNano()
		bucket := nanos / width
		if nanos < 0 && nanos%width != 0 {
			bucket--
		}
		return bucket
	}

	sampled := make([]MetricPoint, 0, len(points))
	for i, point := range points {
		if i > 0 && i < len(points)-1 && bucketOf(point.Timestamp) == bucketOf(points[i-1].Timestamp) {
			continue
		}
		sampled = append(sampled, point)
	}
	return sampled
}

func mockGuestMetricsForChart(resourceType, resourceID string, duration time.Duration) map[string][]MetricPoint {
	resourceType = normalizeMockChartResourceType(resourceType)
	switch resourceType {
	case "vm", "container", "dockerContainer", "dockerHost", "agent", "k8s":
	default:
		return map[string][]MetricPoint{}
	}

	timestamps := mockChartTimestamps(duration)
	metricTypes := mockGuestChartMetricTypes
	if resourceType == "agent" || resourceType == "dockerHost" {
		metricTypes = mockHostChartMetricTypes
	}
	result := make(map[string][]MetricPoint, len(metricTypes)+1)
	for _, metricType := range metricTypes {
		result[metricType] = mockCanonicalMetricSeries(resourceType, resourceID, metricType, timestamps)
	}
	if series := mockGuestMemoryUsedSeries(resourceType, resourceID, result["memory"]); len(series) > 0 {
		result["memoryused"] = series
	}
	return result
}

// mockGuestMemoryUsedSeries derives the host-relative memory byte series from
// the guest-relative percentage series and the fixture memory capacity, the
// same derivation live mock ticks and the history seeder use. Without it a
// chart window that falls outside the seeded history, such as 7d, returns no
// `memoryused` at all and the host-capacity memory column has nothing to draw.
func mockGuestMemoryUsedSeries(resourceType, resourceID string, memoryPercent []MetricPoint) []MetricPoint {
	switch resourceType {
	case "vm", "container":
	default:
		return nil
	}
	if len(memoryPercent) == 0 {
		return nil
	}

	total := mock.MemoryTotalForResource(resourceType, resourceID)
	if total <= 0 {
		return nil
	}

	series := make([]MetricPoint, len(memoryPercent))
	for i, point := range memoryPercent {
		percent := math.Max(0, math.Min(100, point.Value))
		series[i] = MetricPoint{
			Timestamp: point.Timestamp,
			Value:     total * (percent / 100),
		}
	}
	return series
}

func mockNodeMetricsForChart(nodeID string, metricTypes []string, duration time.Duration) map[string][]MetricPoint {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return map[string][]MetricPoint{}
	}
	if len(metricTypes) == 0 {
		metricTypes = mockNodeChartMetricTypes
	}

	timestamps := mockChartTimestamps(duration)
	result := make(map[string][]MetricPoint, len(metricTypes))
	for _, metricType := range metricTypes {
		result[metricType] = mockCanonicalMetricSeries("node", nodeID, metricType, timestamps)
	}
	return result
}

func mockDiskMetricsForChart(resourceID string, metricTypes []string, duration time.Duration) map[string][]MetricPoint {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return map[string][]MetricPoint{}
	}
	if len(metricTypes) == 0 {
		metricTypes = mockDiskChartMetricTypes
	}

	timestamps := mockChartTimestamps(duration)
	result := make(map[string][]MetricPoint, len(metricTypes))
	for _, metricType := range metricTypes {
		result[metricType] = mockCanonicalMetricSeries("disk", resourceID, metricType, timestamps)
	}
	return result
}

func latestMetricPoint(points []MetricPoint) (MetricPoint, bool) {
	if len(points) == 0 {
		return MetricPoint{}, false
	}

	latest := points[0]
	for i := 1; i < len(points); i++ {
		if points[i].Timestamp.After(latest.Timestamp) {
			latest = points[i]
		}
	}
	return latest, true
}

// resolveMockStorageTotal totals one pool for a chart. readState lists the pools
// and is read here only if the pool's chart is not cached, so a request over
// many pools reads the estate view once (see GetStorageMetricsForChartBatch)
// and not once per pool, which on a starved process rebuilt the view for each.
// It is not the structure view: a linked pool takes its capacity from whichever
// source was seen last, and that moves without a structure change.
func (m *Monitor) resolveMockStorageTotal(storageID string, inMemory map[string][]MetricPoint, readState func() unifiedresources.ReadState) float64 {
	if readState := readState(); readState != nil {
		for _, pool := range readState.StoragePools() {
			if pool == nil || strings.TrimSpace(pool.SourceID()) != strings.TrimSpace(storageID) {
				continue
			}
			if total := float64(pool.DiskTotal()); total > 0 {
				return total
			}
		}
	}

	if latest, ok := latestMetricPoint(inMemory["total"]); ok && latest.Value > 0 {
		return latest.Value
	}

	return 0
}

func (m *Monitor) mockStorageMetricsForChart(storageID string, duration time.Duration, inMemory map[string][]MetricPoint, readState func() unifiedresources.ReadState) map[string][]MetricPoint {
	total := m.resolveMockStorageTotal(storageID, inMemory, readState)
	if total <= 0 {
		return cloneMetricPointMap(inMemory)
	}

	timestamps := mockChartTimestamps(duration)
	usageValues := canonicalMetricSeries("storage", storageID, "usage", timestamps)
	if len(usageValues) == 0 {
		return cloneMetricPointMap(inMemory)
	}

	usagePoints := make([]MetricPoint, len(timestamps))
	usedPoints := make([]MetricPoint, len(timestamps))
	availPoints := make([]MetricPoint, len(timestamps))
	totalPoints := make([]MetricPoint, len(timestamps))
	for i, ts := range timestamps {
		usage := clampFloat(usageValues[i], 0, 100)
		used := total * (usage / 100.0)
		avail := math.Max(0, total-used)
		usagePoints[i] = MetricPoint{Timestamp: ts, Value: usage}
		usedPoints[i] = MetricPoint{Timestamp: ts, Value: used}
		availPoints[i] = MetricPoint{Timestamp: ts, Value: avail}
		totalPoints[i] = MetricPoint{Timestamp: ts, Value: total}
	}

	return map[string][]MetricPoint{
		"usage": usagePoints,
		"used":  usedPoints,
		"avail": availPoints,
		"total": totalPoints,
	}
}
