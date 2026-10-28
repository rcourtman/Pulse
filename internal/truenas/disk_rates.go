package truenas

import (
	"math"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Keep each selected disk's valid samples even when another selected graph is
// missing. Host totals still require every selected member at the same instant.
// An unscoped compatibility graph cannot identify any physical disk.
func reportingDiskRates(devices map[string][]TimeSeriesPoint) map[string][]TimeSeriesPoint {
	rates := make(map[string][]TimeSeriesPoint)
	for key, points := range devices {
		identifier := strings.TrimPrefix(key, "disk\x00")
		if identifier != "" && len(points) > 0 {
			rates[identifier] = cloneTimeSeriesPoints(points)
		}
	}
	return rates
}

func latestDeviceRates(series map[string][]TimeSeriesPoint) map[string]TimeSeriesPoint {
	rates := make(map[string]TimeSeriesPoint, len(series))
	for device, points := range series {
		var latest TimeSeriesPoint
		for _, point := range points {
			if point.Timestamp.After(latest.Timestamp) && point.Value >= 0 && !math.IsNaN(point.Value) && !math.IsInf(point.Value, 0) {
				latest = point
			}
		}
		if !latest.Timestamp.IsZero() {
			rates[device] = latest
		}
	}
	return rates
}

func cloneDeviceRates(rates map[string]TimeSeriesPoint) map[string]TimeSeriesPoint {
	if rates == nil {
		return nil
	}
	cloned := make(map[string]TimeSeriesPoint, len(rates))
	for device, point := range rates {
		cloned[device] = point
	}
	return cloned
}

func metricsFromTrueNASDisk(system SystemInfo, disk Disk) *unifiedresources.ResourceMetrics {
	// CORE's reporting catalogue keys disks by the native device name. Do not
	// guess with serials, pool names, disk IDs, suffixes or a different connection.
	device := strings.TrimSpace(disk.Name)
	if device == "" {
		return nil
	}
	metric := func(rates map[string]TimeSeriesPoint) *unifiedresources.MetricValue {
		point, ok := rates[device]
		if !ok || point.Timestamp.IsZero() || point.Value < 0 || math.IsNaN(point.Value) || math.IsInf(point.Value, 0) {
			return nil
		}
		return &unifiedresources.MetricValue{Value: point.Value, Unit: "bytes/s", Source: unifiedresources.SourceTrueNAS, ObservedAt: point.Timestamp}
	}
	read, write := metric(system.DiskReadRates), metric(system.DiskWriteRates)
	if read == nil && write == nil {
		return nil
	}
	return &unifiedresources.ResourceMetrics{DiskRead: read, DiskWrite: write}
}
