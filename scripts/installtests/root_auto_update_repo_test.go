package installtests

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Run the real rewriter in the conditional context used by its installer
// caller. A trailing successful chmod must not hide a failed allocation/mv.
// All data and failure doubles are ordinary-user temporary fixtures.
func TestConfigureAutoUpdateRepositoryPublication(t *testing.T) {
	for _, fault := range []string{"allocation", "write", "mode", "rename", "syntax", "none"} {
		t.Run(fault, func(t *testing.T) {
			dir := t.TempDir()
			dest := filepath.Join(dir, "helper")
			original := "#!/usr/bin/env bash\nGITHUB_REPO=old/repository\nprintf '%s\\n' \"$GITHUB_REPO\"\n"
			if fault == "syntax" {
				original += "if\n"
			}
			if err := os.WriteFile(dest, []byte(original), 0751); err != nil {
				t.Fatal(err)
			}
			script := `set -euo pipefail
GITHUB_REPO=fixture/selected
` + extractRootInstallShellFunction(t, "configure_auto_update_script_repo") + `
mktemp() {
    [[ "$FAULT" != allocation ]] || { echo FAULT >&2; return 71; }
    # Confine the pre-repair /tmp allocation too, for the parent control.
    if [[ "$1" == /tmp/pulse-auto-update-script.XXXXXX ]]; then
        command mktemp "$FIXTURE/legacy.XXXXXX"
    else
        command mktemp "$@"
    fi
}
awk() {
    if [[ "$FAULT" == write ]]; then echo partial; echo FAULT >&2; return 72; fi
    command awk "$@"
}
chmod() {
    [[ "$FAULT" != mode ]] || { echo FAULT >&2; return 73; }
    command chmod "$@"
}
mv() {
    [[ "$FAULT" != rename ]] || { echo FAULT >&2; return 74; }
    command mv "$@"
}
if configure_auto_update_script_repo "$FIXTURE/helper"; then
    echo CONFIGURATION_SUCCESS
else
    echo CONFIGURATION_FAILURE
fi
`
			cmd := exec.Command("bash", "-c", script)
			cmd.Env = append(os.Environ(), "FAULT="+fault, "FIXTURE="+dir)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("conditional caller aborted: %v\n%s", err, out)
			}
			wantSuccess := fault == "none"
			if strings.Contains(string(out), "CONFIGURATION_SUCCESS") != wantSuccess {
				t.Errorf("incorrect configuration result:\n%s", out)
			}
			if fault != "none" && fault != "syntax" && !strings.Contains(string(out), "FAULT") {
				t.Errorf("injected failure was not exercised:\n%s", out)
			}
			after, err := os.ReadFile(dest)
			if err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(dest)
			if err != nil {
				t.Fatal(err)
			}
			if !wantSuccess {
				if string(after) != original || info.Mode().Perm() != 0751 {
					t.Errorf("failed rewrite changed input bytes or mode: mode=%o\n%s", info.Mode().Perm(), after)
				}
			} else {
				want := "#!/usr/bin/env bash\nGITHUB_REPO=\"fixture/selected\"\nprintf '%s\\n' \"$GITHUB_REPO\"\n"
				if string(after) != want || info.Mode().Perm() != 0755 {
					t.Errorf("incorrect configured helper: mode=%o\n%s", info.Mode().Perm(), after)
				}
				result, err := exec.Command("bash", dest).CombinedOutput()
				if err != nil || string(result) != "fixture/selected\n" {
					t.Errorf("configured helper did not consume selected repository: %v\n%s", err, result)
				}
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != "helper" {
				t.Errorf("configuration left temporary inputs: %v", entries)
			}
		})
	}
}

// A configuration mv failure used to fall through to chmod and return zero.
// The real containing caller then published the wrong-repository helper and
// rewrote its units. Refuse only that inner mv, not the outer publication.
func TestInstallAutoUpdateAssetsRejectsRepositoryRewriteFailure(t *testing.T) {
	dir := t.TempDir()
	helper, service, timer := prepareAutoUpdatePaths(t, dir)
	before := map[string]string{
		helper:  "#!/usr/bin/env bash\necho previously-working\n",
		service: "previous-service\n",
		timer:   "previous-timer\n",
	}
	for path, data := range before {
		if err := os.WriteFile(path, []byte(data), 0751); err != nil {
			t.Fatal(err)
		}
	}
	script := "set -euo pipefail\n" + autoUpdateAssetsHarness(t, dir, helper, service, timer, `
GITHUB_REPO=fixture/selected
mktemp() {
    if [[ "$1" == /tmp/pulse-auto-update-script.XXXXXX ]]; then
        command mktemp "${PULSE_AUTO_UPDATE_DEST}.legacy.XXXXXX"
    else
        command mktemp "$@"
    fi
}
mv() {
    case "${@: -1}" in *.staged.*) echo CONFIGURATION_RENAME_FAULT; return 74 ;; esac
    command mv "$@"
}
safe_systemctl() { echo UNEXPECTED_SYSTEMCTL; }
`)
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("caller aborted: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "CONFIGURATION_RENAME_FAULT") ||
		!strings.Contains(string(out), "FAILED_AS_EXPECTED") ||
		strings.Contains(string(out), "UNEXPECTED_SUCCESS") ||
		strings.Contains(string(out), "UNEXPECTED_SYSTEMCTL") {
		t.Errorf("configuration failure was not contained before publication:\n%s", out)
	}
	for path, data := range before {
		after, err := os.ReadFile(path)
		if err != nil || string(after) != data {
			t.Errorf("configuration failure changed %s: %v\n%s", path, err, after)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0751 {
			t.Errorf("configuration failure changed mode of %s: %v", path, err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(helper))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(helper) {
		t.Errorf("failed containing caller left staging files: %v", entries)
	}
}

// Execute the real setup and its real main consumer with bounded systemctl
// doubles and private saved configuration. Optional updater failure must not
// strand monitoring; failed configuration publication must preserve all bytes.
func TestRootAutoUpdateSetupCompletion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix server installer fixture")
	}
	source := repoFile("install.sh")
	if override := os.Getenv("PULSE_AUTO_UPDATE_SETUP_SOURCE"); override != "" {
		source = override
	}
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	extract := func(name string) string {
		t.Helper()
		pattern := regexp.MustCompile(`(?ms)^` + regexp.QuoteMeta(name) + `\(\) \{\n.*?^\}`)
		body := pattern.Find(content)
		if body == nil {
			t.Fatalf("missing installer function %s", name)
		}
		return string(body)
	}
	for _, service := range []string{"pulse", "pulse-blue"} {
		for _, fault := range []string{"none", "new", "no-jq", "no-jq-missing-fields", "assets", "enable", "enable-timeout", "allocation", "jq", "copy", "sed-first", "sed-second", "rename", "write", "start", "start-timeout"} {
			for _, conditional := range []bool{false, true} {
				caller := "main"
				if conditional {
					caller = "conditional-main"
				}
				t.Run(service+"/"+fault+"/"+caller, func(t *testing.T) {
					dir := t.TempDir()
					config := filepath.Join(dir, "system.json")
					original := `{"autoUpdateEnabled":false,"updateChannel":"rc","pollingInterval":17,"marker":"preserved"}` + "\n"
					if fault == "no-jq-missing-fields" {
						original = `{"pollingInterval":17,"marker":"preserved"}` + "\n"
					}
					newConfig := fault == "new" || fault == "write"
					if !newConfig {
						if err := os.WriteFile(config, []byte(original), 0640); err != nil {
							t.Fatal(err)
						}
					}
					bin := filepath.Join(dir, "bin")
					if err := os.Mkdir(bin, 0700); err != nil {
						t.Fatal(err)
					}
					// timeout executes this ordinary child, never host systemctl.
					if err := os.WriteFile(filepath.Join(bin, "systemctl"), []byte(`#!/bin/bash
printf '%s\n' "$*" >> "$FIXTURE/actions"
if [[ "$1" == enable && "$FAULT" == enable || "$1" == start && "$FAULT" == start ]]; then exit 17; fi
if [[ "$1" == enable && "$FAULT" == enable-timeout || "$1" == start && "$FAULT" == start-timeout ]]; then exit 124; fi
`), 0700); err != nil {
						t.Fatal(err)
					}
					body := `set -euo pipefail
SERVICE_NAME="$FIXTURE_SERVICE"
SERVICE_NAME_EXPLICIT=true
DEFAULT_SERVICE_NAME=pulse
CONFIG_DIR="$FIXTURE"
UPDATE_TIMER_UNIT="${SERVICE_NAME}-update.timer"
UPDATE_TIMER_PATH="$FIXTURE/${UPDATE_TIMER_UNIT}"
IN_CONTAINER=true
BUILD_FROM_SOURCE=false
FORCE_VERSION=v6.6.0
ENABLE_AUTO_UPDATES=true
print_info() { printf 'INFO %s\n' "$*"; }
print_warn() { printf 'WARN %s\n' "$*"; }
print_success() { printf 'SUCCESS %s\n' "$*"; }
for fn in print_header check_root detect_os check_docker_environment offer_existing_auto_updates run_upgrade_readiness_preflight backup_existing create_user download_pulse setup_directories setup_update_command ensure_systemd_service_installed create_marker_file; do
    eval "$fn() { :; }"
done
check_existing_installation() { CURRENT_VERSION=v6.5.0; return 0; }
compare_versions() { return 1; }
detect_service_name() { printf '%s\n' "$SERVICE_NAME"; }
selected_update_channel() { echo stable; }
install_auto_update_assets() {
    printf 'assets\n' >> "$FIXTURE/actions"
    [[ "$FAULT" != assets ]]
}
start_pulse() { printf 'monitoring-start\n' >> "$FIXTURE/actions"; }
print_completion() { printf 'INSTALL_COMPLETED flag=%s\n' "$ENABLE_AUTO_UPDATES"; }
chown() { :; }
command() {
    if [[ "$1" == -v && "$2" == jq ]]; then
        case "$FAULT" in no-jq|no-jq-missing-fields|copy|sed-first|sed-second) return 1 ;; esac
    fi
    builtin command "$@"
}
jq() {
    [[ "$FAULT" != jq ]] || return 17
    python3 - "$@" <<'PYJSON'
import json,sys
with open(sys.argv[-1]) as f: value=json.load(f)
value.update(autoUpdateEnabled=True,updateChannel=sys.argv[3])
print(json.dumps(value))
PYJSON
}
mktemp() { [[ "$FAULT" != allocation ]] || return 17; command mktemp "$@"; }
cp() { [[ "$FAULT" != copy ]] || return 17; command cp "$@"; }
SED_CALLS=0
sed() {
    SED_CALLS=$((SED_CALLS + 1))
    if [[ "$FAULT" == sed-first && "$SED_CALLS" == 1 || "$FAULT" == sed-second && "$SED_CALLS" == 2 ]]; then return 17; fi
    command sed "$@"
}
mv() { [[ "$FAULT" != rename ]] || return 17; command mv "$@"; }
printf() {
    if [[ "$FAULT" == write && "$1" == '{"autoUpdateEnabled":true'* ]]; then return 17; fi
    builtin printf "$@"
}
` + extract("safe_systemctl") + "\n" + extract("setup_auto_updates") + "\n" + extract("main") + "\n"
					if conditional {
						body += "if main; then :; else exit 81; fi\n"
					} else {
						body += "main\n"
					}
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					cmd := exec.CommandContext(ctx, "bash", "-c", body)
					cmd.Env = append(os.Environ(), "FIXTURE="+dir, "FIXTURE_SERVICE="+service, "FAULT="+fault, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
					out, runErr := cmd.CombinedOutput()
					if runErr != nil || ctx.Err() != nil || !strings.Contains(string(out), "INSTALL_COMPLETED") {
						t.Fatalf("optional updater setup prevented monitoring completion: %v, %v\n%s", runErr, ctx.Err(), out)
					}
					calls, err := os.ReadFile(filepath.Join(dir, "actions"))
					if err != nil {
						t.Fatal(err)
					}
					wantCalls := "assets\n"
					if fault != "assets" {
						wantCalls += "enable " + service + "-update.timer\n"
					}
					saved := fault == "none" || fault == "new" || fault == "no-jq" || fault == "no-jq-missing-fields" || fault == "start" || fault == "start-timeout"
					complete := saved && fault != "start" && fault != "start-timeout"
					if saved {
						wantCalls += "start " + service + "-update.timer\n"
					}
					wantCalls += "monitoring-start\n"
					if string(calls) != wantCalls {
						t.Fatalf("wrong or repeated timer/monitoring operations: got %q, want %q\n%s", calls, wantCalls, out)
					}
					if strings.Contains(string(out), "Automatic updates enabled") || strings.Contains(string(out), "timer enable/start commands completed") != complete || strings.Contains(string(out), "flag=true") != complete {
						t.Fatalf("unconfirmed timer success or missing partial-setup result:\n%s", out)
					}
					if !complete && fault != "assets" && (!strings.Contains(string(out), "setup is incomplete") || !strings.Contains(string(out), "systemctl status "+service+"-update.timer")) {
						t.Fatalf("missing instance-scoped partial-setup inspection guidance:\n%s", out)
					}
					after, readErr := os.ReadFile(config)
					if saved {
						var value map[string]any
						if readErr != nil || json.Unmarshal(after, &value) != nil || value["autoUpdateEnabled"] != true || value["updateChannel"] != "stable" {
							t.Fatalf("preferences not completely persisted: %v %s", readErr, after)
						}
						if !newConfig && (value["marker"] != "preserved" || value["pollingInterval"] != float64(17)) {
							t.Fatalf("unrelated saved configuration changed: %s", after)
						}
					} else if newConfig {
						if !os.IsNotExist(readErr) {
							t.Fatalf("failed setup published new preferences: %v %s", readErr, after)
						}
					} else if readErr != nil || string(after) != original {
						t.Fatalf("failed setup changed saved configuration: %v %s", readErr, after)
					}
					staged, err := filepath.Glob(filepath.Join(dir, ".system.json.*"))
					if err != nil || len(staged) != 0 {
						t.Fatalf("setup left owned staging files: %v %v", err, staged)
					}
				})
			}
		}
	}
}
