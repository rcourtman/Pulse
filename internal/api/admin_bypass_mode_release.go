//go:build release

package api

func resolveAdminBypassEnv() (enabled, declined bool) {
	return false, false
}
