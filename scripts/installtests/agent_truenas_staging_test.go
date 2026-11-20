//go:build linux

package installtests

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Run the installer from verification through collector publication with a
// real signed CLI and isolated files. Inject execute denial using file modes,
// not mount administration. No service, token, boot job or appliance is used.
func TestTrueNASScaleInstallerChecksVerifiedCollectorOffDownloadScratch(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("signed installer fixture requires ssh-keygen")
	}
	binary := buildPulseAgentLifecycleBinary(t)
	payload, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(payload))
	key := filepath.Join(t.TempDir(), "fixture-signer")
	for _, args := range [][]string{
		{"-q", "-t", "ed25519", "-N", "", "-f", key},
		{"-q", "-Y", "sign", "-f", key, "-n", "pulse-install-fixture", binary},
	} {
		if out, err := exec.Command("ssh-keygen", args...).CombinedOutput(); err != nil {
			t.Fatalf("sign synthetic collector: %v\n%s", err, out)
		}
	}
	publicKey, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	signature, err := os.ReadFile(binary + ".sig")
	if err != nil {
		t.Fatal(err)
	}
	const setup = `set -euo pipefail
TMP_FILES=("$TMP_BIN" "$TMP_HEADERS")
OS=linux
SAFE_PROFILE_ACTION=none
ACTION_RUNNER_ENABLED=false
PRIVILEGED_HELPER_ENABLED=false
UPDATE_ONLY=false
BINARY_NAME=pulse-agent
AGENT_NAME=pulse-agent
EXIT_GENERAL=1
EXIT_CHECKSUM_FAILED=12
EXIT_PREFLIGHT_FAILED=14
EXIT_MISSING_ARGS=16
EXIT_SIGNATURE_FAILED=17
OUTPUT_FORMAT=json
NON_INTERACTIVE=true
INSTALL_SIGNATURE_IDENTITY=fixture
INSTALL_SIGNATURE_NAMESPACE=pulse-install-fixture
log_info() { :; }
systemctl() { printf 'manager %s\n' "$1" >> "$TRACE"; return 1; }
stop_existing_agent_service() { printf 'stop\n' >> "$TRACE"; }
pkill() { printf 'kill\n' >> "$TRACE"; }
sleep() { :; }
chmod() {
    command chmod "$@" || return
    if [[ "$FAULT" != ordinary && "$2" == "$TMP_BIN" ]]; then
        command chmod 0644 "$2"
    fi
    if [[ "$FAULT" == exec-denied-at-destination && "$2" == "$INSTALL_DIR"/.pulse-agent.install.* ]]; then
        command chmod 0644 "$2"
    fi
}
mv() {
    command mv "$@" || return
    if [[ "$1" == "$TMP_BIN" ]]; then
        printf 'stage\n' >> "$TRACE"
        if [[ "$FAULT" == changed-transfer ]]; then printf 'tampered\n' >> "$2"; fi
    else
        printf 'publish\n' >> "$TRACE"
    fi
}
`
	var definitions strings.Builder
	for _, name := range []string{
		"cleanup", "installer_file_sha256", "final_response_header_value",
		"has_pinned_installer_signature_key", "decode_base64_to_file", "verify_download_signature",
		"pulse_url_uses_plain_http", "json_event", "fail", "stage_agent_binary_for_install",
		"validate_staged_agent_url", "prepare_truenas_scale_url_validator",
		"truenas_scale_staging_directory_is_private", "portable_path_mode",
	} {
		definitions.WriteString(extractInstallShellFunction(t, name) + "\n")
	}
	source, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(source), "# Release metadata verification")
	end := strings.Index(string(source), "# --- Service Installation ---")
	if start < 0 || end <= start {
		t.Fatal("missing download verification and publication section")
	}
	script := setup + definitions.String() + "\ntrap cleanup EXIT\n" + string(source[start:end])
	for _, tc := range []struct {
		name, fault, url, verdict string
		upgrade, stage            bool
		exit                      int
	}{
		{name: "ordinary-scratch", fault: "ordinary", stage: true},
		{name: "protected-scratch", stage: true},
		{name: "protected-scratch-upgrade", stage: true, upgrade: true},
		{name: "nonlocal-http-still-refused", url: "http://203.0.113.10:7655", upgrade: true, stage: true, exit: 14, verdict: "Plain HTTP requires a local or private address"},
		{name: "protected-scratch-and-storage", fault: "exec-denied-at-destination", upgrade: true, stage: true, exit: 14, verdict: "Could not run the downloaded agent"},
		{name: "untrusted-storage", fault: "writable-storage", upgrade: true, exit: 14, verdict: "Could not safely stage"},
		{name: "untrusted-storage-ancestor", fault: "writable-parent", upgrade: true, exit: 14, verdict: "Could not safely stage"},
		{name: "symlink-storage", fault: "symlink", upgrade: true, exit: 14, verdict: "Could not safely stage"},
		{name: "non-default-storage", fault: "custom", upgrade: true, exit: 14, verdict: "Could not run the downloaded agent"},
		{name: "wrong-platform", fault: "ordinary-linux", upgrade: true, exit: 14, verdict: "Could not run the downloaded agent"},
		{name: "invalid-signature", fault: "bad-signature", upgrade: true, exit: 17, verdict: "Cryptographic signature verification failed"},
		{name: "changed-staged-bytes", fault: "changed-transfer", upgrade: true, stage: true, exit: 12, verdict: "does not match the verified download"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dataParent := filepath.Join(root, "data")
			storage := filepath.Join(dataParent, "pulse-agent")
			if err := os.MkdirAll(storage, 0700); err != nil {
				t.Fatal(err)
			}
			switch tc.fault {
			case "writable-storage", "writable-parent":
				target := storage
				if tc.fault == "writable-parent" {
					target = dataParent
				}
				if err := os.Chmod(target, 0770); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				destination := filepath.Join(dataParent, "untrusted-target")
				if err := os.Rename(storage, destination); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(destination, storage); err != nil {
					t.Fatal(err)
				}
			}
			download := filepath.Join(root, "download")
			headers := filepath.Join(root, "headers")
			trace := filepath.Join(root, "trace")
			installed := filepath.Join(storage, "pulse-agent")
			saved := filepath.Join(storage, "runtime.token")
			oldImage := []byte("#!/bin/sh\nprintf 'pulse-agent v0.1.0\\n'\n")
			if tc.upgrade {
				writeTestFile(t, installed, oldImage, 0755)
				writeTestFile(t, saved, []byte("preserved state"), 0600)
			}
			sig := signature
			if tc.fault == "bad-signature" {
				sig = []byte("unverifiable fixture signature")
			}
			writeTestFile(t, download, payload, 0600)
			writeTestFile(t, headers, []byte("HTTP/1.1 200 OK\nX-Checksum-Sha256: "+checksum+"\nX-Signature-SSHSIG: "+base64.StdEncoding.EncodeToString(sig)+"\n"), 0600)
			writeTestFile(t, trace, nil, 0600)
			if tc.url == "" {
				tc.url = "https://pulse.example.invalid:7655"
			}
			defaultStorage := storage
			if tc.fault == "custom" {
				defaultStorage = filepath.Join(dataParent, "other-path")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", "-c", script)
			cmd.Env = []string{
				"PATH=" + os.Getenv("PATH"), "HOME=" + root, "TMPDIR=" + root, "LC_ALL=C",
				"INSTALL_DIR=" + storage, "TRUENAS_DEFAULT_STATE_DIR=" + defaultStorage,
				fmt.Sprintf("TRUENAS=%t", tc.fault != "ordinary-linux"),
				"TMP_BIN=" + download, "TMP_HEADERS=" + headers, "TRACE=" + trace,
				"PINNED_INSTALLER_SSH_PUBLIC_KEY=" + strings.TrimSpace(string(publicKey)),
				"FAULT=" + tc.fault, "PULSE_URL=" + tc.url, "SERVER_VERSION=",
				"PULSE_STATE_DIR=" + filepath.Join(root, "unexpected-agent-state"),
				"STAGED_AGENT_BIN=" + filepath.Join(root, "untrusted-environment"),
			}
			out, runErr := cmd.CombinedOutput()
			exit := 0
			if runErr != nil {
				var exitErr *exec.ExitError
				if !errors.As(runErr, &exitErr) {
					t.Fatalf("fixture execution: %v\n%s", runErr, out)
				}
				exit = exitErr.ExitCode()
			}
			if ctx.Err() != nil || exit != tc.exit || !strings.Contains(string(out), tc.verdict) {
				t.Fatalf("install exit=%d, want %d; timeout=%v\n%s", exit, tc.exit, ctx.Err(), out)
			}
			events, err := os.ReadFile(trace)
			if err != nil {
				t.Fatal(err)
			}
			operations := string(events)
			stageCount := strings.Count(operations, "stage\n")
			if stageCount > 1 || (stageCount == 1) != tc.stage {
				t.Fatalf("unexpected staging trace: %q", operations)
			}
			if tc.exit == 0 {
				wantOrder := "stage\n"
				if tc.upgrade {
					wantOrder += "stop\nkill\n"
				} else {
					wantOrder += "manager is-enabled\n"
				}
				wantOrder += "publish\n"
				got, err := os.ReadFile(installed)
				if err != nil || sha256.Sum256(got) != sha256.Sum256(payload) || operations != wantOrder ||
					strings.Count(string(out), "agent_url_validated") != 1 || !strings.Contains(string(out), "signature_ok") {
					t.Fatalf("verified collector did not reach publication in order: %v, %q\n%s", err, operations, out)
				}
			} else if strings.TrimPrefix(operations, "stage\n") != "" ||
				strings.Contains(string(out), "agent_url_validated") {
				t.Fatalf("failed admission disrupted the predecessor: %q\n%s", operations, out)
			} else if tc.upgrade {
				got, err := os.ReadFile(installed)
				if err != nil || string(got) != string(oldImage) {
					t.Fatalf("failed admission changed the installed collector: %v", err)
				}
			}
			if tc.upgrade {
				got, err := os.ReadFile(saved)
				if err != nil || string(got) != "preserved state" {
					t.Fatalf("preparation changed saved state: %v", err)
				}
			}
			leftover, err := filepath.Glob(filepath.Join(storage, ".pulse-agent.install.*"))
			if err != nil || len(leftover) != 0 {
				t.Fatalf("staged collector escaped cleanup: %d, %v", len(leftover), err)
			}
			for _, path := range []string{download, headers, filepath.Join(root, "unexpected-agent-state")} {
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("URL validation left scratch or created collector state: %v", err)
				}
			}
			t.Logf("exit=%d stage=%t signature=%t admitted=%t stopped=%t published=%t",
				exit, tc.stage, strings.Contains(string(out), "signature_ok"),
				strings.Contains(string(out), "agent_url_validated"), strings.Contains(operations, "stop\n"),
				strings.Contains(operations, "publish\n"))
		})
	}
}
