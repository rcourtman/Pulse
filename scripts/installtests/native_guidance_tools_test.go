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

type nativeGuidanceStep struct {
	Name, If, Shell, Run string
}

func nativeGuidanceToolSteps(content []byte) (nativeGuidanceStep, nativeGuidanceStep, error) {
	var workflow struct {
		Jobs map[string]struct{ Steps []nativeGuidanceStep }
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		return nativeGuidanceStep{}, nativeGuidanceStep{}, err
	}
	steps := workflow.Jobs["native-agent"].Steps
	bootstrapIndex, verifyIndex, installerIndex := -1, -1, -1
	for i, step := range steps {
		switch step.Name {
		case "Supply GNU timeout for macOS guidance checks":
			bootstrapIndex = i
		case "Verify native Unix timeout prerequisite":
			verifyIndex = i
		case "Test native Unix installer contracts":
			installerIndex = i
		}
	}
	if bootstrapIndex < 0 || verifyIndex <= bootstrapIndex || installerIndex <= verifyIndex {
		return nativeGuidanceStep{}, nativeGuidanceStep{}, fmt.Errorf("native guidance needs GNU tool supply and verification before installer tests")
	}
	bootstrap, verify := steps[bootstrapIndex], steps[verifyIndex]
	if bootstrap.If != "runner.os == 'macOS'" || verify.If != "matrix.unix" || steps[installerIndex].If != "matrix.unix" || bootstrap.Shell != "bash" || verify.Shell != "bash" {
		return bootstrap, verify, fmt.Errorf("GNU tool preparation must cover macOS, verification and installer tests must cover every native Unix job")
	}
	return bootstrap, verify, nil
}

func TestNativeGuidanceToolsSupplyAndVerifyRealGNUTimeout(t *testing.T) {
	content, err := os.ReadFile(repoFile(".github", "workflows", "unified-agent-native.yml"))
	if err != nil {
		t.Fatal(err)
	}
	bootstrap, verify, err := nativeGuidanceToolSteps(content)
	if err != nil {
		t.Fatal(err)
	}
	realTimeout, err := exec.LookPath("timeout")
	if err != nil {
		t.Fatal("native Unix verification requires the real GNU timeout prerequisite")
	}
	for _, scenario := range []string{"real GNU tool", "install failure", "missing tool", "non-GNU tool"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			bin, prefix := filepath.Join(root, "bin"), filepath.Join(root, "coreutils with spaces")
			gnuBin := filepath.Join(prefix, "libexec", "gnubin")
			for _, dir := range []string{bin, gnuBin} {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			// Only Homebrew's installer is simulated. The positive timeout is
			// the real executable; the copied TERM/KILL recipes remain unchanged.
			const brew = `#!/bin/sh
printf '%s\n' "$*" >> "$BREW_CALLS"
case "$*" in
  'install coreutils') exit "${BREW_EXIT:-0}" ;;
  '--prefix coreutils') printf '%s\n' "$COREUTILS_PREFIX" ;;
  *) exit 64 ;;
esac
`
			if err := os.WriteFile(filepath.Join(bin, "brew"), []byte(brew), 0700); err != nil {
				t.Fatal(err)
			}
			tool := filepath.Join(gnuBin, "timeout")
			switch scenario {
			case "real GNU tool", "install failure":
				if err := os.Symlink(realTimeout, tool); err != nil {
					t.Fatal(err)
				}
			case "non-GNU tool":
				if err := os.WriteFile(tool, []byte("#!/bin/sh\nprintf 'BSD timeout\\n'\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			pathFile, callsFile := filepath.Join(root, "github-path"), filepath.Join(root, "brew-calls")
			installExit := "0"
			if scenario == "install failure" {
				installExit = "23"
			}
			env := append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "GITHUB_PATH="+pathFile,
				"COREUTILS_PREFIX="+prefix, "BREW_CALLS="+callsFile, "BREW_EXIT="+installExit)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", "-c", bootstrap.Run)
			cmd.Env = env
			output, runErr := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("prerequisite observation did not finish: %v", ctx.Err())
			}
			if scenario != "real GNU tool" {
				wantExit := 1
				if scenario == "install failure" {
					wantExit = 23
				}
				exit, ok := runErr.(*exec.ExitError)
				if !ok || exit.ExitCode() != wantExit {
					t.Fatalf("prerequisite exit %v, want %d: %s", runErr, wantExit, output)
				}
				if _, err := os.Stat(pathFile); !os.IsNotExist(err) {
					t.Fatal("failed prerequisite exported an executable search path")
				}
				return
			}
			if runErr != nil {
				t.Fatalf("GNU prerequisite failed: %v: %s", runErr, output)
			}
			path, err := os.ReadFile(pathFile)
			if err != nil || string(path) != gnuBin+"\n" {
				t.Fatal("bootstrap did not export the exact installed gnubin path")
			}
			calls, err := os.ReadFile(callsFile)
			if err != nil || string(calls) != "install coreutils\n--prefix coreutils\n" {
				t.Fatal("bootstrap changed its fixed Homebrew dependency")
			}
			cmd = exec.CommandContext(ctx, "bash", "-c", verify.Run)
			cmd.Env = append(env, "PATH="+strings.TrimSpace(string(path))+":"+os.Getenv("PATH"))
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("next workflow step cannot use supplied GNU timeout: %v: %s", err, output)
			}
			cmd = exec.CommandContext(ctx, "bash", "-c", verify.Run)
			cmd.Env = append(env, "PATH="+bin+":"+os.Getenv("PATH"))
			if err := os.WriteFile(filepath.Join(bin, "timeout"), []byte("#!/bin/sh\nprintf 'BSD timeout\\n'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			if output, err := cmd.CombinedOutput(); err == nil {
				t.Fatalf("Unix prerequisite probe accepted a non-GNU timeout: %s", output)
			}
		})
	}
}
