//go:build !windows

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

	"gopkg.in/yaml.v3"
)

func nativeInstallerPhaseRun(phase string) string {
	return `set -euo pipefail
if [[ -n "${PULSE_INSTALLER_FIXTURE_BIN:-}" ]]; then
  export PATH="$PULSE_INSTALLER_FIXTURE_BIN:$PATH"
fi
python3 scripts/installtests/run_unix_contracts.py --phase ` + phase
}

func validateNativeInstallerPhaseOrder(steps []nativeGuidanceStep) error {
	indices := map[string]int{}
	names := []string{
		"Set up Go",
		"Supply GNU timeout for macOS guidance checks",
		"Verify native Unix timeout prerequisite",
		"Supply Linux server installer fixture tools on macOS",
		"Test NAS removal contracts before frontend preparation",
		"Set up Node.js for native runtime test assets",
		"Test native agent runtime",
		"Test native Unix installer contracts",
	}
	for i, step := range steps {
		for _, name := range names {
			if step.Name == name {
				if _, exists := indices[name]; exists {
					return fmt.Errorf("native installer phase step must run once: %s", name)
				}
				indices[name] = i
			}
		}
	}
	previous := -1
	for _, name := range names {
		index, exists := indices[name]
		if !exists || index <= previous {
			return fmt.Errorf("native installer preparation and both phases must stay in order: %s", name)
		}
		previous = index
	}
	supply := steps[indices[names[3]]]
	if supply.If != "runner.os == 'macOS'" || supply.Shell != "bash" || supply.ContinueOnError ||
		supply.WorkingDirectory != "" || strings.Contains(supply.Run, "GITHUB_PATH") {
		return fmt.Errorf("Linux fixture tools must not replace the native runtime tools or hide a failure")
	}
	for name, phase := range map[string]string{names[4]: "preflight", names[7]: "remaining"} {
		step := steps[indices[name]]
		if step.If != "matrix.unix" || step.Shell != "bash" || step.ContinueOnError || step.WorkingDirectory != "" ||
			strings.TrimSpace(step.Run) != nativeInstallerPhaseRun(phase) {
			return fmt.Errorf("both native installer phases must execute the exact fail-closed runner with step-local tools: %s", name)
		}
	}
	return nil
}

func nativeInstallerFixtureSteps(t *testing.T) (nativeGuidanceStep, nativeGuidanceStep, nativeGuidanceStep) {
	t.Helper()
	content, err := os.ReadFile(repoFile(".github", "workflows", "unified-agent-native.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct{ Steps []nativeGuidanceStep }
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		t.Fatal(err)
	}
	steps := workflow.Jobs["native-agent"].Steps
	if err := validateNativeInstallerPhaseOrder(steps); err != nil {
		t.Fatal(err)
	}
	var supply, preflight, installer nativeGuidanceStep
	for _, step := range steps {
		switch step.Name {
		case "Supply Linux server installer fixture tools on macOS":
			supply = step
		case "Test NAS removal contracts before frontend preparation":
			preflight = step
		case "Test native Unix installer contracts":
			installer = step
		}
	}
	return supply, preflight, installer
}

func TestNativeInstallerPhasesKeepNASBeforeFrontendAndAllChecksGating(t *testing.T) {
	content, err := os.ReadFile(repoFile(".github", "workflows", "unified-agent-native.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct{ Steps []nativeGuidanceStep }
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		t.Fatal(err)
	}
	steps := workflow.Jobs["native-agent"].Steps
	if err := validateNativeInstallerPhaseOrder(steps); err != nil {
		t.Fatal(err)
	}
	indices := map[string]int{}
	for i, step := range steps {
		indices[step.Name] = i
	}
	early := indices["Test NAS removal contracts before frontend preparation"]
	remaining := indices["Test native Unix installer contracts"]
	for _, scenario := range []string{"missing preflight", "missing remaining", "duplicate preflight", "after Node", "before tools", "Linux only", "hidden early failure", "hidden remaining failure", "all checks repeated", "no remaining checks", "nonlocal tools", "wrong directory"} {
		t.Run(scenario, func(t *testing.T) {
			changed := append([]nativeGuidanceStep(nil), steps...)
			switch scenario {
			case "missing preflight":
				changed = append(changed[:early], changed[early+1:]...)
			case "missing remaining":
				changed = append(changed[:remaining], changed[remaining+1:]...)
			case "duplicate preflight":
				changed = append(changed, changed[early])
			case "after Node":
				i := indices["Set up Node.js for native runtime test assets"]
				changed[early], changed[i] = changed[i], changed[early]
			case "before tools":
				i := indices["Supply Linux server installer fixture tools on macOS"]
				changed[early], changed[i] = changed[i], changed[early]
			case "Linux only":
				changed[early].If = "runner.os == 'Linux'"
			case "hidden early failure":
				changed[early].ContinueOnError = true
			case "hidden remaining failure":
				changed[remaining].ContinueOnError = true
			case "all checks repeated":
				changed[remaining].Run = nativeInstallerPhaseRun("all")
			case "no remaining checks":
				changed[remaining].Run = nativeInstallerPhaseRun("preflight")
			case "nonlocal tools":
				i := indices["Supply Linux server installer fixture tools on macOS"]
				changed[i].Run += "\nprintf '%s\\n' \"$fixture_bin\" >> \"$GITHUB_PATH\"\n"
			case "wrong directory":
				changed[early].WorkingDirectory = "frontend-modern"
			}
			if err := validateNativeInstallerPhaseOrder(changed); err == nil {
				t.Fatalf("accepted %s native installer phase", scenario)
			}
		})
	}
}

func TestNativeInstallerFixtureToolsAreRealAndStepLocal(t *testing.T) {
	supply, preflight, installer := nativeInstallerFixtureSteps(t)
	for _, scenario := range []string{"real tools", "symlinked fixture root", "install failure", "missing bash", "legacy bash", "missing stat", "non-GNU chmod", "missing mv", "non-GNU mv", "missing ln", "non-GNU ln", "missing readlink", "non-GNU readlink", "missing tar", "non-GNU tar", "missing awk", "non-GNU awk"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			if scenario == "symlinked fixture root" {
				// macOS temporary roots can be reached through /var -> /private/var.
				// Reproduce that legitimate alias without requiring a macOS host.
				alias := filepath.Join(t.TempDir(), "alias with spaces")
				if err := os.Symlink(root, alias); err != nil {
					t.Fatal(err)
				}
				root = alias
			}
			bin, prefix, tarPrefix, runnerTemp := filepath.Join(root, "bin"), filepath.Join(root, "brew with spaces"), filepath.Join(root, "gnu tar with spaces"), filepath.Join(root, "runner temp")
			for _, path := range []string{bin, filepath.Join(prefix, "bin"), filepath.Join(tarPrefix, "bin"), runnerTemp} {
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			brew := `#!/bin/sh
printf '%s\n' "$*" >> "$BREW_CALLS"
case "$*" in
  'install bash gnu-tar gawk') exit "${BREW_EXIT:-0}" ;;
  '--prefix bash'|'--prefix coreutils') printf '%s\n' "$TOOLS_PREFIX" ;;
  '--prefix gnu-tar') printf '%s\n' "$TAR_PREFIX" ;;
  '--prefix gawk') printf '%s\n' "$TOOLS_PREFIX" ;;
  *) exit 64 ;;
esac
`
			if err := os.WriteFile(filepath.Join(bin, "brew"), []byte(brew), 0700); err != nil {
				t.Fatal(err)
			}
			tools := []string{"bash", "stat", "chmod", "chown", "mv", "ln", "readlink", "tar", "awk"}
			installed := make(map[string]string)
			for _, name := range tools {
				leaf, toolPrefix := name, prefix
				if name != "bash" {
					leaf = "g" + name
				}
				if name == "tar" {
					toolPrefix = tarPrefix
				}
				path := filepath.Join(toolPrefix, "bin", leaf)
				installed[name] = path
				if scenario == "missing "+name {
					continue
				}
				if (scenario == "legacy bash" && name == "bash") || scenario == "non-GNU "+name {
					fake := "#!/bin/sh\nprintf 'BSD utility\\n'\n"
					if name == "bash" {
						fake = "#!/bin/sh\nexit 1\n"
					}
					if err := os.WriteFile(path, []byte(fake), 0700); err != nil {
						t.Fatal(err)
					}
				} else {
					realName := name
					if name == "awk" {
						realName = "gawk"
					}
					realTool, err := exec.LookPath(realName)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(realTool, path); err != nil {
						t.Fatal(err)
					}
				}
			}
			// Native macOS defaults must not be the fallback for Linux-only
			// archive extraction, canonicalization or atomic replacement. Real
			// GNU tools must win even when the native names are unsuitable.
			for _, name := range []string{"mv", "readlink", "tar", "awk"} {
				if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nprintf 'native BSD utility reached\\n' >&2\nexit 64\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			envFile, calls := filepath.Join(root, "github-env"), filepath.Join(root, "brew-calls")
			installExit := "0"
			if scenario == "install failure" {
				installExit = "23"
			}
			env := append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "TOOLS_PREFIX="+prefix,
				"TAR_PREFIX="+tarPrefix, "BREW_CALLS="+calls, "BREW_EXIT="+installExit, "RUNNER_TEMP="+runnerTemp, "GITHUB_ENV="+envFile)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", "-c", supply.Run)
			cmd.Env = env
			output, runErr := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("fixture prerequisite deadline: %v", ctx.Err())
			}
			if scenario != "real tools" && scenario != "symlinked fixture root" {
				want := 1
				if scenario == "install failure" {
					want = 23
				}
				exit, ok := runErr.(*exec.ExitError)
				if !ok || exit.ExitCode() != want {
					t.Fatalf("prerequisite exit %v, want %d: %s", runErr, want, output)
				}
				if _, err := os.Stat(envFile); !os.IsNotExist(err) {
					t.Fatal("failed prerequisite exported an executable path")
				}
				return
			}
			if runErr != nil {
				t.Fatalf("real tool supply: %v: %s", runErr, output)
			}
			data, err := os.ReadFile(envFile)
			if err != nil {
				t.Fatal(err)
			}
			fixtureBin := strings.TrimSuffix(strings.TrimPrefix(string(data), "PULSE_INSTALLER_FIXTURE_BIN="), "\n")
			if filepath.Dir(fixtureBin) != runnerTemp {
				t.Fatal("fixture tools escaped the owned runner directory")
			}
			entries, err := os.ReadDir(fixtureBin)
			if err != nil || len(entries) != len(tools) {
				t.Fatal("fixture supply must select only Bash/stat/chmod/chown/mv/ln/readlink/tar/awk")
			}
			for _, name := range tools {
				if target, err := os.Readlink(filepath.Join(fixtureBin, name)); err != nil || target != installed[name] {
					t.Fatalf("%s did not bind the verified installed executable", name)
				}
			}
			if data, err := os.ReadFile(calls); err != nil || string(data) != "install bash gnu-tar gawk\n--prefix bash\n--prefix coreutils\n--prefix gnu-tar\n--prefix gawk\n" {
				t.Fatal("fixture supply changed its fixed package/resource selection")
			}
			// Exercise the actual workflow step's local PATH boundary, replacing
			// only Python's containing suite to avoid recursively starting Go.
			probe := `#!/bin/sh
set -eu
test "$*" = "scripts/installtests/run_unix_contracts.py --phase $EXPECTED_PHASE" || exit 91
test "$(command -v bash)" = "$PULSE_INSTALLER_FIXTURE_BIN/bash" || exit 92
for tool in stat chmod chown mv ln readlink tar awk; do
  test "$(command -v "$tool")" = "$PULSE_INSTALLER_FIXTURE_BIN/$tool" || exit 93
done
# The expected readlink result must use a physical root too. A temporary
# directory reached through a legitimate parent symlink is not itself the
# canonical spelling (notably /var on macOS).
PROBE_ROOT="$(cd "$PROBE_ROOT" && pwd -P)"
# Exercise the actual Linux-only flags with real executables and file bytes.
# Neither a filename check nor a --version double can establish these semantics.
printf 'archive bytes\n' > "$PROBE_ROOT/source"
tar -czf "$PROBE_ROOT/source.tgz" -C "$PROBE_ROOT" source
mkdir -p "$PROBE_ROOT/extracted"
tar --no-same-owner --no-overwrite-dir -xzf "$PROBE_ROOT/source.tgz" -C "$PROBE_ROOT/extracted"
cmp "$PROBE_ROOT/source" "$PROBE_ROOT/extracted/source"
cp "$PROBE_ROOT/extracted/source" "$PROBE_ROOT/staged"
mv -T "$PROBE_ROOT/staged" "$PROBE_ROOT/published"
cmp "$PROBE_ROOT/source" "$PROBE_ROOT/published"
test ! -e "$PROBE_ROOT/staged"
# DSM Upstart removal links its manual override and restores retired config
# with GNU ln -T: a hard link that never replaces or descends into a target.
links="$PROBE_ROOT/links"
rm -rf "$links"
mkdir "$links" "$links/occupied-directory"
ln -T -- "$PROBE_ROOT/published" "$links/linked"
cmp "$PROBE_ROOT/source" "$links/linked"
printf 'concurrent\n' > "$links/occupied"
if ln -T -- "$PROBE_ROOT/published" "$links/occupied" 2>/dev/null; then exit 94; fi
test "$(cat "$links/occupied")" = concurrent
if ln -T -- "$PROBE_ROOT/published" "$links/occupied-directory" 2>/dev/null; then exit 95; fi
test ! -e "$links/occupied-directory/published"
# Removal validates the entire runtime/data layout before deleting a file.
# Its Linux readlink -m must resolve symlinked parents and missing leaves;
# BSD readlink cannot stand in for that safety boundary on a macOS runner.
mkdir "$links/config"
ln -s -- "$links/config" "$links/config-link"
test "$(readlink -m -- "$links/config-link/future/../settings")" = "$links/config/settings"
test "$(readlink -m -- "$links/config-link/../missing")" = "$links/missing"
# Exercise the NAS filter's actual octal byte range, not a --version double.
# Safe text must pass; NUL and other disallowed controls must fail. Neither
# false-positive empty matching nor dropping binary bytes is acceptable.
printf 'safe boot text\n' > "$PROBE_ROOT/boot-safe"
printf 'unsafe\000boot\n' > "$PROBE_ROOT/boot-nul"
printf 'unsafe\001boot\n' > "$PROBE_ROOT/boot-control"
awk 'BEGIN { bad = 0 } /[\000-\010\013-\037\177]/ { bad = 1 } END { exit bad }' "$PROBE_ROOT/boot-safe"
for file in boot-nul boot-control; do
  if awk 'BEGIN { bad = 0 } /[\000-\010\013-\037\177]/ { bad = 1 } END { exit bad }' "$PROBE_ROOT/$file"; then
    exit 94
  fi
done
exit "${SUITE_EXIT:-0}"
`
			if err := os.WriteFile(filepath.Join(bin, "python3"), []byte(probe), 0700); err != nil {
				t.Fatal(err)
			}
			// Supply uses native ln, while installer phases must use the verified GNU ln.
			if err := os.WriteFile(filepath.Join(bin, "ln"), []byte("#!/bin/sh\nprintf 'native BSD utility reached\\n' >&2\nexit 64\n"), 0700); err != nil {
				t.Fatal(err)
			}
			for phase, step := range map[string]nativeGuidanceStep{"preflight": preflight, "remaining": installer} {
				for _, code := range []string{"0", "23"} {
					cmd := exec.CommandContext(ctx, "bash", "-c", step.Run)
					cmd.Env = append(env, "PULSE_INSTALLER_FIXTURE_BIN="+fixtureBin, "PROBE_ROOT="+root, "SUITE_EXIT="+code, "EXPECTED_PHASE="+phase)
					output, err := cmd.CombinedOutput()
					if code == "0" && err != nil {
						t.Fatalf("fixture-local path did not reach the suite: %v: %s", err, output)
					} else if code == "23" {
						if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 23 {
							t.Fatalf("containing suite failure hidden: %v: %s", err, output)
						}
					}
				}
			}
		})
	}
}
