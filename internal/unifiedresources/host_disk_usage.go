package unifiedresources

import "github.com/rcourtman/pulse-go-rewrite/internal/models"

// HostDiskUsagePercent uses the same first-filesystem admission as the
// canonical host metric. Missing or unavailable inventory is not measured zero.
func HostDiskUsagePercent(disks []models.Disk) (float64, bool) {
	if len(disks) == 0 {
		return 0, false
	}
	return DiskUsagePercent(disks[0])
}

// DiskUsagePercent applies canonical observation admission to one filesystem.
// A listed filesystem can still have an unavailable or contradictory reading.
func DiskUsagePercent(disk models.Disk) (float64, bool) {
	if !hasObservedDiskUsage(disk) {
		return 0, false
	}
	return disk.Usage, true
}

// DiskUsagePercent preserves canonical metric presence, including provider
// readings without agent filesystem inventory. Reporting freshness is separate.
func (v HostView) DiskUsagePercent() (float64, bool) {
	if v.r == nil || v.r.Metrics == nil || v.r.Metrics.Disk == nil {
		return 0, false
	}
	return v.r.Metrics.Disk.Percent, true
}
