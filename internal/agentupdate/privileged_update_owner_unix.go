//go:build unix

package agentupdate

import (
	"os"
	"syscall"
)

func collectorQuarantineOwnedByCurrentUID(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid())
}
