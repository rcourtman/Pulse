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

	summary, ok := models.SummaryDisk(host.Disks)
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
