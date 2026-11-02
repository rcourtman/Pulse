package installtests

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Execute saved-state recovery and the initial removal caller with the real
// collector CLI, real TLS and authenticated POST/confirmation. Only service and
// process operations are doubles; erasure is restricted to private directories.
// This is not the root entry point, native systemd or an appliance installation.
func TestAgentInstalledRemovalSavedTransport(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("confined Linux removal caller")
	}
	buildDir := t.TempDir()
	binary := filepath.Join(buildDir, "pulse-agent")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/pulse-agent")
	build.Dir = repoFile()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build real installed CLI: %v\n%s", err, out)
	}
	if err := os.Chmod(binary, 0755); err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("actual installed CLI sha256=%x bytes=%d", sha256.Sum256(bytes), len(bytes))
	sourceBytes, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	_, entry, ok := strings.Cut(string(sourceBytes), "# --- Uninstall Logic ---\n")
	if !ok {
		t.Fatal("missing removal entry")
	}
	entry, _, ok = strings.Cut(entry, "    # Remove least-privilege helper artifacts.")
	if !ok {
		t.Fatal("missing confined initial teardown boundary")
	}
	entry += "fi\n"
	functions := ""
	for _, name := range []string{"portable_path_uid", "portable_path_mode", "trusted_lifecycle_regular_file",
		"trusted_connection_state_file", "trusted_private_lifecycle_regular_file", "read_connection_state_value",
		"recover_connection_state", "recover_token_from_default_agent_token_file", "read_collector_token_file_safely",
		"collector_lifecycle_binary", "prepare_collector_lifecycle_token_file", "run_collector_lifecycle_command",
		"collector_credential_state_present", "uninstall_collector_registration", "remove_authorized_runtime_dir",
		"remove_agent_state_dir", "remove_privileged_helper_state_dir", "remove_safe_profile_state_dir", "fail"} {
		functions += extractInstallShellFunction(t, name) + "\n"
	}
	for _, mode := range []string{"saved-ca", "saved-pin", "missing-ca", "wrong-pin", "server-refused", "other-identity", "nonloopback-http"} {
		t.Run(mode, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				var body struct {
					AgentID string `json:"agentId"`
				}
				err := json.NewDecoder(r.Body).Decode(&body)
				if err != nil || r.Method != http.MethodPost || r.URL.Path != "/api/agents/agent/uninstall" ||
					r.Header.Get("Authorization") != "Bearer synthetic-runtime-credential" || body.AgentID != "saved-agent" {
					t.Error("removal request lost exact identity, private bearer or method/path")
					http.Error(w, "invalid fixture request", http.StatusBadRequest)
					return
				}
				if mode == "server-refused" {
					http.Error(w, "refused", http.StatusForbidden)
				} else if mode == "other-identity" {
					fmt.Fprint(w, `{"success":true,"agentId":"other-agent"}`)
				} else {
					fmt.Fprint(w, `{"success":true,"agentId":"saved-agent"}`)
				}
			}))
			server.Config.ErrorLog = log.New(io.Discard, "", 0)
			defer server.Close()
			root := t.TempDir()
			state, helper, profile := filepath.Join(root, "state"), filepath.Join(root, "helper"), filepath.Join(root, "profile")
			for _, path := range []string{state, helper, profile} {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "sentinel"), []byte("preserve until confirmed\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			url, ca, pin := server.URL, filepath.Join(state, "saved-ca.pem"), ""
			if mode == "saved-pin" {
				pin = fmt.Sprintf("%x", sha256.Sum256(server.Certificate().Raw))
				ca = ""
			} else if mode == "missing-ca" {
				ca = ""
			} else if mode == "wrong-pin" {
				pin = strings.Repeat("0", 64)
			} else if mode == "nonloopback-http" {
				url, ca = "http://192.0.2.1:7655", ""
			}
			saved := map[string][]byte{
				"agent-id": []byte("saved-agent\n"), "runtime.token": []byte("synthetic-runtime-credential\n"),
				"saved-ca.pem":   pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}),
				"connection.env": []byte(fmt.Sprintf("PULSE_URL='%s'\nPULSE_AGENT_ID='saved-agent'\nPULSE_HOSTNAME='saved-host'\nPULSE_CACERT='%s'\nPULSE_SERVER_FINGERPRINT='%s'\nPULSE_INSECURE_SKIP_VERIFY='true'\n", url, ca, pin)),
			}
			for name, content := range saved {
				if err := os.WriteFile(filepath.Join(state, name), content, 0600); err != nil {
					t.Fatal(err)
				}
			}
			script := `set -euo pipefail
UNINSTALL=true AGENT_NAME=pulse-agent EXIT_GENERAL=1 EXIT_MISSING_ARGS=16
ACTION_RUNNER_NAME=pulse-agent-runner PRIVILEGED_HELPER_NAME=pulse-agent-helper
OUTPUT_FORMAT=text NON_INTERACTIVE=true
PULSE_TOKEN='' PULSE_URL='' CURL_CA_BUNDLE='' SERVER_FINGERPRINT=''
AGENT_ID='' HOSTNAME_OVERRIDE='' REPORT_IP='' RUNTIME_TOKEN_FILE=''
INSECURE=false STATE_DIR_SOURCE=explicit RETARGET_ONLY=false
LEAST_PRIVILEGE_USER=$(id -un)
log_info() { :; }
log_warn() { printf '%s\n' "$*"; }
log_error() { printf '%s\n' "$*"; }
nas_agent_removal_platform() { return 1; }
truenas_scale_agent_removal_present() { return 1; }
systemctl() { return 91; }
command() {
    if [[ "$*" == '-v rc-service' ]]; then return 1; fi
    builtin command "$@"
}
quiesce_agent_units_for_removal() {
    [[ "$*" == 'pulse-agent-runner.service pulse-agent-helper.socket pulse-agent-helper.service pulse-agent.service' ]] || return 90
    printf 'quiesce\n' >> "$EVENTS"
}
pkill() { printf 'process-stop\n' >> "$EVENTS"; }
sleep() { :; }
teardown_action_runner_service() { printf 'runner-teardown\n' >> "$EVENTS"; }
teardown_privileged_helper_service() { printf 'helper-teardown\n' >> "$EVENTS"; }
teardown_systemd_agent_service() { printf 'collector-teardown\n' >> "$EVENTS"; }
rm() {
    [[ $# == 3 && "$1" == -rf && "$2" == -- ]] || return 92
    [[ "$3" == "$STATE_DIR" || "$3" == "$PRIVILEGED_HELPER_STATE_DIR" || "$3" == "$SAFE_PROFILE_STATE_DIR" ]] || return 93
    printf 'state-remove\n' >> "$EVENTS"
    builtin command rm "$@"
}
`
			for key, value := range map[string]string{
				"STATE_DIR": state, "STATE_DIR_REMOVAL_AUTHORITY": state,
				"PRIVILEGED_HELPER_STATE_DIR": helper, "PRIVILEGED_HELPER_STATE_DIR_REMOVAL_AUTHORITY": helper,
				"PRIVILEGED_HELPER_CREDENTIAL_DIR": helper, "SAFE_PROFILE_STATE_DIR": profile,
				"SAFE_PROFILE_STATE_DIR_REMOVAL_AUTHORITY": profile, "COLLECTOR_LIFECYCLE_BINARY_PATH": binary,
				"EVENTS": filepath.Join(root, "events"),
			} {
				script += key + "=" + strconv.Quote(value) + "\n"
			}
			script += functions + "\nrecover_connection_state \"$STATE_DIR/connection.env\"\nmain() {\n" + entry + "}\nmain\n"
			runCtx, stop := context.WithTimeout(context.Background(), 20*time.Second)
			defer stop()
			out, err := exec.CommandContext(runCtx, "bash", "-c", script).CombinedOutput()
			wantSuccess := mode == "saved-ca" || mode == "saved-pin"
			if (err == nil) != wantSuccess || runCtx.Err() != nil || strings.Contains(string(out), "synthetic-runtime-credential") {
				t.Fatalf("removal success=%t timeout=%v error=%v\n%s", wantSuccess, runCtx.Err(), err, out)
			}
			events, readErr := os.ReadFile(filepath.Join(root, "events"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			wantRequests := int32(0)
			if wantSuccess || mode == "server-refused" || mode == "other-identity" {
				wantRequests = 1
			}
			if requests.Load() != wantRequests {
				t.Fatalf("removal requests=%d want=%d", requests.Load(), wantRequests)
			}
			if wantSuccess {
				if string(events) != "quiesce\nprocess-stop\nprocess-stop\nrunner-teardown\nhelper-teardown\ncollector-teardown\nstate-remove\nstate-remove\nstate-remove\n" {
					t.Fatalf("wrong confirmed teardown sequence: %s", events)
				}
				for _, path := range []string{state, helper, profile} {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatalf("confirmed removal retained selected fixture %s", path)
					}
				}
			} else {
				// Quiescence may already have stopped and disabled services. It
				// is not rolled back or described as an unchanged installation.
				if string(events) != "quiesce\n" || !strings.Contains(string(out), "local credentials and services were retained") {
					t.Fatalf("unconfirmed removal proceeded to erasure: %s\n%s", events, out)
				}
				for name, want := range saved {
					path := filepath.Join(state, name)
					got, err := os.ReadFile(path)
					info, statErr := os.Stat(path)
					if err != nil || statErr != nil || string(got) != string(want) || info.Mode().Perm() != 0600 {
						t.Fatalf("refused removal mutated saved %s", name)
					}
				}
				for _, path := range []string{state, helper, profile} {
					if got, err := os.ReadFile(filepath.Join(path, "sentinel")); err != nil || string(got) != "preserve until confirmed\n" {
						t.Fatalf("refused removal lost recovery sentinel: %s %v", path, err)
					}
				}
			}
			t.Logf("mode=%s authenticated_requests=%d confirmed=%t recovery_retained=%t", mode, requests.Load(), wantSuccess, !wantSuccess)
		})
	}
}
