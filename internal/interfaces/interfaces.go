package interfaces

import (
	"context"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/types"
)

// StateStore defines the interface for state management
type StateStore interface {
	GetSnapshot() models.StateSnapshot
	UpdateNodes(nodes []models.Node)
	UpdateNodesForInstance(instanceName string, nodes []models.Node)
	UpdateVMs(vms []models.VM)
	UpdateVMsForInstance(instanceName string, vms []models.VM)
	UpdateContainers(containers []models.Container)
	UpdateContainersForInstance(instanceName string, containers []models.Container)
	UpdateStorage(storage []models.Storage)
	UpdateCephClustersForInstance(instanceName string, clusters []models.CephCluster)
	UpdatePBSInstances(instances []models.PBSInstance)
	SetConnectionHealth(instance string, healthy bool)
}

// WebSocketHub defines the interface for WebSocket hub
type WebSocketHub interface {
	BroadcastState(state interface{})
	GetClientCount() int
	Run(ctx context.Context)
}

// MetricsStore defines the interface for metrics storage
type MetricsStore interface {
	AddGuestMetric(guestID string, metricType string, value float64, timestamp time.Time)
	AddNodeMetric(nodeID string, metricType string, value float64, timestamp time.Time)
	AddStorageMetric(storageID string, metricType string, value float64, timestamp time.Time)
	GetGuestMetrics(guestID string, metricType string, duration time.Duration) []types.MetricPoint
	GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []types.MetricPoint
	GetAllGuestMetrics(guestID string, duration time.Duration) map[string][]types.MetricPoint
	GetAllStorageMetrics(storageID string, duration time.Duration) map[string][]types.MetricPoint
	Cleanup()
}

// Monitor defines the interface for the monitoring system
type Monitor interface {
	Start(ctx context.Context, hub WebSocketHub)
	GetState() models.StateSnapshot
	GetStartTime() time.Time
	GetGuestMetrics(guestID string, duration time.Duration) map[string][]types.MetricPoint
	GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []types.MetricPoint
	GetStorageMetrics(storageID string, duration time.Duration) map[string][]types.MetricPoint
}

// RateTracker defines the interface for rate tracking
type RateTracker interface {
	CalculateRates(guestID string, current types.IOMetrics) (diskRead, diskWrite, netIn, netOut float64)
}
