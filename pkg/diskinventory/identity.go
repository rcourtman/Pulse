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
	compact := strings.NewReplacer("-", "", ":", "", ".", "", " ", "").Replace(upper)
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
