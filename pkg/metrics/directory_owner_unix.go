//go:build unix

package metrics

import (
	"os"
	"syscall"
)

func directoryOwnedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && uint64(stat.Uid) == uint64(os.Geteuid())
}
