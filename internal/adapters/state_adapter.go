package adapters

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/interfaces"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/types"
)

// StateAdapter adapts the concrete State to the StateStore interface
type StateAdapter struct {
	state *models.State
}

// NewStateAdapter creates a new state adapter
func NewStateAdapter(state *models.State) interfaces.StateStore {
	return &StateAdapter{
		state: state,
	}
}

// GetSnapshot implements StateStore interface
func (s *StateAdapter) GetSnapshot() models.StateSnapshot {
	return s.state.GetSnapshot()
}

// UpdateNodes implements StateStore interface
func (s *StateAdapter) UpdateNodes(nodes []models.Node) {
	s.state.UpdateNodes(nodes)
}

// UpdateNodesForInstance implements StateStore interface
func (s *StateAdapter) UpdateNodesForInstance(instanceName string, nodes []models.Node) {
	s.state.UpdateNodesForInstance(instanceName, nodes)
}

// UpdateVMs implements StateStore interface
func (s *StateAdapter) UpdateVMs(vms []models.VM) {
	s.state.UpdateVMs(vms)
}

// UpdateVMsForInstance implements StateStore interface
func (s *StateAdapter) UpdateVMsForInstance(instanceName string, vms []models.VM) {
	s.state.UpdateVMsForInstance(instanceName, vms)
}

// UpdateContainers implements StateStore interface
func (s *StateAdapter) UpdateContainers(containers []models.Container) {
	s.state.UpdateContainers(containers)
}

// UpdateContainersForInstance implements StateStore interface
func (s *StateAdapter) UpdateContainersForInstance(instanceName string, containers []models.Container) {
	s.state.UpdateContainersForInstance(instanceName, containers)
}

// UpdateStorage implements StateStore interface
func (s *StateAdapter) UpdateStorage(storage []models.Storage) {
	s.state.UpdateStorage(storage)
}

// UpdatePBSInstances implements StateStore interface
func (s *StateAdapter) UpdatePBSInstances(instances []models.PBSInstance) {
	s.state.UpdatePBSInstances(instances)
}

// SetConnectionHealth implements StateStore interface
func (s *StateAdapter) SetConnectionHealth(instance string, healthy bool) {
	s.state.SetConnectionHealth(instance, healthy)
}

// MetricsAdapter adapts the concrete MetricsHistory to the MetricsStore interface
type MetricsAdapter struct {
	metrics *monitoring.MetricsHistory
}

// NewMetricsAdapter creates a new metrics adapter
func NewMetricsAdapter(metrics *monitoring.MetricsHistory) interfaces.MetricsStore {
	return &MetricsAdapter{
		metrics: metrics,
	}
}

// AddGuestMetric implements MetricsStore interface
func (m *MetricsAdapter) AddGuestMetric(guestID string, metricType string, value float64, timestamp time.Time) {
	m.metrics.AddGuestMetric(guestID, metricType, value, timestamp)
}

// AddNodeMetric implements MetricsStore interface
func (m *MetricsAdapter) AddNodeMetric(nodeID string, metricType string, value float64, timestamp time.Time) {
	m.metrics.AddNodeMetric(nodeID, metricType, value, timestamp)
}

// AddStorageMetric implements MetricsStore interface
func (m *MetricsAdapter) AddStorageMetric(storageID string, metricType string, value float64, timestamp time.Time) {
	m.metrics.AddStorageMetric(storageID, metricType, value, timestamp)
}

// GetGuestMetrics implements MetricsStore interface
func (m *MetricsAdapter) GetGuestMetrics(guestID string, metricType string, duration time.Duration) []types.MetricPoint {
	return m.metrics.GetGuestMetrics(guestID, metricType, duration)
}

// GetNodeMetrics implements MetricsStore interface
func (m *MetricsAdapter) GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []types.MetricPoint {
	return m.metrics.GetNodeMetrics(nodeID, metricType, duration)
}

// GetAllGuestMetrics implements MetricsStore interface
func (m *MetricsAdapter) GetAllGuestMetrics(guestID string, duration time.Duration) map[string][]types.MetricPoint {
	return m.metrics.GetAllGuestMetrics(guestID, duration)
}

// GetAllStorageMetrics implements MetricsStore interface
func (m *MetricsAdapter) GetAllStorageMetrics(storageID string, duration time.Duration) map[string][]types.MetricPoint {
	return m.metrics.GetAllStorageMetrics(storageID, duration)
}

// Cleanup implements MetricsStore interface
func (m *MetricsAdapter) Cleanup() {
	m.metrics.Cleanup()
}

// RateTrackerAdapter adapts the concrete RateTracker to the interface
type RateTrackerAdapter struct {
	tracker *monitoring.RateTracker
}

// NewRateTrackerAdapter creates a new rate tracker adapter
func NewRateTrackerAdapter(tracker *monitoring.RateTracker) interfaces.RateTracker {
	return &RateTrackerAdapter{
		tracker: tracker,
	}
}

// CalculateRates implements RateTracker interface
func (r *RateTrackerAdapter) CalculateRates(guestID string, current types.IOMetrics) (diskRead, diskWrite, netIn, netOut float64) {
	return r.tracker.CalculateRates(guestID, current)
}
