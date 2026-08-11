//go:build !unix

package metrics

import "os"

// Platforms without Unix ownership metadata retain the existing chmod-based
// hardening. Filesystem-root and symlink targets are still rejected first.
func directoryOwnedByCurrentUser(_ os.FileInfo) bool {
	return true
}
