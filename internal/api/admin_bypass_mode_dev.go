//go:build !release

package api

import (
	"os"
	"strings"
)

func resolveAdminBypassEnv() (enabled, declined bool) {
	if os.Getenv("ALLOW_ADMIN_BYPASS") != "1" {
		return false, false
	}
	if os.Getenv("PULSE_DEV") == "true" || strings.EqualFold(os.Getenv("NODE_ENV"), "development") {
		return true, false
	}
	return false, true
}
