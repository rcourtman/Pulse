//go:build unix

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
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

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// This is an offline, synthetic process journey, not an installed-service or
// reporter recovery test. It joins the actual host issuer, installer token
// writer, agent CLI/transport and authenticated server routes. The terminal
// prompt and the rest of the privileged installer are outside this seam.
func TestHostAgentCredentialFileHandoff(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "pulse-agent")
	buildCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binary, "./cmd/pulse-agent")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build agent from this source: %v\n%s", err, output)
	}

	for _, tc := range []struct {
		name        string
		environment bool
		format      string
		wantStatus  int
		startupErr  bool
	}{
		{name: "file only", wantStatus: http.StatusOK},
		{name: "file over environment", environment: true, wantStatus: http.StatusOK},
		{name: "outer whitespace", format: "whitespace", wantStatus: http.StatusOK},
		{name: "quoted credential", format: "quoted", wantStatus: http.StatusUnauthorized},
		{name: "assignment credential", format: "assignment", wantStatus: http.StatusUnauthorized},
		{name: "different bare credential", format: "different", wantStatus: http.StatusUnauthorized},
		{name: "empty file refuses fallback", environment: true, format: "empty", startupErr: true},
		{name: "missing file refuses fallback", environment: true, format: "missing", startupErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adminRaw := "synthetic-handoff-admin.12345678"
			envRaw := "synthetic-handoff-environment.12345678"
			admin := newTokenRecord(t, adminRaw, []string{config.ScopeWildcard}, nil)
			envRecord := newTokenRecord(t, envRaw, []string{config.ScopeAgentReport, config.ScopeAgentConfigRead}, nil)
			runtime := newHostRemovalLifecycleHTTPRuntime(t, t.TempDir(), []config.APITokenRecord{admin, envRecord})
			t.Cleanup(runtime.stop)

			issued := serveHostRemovalLifecycleRequest(t, runtime, http.MethodPost, "/api/agent-install-command", adminRaw,
				[]byte(`{"type":"host","enableCommands":false,"name":"credential-handoff"}`))
			install := decodeHostAgentInstallResponse(t, issued)
			if install.Token == "" || install.Record == nil {
				t.Fatal("host issuer did not return a scoped credential")
			}
			assertHandoffCredentialUse(t, runtime.config, install.Record.ID, false)
			config.Mu.RLock()
			for _, record := range runtime.config.APITokens {
				if record.ID == install.Record.ID && (record.HasScope(config.ScopeAgentExec) || !record.HasScope(config.ScopeAgentReport) || !record.HasScope(config.ScopeAgentConfigRead)) {
					t.Error("host issuer changed monitoring-only scopes")
				}
			}
			config.Mu.RUnlock()

			events := make(chan handoffHTTPEvent, 64)
			var requests atomic.Int64
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				event := handoffHTTPEvent{path: r.URL.Path}
				if strings.HasSuffix(r.URL.Path, "/report") {
					body, readErr := io.ReadAll(io.LimitReader(r.Body, 4<<20))
					if readErr != nil {
						t.Error("could not observe synthetic report")
					}
					r.Body = io.NopCloser(bytes.NewReader(body))
					if err := json.Unmarshal(body, &event.report); err != nil {
						t.Error("could not decode synthetic report")
					}
				}
				rec := httptest.NewRecorder()
				runtime.router.Handler().ServeHTTP(rec, r)
				for key, values := range rec.Header() {
					w.Header()[key] = values
				}
				w.WriteHeader(rec.Code)
				_, _ = w.Write(rec.Body.Bytes())
				event.status = rec.Code
				if strings.HasSuffix(event.path, "/lookup") || strings.HasSuffix(event.path, "/report") {
					select {
					case events <- event:
					default:
						t.Error("synthetic HTTP event buffer exceeded")
					}
				}
			}))
			t.Cleanup(server.Close)

			state := t.TempDir()
			tokenFile := persistHandoffToken(t, root, state, install.Token)
			if tc.format != "" {
				content := install.Token
				switch tc.format {
				case "whitespace":
					content = " \t" + content + "\r\n"
				case "quoted":
					content = `"` + content + `"`
				case "assignment":
					content = "PULSE_TOKEN=" + content
				case "different":
					content = "different-synthetic-token"
				case "empty", "missing":
					content = ""
				}
				// Adverse file representations are deliberate fixture edits, not
				// claims that the generator or installer writes these formats.
				if err := os.WriteFile(tokenFile, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
				if tc.format == "missing" {
					if err := os.Remove(tokenFile); err != nil {
						t.Fatal(err)
					}
				}
			}
			caFile := filepath.Join(t.TempDir(), "synthetic-server-ca.pem")
			if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
				t.Fatal(err)
			}
			environment := ""
			if tc.environment {
				environment = envRaw
			}
			runs := 1
			if tc.wantStatus == http.StatusOK {
				runs = 2 // Same state, file and identity after an ordinary restart.
			}
			for run := 0; run < runs; run++ {
				stop, exit, logs := startHandoffAgent(t, binary, server.URL, caFile, tokenFile, state, environment)
				if tc.startupErr {
					select {
					case err := <-exit:
						if err == nil || !strings.Contains(logs.String(), "--token-file") {
							t.Fatal("invalid selected file did not produce a startup error")
						}
					case <-time.After(15 * time.Second):
						t.Fatal("invalid selected file did not stop startup")
					}
					if requests.Load() != 0 {
						t.Fatal("invalid selected file sent an HTTP request using fallback authority")
					}
				} else {
					awaitHandoffHTTP(t, events, tc.wantStatus, run > 0)
				}
				stop()
				if !tc.startupErr {
					if err := <-exit; err != nil {
						t.Fatal("synthetic agent did not stop cleanly")
					}
				}
				if strings.Contains(logs.String(), install.Token) || strings.Contains(logs.String(), envRaw) {
					t.Fatal("agent logs disclosed a credential")
				}
			}
			assertHandoffCredentialUse(t, runtime.config, install.Record.ID, tc.wantStatus == http.StatusOK)
			assertHandoffCredentialUse(t, runtime.config, envRecord.ID, false)
			if tc.wantStatus == http.StatusOK && len(runtime.monitor.GetState().Hosts) != 1 {
				t.Fatal("process restart duplicated the host identity")
			}
			t.Logf("runs=%d report_status=%d startup_refused=%v selected_record_used=%v environment_record_used=false", runs, tc.wantStatus, tc.startupErr, tc.wantStatus == http.StatusOK)
		})
	}
}

type handoffHTTPEvent struct {
	path   string
	status int
	report agentshost.Report
}

func awaitHandoffHTTP(t *testing.T, events <-chan handoffHTTPEvent, wantStatus int, restarted bool) {
	t.Helper()
	lookup, report := false, false
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	for !lookup || !report {
		select {
		case event := <-events:
			if strings.HasSuffix(event.path, "/lookup") {
				lookup = true
				// A new valid identity is not registered until its first report.
				if event.status != wantStatus && !(wantStatus == http.StatusOK && !restarted && event.status == http.StatusNotFound) {
					t.Fatalf("lookup status = %d, want authenticated lookup or new-identity absence", event.status)
				}
			} else {
				report = true
				if event.status != wantStatus {
					t.Fatalf("report status = %d, want %d", event.status, wantStatus)
				}
				if event.report.Agent.ID != "credential-handoff-agent" || event.report.Host.Hostname != "credential-handoff.local" || event.report.Agent.CommandsEnabled {
					t.Fatal("report changed selected identity or command authority")
				}
			}
		case <-deadline.C:
			t.Fatal("agent did not complete both lookup and report")
		}
	}
}

func assertHandoffCredentialUse(t *testing.T, cfg *config.Config, id string, used bool) {
	t.Helper()
	config.Mu.RLock()
	defer config.Mu.RUnlock()
	for _, record := range cfg.APITokens {
		if record.ID == id {
			if (record.LastUsedAt != nil) != used {
				t.Fatal("credential last-used evidence does not match authentication")
			}
			return
		}
	}
	t.Fatal("credential record missing")
}

func persistHandoffToken(t *testing.T, root, state, token string) string {
	t.Helper()
	script, err := os.ReadFile(filepath.Join(root, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(script)
	start := strings.Index(text, "ensure_runtime_token_file() {")
	end := strings.Index(text, "\nclear_proxmox_state_if_needed() {")
	if start < 0 || end <= start {
		t.Fatal("canonical installer token writer not found")
	}
	cmd := exec.Command("bash", "-c", `set -euo pipefail
TMP_FILES=(); ENROLL=false; EXIT_GENERAL=1
log_info() { :; }; fail() { exit 1; }
PULSE_TOKEN=$(cat)
`+text[start:end]+`ensure_runtime_token_file "$STATE_DIR"`)
	cmd.Env = append(os.Environ(), "STATE_DIR="+state)
	cmd.Stdin = strings.NewReader(token)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("private installer token writer: %v (output bytes %d)", err, len(output))
	}
	tokenFile := filepath.Join(state, "token")
	for path, mode := range map[string]os.FileMode{state: 0700, tokenFile: 0600} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != mode {
			t.Fatal("installer token writer did not retain private file/parent modes")
		}
	}
	return tokenFile
}

func startHandoffAgent(t *testing.T, binary, url, caFile, tokenFile, state, envToken string) (func(), <-chan error, *bytes.Buffer) {
	t.Helper()
	cmd := exec.Command(binary, "--url", url, "--cacert", caFile, "--token-file", tokenFile,
		"--state-dir", state, "--agent-id", "credential-handoff-agent", "--hostname", "credential-handoff.local",
		"--enable-host", "--enable-docker=false", "--enable-kubernetes=false", "--enable-proxmox=false",
		"--enable-commands=false", "--command-authority", "monitoring-only", "--disable-auto-update", "--interval", "1s", "--health-addr", "127.0.0.1:0")
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "PULSE_") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(cmd.Env, "PULSE_TOKEN="+envToken)
	var logs bytes.Buffer
	cmd.Stdout, cmd.Stderr = &logs, &logs
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal("start synthetic agent")
	}
	exit := make(chan error, 1)
	done := make(chan struct{})
	go func() { exit <- cmd.Wait(); close(done) }()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			select {
			case <-done:
				return
			default:
			}
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				<-done
				t.Error("synthetic agent required forced cleanup")
			}
		})
	}
	t.Cleanup(stop)
	return stop, exit, &logs
}
