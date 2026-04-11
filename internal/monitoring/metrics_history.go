package monitoring

import (
	"sort"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// MetricPoint is an alias for models.MetricPoint
type MetricPoint = models.MetricPoint

// GuestMetrics holds historical metrics for a single guest
type GuestMetrics struct {
	CPU        []MetricPoint `json:"cpu"`
	Memory     []MetricPoint `json:"memory"`
	Disk       []MetricPoint `json:"disk"`
	DiskRead   []MetricPoint `json:"diskread"`
	DiskWrite  []MetricPoint `json:"diskwrite"`
	NetworkIn  []MetricPoint `json:"netin"`
	NetworkOut []MetricPoint `json:"netout"`
}

// StorageMetrics holds historical metrics for a single storage
type StorageMetrics struct {
	Usage []MetricPoint `json:"usage"`
	Used  []MetricPoint `json:"used"`
	Total []MetricPoint `json:"total"`
	Avail []MetricPoint `json:"avail"`
}

// DiskMetrics holds historical metrics for a single physical disk.
type DiskMetrics struct {
	Temperature []MetricPoint `json:"smart_temp"`
	Utilization []MetricPoint `json:"disk"`
	DiskRead    []MetricPoint `json:"diskread"`
	DiskWrite   []MetricPoint `json:"diskwrite"`
}

// MetricsHistory maintains historical metrics for guests, nodes, storage, and disks.
type MetricsHistory struct {
	mu             sync.RWMutex
	guestMetrics   map[string]*GuestMetrics   // key: guestID
	nodeMetrics    map[string]*GuestMetrics   // key: nodeID
	storageMetrics map[string]*StorageMetrics // key: storageID
	diskMetrics    map[string]*DiskMetrics    // key: disk metrics resource ID
	maxDataPoints  int
	retentionTime  time.Duration
}

// NewMetricsHistory creates a new metrics history tracker
func NewMetricsHistory(maxDataPoints int, retentionTime time.Duration) *MetricsHistory {
	return &MetricsHistory{
		guestMetrics:   make(map[string]*GuestMetrics),
		nodeMetrics:    make(map[string]*GuestMetrics),
		storageMetrics: make(map[string]*StorageMetrics),
		diskMetrics:    make(map[string]*DiskMetrics),
		maxDataPoints:  maxDataPoints,
		retentionTime:  retentionTime,
	}
}

// Reset clears all historical metrics data.
func (mh *MetricsHistory) Reset() {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	mh.guestMetrics = make(map[string]*GuestMetrics)
	mh.nodeMetrics = make(map[string]*GuestMetrics)
	mh.storageMetrics = make(map[string]*StorageMetrics)
	mh.diskMetrics = make(map[string]*DiskMetrics)
}

// AddGuestMetric adds a metric value for a guest
func (mh *MetricsHistory) AddGuestMetric(guestID string, metricType string, value float64, timestamp time.Time) {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	// Initialize guest metrics if not exists
	if _, exists := mh.guestMetrics[guestID]; !exists {
		mh.guestMetrics[guestID] = &GuestMetrics{}
	}

	metrics := mh.guestMetrics[guestID]
	point := MetricPoint{Value: value, Timestamp: timestamp}

	// Add metric based on type
	switch metricType {
	case "cpu":
		metrics.CPU = mh.appendMetric(metrics.CPU, point)
	case "memory":
		metrics.Memory = mh.appendMetric(metrics.Memory, point)
	case "disk":
		metrics.Disk = mh.appendMetric(metrics.Disk, point)
	case "diskread":
		metrics.DiskRead = mh.appendMetric(metrics.DiskRead, point)
	case "diskwrite":
		metrics.DiskWrite = mh.appendMetric(metrics.DiskWrite, point)
	case "netin":
		metrics.NetworkIn = mh.appendMetric(metrics.NetworkIn, point)
	case "netout":
		metrics.NetworkOut = mh.appendMetric(metrics.NetworkOut, point)
	}
}

// AddNodeMetric adds a metric value for a node
func (mh *MetricsHistory) AddNodeMetric(nodeID string, metricType string, value float64, timestamp time.Time) {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	// Initialize node metrics if not exists
	if _, exists := mh.nodeMetrics[nodeID]; !exists {
		mh.nodeMetrics[nodeID] = &GuestMetrics{}
	}

	metrics := mh.nodeMetrics[nodeID]
	point := MetricPoint{Value: value, Timestamp: timestamp}

	// Add metric based on type
	switch metricType {
	case "cpu":
		metrics.CPU = mh.appendMetric(metrics.CPU, point)
	case "memory":
		metrics.Memory = mh.appendMetric(metrics.Memory, point)
	case "disk":
		metrics.Disk = mh.appendMetric(metrics.Disk, point)
	case "netin":
		metrics.NetworkIn = mh.appendMetric(metrics.NetworkIn, point)
	case "netout":
		metrics.NetworkOut = mh.appendMetric(metrics.NetworkOut, point)
	}
}

// appendMetric appends a metric point and maintains max data points and retention
func (mh *MetricsHistory) appendMetric(metrics []MetricPoint, point MetricPoint) []MetricPoint {
	// Keep a single canonical value per timestamp so chart consumers never
	// have to guess which duplicate tail point to render.
	if len(metrics) > 0 && metrics[len(metrics)-1].Timestamp.Equal(point.Timestamp) {
		metrics[len(metrics)-1] = point
	} else {
		metrics = append(metrics, point)
	}

	// Remove old points beyond retention time
	cutoffTime := time.Now().Add(-mh.retentionTime)
	found := false
	for i, p := range metrics {
		if p.Timestamp.After(cutoffTime) {
			metrics = metrics[i:]
			found = true
			break
		}
	}
	if !found {
		metrics = metrics[:0]
	}

	// Ensure we don't exceed max data points
	if len(metrics) > mh.maxDataPoints {
		// Keep the most recent points
		metrics = metrics[len(metrics)-mh.maxDataPoints:]
	}

	return metrics
}

// GetGuestMetrics returns historical metrics for a guest
func (mh *MetricsHistory) GetGuestMetrics(guestID string, metricType string, duration time.Duration) []MetricPoint {
	mh.mu.RLock()
	defer mh.mu.RUnlock()

	metrics, exists := mh.guestMetrics[guestID]
	if !exists {
		return []MetricPoint{}
	}

	cutoffTime := time.Now().Add(-duration)
	var data []MetricPoint

	switch metricType {
	case "cpu":
		data = metrics.CPU
	case "memory":
		data = metrics.Memory
	case "disk":
		data = metrics.Disk
	case "diskread":
		data = metrics.DiskRead
	case "diskwrite":
		data = metrics.DiskWrite
	case "netin":
		data = metrics.NetworkIn
	case "netout":
		data = metrics.NetworkOut
	default:
		return []MetricPoint{}
	}

	// Filter by duration
	result := make([]MetricPoint, 0)
	for _, point := range data {
		if point.Timestamp.After(cutoffTime) {
			result = append(result, point)
		}
	}

	return result
}

// GetNodeMetrics returns historical metrics for a node
func (mh *MetricsHistory) GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []MetricPoint {
	mh.mu.RLock()
	defer mh.mu.RUnlock()

	metrics, exists := mh.nodeMetrics[nodeID]
	if !exists {
		return []MetricPoint{}
	}

	cutoffTime := time.Now().Add(-duration)
	var data []MetricPoint

	switch metricType {
	case "cpu":
		data = metrics.CPU
	case "memory":
		data = metrics.Memory
	case "disk":
		data = metrics.Disk
	case "netin":
		data = metrics.NetworkIn
	case "netout":
		data = metrics.NetworkOut
	default:
		return []MetricPoint{}
	}

	// Filter by duration
	result := make([]MetricPoint, 0)
	for _, point := range data {
		if point.Timestamp.After(cutoffTime) {
			result = append(result, point)
		}
	}

	return result
}

// filterMetricsByTime returns only the points whose timestamp is after cutoffTime.
func filterMetricsByTime(data []MetricPoint, cutoffTime time.Time) []MetricPoint {
	filtered := make([]MetricPoint, 0)
	for _, point := range data {
		if point.Timestamp.After(cutoffTime) {
			filtered = append(filtered, point)
		}
	}
	return filtered
}

func metricsSeriesCoverageSpanAfter(data []MetricPoint, cutoffTime time.Time) time.Duration {
	if len(data) < 2 {
		return 0
	}

	first := sort.Search(len(data), func(i int) bool {
		return data[i].Timestamp.After(cutoffTime)
	})
	if first >= len(data)-1 {
		return 0
	}

	oldest := data[first].Timestamp
	newest := data[len(data)-1].Timestamp
	if !newest.After(oldest) {
		return 0
	}
	return newest.Sub(oldest)
}

func guestMetricSeries(metrics *GuestMetrics, metricType string) []MetricPoint {
	if metrics == nil {
		return nil
	}

	switch metricType {
	case "cpu":
		return metrics.CPU
	case "memory":
		return metrics.Memory
	case "disk":
		return metrics.Disk
	case "diskread":
		return metrics.DiskRead
	case "diskwrite":
		return metrics.DiskWrite
	case "netin":
		return metrics.NetworkIn
	case "netout":
		return metrics.NetworkOut
	default:
		return nil
	}
}

func guestMetricsCoverageSpan(metrics *GuestMetrics, metricTypes []string, cutoffTime time.Time) time.Duration {
	if metrics == nil {
		return 0
	}

	if len(metricTypes) == 0 {
		metricTypes = []string{"cpu", "memory", "disk", "diskread", "diskwrite", "netin", "netout"}
	}

	var best time.Duration
	for _, metricType := range metricTypes {
		if span := metricsSeriesCoverageSpanAfter(guestMetricSeries(metrics, metricType), cutoffTime); span > best {
			best = span
		}
	}
	return best
}

// GuestMetricCoverageSpan returns the best in-memory coverage span across the
// requested guest metric types after applying the requested duration window.
func (mh *MetricsHistory) GuestMetricCoverageSpan(guestID string, metricTypes []string, duration time.Duration) time.Duration {
	if mh == nil {
		return 0
	}

	mh.mu.RLock()
	defer mh.mu.RUnlock()

	metrics, exists := mh.guestMetrics[guestID]
	if !exists {
		return 0
	}

	return guestMetricsCoverageSpan(metrics, metricTypes, time.Now().Add(-duration))
}

// NodeMetricCoverageSpan returns the best in-memory coverage span across the
// requested node metric types after applying the requested duration window.
func (mh *MetricsHistory) NodeMetricCoverageSpan(nodeID string, metricTypes []string, duration time.Duration) time.Duration {
	if mh == nil {
		return 0
	}

	mh.mu.RLock()
	defer mh.mu.RUnlock()

	metrics, exists := mh.nodeMetrics[nodeID]
	if !exists {
		return 0
	}

	return guestMetricsCoverageSpan(metrics, metricTypes, time.Now().Add(-duration))
}

// GetAllGuestMetrics returns all metrics for a guest within a duration
func (mh *MetricsHistory) GetAllGuestMetrics(guestID string, duration time.Duration) map[string][]MetricPoint {
	mh.mu.RLock()
	defer mh.mu.RUnlock()

	result := make(map[string][]MetricPoint)
	cutoffTime := time.Now().Add(-duration)

	metrics, exists := mh.guestMetrics[guestID]
	if !exists {
		return result
	}

	result["cpu"] = filterMetricsByTime(metrics.CPU, cutoffTime)
	result["memory"] = filterMetricsByTime(metrics.Memory, cutoffTime)
	result["disk"] = filterMetricsByTime(metrics.Disk, cutoffTime)
	result["diskread"] = filterMetricsByTime(metrics.DiskRead, cutoffTime)
	result["diskwrite"] = filterMetricsByTime(metrics.DiskWrite, cutoffTime)
	result["netin"] = filterMetricsByTime(metrics.NetworkIn, cutoffTime)
	result["netout"] = filterMetricsByTime(metrics.NetworkOut, cutoffTime)

	return result
}

// AddStorageMetric adds a metric value for storage
func (mh *MetricsHistory) AddStorageMetric(storageID string, metricType string, value float64, timestamp time.Time) {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	// Initialize storage metrics if not exists
	if _, exists := mh.storageMetrics[storageID]; !exists {
		mh.storageMetrics[storageID] = &StorageMetrics{}
	}

	metrics := mh.storageMetrics[storageID]
	point := MetricPoint{Value: value, Timestamp: timestamp}

	// Add metric based on type
	switch metricType {
	case "usage":
		metrics.Usage = mh.appendMetric(metrics.Usage, point)
	case "used":
		metrics.Used = mh.appendMetric(metrics.Used, point)
	case "total":
		metrics.Total = mh.appendMetric(metrics.Total, point)
	case "avail":
		metrics.Avail = mh.appendMetric(metrics.Avail, point)
	}
}

// GetAllStorageMetrics returns all metrics for storage within a duration
func (mh *MetricsHistory) GetAllStorageMetrics(storageID string, duration time.Duration) map[string][]MetricPoint {
	mh.mu.RLock()
	defer mh.mu.RUnlock()

	result := make(map[string][]MetricPoint)
	cutoffTime := time.Now().Add(-duration)

	metrics, exists := mh.storageMetrics[storageID]
	if !exists {
		return result
	}

	result["usage"] = filterMetricsByTime(metrics.Usage, cutoffTime)
	result["used"] = filterMetricsByTime(metrics.Used, cutoffTime)
	result["total"] = filterMetricsByTime(metrics.Total, cutoffTime)
	result["avail"] = filterMetricsByTime(metrics.Avail, cutoffTime)

	return result
}

// AddDiskMetric adds a metric value for a physical disk.
func (mh *MetricsHistory) AddDiskMetric(resourceID string, metricType string, value float64, timestamp time.Time) {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	if _, exists := mh.diskMetrics[resourceID]; !exists {
		mh.diskMetrics[resourceID] = &DiskMetrics{}
	}

	metrics := mh.diskMetrics[resourceID]
	point := MetricPoint{Value: value, Timestamp: timestamp}

	switch metricType {
	case "disk":
		metrics.Utilization = mh.appendMetric(metrics.Utilization, point)
	case "diskread":
		metrics.DiskRead = mh.appendMetric(metrics.DiskRead, point)
	case "diskwrite":
		metrics.DiskWrite = mh.appendMetric(metrics.DiskWrite, point)
	case "smart_temp":
		metrics.Temperature = mh.appendMetric(metrics.Temperature, point)
	}
}

// GetDiskMetrics returns historical metrics for a physical disk.
func (mh *MetricsHistory) GetDiskMetrics(resourceID string, metricType string, duration time.Duration) []MetricPoint {
	mh.mu.RLock()
	defer mh.mu.RUnlock()

	metrics, exists := mh.diskMetrics[resourceID]
	if !exists {
		return []MetricPoint{}
	}

	cutoffTime := time.Now().Add(-duration)

	switch metricType {
	case "disk":
		return filterMetricsByTime(metrics.Utilization, cutoffTime)
	case "diskread":
		return filterMetricsByTime(metrics.DiskRead, cutoffTime)
	case "diskwrite":
		return filterMetricsByTime(metrics.DiskWrite, cutoffTime)
	case "smart_temp":
		return filterMetricsByTime(metrics.Temperature, cutoffTime)
	default:
		return []MetricPoint{}
	}
}

// Cleanup removes old data points beyond retention time and deletes
// map entries for resources that have no remaining data points.
// This prevents unbounded memory growth when containers/VMs are deleted.
func (mh *MetricsHistory) Cleanup() {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	cutoffTime := time.Now().Add(-mh.retentionTime)
	var guestsRemoved, nodesRemoved, storageRemoved, disksRemoved int

	// Cleanup guest metrics and remove empty entries
	for key, metrics := range mh.guestMetrics {
		metrics.CPU = mh.cleanupMetrics(metrics.CPU, cutoffTime)
		metrics.Memory = mh.cleanupMetrics(metrics.Memory, cutoffTime)
		metrics.Disk = mh.cleanupMetrics(metrics.Disk, cutoffTime)
		metrics.DiskRead = mh.cleanupMetrics(metrics.DiskRead, cutoffTime)
		metrics.DiskWrite = mh.cleanupMetrics(metrics.DiskWrite, cutoffTime)
		metrics.NetworkIn = mh.cleanupMetrics(metrics.NetworkIn, cutoffTime)
		metrics.NetworkOut = mh.cleanupMetrics(metrics.NetworkOut, cutoffTime)

		// If all slices are empty, remove the map entry entirely to free memory
		if len(metrics.CPU) == 0 && len(metrics.Memory) == 0 && len(metrics.Disk) == 0 &&
			len(metrics.DiskRead) == 0 && len(metrics.DiskWrite) == 0 &&
			len(metrics.NetworkIn) == 0 && len(metrics.NetworkOut) == 0 {
			delete(mh.guestMetrics, key)
			guestsRemoved++
		}
	}

	// Cleanup node metrics and remove empty entries
	for key, metrics := range mh.nodeMetrics {
		metrics.CPU = mh.cleanupMetrics(metrics.CPU, cutoffTime)
		metrics.Memory = mh.cleanupMetrics(metrics.Memory, cutoffTime)
		metrics.Disk = mh.cleanupMetrics(metrics.Disk, cutoffTime)

		if len(metrics.CPU) == 0 && len(metrics.Memory) == 0 && len(metrics.Disk) == 0 {
			delete(mh.nodeMetrics, key)
			nodesRemoved++
		}
	}

	// Cleanup storage metrics and remove empty entries
	for key, metrics := range mh.storageMetrics {
		metrics.Usage = mh.cleanupMetrics(metrics.Usage, cutoffTime)
		metrics.Used = mh.cleanupMetrics(metrics.Used, cutoffTime)
		metrics.Total = mh.cleanupMetrics(metrics.Total, cutoffTime)
		metrics.Avail = mh.cleanupMetrics(metrics.Avail, cutoffTime)

		if len(metrics.Usage) == 0 && len(metrics.Used) == 0 &&
			len(metrics.Total) == 0 && len(metrics.Avail) == 0 {
			delete(mh.storageMetrics, key)
			storageRemoved++
		}
	}

	// Cleanup physical disk metrics and remove empty entries
	for key, metrics := range mh.diskMetrics {
		metrics.Temperature = mh.cleanupMetrics(metrics.Temperature, cutoffTime)
		metrics.Utilization = mh.cleanupMetrics(metrics.Utilization, cutoffTime)
		metrics.DiskRead = mh.cleanupMetrics(metrics.DiskRead, cutoffTime)
		metrics.DiskWrite = mh.cleanupMetrics(metrics.DiskWrite, cutoffTime)

		if len(metrics.Temperature) == 0 && len(metrics.Utilization) == 0 &&
			len(metrics.DiskRead) == 0 && len(metrics.DiskWrite) == 0 {
			delete(mh.diskMetrics, key)
			disksRemoved++
		}
	}

	// Log cleanup activity at debug level
	if guestsRemoved > 0 || nodesRemoved > 0 || storageRemoved > 0 || disksRemoved > 0 {
		log.Debug().
			Int("guestsRemoved", guestsRemoved).
			Int("nodesRemoved", nodesRemoved).
			Int("storageRemoved", storageRemoved).
			Int("disksRemoved", disksRemoved).
			Int("guestsRemaining", len(mh.guestMetrics)).
			Int("nodesRemaining", len(mh.nodeMetrics)).
			Int("storageRemaining", len(mh.storageMetrics)).
			Int("disksRemaining", len(mh.diskMetrics)).
			Msg("Cleaned up stale metrics history entries")
	}
}

// cleanupMetrics removes points older than cutoff time.
// Returns nil (not empty slice) when all points are expired to release backing array memory.
func (mh *MetricsHistory) cleanupMetrics(metrics []MetricPoint, cutoffTime time.Time) []MetricPoint {
	for i, p := range metrics {
		if p.Timestamp.After(cutoffTime) {
			return metrics[i:]
		}
	}
	// Return nil instead of metrics[:0] to release the backing array
	return nil
}
