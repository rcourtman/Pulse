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

// HostSMARTDiskSourceID returns the registry source ID of a disk a host agent
// reports through SMART. A usable serial or WWN names the drive but not the
// machine: a dual-ported SAS shelf, cloned VMs with an explicit serial and
// fixed-serial USB bridges report one identifier on several hosts, and the
// registry keeps one resource per source ID. The key therefore carries the
// reporting host. A row without hardware identity keeps its historical
// host/device/topology key, which is also its metrics key unless the host's
// Unraid inventory reports a serial for the disk.
func HostSMARTDiskSourceID(host models.Host, disk models.HostDiskSMART) string {
	if hardwareID := PreferredPhysicalDiskMetricID(disk.Serial, disk.WWN, ""); hardwareID != "" {
		return hostPhysicalDiskSourceID(host.ID, hardwareID)
	}
	return hostSMARTDiskKey(host, disk, "", "")
}

// HostSMARTDiskMetricID returns the history key a host agent's SMART and disk
// I/O metrics are written under. Like every physical-disk source, it prefers
// the drive's serial or WWN, so a disk keeps one history across sources and
// hosts; see PreferredPhysicalDiskMetricID. The serial is the one the disk
// resource carries (hostSMARTDiskSerial), which its metrics target reads.
func HostSMARTDiskMetricID(host models.Host, disk models.HostDiskSMART) string {
	serial := hostSMARTDiskSerial(host, disk, matchUnraidDisk(host.Unraid, disk))
	return hostSMARTDiskKey(host, disk, serial, disk.WWN)
}

func hostSMARTDiskKey(host models.Host, disk models.HostDiskSMART, serial, wwn string) string {
	return diskinventory.PreferredID(
		serial,
		wwn,
		strings.TrimSpace(host.ID),
		normalizePhysicalDiskDeviceToken(disk.Device),
		disk.Controller,
		disk.Target,
	)
}

// hostSMARTDiskSerial returns the serial of the disk a SMART row describes: the
// row's own, or, when the row reports none, the one the host's Unraid
// inventory reports for the disk. smartctl reports no serial for a disk in
// standby, and the Unraid row still names it. A row without a serial matches
// its Unraid row by device path alone, so it takes no serial from a path
// several of the host's rows share: controller members behind one kernel block
// device, which the Unraid row describes as a whole.
func hostSMARTDiskSerial(host models.Host, disk models.HostDiskSMART, unraidDisk *models.HostUnraidDisk) string {
	serial := strings.TrimSpace(disk.Serial)
	if serial != "" || unraidDisk == nil || hostSMARTDevicePathShared(host, disk) {
		return serial
	}
	return strings.TrimSpace(unraidDisk.Serial)
}

// hostSMARTDevicePathShared reports whether more than one of the host's SMART
// rows, the row itself among them, names the row's device path.
func hostSMARTDevicePathShared(host models.Host, disk models.HostDiskSMART) bool {
	device := strings.ToLower(normalizePhysicalDiskDeviceToken(disk.Device))
	if device == "" {
		return false
	}
	rows := 0
	for _, other := range host.Sensors.SMART {
		if strings.ToLower(normalizePhysicalDiskDeviceToken(other.Device)) == device {
			rows++
		}
	}
	return rows > 1
}

// HostUnraidDeviceMetricID returns the history key of the disk a host's Unraid
// inventory reports at device: its usable serial, which the disk resource
// carries and its metrics target reads. It returns "" when the inventory has
// no row for the device, more than one, or a row without a usable serial.
func HostUnraidDeviceMetricID(host models.Host, device string) string {
	device = strings.ToLower(normalizePhysicalDiskDeviceToken(device))
	if host.Unraid == nil || device == "" {
		return ""
	}
	var matched *models.HostUnraidDisk
	for i := range host.Unraid.Disks {
		if strings.ToLower(normalizePhysicalDiskDeviceToken(host.Unraid.Disks[i].Device)) != device {
			continue
		}
		if matched != nil {
			return ""
		}
		matched = &host.Unraid.Disks[i]
	}
	if matched == nil {
		return ""
	}
	return PreferredPhysicalDiskMetricID(matched.Serial, "", "")
}

// HostUnraidDiskSourceID returns the registry source ID of a disk in a host's
// Unraid inventory. It shares HostSMARTDiskSourceID's host-scoped serial key so
// the SMART and Unraid observations of one disk keep one source mapping.
func HostUnraidDiskSourceID(host models.Host, disk models.HostUnraidDisk) string {
	if serial := PreferredPhysicalDiskMetricID(disk.Serial, "", ""); serial != "" {
		return hostPhysicalDiskSourceID(host.ID, serial)
	}
	device := normalizePhysicalDiskDeviceToken(disk.Device)
	if device != "" {
		return fmt.Sprintf("%s:%s", strings.TrimSpace(host.ID), device)
	}
	if name := strings.TrimSpace(disk.Name); name != "" {
		return fmt.Sprintf("%s:unraid-slot:%s", strings.TrimSpace(host.ID), name)
	}
	return ""
}

const hostPhysicalDiskSourceIDMarker = "/physical-disk:"

func hostPhysicalDiskSourceID(hostID, hardwareID string) string {
	if hostID = strings.TrimSpace(hostID); hostID == "" {
		return hardwareID
	}
	return hostID + hostPhysicalDiskSourceIDMarker + hardwareID
}

// sourceSpecificIDKey returns the part of a source ID that SourceSpecificID
// hashes. An agent disk's source ID carries its host, but it used to be the
// bare serial or WWN, and the ID of a disk split from its match, like the
// operator exclusion that split it, was derived from that. The host stays out
// of the hash so both keep applying across the change.
func sourceSpecificIDKey(resourceType ResourceType, source DataSource, sourceID string) string {
	sourceID = normalizeSourceID(sourceID)
	if source != SourceAgent || CanonicalResourceType(resourceType) != ResourceTypePhysicalDisk {
		return sourceID
	}
	if _, hardwareID, ok := strings.Cut(sourceID, hostPhysicalDiskSourceIDMarker); ok && hardwareID != "" {
		return hardwareID
	}
	return sourceID
}

// seedAgentPhysicalDiskSourceIDLocked rebuilds an agent disk's source ID for a
// registry seeded from unified resources, which carry no source mappings. It
// must reproduce the key HostSMARTDiskSourceID and HostUnraidDiskSourceID
// ingest under, or a rehydrated disk loses its agent source target, and with
// it the metrics target of a disk only the agent reports. When the reporting
// host cannot be found, a disk with hardware identity falls back to the bare
// serial or WWN its source ID used to be.
func (rr *ResourceRegistry) seedAgentPhysicalDiskSourceIDLocked(resource *Resource) string {
	disk := resource.PhysicalDisk
	if disk == nil {
		return ""
	}
	hostID := rr.seedAgentDiskHostIDLocked(resource)
	if hardwareID := PreferredPhysicalDiskMetricID(disk.Serial, disk.WWN, ""); hardwareID != "" {
		return hostPhysicalDiskSourceID(hostID, hardwareID)
	}
	if hostID == "" {
		return ""
	}
	return diskinventory.PreferredID("", "", hostID, disk.DevPath, disk.Controller, disk.Target)
}

// seedAgentDiskHostIDLocked returns the agent ID of the host an agent-reported
// disk sits on: its parent, or the host above the Unraid array or cache pool
// that parents it. A parent that is neither yields no host.
func (rr *ResourceRegistry) seedAgentDiskHostIDLocked(resource *Resource) string {
	parentID := resource.ParentID
	for depth := 0; parentID != nil && depth < 4; depth++ {
		parent := rr.resources[CanonicalResourceID(strings.TrimSpace(*parentID))]
		if parent == nil {
			return ""
		}
		if parent.Agent != nil {
			if agentID := strings.TrimSpace(parent.Agent.AgentID); agentID != "" {
				return agentID
			}
		}
		if CanonicalResourceType(parent.Type) != ResourceTypeStorage {
			return ""
		}
		parentID = parent.ParentID
	}
	return ""
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

// ProxmoxPhysicalDiskAlertResourceID is the persisted resource reference used
// by PVE disk-health and wearout alerts. Keep this distinct from the disk's
// source ID: existing alert occurrences use this shape, while the registry
// needs it as an alias to apply per-disk operator intent.
func ProxmoxPhysicalDiskAlertResourceID(instance, node, device string) string {
	return fmt.Sprintf("%s:%s:disk:%s", strings.TrimSpace(instance), strings.TrimSpace(node), physicalDiskAlertKey(device))
}

func physicalDiskAlertKey(device string) string {
	trimmed := strings.TrimSpace(device)
	// Persisted alert references used an empty key when the device path was
	// absent. Keep that spelling distinct from the root-device key.
	if trimmed == "" {
		return ""
	}
	if trimmed == "/" {
		return "root"
	}
	trimmed = strings.Trim(trimmed, "/\\ ")
	if trimmed == "" {
		trimmed = "root"
	}
	var builder strings.Builder
	previousDash := false
	for _, r := range strings.ToLower(trimmed) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.':
			builder.WriteRune(r)
			previousDash = false
		case !previousDash:
			builder.WriteByte('-')
			previousDash = true
		}
	}
	key := strings.Trim(builder.String(), "-.")
	if key == "" {
		return "disk"
	}
	return key
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
