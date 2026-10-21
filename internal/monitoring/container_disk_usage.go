package monitoring

import (
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type containerDiskOverride struct {
	Used  uint64
	Total uint64
}

func clampToInt64(value uint64) int64 {
	if value > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(value)
}

func storageSupportsContainerVolumes(content string) bool {
	if content == "" {
		return false
	}

	for _, part := range strings.Split(content, ",") {
		switch strings.TrimSpace(strings.ToLower(part)) {
		case "rootdir", "images", "subvol":
			return true
		}
	}

	return false
}

func isRootVolumeForContainer(volid string, vmid int) bool {
	if vmid <= 0 || volid == "" {
		return false
	}

	normalized := strings.ToLower(volid)
	if idx := strings.Index(normalized, "@"); idx != -1 {
		normalized = normalized[:idx]
	}

	vmIDString := strconv.Itoa(vmid)
	patterns := []string{
		"subvol-" + vmIDString + "-disk-0",
		"vm-" + vmIDString + "-disk-0",
	}

	for _, pattern := range patterns {
		if strings.Contains(normalized, pattern) {
			return true
		}
	}

	return false
}

func (m *Monitor) collectContainerRootUsage(ctx context.Context, client PVEClientInterface, node string, vmIDs []int) map[int]containerDiskOverride {
	overrides := make(map[int]containerDiskOverride)
	if len(vmIDs) == 0 {
		return overrides
	}

	vmidSet := make(map[int]struct{}, len(vmIDs))
	for _, vmID := range vmIDs {
		vmidSet[vmID] = struct{}{}
	}

	storages, err := client.GetStorage(ctx, node)
	if err != nil {
		log.Debug().
			Err(err).
			Str("node", node).
			Msg("Unable to list storages for container disk overrides")
		return overrides
	}

	for _, storage := range storages {
		if !storageSupportsContainerVolumes(storage.Content) {
			continue
		}
		if !storageContentQueryable(storage) {
			continue
		}

		contents, err := client.GetStorageContent(ctx, node, storage.Storage)
		if err != nil {
			log.Debug().
				Err(err).
				Str("node", node).
				Str("storage", storage.Storage).
				Msg("Container disk usage query failed; metrics for instance omitted this cycle")
			continue
		}

		for _, item := range contents {
			// The shared content reader also serves backups and snapshots.
			// Neither is an observation of a container's live root volume.
			if (item.Content != "rootdir" && item.Content != "images") || strings.Contains(item.Volid, "@") {
				continue
			}
			vmid := item.VMID
			if vmid == 0 {
				continue
			}
			if _, ok := vmidSet[vmid]; !ok {
				continue
			}
			if !isRootVolumeForContainer(item.Volid, vmid) {
				continue
			}
			if item.Used == 0 {
				continue
			}

			overrides[vmid] = containerDiskOverride{
				Used:  item.Used,
				Total: item.Size,
			}
		}
	}

	return overrides
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
