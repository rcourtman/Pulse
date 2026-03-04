//go:build !freebsd && !linux

package hostmetrics

// readARCSize returns 0 on platforms where ZFS ARC adjustment is not needed.
func readARCSize() (uint64, error) {
	return 0, nil
}
