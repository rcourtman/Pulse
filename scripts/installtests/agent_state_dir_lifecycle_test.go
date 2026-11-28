//go:build unix

package installtests

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

type agentLifecycleControlPlane struct {
	mu              sync.Mutex
	online          bool
	bootstrapToken  string
	runtimeToken    string
	canonicalID     string
	enrollmentCount int
	reportCount     int
	lastReportToken string
	lastReportID    string
	lastCommands    bool
	requests        map[string]int
	enrolledAt      time.Time
	firstReportAt   time.Time
}

type agentLifecycleSnapshot struct {
	online          bool
	enrollmentCount int
	reportCount     int
	report503s      int
	lastReportToken string
	lastReportID    string
	lastCommands    bool
	requests        string
	firstReport     string
	reportIdentity  string
}

func (s *agentLifecycleControlPlane) setCredentials(bootstrapToken, runtimeToken, canonicalID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bootstrapToken = bootstrapToken
	s.runtimeToken = runtimeToken
	s.canonicalID = canonicalID
}

func (s *agentLifecycleControlPlane) setOnline(online bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.online = online
}

func (s *agentLifecycleControlPlane) snapshot() agentLifecycleSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]string, 0, len(s.requests))
	for key := range s.requests {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	counts := make([]string, 0, len(keys))
	for _, key := range keys {
		counts = append(counts, fmt.Sprintf("%s=%d", key, s.requests[key]))
	}
	// These timestamps describe the first enrollment/report over the fixture's
	// lifetime, not a new latency sample on restart, outage or credential rotation.
	firstReport := "no-report-attempt"
	if !s.firstReportAt.IsZero() {
		firstReport = "no-successful-enrollment"
		if !s.enrolledAt.IsZero() {
			if s.firstReportAt.Before(s.enrolledAt) {
				firstReport = "before-successful-enrollment"
			} else {
				firstReport = s.firstReportAt.Sub(s.enrolledAt).String()
			}
		}
	}
	reportIdentity := "missing"
	if s.lastReportID != "" {
		reportIdentity = "other"
		if s.lastReportID == s.canonicalID {
			reportIdentity = "canonical"
		}
	}
	return agentLifecycleSnapshot{
		requests:        strings.Join(counts, ","),
		firstReport:     firstReport,
		reportIdentity:  reportIdentity,
		online:          s.online,
		enrollmentCount: s.enrollmentCount,
		reportCount:     s.reportCount,
		report503s:      s.requests["report/503"],
		lastReportToken: s.lastReportToken,
		lastReportID:    s.lastReportID,
		lastCommands:    s.lastCommands,
	}
}

// Keep only fixed path classes and numeric statuses, never request paths,
// headers or bodies. A report attempt is observable even when token admission
// fails, the server is offline, or the response has not yet completed.
func (s *agentLifecycleControlPlane) serveHTTP(w http.ResponseWriter, r *http.Request) {
	kind := "other"
	switch {
	case r.URL.Path == "/api/agents/agent/report":
		kind = "report"
	case r.URL.Path == "/api/agents/agent/enroll":
		kind = "enrollment"
	case r.URL.Path == "/api/agents/agent/lookup":
		kind = "lookup"
	case strings.HasPrefix(r.URL.Path, "/api/agents/agent/") && strings.HasSuffix(r.URL.Path, "/config"):
		kind = "config"
	}
	s.mu.Lock()
	if s.requests == nil {
		s.requests = make(map[string]int)
	}
	s.requests[kind+"/started"]++
	if kind == "report" && s.firstReportAt.IsZero() {
		s.firstReportAt = time.Now()
	}
	s.mu.Unlock()
	status := &lifecycleStatusWriter{ResponseWriter: w}
	defer func() {
		code := status.code
		if code == 0 {
			code = http.StatusOK
		}
		s.mu.Lock()
		s.requests[fmt.Sprintf("%s/%d", kind, code)]++
		s.mu.Unlock()
	}()
	s.serveRequest(status, r)
}

type lifecycleStatusWriter struct {
	http.ResponseWriter
	code int
}

func (w *lifecycleStatusWriter) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *lifecycleStatusWriter) Write(data []byte) (int, error) {
	if w.code == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (s *agentLifecycleControlPlane) serveRequest(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if !s.online {
		s.mu.Unlock()
		http.Error(w, "server restarting", http.StatusServiceUnavailable)
		return
	}
	bootstrapToken := s.bootstrapToken
	runtimeToken := s.runtimeToken
	canonicalID := s.canonicalID
	s.mu.Unlock()

	switch {
	case r.URL.Path == "/api/agents/agent/lookup":
		http.Error(w, "not found", http.StatusNotFound)
	case strings.HasPrefix(r.URL.Path, "/api/agents/agent/") && strings.HasSuffix(r.URL.Path, "/config"):
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"config":{}}`)
	case r.URL.Path == "/api/agents/agent/enroll":
		if got := r.Header.Get("X-API-Token"); got != bootstrapToken {
			http.Error(w, "bad bootstrap token", http.StatusUnauthorized)
			return
		}
		var payload struct {
			CommandsEnabled bool `json:"commandsEnabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.enrollmentCount++
		if s.enrolledAt.IsZero() {
			s.enrolledAt = time.Now()
		}
		s.lastCommands = payload.CommandsEnabled
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"agentId":      canonicalID,
			"runtimeToken": runtimeToken,
		})
	case r.URL.Path == "/api/agents/agent/report":
		token := r.Header.Get("X-API-Token")
		if token != runtimeToken {
			http.Error(w, "bad runtime token", http.StatusUnauthorized)
			return
		}
		reportID := ""
		if gz, err := gzip.NewReader(r.Body); err == nil {
			var report struct {
				Agent struct {
					ID string `json:"id"`
				} `json:"agent"`
			}
			if json.NewDecoder(gz).Decode(&report) == nil {
				reportID = report.Agent.ID
			}
			_ = gz.Close()
		}
		s.mu.Lock()
		s.reportCount++
		s.lastReportToken = token
		s.lastReportID = reportID
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"agentId": canonicalID,
		})
	default:
		http.NotFound(w, r)
	}
}

func waitForLifecycleState(t *testing.T, timeout time.Duration, describe string, predicate func(agentLifecycleSnapshot) bool, state *agentLifecycleControlPlane, proc *runningLifecycleAgent) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		snapshot := state.snapshot()
		if predicate(snapshot) {
			return
		}
		if result, exited := proc.result(); exited {
			t.Fatalf("pulse-agent exited before %s: %s; %s", describe, result, snapshot.diagnostic())
		}
		time.Sleep(100 * time.Millisecond)
	}
	final := state.snapshot()
	t.Fatalf("timed out waiting for %s; %s", describe, final.diagnostic())
}

func (s agentLifecycleSnapshot) diagnostic() string {
	return fmt.Sprintf("enrollments=%d reports=%d commands=%t requests=[%s] first_report_after_first_enrollment=%s report_id=%s",
		s.enrollmentCount, s.reportCount, s.lastCommands, s.requests, s.firstReport, s.reportIdentity)
}

func buildLifecycleAgent(t *testing.T) string {
	t.Helper()
	return buildPulseAgentLifecycleBinary(t)
}

func renderLifecycleService(t *testing.T, stateDir, stateSource, unitPath, pulseURL, token string, commandsEnabled, recoverExisting bool) {
	t.Helper()
	commandFlag := "false"
	if commandsEnabled {
		commandFlag = "true"
	}
	recovery := ""
	if recoverExisting {
		recovery = `
			PULSE_URL=""
			PULSE_TOKEN=""
			AGENT_ID=""
			HOSTNAME_OVERRIDE=""
			REPORT_IP=""
			INSECURE="false"
			SERVER_FINGERPRINT=""
			CURL_CA_BUNDLE=""
			recover_connection_state "$STATE_DIR/connection.env"
		`
	}
	script := `
		set -euo pipefail
		STATE_DIR="` + stateDir + `"
		DEFAULT_STATE_DIR="` + stateDir + `"
		STATE_DIR_SOURCE="` + stateSource + `"
		TRUENAS_STATE_DIR="` + filepath.Join(filepath.Dir(stateDir), "truenas-state") + `"
		PULSE_URL="` + pulseURL + `"
		IFS= read -r PULSE_TOKEN
		INTERVAL="1s"
		ENABLE_HOST="true"
		ENABLE_DOCKER="false"
		DOCKER_EXPLICIT="true"
		ENABLE_KUBERNETES="false"
		KUBECONFIG_PATH=""
		ENABLE_PROXMOX="false"
		PROXMOX_TYPE=""
		INSECURE="false"
		SERVER_FINGERPRINT=""
		OBSERVERS_FILE=""
		ENABLE_COMMANDS="` + commandFlag + `"
		LEAST_PRIVILEGE="false"
		LEAST_PRIVILEGE_USER="pulse-agent"
		INSTALLER_LIFECYCLE_DIR="` + filepath.Join(filepath.Dir(stateDir), "installer-lifecycle") + `"
		GRANT_SMART="false"
		GRANT_PCT="false"
		HEALTH_ADDR_SET="true"
		HEALTH_ADDR=""
		ENROLL="true"
		KUBE_INCLUDE_ALL_PODS="false"
		KUBE_INCLUDE_ALL_DEPLOYMENTS="false"
		AGENT_ID=""
		HOSTNAME_OVERRIDE="state-lifecycle-host"
		REPORT_IP=""
		DISK_EXCLUDES=()
		CURL_CA_BUNDLE=""
		RUNTIME_TOKEN_FILE=""
		RUNTIME_TOKEN_CHANGED="false"
		SYSTEMD_ENV_LINES=""
		SHELL_EXPORT_LINES=""
		SAVED_INSTALL_SCRIPT=""
		EXIT_GENERAL=1
		TMP_FILES=()
		NON_INTERACTIVE="true"
		log_info() { :; }
		log_warn() { :; }
		fail() { printf 'FAIL:%s\n' "$1" >&2; return 99; }
		curl() { return 1; }
` + extractLifecyclePersistenceShellFunctions(t) + `
` + extractInstallShellFunction(t, "write_connection_state_value") + `
` + extractInstallShellFunction(t, "read_connection_state_value") + `
` + extractTokenRecoveryShellFunctions(t) + `
` + extractInstallShellFunction(t, "recover_connection_state") + `
` + extractInstallShellFunction(t, "ensure_runtime_token_file") + `
` + extractInstallShellFunction(t, "build_exec_arg_items") + `
` + extractInstallShellFunction(t, "join_exec_arg_items") + `
` + extractInstallShellFunction(t, "build_exec_args") + `
` + extractInstallShellFunction(t, "systemd_agent_requires_lxc_attach") + `
` + extractInstallShellFunction(t, "systemd_agent_may_attach_lxc") + `
` + extractInstallShellFunction(t, "render_systemd_agent_unit") + `
` + extractInstallShellFunction(t, "save_connection_info") + recovery + `
		ensure_runtime_token_file "$STATE_DIR"
		build_exec_args
		render_systemd_agent_unit "` + unitPath + `" "/test/pulse-agent" "$EXEC_ARGS" "network-online.target" "network-online.target" "root" ""
		save_connection_info "$STATE_DIR"
	`
	cmd := exec.Command("bash", "-c", script)
	cmd.Stdin = strings.NewReader(token + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("render lifecycle service: %v\n%s", err, out)
	}
}

// Failure diagnostics may snapshot output while the child is still running.
type lifecycleAgentOutput struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lifecycleAgentOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

func (b *lifecycleAgentOutput) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

type runningLifecycleAgent struct {
	cmd     *exec.Cmd
	logs    *lifecycleAgentOutput
	done    chan struct{}
	waitErr error // Read only after done closes; exactly one goroutine calls Wait.
	stopped bool
}

func (p *runningLifecycleAgent) result() (string, bool) {
	select {
	case <-p.done:
		if p.waitErr != nil {
			return p.waitErr.Error(), true
		}
		return "exit status 0", true
	default:
		return "exit not observed", false
	}
}

func (p *runningLifecycleAgent) diagnostics() string {
	result, _ := p.result()
	return fmt.Sprintf("process result: %s\n%s", result, lifecycleDiagnosticOutput(p.logs.String()))
}

// Only these synthetic credentials are supplied by this fixture. Keep the raw
// output for credential-leak assertions; redact before bounding failure output.
func lifecycleDiagnosticOutput(output string) string {
	output = strings.NewReplacer(
		"bootstrap-one", "[fixture credential]", "runtime-one", "[fixture credential]",
		"bootstrap-two", "[fixture credential]", "runtime-two", "[fixture credential]",
	).Replace(output)
	const limit = 256 * 1024
	if len(output) > limit {
		return fmt.Sprintf("output_truncated=true omitted_bytes=%d\n%s", len(output)-limit, output[:limit])
	}
	return "output_truncated=false\n" + output
}

func startLifecycleProcess(t *testing.T, cmd *exec.Cmd) *runningLifecycleAgent {
	t.Helper()
	p := &runningLifecycleAgent{cmd: cmd, logs: &lifecycleAgentOutput{}, done: make(chan struct{})}
	cmd.Stdout, cmd.Stderr = p.logs, p.logs
	// A descendant holding a pipe must not prevent the one Wait observer from
	// returning after the leader exits. ErrWaitDelay remains a failed result.
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Start(); err != nil {
		t.Fatalf("start pulse-agent: %v", err)
	}
	go func() {
		p.waitErr = cmd.Wait()
		close(p.done)
	}()
	t.Cleanup(func() {
		if _, exited := p.result(); !exited {
			if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				t.Errorf("kill lifecycle agent during cleanup: %v", err)
			}
			select {
			case <-p.done:
			case <-time.After(10 * time.Second):
				t.Error("lifecycle agent cleanup did not observe process exit and output drainage")
			}
		}
		if t.Failed() {
			t.Logf("pulse-agent failure evidence (owned leader/output only):\n%s", p.diagnostics())
		}
	})
	return p
}

func startLifecycleAgent(t *testing.T, binaryPath, pulseURL, stateDir string, commandsEnabled bool) *runningLifecycleAgent {
	t.Helper()
	args := []string{
		"--url", pulseURL,
		"--token-file", filepath.Join(stateDir, "token"),
		"--state-dir", stateDir,
		"--interval", "1s",
		"--hostname", "state-lifecycle-host",
		"--enable-host",
		"--enable-docker=false",
		"--disable-auto-update",
		"--health-addr", "",
		"--enroll",
	}
	if commandsEnabled {
		args = append(args, "--enable-commands")
	}
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(), "PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED=false")
	p := startLifecycleProcess(t, cmd)
	for _, arg := range cmd.Args {
		if strings.Contains(arg, "bootstrap-") || strings.Contains(arg, "runtime-") {
			t.Fatalf("agent argv leaked token: %q", cmd.Args)
		}
	}
	return p
}

func (p *runningLifecycleAgent) stop(t *testing.T) string {
	t.Helper()
	if p.stopped {
		return p.logs.String()
	}
	if result, exited := p.result(); exited {
		t.Fatalf("pulse-agent exited before requested shutdown: %s\n%s", result, p.diagnostics())
	}
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signal pulse-agent: %v", err)
	}
	select {
	case <-p.done:
		if p.waitErr != nil {
			t.Fatalf("pulse-agent shutdown: %s", p.diagnostics())
		}
	case <-time.After(10 * time.Second):
		_ = p.cmd.Process.Kill()
		t.Fatalf("pulse-agent did not stop\n%s", p.diagnostics())
	}
	p.stopped = true
	return p.logs.String()
}

func assertPrivateLifecycleFile(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != want {
		t.Fatalf("%s mode = %o, want %o", path, info.Mode().Perm(), want)
	}
}

func TestPulseAgentStateDirLifecycleIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("real agent lifecycle integration")
	}
	// Both the agent and its control plane use this test's private state and
	// loopback listener. Keep real lifecycle coverage within Go's worker budget.
	t.Parallel()
	binaryPath := buildLifecycleAgent(t)

	for _, tc := range []struct {
		name            string
		commandsEnabled bool
		customState     bool
	}{
		{name: "default_state_commands_disabled", commandsEnabled: false, customState: false},
		{name: "custom_state_commands_enabled", commandsEnabled: true, customState: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			stateName := "default-state"
			if tc.customState {
				stateName = "custom-state"
			}
			stateDir := filepath.Join(root, stateName)
			unitPath := filepath.Join(root, "pulse-agent.service")

			controlPlane := &agentLifecycleControlPlane{online: true}
			controlPlane.setCredentials("bootstrap-one", "runtime-one", "agent-one")
			server := httptest.NewServer(http.HandlerFunc(controlPlane.serveHTTP))
			t.Cleanup(server.Close) // Stop/drain the agent before closing its fixture server.

			stateSource := "default"
			if tc.customState {
				stateSource = "explicit"
			}
			renderLifecycleService(t, stateDir, stateSource, unitPath, server.URL, "bootstrap-one", tc.commandsEnabled, false)
			unit, err := os.ReadFile(unitPath)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(unit), "--state-dir "+stateDir) ||
				!strings.Contains(string(unit), "--token-file "+filepath.Join(stateDir, "token")) {
				t.Fatalf("generated service does not own canonical state paths:\n%s", unit)
			}
			if strings.Contains(string(unit), "bootstrap-one") || strings.Contains(string(unit), "runtime-one") {
				t.Fatalf("generated service leaked a token:\n%s", unit)
			}
			if tc.commandsEnabled != strings.Contains(string(unit), "--enable-commands") {
				t.Fatalf("generated service command mode mismatch:\n%s", unit)
			}

			proc := startLifecycleAgent(t, binaryPath, server.URL, stateDir, tc.commandsEnabled)
			waitForLifecycleState(t, 20*time.Second, "initial enrollment and report", func(state agentLifecycleSnapshot) bool {
				return state.enrollmentCount == 1 && state.reportCount >= 1 &&
					state.lastReportToken == "runtime-one" && state.lastReportID == "agent-one" &&
					state.lastCommands == tc.commandsEnabled
			}, controlPlane, proc)
			logOutput := proc.stop(t)

			for path, mode := range map[string]os.FileMode{
				stateDir:                                  0700,
				filepath.Join(stateDir, "token"):          0600,
				filepath.Join(stateDir, "runtime.token"):  0600,
				filepath.Join(stateDir, "agent-id"):       0600,
				filepath.Join(stateDir, "connection.env"): 0600,
			} {
				assertPrivateLifecycleFile(t, path, mode)
			}

			beforeRestart := controlPlane.snapshot()
			proc = startLifecycleAgent(t, binaryPath, server.URL, stateDir, tc.commandsEnabled)
			waitForLifecycleState(t, 20*time.Second, "restart report with persisted identity", func(state agentLifecycleSnapshot) bool {
				return state.reportCount > beforeRestart.reportCount &&
					state.lastReportToken == "runtime-one" && state.lastReportID == "agent-one"
			}, controlPlane, proc)
			if got := controlPlane.snapshot().enrollmentCount; got != 1 {
				t.Fatalf("ordinary restart re-enrolled unexpectedly: enrollments=%d", got)
			}

			beforeOutage := controlPlane.snapshot()
			controlPlane.setOnline(false)
			// Do not reopen after a fixed sleep: a slow report cadence could
			// otherwise make this just another successful steady-state report.
			waitForLifecycleState(t, 20*time.Second, "HTTP 503 served for an agent report during the control-plane outage", func(state agentLifecycleSnapshot) bool {
				return state.report503s > beforeOutage.report503s
			}, controlPlane, proc)
			beforeRecovery := controlPlane.snapshot()
			controlPlane.setOnline(true)
			waitForLifecycleState(t, 20*time.Second, "accepted report with the saved identity after recovery", func(state agentLifecycleSnapshot) bool {
				return state.reportCount > beforeRecovery.reportCount && state.lastReportToken == "runtime-one" &&
					state.lastReportID == "agent-one" && state.enrollmentCount == beforeOutage.enrollmentCount
			}, controlPlane, proc)
			logOutput += proc.stop(t)
			t.Logf("control-plane outage and recovery: %s", controlPlane.snapshot().diagnostic())

			renderLifecycleService(t, stateDir, stateSource, unitPath, server.URL, "bootstrap-one", tc.commandsEnabled, true)
			if _, err := os.Stat(filepath.Join(stateDir, "runtime.token")); err != nil {
				t.Fatalf("update did not preserve runtime enrollment token: %v", err)
			}

			controlPlane.setCredentials("bootstrap-two", "runtime-two", "agent-two")
			renderLifecycleService(t, stateDir, stateSource, unitPath, server.URL, "bootstrap-two", tc.commandsEnabled, false)
			if _, err := os.Stat(filepath.Join(stateDir, "runtime.token")); !os.IsNotExist(err) {
				t.Fatalf("fresh bootstrap did not clear stale runtime token: %v", err)
			}
			proc = startLifecycleAgent(t, binaryPath, server.URL, stateDir, tc.commandsEnabled)
			waitForLifecycleState(t, 20*time.Second, "re-enrollment and canonical report", func(state agentLifecycleSnapshot) bool {
				return state.enrollmentCount == 2 && state.lastReportToken == "runtime-two" &&
					state.lastReportID == "agent-two" && state.lastCommands == tc.commandsEnabled
			}, controlPlane, proc)
			logOutput += proc.stop(t)

			for _, secret := range []string{"bootstrap-one", "runtime-one", "bootstrap-two", "runtime-two"} {
				if strings.Contains(logOutput, secret) {
					t.Fatalf("agent logs leaked %q:\n%s", secret, logOutput)
				}
			}

			removeScript := `
				set -euo pipefail
				STATE_DIR="` + stateDir + `"
				STATE_DIR_REMOVAL_AUTHORITY="$STATE_DIR"
				log_warn() { :; }
` + extractInstallShellFunction(t, "remove_authorized_runtime_dir") + `
` + extractInstallShellFunction(t, "remove_agent_state_dir") + `
				remove_agent_state_dir "$STATE_DIR"
			`
			if out, err := exec.Command("bash", "-c", removeScript).CombinedOutput(); err != nil {
				t.Fatalf("uninstall state cleanup: %v\n%s", err, out)
			}
			if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
				t.Fatalf("uninstall did not remove canonical state directory: %v", err)
			}

			connectionState, err := os.ReadFile(filepath.Join(root, stateName, "connection.env"))
			if err == nil {
				t.Fatalf("connection state survived uninstall: %s", connectionState)
			}
		})
	}
}
