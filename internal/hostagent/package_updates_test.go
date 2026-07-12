package hostagent

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

func TestParseAPTSimulatedUpgrades(t *testing.T) {
	output := `Reading package lists...
Inst openssl [3.0.11-1] (3.0.12-1 Debian-Security:12/stable-security [amd64])
Inst linux-image-amd64 [6.1.0-18] (6.1.0-19 Debian:12/stable [amd64])
Conf openssl (3.0.12-1 Debian-Security:12/stable-security [amd64])`

	got := parseAPTSimulatedUpgrades(output)
	want := []agentexec.HostPackageUpdate{
		{Name: "openssl", InstalledVersion: "3.0.11-1", AvailableVersion: "3.0.12-1"},
		{Name: "linux-image-amd64", InstalledVersion: "6.1.0-18", AvailableVersion: "6.1.0-19"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("updates = %#v, want %#v", got, want)
	}
	if count := countAPTSimulatedUpgrades(output); count != 2 {
		t.Fatalf("pending count = %d, want 2", count)
	}
}

func TestPackageUpdateManagerApplyUsesClosedAPTCommandCatalogAndVerifies(t *testing.T) {
	beforeOutput := "Inst openssl [1.0] (1.1 repo [amd64])\nInst curl [8.0] (8.1 repo [amd64])\n"
	m := newPackageUpdateManager("linux", newPackageManagerLease())
	m.lookPath = func(name string) (string, error) {
		if name != "apt-get" && name != "dpkg" {
			t.Fatalf("lookPath(%q), want fixed apt-get/dpkg catalog", name)
		}
		return "/usr/bin/" + name, nil
	}
	fileInfo, err := os.Stat(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.stat = func(path string) (os.FileInfo, error) {
		if path != "/var/run/reboot-required" {
			t.Fatalf("stat path = %q", path)
		}
		return fileInfo, nil
	}

	type invocation struct {
		env  []string
		name string
		args []string
	}
	var calls []invocation
	simulations := 0
	m.run = func(_ context.Context, env []string, name string, args ...string) packageUpdateCommandResult {
		calls = append(calls, invocation{env: append([]string(nil), env...), name: name, args: append([]string(nil), args...)})
		if name == "dpkg" && strings.Join(args, " ") == "--audit" {
			return packageUpdateCommandResult{}
		}
		if name != "apt-get" {
			return packageUpdateCommandResult{exitCode: -1, err: errors.New("unexpected executable")}
		}
		if strings.Contains(strings.Join(args, " "), "-s") {
			simulations++
			if simulations <= 2 {
				return packageUpdateCommandResult{stdout: beforeOutput}
			}
			return packageUpdateCommandResult{}
		}
		return packageUpdateCommandResult{}
	}

	result := m.Apply(context.Background(), agentexec.HostUpdatePayload{
		RequestID:             "request-1",
		ActionID:              "action-1",
		Operation:             agentexec.HostUpdateOperationInstall,
		ExpectedInventoryHash: aptUpgradeInventoryHash(beforeOutput),
	})
	if !result.Success || result.Verification != agentexec.HostUpdateVerificationVerified {
		t.Fatalf("result = %#v, want successful verified update", result)
	}
	if result.Before.PendingCount != 2 || result.After.PendingCount != 0 || !result.After.RebootRequired {
		t.Fatalf("before/after = %#v / %#v", result.Before, result.After)
	}
	if len(calls) != 7 {
		t.Fatalf("calls = %d, want probe, refresh, preflight, pre-install health, install, verify, post-install health", len(calls))
	}
	if got := strings.Join(calls[1].args, " "); got != "update" {
		t.Fatalf("refresh args = %q", got)
	}
	install := calls[4]
	if install.name != "apt-get" || !strings.Contains(strings.Join(install.args, " "), "--no-remove") || !strings.Contains(strings.Join(install.args, " "), "--force-confold") {
		t.Fatalf("install invocation = %#v", install)
	}
	if !containsString(install.env, "DEBIAN_FRONTEND=noninteractive") || !containsString(install.env, "NEEDRESTART_MODE=a") {
		t.Fatalf("install env = %#v", install.env)
	}
	for _, call := range calls {
		if call.name == "sh" || call.name == "bash" {
			t.Fatalf("typed package update must not dispatch a shell: %#v", call)
		}
	}
}

func TestPackageUpdateManagerFailsClosedWhenRefreshFails(t *testing.T) {
	m := newPackageUpdateManager("linux", newPackageManagerLease())
	m.lookPath = func(string) (string, error) { return "/usr/bin/apt-get", nil }
	m.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	calls := 0
	m.run = func(_ context.Context, _ []string, name string, args ...string) packageUpdateCommandResult {
		calls++
		if name == "dpkg" {
			return packageUpdateCommandResult{}
		}
		if strings.Join(args, " ") == "update" {
			return packageUpdateCommandResult{exitCode: 100, err: errors.New("exit status 100")}
		}
		return packageUpdateCommandResult{stdout: "Inst openssl [1.0] (1.1 repo [amd64])\n"}
	}

	result := m.Apply(context.Background(), agentexec.HostUpdatePayload{RequestID: "r1", ActionID: "a1", Operation: agentexec.HostUpdateOperationInstall, ExpectedInventoryHash: aptUpgradeInventoryHash("Inst openssl [1.0] (1.1 repo [amd64])\n")})
	if result.Success || result.Verification != agentexec.HostUpdateVerificationInconclusive || result.Error != "package index refresh failed" {
		t.Fatalf("result = %#v", result)
	}
	if result.MutationStarted || result.RecoveryRequired || result.ExecutionPhase != agentexec.HostUpdatePhaseRefresh {
		t.Fatalf("refresh failure must not claim install mutation or recovery: %#v", result)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want probe, refresh, and bounded health check", calls)
	}
}

func TestPackageUpdateManagerCanceledRefreshReportsPossibleExternalEffect(t *testing.T) {
	m := newPackageUpdateManager("linux", newPackageManagerLease())
	m.lookPath = func(string) (string, error) { return "/usr/bin/apt-get", nil }
	m.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	m.run = func(ctx context.Context, _ []string, _ string, args ...string) packageUpdateCommandResult {
		if strings.Join(args, " ") == "update" {
			return packageUpdateCommandResult{exitCode: -1, err: context.Canceled}
		}
		return packageUpdateCommandResult{stdout: "Inst openssl [1.0] (1.1 repo [amd64])\n"}
	}
	result := m.Apply(context.Background(), agentexec.HostUpdatePayload{RequestID: "r1", ActionID: "a1", Operation: agentexec.HostUpdateOperationInstall, ExpectedInventoryHash: aptUpgradeInventoryHash("Inst openssl [1.0] (1.1 repo [amd64])\n")})
	if result.MutationStarted || result.RecoveryRequired || result.ExecutionPhase != agentexec.HostUpdatePhaseRefresh || result.Success {
		t.Fatalf("canceled refresh truth = %#v", result)
	}
}

func TestPackageUpdateManagerRefusesRefreshTimeInventoryDriftBeforeInstall(t *testing.T) {
	m := newPackageUpdateManager("linux", newPackageManagerLease())
	m.lookPath = func(string) (string, error) { return "/usr/bin/apt-get", nil }
	m.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	stale := "Inst openssl [1.0] (1.1 repo [amd64])\n"
	refreshed := stale + "Inst curl [8.0] (8.1 repo [amd64])\n"
	simulations := 0
	installCalls := 0
	m.run = func(_ context.Context, _ []string, _ string, args ...string) packageUpdateCommandResult {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "-s") {
			simulations++
			if simulations == 1 {
				return packageUpdateCommandResult{stdout: stale}
			}
			return packageUpdateCommandResult{stdout: refreshed}
		}
		if strings.Contains(joined, "upgrade") {
			installCalls++
		}
		return packageUpdateCommandResult{}
	}

	result := m.Apply(context.Background(), agentexec.HostUpdatePayload{
		RequestID: "r1", ActionID: "a1", Operation: agentexec.HostUpdateOperationInstall, ExpectedInventoryHash: aptUpgradeInventoryHash(stale),
	})
	if result.Success || result.Error != "package update inventory changed; replan required" || result.Before.InventoryHash != aptUpgradeInventoryHash(refreshed) {
		t.Fatalf("result = %#v", result)
	}
	if installCalls != 0 {
		t.Fatalf("install calls = %d, want zero on preflight drift", installCalls)
	}
	if result.MutationStarted || result.RecoveryRequired {
		t.Fatalf("refresh-time drift claimed install mutation/recovery: %#v", result)
	}
}

func TestPackageUpdateManagerPartialInstallAndVerifyFailureCarryExplicitHealthAndRecovery(t *testing.T) {
	pending := "Inst openssl [1.0] (1.1 repo [amd64])\nInst curl [8.0] (8.1 repo [amd64])\n"
	for _, tc := range []struct {
		name          string
		installFails  bool
		auditOutput   string
		wantPhase     string
		wantHealthy   bool
		wantRemaining int
		wantSuccess   bool
	}{
		{name: "partial install unhealthy", installFails: true, auditOutput: "Packages are only half configured", wantPhase: agentexec.HostUpdatePhaseInstall, wantRemaining: 2},
		{name: "verify failure healthy manager", wantPhase: agentexec.HostUpdatePhaseVerify, wantHealthy: true, wantRemaining: 1, wantSuccess: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newPackageUpdateManager("linux", newPackageManagerLease())
			m.lookPath = func(name string) (string, error) { return "/fake/" + name, nil }
			m.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
			simulations := 0
			audits := 0
			m.run = func(_ context.Context, _ []string, name string, args ...string) packageUpdateCommandResult {
				joined := strings.Join(args, " ")
				if name == "dpkg" && joined == "--audit" {
					audits++
					if audits == 1 {
						return packageUpdateCommandResult{}
					}
					return packageUpdateCommandResult{stdout: tc.auditOutput}
				}
				if strings.Contains(joined, "-s") {
					simulations++
					if simulations <= 2 || tc.installFails {
						return packageUpdateCommandResult{stdout: pending}
					}
					return packageUpdateCommandResult{stdout: "Inst curl [8.0] (8.1 repo [amd64])\n"}
				}
				if tc.installFails && strings.Contains(joined, "-y --no-remove") {
					return packageUpdateCommandResult{exitCode: 100, err: errors.New("controlled install failure")}
				}
				return packageUpdateCommandResult{}
			}
			result := m.Apply(context.Background(), agentexec.HostUpdatePayload{RequestID: "partial.dispatch.1", ActionID: "partial", Operation: agentexec.HostUpdateOperationInstall, ExpectedInventoryHash: aptUpgradeInventoryHash(pending)})
			if result.ExecutionPhase != tc.wantPhase || result.Success != tc.wantSuccess || !result.MutationStarted || !result.HealthChecked || result.PackageManagerHealthy != tc.wantHealthy || !result.RecoveryRequired || result.After.PendingCount != tc.wantRemaining {
				t.Fatalf("partial truth=%#v", result)
			}
			if err := agentexec.ValidateHostUpdateResultPayload(&result); err != nil {
				t.Fatalf("agent produced invalid partial truth: %v", err)
			}
		})
	}
}

func TestPackageUpdateManagerPreInstallHealthUnknownRefusesWithoutMutationOrRecoveryClaim(t *testing.T) {
	pending := "Inst openssl [1.0] (1.1 repo [amd64])\n"
	m := newPackageUpdateManager("linux", newPackageManagerLease())
	m.lookPath = func(name string) (string, error) { return "/fake/" + name, nil }
	m.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	installCalls := 0
	m.run = func(_ context.Context, _ []string, name string, args ...string) packageUpdateCommandResult {
		joined := strings.Join(args, " ")
		if name == "dpkg" {
			return packageUpdateCommandResult{exitCode: -1, err: errors.New("controlled audit unavailable")}
		}
		if strings.Contains(joined, "-s") {
			return packageUpdateCommandResult{stdout: pending}
		}
		if strings.Contains(joined, "-y --no-remove") {
			installCalls++
		}
		return packageUpdateCommandResult{}
	}
	result := m.Apply(context.Background(), agentexec.HostUpdatePayload{RequestID: "health.dispatch.1", ActionID: "health", Operation: agentexec.HostUpdateOperationInstall, ExpectedInventoryHash: aptUpgradeInventoryHash(pending)})
	if result.Success || result.MutationStarted || result.RecoveryRequired || result.HealthChecked || result.ExecutionPhase != agentexec.HostUpdatePhasePreflight || installCalls != 0 {
		t.Fatalf("pre-install health refusal=%#v installCalls=%d", result, installCalls)
	}
	if err := agentexec.ValidateHostUpdateResultPayload(&result); err != nil {
		t.Fatalf("agent produced invalid refusal truth: %v", err)
	}
}

func TestPackageUpdateManagerZeroPendingAfterRefreshIsDriftWithoutInstallOrVerifiedClaim(t *testing.T) {
	pending := "Inst openssl [1.0] (1.1 repo [amd64])\n"
	m := newPackageUpdateManager("linux", newPackageManagerLease())
	m.lookPath = func(name string) (string, error) { return "/fake/" + name, nil }
	m.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	simulations := 0
	installCalls := 0
	m.run = func(_ context.Context, _ []string, name string, args ...string) packageUpdateCommandResult {
		joined := strings.Join(args, " ")
		if name == "dpkg" {
			return packageUpdateCommandResult{}
		}
		if strings.Contains(joined, "-s") {
			simulations++
			if simulations == 1 {
				return packageUpdateCommandResult{stdout: pending}
			}
			return packageUpdateCommandResult{}
		}
		if strings.Contains(joined, "-y --no-remove") {
			installCalls++
		}
		return packageUpdateCommandResult{}
	}
	result := m.Apply(context.Background(), agentexec.HostUpdatePayload{RequestID: "noop.dispatch.1", ActionID: "noop", Operation: agentexec.HostUpdateOperationInstall, ExpectedInventoryHash: aptUpgradeInventoryHash(pending)})
	if result.Success || result.MutationStarted || result.RecoveryRequired || result.ExecutionPhase != agentexec.HostUpdatePhaseRefresh || result.Verification != agentexec.HostUpdateVerificationInconclusive || !strings.Contains(result.Error, "replan") || installCalls != 0 {
		t.Fatalf("zero-pending drift=%#v installCalls=%d", result, installCalls)
	}
}

func TestPackageUpdateSnapshotCachesAndUnsupportedPlatformsFailClosed(t *testing.T) {
	now := time.Date(2026, 7, 11, 1, 0, 0, 0, time.UTC)
	m := newPackageUpdateManager("windows", newPackageManagerLease())
	m.now = func() time.Time { return now }
	m.run = func(context.Context, []string, string, ...string) packageUpdateCommandResult {
		t.Fatal("unsupported platform must not execute package manager")
		return packageUpdateCommandResult{}
	}

	first := m.Snapshot(context.Background(), false)
	second := m.Snapshot(context.Background(), false)
	if first.Supported || second.Supported || !first.CheckedAt.Equal(now) || !second.CheckedAt.Equal(now) {
		t.Fatalf("snapshots = %#v / %#v", first, second)
	}
}

func TestPackageUpdateSnapshotSupportsLinuxDistroIdentity(t *testing.T) {
	for _, platform := range []string{"debian", "ubuntu"} {
		t.Run(platform, func(t *testing.T) {
			m := newPackageUpdateManager(platform, newPackageManagerLease())
			m.lookPath = func(string) (string, error) { return "/usr/bin/apt-get", nil }
			m.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
			m.run = func(context.Context, []string, string, ...string) packageUpdateCommandResult {
				return packageUpdateCommandResult{}
			}
			if snapshot := m.Snapshot(context.Background(), true); !snapshot.Supported || snapshot.Manager != "apt" {
				t.Fatalf("distro package manager disabled: %#v", snapshot)
			}
		})
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
