package mock

import (
	"math"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func normalizeMetricClass(resourceClass string) string {
	normalized := strings.ToLower(strings.TrimSpace(resourceClass))
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	return normalized
}

func MetricSeed(resourceClass, resourceID, metric string) uint64 {
	classKey := normalizeMetricClass(resourceClass)
	idKey := strings.TrimSpace(resourceID)
	metricKey := strings.ToLower(strings.TrimSpace(metric))
	return mockStableHash64(classKey, idKey, metricKey)
}

func MetricBounds(resourceClass, metric string) (float64, float64) {
	classKey := normalizeMetricClass(resourceClass)
	metricKey := strings.ToLower(strings.TrimSpace(metric))

	switch metricKey {
	case "cpu":
		switch classKey {
		case "node":
			return 5, 85
		case "vm":
			return 1, 85
		case "container":
			return 1, 75
		case "agent":
			return 4, 97
		case "dockerhost":
			return 3, 99
		case "dockercontainer":
			return 0, 190
		case "pbs", "pbsdatastore":
			return 6, 62
		default:
			return 0, 100
		}
	case "memory":
		switch classKey {
		case "node", "vm":
			return 10, 85
		case "container":
			return 5, 85
		case "agent", "dockerhost":
			return 12, 96
		case "dockercontainer":
			return 1, 97
		case "pbs", "pbsdatastore":
			return 24, 78
		default:
			return 0, 100
		}
	case "disk":
		switch classKey {
		case "node":
			return 5, 95
		case "vm":
			return 10, 90
		case "container":
			return 5, 85
		case "agent":
			return 5, 98
		case "dockerhost":
			return 8, 92
		case "dockercontainer":
			return 5, 72
		default:
			return 0, 100
		}
	case "usage":
		switch classKey {
		case "pbs", "pbsdatastore":
			return 12, 78
		case "storage", "pool", "dataset":
			return 8, 88
		default:
			return 0, 100
		}
	case "diskread":
		switch classKey {
		case "container", "dockercontainer":
			return 0, 30 * 1024 * 1024
		case "vm":
			return 0, 100 * 1024 * 1024
		case "agent", "dockerhost":
			return 0, 120 * 1024 * 1024
		default:
			return 0, 80 * 1024 * 1024
		}
	case "diskwrite":
		switch classKey {
		case "container", "dockercontainer":
			return 0, 18 * 1024 * 1024
		case "vm":
			return 0, 60 * 1024 * 1024
		case "agent", "dockerhost":
			return 0, 90 * 1024 * 1024
		default:
			return 0, 48 * 1024 * 1024
		}
	case "netin":
		switch classKey {
		case "container", "dockercontainer":
			return 0, 40 * 1024 * 1024
		case "vm":
			return 0, 80 * 1024 * 1024
		case "agent", "dockerhost":
			return 0, 250 * 1024 * 1024
		default:
			return 0, 64 * 1024 * 1024
		}
	case "netout":
		switch classKey {
		case "container", "dockercontainer":
			return 0, 24 * 1024 * 1024
		case "vm":
			return 0, 48 * 1024 * 1024
		case "agent", "dockerhost":
			return 0, 200 * 1024 * 1024
		default:
			return 0, 48 * 1024 * 1024
		}
	case "smart_temp", "temperature", "temp":
		return 25, 95
	default:
		return 0, 100
	}
}

func MetricBoundsForResource(resourceClass, resourceID, metric string) (float64, float64) {
	min, max := MetricBounds(resourceClass, metric)
	role := MetricRole(resourceClass, resourceID)
	metricKey := strings.ToLower(strings.TrimSpace(metric))

	switch metricKey {
	case "cpu":
		switch role {
		case metricRoleDatabase:
			min, max = maxFloat(min, 8), max*1.05
		case metricRoleCI, metricRoleBatch:
			max *= 1.18
		case metricRoleBackup, metricRoleStorage:
			max *= 0.9
		case metricRoleWeb, metricRoleAPI, metricRoleIngress, metricRoleMedia:
			max *= 1.1
		}
	case "memory":
		switch role {
		case metricRoleDatabase, metricRoleCache:
			min, max = maxFloat(min, 24), max*1.08
		case metricRoleMonitoring:
			min, max = maxFloat(min, 18), max*1.03
		case metricRoleBackup:
			max *= 0.85
		case metricRoleWeb, metricRoleIngress:
			max *= 0.92
		}
	case "disk", "usage":
		switch role {
		case metricRoleBackup, metricRoleStorage:
			min, max = maxFloat(min, 12), max*1.05
		case metricRoleCache, metricRoleIngress:
			max *= 0.82
		}
	case "diskread", "diskwrite":
		switch role {
		case metricRoleBackup, metricRoleDatabase, metricRoleBatch, metricRoleStorage:
			max *= 1.4
		case metricRoleWeb, metricRoleIngress:
			max *= 0.75
		case metricRoleCache:
			max *= 0.65
		}
	case "netin", "netout":
		switch role {
		case metricRoleWeb, metricRoleAPI, metricRoleIngress, metricRoleMedia, metricRoleSecurity:
			max *= 1.35
		case metricRoleBackup:
			max *= 1.2
		case metricRoleDatabase, metricRoleStorage:
			max *= 0.78
		}
	case "smart_temp", "temperature", "temp":
		switch role {
		case metricRoleMedia, metricRoleCI, metricRoleDatabase:
			max += 4
		case metricRoleBackup:
			max -= 2
		}
	}

	if max <= min {
		max = min + 1
	}
	return min, max
}

func metricSpeed(metric string) float64 {
	switch strings.ToLower(strings.TrimSpace(metric)) {
	case "memory":
		return 0.5
	case "disk", "usage", "storage", "capacity", "smart_temp", "temperature", "temp":
		return 0.12
	default:
		return 1.0
	}
}

func SampleMetric(resourceClass, resourceID, metric string, at time.Time) float64 {
	min, max := MetricBoundsForResource(resourceClass, resourceID, metric)
	return sampleNaturalMetric(resourceClass, resourceID, metric, min, max, metricSpeed(metric), at)
}

func SampleMetricInt(resourceClass, resourceID, metric string, at time.Time) int64 {
	return int64(math.Round(SampleMetric(resourceClass, resourceID, metric, at)))
}

func ApplyStorageUsage(storage *models.Storage, usage float64) {
	if storage == nil || storage.Total <= 0 {
		return
	}

	storage.Usage = clampFloat(usage, 0, 100)
	storage.Used = int64(float64(storage.Total) * (storage.Usage / 100.0))
	if storage.Used < 0 {
		storage.Used = 0
	}
	if storage.Used > storage.Total {
		storage.Used = storage.Total
	}
	storage.Free = storage.Total - storage.Used
	if storage.Free < 0 {
		storage.Free = 0
	}
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
