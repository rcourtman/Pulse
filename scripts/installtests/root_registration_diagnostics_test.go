//go:build linux

package installtests

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Exercise the shipped registration functions. Only curl's remote responses
// and Proxmox's provider commands are inert adapters; no provider, credential,
// guest, service or installed state is accessed by these diagnostic controls.
func TestRootInstallerRegistrationDiagnostics(t *testing.T) {
	const host = "https://fixture.invalid:8006"
	const pulse = "http://127.0.0.1:7655"
	const tokenID = "pulse-monitor@pve!pulse-127-0-0-1"
	const tokenValue = "fixture-monitor-credential"
	const setupToken = "fixture-setup-credential"
	for _, mode := range []string{"success", "status", "token-id", "token-value", "node-name", "malformed", "smoke-7", "smoke-22", "smoke-28"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			setupURL := pulse + "/api/setup-script?backup_perms=true&host=" + url.QueryEscape(host) + "&pulse_url=" + url.QueryEscape(pulse) + "&type=pve"
			command := "curl '" + setupURL + "' | { if [ \"$(id -u)\" -eq 0 ]; then env PULSE_SETUP_TOKEN_FILE=/fixture/token bash; elif command -v sudo >/dev/null 2>&1; then sudo env PULSE_SETUP_TOKEN_FILE=/fixture/token bash; fi; }"
			setup := map[string]any{
				"setupToken": setupToken, "type": "pve", "host": host, "url": setupURL, "downloadURL": setupURL,
				"scriptFileName": "pulse-setup-pve.sh", "command": command, "commandWithEnv": command,
				"commandWithoutEnv": command, "tokenHint": "fixture-hint", "expires": time.Now().Add(time.Hour).Unix(),
			}
			registration := map[string]string{
				"status": "success", "action": "use_token", "type": "pve", "source": "script", "host": host,
				"tokenId": tokenID, "tokenValue": tokenValue, "nodeId": "fixture-pve", "nodeName": "fixture-pve",
				"remoteDetails": "remote-output-only-marker remote-diagnostic-credential \x1b[31m",
			}
			switch mode {
			case "status":
				registration["status"] = "error"
			case "token-id":
				registration["tokenId"] = "other@pve!fixture"
			case "token-value":
				registration["tokenValue"] = "remote-diagnostic-credential"
			case "node-name":
				registration["nodeName"] = "different-node"
			}
			for name, data := range map[string]any{"setup-response": setup, "register-response": registration} {
				body, err := json.Marshal(data)
				if err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, filepath.Join(root, name), body, 0600)
			}
			if mode == "malformed" {
				writeTestFile(t, filepath.Join(root, "register-response"), []byte("remote-output-only-marker "+tokenValue+"\r\n\x1b[31m not JSON"), 0600)
			}
			writeTestFile(t, filepath.Join(root, "remote-output"), []byte("remote-output-only-marker "+tokenValue+" remote-diagnostic-credential\r\n\x1b[31m"), 0600)
			saved := map[string][]byte{"pulse": []byte("saved binary\n"), "system.json": []byte("saved configuration\n"), "token": []byte("saved fixture authority\n")}
			for name, body := range saved {
				writeTestFile(t, filepath.Join(root, name), body, 0600)
			}
			exitCode := "0"
			if strings.HasPrefix(mode, "smoke-") {
				exitCode = strings.TrimPrefix(mode, "smoke-")
			}
			// The installer keeps the administrator's working directory, which may
			// be shared (/tmp). Modules there or on PYTHONPATH are not installer code.
			caller := filepath.Join(root, "caller")
			plantCallerPythonModules(t, caller)
			env := append(rootCurlFixtureEnvironment(t, root, host), "FIXTURE_CURL_EXIT="+exitCode,
				"PULSE_PVE_HOST_URL="+host, "PULSE_SKIP_AUTO_NODE=", "PULSE_AUTO_BACKUP_PERMS=true",
				"PYTHONPATH="+caller)
			script := `source "$ROOT_INSTALLER_SOURCE"
print_info() { :; }
print_warn() { printf 'WARN:%s\n' "$*"; }
print_success() { printf 'SUCCESS:%s\n' "$*"; }
hostname() { printf 'fixture-pve\n'; }
pveum() {
    if [[ "$*" == 'role list' ]]; then
        printf 'Sys.Audit\nVM.GuestAgent.Audit\nVM.GuestAgent.FileRead\n'
    fi
}
create_pve_auto_register_token() {
    printf -v "$2" '%s' '{"value":"fixture-monitor-credential"}'
    printf -v "$3" '%s' 0
}
curl() {
    [[ "$1" == -q ]] || return 97
    local phase= arg
    for arg in "$@"; do
        case "$arg" in
            */api/setup-script-url) phase=setup ;;
            */api2/json/nodes) phase=smoke ;;
            */api/auto-register) phase=register ;;
        esac
    done
    printf '%s\n' "$phase" >> "$FIXTURE_ROOT/stages"
    case "$phase" in
        setup) cat "$FIXTURE_ROOT/setup-response" ;;
        smoke)
            cat > "$FIXTURE_ROOT/smoke-header"
            if [[ "$FIXTURE_CURL_EXIT" != 0 ]]; then
                cat "$FIXTURE_ROOT/remote-output" >&2
                return "$FIXTURE_CURL_EXIT"
            fi
            printf '{"data":[]}' ;;
        register)
            cat > "$FIXTURE_ROOT/register-request"
            cat "$FIXTURE_ROOT/register-response" ;;
        *) return 98 ;;
    esac
}
IN_CONTAINER=false
AUTO_NODE_REGISTERED=false
AUTO_NODE_REGISTERED_NAME=unchanged
AUTO_NODE_REGISTER_ERROR=
cd "$FIXTURE_ROOT/caller"
auto_register_pve_node 600 127.0.0.1 7655 > "$FIXTURE_ROOT/output" 2>&1
printf '%s\n%s\n%s\n' "$AUTO_NODE_REGISTERED" "$AUTO_NODE_REGISTERED_NAME" "$AUTO_NODE_REGISTER_ERROR" > "$FIXTURE_ROOT/result"
`
			if exit := runRootCurlFixture(t, script, env); exit != 0 {
				t.Fatalf("registration fixture failed: exit=%d", exit)
			}
			read := func(name string) string {
				t.Helper()
				body, err := os.ReadFile(filepath.Join(root, name))
				if err != nil {
					t.Fatal(err)
				}
				return string(body)
			}
			output := read("output")
			for _, forbidden := range []string{tokenValue, setupToken, "remote-diagnostic-credential", "remote-output-only-marker", "tokenValue", "\x1b", "\r"} {
				if strings.Contains(output, forbidden) {
					t.Errorf("remote response or credential echoed in %s diagnostics", mode)
				}
			}
			wantResult, wantStages := "false\nunchanged\nauto-register unsuccessful\n", "setup\nsmoke\nregister\n"
			if mode == "success" {
				wantResult = "true\nfixture-pve\n\n"
				if !strings.Contains(output, "Registered fixture-pve with Pulse automatically") {
					t.Error("normal registration success message lost")
				}
			} else if exitCode != "0" {
				wantResult, wantStages = "false\nunchanged\ntoken smoke check failed\n", "setup\nsmoke\n"
				if !strings.Contains(output, "curl exit "+exitCode) {
					t.Error("safe curl exit diagnostic lost")
				}
				if _, err := os.Stat(filepath.Join(root, "register-request")); !os.IsNotExist(err) {
					t.Error("failed smoke check continued to registration")
				}
			} else if !strings.Contains(output, "could not be verified") || !strings.Contains(output, "Check Pulse's node list before re-running the installer") {
				t.Error("rejected-response guidance missing")
			}
			if got := read("result"); got != wantResult {
				t.Errorf("registration state changed: got %q, want %q", got, wantResult)
			}
			if got := read("stages"); got != wantStages {
				t.Errorf("request ordering changed: got %q, want %q", got, wantStages)
			}
			if read("smoke-header") != "Authorization: PVEAPIToken="+tokenID+"="+tokenValue+"\n" {
				t.Error("created-credential smoke identity changed")
			}
			if exitCode == "0" {
				var request map[string]string
				if err := json.Unmarshal([]byte(read("register-request")), &request); err != nil || request["host"] != host || request["authToken"] != setupToken || request["tokenId"] != tokenID || request["tokenValue"] != tokenValue {
					t.Error("registration request identity changed")
				}
			}
			assertRootCurlFixtureUnchanged(t, root, filepath.Join(root, "ambient-trace"), saved)
			assertCallerPythonModulesUnused(t, caller)
			t.Logf("%s: synthetic response; %s; no saved-state or request-identity change", mode, fmt.Sprintf("curl exit %s", exitCode))
		})
	}
}
