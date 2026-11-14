package installtests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A caller's discovery can become stale before it takes the installation lock.
// Exercise the real entry and executable parser with private shell binaries and
// a real flock. The service-state sentinel ends admitted cases before mutation;
// refused/skipped cases must not even reach it, temporary files or transport.
func TestAutoUpdateRevalidatesInstalledIdentityUnderLock(t *testing.T) {
	for _, tool := range []string{"bash", "flock", "timeout", "python3"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("locked updater admission requires %s: %v", tool, err)
		}
	}
	updater := os.Getenv("PULSE_AUTO_UPDATE_IDENTITY_SOURCE")
	if updater == "" {
		updater = repoFile("scripts", "pulse-auto-update.sh")
	}
	updater, err := filepath.Abs(updater)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, identity, layout string
		probeExit, status      int
		admitted, noProbe      bool
	}{
		{name: "older_community", identity: "Pulse v6.7.0", status: 1, admitted: true},
		{name: "same_release_preview", identity: "Pulse v6.8.0-rc.2", status: 1, admitted: true},
		{name: "already_target", identity: "Pulse v6.8.0", status: 3},
		{name: "newer_stable", identity: "Pulse v6.9.0", status: 3},
		{name: "newer_preview", identity: "Pulse v6.9.0-rc.1", status: 3},
		{name: "newly_pro", identity: "Pulse Pro v6.9.0", status: 1},
		{name: "unknown_identity", identity: "unidentified fixture", status: 1},
		{name: "failed_probe", identity: "Pulse v6.7.0", probeExit: 19, status: 1},
		{name: "missing_binary", layout: "missing", status: 1, noProbe: true},
		{name: "broken_primary_does_not_borrow_legacy", identity: "Pulse v6.7.0", layout: "both", probeExit: 19, status: 1},
		{name: "unusable_primary_does_not_borrow_legacy", identity: "Pulse v6.7.0", layout: "unusable", status: 1, noProbe: true},
		{name: "newer_legacy", identity: "Pulse v6.9.0", layout: "legacy", status: 3},
		{name: "older_legacy", identity: "Pulse v6.7.0", layout: "legacy", status: 1, admitted: true},
		{name: "busy_lock_precedes_identity_probe", identity: "Pulse v6.7.0", layout: "busy", status: 1, noProbe: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			install := filepath.Join(dir, "install")
			config := filepath.Join(dir, "config")
			for _, path := range []string{filepath.Join(install, "bin"), config} {
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			// Each real --version execution independently attempts this lock.
			// Its failure must be contention, not an unavailable flock command.
			binary := []byte(`#!/bin/bash
set -eu
[[ "$*" == --version ]] || exit 97
status=0
flock --exclusive --nonblock "$PULSE_INSTALL_DIR" true || status=$?
printf '%s\n' "$status" >> "$FIXTURE/probes"
python3 - "$PULSE_INSTALL_DIR" <<'PY'
import os
import sys
target = os.stat(sys.argv[1])
for name in os.listdir('/dev/fd'):
    try:
        entry = os.fstat(int(name))
    except OSError:
        continue
    if (entry.st_dev, entry.st_ino) == (target.st_dev, target.st_ino):
        sys.exit('version probe inherited the transaction descriptor')
PY
printf '%s\n' "$FIXTURE_IDENTITY"
exit "$FIXTURE_PROBE_EXIT"
`)
			primary := filepath.Join(install, "bin", "pulse")
			if tc.layout == "legacy" {
				primary = filepath.Join(install, "pulse")
			}
			if tc.layout != "missing" {
				if err := os.WriteFile(primary, binary, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if tc.layout == "both" || tc.layout == "unusable" {
				if err := os.WriteFile(filepath.Join(install, "pulse"), []byte("#!/bin/sh\necho legacy >> \"$FIXTURE/unexpected\"\necho 'Pulse v6.7.0'\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if tc.layout == "unusable" {
				if err := os.Chmod(primary, 0600); err != nil {
					t.Fatal(err)
				}
			}
			// A stale sidecar is not an edition/version admission source. Keep
			// both binary layouts, settings and identity byte-for-byte intact.
			for name, data := range map[string]string{
				filepath.Join(install, "VERSION"):    "v6.7.0\n",
				filepath.Join(config, "system.json"): "{\"autoUpdateEnabled\":true}\n",
				filepath.Join(config, "agent-id"):    "saved-fixture-identity\n",
			} {
				if err := os.WriteFile(name, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before := make(map[string]string)
			if err := filepath.WalkDir(install, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil || entry.IsDir() {
					return walkErr
				}
				data, readErr := os.ReadFile(path)
				before[path] = string(data)
				return readErr
			}); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"system.json", "agent-id"} {
				path := filepath.Join(config, name)
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				before[path] = string(data)
			}
			script := `source "$UPDATER_UNDER_TEST"
log() { printf '[%s] %s\n' "$1" "${*:2}"; }
detect_service_name() { echo pulse; }
read_update_service_state() { echo service >> "$FIXTURE/events"; return 67; }
mktemp() { echo temporary >> "$FIXTURE/unexpected"; return 71; }
curl() { echo transport >> "$FIXTURE/unexpected"; return 72; }
systemctl() { echo manager >> "$FIXTURE/unexpected"; return 73; }
if [[ "$FIXTURE_LAYOUT" == busy ]]; then
    exec {held}< "$INSTALL_DIR"
    flock --exclusive --nonblock "$held"
fi
status=0
perform_update v6.8.0 || status=$?
printf 'RESULT=%s\n' "$status"
[[ -z "$(trap -p RETURN)" ]] || { echo 'leaked RETURN trap'; exit 95; }
if [[ "$FIXTURE_LAYOUT" == busy ]]; then exec {held}<&-; fi
flock --exclusive --nonblock "$INSTALL_DIR" true || { echo 'leaked update lock'; exit 96; }
`
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", "-c", script)
			cmd.Env = append(os.Environ(), "UPDATER_UNDER_TEST="+updater,
				"FIXTURE="+dir, "PULSE_INSTALL_DIR="+install, "PULSE_CONFIG_DIR="+config,
				"FIXTURE_IDENTITY="+tc.identity, fmt.Sprintf("FIXTURE_PROBE_EXIT=%d", tc.probeExit), "FIXTURE_LAYOUT="+tc.layout)
			out, err := cmd.CombinedOutput()
			if err != nil || ctx.Err() != nil || !strings.Contains(string(out), fmt.Sprintf("RESULT=%d\n", tc.status)) {
				t.Fatalf("locked admission: command=%v context=%v want status=%d\n%s", err, ctx.Err(), tc.status, out)
			}
			probes, readErr := os.ReadFile(filepath.Join(dir, "probes"))
			if tc.noProbe {
				if !os.IsNotExist(readErr) {
					t.Fatalf("identity unexpectedly executed: %q, %v", probes, readErr)
				}
			} else if readErr != nil || string(probes) != "1\n" {
				t.Fatalf("identity was not read exactly once under the held lock: %q, %v\n%s", probes, readErr, out)
			}
			events, readErr := os.ReadFile(filepath.Join(dir, "events"))
			if tc.admitted {
				if readErr != nil || string(events) != "service\n" {
					t.Fatalf("older community did not reach the unchanged transaction admission: %q, %v\n%s", events, readErr, out)
				}
			} else if !os.IsNotExist(readErr) {
				t.Fatalf("stale or unknown admission reached transaction state: %q, %v\n%s", events, readErr, out)
			}
			if data, readErr := os.ReadFile(filepath.Join(dir, "unexpected")); !os.IsNotExist(readErr) {
				t.Fatalf("refusal/sentinel made an unexpected call: %q, %v", data, readErr)
			}
			for path, want := range before {
				data, err := os.ReadFile(path)
				if err != nil || string(data) != want {
					t.Fatalf("saved file changed at %s: %v", filepath.Base(path), err)
				}
			}
		})
	}
}

// Consume the transaction's distinct no-change result through real main. An
// earlier "new version available" observation is not an installed update.
func TestAutoUpdateMainRetainsLockedAdmissionOutcome(t *testing.T) {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		t.Fatal("updater main-flow proof requires a non-Docker environment")
	}
	updater := os.Getenv("PULSE_AUTO_UPDATE_IDENTITY_SOURCE")
	if updater == "" {
		updater = repoFile("scripts", "pulse-auto-update.sh")
	}
	updater, err := filepath.Abs(updater)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, result, message string
		success, installed    bool
	}{
		{name: "no_change", result: "3", message: "No update needed after locked installed-version check", success: true},
		{name: "refused", result: "1", message: "Update failed"},
		{name: "other_failure", result: "17", message: "Update failed"},
		{name: "installed", result: "0", message: "Update completed successfully to v6.8.0", success: true, installed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", "-c", `source "$UPDATER_UNDER_TEST"
log() { printf '[%s] %s\n' "$1" "${*:2}"; }
check_auto_updates_enabled() { :; }
installed_binary_is_pulse_pro() { return 1; }
get_current_version() { echo v6.7.0; }
get_latest_stable_version() { echo v6.8.0; }
perform_update() { echo "TRANSACTION:$*"; return "$FIXTURE_RESULT"; }
main
`)
			cmd.Env = append(os.Environ(), "UPDATER_UNDER_TEST="+updater, "FIXTURE_RESULT="+tc.result)
			out, err := cmd.CombinedOutput()
			if ctx.Err() != nil || (err == nil) != tc.success || !strings.Contains(string(out), tc.message) ||
				strings.Count(string(out), "TRANSACTION:v6.8.0\n") != 1 ||
				strings.Contains(string(out), "Update completed successfully to") != tc.installed ||
				strings.Contains(string(out), "Auto-update check completed") != tc.success {
				t.Fatalf("main lost locked admission outcome: command=%v context=%v\n%s", err, ctx.Err(), out)
			}
		})
	}
}

// Exercise the real main entry after a controlled installer result. Discovery,
// installation and logging are private doubles: no service, provider, journal
// or notification destination is contacted. The transaction tests separately
// establish what a successful perform_update result requires.
func TestAutoUpdateCompletionDoesNotCallAnUnregisteredNotificationRoute(t *testing.T) {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		t.Fatal("updater main-flow proof requires a non-Docker environment")
	}
	updater := os.Getenv("PULSE_AUTO_UPDATE_COMPLETION_SOURCE")
	if updater == "" {
		updater = repoFile("scripts", "pulse-auto-update.sh")
	}
	updater, err := filepath.Abs(updater)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, configFile string
		installerExit    string
		current          bool
	}{
		{name: "without_legacy_webhooks", installerExit: "0"},
		{name: "legacy_encrypted_webhooks", configFile: "webhooks.enc", installerExit: "0"},
		{name: "legacy_json_webhooks", configFile: "webhooks.json", installerExit: "0"},
		{name: "installer_failure", configFile: "webhooks.json", installerExit: "17"},
		{name: "already_current", configFile: "webhooks.enc", installerExit: "0", current: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.configFile != "" {
				if err := os.WriteFile(filepath.Join(dir, tc.configFile), []byte("synthetic legacy configuration\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			latest := "v6.6.1"
			if tc.current {
				latest = "v6.6.0"
			}
			script := `source "$UPDATER_UNDER_TEST"
log() { printf '[%s] %s\n' "$1" "${*:2}"; }
check_auto_updates_enabled() { :; }
installed_binary_is_pulse_pro() { return 1; }
get_current_version() { echo v6.6.0; }
get_latest_stable_version() { printf '%s\n' "$LATEST"; }
perform_update() {
    printf '%s\n' "$*" >> "$FIXTURE/installer-calls"
    return "$INSTALLER_EXIT"
}
curl() {
    printf '%s\n' "$*" >> "$FIXTURE/unexpected-network"
    return 0
}
main
`
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", "-c", script)
			cmd.Env = append(os.Environ(), "UPDATER_UNDER_TEST="+updater,
				"FIXTURE="+dir, "PULSE_CONFIG_DIR="+dir, "LATEST="+latest,
				"INSTALLER_EXIT="+tc.installerExit)
			out, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("main did not terminate: %v\n%s", ctx.Err(), out)
			}
			exit := 0
			if err != nil {
				failure, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatal(err)
				}
				exit = failure.ExitCode()
			}
			wantExit := 0
			if tc.installerExit != "0" {
				wantExit = 1
			}
			if exit != wantExit || strings.Contains(string(out), "Auto-update check completed") != (wantExit == 0) {
				t.Fatalf("completion exit=%d want=%d:\n%s", exit, wantExit, out)
			}
			calls, readErr := os.ReadFile(filepath.Join(dir, "installer-calls"))
			if tc.current {
				if !os.IsNotExist(readErr) || !strings.Contains(string(out), "Already running latest version") {
					t.Fatalf("current install was changed: calls=%q error=%v\n%s", calls, readErr, out)
				}
			} else if readErr != nil || string(calls) != latest+"\n" {
				t.Fatalf("lost or repeated exact installer handoff: calls=%q error=%v", calls, readErr)
			}
			if tc.installerExit == "0" && !tc.current && !strings.Contains(string(out), "Update completed successfully to "+latest) {
				t.Fatalf("lost verified update completion:\n%s", out)
			}
			if wantExit != 0 && (strings.Contains(string(out), "Update completed successfully") || !strings.Contains(string(out), "Update failed")) {
				t.Fatalf("failed installer was reported as successful:\n%s", out)
			}
			if calls, readErr := os.ReadFile(filepath.Join(dir, "unexpected-network")); !os.IsNotExist(readErr) {
				t.Fatalf("completion attempted an unsupported network request: %q (read=%v)", calls, readErr)
			}
			if tc.configFile != "" {
				bytes, err := os.ReadFile(filepath.Join(dir, tc.configFile))
				if err != nil || string(bytes) != "synthetic legacy configuration\n" {
					t.Fatalf("legacy notification configuration changed: %q, %v", bytes, err)
				}
			}
		})
	}
}
