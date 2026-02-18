//go:build freebsd

package hostmetrics

import (
	"encoding/binary"
	"fmt"
	"syscall"
)

func readFreeBSDARCSize() (uint64, error) {
	// kstat.zfs.misc.arcstats.size holds the current ARC size in bytes.
	// This is a uint64 sysctl on FreeBSD.
	raw, err := syscall.SysctlRaw("kstat.zfs.misc.arcstats.size")
	if err != nil {
		return 0, err
	}

	switch len(raw) {
	case 8:
		return binary.NativeEndian.Uint64(raw), nil
	case 4:
		// Defensive: some environments may expose a 32-bit value.
		return uint64(binary.NativeEndian.Uint32(raw)), nil
	default:
		return 0, fmt.Errorf("unexpected sysctl value size: %d", len(raw))
	}
}
