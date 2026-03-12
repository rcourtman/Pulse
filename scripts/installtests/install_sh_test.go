package installtests

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// TestConnectionEnvRecovery verifies the grep/sed logic that parses connection.env
// without using shell source (to prevent injection).
func TestConnectionEnvRecovery(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantURL string
		wantTok string
	}{
		{
			name:    "single-quoted values",
			content: "PULSE_URL='http://192.168.0.98:7655'\nPULSE_TOKEN='abc123def'\n",
			wantURL: "http://192.168.0.98:7655",
			wantTok: "abc123def",
		},
		{
			name:    "unquoted values",
			content: "PULSE_URL=http://10.0.0.1:7655\nPULSE_TOKEN=deadbeef\n",
			wantURL: "http://10.0.0.1:7655",
			wantTok: "deadbeef",
		},
		{
			name:    "https URL",
			content: "PULSE_URL='https://pulse.example.com'\nPULSE_TOKEN='aabbccdd'\n",
			wantURL: "https://pulse.example.com",
			wantTok: "aabbccdd",
		},
		{
			name:    "extra whitespace lines",
			content: "\nPULSE_URL='http://host:7655'\n\nPULSE_TOKEN='tok123'\n\n",
			wantURL: "http://host:7655",
			wantTok: "tok123",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			connFile := filepath.Join(dir, "connection.env")
			if err := os.WriteFile(connFile, []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}

			// Run the same grep+sed recovery logic used by install.sh
			script := `
				CONN_ENV="` + connFile + `"
				PULSE_URL=$(grep '^PULSE_URL=' "$CONN_ENV" 2>/dev/null | head -1 | sed "s/^PULSE_URL=//; s/^'//; s/'$//" || true)
				PULSE_TOKEN=$(grep '^PULSE_TOKEN=' "$CONN_ENV" 2>/dev/null | head -1 | sed "s/^PULSE_TOKEN=//; s/^'//; s/'$//" || true)
				echo "URL=${PULSE_URL}"
				echo "TOKEN=${PULSE_TOKEN}"
			`
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if err != nil {
				t.Fatalf("bash: %v\n%s", err, out)
			}

			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			gotURL, gotTok := "", ""
			for _, line := range lines {
				if strings.HasPrefix(line, "URL=") {
					gotURL = strings.TrimPrefix(line, "URL=")
				}
				if strings.HasPrefix(line, "TOKEN=") {
					gotTok = strings.TrimPrefix(line, "TOKEN=")
				}
			}

			if gotURL != tc.wantURL {
				t.Errorf("URL = %q, want %q", gotURL, tc.wantURL)
			}
			if gotTok != tc.wantTok {
				t.Errorf("TOKEN = %q, want %q", gotTok, tc.wantTok)
			}
		})
	}
}

// TestAgentIDFileRecovery verifies the agent-id file lookup priority:
// /var/lib/pulse-agent/agent-id > /boot/config/plugins/pulse-agent/agent-id
func TestAgentIDFileRecovery(t *testing.T) {
	cases := []struct {
		name   string
		files  map[string]string // relative path -> content
		wantID string
	}{
		{
			name: "primary location",
			files: map[string]string{
				"var/lib/pulse-agent/agent-id": "uuid-primary",
			},
			wantID: "uuid-primary",
		},
		{
			name: "secondary location (Unraid)",
			files: map[string]string{
				"boot/config/plugins/pulse-agent/agent-id": "uuid-unraid",
			},
			wantID: "uuid-unraid",
		},
		{
			name: "primary takes precedence",
			files: map[string]string{
				"var/lib/pulse-agent/agent-id":             "uuid-primary",
				"boot/config/plugins/pulse-agent/agent-id": "uuid-unraid",
			},
			wantID: "uuid-primary",
		},
		{
			name:   "no file found",
			files:  map[string]string{},
			wantID: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()

			for relPath, content := range tc.files {
				fullPath := filepath.Join(root, relPath)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			// Replicate the install.sh agent-id recovery loop
			script := `
				AGENT_ID=""
				for aid_path in "` + root + `/var/lib/pulse-agent/agent-id" "` + root + `/boot/config/plugins/pulse-agent/agent-id"; do
					if [[ -f "$aid_path" ]]; then
						AGENT_ID=$(cat "$aid_path")
						break
					fi
				done
				echo "$AGENT_ID"
			`
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if err != nil {
				t.Fatalf("bash: %v\n%s", err, out)
			}

			got := strings.TrimSpace(string(out))
			if got != tc.wantID {
				t.Errorf("agent-id = %q, want %q", got, tc.wantID)
			}
		})
	}
}

// TestUnraidGoScriptCleanup verifies the sed commands that remove pulse entries
// from /boot/config/go without disturbing other entries.
func TestUnraidGoScriptCleanup(t *testing.T) {
	cases := []struct {
		name   string
		before string
		after  string
	}{
		{
			name: "pulse entry with trailing blank line",
			before: `#!/bin/bash
/boot/config/pulse/telegraf/start_telegraf.sh

# Pulse Agent
bash /boot/config/plugins/pulse-agent/start-pulse-agent.sh

# Other stuff
echo hello
`,
			// The comment and command lines are removed; the trailing blank line remains.
			// This is harmless in /boot/config/go.
			after: `#!/bin/bash
/boot/config/pulse/telegraf/start_telegraf.sh


# Other stuff
echo hello
`,
		},
		{
			name: "pulse entry without trailing blank line",
			before: `#!/bin/bash
# Pulse Agent
bash /boot/config/plugins/pulse-agent/start-pulse-agent.sh
# Other stuff
echo hello
`,
			after: `#!/bin/bash
# Other stuff
echo hello
`,
		},
		{
			name: "no pulse entries - unchanged",
			before: `#!/bin/bash
echo hello
echo world
`,
			after: `#!/bin/bash
echo hello
echo world
`,
		},
		{
			name: "telegraf line containing pulse is kept",
			before: `#!/bin/bash
/boot/config/pulse/telegraf/start_telegraf.sh
# Pulse Agent
bash /boot/config/plugins/pulse-agent/start-pulse-agent.sh

echo done
`,
			// The telegraf line is NOT removed (no "pulse-agent" in it).
			// Comment and command lines are deleted individually.
			after: `#!/bin/bash
/boot/config/pulse/telegraf/start_telegraf.sh

echo done
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			goScript := filepath.Join(dir, "go")
			if err := os.WriteFile(goScript, []byte(tc.before), 0755); err != nil {
				t.Fatal(err)
			}

			// Run the exact same sed commands from install.sh (line-by-line, not range-based)
			script := `
				GO_SCRIPT="` + goScript + `"
				# Remove unified agent entries
				sed -i '' '/^# Pulse Agent$/d' "$GO_SCRIPT" 2>/dev/null || sed -i '/^# Pulse Agent$/d' "$GO_SCRIPT" 2>/dev/null || true
				sed -i '' '/pulse-agent/d' "$GO_SCRIPT" 2>/dev/null || sed -i '/pulse-agent/d' "$GO_SCRIPT" 2>/dev/null || true
				`
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if err != nil {
				t.Fatalf("bash: %v\n%s", err, out)
			}

			got, err := os.ReadFile(goScript)
			if err != nil {
				t.Fatal(err)
			}

			if string(got) != tc.after {
				t.Errorf("go script mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, tc.after)
			}
		})
	}
}

// TestAPIDeregistrationCurl verifies the curl command sends the correct
// JSON payload and headers to the canonical uninstall endpoint.
func TestAPIDeregistrationCurl(t *testing.T) {
	var (
		mu         sync.Mutex
		gotMethod  string
		gotPath    string
		gotBody    map[string]string
		gotHeaders http.Header
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotHeaders = r.Header.Clone()
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.WriteHeader(200)
		w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	agentID := "test-uuid-1234"
	token := "deadbeef0123456789"

	script := `
		PULSE_URL="` + srv.URL + `"
		PULSE_TOKEN="` + token + `"
		AGENT_ID="` + agentID + `"
		CURL_ARGS=(-fsSL --connect-timeout 5 -X POST -H "Content-Type: application/json" -H "X-API-Token: ${PULSE_TOKEN}")
		curl "${CURL_ARGS[@]}" -d "{\"agentId\": \"${AGENT_ID}\"}" "${PULSE_URL}/api/agents/agent/uninstall" >/dev/null 2>&1 || true
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/agents/agent/uninstall" {
		t.Errorf("path = %q, want /api/agents/agent/uninstall", gotPath)
	}
	if gotBody["agentId"] != agentID {
		t.Errorf("body agentId = %q, want %q", gotBody["agentId"], agentID)
	}
	if got := gotHeaders.Get("X-API-Token"); got != token {
		t.Errorf("X-API-Token = %q, want %q", got, token)
	}
	if got := gotHeaders.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}

func extractInstallShellFunction(t *testing.T, name string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("..", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	pattern := regexp.MustCompile(`(?ms)^` + regexp.QuoteMeta(name) + `\(\) \{\n.*?^\}`)
	match := pattern.Find(content)
	if match == nil {
		t.Fatalf("could not find %s in install.sh", name)
	}
	return string(match)
}

func extractInstallShellSection(t *testing.T, startMarker string, endMarker string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("..", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	text := string(content)
	start := strings.Index(text, startMarker)
	if start == -1 {
		t.Fatalf("could not find start marker %q in install.sh", startMarker)
	}
	end := strings.Index(text[start:], endMarker)
	if end == -1 {
		t.Fatalf("could not find end marker %q in install.sh", endMarker)
	}

	return text[start : start+end]
}

func TestPlainHTTPInstallAutoEnablesInsecure(t *testing.T) {
	script := `
		log_info() { :; }
` + extractInstallShellFunction(t, "pulse_url_uses_plain_http") + `
` + extractInstallShellFunction(t, "auto_enable_insecure_for_plain_http_url") + `
		PULSE_URL="http://192.168.0.98:7655"
		INSECURE="false"
		auto_enable_insecure_for_plain_http_url
		echo "$INSECURE"
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "true" {
		t.Fatalf("INSECURE = %q, want true", got)
	}
}

func TestHTTPSInstallKeepsInsecureDisabledByDefault(t *testing.T) {
	script := `
		log_info() { :; }
` + extractInstallShellFunction(t, "pulse_url_uses_plain_http") + `
` + extractInstallShellFunction(t, "auto_enable_insecure_for_plain_http_url") + `
		PULSE_URL="https://pulse.example.com"
		INSECURE="false"
		auto_enable_insecure_for_plain_http_url
		echo "$INSECURE"
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "false" {
		t.Fatalf("INSECURE = %q, want false", got)
	}
}

func TestBuildExecArgsArrayPersistsInsecureForPlainHTTPInstall(t *testing.T) {
	script := `
		log_info() { :; }
` + extractInstallShellFunction(t, "pulse_url_uses_plain_http") + `
` + extractInstallShellFunction(t, "auto_enable_insecure_for_plain_http_url") + `
` + extractInstallShellFunction(t, "build_exec_args_array") + `
		PULSE_URL="http://pulse.local:7655"
		PULSE_TOKEN="deadbeef"
		INTERVAL="30s"
		ENABLE_HOST="true"
		ENABLE_DOCKER=""
		DOCKER_EXPLICIT="false"
		ENABLE_KUBERNETES=""
		KUBECONFIG_PATH=""
		ENABLE_PROXMOX=""
		PROXMOX_TYPE=""
		INSECURE="false"
		ENABLE_COMMANDS=""
		ENROLL=""
		KUBE_INCLUDE_ALL_PODS=""
		KUBE_INCLUDE_ALL_DEPLOYMENTS=""
		AGENT_ID=""
		HOSTNAME_OVERRIDE=""
		STATE_DIR=""
		DISK_EXCLUDES=()
		auto_enable_insecure_for_plain_http_url
		build_exec_args_array
		printf '%s\n' "${EXEC_ARGS_ARRAY[*]}"
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if !strings.Contains(got, "--insecure") {
		t.Fatalf("EXEC_ARGS_ARRAY missing --insecure: %s", got)
	}
}

func TestStateDirFlagIsAcceptedByInstallerParser(t *testing.T) {
	script := `
		fail() { echo "FAIL:$1"; exit 99; }
		PULSE_URL=""
		PULSE_TOKEN=""
		INTERVAL="30s"
		ENABLE_HOST="true"
		ENABLE_DOCKER=""
		DOCKER_EXPLICIT="false"
		ENABLE_KUBERNETES=""
		KUBERNETES_EXPLICIT="false"
		ENABLE_PROXMOX=""
		PROXMOX_EXPLICIT="false"
		PROXMOX_TYPE=""
		UNINSTALL="false"
		INSECURE="false"
		AGENT_ID=""
		HOSTNAME_OVERRIDE=""
		ENABLE_COMMANDS="false"
		ENROLL="false"
		KUBECONFIG_PATH=""
		KUBE_INCLUDE_ALL_PODS="false"
		KUBE_INCLUDE_ALL_DEPLOYMENTS="false"
		DISK_EXCLUDES=()
		STATE_DIR="/var/lib/pulse-agent"
		CURL_CA_BUNDLE=""
		NON_INTERACTIVE="false"
		TOKEN_FILE_PATH=""
		OUTPUT_FORMAT="text"
		PREFLIGHT_ONLY="false"
		set -- --state-dir /tmp/pulse-agent-state --non-interactive --url https://pulse.example.com --token deadbeef
` + extractInstallShellSection(t, "# --- Parse Arguments ---", "# Read token from file if --token-file was provided") + `
		printf 'STATE_DIR=%s\nNON_INTERACTIVE=%s\nPULSE_URL=%s\n' "$STATE_DIR" "$NON_INTERACTIVE" "$PULSE_URL"
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, "STATE_DIR=/tmp/pulse-agent-state") {
		t.Fatalf("STATE_DIR not parsed correctly:\n%s", got)
	}
	if !strings.Contains(got, "NON_INTERACTIVE=true") {
		t.Fatalf("NON_INTERACTIVE not parsed correctly:\n%s", got)
	}
	if !strings.Contains(got, "PULSE_URL=https://pulse.example.com") {
		t.Fatalf("PULSE_URL not parsed correctly:\n%s", got)
	}
}
