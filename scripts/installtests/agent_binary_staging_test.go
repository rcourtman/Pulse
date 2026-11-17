//go:build unix

package installtests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Execute the shipped staging/upgrade/publication statements with real fixture
// file operations. Only failure injection and service/process observations are
// doubles; no native service, network, installer entrypoint or customer files
// are operated here. In particular, do not substitute a model of the ordering.
func TestAgentInstallerStagesCollectorBeforeStop(t *testing.T) {
	installer, err := os.ReadFile("../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(installer), "# Prepare the ordinary collector replacement before any upgrade stop/kill.")
	end := strings.Index(string(installer), "# --- Service Installation ---")
	if start < 0 || end <= start {
		t.Fatal("collector publication statements not found")
	}
	publication := string(installer[start:end])
	functions := extractInstallShellFunction(t, "cleanup") + "\n" +
		extractInstallShellFunction(t, "stage_agent_binary_for_install") + "\n"
	oldBinary := "#!/bin/sh\nprintf 'old-fixture-version\\n'\n"
	newBinary := "#!/bin/sh\nprintf 'new-fixture-version\\n'\n"
	for _, upgrade := range []bool{false, true} {
		for _, fault := range []string{"none", "directory-creation", "allocation", "partial-copy", "executable-mode", "destination-directory", "publication"} {
			t.Run(fmt.Sprintf("upgrade=%t/%s", upgrade, fault), func(t *testing.T) {
				root := t.TempDir()
				installDir := filepath.Join(root, "installation with spaces")
				if err := os.Mkdir(installDir, 0755); err != nil {
					t.Fatal(err)
				}
				installed := filepath.Join(installDir, "pulse-agent")
				if fault == "destination-directory" {
					if err := os.Mkdir(installed, 0755); err != nil {
						t.Fatal(err)
					}
					writeTestFile(t, filepath.Join(installed, "unrelated"), []byte("retain directory contents"), 0600)
				} else if upgrade {
					writeTestFile(t, installed, []byte(oldBinary), 0755)
				}
				state := map[string]string{
					"agent-id": "original identity\n", "runtime.token": "synthetic saved credential\n",
					"connection.env": "original saved connection\n", "agent.service": "original unit\n",
					"agent.timer": "original timer\n", "retained-install.sh": "original recovery script\n",
				}
				for name, body := range state {
					writeTestFile(t, filepath.Join(root, name), []byte(body), 0600)
				}
				download := filepath.Join(root, "verified-download")
				writeTestFile(t, download, []byte(newBinary), 0755)
				trace := filepath.Join(root, "operations")
				writeTestFile(t, trace, nil, 0600)
				script := `set -euo pipefail
TMP_FILES=("$TMP_BIN")
SAFE_PROFILE_ACTION=none
PRIVILEGED_HELPER_ENABLED=false
ACTION_RUNNER_ENABLED=false
UPDATE_ONLY=false
EXIT_GENERAL=1
EXIT_MISSING_ARGS=2
AGENT_NAME=pulse-agent
BINARY_NAME=pulse-agent
NEW_VERSION=new-fixture-version
log_info() { :; }
fail() { printf '%s\n' "$1" >&2; exit "$2"; }
mkdir() {
    printf 'directory\n' >> "$TRACE"
    [[ "$FAULT" != directory-creation ]] || return 1
    command mkdir "$@"
}
mktemp() {
    printf 'allocate\n' >> "$TRACE"
    [[ "$FAULT" != allocation ]] || return 1
    command mktemp "$@"
}
cp() {
    printf 'copy\n' >> "$TRACE"
    if [[ "$FAULT" == partial-copy ]]; then
        printf 'incomplete replacement' > "$2"
        return 1
    fi
    command cp "$@"
}
chmod() {
    printf 'mode\n' >> "$TRACE"
    [[ "$FAULT" != executable-mode ]] || return 1
    command chmod "$@"
}
stop_existing_agent_service() {
    # Copy and mode must already be complete when disruption becomes possible.
    [[ -f "$STAGED_AGENT_BIN" && -x "$STAGED_AGENT_BIN" ]]
    cmp "$TMP_BIN" "$STAGED_AGENT_BIN"
    printf 'stop\n' >> "$TRACE"
}
systemctl() { return 1; }
pkill() { printf 'kill\n' >> "$TRACE"; }
sleep() { :; }
mv() {
    # The ordinary collector publication must not perform another /tmp copy.
    [[ "${1%/*}" == "$INSTALL_DIR" && "$2" == "$INSTALL_DIR/pulse-agent" ]]
    printf 'publish\n' >> "$TRACE"
    [[ "$FAULT" != publication ]] || return 1
    command mv "$@"
}
` + functions + "\ntrap cleanup EXIT\n" + publication + "\nprintf 'publication-complete\\n'\n"
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "bash", "-c", script)
				cmd.Env = append(os.Environ(), "INSTALL_DIR="+installDir, "TMP_BIN="+download,
					"TRACE="+trace, "FAULT="+fault)
				output, runErr := cmd.CombinedOutput()
				exit := 0
				if runErr != nil {
					var exitErr *exec.ExitError
					if !errors.As(runErr, &exitErr) {
						t.Fatalf("fixture execution: %v\n%s", runErr, output)
					}
					exit = exitErr.ExitCode()
				}
				if ctx.Err() != nil || (exit == 0) != (fault == "none") {
					t.Fatalf("fault=%s exit=%d context=%v error=%v\n%s", fault, exit, ctx.Err(), runErr, output)
				}
				operations, err := os.ReadFile(trace)
				if err != nil {
					t.Fatal(err)
				}
				sequence := string(operations)
				if fault != "none" && fault != "publication" {
					for _, forbidden := range []string{"stop\n", "kill\n", "publish\n"} {
						if strings.Contains(sequence, forbidden) {
							t.Fatalf("staging failure disrupted installation (%s):\n%s\n%s", forbidden, sequence, output)
						}
					}
					if !strings.Contains(string(output), "has not been stopped or replaced; saved state is retained") {
						t.Fatalf("missing early refusal diagnostic:\n%s", output)
					}
				} else {
					want := "directory\nallocate\ncopy\nmode\n"
					if upgrade {
						want += "stop\nkill\n"
					}
					// The existing publication section also ensures the directory.
					want += "directory\npublish\n"
					if sequence != want {
						t.Fatalf("publication order = %q, want %q\n%s", sequence, want, output)
					}
				}
				if strings.Contains(string(output), "publication-complete") != (fault == "none") {
					t.Fatalf("completion did not match publication result:\n%s", output)
				}
				if fault == "destination-directory" {
					body, err := os.ReadFile(filepath.Join(installed, "unrelated"))
					if err != nil || string(body) != "retain directory contents" {
						t.Fatalf("destination directory changed: %v %q", err, body)
					}
				} else if fault == "none" || upgrade {
					body, err := os.ReadFile(installed)
					want := oldBinary
					if fault == "none" {
						want = newBinary
					}
					if err != nil || string(body) != want {
						t.Fatalf("installed bytes changed incorrectly: %v %q", err, body)
					}
					info, err := os.Stat(installed)
					if err != nil || info.Mode().Perm() != 0755 {
						t.Fatalf("installed mode not retained: %v %v", info, err)
					}
				} else if _, err := os.Lstat(installed); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("failed fresh install created a collector: %v", err)
				}
				for name, want := range state {
					body, err := os.ReadFile(filepath.Join(root, name))
					info, statErr := os.Stat(filepath.Join(root, name))
					if err != nil || string(body) != want || statErr != nil || info.Mode().Perm() != 0600 {
						t.Errorf("saved %s changed: %v %v", name, err, statErr)
					}
				}
				stages, err := filepath.Glob(filepath.Join(installDir, ".pulse-agent.install.*"))
				if err != nil || len(stages) != 0 {
					t.Errorf("owned staging not cleaned: %v %v", stages, err)
				}
			})
		}
	}
}
