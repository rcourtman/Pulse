package installtests

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Keep these Bash-only checks runnable as a standalone file as well as part of
// the installer package. They need no application imports or local listeners.
// From this directory: go test -race pulse_auto_update_consent_test.go
func extractAutoUpdateConsentGate(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "pulse-auto-update.sh"))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?ms)^check_auto_updates_enabled\(\) \{\n.*?^\}`).Find(content)
	if match == nil {
		t.Fatal("cannot find the actual unattended-update consent gate")
	}
	return string(match)
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
