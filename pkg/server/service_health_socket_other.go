//go:build !unix && !windows

package server

func serviceHealthSocketIsDualStack(_ uintptr) bool {
	return false
}
