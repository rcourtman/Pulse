package installtests

import (
	"encoding/json"
	"io"
	"net"
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

func TestInstallSHAllowsMissingTokenForOptionalAuth(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`build_exec_arg_items() {`,
		`RUNTIME_TOKEN_FILE=""`,
		`EXEC_ARG_ITEMS+=(--token-file "$RUNTIME_TOKEN_FILE")`,
		`fail "Internal installer error: runtime token file was not prepared before service rendering." "$EXIT_GENERAL"`,
		`build_exec_args_without_token() {`,
		`build_exec_arg_items "false"`,
		`build_exec_arg_items "true"`,
		`if [[ -n "$PULSE_TOKEN" && ! "$PULSE_TOKEN" =~ ^[a-fA-F0-9]+$ ]]; then`,
		`if [[ -n "$PULSE_TOKEN" ]]; then`,
		`log_info "No API token provided; installer will configure token-optional agent runtime."`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing optional-token handling: %s", needle)
		}
	}
}

func TestInstallSHAutoDetectProxmoxKeepsRuntimeTypeUnpinned(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`if detect_proxmox; then`,
		`log_info "Proxmox detected - enabling Proxmox integration"`,
		`log_info "  Proxmox type: auto-detect all installed services"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing unpinned Proxmox auto-detect contract: %s", needle)
		}
	}

	forbidden := []string{
		`detect_proxmox_type() {`,
		`auto_type="$(detect_proxmox_type || true)"`,
		`PROXMOX_TYPE="$auto_type"`,
	}
	for _, needle := range forbidden {
		if strings.Contains(script, needle) {
			t.Fatalf("install.sh preserved stale single-type Proxmox auto-detect contract: %s", needle)
		}
	}
}

func TestInstallSHExplainsCommandExecutionForProxmoxLXCDockerInventory(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`--enable-commands       Enable Pulse command execution (disabled by default; required for Patrol actions and Proxmox LXC Docker inventory)`,
		`log_info "  Pulse command execution: $ENABLE_COMMANDS"`,
		`log_info "    Accepts Pulse-scoped command requests on this agent."`,
		`log_info "    On Proxmox nodes this is required for opted-in LXC Docker inventory via pct exec."`,
		`log_info "    The Pulse server must also be started with PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY=true."`,
		`log_info "    Command execution is off; enable only when Patrol actions or Proxmox LXC Docker inventory are needed."`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing command-execution disclosure: %s", needle)
		}
	}
	if strings.Contains(script, "Enable AI command execution") {
		t.Fatal("install.sh must not describe --enable-commands as AI command execution")
	}
}

func TestInstallSHAcceptsLegacyBooleanFlagValues(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`--enable-host=true) ENABLE_HOST="true"; HOST_EXPLICIT="true"; shift ;;`,
		`--enable-host=false) ENABLE_HOST="false"; HOST_EXPLICIT="true"; shift ;;`,
		`--enable-docker=true) ENABLE_DOCKER="true"; DOCKER_EXPLICIT="true"; shift ;;`,
		`--enable-docker=false) ENABLE_DOCKER="false"; DOCKER_EXPLICIT="true"; shift ;;`,
		`--enable-kubernetes=true) ENABLE_KUBERNETES="true"; KUBERNETES_EXPLICIT="true"; shift ;;`,
		`--enable-kubernetes=false) ENABLE_KUBERNETES="false"; KUBERNETES_EXPLICIT="true"; shift ;;`,
		`--enable-proxmox=true) ENABLE_PROXMOX="true"; PROXMOX_EXPLICIT="true"; shift ;;`,
		`--enable-proxmox=false) ENABLE_PROXMOX="false"; PROXMOX_EXPLICIT="true"; shift ;;`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing legacy boolean flag alias: %s", needle)
		}
	}
}

func TestInstallSHAgentDownloadIsServerVersionAware(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`"${PULSE_URL}/api/version"`,
		`SERVER_VERSION="$(printf '%s' "$server_version_json" | sed -n 's/.*"version"`,
		`DOWNLOAD_QUERY="${DOWNLOAD_QUERY}&serverVersion=${SERVER_VERSION}"`,
		`log_info "Pulse server version: ${SERVER_VERSION}"`,
		`Downloaded agent version (${NEW_VERSION}) does not match Pulse server version (${SERVER_VERSION})`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing version-aware agent download behavior: %s", needle)
		}
	}
}

func TestInstallSHAgentServiceSecurityDefaults(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`HEALTH_ADDR="${PULSE_HEALTH_ADDR:-}"`,
		`if [[ -n "${PULSE_HEALTH_ADDR+x}" ]]; then`,
		`--health-addr <addr>    Health/metrics listener address (default: 127.0.0.1:9191; use "" to disable)`,
		`if [[ "$HEALTH_ADDR_SET" == "true" ]]; then EXEC_ARG_ITEMS+=(--health-addr "$HEALTH_ADDR"); fi`,
		`--health-addr) HEALTH_ADDR="$2"; HEALTH_ADDR_SET="true"; shift 2 ;;`,
		`UMask=0077`,
		`local no_new_privileges="true"`,
		`NoNewPrivileges=${no_new_privileges}`,
		`PrivateTmp=true`,
		`ProtectKernelTunables=true`,
		`ProtectKernelModules=true`,
		`ProtectControlGroups=true`,
		`LockPersonality=true`,
		`local restrict_suidsgid="true"`,
		`RestrictSUIDSGID=${restrict_suidsgid}`,
		`SystemCallArchitectures=native`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing agent service security default: %s", needle)
		}
	}
}

func TestInstallSHAllowsProxmoxCommandAgentLXCAttach(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`systemd_agent_requires_lxc_attach() {`,
		`if [[ "$ENABLE_COMMANDS" != "true" || "$ENABLE_PROXMOX" != "true" ]]; then`,
		`""|pve|all)`,
		`no_new_privileges="false"`,
		`restrict_suidsgid="false"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing Proxmox command-agent LXC attach service handling: %s", needle)
		}
	}
}

func TestInstallSHPreflightChecksAgentDownloadArtifact(t *testing.T) {
	var requestedDownloadPath string
	var requestedDownloadArch string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/download/pulse-agent":
			requestedDownloadPath = r.URL.Path
			requestedDownloadArch = r.URL.Query().Get("arch")
			w.Header().Set("X-Checksum-Sha256", strings.Repeat("a", 64))
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cmd := exec.Command(
		"bash",
		repoFile("scripts", "install.sh"),
		"--url",
		server.URL,
		"--preflight-only",
		"--output",
		"json",
		"--non-interactive",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("preflight failed: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, `"code":"agent_download_available"`) {
		t.Fatalf("preflight did not report agent download availability:\n%s", got)
	}
	if requestedDownloadPath != "/download/pulse-agent" {
		t.Fatalf("download path = %q, want /download/pulse-agent", requestedDownloadPath)
	}
	if requestedDownloadArch == "" {
		t.Fatalf("download arch query was empty")
	}
}

func TestInstallSHPreflightDoesNotRequireRoot(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`if [[ $EUID -ne 0 && "$PREFLIGHT_ONLY" != "true" ]]; then`,
		`DOWNLOAD_CHECK_URL="${PULSE_URL}/download/${BINARY_NAME}?arch=${PF_ARCH_PARAM}"`,
		`CURL_DOWNLOAD_CHECK_ARGS=(-fsSI --connect-timeout 5 --max-time 30 -D "$PREFLIGHT_HEADERS" -o /dev/null)`,
		`grep -i '^X-Checksum-Sha256:' "$PREFLIGHT_HEADERS"`,
		`"agent_download_available"`,
		`"agent_download_unavailable"`,
		`"agent_download_checksum_missing"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing non-root preflight download check: %s", needle)
		}
	}
}

func TestBuildPlistProgramArgumentsUsesSharedExecArgs(t *testing.T) {
	script := `
` + extractInstallShellFunction(t, "build_exec_arg_items") + `
` + extractInstallShellFunction(t, "xml_escape") + `
` + extractInstallShellFunction(t, "append_plist_arg") + `
` + extractInstallShellFunction(t, "build_plist_program_arguments") + `
		PULSE_URL="https://pulse.example/a&b"
		PULSE_TOKEN="deadbeef"
		INTERVAL="30s"
		ENABLE_HOST="true"
		ENABLE_DOCKER="false"
		DOCKER_EXPLICIT="true"
		ENABLE_KUBERNETES="true"
		KUBECONFIG_PATH="/etc/kube config"
		ENABLE_PROXMOX="true"
		PROXMOX_TYPE="pbs"
		INSECURE="true"
		RUNTIME_TOKEN_FILE="/var/lib/pulse-agent/token"
		ENABLE_COMMANDS="true"
		HEALTH_ADDR_SET="true"
		HEALTH_ADDR=""
		ENROLL="true"
		KUBE_INCLUDE_ALL_PODS="true"
		KUBE_INCLUDE_ALL_DEPLOYMENTS="true"
		AGENT_ID="agent-1"
		HOSTNAME_OVERRIDE="Richard's Mac & Mini"
		STATE_DIR="/var/lib/pulse-agent"
		DISK_EXCLUDES=("Time Machine")
		build_plist_program_arguments "/usr/local/bin/pulse-agent"
		printf '%s\n' "$PLIST_ARGS"
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	required := []string{
		`<string>/usr/local/bin/pulse-agent</string>`,
		`<string>https://pulse.example/a&amp;b</string>`,
		`<string>--token-file</string>`,
		`<string>/var/lib/pulse-agent/token</string>`,
		`<string>--enable-docker=false</string>`,
		`<string>--enable-proxmox</string>`,
		`<string>--proxmox-type</string>`,
		`<string>pbs</string>`,
		`<string>--health-addr</string>`,
		`<string>--hostname</string>`,
		`<string>Richard's Mac &amp; Mini</string>`,
		`<string>--disk-exclude</string>`,
		`<string>Time Machine</string>`,
	}
	for _, needle := range required {
		if !strings.Contains(got, needle) {
			t.Fatalf("plist args missing %s:\n%s", needle, got)
		}
	}
	if strings.Contains(got, "--token</string>") || strings.Contains(got, "deadbeef") {
		t.Fatalf("plist args leaked raw token:\n%s", got)
	}
}

// TestConnectionEnvRecovery verifies the canonical helper logic that parses
// connection.env without using shell source (to prevent injection).
func TestConnectionEnvRecovery(t *testing.T) {
	cases := []struct {
		name         string
		content      string
		wantURL      string
		wantTok      string
		wantID       string
		wantHost     string
		wantInsecure string
		wantCACert   string
	}{
		{
			name:         "single-quoted values",
			content:      "PULSE_URL='http://192.168.0.98:7655'\nPULSE_TOKEN='abc123def'\nPULSE_AGENT_ID='agent-123'\nPULSE_HOSTNAME='node.local'\nPULSE_INSECURE_SKIP_VERIFY='true'\nPULSE_CACERT='/etc/pulse/ca.pem'\n",
			wantURL:      "http://192.168.0.98:7655",
			wantTok:      "abc123def",
			wantID:       "agent-123",
			wantHost:     "node.local",
			wantInsecure: "true",
			wantCACert:   "/etc/pulse/ca.pem",
		},
		{
			name:         "unquoted values",
			content:      "PULSE_URL=http://10.0.0.1:7655\nPULSE_TOKEN=deadbeef\nPULSE_AGENT_ID=agent-456\nPULSE_HOSTNAME=node-two.local\nPULSE_INSECURE_SKIP_VERIFY=true\nPULSE_CACERT=/opt/pulse/ca.pem\n",
			wantURL:      "http://10.0.0.1:7655",
			wantTok:      "deadbeef",
			wantID:       "agent-456",
			wantHost:     "node-two.local",
			wantInsecure: "true",
			wantCACert:   "/opt/pulse/ca.pem",
		},
		{
			name:         "https URL",
			content:      "PULSE_URL='https://pulse.example.com'\nPULSE_TOKEN='aabbccdd'\nPULSE_AGENT_ID='agent-https'\nPULSE_HOSTNAME='https-host'\nPULSE_INSECURE_SKIP_VERIFY='false'\nPULSE_CACERT='/usr/local/share/ca.pem'\n",
			wantURL:      "https://pulse.example.com",
			wantTok:      "aabbccdd",
			wantID:       "agent-https",
			wantHost:     "https-host",
			wantInsecure: "false",
			wantCACert:   "/usr/local/share/ca.pem",
		},
		{
			name:         "extra whitespace lines",
			content:      "\nPULSE_URL='http://host:7655'\n\nPULSE_TOKEN='tok123'\n\nPULSE_AGENT_ID='agent-spaced'\nPULSE_HOSTNAME='spaced.local'\n\nPULSE_INSECURE_SKIP_VERIFY='true'\n\nPULSE_CACERT='/tmp/ca.pem'\n\n",
			wantURL:      "http://host:7655",
			wantTok:      "tok123",
			wantID:       "agent-spaced",
			wantHost:     "spaced.local",
			wantInsecure: "true",
			wantCACert:   "/tmp/ca.pem",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			connFile := filepath.Join(dir, "connection.env")
			if err := os.WriteFile(connFile, []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}

			// Run the same helper logic used by install.sh
			script := `
				CONN_ENV="` + connFile + `"
				read_connection_state_value() {
					local file="$1"
					local key="$2"
					awk -F= -v key="$key" '
						$1 == key {
							value = substr($0, index($0, "=") + 1)
							sub(/^'\''/, "", value)
							sub(/'\''$/, "", value)
							print value
							exit
						}
					' "$file" 2>/dev/null || true
				}
				PULSE_URL=$(read_connection_state_value "$CONN_ENV" "PULSE_URL")
				PULSE_TOKEN=$(read_connection_state_value "$CONN_ENV" "PULSE_TOKEN")
				PULSE_AGENT_ID=$(read_connection_state_value "$CONN_ENV" "PULSE_AGENT_ID")
				PULSE_HOSTNAME=$(read_connection_state_value "$CONN_ENV" "PULSE_HOSTNAME")
				PULSE_INSECURE_SKIP_VERIFY=$(read_connection_state_value "$CONN_ENV" "PULSE_INSECURE_SKIP_VERIFY")
				PULSE_CACERT=$(read_connection_state_value "$CONN_ENV" "PULSE_CACERT")
				echo "URL=${PULSE_URL}"
				echo "TOKEN=${PULSE_TOKEN}"
				echo "AGENT_ID=${PULSE_AGENT_ID}"
				echo "HOSTNAME=${PULSE_HOSTNAME}"
				echo "INSECURE=${PULSE_INSECURE_SKIP_VERIFY}"
				echo "CACERT=${PULSE_CACERT}"
			`
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if err != nil {
				t.Fatalf("bash: %v\n%s", err, out)
			}

			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			gotURL, gotTok, gotID, gotHost, gotInsecure, gotCACert := "", "", "", "", "", ""
			for _, line := range lines {
				if strings.HasPrefix(line, "URL=") {
					gotURL = strings.TrimPrefix(line, "URL=")
				}
				if strings.HasPrefix(line, "TOKEN=") {
					gotTok = strings.TrimPrefix(line, "TOKEN=")
				}
				if strings.HasPrefix(line, "AGENT_ID=") {
					gotID = strings.TrimPrefix(line, "AGENT_ID=")
				}
				if strings.HasPrefix(line, "HOSTNAME=") {
					gotHost = strings.TrimPrefix(line, "HOSTNAME=")
				}
				if strings.HasPrefix(line, "INSECURE=") {
					gotInsecure = strings.TrimPrefix(line, "INSECURE=")
				}
				if strings.HasPrefix(line, "CACERT=") {
					gotCACert = strings.TrimPrefix(line, "CACERT=")
				}
			}

			if gotURL != tc.wantURL {
				t.Errorf("URL = %q, want %q", gotURL, tc.wantURL)
			}
			if gotTok != tc.wantTok {
				t.Errorf("TOKEN = %q, want %q", gotTok, tc.wantTok)
			}
			if gotID != tc.wantID {
				t.Errorf("AGENT_ID = %q, want %q", gotID, tc.wantID)
			}
			if gotHost != tc.wantHost {
				t.Errorf("HOSTNAME = %q, want %q", gotHost, tc.wantHost)
			}
			if gotInsecure != tc.wantInsecure {
				t.Errorf("INSECURE = %q, want %q", gotInsecure, tc.wantInsecure)
			}
			if gotCACert != tc.wantCACert {
				t.Errorf("CACERT = %q, want %q", gotCACert, tc.wantCACert)
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

func TestInstallSHUsesHostnameOverrideForUninstallLookup(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`LOOKUP_HOSTNAME="$HOSTNAME_OVERRIDE"`,
		`if [[ -z "$LOOKUP_HOSTNAME" ]]; then`,
		`LOOKUP_HOSTNAME=$(hostname 2>/dev/null || true)`,
		`LOOKUP_HOSTNAME_ESCAPED=$(url_encode "$LOOKUP_HOSTNAME")`,
		`"${PULSE_URL}/api/agents/agent/lookup?hostname=${LOOKUP_HOSTNAME_ESCAPED}"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing uninstall hostname override lookup handling: %s", needle)
		}
	}
}

func TestInstallSHUrlEncodesHostnameLookupQuery(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`url_encode() {`,
		`printf -v encoded '%%%02X' "'$c"`,
		`LOOKUP_HOSTNAME_ESCAPED=$(url_encode "$LOOKUP_HOSTNAME")`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing encoded hostname lookup transport: %s", needle)
		}
	}
}

func TestInstallSHPersistsIdentityInConnectionEnv(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`write_connection_state_value() {`,
		`read_connection_state_value() {`,
		`recover_connection_state() {`,
		`find_connection_state_file() {`,
		`write_connection_state_value "$conn_env" "PULSE_TOKEN_FILE" "$RUNTIME_TOKEN_FILE"`,
		`write_connection_state_value "$conn_env" "PULSE_AGENT_ID" "$AGENT_ID"`,
		`write_connection_state_value "$conn_env" "PULSE_HOSTNAME" "$HOSTNAME_OVERRIDE"`,
		`write_connection_state_value "$conn_env" "PULSE_INSECURE_SKIP_VERIFY" "true"`,
		`write_connection_state_value "$conn_env" "PULSE_CACERT" "$CURL_CA_BUNDLE"`,
		`recover_connection_state "$conn_env"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing persisted identity recovery: %s", needle)
		}
	}
}

func TestInstallSHRecoversSavedStateForPartialUninstallContext(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	needles := []string{
		`if [[ -z "$PULSE_URL" || -z "$PULSE_TOKEN" || -z "$AGENT_ID" || -z "$HOSTNAME_OVERRIDE" || -z "$CURL_CA_BUNDLE" || "$INSECURE" != "true" ]]; then`,
		`# Recover connection details from the canonical installer-owned state artifact`,
		`conn_env=$(find_connection_state_file || true)`,
	}
	for _, needle := range needles {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing partial uninstall saved-state recovery guard: %s", needle)
		}
	}
}

func TestInstallSHSupportsSavedStateUpdateMode(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`--update            Update an existing agent using saved connection state`,
		`UPDATE_ONLY="false"`,
		`--update) UPDATE_ONLY="true"; shift ;;`,
		`if [[ "$UPDATE_ONLY" == "true" ]]; then`,
		`update_conn_env=$(find_connection_state_file || true)`,
		`recover_connection_state "$update_conn_env"`,
		`recover_connection_state_from_existing_agent() {`,
		`recover_connection_state_from_running_agent`,
		`recover_connection_state_from_systemd_unit`,
		`recover_connection_state_from_arg_stream`,
		`recover_connection_state_from_env_stream`,
		`recover_connection_state_from_existing_agent || true`,
		`if [[ -n "$PULSE_URL" && -n "$PULSE_TOKEN" ]]; then`,
		`recover_agent_id_from_state_file() {`,
		`AGENT_ID=$(recover_agent_id_from_state_file || true)`,
		`No existing Pulse Agent connection state found. Use the install command instead.`,
		`if [[ "$UPDATE_ONLY" == "true" && "$UPGRADE_MODE" != "true" ]]; then`,
		`No existing Pulse Agent installation found to update. Use the install command instead.`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing saved-state update mode contract: %s", needle)
		}
	}
}

func TestInstallSHRecoversV5ProcessArgsForSavedStateUpdate(t *testing.T) {
	script := `
		fail() { echo "FAIL:$1"; exit 99; }
		PULSE_URL=""
		PULSE_TOKEN=""
		INTERVAL="30s"
		INTERVAL_EXPLICIT="false"
		ENABLE_HOST="true"
		HOST_EXPLICIT="false"
		ENABLE_DOCKER=""
		DOCKER_EXPLICIT="false"
		ENABLE_KUBERNETES=""
		KUBERNETES_EXPLICIT="false"
		KUBECONFIG_PATH=""
		ENABLE_PROXMOX=""
		PROXMOX_EXPLICIT="false"
		PROXMOX_TYPE=""
		INSECURE="false"
		ENABLE_COMMANDS="false"
		ENROLL="false"
		HEALTH_ADDR=""
		HEALTH_ADDR_SET="false"
		AGENT_ID=""
		HOSTNAME_OVERRIDE=""
		STATE_DIR="/var/lib/pulse-agent"
		CURL_CA_BUNDLE=""
		KUBE_INCLUDE_ALL_PODS="false"
		KUBE_INCLUDE_ALL_DEPLOYMENTS="false"
		DISK_EXCLUDES=()
		RUNTIME_TOKEN_FILE="/var/lib/pulse-agent/token"
` + extractInstallShellFunction(t, "strip_recovered_arg_quotes") + `
` + extractInstallShellFunction(t, "apply_recovered_agent_arg_value") + `
` + extractInstallShellFunction(t, "recover_connection_state_from_arg_stream") + `
` + extractInstallShellFunction(t, "build_exec_arg_items") + `
` + extractInstallShellFunction(t, "join_exec_arg_items") + `
` + extractInstallShellFunction(t, "build_exec_args") + `
		recover_connection_state_from_arg_stream <<'ARGS'
/usr/local/bin/pulse-agent
--url
http://192.168.2.96:7655
--token
deadbeef
--interval
30s
--enable-host
--enable-docker
--insecure
--agent-id
agent-123
--hostname
pve-one
ARGS
		build_exec_args
		printf 'URL=%s\nTOKEN=%s\nDOCKER=%s\nDOCKER_EXPLICIT=%s\nINSECURE=%s\nAGENT_ID=%s\nHOSTNAME=%s\nEXEC_ARGS=%s\n' \
			"$PULSE_URL" "$PULSE_TOKEN" "$ENABLE_DOCKER" "$DOCKER_EXPLICIT" "$INSECURE" "$AGENT_ID" "$HOSTNAME_OVERRIDE" "$EXEC_ARGS"
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	required := []string{
		"URL=http://192.168.2.96:7655",
		"TOKEN=deadbeef",
		"DOCKER=true",
		"DOCKER_EXPLICIT=true",
		"INSECURE=true",
		"AGENT_ID=agent-123",
		"HOSTNAME=pve-one",
		"--token-file /var/lib/pulse-agent/token",
		"--enable-docker",
	}
	for _, needle := range required {
		if !strings.Contains(got, needle) {
			t.Fatalf("recovered update state missing %q:\n%s", needle, got)
		}
	}
	if strings.Contains(got, "--token deadbeef") {
		t.Fatalf("recovered service args leaked raw token:\n%s", got)
	}
}

func TestInstallSHUsesCanonicalServiceLifecycleHelpers(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`stop_existing_agent_service() {`,
		`restart_systemd_agent_service() {`,
		`restart_openrc_agent_service() {`,
		`restart_service_command_agent() {`,
		`restart_sysv_agent_service() {`,
		`stop_existing_agent_service || true`,
		`restart_systemd_agent_service`,
		`restart_openrc_agent_service`,
		`restart_service_command_agent`,
		`restart_sysv_agent_service "$RCSCRIPT"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing canonical service lifecycle helper usage: %s", needle)
		}
	}
}

func TestInstallSHUsesCanonicalServiceTeardownHelpers(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`teardown_systemd_agent_service() {`,
		`teardown_openrc_agent_service() {`,
		`teardown_service_command_agent() {`,
		`teardown_sysv_agent_service() {`,
		`teardown_systemd_agent_service`,
		`teardown_service_command_agent "/usr/local/etc/rc.d/${AGENT_NAME}"`,
		`teardown_openrc_agent_service`,
		`teardown_sysv_agent_service "/etc/init.d/${AGENT_NAME}"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing canonical service teardown helper usage: %s", needle)
		}
	}
}

func TestWriteTrueNASBootstrapScriptUsesCanonicalRenderer(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`write_truenas_bootstrap_script() {`,
		`require_bootstrap_file() {`,
		`sync_runtime_binary() {`,
		`link_service_artifact() {`,
		`start_agent_service() {`,
		`ensure_freebsd_agent_enabled() {`,
		`service_link="/etc/systemd/system/${AGENT_NAME}.service"`,
		`service_link="/usr/local/etc/rc.d/${AGENT_NAME}"`,
		`write_truenas_bootstrap_script "$(uname -s)"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing canonical TrueNAS bootstrap renderer content: %s", needle)
		}
	}

	if strings.Count(script, `cat > "$TRUENAS_BOOTSTRAP_SCRIPT"`) != 1 {
		t.Fatalf("expected one canonical TrueNAS bootstrap writer, found %d", strings.Count(script, `cat > "$TRUENAS_BOOTSTRAP_SCRIPT"`))
	}
}

func TestInstallSHUsesCanonicalQNAPBootstrapRenderer(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`detect_qnap_data_volume() {`,
		`find_qnap_state_dir() {`,
		`remove_qnap_autorun_block() {`,
		`write_qnap_wrapper_script() {`,
		`append_qnap_autorun_block() {`,
		`STATE_DIR="${QNAP_VOL}/.pulse-agent"`,
		`write_qnap_wrapper_script "$WRAPPER_SCRIPT" "$RUNTIME_BINARY" "$QNAP_STORED_BINARY"`,
		`append_qnap_autorun_block "$AUTORUN_PATH" "$WRAPPER_SCRIPT" "$STATE_DIR"`,
		`complete_installation_flow "$STATE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent is running." "tail -f /var/log/${AGENT_NAME}.log"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing canonical QNAP bootstrap ownership: %s", needle)
		}
	}
}

func TestInstallSHUsesQNAPStateForUninstallRecovery(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`qnap_state_dir=$(find_qnap_state_dir || true)`,
		`aid_paths+=("$qnap_state_dir/agent-id")`,
		`if [[ -n "$qnap_state_dir" ]] && [[ -f "$qnap_state_dir/connection.env" ]]; then`,
		`remove_qnap_autorun_block "$AUTORUN_PATH"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing QNAP uninstall continuity handling: %s", needle)
		}
	}
}

func TestInstallSHUsesSharedServiceRenderers(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`render_systemd_agent_unit() {`,
		`render_freebsd_rc_agent_script() {`,
		`render_systemd_agent_unit "$UNIT" "${INSTALL_DIR}/${BINARY_NAME}" "${EXEC_ARGS}" "network.target" "" "" ""`,
		`render_systemd_agent_unit "$TRUENAS_SERVICE_STORAGE" "${TRUENAS_RUNTIME_BINARY}" "${EXEC_ARGS}" "network-online.target docker.service" "network-online.target" "root" "${TRUENAS_LOG_TARGET}"`,
		`render_systemd_agent_unit "$UNIT" "${INSTALL_DIR}/${BINARY_NAME}" "${EXEC_ARGS}" "network-online.target docker.service" "network-online.target" "root" ""`,
		`render_freebsd_rc_agent_script "$TRUENAS_SERVICE_STORAGE" "${TRUENAS_RUNTIME_BINARY}" "${EXEC_ARGS}"`,
		`render_freebsd_rc_agent_script "$RCSCRIPT" "${INSTALL_DIR}/${BINARY_NAME}" "${EXEC_ARGS}"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing shared service renderer usage: %s", needle)
		}
	}
}

func TestInstallSHPersistsRootlessContainerRuntimeServiceEnvironment(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`ROOTLESS_RUNTIME_SOCKET_URI=""`,
		`discover_rootless_container_runtime() {`,
		`discover_single_socket_match "/run/user/*/docker.sock"`,
		`discover_single_socket_match "/run/user/*/podman/podman.sock"`,
		`append_service_env "DOCKER_HOST" "$ROOTLESS_RUNTIME_SOCKET_URI"`,
		`append_service_env "PULSE_DOCKER_RUNTIME" "podman"`,
		`append_service_env "CONTAINER_HOST" "$ROOTLESS_RUNTIME_SOCKET_URI"`,
		`append_service_env "PODMAN_HOST" "$ROOTLESS_RUNTIME_SOCKET_URI"`,
		`append_service_env "XDG_RUNTIME_DIR" "$ROOTLESS_RUNTIME_XDG_DIR"`,
		`env_line="$SYSTEMD_ENV_LINES"`,
		`local service_env_lines="$SHELL_EXPORT_LINES"`,
		`</array>${PLIST_ENV_BLOCK}`,
		`respawn limit 5 10${UPSTART_ENV_LINES}`,
		`sed -i "s|SSL_CERT_FILE_PLACEHOLDER|${SED_EXPORT_LINES}|g" "$INITSCRIPT"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing rootless service environment persistence contract: %s", needle)
		}
	}
}

func TestInstallSHServiceEnvAccumulatorRendersRootlessSocketVariables(t *testing.T) {
	script := `
		APPLIED_SERVICE_ENV_KEYS="|"
		SYSTEMD_ENV_LINES=""
		SHELL_EXPORT_LINES=""
		UPSTART_ENV_LINES=""
		SED_EXPORT_LINES=""
		PLIST_ENV_ENTRIES=""
		PLIST_ENV_BLOCK=""
` + extractInstallShellFunction(t, "xml_escape") + `
` + extractInstallShellFunction(t, "service_env_has_key") + `
` + extractInstallShellFunction(t, "shell_export_value") + `
` + extractInstallShellFunction(t, "append_service_env") + `
` + extractInstallShellFunction(t, "finalize_plist_env_block") + `
		append_service_env "DOCKER_HOST" "unix:///run/user/1000/docker.sock"
		append_service_env "XDG_RUNTIME_DIR" "/run/user/1000"
		append_service_env "DOCKER_HOST" "unix:///run/user/2000/docker.sock"
		finalize_plist_env_block
		printf '%s\n---shell---\n%s\n---sed---\n%s\n---plist---\n%s\n' "$SYSTEMD_ENV_LINES" "$SHELL_EXPORT_LINES" "$SED_EXPORT_LINES" "$PLIST_ENV_BLOCK"
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	required := []string{
		`Environment="DOCKER_HOST=unix:///run/user/1000/docker.sock"`,
		`Environment="XDG_RUNTIME_DIR=/run/user/1000"`,
		`export DOCKER_HOST="unix:///run/user/1000/docker.sock"`,
		`export XDG_RUNTIME_DIR="/run/user/1000"`,
		`export DOCKER_HOST="unix:///run/user/1000/docker.sock"; export XDG_RUNTIME_DIR="/run/user/1000"`,
		`<key>DOCKER_HOST</key>`,
		`<string>unix:///run/user/1000/docker.sock</string>`,
	}
	for _, needle := range required {
		if !strings.Contains(got, needle) {
			t.Fatalf("service env output missing %s:\n%s", needle, got)
		}
	}
	if strings.Contains(got, "unix:///run/user/2000/docker.sock") {
		t.Fatalf("service env accumulator did not ignore duplicate key:\n%s", got)
	}
}

func TestInstallSHDiscoverSingleSocketMatch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("/tmp", "pulse-sock-*")
	if err != nil {
		t.Fatalf("mktemp socket dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	socketPath := filepath.Join(tmpDir, "docker.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	defer listener.Close()

	script := `
` + extractInstallShellFunction(t, "discover_single_socket_match") + `
		discover_single_socket_match "` + filepath.ToSlash(tmpDir) + `/*.sock"
	`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != filepath.ToSlash(socketPath) {
		t.Fatalf("socket match = %q, want %q", strings.TrimSpace(string(out)), filepath.ToSlash(socketPath))
	}
}

func TestInstallSHFreeBSDRendererUsesDaemonSupervisorPidfile(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`child_pidfile="/var/run/\${name}.child.pid"`,
		`pulse_agent_supervisor_pid()`,
		`parent_pid=\$(ps -o ppid= -p "\${agent_pid}" 2>/dev/null | tr -d '[:space:]')`,
		`daemon:*)`,
		`/usr/sbin/daemon -r -P \${pidfile} -p \${child_pidfile} -f "\${command}" \${command_args}`,
		`kill -KILL "\${supervisor_pid}" 2>/dev/null || true`,
		`rm -f \${pidfile} \${child_pidfile}`,
		`legacy child pid \${legacy_pid} supervised by pid \${legacy_supervisor}`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing FreeBSD daemon supervisor contract: %s", needle)
		}
	}

	if strings.Contains(script, `/usr/sbin/daemon -r -p \${pidfile}`) {
		t.Fatal("install.sh still writes the child pid to the service pidfile under daemon -r")
	}
}

func TestInstallSHUsesCanonicalCompletionHelper(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`complete_installation_flow() {`,
		`save_connection_info "$state_dir"`,
		`json_event "complete" "updated" "Installation updated"`,
		`json_event "complete" "installed" "Installation installed"`,
		`json_event "complete" "updated_unhealthy" "Agent updated but not responding"`,
		`json_event "complete" "installed_unhealthy" "Agent installed but not responding"`,
		`complete_installation_flow "$STATE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent restarted with new configuration." "tail -f $LOG_FILE"`,
		`complete_installation_flow "$UNRAID_STORAGE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent is running." "tail -f /var/log/${AGENT_NAME}.log"`,
		`complete_installation_flow "$STATE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent is running." "tail -f /var/log/${AGENT_NAME}.log"`,
		`complete_installation_flow "$TRUENAS_STATE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent is running." ""`,
		`complete_installation_flow "$STATE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent restarted with new configuration." "tail -f /var/log/messages"`,
		`complete_installation_flow "$STATE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent restarted with new configuration." "journalctl -u ${AGENT_NAME} --no-pager -n 20"`,
		`complete_installation_flow "$STATE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent restarted with new configuration." "tail -f /var/log/${AGENT_NAME}.log"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing canonical completion helper usage: %s", needle)
		}
	}
}

func TestInstallSHUsesCanonicalFreeBSDAgentEnablement(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`freebsd_enable_snippet() {`,
		`ensure_freebsd_agent_enabled() {`,
		`service_management_functions="$(freebsd_enable_snippet)`,
		`ensure_freebsd_agent_enabled`,
		`apply_freebsd_agent_enablement() {`,
		`eval "$(freebsd_enable_snippet)"`,
		`apply_freebsd_agent_enablement`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing canonical FreeBSD enablement ownership: %s", needle)
		}
	}

	if strings.Count(script, `grep -q "pulse_agent_enable" /etc/rc.conf`) != 1 {
		t.Fatalf("expected one canonical FreeBSD enablement definition, found %d", strings.Count(script, `grep -q "pulse_agent_enable" /etc/rc.conf`))
	}
}

func TestInstallSHUsesCanonicalSysVEnablementHelper(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`enable_sysv_agent_service() {`,
		`update-rc.d "${AGENT_NAME}" defaults >/dev/null 2>&1 || true`,
		`chkconfig --add "${AGENT_NAME}" >/dev/null 2>&1 || true`,
		`chkconfig "${AGENT_NAME}" on >/dev/null 2>&1 || true`,
		`enable_sysv_agent_service "$INITSCRIPT"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing canonical SysV enablement ownership: %s", needle)
		}
	}

	if strings.Count(script, `update-rc.d "${AGENT_NAME}" defaults >/dev/null 2>&1 || true`) != 1 {
		t.Fatalf("expected one canonical SysV update-rc.d enable path, found %d", strings.Count(script, `update-rc.d "${AGENT_NAME}" defaults >/dev/null 2>&1 || true`))
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
		_ = json.Unmarshal(body, &gotBody)
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
		CURL_ARGS=(-fsSL --connect-timeout 5 -X POST -H "Content-Type: application/json")
		if [[ -n "$PULSE_TOKEN" ]]; then CURL_ARGS+=(-H "X-API-Token: ${PULSE_TOKEN}"); fi
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

func TestAPIDeregistrationCurlWithoutToken(t *testing.T) {
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
		_ = json.Unmarshal(body, &gotBody)
		w.WriteHeader(200)
		w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	agentID := "test-uuid-5678"

	script := `
		PULSE_URL="` + srv.URL + `"
		PULSE_TOKEN=""
		AGENT_ID="` + agentID + `"
		CURL_ARGS=(-fsSL --connect-timeout 5 -X POST -H "Content-Type: application/json")
		if [[ -n "$PULSE_TOKEN" ]]; then CURL_ARGS+=(-H "X-API-Token: ${PULSE_TOKEN}"); fi
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
	if got := gotHeaders.Get("X-API-Token"); got != "" {
		t.Errorf("X-API-Token = %q, want empty", got)
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

func extractRootInstallShellFunction(t *testing.T, name string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read root install.sh: %v", err)
	}

	pattern := regexp.MustCompile(`(?ms)^` + regexp.QuoteMeta(name) + `\(\) \{\n.*?^\}`)
	match := pattern.Find(content)
	if match == nil {
		t.Fatalf("could not find %s in root install.sh", name)
	}
	return string(match)
}

func extractSelectedUpdateChannelShellFunctions(t *testing.T) string {
	t.Helper()

	return extractRootInstallShellFunction(t, "read_configured_update_channel") + "\n" +
		extractRootInstallShellFunction(t, "selected_update_channel")
}

func extractSetupAutoUpdatesShellFunctions(t *testing.T) string {
	t.Helper()

	return extractSelectedUpdateChannelShellFunctions(t) + "\n" +
		extractRootInstallShellFunction(t, "repo_web_url") + "\n" +
		extractRootInstallShellFunction(t, "configure_auto_update_script_repo") + "\n" +
		extractRootInstallShellFunction(t, "install_auto_update_assets") + "\n" +
		extractRootInstallShellFunction(t, "setup_auto_updates")
}

func prepareAutoUpdatePaths(t *testing.T, tmpDir string) (string, string, string) {
	t.Helper()

	autoUpdateDest := filepath.Join(tmpDir, "bin", "pulse-auto-update.sh")
	servicePath := filepath.Join(tmpDir, "systemd", "pulse-update.service")
	timerPath := filepath.Join(tmpDir, "systemd", "pulse-update.timer")

	if err := os.MkdirAll(filepath.Dir(autoUpdateDest), 0755); err != nil {
		t.Fatalf("mkdir auto-update dest dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(servicePath), 0755); err != nil {
		t.Fatalf("mkdir systemd dir: %v", err)
	}

	return autoUpdateDest, servicePath, timerPath
}

func extractAutoUpdateFunction(t *testing.T, name string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("..", "pulse-auto-update.sh"))
	if err != nil {
		t.Fatalf("read pulse-auto-update.sh: %v", err)
	}

	pattern := regexp.MustCompile(`(?ms)^` + regexp.QuoteMeta(name) + `\(\) \{\n.*?^\}`)
	match := pattern.Find(content)
	if match == nil {
		t.Fatalf("could not find %s in pulse-auto-update.sh", name)
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
` + extractInstallShellFunction(t, "build_exec_arg_items") + `
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
			RUNTIME_TOKEN_FILE="/var/lib/pulse-agent/token"
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
	if !strings.Contains(got, "--token-file /var/lib/pulse-agent/token") {
		t.Fatalf("EXEC_ARGS_ARRAY missing runtime token file: %s", got)
	}
	if strings.Contains(got, "--token deadbeef") {
		t.Fatalf("EXEC_ARGS_ARRAY preserved raw token: %s", got)
	}
}

func TestBuildExecArgsWithoutTokenOmitsPersistedToken(t *testing.T) {
	script := `
` + extractInstallShellFunction(t, "build_exec_arg_items") + `
` + extractInstallShellFunction(t, "join_exec_arg_items") + `
` + extractInstallShellFunction(t, "build_exec_args_without_token") + `
		PULSE_URL="https://pulse.example.com"
		PULSE_TOKEN="deadbeef"
		INTERVAL="30s"
		ENABLE_HOST="true"
		ENABLE_DOCKER=""
		DOCKER_EXPLICIT="false"
		ENABLE_KUBERNETES=""
		KUBECONFIG_PATH=""
		ENABLE_PROXMOX="true"
		PROXMOX_TYPE="pbs"
		INSECURE="false"
		ENABLE_COMMANDS=""
		ENROLL=""
		KUBE_INCLUDE_ALL_PODS=""
		KUBE_INCLUDE_ALL_DEPLOYMENTS=""
		AGENT_ID=""
		HOSTNAME_OVERRIDE=""
		STATE_DIR="/var/lib/pulse-agent"
		DISK_EXCLUDES=("/boot pool")
		build_exec_args_without_token
		printf '%s\n' "$EXEC_ARGS"
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if strings.Contains(got, "--token") {
		t.Fatalf("EXEC_ARGS unexpectedly preserved token: %s", got)
	}
	if !strings.Contains(got, "--proxmox-type pbs") {
		t.Fatalf("EXEC_ARGS missing proxmox type: %s", got)
	}
	if !strings.Contains(got, `--disk-exclude /boot\ pool`) {
		t.Fatalf("EXEC_ARGS missing quoted disk exclude: %s", got)
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

func TestSetupUpdateCommandHonorsRCChannelAndCustomPaths(t *testing.T) {
	tmpDir := t.TempDir()
	updatePath := filepath.Join(tmpDir, "update")
	profilePath := filepath.Join(tmpDir, "profile")
	bashrcPath := filepath.Join(tmpDir, "bashrc")
	configDir := filepath.Join(tmpDir, "pulse-config")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "system.json"), []byte(`{"updateChannel":"rc"}`), 0644); err != nil {
		t.Fatalf("write system.json: %v", err)
	}
	if err := os.WriteFile(bashrcPath, []byte(""), 0644); err != nil {
		t.Fatalf("write bashrc: %v", err)
	}

	script := `
		PULSE_UPDATE_HELPER_PATH="` + updatePath + `"
		PULSE_PROFILE_PATH="` + profilePath + `"
		PULSE_BASHRC_PATH="` + bashrcPath + `"
		GITHUB_REPO="example/pulse-fork"
` + extractRootInstallShellFunction(t, "setup_update_command") + `
		setup_update_command
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	content, err := os.ReadFile(updatePath)
	if err != nil {
		t.Fatalf("read update helper: %v", err)
	}
	got := string(content)
	if !strings.Contains(got, `CONFIG_DIR=/etc/pulse`) {
		t.Fatalf("update helper missing config dir logic:\n%s", got)
	}
	if !strings.Contains(got, `helper_args=()`) || !strings.Contains(got, `helper_args=("$@")`) {
		t.Fatalf("update helper missing passthrough helper args:\n%s", got)
	}
	if !strings.Contains(got, `-h|--help|--uninstall|--version|--rc|--pre|--stable|--source|--from-source|--branch|--archive|--archive=*|--skip-upgrade-preflight)`) {
		t.Fatalf("update helper missing auto-selector guard for explicit flags:\n%s", got)
	}
	if !strings.Contains(got, `extra_args+=("${helper_args[@]}")`) {
		t.Fatalf("update helper missing forwarded helper args:\n%s", got)
	}
	if !strings.Contains(got, `extra_args+=(--rc)`) {
		t.Fatalf("update helper missing rc channel forwarding:\n%s", got)
	}
	if !strings.Contains(got, `INSTALLER_URL="https://github.com/example/pulse-fork/releases/latest/download/install.sh"`) {
		t.Fatalf("update helper missing configured repo installer url:\n%s", got)
	}
	if !strings.Contains(got, `INSTALLER_SIG_URL="${INSTALLER_URL}.sshsig"`) {
		t.Fatalf("update helper missing installer signature url:\n%s", got)
	}
	if !strings.Contains(got, `verify_release_signature "$tmp_installer" "$tmp_signature" "downloaded Pulse installer"`) {
		t.Fatalf("update helper missing signed installer verification:\n%s", got)
	}
	if strings.Contains(got, `curl -fsSL "$INSTALLER_URL" |`) {
		t.Fatalf("update helper still pipes installer directly to bash:\n%s", got)
	}

	profileContent, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}
	if !strings.Contains(string(profileContent), `/usr/local/bin`) {
		t.Fatalf("profile not updated with /usr/local/bin path:\n%s", profileContent)
	}
}

func TestSetupUpdateCommandUsesConfiguredInstallerRepo(t *testing.T) {
	tmpDir := t.TempDir()
	updatePath := filepath.Join(tmpDir, "update")
	profilePath := filepath.Join(tmpDir, "profile")
	bashrcPath := filepath.Join(tmpDir, "bashrc")

	if err := os.WriteFile(bashrcPath, []byte(""), 0644); err != nil {
		t.Fatalf("write bashrc: %v", err)
	}

	script := `
		PULSE_UPDATE_HELPER_PATH="` + updatePath + `"
		PULSE_PROFILE_PATH="` + profilePath + `"
		PULSE_BASHRC_PATH="` + bashrcPath + `"
		GITHUB_REPO="example/pulse-fork"
` + extractRootInstallShellFunction(t, "setup_update_command") + `
		setup_update_command
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	content, err := os.ReadFile(updatePath)
	if err != nil {
		t.Fatalf("read update helper: %v", err)
	}
	got := string(content)
	if strings.Contains(got, "https://github.com/rcourtman/Pulse/releases/latest/download/install.sh") {
		t.Fatalf("update helper still hardcodes upstream repo:\n%s", got)
	}
	if !strings.Contains(got, `INSTALLER_URL="https://github.com/example/pulse-fork/releases/latest/download/install.sh"`) {
		t.Fatalf("update helper missing configured installer repo:\n%s", got)
	}
	if !strings.Contains(got, `INSTALLER_SIG_URL="${INSTALLER_URL}.sshsig"`) {
		t.Fatalf("update helper missing signature sidecar url:\n%s", got)
	}
}

func TestSetupUpdateCommandFailsWhenInstallerDownloadFails(t *testing.T) {
	tmpDir := t.TempDir()
	updatePath := filepath.Join(tmpDir, "update")
	profilePath := filepath.Join(tmpDir, "profile")
	bashrcPath := filepath.Join(tmpDir, "bashrc")
	curlPath := filepath.Join(tmpDir, "curl")
	fakeBashPath := filepath.Join(tmpDir, "bash")

	if err := os.WriteFile(bashrcPath, []byte(""), 0644); err != nil {
		t.Fatalf("write bashrc: %v", err)
	}
	if err := os.WriteFile(curlPath, []byte("#!/usr/bin/env bash\nexit 22\n"), 0755); err != nil {
		t.Fatalf("write curl stub: %v", err)
	}
	if err := os.WriteFile(fakeBashPath, []byte("#!/usr/bin/env bash\ncat >/dev/null\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write bash stub: %v", err)
	}

	script := `
		PULSE_UPDATE_HELPER_PATH="` + updatePath + `"
		PULSE_PROFILE_PATH="` + profilePath + `"
		PULSE_BASHRC_PATH="` + bashrcPath + `"
` + extractRootInstallShellFunction(t, "setup_update_command") + `
		setup_update_command
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	cmd := exec.Command("bash", updatePath)
	cmd.Env = append(os.Environ(), "PATH="+tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected generated update helper to fail when curl fails:\n%s", out)
	}
	if !strings.Contains(string(out), "Updating Pulse...") {
		t.Fatalf("expected helper output before failure, got:\n%s", out)
	}
}

func TestResolveInstallScriptDownloadURLUsesForcedVersion(t *testing.T) {
	script := `
		GITHUB_REPO="rcourtman/Pulse"
		FORCE_VERSION="v1.2.3"
		FORCE_CHANNEL=""
		UPDATE_CHANNEL=""
` + extractRootInstallShellFunction(t, "resolve_install_script_download_url") + `
		resolve_install_script_download_url
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	got := strings.TrimSpace(string(out))
	want := "https://github.com/rcourtman/Pulse/releases/download/v1.2.3/install.sh"
	if got != want {
		t.Fatalf("download url = %q, want %q", got, want)
	}
}

func TestRootInstallStableReleaseTagRejectsPrereleaseShapes(t *testing.T) {
	script := `
` + extractRootInstallShellFunction(t, "is_stable_release_tag") + `
		if ! is_stable_release_tag v5.1.29; then
			echo "rejected v-prefixed stable tag" >&2
			exit 1
		fi
		if ! is_stable_release_tag 5.1.29; then
			echo "rejected bare stable tag" >&2
			exit 1
		fi
		if is_stable_release_tag v6.0.0-rc.2; then
			echo "accepted rc tag" >&2
			exit 1
		fi
		if is_stable_release_tag v6.0.0-beta.1; then
			echo "accepted beta tag" >&2
			exit 1
		fi
		if is_stable_release_tag latest; then
			echo "accepted floating tag" >&2
			exit 1
		fi
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
}

func TestResolveLatestReleaseTagForStableChannelSkipsPrereleaseShapedTags(t *testing.T) {
	tmpDir := t.TempDir()
	timeoutPath := filepath.Join(tmpDir, "timeout")
	curlPath := filepath.Join(tmpDir, "curl")

	if err := os.WriteFile(timeoutPath, []byte("#!/usr/bin/env bash\nshift\nexec \"$@\"\n"), 0755); err != nil {
		t.Fatalf("write timeout stub: %v", err)
	}

	curlStub := `#!/usr/bin/env bash
for arg in "$@"; do
	if [[ "$arg" == "https://api.github.com/repos/rcourtman/Pulse/releases" ]]; then
		printf '%s\n' '[{"draft":false,"prerelease":false,"tag_name":"v6.0.0-rc.2"},{"draft":false,"prerelease":false,"tag_name":"v5.1.29"}]'
		exit 0
	fi
done
echo "unexpected curl invocation: $*" >&2
exit 1
`
	if err := os.WriteFile(curlPath, []byte(curlStub), 0755); err != nil {
		t.Fatalf("write curl stub: %v", err)
	}

	script := `
		PATH="` + tmpDir + `:$PATH"
		GITHUB_REPO="rcourtman/Pulse"
		get_latest_release_from_redirect() {
			printf '%s\n' v6.0.0-rc.2
		}
` + extractRootInstallShellFunction(t, "is_stable_release_tag") + `
` + extractRootInstallShellFunction(t, "latest_stable_release_tag_from_json") + `
` + extractRootInstallShellFunction(t, "resolve_latest_release_tag_for_channel") + `
		resolve_latest_release_tag_for_channel stable
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	if got := strings.TrimSpace(string(out)); got != "v5.1.29" {
		t.Fatalf("stable release = %q, want v5.1.29", got)
	}
}

func TestResolveLatestReleaseTagForStableChannelRejectsPrereleaseRedirect(t *testing.T) {
	script := `
		GITHUB_REPO="rcourtman/Pulse"
		curl() { return 1; }
		timeout() { shift; "$@"; }
		get_latest_release_from_redirect() {
			printf '%s\n' v6.0.0-rc.2
		}
` + extractRootInstallShellFunction(t, "is_stable_release_tag") + `
` + extractRootInstallShellFunction(t, "latest_stable_release_tag_from_json") + `
` + extractRootInstallShellFunction(t, "resolve_latest_release_tag_for_channel") + `
		if resolve_latest_release_tag_for_channel stable; then
			echo "stable channel accepted prerelease redirect" >&2
			exit 1
		fi
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
}

func TestResolveInstallScriptDownloadURLUsesStableReleaseTag(t *testing.T) {
	tmpDir := t.TempDir()
	timeoutPath := filepath.Join(tmpDir, "timeout")
	curlPath := filepath.Join(tmpDir, "curl")

	if err := os.WriteFile(timeoutPath, []byte("#!/usr/bin/env bash\nshift\nexec \"$@\"\n"), 0755); err != nil {
		t.Fatalf("write timeout stub: %v", err)
	}

	curlStub := `#!/usr/bin/env bash
for arg in "$@"; do
	if [[ "$arg" == "https://api.github.com/repos/rcourtman/Pulse/releases" ]]; then
		printf '%s\n' '[{"draft":false,"prerelease":true,"tag_name":"v6.0.0-rc.2"},{"draft":false,"prerelease":false,"tag_name":"v5.1.29"}]'
		exit 0
	fi
done
echo "unexpected curl invocation: $*" >&2
exit 1
`
	if err := os.WriteFile(curlPath, []byte(curlStub), 0755); err != nil {
		t.Fatalf("write curl stub: %v", err)
	}

	script := `
		PATH="` + tmpDir + `:$PATH"
		GITHUB_REPO="rcourtman/Pulse"
		FORCE_VERSION=""
		FORCE_CHANNEL=""
		UPDATE_CHANNEL=""
		get_latest_release_from_redirect() {
			printf '%s\n' v6.0.0-rc.2
		}
` + extractRootInstallShellFunction(t, "is_stable_release_tag") + `
` + extractRootInstallShellFunction(t, "latest_stable_release_tag_from_json") + `
` + extractRootInstallShellFunction(t, "resolve_latest_release_tag_for_channel") + `
` + extractRootInstallShellFunction(t, "resolve_install_script_download_url") + `
		resolve_install_script_download_url
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	got := strings.TrimSpace(string(out))
	want := "https://github.com/rcourtman/Pulse/releases/download/v5.1.29/install.sh"
	if got != want {
		t.Fatalf("download url = %q, want %q", got, want)
	}
}

func TestResolveInstallScriptDownloadURLUsesRCReleaseTag(t *testing.T) {
	tmpDir := t.TempDir()
	timeoutPath := filepath.Join(tmpDir, "timeout")
	curlPath := filepath.Join(tmpDir, "curl")

	if err := os.WriteFile(timeoutPath, []byte("#!/usr/bin/env bash\nshift\nexec \"$@\"\n"), 0755); err != nil {
		t.Fatalf("write timeout stub: %v", err)
	}

	curlStub := `#!/usr/bin/env bash
for arg in "$@"; do
	if [[ "$arg" == "https://api.github.com/repos/rcourtman/Pulse/releases" ]]; then
		printf '%s\n' '[{"draft":false,"tag_name":"v6.0.0-rc.2"},{"draft":false,"prerelease":false,"tag_name":"v5.9.0"}]'
		exit 0
	fi
done
echo "unexpected curl invocation: $*" >&2
exit 1
`
	if err := os.WriteFile(curlPath, []byte(curlStub), 0755); err != nil {
		t.Fatalf("write curl stub: %v", err)
	}

	script := `
		PATH="` + tmpDir + `:$PATH"
		GITHUB_REPO="rcourtman/Pulse"
		FORCE_VERSION=""
		FORCE_CHANNEL="rc"
		UPDATE_CHANNEL=""
` + extractRootInstallShellFunction(t, "resolve_latest_release_tag_for_channel") + `
` + extractRootInstallShellFunction(t, "resolve_install_script_download_url") + `
		resolve_install_script_download_url
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	got := strings.TrimSpace(string(out))
	want := "https://github.com/rcourtman/Pulse/releases/download/v6.0.0-rc.2/install.sh"
	if got != want {
		t.Fatalf("download url = %q, want %q", got, want)
	}
}

func TestInstallAdditionalAgentBinariesCopiesLocalExtrasWithoutNetwork(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	installDir := filepath.Join(tmpDir, "opt", "pulse")
	if err := os.MkdirAll(filepath.Join(sourceDir, "bin"), 0755); err != nil {
		t.Fatalf("mkdir source bin: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(installDir, "bin"), 0755); err != nil {
		t.Fatalf("mkdir install bin: %v", err)
	}
	agentPath := filepath.Join(sourceDir, "bin", "pulse-agent-linux-arm64")
	if err := os.WriteFile(agentPath, []byte("unified-agent\n"), 0755); err != nil {
		t.Fatalf("write agent: %v", err)
	}

	script := `
		INSTALL_DIR="` + installDir + `"
		GITHUB_REPO="rcourtman/Pulse"
		curl_calls=0
		wget_calls=0
		chown() { :; }
		print_info() { :; }
		print_warn() { :; }
		print_success() { :; }
		curl() { curl_calls=$((curl_calls + 1)); return 99; }
		wget() { wget_calls=$((wget_calls + 1)); return 99; }
` + extractRootInstallShellFunction(t, "copy_unified_agent_binaries_from_dir") + `
` + extractRootInstallShellFunction(t, "install_additional_agent_binaries") + `
		install_additional_agent_binaries "v6.0.0-rc.3" "` + sourceDir + `"
		printf 'curl=%s wget=%s\n' "$curl_calls" "$wget_calls"
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "curl=0 wget=0" {
		t.Fatalf("expected no network fallback calls, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(installDir, "bin", "pulse-agent-linux-arm64")); err != nil {
		t.Fatalf("expected local unified agent binary to be copied: %v", err)
	}
}

func TestInstallAdditionalAgentBinariesSkipsNetworkWhenLocalExtrasAreMissing(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	installDir := filepath.Join(tmpDir, "opt", "pulse")
	if err := os.MkdirAll(filepath.Join(sourceDir, "bin"), 0755); err != nil {
		t.Fatalf("mkdir source bin: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(installDir, "bin"), 0755); err != nil {
		t.Fatalf("mkdir install bin: %v", err)
	}

	script := `
		INSTALL_DIR="` + installDir + `"
		GITHUB_REPO="rcourtman/Pulse"
		curl_calls=0
		wget_calls=0
		chown() { :; }
		print_info() { :; }
		print_warn() { :; }
		print_success() { :; }
		curl() { curl_calls=$((curl_calls + 1)); return 99; }
		wget() { wget_calls=$((wget_calls + 1)); return 99; }
` + extractRootInstallShellFunction(t, "copy_unified_agent_binaries_from_dir") + `
` + extractRootInstallShellFunction(t, "install_additional_agent_binaries") + `
		install_additional_agent_binaries "v6.0.0-rc.3" "` + sourceDir + `"
		printf 'curl=%s wget=%s\n' "$curl_calls" "$wget_calls"
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "curl=0 wget=0" {
		t.Fatalf("expected missing local extras to skip network fallback, got %q", got)
	}
}

func TestInstallSHRequiresPinnedSignatureVerificationForReleaseDownloads(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`PINNED_INSTALLER_SSH_PUBLIC_KEY="__PULSE_INSTALLER_SSH_PUBLIC_KEY__"`,
		`has_pinned_installer_signature_key() {`,
		`grep -i '^X-Signature-SSHSIG:' "$TMP_HEADERS"`,
		`Server did not provide checksum header; refusing install.`,
		`Server did not provide SSH signature header; refusing signed install.`,
		`ssh-keygen -Y verify`,
		`Binary signature verified`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing signed-download verification contract: %s", needle)
		}
	}
}

func TestBuildContainerInstallCommandPreservesForcedVersion(t *testing.T) {
	script := `
		FORCE_VERSION="v1.2.3"
		FORCE_CHANNEL=""
		UPDATE_CHANNEL=""
		auto_updates_flag="--enable-auto-updates"
		BUILD_FROM_SOURCE="false"
		SOURCE_BRANCH="main"
		frontend_port="7655"
		CONFIG_DIR="` + t.TempDir() + `"
` + extractSelectedUpdateChannelShellFunctions(t) + `
` + extractRootInstallShellFunction(t, "build_container_install_command") + `
		build_container_install_command
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	got := strings.TrimSpace(string(out))
	want := "bash /tmp/install.sh --in-container --version 'v1.2.3' --enable-auto-updates"
	if got != want {
		t.Fatalf("install cmd = %q, want %q", got, want)
	}
}

func TestBuildContainerInstallCommandPreservesExplicitAutoUpdateDisable(t *testing.T) {
	script := `
		FORCE_VERSION="v1.2.3"
		FORCE_CHANNEL=""
		UPDATE_CHANNEL=""
		auto_updates_flag="--disable-auto-updates"
		BUILD_FROM_SOURCE="false"
		SOURCE_BRANCH="main"
		frontend_port="7655"
		CONFIG_DIR="` + t.TempDir() + `"
` + extractSelectedUpdateChannelShellFunctions(t) + `
` + extractRootInstallShellFunction(t, "build_container_install_command") + `
		build_container_install_command
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	got := strings.TrimSpace(string(out))
	want := "bash /tmp/install.sh --in-container --version 'v1.2.3' --disable-auto-updates"
	if got != want {
		t.Fatalf("install cmd = %q, want %q", got, want)
	}
}

func TestBuildContainerInstallCommandPassesArchiveToContainer(t *testing.T) {
	script := `
		FORCE_VERSION=""
		FORCE_CHANNEL=""
		UPDATE_CHANNEL=""
		auto_updates_flag=""
		BUILD_FROM_SOURCE="false"
		SOURCE_BRANCH="main"
		frontend_port="7655"
		container_archive_dest="/tmp/pulse-v6.0.0-rc.1-linux-amd64.tar.gz"
		CONFIG_DIR="` + t.TempDir() + `"
` + extractSelectedUpdateChannelShellFunctions(t) + `
` + extractRootInstallShellFunction(t, "build_container_install_command") + `
		build_container_install_command
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	got := strings.TrimSpace(string(out))
	want := "bash /tmp/install.sh --in-container --archive /tmp/pulse-v6.0.0-rc.1-linux-amd64.tar.gz"
	if got != want {
		t.Fatalf("install cmd = %q, want %q", got, want)
	}
}

func TestBuildContainerInstallCommandQuotesArchivePath(t *testing.T) {
	script := `
		FORCE_VERSION=""
		FORCE_CHANNEL=""
		UPDATE_CHANNEL=""
		auto_updates_flag=""
		BUILD_FROM_SOURCE="false"
		SOURCE_BRANCH="main"
		frontend_port="7655"
		container_archive_dest="/tmp/pulse archive-linux-amd64.tar.gz"
		CONFIG_DIR="` + t.TempDir() + `"
` + extractSelectedUpdateChannelShellFunctions(t) + `
` + extractRootInstallShellFunction(t, "build_container_install_command") + `
		build_container_install_command
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	got := strings.TrimSpace(string(out))
	want := `bash /tmp/install.sh --in-container --archive /tmp/pulse\ archive-linux-amd64.tar.gz`
	if got != want {
		t.Fatalf("install cmd = %q, want %q", got, want)
	}
}

func TestBuildContainerInstallCommandPreservesRCChannel(t *testing.T) {
	script := `
		FORCE_VERSION=""
		FORCE_CHANNEL="rc"
		UPDATE_CHANNEL=""
		auto_updates_flag=""
		BUILD_FROM_SOURCE="false"
		SOURCE_BRANCH="main"
		frontend_port="7766"
		CONFIG_DIR="` + t.TempDir() + `"
` + extractSelectedUpdateChannelShellFunctions(t) + `
` + extractRootInstallShellFunction(t, "build_container_install_command") + `
		build_container_install_command
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	got := strings.TrimSpace(string(out))
	want := "FRONTEND_PORT=7766 bash /tmp/install.sh --in-container --rc"
	if got != want {
		t.Fatalf("install cmd = %q, want %q", got, want)
	}
}

func TestBuildContainerInstallCommandIgnoresHostConfiguredRCChannelForFreshLXCInstall(t *testing.T) {
	configDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDir, "system.json"), []byte(`{"updateChannel":"rc"}`), 0644); err != nil {
		t.Fatalf("write system.json: %v", err)
	}

	script := `
		FORCE_VERSION=""
		FORCE_CHANNEL=""
		UPDATE_CHANNEL=""
		IGNORE_CONFIGURED_UPDATE_CHANNEL="true"
		auto_updates_flag=""
		BUILD_FROM_SOURCE="false"
		SOURCE_BRANCH="main"
		frontend_port="7655"
		CONFIG_DIR="` + configDir + `"
` + extractSelectedUpdateChannelShellFunctions(t) + `
` + extractRootInstallShellFunction(t, "build_container_install_command") + `
		build_container_install_command
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	got := strings.TrimSpace(string(out))
	want := "bash /tmp/install.sh --in-container"
	if got != want {
		t.Fatalf("install cmd = %q, want %q", got, want)
	}
}

func TestPrintContainerRecoveryCommandPreservesForcedVersion(t *testing.T) {
	script := `
		FORCE_VERSION="v1.2.3"
		FORCE_CHANNEL=""
		UPDATE_CHANNEL=""
		auto_updates_flag="--enable-auto-updates"
		BUILD_FROM_SOURCE="false"
		SOURCE_BRANCH="main"
		frontend_port="7655"
		CONFIG_DIR="` + t.TempDir() + `"
		print_info() { printf '%s\n' "$1"; }
` + extractSelectedUpdateChannelShellFunctions(t) + `
` + extractRootInstallShellFunction(t, "build_container_install_command") + `
` + extractRootInstallShellFunction(t, "print_container_recovery_command") + `
		print_container_recovery_command
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	got := strings.TrimSpace(string(out))
	want := "bash /tmp/install.sh --in-container --version 'v1.2.3' --enable-auto-updates"
	if got != want {
		t.Fatalf("recovery command = %q, want %q", got, want)
	}
}

func TestPrintContainerRecoveryCommandPreservesExplicitAutoUpdateDisable(t *testing.T) {
	script := `
		FORCE_VERSION="v1.2.3"
		FORCE_CHANNEL=""
		UPDATE_CHANNEL=""
		auto_updates_flag="--disable-auto-updates"
		BUILD_FROM_SOURCE="false"
		SOURCE_BRANCH="main"
		frontend_port="7655"
		CONFIG_DIR="` + t.TempDir() + `"
		print_info() { printf '%s\n' "$1"; }
` + extractSelectedUpdateChannelShellFunctions(t) + `
` + extractRootInstallShellFunction(t, "build_container_install_command") + `
` + extractRootInstallShellFunction(t, "print_container_recovery_command") + `
		print_container_recovery_command
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	got := strings.TrimSpace(string(out))
	want := "bash /tmp/install.sh --in-container --version 'v1.2.3' --disable-auto-updates"
	if got != want {
		t.Fatalf("recovery command = %q, want %q", got, want)
	}
}

func TestPrintContainerRecoveryCommandPreservesRCChannel(t *testing.T) {
	script := `
		FORCE_VERSION=""
		FORCE_CHANNEL="rc"
		UPDATE_CHANNEL=""
		auto_updates_flag=""
		BUILD_FROM_SOURCE="false"
		SOURCE_BRANCH="main"
		frontend_port="7766"
		CONFIG_DIR="` + t.TempDir() + `"
		print_info() { printf '%s\n' "$1"; }
` + extractSelectedUpdateChannelShellFunctions(t) + `
` + extractRootInstallShellFunction(t, "build_container_install_command") + `
` + extractRootInstallShellFunction(t, "print_container_recovery_command") + `
		print_container_recovery_command
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	got := strings.TrimSpace(string(out))
	want := "FRONTEND_PORT=7766 bash /tmp/install.sh --in-container --rc"
	if got != want {
		t.Fatalf("recovery command = %q, want %q", got, want)
	}
}

func TestResolveReleaseAssetBaseURLUsesLatestRelease(t *testing.T) {
	script := `
		GITHUB_REPO="rcourtman/Pulse"
		LATEST_RELEASE="v1.2.3"
` + extractRootInstallShellFunction(t, "resolve_release_asset_base_url") + `
		resolve_release_asset_base_url
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	got := strings.TrimSpace(string(out))
	want := "https://github.com/rcourtman/Pulse/releases/download/v1.2.3"
	if got != want {
		t.Fatalf("asset base url = %q, want %q", got, want)
	}
}

func TestDownloadAutoUpdateScriptUsesSelectedReleaseAssets(t *testing.T) {
	tmpDir := t.TempDir()
	curlPath := filepath.Join(tmpDir, "curl")
	sshKeygenPath := filepath.Join(tmpDir, "ssh-keygen")
	destPath := filepath.Join(tmpDir, "pulse-auto-update.sh")
	logPath := filepath.Join(tmpDir, "curl.log")

	curlStub := `#!/usr/bin/env bash
set -e
out=""
url=""
while [[ $# -gt 0 ]]; do
	case "$1" in
		-o)
			out="$2"
			shift 2
			;;
		--connect-timeout|--max-time)
			shift 2
			;;
		-fsSL|-fsS|-fsSL)
			shift
			;;
		*)
			url="$1"
			shift
			;;
	esac
done
printf '%s\n' "$url" >> "` + logPath + `"
case "$url" in
	"https://github.com/rcourtman/Pulse/releases/download/v9.9.9/pulse-auto-update.sh")
		printf '#!/usr/bin/env bash\nexit 0\n' > "$out"
		;;
	"https://github.com/rcourtman/Pulse/releases/download/v9.9.9/pulse-auto-update.sh.sshsig")
		printf 'signed-payload\n' > "$out"
		;;
	*)
		echo "unexpected url: $url" >&2
		exit 1
		;;
esac
`
	if err := os.WriteFile(curlPath, []byte(curlStub), 0755); err != nil {
		t.Fatalf("write curl stub: %v", err)
	}

	sshKeygenStub := `#!/usr/bin/env bash
exit 0
`
	if err := os.WriteFile(sshKeygenPath, []byte(sshKeygenStub), 0755); err != nil {
		t.Fatalf("write ssh-keygen stub: %v", err)
	}

	script := `
		PATH="` + tmpDir + `:$PATH"
		GITHUB_REPO="rcourtman/Pulse"
		LATEST_RELEASE="v9.9.9"
		PULSE_AUTO_UPDATE_DEST="` + destPath + `"
		print_warn() { :; }
		print_info() { :; }
		INSTALL_SIGNATURE_IDENTITY="pulse-installer"
		INSTALL_SIGNATURE_NAMESPACE="pulse-install"
		PINNED_RELEASE_SSH_PUBLIC_KEY="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer"
` + extractRootInstallShellFunction(t, "release_signature_key_available") + `
` + extractRootInstallShellFunction(t, "require_release_signature_verifier") + `
` + extractRootInstallShellFunction(t, "verify_release_signature") + `
` + extractRootInstallShellFunction(t, "resolve_release_asset_base_url") + `
` + extractRootInstallShellFunction(t, "download_auto_update_script") + `
		download_auto_update_script
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	logContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read curl log: %v", err)
	}
	got := string(logContent)
	if !strings.Contains(got, "https://github.com/rcourtman/Pulse/releases/download/v9.9.9/pulse-auto-update.sh") {
		t.Fatalf("missing versioned helper download:\n%s", got)
	}
	if !strings.Contains(got, "https://github.com/rcourtman/Pulse/releases/download/v9.9.9/pulse-auto-update.sh.sshsig") {
		t.Fatalf("missing versioned signature download:\n%s", got)
	}
}

func TestPulseAutoUpdatePerformUpdateUsesVersionedInstallerURL(t *testing.T) {
	tmpDir := t.TempDir()
	curlPath := filepath.Join(tmpDir, "curl")
	sshKeygenPath := filepath.Join(tmpDir, "ssh-keygen")
	logPath := filepath.Join(tmpDir, "curl.log")
	installDir := filepath.Join(tmpDir, "install")

	if err := os.MkdirAll(filepath.Join(installDir, "bin"), 0755); err != nil {
		t.Fatalf("mkdir install bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "bin", "pulse"), []byte("old"), 0755); err != nil {
		t.Fatalf("write fake pulse binary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "VERSION"), []byte("v1.0.0\n"), 0644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}

	curlStub := `#!/usr/bin/env bash
set -e
printf '%s\n' "$*" >> "` + logPath + `"
out=""
url=""
while [[ $# -gt 0 ]]; do
	case "$1" in
		-o)
			out="$2"
			shift 2
			;;
		-fsSL)
			shift
			;;
		*)
			url="$1"
			shift
			;;
	esac
done
case "$url" in
	"https://github.com/rcourtman/Pulse/releases/download/v9.9.9/install.sh")
		printf '#!/usr/bin/env bash\nexit 0\n' > "$out"
		;;
	"https://github.com/rcourtman/Pulse/releases/download/v9.9.9/install.sh.sshsig")
		printf 'signed-payload\n' > "$out"
		;;
esac
`
	if err := os.WriteFile(curlPath, []byte(curlStub), 0755); err != nil {
		t.Fatalf("write curl stub: %v", err)
	}
	if err := os.WriteFile(sshKeygenPath, []byte("#!/usr/bin/env bash\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write ssh-keygen stub: %v", err)
	}

	script := `
		PATH="` + tmpDir + `:$PATH"
		GITHUB_REPO="rcourtman/Pulse"
		INSTALL_DIR="` + installDir + `"
		log() { :; }
		detect_service_name() { echo pulse; }
		get_current_version() { echo v9.9.9; }
		systemctl() { return 0; }
		INSTALL_SIGNATURE_IDENTITY="pulse-installer"
		INSTALL_SIGNATURE_NAMESPACE="pulse-install"
		PINNED_RELEASE_SSH_PUBLIC_KEY="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer"
` + extractAutoUpdateFunction(t, "release_signature_key_available") + `
` + extractAutoUpdateFunction(t, "require_release_signature_verifier") + `
` + extractAutoUpdateFunction(t, "verify_release_signature") + `
` + extractAutoUpdateFunction(t, "resolve_install_script_url") + `
` + extractAutoUpdateFunction(t, "is_prerelease_tag") + `
		wait_for_service_active() { return 0; }
` + extractAutoUpdateFunction(t, "perform_update") + `
		perform_update v9.9.9
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	logContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read curl log: %v", err)
	}
	got := string(logContent)
	if !strings.Contains(got, "https://github.com/rcourtman/Pulse/releases/download/v9.9.9/install.sh") {
		t.Fatalf("perform_update did not use versioned installer url:\n%s", got)
	}
	if !strings.Contains(got, "https://github.com/rcourtman/Pulse/releases/download/v9.9.9/install.sh.sshsig") {
		t.Fatalf("perform_update did not use versioned installer signature url:\n%s", got)
	}
	if strings.Contains(got, "releases/latest/download/install.sh") {
		t.Fatalf("perform_update still used latest installer url:\n%s", got)
	}
}

func TestPulseAutoUpdateResolveInstallScriptURLUsesConfiguredRepo(t *testing.T) {
	script := `
		GITHUB_REPO="example/pulse-fork"
` + extractAutoUpdateFunction(t, "resolve_install_script_url") + `
		resolve_install_script_url v9.9.9
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "https://github.com/example/pulse-fork/releases/download/v9.9.9/install.sh" {
		t.Fatalf("resolve_install_script_url = %q", got)
	}
}

func TestRepoDockerDocsURLUsesConfiguredRepo(t *testing.T) {
	script := `
		GITHUB_REPO="example/pulse-fork"
		LATEST_RELEASE="v9.9.9"
` + extractRootInstallShellFunction(t, "repo_web_url") + `
` + extractRootInstallShellFunction(t, "resolve_latest_release_tag_for_channel") + `
` + extractRootInstallShellFunction(t, "repo_release_docs_ref") + `
` + extractRootInstallShellFunction(t, "repo_docs_url_for_path") + `
` + extractRootInstallShellFunction(t, "repo_docker_docs_url") + `
		repo_docker_docs_url
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "https://github.com/example/pulse-fork/blob/v9.9.9/docs/DOCKER.md" {
		t.Fatalf("repo_docker_docs_url = %q", got)
	}
}

func TestRepoDockerDocsURLFallsBackToReleaseLandingPageWhenVersionUnknown(t *testing.T) {
	script := `
		GITHUB_REPO="example/pulse-fork"
		get_latest_release_from_redirect() { return 1; }
		curl() { return 1; }
		timeout() { return 1; }
` + extractRootInstallShellFunction(t, "repo_web_url") + `
` + extractRootInstallShellFunction(t, "is_stable_release_tag") + `
` + extractRootInstallShellFunction(t, "latest_stable_release_tag_from_json") + `
` + extractRootInstallShellFunction(t, "resolve_latest_release_tag_for_channel") + `
` + extractRootInstallShellFunction(t, "repo_release_docs_ref") + `
` + extractRootInstallShellFunction(t, "repo_docs_url_for_path") + `
` + extractRootInstallShellFunction(t, "repo_docker_docs_url") + `
		repo_docker_docs_url
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "https://github.com/example/pulse-fork/releases/latest" {
		t.Fatalf("repo_docker_docs_url fallback = %q", got)
	}
}

func TestRepoDockerImageRefUsesConfiguredImageRepo(t *testing.T) {
	script := `
		DOCKER_IMAGE_REPO="example/pulse-enterprise"
` + extractRootInstallShellFunction(t, "repo_docker_image_ref") + `
		repo_docker_image_ref latest
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "example/pulse-enterprise:latest" {
		t.Fatalf("repo_docker_image_ref = %q", got)
	}
}

func TestCheckDockerEnvironmentUsesConfiguredImageAndDocs(t *testing.T) {
	script := `
		GITHUB_REPO="example/pulse-fork"
		LATEST_RELEASE="v9.9.9"
		DOCKER_IMAGE_REPO="example/pulse-enterprise"
		print_error() { printf 'ERR:%s\n' "$1"; }
		grep() { return 1; }
` + extractRootInstallShellFunction(t, "repo_web_url") + `
` + extractRootInstallShellFunction(t, "resolve_latest_release_tag_for_channel") + `
` + extractRootInstallShellFunction(t, "repo_release_docs_ref") + `
` + extractRootInstallShellFunction(t, "repo_docs_url_for_path") + `
` + extractRootInstallShellFunction(t, "repo_docker_docs_url") + `
` + extractRootInstallShellFunction(t, "repo_docker_image_ref") + `
` + extractRootInstallShellFunction(t, "check_docker_environment") + `
		container="docker"
		check_docker_environment
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err == nil {
		t.Fatalf("expected docker environment check to exit non-zero:\n%s", out)
	}
	got := string(out)
	if !strings.Contains(got, "docker run -d -p 7655:7655 example/pulse-enterprise:latest") {
		t.Fatalf("docker guidance missing configured image repo:\n%s", got)
	}
	if !strings.Contains(got, "https://github.com/example/pulse-fork/blob/v9.9.9/docs/DOCKER.md") {
		t.Fatalf("docker guidance missing configured docs url:\n%s", got)
	}
}

func TestBuildPrintedManagementCommandPreservesRCChannel(t *testing.T) {
	script := `
		GITHUB_REPO="rcourtman/Pulse"
		FORCE_VERSION=""
		FORCE_CHANNEL="rc"
		UPDATE_CHANNEL=""
` + extractRootInstallShellFunction(t, "build_printed_management_command") + `
		build_printed_management_command update
		build_printed_management_command reset
		build_printed_management_command uninstall
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 commands, got %d:\n%s", len(lines), out)
	}
	if got := lines[0]; got != "/bin/update --rc" {
		t.Fatalf("update command missing rc flag: %s", lines[0])
	}
	if got := lines[1]; got != "/bin/update --rc --reset" {
		t.Fatalf("reset command missing rc flag: %s", lines[1])
	}
	if strings.Contains(lines[2], "--rc") {
		t.Fatalf("uninstall command should not include channel flags: %s", lines[2])
	}
}

func TestRootPrintCompletionRevealsBootstrapTokenThroughCLI(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	installDir := filepath.Join(tmpDir, "install")
	pulseBin := filepath.Join(tmpDir, "bin", "pulse")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(pulseBin), 0755); err != nil {
		t.Fatalf("mkdir pulse bin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, ".bootstrap_token"), []byte(`{"version":2,"token_ciphertext":"encrypted-token","token_hash":"hash"}`), 0600); err != nil {
		t.Fatalf("write encrypted bootstrap token marker: %v", err)
	}
	if err := os.WriteFile(pulseBin, []byte(`#!/usr/bin/env bash
if [[ "$1" != "bootstrap-token" ]]; then
  exit 2
fi
printf 'Token: raw-bootstrap-token\n'
printf 'Data: %s\n' "$PULSE_DATA_DIR"
`), 0755); err != nil {
		t.Fatalf("write fake pulse binary: %v", err)
	}

	script := `
		RED=''
		GREEN=''
		YELLOW=''
		BLUE=''
		NC=''
		CONFIG_DIR="$TEST_CONFIG_DIR"
		INSTALL_DIR="$TEST_INSTALL_DIR"
		BINARY_LINK_PATH="$TEST_PULSE_BIN"
		UPDATE_HELPER_PATH="/bin/update"
		SERVICE_NAME="pulse"
		UPDATE_TIMER_UNIT="pulse-update.timer"
		hostname() { if [[ "${1:-}" == "-I" ]]; then printf '127.0.0.1\n'; else command hostname "$@"; fi; }
		current_frontend_port() { printf '7655\n'; }
		print_header() { :; }
		print_success() { printf '%s\n' "$1"; }
		print_warn() { printf 'WARN: %s\n' "$1"; }
		print_info() { printf 'INFO: %s\n' "$1"; }
		update_timer_exists() { return 1; }
		update_timer_enabled() { return 1; }
		build_printed_management_command() { printf '/bin/update\n'; }
` + extractRootInstallShellFunction(t, "print_completion") + `
		print_completion
	`

	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(),
		"TEST_CONFIG_DIR="+configDir,
		"TEST_INSTALL_DIR="+installDir,
		"TEST_PULSE_BIN="+pulseBin,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, "raw-bootstrap-token") {
		t.Fatalf("completion output did not include revealed token:\n%s", got)
	}
	if !strings.Contains(got, "Data: "+configDir) {
		t.Fatalf("completion output did not pass PULSE_DATA_DIR to bootstrap-token:\n%s", got)
	}
	if strings.Contains(got, "encrypted-token") || strings.Contains(got, "token_ciphertext") {
		t.Fatalf("completion output leaked encrypted bootstrap file contents:\n%s", got)
	}
}

func TestBuildPrintedManagementCommandPreservesForcedVersion(t *testing.T) {
	script := `
		GITHUB_REPO="rcourtman/Pulse"
		FORCE_VERSION="v1.2.3"
		FORCE_CHANNEL=""
		UPDATE_CHANNEL=""
` + extractRootInstallShellFunction(t, "build_printed_management_command") + `
		build_printed_management_command update
		build_printed_management_command reset
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 commands, got %d:\n%s", len(lines), out)
	}
	if got := lines[0]; got != "/bin/update --version v1.2.3" {
		t.Fatalf("update command missing version pin: %s", lines[0])
	}
	if got := lines[1]; got != "/bin/update --version v1.2.3 --reset" {
		t.Fatalf("reset command missing version pin: %s", lines[1])
	}
}

func TestBuildPrintedManagementCommandUsesConfiguredHelperPath(t *testing.T) {
	script := `
		GITHUB_REPO="rcourtman/Pulse"
		FORCE_VERSION=""
		FORCE_CHANNEL=""
		UPDATE_CHANNEL=""
		UPDATE_HELPER_PATH="/usr/local/bin/update-pulse-preview"
` + extractRootInstallShellFunction(t, "build_printed_management_command") + `
		build_printed_management_command update
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	if got := strings.TrimSpace(string(out)); got != "/usr/local/bin/update-pulse-preview" {
		t.Fatalf("printed command = %q, want configured helper path", got)
	}
}

func TestSelectedUpdateChannelTreatsPrereleaseVersionAsRC(t *testing.T) {
	script := `
		FORCE_CHANNEL=""
		FORCE_VERSION="v1.2.3-rc.4"
		UPDATE_CHANNEL=""
		CONFIG_DIR="` + t.TempDir() + `"
` + extractSelectedUpdateChannelShellFunctions(t) + `
		selected_update_channel
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "rc" {
		t.Fatalf("selected_update_channel = %q, want rc", got)
	}
}

func TestResolveTargetReleaseIgnoresHostConfiguredRCChannelForFreshLXCInstall(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "system.json"), []byte(`{"updateChannel":"rc"}`), 0644); err != nil {
		t.Fatalf("write system.json: %v", err)
	}

	timeoutPath := filepath.Join(tmpDir, "timeout")
	curlPath := filepath.Join(tmpDir, "curl")

	if err := os.WriteFile(timeoutPath, []byte("#!/usr/bin/env bash\nshift\nexec \"$@\"\n"), 0755); err != nil {
		t.Fatalf("write timeout stub: %v", err)
	}

	curlStub := `#!/usr/bin/env bash
for arg in "$@"; do
	if [[ "$arg" == "https://api.github.com/repos/rcourtman/Pulse/releases" ]]; then
		printf '%s\n' '[{"draft":false,"prerelease":false,"tag_name":"v6.0.0-rc.1"},{"draft":false,"prerelease":false,"tag_name":"v5.1.28"}]'
		exit 0
	fi
done
echo "unexpected curl invocation: $*" >&2
exit 1
`
	if err := os.WriteFile(curlPath, []byte(curlStub), 0755); err != nil {
		t.Fatalf("write curl stub: %v", err)
	}

	script := `
		PATH="` + tmpDir + `:$PATH"
		GITHUB_REPO="rcourtman/Pulse"
		FORCE_VERSION=""
		FORCE_CHANNEL=""
		UPDATE_CHANNEL=""
		IGNORE_CONFIGURED_UPDATE_CHANNEL="true"
		CONFIG_DIR="` + configDir + `"
		print_info() { :; }
		print_warn() { :; }
		get_latest_release_from_redirect() { return 1; }
` + extractRootInstallShellFunction(t, "read_configured_update_channel") + `
` + extractRootInstallShellFunction(t, "is_stable_release_tag") + `
` + extractRootInstallShellFunction(t, "latest_stable_release_tag_from_json") + `
` + extractRootInstallShellFunction(t, "resolve_target_release") + `
		resolve_target_release
		printf '%s\n' "$LATEST_RELEASE"
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "v5.1.28" {
		t.Fatalf("LATEST_RELEASE = %q, want v5.1.28", got)
	}
}

func TestSetupAutoUpdatesCreatesSystemJSONWithSelectedChannel(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	installDir := filepath.Join(tmpDir, "install")
	autoUpdateSrc := filepath.Join(installDir, "scripts", "pulse-auto-update.sh")
	autoUpdateDest, servicePath, timerPath := prepareAutoUpdatePaths(t, tmpDir)

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(autoUpdateSrc), 0755); err != nil {
		t.Fatalf("mkdir auto-update src dir: %v", err)
	}
	if err := os.WriteFile(autoUpdateSrc, []byte("#!/usr/bin/env bash\n"), 0755); err != nil {
		t.Fatalf("write auto-update src: %v", err)
	}

	script := `
		CONFIG_DIR="` + configDir + `"
		INSTALL_DIR="` + installDir + `"
		PULSE_AUTO_UPDATE_DEST="` + autoUpdateDest + `"
		PULSE_UPDATE_SERVICE_PATH="` + servicePath + `"
		PULSE_UPDATE_TIMER_PATH="` + timerPath + `"
		FORCE_CHANNEL="rc"
		UPDATE_CHANNEL=""
		GITHUB_REPO="rcourtman/Pulse"
		print_info() { :; }
		print_warn() { :; }
		print_success() { :; }
		safe_systemctl() { :; }
		systemctl() { return 0; }
		cp() { command cp "$@"; }
		chmod() { command chmod "$@"; }
		chown() { :; }
		cat() { command cat "$@"; }
		mkdir() { command mkdir "$@"; }
` + extractSetupAutoUpdatesShellFunctions(t) + `
		setup_auto_updates
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	content, err := os.ReadFile(filepath.Join(configDir, "system.json"))
	if err != nil {
		t.Fatalf("read system.json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(content, &got); err != nil {
		t.Fatalf("parse system.json: %v\n%s", err, content)
	}
	if enabled, ok := got["autoUpdateEnabled"].(bool); !ok || !enabled {
		t.Fatalf("system.json missing autoUpdateEnabled=true:\n%s", content)
	}
	if channel, ok := got["updateChannel"].(string); !ok || channel != "rc" {
		t.Fatalf("system.json missing updateChannel rc:\n%s", content)
	}
}

func TestSetupAutoUpdatesConfiguresInstalledAutoUpdateRepo(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	systemdDir := filepath.Join(tmpDir, "systemd")
	installDir := filepath.Join(tmpDir, "install")
	autoUpdateSrc := filepath.Join(installDir, "scripts", "pulse-auto-update.sh")
	autoUpdateDest := filepath.Join(tmpDir, "bin", "pulse-auto-update.sh")
	servicePath := filepath.Join(systemdDir, "pulse-update.service")
	timerPath := filepath.Join(systemdDir, "pulse-update.timer")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.MkdirAll(systemdDir, 0755); err != nil {
		t.Fatalf("mkdir systemd dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(autoUpdateSrc), 0755); err != nil {
		t.Fatalf("mkdir auto-update src dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(autoUpdateDest), 0755); err != nil {
		t.Fatalf("mkdir auto-update dest dir: %v", err)
	}
	if err := os.WriteFile(autoUpdateSrc, []byte("#!/usr/bin/env bash\nGITHUB_REPO=\"rcourtman/Pulse\"\n"), 0755); err != nil {
		t.Fatalf("write auto-update src: %v", err)
	}

	script := `
		CONFIG_DIR="` + configDir + `"
		INSTALL_DIR="` + installDir + `"
		PULSE_AUTO_UPDATE_DEST="` + autoUpdateDest + `"
		PULSE_UPDATE_SERVICE_PATH="` + servicePath + `"
		PULSE_UPDATE_TIMER_PATH="` + timerPath + `"
		FORCE_CHANNEL=""
		UPDATE_CHANNEL=""
		GITHUB_REPO="example/pulse-fork"
		print_info() { :; }
		print_warn() { :; }
		print_success() { :; }
		safe_systemctl() { :; }
		systemctl() { return 0; }
		cp() { command cp "$@"; }
		chmod() { command chmod "$@"; }
		chown() { :; }
		cat() { command cat "$@"; }
		mkdir() { command mkdir "$@"; }
		mv() { command mv "$@"; }
		rm() { command rm "$@"; }
		awk() { command awk "$@"; }
		mktemp() { command mktemp "$@"; }
` + extractSetupAutoUpdatesShellFunctions(t) + `
		setup_auto_updates
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	content, err := os.ReadFile(autoUpdateDest)
	if err != nil {
		t.Fatalf("read configured auto-update script: %v", err)
	}
	got := string(content)
	if strings.Contains(got, "GITHUB_REPO=\"rcourtman/Pulse\"") {
		t.Fatalf("auto-update script kept upstream repo:\n%s", got)
	}
	if !strings.Contains(got, "GITHUB_REPO=\"example/pulse-fork\"") {
		t.Fatalf("auto-update script missing configured repo:\n%s", got)
	}

	serviceContent, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("read service file: %v", err)
	}
	if !strings.Contains(string(serviceContent), "ExecStart="+autoUpdateDest) {
		t.Fatalf("service file missing configured auto-update path:\n%s", serviceContent)
	}
	if !strings.Contains(string(serviceContent), "Documentation=https://github.com/example/pulse-fork") {
		t.Fatalf("service file missing configured documentation url:\n%s", serviceContent)
	}
	if _, err := os.Stat(timerPath); err != nil {
		t.Fatalf("timer file missing: %v", err)
	}
	timerContent, err := os.ReadFile(timerPath)
	if err != nil {
		t.Fatalf("read timer file: %v", err)
	}
	if !strings.Contains(string(timerContent), "Documentation=https://github.com/example/pulse-fork") {
		t.Fatalf("timer file missing configured documentation url:\n%s", timerContent)
	}
}

func TestSetupAutoUpdatesTreatsPrereleaseVersionAsRCChannel(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	installDir := filepath.Join(tmpDir, "install")
	autoUpdateSrc := filepath.Join(installDir, "scripts", "pulse-auto-update.sh")
	autoUpdateDest, servicePath, timerPath := prepareAutoUpdatePaths(t, tmpDir)

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(autoUpdateSrc), 0755); err != nil {
		t.Fatalf("mkdir auto-update src dir: %v", err)
	}
	if err := os.WriteFile(autoUpdateSrc, []byte("#!/usr/bin/env bash\n"), 0755); err != nil {
		t.Fatalf("write auto-update src: %v", err)
	}

	script := `
		CONFIG_DIR="` + configDir + `"
		INSTALL_DIR="` + installDir + `"
		PULSE_AUTO_UPDATE_DEST="` + autoUpdateDest + `"
		PULSE_UPDATE_SERVICE_PATH="` + servicePath + `"
		PULSE_UPDATE_TIMER_PATH="` + timerPath + `"
		FORCE_CHANNEL=""
		FORCE_VERSION="v1.2.3-rc.4"
		UPDATE_CHANNEL=""
		GITHUB_REPO="rcourtman/Pulse"
		print_info() { :; }
		print_warn() { :; }
		print_success() { :; }
		safe_systemctl() { :; }
		systemctl() { return 0; }
		chown() { :; }
` + extractSetupAutoUpdatesShellFunctions(t) + `
		setup_auto_updates
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	content, err := os.ReadFile(filepath.Join(configDir, "system.json"))
	if err != nil {
		t.Fatalf("read system.json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(content, &got); err != nil {
		t.Fatalf("parse system.json: %v\n%s", err, content)
	}
	if channel, ok := got["updateChannel"].(string); !ok || channel != "rc" {
		t.Fatalf("prerelease version did not persist rc channel:\n%s", content)
	}
}

func TestSetupAutoUpdatesPreservesRCChannelWhenUpdatingExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	installDir := filepath.Join(tmpDir, "install")
	autoUpdateSrc := filepath.Join(installDir, "scripts", "pulse-auto-update.sh")
	autoUpdateDest, servicePath, timerPath := prepareAutoUpdatePaths(t, tmpDir)

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(autoUpdateSrc), 0755); err != nil {
		t.Fatalf("mkdir auto-update src dir: %v", err)
	}
	if err := os.WriteFile(autoUpdateSrc, []byte("#!/usr/bin/env bash\n"), 0755); err != nil {
		t.Fatalf("write auto-update src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "system.json"), []byte(`{"updateChannel":"rc","autoUpdateEnabled":false}`), 0644); err != nil {
		t.Fatalf("write system.json: %v", err)
	}

	script := `
		CONFIG_DIR="` + configDir + `"
		INSTALL_DIR="` + installDir + `"
		PULSE_AUTO_UPDATE_DEST="` + autoUpdateDest + `"
		PULSE_UPDATE_SERVICE_PATH="` + servicePath + `"
		PULSE_UPDATE_TIMER_PATH="` + timerPath + `"
		FORCE_CHANNEL=""
		UPDATE_CHANNEL=""
		GITHUB_REPO="rcourtman/Pulse"
		print_info() { :; }
		print_warn() { :; }
		print_success() { :; }
		safe_systemctl() { :; }
		systemctl() { return 0; }
		command -v jq >/dev/null 2>&1 || true
		chown() { :; }
` + extractSetupAutoUpdatesShellFunctions(t) + `
		setup_auto_updates
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	content, err := os.ReadFile(filepath.Join(configDir, "system.json"))
	if err != nil {
		t.Fatalf("read system.json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(content, &got); err != nil {
		t.Fatalf("parse system.json: %v\n%s", err, content)
	}
	if enabled, ok := got["autoUpdateEnabled"].(bool); !ok || !enabled {
		t.Fatalf("system.json missing enabled flag:\n%s", content)
	}
	if channel, ok := got["updateChannel"].(string); !ok || channel != "rc" {
		t.Fatalf("system.json lost rc channel:\n%s", content)
	}
}
