//go:build !windows

package collectorlifecycle

import "os"

func testTokenOwnerUID() *uint64 {
	uid := uint64(os.Geteuid())
	return &uid
}
