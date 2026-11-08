//go:build linux

package installtests

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// Run the complete non-mutating installer entry with real curl and TLS. Only
// service inventory is doubled; no binary is downloaded, installed or run.
// Stalls use the production ten-second budget, not an accelerated curl double.
func TestAgentInstallerPreflightHealthDeadline(t *testing.T) {
	for _, mode := range []string{"healthy", "redirect", "http-error", "untrusted-ca", "stalled-headers", "stalled-body", "redirect-budget", "metadata-http-error", "configured-url", "explicit-insecure", "typed-artifacts", "environment-proxy"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			bootstrapToken := strings.Repeat("a1", 32)
			// Bound setup independently. The health observer starts only when
			// that request arrives, rather than spending its budget on startup.
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			var mu sync.Mutex
			var healthStarted, metadataStarted time.Time
			var healthObserver *time.Timer
			var requests []string
			var disclosed bool
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requests = append(requests, r.Method+" "+r.URL.Path)
				if r.URL.Path == "/api/health" && healthStarted.IsZero() {
					healthStarted = time.Now()
					healthObserver = time.AfterFunc(18*time.Second, cancel)
				}
				if strings.HasPrefix(r.URL.Path, "/download/") && metadataStarted.IsZero() {
					metadataStarted = time.Now()
				}
				disclosed = disclosed || r.Header.Get("Authorization") != "" || r.Header.Get("X-API-Token") != "" ||
					r.Header.Get("X-Curlrc-Injected") != "" || strings.Contains(r.URL.String(), bootstrapToken)
				mu.Unlock()
				if r.URL.Path == "/download/pulse-agent" || r.URL.Path == "/download/pulse-agent-helper" || r.URL.Path == "/download/pulse-agent-runner" {
					if mode == "metadata-http-error" {
						http.Error(w, "fixture metadata unavailable", http.StatusServiceUnavailable)
						return
					}
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
			})
			server := httptest.NewTLSServer(handler)
			defer server.Close()
			pulseURL := server.URL
			var proxyRequests atomic.Int32
			var proxyEnv []string
			if mode == "environment-proxy" {
				proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Scheme != "http" || r.URL.Host != "pulse-proxy.fixture.invalid" {
						t.Errorf("unexpected proxy destination: %s", r.URL)
						http.Error(w, "unexpected fixture target", http.StatusBadGateway)
						return
					}
					proxyRequests.Add(1)
					handler.ServeHTTP(w, r)
				}))
				defer proxy.Close()
				pulseURL = "http://pulse-proxy.fixture.invalid"
				proxyEnv = []string{"http_proxy=" + proxy.URL, "NO_PROXY=", "no_proxy="}
			}

			state, tools, temporary, home := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
			ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
			if mode == "untrusted-ca" || mode == "explicit-insecure" {
				// httptest servers share a default certificate. Generate a
				// genuinely different trust root instead of using a second server.
				public, private, err := ed25519.GenerateKey(rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
				template := &x509.Certificate{
					SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "unrelated preflight fixture CA"},
					NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
					IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
				}
				der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
				if err != nil {
					t.Fatal(err)
				}
				ca = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
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
			// An ambient curlrc must not inject retries, TLS bypass, headers,
			// trace files or another URL into a non-mutating availability check.
			trace := filepath.Join(home, "ambient-trace")
			configPath := filepath.Join(home, ".curlrc")
			config := "insecure\nretry = 2\nretry-delay = 1\nheader = \"X-Curlrc-Injected: ambient-sentinel\"\ntrace-ascii = \"" + trace + "\"\n"
			if mode == "configured-url" {
				config += "url = \"" + server.URL + "/configured\"\n"
			}
			if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
				t.Fatal(err)
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
			cmd := exec.CommandContext(ctx, "bash", repoFile("scripts", "install.sh"),
				"--url", pulseURL, "--cacert", filepath.Join(state, "ca.pem"),
				"--token-file", filepath.Join(state, "token"), "--state-dir", state,
				"--preflight-only", "--output", "json", "--non-interactive",
				"--disable-docker", "--disable-kubernetes", "--disable-proxmox")
			if mode == "explicit-insecure" {
				cmd.Args = append(cmd.Args, "--insecure")
			}
			if mode == "typed-artifacts" {
				cmd.Args = append(cmd.Args, "--least-privilege", "--enable-privileged-helper", "--enable-action-runner")
			}
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
				if !strings.HasPrefix(value, "PULSE_") && !strings.HasSuffix(strings.ToUpper(strings.SplitN(value, "=", 2)[0]), "_PROXY") {
					cmd.Env = append(cmd.Env, value)
				}
			}
			cmd.Env = append(cmd.Env, "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"),
				"HOME="+home, "CURL_HOME="+home, "TMPDIR="+temporary, "NO_PROXY=*", "no_proxy=*",
				"SERVICE_LOG="+filepath.Join(tools, "service.log"))
			cmd.Env = append(cmd.Env, proxyEnv...)
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
			if mode == "healthy" || mode == "redirect" || mode == "configured-url" || mode == "explicit-insecure" || mode == "typed-artifacts" || mode == "environment-proxy" {
				wantExit, wantHealthExit = 0, 0
			}
			if mode == "metadata-http-error" {
				wantExit, wantHealthExit = 11, 0
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
			if healthObserver != nil {
				healthObserver.Stop()
			}
			var startupElapsed, healthElapsed time.Duration
			if !healthStarted.IsZero() {
				startupElapsed = healthStarted.Sub(start)
				healthElapsed = time.Since(healthStarted)
				if !metadataStarted.IsZero() {
					healthElapsed = metadataStarted.Sub(healthStarted)
				}
			}
			t.Logf("full preflight elapsed=%s startup=%s health-to-metadata=%s exit=%d observer=%v requests=%v", elapsed, startupElapsed, healthElapsed, exit, ctx.Err(), requests)
			mu.Unlock()
			if ctx.Err() != nil || exit != wantExit {
				t.Fatalf("preflight did not return its own result: exit=%d want=%d observer=%v\n%s", exit, wantExit, ctx.Err(), out)
			}
			if strings.HasPrefix(mode, "stalled-") || mode == "redirect-budget" {
				if healthElapsed < 9*time.Second || healthElapsed >= 17*time.Second {
					t.Fatalf("health request did not use its ten-second total budget: %s (whole preflight %s)", healthElapsed, elapsed)
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
			if wantHealthExit == 0 {
				wantCode = "pulse_reachable"
			}
			if wantExit == 0 {
				wantComplete = "ok"
			}
			foundHealth, foundDownload, foundMetadataFailure := false, false, false
			for _, event := range events {
				if event.Phase == "preflight" && event.Code == wantCode && event.ExitCode == wantHealthExit {
					foundHealth = true
				}
				if event.Code == "agent_download_unavailable" && event.ExitCode == 11 {
					foundMetadataFailure = true
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
			if mode == "typed-artifacts" {
				wantRequests += ", HEAD /download/pulse-agent-helper, HEAD /download/pulse-agent-runner"
			}
			if gotRequests != wantRequests || leaked || strings.Contains(stdout.String()+stderr.String(), bootstrapToken) || strings.Contains(stdout.String()+stderr.String(), "saved-runtime-token") {
				t.Fatalf("unexpected request inventory/retry or credential exposure: %s", gotRequests)
			}
			if mode == "metadata-http-error" {
				if foundDownload || !foundMetadataFailure {
					t.Fatalf("metadata refusal lost its own result: %s", out)
				}
			} else if mode != "untrusted-ca" && !foundDownload {
				t.Fatalf("health result bypassed the separate metadata check: %s", out)
			}
			if mode == "environment-proxy" && proxyRequests.Load() != 2 {
				t.Fatalf("preflight did not retain environment proxy support: %d requests", proxyRequests.Load())
			}
			if _, err := os.Stat(trace); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("preflight used ambient curl tracing: %v", err)
			}
			if got, err := os.ReadFile(configPath); err != nil || string(got) != config {
				t.Fatalf("preflight changed ambient config: %v", err)
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
