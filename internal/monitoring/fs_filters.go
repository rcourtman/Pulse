package monitoring

import "github.com/rcourtman/pulse-go-rewrite/pkg/fsfilters"

// readOnlyFilesystemReason is a compatibility wrapper around the shared filter.
// Use fsfilters.ReadOnlyFilesystemReason directly in new code.
func readOnlyFilesystemReason(fsType string, totalBytes, usedBytes uint64) (string, bool) {
	return fsfilters.ReadOnlyFilesystemReason(fsType, totalBytes, usedBytes)
}

// shouldIgnoreReadOnlyFilesystem is a compatibility wrapper around the shared filter.
// Use fsfilters.ShouldIgnoreReadOnlyFilesystem directly in new code.
func shouldIgnoreReadOnlyFilesystem(fsType string, totalBytes, usedBytes uint64) bool {
	return fsfilters.ShouldIgnoreReadOnlyFilesystem(fsType, totalBytes, usedBytes)
}
