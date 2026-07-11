package unifiedresources

import "strings"

const hostStorageCleanupTargetPath = "/var/cache/apt/archives"

// HostStorageCleanupTargetDisk resolves the actual filesystem containing the
// fixed package-cache target. Longest-prefix selection is required when /var
// is a separate mount; root filesystem pressure must not authorize cleanup on
// a healthy /var filesystem.
func HostStorageCleanupTargetDisk(disks []DiskInfo) (DiskInfo, bool) {
	selectedLength := -1
	var selected DiskInfo
	for _, disk := range disks {
		mountpoint := strings.TrimSpace(disk.Mountpoint)
		if !hostStorageCleanupMountContainsPath(mountpoint) {
			continue
		}
		if len(mountpoint) > selectedLength {
			selected = disk
			selectedLength = len(mountpoint)
		}
	}
	return selected, selectedLength >= 0
}

func HostStorageCleanupPressureDisk(disks []DiskInfo) (DiskInfo, bool) {
	disk, ok := HostStorageCleanupTargetDisk(disks)
	return disk, ok && disk.Usage >= HostStoragePressureThreshold
}

func hostStorageCleanupMountContainsPath(mountpoint string) bool {
	mountpoint = strings.TrimSpace(mountpoint)
	if mountpoint == "" {
		return false
	}
	if mountpoint == "/" {
		return true
	}
	mountpoint = strings.TrimRight(mountpoint, "/")
	return mountpoint != "" && (hostStorageCleanupTargetPath == mountpoint || strings.HasPrefix(hostStorageCleanupTargetPath, mountpoint+"/"))
}
