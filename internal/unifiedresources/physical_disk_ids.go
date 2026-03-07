package unifiedresources

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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
	device := strings.TrimSpace(strings.TrimPrefix(disk.Device, "/dev/"))
	fallback := ""
	if device != "" {
		fallback = fmt.Sprintf("%s:%s", strings.TrimSpace(host.ID), device)
	}
	return PreferredPhysicalDiskMetricID(disk.Serial, disk.WWN, fallback)
}

func PhysicalDiskMetricID(disk models.PhysicalDisk) string {
	fallback := strings.TrimSpace(disk.ID)
	if fallback == "" && strings.TrimSpace(disk.DevPath) != "" {
		fallback = fmt.Sprintf(
			"%s-%s-%s",
			strings.TrimSpace(disk.Instance),
			strings.TrimSpace(disk.Node),
			strings.ReplaceAll(strings.TrimSpace(disk.DevPath), "/", "-"),
		)
	}
	return PreferredPhysicalDiskMetricID(disk.Serial, disk.WWN, fallback)
}

func PhysicalDiskMetaMetricID(disk *PhysicalDiskMeta, fallback string) string {
	if disk == nil {
		return strings.TrimSpace(fallback)
	}
	return PreferredPhysicalDiskMetricID(disk.Serial, disk.WWN, fallback)
}
