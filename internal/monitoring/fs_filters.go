package monitoring

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

// readOnlyFilesystemReason returns a label explaining why a filesystem should be
// ignored for usage calculations, along with a boolean indicating whether it is
// a read-only filesystem that always reports full usage. This helps us avoid
// false positives from immutable root images (overlay/squashfs/erofs) that ship
// with Home Assistant OS and similar appliances (see issue #505).
func readOnlyFilesystemReason(fsType string, totalBytes, usedBytes uint64) (string, bool) {
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

// shouldIgnoreReadOnlyFilesystem reports whether the filesystem should be
// skipped from usage aggregation.
func shouldIgnoreReadOnlyFilesystem(fsType string, totalBytes, usedBytes uint64) bool {
	_, skip := readOnlyFilesystemReason(fsType, totalBytes, usedBytes)
	return skip
}
