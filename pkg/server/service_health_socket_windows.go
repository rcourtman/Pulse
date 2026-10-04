//go:build windows

package server

import "golang.org/x/sys/windows"

func serviceHealthSocketIsDualStack(fd uintptr) bool {
	v6Only, err := windows.GetsockoptInt(windows.Handle(fd), windows.IPPROTO_IPV6, windows.IPV6_V6ONLY)
	return err == nil && v6Only == 0
}
