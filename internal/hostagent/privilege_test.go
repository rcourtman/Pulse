package hostagent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectPrivilegeStatusReportsProcessFacts(t *testing.T) {
	t.Setenv("PULSE_SMARTCTL_PATH", "")
	t.Setenv("PULSE_PCT_PATH", "")

	status := collectPrivilegeStatus(CommandAuthorityMonitoringOnly)
	if status == nil {
		t.Fatal("collectPrivilegeStatus returned nil")
	}
	if status.RunningAsRoot != (os.Geteuid() == 0) {
		t.Fatalf("RunningAsRoot = %v, euid = %d", status.RunningAsRoot, os.Geteuid())
	}
	if status.ServiceUser == "" {
		t.Fatal("ServiceUser is empty")
	}
	if status.CommandAuthority != string(CommandAuthorityMonitoringOnly) {
		t.Fatalf("CommandAuthority = %q", status.CommandAuthority)
	}
	if status.SmartctlHelper || status.PctHelper {
		t.Fatalf("helper flags set without overrides: %+v", status)
	}
}

func TestCollectPrivilegeStatusReportsHelperOverrides(t *testing.T) {
	t.Setenv("PULSE_SMARTCTL_PATH", "/usr/local/lib/pulse-agent/smartctl-helper")
	t.Setenv("PULSE_PCT_PATH", "/usr/local/lib/pulse-agent/pct-helper")

	status := collectPrivilegeStatus(CommandAuthorityCommandCapable)
	if !status.SmartctlHelper || !status.PctHelper {
		t.Fatalf("helper overrides not reported: %+v", status)
	}
}

func TestResolvePctPathHonorsAbsoluteOverride(t *testing.T) {
	override := filepath.Join(t.TempDir(), "pct-helper")
	t.Setenv("PULSE_PCT_PATH", override)

	resolved, err := resolvePctPath(func(string) (string, error) {
		t.Fatal("lookPath must not be consulted when the override is set")
		return "", nil
	})
	if err != nil {
		t.Fatalf("resolvePctPath: %v", err)
	}
	if resolved != override {
		t.Fatalf("resolved = %q", resolved)
	}
}

func TestResolvePctPathRejectsRelativeOverride(t *testing.T) {
	t.Setenv("PULSE_PCT_PATH", "bin/pct")

	if _, err := resolvePctPath(func(string) (string, error) { return "/usr/sbin/pct", nil }); err == nil {
		t.Fatal("relative PULSE_PCT_PATH accepted; a PATH-relative helper could be hijacked")
	}
}

func TestResolvePctPathFallsBackToLookPath(t *testing.T) {
	t.Setenv("PULSE_PCT_PATH", "")

	resolved, err := resolvePctPath(func(name string) (string, error) {
		if name != "pct" {
			t.Fatalf("lookPath(%q)", name)
		}
		return "/usr/sbin/pct", nil
	})
	if err != nil || resolved != "/usr/sbin/pct" {
		t.Fatalf("resolved = %q, err = %v", resolved, err)
	}
}
