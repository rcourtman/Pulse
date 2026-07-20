package unifiedresources

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
)

// PreferredPhysicalDiskMetricID returns the canonical history key used for
// physical-disk metrics across sources. Stable hardware identity wins; a
// source-specific fallback is only used when the disk exposes no serial/WWN.
func PreferredPhysicalDiskMetricID(serial, wwn, fallback string) string {
	if serial = strings.TrimSpace(serial); serial != "" {
		return serial
	}
	if wwn = strings.TrimSpace(wwn); wwn != "" {
		return wwn
	}
	return strings.TrimSpace(fallback)
}

func HostSMARTDiskSourceID(host models.Host, disk models.HostDiskSMART) string {
	device := normalizePhysicalDiskDeviceToken(disk.Device)
	return diskinventory.PreferredID(
		disk.Serial,
		disk.WWN,
		strings.TrimSpace(host.ID),
		device,
		disk.Controller,
		disk.Target,
	)
}

func HostUnraidDiskSourceID(host models.Host, disk models.HostUnraidDisk) string {
	device := normalizePhysicalDiskDeviceToken(disk.Device)
	fallback := ""
	if device != "" {
		fallback = fmt.Sprintf("%s:%s", strings.TrimSpace(host.ID), device)
	} else if strings.TrimSpace(disk.Name) != "" {
		fallback = fmt.Sprintf("%s:unraid-slot:%s", strings.TrimSpace(host.ID), strings.TrimSpace(disk.Name))
	}
	return PreferredPhysicalDiskMetricID(disk.Serial, "", fallback)
}

func PhysicalDiskMetricID(disk models.PhysicalDisk) string {
	if strings.TrimSpace(disk.Serial) != "" || strings.TrimSpace(disk.WWN) != "" {
		return PreferredPhysicalDiskMetricID(disk.Serial, disk.WWN, "")
	}
	fallback := strings.TrimSpace(disk.ID)
	if fallback == "" && strings.TrimSpace(disk.DevPath) != "" {
		fallback = fmt.Sprintf(
			"%s-%s-%s",
			strings.TrimSpace(disk.Instance),
			strings.TrimSpace(disk.Node),
			strings.ReplaceAll(strings.TrimSpace(disk.DevPath), "/", "-"),
		)
	}
	if diskinventory.IsControllerMemberTarget(disk.Target) {
		return diskinventory.PreferredID(
			"",
			"",
			fallback,
			disk.DevPath,
			disk.Controller,
			disk.Target,
		)
	}
	return strings.TrimSpace(fallback)
}

func PhysicalDiskMetaMetricID(disk *PhysicalDiskMeta, fallback string) string {
	if disk == nil {
		return strings.TrimSpace(fallback)
	}
	if serial := strings.TrimSpace(disk.Serial); serial != "" {
		return serial
	}
	if wwn := strings.TrimSpace(disk.WWN); wwn != "" {
		return wwn
	}
	if diskinventory.IsControllerMemberTarget(disk.Target) && strings.TrimSpace(fallback) != "" {
		return diskinventory.PreferredID(
			"",
			"",
			strings.TrimSpace(fallback),
			disk.DevPath,
			disk.Controller,
			disk.Target,
		)
	}
	return PreferredPhysicalDiskMetricID(disk.Serial, disk.WWN, fallback)
}

func normalizePhysicalDiskDeviceToken(device string) string {
	return diskinventory.DeviceToken(device)
}
