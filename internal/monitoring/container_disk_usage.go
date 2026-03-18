package monitoring

import (
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
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
