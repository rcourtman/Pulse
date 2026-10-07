package installtests

import (
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
systemctl() { test "$*" = 'is-enabled --quiet pulse-update.timer'; }
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
systemctl() { return 0; }
`
			wantExit, wantAdmitted := 0, false
			switch scenario {
			case "missing_config_enabled_timer":
				wantAdmitted = true // Preserve the legacy explicit timer opt-in.
			case "disabled_timer":
				script += "systemctl() { return 1; }\n"
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
