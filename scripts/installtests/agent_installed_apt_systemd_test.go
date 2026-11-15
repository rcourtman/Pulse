//go:build linux

package installtests

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

const nativeAPTDropIn = "/etc/systemd/system/pulse-agent.service.d/native-apt-fixture.conf"

func nativeProfileScenario(value string) (string, error) {
	switch value {
	case "", "recovery":
		return "recovery", nil
	case "collector-only-apt":
		return value, nil
	default:
		return "", errors.New("unknown native installed-profile scenario")
	}
}

type nativeAPTInventory struct {
	Count    int                        `json:"pending_count"`
	Hash     string                     `json:"inventory_hash"`
	Packages []agentshost.PackageUpdate `json:"packages"`
}

// Independent expectation from the actual APT simulation, not a call to the
// collector's parser. Inventory identity covers every Inst line even when the
// report's display list is bounded to 200 entries.
func nativeAPTParse(output string) (nativeAPTInventory, error) {
	var inventory nativeAPTInventory
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Inst ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return inventory, errors.New("reference APT inventory is incomplete or has an unexpected format")
		}
		installed, availableAt := "", 2
		if strings.HasPrefix(fields[2], "[") {
			if !strings.HasSuffix(fields[2], "]") {
				return inventory, errors.New("reference installed version is incomplete")
			}
			installed, availableAt = strings.Trim(fields[2], "[]"), 3
		}
		if len(fields) <= availableAt || !strings.HasPrefix(fields[availableAt], "(") || !strings.HasSuffix(line, ")") {
			return inventory, errors.New("reference available version is incomplete")
		}
		lines = append(lines, line)
		if len(inventory.Packages) < 200 {
			inventory.Packages = append(inventory.Packages, agentshost.PackageUpdate{
				Name: fields[1], InstalledVersion: installed, AvailableVersion: strings.Trim(fields[availableAt], "()"),
			})
		}
	}
	inventory.Count = len(lines)
	sort.Strings(lines)
	inventory.Hash = "sha256:" + secureRuntimeHash([]byte(strings.Join(lines, "\n")))
	return inventory, nil
}

func nativeAPTReportMatches(report secureRuntimeLabReport, want nativeAPTInventory, version string, after time.Time, safe bool) bool {
	status := report.PackageUpdates
	if status == nil || !report.ReceivedAt.After(after) || !status.CheckedAt.After(after) || status.CheckedAt.After(report.ReceivedAt) ||
		report.AgentID != secureRuntimeLabAgentID || report.Hostname != secureRuntimeLabHostname || report.AgentVersion != version ||
		report.CommandsEnabled || report.Authority != "monitoring-only" || report.RunningAsRoot == safe ||
		(safe && (report.ServiceUser != "pulse-agent" || !report.TypedHelper)) ||
		!status.Supported || status.Manager != "apt" || status.Error != "" || status.PendingCount != want.Count || status.InventoryHash != want.Hash {
		return false
	}
	// nil and an empty wire list both describe no packages.
	return len(status.Packages) == len(want.Packages) && (len(want.Packages) == 0 || reflect.DeepEqual(status.Packages, want.Packages))
}

// No package payload is ever installed or downloaded. A real APT resolver reads
// this tiny, code-owned index and status file. The native service sees these
// bytes only through read-only mount bindings; the host's package state is not
// replaced. The closed loopback source is never contacted by a simulation.
func nativeAPTFixture(t *testing.T, root string) {
	t.Helper()
	arch := runtime.GOARCH
	if arch != "amd64" && arch != "arm64" {
		t.Fatalf("unsupported native APT fixture architecture %s", arch)
	}
	for _, dir := range []string{"etc/apt/apt.conf.d", "etc/apt/sources.list.d", "lists/partial"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var status, packages strings.Builder
	for _, name := range []string{"pulse-native-apt-one", "pulse-native-apt-two"} {
		fmt.Fprintf(&status, "Package: %s\nStatus: install ok installed\nArchitecture: %s\nVersion: 1.0\nDescription: disposable native fixture\n\n", name, arch)
		fmt.Fprintf(&packages, "Package: %s\nArchitecture: %s\nVersion: 2.0\nFilename: pool/%s_2.0_%s.deb\nSize: 1\nSHA256: %s\nDescription: disposable native fixture\n\n", name, arch, name, arch, strings.Repeat("0", 64))
	}
	files := map[string]string{
		"status": status.String(), "extended_states": "",
		"etc/apt/sources.list": "deb [trusted=yes] http://127.0.0.1:9 native main\n",
		"etc/apt/apt.conf":     "Dir::Cache::pkgcache \"\";\nDir::Cache::srcpkgcache \"\";\nDir::State::extended_states \"/dev/null\";\nAcquire::Languages \"none\";\n",
		"lists/127.0.0.1:9_dists_native_main_binary-" + arch + "_Packages": packages.String(),
	}
	for path, data := range files {
		if err := os.WriteFile(filepath.Join(root, path), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Extra directory flags belong only to the independent reference. The real
// installed collector still executes its unchanged fixed invocation, ignores
// APT_CONFIG and receives no test hook, fake executable or parser injection.
func nativeAPTReferenceArgs(root string) []string {
	args := []string{"-s", "-o", "Debug::NoLocking=1"}
	if root != "" {
		args = append(args, "-o", "Dir::Etc="+filepath.Join(root, "etc/apt"),
			"-o", "Dir::State::status="+filepath.Join(root, "status"),
			"-o", "Dir::State::lists="+filepath.Join(root, "lists"),
			"-o", "Dir::State::extended_states="+filepath.Join(root, "extended_states"),
			"-o", "Dir::Cache::pkgcache=", "-o", "Dir::Cache::srcpkgcache=")
	}
	return append(args, "upgrade")
}

func nativeAPTReference(t *testing.T, receipt *nativeProfileReceipt, root, user string) nativeAPTInventory {
	t.Helper()
	args := append([]string{"-i", "PATH=/usr/sbin:/usr/bin:/sbin:/bin", "LC_ALL=C", "/usr/bin/apt-get"}, nativeAPTReferenceArgs(root)...)
	executable := "env"
	if user != "" {
		executable, args = "runuser", append([]string{"-u", user, "--", "env"}, args...)
	}
	output := nativeProfileMust(t, receipt, executable, args...)
	inventory, err := nativeAPTParse(string(output))
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}

func nativeAPTMachineState(t *testing.T) map[string]secureRuntimeStableIdentity {
	t.Helper()
	lists, err := nativeAPTSnapshotTree("/var/lib/apt/lists")
	if err != nil {
		t.Fatal(err)
	}
	return map[string]secureRuntimeStableIdentity{
		"dpkg_status":  nativeProfileFiles(t, "/var/lib/dpkg/status"),
		"dpkg_updates": nativeProfileTree(t, "/var/lib/dpkg/updates"),
		"apt_lists":    lists,
		"apt_state":    nativeProfileOptionalFiles(t, "/var/lib/apt/extended_states"),
	}
}

// APT's auxfiles can contain symlinks on hosted images. Observe those entries'
// literal link bytes and own metadata, never their referents. Keep the strict
// nativeProfileTree used for saved identity and recovery files unchanged.
// Directories (including the root), complete file bytes and full type/mode,
// UID and GID all participate in the before/after comparison. This binds the
// index tree itself; a link does not extend the observation to another tree.
func nativeAPTSnapshotTree(path string) (secureRuntimeStableIdentity, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("APT snapshot root is not a directory")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("APT snapshot root changed while opening")
	}
	result := make(secureRuntimeStableIdentity)
	err = fs.WalkDir(root.FS(), ".", func(name string, _ fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return errors.New("APT snapshot ownership is unavailable")
		}
		identity := secureRuntimeFileIdentity{Present: true, Mode: info.Mode(), UID: stat.Uid, GID: stat.Gid}
		switch {
		case info.Mode().IsRegular():
			// Refuse a substituted link or special node, rather than following
			// it or blocking on it. Root confines ancestor resolution too.
			file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
			if err != nil {
				return err
			}
			opened, err := file.Stat()
			if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
				file.Close()
				return errors.New("APT snapshot file changed while opening")
			}
			hash := sha256.New()
			_, readErr := io.Copy(hash, file)
			if err := errors.Join(readErr, file.Close()); err != nil {
				return err
			}
			identity.Hash = fmt.Sprintf("%x", hash.Sum(nil))
		case info.Mode()&os.ModeSymlink != 0:
			target, err := root.Readlink(name)
			if err != nil {
				return err
			}
			identity.Hash = secureRuntimeHash([]byte(target))
		case info.IsDir():
		default:
			return fmt.Errorf("APT snapshot contains an unsupported node: %s", name)
		}
		result[name] = identity
		return nil
	})
	if err != nil {
		return nil, err // A partial inventory cannot establish unchanged state.
	}
	return result, nil
}

func nativeAPTNoRunner(t *testing.T, receipt *nativeProfileReceipt) {
	t.Helper()
	load := strings.TrimSpace(string(nativeProfileMust(t, receipt, "systemctl", "show", "pulse-agent-runner.service", "--property=LoadState", "--value")))
	if load != "not-found" {
		t.Fatal("collector-only acceptance found an action runner unit")
	}
	for _, path := range []string{"/etc/pulse-agent-runner", "/var/lib/pulse-agent-runner", "/usr/local/lib/pulse-agent/pulse-agent-runner"} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("collector-only acceptance found runner files or could not establish absence")
		}
	}
	pid := secureRuntimeCollectorMainPID(t)
	if secureRuntimeProcessExecutableHash(t, pid) != receipt.Artifacts["collector-v1"] ||
		nativeProfileFiles(t, "/usr/local/bin/pulse-agent")["/usr/local/bin/pulse-agent"].Hash != receipt.Artifacts["collector-v1"] {
		t.Fatal("APT report is not from the exact installed source-built collector")
	}
	for _, item := range strings.Split(string(secureRuntimeReadFile(t, fmt.Sprintf("/proc/%d/environ", pid))), "\x00") {
		if strings.HasPrefix(item, "PULSE_AGENT_RUNNER_") || strings.HasPrefix(item, "APT_CONFIG=") {
			t.Fatal("installed collector inherited runner state or an APT configuration override")
		}
	}
	if secureRuntimeCollectorHasArgument("--enable-commands") || secureRuntimeCollectorHasArgument("--insecure") {
		t.Fatal("collector-only acceptance gained command authority or disabled TLS verification")
	}
}

func nativeProfileAPT(t *testing.T, receipt *nativeProfileReceipt, fixture *secureRuntimeLabFixture, ca, version string,
	install func(string, ...string) ([]byte, error), phase func(string, any)) {
	t.Helper()
	hostBefore := nativeAPTMachineState(t)
	phase("host-package-state-before", hostBefore)
	live := nativeAPTReference(t, receipt, "", "")
	waitReport := func(want nativeAPTInventory, after time.Time, safe bool) secureRuntimeLabReport {
		deadline := time.Now().Add(45 * time.Second)
		for time.Now().Before(deadline) {
			reports, _, _, _ := fixture.snapshot()
			for _, report := range reports {
				if nativeAPTReportMatches(report, want, version, after, safe) {
					return report
				}
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Fatal("no fresh authenticated no-runner report matching the complete reference APT inventory and profile")
		return secureRuntimeLabReport{}
	}
	started := time.Now()
	if output, err := install(ca); err != nil {
		t.Fatalf("default monitoring-only install: %v\n%s", err, output)
	}
	report := waitReport(live, started, false)
	secureRuntimeAssertRootMonitoringProfile(t)
	nativeAPTNoRunner(t, receipt)
	if !reflect.DeepEqual(live, nativeAPTReference(t, receipt, "", "")) {
		t.Fatal("live APT reference changed across the installed report")
	}
	phase("root-no-runner-live-apt", map[string]any{"reference": live, "authenticated_report": report})

	root := filepath.Join(nativeProfileRoot, "apt-fixture")
	nativeAPTFixture(t, root)
	fixtureBefore := nativeProfileTree(t, root)
	want := nativeAPTReference(t, receipt, root, "")
	if want.Count != 2 {
		t.Fatal("native APT fixture did not resolve both pending upgrades")
	}
	// No profile transition runs with an override. The existing safe-profile
	// verifier deliberately refuses drop-ins, and remains unchanged. Each
	// positive experiment adds only read-only fixture mounts, then removes them
	// and re-verifies the unmodified installed profile on the live inventory.
	t.Cleanup(func() {
		for _, path := range []string{nativeAPTDropIn, filepath.Dir(nativeAPTDropIn)} {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Error(err)
			}
		}
	})
	positive := func(safe bool) {
		if err := os.Mkdir(filepath.Dir(nativeAPTDropIn), 0o755); err != nil {
			t.Fatal(err)
		}
		dropIn := "[Service]\n"
		for _, pair := range [][2]string{{"etc/apt", "/etc/apt"}, {"status", "/var/lib/dpkg/status"}, {"lists", "/var/lib/apt/lists"}} {
			dropIn += "BindReadOnlyPaths=" + filepath.Join(root, pair[0]) + ":" + pair[1] + "\n"
		}
		if err := os.WriteFile(nativeAPTDropIn, []byte(dropIn), 0o644); err != nil {
			t.Fatal(err)
		}
		nativeProfileMust(t, receipt, "systemctl", "daemon-reload")
		started := time.Now()
		nativeProfileMust(t, receipt, "systemctl", "restart", "pulse-agent.service")
		report := waitReport(want, started, safe)
		nativeAPTNoRunner(t, receipt)
		if got := strings.TrimSpace(string(nativeProfileMust(t, receipt, "systemctl", "show", "pulse-agent.service", "--property=DropInPaths", "--value"))); got != nativeAPTDropIn {
			t.Fatal("positive APT experiment has an unexpected service override")
		}
		pid := secureRuntimeCollectorMainPID(t)
		mounts := string(secureRuntimeReadFile(t, fmt.Sprintf("/proc/%d/mountinfo", pid)))
		for _, target := range []string{"/etc/apt", "/var/lib/dpkg/status", "/var/lib/apt/lists"} {
			readonly := false
			for _, line := range strings.Split(mounts, "\n") {
				fields := strings.Fields(line)
				if len(fields) > 5 && fields[4] == target && strings.Contains(","+fields[5]+",", ",ro,") {
					readonly = true
				}
			}
			if !readonly {
				t.Fatal("positive APT fixture mount is not read-only")
			}
		}
		if secureRuntimeHash(secureRuntimeReadFile(t, fmt.Sprintf("/proc/%d/root/var/lib/dpkg/status", pid))) != fixtureBefore["/status"].Hash {
			t.Fatal("installed collector did not receive the code-owned package status")
		}
		phase("positive-apt-safe-"+strconv.FormatBool(safe), map[string]any{"reference": want, "authenticated_report": report,
			"readonly_fixture": fixtureBefore, "readonly_mounts_verified": true, "drop_in_sha256": secureRuntimeHash([]byte(dropIn)),
			"scope": "installed executable/profile plus temporary read-only package fixture bindings; not an unmodified live package index"})
		for _, path := range []string{nativeAPTDropIn, filepath.Dir(nativeAPTDropIn)} {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}
		nativeProfileMust(t, receipt, "systemctl", "daemon-reload")
		started = time.Now()
		nativeProfileMust(t, receipt, "systemctl", "restart", "pulse-agent.service")
		liveReport := waitReport(live, started, safe)
		if safe {
			secureRuntimeAssertSafeProfile(t)
		} else {
			secureRuntimeAssertRootMonitoringProfile(t)
		}
		nativeAPTNoRunner(t, receipt)
		phase("live-apt-restored-safe-"+strconv.FormatBool(safe), map[string]any{"reference": live, "authenticated_report": liveReport, "fixture_override_absent": true})
	}
	positive(false)
	historyPath := "/var/lib/pulse-agent/native-history"
	if err := os.WriteFile(historyPath, []byte("apt-profile-history-must-survive\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	statePaths := []string{ca, "/var/lib/pulse-agent/agent-id", "/var/lib/pulse-agent/connection.env", "/var/lib/pulse-agent/token"}
	state := nativeProfileFiles(t, statePaths...)
	history := nativeProfileFiles(t, historyPath)[historyPath]
	stateRoot := nativeProfileDirectory(t, "/var/lib/pulse-agent")
	lifecyclePaths := []string{"/etc/pulse-agent/connection.env", "/etc/pulse-agent/install.sh", "/etc/pulse-agent/install.sh.sha256"}
	lifecycle := nativeProfileOptionalFiles(t, lifecyclePaths...)
	started = time.Now()
	if output, err := install(ca, "--safe-profile-apply"); err != nil {
		t.Fatalf("no-runner safe profile: %v\n%s", err, output)
	}
	report = waitReport(live, started, true)
	secureRuntimeAssertSafeProfile(t)
	nativeAPTNoRunner(t, receipt)
	if !reflect.DeepEqual(want, nativeAPTReference(t, receipt, root, "pulse-agent")) ||
		!reflect.DeepEqual(live, nativeAPTReference(t, receipt, "", "pulse-agent")) {
		t.Fatal("root and unprivileged APT reference inventories differ")
	}
	uid, err := strconv.ParseUint(strings.TrimSpace(string(nativeProfileMust(t, receipt, "id", "-u", "pulse-agent"))), 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	gid, err := strconv.ParseUint(strings.TrimSpace(string(nativeProfileMust(t, receipt, "id", "-g", "pulse-agent"))), 10, 32)
	if err != nil || !nativeProfileHistoryHandedOff(history, nativeProfileFiles(t, historyPath)[historyPath], uint32(uid), uint32(gid)) {
		t.Fatal("safe profile did not preserve history bytes/mode and collector ownership")
	}
	phase("unprivileged-no-runner-live-apt", map[string]any{"reference": live, "authenticated_report": report, "history_preserved": true})
	positive(true)
	started = time.Now()
	nativeProfileMust(t, receipt, "bash", "/etc/pulse-agent/install.sh", "--safe-profile-rollback")
	report = waitReport(live, started, false)
	secureRuntimeAssertRootMonitoringProfile(t)
	nativeAPTNoRunner(t, receipt)
	if !reflect.DeepEqual(state, nativeProfileFiles(t, statePaths...)) || !reflect.DeepEqual(lifecycle, nativeProfileOptionalFiles(t, lifecyclePaths...)) ||
		stateRoot != nativeProfileDirectory(t, "/var/lib/pulse-agent") ||
		!nativeProfileHistoryHandedOff(history, nativeProfileFiles(t, historyPath)[historyPath], uint32(uid), uint32(gid)) {
		t.Fatal("no-runner rollback changed saved identity/CA/token/lifecycle/history or state-root metadata")
	}
	hostAfter := nativeAPTMachineState(t)
	if !reflect.DeepEqual(hostBefore, hostAfter) || !reflect.DeepEqual(fixtureBefore, nativeProfileTree(t, root)) {
		t.Fatal("APT inspection changed package database/index bytes or metadata")
	}
	phase("root-no-runner-restored", map[string]any{"authenticated_report": report, "state": state, "history_preserved": true,
		"host_package_state_unchanged": true, "host_package_state_after": hostAfter,
		"fixture_unchanged": true, "state_root": stateRoot, "lifecycle": lifecycle})
	receipt.Passed = true
}
