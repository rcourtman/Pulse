package installtests

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

func TestRootInstallSystemdServiceExposesOnlyPulseOwnedSubscriptionCLIHome(t *testing.T) {
	body := extractRootInstallShellFunction(t, "install_systemd_service")
	for _, required := range []string{
		`Environment="HOME=$INSTALL_DIR"`,
		`Environment="PATH=$INSTALL_DIR/.local/bin:$INSTALL_DIR/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"`,
		`User=pulse`,
		`ProtectHome=true`,
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("systemd service missing subscription CLI boundary %q", required)
		}
	}
	if strings.Contains(body, "/root/") {
		t.Fatal("systemd service must not expose a privileged home to local subscription CLIs")
	}
}

func TestRootInstallScriptStagesUpdateBeforeStoppingService(t *testing.T) {
	downloadPulse := extractRootInstallShellFunction(t, "download_pulse")
	orderedSteps := []string{
		`ensure_update_disk_headroom "/tmp" "$INSTALL_DIR"`,
		`download_release_archive "$LATEST_RELEASE" "$pulse_arch" "$archive_path"`,
		`run_upgrade_readiness_preflight "$CURRENT_VERSION" "$expected_release"`,
		`safe_systemctl stop "$EXISTING_SERVICE"`,
		`install_pulse_archive "$archive_path" "$expected_release"`,
	}

	previous := -1
	for _, step := range orderedSteps {
		position := strings.Index(downloadPulse, step)
		if position < 0 {
			t.Fatalf("download_pulse missing update safety step: %s", step)
		}
		if position <= previous {
			t.Fatalf("download_pulse update safety steps are out of order at: %s", step)
		}
		previous = position
	}
}

func TestRootInstallScriptInstallsSignatureVerificationDependencies(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read root install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`apt-get install -y -qq curl wget ca-certificates openssh-client jq`,
		`apt-get install -y -qq curl wget ca-certificates openssh-client`,
		`ssh-keygen is required to verify signed Pulse release assets.`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing signature verification dependency contract: %s", needle)
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

func TestRootInstallScriptConfigBackupHeadroomRejectsLowSpace(t *testing.T) {
	configDir := t.TempDir()
	configParent := filepath.Dir(configDir)

	script := `
		set -euo pipefail
		print_error() { :; }
		print_info() { :; }
		print_warn() { :; }
		CONFIG_DIR="$CONFIG_DIR_UNDER_TEST"
		CONFIG_BACKUP_MIN_EXTRA_BYTES=$((64 * 1024))
` + extractRootInstallShellFunction(t, "bytes_to_human") + `
` + extractRootInstallShellFunction(t, "get_available_bytes_for_path") + `
` + extractRootInstallShellFunction(t, "get_directory_size_bytes") + `
` + extractRootInstallShellFunction(t, "ensure_config_backup_headroom") + `
		df() {
			if [[ "$1" == "-Pk" && "$2" == "$CONFIG_PARENT_UNDER_TEST" ]]; then
				printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\n'
				printf '/dev/root 1000 0 65 0%% /\n'
				return 0
			fi
			command df "$@"
		}
		du() {
			if [[ "$1" == "-sk" && "$2" == "$CONFIG_DIR_UNDER_TEST" ]]; then
				printf '4\t%s\n' "$CONFIG_DIR_UNDER_TEST"
				return 0
			fi
			command du "$@"
		}
		if ensure_config_backup_headroom "$CONFIG_DIR"; then
			echo "ensure_config_backup_headroom unexpectedly passed with no backup margin" >&2
			exit 1
		fi
	`

	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(),
		"CONFIG_DIR_UNDER_TEST="+configDir,
		"CONFIG_PARENT_UNDER_TEST="+configParent,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
}

func TestRootInstallScriptConfigBackupCleansPartialCopy(t *testing.T) {
	configDir := t.TempDir()
	backupDir := configDir + ".backup.20260704-141422"

	script := `
		set -euo pipefail
		print_error() { :; }
		print_info() { :; }
		print_warn() { :; }
		CONFIG_DIR="$CONFIG_DIR_UNDER_TEST"
		CONFIG_BACKUP_MIN_EXTRA_BYTES=$((64 * 1024))
` + extractRootInstallShellFunction(t, "bytes_to_human") + `
` + extractRootInstallShellFunction(t, "get_available_bytes_for_path") + `
` + extractRootInstallShellFunction(t, "get_directory_size_bytes") + `
` + extractRootInstallShellFunction(t, "ensure_config_backup_headroom") + `
` + extractRootInstallShellFunction(t, "backup_existing") + `
		date() { printf '20260704-141422\n'; }
		cp() {
			if [[ "$1" == "-a" && "$2" == "$CONFIG_DIR_UNDER_TEST" ]]; then
				mkdir -p "$3"
				printf 'partial\n' > "$3/partial"
				return 1
			fi
			command cp "$@"
		}
		if backup_existing; then
			echo "backup_existing unexpectedly passed after a failed copy" >&2
			exit 1
		fi
		if [[ -e "$BACKUP_DIR_UNDER_TEST" ]]; then
			echo "partial backup was not removed" >&2
			exit 1
		fi
	`

	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(),
		"CONFIG_DIR_UNDER_TEST="+configDir,
		"BACKUP_DIR_UNDER_TEST="+backupDir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
}

// Issue #1646: unattended updates created one config snapshot per run under
// the hardened-unit fallback directory and never pruned old ones, so small
// root filesystems filled up. backup_existing must rotate snapshots after a
// successful copy.
func TestRootInstallScriptConfigBackupRotatesOldSnapshots(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores write bits, so the read-only fallback path cannot be simulated")
	}
	configDir := t.TempDir()
	installDir := t.TempDir()
	backupParent := filepath.Join(installDir, "config-backups")
	if err := os.MkdirAll(backupParent, 0755); err != nil {
		t.Fatalf("mkdir backup parent: %v", err)
	}
	prefix := filepath.Base(configDir) + ".backup."
	oldStamps := []string{"20260701-020000", "20260702-020000", "20260703-020000", "20260704-020000", "20260705-020000"}
	for _, stamp := range oldStamps {
		if err := os.MkdirAll(filepath.Join(backupParent, prefix+stamp), 0755); err != nil {
			t.Fatalf("mkdir old snapshot: %v", err)
		}
	}

	script := `
		set -euo pipefail
		print_error() { :; }
		print_info() { :; }
		print_warn() { :; }
		CONFIG_DIR="$CONFIG_DIR_UNDER_TEST"
		INSTALL_DIR="$INSTALL_DIR_UNDER_TEST"
		CONFIG_BACKUP_MIN_EXTRA_BYTES=0
` + extractRootInstallShellFunction(t, "bytes_to_human") + `
` + extractRootInstallShellFunction(t, "get_available_bytes_for_path") + `
` + extractRootInstallShellFunction(t, "get_directory_size_bytes") + `
` + extractRootInstallShellFunction(t, "ensure_config_backup_headroom") + `
` + extractRootInstallShellFunction(t, "prune_config_backups") + `
` + extractRootInstallShellFunction(t, "backup_existing") + `
		date() { printf '20260706-020000\n'; }
		chmod a-w "$(dirname "$CONFIG_DIR_UNDER_TEST")" 2>/dev/null || true
		trap 'chmod u+w "$(dirname "$CONFIG_DIR_UNDER_TEST")" 2>/dev/null || true' EXIT
		backup_existing
	`

	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(),
		"CONFIG_DIR_UNDER_TEST="+configDir,
		"INSTALL_DIR_UNDER_TEST="+installDir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	entries, err := os.ReadDir(backupParent)
	if err != nil {
		t.Fatalf("read backup parent: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if len(names) != 5 {
		t.Fatalf("expected 5 snapshots after rotation, got %d: %v", len(names), names)
	}
	for _, gone := range []string{prefix + "20260701-020000"} {
		for _, name := range names {
			if name == gone {
				t.Fatalf("expected oldest snapshot %s to be pruned, still present: %v", gone, names)
			}
		}
	}
	found := false
	for _, name := range names {
		if name == prefix+"20260706-020000" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected fresh snapshot to exist, got %v", names)
	}
}

func TestRootInstallScriptV5ToV6PreflightWarnsWhenAgentScopeMissing(t *testing.T) {
	configDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDir, "api_tokens.json"), []byte(`[{"id":"tok-1","name":"admin","hash":"hash","scopes":["settings:read"]}]`), 0600); err != nil {
		t.Fatalf("write api_tokens.json: %v", err)
	}

	script := `
		set -euo pipefail
		print_error() { echo "ERROR: $*"; }
		print_info() { echo "INFO: $*"; }
		print_warn() { echo "WARN: $*"; }
		print_success() { echo "SUCCESS: $*"; }
		UPGRADE_PREFLIGHT_RAN=false
		SKIP_UPGRADE_PREFLIGHT=false
` + extractRootInstallShellFunction(t, "version_major") + `
` + extractRootInstallShellFunction(t, "is_pre_v6_to_v6_upgrade") + `
` + extractRootInstallShellFunction(t, "inspect_api_tokens_for_upgrade") + `
` + extractRootInstallShellFunction(t, "run_upgrade_readiness_preflight") + `
		run_upgrade_readiness_preflight v5.1.23 v6.0.0
	`

	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), "CONFIG_DIR="+configDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "No agent reporting token scope was found") {
		t.Fatalf("expected missing-scope warning, got:\n%s", out)
	}
}

func TestRootInstallScriptV5ToV6PreflightAcceptsLegacyHostAgentScope(t *testing.T) {
	configDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDir, "api_tokens.json"), []byte(`[{"id":"tok-1","name":"agent","hash":"hash","scopes":["host-agent:report"]}]`), 0600); err != nil {
		t.Fatalf("write api_tokens.json: %v", err)
	}

	script := `
		set -euo pipefail
		print_error() { echo "ERROR: $*"; }
		print_info() { echo "INFO: $*"; }
		print_warn() { echo "WARN: $*"; }
		print_success() { echo "SUCCESS: $*"; }
		UPGRADE_PREFLIGHT_RAN=false
		SKIP_UPGRADE_PREFLIGHT=false
` + extractRootInstallShellFunction(t, "version_major") + `
` + extractRootInstallShellFunction(t, "is_pre_v6_to_v6_upgrade") + `
` + extractRootInstallShellFunction(t, "inspect_api_tokens_for_upgrade") + `
` + extractRootInstallShellFunction(t, "run_upgrade_readiness_preflight") + `
		run_upgrade_readiness_preflight v5.1.23 v6.0.0
	`

	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), "CONFIG_DIR="+configDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Agent reporting token scope is present") {
		t.Fatalf("expected success output, got:\n%s", out)
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
		`pveum role add PulseGuestFileReadProbe -privs VM.GuestAgent.FileRead`,
		`extra_privs+=("VM.GuestAgent.Audit")`,
		`extra_privs+=("VM.GuestAgent.FileRead")`,
		`extra_privs+=("VM.Monitor")`,
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
	guestBranch := strings.Index(script, `if [[ "$has_guest_audit" == true ]]; then`)
	monitorBranch := strings.Index(script, `if [[ "$has_vm_monitor" == true ]]; then`)
	if guestBranch < 0 || monitorBranch < 0 || guestBranch > monitorBranch {
		t.Fatalf("root install.sh must prefer VM.GuestAgent.* privileges before legacy VM.Monitor")
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

func TestCanonicalServerDeploymentMethodsAreStampedForTelemetry(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	required := map[string]string{
		"Dockerfile":         "ENV PULSE_DEPLOYMENT_METHOD=container_other",
		"docker-compose.yml": "PULSE_DEPLOYMENT_METHOD=docker_compose",
		"install.sh":         `Environment="PULSE_DEPLOYMENT_METHOD=systemd"`,
		"README.md":          "PULSE_DEPLOYMENT_METHOD=docker_run",
	}
	for relativePath, marker := range required {
		content, err := os.ReadFile(filepath.Join(repoRoot, relativePath))
		if err != nil {
			t.Fatalf("read %s: %v", relativePath, err)
		}
		if !strings.Contains(string(content), marker) {
			t.Errorf("%s must stamp coarse deployment method %q", relativePath, marker)
		}
	}
}

func TestSystemdServerLogsPreservePulseSeverityInJournal(t *testing.T) {
	rootInstall, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read root install.sh: %v", err)
	}

	unitContract := string(rootInstall)
	for _, marker := range []string{
		"StandardOutput=journal",
		"StandardError=journal",
		"SyslogLevelPrefix=true",
		`Environment="PULSE_LOG_JOURNAL_LEVEL_PREFIX=true"`,
	} {
		if !strings.Contains(unitContract, marker) {
			t.Errorf("systemd server unit must include %q", marker)
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
		`install_binary_symlink "$INSTALL_DIR/bin/pulse" "$BINARY_LINK_PATH"`,
		`ln -sf "$target" "$link_path"`,
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

// TestRootInstallDeployAgentScriptsDeploysSignatureSidecars guards the fix for
// the rc.6 "Install on Linux" agent-wizard regression (issue #1470). The
// running server serves /opt/pulse/scripts/install.sh at its /install.sh
// endpoint, but for published releases the handler only serves that local copy
// when its .sig and .sshsig sidecars are present next to it; otherwise it
// proxies the top-level GitHub install.sh asset, which is the SERVER installer
// (not the agent installer) and rejects the wizard's --url/--token-file flags.
// The Docker image deploys these sidecars; deploy_agent_scripts must too.
// TestRootInstallUninstallCleansLegacySensorProxy guards #34: `install.sh
// --uninstall` on a Proxmox host that was upgraded from v5 must remove the
// leftover pulse-sensor-proxy footprint locally — binary, units, runtime/state,
// service user, and (security-relevant) the managed SSH keys in root's
// authorized_keys — so a "complete uninstall" leaves nothing behind. Cluster-
// wide key removal and Proxmox API-user deletion stay behind the explicit
// standalone scripts/uninstall-sensor-proxy.sh, which we only point users to.
func TestRootInstallUninstallCleansLegacySensorProxy(t *testing.T) {
	tmp := t.TempDir()

	binPath := filepath.Join(tmp, "bin", "pulse-sensor-proxy")
	systemdDir := filepath.Join(tmp, "systemd")
	unitPath := filepath.Join(systemdDir, "pulse-sensor-proxy.service")
	authKeys := filepath.Join(tmp, "authorized_keys")
	marker := filepath.Join(tmp, "calls.log")

	for _, dir := range []string{filepath.Dir(binPath), systemdDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	if err := os.WriteFile(unitPath, []byte("[Unit]\n"), 0o644); err != nil {
		t.Fatalf("write unit: %v", err)
	}
	authContent := "ssh-ed25519 AAAA keepme@admin\n" +
		"ssh-ed25519 BBBB # pulse-managed-key\n" +
		"ssh-ed25519 CCCC # pulse-proxy-key\n" +
		"ssh-rsa DDDD keep-this-too\n"
	if err := os.WriteFile(authKeys, []byte(authContent), 0o600); err != nil {
		t.Fatalf("write authorized_keys: %v", err)
	}

	env := `
		set -uo pipefail
		SENSOR_PROXY_BINARY_PATH="` + binPath + `"
		SENSOR_PROXY_SYSTEMD_DIR="` + systemdDir + `"
		SENSOR_PROXY_INSTALL_ROOT="` + filepath.Join(tmp, "sensor-proxy") + `"
		SENSOR_PROXY_RUNTIME_DIR="` + filepath.Join(tmp, "run") + `"
		SENSOR_PROXY_WORK_DIR="` + filepath.Join(tmp, "work") + `"
		SENSOR_PROXY_CONFIG_DIR="` + filepath.Join(tmp, "config") + `"
		SENSOR_PROXY_LOG_DIR="` + filepath.Join(tmp, "log") + `"
		SENSOR_PROXY_SERVICE_USER="pulse-sensor-proxy-test"
		SENSOR_PROXY_AUTHORIZED_KEYS_PATH="` + authKeys + `"
		systemctl() { return 0; }
		userdel() { echo "userdel $*" >>"` + marker + `"; return 0; }
		groupdel() { echo "groupdel $*" >>"` + marker + `"; return 0; }
		id() { return 0; }
		getent() { return 0; }
	`

	funcs := extractRootInstallShellFunction(t, "local_sensor_proxy_present") + "\n" +
		extractRootInstallShellFunction(t, "remove_local_sensor_proxy_managed_keys") + "\n" +
		extractRootInstallShellFunction(t, "cleanup_local_sensor_proxy")

	out, err := exec.Command("bash", "-c", env+funcs+"\ncleanup_local_sensor_proxy\n").CombinedOutput()
	if err != nil {
		t.Fatalf("cleanup_local_sensor_proxy failed: %v\n%s", err, out)
	}

	if _, statErr := os.Stat(binPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected sensor-proxy binary removed, stat err = %v", statErr)
	}
	if _, statErr := os.Stat(unitPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected sensor-proxy unit removed, stat err = %v", statErr)
	}

	keysAfter, err := os.ReadFile(authKeys)
	if err != nil {
		t.Fatalf("read authorized_keys after cleanup: %v", err)
	}
	keysText := string(keysAfter)
	if strings.Contains(keysText, "pulse-managed-key") || strings.Contains(keysText, "pulse-proxy-key") {
		t.Fatalf("expected managed/proxy SSH keys stripped, got:\n%s", keysText)
	}
	for _, keep := range []string{"keepme@admin", "keep-this-too"} {
		if !strings.Contains(keysText, keep) {
			t.Fatalf("expected unrelated SSH key %q preserved, got:\n%s", keep, keysText)
		}
	}

	markerBytes, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("expected userdel/groupdel to run: %v", err)
	}
	if !strings.Contains(string(markerBytes), "userdel") {
		t.Fatalf("expected service user removal, got marker:\n%s", markerBytes)
	}

	if !strings.Contains(string(out), "uninstall-sensor-proxy.sh") {
		t.Fatalf("expected pointer to standalone cluster cleanup script, got:\n%s", out)
	}
	if !strings.Contains(string(out), "--local-only") {
		t.Fatalf("expected standalone cleanup pointer to avoid unprovisioned cluster SSH, got:\n%s", out)
	}

	// Presence-gated: a host with no sensor-proxy footprint is a silent no-op.
	empty := t.TempDir()
	noopEnv := `
		set -uo pipefail
		SENSOR_PROXY_BINARY_PATH="` + filepath.Join(empty, "pulse-sensor-proxy") + `"
		SENSOR_PROXY_SYSTEMD_DIR="` + empty + `"
		SENSOR_PROXY_INSTALL_ROOT="` + filepath.Join(empty, "sensor-proxy") + `"
		SENSOR_PROXY_RUNTIME_DIR="` + filepath.Join(empty, "run") + `"
		SENSOR_PROXY_WORK_DIR="` + filepath.Join(empty, "work") + `"
		SENSOR_PROXY_CONFIG_DIR="` + filepath.Join(empty, "config") + `"
		SENSOR_PROXY_LOG_DIR="` + filepath.Join(empty, "log") + `"
		SENSOR_PROXY_SERVICE_USER="pulse-sensor-proxy-test"
		SENSOR_PROXY_AUTHORIZED_KEYS_PATH="` + filepath.Join(empty, "authorized_keys") + `"
		systemctl() { return 0; }
		userdel() { return 0; }
		groupdel() { return 0; }
		id() { return 0; }
		getent() { return 0; }
	`
	noopOut, err := exec.Command("bash", "-c", noopEnv+funcs+"\ncleanup_local_sensor_proxy\n").CombinedOutput()
	if err != nil {
		t.Fatalf("cleanup_local_sensor_proxy no-op path failed: %v\n%s", err, noopOut)
	}
	if strings.TrimSpace(string(noopOut)) != "" {
		t.Fatalf("expected silent no-op when no footprint present, got:\n%s", noopOut)
	}
}

// TestRootInstallUninstallWiresSensorProxyCleanup pins that uninstall_pulse
// actually invokes the local sensor-proxy cleanup (the functional test above
// only exercises the helper in isolation).
func TestRootInstallUninstallWiresSensorProxyCleanup(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read root install.sh: %v", err)
	}

	uninstall := extractRootInstallShellFunction(t, "uninstall_pulse")
	if !strings.Contains(uninstall, "cleanup_local_sensor_proxy") {
		t.Fatalf("uninstall_pulse does not invoke cleanup_local_sensor_proxy:\n%s", uninstall)
	}

	script := string(content)
	for _, needle := range []string{
		`cleanup_local_sensor_proxy() {`,
		`local_sensor_proxy_present() {`,
		`remove_local_sensor_proxy_managed_keys() {`,
		`# pulse-(managed|proxy)-key$`,
	} {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing sensor-proxy cleanup contract: %s", needle)
		}
	}
}

// TestRootInstallExtractsPveTokenValue guards the #44/#1312 token-extraction
// hardening for the install-time auto-register path: token capture must prefer
// the deterministic `pveum --output-format json` form and parse the `value`
// field, while still recovering from the legacy box-drawing table layout that
// older pveum builds emit, so it does not silently fail or mis-parse when
// pveum's table formatting drifts.
func TestRootInstallExtractsPveTokenValue(t *testing.T) {
	fn := extractRootInstallShellFunction(t, "extract_pve_token_value")

	const secret = "12345678-1234-1234-1234-1234567890ab"

	jsonOutput := `{"full-tokenid":"pulse-monitor@pve!pulse-x","info":{"privsep":"1"},"value":"` + secret + `"}`
	tableOutput := "" +
		"┌──────────────┬──────────────────────────────────────┐\n" +
		"│ key          │ value                                │\n" +
		"╞══════════════╪══════════════════════════════════════╡\n" +
		"│ full-tokenid │ pulse-monitor@pve!pulse-x            │\n" +
		"├──────────────┼──────────────────────────────────────┤\n" +
		"│ info         │ {\"privsep\":\"1\"}                       │\n" +
		"├──────────────┼──────────────────────────────────────┤\n" +
		"│ value        │ " + secret + " │\n" +
		"└──────────────┴──────────────────────────────────────┘\n"

	cases := []struct {
		name   string
		output string
		want   string
	}{
		{"json", jsonOutput, secret},
		{"table", tableOutput, secret},
		{"garbage", "no token here\n", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			script := fn + "\nextract_pve_token_value \"$TOKEN_OUTPUT\"\n"
			cmd := exec.Command("bash", "-c", script)
			cmd.Env = append(os.Environ(), "TOKEN_OUTPUT="+tc.output)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("extract_pve_token_value failed: %v\n%s", err, out)
			}
			if got := strings.TrimSpace(string(out)); got != tc.want {
				t.Fatalf("extract_pve_token_value(%s) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

// TestRootInstallAutoRegisterPrefersJsonTokenForm pins that the install-time
// auto-register path requests the JSON form first and keeps the legacy table
// form only as an explicit fallback (so the secure-installer contract pin on
// the bare form stays satisfied).
func TestRootInstallAutoRegisterPrefersJsonTokenForm(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read root install.sh: %v", err)
	}
	script := string(content)

	required := []string{
		`pveum user token add pulse-monitor@pve "$token_name" --privsep 1 --output-format json 2>&1`,
		`pveum user token add pulse-monitor@pve "$token_name" --privsep 1 2>&1`,
		`token_value=$(extract_pve_token_value "$token_output"`,
		`extract_pve_token_value() {`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing hardened token-extraction contract: %s", needle)
		}
	}

	jsonIdx := strings.Index(script, `--privsep 1 --output-format json 2>&1`)
	bareIdx := strings.Index(script, "\n        token_output=$(pveum user token add pulse-monitor@pve \"$token_name\" --privsep 1 2>&1)")
	if jsonIdx < 0 || bareIdx < 0 || jsonIdx > bareIdx {
		t.Fatalf("expected JSON token form to precede the legacy table fallback (json=%d bare=%d)", jsonIdx, bareIdx)
	}
}

func TestRootInstallAutoRegisterRotatesExistingDeterministicToken(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read root install.sh: %v", err)
	}
	script := string(content)

	required := []string{
		`pve_token_already_exists_error() {`,
		`create_pve_auto_register_token() {`,
		`create_pve_auto_register_token "$token_name" token_output token_status`,
		`pve_token_already_exists_error "$token_output"`,
		`pveum user token remove pulse-monitor@pve "$token_name"`,
		`Existing Proxmox monitoring token '${token_name}' found; rotating it so Pulse receives a fresh secret`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing deterministic token rotation contract: %s", needle)
		}
	}

	alreadyExistsIdx := strings.Index(script, `pve_token_already_exists_error "$token_output"`)
	removeIdx := strings.Index(script, `pveum user token remove pulse-monitor@pve "$token_name"`)
	retryIdx := strings.LastIndex(script, `create_pve_auto_register_token "$token_name" token_output token_status`)
	if alreadyExistsIdx < 0 || removeIdx < 0 || retryIdx < 0 || !(alreadyExistsIdx < removeIdx && removeIdx < retryIdx) {
		t.Fatalf("expected existing-token detection to remove then retry token creation (exists=%d remove=%d retry=%d)", alreadyExistsIdx, removeIdx, retryIdx)
	}
	if strings.Contains(script, `pveum user token remove pulse-monitor@pve`) && !strings.Contains(script, `pve_token_already_exists_error "$token_output"`) {
		t.Fatalf("token removal must stay gated by an explicit existing-token create error")
	}
}

func TestRootInstallAutoRegisterSmokeTestsCreatedTokenBeforeRegistration(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read root install.sh: %v", err)
	}
	script := string(content)

	for _, needle := range []string{
		`smoke_test_pve_auto_register_token() {`,
		`curl --retry 2 --retry-delay 1 -kfsS -H "Authorization: PVEAPIToken=${token_id}=${token_value}" "${host_url%/}/api2/json/nodes"`,
		`AUTO_NODE_REGISTER_ERROR="token smoke check failed"`,
		`smoke_test_pve_auto_register_token "$normalized_host_url" "$token_id" "$token_value"`,
	} {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing Proxmox token smoke-check contract: %s", needle)
		}
	}

	smokeCallIdx := strings.Index(script, `smoke_test_pve_auto_register_token "$normalized_host_url" "$token_id" "$token_value"`)
	registerIdx := strings.Index(script, `curl --retry 3 --retry-delay 2 -fsS -X POST "$pulse_url/api/auto-register" -H "Content-Type: application/json" -d "$register_payload"`)
	if smokeCallIdx < 0 || registerIdx < 0 || smokeCallIdx > registerIdx {
		t.Fatalf("expected token smoke check to run before /api/auto-register (smoke=%d register=%d)", smokeCallIdx, registerIdx)
	}
}

func TestRootInstallDeployAgentScriptsDeploysSignatureSidecars(t *testing.T) {
	extractDir := t.TempDir()
	installDir := t.TempDir()

	scriptsSrc := filepath.Join(extractDir, "scripts")
	if err := os.MkdirAll(scriptsSrc, 0o755); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	for _, name := range []string{
		"install.sh", "install.sh.sig", "install.sh.sshsig",
		"install.ps1", "install.ps1.sig", "install.ps1.sshsig",
	} {
		if err := os.WriteFile(filepath.Join(scriptsSrc, name), []byte("payload-"+name), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	script := `
		set -euo pipefail
		print_warn() { :; }
		print_success() { :; }
		chown() { :; }
		INSTALL_DIR="` + installDir + `"
` + extractRootInstallShellFunction(t, "deploy_agent_scripts") + `
		deploy_agent_scripts "` + extractDir + `"
	`

	if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("deploy_agent_scripts failed: %v\n%s", err, out)
	}

	for _, name := range []string{
		"install.sh", "install.sh.sig", "install.sh.sshsig",
		"install.ps1", "install.ps1.sig", "install.ps1.sshsig",
	} {
		if _, err := os.Stat(filepath.Join(installDir, "scripts", name)); err != nil {
			t.Fatalf("deploy_agent_scripts did not deploy %s next to the served script: %v", name, err)
		}
	}
}

// Regression test for the corrupted ExecCondition: setup_auto_updates used to
// render the pulse-update.service unit through an unquoted heredoc containing
// `$${PULSE_SERVICE_NAME}`, which bash expanded to the installer's PID. The
// resulting condition always failed, so systemd silently skipped every
// scheduled auto-update run. This test renders the real unit and executes the
// rendered ExecCondition command, instead of asserting source-text fragments.
func TestSetupAutoUpdatesRendersExecutableExecCondition(t *testing.T) {
	for _, tc := range []struct {
		name        string
		serviceName string // empty = rely on the default
		want        string
	}{
		{name: "default service name", serviceName: "", want: "pulse"},
		{name: "instance-scoped service name", serviceName: "pulse-blue", want: "pulse-blue"},
	} {
		t.Run(tc.name, func(t *testing.T) {
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

			serviceNameLine := ""
			if tc.serviceName != "" {
				serviceNameLine = `SERVICE_NAME="` + tc.serviceName + `"`
			}
			script := `
		CONFIG_DIR="` + configDir + `"
		INSTALL_DIR="` + installDir + `"
		PULSE_AUTO_UPDATE_DEST="` + autoUpdateDest + `"
		PULSE_UPDATE_SERVICE_PATH="` + servicePath + `"
		PULSE_UPDATE_TIMER_PATH="` + timerPath + `"
		` + serviceNameLine + `
		FORCE_CHANNEL=""
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

			unitBytes, err := os.ReadFile(servicePath)
			if err != nil {
				t.Fatalf("read rendered service unit: %v", err)
			}
			unit := string(unitBytes)

			wantLine := `ExecCondition=/bin/sh -c 'systemctl is-active --quiet ` + tc.want + `'`
			if !strings.Contains(unit, wantLine+"\n") {
				t.Fatalf("rendered unit missing %q:\n%s", wantLine, unit)
			}
			// The heredoc substitutes every variable at render time; any $
			// left in the unit means an unexpanded (or PID-corrupted)
			// reference leaked through again.
			if strings.Contains(unit, "$") {
				t.Fatalf("rendered unit contains an unexpanded $:\n%s", unit)
			}

			// Execute the rendered condition the way systemd would, with a
			// recording systemctl stub, to prove the command itself is sound.
			binDir := filepath.Join(tmpDir, "stub-bin")
			if err := os.MkdirAll(binDir, 0755); err != nil {
				t.Fatalf("mkdir stub bin: %v", err)
			}
			recordPath := filepath.Join(tmpDir, "systemctl-args")
			stub := "#!/bin/sh\nprintf '%s' \"$*\" > \"" + recordPath + "\"\nexit 0\n"
			if err := os.WriteFile(filepath.Join(binDir, "systemctl"), []byte(stub), 0755); err != nil {
				t.Fatalf("write systemctl stub: %v", err)
			}
			condition := strings.TrimPrefix(wantLine, "ExecCondition=")
			condOut, err := exec.Command("bash", "-c", `PATH="`+binDir+`:$PATH" `+condition).CombinedOutput()
			if err != nil {
				t.Fatalf("rendered ExecCondition failed to execute: %v\n%s", err, condOut)
			}
			recorded, err := os.ReadFile(recordPath)
			if err != nil {
				t.Fatalf("ExecCondition never invoked systemctl: %v", err)
			}
			if got, want := string(recorded), "is-active --quiet "+tc.want; got != want {
				t.Fatalf("ExecCondition invoked systemctl %q, want %q", got, want)
			}
		})
	}
}

// Regression test for stale updater scripts surviving upgrades: a v5 box with
// auto-updates already enabled keeps pulse-update.timer, so the update flow
// never re-ran setup_auto_updates and the v5.1-pinned helper script stayed in
// place, logging "Already running latest version" forever. refresh_auto_updates
// must replace the helper and units without touching system.json or the
// timer's enabled/started state.
func TestRefreshAutoUpdatesReplacesStaleHelperWithoutChangingEnablement(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	installDir := filepath.Join(tmpDir, "install")
	autoUpdateSrc := filepath.Join(installDir, "scripts", "pulse-auto-update.sh")
	autoUpdateDest, servicePath, timerPath := prepareAutoUpdatePaths(t, tmpDir)
	callsPath := filepath.Join(tmpDir, "systemctl-calls")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(autoUpdateSrc), 0755); err != nil {
		t.Fatalf("mkdir auto-update src dir: %v", err)
	}
	if err := os.WriteFile(autoUpdateSrc, []byte("#!/usr/bin/env bash\necho v6-helper\n"), 0755); err != nil {
		t.Fatalf("write auto-update src: %v", err)
	}
	// The stale v5.1-pinned helper and a v5-style unit without ExecCondition.
	if err := os.WriteFile(autoUpdateDest, []byte("#!/usr/bin/env bash\necho v5-stale-helper\n"), 0755); err != nil {
		t.Fatalf("write stale auto-update dest: %v", err)
	}
	if err := os.WriteFile(servicePath, []byte("[Service]\nExecStart="+autoUpdateDest+"\n"), 0644); err != nil {
		t.Fatalf("write stale service unit: %v", err)
	}
	// The user explicitly disabled auto-updates; a refresh must not flip it.
	systemJSON := `{"autoUpdateEnabled":false,"updateChannel":"stable"}`
	if err := os.WriteFile(filepath.Join(configDir, "system.json"), []byte(systemJSON), 0644); err != nil {
		t.Fatalf("write system.json: %v", err)
	}

	script := `
		CONFIG_DIR="` + configDir + `"
		INSTALL_DIR="` + installDir + `"
		PULSE_AUTO_UPDATE_DEST="` + autoUpdateDest + `"
		PULSE_UPDATE_SERVICE_PATH="` + servicePath + `"
		PULSE_UPDATE_TIMER_PATH="` + timerPath + `"
		GITHUB_REPO="rcourtman/Pulse"
		print_info() { :; }
		print_warn() { :; }
		safe_systemctl() { printf '%s\n' "$*" >> "` + callsPath + `"; }
` + extractRootInstallShellFunction(t, "repo_web_url") + `
` + extractRootInstallShellFunction(t, "configure_auto_update_script_repo") + `
` + extractInstallAutoUpdateAssetsShellFunctions(t) + `
` + extractRootInstallShellFunction(t, "refresh_auto_updates") + `
		refresh_auto_updates
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	helper, err := os.ReadFile(autoUpdateDest)
	if err != nil {
		t.Fatalf("read refreshed helper: %v", err)
	}
	if strings.Contains(string(helper), "v5-stale-helper") {
		t.Fatalf("refresh left the stale helper in place:\n%s", helper)
	}
	if !strings.Contains(string(helper), "v6-helper") {
		t.Fatalf("refresh did not install the release helper:\n%s", helper)
	}

	unit, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("read refreshed service unit: %v", err)
	}
	if !strings.Contains(string(unit), "ExecCondition=/bin/sh -c 'systemctl is-active --quiet pulse'") {
		t.Fatalf("refresh did not rewrite the service unit:\n%s", unit)
	}
	if _, err := os.Stat(timerPath); err != nil {
		t.Fatalf("refresh did not write the timer unit: %v", err)
	}

	gotJSON, err := os.ReadFile(filepath.Join(configDir, "system.json"))
	if err != nil {
		t.Fatalf("read system.json: %v", err)
	}
	if string(gotJSON) != systemJSON {
		t.Fatalf("refresh modified system.json:\n got: %s\nwant: %s", gotJSON, systemJSON)
	}

	calls, err := os.ReadFile(callsPath)
	if err != nil {
		t.Fatalf("read recorded systemctl calls: %v", err)
	}
	if string(calls) != "daemon-reload\n" {
		t.Fatalf("refresh changed systemd state beyond daemon-reload:\n%s", calls)
	}
}

// Pins the wiring: every existing-install flow (update, reinstall, --version,
// --source) and the fresh-install tail must refresh already-installed
// auto-update assets when the user did not opt into a full re-setup.
func TestRootInstallScriptUpdateFlowsRefreshExistingAutoUpdateAssets(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read root install.sh: %v", err)
	}

	if !strings.Contains(string(content), "refresh_auto_updates() {") {
		t.Fatal("install.sh missing refresh_auto_updates definition")
	}

	wired := regexp.MustCompile(`(?m)^\s*elif update_timer_exists; then\n\s*refresh_auto_updates$`)
	if got := len(wired.FindAll(content, -1)); got < 5 {
		t.Fatalf("expected at least 5 install flows to refresh existing auto-update assets, found %d", got)
	}
}

// Regression test for #1526 (and the earlier #1396): when the installer is piped
// to bash (curl ... | bash) there is no source file, so BASH_SOURCE is unset.
// The "am I being sourced?" guard must default the lookup or `set -u` aborts the
// whole run before the installer body with "BASH_SOURCE[0]: unbound variable".
func TestRootInstallScriptSourceGuardSurvivesPipedExecution(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read root install.sh: %v", err)
	}
	script := string(content)

	if strings.Contains(script, `[[ "${BASH_SOURCE[0]}" == "$0" ]] || return 0`) {
		t.Fatal("install.sh still uses the unguarded BASH_SOURCE source check that aborts under curl | bash")
	}
	guard := `if [[ -n "${BASH_SOURCE[0]:-}" && "${BASH_SOURCE[0]}" != "$0" ]]; then`
	if !strings.Contains(script, guard) {
		t.Fatalf("install.sh missing piped-safe source guard: %s", guard)
	}

	// Run the guard the way `curl ... | bash` does: fed through stdin with no
	// source file, so BASH_SOURCE is empty. It must fall through to the body.
	harness := "set -euo pipefail\n" + guard + "\n    return 0\nfi\necho INSTALLER_BODY_REACHED\n"
	cmd := exec.Command("bash")
	cmd.Stdin = strings.NewReader(harness)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("piped source guard failed: %v\n%s", err, out)
	}
	got := string(out)
	if strings.Contains(got, "unbound variable") {
		t.Fatalf("source guard aborted piped execution with unbound variable:\n%s", got)
	}
	if !strings.Contains(got, "INSTALLER_BODY_REACHED") {
		t.Fatalf("source guard did not fall through to the installer body when piped:\n%s", got)
	}
}

func TestRootInstallServiceGrantsIcmpProbeCapability(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read root install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`NoNewPrivileges=true`,
		`AmbientCapabilities=CAP_NET_RAW`,
		`CapabilityBoundingSet=CAP_NET_RAW`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing systemd ICMP capability grant: %s", needle)
		}
	}

	// The capability grant must live in the same hardening block that sets
	// NoNewPrivileges, so every unit the installer writes gets both.
	hardening := script[strings.Index(script, "# Security hardening"):]
	if end := strings.Index(hardening, "[Install]"); end >= 0 {
		hardening = hardening[:end]
	}
	if !strings.Contains(hardening, "AmbientCapabilities=CAP_NET_RAW") {
		t.Fatal("AmbientCapabilities=CAP_NET_RAW is not in the unit's security hardening block")
	}
}

// TestRootInstallScriptUpdateHelperWriteIsNonFatalOnReadOnlyPath asserts the
// #1630 guarantee: setup_update_command's write of the /bin/update helper
// (and its PATH appends) must not abort the installer under errexit when the
// destination is unwritable. The stock pulse-update.service runs the
// unattended updater with ProtectSystem=strict; its ReadWritePaths covers the
// install dir, config dir, /tmp and the auto-update helper and unit
// directories, so /bin (the default helper path) and /etc/profile stay
// read-only. The old behavior killed the installer after the new binary was
// installed and the service stopped, and the auto-update rollback then left
// Pulse down. A regular file as the "parent directory"
// makes writes beneath it fail with ENOTDIR, which also fails when the test
// runs as root (unlike chmod 555).
func TestRootInstallScriptUpdateHelperWriteIsNonFatalOnReadOnlyPath(t *testing.T) {
	script := `
set -euo pipefail
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
touch "$TMP/blocker"
GITHUB_REPO="rcourtman/Pulse"
INSTALL_SIGNATURE_IDENTITY="pulse-installer"
INSTALL_SIGNATURE_NAMESPACE="pulse-install"
PINNED_RELEASE_SSH_PUBLIC_KEY="test-key"
print_warn() { echo "WARN: $*"; }
print_success() { echo "OK: $*"; }
release_signature_key_available() { :; }
require_release_signature_verifier() { :; }
verify_release_signature() { :; }
` + extractRootInstallShellFunction(t, "setup_update_command") + `
UPDATE_HELPER_PATH="$TMP/blocker/update" \
PULSE_PROFILE_PATH="$TMP/profile" \
PULSE_BASHRC_PATH="$TMP/bashrc" \
setup_update_command
echo "SURVIVED_UNWRITABLE"
UPDATE_HELPER_PATH="$TMP/bin/update" \
PULSE_PROFILE_PATH="$TMP/profile" \
PULSE_BASHRC_PATH="$TMP/bashrc" \
setup_update_command
[[ -x "$TMP/bin/update" ]] && echo "HELPER_WRITTEN"
grep -q "Pulse update command" "$TMP/bin/update" && echo "HELPER_BODY_OK"
`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("setup_update_command aborted the installer on an unwritable helper path: %v\n%s", err, out)
	}
	got := string(out)
	for _, want := range []string{"SURVIVED_UNWRITABLE", "WARN:", "HELPER_WRITTEN", "HELPER_BODY_OK"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in setup_update_command output:\n%s", want, got)
		}
	}
}

// TestRootInstallScriptBinarySymlinkIsIdempotentAndNonFatal asserts the
// companion #1630 guarantee for the /usr/local/bin/pulse convenience
// symlink: an unwritable link path only warns, a writable one creates the
// link, and an already-correct link is kept without needing ln at all
// (the update-run case where the link survives but the fs is read-only).
func TestRootInstallScriptBinarySymlinkIsIdempotentAndNonFatal(t *testing.T) {
	script := `
set -euo pipefail
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
touch "$TMP/blocker"
mkdir -p "$TMP/bin"
touch "$TMP/bin/pulse-binary"
print_warn() { echo "WARN: $*"; }
print_success() { echo "OK: $*"; }
` + extractRootInstallShellFunction(t, "install_binary_symlink") + `
install_binary_symlink "$TMP/bin/pulse-binary" "$TMP/blocker/pulse"
echo "SURVIVED_UNWRITABLE"
install_binary_symlink "$TMP/bin/pulse-binary" "$TMP/bin/pulse"
[[ "$(readlink "$TMP/bin/pulse")" == "$TMP/bin/pulse-binary" ]] && echo "LINK_CREATED"
ln() { echo "LN_CALLED_AGAIN"; return 1; }
install_binary_symlink "$TMP/bin/pulse-binary" "$TMP/bin/pulse"
echo "SURVIVED_EXISTING_LINK"
`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("install_binary_symlink aborted under errexit: %v\n%s", err, out)
	}
	got := string(out)
	for _, want := range []string{"SURVIVED_UNWRITABLE", "WARN:", "LINK_CREATED", "already in place", "SURVIVED_EXISTING_LINK"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in install_binary_symlink output:\n%s", want, got)
		}
	}
	if strings.Contains(got, "LN_CALLED_AGAIN") {
		t.Fatalf("install_binary_symlink should not invoke ln when the correct link already exists:\n%s", got)
	}
}

// renderAutoUpdateUnits runs install_auto_update_assets against a seeded
// release helper and returns the rendered service and timer unit contents.
func renderAutoUpdateUnits(t *testing.T) (string, string, string, string) {
	t.Helper()

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
		GITHUB_REPO="rcourtman/Pulse"
		print_info() { :; }
		print_warn() { :; }
		safe_systemctl() { :; }
` + extractRootInstallShellFunction(t, "repo_web_url") + `
` + extractRootInstallShellFunction(t, "configure_auto_update_script_repo") + `
` + extractInstallAutoUpdateAssetsShellFunctions(t) + `
		install_auto_update_assets
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}

	serviceBytes, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("read rendered service unit: %v", err)
	}
	timerBytes, err := os.ReadFile(timerPath)
	if err != nil {
		t.Fatalf("read rendered timer unit: %v", err)
	}
	return string(serviceBytes), string(timerBytes), autoUpdateDest, servicePath
}

// Regression test for the doubled auto-update schedule (issue #1643): the
// rendered timer carried both OnCalendar=daily and OnCalendar=02:00, so with
// RandomizedDelaySec=4h every box attempted two updates per day, one in the
// 00:00-04:00 window and one in 02:00-06:00. Exactly one OnCalendar line may
// survive, and it must be the documented 02:00 schedule.
func TestAutoUpdateTimerSchedulesSingleDailyRun(t *testing.T) {
	_, timer, _, _ := renderAutoUpdateUnits(t)

	var schedules []string
	for _, line := range strings.Split(timer, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "OnCalendar=") {
			schedules = append(schedules, strings.TrimSpace(line))
		}
	}
	if len(schedules) != 1 {
		t.Fatalf("rendered timer must hold exactly one OnCalendar line, got %d:\n%s", len(schedules), timer)
	}
	if schedules[0] != "OnCalendar=*-*-* 02:00:00" {
		t.Fatalf("rendered timer schedule = %q, want the documented 02:00 daily run:\n%s", schedules[0], timer)
	}
	if !strings.Contains(timer, "RandomizedDelaySec=4h\n") {
		t.Fatalf("rendered timer lost the 4h random spread:\n%s", timer)
	}
}

// Regression test for the sandbox that froze updater fixes (issue #1637
// triage): pulse-update.service runs install.sh with ProtectSystem=strict and
// ReadWritePaths that excluded the helper and unit directories, so
// refresh_auto_updates could never replace /usr/local/bin/pulse-auto-update.sh
// or rewrite the units during an unattended update — helper fixes only reached
// boxes via manual installs. The rendered sandbox must grant write access to
// both directories.
func TestAutoUpdateServiceSandboxAllowsHelperAndUnitRefresh(t *testing.T) {
	service, _, autoUpdateDest, servicePath := renderAutoUpdateUnits(t)

	var rwLine string
	for _, line := range strings.Split(service, "\n") {
		if strings.HasPrefix(line, "ReadWritePaths=") {
			rwLine = line
		}
	}
	if rwLine == "" {
		t.Fatalf("rendered service unit lost its ReadWritePaths line:\n%s", service)
	}
	paths := strings.Fields(strings.TrimPrefix(rwLine, "ReadWritePaths="))
	want := map[string]bool{
		filepath.Dir(autoUpdateDest): false,
		filepath.Dir(servicePath):    false,
	}
	for _, p := range paths {
		if _, ok := want[p]; ok {
			want[p] = true
		}
	}
	for dir, found := range want {
		if !found {
			t.Fatalf("ReadWritePaths %q is missing %q; unattended refreshes cannot write there:\n%s", rwLine, dir, service)
		}
	}
}

// Regression test for the destructive failure path in
// install_auto_update_assets (issue #1637 triage): a failed
// configure_auto_update_script_repo used to rm -f the installed helper,
// leaving the still-enabled timer with a dangling ExecStart. The helper is now
// staged in its destination directory and swapped in with an atomic rename
// only after configuration succeeds, so any failure must leave the previously
// working helper untouched, the units unwritten, and no staging litter behind.
func TestInstallAutoUpdateAssetsKeepsWorkingHelperWhenConfigureFails(t *testing.T) {
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
	if err := os.WriteFile(autoUpdateSrc, []byte("#!/usr/bin/env bash\necho new-helper\n"), 0755); err != nil {
		t.Fatalf("write auto-update src: %v", err)
	}
	workingHelper := "#!/usr/bin/env bash\necho working-helper\n"
	if err := os.WriteFile(autoUpdateDest, []byte(workingHelper), 0755); err != nil {
		t.Fatalf("write installed helper: %v", err)
	}

	// The failing configure stub is defined after the extracted functions so
	// it overrides the real implementation.
	script := `
		CONFIG_DIR="` + configDir + `"
		INSTALL_DIR="` + installDir + `"
		PULSE_AUTO_UPDATE_DEST="` + autoUpdateDest + `"
		PULSE_UPDATE_SERVICE_PATH="` + servicePath + `"
		PULSE_UPDATE_TIMER_PATH="` + timerPath + `"
		GITHUB_REPO="rcourtman/Pulse"
		print_info() { :; }
		print_warn() { :; }
		safe_systemctl() { :; }
` + extractRootInstallShellFunction(t, "repo_web_url") + `
` + extractInstallAutoUpdateAssetsShellFunctions(t) + `
		configure_auto_update_script_repo() { return 1; }
		if install_auto_update_assets; then
			echo "UNEXPECTED_SUCCESS"
		else
			echo "FAILED_AS_EXPECTED"
		fi
	`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "FAILED_AS_EXPECTED") {
		t.Fatalf("install_auto_update_assets should report failure when configure fails:\n%s", out)
	}

	helper, err := os.ReadFile(autoUpdateDest)
	if err != nil {
		t.Fatalf("installed helper is gone after failed configure: %v", err)
	}
	if string(helper) != workingHelper {
		t.Fatalf("failed configure replaced the working helper:\n%s", helper)
	}
	if _, err := os.Stat(servicePath); !os.IsNotExist(err) {
		t.Fatalf("failed configure still rewrote the service unit (stat err %v)", err)
	}
	entries, err := os.ReadDir(filepath.Dir(autoUpdateDest))
	if err != nil {
		t.Fatalf("read helper dir: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".staged.") {
			t.Fatalf("failed configure left staging litter behind: %s", entry.Name())
		}
	}
}

// autoUpdateAssetsHarness builds a bash harness that runs the real
// install_auto_update_assets against temp paths, with a seeded release helper
// and a previously installed working helper. extra is appended after the
// extracted functions so it can override them with stubs.
func autoUpdateAssetsHarness(t *testing.T, tmpDir string, autoUpdateDest, servicePath, timerPath, extra string) string {
	t.Helper()

	configDir := filepath.Join(tmpDir, "config")
	installDir := filepath.Join(tmpDir, "install")
	autoUpdateSrc := filepath.Join(installDir, "scripts", "pulse-auto-update.sh")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(autoUpdateSrc), 0755); err != nil {
		t.Fatalf("mkdir auto-update src dir: %v", err)
	}
	if err := os.WriteFile(autoUpdateSrc, []byte("#!/usr/bin/env bash\necho new-helper\n"), 0755); err != nil {
		t.Fatalf("write auto-update src: %v", err)
	}

	return `
		CONFIG_DIR="` + configDir + `"
		INSTALL_DIR="` + installDir + `"
		PULSE_AUTO_UPDATE_DEST="` + autoUpdateDest + `"
		PULSE_UPDATE_SERVICE_PATH="` + servicePath + `"
		PULSE_UPDATE_TIMER_PATH="` + timerPath + `"
		GITHUB_REPO="rcourtman/Pulse"
		print_info() { :; }
		print_warn() { echo "WARN: $*"; }
		print_success() { :; }
		safe_systemctl() { :; }
` + extractRootInstallShellFunction(t, "repo_web_url") + `
` + extractRootInstallShellFunction(t, "configure_auto_update_script_repo") + `
` + extractInstallAutoUpdateAssetsShellFunctions(t) + `
` + extra + `
		if install_auto_update_assets; then
			echo "UNEXPECTED_SUCCESS"
		else
			echo "FAILED_AS_EXPECTED"
		fi
	`
}

// assertNoAutoUpdateStagingLitter fails when a staged helper or a staged unit
// file survived a run. `.service.tmp` is not a unit suffix systemd loads, but
// leaving one behind still means the function abandoned a partial write.
func assertNoAutoUpdateStagingLitter(t *testing.T, dirs ...string) {
	t.Helper()

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if strings.Contains(name, ".staged.") || strings.HasSuffix(name, ".tmp") {
				t.Fatalf("run left staging litter behind in %s: %s", dir, name)
			}
		}
	}
}

// Regression test for the unchecked staging copy in install_auto_update_assets:
// the bundled helper was copied into the staged mktemp file with a bare `cp`,
// and because both call sites invoke the function under `if !` errexit is
// suppressed for its whole body. A cp that failed (ENOSPC) therefore fell
// through to configure_auto_update_script_repo, whose awk happily emits a lone
// GITHUB_REPO= line for empty input, and the resulting one-line shebang-less
// stub replaced the working helper with a "script" whose only behaviour is to
// exit 0 — silently disabling unattended updates on the box. A failed or
// truncated copy must leave the installed helper and the units untouched.
func TestInstallAutoUpdateAssetsKeepsWorkingHelperWhenStagingCopyFails(t *testing.T) {
	workingHelper := "#!/usr/bin/env bash\necho working-helper\n"

	for _, tc := range []struct {
		name string
		stub string
	}{
		{
			name: "copy reports failure",
			stub: `cp() { return 1; }`,
		},
		{
			// cp that "succeeds" having written nothing: the shape the awk
			// rewrite turns into a plausible-looking one-line helper.
			name: "copy silently produces an empty file",
			stub: `cp() { : > "$2"; }`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			autoUpdateDest, servicePath, timerPath := prepareAutoUpdatePaths(t, tmpDir)
			if err := os.WriteFile(autoUpdateDest, []byte(workingHelper), 0755); err != nil {
				t.Fatalf("write installed helper: %v", err)
			}

			script := autoUpdateAssetsHarness(t, tmpDir, autoUpdateDest, servicePath, timerPath, tc.stub)
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if err != nil {
				t.Fatalf("bash: %v\n%s", err, out)
			}
			if !strings.Contains(string(out), "FAILED_AS_EXPECTED") {
				t.Fatalf("install_auto_update_assets should report failure when the staging copy fails:\n%s", out)
			}

			helper, err := os.ReadFile(autoUpdateDest)
			if err != nil {
				t.Fatalf("installed helper is gone after a failed staging copy: %v", err)
			}
			if string(helper) != workingHelper {
				t.Fatalf("failed staging copy replaced the working helper:\n%s", helper)
			}
			if _, err := os.Stat(servicePath); !os.IsNotExist(err) {
				t.Fatalf("failed staging copy still rewrote the service unit (stat err %v)", err)
			}
			if _, err := os.Stat(timerPath); !os.IsNotExist(err) {
				t.Fatalf("failed staging copy still rewrote the timer unit (stat err %v)", err)
			}
			assertNoAutoUpdateStagingLitter(t, filepath.Dir(autoUpdateDest), filepath.Dir(servicePath))
		})
	}
}

// Regression test for the unit writes in install_auto_update_assets: both units
// were rendered with a bare truncating `cat > "$unit"` whose status was never
// checked, and the function's last statement is safe_systemctl daemon-reload,
// which returns 0 by design even when systemctl fails. A failing write
// therefore truncated a working unit and still reported success. Each unit is
// now rendered beside its destination and committed with a rename, so a failure
// at either step must be reported and must leave the installed unit intact.
func TestInstallAutoUpdateAssetsWritesUnitsAtomically(t *testing.T) {
	workingHelper := "#!/usr/bin/env bash\necho working-helper\n"
	workingUnit := "[Service]\nExecStart=/usr/local/bin/pulse-auto-update.sh\n"

	for _, tc := range []struct {
		name string
		stub string
	}{
		{
			// The render itself fails (a full disk hitting the heredoc).
			name: "unit render fails",
			stub: `cat() { return 1; }`,
		},
		{
			// The render succeeds but the commit does not; the live unit must
			// still hold its previous contents rather than a truncated file.
			name: "unit commit fails",
			stub: `mv() { if [[ "${2:-}" == "` + "SERVICE_PATH_PLACEHOLDER" + `" ]]; then return 1; fi; command mv "$@"; }`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			autoUpdateDest, servicePath, timerPath := prepareAutoUpdatePaths(t, tmpDir)
			if err := os.WriteFile(autoUpdateDest, []byte(workingHelper), 0755); err != nil {
				t.Fatalf("write installed helper: %v", err)
			}
			if err := os.WriteFile(servicePath, []byte(workingUnit), 0644); err != nil {
				t.Fatalf("write installed service unit: %v", err)
			}

			stub := strings.ReplaceAll(tc.stub, "SERVICE_PATH_PLACEHOLDER", servicePath)
			script := autoUpdateAssetsHarness(t, tmpDir, autoUpdateDest, servicePath, timerPath, stub)
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if err != nil {
				t.Fatalf("bash: %v\n%s", err, out)
			}
			if !strings.Contains(string(out), "FAILED_AS_EXPECTED") {
				t.Fatalf("install_auto_update_assets should report failure when a unit write fails:\n%s", out)
			}

			unit, err := os.ReadFile(servicePath)
			if err != nil {
				t.Fatalf("installed service unit is gone after a failed write: %v", err)
			}
			if string(unit) != workingUnit {
				t.Fatalf("failed unit write clobbered the installed unit:\n%s", unit)
			}
			if _, err := os.Stat(timerPath); !os.IsNotExist(err) {
				t.Fatalf("failed service unit write still installed the timer (stat err %v)", err)
			}
			assertNoAutoUpdateStagingLitter(t, filepath.Dir(autoUpdateDest), filepath.Dir(servicePath))
		})
	}
}

// A successful run must commit through renames and leave nothing staged.
func TestInstallAutoUpdateAssetsLeavesNoStagingFilesOnSuccess(t *testing.T) {
	_, _, autoUpdateDest, servicePath := renderAutoUpdateUnits(t)
	assertNoAutoUpdateStagingLitter(t, filepath.Dir(autoUpdateDest), filepath.Dir(servicePath))
}

// Migration coverage for deployed boxes (the EROFS chicken-and-egg): a box
// installed before the sandbox was widened runs this installer from a
// pulse-update.service whose ReadWritePaths excludes /etc/systemd/system and
// /usr/local/bin, so the run that would install the corrected unit cannot write
// it. install_auto_update_assets must detect the unwritable directory up front
// and hand off to the sandbox escape rather than failing the refresh.
func TestInstallAutoUpdateAssetsMigratesWhenSandboxBlocksUnitDir(t *testing.T) {
	tmpDir := t.TempDir()
	autoUpdateDest, _, _ := prepareAutoUpdatePaths(t, tmpDir)

	// A regular file standing in for the unit directory makes it unwritable in
	// a way that also holds when the test runs as root (unlike chmod 555).
	blocker := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blocker, nil, 0644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	servicePath := filepath.Join(blocker, "pulse-update.service")
	timerPath := filepath.Join(blocker, "pulse-update.timer")
	recordPath := filepath.Join(tmpDir, "migrated-dir")

	stub := `migrate_auto_update_assets_outside_sandbox() { printf '%s' "$1" > "` + recordPath + `"; return 0; }`
	script := autoUpdateAssetsHarness(t, tmpDir, autoUpdateDest, servicePath, timerPath, stub)

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "UNEXPECTED_SUCCESS") {
		t.Fatalf("a successful sandbox escape must be reported as success:\n%s", out)
	}

	recorded, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("unwritable unit directory did not trigger the sandbox escape: %v\n%s", err, out)
	}
	if string(recorded) != blocker {
		t.Fatalf("sandbox escape was told %q was blocked, want %q", recorded, blocker)
	}
	assertNoAutoUpdateStagingLitter(t, filepath.Dir(autoUpdateDest))
}

// The sandbox escape itself: systemd-run asks PID 1 to fork the repair, so the
// transient unit runs in the host mount namespace instead of inheriting the
// update unit's ProtectSystem=strict. The installer is copied into the install
// dir first because the calling unit's PrivateTmp=yes hides the helper's /tmp
// copy of it from PID 1, and the escaped run must be handed the same service,
// helper and unit paths or it would repair the defaults instead.
func TestMigrateAutoUpdateAssetsOutsideSandboxReExecsInstallerViaSystemdRun(t *testing.T) {
	tmpDir := t.TempDir()
	installDir := filepath.Join(tmpDir, "install")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		t.Fatalf("mkdir install dir: %v", err)
	}
	argsPath := filepath.Join(tmpDir, "systemd-run-args")

	harness := `#!/usr/bin/env bash
set -uo pipefail
INSTALL_DIR="` + installDir + `"
CONFIG_DIR="` + tmpDir + `/config"
SERVICE_NAME="pulse"
AUTO_UPDATE_DEST="` + tmpDir + `/bin/pulse-auto-update.sh"
UPDATE_SERVICE_PATH="` + tmpDir + `/systemd/pulse-update.service"
UPDATE_TIMER_PATH="` + tmpDir + `/systemd/pulse-update.timer"
print_info() { :; }
print_warn() { echo "WARN: $*"; }
print_success() { echo "MIGRATED"; }
id() { echo 0; }
systemd-run() { printf '%s\n' "$@" > "` + argsPath + `"; return 0; }
` + extractRootInstallShellFunction(t, "migrate_auto_update_assets_outside_sandbox") + `
if migrate_auto_update_assets_outside_sandbox /etc/systemd/system; then
	echo "ESCAPED"
else
	echo "DID_NOT_ESCAPE"
fi
# The escape must refuse to recurse: the escaped run has no sandbox, so if it
# still cannot write, escaping again would loop forever.
rm -f "` + argsPath + `.recursion"
systemd-run() { printf '%s\n' "$@" > "` + argsPath + `.recursion"; return 0; }
if PULSE_AUTO_UPDATE_ASSET_REPAIR=1 migrate_auto_update_assets_outside_sandbox /etc/systemd/system; then
	echo "RECURSED"
fi
[[ -e "` + argsPath + `.recursion" ]] && echo "RECURSION_INVOKED_SYSTEMD_RUN"
exit 0
`
	harnessPath := filepath.Join(tmpDir, "installer-harness.sh")
	if err := os.WriteFile(harnessPath, []byte(harness), 0755); err != nil {
		t.Fatalf("write harness: %v", err)
	}

	out, err := exec.Command("bash", harnessPath).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "ESCAPED") || !strings.Contains(got, "MIGRATED") {
		t.Fatalf("sandbox escape did not report success:\n%s", got)
	}
	for _, unwanted := range []string{"RECURSED", "RECURSION_INVOKED_SYSTEMD_RUN"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("sandbox escape recursed inside the escaped run (%s):\n%s", unwanted, got)
		}
	}

	argsBytes, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("sandbox escape never invoked systemd-run: %v\n%s", err, got)
	}
	args := strings.Split(strings.TrimRight(string(argsBytes), "\n"), "\n")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--wait",
		"--property=Type=oneshot",
		"--setenv=PULSE_AUTO_UPDATE_ASSET_REPAIR=1",
		"--repair-auto-update-units",
		"--setenv=PULSE_UPDATE_SERVICE_PATH=" + tmpDir + "/systemd/pulse-update.service",
		"--setenv=PULSE_UPDATE_TIMER_PATH=" + tmpDir + "/systemd/pulse-update.timer",
		"--setenv=PULSE_AUTO_UPDATE_DEST=" + tmpDir + "/bin/pulse-auto-update.sh",
		"--setenv=PULSE_INSTALL_DIR=" + installDir,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("systemd-run invocation missing %q:\n%s", want, joined)
		}
	}

	// The re-exec target must be the copy inside the install dir: PrivateTmp
	// hides the helper's /tmp copy of the installer from PID 1.
	repairCopy := filepath.Join(installDir, ".pulse-update-asset-repair.sh")
	if !strings.Contains(joined, repairCopy) {
		t.Fatalf("systemd-run was not pointed at the install-dir copy %q:\n%s", repairCopy, joined)
	}
	if _, err := os.Stat(repairCopy); !os.IsNotExist(err) {
		t.Fatalf("sandbox escape left the installer copy behind (stat err %v)", err)
	}
}

// The other half of the migration: install.sh must expose the re-entry point
// the transient unit invokes, and it must rewrite the helper and both units
// without running a full install. This runs the real installer file.
func TestRootInstallScriptRepairAutoUpdateUnitsEntryPoint(t *testing.T) {
	tmpDir := t.TempDir()
	installDir := filepath.Join(tmpDir, "install")
	configDir := filepath.Join(tmpDir, "config")
	autoUpdateDest, servicePath, timerPath := prepareAutoUpdatePaths(t, tmpDir)

	if err := os.MkdirAll(filepath.Join(installDir, "scripts"), 0755); err != nil {
		t.Fatalf("mkdir install scripts dir: %v", err)
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "scripts", "pulse-auto-update.sh"),
		[]byte("#!/usr/bin/env bash\necho repaired-helper\n"), 0755); err != nil {
		t.Fatalf("write release helper: %v", err)
	}

	installer, err := filepath.Abs(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("resolve install.sh: %v", err)
	}
	cmd := exec.Command("bash", installer, "--repair-auto-update-units")
	cmd.Env = append(os.Environ(),
		"PULSE_INSTALL_DIR="+installDir,
		"PULSE_CONFIG_DIR="+configDir,
		"PULSE_AUTO_UPDATE_DEST="+autoUpdateDest,
		"PULSE_UPDATE_SERVICE_PATH="+servicePath,
		"PULSE_UPDATE_TIMER_PATH="+timerPath,
		"PULSE_AUTO_UPDATE_ASSET_REPAIR=1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh --repair-auto-update-units failed: %v\n%s", err, out)
	}

	helper, err := os.ReadFile(autoUpdateDest)
	if err != nil {
		t.Fatalf("repair did not install the helper: %v\n%s", err, out)
	}
	if !strings.Contains(string(helper), "repaired-helper") {
		t.Fatalf("repair installed the wrong helper:\n%s", helper)
	}
	if info, err := os.Stat(autoUpdateDest); err != nil {
		t.Fatalf("stat repaired helper: %v", err)
	} else if info.Mode().Perm() != 0755 {
		t.Fatalf("repaired helper mode = %v, want 0755 (the update unit's ExecStart)", info.Mode().Perm())
	}

	unit, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("repair did not write the service unit: %v\n%s", err, out)
	}
	if !strings.Contains(string(unit), "ReadWritePaths=") ||
		!strings.Contains(string(unit), filepath.Dir(servicePath)) ||
		!strings.Contains(string(unit), filepath.Dir(autoUpdateDest)) {
		t.Fatalf("repaired unit did not carry the widened sandbox:\n%s", unit)
	}

	timer, err := os.ReadFile(timerPath)
	if err != nil {
		t.Fatalf("repair did not write the timer unit: %v\n%s", err, out)
	}
	if strings.Count(string(timer), "\nOnCalendar=") != 1 {
		t.Fatalf("repaired timer does not hold exactly one schedule:\n%s", timer)
	}
	assertNoAutoUpdateStagingLitter(t, filepath.Dir(autoUpdateDest), filepath.Dir(servicePath))
}

// Regression tests for #1663: a half-removed installation (binary still at
// /opt/pulse/bin/pulse, but /etc/pulse and the systemd unit deleted by hand)
// re-ran the installer, which took the update path ("Reinstalling version
// ..."). With auto-updates enabled it crashed writing system.json into the
// missing config dir; without them it printed a success completion while
// `systemctl enable/start` had failed with "Unit pulse.service could not be
// found", softened into the unprivileged-container note. The update flows
// must recreate the config dir and the unit, and start_pulse must fail
// loudly when the unit does not exist at all.
func TestRootInstallScriptUpdateFlowsRepairHalfRemovedInstall(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read root install.sh: %v", err)
	}

	wired := regexp.MustCompile(`(?m)^\s*download_pulse\n(?:\s*#[^\n]*\n)*\s*setup_directories\n\s*setup_update_command\n\s*ensure_systemd_service_installed$`)
	if got := len(wired.FindAll(content, -1)); got != 2 {
		t.Fatalf("expected both update flows (--version and menu update) to run setup_directories and ensure_systemd_service_installed after download_pulse, found %d", got)
	}
}

func TestRootInstallEnsureSystemdServiceRecreatesMissingUnit(t *testing.T) {
	script := `
set -euo pipefail
SERVICE_NAME="pulse-missing-unit-1663"
print_warn() { echo "WARN: $*"; }
systemctl() { return 0; }
install_systemd_service() { echo "INSTALL_SYSTEMD_SERVICE_CALLED"; }
` + extractRootInstallShellFunction(t, "ensure_systemd_service_installed") + `
ensure_systemd_service_installed
`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("ensure_systemd_service_installed failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "INSTALL_SYSTEMD_SERVICE_CALLED") {
		t.Fatalf("missing unit was not recreated:\n%s", out)
	}
}

func TestRootInstallStartPulseFailsWhenUnitMissing(t *testing.T) {
	stubs := `
set -euo pipefail
PULSE_WAS_ACTIVE="false"
print_info() { echo "INFO: $*"; }
print_error() { echo "ERROR: $*"; }
print_success() { echo "SUCCESS: $*"; }
safe_systemctl() { return 0; }
timeout() { shift; "$@"; }
sleep() { :; }
journalctl() { return 0; }
ensure_pulse_running_after_update() { return 0; }
` + extractRootInstallShellFunction(t, "start_pulse") + "\n"

	// Unit genuinely absent: no unit file and `systemctl cat` cannot find it.
	// start_pulse must fail with a clear error instead of reporting success
	// behind the unprivileged-container note.
	missing := stubs + `
SERVICE_NAME="pulse-missing-unit-1663"
systemctl() { return 1; }
start_pulse
`
	out, err := exec.Command("bash", "-c", missing).CombinedOutput()
	if err == nil {
		t.Fatalf("start_pulse reported success with no unit installed:\n%s", out)
	}
	if !strings.Contains(string(out), "does not exist") {
		t.Fatalf("start_pulse did not explain the missing unit:\n%s", out)
	}
	if strings.Contains(string(out), "SUCCESS:") {
		t.Fatalf("start_pulse printed success for a missing unit:\n%s", out)
	}

	// Unit resolvable via systemctl cat: the guard must fall through and the
	// normal start path must succeed.
	present := stubs + `
SERVICE_NAME="pulse-missing-unit-1663"
systemctl() { return 0; }
start_pulse
`
	out, err = exec.Command("bash", "-c", present).CombinedOutput()
	if err != nil {
		t.Fatalf("start_pulse failed with a resolvable unit: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "SUCCESS:") {
		t.Fatalf("start_pulse did not report a successful start:\n%s", out)
	}
}

// Reported on #1663: reinstalling after `rm -rf /etc/pulse` reached
// setup_auto_updates before setup_directories had recreated the config
// directory, so the system.json write failed with "No such file or
// directory" while the run still reported that auto-updates were enabled.
func TestRootInstallScriptAutoUpdateSetupCreatesMissingConfigDir(t *testing.T) {
	parent := t.TempDir()
	configDir := filepath.Join(parent, "pulse")

	script := `
		set -euo pipefail
		print_info() { :; }
		print_warn() { echo "WARN: $*"; }
		print_success() { :; }
		selected_update_channel() { printf 'stable\n'; }
		install_auto_update_assets() { return 0; }
		safe_systemctl() { return 0; }
		chown() { return 0; }
		CONFIG_DIR="$CONFIG_DIR_UNDER_TEST"
		SERVICE_NAME="pulse"
		UPDATE_TIMER_PATH="/tmp/pulse-update.timer"
		ENABLE_AUTO_UPDATES=true
` + extractRootInstallShellFunction(t, "setup_auto_updates") + `
		setup_auto_updates
	`

	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), "CONFIG_DIR_UNDER_TEST="+configDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "No such file or directory") {
		t.Fatalf("expected config dir to be created before the system.json write, got:\n%s", out)
	}

	systemJSON := filepath.Join(configDir, "system.json")
	contents, readErr := os.ReadFile(systemJSON)
	if readErr != nil {
		t.Fatalf("expected %s to be written, got: %v\n%s", systemJSON, readErr, out)
	}
	if !strings.Contains(string(contents), `"autoUpdateEnabled":true`) {
		t.Fatalf("expected auto-updates to be enabled in system.json, got: %s", contents)
	}
}

// The config-directory guard must run before any state is mutated. An
// earlier version sat after `safe_systemctl enable`, so a failure to create
// the directory left the update timer enabled while the installer reported
// automatic updates as disabled and never wrote system.json.
func TestRootInstallScriptAutoUpdateConfigDirGuardRunsBeforeStateChanges(t *testing.T) {
	body := extractRootInstallShellFunction(t, "setup_auto_updates")

	mkdirIdx := strings.Index(body, `mkdir -p "$config_dir"`)
	if mkdirIdx < 0 {
		t.Fatal("expected setup_auto_updates to create the config directory")
	}
	for _, mutation := range []string{"install_auto_update_assets", "safe_systemctl enable"} {
		idx := strings.Index(body, mutation)
		if idx < 0 {
			t.Fatalf("expected setup_auto_updates to reference %s", mutation)
		}
		if idx < mkdirIdx {
			t.Fatalf("%s runs before the config-directory guard, so a guard failure would leave partial state", mutation)
		}
	}
}

// pct exec runs with PATH=/sbin:/bin:/usr/sbin:/usr/bin. Anything installed to
// /usr/local/bin, which is where BINARY_LINK_PATH and the non-default update
// helper live, is therefore unreachable by bare name and fails with exit 127.
// Every pct exec invocation the installer runs or prints must name a command
// that is either absolute or genuinely resolvable on that PATH.
func TestRootInstallPctExecCommandsResolveOnPctExecPath(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read root install.sh: %v", err)
	}

	// Commands that really do live on the pct exec PATH.
	resolvable := map[string]bool{
		"bash":      true,
		"sh":        true,
		"rm":        true,
		"hostname":  true,
		"systemctl": true,
		"env":       true,
	}

	pctExec := regexp.MustCompile(`pct exec [^\n]*? -- (.+)`)
	for i, line := range strings.Split(string(content), "\n") {
		match := pctExec.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		fields := strings.Fields(match[1])
		// Skip any leading `env KEY=VALUE` prefix to reach the real command.
		for len(fields) > 1 && (fields[0] == "env" || strings.Contains(fields[0], "=")) {
			fields = fields[1:]
		}
		if len(fields) == 0 {
			continue
		}
		command := strings.Trim(fields[0], `"'`)
		if strings.HasPrefix(command, "$") || strings.HasPrefix(command, "${") {
			// A variable reference is fine as long as it is not wrapped in
			// basename, which is what strips the directory off an absolute path.
			if strings.Contains(match[1], "basename") {
				t.Fatalf("install.sh:%d passes a basename to pct exec, which drops the directory and breaks PATH resolution: %s", i+1, strings.TrimSpace(line))
			}
			continue
		}
		if strings.HasPrefix(command, "/") {
			continue
		}
		if resolvable[command] {
			continue
		}
		t.Fatalf("install.sh:%d runs %q through pct exec by bare name, which is not on PATH=/sbin:/bin:/usr/sbin:/usr/bin: %s", i+1, command, strings.TrimSpace(line))
	}
}
