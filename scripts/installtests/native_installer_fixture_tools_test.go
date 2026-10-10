//go:build !windows

package installtests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func nativeInstallerFixtureSteps(t *testing.T) (nativeGuidanceStep, nativeGuidanceStep) {
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
	runtimeAt, supplyAt, installerAt := -1, -1, -1
	for i, step := range steps {
		switch step.Name {
		case "Test native agent runtime":
			runtimeAt = i
		case "Supply Linux server installer fixture tools on macOS":
			supplyAt = i
		case "Test native Unix installer contracts":
			installerAt = i
		}
	}
	if runtimeAt < 0 || supplyAt <= runtimeAt || installerAt <= supplyAt {
		t.Fatal("Linux fixture tool supply must follow native runtime testing and precede Unix installer checks")
	}
	supply, installer := steps[supplyAt], steps[installerAt]
	if supply.If != "runner.os == 'macOS'" || supply.Shell != "bash" || installer.If != "matrix.unix" || installer.Shell != "bash" ||
		strings.Contains(supply.Run, "GITHUB_PATH") || !strings.Contains(installer.Run, `export PATH="$PULSE_INSTALLER_FIXTURE_BIN:$PATH"`) ||
		!strings.Contains(installer.Run, "python3 scripts/installtests/run_unix_contracts.py") {
		t.Fatal("Linux tools must stay local to complete Unix installer qualification")
	}
	return supply, installer
}

func TestNativeInstallerFixtureToolsAreRealAndStepLocal(t *testing.T) {
	supply, installer := nativeInstallerFixtureSteps(t)
	for _, scenario := range []string{"real tools", "install failure", "missing bash", "legacy bash", "missing stat", "non-GNU chmod", "missing mv", "non-GNU mv", "missing tar", "non-GNU tar"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			bin, prefix, tarPrefix, runnerTemp := filepath.Join(root, "bin"), filepath.Join(root, "brew with spaces"), filepath.Join(root, "gnu tar with spaces"), filepath.Join(root, "runner temp")
			for _, path := range []string{bin, filepath.Join(prefix, "bin"), filepath.Join(tarPrefix, "bin"), runnerTemp} {
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			brew := `#!/bin/sh
printf '%s\n' "$*" >> "$BREW_CALLS"
case "$*" in
  'install bash gnu-tar') exit "${BREW_EXIT:-0}" ;;
  '--prefix bash'|'--prefix coreutils') printf '%s\n' "$TOOLS_PREFIX" ;;
  '--prefix gnu-tar') printf '%s\n' "$TAR_PREFIX" ;;
  *) exit 64 ;;
esac
`
			if err := os.WriteFile(filepath.Join(bin, "brew"), []byte(brew), 0700); err != nil {
				t.Fatal(err)
			}
			tools := []string{"bash", "stat", "chmod", "chown", "mv", "tar"}
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
					realTool, err := exec.LookPath(name)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(realTool, path); err != nil {
						t.Fatal(err)
					}
				}
			}
			// Native macOS defaults must not be the fallback for Linux-only
			// archive extraction or atomic replacement. The supplied real GNU
			// tools below must win even when the native names are unsuitable.
			for _, name := range []string{"mv", "tar"} {
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
			if scenario != "real tools" {
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
				t.Fatal("fixture supply must select only Bash/stat/chmod/chown/mv/tar")
			}
			for _, name := range tools {
				if target, err := os.Readlink(filepath.Join(fixtureBin, name)); err != nil || target != installed[name] {
					t.Fatalf("%s did not bind the verified installed executable", name)
				}
			}
			if data, err := os.ReadFile(calls); err != nil || string(data) != "install bash gnu-tar\n--prefix bash\n--prefix coreutils\n--prefix gnu-tar\n" {
				t.Fatal("fixture supply changed its fixed package/resource selection")
			}
			// Exercise the actual workflow step's local PATH boundary, replacing
			// only Python's containing suite to avoid recursively starting Go.
			probe := `#!/bin/sh
set -eu
test "$*" = 'scripts/installtests/run_unix_contracts.py' || exit 91
test "$(command -v bash)" = "$PULSE_INSTALLER_FIXTURE_BIN/bash" || exit 92
for tool in stat chmod chown mv tar; do
  test "$(command -v "$tool")" = "$PULSE_INSTALLER_FIXTURE_BIN/$tool" || exit 93
done
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
exit "${SUITE_EXIT:-0}"
`
			if err := os.WriteFile(filepath.Join(bin, "python3"), []byte(probe), 0700); err != nil {
				t.Fatal(err)
			}
			for _, code := range []string{"0", "23"} {
				cmd := exec.CommandContext(ctx, "bash", "-c", installer.Run)
				cmd.Env = append(env, "PULSE_INSTALLER_FIXTURE_BIN="+fixtureBin, "PROBE_ROOT="+root, "SUITE_EXIT="+code)
				output, err := cmd.CombinedOutput()
				if code == "0" && err != nil {
					t.Fatalf("fixture-local path did not reach the suite: %v: %s", err, output)
				} else if code == "23" {
					if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 23 {
						t.Fatalf("containing suite failure hidden: %v: %s", err, output)
					}
				}
			}
		})
	}
}
