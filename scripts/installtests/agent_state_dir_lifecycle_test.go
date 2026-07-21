package installtests

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
}

type agentLifecycleSnapshot struct {
	online          bool
	enrollmentCount int
	reportCount     int
	lastReportToken string
	lastReportID    string
	lastCommands    bool
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
	return agentLifecycleSnapshot{
		online:          s.online,
		enrollmentCount: s.enrollmentCount,
		reportCount:     s.reportCount,
		lastReportToken: s.lastReportToken,
		lastReportID:    s.lastReportID,
		lastCommands:    s.lastCommands,
	}
}

func (s *agentLifecycleControlPlane) serveHTTP(w http.ResponseWriter, r *http.Request) {
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

func waitForLifecycleState(t *testing.T, timeout time.Duration, describe string, predicate func(agentLifecycleSnapshot) bool, state *agentLifecycleControlPlane) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		snapshot := state.snapshot()
		if predicate(snapshot) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	final := state.snapshot()
	t.Fatalf("timed out waiting for %s; enrollments=%d reports=%d report_id=%q commands=%t",
		describe, final.enrollmentCount, final.reportCount, final.lastReportID, final.lastCommands)
}

func buildLifecycleAgent(t *testing.T) string {
	t.Helper()
	binaryPath := filepath.Join(t.TempDir(), "pulse-agent")
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/pulse-agent")
	cmd.Dir = repoFile()
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build pulse-agent: %v\n%s", err, output)
	}
	return binaryPath
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
		NON_INTERACTIVE="true"
		log_info() { :; }
		log_warn() { :; }
		fail() { printf 'FAIL:%s\n' "$1" >&2; return 99; }
		curl() { return 1; }
` + extractInstallShellFunction(t, "write_connection_state_value") + `
` + extractInstallShellFunction(t, "read_connection_state_value") + `
` + extractInstallShellFunction(t, "recover_token_from_default_agent_token_file") + `
` + extractInstallShellFunction(t, "recover_connection_state") + `
` + extractInstallShellFunction(t, "ensure_runtime_token_file") + `
` + extractInstallShellFunction(t, "build_exec_arg_items") + `
` + extractInstallShellFunction(t, "join_exec_arg_items") + `
` + extractInstallShellFunction(t, "build_exec_args") + `
` + extractInstallShellFunction(t, "systemd_agent_requires_lxc_attach") + `
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

type runningLifecycleAgent struct {
	cmd  *exec.Cmd
	logs *bytes.Buffer
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
	logs := &bytes.Buffer{}
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(), "PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED=false")
	cmd.Stdout = logs
	cmd.Stderr = logs
	if err := cmd.Start(); err != nil {
		t.Fatalf("start pulse-agent: %v", err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	})
	for _, arg := range cmd.Args {
		if strings.Contains(arg, "bootstrap-") || strings.Contains(arg, "runtime-") {
			t.Fatalf("agent argv leaked token: %q", cmd.Args)
		}
	}
	return &runningLifecycleAgent{cmd: cmd, logs: logs}
}

func (p *runningLifecycleAgent) stop(t *testing.T) string {
	t.Helper()
	if p.cmd.Process == nil {
		return p.logs.String()
	}
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signal pulse-agent: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- p.cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("pulse-agent shutdown: %v\n%s", err, p.logs.String())
		}
	case <-time.After(10 * time.Second):
		_ = p.cmd.Process.Kill()
		t.Fatalf("pulse-agent did not stop\n%s", p.logs.String())
	}
	p.cmd.Process = nil
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
			defer server.Close()

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
			}, controlPlane)
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
			}, controlPlane)
			if got := controlPlane.snapshot().enrollmentCount; got != 1 {
				t.Fatalf("ordinary restart re-enrolled unexpectedly: enrollments=%d", got)
			}

			controlPlane.setOnline(false)
			reportsBeforeOutage := controlPlane.snapshot().reportCount
			time.Sleep(1500 * time.Millisecond)
			controlPlane.setOnline(true)
			waitForLifecycleState(t, 20*time.Second, "report recovery after server restart", func(state agentLifecycleSnapshot) bool {
				return state.reportCount > reportsBeforeOutage && state.lastReportToken == "runtime-one"
			}, controlPlane)
			logOutput += proc.stop(t)

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
			}, controlPlane)
			logOutput += proc.stop(t)

			for _, secret := range []string{"bootstrap-one", "runtime-one", "bootstrap-two", "runtime-two"} {
				if strings.Contains(logOutput, secret) {
					t.Fatalf("agent logs leaked %q:\n%s", secret, logOutput)
				}
			}

			removeScript := `
				set -euo pipefail
				STATE_DIR="` + stateDir + `"
				log_warn() { :; }
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
