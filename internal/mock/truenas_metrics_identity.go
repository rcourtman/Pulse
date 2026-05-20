package mock

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
)

func TrueNASPoolMetricID(hostname string, pool string) string {
	return TrueNASScopedMetricID(hostname, "pool:"+strings.TrimSpace(pool))
}

func TrueNASDatasetMetricID(hostname string, dataset string) string {
	return TrueNASScopedMetricID(hostname, "dataset:"+strings.TrimSpace(dataset))
}

func TrueNASAppMetricID(hostname string, app truenas.App) string {
	appID := strings.TrimSpace(app.ID)
	if appID == "" {
		appID = strings.TrimSpace(app.Name)
	}
	if appID == "" {
		return ""
	}
	return TrueNASScopedMetricID(hostname, "app:"+appID)
}

func TrueNASScopedMetricID(hostname string, sourceID string) string {
	hostname = strings.TrimSpace(hostname)
	sourceID = strings.TrimSpace(sourceID)
	if hostname == "" {
		return sourceID
	}
	if sourceID == "" {
		return "system:" + hostname
	}
	return "system:" + hostname + "/" + sourceID
}

func trueNASDiskMetricsResourceID(disk truenas.Disk) string {
	resourceID := strings.TrimSpace(disk.Serial)
	if resourceID == "" {
		resourceID = strings.TrimSpace(disk.ID)
	}
	if resourceID == "" {
		resourceID = strings.TrimSpace(disk.Name)
	}
	return resourceID
}
