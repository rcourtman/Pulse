//go:build !windows

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Run the real main/CLI dispatch inside the compiled test executable, before
// any agent module can start. No synthetic token-reader output is substituted.
func TestCollectorLifecycleTokenReaderProcess(t *testing.T) {
	if os.Getenv("PULSE_TEST_COLLECTOR_TOKEN_READER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) && os.Args[i+1] == collectorReadTokenCommand {
			os.Args = append(os.Args[:1], os.Args[i+1:]...)
			main()
			os.Exit(0) // Do not append the test harness's PASS line to the value.
		}
	}
	os.Exit(90)
}

func TestCollectorTokenReaderInstallerIngress(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	var functions strings.Builder
	for _, name := range []string{"portable_path_uid", "portable_path_mode", "trusted_lifecycle_regular_file",
		"trusted_private_lifecycle_regular_file", "read_collector_token_file_safely"} {
		function := regexp.MustCompile(`(?ms)^` + name + `\(\) \{\n.*?^\}`).Find(source)
		if len(function) == 0 {
			t.Fatalf("missing production function %s", name)
		}
		functions.Write(function)
		functions.WriteByte('\n')
	}
	_, ingress, ok := strings.Cut(string(source), "# Read token from file if --token-file was provided\n")
	if !ok {
		t.Fatal("missing production token ingress")
	}
	ingress, _, ok = strings.Cut(ingress, `if [[ -n "$PROXMOX_TYPE"`)
	if !ok {
		t.Fatal("missing production token ingress boundary")
	}
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for name, encoded := range map[string]string{
		"bare": "synthetic-secret", "LF": "synthetic-secret\n",
		"normalized": " \tsynthetic-secret\r\n", "literal": "PULSE_TOKEN='synthetic-secret'",
		"UTF8": "synthetic-caf\u00e9", "limit": strings.Repeat("s", 4096),
	} {
		for _, option := range []string{"collector", "action"} {
			t.Run(name+"/"+option, func(t *testing.T) {
				directory := t.TempDir()
				path := filepath.Join(directory, "selected.token")
				if err := os.WriteFile(path, []byte(encoded), 0600); err != nil {
					t.Fatal(err)
				}
				before, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				// Only executable discovery is fixture-wired. File/parent trust,
				// the Go reader, shell substitution and both ingress gates are real.
				reader := filepath.Join(directory, "reader")
				adapter := "#!/usr/bin/env bash\nset -eu\nprintf 'invoked\\n' >> \"$READER_EVENTS\"\n" +
					"exec \"$READER_TEST_BINARY\" -test.run='^TestCollectorLifecycleTokenReaderProcess$' -- \"$@\"\n"
				if err := os.WriteFile(reader, []byte(adapter), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(reader, 0755); err != nil {
					t.Fatal(err)
				}
				script := `set -euo pipefail
STATE_DIR="$FIXTURE"
RUNTIME_TOKEN_FILE="" PULSE_TOKEN="" ACTION_TOKEN=""
TOKEN_FILE_PATH="" ACTION_TOKEN_FILE_PATH="" EXIT_MISSING_ARGS=16
LEAST_PRIVILEGE_USER="$(id -u)"
collector_lifecycle_binary() { printf '%s\n' "$READER"; }
fail() { printf '%s\n' "$1" >&2; exit "$2"; }
if [[ "$TOKEN_OPTION" == collector ]]; then TOKEN_FILE_PATH="$SELECTED"; else ACTION_TOKEN_FILE_PATH="$SELECTED"; fi
` + functions.String() + ingress + `printf '%s' "$PULSE_TOKEN$ACTION_TOKEN"`
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "bash", "-c", script)
				cmd.Env = append(os.Environ(), "PULSE_TEST_COLLECTOR_TOKEN_READER=1", "FIXTURE="+directory,
					"SELECTED="+path, "READER="+reader, "READER_TEST_BINARY="+testBinary,
					"READER_EVENTS="+filepath.Join(directory, "events"), "TOKEN_OPTION="+option)
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				err = cmd.Run()
				if strings.ContainsRune(encoded, 0) {
					if err == nil || cmd.ProcessState.ExitCode() != 16 || stdout.Len() != 0 {
						t.Fatal("malformed credential was not refused without output at installer ingress")
					}
					if strings.Contains(stderr.String(), "synthetic") || strings.Contains(stderr.String(), "ignored null byte") {
						t.Fatal("malformed credential reached shell output or lossy substitution")
					}
				} else if err != nil || stdout.String() != strings.TrimSpace(encoded) || stderr.Len() != 0 {
					t.Fatal("valid credential did not reach the installer unchanged after existing normalization")
				}
				events, err := os.ReadFile(filepath.Join(directory, "events"))
				if err != nil || string(events) != "invoked\n" {
					t.Fatal("production Go command was not invoked exactly once")
				}
				after, err := os.Stat(path)
				if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
					t.Fatal("token ingress replaced the caller's file or changed its mode")
				}
				content, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(content, []byte(encoded)) {
					t.Fatal("token ingress changed the caller's credential bytes")
				}
			})
		}
	}
}
