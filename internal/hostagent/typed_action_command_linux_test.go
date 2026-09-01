//go:build linux

package hostagent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == typedActionLauncherCommand {
		if target := os.Getenv("PULSE_TEST_TYPED_ACTION_TARGET"); target != "" {
			typedActionResolveTarget = func(typedActionCatalog, string) (string, error) { return target, nil }
		}
		os.Exit(RunTypedActionLauncher(os.Args[2:]))
	}
	os.Exit(m.Run())
}

func withTypedActionLinuxHooks(t *testing.T) {
	t.Helper()
	oldTool := typedActionResolveSystemdTool
	oldExecutable := typedActionCurrentExecutable
	oldQuery := typedActionQueryUnitLoadState
	oldStop := typedActionStopUnit
	oldTarget := typedActionResolveTarget
	oldSubreaper := typedActionBecomeSubreaper
	oldInspect := typedActionInspectDescendants
	oldWait4 := typedActionWait4
	t.Cleanup(func() {
		typedActionResolveSystemdTool = oldTool
		typedActionCurrentExecutable = oldExecutable
		typedActionQueryUnitLoadState = oldQuery
		typedActionStopUnit = oldStop
		typedActionResolveTarget = oldTarget
		typedActionBecomeSubreaper = oldSubreaper
		typedActionInspectDescendants = oldInspect
		typedActionWait4 = oldWait4
	})
}

func TestTypedActionSystemdArgumentsAreHardenedAndTerminateOptions(t *testing.T) {
	args, err := typedActionSystemdRunArgs(
		"pulse-agent-action-0123456789abcdef0123456789abcdef.service",
		"/usr/local/bin/pulse-agent-runner", "/var/lib/pulse-agent-runner/typed-actions/result.json",
		[]string{"DEBIAN_FRONTEND=noninteractive"}, typedActionCatalogPackage, "apt-get", []string{"update"},
	)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, "\n")
	for _, required := range []string{
		"--property=KillMode=control-group", "--property=SendSIGKILL=yes", "--property=ProtectControlGroups=yes",
		"--property=BindsTo=pulse-agent-runner.service", "--property=PartOf=pulse-agent-runner.service",
		"--property=NoNewPrivileges=yes", "--property=RuntimeMaxSec=20m", "--property=RestrictSUIDSGID=no",
		"--property=ProtectKernelModules=no", "--collect", "--pipe", "--wait",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("systemd arguments missing %q:\n%s", required, joined)
		}
	}
	separator := -1
	for index, value := range args {
		if value == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || args[separator+1] != "/usr/local/bin/pulse-agent-runner" || args[separator+2] != typedActionLauncherCommand {
		t.Fatalf("systemd option terminator/launcher binding is wrong: %v", args)
	}
}

func TestTypedActionNonPackageCatalogKeepsKernelAndSUIDRestrictions(t *testing.T) {
	args, err := typedActionSystemdRunArgs(
		"pulse-agent-action-0123456789abcdef0123456789abcdef.service",
		"/usr/local/bin/pulse-agent-runner", "/var/lib/pulse-agent-runner/typed-actions/result.json",
		nil, typedActionCatalogProxmox, "qm", []string{"status", "100"},
	)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, "\n")
	for _, required := range []string{"--property=RestrictSUIDSGID=yes", "--property=ProtectKernelModules=yes"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("Proxmox transient service missing %q:\n%s", required, joined)
		}
	}
}

func TestTypedActionLauncherSidebandDisambiguatesReservedExitAndDescendants(t *testing.T) {
	withTypedActionLinuxHooks(t)
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target.sh")
	if err := os.WriteFile(target, []byte("#!/bin/sh\nexit 200\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	typedActionResolveTarget = func(typedActionCatalog, string) (string, error) { return target, nil }
	typedActionBecomeSubreaper = func() error { return nil }
	typedActionInspectDescendants = func() (bool, error) { return false, nil }
	resultPath := filepath.Join(tmp, "result.json")
	exitCode := RunTypedActionLauncher([]string{"--catalog", "package", "--result-file", resultPath, "--", "apt-get", "update"})
	if exitCode != 200 {
		t.Fatalf("launcher exit=%d want target's reserved-value exit", exitCode)
	}
	result, err := readTypedActionLauncherResult(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	if result.DescendantsObserved || !result.ScanComplete || result.ExitCode != 200 {
		t.Fatalf("sideband did not preserve target result: %#v", result)
	}

	typedActionInspectDescendants = func() (bool, error) { return true, nil }
	resultPath = filepath.Join(tmp, "descendant.json")
	exitCode = RunTypedActionLauncher([]string{"--catalog", "package", "--result-file", resultPath, "--", "apt-get", "update"})
	result, err = readTypedActionLauncherResult(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	if exitCode != typedActionLauncherDescendantExit || !result.DescendantsObserved || result.ExitCode != 200 {
		t.Fatalf("descendant sideband=%#v launcher_exit=%d", result, exitCode)
	}
}

func TestTypedActionLauncherInspectionFailureIsIndeterminate(t *testing.T) {
	withTypedActionLinuxHooks(t)
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target.sh")
	if err := os.WriteFile(target, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	typedActionResolveTarget = func(typedActionCatalog, string) (string, error) { return target, nil }
	typedActionBecomeSubreaper = func() error { return nil }
	typedActionInspectDescendants = func() (bool, error) { return false, errors.New("wait unavailable") }
	resultPath := filepath.Join(tmp, "result.json")
	if got := RunTypedActionLauncher([]string{"--catalog", "package", "--result-file", resultPath, "--", "apt-get", "update"}); got != typedActionLauncherInspectionFailureExit {
		t.Fatalf("launcher exit=%d", got)
	}
	result, err := readTypedActionLauncherResult(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	if result.ScanComplete || result.Error == "" {
		t.Fatalf("inspection failure sideband=%#v", result)
	}
}

func TestTypedActionAdoptedChildFailureIsIndeterminate(t *testing.T) {
	withTypedActionLinuxHooks(t)
	for _, test := range []struct {
		name   string
		status syscall.WaitStatus
		want   bool
	}{
		{name: "zero exit is reaped", status: syscall.WaitStatus(0), want: false},
		{name: "nonzero exit is indeterminate", status: syscall.WaitStatus(42 << 8), want: true},
		{name: "signal is indeterminate", status: syscall.WaitStatus(syscall.SIGTERM), want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			typedActionWait4 = func(_ int, status *syscall.WaitStatus, _ int, _ *syscall.Rusage) (int, error) {
				calls++
				if calls == 1 {
					*status = test.status
					return 123, nil
				}
				return -1, syscall.ECHILD
			}
			got, err := typedActionHasLiveDescendants()
			if err != nil || got != test.want {
				t.Fatalf("adopted child status=%#x got=%v err=%v want=%v", uint32(test.status), got, err, test.want)
			}
		})
	}
}

func TestValidateTrustedExecutableRequiresControlledExecutableFile(t *testing.T) {
	valid := filepath.Join(t.TempDir(), "valid")
	if err := os.WriteFile(valid, []byte("binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := validateTrustedExecutable(valid, uint32(os.Geteuid())); err != nil {
		t.Fatalf("valid executable rejected: %v", err)
	}
	nonExecutable := filepath.Join(t.TempDir(), "non-executable")
	if err := os.WriteFile(nonExecutable, []byte("binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateTrustedExecutable(nonExecutable, uint32(os.Geteuid())); err == nil {
		t.Fatal("non-executable file was trusted")
	}
	writable := filepath.Join(t.TempDir(), "group-writable")
	if err := os.WriteFile(writable, []byte("binary"), 0o720); err != nil {
		t.Fatal(err)
	}
	if err := validateTrustedExecutable(writable, uint32(os.Geteuid())); err == nil {
		t.Fatal("group-writable executable was trusted")
	}
	symlink := filepath.Join(t.TempDir(), "symlink")
	if err := os.Symlink(valid, symlink); err != nil {
		t.Fatal(err)
	}
	if err := validateTrustedExecutable(symlink, uint32(os.Geteuid())); err == nil {
		t.Fatal("symlink was trusted without resolution at the boundary")
	}
	if err := validateTrustedExecutable(t.TempDir(), uint32(os.Geteuid())); err == nil {
		t.Fatal("directory was trusted as an executable")
	}
}

func TestTypedActionWaitsForUnitAbsenceBeforeReturning(t *testing.T) {
	withTypedActionLinuxHooks(t)
	tmp := t.TempDir()
	t.Setenv("PULSE_AGENT_RUNNER_STATE_DIR", tmp)
	fakeRun := filepath.Join(tmp, "systemd-run")
	script := `#!/bin/sh
result=""
previous=""
for arg in "$@"; do
  if [ "$previous" = "--result-file" ]; then result="$arg"; break; fi
  previous="$arg"
done
printf '%s' '{"version":1,"target_started":true,"exit_code":0,"scan_complete":true,"descendants_observed":false}' > "$result"
chmod 600 "$result"
`
	if err := os.WriteFile(fakeRun, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	typedActionResolveSystemdTool = func(string) (string, error) { return fakeRun, nil }
	typedActionCurrentExecutable = func() (string, error) { return fakeRun, nil }
	var queries atomic.Int32
	releaseQuery := make(chan struct{})
	typedActionQueryUnitLoadState = func(string, string) (string, error) {
		queries.Add(1)
		select {
		case <-releaseQuery:
			return "not-found", nil
		default:
			return "loaded", nil
		}
	}
	typedActionStopUnit = func(string, string) error { return nil }
	done := make(chan typedActionCommandResult, 1)
	go func() {
		done <- runTypedActionCommand(context.Background(), nil, typedActionCatalogPackage, "apt-get", "update")
	}()
	deadline := time.Now().Add(2 * time.Second)
	for queries.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	select {
	case result := <-done:
		t.Fatalf("command returned before unit collection: %#v", result)
	default:
	}
	close(releaseQuery)
	select {
	case result := <-done:
		if result.err != nil || result.exitCode != 0 {
			t.Fatalf("command result=%#v", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("command did not return after unit collection")
	}
}

func TestTypedActionCancellationBeforeUnitCreationStillStopsLateUnit(t *testing.T) {
	withTypedActionLinuxHooks(t)
	tmp := t.TempDir()
	t.Setenv("PULSE_AGENT_RUNNER_STATE_DIR", tmp)
	startedPath := filepath.Join(tmp, "started")
	releasePath := filepath.Join(tmp, "release")
	fakeRun := filepath.Join(tmp, "systemd-run")
	script := "#!/bin/sh\n: > " + startedPath + "\nwhile [ ! -f " + releasePath + " ]; do sleep 0.01; done\n"
	if err := os.WriteFile(fakeRun, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	typedActionResolveSystemdTool = func(string) (string, error) { return fakeRun, nil }
	typedActionCurrentExecutable = func() (string, error) { return fakeRun, nil }
	var queries atomic.Int32
	var stops atomic.Int32
	typedActionQueryUnitLoadState = func(string, string) (string, error) {
		query := queries.Add(1)
		if query == 1 {
			return "not-found", nil
		}
		if _, err := os.Stat(releasePath); err == nil {
			return "not-found", nil
		}
		return "loaded", nil
	}
	typedActionStopUnit = func(string, string) error {
		if stops.Add(1) == 1 {
			return errors.New("unit not created yet")
		}
		return os.WriteFile(releasePath, []byte("release"), 0o600)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan typedActionCommandResult, 1)
	go func() {
		done <- runTypedActionCommand(ctx, nil, typedActionCatalogPackage, "apt-get", "update")
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(startedPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fake systemd-run did not start")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case result := <-done:
		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf("canceled result=%#v", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("canceled command did not finish after the late unit was stopped")
	}
	if stops.Load() < 2 || queries.Load() < 3 {
		t.Fatalf("creation race was not retried: queries=%d stops=%d", queries.Load(), stops.Load())
	}
}

func TestPackageLeaseRemainsHeldUntilTypedActionUnitIsAbsent(t *testing.T) {
	withTypedActionLinuxHooks(t)
	tmp := t.TempDir()
	t.Setenv("PULSE_AGENT_RUNNER_STATE_DIR", tmp)
	fakeRun := filepath.Join(tmp, "systemd-run")
	script := `#!/bin/sh
result=""
previous=""
for arg in "$@"; do
  if [ "$previous" = "--result-file" ]; then result="$arg"; break; fi
  previous="$arg"
done
printf '%s' '{"version":1,"target_started":true,"exit_code":0,"scan_complete":true,"descendants_observed":false}' > "$result"
chmod 600 "$result"
`
	if err := os.WriteFile(fakeRun, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	typedActionResolveSystemdTool = func(string) (string, error) { return fakeRun, nil }
	typedActionCurrentExecutable = func() (string, error) { return fakeRun, nil }
	unitAbsent := make(chan struct{})
	queried := make(chan struct{}, 1)
	typedActionQueryUnitLoadState = func(string, string) (string, error) {
		select {
		case queried <- struct{}{}:
		default:
		}
		select {
		case <-unitAbsent:
			return "not-found", nil
		default:
			return "loaded", nil
		}
	}
	typedActionStopUnit = func(string, string) error { return nil }
	lease := newPackageManagerLease()
	release, err := lease.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan typedActionCommandResult, 1)
	go func() {
		result := runTypedActionCommand(context.Background(), nil, typedActionCatalogPackage, "apt-get", "update")
		release()
		done <- result
	}()
	select {
	case <-queried:
	case <-time.After(2 * time.Second):
		t.Fatal("typed action never reached the unit-collection fence")
	}
	blockedCtx, blockedCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer blockedCancel()
	if acquired, err := lease.acquire(blockedCtx); !errors.Is(err, context.DeadlineExceeded) {
		if acquired != nil {
			acquired()
		}
		t.Fatalf("package lease released before unit absence: %v", err)
	}
	close(unitAbsent)
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("typed action result=%#v", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("typed action did not release after unit absence")
	}
	postCtx, postCancel := context.WithTimeout(context.Background(), time.Second)
	defer postCancel()
	postRelease, err := lease.acquire(postCtx)
	if err != nil {
		t.Fatalf("package lease remained fenced after unit absence: %v", err)
	}
	postRelease()
}

func TestTypedActionManagerQueryFailureKeepsGlobalGateFenced(t *testing.T) {
	withTypedActionLinuxHooks(t)
	tmp := t.TempDir()
	t.Setenv("PULSE_AGENT_RUNNER_STATE_DIR", tmp)
	fakeRun := filepath.Join(tmp, "systemd-run")
	script := `#!/bin/sh
result=""
previous=""
for arg in "$@"; do
  if [ "$previous" = "--result-file" ]; then result="$arg"; break; fi
  previous="$arg"
done
printf '%s' '{"version":1,"target_started":true,"exit_code":0,"scan_complete":true,"descendants_observed":false}' > "$result"
chmod 600 "$result"
`
	if err := os.WriteFile(fakeRun, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	typedActionResolveSystemdTool = func(string) (string, error) { return fakeRun, nil }
	typedActionCurrentExecutable = func() (string, error) { return fakeRun, nil }
	recoverManager := make(chan struct{})
	queried := make(chan struct{}, 1)
	typedActionQueryUnitLoadState = func(string, string) (string, error) {
		select {
		case queried <- struct{}{}:
		default:
		}
		select {
		case <-recoverManager:
			return "not-found", nil
		default:
			return "", errors.New("system manager unavailable")
		}
	}
	typedActionStopUnit = func(string, string) error { return errors.New("system manager unavailable") }
	firstDone := make(chan typedActionCommandResult, 1)
	go func() {
		firstDone <- runTypedActionCommand(context.Background(), nil, typedActionCatalogPackage, "apt-get", "update")
	}()
	select {
	case <-queried:
	case <-time.After(2 * time.Second):
		t.Fatal("typed action did not reach manager verification")
	}
	blockedCtx, blockedCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer blockedCancel()
	blocked := runTypedActionCommand(blockedCtx, nil, typedActionCatalogPackage, "apt-get", "update")
	if !errors.Is(blocked.err, context.DeadlineExceeded) {
		t.Fatalf("global gate admitted overlapping work while manager state was unknown: %#v", blocked)
	}
	select {
	case result := <-firstDone:
		t.Fatalf("first action returned while manager state was unknown: %#v", result)
	default:
	}
	close(recoverManager)
	select {
	case result := <-firstDone:
		if result.err != nil {
			t.Fatalf("first action did not recover after authoritative absence: %#v", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("first action remained fenced after manager recovery")
	}
}

func TestTypedActionRealSystemdContainmentProbe(t *testing.T) {
	if os.Getenv("PULSE_TEST_SYSTEMD_TYPED_ACTION") != "1" {
		t.Skip("set PULSE_TEST_SYSTEMD_TYPED_ACTION=1 inside a root systemd host")
	}
	if os.Geteuid() != 0 {
		t.Fatal("real transient-service proof must run as root")
	}
	withTypedActionLinuxHooks(t)
	stateDir, err := os.MkdirTemp("/run", "pulse-agent-runner-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(stateDir) })
	t.Setenv("PULSE_AGENT_RUNNER_STATE_DIR", stateDir)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	runnerPath := filepath.Join(stateDir, "pulse-agent-runner-test")
	binary, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runnerPath, binary, 0o700); err != nil {
		t.Fatal(err)
	}
	typedActionCurrentExecutable = func() (string, error) { return runnerPath, nil }
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	result := runTypedActionCommand(ctx, nil, typedActionCatalogProbe, "true")
	if result.err != nil || result.exitCode != 0 {
		t.Fatalf("real systemd containment probe exit=%d err=%v stderr=%q", result.exitCode, result.err, result.stderr)
	}
}

func TestTypedActionRealSystemdKillsDetachedDescendantBeforeReturn(t *testing.T) {
	if os.Getenv("PULSE_TEST_SYSTEMD_TYPED_ACTION") != "1" {
		t.Skip("set PULSE_TEST_SYSTEMD_TYPED_ACTION=1 inside a root systemd host")
	}
	if os.Geteuid() != 0 {
		t.Fatal("real transient-service proof must run as root")
	}
	withTypedActionLinuxHooks(t)
	stateDir, err := os.MkdirTemp("/run", "pulse-agent-runner-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(stateDir) })
	t.Setenv("PULSE_AGENT_RUNNER_STATE_DIR", stateDir)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	testBinary := filepath.Join(stateDir, "hostagent.test")
	binary, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testBinary, binary, 0o700); err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(stateDir, "descendant.pid")
	target := filepath.Join(stateDir, "detached-target.sh")
	targetScript := "#!/bin/sh\nsh -c 'sleep 300 >/dev/null 2>&1 & echo $! > \"$1\"; exit 0' child " + pidFile + " &\nwait\nexit 0\n"
	if err := os.WriteFile(target, []byte(targetScript), 0o700); err != nil {
		t.Fatal(err)
	}
	runnerPath := filepath.Join(stateDir, "pulse-agent-runner-test")
	runnerScript := "#!/bin/sh\nexport PULSE_TEST_TYPED_ACTION_TARGET=" + target + "\nexec " + testBinary + " \"$@\"\n"
	if err := os.WriteFile(runnerPath, []byte(runnerScript), 0o700); err != nil {
		t.Fatal(err)
	}
	typedActionCurrentExecutable = func() (string, error) { return runnerPath, nil }
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	result := runTypedActionCommand(ctx, nil, typedActionCatalogProbe, "true")
	if !errors.Is(result.err, errTypedActionContainmentIndeterminate) || result.exitCode != -1 {
		t.Fatalf("detached descendant result exit=%d err=%v stderr=%q", result.exitCode, result.err, result.stderr)
	}
	pid := readPidFile(t, pidFile)
	waitForProcessGone(t, pid, 5*time.Second)
}

func TestTypedActionRealSystemdPackageUnitCanInstallSetuidPayload(t *testing.T) {
	if os.Getenv("PULSE_TEST_SYSTEMD_TYPED_ACTION") != "1" {
		t.Skip("set PULSE_TEST_SYSTEMD_TYPED_ACTION=1 inside a root systemd host")
	}
	if os.Geteuid() != 0 {
		t.Fatal("real transient-service proof must run as root")
	}
	withTypedActionLinuxHooks(t)
	stateDir, err := os.MkdirTemp("/run", "pulse-agent-runner-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(stateDir) })
	t.Setenv("PULSE_AGENT_RUNNER_STATE_DIR", stateDir)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	testBinary := filepath.Join(stateDir, "hostagent.test")
	binary, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testBinary, binary, 0o700); err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(stateDir, "setuid-payload")
	target := filepath.Join(stateDir, "package-target.sh")
	targetScript := "#!/bin/sh\n: > " + payload + "\nchmod 4755 " + payload + "\n"
	if err := os.WriteFile(target, []byte(targetScript), 0o700); err != nil {
		t.Fatal(err)
	}
	runnerPath := filepath.Join(stateDir, "pulse-agent-runner-test")
	runnerScript := "#!/bin/sh\nexport PULSE_TEST_TYPED_ACTION_TARGET=" + target + "\nexec " + testBinary + " \"$@\"\n"
	if err := os.WriteFile(runnerPath, []byte(runnerScript), 0o700); err != nil {
		t.Fatal(err)
	}
	typedActionCurrentExecutable = func() (string, error) { return runnerPath, nil }
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	result := runTypedActionCommand(ctx, nil, typedActionCatalogPackage, "apt-get", "update")
	if result.err != nil || result.exitCode != 0 {
		t.Fatalf("package setuid fixture exit=%d err=%v stderr=%q", result.exitCode, result.err, result.stderr)
	}
	info, err := os.Stat(payload)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSetuid == 0 {
		t.Fatalf("package fixture mode=%v, setuid bit was not preserved", info.Mode())
	}
}
