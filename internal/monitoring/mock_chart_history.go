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
	return mockChartTimestampsAnchored(now, duration)
}

func mockChartTimestampsAnchored(end time.Time, duration time.Duration) []time.Time {
	if duration <= 0 {
		duration = time.Hour
	}

	anchor := normalizeMockMetricTimestamp(end.UTC(), defaultMockSampleInterval)
	timestamps := buildTieredTimestamps(anchor, duration)
	if len(timestamps) > 0 {
		// Canonical values depend only on identity and timestamp. Select the
		// chart's retained timestamps before synthesizing values, rather than
		// computing a dense history for every guest and discarding most of it.
		return sampleOnMockChartGrid(timestamps, func(at time.Time) time.Time { return at }, mockChartGridInterval(duration))
	}

	return []time.Time{
		anchor.Add(-duration),
		anchor,
	}
}

func mockChartTimestampsInWindow(start, end time.Time) []time.Time {
	if !end.After(start) {
		return nil
	}
	duration := end.Sub(start)
	// Generate on the canonical grid anchored at end, then filter strictly to
	// [start, end] so custom windows stay deterministic and within bounds.
	candidates := mockChartTimestampsAnchored(end, duration)
	filtered := make([]time.Time, 0, len(candidates))
	for _, ts := range candidates {
		if !ts.Before(start) && !ts.After(end) {
			filtered = append(filtered, ts)
		}
	}
	if len(filtered) >= 2 {
		return filtered
	}
	// Short sub-minute or off-grid windows still need deterministic points inside [start, end].
	interval := defaultMockSampleInterval
	if duration < interval*2 {
		interval = max(time.Second, duration/12)
	}
	startSec := start.UTC().Truncate(time.Second)
	if startSec.Before(start) {
		startSec = startSec.Add(time.Second)
	}
	endSec := end.UTC().Truncate(time.Second)
	if endSec.Before(startSec) {
		return nil
	}
	out := make([]time.Time, 0, 24)
	for ts := startSec; !ts.After(endSec) && len(out) < 240; ts = ts.Add(interval) {
		out = append(out, ts)
	}
	if len(out) > 0 && out[len(out)-1].Before(endSec) {
		out = append(out, endSec)
	}
	return out
}

func mockGuestMetricsForWindow(resourceType, resourceID string, start, end time.Time) map[string][]MetricPoint {
	resourceType = normalizeMockChartResourceType(resourceType)
	switch resourceType {
	case "vm", "container", "dockerContainer", "dockerHost", "agent", "k8s":
	default:
		return map[string][]MetricPoint{}
	}

	timestamps := mockChartTimestampsInWindow(start, end)
	if len(timestamps) == 0 {
		return map[string][]MetricPoint{}
	}
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

func mockNodeMetricsForWindow(nodeID string, metricTypes []string, start, end time.Time) map[string][]MetricPoint {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return map[string][]MetricPoint{}
	}
	if len(metricTypes) == 0 {
		metricTypes = mockNodeChartMetricTypes
	}
	timestamps := mockChartTimestampsInWindow(start, end)
	if len(timestamps) == 0 {
		return map[string][]MetricPoint{}
	}
	result := make(map[string][]MetricPoint, len(metricTypes))
	for _, metricType := range metricTypes {
		result[metricType] = mockCanonicalMetricSeries("node", nodeID, metricType, timestamps)
	}
	return result
}

func isSupportedMockDiskMetric(metricType string) bool {
	for _, supported := range mockDiskChartMetricTypes {
		if metricType == supported {
			return true
		}
	}
	return false
}

func mockDiskMetricsForWindow(resourceID string, metricTypes []string, start, end time.Time) map[string][]MetricPoint {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return map[string][]MetricPoint{}
	}
	if len(metricTypes) == 0 {
		metricTypes = mockDiskChartMetricTypes
	}
	timestamps := mockChartTimestampsInWindow(start, end)
	if len(timestamps) == 0 {
		return map[string][]MetricPoint{}
	}
	result := make(map[string][]MetricPoint, len(metricTypes))
	for _, metricType := range metricTypes {
		if !isSupportedMockDiskMetric(metricType) {
			continue
		}
		result[metricType] = mockCanonicalMetricSeries("disk", resourceID, metricType, timestamps)
	}
	return result
}

// MockStorageUsageForWindow synthesizes deterministic mock storage usage
// readings strictly inside [start, end] from identity and timestamp alone,
// without anchoring historical slices to the current live reading.
func MockStorageUsageForWindow(storageID string, start, end time.Time) []MetricPoint {
	storageID = strings.TrimSpace(storageID)
	if storageID == "" {
		return nil
	}
	timestamps := mockChartTimestampsInWindow(start, end)
	if len(timestamps) == 0 {
		return nil
	}
	return mockCanonicalMetricSeries("storage", storageID, "usage", timestamps)
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
	return sampleOnMockChartGrid(points, func(point MetricPoint) time.Time { return point.Timestamp }, interval)
}

func sampleOnMockChartGrid[T any](points []T, timestamp func(T) time.Time, interval time.Duration) []T {
	if len(points) == 0 {
		return nil
	}
	width := int64(interval)
	if width <= 0 {
		return append([]T(nil), points...)
	}

	bucketOf := func(ts time.Time) int64 {
		nanos := ts.UnixNano()
		bucket := nanos / width
		if nanos < 0 && nanos%width != 0 {
			bucket--
		}
		return bucket
	}

	var sampled []T
	for i, point := range points {
		if i > 0 && i < len(points)-1 && bucketOf(timestamp(point)) == bucketOf(timestamp(points[i-1])) {
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
