package tools

import (
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// GetResourceMetricsForTarget reads the retained store using the registry's
// metrics coordinates. A process restart must not shorten a requested history
// window. The in-memory source remains available on installations without a store.
func (a *MetricsHistoryToolAdapter) GetResourceMetricsForTarget(target unifiedresources.MetricsTarget, period time.Duration) ([]MetricPoint, error) {
	if a.retainedMetrics == nil {
		return a.GetResourceMetrics(target.ResourceID, period)
	}
	var storeTypes []string
	switch target.ResourceType {
	case "agent":
		storeTypes = []string{"agent", "node"}
	case "node", "vm":
		storeTypes = []string{target.ResourceType}
	case "system-container", "container":
		storeTypes = []string{"container"}
	default:
		return nil, fmt.Errorf("unsupported metrics target type %q", target.ResourceType)
	}
	end := time.Now()
	start := end.Add(-period)
	for _, storeType := range storeTypes {
		observations := make(map[string][]RawMetricPoint)
		for _, metric := range []string{"cpu", "memory", "disk"} {
			points, err := a.retainedMetrics.Query(storeType, target.ResourceID, metric, start, end, 0)
			if err != nil {
				return nil, err
			}
			for _, point := range points {
				observations[metric] = append(observations[metric], RawMetricPoint{Timestamp: point.Timestamp, Value: point.Value})
			}
		}
		if len(observations) > 0 {
			return mergeMetricsByTimestamp(observations), nil
		}
	}
	return nil, nil
}
