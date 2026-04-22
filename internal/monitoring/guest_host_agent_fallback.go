package monitoring

import "github.com/rcourtman/pulse-go-rewrite/internal/models"

func resolveGuestDiskFromLinkedHostAgent(guestID string, vmIDToHostAgent map[string]models.Host) (models.Disk, []models.Disk, bool) {
	if guestID == "" || len(vmIDToHostAgent) == 0 {
		return models.Disk{}, nil, false
	}

	host, ok := vmIDToHostAgent[guestID]
	if !ok {
		return models.Disk{}, nil, false
	}

	summary, ok := models.AggregateDisk(host.Disks)
	if !ok {
		return models.Disk{}, nil, false
	}

	disks := append([]models.Disk(nil), host.Disks...)
	return models.Disk{
		Total:      summary.Total,
		Used:       summary.Used,
		Free:       summary.Free,
		Usage:      summary.Usage,
		Mountpoint: summary.Mountpoint,
		Type:       summary.Type,
		Device:     summary.Device,
	}, disks, true
}

func preferLinkedHostAgentDiskInventory(
	guestID string,
	vmIDToHostAgent map[string]models.Host,
	diskTotal uint64,
	diskUsed uint64,
	diskFree uint64,
	diskUsage float64,
	individualDisks []models.Disk,
	diskStatusReason string,
) (uint64, uint64, uint64, float64, []models.Disk, string, bool) {
	summary, hostDisks, ok := resolveGuestDiskFromLinkedHostAgent(guestID, vmIDToHostAgent)
	if !ok || summary.Total <= 0 {
		return diskTotal, diskUsed, diskFree, diskUsage, individualDisks, diskStatusReason, false
	}

	return uint64(summary.Total), uint64(summary.Used), uint64(summary.Free), summary.Usage, hostDisks, "", true
}
