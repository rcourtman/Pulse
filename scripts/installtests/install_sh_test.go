package installtests

import (
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestContainerAgentCompatibilityWrapperDefaultsToUnifiedHostAndDockerMonitoring(t *testing.T) {
	wrapperBytes, err := os.ReadFile(repoFile("scripts", "install-container-agent.sh"))
	if err != nil {
		t.Fatalf("read container-agent compatibility wrapper: %v", err)
	}
	wrapper := string(wrapperBytes)

	if !strings.Contains(wrapper, `forward_args+=(--enable-docker --enable-host)`) {
		t.Fatal("container-agent compatibility wrapper must preserve host telemetry while enabling Docker")
	}
	if strings.Contains(wrapper, `forward_args+=(--enable-docker --disable-host)`) {
		t.Fatal("container-agent compatibility wrapper must not silently force workload-only mode")
	}
	if !strings.Contains(wrapper, `--enable-host=false for an intentional`) {
		t.Fatal("container-agent compatibility wrapper must document the explicit workload-only path")
	}
}

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

func TestInstallSHPersistsAndVerifiesServerFingerprint(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`SERVER_FINGERPRINT="${PULSE_SERVER_FINGERPRINT:-}"`,
		`--server-fingerprint <sha256>`,
		`verify_pinned_server_certificate() {`,
		`openssl s_client -connect "$target" -servername "$host"`,
		`EXEC_ARG_ITEMS+=(--server-fingerprint "$SERVER_FINGERPRINT")`,
		`write_connection_state_value "$conn_tmp" "PULSE_SERVER_FINGERPRINT" "$SERVER_FINGERPRINT"`,
		`SERVER_FINGERPRINT=$(read_connection_state_value "$file" "PULSE_SERVER_FINGERPRINT")`,
		`if [[ "$OS" == "linux" || "$OS" == "freebsd" ]]; then`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing server-fingerprint lifecycle contract: %s", needle)
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
		`NEW_VERSION_NORMALIZED="${NEW_VERSION#v}"`,
		`SERVER_VERSION_NORMALIZED="${SERVER_VERSION#v}"`,
		`"$NEW_VERSION_NORMALIZED" != "$SERVER_VERSION_NORMALIZED"`,
		`Downloaded agent version (${NEW_VERSION}) does not match Pulse server version (${SERVER_VERSION})`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing version-aware agent download behavior: %s", needle)
		}
	}
}

// Regression test for #1527: the agent binary reports its version as "v6.0.4"
// while the server /api/version reports "6.0.4", so the mismatch check must
// strip a leading "v" before comparing and only warn on a genuine difference.
func TestInstallSHAgentVersionMismatchIgnoresVPrefix(t *testing.T) {
	harness := `
set -euo pipefail
log_warn() { echo "WARN:$*"; }
SERVER_VERSION="$1"
NEW_VERSION="$2"
NEW_VERSION_NORMALIZED="${NEW_VERSION#v}"
SERVER_VERSION_NORMALIZED="${SERVER_VERSION#v}"
if [[ -n "$SERVER_VERSION" && -n "$NEW_VERSION" && "$NEW_VERSION" != "unknown" && "$NEW_VERSION_NORMALIZED" != "$SERVER_VERSION_NORMALIZED" ]]; then
    log_warn "Downloaded agent version (${NEW_VERSION}) does not match Pulse server version (${SERVER_VERSION})."
fi
echo DONE
`
	run := func(server, agent string) string {
		out, err := exec.Command("bash", "-c", harness, "_", server, agent).CombinedOutput()
		if err != nil {
			t.Fatalf("bash: %v\n%s", err, out)
		}
		return string(out)
	}

	if got := run("6.0.4", "v6.0.4"); strings.Contains(got, "WARN:") {
		t.Fatalf("equal versions differing only by a leading v warned:\n%s", got)
	}
	if got := run("6.0.4", "6.0.4"); strings.Contains(got, "WARN:") {
		t.Fatalf("identical versions warned:\n%s", got)
	}
	if got := run("6.0.4", "v6.0.3"); !strings.Contains(got, "WARN:") {
		t.Fatalf("genuine version mismatch did not warn:\n%s", got)
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
		`if [[ "$ENABLE_COMMANDS" != "true" ]]; then`,
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

// Only a PVE agent explicitly installed with --enable-commands may receive the
// lxc-attach capability grant. A monitoring-only unit must not carry dormant
// CAP_SETUID/CAP_SETGID authority for a future remote toggle.
func TestInstallSHGrantsLXCAttachCapabilitiesOnlyForCommandPVEAgent(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`systemd_agent_may_attach_lxc() {`,
		`if [[ "$ENABLE_COMMANDS" != "true" ]]; then`,
		`if [[ "$ENABLE_PROXMOX" != "true" ]]; then`,
		`if systemd_agent_may_attach_lxc; then`,
		`AmbientCapabilities=CAP_SETUID CAP_SETGID`,
		`SystemCallArchitectures=native${ambient_line}`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing unprivileged-LXC attach capability grant: %s", needle)
		}
	}

	if strings.Contains(script, "command execution can be turned on later") {
		t.Fatal("install.sh still provisions dormant attach capabilities for remote command promotion")
	}
}

func TestRenderedSystemdUnitDoesNotPregrantCommandCapabilities(t *testing.T) {
	for _, tc := range []struct {
		name           string
		enableCommands bool
		enableProxmox  bool
		proxmoxType    string
		leastPrivilege bool
		wantCaps       bool
	}{
		{name: "monitoring PVE", enableProxmox: true, proxmoxType: "pve"},
		{name: "explicit command PVE", enableCommands: true, enableProxmox: true, proxmoxType: "pve", wantCaps: true},
		{name: "PBS command profile", enableCommands: true, enableProxmox: true, proxmoxType: "pbs"},
		{name: "least privilege PVE", enableCommands: true, enableProxmox: true, proxmoxType: "pve", leastPrivilege: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unitPath := filepath.Join(t.TempDir(), "pulse-agent.service")
			script := `
set -euo pipefail
ENABLE_COMMANDS="` + strconv.FormatBool(tc.enableCommands) + `"
ENABLE_PROXMOX="` + strconv.FormatBool(tc.enableProxmox) + `"
PROXMOX_TYPE="` + tc.proxmoxType + `"
LEAST_PRIVILEGE="` + strconv.FormatBool(tc.leastPrivilege) + `"
GRANT_SMART="false"
GRANT_PCT="false"
SYSTEMD_ENV_LINES=""
` + extractInstallShellFunction(t, "systemd_agent_requires_lxc_attach") + `
` + extractInstallShellFunction(t, "systemd_agent_may_attach_lxc") + `
` + extractInstallShellFunction(t, "render_systemd_agent_unit") + `
render_systemd_agent_unit "` + unitPath + `" "/usr/local/bin/pulse-agent" "--url https://pulse" "network-online.target" "" "root" ""
`
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if err != nil {
				t.Fatalf("render systemd unit: %v\n%s", err, out)
			}
			unit, err := os.ReadFile(unitPath)
			if err != nil {
				t.Fatalf("read rendered unit: %v", err)
			}
			text := string(unit)
			hasCaps := strings.Contains(text, "AmbientCapabilities=CAP_SETUID CAP_SETGID")
			if hasCaps != tc.wantCaps {
				t.Fatalf("rendered capability grant = %v, want %v:\n%s", hasCaps, tc.wantCaps, text)
			}
			if tc.wantCaps {
				if !strings.Contains(text, "NoNewPrivileges=false") || !strings.Contains(text, "RestrictSUIDSGID=false") {
					t.Fatalf("command-capable PVE unit omitted required attach relaxations:\n%s", text)
				}
			} else if !strings.Contains(text, "NoNewPrivileges=true") || !strings.Contains(text, "RestrictSUIDSGID=true") {
				t.Fatalf("monitoring unit did not retain hardening:\n%s", text)
			}
		})
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

func TestInstallSHPreflightFollowsAgentDownloadRedirect(t *testing.T) {
	var requestedArtifactMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/download/pulse-agent":
			w.Header().Set("X-Checksum-Sha256", strings.Repeat("a", 64))
			http.Redirect(w, r, "/artifacts/pulse-agent?"+r.URL.RawQuery, http.StatusTemporaryRedirect)
		case "/artifacts/pulse-agent":
			requestedArtifactMethod = r.Method
			w.Header().Set("X-Checksum-Sha256", strings.Repeat("b", 64))
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
		t.Fatalf("redirected preflight failed: %v\n%s", err, out)
	}

	if got := string(out); !strings.Contains(got, `"code":"agent_download_available"`) {
		t.Fatalf("redirected preflight did not report agent download availability:\n%s", got)
	}
	if requestedArtifactMethod != http.MethodHead {
		t.Fatalf("redirected artifact method = %q, want HEAD", requestedArtifactMethod)
	}
}

func TestInstallSHPreflightRejectsChecksumOnlyOnRedirectResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/download/pulse-agent":
			w.Header().Set("X-Checksum-Sha256", strings.Repeat("a", 64))
			http.Redirect(w, r, "/artifacts/pulse-agent?"+r.URL.RawQuery, http.StatusTemporaryRedirect)
		case "/artifacts/pulse-agent":
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
	if err == nil {
		t.Fatalf("preflight accepted checksum metadata from an intermediate redirect:\n%s", out)
	}
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 12 {
		t.Fatalf("redirect-only checksum exit = %v, want 12\n%s", err, out)
	}
	if got := string(out); !strings.Contains(got, `"code":"agent_download_checksum_missing"`) {
		t.Fatalf("preflight did not report missing final-response checksum metadata:\n%s", got)
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
		`CURL_DOWNLOAD_CHECK_ARGS=(-fsSIL --connect-timeout 5 --max-time 30 -D "$PREFLIGHT_HEADERS" -o /dev/null)`,
		`final_response_header_value "$PREFLIGHT_HEADERS" "X-Checksum-Sha256"`,
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
		DISK_INCLUDES=("/var/log")
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
		`<string>--state-dir</string>`,
		`<string>/var/lib/pulse-agent</string>`,
		`<string>--disk-exclude</string>`,
		`<string>Time Machine</string>`,
		`<string>--disk-include</string>`,
		`<string>/var/log</string>`,
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
		`write_connection_state_value "$conn_tmp" "PULSE_STATE_DIR" "$state_dir"`,
		`write_connection_state_value "$conn_tmp" "PULSE_TOKEN_FILE" "$RUNTIME_TOKEN_FILE"`,
		`write_connection_state_value "$conn_tmp" "PULSE_AGENT_ID" "$AGENT_ID"`,
		`write_connection_state_value "$conn_tmp" "PULSE_HOSTNAME" "$HOSTNAME_OVERRIDE"`,
		`write_connection_state_value "$conn_tmp" "PULSE_INSECURE_SKIP_VERIFY" "true"`,
		`write_connection_state_value "$conn_tmp" "PULSE_CACERT" "$CURL_CA_BUNDLE"`,
		`recover_connection_state "$lifecycle_conn_env"`,
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
		`if [[ "$UPDATE_ONLY" == "true" || "$UNINSTALL" == "true" ]]; then`,
		`# An explicit state directory is authoritative.`,
		`lifecycle_conn_env=$(find_connection_state_file || true)`,
		`recover_connection_state "$lifecycle_conn_env"`,
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
		`--retarget              Point an existing agent at --url using saved identity and token`,
		`UPDATE_ONLY="false"`,
		`RETARGET_ONLY="false"`,
		`--update) UPDATE_ONLY="true"; shift ;;`,
		`--retarget) RETARGET_ONLY="true"; UPDATE_ONLY="true"; shift ;;`,
		`--retarget requires the new Pulse endpoint in --url`,
		`if [[ "$UPDATE_ONLY" == "true" || "$UNINSTALL" == "true" ]]; then`,
		`lifecycle_conn_env=$(find_connection_state_file || true)`,
		`recover_connection_state "$lifecycle_conn_env"`,
		`recover_connection_state_from_existing_agent() {`,
		`recover_connection_state_from_running_agent`,
		`recover_connection_state_from_systemd_unit`,
		`recover_connection_state_from_launchd_plist`,
		`recover_connection_state_from_service_scripts`,
		`running_agent_arg_stream() {`,
		`running_agent_env_stream() {`,
		`recover_connection_state_from_arg_stream`,
		`recover_token_from_default_agent_token_file() {`,
		`normalize_recovered_agent_arg_key() {`,
		`-url|-pulse-url|-token|-token-file|-interval|-agent-id|-hostname|-report-ip|-cacert|-server-fingerprint|-observers-file|-command-authority|-health-addr|-state-dir|-kubeconfig|-proxmox-type|-disk-exclude|-disk-include)`,
		`--enable-host|-enable-host|--enable-host=true|-enable-host=true)`,
		`recover_connection_state_from_env_stream`,
		`recovered_connection_state_ready() {`,
		`update_connection_state_incomplete() {`,
		`[[ "$RECOVERED_AGENT_ARG_STATE" == "true" ]] && recovered_connection_state_ready`,
		`[[ "$RECOVERED_AGENT_ENV_STATE" == "true" ]] && recovered_connection_state_ready`,
		`if update_connection_state_incomplete; then`,
		`recover_connection_state_from_existing_agent || true`,
		`if [[ "$UPDATE_ONLY" == "true" && ( -z "$PULSE_URL" || -z "$PULSE_TOKEN" ) ]]; then`,
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

func TestInstallSHRetargetPreservesIdentityWithoutOldEndpointTrust(t *testing.T) {
	stateDir := t.TempDir()
	tokenPath := filepath.Join(stateDir, "token")
	if err := os.WriteFile(tokenPath, []byte("deadbeef\n"), 0600); err != nil {
		t.Fatalf("write token fixture: %v", err)
	}
	connectionPath := filepath.Join(stateDir, "connection.env")
	connection := strings.Join([]string{
		"PULSE_URL='https://old-pulse.example.test:7655'",
		"PULSE_TOKEN_FILE='" + tokenPath + "'",
		"PULSE_AGENT_ID='agent-123'",
		"PULSE_HOSTNAME='pve-one'",
		"PULSE_INSECURE_SKIP_VERIFY='true'",
		"PULSE_SERVER_FINGERPRINT='aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'",
		"PULSE_CACERT='/etc/pulse/old-ca.pem'",
	}, "\n") + "\n"
	if err := os.WriteFile(connectionPath, []byte(connection), 0600); err != nil {
		t.Fatalf("write connection fixture: %v", err)
	}

	script := `
		PULSE_URL="https://new-pulse.example.test:7655"
		PULSE_TOKEN=""
		RETARGET_ONLY="true"
		INSECURE="false"
		SERVER_FINGERPRINT=""
		CURL_CA_BUNDLE=""
		AGENT_ID=""
		HOSTNAME_OVERRIDE=""
		REPORT_IP=""
		STATE_DIR="/var/lib/pulse-agent"
		STATE_DIR_SOURCE="default"
		DEFAULT_STATE_DIR="/var/lib/pulse-agent"
		TRUENAS_STATE_DIR="/data/pulse-agent"
` + extractInstallShellFunction(t, "read_connection_state_value") + `
` + extractInstallShellFunction(t, "recover_token_from_default_agent_token_file") + `
` + extractInstallShellFunction(t, "recover_connection_state") + `
		recover_connection_state "${PULSE_TEST_CONNECTION:?}"
		printf 'URL=%s\nTOKEN=%s\nAGENT_ID=%s\nHOSTNAME=%s\nINSECURE=%s\nFINGERPRINT=%s\nCACERT=%s\n' \
			"$PULSE_URL" "$PULSE_TOKEN" "$AGENT_ID" "$HOSTNAME_OVERRIDE" "$INSECURE" "$SERVER_FINGERPRINT" "$CURL_CA_BUNDLE"
	`

	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), "PULSE_TEST_CONNECTION="+connectionPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	for _, needle := range []string{
		"URL=https://new-pulse.example.test:7655",
		"TOKEN=deadbeef",
		"AGENT_ID=agent-123",
		"HOSTNAME=pve-one",
		"INSECURE=false",
		"FINGERPRINT=",
		"CACERT=",
	} {
		if !strings.Contains(got, needle) {
			t.Fatalf("retarget state missing %q:\n%s", needle, got)
		}
	}
	if strings.Contains(got, "old-pulse") || strings.Contains(got, "old-ca") || strings.Contains(got, "aaaaaaaa") {
		t.Fatalf("retarget carried old endpoint trust or URL:\n%s", got)
	}
}

func TestInstallSHRetargetDoesNotRecoverLegacyServiceTrust(t *testing.T) {
	script := `
		PULSE_URL="https://new-pulse.example.test:7655"
		PULSE_TOKEN=""
		RETARGET_ONLY="true"
		INSECURE="false"
		INSECURE_EXPLICIT="false"
		SERVER_FINGERPRINT=""
		CURL_CA_BUNDLE=""
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
		ENABLE_COMMANDS="false"
		ENROLL="false"
		COMMAND_AUTHORITY_SOURCE=""
		HEALTH_ADDR=""
		HEALTH_ADDR_SET="false"
		AGENT_ID=""
		HOSTNAME_OVERRIDE=""
		REPORT_IP=""
		STATE_DIR="/var/lib/pulse-agent"
		STATE_DIR_SOURCE="default"
		OBSERVERS_FILE=""
		KUBE_INCLUDE_ALL_PODS="false"
		KUBE_INCLUDE_ALL_DEPLOYMENTS="false"
		DISK_EXCLUDES=()
		DISK_INCLUDES=()
` + extractInstallShellFunction(t, "strip_recovered_arg_quotes") + `
` + extractInstallShellFunction(t, "normalize_recovered_agent_arg_key") + `
` + extractInstallShellFunction(t, "apply_recovered_agent_arg_value") + `
` + extractInstallShellFunction(t, "recovered_connection_state_ready") + `
` + extractInstallShellFunction(t, "recover_token_from_default_agent_token_file") + `
` + extractInstallShellFunction(t, "recover_connection_state_from_arg_stream") + `
` + extractInstallShellFunction(t, "recover_connection_state_from_env_stream") + `
		recover_connection_state_from_arg_stream <<'ARGS'
/usr/local/bin/pulse-agent
--url=https://old-pulse.example.test:7655
--token=deadbeef
--agent-id=agent-123
--hostname=pve-one
--insecure
--cacert=/etc/pulse/old-ca.pem
--server-fingerprint=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
ARGS
		recover_connection_state_from_env_stream <<'ENV'
PULSE_INSECURE_SKIP_VERIFY=true
PULSE_CACERT=/etc/pulse/older-ca.pem
PULSE_SERVER_FINGERPRINT=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
ENV
		printf 'URL=%s\nTOKEN=%s\nAGENT_ID=%s\nHOSTNAME=%s\nINSECURE=%s\nFINGERPRINT=%s\nCACERT=%s\n' \
			"$PULSE_URL" "$PULSE_TOKEN" "$AGENT_ID" "$HOSTNAME_OVERRIDE" "$INSECURE" "$SERVER_FINGERPRINT" "$CURL_CA_BUNDLE"
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	for _, needle := range []string{
		"URL=https://new-pulse.example.test:7655",
		"TOKEN=deadbeef",
		"AGENT_ID=agent-123",
		"HOSTNAME=pve-one",
		"INSECURE=false",
		"FINGERPRINT=",
		"CACERT=",
	} {
		if !strings.Contains(got, needle) {
			t.Fatalf("retarget service recovery missing %q:\n%s", needle, got)
		}
	}
	if strings.Contains(got, "old-pulse") || strings.Contains(got, "old-ca") || strings.Contains(got, "aaaaaaaa") || strings.Contains(got, "bbbbbbbb") {
		t.Fatalf("retarget carried old service trust or URL:\n%s", got)
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
		DISK_INCLUDES=()
		RUNTIME_TOKEN_FILE="/var/lib/pulse-agent/token"
` + extractInstallShellFunction(t, "strip_recovered_arg_quotes") + `
` + extractInstallShellFunction(t, "normalize_recovered_agent_arg_key") + `
` + extractInstallShellFunction(t, "apply_recovered_agent_arg_value") + `
` + extractInstallShellFunction(t, "recovered_connection_state_ready") + `
` + extractInstallShellFunction(t, "recover_token_from_default_agent_token_file") + `
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
--disk-exclude
/dev/sda
--disk-exclude=/var/run/samba/fd
--disk-include
/var/log
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
		"--disk-exclude /dev/sda",
		"--disk-exclude /var/run/samba/fd",
		"--disk-include /var/log",
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

func TestInstallSHRecoversV5ProcessArgsWithoutProcfs(t *testing.T) {
	script := `
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
		TRUENAS_STATE_DIR="/data/pulse-agent"
		CURL_CA_BUNDLE=""
		KUBE_INCLUDE_ALL_PODS="false"
		KUBE_INCLUDE_ALL_DEPLOYMENTS="false"
		DISK_EXCLUDES=()
		RUNTIME_TOKEN_FILE="/var/lib/pulse-agent/token"
		BINARY_NAME="pulse-agent"
` + extractInstallShellFunction(t, "strip_recovered_arg_quotes") + `
` + extractInstallShellFunction(t, "normalize_recovered_agent_arg_key") + `
` + extractInstallShellFunction(t, "apply_recovered_agent_arg_value") + `
` + extractInstallShellFunction(t, "recovered_connection_state_ready") + `
` + extractInstallShellFunction(t, "recover_token_from_default_agent_token_file") + `
` + extractInstallShellFunction(t, "recover_connection_state_from_arg_stream") + `
` + extractInstallShellFunction(t, "split_recovered_shell_words") + `
` + extractInstallShellFunction(t, "running_agent_arg_stream") + `
` + extractInstallShellFunction(t, "recover_connection_state_from_running_agent") + `
` + extractInstallShellFunction(t, "build_exec_arg_items") + `
` + extractInstallShellFunction(t, "join_exec_arg_items") + `
` + extractInstallShellFunction(t, "build_exec_args") + `
		collect_running_agent_pids() { printf '%s\n' 4242; }
		running_agent_env_stream() { return 1; }
		ps() {
			printf '%s\n' "/usr/local/bin/pulse-agent -url http://192.168.2.96:7655 -token deadbeef -enable-host -enable-docker -insecure -agent-id firewall-1 -hostname 'edge firewall'"
		}

		if recover_connection_state_from_running_agent; then
			echo "READY"
		else
			echo "NOT_READY"
		fi
		build_exec_args
		printf 'URL=%s\nTOKEN=%s\nDOCKER=%s\nINSECURE=%s\nAGENT_ID=%s\nHOSTNAME=%s\nEXEC_ARGS=%s\n' \
			"$PULSE_URL" "$PULSE_TOKEN" "$ENABLE_DOCKER" "$INSECURE" "$AGENT_ID" "$HOSTNAME_OVERRIDE" "$EXEC_ARGS"
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	for _, needle := range []string{
		"READY",
		"URL=http://192.168.2.96:7655",
		"TOKEN=deadbeef",
		"DOCKER=true",
		"INSECURE=true",
		"AGENT_ID=firewall-1",
		"HOSTNAME=edge firewall",
		"--token-file /var/lib/pulse-agent/token",
	} {
		if !strings.Contains(got, needle) {
			t.Fatalf("non-procfs process recovery missing %q:\n%s", needle, got)
		}
	}
	if strings.Contains(got, "--token deadbeef") {
		t.Fatalf("non-procfs process recovery leaked raw token into service args:\n%s", got)
	}
}

func TestInstallSHRecoversV5FreeBSDRCServiceState(t *testing.T) {
	rcScript := filepath.Join(t.TempDir(), "pulse-agent")
	if err := os.WriteFile(rcScript, []byte(`#!/bin/sh
command="/usr/local/bin/pulse-agent"
command_args="-url http://192.168.2.96:7655 -token deadbeef -enable-host -insecure -agent-id firewall-1 -hostname 'edge firewall'"
export PULSE_CACERT='/conf/pulse-ca.pem'
`), 0600); err != nil {
		t.Fatalf("write rc.d fixture: %v", err)
	}

	script := `
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
		TRUENAS_STATE_DIR="/data/pulse-agent"
		CURL_CA_BUNDLE=""
		KUBE_INCLUDE_ALL_PODS="false"
		KUBE_INCLUDE_ALL_DEPLOYMENTS="false"
		DISK_EXCLUDES=()
		RUNTIME_TOKEN_FILE="/var/lib/pulse-agent/token"
		AGENT_NAME="pulse-agent"
` + extractInstallShellFunction(t, "strip_recovered_arg_quotes") + `
` + extractInstallShellFunction(t, "normalize_recovered_agent_arg_key") + `
` + extractInstallShellFunction(t, "apply_recovered_agent_arg_value") + `
` + extractInstallShellFunction(t, "recovered_connection_state_ready") + `
` + extractInstallShellFunction(t, "recover_token_from_default_agent_token_file") + `
` + extractInstallShellFunction(t, "recover_connection_state_from_arg_stream") + `
` + extractInstallShellFunction(t, "recover_connection_state_from_env_stream") + `
` + extractInstallShellFunction(t, "split_recovered_shell_words") + `
` + extractInstallShellFunction(t, "recover_connection_state_from_service_scripts") + `
` + extractInstallShellFunction(t, "build_exec_arg_items") + `
` + extractInstallShellFunction(t, "join_exec_arg_items") + `
` + extractInstallShellFunction(t, "build_exec_args") + `
		if recover_connection_state_from_service_scripts "${PULSE_TEST_RC_SCRIPT:?}"; then
			echo "READY"
		else
			echo "NOT_READY"
		fi
		build_exec_args
		printf 'URL=%s\nTOKEN=%s\nINSECURE=%s\nAGENT_ID=%s\nHOSTNAME=%s\nCACERT=%s\nEXEC_ARGS=%s\n' \
			"$PULSE_URL" "$PULSE_TOKEN" "$INSECURE" "$AGENT_ID" "$HOSTNAME_OVERRIDE" "$CURL_CA_BUNDLE" "$EXEC_ARGS"
	`

	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), "PULSE_TEST_RC_SCRIPT="+rcScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	for _, needle := range []string{
		"READY",
		"URL=http://192.168.2.96:7655",
		"TOKEN=deadbeef",
		"INSECURE=true",
		"AGENT_ID=firewall-1",
		"HOSTNAME=edge firewall",
		"CACERT=/conf/pulse-ca.pem",
		"--token-file /var/lib/pulse-agent/token",
	} {
		if !strings.Contains(got, needle) {
			t.Fatalf("FreeBSD rc.d recovery missing %q:\n%s", needle, got)
		}
	}
	if strings.Contains(got, "--token deadbeef") {
		t.Fatalf("FreeBSD rc.d recovery leaked raw token into service args:\n%s", got)
	}
}

func TestInstallSHRejectsPartialRecoveredProcessConnectionState(t *testing.T) {
	script := `
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
` + extractInstallShellFunction(t, "strip_recovered_arg_quotes") + `
` + extractInstallShellFunction(t, "normalize_recovered_agent_arg_key") + `
` + extractInstallShellFunction(t, "apply_recovered_agent_arg_value") + `
` + extractInstallShellFunction(t, "recovered_connection_state_ready") + `
` + extractInstallShellFunction(t, "recover_token_from_default_agent_token_file") + `
` + extractInstallShellFunction(t, "recover_connection_state_from_arg_stream") + `
		if recover_connection_state_from_arg_stream <<'ARGS'
/usr/local/bin/pulse-agent
--url
http://192.168.2.96:7655
--enable-host
--agent-id
agent-123
ARGS
		then
			echo "UNEXPECTED_SUCCESS"
		else
			echo "EXPECTED_FAILURE"
		fi
		printf 'URL=%s\nTOKEN=%s\nAGENT_ID=%s\n' "$PULSE_URL" "$PULSE_TOKEN" "$AGENT_ID"
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "EXPECTED_FAILURE") {
		t.Fatalf("partial recovered state should fail closed:\n%s", got)
	}
	if strings.Contains(got, "UNEXPECTED_SUCCESS") {
		t.Fatalf("partial recovered state reported success:\n%s", got)
	}
	if !strings.Contains(got, "URL=http://192.168.2.96:7655") || !strings.Contains(got, "AGENT_ID=agent-123") {
		t.Fatalf("partial recovery should still preserve non-secret fields for later sources:\n%s", got)
	}
	if !strings.Contains(got, "TOKEN=") {
		t.Fatalf("expected token to remain empty:\n%s", got)
	}
}

func TestInstallSHRecoversLegacyDefaultTokenFileForSavedStateUpdate(t *testing.T) {
	stateDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(stateDir, "token"), []byte("deadbeef\n"), 0600); err != nil {
		t.Fatalf("write legacy token file: %v", err)
	}

	script := `
		fail() { echo "FAIL:$1"; exit 99; }
		PULSE_URL="http://192.168.2.96:7655"
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
		INSECURE="true"
		ENABLE_COMMANDS="false"
		ENROLL="false"
		HEALTH_ADDR=""
		HEALTH_ADDR_SET="false"
		AGENT_ID=""
		HOSTNAME_OVERRIDE=""
		STATE_DIR="${PULSE_TEST_STATE_DIR:?}"
		TRUENAS_STATE_DIR="${PULSE_TEST_STATE_DIR:?}/truenas"
		CURL_CA_BUNDLE=""
		KUBE_INCLUDE_ALL_PODS="false"
		KUBE_INCLUDE_ALL_DEPLOYMENTS="false"
		DISK_EXCLUDES=()
		RUNTIME_TOKEN_FILE="${STATE_DIR}/token"
` + extractInstallShellFunction(t, "strip_recovered_arg_quotes") + `
` + extractInstallShellFunction(t, "normalize_recovered_agent_arg_key") + `
` + extractInstallShellFunction(t, "apply_recovered_agent_arg_value") + `
` + extractInstallShellFunction(t, "recovered_connection_state_ready") + `
` + extractInstallShellFunction(t, "recover_token_from_default_agent_token_file") + `
` + extractInstallShellFunction(t, "recover_connection_state_from_arg_stream") + `
` + extractInstallShellFunction(t, "build_exec_arg_items") + `
` + extractInstallShellFunction(t, "join_exec_arg_items") + `
` + extractInstallShellFunction(t, "build_exec_args") + `
		if recover_connection_state_from_arg_stream <<'ARGS'
/usr/local/bin/pulse-agent
-url
http://192.168.2.96:7655
-interval
30s
-enable-host
-enable-docker
-agent-id
machine-1
-hostname
docker1
ARGS
		then
			echo "READY"
		else
			echo "NOT_READY"
		fi
		build_exec_args
		printf 'URL=%s\nTOKEN=%s\nDOCKER=%s\nAGENT_ID=%s\nHOSTNAME=%s\nEXEC_ARGS=%s\n' \
			"$PULSE_URL" "$PULSE_TOKEN" "$ENABLE_DOCKER" "$AGENT_ID" "$HOSTNAME_OVERRIDE" "$EXEC_ARGS"
	`

	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), "PULSE_TEST_STATE_DIR="+stateDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	for _, needle := range []string{
		"READY",
		"URL=http://192.168.2.96:7655",
		"TOKEN=deadbeef",
		"DOCKER=true",
		"AGENT_ID=machine-1",
		"HOSTNAME=docker1",
		"--token-file",
		stateDir + "/token",
	} {
		if !strings.Contains(got, needle) {
			t.Fatalf("legacy default token recovery missing %q:\n%s", needle, got)
		}
	}
	if strings.Contains(got, "--token deadbeef") {
		t.Fatalf("legacy default token recovery leaked raw token into service args:\n%s", got)
	}
}

func TestInstallSHCombinesRecoveredProcessArgsAndEnvConnectionState(t *testing.T) {
	script := `
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
` + extractInstallShellFunction(t, "strip_recovered_arg_quotes") + `
` + extractInstallShellFunction(t, "normalize_recovered_agent_arg_key") + `
` + extractInstallShellFunction(t, "apply_recovered_agent_arg_value") + `
` + extractInstallShellFunction(t, "recovered_connection_state_ready") + `
` + extractInstallShellFunction(t, "recover_token_from_default_agent_token_file") + `
` + extractInstallShellFunction(t, "recover_connection_state_from_arg_stream") + `
` + extractInstallShellFunction(t, "recover_connection_state_from_env_stream") + `
		if recover_connection_state_from_arg_stream <<'ARGS'
/usr/local/bin/pulse-agent
--url
http://192.168.2.96:7655
ARGS
		then
			echo "ARGS_READY"
		else
			echo "ARGS_PARTIAL"
		fi
		if recover_connection_state_from_env_stream <<'ENV'
PULSE_TOKEN=deadbeef
ENV
		then
			echo "ENV_READY"
		else
			echo "ENV_PARTIAL"
		fi
		printf 'URL=%s\nTOKEN=%s\n' "$PULSE_URL" "$PULSE_TOKEN"
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	for _, needle := range []string{
		"ARGS_PARTIAL",
		"ENV_READY",
		"URL=http://192.168.2.96:7655",
		"TOKEN=deadbeef",
	} {
		if !strings.Contains(got, needle) {
			t.Fatalf("combined recovery missing %q:\n%s", needle, got)
		}
	}
}

func TestInstallSHUpdateModeMergesExplicitURLWithRunningV5ProcessState(t *testing.T) {
	script := `
		fail() { echo "FAIL:$1"; exit 99; }
		log_info() { :; }
		UPDATE_ONLY="true"
		PULSE_URL="http://192.168.2.96:7655"
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
		INSECURE="true"
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
` + extractInstallShellFunction(t, "normalize_recovered_agent_arg_key") + `
` + extractInstallShellFunction(t, "apply_recovered_agent_arg_value") + `
` + extractInstallShellFunction(t, "recovered_connection_state_ready") + `
` + extractInstallShellFunction(t, "update_connection_state_incomplete") + `
` + extractInstallShellFunction(t, "recover_token_from_default_agent_token_file") + `
` + extractInstallShellFunction(t, "recover_connection_state_from_arg_stream") + `
` + extractInstallShellFunction(t, "build_exec_arg_items") + `
` + extractInstallShellFunction(t, "join_exec_arg_items") + `
` + extractInstallShellFunction(t, "build_exec_args") + `
		find_connection_state_file() { return 1; }
		recover_agent_id_from_state_file() { return 1; }
		recover_connection_state_from_existing_agent() {
			recover_connection_state_from_arg_stream <<'ARGS'
/usr/local/bin/pulse-agent
-url
http://192.168.2.96:7655
-token
deadbeef
-interval
30s
-enable-host
-enable-docker
-agent-id
machine-1
-hostname
docker1
ARGS
		}

		if [[ "$UPDATE_ONLY" == "true" ]]; then
			if update_connection_state_incomplete; then
				update_conn_env=$(find_connection_state_file || true)
				if [[ -n "$update_conn_env" ]]; then
					recover_connection_state "$update_conn_env"
				fi
				if update_connection_state_incomplete; then
					recover_connection_state_from_existing_agent || true
				fi
				if [[ -n "$PULSE_URL" && -n "$PULSE_TOKEN" ]]; then
					echo "READY"
				elif [[ -z "$PULSE_URL" || -z "$PULSE_TOKEN" ]]; then
					fail "No existing Pulse Agent connection state found. Use the install command instead."
				fi
				if [[ -z "$AGENT_ID" ]]; then
					AGENT_ID=$(recover_agent_id_from_state_file || true)
				fi
			fi
		fi
		build_exec_args
		printf 'URL=%s\nTOKEN=%s\nDOCKER=%s\nDOCKER_EXPLICIT=%s\nAGENT_ID=%s\nHOSTNAME=%s\nEXEC_ARGS=%s\n' \
			"$PULSE_URL" "$PULSE_TOKEN" "$ENABLE_DOCKER" "$DOCKER_EXPLICIT" "$AGENT_ID" "$HOSTNAME_OVERRIDE" "$EXEC_ARGS"
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	required := []string{
		"READY",
		"URL=http://192.168.2.96:7655",
		"TOKEN=deadbeef",
		"DOCKER=true",
		"DOCKER_EXPLICIT=true",
		"AGENT_ID=machine-1",
		"HOSTNAME=docker1",
		"--token-file /var/lib/pulse-agent/token",
	}
	for _, needle := range required {
		if !strings.Contains(got, needle) {
			t.Fatalf("reported v5 update recovery missing %q:\n%s", needle, got)
		}
	}
	if strings.Contains(got, "--token deadbeef") {
		t.Fatalf("reported v5 update recovery leaked raw token into service args:\n%s", got)
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
		`teardown_freebsd_agent_service() {`,
		`teardown_sysv_agent_service() {`,
		`teardown_systemd_agent_service`,
		`teardown_freebsd_agent_service "/usr/local/etc/rc.d/${AGENT_NAME}"`,
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
		`select_platform_state_dir "${QNAP_VOL}/.pulse-agent"`,
		`write_qnap_wrapper_script "$WRAPPER_SCRIPT" "$RUNTIME_BINARY" "$QNAP_STORED_BINARY" "$QNAP_LOG_DIR" "$STATE_DIR"`,
		`append_qnap_autorun_block "$AUTORUN_PATH" "$WRAPPER_SCRIPT" "$STATE_DIR"`,
		`complete_installation_flow "$STATE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent is running." "tail -f ${AGENT_LOG_FILE}"`,
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

func TestInstallSHAgentDiskHeadroomRejectsSharedLowSpaceFilesystem(t *testing.T) {
	script := `
		set -euo pipefail
		log_error() { :; }
		log_info() { :; }
		log_warn() { :; }
		INSTALL_DIR="/usr/local/bin"
		AGENT_MIN_TEMP_FREE_BYTES=$((100 * 1024))
		AGENT_MIN_INSTALL_FREE_BYTES=$((80 * 1024))
` + extractInstallShellFunction(t, "bytes_to_human") + `
` + extractInstallShellFunction(t, "nearest_existing_dir") + `
` + extractInstallShellFunction(t, "get_available_bytes_for_path") + `
` + extractInstallShellFunction(t, "get_filesystem_device_for_path") + `
` + extractInstallShellFunction(t, "ensure_agent_disk_headroom") + `
		df() {
			if [[ "$1" == "-Pk" ]]; then
				printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\n'
				printf '/dev/shared 1000 0 150 0%% /\n'
				return 0
			fi
			command df "$@"
		}
		if ensure_agent_disk_headroom /tmp /usr/local/bin; then
			echo "ensure_agent_disk_headroom unexpectedly passed on a shared full filesystem" >&2
			exit 1
		fi
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
}

func TestInstallSHAgentDiskHeadroomAcceptsSeparateFilesystems(t *testing.T) {
	script := `
		set -euo pipefail
		log_error() { :; }
		log_info() { :; }
		log_warn() { :; }
		INSTALL_DIR="/usr/local/bin"
		AGENT_MIN_TEMP_FREE_BYTES=$((100 * 1024))
		AGENT_MIN_INSTALL_FREE_BYTES=$((80 * 1024))
` + extractInstallShellFunction(t, "bytes_to_human") + `
` + extractInstallShellFunction(t, "nearest_existing_dir") + `
` + extractInstallShellFunction(t, "get_available_bytes_for_path") + `
` + extractInstallShellFunction(t, "get_filesystem_device_for_path") + `
` + extractInstallShellFunction(t, "ensure_agent_disk_headroom") + `
		df() {
			if [[ "$1" == "-Pk" ]]; then
				case "$2" in
					/tmp)
						printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\n'
						printf '/dev/tmp 1000 0 120 0%% /tmp\n'
						return 0
						;;
					*)
						printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\n'
						printf '/dev/root 1000 0 90 0%% /\n'
						return 0
						;;
				esac
			fi
			command df "$@"
		}
		ensure_agent_disk_headroom /tmp /usr/local/bin
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
}

func TestInstallSHAgentDiskHeadroomResolvesMissingDirectories(t *testing.T) {
	script := `
		set -euo pipefail
		log_error() { :; }
		log_info() { :; }
		log_warn() { :; }
		INSTALL_DIR="/usr/local/bin"
		AGENT_MIN_TEMP_FREE_BYTES=$((100 * 1024))
		AGENT_MIN_INSTALL_FREE_BYTES=$((80 * 1024))
` + extractInstallShellFunction(t, "nearest_existing_dir") + `
		resolved=$(nearest_existing_dir /definitely/not/a/real/path/anywhere)
		if [[ "$resolved" != "/" ]]; then
			echo "nearest_existing_dir resolved to $resolved, want /" >&2
			exit 1
		fi
		resolved=$(nearest_existing_dir /tmp)
		if [[ "$resolved" != "/tmp" ]]; then
			echo "nearest_existing_dir resolved to $resolved, want /tmp" >&2
			exit 1
		fi
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
}

// TestInstallSHChecksDiskHeadroomBeforeDownload pins the ENOSPC preflight from
// issue #1617: the installer must verify temp and install-dir headroom before
// the agent binary download stages anything, and the --preflight-only mode must
// report the same check.
func TestInstallSHChecksDiskHeadroomBeforeDownload(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	headroomCall := `if ! ensure_agent_disk_headroom "${TMPDIR:-/tmp}" "$INSTALL_DIR"; then`
	stagingCall := `TMP_BIN=$(mktemp)`

	headroomPos := strings.Index(script, headroomCall)
	if headroomPos < 0 {
		t.Fatalf("install.sh does not gate the download on ensure_agent_disk_headroom")
	}
	stagingPos := strings.Index(script, stagingCall)
	if stagingPos < 0 {
		t.Fatalf("install.sh no longer stages the download via mktemp; update this test")
	}
	if headroomPos > stagingPos {
		t.Fatalf("disk headroom check happens after the download staging mktemp")
	}

	for _, needle := range []string{
		`json_event "preflight" "disk_ok" "Sufficient disk space for agent install"`,
		`json_event "preflight" "disk_low"`,
	} {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh preflight-only mode missing disk headroom reporting: %s", needle)
		}
	}
}

func TestInstallSHSetsAgentBinaryModeExplicitly(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	for _, want := range []string{
		`chmod 0755 "$TMP_BIN"`,
		`chmod 0755 "${INSTALL_DIR}/${BINARY_NAME}"`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("install.sh does not pin executable mode with %q", want)
		}
	}
}

// TestInstallSHWatchdogPathsUseRotatingAgentLog pins the logging half of issue
// #1617: the QNAP and Unraid watchdog loops must not shell-append agent stdout
// to an unrotated file on the RAM-backed root; the agent's own rotating writer
// (--log-file) owns the agent log instead.
func TestInstallSHWatchdogPathsUseRotatingAgentLog(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`if [[ -n "${AGENT_LOG_FILE:-}" ]]; then EXEC_ARG_ITEMS+=(--log-file "$AGENT_LOG_FILE"); fi`,
		`QNAP_LOG_DIR="${STATE_DIR}/logs"`,
		`AGENT_LOG_FILE="${QNAP_LOG_DIR}/${AGENT_NAME}.log"`,
		`UNRAID_LOG_DIR="/var/log/${AGENT_NAME}"`,
		`AGENT_LOG_FILE="${UNRAID_LOG_DIR}/${AGENT_NAME}.log"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing rotating agent log wiring: %s", needle)
		}
	}

	if got := strings.Count(script, `${EXEC_ARGS} > /dev/null 2>> "\$WATCHDOG_LOG"`); got != 2 {
		t.Fatalf("expected both watchdog loops (QNAP, Unraid) to discard the stdout mirror and capture stderr in the watchdog log, found %d", got)
	}
	if strings.Contains(script, `${EXEC_ARGS} >> /var/log/${AGENT_NAME}.log`) {
		t.Fatalf("a watchdog loop still shell-appends agent output to the unrotated /var/log/${AGENT_NAME}.log")
	}
}

// TestInstallSHQNAPWatchdogIsSingleton pins the process-ownership half of issue
// #1617. QNAP can launch the persistent wrapper both from autorun.sh and from an
// install/upgrade, so the wrapper must elect one supervisor before killing or
// starting an agent and must clean up only state that it owns.
func TestInstallSHQNAPWatchdogIsSingleton(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`WATCHDOG_PIDFILE="${state_dir}/${AGENT_NAME}.watchdog.pid"`,
		`AGENT_PIDFILE="${state_dir}/${AGENT_NAME}.pid"`,
		`LOCK_DIR="${state_dir}/${AGENT_NAME}.watchdog.lock"`,
		`while ! mkdir "\$LOCK_DIR" 2>/dev/null; do`,
		`if pid_is_live "\$_owner"; then`,
		`if ! acquire_watchdog_lock; then`,
		`[ "\$(cat "\$WATCHDOG_PIDFILE" 2>/dev/null)" = "\$\$" ]`,
		`trap cleanup_watchdog EXIT`,
		`trap shutdown_watchdog INT TERM HUP`,
		`CURRENT_AGENT_PID=\$!`,
		`echo "\$CURRENT_AGENT_PID" > "\$AGENT_PIDFILE"`,
		`wait "\$CURRENT_AGENT_PID"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing QNAP watchdog singleton contract: %s", needle)
		}
	}

	lockPos := strings.Index(script, `if ! acquire_watchdog_lock; then`)
	killPos := strings.Index(script, `pkill -x "pulse-agent" 2>/dev/null || true`)
	if lockPos < 0 || killPos < 0 || lockPos > killPos {
		t.Fatalf("QNAP wrapper must acquire singleton ownership before killing an existing agent")
	}
}

func TestRenderedQNAPWatchdogRejectsASecondSupervisor(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	start := strings.Index(script, "write_qnap_wrapper_script() {")
	if start < 0 {
		t.Fatal("install.sh missing QNAP wrapper renderer")
	}
	endOffset := strings.Index(script[start:], "\nappend_qnap_autorun_block() {")
	if endOffset < 0 {
		t.Fatal("could not isolate QNAP wrapper renderer")
	}
	renderer := script[start : start+endOffset]

	tempDir := t.TempDir()
	stateDir := filepath.Join(tempDir, "state")
	logDir := filepath.Join(stateDir, "logs")
	wrapperPath := filepath.Join(stateDir, "start-pulse-agent.sh")
	storedPath := filepath.Join(stateDir, "stored-agent")
	runtimePath := filepath.Join(tempDir, "runtime", "pulse-agent")
	startsPath := filepath.Join(tempDir, "agent-starts")
	mockBinDir := filepath.Join(tempDir, "bin")

	for _, dir := range []string{stateDir, mockBinDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create test directory: %v", err)
		}
	}
	agentScript := "#!/bin/sh\n" +
		"echo \"$$\" >> " + strconv.Quote(startsPath) + "\n" +
		"trap 'exit 0' INT TERM HUP\n" +
		"while :; do sleep 1; done\n"
	if err := os.WriteFile(storedPath, []byte(agentScript), 0o755); err != nil {
		t.Fatalf("write fake agent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mockBinDir, "pkill"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake pkill: %v", err)
	}

	harness := renderer + `
AGENT_NAME=pulse-agent
SHELL_EXPORT_LINES=""
EXEC_ARGS=""
write_qnap_wrapper_script "$1" "$2" "$3" "$4" "$5"
`
	render := exec.Command("bash", "-c", harness, "_", wrapperPath, runtimePath, storedPath, logDir, stateDir)
	if output, err := render.CombinedOutput(); err != nil {
		t.Fatalf("render QNAP wrapper: %v\n%s", err, output)
	}

	env := []string{"PATH=" + mockBinDir + ":/usr/bin:/bin"}
	first := exec.Command("sh", wrapperPath)
	first.Env = env
	if err := first.Start(); err != nil {
		t.Fatalf("start first watchdog: %v", err)
	}
	t.Cleanup(func() {
		if first.Process != nil {
			_ = first.Process.Signal(syscall.SIGTERM)
			_, _ = first.Process.Wait()
		}
	})

	watchdogPIDFile := filepath.Join(stateDir, "pulse-agent.watchdog.pid")
	agentPIDFile := filepath.Join(stateDir, "pulse-agent.pid")
	waitForFile := func(path string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(path); err == nil {
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
		t.Fatalf("timed out waiting for %s", path)
	}
	waitForFile(watchdogPIDFile)
	waitForFile(agentPIDFile)
	waitForFile(startsPath)

	second := exec.Command("sh", wrapperPath)
	second.Env = env
	if output, err := second.CombinedOutput(); err != nil {
		t.Fatalf("second watchdog should exit cleanly: %v\n%s", err, output)
	}

	starts, err := os.ReadFile(startsPath)
	if err != nil {
		t.Fatalf("read fake-agent starts: %v", err)
	}
	if got := len(strings.Fields(string(starts))); got != 1 {
		t.Fatalf("expected one supervised agent after two watchdog launches, got %d", got)
	}

	agentPIDBytes, err := os.ReadFile(agentPIDFile)
	if err != nil {
		t.Fatalf("read agent pid: %v", err)
	}
	agentPID, err := strconv.Atoi(strings.TrimSpace(string(agentPIDBytes)))
	if err != nil {
		t.Fatalf("parse agent pid: %v", err)
	}

	if err := first.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("stop first watchdog: %v", err)
	}
	if err := first.Wait(); err != nil {
		t.Fatalf("wait for first watchdog shutdown: %v", err)
	}
	first.Process = nil

	if err := exec.Command("kill", "-0", strconv.Itoa(agentPID)).Run(); err == nil {
		t.Fatalf("watchdog shutdown left agent pid %d alive", agentPID)
	}
	for _, path := range []string{watchdogPIDFile, agentPIDFile, filepath.Join(stateDir, "pulse-agent.watchdog.lock")} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("watchdog shutdown left singleton state %s (stat: %v)", path, err)
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
		// The Linux systemd unit takes the resolved service user so the
		// least-privilege profile can swap root for pulse-agent.
		`render_systemd_agent_unit "$UNIT" "${INSTALL_DIR}/${BINARY_NAME}" "${EXEC_ARGS}" "network-online.target docker.service" "network-online.target" "$SERVICE_USER" ""`,
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
		`system_docker_runtime_is_active() {`,
		`discover_rootless_container_runtime() {`,
		`discover_safe_profile_rootless_container_runtime() {`,
		`recover_safe_profile_rootless_runtime_pin() {`,
		`resolve_initial_container_monitoring_detection() {`,
		`resolve_safe_profile_container_runtime() {`,
		`configure_discovered_rootless_runtime_environment() {`,
		// Issue #1647: a live rootful Docker must win before any rootless
		// socket discovery pins podman variables into the service environment.
		`if system_docker_runtime_is_active; then`,
		`discover_single_socket_match "${rootless_root}/*/docker.sock"`,
		`discover_single_socket_match "${rootless_root}/*/podman/podman.sock"`,
		`append_service_env "DOCKER_HOST" "$ROOTLESS_RUNTIME_SOCKET_URI"`,
		`append_service_env "PULSE_DOCKER_RUNTIME" "podman"`,
		`append_service_env "CONTAINER_HOST" "$ROOTLESS_RUNTIME_SOCKET_URI"`,
		`append_service_env "PODMAN_HOST" "$ROOTLESS_RUNTIME_SOCKET_URI"`,
		`append_service_env "XDG_RUNTIME_DIR" "$ROOTLESS_RUNTIME_XDG_DIR"`,
		`Multiple collector-owned rootless container-runtime sockets are usable`,
		`provision_least_privilege_user`,
		`discover_safe_profile_rootless_container_runtime`,
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

func TestInstallSHSafeProfileRootlessDiscoveryIsCollectorOwnedAndUnambiguous(t *testing.T) {
	tmpDir, err := os.MkdirTemp("/tmp", "pulse-safe-rootless-*")
	if err != nil {
		t.Fatalf("mktemp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dockerDir := filepath.Join(tmpDir, "run", "user", "991")
	podmanDir := filepath.Join(tmpDir, "run", "user", "991", "podman")
	if err := os.MkdirAll(dockerDir, 0o755); err != nil {
		t.Fatalf("mkdir Docker runtime dir: %v", err)
	}
	if err := os.MkdirAll(podmanDir, 0o755); err != nil {
		t.Fatalf("mkdir Podman runtime dir: %v", err)
	}
	dockerPath := filepath.Join(dockerDir, "docker.sock")
	dockerListener, err := net.Listen("unix", dockerPath)
	if err != nil {
		t.Fatalf("listen Docker socket: %v", err)
	}
	defer dockerListener.Close()
	podmanPath := filepath.Join(podmanDir, "podman.sock")
	podmanListener, err := net.Listen("unix", podmanPath)
	if err != nil {
		t.Fatalf("listen Podman socket: %v", err)
	}
	defer podmanListener.Close()

	common := `
		set -euo pipefail
		LEAST_PRIVILEGE_USER=pulse-agent
		PULSE_ROOTLESS_RUNTIME_ROOT="` + filepath.Join(tmpDir, "run", "user") + `"
		log_warn() { :; }
		uname() { echo Linux; }
		id() { [[ "$1" == "-u" && "$2" == "pulse-agent" ]] && echo 991; }
		runuser() { return 0; }
` + extractInstallShellFunction(t, "discover_safe_profile_rootless_container_runtime") + `
`

	oneOwned := common + `
		stat() {
			[[ "$2" == "%a" ]] && { echo 600; return; }
			case "$3" in
				"` + dockerPath + `") echo 991 ;;
				"` + podmanPath + `") echo 1000 ;;
				*) return 1 ;;
			esac
		}
		discover_safe_profile_rootless_container_runtime
		[[ "$ROOTLESS_RUNTIME_KIND" == docker ]]
		[[ "$ROOTLESS_RUNTIME_SOCKET_PATH" == "` + dockerPath + `" ]]
	`
	if out, err := exec.Command("bash", "-c", oneOwned).CombinedOutput(); err != nil {
		t.Fatalf("single collector-owned socket: %v\n%s", err, out)
	}

	ambiguous := common + `
		stat() { [[ "$2" == "%a" ]] && echo 600 || echo 991; }
		if discover_safe_profile_rootless_container_runtime; then
			echo "ambiguous sockets were accepted" >&2
			exit 1
		fi
		[[ -z "$ROOTLESS_RUNTIME_SOCKET_PATH" ]]
	`
	if out, err := exec.Command("bash", "-c", ambiguous).CombinedOutput(); err != nil {
		t.Fatalf("ambiguous collector-owned sockets: %v\n%s", err, out)
	}

	crossUser := common + `
		stat() { echo 1000; }
		if discover_safe_profile_rootless_container_runtime; then
			echo "cross-user sockets were accepted" >&2
			exit 1
		fi
	`
	if out, err := exec.Command("bash", "-c", crossUser).CombinedOutput(); err != nil {
		t.Fatalf("cross-user sockets: %v\n%s", err, out)
	}

	unreadable := common + `
		stat() {
			[[ "$2" == "%a" ]] && { echo 600; return; }
			case "$3" in
				"` + dockerPath + `") echo 991 ;;
				*) echo 1000 ;;
			esac
		}
		runuser() { return 1; }
		if discover_safe_profile_rootless_container_runtime; then
			echo "collector-unreadable socket was accepted" >&2
			exit 1
		fi
	`
	if out, err := exec.Command("bash", "-c", unreadable).CombinedOutput(); err != nil {
		t.Fatalf("collector-unreadable socket: %v\n%s", err, out)
	}
}

func TestInstallSHSafeProfileRejectsSymlinkedRootlessSocket(t *testing.T) {
	tmpDir, err := os.MkdirTemp("/tmp", "psr-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	runtimeRoot := filepath.Join(tmpDir, "run", "user")
	runtimeDir := filepath.Join(runtimeRoot, "991")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	realPath := filepath.Join(runtimeDir, "real.sock")
	listener, err := net.Listen("unix", realPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := os.Symlink(realPath, filepath.Join(runtimeDir, "docker.sock")); err != nil {
		t.Fatal(err)
	}

	script := `
		set -euo pipefail
		LEAST_PRIVILEGE_USER=pulse-agent
		PULSE_ROOTLESS_RUNTIME_ROOT="` + runtimeRoot + `"
		log_warn() { :; }
		uname() { echo Linux; }
		id() { [[ "$1" == -u && "$2" == pulse-agent ]] && echo 991; }
		stat() { echo 991; }
		runuser() { return 0; }
` + extractInstallShellFunction(t, "discover_safe_profile_rootless_container_runtime") + `
		if discover_safe_profile_rootless_container_runtime; then
			echo "symlinked runtime socket was accepted" >&2
			exit 1
		fi
	`
	if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("symlinked safe rootless socket: %v\n%s", err, out)
	}
}

func TestInstallSHSafeProfileDefersDetectionWithoutDaemonProbe(t *testing.T) {
	script := `
		set -euo pipefail
		DOCKER_EXPLICIT=false
		LEAST_PRIVILEGE=true
		PRIVILEGED_HELPER_ENABLED=true
		SAFE_PROFILE_DOCKER_DETECTION_DEFERRED=false
		ENABLE_DOCKER=true
		probe_log="${TMPDIR}/probe-log"
		log_info() { :; }
		detect_docker() { echo daemon-probe >> "$probe_log"; return 0; }
` + extractInstallShellFunction(t, "resolve_initial_container_monitoring_detection") + `
		resolve_initial_container_monitoring_detection
		[[ "$SAFE_PROFILE_DOCKER_DETECTION_DEFERRED" == true ]]
		[[ ! -e "$probe_log" ]]
	`
	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), "TMPDIR="+t.TempDir())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("deferred safe detection: %v\n%s", err, out)
	}
}

func TestInstallSHSafeProfileResolvesRuntimeOnlyAfterCollectorProvisioning(t *testing.T) {
	script := `
		set -euo pipefail
		SAFE_PROFILE_DOCKER_DETECTION_DEFERRED=true
		ENABLE_DOCKER=false
		configured=false
		log_info() { :; }
		recover_safe_profile_rootless_runtime_pin() { return 1; }
		discover_safe_profile_rootless_container_runtime() {
			ROOTLESS_RUNTIME_KIND=docker
			ROOTLESS_RUNTIME_SOCKET_PATH=/run/user/991/docker.sock
			ROOTLESS_RUNTIME_SOCKET_URI=unix:///run/user/991/docker.sock
			ROOTLESS_RUNTIME_XDG_DIR=/run/user/991
			return 0
		}
		safe_profile_fixed_container_runtime_socket_present() { echo unexpected-fixed-probe >&2; return 1; }
		configure_discovered_rootless_runtime_environment() { configured=true; }
		safe_profile_apply_docker_degradation() { :; }
` + extractInstallShellFunction(t, "resolve_safe_profile_container_runtime") + `
		resolve_safe_profile_container_runtime
		[[ "$ENABLE_DOCKER" == true && "$configured" == true ]]
	`
	if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("post-provision safe runtime resolution: %v\n%s", err, out)
	}
}

func TestInstallSHRecoversExactRootlessPinWhileSocketIsOffline(t *testing.T) {
	tmpDir := t.TempDir()
	unitPath := filepath.Join(tmpDir, "pulse-agent.service")
	rootlessRoot := filepath.Join(tmpDir, "run", "user")
	expectedPath := filepath.Join(rootlessRoot, "991", "docker.sock")
	unit := "[Service]\nUser=pulse-agent\nEnvironment=\"DOCKER_HOST=unix://" + expectedPath + "\"\n"
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		t.Fatal(err)
	}

	script := `
		set -euo pipefail
		AGENT_NAME=pulse-agent
		LEAST_PRIVILEGE_USER=pulse-agent
		PULSE_AGENT_SYSTEMD_UNIT_PATH="` + unitPath + `"
		PULSE_ROOTLESS_RUNTIME_ROOT="` + rootlessRoot + `"
		ROOTLESS_RUNTIME_KIND=""
		ROOTLESS_RUNTIME_SOCKET_PATH=""
		ROOTLESS_RUNTIME_SOCKET_URI=""
		ROOTLESS_RUNTIME_XDG_DIR=""
		id() { [[ "$1" == -u && "$2" == pulse-agent ]] && echo 991; }
		stat() {
			case "$2" in
				%u) echo 0 ;;
				%a) echo 644 ;;
				*) return 1 ;;
			esac
		}
` + extractInstallShellFunction(t, "recover_safe_profile_rootless_runtime_pin") + `
		if [[ "${EXPECT_REJECT:-false}" == true ]]; then
			if recover_safe_profile_rootless_runtime_pin; then exit 9; fi
			exit 0
		fi
		recover_safe_profile_rootless_runtime_pin
		[[ "$ROOTLESS_RUNTIME_KIND" == docker ]]
		[[ "$ROOTLESS_RUNTIME_SOCKET_PATH" == "` + expectedPath + `" ]]
		[[ "$ROOTLESS_RUNTIME_SOCKET_URI" == "unix://` + expectedPath + `" ]]
		[[ ! -e "$ROOTLESS_RUNTIME_SOCKET_PATH" ]]
	`
	if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("recover offline rootless pin: %v\n%s", err, out)
	}

	conflicting := strings.Replace(unit, "\n", "\nEnvironment=\"PODMAN_HOST=unix://"+filepath.Join(rootlessRoot, "991", "podman", "podman.sock")+"\"\n", 1)
	if err := os.WriteFile(unitPath, []byte(conflicting), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), "EXPECT_REJECT=true")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("conflicting recovered pin was not rejected: %v\n%s", err, out)
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
		append_service_env "PULSE_DOCKER_RUNTIME" "podman"
		append_service_env "CONTAINER_HOST" "unix:///run/user/1000/podman/podman.sock"
		append_service_env "PODMAN_HOST" "unix:///run/user/1000/podman/podman.sock"
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
		`Environment="PULSE_DOCKER_RUNTIME=podman"`,
		`Environment="CONTAINER_HOST=unix:///run/user/1000/podman/podman.sock"`,
		`Environment="PODMAN_HOST=unix:///run/user/1000/podman/podman.sock"`,
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

// TestInstallSHIssue1647RootfulDockerOutranksRootlessPodmanSocket pins the fix
// for issue #1647: a transient rootless Podman API socket (socket-activated for
// a login session) must not win runtime discovery while a rootful Docker daemon
// is alive, or the agent service gets podman variables pinned to a socket that
// disappears when the session ends.
func TestInstallSHIssue1647RootfulDockerOutranksRootlessPodmanSocket(t *testing.T) {
	tmpDir, err := os.MkdirTemp("/tmp", "pulse-1647-*")
	if err != nil {
		t.Fatalf("mktemp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	podmanDir := filepath.Join(tmpDir, "run", "user", "1000", "podman")
	if err := os.MkdirAll(podmanDir, 0o755); err != nil {
		t.Fatalf("mkdir podman socket dir: %v", err)
	}
	socketPath := filepath.Join(podmanDir, "podman.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	defer listener.Close()

	common := `
		set -uo pipefail
		log_warn() { :; }
		log_info() { :; }
		uname() { echo Linux; }
		PULSE_ROOTLESS_RUNTIME_ROOT="` + filepath.Join(tmpDir, "run", "user") + `"
		PULSE_SYSTEM_DOCKER_SOCKET="` + filepath.Join(tmpDir, "absent", "docker.sock") + `"
` + extractInstallShellFunction(t, "system_docker_runtime_is_active") + `
` + extractInstallShellFunction(t, "discover_single_socket_match") + `
` + extractInstallShellFunction(t, "discover_rootless_container_runtime") + `
	`

	liveDocker := common + `
		docker() { return 0; }
		if discover_rootless_container_runtime; then
			echo "rootless podman socket must not outrank a live rootful docker" >&2
			exit 1
		fi
	`
	out, err := exec.Command("bash", "-c", liveDocker).CombinedOutput()
	if err != nil {
		t.Fatalf("live rootful docker case: %v\n%s", err, out)
	}

	deadDocker := common + `
		docker() { return 1; }
		if ! discover_rootless_container_runtime; then
			echo "expected rootless podman discovery when no rootful docker answers" >&2
			exit 1
		fi
		if [[ "$ROOTLESS_RUNTIME_KIND" != "podman" ]]; then
			echo "kind=$ROOTLESS_RUNTIME_KIND, want podman" >&2
			exit 1
		fi
		if [[ "$ROOTLESS_RUNTIME_SOCKET_URI" != "unix://` + socketPath + `" ]]; then
			echo "uri=$ROOTLESS_RUNTIME_SOCKET_URI" >&2
			exit 1
		fi
	`
	out, err = exec.Command("bash", "-c", deadDocker).CombinedOutput()
	if err != nil {
		t.Fatalf("no rootful docker case: %v\n%s", err, out)
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
		`complete_installation_flow "$UNRAID_STORAGE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent is running." "tail -f ${AGENT_LOG_FILE}"`,
		`complete_installation_flow "$STATE_DIR" "Installation complete! Agent is running." "Upgrade complete! Agent is running." "tail -f ${AGENT_LOG_FILE}"`,
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

func TestInstallSHUsesCanonicalFreeBSDAgentTeardown(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`teardown_freebsd_agent_service() {`,
		`"$service_path" stop 2>/dev/null || true`,
		`sysrc -x pulse_agent_enable >/dev/null 2>&1 || true`,
		`rm -f /usr/local/etc/rc.d/pulse_agent.sh`,
		`rm -f /var/run/pulse_agent.pid /var/run/pulse_agent.child.pid`,
		`log_info "Removing FreeBSD rc.d installation..."`,
		`teardown_freebsd_agent_service "/usr/local/etc/rc.d/${AGENT_NAME}"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing canonical FreeBSD teardown contract: %s", needle)
		}
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

func extractCollectorLifecycleShellFunctions(t *testing.T, includeVerify bool) string {
	t.Helper()
	functions := extractInstallShellFunction(t, "collector_lifecycle_binary") + "\n" +
		extractInstallShellFunction(t, "prepare_collector_lifecycle_token_file") + "\n" +
		extractInstallShellFunction(t, "run_collector_lifecycle_command")
	if includeVerify {
		functions += "\n" + extractInstallShellFunction(t, "verify_agent_server_registration")
	}
	return functions
}

func buildPulseAgentLifecycleBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "pulse-agent")
	build := exec.Command("go", "build", "-o", binary, "./cmd/pulse-agent")
	build.Dir = filepath.Dir(repoFile("go.mod"))
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build pulse-agent lifecycle binary: %v\n%s", err, output)
	}
	return binary
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
		extractInstallAutoUpdateAssetsShellFunctions(t) + "\n" +
		extractRootInstallShellFunction(t, "setup_auto_updates")
}

// install_auto_update_assets delegates its writability probe, its staged-helper
// sanity check and the sandbox-escape migration to sibling functions, so every
// harness that executes it has to pull those in alongside it.
func extractInstallAutoUpdateAssetsShellFunctions(t *testing.T) string {
	t.Helper()

	return extractRootInstallShellFunction(t, "auto_update_dir_writable") + "\n" +
		extractRootInstallShellFunction(t, "auto_update_helper_is_sane") + "\n" +
		extractRootInstallShellFunction(t, "migrate_auto_update_assets_outside_sandbox") + "\n" +
		extractRootInstallShellFunction(t, "install_auto_update_assets")
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

func TestInstallSHCustomStateDirOwnsTokenAndEnrollmentContinuity(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "custom-state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	tokenPath := filepath.Join(stateDir, "token")
	runtimePath := filepath.Join(stateDir, "runtime.token")
	if err := os.WriteFile(tokenPath, []byte("aaa111"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimePath, []byte("runtime-one"), 0600); err != nil {
		t.Fatal(err)
	}

	ensureToken := extractInstallShellFunction(t, "ensure_runtime_token_file")
	run := func(token string) string {
		t.Helper()
		script := `
			set -euo pipefail
			PULSE_TOKEN="` + token + `"
			STATE_DIR="` + stateDir + `"
			ENROLL="true"
			RUNTIME_TOKEN_FILE=""
			RUNTIME_TOKEN_CHANGED="false"
			log_info() { :; }
` + ensureToken + `
			ensure_runtime_token_file "$STATE_DIR"
			printf 'changed=%s runtime=%s token_file=%s\n' \
				"$RUNTIME_TOKEN_CHANGED" "$([[ -f "$STATE_DIR/runtime.token" ]] && echo present || echo missing)" "$RUNTIME_TOKEN_FILE"
		`
		out, err := exec.Command("bash", "-c", script).CombinedOutput()
		if err != nil {
			t.Fatalf("bash: %v\n%s", err, out)
		}
		return string(out)
	}

	if got := run("aaa111"); !strings.Contains(got, "changed=false runtime=present") {
		t.Fatalf("unchanged bootstrap token did not preserve enrollment runtime state:\n%s", got)
	}
	if got := run("bbb222"); !strings.Contains(got, "changed=true runtime=missing") {
		t.Fatalf("changed bootstrap token did not force re-enrollment:\n%s", got)
	}

	for path, want := range map[string]os.FileMode{
		stateDir:  0700,
		tokenPath: 0600,
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Fatalf("%s mode = %o, want %o", path, got, want)
		}
	}
}

func TestInstallSHExplicitCustomStateNeverFallsBackToDefaultInstance(t *testing.T) {
	root := t.TempDir()
	customState := filepath.Join(root, "custom")
	defaultState := filepath.Join(root, "default")
	for _, dir := range []string{customState, defaultState} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	customConn := filepath.Join(customState, "connection.env")
	if err := os.WriteFile(customConn, []byte("PULSE_URL='https://custom.example'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(defaultState, "connection.env"), []byte("PULSE_URL='https://default.example'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(defaultState, "agent-id"), []byte("default-agent"), 0600); err != nil {
		t.Fatal(err)
	}

	script := `
		set -euo pipefail
		STATE_DIR="` + customState + `"
		STATE_DIR_SOURCE="explicit"
		DEFAULT_STATE_DIR="` + defaultState + `"
		TRUENAS_STATE_DIR="` + filepath.Join(root, "truenas") + `"
` + extractInstallShellFunction(t, "find_connection_state_file") + `
` + extractInstallShellFunction(t, "recover_agent_id_from_state_file") + `
		printf 'connection=%s\n' "$(find_connection_state_file)"
		rm -f "$STATE_DIR/connection.env"
		printf 'fallback_connection=%s\n' "$(find_connection_state_file || true)"
		printf 'fallback_agent=%s\n' "$(recover_agent_id_from_state_file || true)"
	`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "connection="+customConn) {
		t.Fatalf("custom connection state was not preferred:\n%s", got)
	}
	if !strings.Contains(got, "fallback_connection=\n") || !strings.Contains(got, "fallback_agent=\n") {
		t.Fatalf("explicit custom state borrowed default instance state:\n%s", got)
	}
}

func TestInstallSHSavedInstallerDiscoversItsCustomStateDir(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "saved custom state")
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		t.Fatal(err)
	}
	installerPath := filepath.Join(stateDir, "install.sh")
	if err := os.WriteFile(installerPath, []byte("#!/usr/bin/env bash\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "connection.env"), []byte("PULSE_URL='https://pulse.example'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	script := `
		set -euo pipefail
		STATE_DIR="/var/lib/pulse-agent"
		STATE_DIR_SOURCE="default"
` + extractInstallShellFunction(t, "discover_state_dir_from_saved_installer") + `
		discover_state_dir_from_saved_installer "` + installerPath + `"
		printf 'state=%s source=%s\n' "$STATE_DIR" "$STATE_DIR_SOURCE"
	`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	resolvedStateDir, err := filepath.EvalSymlinks(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "state="+resolvedStateDir) || !strings.Contains(got, "source=recovered") {
		t.Fatalf("saved installer did not discover adjacent custom state:\n%s", got)
	}
}

func TestInstallSHDiscoversCustomStateDirFromGeneratedSystemdUnit(t *testing.T) {
	for _, commandsEnabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "commands_disabled", true: "commands_enabled"}[commandsEnabled], func(t *testing.T) {
			root := t.TempDir()
			customState := filepath.Join(root, "custom state")
			defaultState := filepath.Join(root, "default")
			if err := os.MkdirAll(customState, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(defaultState, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(customState, "token"), []byte("custom123"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(defaultState, "token"), []byte("default999"), 0600); err != nil {
				t.Fatal(err)
			}
			commandArg := ""
			if commandsEnabled {
				commandArg = " --enable-commands"
			}
			unitPath := filepath.Join(root, "pulse-agent.service")
			unit := `[Service]
ExecStart=/usr/local/bin/pulse-agent --url https://custom.example --token-file ` +
				strings.ReplaceAll(filepath.Join(customState, "token"), " ", `\ `) +
				` --state-dir ` + strings.ReplaceAll(customState, " ", `\ `) + commandArg + "\n"
			if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
				t.Fatal(err)
			}

			script := `
				set -euo pipefail
				AGENT_NAME="pulse-agent"
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
				STATE_DIR="` + defaultState + `"
				STATE_DIR_SOURCE="default"
				DEFAULT_STATE_DIR="` + defaultState + `"
				TRUENAS_STATE_DIR="` + filepath.Join(root, "truenas") + `"
				CURL_CA_BUNDLE=""
				SERVER_FINGERPRINT=""
				OBSERVERS_FILE=""
				KUBE_INCLUDE_ALL_PODS="false"
				KUBE_INCLUDE_ALL_DEPLOYMENTS="false"
				DISK_EXCLUDES=()
				log_warn() { :; }
				systemctl() { printf '%s\n' "` + unitPath + `"; }
` + extractInstallShellFunction(t, "strip_recovered_arg_quotes") + `
` + extractInstallShellFunction(t, "normalize_recovered_agent_arg_key") + `
` + extractInstallShellFunction(t, "apply_recovered_agent_arg_value") + `
` + extractInstallShellFunction(t, "recovered_connection_state_ready") + `
` + extractInstallShellFunction(t, "recover_token_from_default_agent_token_file") + `
` + extractInstallShellFunction(t, "recover_connection_state_from_arg_stream") + `
` + extractInstallShellFunction(t, "recover_connection_state_from_env_stream") + `
` + extractInstallShellFunction(t, "split_recovered_shell_words") + `
` + extractInstallShellFunction(t, "recover_connection_state_from_systemd_unit") + `
` + extractInstallShellFunction(t, "remove_agent_state_dir") + `
				recover_connection_state_from_systemd_unit
				printf 'state=%s source=%s url=%s token=%s commands=%s\n' \
					"$STATE_DIR" "$STATE_DIR_SOURCE" "$PULSE_URL" "$PULSE_TOKEN" "$ENABLE_COMMANDS"
				remove_agent_state_dir "$STATE_DIR"
			`
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if err != nil {
				t.Fatalf("bash: %v\n%s", err, out)
			}
			got := string(out)
			for _, want := range []string{
				"state=" + customState,
				"source=recovered",
				"url=https://custom.example",
				"token=custom123",
				"commands=" + map[bool]string{false: "false", true: "true"}[commandsEnabled],
			} {
				if !strings.Contains(got, want) {
					t.Fatalf("systemd discovery missing %q:\n%s", want, got)
				}
			}
			if strings.Contains(got, "default999") {
				t.Fatalf("systemd discovery borrowed default token:\n%s", got)
			}
			if _, err := os.Stat(customState); !os.IsNotExist(err) {
				t.Fatalf("uninstall did not remove discovered custom state: %v", err)
			}
			if _, err := os.Stat(defaultState); err != nil {
				t.Fatalf("uninstall removed the default instance instead of discovered custom state: %v", err)
			}
		})
	}
}

func TestInstallSHDiscoversCustomStateDirFromGeneratedLaunchdPlist(t *testing.T) {
	root := t.TempDir()
	customState := filepath.Join(root, "custom & state")
	if err := os.MkdirAll(customState, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(customState, "token"), []byte("launch123"), 0600); err != nil {
		t.Fatal(err)
	}
	plistPath := filepath.Join(root, "com.pulse.agent.plist")
	escapedState := strings.ReplaceAll(customState, "&", "&amp;")
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>ProgramArguments</key>
<array>
<string>/usr/local/bin/pulse-agent</string>
<string>--url</string>
<string>https://launch.example</string>
<string>--token-file</string>
<string>` + escapedState + `/token</string>
<string>--state-dir</string>
<string>` + escapedState + `</string>
<string>--enable-commands</string>
</array>
</dict></plist>`
	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		t.Fatal(err)
	}

	script := `
		set -euo pipefail
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
		STATE_DIR_SOURCE="default"
		DEFAULT_STATE_DIR="/var/lib/pulse-agent"
		TRUENAS_STATE_DIR="/data/pulse-agent"
		CURL_CA_BUNDLE=""
		SERVER_FINGERPRINT=""
		OBSERVERS_FILE=""
		KUBE_INCLUDE_ALL_PODS="false"
		KUBE_INCLUDE_ALL_DEPLOYMENTS="false"
		DISK_EXCLUDES=()
` + extractInstallShellFunction(t, "strip_recovered_arg_quotes") + `
` + extractInstallShellFunction(t, "normalize_recovered_agent_arg_key") + `
` + extractInstallShellFunction(t, "apply_recovered_agent_arg_value") + `
` + extractInstallShellFunction(t, "recovered_connection_state_ready") + `
` + extractInstallShellFunction(t, "recover_token_from_default_agent_token_file") + `
` + extractInstallShellFunction(t, "recover_connection_state_from_arg_stream") + `
` + extractInstallShellFunction(t, "launchd_agent_arg_stream") + `
` + extractInstallShellFunction(t, "recover_connection_state_from_launchd_plist") + `
		recover_connection_state_from_launchd_plist "` + plistPath + `"
		printf 'state=%s url=%s token=%s commands=%s\n' "$STATE_DIR" "$PULSE_URL" "$PULSE_TOKEN" "$ENABLE_COMMANDS"
	`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	for _, want := range []string{
		"state=" + customState,
		"url=https://launch.example",
		"token=launch123",
		"commands=true",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("launchd discovery missing %q:\n%s", want, got)
		}
	}
}

func TestInstallSHConnectionEnvPersistsCanonicalStateDirWithoutTokenValue(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	script := `
		set -euo pipefail
		STATE_DIR="` + stateDir + `"
		PULSE_URL="https://pulse.example.com"
		PULSE_TOKEN="deadbeef"
		RUNTIME_TOKEN_FILE="$STATE_DIR/token"
		AGENT_ID="agent-custom"
		HOSTNAME_OVERRIDE="host-custom"
		REPORT_IP="192.168.1.9"
		INSECURE="false"
		SERVER_FINGERPRINT=""
		CURL_CA_BUNDLE=""
		SAVED_INSTALL_SCRIPT=""
` + extractInstallShellFunction(t, "write_connection_state_value") + `
` + extractInstallShellFunction(t, "save_connection_info") + `
		curl() { return 1; }
		save_connection_info "$STATE_DIR"
		cat "$STATE_DIR/connection.env"
	`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "PULSE_STATE_DIR='"+stateDir+"'") ||
		!strings.Contains(got, "PULSE_TOKEN_FILE='"+filepath.Join(stateDir, "token")+"'") {
		t.Fatalf("connection.env did not persist canonical state paths:\n%s", got)
	}
	if !strings.Contains(got, "PULSE_REPORT_IP='192.168.1.9'") {
		t.Fatalf("connection.env did not persist the report IP:\n%s", got)
	}
	if strings.Contains(got, "PULSE_TOKEN='") || strings.Contains(got, "deadbeef") {
		t.Fatalf("connection.env leaked the token value:\n%s", got)
	}
	info, err := os.Stat(filepath.Join(stateDir, "connection.env"))
	if err != nil {
		t.Fatal(err)
	}
	if gotMode := info.Mode().Perm(); gotMode != 0600 {
		t.Fatalf("connection.env mode = %o, want 600", gotMode)
	}
}

func TestInstallSHStateWritesReplaceSymlinksAtomically(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		t.Fatal(err)
	}
	victimToken := filepath.Join(t.TempDir(), "victim-token")
	victimConnection := filepath.Join(t.TempDir(), "victim-connection")
	if err := os.WriteFile(victimToken, []byte("victim-token-content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(victimConnection, []byte("victim-connection-content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victimToken, filepath.Join(stateDir, "token")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victimConnection, filepath.Join(stateDir, "connection.env")); err != nil {
		t.Fatal(err)
	}

	script := `
		set -euo pipefail
		STATE_DIR="` + stateDir + `"
		PULSE_URL="https://pulse.example.com"
		IFS= read -r PULSE_TOKEN
		RUNTIME_TOKEN_FILE=""
		RUNTIME_TOKEN_CHANGED="false"
		ENROLL="false"
		AGENT_ID="agent-one"
		HOSTNAME_OVERRIDE="host-one"
		REPORT_IP=""
		INSECURE="false"
		SERVER_FINGERPRINT=""
		CURL_CA_BUNDLE=""
		SAVED_INSTALL_SCRIPT=""
		NON_INTERACTIVE="true"
		TMP_FILES=()
		log_info() { :; }
		curl() { return 1; }
` + extractInstallShellFunction(t, "write_connection_state_value") + `
` + extractInstallShellFunction(t, "ensure_runtime_token_file") + `
` + extractInstallShellFunction(t, "save_connection_info") + `
		ensure_runtime_token_file "$STATE_DIR"
		save_connection_info "$STATE_DIR"
	`
	cmd := exec.Command("bash", "-c", script)
	cmd.Stdin = strings.NewReader("new-token-content\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	for path, want := range map[string]string{
		victimToken:      "victim-token-content",
		victimConnection: "victim-connection-content",
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Fatalf("state write followed symlink and modified %s: %q", path, got)
		}
	}
	for _, path := range []string{filepath.Join(stateDir, "token"), filepath.Join(stateDir, "connection.env")} {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("%s remained a symlink after atomic replacement", path)
		}
	}
}

func TestInstallSHCurlTokenTransportKeepsSecretOutOfArgv(t *testing.T) {
	recordDir := t.TempDir()
	argsPath := filepath.Join(recordDir, "args")
	configPath := filepath.Join(recordDir, "config")
	script := `
		set -euo pipefail
		PULSE_TOKEN="feedface"
		curl() {
			printf '%s\n' "$@" > "` + argsPath + `"
			while [[ $# -gt 0 ]]; do
				if [[ "$1" == "--config" ]]; then
					cp "$2" "` + configPath + `"
					shift 2
					continue
				fi
				shift
			done
		}
` + extractInstallShellFunction(t, "curl_with_pulse_token") + `
		curl_with_pulse_token -sS https://pulse.example.com/api/agents/agent/lookup
	`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(args), "feedface") || strings.Contains(string(args), "X-API-Token") {
		t.Fatalf("curl argv leaked token material:\n%s", args)
	}
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "X-API-Token: feedface") {
		t.Fatalf("private curl config did not carry auth header:\n%s", config)
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
	if err := os.WriteFile(curlPath, []byte("#!/bin/sh\nexit 22\n"), 0755); err != nil {
		t.Fatalf("write curl stub: %v", err)
	}
	if err := os.WriteFile(fakeBashPath, []byte("#!/bin/sh\ncat >/dev/null\nexit 0\n"), 0755); err != nil {
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
		`final_response_header_value "$TMP_HEADERS" "X-Signature-SSHSIG"`,
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

// TestInstallSHVerifyAgentServerRegistrationDetectsRejectedToken verifies that
// the post-install health check tells a rejected token (401/403) apart from a
// transient "agent has not reported yet" state, so the installer can surface an
// actionable error instead of leaving a silent 401 loop behind (issue #1515).
func TestInstallSHVerifyAgentServerRegistrationDetectsRejectedToken(t *testing.T) {
	pulseAgent := buildPulseAgentLifecycleBinary(t)
	verifyFns := extractCollectorLifecycleShellFunctions(t, true)

	cases := []struct {
		name   string
		status int
		body   string
		wantRC string
	}{
		{"rejected token 401", http.StatusUnauthorized, `{"error":"Authentication required"}`, "rc=2"},
		{"rejected token 403", http.StatusForbidden, `{"error":"missing_scope"}`, "rc=2"},
		{"previous hostname owner during binding", http.StatusForbidden, `{"error":{"code":"agent_lookup_forbidden","message":"Agent does not belong to this API token"}}`, "rc=1"},
		{"registered", http.StatusOK, `{"success":true,"agent":{"id":"agent-omv","hostname":"omv","lastSeen":"2026-08-30T12:00:01Z"}}`, "rc=0"},
		{"not reported yet", http.StatusNotFound, `{"error":"agent_not_found"}`, "rc=1"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			stateDir := t.TempDir()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasPrefix(r.URL.Path, "/api/agents/agent/lookup") {
					http.Error(w, "not found", http.StatusNotFound)
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			script := `
				PULSE_URL="` + server.URL + `"
				PULSE_TOKEN="stale-token"
				STATE_DIR="` + stateDir + `"
				RUNTIME_TOKEN_FILE=""
				COLLECTOR_LIFECYCLE_BINARY_PATH="` + pulseAgent + `"
				LEAST_PRIVILEGE_USER="$(id -un)"
				SERVER_FINGERPRINT=""
				AGENT_ID=""
				HOSTNAME_OVERRIDE="omv"
				INSECURE="false"
				CURL_CA_BUNDLE=""
` + verifyFns + `
				verify_agent_server_registration
				echo "rc=$?"
			`
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if err != nil {
				t.Fatalf("bash: %v\n%s", err, out)
			}
			if !strings.Contains(string(out), tc.wantRC) {
				t.Fatalf("case %q: want %s, got:\n%s", tc.name, tc.wantRC, out)
			}
		})
	}
}

func TestInstallSHVerifyAgentServerRegistrationPrefersCanonicalAgentID(t *testing.T) {
	pulseAgent := buildPulseAgentLifecycleBinary(t)
	verifyFns := extractCollectorLifecycleShellFunctions(t, true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("id"); got != "agent-current" {
			t.Errorf("lookup id = %q, want canonical agent ID", got)
		}
		if got := r.URL.Query().Get("hostname"); got != "" {
			t.Errorf("hostname lookup = %q, want ID-only lookup", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"agent":{"id":"agent-current","hostname":"hostname-owned-by-previous-token","lastSeen":"2026-08-30T12:00:01Z"}}`))
	}))
	defer server.Close()

	script := `
		PULSE_URL="` + server.URL + `"
		PULSE_TOKEN="install-token"
		STATE_DIR="` + t.TempDir() + `"
		RUNTIME_TOKEN_FILE=""
		COLLECTOR_LIFECYCLE_BINARY_PATH="` + pulseAgent + `"
		LEAST_PRIVILEGE_USER="$(id -un)"
		SERVER_FINGERPRINT=""
		AGENT_ID="agent-current"
		HOSTNAME_OVERRIDE="hostname-owned-by-previous-token"
		INSECURE="false"
		CURL_CA_BUNDLE=""
` + verifyFns + `
		verify_agent_server_registration
		echo "rc=$?"
	`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "rc=0") {
		t.Fatalf("canonical ID lookup failed:\n%s", out)
	}
}

// TestInstallSHRegistrationRetryWindowOutlastsFirstReportCycle verifies that
// the post-install registration check polls for a window instead of warning
// after a single lookup: /readyz flips before the agent's first report cycle
// completes, so an immediate one-shot lookup routinely misses a healthy
// registration (issue #1644). A rejected token still short-circuits because
// more polling cannot change a definitive 401/403.
func TestInstallSHRegistrationRetryWindowOutlastsFirstReportCycle(t *testing.T) {
	pulseAgent := buildPulseAgentLifecycleBinary(t)
	verifyFns := extractCollectorLifecycleShellFunctions(t, true)
	retryFn := extractInstallShellFunction(t, "verify_agent_server_registration_with_retry")

	verifyStarted := extractInstallShellFunction(t, "verify_agent_started")
	if !strings.Contains(verifyStarted, "verify_agent_server_registration_with_retry") {
		t.Fatal("verify_agent_started does not use the registration retry window")
	}

	cases := []struct {
		name string
		// statuses is consumed one per lookup; the last value repeats.
		statuses []int
		wantRC   string
	}{
		{"registered after report cycle", []int{http.StatusNotFound, http.StatusNotFound, http.StatusOK}, "rc=0"},
		{"rejected token short-circuits", []int{http.StatusUnauthorized}, "rc=2"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			lookups := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasPrefix(r.URL.Path, "/api/agents/agent/lookup") {
					http.Error(w, "not found", http.StatusNotFound)
					return
				}
				mu.Lock()
				idx := lookups
				lookups++
				mu.Unlock()
				if idx >= len(tc.statuses) {
					idx = len(tc.statuses) - 1
				}
				w.WriteHeader(tc.statuses[idx])
				if tc.statuses[idx] == http.StatusOK {
					_, _ = w.Write([]byte(`{"success":true,"agent":{"id":"agent-1644","hostname":"pve-1644","lastSeen":"2026-08-30T12:00:01Z"}}`))
				} else {
					_, _ = w.Write([]byte(`{"error":"agent_not_found"}`))
				}
			}))
			defer server.Close()

			script := `
				PULSE_URL="` + server.URL + `"
				PULSE_TOKEN="install-token"
				STATE_DIR="` + t.TempDir() + `"
				RUNTIME_TOKEN_FILE=""
				COLLECTOR_LIFECYCLE_BINARY_PATH="` + pulseAgent + `"
				LEAST_PRIVILEGE_USER="$(id -un)"
				SERVER_FINGERPRINT=""
				AGENT_ID=""
				HOSTNAME_OVERRIDE="pve-1644"
				INSECURE="false"
				CURL_CA_BUNDLE=""
				sleep() { :; }
` + verifyFns + `
` + retryFn + `
				verify_agent_server_registration_with_retry
				echo "rc=$?"
			`
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if err != nil {
				t.Fatalf("bash: %v\n%s", err, out)
			}
			if !strings.Contains(string(out), tc.wantRC) {
				t.Fatalf("case %q: want %s, got:\n%s", tc.name, tc.wantRC, out)
			}
			if tc.wantRC == "rc=2" {
				mu.Lock()
				got := lookups
				mu.Unlock()
				if got != 1 {
					t.Fatalf("rejected token was retried %d times, want 1 lookup", got)
				}
			}
		})
	}
}

func TestInstallSHRejectedCredentialPreventsSuccessfulCompletion(t *testing.T) {
	completeFlow := extractInstallShellFunction(t, "complete_installation_flow")
	script := `
		EXIT_AUTH_REJECTED=18
		UPGRADE_MODE="true"
		save_connection_info() { :; }
		verify_agent_started() { return 2; }
		report_proxmox_registration_outcome() { printf 'unexpected-proxmox-report\n'; }
		log_info() { printf 'INFO:%s\n' "$*"; }
		log_warn() { printf 'WARN:%s\n' "$*"; }
		log_error() { printf 'ERROR:%s\n' "$*"; }
		json_event() { printf 'JSON:%s:%s:%s:%s\n' "$1" "$2" "$3" "${4:-}"; }
` + completeFlow + `
		complete_installation_flow "/tmp/state" "INSTALL-SUCCESS" "UPGRADE-SUCCESS" "logs"
		printf 'unreachable\n'
	`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 18 {
		t.Fatalf("rejected credential exit = %v, want 18\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "JSON:complete:auth_rejected") || !strings.Contains(output, "authentication failed") {
		t.Fatalf("rejected credential did not produce actionable non-success completion:\n%s", output)
	}
	for _, forbidden := range []string{"INSTALL-SUCCESS", "UPGRADE-SUCCESS", "unreachable", "unexpected-proxmox-report"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("rejected credential emitted forbidden success output %q:\n%s", forbidden, output)
		}
	}
}

// TestInstallSHSurfacesBlockedProxmoxRegistration verifies the installer reads
// the agent's proxmox-<type>-registration-blocked marker and prints the denial
// in its own output instead of leaving it buried in the agent journal (#1644).
func TestInstallSHSurfacesBlockedProxmoxRegistration(t *testing.T) {
	logInfo := extractInstallShellFunction(t, "log_info")
	logWarn := extractInstallShellFunction(t, "log_warn")
	logError := extractInstallShellFunction(t, "log_error")
	reportFn := extractInstallShellFunction(t, "report_proxmox_registration_outcome")

	completeFlow := extractInstallShellFunction(t, "complete_installation_flow")
	if !strings.Contains(completeFlow, `report_proxmox_registration_outcome "$state_dir"`) {
		t.Fatal("complete_installation_flow does not report the Proxmox registration outcome")
	}
	clearFn := extractInstallShellFunction(t, "clear_proxmox_state_if_needed")
	for _, marker := range []string{"proxmox-pve-registration-blocked", "proxmox-pbs-registration-blocked", "proxmox-detected-types"} {
		if !strings.Contains(clearFn, marker) {
			t.Fatalf("clear_proxmox_state_if_needed does not clear %s", marker)
		}
	}

	runReport := func(t *testing.T, stateDir string) (string, int) {
		t.Helper()
		script := `
			NON_INTERACTIVE="false"
			ENABLE_PROXMOX="true"
			redact_token() { printf '%s' "$1"; }
			sleep() { :; }
` + logInfo + `
` + logWarn + `
` + logError + `
` + reportFn + `
			report_proxmox_registration_outcome "` + stateDir + `"
		`
		out, err := exec.Command("bash", "-c", script).CombinedOutput()
		rc := 0
		if err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("bash: %v\n%s", err, out)
			}
			rc = exitErr.ExitCode()
		}
		return string(out), rc
	}

	t.Run("blocked marker surfaces reason", func(t *testing.T) {
		stateDir := t.TempDir()
		reason := "Pulse at https://pulse.local has no PVE source for https://pve.local:8006 and this agent's token cannot create one."
		if err := os.WriteFile(filepath.Join(stateDir, "proxmox-pve-registration-blocked"), []byte(reason+"\n"), 0o600); err != nil {
			t.Fatalf("write blocked marker: %v", err)
		}
		out, rc := runReport(t, stateDir)
		if rc == 0 {
			t.Fatalf("blocked registration reported success:\n%s", out)
		}
		if !strings.Contains(out, "[ERROR]") || !strings.Contains(out, "registration failed") {
			t.Fatalf("blocked registration not surfaced as an error:\n%s", out)
		}
		if !strings.Contains(out, reason) {
			t.Fatalf("blocked registration output missing agent-recorded reason:\n%s", out)
		}
	})

	t.Run("registered marker reports success", func(t *testing.T) {
		stateDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(stateDir, "proxmox-pve-registered"), []byte("ok\n"), 0o600); err != nil {
			t.Fatalf("write registered marker: %v", err)
		}
		out, rc := runReport(t, stateDir)
		if rc != 0 {
			t.Fatalf("registered marker returned rc=%d:\n%s", rc, out)
		}
		if !strings.Contains(out, "Proxmox pve node registered with Pulse.") {
			t.Fatalf("registered marker did not report success:\n%s", out)
		}
	})

	// A host with both PVE and PBS installed registers each product separately
	// (#1644). The installer must report each outcome rather than letting the
	// first marker it finds speak for the whole install.
	t.Run("combined host reports both products", func(t *testing.T) {
		stateDir := t.TempDir()
		writeMarker(t, stateDir, "proxmox-detected-types", "pve\npbs\n")
		writeMarker(t, stateDir, "proxmox-pve-registered", "ok\n")
		writeMarker(t, stateDir, "proxmox-pbs-registered", "ok\n")

		out, rc := runReport(t, stateDir)
		if rc != 0 {
			t.Fatalf("combined host returned rc=%d:\n%s", rc, out)
		}
		for _, want := range []string{"Proxmox pve node registered with Pulse.", "Proxmox pbs node registered with Pulse."} {
			if !strings.Contains(out, want) {
				t.Fatalf("combined host output missing %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "[ERROR]") {
			t.Fatalf("fully registered combined host reported an error:\n%s", out)
		}
	})

	// The regression behind #1644: PVE registered, PBS was refused, and the
	// installer printed a bare ERROR banner that read as a failed install.
	// Both outcomes must now show up, each attributed to its own product.
	t.Run("combined host keeps a successful product visible next to a refusal", func(t *testing.T) {
		stateDir := t.TempDir()
		reason := "Pulse at https://pulse.local has no PBS source for https://pbs.local:8007 and this agent's token cannot create one."
		writeMarker(t, stateDir, "proxmox-detected-types", "pve\npbs\n")
		writeMarker(t, stateDir, "proxmox-pve-registered", "ok\n")
		writeMarker(t, stateDir, "proxmox-pbs-registration-blocked", reason+"\n")

		out, rc := runReport(t, stateDir)
		if rc == 0 {
			t.Fatalf("refused pbs registration reported overall success:\n%s", out)
		}
		if !strings.Contains(out, "Proxmox pve node registered with Pulse.") {
			t.Fatalf("pve success was erased by the pbs refusal:\n%s", out)
		}
		if !strings.Contains(out, "Proxmox pbs registration failed:") || !strings.Contains(out, reason) {
			t.Fatalf("pbs refusal not surfaced with its reason:\n%s", out)
		}
	})

	// Only the detected products are awaited. A PVE-only host must not sit out
	// the full timeout waiting for a PBS outcome that will never arrive.
	t.Run("detected types bound what is awaited", func(t *testing.T) {
		stateDir := t.TempDir()
		writeMarker(t, stateDir, "proxmox-detected-types", "pve\n")
		writeMarker(t, stateDir, "proxmox-pve-registered", "ok\n")

		out, rc := runReport(t, stateDir)
		if rc != 0 {
			t.Fatalf("pve-only host returned rc=%d:\n%s", rc, out)
		}
		if strings.Contains(out, "pbs") {
			t.Fatalf("pve-only host reported on pbs:\n%s", out)
		}
		if strings.Contains(out, "not confirmed") {
			t.Fatalf("pve-only host warned about an unconfirmed registration:\n%s", out)
		}
	})

	// Agents older than the detected-types marker keep the previous timing:
	// the first outcome that appears ends the wait.
	t.Run("missing detected types falls back to first outcome", func(t *testing.T) {
		stateDir := t.TempDir()
		writeMarker(t, stateDir, "proxmox-pbs-registered", "ok\n")

		out, rc := runReport(t, stateDir)
		if rc != 0 {
			t.Fatalf("legacy agent marker returned rc=%d:\n%s", rc, out)
		}
		if !strings.Contains(out, "Proxmox pbs node registered with Pulse.") {
			t.Fatalf("legacy agent marker did not report success:\n%s", out)
		}
		if strings.Contains(out, "not confirmed") {
			t.Fatalf("legacy agent marker warned about an unconfirmed registration:\n%s", out)
		}
	})

	t.Run("no outcome warns once", func(t *testing.T) {
		stateDir := t.TempDir()
		out, rc := runReport(t, stateDir)
		if rc != 0 {
			t.Fatalf("unconfirmed registration returned rc=%d:\n%s", rc, out)
		}
		if !strings.Contains(out, "not confirmed") || !strings.Contains(out, "Check the agent logs") {
			t.Fatalf("unconfirmed registration did not warn:\n%s", out)
		}
	})
}

func writeMarker(t *testing.T, stateDir, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(stateDir, name), []byte(contents), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestInstallSHWarnAgentTokenRejectedIsActionable pins the actionable recovery
// copy the installer prints when the server rejects the agent's token.
func TestInstallSHWarnAgentTokenRejectedIsActionable(t *testing.T) {
	logWarn := extractInstallShellFunction(t, "log_warn")
	warnFn := extractInstallShellFunction(t, "warn_agent_token_rejected")

	script := `
		NON_INTERACTIVE="false"
		redact_token() { printf '%s' "$1"; }
` + logWarn + `
` + warnFn + `
		warn_agent_token_rejected
	`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	for _, needle := range []string{
		"rejected this agent's API token",
		"Re-run the full agent install command",
		"mint a fresh token",
	} {
		if !strings.Contains(string(out), needle) {
			t.Fatalf("warn_agent_token_rejected missing %q:\n%s", needle, out)
		}
	}
}

// TestInstallSHAgentKillPatternsExcludeSiblingAgents guards a real incident:
// the installer and the Unraid wrapper it generates both killed agents with
// pkill -f "^<binary>". pkill -f matches the whole command line and "^" only
// anchors the start, so on a host running a second, co-installed agent whose
// binary name merely starts with the same prefix (pulse-agent alongside
// pulse-agent-prod) every install, upgrade, and wrapper restart silently took
// down the other agent too.
func TestInstallSHAgentKillPatternsExcludeSiblingAgents(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	// Every pkill -f whose pattern is anchored at the agent binary must also
	// bound the far end, or it matches the sibling agent.
	unbounded := regexp.MustCompile(`pkill[^\n]*-f "\^\$\{(?:RUNTIME_BINARY|INSTALL_DIR)\}[^"\n]*"`)
	for _, match := range unbounded.FindAllString(string(content), -1) {
		if !strings.Contains(match, "[[:space:]]") {
			t.Errorf("unbounded agent pkill pattern would also match a sibling agent: %s", match)
		}
	}

	// A bare substring kill is worse still: it needs no prefix anchor at all.
	if strings.Contains(string(content), `pkill -9 -f "pulse-agent"`) {
		t.Error(`pkill -9 -f "pulse-agent" matches pulse-agent-prod; use -x on the exact process name`)
	}
}

// TestInstallSHAgentKillPatternSparesSiblingAgent proves the pattern semantics
// rather than its spelling. pkill -f applies a POSIX ERE to the whole command
// line, so the same engine is exercised here against the two command lines a
// dual-agent host actually presents.
func TestInstallSHAgentKillPatternSparesSiblingAgent(t *testing.T) {
	const (
		targetCmd  = "/usr/local/bin/pulse-agent --url http://192.168.0.113:7655 --interval 30s"
		siblingCmd = "/usr/local/bin/pulse-agent-prod --url http://192.168.0.220:7655 --interval 30s"
	)

	ereMatches := func(t *testing.T, pattern, line string) bool {
		t.Helper()
		cmd := exec.Command("grep", "-E", "-q", pattern)
		cmd.Stdin = strings.NewReader(line + "\n")
		err := cmd.Run()
		if err == nil {
			return true
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false
		}
		t.Fatalf("grep -E %q: %v", pattern, err)
		return false
	}

	unbounded := "^/usr/local/bin/pulse-agent"
	if !ereMatches(t, unbounded, siblingCmd) {
		t.Fatal("premise check failed: the unbounded pattern is supposed to match the sibling agent")
	}

	bounded := "^/usr/local/bin/pulse-agent([[:space:]]|$)"
	if !ereMatches(t, bounded, targetCmd) {
		t.Error("bounded pattern must still match its own agent")
	}
	if ereMatches(t, bounded, siblingCmd) {
		t.Error("bounded pattern must not match pulse-agent-prod")
	}
}

// TestInstallSHUnraidStopsWatchdogBeforeAgent guards a real incident: the
// Unraid install path killed the agent but never the wrapper supervising it,
// then started a second wrapper at the end of the install. The survivor and
// the newcomer both loop trying to own the same agent id, and because the old
// wrapper is a watchdog it can respawn the agent during the install window
// using the previous binary and arguments.
func TestInstallSHUnraidStopsWatchdogBeforeAgent(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}
	script := string(content)

	start := strings.Index(script, "if [[ -f /etc/unraid-version ]]; then")
	if start < 0 {
		t.Fatal("could not locate the Unraid install branch")
	}
	endOffset := strings.Index(script[start:], "\n# 4.")
	if endOffset < 0 {
		endOffset = len(script) - start
	}
	unraid := script[start : start+endOffset]

	wrapperKill := strings.Index(unraid, `pkill -f "/start-pulse-agent\.sh`)
	if wrapperKill < 0 {
		t.Fatal("Unraid install must stop the existing wrapper supervisor, or its watchdog survives the install and races the new one")
	}
	agentKill := strings.Index(unraid, `pkill -f "^${RUNTIME_BINARY}`)
	if agentKill < 0 {
		t.Fatal("Unraid install must stop the existing agent")
	}
	if wrapperKill > agentKill {
		t.Error("the wrapper supervisor must be stopped before the agent, otherwise the watchdog respawns the agent during the install")
	}
}

// TestInstallSHWatchdogKillPatternSparesSiblingWrapper proves the wrapper
// pattern discriminates, so stopping this agent's supervisor never stops a
// co-installed agent's supervisor on the same host.
func TestInstallSHWatchdogKillPatternSparesSiblingWrapper(t *testing.T) {
	const (
		targetCmd  = "bash /boot/config/plugins/pulse-agent/start-pulse-agent.sh"
		siblingCmd = "bash /boot/config/plugins/pulse-agent-prod/start-pulse-agent-prod.sh"
	)

	ereMatches := func(t *testing.T, pattern, line string) bool {
		t.Helper()
		cmd := exec.Command("grep", "-E", "-q", pattern)
		cmd.Stdin = strings.NewReader(line + "\n")
		err := cmd.Run()
		if err == nil {
			return true
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false
		}
		t.Fatalf("grep -E %q: %v", pattern, err)
		return false
	}

	pattern := `/start-pulse-agent\.sh([[:space:]]|$)`
	if !ereMatches(t, pattern, targetCmd) {
		t.Error("wrapper pattern must match this agent's own supervisor")
	}
	if ereMatches(t, pattern, siblingCmd) {
		t.Error("wrapper pattern must not match a co-installed agent's supervisor")
	}

	// Premise check: an unescaped dot is what makes this worth pinning.
	if !ereMatches(t, "start-pulse-agent.sh", targetCmd) {
		t.Fatal("premise check failed: the loose pattern is supposed to match the target")
	}
}

// TestInstallSHWrapperKillsAreBoundedEverywhere pins every wrapper kill in the
// installer, not just the Unraid install branch. A bare "start-pulse-agent.sh"
// pattern leaves the dot as a wildcard and the far end unbounded, so it also
// matches a .bak copy of the wrapper or an editor session holding it open.
func TestInstallSHWrapperKillsAreBoundedEverywhere(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}
	script := string(content)

	if strings.Contains(script, `pkill -f "start-pulse-agent.sh"`) {
		t.Error(`bare pkill -f "start-pulse-agent.sh" leaves the dot as a wildcard and the tail unbounded`)
	}

	wrapperKill := regexp.MustCompile(`pkill -f "[^"\n]*start-pulse-agent[^"\n]*"`)
	found := wrapperKill.FindAllString(script, -1)
	if len(found) == 0 {
		t.Fatal("expected the installer to stop wrapper supervisors somewhere")
	}
	for _, match := range found {
		if !strings.Contains(match, `\.sh`) {
			t.Errorf("wrapper kill must escape the dot: %s", match)
		}
		if !strings.Contains(match, "[[:space:]]") {
			t.Errorf("wrapper kill must bound its far end: %s", match)
		}
	}
}

// TestInstallSHStopsWrapperBeforeAgentInEveryBranch pins the ordering rule the
// agent-lifecycle and deployment-installability contracts now carry: a wrapper
// is a watchdog, so stopping the agent while its wrapper still loops merely
// races the respawn. Every branch that stops both must stop the wrapper first.
func TestInstallSHStopsWrapperBeforeAgentInEveryBranch(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}
	lines := strings.Split(string(content), "\n")

	// Walk the file and, for each wrapper kill, confirm the nearest agent kill
	// that shares its stop block does not precede it.
	for i, line := range lines {
		if !strings.Contains(line, "pkill") || !strings.Contains(line, "start-pulse-agent") {
			continue
		}
		// Look back a short window for an agent kill in the same block.
		for back := i - 1; back >= 0 && back > i-12; back-- {
			prev := lines[back]
			if !strings.Contains(prev, "pkill") {
				continue
			}
			if strings.Contains(prev, "start-pulse-agent") {
				break
			}
			if strings.Contains(prev, `-x "pulse-agent"`) || strings.Contains(prev, "${RUNTIME_BINARY}") ||
				strings.Contains(prev, "${BINARY_NAME}") || strings.Contains(prev, "${INSTALL_DIR}") {
				t.Errorf("line %d stops the agent before its wrapper at line %d; the watchdog will respawn it:\n  %s\n  %s",
					back+1, i+1, strings.TrimSpace(prev), strings.TrimSpace(line))
			}
			break
		}
	}
}

// TestInstallSHVersionMismatchWarningIgnoresBuildMetadata pins the comparison
// behind the "Downloaded agent version does not match" warning. A server built
// from a working tree reports build metadata the agent it serves never carries,
// so a raw comparison fired on every correct development install. That is not a
// cosmetic annoyance: the warning is the only client-side signal that a stale
// agent was downloaded, and one that cries wolf gets skipped the time it counts.
func TestInstallSHVersionMismatchWarningIgnoresBuildMetadata(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}
	script := string(content)

	for _, required := range []string{
		`NEW_VERSION_NORMALIZED="${NEW_VERSION_NORMALIZED%%+*}"`,
		`SERVER_VERSION_NORMALIZED="${SERVER_VERSION_NORMALIZED%%+*}"`,
	} {
		if !strings.Contains(script, required) {
			t.Errorf("version comparison must strip semver build metadata, missing: %s", required)
		}
	}

	// Exercise the same normalisation the installer performs.
	normalize := func(v string) string {
		v = strings.TrimPrefix(v, "v")
		if idx := strings.Index(v, "+"); idx >= 0 {
			v = v[:idx]
		}
		return v
	}
	cases := []struct {
		agent  string
		server string
		warns  bool
	}{
		// The shape that fired on every correct dev install.
		{"v6.2.0-rc.8", "6.2.0-rc.8+git.46.g98a638e00.dirty", false},
		{"v6.2.0-rc.8", "6.2.0-rc.8", false},
		// The stale download this warning exists to catch.
		{"v6.0.5-54-gc862fb0ca0", "6.2.0-rc.8+git.46.g98a638e00.dirty", true},
		// A prerelease is genuinely not its release.
		{"v6.2.0", "6.2.0-rc.8+git.46.gabc.dirty", true},
		{"v6.1.2", "6.2.0-rc.8", true},
	}
	for _, tc := range cases {
		got := normalize(tc.agent) != normalize(tc.server)
		if got != tc.warns {
			t.Errorf("agent %q vs server %q: warns=%v, want %v", tc.agent, tc.server, got, tc.warns)
		}
	}
}

// The least-privilege profile must stay a real profile, not a cosmetic flag:
// a dedicated nologin system user, validated exact-command sudoers grants, a
// pct grant that can never widen into pct exec, env-pinned absolute helper
// paths, update-time preservation of the profile, and no ambient capability
// grant. Silently falling back to root on unsupported platforms is forbidden.
func TestInstallSHLeastPrivilegeProfile(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`--least-privilege) LEAST_PRIVILEGE="true"; shift ;;`,
		`--grant-smart) GRANT_SMART="true"; shift ;;`,
		`--grant-pct) GRANT_PCT="true"; shift ;;`,
		`LEAST_PRIVILEGE_USER="pulse-agent"`,
		`PRIVILEGE_SUDOERS_FILE="/etc/sudoers.d/pulse-agent"`,
		"--least-privilege and --enable-commands are mutually exclusive",
		"--least-privilege is supported only on standard Linux systemd hosts",
		`useradd --system --user-group --home-dir "$STATE_DIR" --no-create-home`,
		`visudo -cf`,
		`install -o root -g root -m 0440 "$sudoers_tmp" "$PRIVILEGE_SUDOERS_FILE"`,
		"${pct_path} list, ${pct_path} df *",
		`append_service_env "PULSE_SMARTCTL_PATH" "${PRIVILEGE_HELPER_DIR}/smartctl"`,
		`append_service_env "PULSE_PCT_PATH" "${PRIVILEGE_HELPER_DIR}/pct"`,
		`grep -q "^User=${LEAST_PRIVILEGE_USER}\$" "$UNIT"`,
		`"network-online.target" "$SERVICE_USER" ""`,
		"# The least-privilege profile never attaches into guests",
		`rm -f "$PRIVILEGE_SUDOERS_FILE"`,
		// NoNewPrivileges blocks sudo, so an active grant must relax it or
		// the helpers silently fail inside the service (proven live).
		`if [[ "$LEAST_PRIVILEGE" == "true" ]] && [[ "$GRANT_SMART" == "true" || "$GRANT_PCT" == "true" ]]; then`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing least-privilege profile invariant: %s", needle)
		}
	}

	if strings.Contains(script, `NOPASSWD: ${pct_path}`) && !strings.Contains(script, "does NOT cover pct exec") {
		t.Fatal("install.sh must document that the pct grant excludes pct exec")
	}
}

func TestInstallSHTypedPrivilegedHelperUnits(t *testing.T) {
	root := t.TempDir()
	socketUnit := filepath.Join(root, "pulse-agent-helper.socket")
	serviceUnit := filepath.Join(root, "pulse-agent-helper.service")

	script := `
		set -euo pipefail
		PRIVILEGED_HELPER_NAME="pulse-agent-helper"
		PRIVILEGED_HELPER_SOCKET_PATH="/run/pulse-agent/helper.sock"
		PRIVILEGED_HELPER_UPDATE_QUARANTINE_DIR="/var/lib/pulse-agent/update-quarantine"
		PRIVILEGED_HELPER_STATE_DIR="/var/lib/pulse-agent-helper"
		LEAST_PRIVILEGE_USER="pulse-agent"
` + extractInstallShellFunction(t, "render_privileged_helper_socket_unit") + `
` + extractInstallShellFunction(t, "render_privileged_helper_service_unit") + `
		render_privileged_helper_socket_unit "` + socketUnit + `"
		render_privileged_helper_service_unit "` + serviceUnit + `" "/usr/local/lib/pulse-agent/pulse-agent-helper"
	`
	if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("render helper units: %v\n%s", err, out)
	}

	socketBytes, err := os.ReadFile(socketUnit)
	if err != nil {
		t.Fatal(err)
	}
	socket := string(socketBytes)
	for _, required := range []string{
		"ListenStream=/run/pulse-agent/helper.sock",
		"SocketUser=root",
		"SocketGroup=pulse-agent",
		"SocketMode=0660",
		"DirectoryMode=0755",
		"RemoveOnStop=true",
	} {
		if !strings.Contains(socket, required) {
			t.Fatalf("typed helper socket unit missing %q:\n%s", required, socket)
		}
	}

	serviceBytes, err := os.ReadFile(serviceUnit)
	if err != nil {
		t.Fatal(err)
	}
	service := string(serviceBytes)
	for _, required := range []string{
		"ExecStart=/usr/local/lib/pulse-agent/pulse-agent-helper",
		"User=root",
		"Group=root",
		"NoNewPrivileges=true",
		"PrivateNetwork=true",
		"RestrictAddressFamilies=AF_UNIX",
		"ProtectSystem=strict",
		"ProtectHome=true",
		"ReadOnlyPaths=/var/lib/pulse-agent/update-quarantine",
		"ReadWritePaths=/var/lib/pulse-agent-helper /usr/local/bin",
	} {
		if !strings.Contains(service, required) {
			t.Fatalf("typed helper service unit missing %q:\n%s", required, service)
		}
	}
	if strings.Contains(service, "PrivateDevices") {
		t.Fatalf("typed helper service must retain host device visibility for SMART:\n%s", service)
	}
}

func TestInstallSHTypedPrivilegedHelperProfileIsOptInAndFailClosed(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}
	script := string(content)

	for _, required := range []string{
		`PRIVILEGED_HELPER_ENABLED="false"`,
		`--enable-privileged-helper) PRIVILEGED_HELPER_ENABLED="true"; PRIVILEGED_HELPER_EXPLICIT="true"; shift ;;`,
		`--disable-privileged-helper) PRIVILEGED_HELPER_ENABLED="false"; PRIVILEGED_HELPER_EXPLICIT="true"; shift ;;`,
		`--enable-privileged-helper requires --least-privilege`,
		`--enable-privileged-helper cannot be combined with legacy --grant-smart/--grant-pct sudo paths`,
		`--enable-privileged-helper is supported only on standard Linux systemd hosts; no broader-privilege fallback was applied`,
		`Preserving existing typed privileged-helper profile`,
		`PULSE_AGENT_HELPER_SOCKET`,
		`chown root:root "${INSTALL_DIR}/${BINARY_NAME}"`,
		`chown root:root "$PRIVILEGED_HELPER_BINARY_PATH"`,
		`chown -R "${LEAST_PRIVILEGE_USER}:${LEAST_PRIVILEGE_USER}" "$STATE_DIR"`,
		`protect_typed_profile_credentials`,
		`PRIVILEGED_HELPER_CREDENTIAL_DIR="/etc/pulse-agent"`,
		`chown "root:${LEAST_PRIVILEGE_USER}" "$PRIVILEGED_HELPER_CREDENTIAL_DIR"`,
		`chmod 0750 "$PRIVILEGED_HELPER_CREDENTIAL_DIR"`,
		`chown "root:${LEAST_PRIVILEGE_USER}" "$RUNTIME_TOKEN_FILE"`,
		`chmod 0640 "$RUNTIME_TOKEN_FILE"`,
		`if [[ "$PRIVILEGED_HELPER_ENABLED" != "true" && "$ENABLE_DOCKER" != "false" ]]`,
		`/download/${PRIVILEGED_HELPER_BINARY_NAME}?${DOWNLOAD_QUERY}`,
		`verify_download_signature "$TMP_HELPER_BIN" "$helper_signature"`,
		`teardown_privileged_helper_service`,
		`socket_owner=$(stat -c '%u:%g' "$PRIVILEGED_HELPER_SOCKET_PATH"`,
		`socket_mode=$(stat -c '%a' "$PRIVILEGED_HELPER_SOCKET_PATH"`,
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("install.sh missing typed-helper invariant: %s", required)
		}
	}
	if strings.Contains(script, `EXEC_ARG_ITEMS+=(--disable-auto-update)`) {
		t.Fatal("typed helper profile must keep updater enabled for signed helper-backed activation")
	}
}

func TestInstallSHTypedPrivilegeHelperUpdateFilesystemBoundary(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}
	script := string(content)
	for _, required := range []string{
		`PRIVILEGED_HELPER_STATE_DIR="/var/lib/pulse-agent-helper"`,
		`PRIVILEGED_HELPER_UPDATE_STAGING_DIR="${PRIVILEGED_HELPER_STATE_DIR}/update-staging"`,
		`PRIVILEGED_HELPER_UPDATE_QUARANTINE_DIR="/var/lib/pulse-agent/update-quarantine"`,
		`ReadOnlyPaths=${PRIVILEGED_HELPER_UPDATE_QUARANTINE_DIR}`,
		`ReadWritePaths=${PRIVILEGED_HELPER_STATE_DIR} /usr/local/bin`,
		`install -d -o "$LEAST_PRIVILEGE_USER" -g "$LEAST_PRIVILEGE_USER" -m 0700`,
		`install -d -o root -g root -m 0700`,
		`Typed privileged-helper updates require the fixed /usr/local/bin/pulse-agent target`,
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("install.sh missing typed helper update boundary %q", required)
		}
	}
}

func TestInstallSHActionRunnerIsSeparateOptInLifecycle(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}
	script := string(content)
	for _, required := range []string{
		`ACTION_RUNNER_ENABLED="false"`,
		`ACTION_RUNNER_NAME="pulse-agent-runner"`,
		`ACTION_RUNNER_TOKEN_FILE="${ACTION_RUNNER_CONFIG_DIR}/token"`,
		`--enable-action-runner) ACTION_RUNNER_ENABLED="true"; ACTION_RUNNER_EXPLICIT="true"; shift ;;`,
		`--disable-action-runner) ACTION_RUNNER_ENABLED="false"; ACTION_RUNNER_EXPLICIT="true"; shift ;;`,
		`--uninstall-action-runner) UNINSTALL_ACTION_RUNNER="true"; shift ;;`,
		`--enable-action-runner requires the safe --least-privilege --enable-privileged-helper collector profile`,
		`--enable-action-runner cannot be combined with legacy collector --enable-commands`,
		`--action-token-file requires --enable-action-runner (or an existing preserved runner profile)`,
		`The action runner must use a separate credential from the collector token`,
		`Preserving existing separately enabled action-runner profile`,
		`write_action_runner_env_value "PULSE_AGENT_RUNNER_TOKEN_FILE" "$ACTION_RUNNER_TOKEN_FILE"`,
		`write_action_runner_env_value "PULSE_AGENT_RUNNER_AGENT_ID" "$runner_agent_id"`,
		`collector-read-agent-id --agent-id-file "${STATE_DIR%/}/agent-id"`,
		`identity_args=(collector-read-agent-id --agent-id-file "$runner_agent_id_file")`,
		`write_action_runner_env_value "PULSE_AGENT_RUNNER_HEALTH_FILE" "$ACTION_RUNNER_HEALTH_FILE"`,
		`write_action_runner_env_value "PULSE_AGENT_RUNNER_ACTIVATION_NONCE" "$ACTION_RUNNER_ACTIVATION_NONCE"`,
		`write_action_runner_env_value "PULSE_AGENT_RUNNER_HOSTNAME" "$runner_hostname"`,
		`revoke-credential --url "$runner_url" --token-file "$runner_token_file"`,
		`cancel-pending-credential --url "$PULSE_URL" --token-file "$token_tmp"`,
		`Action runner removal requires a successful credential revocation`,
		`/download/${ACTION_RUNNER_BINARY_NAME}?${DOWNLOAD_QUERY}`,
		`verify_download_signature "$TMP_ACTION_RUNNER_BIN" "$runner_signature"`,
		`install -o root -g root -m 0755 "$TMP_ACTION_RUNNER_BIN"`,
		`chmod 0600 "$ACTION_RUNNER_TOKEN_FILE"`,
		`ACTION_TOKEN=""`,
		`action_runner_health_matches_activation "$expected_agent_id" "$activation_nonce"`,
		`[[ "$health_agent_id" == "$expected_agent_id" && "$health_activation_nonce" == "$expected_nonce" ]]`,
		`cancel_pending_action_runner_credential "$expected_agent_id" "$expected_hostname" "$replacement_action_token"`,
		`ACTION_TOKEN="$replacement_action_token"`,
		`Could not durably persist the complete replacement action-runner credential and environment`,
		`if ! cancel_pending_action_runner_credential`,
		`The exact replacement credential and runtime were retained durably`,
		`did not durably confirm cancellation`,
		`rolling back runner-only files while leaving monitoring active`,
		`Pulse action runner removed. Collector monitoring was left installed and running.`,
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("install.sh missing action-runner invariant: %s", required)
		}
	}
	teardown := extractInstallShellFunction(t, "teardown_action_runner_service")
	if strings.Contains(teardown, "teardown_systemd_agent_service") || strings.Contains(teardown, `rm -f "${INSTALL_DIR}/${BINARY_NAME}"`) {
		t.Fatalf("runner teardown must not remove the monitoring collector:\n%s", teardown)
	}
	if strings.Contains(script, `head -1 "$runner_agent_id_file"`) {
		t.Fatal("legacy runner removal must not path-read the collector-owned identity file")
	}
}

func TestInstallSHActionRunnerConfigBindsKnownAgentIDImmediately(t *testing.T) {
	root := t.TempDir()
	envFile := filepath.Join(root, "runner.env")
	script := `
set -euo pipefail
ACTION_RUNNER_CONFIG_DIR="` + filepath.Join(root, "config") + `"
ACTION_RUNNER_STATE_DIR="` + filepath.Join(root, "runner-state") + `"
ACTION_RUNNER_TOKEN_FILE="` + filepath.Join(root, "config", "token") + `"
ACTION_RUNNER_ENV_FILE="` + envFile + `"
ACTION_RUNNER_HEALTH_FILE="` + filepath.Join(root, "runner-state", "health.json") + `"
ACTION_RUNNER_ACTIVATION_NONCE="` + strings.Repeat("a", 64) + `"
STATE_DIR="` + filepath.Join(root, "collector-state") + `"
HOSTNAME_OVERRIDE="secure-runtime.lab"
AGENT_ID="agent-bound-before-collector-report"
ACTION_TOKEN="runner-secret"
PULSE_URL="https://pulse.example"
SERVER_FINGERPRINT=""
CURL_CA_BUNDLE=""
INSECURE="false"
EXIT_GENERAL=1
EXIT_MISSING_ARGS=2
fail() { printf '%s\n' "$1" >&2; exit "$2"; }
install() { local destination="${!#}"; mkdir -p "$destination"; chmod 0700 "$destination"; }
chown() { return 0; }
stat() {
    case "$1:$2" in
        '-c:%u') printf '0\n' ;;
        '-c:%a') printf '600\n' ;;
        *) command stat "$@" ;;
    esac
}
` + extractInstallShellFunction(t, "write_action_runner_env_value") + `
` + extractInstallShellFunction(t, "action_runner_url_uses_loopback_http") + `
` + extractInstallShellFunction(t, "action_runner_url_transport_allowed") + `
` + extractInstallShellFunction(t, "resolve_action_runner_agent_id") + `
` + extractInstallShellFunction(t, "persist_action_runner_replacement_token") + `
` + extractInstallShellFunction(t, "write_action_runner_config") + `
write_action_runner_config
`
	if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("write action-runner config: %v\n%s", err, out)
	}
	content, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	want := `PULSE_AGENT_RUNNER_AGENT_ID="agent-bound-before-collector-report"`
	if !strings.Contains(string(content), want) {
		t.Fatalf("action-runner environment missing immediate identity binding %q:\n%s", want, content)
	}
	for path, wantMode := range map[string]os.FileMode{
		filepath.Join(root, "config", "token"): 0o600,
		envFile:                                0o600,
	} {
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatal(statErr)
		}
		if got := info.Mode().Perm(); got != wantMode {
			t.Fatalf("%s mode = %o, want %o", path, got, wantMode)
		}
	}
}

func TestInstallSHActionRunnerConfigFailureLeavesOnlyCompleteFiles(t *testing.T) {
	for _, test := range []struct {
		name            string
		failPattern     string
		wantToken       string
		wantEnvironment string
		wantDiagnostic  string
	}{
		{
			name:            "credential sync failure preserves predecessor",
			failPattern:     ".replacement-token.",
			wantToken:       "old-runner-token\n",
			wantEnvironment: "old-runner-environment\n",
			wantDiagnostic:  "atomically persist the action-runner credential",
		},
		{
			name:            "environment sync failure preserves complete environment",
			failPattern:     ".runner-env.",
			wantToken:       "new-runner-token\n",
			wantEnvironment: "old-runner-environment\n",
			wantDiagnostic:  "atomically persist the action-runner environment",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			configDir := filepath.Join(root, "config")
			stateDir := filepath.Join(root, "state")
			if err := os.MkdirAll(configDir, 0o700); err != nil {
				t.Fatal(err)
			}
			tokenPath := filepath.Join(configDir, "token")
			envPath := filepath.Join(configDir, "runner.env")
			if err := os.WriteFile(tokenPath, []byte("old-runner-token\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(envPath, []byte("old-runner-environment\n"), 0o600); err != nil {
				t.Fatal(err)
			}

			script := `
set -u
ACTION_RUNNER_CONFIG_DIR="` + configDir + `"
ACTION_RUNNER_STATE_DIR="` + stateDir + `"
ACTION_RUNNER_TOKEN_FILE="` + tokenPath + `"
ACTION_RUNNER_ENV_FILE="` + envPath + `"
ACTION_RUNNER_HEALTH_FILE="` + filepath.Join(stateDir, "health.json") + `"
ACTION_RUNNER_ACTIVATION_NONCE="` + strings.Repeat("a", 64) + `"
STATE_DIR="` + filepath.Join(root, "collector") + `"
HOSTNAME_OVERRIDE="secure-runtime.lab"
AGENT_ID="agent-durable"
ACTION_TOKEN="new-runner-token"
PULSE_URL="https://pulse.example"
SERVER_FINGERPRINT=""
CURL_CA_BUNDLE=""
INSECURE="false"
EXIT_GENERAL=1
EXIT_MISSING_ARGS=2
FAIL_PATTERN="` + test.failPattern + `"
fail() { printf '%s\n' "$1" >&2; exit "$2"; }
install() { local destination="${!#}"; mkdir -p "$destination"; chmod 0700 "$destination"; }
chown() { return 0; }
stat() {
    case "$1:$2" in
        '-c:%u') printf '0\n' ;;
        '-c:%a') printf '600\n' ;;
        *) command stat "$@" ;;
    esac
}
sync() {
    if [[ "${2:-}" == *"$FAIL_PATTERN"* ]]; then
        return 1
    fi
    command sync "$@"
}
` + extractInstallShellFunction(t, "write_action_runner_env_value") + `
` + extractInstallShellFunction(t, "action_runner_url_uses_loopback_http") + `
` + extractInstallShellFunction(t, "action_runner_url_transport_allowed") + `
` + extractInstallShellFunction(t, "resolve_action_runner_agent_id") + `
` + extractInstallShellFunction(t, "persist_action_runner_replacement_token") + `
` + extractInstallShellFunction(t, "write_action_runner_config") + `
write_action_runner_config
`
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if err == nil || !strings.Contains(string(out), test.wantDiagnostic) {
				t.Fatalf("fault injection did not fail closed: %v\n%s", err, out)
			}
			if strings.Contains(string(out), "new-runner-token") {
				t.Fatalf("credential leaked in failure output: %s", out)
			}
			for path, want := range map[string]string{tokenPath: test.wantToken, envPath: test.wantEnvironment} {
				got, readErr := os.ReadFile(path)
				if readErr != nil {
					t.Fatalf("read %s: %v", path, readErr)
				}
				if string(got) != want {
					t.Fatalf("%s = %q, want complete %q", path, got, want)
				}
			}
			for _, pattern := range []string{".replacement-token.*", ".runner-env.*"} {
				matches, globErr := filepath.Glob(filepath.Join(configDir, pattern))
				if globErr != nil {
					t.Fatal(globErr)
				}
				if len(matches) != 0 {
					t.Fatalf("temporary authority files were retained: %v", matches)
				}
			}
		})
	}
}

func TestInstallSHActionRunnerRejectsGenericInsecureHTTPSBeforeCredentialWrite(t *testing.T) {
	root := t.TempDir()
	tokenPath := filepath.Join(root, "config", "token")
	script := `
set -u
ACTION_RUNNER_CONFIG_DIR="` + filepath.Join(root, "config") + `"
ACTION_RUNNER_STATE_DIR="` + filepath.Join(root, "state") + `"
ACTION_RUNNER_TOKEN_FILE="` + tokenPath + `"
ACTION_RUNNER_ENV_FILE="` + filepath.Join(root, "config", "runner.env") + `"
ACTION_RUNNER_HEALTH_FILE="` + filepath.Join(root, "state", "health.json") + `"
ACTION_RUNNER_ACTIVATION_NONCE="` + strings.Repeat("a", 64) + `"
STATE_DIR="` + filepath.Join(root, "collector") + `"
HOSTNAME_OVERRIDE="host.local"
AGENT_ID="agent-1"
ACTION_TOKEN="must-not-be-written"
PULSE_URL="https://pulse.example"
SERVER_FINGERPRINT=""
CURL_CA_BUNDLE=""
INSECURE="true"
EXIT_GENERAL=1
EXIT_MISSING_ARGS=2
fail() { printf '%s\n' "$1" >&2; exit "$2"; }
install() { local destination="${!#}"; mkdir -p "$destination"; chmod 0700 "$destination"; }
` + extractInstallShellFunction(t, "write_action_runner_env_value") + `
` + extractInstallShellFunction(t, "action_runner_url_uses_loopback_http") + `
` + extractInstallShellFunction(t, "action_runner_url_transport_allowed") + `
` + extractInstallShellFunction(t, "persist_action_runner_replacement_token") + `
` + extractInstallShellFunction(t, "write_action_runner_config") + `
write_action_runner_config
`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err == nil || !strings.Contains(string(out), "refuses generic insecure HTTPS") {
		t.Fatalf("generic insecure HTTPS was not rejected: %v\n%s", err, out)
	}
	if _, statErr := os.Stat(tokenPath); !os.IsNotExist(statErr) {
		t.Fatalf("runner credential was written before transport rejection: %v", statErr)
	}
}

func TestInstallSHActionRunnerDirectIdentityOverridesStaleCollectorFile(t *testing.T) {
	stateDir := t.TempDir()
	mustWrite(t, filepath.Join(stateDir, "agent-id"), "stale-agent-id\n")
	script := `
set -euo pipefail
STATE_DIR="` + stateDir + `"
AGENT_ID="current-agent-id"
` + extractInstallShellFunction(t, "resolve_action_runner_agent_id") + `
resolve_action_runner_agent_id
`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("resolve action-runner identity: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "current-agent-id" {
		t.Fatalf("resolved action-runner identity = %q, want direct binding", got)
	}
}

func TestInstallSHActionRunnerPersistsResolvedIdentityOutsideCollectorState(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "collector-state")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	identityPath := filepath.Join(stateDir, "agent-id")
	if err := os.WriteFile(identityPath, []byte("agent-canonical\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	pulseAgent := buildPulseAgentLifecycleBinary(t)
	envFile := filepath.Join(root, "runner-config", "runner.env")
	script := `
set -euo pipefail
COLLECTOR_LIFECYCLE_BINARY_PATH="` + pulseAgent + `"
LEAST_PRIVILEGE_USER="$(id -un)"
INSTALL_DIR="` + root + `"
BINARY_NAME="legacy-agent"
TMP_BIN=""
ACTION_RUNNER_CONFIG_DIR="` + filepath.Join(root, "runner-config") + `"
ACTION_RUNNER_STATE_DIR="` + filepath.Join(root, "runner-state") + `"
ACTION_RUNNER_TOKEN_FILE="` + filepath.Join(root, "runner-config", "token") + `"
ACTION_RUNNER_ENV_FILE="` + envFile + `"
ACTION_RUNNER_HEALTH_FILE="` + filepath.Join(root, "runner-state", "health.json") + `"
ACTION_RUNNER_ACTIVATION_NONCE="` + strings.Repeat("a", 64) + `"
STATE_DIR="` + stateDir + `"
HOSTNAME_OVERRIDE="secure-runtime.lab"
AGENT_ID=""
ACTION_TOKEN="runner-secret"
PULSE_URL="https://pulse.example"
SERVER_FINGERPRINT=""
CURL_CA_BUNDLE=""
INSECURE="false"
EXIT_GENERAL=1
EXIT_MISSING_ARGS=2
fail() { printf '%s\n' "$1" >&2; exit "$2"; }
install() { local destination="${!#}"; mkdir -p "$destination"; chmod 0700 "$destination"; }
chown() { return 0; }
stat() {
    case "$1:$2" in
        '-c:%u') printf '0\n' ;;
        '-c:%a') printf '600\n' ;;
        *) command stat "$@" ;;
    esac
}
` + extractInstallShellFunction(t, "collector_lifecycle_binary") + `
` + extractInstallShellFunction(t, "read_action_runner_env_value") + `
` + extractInstallShellFunction(t, "resolve_action_runner_agent_id") + `
` + extractInstallShellFunction(t, "write_action_runner_env_value") + `
` + extractInstallShellFunction(t, "action_runner_url_uses_loopback_http") + `
` + extractInstallShellFunction(t, "action_runner_url_transport_allowed") + `
` + extractInstallShellFunction(t, "persist_action_runner_replacement_token") + `
` + extractInstallShellFunction(t, "write_action_runner_config") + `
write_action_runner_config
printf 'attacker-rewrite\n' > "${STATE_DIR}/agent-id"
grep '^PULSE_AGENT_RUNNER_AGENT_ID="agent-canonical"$' "$ACTION_RUNNER_ENV_FILE"
if grep -q 'PULSE_AGENT_RUNNER_AGENT_ID_FILE' "$ACTION_RUNNER_ENV_FILE"; then exit 9; fi
`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("persist action-runner identity: %v\n%s", err, out)
	}
}

func TestInstallSHSafeProfileLifecycleUsesVerifiedInstallFilesystemStage(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "pulse-agent")
	tmpSource := filepath.Join(root, "download.tmp")
	staged := filepath.Join(root, ".pulse-agent.safe-profile-new")
	logPath := filepath.Join(root, "binary.log")
	mustWrite(t, legacy, "#!/bin/sh\nprintf 'legacy:%s\\n' \"$*\" >> \""+logPath+"\"\nexit 91\n")
	mustWrite(t, tmpSource, "verified bytes that cannot execute on a noexec mount\n")
	if err := os.Chmod(tmpSource, 0o644); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, staged, "#!/bin/sh\nprintf 'staged:%s\\n' \"$*\" >> \""+logPath+"\"\nexit 0\n")
	script := `
set -euo pipefail
INSTALL_DIR="` + root + `"
BINARY_NAME="pulse-agent"
TMP_BIN="` + tmpSource + `"
COLLECTOR_LIFECYCLE_BINARY_PATH="` + staged + `"
` + extractInstallShellFunction(t, "collector_lifecycle_binary") + `
resolved=$(collector_lifecycle_binary)
[[ "$resolved" == "` + staged + `" ]]
"$resolved" collector-reduce-authority
`
	if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("select staged lifecycle binary: %v\n%s", err, out)
	}
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(logData), "legacy:") || !strings.Contains(string(logData), "staged:collector-reduce-authority") {
		t.Fatalf("lifecycle binary selection log: %s", logData)
	}
	installScript, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(installScript)
	stageAt := strings.Index(source, `install -o root -g root -m 0755 "$TMP_BIN" "$SAFE_PROFILE_STAGED_COLLECTOR"`)
	reduceAt := strings.Index(source, `reduce_safe_profile_collector_authority ||`)
	moveAt := strings.Index(source, `mv "$SAFE_PROFILE_STAGED_COLLECTOR" "${INSTALL_DIR}/${BINARY_NAME}"`)
	if stageAt < 0 || reduceAt <= stageAt || moveAt <= reduceAt {
		t.Fatalf("safe-profile staged lifecycle ordering invalid: stage=%d reduce=%d move=%d", stageAt, reduceAt, moveAt)
	}
}

func TestInstallSHActionRunnerReadinessRequiresCurrentActivationNonce(t *testing.T) {
	root := t.TempDir()
	healthPath := filepath.Join(root, "health.json")
	currentNonce := strings.Repeat("a", 64)
	priorNonce := strings.Repeat("b", 64)
	writeHealth := func(activated bool, nonce string) {
		t.Helper()
		payload := fmt.Sprintf(`{"registered":true,"activated":%t,"activation_nonce":%q,"host_id":"agent-1"}`, activated, nonce)
		if err := os.WriteFile(healthPath, []byte(payload), 0o600); err != nil {
			t.Fatal(err)
		}
		future := time.Now().Add(24 * time.Hour)
		if err := os.Chtimes(healthPath, future, future); err != nil {
			t.Fatal(err)
		}
	}
	check := func(nonce string) bool {
		t.Helper()
		script := `
set -euo pipefail
ACTION_RUNNER_HEALTH_FILE="` + healthPath + `"
stat() {
    case "$1" in
        -c) case "$2" in '%u') printf '0\n' ;; '%a') printf '600\n' ;; esac ;;
    esac
}
` + extractInstallShellFunction(t, "action_runner_health_matches_activation") + `
action_runner_health_matches_activation "agent-1" "` + nonce + `"
`
		return exec.Command("bash", "-c", script).Run() == nil
	}

	writeHealth(true, priorNonce)
	if check(currentNonce) {
		t.Fatal("stale marker from a prior activation nonce was accepted")
	}
	writeHealth(false, currentNonce)
	if check(currentNonce) {
		t.Fatal("registered but uncommitted marker was accepted")
	}
	writeHealth(true, currentNonce)
	if !check(currentNonce) {
		t.Fatal("current activated marker was rejected because of filesystem clock skew")
	}
	if err := os.Remove(healthPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "missing"), healthPath); err != nil {
		t.Fatal(err)
	}
	if check(currentNonce) {
		t.Fatal("symlinked activation marker was accepted")
	}
}

func TestInstallSHActionRunnerPostCommitReadinessFailureRetainsReplacement(t *testing.T) {
	for _, test := range []struct {
		name                  string
		cancelBody            string
		recoveryConfigFailure bool
		stopFailure           bool
		rollback              bool
		tokenWriteBody        string
		wantToken             string
	}{
		{
			name: "durable pending cancellation authorizes rollback", cancelBody: `[[ "$3" == "new-runner-token" ]] || return 1; return 0`, rollback: true,
			tokenWriteBody: `printf 'new-runner-token\n' > "$ACTION_RUNNER_TOKEN_FILE"`, wantToken: "old:token\n",
		},
		{
			name: "activation committed or cancel indeterminate", cancelBody: `[[ "$3" == "new-runner-token" ]] || return 1; return 1`,
			tokenWriteBody: `printf 'new-runner-token\n' > "$ACTION_RUNNER_TOKEN_FILE"`, wantToken: "new-runner-token\n",
		},
		{
			name: "replacement authority state durable write failure", cancelBody: "return 1", recoveryConfigFailure: true,
			tokenWriteBody: `: > "$ACTION_RUNNER_TOKEN_FILE"`, wantToken: "",
		},
		{
			name: "failed stop retains fenced recovery material", cancelBody: "return 91", stopFailure: true,
			tokenWriteBody: `printf 'new-runner-token\n' > "$ACTION_RUNNER_TOKEN_FILE"`, wantToken: "new-runner-token\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			testInstallSHActionRunnerReadinessFailureRetainsReplacement(t, test.cancelBody, test.recoveryConfigFailure, test.stopFailure, test.rollback, test.tokenWriteBody, test.wantToken)
		})
	}
}

func testInstallSHActionRunnerReadinessFailureRetainsReplacement(t *testing.T, cancelBody string, recoveryConfigFailure, stopFailure, rollback bool, tokenWriteBody, wantToken string) {
	t.Helper()
	cancelFunction := `cancel_pending_action_runner_credential() { ` + cancelBody + `; }`
	root := t.TempDir()
	binPath := filepath.Join(root, "bin", "pulse-agent-runner")
	unitPath := filepath.Join(root, "systemd", "pulse-agent-runner.service")
	envPath := filepath.Join(root, "config", "runner.env")
	tokenPath := filepath.Join(root, "config", "token")
	stateDir := filepath.Join(root, "state")
	for _, path := range []string{binPath, unitPath, envPath, tokenPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, path, "old:"+filepath.Base(path)+"\n")
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	newBinary := filepath.Join(root, "new-runner")
	mustWrite(t, newBinary, "new:runner\n")
	serviceLog := filepath.Join(root, "systemctl.log")

	script := `
set -u
ACTION_RUNNER_NAME="pulse-agent-runner"
ACTION_RUNNER_BINARY_PATH="` + binPath + `"
ACTION_RUNNER_SERVICE_UNIT="` + unitPath + `"
ACTION_RUNNER_ENV_FILE="` + envPath + `"
ACTION_RUNNER_TOKEN_FILE="` + tokenPath + `"
ACTION_RUNNER_HEALTH_FILE="` + filepath.Join(stateDir, "health.json") + `"
ACTION_RUNNER_STATE_DIR="` + stateDir + `"
ACTION_RUNNER_CONFIG_DIR="` + filepath.Dir(tokenPath) + `"
PRIVILEGE_HELPER_DIR="` + filepath.Join(root, "helper") + `"
TMP_ACTION_RUNNER_BIN="` + newBinary + `"
ACTION_TOKEN="new-runner-token"
ACTION_RUNNER_ACTIVATION_NONCE=""
HOSTNAME_OVERRIDE="host-1.local"
STATE_DIR="` + filepath.Join(root, "collector-state") + `"
PULSE_URL="http://127.0.0.1:7655"
INSECURE="true"
CURL_CA_BUNDLE=""
EXIT_GENERAL=1
RECOVERY_CONFIG_FAILURE="` + strconv.FormatBool(recoveryConfigFailure) + `"
STOP_FAILURE="` + strconv.FormatBool(stopFailure) + `"
CONFIG_WRITE_MARKER="` + filepath.Join(root, "config-write-marker") + `"
SERVICE_ACTIVE_MARKER="` + filepath.Join(root, "service-active-marker") + `"
generate_action_runner_activation_nonce() { printf '%064d\n' 0; }
restore_selinux_contexts() { return 0; }
write_action_runner_config() {
    printf 'write-config\n' >> "` + serviceLog + `"
    if [[ -e "$CONFIG_WRITE_MARKER" && "$RECOVERY_CONFIG_FAILURE" == "true" ]]; then
        return 1
    fi
    : > "$CONFIG_WRITE_MARKER"
    ` + tokenWriteBody + `
    printf 'new:env\n' > "$ACTION_RUNNER_ENV_FILE"
}
render_action_runner_service_unit() { printf 'new:unit\n' > "$1"; }
install() {
    local source="${@: -2:1}"
    local destination="${@: -1}"
    mkdir -p "$(dirname "$destination")"
    cp "$source" "$destination"
}
chown() { return 0; }
chmod() { return 0; }
stat() {
    case "$1:$2" in
        '-c:%u') printf '0\n' ;;
        '-c:%a') printf '600\n' ;;
        *) command stat "$@" ;;
    esac
}
systemctl() {
    printf '%s\n' "$*" >> "` + serviceLog + `"
    case "$1" in
        restart) : > "$SERVICE_ACTIVE_MARKER" ;;
        stop)
            if [[ "$STOP_FAILURE" == "true" ]]; then
                return 1
            fi
            rm -f "$SERVICE_ACTIVE_MARKER"
            ;;
        is-active) [[ -e "$SERVICE_ACTIVE_MARKER" ]] ; return ;;
    esac
    return 0
}
action_runner_health_matches_activation() { return 1; }
resolve_action_runner_agent_id() { printf 'agent-1\n'; }
` + cancelFunction + `
sleep() { return 0; }
log_error() { printf 'ERROR: %s\n' "$1" >&2; }
log_info() { printf 'INFO: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n' "$1" >&2; exit "$2"; }
` + extractInstallShellFunction(t, "provision_action_runner") + `
provision_action_runner
`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err == nil {
		t.Fatalf("post-commit readiness failure unexpectedly succeeded:\n%s", out)
	}
	if rollback {
		if !strings.Contains(string(out), "rolling back runner-only files") || strings.Contains(string(out), "retained durably") {
			t.Fatalf("204 cancellation did not exclusively authorize predecessor restore:\n%s", out)
		}
	} else if stopFailure {
		if !strings.Contains(string(out), "could not confirm the replacement is inactive") ||
			strings.Contains(string(out), "did not durably confirm cancellation") {
			t.Fatalf("failed stop did not retain fenced recovery state before classification:\n%s", out)
		}
	} else if recoveryConfigFailure {
		if strings.Contains(string(out), "retained durably") || !strings.Contains(string(out), "re-enrollment") {
			t.Fatalf("durable authority-state failure did not fail closed for re-enrollment:\n%s", out)
		}
	} else if !strings.Contains(string(out), "replacement credential and runtime were retained durably") || !strings.Contains(string(out), "repair") {
		t.Fatalf("missing repair-required result:\n%s", out)
	}
	if !rollback && !stopFailure && !strings.Contains(string(out), "did not durably confirm cancellation") {
		t.Fatalf("missing atomic cancellation diagnostic:\n%s", out)
	}
	wantBinary, wantUnit, wantEnv := "new:runner\n", "new:unit\n", "new:env\n"
	if rollback {
		wantBinary, wantUnit, wantEnv = "old:pulse-agent-runner\n", "old:pulse-agent-runner.service\n", "old:runner.env\n"
	}
	for path, want := range map[string]string{
		binPath:   wantBinary,
		unitPath:  wantUnit,
		envPath:   wantEnv,
		tokenPath: wantToken,
	} {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read retained %s: %v", path, readErr)
		}
		if string(data) != want {
			t.Fatalf("retained %s = %q, want %q", path, data, want)
		}
	}
	backups, err := filepath.Glob(filepath.Join(root, "**", "*.pulse-install-backup.*"))
	if err != nil {
		t.Fatal(err)
	}
	if (recoveryConfigFailure || stopFailure) && len(backups) == 0 {
		t.Fatal("durable authority-state failure removed every predecessor backup before repair")
	}
	if !recoveryConfigFailure && !stopFailure && len(backups) != 0 {
		t.Fatalf("revoked predecessor backups retained: %v", backups)
	}
	serviceCalls, err := os.ReadFile(serviceLog)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(serviceCalls), "stop pulse-agent-runner.service") {
		t.Fatalf("replacement was not stopped before classification: %s", serviceCalls)
	}
	disableAt := strings.Index(string(serviceCalls), "disable pulse-agent-runner.service")
	maskAt := strings.Index(string(serviceCalls), "mask --runtime pulse-agent-runner.service")
	writeAt := strings.Index(string(serviceCalls), "write-config")
	if disableAt < 0 || maskAt < disableAt || writeAt < maskAt {
		t.Fatalf("runner was not disabled and runtime-masked before authority files changed: %s", serviceCalls)
	}
	if (recoveryConfigFailure || stopFailure) && strings.Contains(string(serviceCalls), "enable --now pulse-agent-runner.service") {
		t.Fatalf("runner restarted without a complete durable authority state: %s", serviceCalls)
	}
	serviceLines := "\n" + string(serviceCalls)
	lastMaskAt := strings.LastIndex(serviceLines, "\nmask --runtime pulse-agent-runner.service")
	lastUnmaskAt := strings.LastIndex(serviceLines, "\nunmask --runtime pulse-agent-runner.service")
	if (recoveryConfigFailure || stopFailure) && lastMaskAt < lastUnmaskAt {
		t.Fatalf("runner remained unmasked without a complete durable authority state: %s", serviceCalls)
	}
	if !recoveryConfigFailure && !stopFailure && !strings.Contains(string(serviceCalls), "enable --now pulse-agent-runner.service") {
		t.Fatalf("replacement was not stopped for classification and restarted for repair: %s", serviceCalls)
	}
	if !recoveryConfigFailure && !stopFailure && lastUnmaskAt < lastMaskAt {
		t.Fatalf("complete authority state was not unmasked for restart: %s", serviceCalls)
	}
}

func TestInstallSHActionRunnerAtomicCancelUsesPrivateGoTransportAndRequires204(t *testing.T) {
	const token = "runner-cancel-secret"
	root := t.TempDir()
	runnerBinary := filepath.Join(root, "pulse-agent-runner")
	build := exec.Command("go", "build", "-o", runnerBinary, "./cmd/pulse-agent-runner")
	build.Dir = filepath.Dir(repoFile("go.mod"))
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build action runner: %v\n%s", err, output)
	}
	caSource := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer caSource.Close()
	caFile := filepath.Join(root, "runner-ca.pem")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caSource.Certificate().Raw}), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		statusCode int
		wantOK     bool
	}{{"durable cancellation", http.StatusNoContent, true}, {"activation committed", http.StatusConflict, false}} {
		t.Run(tc.name, func(t *testing.T) {
			var gotAuthorization, gotMethod, gotBody string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuthorization = r.Header.Get("Authorization")
				gotMethod = r.Method
				body, _ := io.ReadAll(r.Body)
				gotBody = string(body)
				w.WriteHeader(tc.statusCode)
			}))
			defer server.Close()
			configDir := filepath.Join(root, strings.ReplaceAll(tc.name, " ", "-"))
			if err := os.MkdirAll(configDir, 0o700); err != nil {
				t.Fatal(err)
			}
			argsLog := filepath.Join(configDir, "args.log")
			wrapper := filepath.Join(configDir, "runner-wrapper")
			mustWrite(t, wrapper, "#!/bin/bash\nprintf '%s\\n' \"$@\" > \""+argsLog+"\"\nexec \""+runnerBinary+"\" \"$@\"\n")
			if err := os.Chmod(wrapper, 0o755); err != nil {
				t.Fatal(err)
			}
			script := `
set -euo pipefail
ACTION_RUNNER_BINARY_PATH="` + wrapper + `"
ACTION_RUNNER_CONFIG_DIR="` + configDir + `"
PULSE_URL="` + server.URL + `"
CURL_CA_BUNDLE="` + caFile + `"
SERVER_FINGERPRINT="` + strings.Repeat("ab", 32) + `"
chown() { return 0; }
sync() { return 0; }
` + extractInstallShellFunction(t, "action_runner_url_uses_loopback_http") + `
` + extractInstallShellFunction(t, "action_runner_url_transport_allowed") + `
` + extractInstallShellFunction(t, "cancel_pending_action_runner_credential") + `
cancel_pending_action_runner_credential "agent-1" "host-1.local" "` + token + `"
`
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if (err == nil) != tc.wantOK {
				t.Fatalf("cancel result error=%v, want success=%v\n%s", err, tc.wantOK, out)
			}
			if gotAuthorization != "Bearer "+token || gotMethod != http.MethodDelete || gotBody != "" {
				t.Fatalf("cancel request auth=%q method=%q body=%q", gotAuthorization, gotMethod, gotBody)
			}
			if strings.Contains(string(out), token) {
				t.Fatalf("cancel credential leaked in output: %s", out)
			}
			args, readErr := os.ReadFile(argsLog)
			if readErr != nil || !strings.Contains(string(args), "--cacert\n"+caFile) || !strings.Contains(string(args), "--server-fingerprint\n"+strings.Repeat("ab", 32)) {
				t.Fatalf("runner lifecycle CA/pin args = %q, error=%v", args, readErr)
			}
		})
	}
}

func TestInstallSHActionRunnerSelfRevokeUsesPrivateCredential(t *testing.T) {
	const token = "runner-secret-that-must-not-appear-in-output"
	var gotAuthorization string
	var gotMethod string
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode revoke body: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	root := t.TempDir()
	runnerBinary := filepath.Join(root, "pulse-agent-runner")
	build := exec.Command("go", "build", "-o", runnerBinary, "./cmd/pulse-agent-runner")
	build.Dir = filepath.Dir(repoFile("go.mod"))
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build action runner: %v\n%s", err, output)
	}
	envFile := filepath.Join(root, "runner.env")
	tokenFile := filepath.Join(root, "token")
	mustWrite(t, tokenFile, token+"\n")
	if err := os.Chmod(tokenFile, 0o600); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, envFile, strings.Join([]string{
		`PULSE_URL="` + server.URL + `"`,
		`PULSE_AGENT_RUNNER_HOSTNAME="secure-runtime.lab"`,
		`PULSE_AGENT_RUNNER_AGENT_ID="agent-secure-runtime"`,
		`PULSE_AGENT_RUNNER_AGENT_ID_FILE="` + filepath.Join(root, "missing-agent-id") + `"`,
		`PULSE_AGENT_RUNNER_TOKEN_FILE="` + tokenFile + `"`,
		`PULSE_INSECURE="true"`,
	}, "\n")+"\n")

	script := `
set -euo pipefail
ACTION_RUNNER_ENV_FILE="` + envFile + `"
ACTION_RUNNER_BINARY_PATH="` + runnerBinary + `"
stat() {
  if [[ "$1" == "-c" && "$2" == "%a" ]]; then printf '600\n'; return 0; fi
  command stat "$@"
}
` + extractInstallShellFunction(t, "read_action_runner_env_value") + `
` + extractInstallShellFunction(t, "action_runner_url_uses_loopback_http") + `
` + extractInstallShellFunction(t, "action_runner_url_transport_allowed") + `
` + extractInstallShellFunction(t, "revoke_action_runner_credential") + `
revoke_action_runner_credential
printf 'revoked\n'
`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("runner self-revoke: %v\n%s", err, out)
	}
	if strings.Contains(string(out), token) {
		t.Fatalf("runner credential leaked in output: %s", out)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("revoke method = %q, want DELETE", gotMethod)
	}
	if gotAuthorization != "Bearer "+token {
		t.Fatalf("authorization header mismatch")
	}
	if gotBody["agentId"] != "agent-secure-runtime" || gotBody["hostname"] != "secure-runtime.lab" {
		t.Fatalf("revoke body = %#v", gotBody)
	}
}

func TestInstallSHActionRunnerUninstallRetainsRecoveryMaterialWhenRevokeIsUnconfirmed(t *testing.T) {
	root := t.TempDir()
	paths := []string{
		filepath.Join(root, "bin", "pulse-agent-runner"),
		filepath.Join(root, "systemd", "pulse-agent-runner.service"),
		filepath.Join(root, "config", "runner.env"),
		filepath.Join(root, "config", "token"),
		filepath.Join(root, "state", "health.json"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, path, "retained\n")
	}
	serviceLog := filepath.Join(root, "systemctl.log")
	script := `
set -u
ACTION_RUNNER_NAME="pulse-agent-runner"
ACTION_RUNNER_BINARY_PATH="` + paths[0] + `"
ACTION_RUNNER_SERVICE_UNIT="` + paths[1] + `"
ACTION_RUNNER_ENV_FILE="` + paths[2] + `"
ACTION_RUNNER_TOKEN_FILE="` + paths[3] + `"
ACTION_RUNNER_CONFIG_DIR="` + filepath.Dir(paths[3]) + `"
ACTION_RUNNER_STATE_DIR="` + filepath.Dir(paths[4]) + `"
EXIT_GENERAL=1
systemctl() { printf '%s\n' "$*" >> "` + serviceLog + `"; return 0; }
revoke_action_runner_credential() { return 1; }
log_error() { printf 'ERROR: %s\n' "$1" >&2; }
log_info() { printf 'INFO: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n' "$1" >&2; exit "$2"; }
` + extractInstallShellFunction(t, "teardown_action_runner_service") + `
teardown_action_runner_service
`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err == nil || !strings.Contains(string(out), "successful credential revocation") || !strings.Contains(string(out), "retained for a safe retry") {
		t.Fatalf("unconfirmed revoke did not fail closed: %v\n%s", err, out)
	}
	for _, path := range paths {
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("recovery material %s was removed: %v", path, statErr)
		}
	}
	serviceCalls, readErr := os.ReadFile(serviceLog)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(serviceCalls), "stop pulse-agent-runner.service") || !strings.Contains(string(serviceCalls), "disable pulse-agent-runner.service") {
		t.Fatalf("runner was not stopped and disabled before failed revoke: %s", serviceCalls)
	}
}

func TestInstallSHSafeProfileDurablyReducesCollectorAuthority(t *testing.T) {
	const token = "collector-secret-that-must-not-appear-in-output"
	var gotAuthorization string
	var gotMethod string
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode authority reduction body: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	stateDir := t.TempDir()
	tokenPath := filepath.Join(stateDir, "runtime.token")
	mustWrite(t, tokenPath, token+"\n")
	if err := os.Chmod(tokenPath, 0o640); err != nil {
		t.Fatal(err)
	}
	pulseAgent := buildPulseAgentLifecycleBinary(t)
	script := `
set -euo pipefail
STATE_DIR="` + stateDir + `"
RUNTIME_TOKEN_FILE=""
COLLECTOR_LIFECYCLE_BINARY_PATH="` + pulseAgent + `"
LEAST_PRIVILEGE_USER="$(id -un)"
PULSE_TOKEN=""
AGENT_ID="agent-secure-runtime"
HOSTNAME_OVERRIDE=""
PULSE_URL="` + server.URL + `"
INSECURE="false"
CURL_CA_BUNDLE=""
SERVER_FINGERPRINT=""
hostname() { printf 'Secure-Runtime.Lab.\n'; }
log_info() { printf '%s\n' "$*"; }
` + extractCollectorLifecycleShellFunctions(t, false) + `
` + extractInstallShellFunction(t, "resolve_safe_profile_hostname") + `
` + extractInstallShellFunction(t, "reduce_safe_profile_collector_authority") + `
resolve_safe_profile_hostname
reduce_safe_profile_collector_authority
`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("collector authority reduction: %v\n%s", err, out)
	}
	if strings.Contains(string(out), token) {
		t.Fatalf("collector credential leaked in output: %s", out)
	}
	if gotMethod != http.MethodPost || gotAuthorization != "Bearer "+token {
		t.Fatalf("authority reduction transport: method=%q authorization=%q", gotMethod, gotAuthorization)
	}
	if gotBody["agentId"] != "agent-secure-runtime" || gotBody["hostname"] != "secure-runtime.lab" {
		t.Fatalf("authority reduction body = %#v", gotBody)
	}
}

func TestInstallSHCollectorLifecycleRejectsActiveMITMForgedSuccess(t *testing.T) {
	const token = "collector-mitm-secret"
	var authorizationSeen bool
	mitm := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			authorizationSeen = true
		}
		if r.URL.Path == "/api/agents/collector/reduce-authority" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"agent":{"id":"agent-secure-runtime","hostname":"secure-runtime.lab","lastSeen":"2026-08-30T12:00:02Z"}}`))
	}))
	defer mitm.Close()

	stateDir := t.TempDir()
	tokenPath := filepath.Join(stateDir, "runtime.token")
	mustWrite(t, tokenPath, token)
	if err := os.Chmod(tokenPath, 0o640); err != nil {
		t.Fatal(err)
	}
	pulseAgent := buildPulseAgentLifecycleBinary(t)
	script := `
set -u
STATE_DIR="` + stateDir + `"
RUNTIME_TOKEN_FILE=""
COLLECTOR_LIFECYCLE_BINARY_PATH="` + pulseAgent + `"
LEAST_PRIVILEGE_USER="$(id -un)"
PULSE_TOKEN=""
AGENT_ID="agent-secure-runtime"
HOSTNAME_OVERRIDE="secure-runtime.lab"
PULSE_URL="` + mitm.URL + `"
INSECURE="true"
CURL_CA_BUNDLE=""
SERVER_FINGERPRINT="` + strings.Repeat("00", 32) + `"
log_info() { printf '%s\n' "$*"; }
` + extractCollectorLifecycleShellFunctions(t, true) + `
` + extractInstallShellFunction(t, "reduce_safe_profile_collector_authority") + `
if reduce_safe_profile_collector_authority; then
  printf 'forged reduction accepted\n'
  exit 3
fi
if verify_agent_server_registration "2026-08-30T12:00:01Z"; then
  printf 'forged registration accepted\n'
  exit 4
fi
printf 'forged success rejected\n'
`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("MITM rejection script: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "forged success rejected") {
		t.Fatalf("missing forged-success rejection: %s", out)
	}
	if strings.Contains(string(out), token) {
		t.Fatalf("collector bearer leaked in output: %s", out)
	}
	if authorizationSeen {
		t.Fatal("MITM handler received the collector Authorization header")
	}
}

func TestInstallSHRendersHardenedActionRunnerUnit(t *testing.T) {
	root := t.TempDir()
	unitPath := filepath.Join(root, "pulse-agent-runner.service")
	script := `
		set -euo pipefail
		ACTION_RUNNER_ENV_FILE="/etc/pulse-agent-runner/runner.env"
		ACTION_RUNNER_STATE_DIR="/var/lib/pulse-agent-runner"
` + extractInstallShellFunction(t, "render_action_runner_service_unit") + `
		render_action_runner_service_unit "` + unitPath + `" "/usr/local/lib/pulse-agent/pulse-agent-runner"
	`
	if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("render action runner unit: %v\n%s", err, out)
	}
	content, err := os.ReadFile(unitPath)
	if err != nil {
		t.Fatal(err)
	}
	unit := string(content)
	for _, required := range []string{
		"ExecStart=/usr/local/lib/pulse-agent/pulse-agent-runner",
		"EnvironmentFile=/etc/pulse-agent-runner/runner.env",
		"User=root",
		"Group=root",
		"NoNewPrivileges=true",
		"ProtectHome=true",
		"ProtectSystem=false",
		"ProtectKernelTunables=true",
		"ProtectKernelModules=true",
		"ProtectControlGroups=true",
		"RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6",
		"ReadWritePaths=/var/lib/pulse-agent-runner",
	} {
		if !strings.Contains(unit, required) {
			t.Fatalf("action runner unit missing %q:\n%s", required, unit)
		}
	}
	if strings.Contains(unit, "PrivateNetwork=true") {
		t.Fatalf("networked action runner cannot use the helper's private-network sandbox:\n%s", unit)
	}
	if strings.Contains(unit, "ProtectSystem=strict") {
		t.Fatalf("host-mutating action runner cannot make the host filesystem read-only:\n%s", unit)
	}
}

func TestInstallSHTypedPrivilegedHelperProtectsCredentialsAfterStateChown(t *testing.T) {
	stateDir := t.TempDir()
	credentialDir := t.TempDir()
	legacyToken := filepath.Join(stateDir, "token")
	protectedToken := filepath.Join(credentialDir, "token")
	for _, path := range []string{legacyToken, protectedToken} {
		if err := os.WriteFile(path, []byte("secret"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	callLog := filepath.Join(t.TempDir(), "calls")
	script := `
		set -euo pipefail
		STATE_DIR="` + stateDir + `"
		PRIVILEGED_HELPER_CREDENTIAL_DIR="` + credentialDir + `"
		RUNTIME_TOKEN_FILE="` + protectedToken + `"
		LEAST_PRIVILEGE_USER="pulse-agent"
		EXIT_GENERAL=1
		CALL_LOG="` + callLog + `"
		fail() { printf 'fail:%s\n' "$1" >> "$CALL_LOG"; return "$2"; }
		chown() { printf 'chown:%s:%s\n' "$1" "$2" >> "$CALL_LOG"; }
		chmod() { printf 'chmod:%s:%s\n' "$1" "$2" >> "$CALL_LOG"; }
		rm() { printf 'rm:%s\n' "$2" >> "$CALL_LOG"; }
` + extractInstallShellFunction(t, "protect_typed_profile_credentials") + `
		protect_typed_profile_credentials
	`
	if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("protect typed-profile credentials: %v\n%s", err, out)
	}

	logBytes, err := os.ReadFile(callLog)
	if err != nil {
		t.Fatal(err)
	}
	log := string(logBytes)
	for _, want := range []string{
		"chown:root:pulse-agent:" + credentialDir,
		"chmod:0750:" + credentialDir,
		"chown:root:pulse-agent:" + protectedToken,
		"chmod:0640:" + protectedToken,
		"rm:" + legacyToken,
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("credential protection log missing %q:\n%s", want, log)
		}
	}
}
