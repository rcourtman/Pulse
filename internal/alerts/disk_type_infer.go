package alerts

import "strings"

// inferDiskHardwareType infers disk hardware type from device path.
// Returns "nvme" for NVMe block devices (matched case-insensitively),
// "" (unknown) for anything else. Other types (sata/hdd) cannot be
// reliably inferred from device paths today.
func inferDiskHardwareType(device string) string {
	if strings.Contains(strings.ToLower(device), "nvme") {
		return "nvme"
	}
	return ""
}
