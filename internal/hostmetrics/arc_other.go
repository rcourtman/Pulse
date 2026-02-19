//go:build !freebsd

package hostmetrics

import "fmt"

func readFreeBSDARCSize() (uint64, error) {
	return 0, fmt.Errorf("freebsd ARC sysctl not supported on this platform")
}
