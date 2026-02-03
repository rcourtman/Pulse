package monitoring

import (
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/types"
	"github.com/rs/zerolog/log"
)

// Use MetricPoint from types package
type MetricPoint = types.MetricPoint

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

// MetricsHistory maintains historical metrics for all guests and nodes
type MetricsHistory struct {
	mu             sync.RWMutex
	guestMetrics   map[string]*GuestMetrics   // key: guestID
	nodeMetrics    map[string]*GuestMetrics   // key: nodeID
	storageMetrics map[string]*StorageMetrics // key: storageID
	maxDataPoints  int
	retentionTime  time.Duration
}

// NewMetricsHistory creates a new metrics history tracker
func NewMetricsHistory(maxDataPoints int, retentionTime time.Duration) *MetricsHistory {
	return &MetricsHistory{
		guestMetrics:   make(map[string]*GuestMetrics),
		nodeMetrics:    make(map[string]*GuestMetrics),
		storageMetrics: make(map[string]*StorageMetrics),
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
}

// AddGuestMetric adds a metric value for a guest
func (mh *MetricsHistory) AddGuestMetric(guestID string, metricType string, value float64, timestamp time.Time) {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	// Initialize guest metrics if not exists
	if _, exists := mh.guestMetrics[guestID]; !exists {
		mh.guestMetrics[guestID] = &GuestMetrics{
			CPU:        make([]MetricPoint, 0, mh.maxDataPoints),
			Memory:     make([]MetricPoint, 0, mh.maxDataPoints),
			Disk:       make([]MetricPoint, 0, mh.maxDataPoints),
			DiskRead:   make([]MetricPoint, 0, mh.maxDataPoints),
			DiskWrite:  make([]MetricPoint, 0, mh.maxDataPoints),
			NetworkIn:  make([]MetricPoint, 0, mh.maxDataPoints),
			NetworkOut: make([]MetricPoint, 0, mh.maxDataPoints),
		}
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
		mh.nodeMetrics[nodeID] = &GuestMetrics{
			CPU:    make([]MetricPoint, 0, mh.maxDataPoints),
			Memory: make([]MetricPoint, 0, mh.maxDataPoints),
			Disk:   make([]MetricPoint, 0, mh.maxDataPoints),
		}
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
	}
}

// appendMetric appends a metric point and maintains max data points and retention
func (mh *MetricsHistory) appendMetric(metrics []MetricPoint, point MetricPoint) []MetricPoint {
	// Append new point
	metrics = append(metrics, point)

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

	// Helper function to filter by time
	filterByTime := func(data []MetricPoint) []MetricPoint {
		filtered := make([]MetricPoint, 0)
		for _, point := range data {
			if point.Timestamp.After(cutoffTime) {
				filtered = append(filtered, point)
			}
		}
		return filtered
	}

	result["cpu"] = filterByTime(metrics.CPU)
	result["memory"] = filterByTime(metrics.Memory)
	result["disk"] = filterByTime(metrics.Disk)
	result["diskread"] = filterByTime(metrics.DiskRead)
	result["diskwrite"] = filterByTime(metrics.DiskWrite)
	result["netin"] = filterByTime(metrics.NetworkIn)
	result["netout"] = filterByTime(metrics.NetworkOut)

	return result
}

// AddStorageMetric adds a metric value for storage
func (mh *MetricsHistory) AddStorageMetric(storageID string, metricType string, value float64, timestamp time.Time) {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	// Initialize storage metrics if not exists
	if _, exists := mh.storageMetrics[storageID]; !exists {
		mh.storageMetrics[storageID] = &StorageMetrics{
			Usage: make([]MetricPoint, 0, mh.maxDataPoints),
			Used:  make([]MetricPoint, 0, mh.maxDataPoints),
			Total: make([]MetricPoint, 0, mh.maxDataPoints),
			Avail: make([]MetricPoint, 0, mh.maxDataPoints),
		}
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

	// Helper function to filter by time
	filterByTime := func(data []MetricPoint) []MetricPoint {
		filtered := make([]MetricPoint, 0)
		for _, point := range data {
			if point.Timestamp.After(cutoffTime) {
				filtered = append(filtered, point)
			}
		}
		return filtered
	}

	result["usage"] = filterByTime(metrics.Usage)
	result["used"] = filterByTime(metrics.Used)
	result["total"] = filterByTime(metrics.Total)
	result["avail"] = filterByTime(metrics.Avail)

	return result
}

// Cleanup removes old data points beyond retention time and deletes
// map entries for resources that have no remaining data points.
// This prevents unbounded memory growth when containers/VMs are deleted.
func (mh *MetricsHistory) Cleanup() {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	cutoffTime := time.Now().Add(-mh.retentionTime)
	var guestsRemoved, nodesRemoved, storageRemoved int

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

	// Log cleanup activity at debug level
	if guestsRemoved > 0 || nodesRemoved > 0 || storageRemoved > 0 {
		log.Debug().
			Int("guestsRemoved", guestsRemoved).
			Int("nodesRemoved", nodesRemoved).
			Int("storageRemoved", storageRemoved).
			Int("guestsRemaining", len(mh.guestMetrics)).
			Int("nodesRemaining", len(mh.nodeMetrics)).
			Int("storageRemaining", len(mh.storageMetrics)).
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
