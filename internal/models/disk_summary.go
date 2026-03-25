package models

import "strings"

// SummaryDisk selects the canonical summary disk for host-level metrics.
// Prefer the root-mounted filesystem when present; otherwise fall back to the
// first disk with non-zero capacity in the existing collection order.
func SummaryDisk(disks []Disk) (Disk, bool) {
	for _, disk := range disks {
		if disk.Total <= 0 {
			continue
		}
		if strings.TrimSpace(disk.Mountpoint) == "/" {
			return disk, true
		}
	}

	for _, disk := range disks {
		if disk.Total > 0 {
			return disk, true
		}
	}

	return Disk{}, false
}
