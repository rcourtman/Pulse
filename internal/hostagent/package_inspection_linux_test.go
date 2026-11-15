//go:build linux

package hostagent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rs/zerolog"
	gohost "github.com/shirou/gopsutil/v4/host"
)

const inspectionFixtureUpdates = "Inst openssl [1.0] (1.1 repo [amd64])\nInst curl [8.0] (8.1 repo [amd64])\n"

func withPackageInspectionExecutable(t *testing.T, script string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "apt-get")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	old := packageInspectionResolveAPT
	packageInspectionResolveAPT = func() (string, error) { return path, nil }
	t.Cleanup(func() { packageInspectionResolveAPT = old })
}

func TestCollectorPackageInspectionReportsWithoutRunner(t *testing.T) {
	t.Setenv("PULSE_AGENT_RUNNER_STATE_DIR", "")
	withPackageInspectionExecutable(t, fmt.Sprintf("printf '%%b' %q\n", inspectionFixtureUpdates))
	reporter, _ := configurePackageManagers("debian", nil, nil)
	inspector, ok := reporter.(*packageUpdateInspector)
	if !ok {
		t.Fatalf("collector reporter is %T, want a report-only inspector", reporter)
	}
	if _, exposesActions := reporter.(interface {
		Preflight(context.Context, agentexec.HostUpdatePayload) (bool, string)
	}); exposesActions {
		t.Fatal("collector reporter exposes action preflight")
	}
	inspector.manager.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	agent := &Agent{packageUpdates: reporter}
	status := agent.currentPackageUpdateStatus(context.Background())
	if status == nil || !status.Supported || status.Manager != "apt" || status.Error != "" || status.PendingCount != 2 || len(status.Packages) != 2 || status.InventoryHash != aptUpgradeInventoryHash(inspectionFixtureUpdates) {
		t.Fatalf("collector report = %+v", status)
	}
	// Cached reports keep their count and are detached from the returned slice.
	status.Packages[0].Name = "changed by caller"
	if again := agent.currentPackageUpdateStatus(context.Background()); again.Packages[0].Name != "openssl" {
		t.Fatalf("report mutated the saved observation: %+v", again)
	}
}

func TestPackageInspectionUsesFixedInvocationAndCleanEnvironment(t *testing.T) {
	t.Setenv("APT_CONFIG", "/private/not-an-apt-config")
	t.Setenv("PULSE_TOKEN", "fixture-private-token")
	t.Setenv("LC_ALL", "not-a-locale")
	t.Setenv("PATH", t.TempDir())
	withPackageInspectionExecutable(t, `printf 'cwd=%s\nargs=%s\nlocale=%s\npath=%s\nconfig=%s\ntoken=%s\n' "$PWD" "$*" "$LC_ALL" "$PATH" "${APT_CONFIG-unset}" "${PULSE_TOKEN-unset}"
read unexpected && exit 99
exit 0
`)
	result := runPackageInspectionCommand(context.Background())
	want := "cwd=/\nargs=-s -o Debug::NoLocking=1 upgrade\nlocale=C\npath=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\nconfig=unset\ntoken=unset\n"
	if result.err != nil || result.exitCode != 0 || result.stdout != want {
		t.Fatalf("fixed local read: result=%+v, want stdout %q", result, want)
	}
}

func TestPackageInspectionAdapterRefusesActionsAndCallerConfiguration(t *testing.T) {
	old := packageInspectionResolveAPT
	resolved := false
	packageInspectionResolveAPT = func() (string, error) { resolved = true; return "", errors.New("must not resolve") }
	t.Cleanup(func() { packageInspectionResolveAPT = old })
	for _, tc := range []struct {
		name string
		args []string
		env  []string
	}{
		{"apt-get", []string{"update"}, nil},
		{"apt-get", []string{"upgrade"}, nil},
		{"apt-get", []string{"clean"}, nil},
		{"dpkg", []string{"--audit"}, nil},
		{"apt-get", []string{"-s", "-o", "Debug::NoLocking=1", "upgrade", "extra"}, nil},
		{"apt-get", []string{"-s", "-o", "Debug::NoLocking=1", "upgrade"}, []string{"APT_CONFIG=/tmp/config"}},
	} {
		result := runPackageInspectionInvocation(context.Background(), tc.env, tc.name, tc.args...)
		if result.err == nil || result.exitCode != -1 || resolved {
			t.Fatalf("unapproved invocation resolved or ran: %+v: %+v", tc, result)
		}
	}
}

func TestPackageInspectionFailuresNeverPublishPartialInventory(t *testing.T) {
	for _, tc := range []struct {
		name, script string
	}{
		{"nonzero", fmt.Sprintf("printf '%%b' %q\nexit 100\n", inspectionFixtureUpdates)},
		{"signal", "kill -KILL $$\n"},
		{"stdout-limit", "head -c 1048577 /dev/zero\n"},
		{"stderr-limit", "head -c 1048577 /dev/zero >&2\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withPackageInspectionExecutable(t, tc.script)
			inspector := newPackageUpdateInspector("linux", newPackageManagerLease())
			snapshot := inspector.Snapshot(context.Background(), true)
			if !snapshot.Supported || snapshot.Error == "" || snapshot.InventoryHash != "" || snapshot.PendingCount != 0 || len(snapshot.Packages) != 0 {
				t.Fatalf("failed read produced a known inventory: %+v", snapshot)
			}
		})
	}
}

func TestPackageInspectionCanceledBeforeResolution(t *testing.T) {
	old := packageInspectionResolveAPT
	packageInspectionResolveAPT = func() (string, error) { t.Fatal("canceled read resolved executable"); return "", nil }
	t.Cleanup(func() { packageInspectionResolveAPT = old })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := runPackageInspectionCommand(ctx)
	if !errors.Is(result.err, context.Canceled) || result.exitCode != -1 {
		t.Fatalf("canceled result = %+v", result)
	}
}

func TestPackageInspectionDeadlineStopsProcessGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	withPackageInspectionExecutable(t, fmt.Sprintf("sleep 300 &\nprintf '%%s' \"$!\" > %q\nwait\n", pidFile))
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	result := runPackageInspectionCommand(ctx)
	if !errors.Is(result.err, context.DeadlineExceeded) || result.exitCode == 0 || time.Since(start) > 3*time.Second {
		t.Fatalf("deadline not retained/bounded: %+v (%s)", result, time.Since(start))
	}
	waitForProcessGone(t, readPidFile(t, pidFile), 3*time.Second)
}

func TestPackageInspectionPipeWaitIsNotSuccessfulInventory(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	// The deliberately escaped fixture has a finite lifetime and no work. It
	// witnesses bounded pipe waiting, not successful descendant containment.
	withPackageInspectionExecutable(t, fmt.Sprintf("sleep 2 &\nprintf '%%s' \"$!\" > %q\nprintf '%%b' %q\nexit 0\n", pidFile, inspectionFixtureUpdates))
	inspector := newPackageUpdateInspector("linux", newPackageManagerLease())
	snapshot := inspector.Snapshot(context.Background(), true)
	if snapshot.Error == "" || snapshot.InventoryHash != "" || len(snapshot.Packages) != 0 {
		t.Fatalf("inherited pipe turned partial completion into inventory: %+v", snapshot)
	}
	waitForProcessGone(t, readPidFile(t, pidFile), 3*time.Second)
}

func TestCollectorPackageInspectionDoesNotReachCommandRemediation(t *testing.T) {
	withPackageInspectionExecutable(t, fmt.Sprintf("printf '%%b' %q\n", inspectionFixtureUpdates))
	collector := &mockCollector{hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
		return &gohost.InfoStat{Hostname: "apt-host", Platform: "debian", HostID: "fixture-id", KernelArch: "amd64"}, nil
	}}
	agent, err := New(Config{APIToken: "fixture-token", Collector: collector, EnableCommands: true, StateDir: t.TempDir(), LogLevel: zerolog.Disabled})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := agent.packageUpdates.(*packageUpdateInspector); !ok || agent.cfg.packageUpdates != nil {
		t.Fatalf("report/command managers were not separated: %T / %p", agent.packageUpdates, agent.cfg.packageUpdates)
	}
	client := NewCommandClient(agent.cfg, "fixture-agent", "apt-host", "linux", "test")
	if client.operationReceipts != nil {
		t.Cleanup(func() { _ = client.operationReceipts.Close() })
	}
	if client.packageUpdates != nil {
		t.Fatal("collector command client received a package remediation manager")
	}
	if feasible, reason := client.preflightHostUpdate(context.Background(), agentexec.HostUpdatePayload{}); feasible || reason != agentexec.ActionRefusalCapabilityUnavailable {
		t.Fatalf("collector package action became feasible: %v %q", feasible, reason)
	}
	runner := NewActionRunnerClient(ActionRunnerClientConfig{StateDir: t.TempDir()}, "fixture-agent", "apt-host", "test")
	if runner.operationReceipts != nil {
		t.Cleanup(func() { _ = runner.operationReceipts.Close() })
	}
	if runner.packageUpdates == nil || !runner.actionRunnerOnly {
		t.Fatal("separate runner lost its package remediation manager")
	}
}

func TestPackageInspectionDoesNotChangeRunnerPreflightOrApply(t *testing.T) {
	old := packageInspectionResolveAPT
	packageInspectionResolveAPT = func() (string, error) { t.Fatal("runner entered collector inspector"); return "", nil }
	t.Cleanup(func() { packageInspectionResolveAPT = old })
	m := newPackageUpdateManager("linux", newPackageManagerLease())
	m.lookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	m.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	installed := false
	var calls []string
	m.run = func(_ context.Context, _ []string, name string, args ...string) packageUpdateCommandResult {
		invocation := name + " " + strings.Join(args, " ")
		calls = append(calls, invocation)
		switch invocation {
		case "apt-get -s -o Debug::NoLocking=1 upgrade":
			if !installed {
				return packageUpdateCommandResult{stdout: inspectionFixtureUpdates}
			}
		case "apt-get -y --no-remove -o Dpkg::Options::=--force-confold upgrade":
			installed = true
		case "apt-get update", "dpkg --audit":
		default:
			t.Fatalf("unapproved runner invocation %q", invocation)
		}
		return packageUpdateCommandResult{}
	}
	request := agentexec.HostUpdatePayload{RequestID: "r", ActionID: "a", Operation: agentexec.HostUpdateOperationInstall, ExpectedInventoryHash: aptUpgradeInventoryHash(inspectionFixtureUpdates)}
	if err := agentexec.BindHostUpdatePayload(&request); err != nil {
		t.Fatal(err)
	}
	if allowed, reason := m.Preflight(context.Background(), request); !allowed || reason != "" {
		t.Fatalf("runner preflight=%v reason=%q", allowed, reason)
	}
	result := m.Apply(context.Background(), request)
	if !result.Success || result.Verification != agentexec.HostUpdateVerificationVerified || !installed || result.After.PendingCount != 0 {
		t.Fatalf("runner result=%+v", result)
	}
	want := []string{"apt-get -s -o Debug::NoLocking=1 upgrade", "dpkg --audit", "apt-get -s -o Debug::NoLocking=1 upgrade", "apt-get update", "apt-get -s -o Debug::NoLocking=1 upgrade", "dpkg --audit", "apt-get -y --no-remove -o Dpkg::Options::=--force-confold upgrade", "apt-get -s -o Debug::NoLocking=1 upgrade", "dpkg --audit"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("runner calls=%q, want %q", calls, want)
	}
}

func TestContainedPackageInspectionRejectsTruncatedInventory(t *testing.T) {
	for _, stream := range []string{"stdout", "stderr"} {
		t.Run(stream, func(t *testing.T) {
			withTypedActionLinuxHooks(t)
			dir := t.TempDir()
			t.Setenv("PULSE_AGENT_RUNNER_STATE_DIR", dir)
			fakeRun := filepath.Join(dir, "systemd-run")
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
			if stream == "stderr" {
				script += "head -c 1048577 /dev/zero >&2\n"
			} else {
				script += "head -c 1048577 /dev/zero\n"
			}
			if err := os.WriteFile(fakeRun, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			typedActionResolveSystemdTool = func(string) (string, error) { return fakeRun, nil }
			typedActionCurrentExecutable = func() (string, error) { return fakeRun, nil }
			typedActionQueryUnitLoadState = func(string, string) (string, error) { return "not-found", nil }
			m := newPackageUpdateManager("linux", newPackageManagerLease())
			m.lookPath = func(string) (string, error) { return "/usr/bin/apt-get", nil }
			snapshot := m.Snapshot(context.Background(), true)
			if snapshot.Error == "" || snapshot.InventoryHash != "" || snapshot.PendingCount != 0 || len(snapshot.Packages) != 0 {
				t.Fatalf("contained simulation published truncated %s as inventory: %+v", stream, snapshot)
			}
		})
	}
}
