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
	testAgentInstallerStaging(t, false)
}

func TestAgentInstallerStagesHelperBeforeStop(t *testing.T) {
	testAgentInstallerStaging(t, true)
}

func testAgentInstallerStaging(t *testing.T, helper bool) {
	t.Helper()
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
		extractInstallShellFunction(t, "stage_agent_binary_for_install") + "\n" +
		extractInstallShellFunction(t, "stage_privileged_helper_for_install") + "\n" +
		extractInstallShellFunction(t, "validate_staged_agent_url") + "\n" +
		extractInstallShellFunction(t, "fail") + "\n" +
		extractInstallShellFunction(t, "json_event") + "\n"
	oldBinary := "#!/bin/sh\nprintf 'old-fixture-version\\n'\n"
	newBinary := `#!/bin/sh
case "$1" in
    --validate-url)
        [ "$#" -eq 4 ] && [ "$2" = --url ] &&
            [ "$3" = https://pulse.example.invalid ] &&
            [ "$4" = --allow-plaintext-http=false ] || exit 95
        case "$0" in "$INSTALL_DIR"/.pulse-agent.install.*) ;; *) exit 95 ;; esac
        printf 'destination-admission\n' >> "$TRACE"
        [ "$FAULT" != destination-command-failure ] || exit 86
        ;;
    --version) printf 'new-fixture-version\n' ;;
    *) exit 95 ;;
esac
`
	oldHelper := "#!/bin/sh\nprintf 'old-helper-version\\n'\n"
	newHelper := "#!/bin/sh\nprintf 'new-helper-version\\n'\n"
	faults := []string{"none", "directory-creation", "allocation", "partial-transfer", "executable-mode", "destination-directory", "destination-command-failure", "destination-not-executable", "publication"}
	if helper {
		faults = append(faults, "helper-directory-creation", "helper-directory-owner", "helper-directory-mode",
			"helper-allocation", "helper-partial-transfer", "helper-owner", "helper-mode",
			"helper-destination-directory", "helper-publication")
	}
	for _, upgrade := range []bool{false, true} {
		for _, fault := range faults {
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
				helperDir := filepath.Join(root, "helpers with spaces")
				installedHelper := filepath.Join(helperDir, "pulse-agent-helper")
				if helper && (upgrade || fault == "helper-destination-directory") {
					if err := os.Mkdir(helperDir, 0755); err != nil {
						t.Fatal(err)
					}
					if fault == "helper-destination-directory" {
						if err := os.Mkdir(installedHelper, 0755); err != nil {
							t.Fatal(err)
						}
						writeTestFile(t, filepath.Join(installedHelper, "unrelated"), []byte("retain helper directory contents"), 0600)
					} else {
						writeTestFile(t, installedHelper, []byte(oldHelper), 0755)
					}
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
				downloadInfo, err := os.Stat(download)
				if err != nil {
					t.Fatal(err)
				}
				helperDownload := filepath.Join(root, "verified-helper-download")
				writeTestFile(t, helperDownload, []byte(newHelper), 0755)
				helperDownloadInfo, err := os.Stat(helperDownload)
				if err != nil {
					t.Fatal(err)
				}
				trace := filepath.Join(root, "operations")
				writeTestFile(t, trace, nil, 0600)
				script := `set -euo pipefail
TMP_FILES=("$TMP_BIN" "$TMP_HELPER_BIN")
SAFE_PROFILE_ACTION=none
ACTION_RUNNER_ENABLED=false
UPDATE_ONLY=false
EXIT_GENERAL=1
EXIT_MISSING_ARGS=2
EXIT_PREFLIGHT_FAILED=14
OUTPUT_FORMAT=json
NON_INTERACTIVE=true
PULSE_URL=https://pulse.example.invalid
AGENT_NAME=pulse-agent
BINARY_NAME=pulse-agent
NEW_VERSION=new-fixture-version
log_info() { :; }
mkdir() {
    if [[ "${@: -1}" == "$PRIVILEGE_HELPER_DIR" ]]; then
        printf 'helper-directory\n' >> "$TRACE"
        [[ "$FAULT" != helper-directory-creation ]] || return 1
    else
        printf 'directory\n' >> "$TRACE"
        [[ "$FAULT" != directory-creation ]] || return 1
    fi
    command mkdir "$@"
}
mktemp() {
    if [[ "$1" == "$PRIVILEGED_HELPER_BINARY_PATH".install.* ]]; then
        printf 'helper-allocate\n' >> "$TRACE"
        [[ "$FAULT" != helper-allocation ]] || return 1
    else
        printf 'allocate\n' >> "$TRACE"
        [[ "$FAULT" != allocation ]] || return 1
    fi
    command mktemp "$@"
}
chown() {
    # The proof guest is unprivileged. Check the actual root-owner request;
    # do not claim this double establishes native root ownership acceptance.
    [[ "$#" == 2 && "$1" == root:root ]] || exit 95
    if [[ "$2" == "$PRIVILEGE_HELPER_DIR" ]]; then
        printf 'helper-directory-owner\n' >> "$TRACE"
        [[ "$FAULT" != helper-directory-owner ]]
    else
        [[ "$2" == "$PRIVILEGED_HELPER_BINARY_PATH".install.* ]] || exit 95
        printf 'helper-owner\n' >> "$TRACE"
        [[ "$FAULT" != helper-owner ]]
    fi
}
chmod() {
    [[ "$#" == 2 && "$1" == 0755 ]] || exit 95
    if [[ "$2" == "$PRIVILEGE_HELPER_DIR" ]]; then
        printf 'helper-directory-mode\n' >> "$TRACE"
        [[ "$FAULT" != helper-directory-mode ]] || return 1
    elif [[ "$2" == "$PRIVILEGED_HELPER_BINARY_PATH".install.* ]]; then
        printf 'helper-mode\n' >> "$TRACE"
        [[ "$FAULT" != helper-mode ]] || return 1
    else
        printf 'mode\n' >> "$TRACE"
        [[ "$FAULT" != executable-mode ]] || return 1
    fi
    command chmod "$@" || return
    # Revoke actual execution after the successful mode operation. This checks
    # kernel refusal, not a claim to reproduce a native noexec mount or ACL.
    if [[ "$FAULT" == destination-not-executable && "$2" == "$INSTALL_DIR"/.pulse-agent.install.* ]]; then
        command chmod 0644 "$2"
    fi
}
stop_existing_agent_service() {
    # Transfer and mode must already be complete when disruption becomes possible.
    [[ -f "$STAGED_AGENT_BIN" ]] || exit 96
    if [[ "$FAULT" != destination-not-executable ]]; then
        [[ -x "$STAGED_AGENT_BIN" && "$("$STAGED_AGENT_BIN" --version)" == new-fixture-version ]] || exit 96
    fi
    if [[ "$PRIVILEGED_HELPER_ENABLED" == true ]]; then
        [[ -f "$STAGED_HELPER_BIN" && -x "$STAGED_HELPER_BIN" ]] || exit 96
        [[ "$("$STAGED_HELPER_BIN" --version)" == new-helper-version ]] || exit 96
    fi
    printf 'stop\n' >> "$TRACE"
}
systemctl() { return 1; }
pkill() { printf 'kill\n' >> "$TRACE"; }
sleep() { :; }
mv() {
    if [[ "$1" == "$TMP_BIN" && "$2" == "$INSTALL_DIR"/.pulse-agent.install.* ]]; then
        printf 'transfer\n' >> "$TRACE"
        if [[ "$FAULT" == partial-transfer ]]; then
            printf 'incomplete replacement' > "$2"
            return 1
        fi
        command mv "$@"
        return
    fi
    if [[ "$1" == "$TMP_HELPER_BIN" && "$2" == "$PRIVILEGED_HELPER_BINARY_PATH".install.* ]]; then
        printf 'helper-transfer\n' >> "$TRACE"
        if [[ "$FAULT" == helper-partial-transfer ]]; then
            printf 'incomplete helper replacement' > "$2"
            return 1
        fi
        command mv "$@"
        return
    fi
    if [[ "$2" == "$PRIVILEGED_HELPER_BINARY_PATH" ]]; then
        [[ "${1%/*}" == "$PRIVILEGE_HELPER_DIR" ]]
        printf 'helper-publish\n' >> "$TRACE"
        [[ "$FAULT" != helper-publication ]] || return 1
        command mv "$@"
        return
    fi
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
					"TRACE="+trace, "FAULT="+fault, "PRIVILEGE_HELPER_DIR="+helperDir,
					"PRIVILEGED_HELPER_BINARY_PATH="+installedHelper, "TMP_HELPER_BIN="+helperDownload,
					fmt.Sprintf("PRIVILEGED_HELPER_ENABLED=%t", helper))
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
				if (fault == "destination-command-failure" || fault == "destination-not-executable") &&
					(exit != 14 || !strings.Contains(string(output), `"exitCode":14`)) {
					t.Fatalf("destination execution refusal did not retain preflight exit 14: %d\n%s", exit, output)
				}
				operations, err := os.ReadFile(trace)
				if err != nil {
					t.Fatal(err)
				}
				sequence := string(operations)
				if fault != "none" && fault != "publication" && fault != "helper-publication" {
					for _, forbidden := range []string{"stop\n", "kill\n", "publish\n", "helper-publish\n"} {
						if strings.Contains(sequence, forbidden) {
							t.Fatalf("staging failure disrupted installation (%s):\n%s\n%s", forbidden, sequence, output)
						}
					}
					message := "has not been stopped or replaced; saved state is retained"
					if fault == "destination-command-failure" || fault == "destination-not-executable" {
						message = "The installed service and saved credentials have not been changed"
					}
					if !strings.Contains(string(output), message) {
						t.Fatalf("missing early refusal diagnostic:\n%s", output)
					}
				} else {
					want := "directory\nallocate\ntransfer\nmode\n"
					if helper {
						want += "helper-directory\nhelper-directory-owner\nhelper-directory-mode\nhelper-allocate\nhelper-transfer\nhelper-owner\nhelper-mode\n"
					}
					want += "destination-admission\n"
					if upgrade {
						want += "stop\nkill\n"
					}
					// The existing publication section also ensures the directory.
					want += "directory\npublish\n"
					if helper && fault != "publication" {
						want += "helper-publish\n"
					}
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
				} else if fault == "none" || fault == "helper-publication" || upgrade {
					body, err := os.ReadFile(installed)
					want := oldBinary
					if fault == "none" || fault == "helper-publication" {
						want = newBinary
					}
					if err != nil || string(body) != want {
						t.Fatalf("installed bytes changed incorrectly: %v %q", err, body)
					}
					info, err := os.Stat(installed)
					if err != nil || info.Mode().Perm() != 0755 {
						t.Fatalf("installed mode not retained: %v %v", info, err)
					}
					if fault == "none" && !os.SameFile(downloadInfo, info) {
						t.Fatal("same-filesystem staging unnecessarily copied the downloaded payload")
					}
				} else if _, err := os.Lstat(installed); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("failed fresh install created a collector: %v", err)
				}
				if helper {
					if fault == "helper-destination-directory" {
						body, err := os.ReadFile(filepath.Join(installedHelper, "unrelated"))
						if err != nil || string(body) != "retain helper directory contents" {
							t.Fatalf("helper destination directory changed: %v %q", err, body)
						}
					} else if fault == "none" || upgrade {
						body, err := os.ReadFile(installedHelper)
						want := oldHelper
						if fault == "none" {
							want = newHelper
						}
						info, statErr := os.Stat(installedHelper)
						if err != nil || string(body) != want || statErr != nil || info.Mode().Perm() != 0755 {
							t.Fatalf("helper bytes/mode changed incorrectly: %v %v %q", err, statErr, body)
						}
						if fault == "none" && !os.SameFile(helperDownloadInfo, info) {
							t.Fatal("same-filesystem helper staging unnecessarily copied the downloaded payload")
						}
					} else if _, err := os.Lstat(installedHelper); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("failed fresh install created a helper: %v", err)
					}
				} else if _, err := os.Lstat(helperDir); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("helper-disabled install touched helper filesystem: %v", err)
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
				helperStages, err := filepath.Glob(installedHelper + ".install.*")
				if err != nil || len(helperStages) != 0 {
					t.Errorf("owned helper staging not cleaned: %v %v", helperStages, err)
				}
			})
		}
	}
}
