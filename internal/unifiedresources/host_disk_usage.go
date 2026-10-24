package unifiedresources

import "github.com/rcourtman/pulse-go-rewrite/internal/models"

// HostDiskUsagePercent uses the same first-filesystem admission as the
// canonical host metric. Missing or unavailable inventory is not measured zero.
func HostDiskUsagePercent(disks []models.Disk) (float64, bool) {
	if len(disks) == 0 || !hasObservedDiskUsage(disks[0]) {
		return 0, false
	}
	return disks[0].Usage, true
}

// DiskUsagePercent preserves canonical metric presence, including provider
// readings without agent filesystem inventory. Reporting freshness is separate.
func (v HostView) DiskUsagePercent() (float64, bool) {
	if v.r == nil || v.r.Metrics == nil || v.r.Metrics.Disk == nil {
		return 0, false
	}
	return v.r.Metrics.Disk.Percent, true
}
