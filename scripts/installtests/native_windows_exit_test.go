package installtests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

type nativeWindowsExitStep struct {
	Name            string
	If              string
	Shell           string
	ContinueOnError bool `yaml:"continue-on-error"`
	Run             string
}

func nativeWindowsExitSteps(t *testing.T) []nativeWindowsExitStep {
	t.Helper()
	content, err := os.ReadFile(repoFile(".github", "workflows", "unified-agent-native.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct{ Steps []nativeWindowsExitStep }
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		t.Fatal(err)
	}
	return workflow.Jobs["native-agent"].Steps
}

// These are the multi-command PowerShell steps. Actions checks only the final
// LASTEXITCODE, so every executable must be checked before a later one runs.
func nativeWindowsExitGuards(steps []nativeWindowsExitStep) ([]string, error) {
	required := map[string]int{
		"Build and execute native Windows agent":    3,
		"Exercise native Windows service lifecycle": 4,
	}
	var guards []string
	for _, step := range steps {
		want, ok := required[step.Name]
		if !ok {
			continue
		}
		shell := "pwsh"
		if step.Name == "Exercise native Windows service lifecycle" {
			shell = "powershell"
		}
		if step.If != "${{ !matrix.unix }}" || step.Shell != shell || step.ContinueOnError {
			return nil, fmt.Errorf("%s must be a required native Windows PowerShell step", step.Name)
		}
		found := 0
		lines := strings.Split(step.Run, "\n")
		for i := 0; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if !strings.HasPrefix(line, "go build ") &&
				!strings.HasPrefix(line, `.\pulse-agent-native.exe `) &&
				!strings.HasPrefix(line, "& ./scripts/installtests/windows_agent_lifecycle.ps1 ") {
				continue
			}
			// The lifecycle invocation spans backtick-continued argument lines.
			for strings.HasSuffix(strings.TrimSpace(lines[i]), "`") && i+1 < len(lines) {
				i++
			}
			if i+1 >= len(lines) {
				return nil, fmt.Errorf("%s ends without checking %s", step.Name, line)
			}
			guard := strings.TrimSpace(lines[i+1])
			if !strings.HasPrefix(guard, `if ($LASTEXITCODE -ne 0) { throw "`) ||
				!strings.HasSuffix(guard, `(exit $LASTEXITCODE)." }`) {
				return nil, fmt.Errorf("%s must immediately check %s before any later command", step.Name, line)
			}
			guards = append(guards, guard)
			found++
		}
		if found != want {
			return nil, fmt.Errorf("%s checked %d native commands, want %d", step.Name, found, want)
		}
		delete(required, step.Name)
	}
	if len(required) != 0 {
		return nil, fmt.Errorf("missing required native Windows steps: %v", required)
	}
	return guards, nil
}

func nativeWindowsHarnessExitGuards(script string) ([]string, error) {
	commands := []string{
		`$version = ((& $AgentPath --version`,
		`& $script:powerShellPath -NoLogo`,
		`$installedVersion = ((& "$env:ProgramFiles\Pulse\pulse-agent.exe" --version`,
		`$recovery = (& sc.exe qfailure `,
		`$failureFlag = (& sc.exe qfailureflag `,
	}
	guardPattern := regexp.MustCompile(`^\s*if \(\$LASTEXITCODE -ne 0\) \{\s*throw "[^"\n]*\$LASTEXITCODE[^"\n]*"\s*\}`)
	var guards []string
	for _, command := range commands {
		i := strings.Index(script, command)
		if i < 0 {
			return nil, fmt.Errorf("missing lifecycle native command %s", command)
		}
		newline := strings.IndexByte(script[i:], '\n')
		if newline < 0 {
			return nil, fmt.Errorf("lifecycle command %s has no exit check", command)
		}
		guard := guardPattern.FindString(script[i+newline+1:])
		if guard == "" {
			return nil, fmt.Errorf("lifecycle command %s must fail before interpreting its output", command)
		}
		guards = append(guards, strings.TrimSpace(guard))
	}
	return guards, nil
}

func TestWindowsAgentLifecycleNativeExitChecksStopFollowingCommands(t *testing.T) {
	powerShell := nativeInstallerPowerShell(t)
	guards, err := nativeWindowsExitGuards(nativeWindowsExitSteps(t))
	if err != nil {
		t.Fatal(err)
	}
	harness, err := os.ReadFile(repoFile("scripts", "installtests", "windows_agent_lifecycle.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	harnessGuards, err := nativeWindowsHarnessExitGuards(string(harness))
	if err != nil {
		t.Fatal(err)
	}
	guards = append(guards, harnessGuards...)
	nativeExit := func(code int) string {
		if runtime.GOOS == "windows" {
			return fmt.Sprintf(`cmd.exe /d /c "exit /b %d"`, code)
		}
		return fmt.Sprintf(`sh -c 'exit %d'`, code)
	}
	run := func(t *testing.T, guard string, code int) ([]byte, error) {
		t.Helper()
		// Match Actions' error preference and final native exit propagation.
		// The second successful native command reproduces the masking case.
		script := "$ErrorActionPreference = 'Stop'\n$Label = 'Controlled installer'\n" + nativeExit(code) + "\n" + guard + "\n" +
			nativeExit(0) + "\nWrite-Output 'FOLLOWING_COMMAND_RAN'\nexit $LASTEXITCODE\n"
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, powerShell, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
		return cmd.CombinedOutput()
	}
	t.Run("unguarded parent control", func(t *testing.T) {
		output, err := run(t, "", 23)
		if err != nil || !strings.Contains(string(output), "FOLLOWING_COMMAND_RAN") {
			t.Fatalf("unguarded failed command did not demonstrate later-success masking: %v\n%s", err, output)
		}
	})
	for i, guard := range guards {
		t.Run(fmt.Sprintf("command %d", i+1), func(t *testing.T) {
			output, err := run(t, guard, 23)
			if err == nil || strings.Contains(string(output), "FOLLOWING_COMMAND_RAN") ||
				(!strings.Contains(string(output), "exit 23") && !strings.Contains(string(output), "exit code 23")) {
				t.Fatalf("failed native command must stop immediately and retain its exit: %v\n%s", err, output)
			}
			output, err = run(t, guard, 0)
			if err != nil || !strings.Contains(string(output), "FOLLOWING_COMMAND_RAN") {
				t.Fatalf("successful native command must continue: %v\n%s", err, output)
			}
		})
	}
}
