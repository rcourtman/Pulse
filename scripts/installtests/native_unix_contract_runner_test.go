//go:build !windows

package installtests

import (
	"context"
	"crypto/sha256"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// The Python controls exercise both workflow phases and real compiled Go tests
// without recursively running the installer suite. Native CI supplies the
// platform timing proof; these controls reject lost checks and false passes.
func TestNativeUnixContractRunnerPartitionsWithoutDroppingChecks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", "-I", "-S", repoFile("scripts", "tests", "test_native_installer_contract_runner.py"))
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("native installer runner control deadline: %v", ctx.Err())
	}
	if err != nil {
		t.Fatalf("native installer runner control: %v\n%s", err, output)
	}
}

// Exercise the whole installer with real curl, TLS, signatures and publication.
// The curl shim changes only the fixed release URL to an isolated local peer;
// arguments, transport bounds and certificate verification stay real. The one
// source substitution is the same disposable signing key as the existing tests.
// This is a synthetic Unix install, not a released binary or native client proof.
func TestInstallMCPTransport(t *testing.T) {
	t.Parallel()
	realCurl, err := exec.LookPath("curl")
	if err != nil {
		t.Fatal("curl is required for the MCP installer transport controls")
	}
	for _, command := range []string{"bash", "ssh-keygen"} {
		if _, err := exec.LookPath(command); err != nil {
			t.Fatal(err)
		}
	}
	binaryName := "pulse-mcp-" + runtime.GOOS + "-" + runtime.GOARCH
	cases := []struct {
		name, mode, phase string
		trusted, success  bool
	}{
		{"complete", "complete", "", true, true},
		{"https_redirects", "redirect", "", true, true},
		{"ambient_proxy_ignored", "proxy", "", true, true},
		{"untrusted_certificate", "complete", "", false, false},
		{"ambient_insecure_ignored", "insecure", "", false, false},
		{"binary_downgrade", "downgrade", binaryName, true, false},
		{"checksum_downgrade", "downgrade", "checksums.txt", true, false},
		{"signature_downgrade", "downgrade", "checksums.txt.sshsig", true, false},
		{"stalled_binary", "stalled", binaryName, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tools, downloads, destination := filepath.Join(root, "tools"), filepath.Join(root, "downloads"), filepath.Join(root, "install with spaces")
			for _, path := range []string{tools, downloads, destination} {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			key := filepath.Join(root, "signer")
			runTestCommand(t, exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key))
			publicKey, err := os.ReadFile(key + ".pub")
			if err != nil {
				t.Fatal(err)
			}
			script, err := os.ReadFile(repoFile("scripts", "install-mcp.sh"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(script), productionMCPReleaseKey) != 1 {
				t.Fatal("unexpected signing authority")
			}
			scriptPath := filepath.Join(root, "install-mcp.sh")
			writeTestFile(t, scriptPath, []byte(strings.Replace(string(script), productionMCPReleaseKey, strings.TrimSpace(string(publicKey)), 1)), 0700)
			replacement := []byte("signed MCP transport fixture\n")
			manifest := []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(replacement), binaryName))
			manifestPath := filepath.Join(root, "checksums.txt")
			writeTestFile(t, manifestPath, manifest, 0600)
			runTestCommand(t, exec.Command("ssh-keygen", "-q", "-Y", "sign", "-f", key, "-n", "pulse-install", manifestPath))
			signature, err := os.ReadFile(manifestPath + ".sig")
			if err != nil {
				t.Fatal(err)
			}
			assets := map[string][]byte{binaryName: replacement, "checksums.txt": manifest, "checksums.txt.sshsig": signature}
			var httpRequests, tlsRequests atomic.Int32
			insecure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				httpRequests.Add(1)
				_, _ = w.Write(assets[filepath.Base(r.URL.Path)])
			}))
			defer insecure.Close()
			peer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tlsRequests.Add(1)
				name := filepath.Base(r.URL.Path)
				data, found := assets[name]
				if !found {
					http.NotFound(w, r)
					return
				}
				if !strings.HasPrefix(r.URL.Path, "/bytes/") {
					if tc.mode == "redirect" {
						http.Redirect(w, r, "/bytes/"+name, http.StatusFound)
						return
					}
					if name == tc.phase {
						switch tc.mode {
						case "downgrade":
							http.Redirect(w, r, insecure.URL+"/"+name, http.StatusFound)
							return
						case "stalled":
							w.Header().Set("Content-Length", "100")
							_, _ = io.WriteString(w, "partial bytes")
							w.(http.Flusher).Flush()
							<-r.Context().Done()
							return
						}
					}
				}
				_, _ = w.Write(data)
			}))
			defer peer.Close()
			ca := filepath.Join(root, "ca.pem")
			writeTestFile(t, ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: peer.Certificate().Raw}), 0600)
			// Deliberately conflicting curlrc files must not override the
			// installer's verified HTTPS policy or break a healthy download.
			curlrc := ""
			if tc.mode == "proxy" {
				curlrc = "proxy = http://127.0.0.1:1\nnoproxy = \"\"\n"
			}
			if tc.mode == "insecure" {
				curlrc = "insecure\n"
			}
			writeTestFile(t, filepath.Join(root, ".curlrc"), []byte(curlrc), 0600)
			writeTestFile(t, filepath.Join(tools, "curl"), []byte(`#!/bin/bash
set -euo pipefail
args=()
for arg in "$@"; do
    case "$arg" in
        https://github.com/rcourtman/Pulse/releases/download/v-transport/*)
            arg="${MCP_TEST_PEER}/${arg##*/}" ;;
    esac
    args+=("$arg")
done
exec "$MCP_TEST_CURL" "${args[@]}"
`), 0700)
			installed := filepath.Join(destination, "pulse-mcp")
			old := []byte("previous working adapter\n")
			writeTestFile(t, installed, old, 0700)
			ctx, cancel := context.WithTimeout(context.Background(), 210*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", scriptPath)
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
			cmd.WaitDelay = 5 * time.Second
			cmd.Env = append(os.Environ(), "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"),
				"HOME="+root, "CURL_HOME="+root, "XDG_CONFIG_HOME="+root,
				"TMPDIR="+downloads, "PULSE_MCP_BIN_DIR="+destination,
				"PULSE_MCP_VERSION=v-transport", "PULSE_MCP_REPO=rcourtman/Pulse",
				"MCP_TEST_CURL="+realCurl, "MCP_TEST_PEER="+peer.URL,
				"HTTPS_PROXY=", "https_proxy=", "HTTP_PROXY=", "http_proxy=", "ALL_PROXY=", "all_proxy=",
				"NO_PROXY=", "no_proxy=", "SSL_CERT_FILE=", "CURL_CA_BUNDLE=")
			if tc.trusted {
				cmd.Env = append(cmd.Env, "CURL_CA_BUNDLE="+ca)
			}
			started := time.Now()
			output, runErr := cmd.CombinedOutput()
			elapsed := time.Since(started)
			if ctx.Err() != nil {
				t.Fatalf("installer exceeded its transport bound: %v\n%s", ctx.Err(), output)
			}
			if (runErr == nil) != tc.success {
				t.Fatalf("installer success=%v want=%v: %v\n%s", runErr == nil, tc.success, runErr, output)
			}
			got, err := os.ReadFile(installed)
			if err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(installed)
			if err != nil {
				t.Fatal(err)
			}
			if tc.success {
				if string(got) != string(replacement) || info.Mode().Perm() != 0755 || !strings.Contains(string(output), "installed:") {
					t.Fatalf("verified publication missing: mode=%v bytes=%q\n%s", info.Mode(), got, output)
				}
			} else {
				if string(got) != string(old) || info.Mode().Perm() != 0700 || strings.Contains(string(output), "installed:") {
					t.Fatalf("failed transport replaced or reported installation: mode=%v bytes=%q\n%s", info.Mode(), got, output)
				}
				diagnostic := "download failed:"
				if tc.phase == "checksums.txt" {
					diagnostic = "could not fetch checksums.txt; refusing unverified install"
				}
				if tc.phase == "checksums.txt.sshsig" {
					diagnostic = "could not fetch checksums.txt.sshsig; refusing unverified install"
				}
				if !strings.Contains(string(output), diagnostic) {
					t.Fatalf("missing phase diagnostic %q\n%s", diagnostic, output)
				}
			}
			if count := httpRequests.Load(); count != 0 {
				t.Fatalf("HTTPS downgrade contacted HTTP peer %d times", count)
			}
			leftovers, err := os.ReadDir(downloads)
			if err != nil || len(leftovers) != 0 {
				t.Fatalf("download cleanup incomplete: %v %v", leftovers, err)
			}
			stages, err := filepath.Glob(filepath.Join(destination, ".pulse-mcp.*"))
			if err != nil || len(stages) != 0 {
				t.Fatalf("staged cleanup incomplete: %v %v", stages, err)
			}
			if tc.mode == "stalled" {
				if elapsed < 55*time.Second || elapsed > 185*time.Second || tlsRequests.Load() < 1 || !strings.Contains(string(output), "curl: (28)") {
					t.Fatalf("deadline/retry result elapsed=%v requests=%d\n%s", elapsed, tlsRequests.Load(), output)
				}
			}
			t.Logf("success=%v tls_requests=%d http_requests=%d elapsed=%s previous_preserved=%v owned_cleanup=complete", tc.success, tlsRequests.Load(), httpRequests.Load(), elapsed.Round(time.Millisecond), !tc.success)
		})
	}
}
