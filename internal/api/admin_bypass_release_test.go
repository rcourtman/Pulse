//go:build release

package api

import "testing"

func TestAdminBypassEnabled_IgnoredInReleaseBuild(t *testing.T) {
	t.Setenv("ALLOW_ADMIN_BYPASS", "1")
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("NODE_ENV", "development")
	resetAdminBypassState()

	if adminBypassEnabled() {
		t.Fatal("adminBypassEnabled() should remain false in release builds")
	}
	if adminBypassState.declined {
		t.Fatal("release builds should not treat ALLOW_ADMIN_BYPASS as a runtime path")
	}
}
