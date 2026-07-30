//go:build !windows

package hostagent

import (
	"fmt"
	"os"
	"syscall"
)

func validateCustomSensorFileOwner(info os.FileInfo, label string) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("%s ownership could not be verified", label)
	}
	effectiveUID := uint32(os.Geteuid())
	if stat.Uid != effectiveUID {
		return fmt.Errorf("%s must be owned by the agent service user (uid %d)", label, effectiveUID)
	}
	return nil
}
