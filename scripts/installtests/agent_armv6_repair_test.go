//go:build unix

package installtests

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestAgentInstallerARMv6SavedStateRepairPreflight(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		if r.URL.Path == "/download/pulse-agent" {
			w.Header().Set("X-Checksum-Sha256", strings.Repeat("a", 64))
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	state := t.TempDir()
	tools := t.TempDir()
	token := filepath.Join(state, "runtime.token")
	files := map[string][]byte{
		"runtime.token":  []byte(strings.Repeat("a", 64) + "\n"),
		"agent-id":       []byte("installation-specific-id\n"),
		"connection.env": []byte(fmt.Sprintf("PULSE_URL='%s'\nPULSE_TOKEN_FILE='%s'\nPULSE_AGENT_ID='%s'\n", server.URL, token, "installation-specific-id")),
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(state, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	uname := []byte("#!/bin/sh\ncase \"$1\" in -s) echo Linux;; -m) echo armv6l;; *) exit 1;; esac\n")
	if err := os.WriteFile(filepath.Join(tools, "uname"), uname, 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", repoFile("scripts", "install.sh"), "--update", "--state-dir", state, "--preflight-only", "--output", "json", "--non-interactive")
	cmd.Env = append(os.Environ(), "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	server.Close()
	if err != nil {
		t.Fatalf("saved-state ARMv6 repair preflight: %v\n%s", err, out)
	}
	if !strings.Contains(strings.Join(requests, "\n"), "HEAD /download/pulse-agent?arch=linux-armv6") {
		t.Fatalf("wrong target: %v\n%s", requests, out)
	}
	if strings.Contains(string(out), strings.Repeat("a", 64)) {
		t.Fatal("credential leaked into output")
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(state, name))
		info, statErr := os.Stat(filepath.Join(state, name))
		if err != nil || statErr != nil || string(got) != string(want) || info.Mode().Perm() != 0600 {
			t.Fatalf("saved %s changed", name)
		}
	}
	if entries, _ := os.ReadDir(state); len(entries) != len(files) {
		t.Fatal("preflight created state")
	}
}

// Exercise the production download, signature, staged URL admission, upgrade
// detection and replacement blocks in private directories. Service-manager and
// process-stop effects are recorded locally; host service registration remains
// a release/device qualification boundary. ARM target selection is covered by
// the full installer preflight above and ARM execution by the updater proof.
func TestAgentInstallerSavedStateRepairVerifiedReplacement(t *testing.T) {
	assets := os.Getenv("PULSE_AGENT_RELEASE_TEST_DIR")
	if assets == "" {
		t.Skip("requires canonical native release executable packet")
	}
	data, err := os.ReadFile(filepath.Join(assets, "linux-amd64"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	state := filepath.Join(dir, "state")
	install := filepath.Join(dir, "bin")
	for _, path := range []string{state, install} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	token := filepath.Join(state, "token")
	if err := os.WriteFile(token, []byte("abc123\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "agent-id"), []byte("preserved-agent-id\n"), 0600); err != nil {
		t.Fatal(err)
	}
	old := []byte("#!/bin/sh\nprintf 'v6.1.0\\n'\n")
	if err := os.WriteFile(filepath.Join(install, "pulse-agent"), old, 0700); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(dir, "download")
	if err := os.WriteFile(artifact, data, 0600); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(dir, "signer")
	for _, args := range [][]string{{"-q", "-t", "ed25519", "-N", "", "-f", key}, {"-Y", "sign", "-f", key, "-n", "pulse-install", artifact}} {
		if out, err := exec.Command("ssh-keygen", args...).CombinedOutput(); err != nil {
			t.Fatalf("fixture signature: %v %s", err, out)
		}
	}
	public, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	signature, err := os.ReadFile(artifact + ".sig")
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	var mu sync.Mutex
	var requested string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/version" {
			fmt.Fprint(w, `{"version":"6.2.1"}`)
			return
		}
		if r.URL.Path != "/download/pulse-agent" {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		requested = r.URL.Query().Get("arch")
		mu.Unlock()
		w.Header().Set("X-Checksum-Sha256", hex.EncodeToString(hash[:]))
		w.Header().Set("X-Signature-SSHSIG", base64.StdEncoding.EncodeToString(signature))
		w.Write(data)
	}))
	defer server.Close()
	connection := []byte(fmt.Sprintf("PULSE_STATE_DIR='%s'\nPULSE_URL='%s'\nPULSE_TOKEN_FILE='%s'\nPULSE_AGENT_ID='preserved-agent-id'\nPULSE_INSECURE_SKIP_VERIFY='true'\n", state, server.URL, token))
	if err := os.WriteFile(filepath.Join(state, "connection.env"), connection, 0600); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	archStart := strings.Index(text, "# --- Download ---")
	archEnd := strings.Index(text[archStart:], "# Fail before downloading") + archStart
	downloadStart := strings.Index(text[archStart:], "# Create temp file") + archStart
	upgradeStart := strings.Index(text, "# --- Upgrade Detection ---")
	installStart := strings.Index(text[upgradeStart:], "# Install Binary") + upgradeStart
	installEnd := strings.Index(text[installStart:], "\nif [[ \"$PRIVILEGED_HELPER_ENABLED\"") + installStart
	if archStart < 0 || archEnd < archStart || downloadStart < archEnd || upgradeStart < downloadStart || installEnd < installStart {
		t.Fatal("installer boundary markers changed")
	}
	script := `set -euo pipefail
 STATE_DIR="$1"; STATE_DIR_SOURCE=explicit; INSTALL_DIR="$2"; BINARY_NAME=pulse-agent; AGENT_NAME=pulse-agent
 PULSE_URL=""; PULSE_TOKEN=""; AGENT_ID=""; HOSTNAME_OVERRIDE=""; REPORT_IP=""; INSECURE=false; SERVER_FINGERPRINT=""; CURL_CA_BUNDLE=""
 TMP_FILES=(); SAFE_PROFILE_ACTION=""; PRIVILEGED_HELPER_ENABLED=false; ACTION_RUNNER_ENABLED=false; UPDATE_ONLY=true
 EXIT_DOWNLOAD_FAILED=10; EXIT_CHECKSUM_FAILED=11; EXIT_SIGNATURE_FAILED=12; EXIT_PREFLIGHT_FAILED=13; EXIT_MISSING_ARGS=14
 INSTALL_SIGNATURE_IDENTITY=pulse-release; INSTALL_SIGNATURE_NAMESPACE=pulse-install; PINNED_INSTALLER_SSH_PUBLIC_KEY="$3"
 trap 'for path in "${TMP_FILES[@]}";do rm -f "$path";done' EXIT
 log_info(){ :; }; log_warn(){ :; }; json_event(){ :; }; fail(){ printf '%s\n' "$1" >&2;exit 99; }
 stop_existing_agent_service(){ printf 'stop\n' >> "$STATE_DIR/effects"; }
 pkill(){ printf 'stop-process\n' >> "$STATE_DIR/effects"; }
 sleep(){ :; }
 ` + extractLifecycleTrustShellFunctions(t) + "\n" + extractTokenRecoveryShellFunctions(t) + "\n"
	for _, name := range []string{"read_connection_state_value", "recover_connection_state", "final_response_header_value", "installer_file_sha256", "has_pinned_installer_signature_key", "decode_base64_to_file", "verify_download_signature", "validate_staged_agent_url"} {
		script += extractInstallShellFunction(t, name) + "\n"
	}
	script += `recover_connection_state "$STATE_DIR/connection.env"
 [[ "$AGENT_ID" == preserved-agent-id && "$PULSE_TOKEN" == abc123 ]]
 ` + text[archStart:archEnd] + text[downloadStart:installEnd] + `
 [[ "$UPGRADE_MODE" == true && "$NEW_VERSION" == v6.2.1 ]]
 `
	cmd := exec.Command("bash", "-c", script, "repair-proof", state, install, strings.TrimSpace(string(public)))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("canonical installer replacement: %v\n%s", err, out)
	}
	installed, err := os.ReadFile(filepath.Join(install, "pulse-agent"))
	if err != nil || !bytes.Equal(installed, data) {
		t.Fatal("verified executable was not installed")
	}
	for name, want := range map[string][]byte{"token": []byte("abc123\n"), "agent-id": []byte("preserved-agent-id\n"), "connection.env": connection} {
		got, err := os.ReadFile(filepath.Join(state, name))
		info, statErr := os.Stat(filepath.Join(state, name))
		if err != nil || statErr != nil || !bytes.Equal(got, want) || info.Mode().Perm() != 0600 {
			t.Fatalf("saved %s changed", name)
		}
	}
	mu.Lock()
	target := requested
	mu.Unlock()
	if target != "linux-amd64" {
		t.Fatalf("native target %q", target)
	}
	out, err := exec.Command(filepath.Join(install, "pulse-agent"), "--release-identity").CombinedOutput()
	if err != nil || string(out) != "[[pulse-agent-release:v1:v6.2.1]]\n" {
		t.Fatalf("activated runtime identity: %v %q", err, out)
	}
}
