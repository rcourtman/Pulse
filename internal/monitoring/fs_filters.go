package monitoring

import "strings"

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
	if strings.Contains(ft, "erofs") {
		return "erofs", true
	}
	if strings.Contains(ft, "squashfs") || strings.Contains(ft, "squash-fs") {
		return "squashfs", true
	}
	if strings.Contains(ft, "iso9660") {
		return "iso9660", true
	}
	if strings.Contains(ft, "udf") {
		return "udf", true
	}
	if strings.Contains(ft, "cramfs") {
		return "cramfs", true
	}
	if strings.Contains(ft, "romfs") {
		return "romfs", true
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
