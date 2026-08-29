package installtests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func safeProfileInspectFunctions(t *testing.T) string {
	t.Helper()
	return extractInstallShellFunction(t, "safe_profile_platform_supported") + "\n" +
		extractInstallShellFunction(t, "safe_profile_detect_current_profile") + "\n" +
		extractInstallShellFunction(t, "safe_profile_unit_property") + "\n" +
		extractInstallShellFunction(t, "safe_profile_inspect")
}

func safeProfileTransactionFunctions(t *testing.T) string {
	t.Helper()
	return extractInstallShellFunction(t, "safe_profile_detect_current_profile") + "\n" +
		extractInstallShellFunction(t, "safe_profile_snapshot_entry") + "\n" +
		extractInstallShellFunction(t, "safe_profile_manifest_value") + "\n" +
		extractInstallShellFunction(t, "safe_profile_begin_transaction") + "\n" +
		extractInstallShellFunction(t, "safe_profile_restore_entry") + "\n" +
		extractInstallShellFunction(t, "safe_profile_restore_transaction") + "\n" +
		extractInstallShellFunction(t, "safe_profile_commit_transaction")
}

func TestSafeProfileInspectIsReadOnlyAndReportsDifferences(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	unitDir := filepath.Join(root, "systemd")
	mustMkdirAll(t, binDir, unitDir)
	binary := filepath.Join(binDir, "pulse-agent")
	unit := filepath.Join(unitDir, "pulse-agent.service")
	runner := filepath.Join(unitDir, "pulse-agent-runner.service")
	unitBody := "[Service]\nUser=root\nAmbientCapabilities=CAP_SETUID CAP_SETGID\nExecStart=" + binary + " --enable-host --enable-docker --enable-proxmox --enable-commands\n"
	mustWrite(t, binary, "collector-before\n")
	mustWrite(t, unit, unitBody)
	mustWrite(t, runner, "runner-independent\n")

	harness := `
set -euo pipefail
AGENT_NAME=pulse-agent
BINARY_NAME=pulse-agent
INSTALL_DIR="` + binDir + `"
LEAST_PRIVILEGE_USER=pulse-agent
SAFE_PROFILE_COLLECTOR_UNIT="` + unit + `"
ACTION_RUNNER_SERVICE_UNIT="` + runner + `"
log_error() { printf 'ERROR:%s\n' "$*" >&2; }
uname() { printf 'Linux\n'; }
systemctl() {
  if [[ "$1" == show ]]; then
    case "$4" in User) printf 'root\n' ;; AmbientCapabilities) printf 'CAP_SETUID CAP_SETGID\n' ;; esac
  fi
}
id() { if [[ "${1:-}" == -nG ]]; then printf 'root docker\n'; fi; return 0; }
` + safeProfileInspectFunctions(t) + `
safe_profile_inspect
`
	out, err := exec.Command("bash", "-c", harness).CombinedOutput()
	if err != nil {
		t.Fatalf("inspect: %v\n%s", err, out)
	}
	for _, want := range []string{
		"platform_supported=true", "current_profile=legacy-root-command-capable",
		"unit_user=root", "unit_groups=root docker", "ambient_capabilities=CAP_SETUID CAP_SETGID",
		"provider_docker=true", "provider_proxmox=true", "collector_commands=true",
		"action_runner_independent=true", "target_profile=typed-helper-monitoring-only",
		"target_groups=no-rootful-docker-group", "degraded_docker=rootful daemon access is removed",
		"degraded_actions=collector command authority is removed",
	} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("inspect output missing %q:\n%s", want, out)
		}
	}
	assertFileBody(t, binary, "collector-before\n")
	assertFileBody(t, unit, unitBody)
	assertFileBody(t, runner, "runner-independent\n")
}

func TestSafeProfileTransactionCommitAndFailureRollback(t *testing.T) {
	t.Run("commit", func(t *testing.T) {
		root := t.TempDir()
		harness := safeProfileHarness(t, root, false) + `
safe_profile_begin_transaction
safe_profile_commit_transaction
grep -q '^PRIOR_PROFILE=legacy-root-command-capable$' "$SAFE_PROFILE_CURRENT_FILE"
grep -q '^CURRENT_PROFILE=typed-helper-monitoring-only$' "$SAFE_PROFILE_CURRENT_FILE"
grep -q "^TRANSACTION_DIR=${SAFE_PROFILE_TRANSACTION_DIR}$" "$SAFE_PROFILE_CURRENT_FILE"
test -f "${SAFE_PROFILE_TRANSACTION_DIR}/collector-binary"
test "$SAFE_PROFILE_TRANSACTION_ACTIVE" = false
test "$SAFE_PROFILE_TRANSACTION_COMMITTED" = true
`
		if out, err := exec.Command("bash", "-c", harness).CombinedOutput(); err != nil {
			t.Fatalf("commit rehearsal: %v\n%s", err, out)
		}
	})

	t.Run("failure rollback", func(t *testing.T) {
		root := t.TempDir()
		harness := safeProfileHarness(t, root, true) + `
safe_profile_begin_transaction
transaction="$SAFE_PROFILE_TRANSACTION_DIR"
printf 'new-binary\n' > "$INSTALL_DIR/$BINARY_NAME"
printf '[Service]\nUser=pulse-agent\nEnvironment=PULSE_AGENT_HELPER_SOCKET=/run/pulse-agent/helper.sock\n' > "$SAFE_PROFILE_COLLECTOR_UNIT"
printf 'typed-helper\n' > "$PRIVILEGED_HELPER_BINARY_PATH"
printf 'helper-unit\n' > "$PRIVILEGED_HELPER_SERVICE_UNIT"
printf 'helper-socket\n' > "$PRIVILEGED_HELPER_SOCKET_UNIT"
rm -f "$STATE_DIR/token" "$STATE_DIR/runtime.token"
printf 'changed-agent-id\n' > "$STATE_DIR/agent-id"
printf 'changed-connection\n' > "$STATE_DIR/connection.env"
mkdir -p "$PRIVILEGED_HELPER_CREDENTIAL_DIR"
printf 'moved-monitoring-token\n' > "$PRIVILEGED_HELPER_CREDENTIAL_DIR/token"
printf 'runner-still-independent\n' > "$ACTION_RUNNER_SENTINEL"
safe_profile_restore_transaction "$transaction" automatic-failure
cmp "$INSTALL_DIR/$BINARY_NAME" "$EXPECTED_DIR/collector-binary"
cmp "$SAFE_PROFILE_COLLECTOR_UNIT" "$EXPECTED_DIR/collector-unit"
cmp "$STATE_DIR/token" "$EXPECTED_DIR/state-token"
cmp "$STATE_DIR/runtime.token" "$EXPECTED_DIR/runtime-token"
cmp "$STATE_DIR/agent-id" "$EXPECTED_DIR/agent-id"
cmp "$STATE_DIR/connection.env" "$EXPECTED_DIR/connection-env"
test ! -e "$PRIVILEGED_HELPER_BINARY_PATH"
test ! -e "$PRIVILEGED_HELPER_SERVICE_UNIT"
test ! -e "$PRIVILEGED_HELPER_SOCKET_UNIT"
test ! -e "$PRIVILEGED_HELPER_CREDENTIAL_DIR/token"
grep -q '^legacy sudo grant$' "$PRIVILEGE_SUDOERS_FILE"
grep -q '^legacy smart wrapper$' "$PRIVILEGE_HELPER_DIR/smartctl"
grep -q '^legacy pct wrapper$' "$PRIVILEGE_HELPER_DIR/pct"
grep -q '^runner-still-independent$' "$ACTION_RUNNER_SENTINEL"
grep -q '^CURRENT_PROFILE=legacy-root-command-capable$' "$SAFE_PROFILE_CURRENT_FILE"
grep -q '^gpasswd -a pulse-agent docker$' "$CALL_LOG"
`
		if out, err := exec.Command("bash", "-c", harness).CombinedOutput(); err != nil {
			t.Fatalf("failure rollback rehearsal: %v\n%s", err, out)
		}
	})
}

func TestSafeProfileMigrationIsExplicitAndRunnerIndependent(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(content)
	for _, want := range []string{
		`--safe-profile-inspect) SAFE_PROFILE_ACTION="inspect"`,
		`--safe-profile-apply) SAFE_PROFILE_ACTION="apply"`,
		`--safe-profile-rollback) SAFE_PROFILE_ACTION="rollback"`,
		`# Explicit safe-profile migration lifecycle. Ordinary --update deliberately`,
		`safe_profile_verify_declared_health`, `safe_profile_commit_transaction`,
		`"$SAFE_PROFILE_ACTION" != "apply"`, `target_action_runner=unchanged`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("installer missing migration invariant %q", want)
		}
	}
	rollback := extractInstallShellFunction(t, "safe_profile_restore_transaction")
	for _, forbidden := range []string{"ACTION_RUNNER_BINARY_PATH", "ACTION_RUNNER_SERVICE_UNIT", "teardown_action_runner_service", "provision_action_runner"} {
		if strings.Contains(rollback, forbidden) {
			t.Fatalf("collector rollback touched independent runner through %q", forbidden)
		}
	}
}

func TestSafeProfileApplyRequiresReadinessHelperAndRegistration(t *testing.T) {
	gate := extractInstallShellFunction(t, "safe_profile_verify_declared_health")
	script := `
set -euo pipefail
AGENT_NAME=pulse-agent
PRIVILEGED_HELPER_NAME=pulse-agent-helper
resolve_agent_health_url() { printf 'http://127.0.0.1:9191/readyz\n'; }
curl() { return 0; }
systemctl() { return 0; }
verify_agent_server_registration_with_retry() { return 0; }
` + gate + `
safe_profile_verify_declared_health
verify_agent_server_registration_with_retry() { return 1; }
if safe_profile_verify_declared_health; then
  echo 'registration failure was accepted' >&2
  exit 1
fi
`
	if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("health gate rehearsal: %v\n%s", err, out)
	}
}

func safeProfileHarness(t *testing.T, root string, dockerMember bool) string {
	t.Helper()
	binDir := filepath.Join(root, "bin")
	unitDir := filepath.Join(root, "systemd")
	helperDir := filepath.Join(root, "helper")
	stateDir := filepath.Join(root, "state")
	credentialDir := filepath.Join(root, "credential")
	expectedDir := filepath.Join(root, "expected")
	mustMkdirAll(t, binDir, unitDir, helperDir, stateDir, expectedDir)
	files := map[string]string{
		filepath.Join(binDir, "pulse-agent"):          "old-binary\n",
		filepath.Join(unitDir, "pulse-agent.service"): "[Service]\nUser=root\nAmbientCapabilities=CAP_SETUID CAP_SETGID\nExecStart=/bin/pulse-agent --enable-commands\n",
		filepath.Join(root, "sudoers"):                "legacy sudo grant\n",
		filepath.Join(helperDir, "smartctl"):          "legacy smart wrapper\n",
		filepath.Join(helperDir, "pct"):               "legacy pct wrapper\n",
		filepath.Join(stateDir, "token"):              "monitoring-token\n",
		filepath.Join(stateDir, "runtime.token"):      "runtime-monitoring-token\n",
		filepath.Join(stateDir, "agent-id"):           "stable-agent-id\n",
		filepath.Join(stateDir, "connection.env"):     "PULSE_URL='https://pulse.example'\n",
	}
	for path, body := range files {
		mustWrite(t, path, body)
	}
	for source, name := range map[string]string{
		filepath.Join(binDir, "pulse-agent"):          "collector-binary",
		filepath.Join(unitDir, "pulse-agent.service"): "collector-unit",
		filepath.Join(stateDir, "token"):              "state-token",
		filepath.Join(stateDir, "runtime.token"):      "runtime-token",
		filepath.Join(stateDir, "agent-id"):           "agent-id",
		filepath.Join(stateDir, "connection.env"):     "connection-env",
	} {
		body, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(expectedDir, name), string(body))
	}
	membership := "pulse-agent"
	if dockerMember {
		membership = "pulse-agent docker"
	}
	return `
set -euo pipefail
AGENT_NAME=pulse-agent
BINARY_NAME=pulse-agent
INSTALL_DIR="` + binDir + `"
LEAST_PRIVILEGE_USER=pulse-agent
PRIVILEGE_HELPER_DIR="` + helperDir + `"
PRIVILEGE_SUDOERS_FILE="` + filepath.Join(root, "sudoers") + `"
PRIVILEGED_HELPER_BINARY_PATH="` + filepath.Join(helperDir, "pulse-agent-helper") + `"
PRIVILEGED_HELPER_SERVICE_UNIT="` + filepath.Join(unitDir, "pulse-agent-helper.service") + `"
PRIVILEGED_HELPER_SOCKET_UNIT="` + filepath.Join(unitDir, "pulse-agent-helper.socket") + `"
PRIVILEGED_HELPER_SOCKET_PATH="` + filepath.Join(root, "run", "helper.sock") + `"
PRIVILEGED_HELPER_NAME=pulse-agent-helper
PRIVILEGED_HELPER_CREDENTIAL_DIR="` + credentialDir + `"
SAFE_PROFILE_COLLECTOR_UNIT="` + filepath.Join(unitDir, "pulse-agent.service") + `"
SAFE_PROFILE_STATE_DIR="` + filepath.Join(root, "profile") + `"
SAFE_PROFILE_CURRENT_FILE="${SAFE_PROFILE_STATE_DIR}/current.env"
SAFE_PROFILE_TRANSACTION_DIR=""
SAFE_PROFILE_TRANSACTION_ACTIVE=false
SAFE_PROFILE_TRANSACTION_COMMITTED=false
STATE_DIR="` + stateDir + `"
ACTION_RUNNER_SENTINEL="` + filepath.Join(root, "runner-sentinel") + `"
EXPECTED_DIR="` + expectedDir + `"
CALL_LOG="` + filepath.Join(root, "calls.log") + `"
EXIT_GENERAL=1
EXIT_MISSING_ARGS=2
log_info() { :; }
log_error() { printf 'ERROR:%s\n' "$*" >&2; }
fail() { printf 'FAIL:%s\n' "$1" >&2; return "${2:-1}"; }
systemctl() { case "${1:-}" in is-active|is-enabled) return 0 ;; *) return 0 ;; esac; }
getent() { [[ "${1:-}" == group && "${2:-}" == docker ]]; }
id() { if [[ "${1:-}" == -nG ]]; then printf '` + membership + `\n'; fi; return 0; }
gpasswd() { printf 'gpasswd %s\n' "$*" >> "$CALL_LOG"; }
` + safeProfileTransactionFunctions(t) + "\n"
}

func mustMkdirAll(t *testing.T, dirs ...string) {
	t.Helper()
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func assertFileBody(t *testing.T, path, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil || string(body) != want {
		t.Fatalf("%s body=%q want=%q err=%v", path, body, want, err)
	}
}
