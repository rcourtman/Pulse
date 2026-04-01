package monitoring

import (
	"math"
	"strings"
	"time"
)

var (
	mockGuestChartMetricTypes = []string{"cpu", "memory", "disk", "diskread", "diskwrite", "netin", "netout"}
	mockNodeChartMetricTypes  = []string{"cpu", "memory", "disk", "netin", "netout"}
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

	return lttb(points, chartDownsampleTarget)
}

func mockGuestMetricsForChart(resourceType, resourceID string, duration time.Duration) map[string][]MetricPoint {
	resourceType = normalizeMockChartResourceType(resourceType)
	switch resourceType {
	case "vm", "container", "dockerContainer", "dockerHost", "agent", "k8s":
	default:
		return map[string][]MetricPoint{}
	}

	timestamps := mockChartTimestamps(duration)
	result := make(map[string][]MetricPoint, len(mockGuestChartMetricTypes))
	for _, metricType := range mockGuestChartMetricTypes {
		result[metricType] = mockCanonicalMetricSeries(resourceType, resourceID, metricType, timestamps)
	}
	return result
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

func (m *Monitor) resolveMockStorageTotal(storageID string, inMemory map[string][]MetricPoint) float64 {
	readState := m.GetUnifiedReadStateOrSnapshot()
	if readState != nil {
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

func (m *Monitor) mockStorageMetricsForChart(storageID string, duration time.Duration, inMemory map[string][]MetricPoint) map[string][]MetricPoint {
	total := m.resolveMockStorageTotal(storageID, inMemory)
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
		"usage": lttb(usagePoints, chartDownsampleTarget),
		"used":  lttb(usedPoints, chartDownsampleTarget),
		"avail": lttb(availPoints, chartDownsampleTarget),
		"total": lttb(totalPoints, chartDownsampleTarget),
	}
}

func (m *Monitor) mockPhysicalDiskTemperatureCharts(duration time.Duration) map[string]DiskChartEntry {
	if m == nil {
		return nil
	}

	readState := m.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		return nil
	}
	metricsTargetStore := m.currentMetricsTargetStore()
	timestamps := mockChartTimestamps(duration)
	if len(timestamps) == 0 {
		return nil
	}

	result := make(map[string]DiskChartEntry)
	for _, disk := range readState.PhysicalDisks() {
		if disk == nil || disk.Temperature() <= 0 {
			continue
		}

		resourceID := ""
		if metricsTargetStore != nil {
			if target := metricsTargetStore.MetricsTargetForResource(disk.ID()); target != nil {
				resourceID = strings.TrimSpace(target.ResourceID)
			}
		}
		if resourceID == "" {
			resourceID = strings.TrimSpace(disk.MetricResourceID())
		}
		if resourceID == "" {
			continue
		}

		name := strings.TrimSpace(disk.Model())
		if name == "" {
			name = strings.TrimSpace(disk.DevPath())
		}

		result[resourceID] = DiskChartEntry{
			Name:        name,
			Node:        strings.TrimSpace(disk.Node()),
			Instance:    strings.TrimSpace(disk.Instance()),
			Temperature: mockCanonicalMetricSeries("disk", resourceID, "smart_temp", timestamps),
		}
	}

	return result
}
