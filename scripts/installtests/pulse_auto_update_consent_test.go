package installtests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Keep these Bash-only checks runnable as a standalone file as well as part of
// the installer package. They need no application imports or local listeners.
// From this directory: go test -race pulse_auto_update_consent_test.go
func extractAutoUpdateConsentGate(t *testing.T) string {
	return extractAutoUpdateAdmissionFunction(t, "check_auto_updates_enabled")
}

func extractAutoUpdateAdmissionFunction(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "pulse-auto-update.sh"))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?ms)^` + regexp.QuoteMeta(name) + `\(\) \{\n.*?^\}`).Find(content)
	if match == nil {
		t.Fatalf("cannot find actual unattended-update function %s", name)
	}
	return string(match)
}

func TestAutoUpdateDiscoveryParserKeepsDiagnosticsOutOfVersion(t *testing.T) {
	updater, err := filepath.Abs(filepath.Join("..", "pulse-auto-update.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := `source "$UPDATER_UNDER_TEST"
# Keep the actual log function; a stderr-only log double concealed this fault.
logger() { :; }
command() { if [[ "$*" == '-v jq' ]]; then return 1; fi; builtin command "$@"; }
curl() { echo UNEXPECTED_TRANSPORT; return 97; }
get_latest_stable_version
`
	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), "UPDATER_UNDER_TEST="+updater)
	var diagnostic strings.Builder
	cmd.Stderr = &diagnostic
	out, err := cmd.Output()
	if err != nil || len(out) != 0 || !strings.Contains(diagnostic.String(), "jq is required to verify release metadata") {
		t.Fatalf("missing parser contaminated version output: %v; stdout=%q stderr=%q", err, out, diagnostic.String())
	}
}

func TestAutoUpdateDiscoveryFailuresCannotReportLatest(t *testing.T) {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		t.Fatal("discovery main-flow proof requires a non-Docker environment")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Fatal("jq is required for the real release-selector controls")
	}
	updater, err := filepath.Abs(filepath.Join("..", "pulse-auto-update.sh"))
	if err != nil {
		t.Fatal(err)
	}
	const stable = `[{"tag_name":"v6.6.0","draft":false,"prerelease":false}]`
	for _, tc := range []struct {
		name, document, selector string
		curlExit, exit, reads    int
		missingParser, install   bool
	}{
		{name: "missing parser", missingParser: true, exit: 1},
		{name: "failed transport with partial valid output", document: stable, curlExit: 22, exit: 1, reads: 2},
		{name: "malformed metadata", document: "[", exit: 1, reads: 2},
		{name: "unconfirmed maturity", document: `[{"tag_name":"v6.6.0"}]`, exit: 1, reads: 2},
		{name: "failed selector with valid tag", selector: `printf '%s\n' v6.6.0; return 23`, exit: 1},
		{name: "diagnostic instead of tag", selector: `printf '%s\n' 'release lookup unavailable'`, exit: 1},
		{name: "preview instead of stable", selector: `printf '%s\n' v6.6.0-rc.1`, exit: 1},
		{name: "verified current stable", document: `[{"tag_name":"v6.5.0","draft":false,"prerelease":false}]`, reads: 1},
		{name: "verified newer stable", document: stable, reads: 1, install: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			script := `source "$UPDATER_UNDER_TEST"
logger() { :; }
check_auto_updates_enabled() { :; }
installed_binary_is_pulse_pro() { return 1; }
get_current_version() { printf '%s\n' v6.5.0; }
curl() {
  printf 'READ\n' >> "$FIXTURE_DIR/actions"
  printf '%s\n' "$RELEASE_DOCUMENT"
  return "$CURL_EXIT"
}
perform_update() { printf 'INSTALL %s\n' "$1" >> "$FIXTURE_DIR/actions"; }
`
			if tc.missingParser {
				script += "command() { if [[ \"$*\" == '-v jq' ]]; then return 1; fi; builtin command \"$@\"; }\n"
			}
			if tc.selector != "" {
				script += "get_latest_stable_version() { " + tc.selector + "; }\n"
			}
			cmd := exec.Command("bash", "-c", script+"main\n")
			cmd.Env = append(os.Environ(), "UPDATER_UNDER_TEST="+updater, "FIXTURE_DIR="+dir,
				"RELEASE_DOCUMENT="+tc.document, "CURL_EXIT="+strconv.Itoa(tc.curlExit))
			out, err := cmd.CombinedOutput()
			exit := 0
			if err != nil {
				if e, ok := err.(*exec.ExitError); ok {
					exit = e.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			actions, readErr := os.ReadFile(filepath.Join(dir, "actions"))
			if readErr != nil && !os.IsNotExist(readErr) {
				t.Fatal(readErr)
			}
			wantActions := strings.Repeat("READ\n", tc.reads)
			if tc.install {
				wantActions += "INSTALL v6.6.0\n"
			}
			if exit != tc.exit || string(actions) != wantActions {
				t.Fatalf("exit=%d actions=%q; want exit=%d actions=%q; output %s", exit, actions, tc.exit, wantActions, out)
			}
			if tc.exit != 0 {
				for _, forbidden := range []string{"Already running latest version", "Auto-update check completed", "Update completed successfully", "Latest stable version:", "operand expected"} {
					if strings.Contains(string(out), forbidden) {
						t.Fatalf("failed discovery reached comparison/completion (%s): %s", forbidden, out)
					}
				}
				if !strings.Contains(string(out), "Could not determine latest version from GitHub") {
					t.Fatalf("missing useful discovery failure: %s", out)
				}
			} else if !strings.Contains(string(out), "Auto-update check completed") {
				t.Fatalf("verified stable control lost normal completion: %s", out)
			}
		})
	}
}

func TestAutoUpdateTimerObservationFailsClosed(t *testing.T) {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		t.Fatal("timer main-flow proof requires a non-Docker environment")
	}
	updater, err := filepath.Abs(filepath.Join("..", "pulse-auto-update.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, state, diagnostic, unit string
		managerExit, exit             int
		admitted                      bool
	}{
		{name: "enabled", state: "enabled\n", admitted: true},
		{name: "runtime enabled", state: "enabled-runtime\n", admitted: true},
		{name: "alias", state: "alias\n", admitted: true},
		{name: "static", state: "static\n", admitted: true},
		{name: "indirect", state: "indirect\n", admitted: true},
		{name: "generated", state: "generated\n", admitted: true},
		{name: "transient", state: "transient\n", admitted: true},
		{name: "disabled", state: "disabled\n", managerExit: 1},
		{name: "masked", state: "masked\n", managerExit: 1},
		{name: "runtime masked", state: "masked-runtime\n", managerExit: 1},
		{name: "linked", state: "linked\n", managerExit: 1},
		{name: "runtime linked", state: "linked-runtime\n", managerExit: 1},
		{name: "unavailable manager", diagnostic: "synthetic-private-manager-diagnostic\n", managerExit: 1, exit: 1},
		{name: "empty success", exit: 1},
		{name: "failed enabled", state: "enabled\n", managerExit: 1, exit: 1},
		{name: "enabled with diagnostic", state: "enabled\n", diagnostic: "synthetic-private-manager-diagnostic\n", exit: 1},
		{name: "unknown state", state: "unknown\n", exit: 1},
		{name: "truncated record", state: "enabled", exit: 1},
		{name: "trailing blank record", state: "enabled\n\n", exit: 1},
		{name: "duplicate record", state: "enabled\nenabled\n", exit: 1},
		{name: "wrong disabled exit", state: "disabled\n", exit: 1},
		{name: "wrong static exit", state: "static\n", managerExit: 1, exit: 1},
		{name: "missing unit", state: "not-found\n", managerExit: 4, exit: 1},
		{name: "timed out partial record", state: "enabled\n", managerExit: 124, exit: 1},
		{name: "killed partial record", state: "enabled\n", managerExit: 137, exit: 1},
		{name: "configured timer", state: "enabled\n", unit: "pulse-custom-update.timer", admitted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			manager := `#!/bin/bash
printf '%s\n' "$*" >> "$FIXTURE_DIR/manager-args"
printf '%s' "$TIMER_OBSERVATION"
printf '%s' "$TIMER_DIAGNOSTIC" >&2
exit "$TIMER_EXIT"
`
			if err := os.WriteFile(filepath.Join(dir, "systemctl"), []byte(manager), 0700); err != nil {
				t.Fatal(err)
			}
			script := `source "$UPDATER_UNDER_TEST"
logger() { :; }
installed_binary_is_pulse_pro() { return 1; }
get_current_version() { printf '%s\n' v6.5.0; }
get_latest_stable_version() { printf 'READ\n' >> "$FIXTURE_DIR/actions"; printf '%s\n' v6.5.0; }
perform_update() { printf 'UNEXPECTED_INSTALL\n' >> "$FIXTURE_DIR/actions"; }
main
`
			unit := tc.unit
			if unit == "" {
				unit = "pulse-update.timer"
			}
			cmd := exec.Command("bash", "-c", script)
			cmd.Env = append(os.Environ(), "UPDATER_UNDER_TEST="+updater, "FIXTURE_DIR="+dir,
				"PULSE_CONFIG_DIR="+filepath.Join(dir, "missing-config"), "PULSE_UPDATE_TIMER_UNIT="+unit,
				"TIMER_OBSERVATION="+tc.state, "TIMER_DIAGNOSTIC="+tc.diagnostic,
				"TIMER_EXIT="+strconv.Itoa(tc.managerExit), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			out, err := cmd.CombinedOutput()
			exit := 0
			if err != nil {
				if e, ok := err.(*exec.ExitError); ok {
					exit = e.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			actions, readErr := os.ReadFile(filepath.Join(dir, "actions"))
			if readErr != nil && !os.IsNotExist(readErr) {
				t.Fatal(readErr)
			}
			wantActions := ""
			if tc.admitted {
				wantActions = "READ\n"
			}
			if exit != tc.exit || string(actions) != wantActions {
				t.Fatalf("exit=%d actions=%q; want exit=%d actions=%q; output %s", exit, actions, tc.exit, wantActions, out)
			}
			if strings.Contains(string(out), "synthetic-private-manager-diagnostic") {
				t.Fatalf("raw manager diagnostic leaked: %s", out)
			}
			if tc.exit != 0 && !strings.Contains(string(out), "Cannot verify auto-update timer state") {
				t.Fatalf("missing truthful unknown-state diagnostic: %s", out)
			}
			if !tc.admitted && strings.Contains(string(out), "Auto-update check completed") {
				t.Fatalf("unadmitted timer reported completion: %s", out)
			}
			args, err := os.ReadFile(filepath.Join(dir, "manager-args"))
			if err != nil || string(args) != "is-enabled "+unit+"\n" {
				t.Fatalf("timer observation must be one exact non-quiet read: %v %q", err, args)
			}
		})
	}
}

func TestAutoUpdateTimerReadHasHardDeadline(t *testing.T) {
	updater, err := filepath.Abs(filepath.Join("..", "pulse-auto-update.sh"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	// Even a plausible enabled record must not admit an unfinished process.
	// The self-bounded fixture execs sleep so the actual timeout owns all work.
	manager := "#!/bin/bash\ntrap '' TERM\nprintf 'enabled\\n'\nexec sleep 9\n"
	if err := os.WriteFile(filepath.Join(dir, "systemctl"), []byte(manager), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", `source "$UPDATER_UNDER_TEST"
logger() { :; }
check_auto_updates_enabled
printf 'ADMITTED\n'
`)
	cmd.Env = append(os.Environ(), "UPDATER_UNDER_TEST="+updater,
		"PULSE_CONFIG_DIR="+filepath.Join(dir, "missing-config"), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	start := time.Now()
	out, err := cmd.CombinedOutput()
	status, ok := err.(*exec.ExitError)
	if !ok || status.ExitCode() != 1 || ctx.Err() != nil || time.Since(start) > 8*time.Second ||
		!strings.Contains(string(out), "Cannot verify auto-update timer state") || strings.Contains(string(out), "ADMITTED") {
		t.Fatalf("timer deadline: %v, duration %s, context %v, output %s", err, time.Since(start), ctx.Err(), out)
	}
}

func TestAutoUpdateRequiresVerifiedCommunityEdition(t *testing.T) {
	cases := []struct {
		name, body, legacy string
		status             int // Pro=0, verified community=1, unknown=2.
		nonExecutable      bool
		directory          bool
	}{
		{name: "community", body: "echo 'Pulse v6.5.0'", status: 1},
		{name: "community_build_details", body: "printf 'Pulse v6.5.0\\nBuilt: yesterday\\nCommit: abc\\n'", status: 1},
		{name: "community_rc", body: "echo 'Pulse v6.5.0-rc.1'", status: 1},
		{name: "legacy_community", legacy: "echo 'Pulse v6.5.0'", status: 1},
		{name: "pro", body: "echo 'Pulse Pro v6.5.0'"},
		{name: "legacy_pro", legacy: "echo 'Pulse Pro v6.5.0'"},
		{name: "primary_pro_over_legacy_community", body: "echo 'Pulse Pro v6.5.0'", legacy: "echo 'Pulse v6.5.0'"},
		{name: "failed_probe", body: "exit 1", status: 2},
		{name: "version_then_failure", body: "echo 'Pulse v6.5.0'; exit 1", status: 2},
		{name: "pro_then_failure", body: "echo 'Pulse Pro v6.5.0'; exit 1", status: 2},
		{name: "empty_probe", body: ":", status: 2},
		{name: "unknown_brand", body: "echo 'Other v6.5.0 private-detail'", status: 2},
		{name: "bare_version", body: "echo 'v6.5.0'", status: 2},
		{name: "invalid_version", body: "echo 'Pulse broken'", status: 2},
		{name: "non_executable", body: "echo 'Pulse v6.5.0'", nonExecutable: true, status: 2},
		{name: "primary_failure_over_legacy_community", body: "exit 1", legacy: "echo 'Pulse v6.5.0'", status: 2},
		{name: "primary_directory_over_legacy_community", directory: true, legacy: "echo 'Pulse v6.5.0'", status: 2},
		{name: "missing_binary_with_version_file", status: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "bin"), 0700); err != nil {
				t.Fatal(err)
			}
			primary := filepath.Join(dir, "bin", "pulse")
			if tc.directory {
				if err := os.Mkdir(primary, 0700); err != nil {
					t.Fatal(err)
				}
			} else if tc.body != "" {
				mode := os.FileMode(0700)
				if tc.nonExecutable {
					mode = 0600
				}
				if err := os.WriteFile(primary, []byte("#!/bin/bash\n"+tc.body+"\n"), mode); err != nil {
					t.Fatal(err)
				}
			}
			if tc.legacy != "" {
				if err := os.WriteFile(filepath.Join(dir, "pulse"), []byte("#!/bin/bash\n"+tc.legacy+"\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			// VERSION deliberately offers a usable public version even when the
			// installed executable cannot establish its edition.
			if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("v6.5.0\n"), 0600); err != nil {
				t.Fatal(err)
			}
			script := "set -euo pipefail\n" + extractAutoUpdateAdmissionFunction(t, "installed_binary_is_pulse_pro") + `
if installed_binary_is_pulse_pro; then status=0; else status=$?; fi
printf 'STATUS=%s\n' "$status"
`
			cmd := exec.Command("bash", "-c", script)
			cmd.Env = append(os.Environ(), "INSTALL_DIR="+dir)
			out, err := cmd.CombinedOutput()
			if err != nil || strings.TrimSpace(string(out)) != "STATUS="+strconv.Itoa(tc.status) {
				t.Fatalf("classification: %v, output %s; want status %d", err, out, tc.status)
			}
		})
	}
}

func TestAutoUpdateEditionProbeHasHardDeadline(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	// A version line is not success until the probe exits successfully. Even
	// a TERM-resistant executable must stop at the timeout's KILL backstop.
	if err := os.WriteFile(filepath.Join(dir, "bin", "pulse"), []byte("#!/bin/bash\ntrap '' TERM\necho 'Pulse Pro v6.5.0'\nexec sleep 60\n"), 0700); err != nil {
		t.Fatal(err)
	}
	script := extractAutoUpdateAdmissionFunction(t, "installed_binary_is_pulse_pro") + `
if installed_binary_is_pulse_pro; then status=0; else status=$?; fi
printf 'STATUS=%s\n' "$status"
`
	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), "INSTALL_DIR="+dir)
	start := time.Now()
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.HasSuffix(string(out), "STATUS=2\n") || time.Since(start) > 9*time.Second {
		t.Fatalf("deadline: %v, duration %s, output %s", err, time.Since(start), out)
	}
}

func TestAutoUpdateEditionGateStopsMainBeforeDownloads(t *testing.T) {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		t.Fatal("edition main-flow proof requires a non-Docker environment")
	}
	updater, err := filepath.Abs(filepath.Join("..", "pulse-auto-update.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, body string
		exit       int
		admitted   bool
	}{
		{"community", "echo 'Pulse v6.5.0'", 0, true},
		{"pro", "echo 'Pulse Pro v6.5.0'", 0, false},
		{"failed_probe", "exit 1", 1, false},
		{"misleading_failed_probe", "echo 'Pulse v6.5.0'; exit 1", 1, false},
		{"unknown_brand", "echo 'Other v6.5.0 private-detail'", 1, false},
		{"missing_binary", "", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "bin"), 0700); err != nil {
				t.Fatal(err)
			}
			if tc.body != "" {
				if err := os.WriteFile(filepath.Join(dir, "bin", "pulse"), []byte("#!/bin/bash\n"+tc.body+"\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("v6.5.0\n"), 0600); err != nil {
				t.Fatal(err)
			}
			script := `source "$UPDATER_UNDER_TEST"
log() { printf '%s\n' "$*"; }
check_auto_updates_enabled() { :; }
get_latest_stable_version() { echo RELEASE_READ >> "$INSTALL_DIR/actions"; echo v6.5.1; }
perform_update() { echo INSTALLER >> "$INSTALL_DIR/actions"; }
main
`
			cmd := exec.Command("bash", "-c", script)
			cmd.Env = append(os.Environ(), "UPDATER_UNDER_TEST="+updater, "PULSE_INSTALL_DIR="+dir, "PULSE_CONFIG_DIR="+filepath.Join(dir, "config"))
			out, err := cmd.CombinedOutput()
			exit := 0
			if err != nil {
				if e, ok := err.(*exec.ExitError); ok {
					exit = e.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			actions, readErr := os.ReadFile(filepath.Join(dir, "actions"))
			if exit != tc.exit || (readErr == nil) != tc.admitted || (tc.admitted && string(actions) != "RELEASE_READ\nINSTALLER\n") {
				t.Fatalf("exit=%d, actions=%q, read=%v; want exit=%d admitted=%t; output %s", exit, actions, readErr, tc.exit, tc.admitted, out)
			}
			if strings.Contains(string(out), "private-detail") {
				t.Fatalf("edition output leaked: %s", out)
			}
		})
	}
}

func TestAutoUpdateRequiresCompleteTopLevelConsent(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Fatal("jq is required to execute the unattended-update consent checks")
	}
	cases := []struct {
		name, config string
		admitted     bool
		exit         int
	}{
		{"stable", `{"autoUpdateEnabled":true,"updateChannel":"stable"}`, true, 0},
		{"normalized_stable", `{"autoUpdateEnabled":true,"updateChannel":" STABLE "}`, true, 0},
		{"legacy_channel_default", `{"autoUpdateEnabled":true}`, true, 0},
		{"disabled", `{"autoUpdateEnabled":false,"updateChannel":"stable"}`, false, 0},
		{"missing_consent", `{"updateChannel":"stable"}`, false, 0},
		{"null_consent", `{"autoUpdateEnabled":null}`, false, 0},
		{"nested_consent", `{"notification":{"autoUpdateEnabled":true}}`, false, 0},
		{"nested_enabled_over_disabled", `{"autoUpdateEnabled":false,"notification":{"autoUpdateEnabled":true}}`, false, 0},
		{"quoted_consent", `{"notes":"\"autoUpdateEnabled\":true"}`, false, 0},
		{"rc", `{"autoUpdateEnabled":true,"updateChannel":"rc"}`, false, 0},
		{"normalized_rc", `{"autoUpdateEnabled":true,"updateChannel":" RC "}`, false, 0},
		{"unknown_channel", `{"autoUpdateEnabled":true,"updateChannel":"beta"}`, false, 0},
		{"nested_stable_over_rc", `{"autoUpdateEnabled":true,"updateChannel":"rc","other":{"updateChannel":"stable"}}`, false, 0},
		{"truncated_after_consent", `{"autoUpdateEnabled":true,`, false, 1},
		{"trailing_garbage", `{"autoUpdateEnabled":true}secret-invalid-suffix`, false, 1},
		{"multiple_objects", "{\"autoUpdateEnabled\":true}\n{\"autoUpdateEnabled\":false}", false, 1},
		{"empty", "", false, 1},
		{"array", `[{"autoUpdateEnabled":true}]`, false, 1},
		{"string_consent", `{"autoUpdateEnabled":"true"}`, false, 1},
		{"number_consent", `{"autoUpdateEnabled":1}`, false, 1},
		{"invalid_channel_type", `{"autoUpdateEnabled":true,"updateChannel":["stable"]}`, false, 1},
		{"false_channel", `{"autoUpdateEnabled":true,"updateChannel":false}`, false, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "system.json"), []byte(tc.config), 0600); err != nil {
				t.Fatal(err)
			}
			script := `set -eu
log() { printf '%s\n' "$*"; }
systemctl() { test "$*" = 'is-enabled pulse-update.timer' && printf 'enabled\n'; }
timeout() { [[ "$*" == '-k 1 5 systemctl is-enabled pulse-update.timer' ]] || return 98; shift 3; "$@"; }
curl() { echo UNEXPECTED_NETWORK; exit 99; }
` + extractAutoUpdateConsentGate(t) + `
check_auto_updates_enabled
echo ADMITTED
`
			cmd := exec.Command("bash", "-c", script)
			cmd.Env = append(os.Environ(), "CONFIG_DIR="+dir, "UPDATE_TIMER_UNIT=pulse-update.timer")
			out, err := cmd.CombinedOutput()
			exit := 0
			if err != nil {
				if e, ok := err.(*exec.ExitError); ok {
					exit = e.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			if exit != tc.exit || strings.Contains(string(out), "ADMITTED") != tc.admitted {
				t.Fatalf("exit=%d, admitted=%t; want exit=%d, admitted=%t; output:\n%s", exit, strings.Contains(string(out), "ADMITTED"), tc.exit, tc.admitted, out)
			}
			if strings.Contains(string(out), "UNEXPECTED_NETWORK") || strings.Contains(string(out), "secret-invalid-suffix") {
				t.Fatalf("configuration gate leaked contents or contacted the network:\n%s", out)
			}
		})
	}
}

func TestAutoUpdateConsentKeepsTimerAndParserBoundaries(t *testing.T) {
	for _, scenario := range []string{"missing_config_enabled_timer", "disabled_timer", "missing_jq", "nonregular_config"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			script := `set -eu
log() { printf '%s\n' "$*"; }
systemctl() { printf 'enabled\n'; }
timeout() { [[ "$*" == '-k 1 5 systemctl is-enabled pulse-update.timer' ]] || return 98; shift 3; "$@"; }
`
			wantExit, wantAdmitted := 0, false
			switch scenario {
			case "missing_config_enabled_timer":
				wantAdmitted = true // Preserve the legacy explicit timer opt-in.
			case "disabled_timer":
				script += "systemctl() { printf 'disabled\\n'; return 1; }\n"
			case "missing_jq":
				if err := os.WriteFile(filepath.Join(dir, "system.json"), []byte(`{"autoUpdateEnabled":true}`), 0600); err != nil {
					t.Fatal(err)
				}
				script += "command() { if [[ \"$*\" == '-v jq' ]]; then return 1; fi; builtin command \"$@\"; }\n"
				wantExit = 1
			case "nonregular_config":
				if err := os.Mkdir(filepath.Join(dir, "system.json"), 0700); err != nil {
					t.Fatal(err)
				}
				wantExit = 1
			}
			script += extractAutoUpdateConsentGate(t) + "\ncheck_auto_updates_enabled\necho ADMITTED\n"
			cmd := exec.Command("bash", "-c", script)
			cmd.Env = append(os.Environ(), "CONFIG_DIR="+dir, "UPDATE_TIMER_UNIT=pulse-update.timer")
			out, err := cmd.CombinedOutput()
			exit := 0
			if err != nil {
				if e, ok := err.(*exec.ExitError); ok {
					exit = e.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			if exit != wantExit || strings.Contains(string(out), "ADMITTED") != wantAdmitted {
				t.Fatalf("exit=%d, admitted=%t; want exit=%d, admitted=%t; output:\n%s", exit, strings.Contains(string(out), "ADMITTED"), wantExit, wantAdmitted, out)
			}
		})
	}
}

// VERSION is deliberately plausible in every fixture. Neither it nor a legacy
// executable can rescue a present primary whose own version evidence is bad.
func TestAutoUpdateVersionNeedsSuccessfulExecutable(t *testing.T) {
	for _, tc := range []struct {
		name, body, legacy, want string
		nonExecutable, directory bool
	}{
		{name: "community", body: "echo 'Pulse v6.5.0'", want: "v6.5.0"},
		{name: "unprefixed", body: "echo 'Pulse 6.5.0'", want: "v6.5.0"},
		{name: "build_details", body: "printf 'Pulse v6.5.0\\nBuilt: yesterday\\n'", want: "v6.5.0"},
		{name: "preview", body: "echo 'Pulse v6.5.0-rc.1'", want: "v6.5.0-rc.1"},
		{name: "legacy", legacy: "echo 'Pulse v6.5.0'", want: "v6.5.0"},
		{name: "failed", body: "echo 'Pulse v6.5.0'; exit 1"},
		{name: "empty", body: ":"},
		{name: "pro", body: "echo 'Pulse Pro v6.5.0'"},
		{name: "other_brand", body: "echo 'Other v6.5.0 private-detail'"},
		{name: "later_line", body: "printf 'Other\\nPulse v6.5.0\\n'"},
		{name: "invalid_suffix", body: "echo 'Pulse v6.5.0-extra!'"},
		{name: "non_executable", body: "echo 'Pulse v6.5.0'", nonExecutable: true},
		{name: "missing"},
		{name: "bad_primary_over_legacy", body: "exit 1", legacy: "echo 'Pulse v6.5.0'"},
		{name: "primary_directory", directory: true, legacy: "echo 'Pulse v6.5.0'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "bin"), 0700); err != nil {
				t.Fatal(err)
			}
			primary := filepath.Join(dir, "bin", "pulse")
			if tc.directory {
				if err := os.Mkdir(primary, 0700); err != nil {
					t.Fatal(err)
				}
			} else if tc.body != "" {
				mode := os.FileMode(0700)
				if tc.nonExecutable {
					mode = 0600
				}
				if err := os.WriteFile(primary, []byte("#!/bin/bash\n"+tc.body+"\n"), mode); err != nil {
					t.Fatal(err)
				}
			}
			if tc.legacy != "" {
				if err := os.WriteFile(filepath.Join(dir, "pulse"), []byte("#!/bin/bash\n"+tc.legacy+"\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("v6.5.0\n"), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "-c", "set -euo pipefail\n"+extractAutoUpdateAdmissionFunction(t, "get_current_version")+"\nget_current_version\n")
			cmd.Env = append(os.Environ(), "INSTALL_DIR="+dir)
			out, err := cmd.CombinedOutput()
			want := tc.want
			if want == "" {
				want = "unknown"
			}
			if err != nil || strings.TrimSpace(string(out)) != want {
				t.Fatalf("version: %v, output %q; want %s", err, out, want)
			}
		})
	}
}

func TestAutoUpdateVersionProbeHasHardDeadline(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "pulse"), []byte("#!/bin/bash\ntrap '' TERM\necho 'Pulse v6.5.0'\nexec sleep 60\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("v6.5.0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-c", extractAutoUpdateAdmissionFunction(t, "get_current_version")+"\nget_current_version\n")
	cmd.Env = append(os.Environ(), "INSTALL_DIR="+dir)
	start := time.Now()
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.HasSuffix(string(out), "unknown\n") || time.Since(start) > 9*time.Second {
		t.Fatalf("deadline: %v, duration %s, output %s", err, time.Since(start), out)
	}
}
