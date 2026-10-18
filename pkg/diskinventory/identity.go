package diskinventory

import (
	"fmt"
	"strings"
)

// DeviceToken returns the kernel block-device token from either a canonical
// /dev path or a legacy smartctl display label such as "sda [scsi]".
func DeviceToken(device string) string {
	device = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(device), "/dev/"))
	if fields := strings.Fields(device); len(fields) > 0 {
		device = fields[0]
	}
	return strings.TrimSpace(device)
}

// PreferredID selects stable hardware identity first and scopes topology
// fallbacks to the reporting host. Controller and target are included only
// when present, preserving legacy direct-device IDs for SATA/NVMe disks.
func PreferredID(serial, wwn, scope, device, controller, target string) string {
	if serial = strings.TrimSpace(serial); IsUsableHardwareID(serial) {
		return serial
	}
	if wwn = strings.TrimSpace(wwn); IsUsableHardwareID(wwn) {
		return wwn
	}

	scope = strings.TrimSpace(scope)
	device = DeviceToken(device)
	controller = strings.TrimSpace(controller)
	target = strings.TrimSpace(target)
	if scope == "" || device == "" {
		return ""
	}
	if !IsControllerMemberTarget(target) {
		return fmt.Sprintf("%s:%s", scope, device)
	}
	return fmt.Sprintf("%s:%s@%s/%s", scope, device, controller, target)
}

// IsUsableHardwareID rejects controller placeholders that are not unique disk
// identities. Treating these as real serials collapses different disks and
// sends their SMART history to the same metric key.
func IsUsableHardwareID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	upper := strings.ToUpper(value)
	switch upper {
	case "UNKNOWN", "N/A", "NA", "NONE", "NULL", "DEFAULT", "DEFAULT-SERIAL",
		"TO BE FILLED BY O.E.M.", "0123456789":
		return false
	}
	if isQEMUDefaultDiskSerial(upper) {
		return false
	}
	// A WWN framing prefix must not hide an all-zero value: udev reports
	// ID_WWN=0x0000000000000000 for some bridges, and the 0x would otherwise
	// keep it from counting as all zeros.
	digits := upper
	for {
		trimmed := digits
		for _, prefix := range []string{"NAA.", "EUI.", "WWN-", "0X"} {
			trimmed = strings.TrimPrefix(trimmed, prefix)
		}
		if trimmed == digits {
			break
		}
		digits = trimmed
	}
	compact := strings.NewReplacer("-", "", ":", "", ".", "", " ", "").Replace(digits)
	if compact == "" {
		return false
	}
	allZero := true
	allF := true
	for _, char := range compact {
		allZero = allZero && char == '0'
		allF = allF && char == 'F'
	}
	return !allZero && !allF
}

// isQEMUDefaultDiskSerial recognizes the serial QEMU reports for a virtual
// disk configured without one. Every VM built the same way reports the same
// value, so it names a slot, not a disk: each node of a nested Proxmox cluster
// lists its first disk with serial drive-scsi0.
//
//   - A SCSI disk falls back to its drive ID: "drive-scsi0" under Proxmox,
//     "drive-scsi0-0-0-0" under libvirt, and QEMU's automatic ID for a -drive
//     given none, "scsi0-hd0" for if=scsi or "none0" for if=none.
//   - An ATA disk takes a per-VM counter, "QM00001".
//
// The grammar is kept to those exact shapes so a real serial that merely
// starts with "DRIVE-" or "QM" stays usable. The argument must already be
// upper-cased.
func isQEMUDefaultDiskSerial(upper string) bool {
	if counter, ok := strings.CutPrefix(upper, "QM"); ok {
		return len(counter) == 5 && isASCIIDigits(counter)
	}
	if address, ok := strings.CutPrefix(upper, "DRIVE-SCSI"); ok {
		groups := strings.Split(address, "-")
		if len(groups) > 4 {
			return false
		}
		for _, group := range groups {
			if !isASCIIDigits(group) {
				return false
			}
		}
		return true
	}
	if bus, unit, ok := strings.Cut(upper, "-HD"); ok {
		index, isSCSI := strings.CutPrefix(bus, "SCSI")
		return isSCSI && isASCIIDigits(index) && isASCIIDigits(unit)
	}
	if unit, ok := strings.CutPrefix(upper, "NONE"); ok {
		return isASCIIDigits(unit)
	}
	return false
}

func isASCIIDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// normalizeHardwareID canonicalizes a serial or WWN for cross-source
// comparison. Reporters disagree on framing, not identity: smartctl emits
// naa./eui.-prefixed WWNs, udev emits wwn-0x tokens, and PVE surfaces bare
// hex. Values are only ever prefix-stripped and case-folded, never truncated:
// sibling volumes on one RAID controller share their leading WWN bytes, so a
// truncated form must stay unequal to the full identifier.
func normalizeHardwareID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if !IsUsableHardwareID(value) {
		return ""
	}
	for {
		trimmed := value
		for _, prefix := range []string{"naa.", "eui.", "wwn-", "0x"} {
			trimmed = strings.TrimPrefix(trimmed, prefix)
		}
		if trimmed == value {
			break
		}
		value = trimmed
	}
	if !IsUsableHardwareID(value) {
		return ""
	}
	return value
}

// normalizeHardwareWWN is normalizeHardwareID for a value reported as a WWN.
// The host agent also spells smartctl's NAA 5 WWN as unpadded naa-oui-id hex
// fields, which only a WWN field can carry, so only WWNs are re-padded.
func normalizeHardwareWWN(value string) string {
	value = normalizeHardwareID(value)
	if expanded, ok := expandNAA5FieldWWN(value); ok {
		return expanded
	}
	return value
}

// expandNAA5FieldWWN rewrites the host agent's "5-c50-a1b2c3d4" spelling of a
// NAA 5 WWN (4-bit NAA, 24-bit OUI, 36-bit vendor ID, each printed as unpadded
// hex) into the 16-digit form udev and PVE report, "5000c500a1b2c3d4".
// Anything that does not fit those field widths is left alone.
func expandNAA5FieldWWN(value string) (string, bool) {
	fields := strings.Split(value, "-")
	if len(fields) != 3 || fields[0] != "5" ||
		len(fields[1]) > 6 || len(fields[2]) > 9 ||
		!isLowerHex(fields[1]) || !isLowerHex(fields[2]) {
		return "", false
	}
	return "5" + strings.Repeat("0", 6-len(fields[1])) + fields[1] +
		strings.Repeat("0", 9-len(fields[2])) + fields[2], true
}

func isLowerHex(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

// HardwareIdentityMatch reports whether two disk observations carry the same
// stable hardware identity. Serial and WWN are folded together because
// sources disagree on which field holds the durable identifier: PVE reports a
// RAID array volume's NAA identifier as its serial while smartctl reports the
// same value as a naa.-prefixed WWN with no serial at all.
func HardwareIdentityMatch(leftSerial, leftWWN, rightSerial, rightWWN string) bool {
	left := [2]string{normalizeHardwareID(leftSerial), normalizeHardwareWWN(leftWWN)}
	right := [2]string{normalizeHardwareID(rightSerial), normalizeHardwareWWN(rightWWN)}
	for _, l := range left {
		if l == "" {
			continue
		}
		for _, r := range right {
			if r != "" && l == r {
				return true
			}
		}
	}
	return false
}

// IsControllerMemberTarget reports whether target addresses one member behind
// a shared controller block path. Controller grammars vary after the numeric
// member prefix (for example megaraid,7, areca,1/1, and sssraid,0,1).
func IsControllerMemberTarget(target string) bool {
	target = strings.TrimSpace(target)
	index := strings.IndexByte(target, ',')
	if index < 0 || index+1 >= len(target) {
		return false
	}
	next := target[index+1]
	return next >= '0' && next <= '9'
}
