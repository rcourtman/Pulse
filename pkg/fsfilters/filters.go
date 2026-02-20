package fsfilters

import "strings"

var readOnlyFilesystemPatterns = []struct {
	reason     string
	substrings []string
}{
	{reason: "erofs", substrings: []string{"erofs"}},
	{reason: "squashfs", substrings: []string{"squashfs", "squash-fs"}},
	{reason: "iso9660", substrings: []string{"iso9660"}},
	{reason: "cdfs", substrings: []string{"cdfs"}},
	{reason: "udf", substrings: []string{"udf"}},
	{reason: "cramfs", substrings: []string{"cramfs"}},
	{reason: "romfs", substrings: []string{"romfs"}},
}

// ReadOnlyFilesystemReason returns a label explaining why a filesystem should be
// ignored for usage calculations, along with a boolean indicating whether it is
// a read-only filesystem that always reports full usage. This helps us avoid
// false positives from immutable root images (overlay/squashfs/erofs) that ship
// with Home Assistant OS and similar appliances, as well as snap mounts on Ubuntu
// systems (see issues #505, #690).
func ReadOnlyFilesystemReason(fsType string, totalBytes, usedBytes uint64) (string, bool) {
	ft := strings.ToLower(strings.TrimSpace(fsType))
	if ft == "" {
		return "", false
	}

	// Common read-only filesystem types used for immutable system partitions.
	for _, pattern := range readOnlyFilesystemPatterns {
		for _, needle := range pattern.substrings {
			if strings.Contains(ft, needle) {
				return pattern.reason, true
			}
		}
	}

	// Overlay-style filesystems can report 100% usage even though writes are
	// redirected elsewhere. Treat them as read-only when the reported usage is
	// saturated so we fall back to the writable layer metrics instead.
	if strings.Contains(ft, "overlay") || strings.Contains(ft, "overlayfs") {
		if totalBytes > 0 && usedBytes >= totalBytes {
			return "overlay", true
		}
	}

	return "", false
}

// ShouldIgnoreReadOnlyFilesystem reports whether the filesystem should be
// skipped from usage aggregation.
func ShouldIgnoreReadOnlyFilesystem(fsType string, totalBytes, usedBytes uint64) bool {
	_, skip := ReadOnlyFilesystemReason(fsType, totalBytes, usedBytes)
	return skip
}

// virtualFSTypes are filesystem types that represent virtual/pseudo filesystems
// which should not be counted toward disk usage.
var virtualFSTypes = map[string]bool{
	"tmpfs":      true,
	"devtmpfs":   true,
	"cgroup":     true,
	"cgroup2":    true,
	"sysfs":      true,
	"proc":       true,
	"devpts":     true,
	"securityfs": true,
	"debugfs":    true,
	"tracefs":    true,
	"fusectl":    true,
	"configfs":   true,
	"pstore":     true,
	"hugetlbfs":  true,
	"mqueue":     true,
	"bpf":        true,
	"overlay":    true, // Docker/container overlay filesystems (issue #942)
	"overlayfs":  true, // Alternative overlay name
	"autofs":     true, // Systemd automount placeholders (issue #942)
	"fdescfs":    true, // FreeBSD file descriptor filesystem (issue #1142)
	"devfs":      true, // FreeBSD device filesystem
	"linprocfs":  true, // FreeBSD Linux proc compatibility
	"linsysfs":   true, // FreeBSD Linux sys compatibility
}

// networkFSPatterns are substrings that indicate network/remote filesystems.
var networkFSPatterns = []string{"fuse", "9p", "nfs", "cifs", "smb"}

// specialMountPrefixes are mountpoint prefixes that indicate system mounts.
var specialMountPrefixes = []string{
	"/dev",
	"/proc",
	"/sys",
	"/run",
	"/var/run/", // FreeBSD (not a symlink to /run like on Linux)
	"/var/lib/containers",
	"/snap",
}

// containerOverlayPatterns detect container overlay filesystem paths from various
// container runtimes (Docker, Podman, LXC, EnhanceCP, etc.) that may not be in
// standard locations. These paths should be excluded from disk usage as they
// represent container layers, not actual storage usage.
var containerOverlayPatterns = []string{
	"/overlay2/",
	"/overlay/",
	"/diff/",
	"/merged",
}

// containerPathPrefixes detect container-related paths that should be excluded
// from disk usage, even if they don't contain "/containers/".
var containerPathPrefixes = []string{
	"/mnt/.ix-apps/docker/", // TrueNAS SCALE Docker overlay mounts
}

// ShouldSkipFilesystem determines if a filesystem should be excluded from disk
// usage aggregation. It checks for read-only filesystems, virtual/pseudo filesystems,
// network mounts, and special system mountpoints. Returns skip=true if the filesystem
// should be excluded, along with a list of reason strings.
func ShouldSkipFilesystem(fsType, mountpoint string, totalBytes, usedBytes uint64) (skip bool, reasons []string) {
	fsTypeLower := strings.ToLower(strings.TrimSpace(fsType))

	// Check read-only filesystems (existing logic)
	if reason, isReadOnly := ReadOnlyFilesystemReason(fsType, totalBytes, usedBytes); isReadOnly {
		reasons = append(reasons, "read-only-"+reason)
	}

	// Check virtual filesystem types
	if virtualFSTypes[fsTypeLower] {
		reasons = append(reasons, "special-fs-type")
	}

	// Check network filesystem patterns
	for _, pattern := range networkFSPatterns {
		if strings.Contains(fsTypeLower, pattern) {
			reasons = append(reasons, "special-fs-type")
			break
		}
	}

	// Check special mountpoint prefixes
	for _, prefix := range specialMountPrefixes {
		if strings.HasPrefix(mountpoint, prefix) {
			reasons = append(reasons, "special-mountpoint")
			break
		}
	}

	// Check specific special mountpoints
	if mountpoint == "/boot/efi" {
		reasons = append(reasons, "special-mountpoint")
	}

	// Windows System Reserved partition
	if mountpoint == "System Reserved" || strings.Contains(mountpoint, "System Reserved") {
		reasons = append(reasons, "special-mountpoint")
	}

	// Check for container overlay paths from various runtimes in non-standard locations
	// (e.g., /var/local/enhance/containers/*/overlay/merged). Related to #790.
	if strings.Contains(mountpoint, "/containers/") {
		for _, pattern := range containerOverlayPatterns {
			if strings.Contains(mountpoint, pattern) {
				reasons = append(reasons, "container-overlay")
				break
			}
		}
	}

	// Check for container paths that don't follow the /containers/ pattern
	// (e.g., TrueNAS SCALE uses /mnt/.ix-apps/docker/overlay2/...). Related to #718.
	for _, prefix := range containerPathPrefixes {
		if strings.HasPrefix(mountpoint, prefix) {
			reasons = append(reasons, "container-overlay")
			break
		}
	}

	// EnhanceCP uses /var/container_tmp/{uuid}/merged for container overlays.
	// Filter these as they're ephemeral container layers, not user storage. Related to #790.
	if strings.Contains(mountpoint, "/container_tmp/") {
		for _, pattern := range containerOverlayPatterns {
			if strings.Contains(mountpoint, pattern) {
				reasons = append(reasons, "container-overlay")
				break
			}
		}
	}

	return len(reasons) > 0, reasons
}

// MatchesUserExclude checks if a mountpoint matches any user-defined exclusion patterns.
// Patterns can be:
//   - Exact paths: "/mnt/backup" matches only "/mnt/backup"
//   - Prefix patterns (ending with *): "/mnt/ext*" matches "/mnt/external", "/mnt/ext-drive"
//   - Contains patterns (surrounded by *): "*pbs*" matches any path containing "pbs"
func MatchesUserExclude(mountpoint string, excludePatterns []string) bool {
	return matchesPattern(mountpoint, excludePatterns)
}

// MatchesDiskExclude checks if a disk (device or mountpoint) matches any exclusion pattern.
// This checks both the device path (e.g., /dev/sda) and the mountpoint (e.g., /mnt/backup)
// to allow users to exclude disks by either identifier.
func MatchesDiskExclude(device, mountpoint string, excludePatterns []string) bool {
	if len(excludePatterns) == 0 {
		return false
	}

	// Check against mountpoint
	if mountpoint != "" && matchesPattern(mountpoint, excludePatterns) {
		return true
	}

	// Check against device path
	if device != "" && matchesPattern(device, excludePatterns) {
		return true
	}

	// Check against device name (without /dev/ prefix)
	if device != "" {
		deviceName := device
		if strings.HasPrefix(device, "/dev/") {
			deviceName = device[5:]
		}
		if matchesPattern(deviceName, excludePatterns) {
			return true
		}
	}

	return false
}

// MatchesDeviceExclude checks if a device name/path matches exclusion patterns.
// For disk I/O collection where we only have device names (not mountpoints).
func MatchesDeviceExclude(device string, excludePatterns []string) bool {
	if len(excludePatterns) == 0 {
		return false
	}

	// Check against full device path
	if matchesPattern(device, excludePatterns) {
		return true
	}

	// Check with /dev/ prefix if not present
	if !strings.HasPrefix(device, "/dev/") {
		if matchesPattern("/dev/"+device, excludePatterns) {
			return true
		}
	}

	// Check device name without /dev/ prefix
	deviceName := device
	if strings.HasPrefix(device, "/dev/") {
		deviceName = device[5:]
	}
	if matchesPattern(deviceName, excludePatterns) {
		return true
	}

	return false
}

// matchesPattern checks if a value matches any of the exclusion patterns.
func matchesPattern(value string, excludePatterns []string) bool {
	if len(excludePatterns) == 0 {
		return false
	}

	for _, pattern := range excludePatterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		// Contains pattern: *substring*
		if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") && len(pattern) > 2 {
			substring := pattern[1 : len(pattern)-1]
			if strings.Contains(value, substring) {
				return true
			}
			continue
		}

		// Prefix pattern: /path/prefix*
		if strings.HasSuffix(pattern, "*") {
			prefix := pattern[:len(pattern)-1]
			if strings.HasPrefix(value, prefix) {
				return true
			}
			continue
		}

		// Exact match
		if value == pattern {
			return true
		}
	}

	return false
}
