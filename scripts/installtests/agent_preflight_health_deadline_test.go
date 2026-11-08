//go:build linux

package installtests

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// Run the complete non-mutating installer entry with real curl and TLS. Only
// service inventory is doubled; no binary is downloaded, installed or run.
// Stalls use the production ten-second budget, not an accelerated curl double.
func TestAgentInstallerPreflightHealthDeadline(t *testing.T) {
	for _, mode := range []string{"healthy", "redirect", "http-error", "untrusted-ca", "stalled-headers", "stalled-body", "redirect-budget"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			bootstrapToken := strings.Repeat("a1", 32)
			var mu sync.Mutex
			var requests []string
			var disclosed bool
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requests = append(requests, r.Method+" "+r.URL.Path)
				disclosed = disclosed || r.Header.Get("Authorization") != "" || strings.Contains(r.URL.String(), bootstrapToken)
				mu.Unlock()
				if r.URL.Path == "/download/pulse-agent" {
					if r.Method != http.MethodHead || r.URL.Query().Get("arch") != "linux-amd64" {
						t.Errorf("unexpected metadata request: %s %s", r.Method, r.URL)
					}
					w.Header().Set("X-Checksum-Sha256", strings.Repeat("a", 64))
					return
				}
				if r.Method != http.MethodGet || (r.URL.Path != "/api/health" && r.URL.Path != "/ready") {
					t.Errorf("unexpected preflight request: %s %s", r.Method, r.URL)
					http.NotFound(w, r)
					return
				}
				switch mode {
				case "stalled-headers":
					<-r.Context().Done()
					return
				case "stalled-body":
					w.Header().Set("Content-Length", "100")
					fmt.Fprint(w, "{")
					w.(http.Flusher).Flush()
					<-r.Context().Done()
					return
				case "redirect-budget":
					// Both connected exchanges are individually shorter than ten
					// seconds. The original request's budget must span the redirect.
					timer := time.NewTimer(6 * time.Second)
					defer timer.Stop()
					select {
					case <-timer.C:
					case <-r.Context().Done():
						return
					}
				case "http-error":
					http.Error(w, "fixture unavailable", http.StatusServiceUnavailable)
					return
				}
				if (mode == "redirect" || mode == "redirect-budget") && r.URL.Path == "/api/health" {
					http.Redirect(w, r, "/ready", http.StatusTemporaryRedirect)
					return
				}
				fmt.Fprint(w, `{"status":"ok"}`)
			}))
			defer server.Close()

			state, tools, temporary, home := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
			ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
			if mode == "untrusted-ca" {
				other := httptest.NewTLSServer(http.NotFoundHandler())
				ca = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: other.Certificate().Raw})
				other.Close()
			}
			saved := map[string][]byte{
				"ca.pem": ca, "token": []byte(bootstrapToken + "\n"),
				"runtime.token": []byte("saved-runtime-token\n"), "agent-id": []byte("saved-agent-identity\n"),
				"connection.env": []byte("saved connection sentinel\n"), "pulse-agent": []byte("installed predecessor sentinel\n"),
			}
			for name, body := range saved {
				if err := os.WriteFile(filepath.Join(state, name), body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			// Fixed architecture keeps the HEAD assertion portable across Linux
			// CI hosts; it does not simulate execution of a target agent binary.
			for name, body := range map[string]string{
				"uname":     "#!/bin/sh\ncase \"$1\" in -s) echo Linux;; -m) echo x86_64;; *) exit 1;; esac\n",
				"systemctl": "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$SERVICE_LOG\"\n[ \"$*\" = 'is-active --quiet pulse-agent' ] || exit 92\nexit 3\n",
			} {
				if err := os.WriteFile(filepath.Join(tools, name), []byte(body), 0700); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", repoFile("scripts", "install.sh"),
				"--url", server.URL, "--cacert", filepath.Join(state, "ca.pem"),
				"--token-file", filepath.Join(state, "token"), "--state-dir", state,
				"--preflight-only", "--output", "json", "--non-interactive",
				"--disable-docker", "--disable-kubernetes", "--disable-proxmox")
			// A regression must not orphan curl when the outer observer expires.
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			cmd.Cancel = func() error {
				err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				if errors.Is(err, syscall.ESRCH) {
					return os.ErrProcessDone
				}
				return err
			}
			cmd.WaitDelay = 2 * time.Second
			for _, value := range os.Environ() {
				if !strings.HasPrefix(value, "PULSE_") {
					cmd.Env = append(cmd.Env, value)
				}
			}
			cmd.Env = append(cmd.Env, "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"),
				"HOME="+home, "CURL_HOME="+home, "TMPDIR="+temporary, "NO_PROXY=*", "no_proxy=*",
				"SERVICE_LOG="+filepath.Join(tools, "service.log"))
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			start := time.Now()
			err := cmd.Run()
			out := stdout.Bytes()
			elapsed := time.Since(start)
			wantExit, wantHealthExit := 14, 14
			if mode == "untrusted-ca" {
				wantExit = 11 // The separate artifact probe preserves its existing failure precedence.
			}
			if mode == "healthy" || mode == "redirect" {
				wantExit, wantHealthExit = 0, 0
			}
			exit := 0
			if err != nil {
				var failure *exec.ExitError
				if !errors.As(err, &failure) {
					t.Fatalf("preflight execution: %v", err)
				}
				exit = failure.ExitCode()
			}
			mu.Lock()
			t.Logf("full preflight elapsed=%s exit=%d observer=%v requests=%v", elapsed, exit, ctx.Err(), requests)
			mu.Unlock()
			if ctx.Err() != nil || exit != wantExit {
				t.Fatalf("preflight did not return its own result: exit=%d want=%d observer=%v\n%s", exit, wantExit, ctx.Err(), out)
			}
			if strings.HasPrefix(mode, "stalled-") || mode == "redirect-budget" {
				if elapsed < 9*time.Second || elapsed >= 17*time.Second {
					t.Fatalf("health request did not use its ten-second total budget: %s", elapsed)
				}
			}
			var events []struct {
				Phase    string `json:"phase"`
				Code     string `json:"code"`
				ExitCode int    `json:"exitCode"`
			}
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				// The existing CLI emits informational lines alongside JSON events.
				if strings.HasPrefix(line, "[INFO] ") || strings.HasPrefix(line, "[WARN] ") || strings.HasPrefix(line, "[ERROR] ") {
					continue
				}
				var event struct {
					Phase    string `json:"phase"`
					Code     string `json:"code"`
					ExitCode int    `json:"exitCode"`
				}
				if err := json.Unmarshal([]byte(line), &event); err != nil {
					t.Fatalf("non-JSON preflight output: %s", line)
				}
				events = append(events, event)
			}
			wantCode, wantComplete := "pulse_unreachable", "failed"
			if wantExit == 0 {
				wantCode, wantComplete = "pulse_reachable", "ok"
			}
			foundHealth, foundDownload := false, false
			for _, event := range events {
				if event.Phase == "preflight" && event.Code == wantCode && event.ExitCode == wantHealthExit {
					foundHealth = true
				}
				if event.Code == "agent_download_available" {
					foundDownload = true
				}
			}
			if len(events) == 0 || !foundHealth {
				t.Fatalf("missing truthful health event: %s", out)
			}
			last := events[len(events)-1]
			if last.Phase != "preflight_complete" || last.Code != wantComplete || last.ExitCode != wantExit {
				t.Fatalf("wrong final preflight result: %s", out)
			}
			mu.Lock()
			gotRequests := strings.Join(requests, ", ")
			leaked := disclosed
			mu.Unlock()
			wantRequests := "GET /api/health, HEAD /download/pulse-agent"
			if mode == "redirect" || mode == "redirect-budget" {
				wantRequests = "GET /api/health, GET /ready, HEAD /download/pulse-agent"
			}
			if mode == "untrusted-ca" {
				wantRequests = ""
			}
			if gotRequests != wantRequests || leaked || strings.Contains(stdout.String()+stderr.String(), bootstrapToken) || strings.Contains(stdout.String()+stderr.String(), "saved-runtime-token") {
				t.Fatalf("unexpected request inventory/retry or credential exposure: %s", gotRequests)
			}
			if mode != "untrusted-ca" && !foundDownload {
				t.Fatalf("health result bypassed the separate metadata check: %s", out)
			}
			if serviceLog, err := os.ReadFile(filepath.Join(tools, "service.log")); err == nil {
				if string(serviceLog) != "is-active --quiet pulse-agent\n" {
					t.Fatalf("preflight changed service state or repeated inventory: %s", serviceLog)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
			for name, want := range saved {
				got, err := os.ReadFile(filepath.Join(state, name))
				info, statErr := os.Stat(filepath.Join(state, name))
				if err != nil || statErr != nil || string(got) != string(want) || info.Mode().Perm() != 0600 {
					t.Fatalf("preflight changed saved %s", name)
				}
			}
			if entries, err := os.ReadDir(state); err != nil || len(entries) != len(saved) {
				t.Fatalf("preflight added saved state: %v", err)
			}
			if entries, err := os.ReadDir(temporary); err != nil || len(entries) != 0 {
				t.Fatalf("preflight left temporary files: %v, %v", entries, err)
			}
		})
	}
}
