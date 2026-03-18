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
	}
	for _, needle := range forbidden {
		if strings.Contains(script, needle) {
			t.Fatalf("root install.sh preserved stale setup-script-url bootstrap auth fragment: %s", needle)
		}
	}
}
