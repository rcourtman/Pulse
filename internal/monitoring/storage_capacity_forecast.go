package monitoring

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

const (
	storageCapacityForecastLookback = 7 * 24 * time.Hour
	storageCapacityForecastRefresh  = 15 * time.Minute
)

type storageCapacityForecastCacheEntry struct {
	trend     alerts.CapacityTrendObservation
	expiresAt time.Time
}

// storageCapacityTrend bridges monitoring-owned history into alert-owned
// predictive policy. SQLite history is included so a process restart does not
// erase confidence; the in-memory tail and current observation cover buffered
// writes and installations where durable metrics are unavailable.
func (m *Monitor) storageCapacityTrend(storage models.Storage, now time.Time) alerts.CapacityTrendObservation {
	if m == nil || storage.ID == "" || storage.Usage <= 0 {
		return alerts.CapacityTrendObservation{Reason: "invalid-current-usage"}
	}
	return m.storageCapacityTrendFor(storage.ID, storage.ID, storage.Usage, now)
}

func (m *Monitor) storageCapacityTrendFor(cacheID, historyID string, currentUsage float64, now time.Time) alerts.CapacityTrendObservation {
	if m == nil || cacheID == "" || historyID == "" || currentUsage <= 0 {
		return alerts.CapacityTrendObservation{Reason: "invalid-current-usage"}
	}
	if now.IsZero() {
		now = time.Now()
	}

	if m.metricsHistory != nil {
		m.metricsHistory.capacityForecastMu.Lock()
		if cached, ok := m.metricsHistory.capacityForecastCache[cacheID]; ok && now.Before(cached.expiresAt) {
			m.metricsHistory.capacityForecastMu.Unlock()
			return cached.trend
		}
		m.metricsHistory.capacityForecastMu.Unlock()
	}

	points := make([]alerts.CapacityMetricPoint, 0, 256)
	if m.metricsStore != nil {
		stored, err := m.metricsStore.Query(
			"storage",
			historyID,
			"usage",
			now.Add(-storageCapacityForecastLookback),
			now,
			int64(time.Hour/time.Second),
		)
		if err != nil {
			log.Debug().Err(err).Str("storage", cacheID).Msg("Persistent capacity history unavailable; using in-memory tail")
		} else {
			for _, point := range stored {
				points = append(points, alerts.CapacityMetricPoint{Timestamp: point.Timestamp, Value: point.Value})
			}
		}
	}
	if m.metricsHistory != nil {
		for _, point := range m.metricsHistory.GetAllStorageMetrics(historyID, storageCapacityForecastLookback)["usage"] {
			points = append(points, alerts.CapacityMetricPoint{Timestamp: point.Timestamp, Value: point.Value})
		}
	}
	points = append(points, alerts.CapacityMetricPoint{Timestamp: now, Value: currentUsage})

	trend := alerts.EstimateCapacityTrend(points, now)
	if m.metricsHistory != nil {
		m.metricsHistory.capacityForecastMu.Lock()
		if m.metricsHistory.capacityForecastCache == nil {
			m.metricsHistory.capacityForecastCache = make(map[string]storageCapacityForecastCacheEntry)
		}
		m.metricsHistory.capacityForecastCache[cacheID] = storageCapacityForecastCacheEntry{
			trend:     trend,
			expiresAt: now.Add(storageCapacityForecastRefresh),
		}
		m.metricsHistory.capacityForecastMu.Unlock()
	}
	return trend
}

func (m *Monitor) unifiedStorageCapacityTrends(resources []unifiedresources.Resource, now time.Time) map[string]alerts.CapacityTrendObservation {
	trends := make(map[string]alerts.CapacityTrendObservation)
	var resolver MetricsTargetResourceStore
	if candidate, ok := m.resourceStore.(MetricsTargetResourceStore); ok {
		resolver = candidate
	}
	for _, resource := range resources {
		input, ok := alerts.UnifiedResourceInputFromResource(resource)
		if !ok || input.Disk == nil {
			continue
		}
		switch input.Type {
		case "truenas-pool", "truenas-dataset", "vmware-datastore":
		default:
			continue
		}
		historyID := input.ID
		if resolver != nil {
			if target := resolver.MetricsTargetForResource(resource.ID); target != nil && target.ResourceType == "storage" && target.ResourceID != "" {
				historyID = target.ResourceID
			}
		}
		trends[input.ID] = m.storageCapacityTrendFor(input.ID, historyID, input.DiskValue(), now)
	}
	return trends
}
