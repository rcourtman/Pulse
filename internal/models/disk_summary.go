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

// AggregateDisk rolls a disk collection into one capacity summary.
// This is used when the canonical source of truth is a full disk inventory
// rather than one distinguished root filesystem.
func AggregateDisk(disks []Disk) (Disk, bool) {
	var total int64
	var used int64
	found := false

	for _, disk := range disks {
		if disk.Total <= 0 {
			continue
		}
		found = true
		total += disk.Total
		if disk.Used > 0 {
			used += disk.Used
		}
	}

	if !found || total <= 0 {
		return Disk{}, false
	}

	if used > total {
		used = total
	}

	usage := 0.0
	if total > 0 {
		usage = (float64(used) / float64(total)) * 100
	}

	return Disk{
		Total: total,
		Used:  used,
		Free:  total - used,
		Usage: usage,
	}, true
}
