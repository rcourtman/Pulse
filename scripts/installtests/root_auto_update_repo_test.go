package installtests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
