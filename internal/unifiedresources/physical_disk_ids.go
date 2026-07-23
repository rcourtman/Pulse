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
	if serial = strings.TrimSpace(serial); diskinventory.IsUsableHardwareID(serial) {
		return serial
	}
	if wwn = strings.TrimSpace(wwn); diskinventory.IsUsableHardwareID(wwn) {
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

// ProxmoxPhysicalDiskSourceID returns the source-native ID used for physical
// disks produced by the Proxmox monitor. Direct SATA/NVMe/SAS devices retain
// the historical path-shaped ID. Controller members that share one kernel
// device include their member target so they cannot overwrite each other.
func ProxmoxPhysicalDiskSourceID(instance, node, device, controller, target string) string {
	legacy := fmt.Sprintf(
		"%s-%s-%s",
		strings.TrimSpace(instance),
		strings.TrimSpace(node),
		strings.ReplaceAll(strings.TrimSpace(device), "/", "-"),
	)
	if !diskinventory.IsControllerMemberTarget(target) {
		return legacy
	}
	if topologyID := diskinventory.PreferredID("", "", legacy, device, controller, target); topologyID != "" {
		return topologyID
	}
	return legacy
}

func PhysicalDiskMetricID(disk models.PhysicalDisk) string {
	if diskinventory.IsUsableHardwareID(disk.Serial) || diskinventory.IsUsableHardwareID(disk.WWN) {
		return PreferredPhysicalDiskMetricID(disk.Serial, disk.WWN, "")
	}
	fallback := strings.TrimSpace(disk.ID)
	canonicalFallback := ProxmoxPhysicalDiskSourceID(
		disk.Instance,
		disk.Node,
		disk.DevPath,
		disk.Controller,
		disk.Target,
	)
	if fallback == "" && strings.TrimSpace(disk.DevPath) != "" {
		fallback = canonicalFallback
	}
	if diskinventory.IsControllerMemberTarget(disk.Target) {
		if fallback == canonicalFallback {
			return fallback
		}
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
	if serial := strings.TrimSpace(disk.Serial); diskinventory.IsUsableHardwareID(serial) {
		return serial
	}
	if wwn := strings.TrimSpace(disk.WWN); diskinventory.IsUsableHardwareID(wwn) {
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
