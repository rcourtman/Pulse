package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// This is an offline HTTP/preflight contract, NOT native pfSense acceptance.
// Only uname is simulated. Bash, curl, the served installer and download
// handler are real; the agent bytes and signature sidecars are fixtures and
// are never executed or claimed to be cryptographically qualified artifacts.
func TestFreeBSDOfflineAgentPreflight(t *testing.T) {
	for _, tc := range []struct {
		name             string
		binary, sidecars bool
		wantExit         int
	}{
		{"bundled", true, true, 0},
		{"missing_binary", false, false, 11},
		{"missing_sidecars", true, false, 11},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			binDir := setupTempPulseBin(t)
			payload := validTestUnifiedAgentBinary("offline-freebsd-fixture")
			binaryPath := filepath.Join(binDir, "pulse-agent-freebsd-amd64")
			write := func(path string, data []byte) {
				t.Helper()
				if err := os.WriteFile(path, data, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if tc.binary {
				write(binaryPath, payload)
			}
			if tc.sidecars {
				write(binaryPath+".sig", []byte("synthetic-signature"))
				write(binaryPath+".sshsig", []byte("synthetic-ssh-signature"))
			}
			var externalAttempts, healthRequests, downloadRequests atomic.Int32
			router := &Router{
				projectRoot: dir, serverVersion: "v6.4.5-rc.1",
				checksumCache: make(map[string]checksumCacheEntry),
				installScriptClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					externalAttempts.Add(1)
					if req.URL.Host != "github.com" || !strings.HasSuffix(req.URL.Path, "/pulse-agent-freebsd-amd64") {
						t.Errorf("unexpected fallback destination: %s", req.URL)
					}
					return nil, errors.New("fixture: server has no external connectivity")
				})},
			}
			script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "install.sh"))
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			mux.HandleFunc("/install.sh", func(w http.ResponseWriter, r *http.Request) {
				handleDownloadInstallScriptCommon(w, r, "dev", filepath.Join(dir, "absent.sh"), script, "install.sh", "text/x-shellscript")
			})
			mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) { healthRequests.Add(1); w.WriteHeader(http.StatusOK) })
			mux.HandleFunc("/download/pulse-agent", func(w http.ResponseWriter, r *http.Request) {
				downloadRequests.Add(1)
				if r.URL.Query().Get("arch") != "freebsd-amd64" {
					t.Errorf("wrong arch: %s", r.URL)
				}
				router.handleDownloadUnifiedAgent(w, r)
			})
			server := httptest.NewServer(mux)
			defer server.Close()

			downloadedScript := filepath.Join(dir, "install.sh")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			fetch := exec.CommandContext(ctx, "curl", "--noproxy", "*", "-fsS", server.URL+"/install.sh", "-o", downloadedScript)
			if out, err := fetch.CombinedOutput(); err != nil {
				t.Fatalf("fetch installer: %v\n%s", err, out)
			}
			original, err := os.ReadFile(script)
			if err != nil {
				t.Fatal(err)
			}
			downloaded, err := os.ReadFile(downloadedScript)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(original, downloaded) {
				t.Fatal("served installer differs from source")
			}
			shimDir := filepath.Join(dir, "shim")
			if err := os.Mkdir(shimDir, 0700); err != nil {
				t.Fatal(err)
			}
			write(filepath.Join(shimDir, "uname"), []byte("#!/bin/sh\ncase \"$1\" in -s) echo FreeBSD;; -m) echo amd64;; *) exec /usr/bin/uname \"$@\";; esac\n"))
			cmd := exec.CommandContext(ctx, "bash", downloadedScript, "--url", server.URL, "--preflight-only", "--output", "json", "--non-interactive")
			cmd.Env = append(os.Environ(), "PATH="+shimDir+":"+os.Getenv("PATH"), "NO_PROXY=*", "no_proxy=*")
			out, runErr := cmd.CombinedOutput()
			exitCode := 0
			if runErr != nil {
				var ee *exec.ExitError
				if !errors.As(runErr, &ee) {
					t.Fatalf("preflight: %v\n%s", runErr, out)
				}
				exitCode = ee.ExitCode()
			}
			t.Logf("preflight exit=%d; external attempts=%d\n%s", exitCode, externalAttempts.Load(), out)
			if exitCode != tc.wantExit {
				t.Fatalf("exit=%d, want %d", exitCode, tc.wantExit)
			}
			if healthRequests.Load() != 1 || downloadRequests.Load() != 1 {
				t.Fatalf("health/download requests = %d/%d", healthRequests.Load(), downloadRequests.Load())
			}
			if tc.wantExit != 0 {
				if externalAttempts.Load() != 1 || !strings.Contains(string(out), `"code":"agent_download_unavailable"`) {
					t.Fatal("missing artifact must fail visibly at preflight")
				}
				return
			}
			if externalAttempts.Load() != 0 || !strings.Contains(string(out), `"code":"agent_download_available"`) {
				t.Fatal("bundled artifact must preflight without external fetch")
			}
			response, err := server.Client().Get(server.URL + "/download/pulse-agent?arch=freebsd-amd64")
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			data, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != http.StatusOK || !bytes.Equal(data, payload) || response.Header.Get(checksumHeaderName) != fmt.Sprintf("%x", sha256.Sum256(payload)) {
				t.Fatal("local payload/checksum mismatch")
			}
			if externalAttempts.Load() != 0 {
				t.Fatal("local download attempted external access")
			}
		})
	}
}
