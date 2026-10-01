//go:build unix

package server

import "golang.org/x/sys/unix"

func serviceHealthSocketIsDualStack(fd uintptr) bool {
	v6Only, err := unix.GetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_V6ONLY)
	return err == nil && v6Only == 0
}
