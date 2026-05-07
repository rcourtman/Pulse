package installtests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootInstallScriptVersionFlagRequiresValue(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "install.sh")

	cmd := exec.Command("bash", scriptPath, "--version")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected install.sh --version without value to fail")
	}

	got := string(out)
	if !strings.Contains(got, "Missing value for --version") {
		t.Fatalf("expected friendly missing-value error, got:\n%s", got)
	}
	if strings.Contains(got, "unbound variable") {
		t.Fatalf("expected guarded parser error, got shell failure:\n%s", got)
	}
}

func TestRootInstallScriptArchiveFlagRequiresValue(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "install.sh")

	cmd := exec.Command("bash", scriptPath, "--archive")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected install.sh --archive without value to fail")
	}

	got := string(out)
	if !strings.Contains(got, "--archive requires a local .tar.gz path") {
		t.Fatalf("expected friendly archive missing-value error, got:\n%s", got)
	}
	if strings.Contains(got, "unbound variable") {
		t.Fatalf("expected guarded parser error, got shell failure:\n%s", got)
	}
}

func TestRootInstallScriptArchiveCannotBeUsedWithSource(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "install.sh")

	cmd := exec.Command("bash", scriptPath, "--source", "--archive", "/tmp/pulse-v6.0.0-linux-amd64.tar.gz")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected install.sh --source --archive to fail")
	}

	got := string(out)
	if !strings.Contains(got, "--archive cannot be used with --source") {
		t.Fatalf("expected archive/source conflict error, got:\n%s", got)
	}
}

func TestRootInstallScriptArchiveSupportContract(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read root install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`ARCHIVE_OVERRIDE="${PULSE_ARCHIVE_PATH:-}"`,
		`--archive PATH`,
		`resolve_archive_override()`,
		`infer_release_from_archive_name()`,
		`validate_pulse_binary_architecture()`,
		`ensure_update_disk_headroom()`,
		`UPDATE_MIN_TEMP_FREE_BYTES=$((900 * 1024 * 1024))`,
		`UPDATE_MIN_INSTALL_FREE_BYTES=$((256 * 1024 * 1024))`,
		`ensure_update_disk_headroom "/tmp" "$INSTALL_DIR"`,
		`prefetch_pulse_archive_for_container()`,
		`download_release_archive()`,
		`install_pulse_archive()`,
		`Archive version $inferred_release does not match requested version $FORCE_VERSION`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing archive support contract: %s", needle)
		}
	}
}

func TestRootInstallScriptInfersPrivateProArchiveVersion(t *testing.T) {
	script := `
		set -euo pipefail
` + extractRootInstallShellFunction(t, "infer_release_from_archive_name") + `
		infer_release_from_archive_name /tmp/pulse-v6.0.0-rc.5-linux-amd64.tar.gz
		infer_release_from_archive_name /tmp/pulse-pro-v6.0.0-rc.5-linux-amd64.tar.gz
		infer_release_from_archive_name /tmp/pulse-pro-v6.0.0-linux-arm64.tar.gz
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	got := strings.Fields(string(out))
	want := []string{"v6.0.0-rc.5", "v6.0.0-rc.5", "v6.0.0"}
	if len(got) != len(want) {
		t.Fatalf("versions = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("versions = %#v, want %#v", got, want)
		}
	}
}

func TestRootInstallScriptUpdateDiskHeadroomRejectsSharedLowSpaceFilesystem(t *testing.T) {
	script := `
		set -euo pipefail
		print_error() { :; }
		print_info() { :; }
		print_warn() { :; }
		INSTALL_DIR="/opt/pulse"
		UPDATE_MIN_TEMP_FREE_BYTES=$((100 * 1024))
		UPDATE_MIN_INSTALL_FREE_BYTES=$((80 * 1024))
` + extractRootInstallShellFunction(t, "bytes_to_human") + `
` + extractRootInstallShellFunction(t, "get_available_bytes_for_path") + `
` + extractRootInstallShellFunction(t, "get_filesystem_device_for_path") + `
` + extractRootInstallShellFunction(t, "ensure_update_disk_headroom") + `
		df() {
			if [[ "$1" == "-Pk" ]]; then
				case "$2" in
					/tmp|/opt/pulse)
						printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\n'
						printf '/dev/shared 1000 0 150 0%% /\n'
						return 0
						;;
				esac
			fi
			command df "$@"
		}
		if ensure_update_disk_headroom /tmp /opt/pulse; then
			echo "ensure_update_disk_headroom unexpectedly passed on a shared full filesystem" >&2
			exit 1
		fi
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
}

func TestRootInstallScriptUpdateDiskHeadroomAcceptsSeparateFilesystems(t *testing.T) {
	script := `
		set -euo pipefail
		print_error() { :; }
		print_info() { :; }
		print_warn() { :; }
		INSTALL_DIR="/opt/pulse"
		UPDATE_MIN_TEMP_FREE_BYTES=$((100 * 1024))
		UPDATE_MIN_INSTALL_FREE_BYTES=$((80 * 1024))
` + extractRootInstallShellFunction(t, "bytes_to_human") + `
` + extractRootInstallShellFunction(t, "get_available_bytes_for_path") + `
` + extractRootInstallShellFunction(t, "get_filesystem_device_for_path") + `
` + extractRootInstallShellFunction(t, "ensure_update_disk_headroom") + `
		df() {
			if [[ "$1" == "-Pk" ]]; then
				case "$2" in
					/tmp)
						printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\n'
						printf '/dev/tmp 1000 0 120 0%% /tmp\n'
						return 0
						;;
					/opt/pulse)
						printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\n'
						printf '/dev/root 1000 0 90 0%% /\n'
						return 0
						;;
				esac
			fi
			command df "$@"
		}
		ensure_update_disk_headroom /tmp /opt/pulse
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
}

func TestRootInstallScriptAutoRegisterUsesSecureContractShape(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read root install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`str(data.get("setupToken", ""))`,
		`str(data.get("tokenHint", ""))`,
		`str(data.get("type", ""))`,
		`str(data.get("host", ""))`,
		`str(data.get("url", ""))`,
		`str(data.get("downloadURL", ""))`,
		`str(data.get("scriptFileName", ""))`,
		`str(data.get("command", ""))`,
		`str(data.get("commandWithEnv", ""))`,
		`str(data.get("commandWithoutEnv", ""))`,
		`expires_raw = data.get("expires", "")`,
		`str(expires_raw)`,
		`expiry_state = "live"`,
		`expires_int > int(time.time())`,
		`expected_setup_url = f"{pulse_url}/api/setup-script?host={quote(host, safe='')}&pulse_url={quote(pulse_url, safe='')}&type=pve"`,
		`expected_download_url = f"{pulse_url}/api/setup-script?host={quote(host, safe='')}&pulse_url={quote(pulse_url, safe='')}&setup_token={quote(setup_token, safe='')}&type=pve"`,
		`expected_script_name = "pulse-setup-pve.sh"`,
		`setup_url != expected_setup_url`,
		`setup_download_url != expected_download_url`,
		`setup_script_name != expected_script_name`,
		`not setup_command`,
		`not setup_command_with_env`,
		`not setup_command_without_env`,
		`command_fields = (`,
		`if not _value or expected_setup_url not in _value:`,
		`'if [ "$(id -u)" -eq 0 ]; then' not in _value`,
		`'elif command -v sudo >/dev/null 2>&1; then' not in _value`,
		`if "PULSE_SETUP_TOKEN=" not in _value or setup_token not in _value:`,
		`elif "PULSE_SETUP_TOKEN=" in _value or setup_token in _value:`,
		`not token_hint or token_hint == setup_token`,
		`[[ "$setup_type" != "pve" ]]`,
		`[[ "$setup_host" != "$normalized_host_url" ]]`,
		`[[ "$setup_url" != "$expected_setup_url" ]]`,
		`[[ "$setup_download_url" != "$expected_download_url" ]]`,
		`[[ "$setup_script_name" != "$expected_script_name" ]]`,
		`[[ -z "$setup_command" ]]`,
		`[[ -z "$setup_command_with_env" ]]`,
		`[[ -z "$setup_command_without_env" ]]`,
		`[[ -z "$setup_token_hint" ]]`,
		`[[ "$setup_expiry_state" != "live" ]]`,
		`host, token_id, token_value, server_name, setup_token = sys.argv[1:]`,
		`"tokenId": token_id`,
		`"tokenValue": token_value`,
		`"authToken": setup_token`,
		`"source": "script"`,
		`data.get("action", "")`,
		`data.get("type", "")`,
		`data.get("source", "")`,
		`data.get("host", "")`,
		`data.get("tokenId", "")`,
		`data.get("tokenValue", "")`,
		`data.get("nodeId", "")`,
		`data.get("nodeName", "")`,
		`[[ "$register_status" != "success" ]] || [[ "$register_action" != "use_token" ]] || [[ "$register_type" != "pve" ]] || [[ "$register_source" != "script" ]]`,
		`AUTO_NODE_REGISTERED_NAME="$register_node_name"`,
		`curl --retry 3 --retry-delay 2 -fsS -X POST "$pulse_url/api/setup-script-url" -H "Content-Type: application/json" -d "$setup_payload"`,
		`curl --retry 3 --retry-delay 2 -fsS -X POST "$pulse_url/api/auto-register" -H "Content-Type: application/json" -d "$register_payload"`,
		`token_output=$(pveum user token add pulse-monitor@pve "$token_name" --privsep 1 2>&1)`,
		`pveum aclmod / -token "$token_id" -role PVEAuditor`,
		`pveum aclmod / -token "$token_id" -role PulseMonitor`,
		`pveum aclmod /storage -token "$token_id" -role PVEDatastoreAdmin`,
		`priv_string="$(IFS=,; echo "${extra_privs[*]}")"`,
		`pveum role modify PulseMonitor -privs "$priv_string"`,
		`slug = re.sub(r"[^a-z0-9]+", "-", host)`,
		`print(f"pulse-{slug}")`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("root install.sh missing secure installer auto-register contract fragment: %s", needle)
		}
	}
	if strings.Contains(script, `local token_name="pulse-${pulse_host_slug}-$(date +%s)"`) {
		t.Fatalf("root install.sh preserved stale timestamp-suffixed Proxmox token naming")
	}
	forbidden := []string{
		`local bootstrap_token=""`,
		`X-Setup-Token: $bootstrap_token`,
		`Discovered bootstrap token from container`,
		`--privsep 0`,
		`pveum role delete PulseMonitor`,
	}
	for _, needle := range forbidden {
		if strings.Contains(script, needle) {
			t.Fatalf("root install.sh preserved stale setup-script-url bootstrap auth fragment: %s", needle)
		}
	}
}

func TestRootInstallShowsBootstrapTokenCommandInsteadOfEncryptedFile(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read root install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`pulse bootstrap-token`,
		`PULSE_DATA_DIR=`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("root install.sh missing bootstrap-token display fragment: %s", needle)
		}
	}

	forbidden := []string{
		`cat $CONFIG_DIR/.bootstrap_token`,
		`cat "$TOKEN_FILE"`,
		`Token: ${GREEN}`,
	}
	for _, needle := range forbidden {
		if strings.Contains(script, needle) {
			t.Fatalf("root install.sh still exposes encrypted bootstrap file contents: %s", needle)
		}
	}
}

func TestPrereleaseUpdateCopyUsesPreviewFraming(t *testing.T) {
	rootInstall, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read root install.sh: %v", err)
	}

	installScript := string(rootInstall)
	requiredInstall := []string{
		`Update to $RC_VERSION (prerelease preview)`,
		`--rc, --pre        Install latest prerelease preview version`,
		`Prerelease channel detected in configuration`,
		`Prerelease channel: get latest release (including prereleases, but skip drafts)`,
	}
	for _, needle := range requiredInstall {
		if !strings.Contains(installScript, needle) {
			t.Fatalf("root install.sh missing prerelease framing fragment: %s", needle)
		}
	}
	forbiddenInstall := []string{
		`Update to $RC_VERSION (release candidate)`,
		`--rc, --pre        Install latest RC/pre-release version`,
		`RC channel detected in configuration`,
		`RC channel: Get latest release (including pre-releases, but skip drafts)`,
	}
	for _, needle := range forbiddenInstall {
		if strings.Contains(installScript, needle) {
			t.Fatalf("root install.sh preserved stale release-candidate framing fragment: %s", needle)
		}
	}

	autoUpdate, err := os.ReadFile(filepath.Join("..", "..", "scripts", "pulse-auto-update.sh"))
	if err != nil {
		t.Fatalf("read pulse-auto-update.sh: %v", err)
	}
	autoUpdateScript := string(autoUpdate)
	if !strings.Contains(autoUpdateScript, `Prerelease channel detected; unattended auto-updates run only on stable`) {
		t.Fatalf("pulse-auto-update.sh missing prerelease channel log message")
	}
	if strings.Contains(autoUpdateScript, `RC channel detected; unattended auto-updates run only on stable`) {
		t.Fatalf("pulse-auto-update.sh preserved stale release-candidate channel log message")
	}
}

func TestRootInstallScriptSupportsInstanceScopedServerInstalls(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read root install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`SERVICE_NAME_EXPLICIT="false"`,
		`SERVICE_NAME="${PULSE_SERVICE_NAME:-$DEFAULT_SERVICE_NAME}"`,
		`INSTALL_DIR="${PULSE_INSTALL_DIR:-$(default_install_dir_for_service "$SERVICE_NAME")}"`,
		`CONFIG_DIR="${PULSE_CONFIG_DIR:-$(default_config_dir_for_service "$SERVICE_NAME")}"`,
		`BINARY_LINK_PATH="${PULSE_BINARY_LINK_PATH:-$(default_binary_link_path_for_service "$SERVICE_NAME")}"`,
		`UPDATE_HELPER_PATH="${PULSE_UPDATE_HELPER_PATH:-$(default_update_helper_path_for_service "$SERVICE_NAME")}"`,
		`AUTO_UPDATE_DEST="${PULSE_AUTO_UPDATE_DEST:-$(default_auto_update_dest_for_service "$SERVICE_NAME")}"`,
		`UPDATE_SERVICE_PATH="${PULSE_UPDATE_SERVICE_PATH:-$(default_update_service_path_for_service "$SERVICE_NAME")}"`,
		`UPDATE_TIMER_PATH="${PULSE_UPDATE_TIMER_PATH:-$(default_update_timer_path_for_service "$SERVICE_NAME")}"`,
		`if [[ "$SERVICE_NAME_EXPLICIT" == "true" ]]; then`,
		`mkdir -p "$(dirname "$BINARY_LINK_PATH")"`,
		`ln -sf "$INSTALL_DIR/bin/pulse" "$BINARY_LINK_PATH"`,
		`safe_systemctl enable "$update_timer_unit" || true`,
		`safe_systemctl start "$update_timer_unit" || true`,
		`Environment="PULSE_SERVICE_NAME=$service_name"`,
		`Environment="PULSE_INSTALL_DIR=$install_dir"`,
		`Environment="PULSE_CONFIG_DIR=$config_dir"`,
		`Environment="PULSE_UPDATE_TIMER_UNIT=$update_timer_unit"`,
		`local update_helper_path="${UPDATE_HELPER_PATH:-${PULSE_UPDATE_HELPER_PATH:-/bin/update}}"`,
		`printf '%q' "$update_helper_path"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("root install.sh missing instance-scoped install contract fragment: %s", needle)
		}
	}
}

func TestRootInstallScriptRequiresSignedReleaseDownloads(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read root install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`PINNED_RELEASE_SSH_PUBLIC_KEY="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer"`,
		`require_release_signature_verifier() {`,
		`verify_release_signature() {`,
		`local signature_url="${download_url}.sshsig"`,
		`Failed to download signature for Pulse release`,
		`verify_release_signature "$archive_path" "$signature_file" "downloaded Pulse release"`,
		`INSTALLER_SIG_URL="\${INSTALLER_URL}.sshsig"`,
		`verify_release_signature "\$tmp_installer" "\$tmp_signature" "downloaded Pulse installer"`,
		`Failed to download signature for pulse-auto-update.sh`,
		`verify_release_signature "$dest" "$signature_file" "downloaded pulse-auto-update.sh"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("root install.sh missing signed-release verification contract: %s", needle)
		}
	}
}

func TestPulseAutoUpdateScriptSupportsInstanceScopedServerInstalls(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "scripts", "pulse-auto-update.sh"))
	if err != nil {
		t.Fatalf("read pulse-auto-update.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`SERVICE_NAME="${PULSE_SERVICE_NAME:-pulse}"`,
		`INSTALL_DIR="${PULSE_INSTALL_DIR:-/opt/pulse}"`,
		`CONFIG_DIR="${PULSE_CONFIG_DIR:-/etc/pulse}"`,
		`UPDATE_TIMER_UNIT="${PULSE_UPDATE_TIMER_UNIT:-${SERVICE_NAME}-update.timer}"`,
		`if [[ -n "${PULSE_SERVICE_NAME:-}" ]]; then`,
		`"PULSE_SERVICE_NAME=$service_name"`,
		`"PULSE_INSTALL_DIR=$INSTALL_DIR"`,
		`"PULSE_CONFIG_DIR=$CONFIG_DIR"`,
		`systemctl is-enabled --quiet "$UPDATE_TIMER_UNIT"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("pulse-auto-update.sh missing instance-scoped install contract fragment: %s", needle)
		}
	}
}

func TestPulseAutoUpdateScriptRequiresSignedInstallerDownloads(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "scripts", "pulse-auto-update.sh"))
	if err != nil {
		t.Fatalf("read pulse-auto-update.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`PINNED_RELEASE_SSH_PUBLIC_KEY="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer"`,
		`require_release_signature_verifier() {`,
		`verify_release_signature() {`,
		`local install_signature_url="${install_script_url}.sshsig"`,
		`Failed to download installer signature from $install_signature_url`,
		`verify_release_signature "$installer_tmp" "$signature_tmp" "downloaded Pulse installer"`,
		`Installer signature verified`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("pulse-auto-update.sh missing signed-installer verification contract: %s", needle)
		}
	}
}

func TestOperatorInstallDocsAvoidUnverifiedBootstrapAndFloatingImageTags(t *testing.T) {
	files := []string{
		filepath.Join("..", "..", "README.md"),
		filepath.Join("..", "..", "docs", "INSTALL.md"),
		filepath.Join("..", "..", "docs", "UPGRADE_v6.md"),
		filepath.Join("..", "..", "docs", "UPGRADE_v5.md"),
		filepath.Join("..", "..", "docs", "DOCKER.md"),
		filepath.Join("..", "..", "docs", "AUTO_UPDATE.md"),
		filepath.Join("..", "..", "docs", "operations", "AUTO_UPDATE.md"),
		filepath.Join("..", "..", "docs", "FAQ.md"),
	}
	forbidden := []string{
		`curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh |`,
		`curl -sL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh |`,
		`rcourtman/pulse:latest`,
		`docker pull rcourtman/pulse:latest`,
		`image: rcourtman/pulse:latest`,
	}

	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(content)
		for _, needle := range forbidden {
			if strings.Contains(text, needle) {
				t.Fatalf("%s preserved insecure operator guidance: %s", path, needle)
			}
		}
	}
}
