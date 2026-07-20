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
	if serial = strings.TrimSpace(serial); serial != "" {
		return serial
	}
	if wwn = strings.TrimSpace(wwn); wwn != "" {
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

// IsControllerMemberTarget reports whether target addresses one member behind
// a shared controller block path (for example megaraid,7 or cciss,1).
func IsControllerMemberTarget(target string) bool {
	target = strings.TrimSpace(target)
	index := strings.IndexByte(target, ',')
	if index < 0 || index+1 >= len(target) {
		return false
	}
	for _, char := range target[index+1:] {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
