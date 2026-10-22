package monitoring

import (
	"math"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func clampToInt64(value uint64) int64 {
	if value > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(value)
}

// LXC listings sometimes omit counters. Capacity/config alone is not a measured
// empty root filesystem. Keep the existing unknown-usage sentinel through live
// state, filesystem seeding, alerts and History on both collection routes.
func containerDiskUsage(total, used uint64, presence proxmox.DiskUsagePresence) models.Disk {
	disk := models.Disk{Total: clampToInt64(total), Used: clampToInt64(used), Usage: -1}
	if disk.Total > 0 && disk.Used > disk.Total {
		disk.Used = disk.Total // Preserve existing LXC overcommit accounting.
	}
	if presence.Known() && disk.Total > 0 {
		disk.Free = maxInt64(0, disk.Total-disk.Used)
		disk.Usage = safePercentage(float64(disk.Used), float64(disk.Total))
	}
	return disk
}
