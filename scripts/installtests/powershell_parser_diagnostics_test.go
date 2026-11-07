package installtests

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPowerShellParserDiagnosticCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkout ' $()", "install.ps1")
	cmd := newPowerShellParserCheck("pwsh", path)
	if !reflect.DeepEqual(cmd.Args, []string{"pwsh", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", powerShellParserCommand}) {
		t.Fatalf("parser command changed: %q", cmd.Args)
	}
	if strings.Contains(cmd.Args[5], path) || !strings.Contains(strings.Join(cmd.Env, "\n"), "PULSE_PARSER_CHECK_PATH="+path) {
		t.Fatal("script path must be an environment value, never interpolated PowerShell")
	}
	parse := strings.Index(powerShellParserCommand, "[System.Management.Automation.Language.Parser]::ParseFile(")
	flush := strings.Index(powerShellParserCommand, "[Console]::Out.Flush()")
	if flush < 0 || parse <= flush || strings.Count(powerShellParserCommand, "::ParseFile(") != 1 ||
		!strings.HasSuffix(powerShellParserCommand, "if ($errors.Count) { $errors | ForEach-Object { Write-Error $_.ToString() }; exit 1 }") {
		t.Fatal("identity must flush before the single unchanged fail-closed parser check")
	}
	for _, field := range []string{"powershell_version", "edition", "dotnet_version", "process_bits", "process_architecture", "engine_version", "engine_mvid", "engine_sha256", "PULSE_PARSER_RUNTIME unavailable"} {
		if !strings.Contains(powerShellParserCommand[:flush], field) {
			t.Errorf("missing runtime observation or explicit unavailable marker: %s", field)
		}
	}
}

// Controlled ordinary exits exercise the observer, not a PowerShell crash or
// installer. This same test binary works on Unix and Windows without a shell.
func TestPowerShellParserDiagnosticObservations(t *testing.T) {
	if mode := os.Getenv("PULSE_PARSER_OBSERVER_CONTROL"); mode != "" {
		file, err := os.OpenFile(os.Getenv("PULSE_PARSER_OBSERVER_COUNT"), os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			os.Exit(91)
		}
		fmt.Fprintln(file, "one invocation")
		file.Close()
		switch mode {
		case "success":
			fmt.Print("PULSE_PARSER_RUNTIME controlled-runtime\n")
		case "failure":
			fmt.Print("PULSE_PARSER_RUNTIME controlled-runtime\ncontrolled parser error\n")
			os.Exit(23)
		case "before-runtime":
			os.Exit(29)
		case "unavailable-runtime":
			fmt.Print("PULSE_PARSER_RUNTIME unavailable\ncontrolled parser error\n")
			os.Exit(31)
		default:
			os.Exit(92)
		}
		os.Exit(0)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		mode string
		exit int
	}{
		{"success", 0}, {"failure", 23}, {"before-runtime", 29}, {"unavailable-runtime", 31},
	} {
		t.Run(scenario.mode, func(t *testing.T) {
			dir := t.TempDir()
			target, count := filepath.Join(dir, "target.ps1"), filepath.Join(dir, "count")
			content := []byte("# controlled source, not executed\n")
			for path, data := range map[string][]byte{target: content, count: nil} {
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command(executable, "-test.run=^TestPowerShellParserDiagnosticObservations$")
			cmd.Env = append(os.Environ(), "PULSE_PARSER_OBSERVER_CONTROL="+scenario.mode, "PULSE_PARSER_OBSERVER_COUNT="+count)
			var records []string
			err := observePowerShellParser(cmd, target, func(format string, args ...any) {
				records = append(records, fmt.Sprintf(format, args...))
			})
			if scenario.exit == 0 {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != scenario.exit {
					t.Fatalf("observer replaced the child exit %d: %v", scenario.exit, err)
				}
			}
			invocations, err := os.ReadFile(count)
			if err != nil || string(invocations) != "one invocation\n" {
				t.Fatalf("observer did not execute exactly once: %q, %v", invocations, err)
			}
			if len(records) < 2 || !strings.Contains(records[0], "executable=(sha256=") ||
				!strings.Contains(records[0], fmt.Sprintf("script=(sha256=%x bytes=%d)", sha256.Sum256(content), len(content))) ||
				!strings.Contains(records[1], fmt.Sprintf("exit_code=%d", scenario.exit)) || strings.Contains(strings.Join(records, "\n"), dir) {
				t.Fatalf("missing source/result identity or unexpected private path: %q", records)
			}
			if scenario.mode == "before-runtime" {
				if len(records) != 2 || !strings.Contains(records[1], "output_bytes=0") {
					t.Fatalf("invented startup output: %q", records)
				}
			} else if len(records) != 3 || !strings.HasPrefix(records[2], "PULSE_PARSER_RUNTIME ") ||
				(scenario.exit != 0 && !strings.Contains(records[2], "controlled parser error")) {
				t.Fatalf("lost child output: %q", records)
			}
		})
	}
}

func TestPowerShellParserDiagnosticStartFailure(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	cmd := exec.Command(missing)
	var records []string
	err := observePowerShellParser(cmd, missing, func(format string, args ...any) {
		records = append(records, fmt.Sprintf(format, args...))
	})
	if err == nil || cmd.ProcessState != nil || len(records) != 2 ||
		!strings.Contains(records[0], "executable=(unavailable) script=(unavailable)") ||
		!strings.Contains(records[1], "exit_code=-1 state=unavailable output_bytes=0") {
		t.Fatalf("start failure became execution evidence: err=%v records=%q", err, records)
	}
}
