//go:build linux

package hostagent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rs/zerolog"
)

// The collector's transient unit is the runner's unit with one difference:
// the service it is bound to. No sandbox property is added, dropped or
// changed, for the status read and for the start.
func TestCollectorTypedActionUnitDiffersFromTheRunnersOnlyInItsService(t *testing.T) {
	for _, tc := range []struct {
		catalog typedActionCatalog
		verb    string
	}{
		{typedActionCatalogProxmox, "status"},
		{typedActionCatalogProxmoxHandoff, "start"},
		{typedActionCatalogProbe, ""},
	} {
		name, args := "pct", []string{tc.verb, "112"}
		if tc.catalog == typedActionCatalogProbe {
			name, args = "true", nil
		}
		const unit, binary, result = "pulse-agent-collector-action-0123456789abcdef0123456789abcdef.service", "/usr/local/bin/pulse-agent", "/var/lib/pulse-agent/typed-actions/result.json"
		runner, err := typedActionSystemdRunArgs(unit, binary, result, nil, tc.catalog, name, args)
		if err != nil {
			t.Fatal(err)
		}
		collector, err := typedActionSystemdRunArgsFor(typedActionCollectorServiceUnit, unit, binary, result, nil, tc.catalog, name, args)
		if err != nil {
			t.Fatal(err)
		}
		if len(runner) != len(collector) {
			t.Fatalf("%s: the collector unit has %d arguments, the runner's has %d", tc.catalog, len(collector), len(runner))
		}
		bound := 0
		for index := range runner {
			if runner[index] == collector[index] {
				continue
			}
			want := strings.Replace(runner[index], typedActionRunnerServiceUnit, typedActionCollectorServiceUnit, 1)
			if collector[index] != want || !strings.HasPrefix(collector[index], "--property=") || !strings.HasSuffix(collector[index], "="+typedActionCollectorServiceUnit) {
				t.Fatalf("%s: argument %d differs beyond the bound service: runner %q, collector %q", tc.catalog, index, runner[index], collector[index])
			}
			bound++
		}
		if bound != 3 {
			t.Fatalf("%s: %d arguments name the service, want BindsTo, After and PartOf", tc.catalog, bound)
		}
		joined := strings.Join(collector, "\n")
		for _, required := range []string{"--property=BindsTo=pulse-agent.service", "--property=After=pulse-agent.service", "--property=PartOf=pulse-agent.service", "--property=KillMode=control-group", "--property=User=root"} {
			if !strings.Contains(joined, required) {
				t.Fatalf("%s: collector unit is missing %q:\n%s", tc.catalog, required, joined)
			}
		}
		if strings.Contains(joined, typedActionRunnerServiceUnit) {
			t.Fatalf("%s: collector unit still names the runner's service:\n%s", tc.catalog, joined)
		}
	}
	// The start keeps the runner's handoff shape, and the read keeps the
	// runner's full sandbox. Neither is the collector's to choose.
	read, _ := typedActionSystemdRunArgsFor(typedActionCollectorServiceUnit, "u.service", "/usr/local/bin/pulse-agent", "/r.json", nil, typedActionCatalogProxmox, "pct", []string{"status", "112"})
	if joined := strings.Join(read, "\n"); !strings.Contains(joined, "--property=NoNewPrivileges=yes") || !strings.Contains(joined, "--property=RestrictSUIDSGID=yes") {
		t.Fatalf("the collector's status read lost sandbox properties:\n%s", joined)
	}
	if _, err := typedActionSystemdRunArgsFor("ssh.service", "u.service", "/usr/local/bin/pulse-agent", "/r.json", nil, typedActionCatalogProxmox, "pct", []string{"status", "112"}); err == nil {
		t.Fatal("a unit was bound to a service that is not a Pulse agent service")
	}
}

func TestSystemdUnitFromCgroupNamesTheServiceTheProcessRunsAs(t *testing.T) {
	for content, want := range map[string]string{
		"0::/system.slice/pulse-agent.service\n": "pulse-agent.service",
		"12:pids:/system.slice/pulse-agent.service\n1:name=systemd:/system.slice/pulse-agent.service\n0::/system.slice/pulse-agent.service\n": "pulse-agent.service",
		"1:name=systemd:/system.slice/pulse-agent.service\n0::/\n":                                                                            "pulse-agent.service",
		"0::/user.slice/user-0.slice/session-3.scope\n":                                                                                       "session-3.scope",
		"0::/system.slice/pulse-agent-runner.service\n":                                                                                       "pulse-agent-runner.service",
		"0::/system.slice/pulse-agent.service/worker\n":                                                                                       "worker",
		"0::/\n":                            "/",
		"":                                  "",
		"garbage":                           "",
		"0::relative/pulse-agent.service\n": "",
	} {
		if got := systemdUnitFromCgroup(content); got != want {
			t.Fatalf("systemdUnitFromCgroup(%q) = %q, want %q", content, got, want)
		}
	}
}

func withCollectorTypedActionHooks(t *testing.T, proxmox bool, euid int, cgroup string, reconcile func(context.Context, typedActionSupervisor) error) {
	t.Helper()
	oldProxmox, oldEUID, oldCgroup, oldReconcile := collectorTypedActionHasProxmox, collectorTypedActionEUID, collectorTypedActionSelfCgroup, collectorTypedActionReconcile
	t.Cleanup(func() {
		collectorTypedActionHasProxmox, collectorTypedActionEUID, collectorTypedActionSelfCgroup, collectorTypedActionReconcile = oldProxmox, oldEUID, oldCgroup, oldReconcile
	})
	collectorTypedActionHasProxmox = func() bool { return proxmox }
	collectorTypedActionEUID = func() int { return euid }
	collectorTypedActionSelfCgroup = func() ([]byte, error) {
		if cgroup == "" {
			return nil, os.ErrNotExist
		}
		return []byte(cgroup), nil
	}
	collectorTypedActionReconcile = reconcile
}

// The collector offers guest start only on a Proxmox node, as root, running
// as its own systemd service, with the containment probe passed. Each other
// case has its own answer, and none of them reaches systemd.
func TestPrepareCollectorTypedActionsRequiresProxmoxRootOwnServiceAndProbe(t *testing.T) {
	const ownService = "0::/system.slice/pulse-agent.service\n"
	probed := 0
	pass := func(_ context.Context, supervisor typedActionSupervisor) error {
		probed++
		if supervisor.serviceUnit != typedActionCollectorServiceUnit || supervisor.unitPrefix != typedActionCollectorUnitPrefix || supervisor.allow == nil {
			t.Errorf("probe ran under %+v", supervisor)
		}
		return nil
	}
	supervisor := newCollectorTypedActions(t.TempDir(), zerolog.Nop()).supervisor()

	withCollectorTypedActionHooks(t, false, 0, ownService, pass)
	if reason, err := prepareCollectorTypedActions(context.Background(), supervisor); !errors.Is(err, errCollectorTypedActionsNotApplicable) || reason != "" || probed != 0 {
		t.Fatalf("not a Proxmox node: %q %v probed=%d", reason, err, probed)
	}
	withCollectorTypedActionHooks(t, true, 1000, ownService, pass)
	if reason, err := prepareCollectorTypedActions(context.Background(), supervisor); err == nil || reason != agentexec.ProxmoxGuestStartUnavailableContainment || probed != 0 {
		t.Fatalf("not root: %q %v probed=%d", reason, err, probed)
	}
	for _, cgroup := range []string{"0::/user.slice/user-0.slice/session-3.scope\n", "0::/system.slice/pulse-agent-runner.service\n", "0::/system.slice/pulse-agent.service/worker\n", ""} {
		withCollectorTypedActionHooks(t, true, 0, cgroup, pass)
		if reason, err := prepareCollectorTypedActions(context.Background(), supervisor); err == nil || reason != agentexec.ProxmoxGuestStartUnavailableNotSystemdService || probed != 0 {
			t.Fatalf("cgroup %q: %q %v probed=%d", cgroup, reason, err, probed)
		}
	}
	withCollectorTypedActionHooks(t, true, 0, ownService, func(context.Context, typedActionSupervisor) error { return errors.New("probe failed") })
	if reason, err := prepareCollectorTypedActions(context.Background(), supervisor); err == nil || reason != agentexec.ProxmoxGuestStartUnavailableContainment {
		t.Fatalf("failed probe: %q %v", reason, err)
	}
	withCollectorTypedActionHooks(t, true, 0, ownService, pass)
	if reason, err := prepareCollectorTypedActions(context.Background(), supervisor); err != nil || reason != "" || probed != 1 {
		t.Fatalf("ready collector: %q %v probed=%d", reason, err, probed)
	}
}

// The launcher the collector binary serves refuses a stop, a shutdown, a
// reboot and a package command before it resolves or starts anything, and it
// leaves no result behind. The same arguments pass the runner's launcher
// checks, so this is the collector's own narrowing.
func TestCollectorTypedActionLauncherRefusesEverythingButProbeStatusAndStart(t *testing.T) {
	withTypedActionLinuxHooks(t)
	resolved := 0
	typedActionResolveTarget = func(typedActionCatalog, string) (string, error) {
		resolved++
		return "", errors.New("the launcher resolved a target")
	}
	for _, refused := range [][]string{
		{string(typedActionCatalogProxmox), "pct", "stop", "112"},
		{string(typedActionCatalogProxmox), "pct", "shutdown", "112"},
		{string(typedActionCatalogProxmox), "qm", "reboot", "160"},
		{string(typedActionCatalogProxmoxHandoff), "pct", "reboot", "112"},
		{string(typedActionCatalogPackage), "apt-get", "update"},
		{string(typedActionCatalogPackage), "dpkg", "--audit"},
	} {
		resultPath := filepath.Join(t.TempDir(), "result.json")
		args := append([]string{"--catalog", refused[0], "--result-file", resultPath, "--"}, refused[1:]...)
		if _, _, _, _, err := parseTypedActionLauncherArgs(args); err != nil {
			t.Fatalf("%v is not in the runner's catalog either: %v", refused, err)
		}
		if exit := RunCollectorTypedActionLauncher(args); exit != typedActionLauncherSupervisorFailureExit {
			t.Fatalf("%v: collector launcher exit = %d, want a refusal", refused, exit)
		}
		if _, err := os.Lstat(resultPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%v: the refused launcher left a result file: %v", refused, err)
		}
	}
	if resolved != 0 {
		t.Fatalf("a refused launcher resolved %d targets", resolved)
	}
	// A start gets past the collector's catalog and on to target resolution.
	resultPath := filepath.Join(t.TempDir(), "result.json")
	if exit := RunCollectorTypedActionLauncher([]string{"--catalog", string(typedActionCatalogProxmoxHandoff), "--result-file", resultPath, "--", "pct", "start", "112"}); exit != typedActionLauncherStartFailureExit || resolved != 1 {
		t.Fatalf("start: exit = %d resolved = %d, want it to reach target resolution", exit, resolved)
	}
}

func TestCollectorAndRunnerTypedActionUnitsDoNotShareAPrefix(t *testing.T) {
	name, err := newTypedActionUnitName(typedActionCollectorUnitPrefix)
	if err != nil {
		t.Fatal(err)
	}
	if !validTypedActionUnitNameWithPrefix(typedActionCollectorUnitPrefix, name) || validTypedActionUnitName(name) {
		t.Fatalf("collector unit %q is not the collector's alone", name)
	}
	runnerName, err := newTypedActionUnitName(typedActionUnitPrefix)
	if err != nil {
		t.Fatal(err)
	}
	if !validTypedActionUnitName(runnerName) || validTypedActionUnitNameWithPrefix(typedActionCollectorUnitPrefix, runnerName) {
		t.Fatalf("runner unit %q could be reconciled by the collector", runnerName)
	}
	if strings.HasPrefix(typedActionCollectorUnitPrefix, typedActionUnitPrefix) || strings.HasPrefix(typedActionUnitPrefix, typedActionCollectorUnitPrefix) {
		t.Fatal("one supervisor's unit glob would match the other's units")
	}
}
