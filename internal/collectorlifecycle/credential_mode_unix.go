//go:build !windows

package collectorlifecycle

import "os"

func credentialFileModePrivate(info os.FileInfo) bool {
	// The typed collector credential is root-owned 0640 with read access only
	// for its dedicated service group. Group write/execute and all other access
	// are forbidden; 0600 remains valid for legacy/root-only installs.
	return info.Mode().Perm()&0037 == 0
}
