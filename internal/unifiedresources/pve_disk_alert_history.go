package unifiedresources

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
)

// PVE disk health and wearout alerts reference the disk by instance, node and
// device path (ProxmoxPhysicalDiskAlertResourceID). A device path is not
// hardware identity: a replacement disk in the same slot, or a reboot that
// reorders devices, takes it over, and the alerts are evaluated before their
// poll's disks reach the registry. History therefore never binds the
// reference. Each lifecycle row is owned instead by the disk its own recorded
// hardware identity names, and reads by the reference find those rows by alert
// identifier.
const (
	// MetadataDiskSerial and MetadataDiskWWN carry the evaluated disk's
	// hardware identity on PVE disk alerts and their lifecycle rows.
	MetadataDiskSerial = "disk_serial"
	MetadataDiskWWN    = "disk_wwn"
)

// proxmoxDiskAlertSpecSuffixes name the PVE disk health and wearout alert
// specs. The alerts manager identifies each alert as
// "<reference>::<reference><suffix>" (buildCanonicalStateID over the spec ID).
var proxmoxDiskAlertSpecSuffixes = []string{"-health", "-wearout"}

// proxmoxDiskTemperatureAlertSpecID is the PVE disk temperature alert's spec,
// a metric threshold spec (canonicalMetricSpecID over "diskTemperature"), so
// its identifier is "<reference>::metric-threshold:diskTemperature".
const proxmoxDiskTemperatureAlertSpecID = "metric-threshold:diskTemperature"

// ProxmoxPhysicalDiskAlertIdentifiers lists the alert identifiers of the PVE
// disk alerts raised under ref, or nil when ref is not shaped like a PVE disk
// alert reference.
func ProxmoxPhysicalDiskAlertIdentifiers(ref string) []string {
	if !isProxmoxPhysicalDiskAlertReference(ref) {
		return nil
	}
	identifiers := make([]string, 0, len(proxmoxDiskAlertSpecSuffixes)+1)
	for _, suffix := range proxmoxDiskAlertSpecSuffixes {
		identifiers = append(identifiers, ref+"::"+ref+suffix)
	}
	return append(identifiers, ref+"::"+proxmoxDiskTemperatureAlertSpecID)
}

// isProxmoxPhysicalDiskAlertReference matches "<instance>:<node>:disk:<key>"
// with a non-empty device key (physicalDiskAlertKey). Instance names are free
// text; node names and device keys never contain a colon.
func isProxmoxPhysicalDiskAlertReference(ref string) bool {
	scope, key, found := cutLast(ref, ":disk:")
	if !found || key == "" {
		return false
	}
	instance, node, found := cutLast(scope, ":")
	if !found || strings.TrimSpace(instance) == "" || node == "" || strings.TrimSpace(node) != node {
		return false
	}
	for _, r := range key {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '.' && r != '-' {
			return false
		}
	}
	return true
}

func cutLast(value, sep string) (before, after string, found bool) {
	i := strings.LastIndex(value, sep)
	if i < 0 {
		return value, "", false
	}
	return value[:i], value[i+len(sep):], true
}

// proxmoxDiskAlertRowReference reports the PVE disk alert reference of an
// alert lifecycle row journaled under it: the row's resource ID is the
// reference and its alert identifier is one of that reference's alerts.
func proxmoxDiskAlertRowReference(change ResourceChange) (string, bool) {
	if !strings.HasPrefix(string(change.Kind), "alert_") {
		return "", false
	}
	ref := CanonicalResourceID(change.ResourceID)
	identifier, _ := change.Metadata[MetadataAlertIdentifier].(string)
	for _, candidate := range ProxmoxPhysicalDiskAlertIdentifiers(ref) {
		if identifier == candidate {
			return ref, true
		}
	}
	return "", false
}

// OwnedAlertReference returns the alert's own resource reference recorded on
// a lifecycle row that hardware ownership wrote under another resource, or
// "" for every other row. The row's alert identifier must be one of that
// reference's alerts.
func OwnedAlertReference(change ResourceChange) string {
	ref, _ := change.Metadata[MetadataAlertResourceID].(string)
	if ref == "" || ref == CanonicalResourceID(change.ResourceID) {
		return ""
	}
	identifier, _ := change.Metadata[MetadataAlertIdentifier].(string)
	for _, candidate := range ProxmoxPhysicalDiskAlertIdentifiers(ref) {
		if identifier == candidate {
			return ref
		}
	}
	return ""
}

// proxmoxDiskAlertOwner names the physical disk that owns one PVE disk alert
// lifecycle row, from the hardware identity the row records, or "" to leave
// the row under its reference. The registry only translates that identity to
// a canonical ID; it never supplies identity from the path, so a generation
// that still places another disk at the path cannot claim the row.
//
//   - A usable serial or WWN names the one disk the registry knows by the
//     same hardware identity (diskinventory.HardwareIdentityMatch), wherever
//     it sits now. A differing WWN does not rule a disk out: the registry
//     keeps a merged agent observation's WWN, which smartctl frames
//     differently from PVE, and merges disks that report the same serial.
//     Of two matching disks, a WWN only one of them shares with the row
//     decides; otherwise they are ambiguous. With none in inventory (a
//     new disk before its first poll reaches the registry, or a removed one),
//     the row goes to the canonical ID the registry mints for that identity,
//     unless another resource already holds that ID.
//   - A row without usable identity names a disk only by its path: the one
//     disk at the path that reports no usable identity either.
func (rr *ResourceRegistry) proxmoxDiskAlertOwner(ref string, metadata map[string]any) string {
	if rr == nil {
		return ""
	}
	serial, _ := metadata[MetadataDiskSerial].(string)
	wwn, _ := metadata[MetadataDiskWWN].(string)
	serial, wwn = strings.TrimSpace(serial), strings.TrimSpace(wwn)
	machineID := ""
	if diskinventory.IsUsableHardwareID(serial) {
		machineID = serial
	} else if diskinventory.IsUsableHardwareID(wwn) {
		machineID = wwn
	}

	rr.mu.RLock()
	defer rr.mu.RUnlock()
	var matches, sameWWN []string
	for id, resource := range rr.resources {
		if resource == nil || CanonicalResourceType(resource.Type) != ResourceTypePhysicalDisk || resource.PhysicalDisk == nil {
			continue
		}
		disk := resource.PhysicalDisk
		if machineID != "" {
			if !diskinventory.HardwareIdentityMatch(serial, wwn, disk.Serial, disk.WWN) {
				continue
			}
			if diskinventory.HardwareIdentityMatch("", wwn, "", disk.WWN) {
				sameWWN = append(sameWWN, id)
			}
		} else if resource.Proxmox == nil || strings.TrimSpace(disk.DevPath) == "" ||
			ProxmoxPhysicalDiskAlertResourceID(resource.Proxmox.Instance, resource.Proxmox.NodeName, disk.DevPath) != ref ||
			diskinventory.IsUsableHardwareID(disk.Serial) || diskinventory.IsUsableHardwareID(disk.WWN) {
			continue
		}
		matches = append(matches, id)
	}
	switch {
	case len(matches) == 1:
		return matches[0]
	case len(matches) > 1 && len(sameWWN) == 1:
		return sameWWN[0]
	case len(matches) > 1 || machineID == "":
		return ""
	}
	derived := MachineIdentityCanonicalID(ResourceTypePhysicalDisk, machineID)
	if rr.resources[derived] != nil {
		return ""
	}
	return derived
}
