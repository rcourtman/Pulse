package installtests

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
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
		extractInstallShellFunction(t, "systemd_effective_unit_property") + "\n" +
		extractInstallShellFunction(t, "systemd_effective_unit_unoverridden") + "\n" +
		extractInstallShellFunction(t, "safe_profile_effective_unit_unoverridden") + "\n" +
		extractInstallShellFunction(t, "safe_profile_inspect")
}

func safeProfileTransactionFunctions(t *testing.T) string {
	t.Helper()
	return extractInstallShellFunction(t, "safe_profile_detect_current_profile") + "\n" +
		extractInstallShellFunction(t, "systemd_effective_unit_property") + "\n" +
		extractInstallShellFunction(t, "systemd_effective_unit_unoverridden") + "\n" +
		extractInstallShellFunction(t, "safe_profile_effective_unit_unoverridden") + "\n" +
		extractInstallShellFunction(t, "safe_profile_snapshot_entry") + "\n" +
		extractInstallShellFunction(t, "safe_profile_manifest_value") + "\n" +
		extractInstallShellFunction(t, "safe_profile_snapshot_state_metadata") + "\n" +
		extractInstallShellFunction(t, "safe_profile_restore_state_metadata") + "\n" +
		extractInstallShellFunction(t, "safe_profile_begin_transaction") + "\n" +
		extractInstallShellFunction(t, "safe_profile_restore_entry") + "\n" +
		extractInstallShellFunction(t, "safe_profile_remove_collector_command_authority") + "\n" +
		extractInstallShellFunction(t, "safe_profile_restore_transaction") + "\n" +
		extractInstallShellFunction(t, "safe_profile_commit_transaction")
}

func safeProfileEffectiveSystemdFunctions(t *testing.T) string {
	t.Helper()
	return extractInstallShellFunction(t, "safe_profile_unit_property") + "\n" +
		extractInstallShellFunction(t, "systemd_effective_unit_property") + "\n" +
		extractInstallShellFunction(t, "systemd_effective_unit_unoverridden") + "\n" +
		extractInstallShellFunction(t, "systemd_effective_exec_argv") + "\n" +
		extractInstallShellFunction(t, "systemd_effective_exec_exact") + "\n" +
		extractInstallShellFunction(t, "systemd_effective_words_equal") + "\n" +
		extractInstallShellFunction(t, "systemd_effective_common_hardening") + "\n" +
		extractInstallShellFunction(t, "safe_profile_verify_helper_effective_target") + "\n" +
		extractInstallShellFunction(t, "action_runner_verify_effective_target") + "\n" +
		extractInstallShellFunction(t, "safe_profile_effective_unit_unoverridden") + "\n" +
		extractInstallShellFunction(t, "safe_profile_verify_effective_target")
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
    case "$4" in
      User) printf 'root\n' ;;
      AmbientCapabilities) printf 'CAP_SETUID CAP_SETGID\n' ;;
      FragmentPath) printf '%s\n' "$SAFE_PROFILE_COLLECTOR_UNIT" ;;
      DropInPaths) printf '\n' ;;
    esac
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
		"unit_fragment_path=" + unit, "unit_drop_in_paths=none", "unit_unoverridden=true",
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
printf 'transient-wal\n' > "$STATE_DIR/cache/runtime.db-wal"
chmod 0600 "$STATE_DIR/cache/runtime.db-wal"
safe_profile_begin_transaction
transaction="$SAFE_PROFILE_TRANSACTION_DIR"
rm -f "$STATE_DIR/cache/runtime.db-wal"
printf 'new-binary\n' > "$INSTALL_DIR/$BINARY_NAME"
printf '[Service]\nUser=pulse-agent\nEnvironment=PULSE_AGENT_HELPER_SOCKET=/run/pulse-agent/helper.sock\n' > "$SAFE_PROFILE_COLLECTOR_UNIT"
printf 'typed-helper\n' > "$PRIVILEGED_HELPER_BINARY_PATH"
printf 'helper-unit\n' > "$PRIVILEGED_HELPER_SERVICE_UNIT"
printf 'helper-socket\n' > "$PRIVILEGED_HELPER_SOCKET_UNIT"
rm -f "$STATE_DIR/token" "$STATE_DIR/runtime.token"
rm -f "$STATE_DIR/proxmox-registered" "$STATE_DIR/proxmox-pve-registered" "$STATE_DIR/proxmox-pbs-registered"
rm -f "$STATE_DIR/proxmox-pve-registration-blocked" "$STATE_DIR/proxmox-pbs-registration-blocked" "$STATE_DIR/proxmox-detected-types"
printf 'changed-agent-id\n' > "$STATE_DIR/agent-id"
printf 'changed-connection\n' > "$STATE_DIR/connection.env"
printf 'changed-lifecycle-connection\n' > "$INSTALLER_LIFECYCLE_DIR/connection.env"
printf 'changed-lifecycle-installer\n' > "$INSTALLER_LIFECYCLE_DIR/install.sh"
printf 'changed-lifecycle-checksum\n' > "$INSTALLER_LIFECYCLE_DIR/install.sh.sha256"
chmod 0777 "$STATE_DIR" "$STATE_DIR/cache" "$STATE_DIR/cache/sample"
printf 'outside-state\n' > "$EXPECTED_DIR/outside-target"
chmod 0600 "$EXPECTED_DIR/outside-target"
rm -f "$STATE_DIR/cache/sample"
ln -s "$EXPECTED_DIR/outside-target" "$STATE_DIR/cache/sample"
mkdir -p "$PRIVILEGED_HELPER_CREDENTIAL_DIR"
printf 'moved-monitoring-token\n' > "$PRIVILEGED_HELPER_CREDENTIAL_DIR/token"
printf 'runner-still-independent\n' > "$ACTION_RUNNER_SENTINEL"
safe_profile_restore_transaction "$transaction" automatic-failure
cmp "$INSTALL_DIR/$BINARY_NAME" "$EXPECTED_DIR/collector-binary"
grep -q '^User=root$' "$SAFE_PROFILE_COLLECTOR_UNIT"
! grep -Eq -- '(^|[[:space:]])--enable-commands([[:space:]]|$)' "$SAFE_PROFILE_COLLECTOR_UNIT"
cmp "$STATE_DIR/token" "$EXPECTED_DIR/state-token"
cmp "$STATE_DIR/runtime.token" "$EXPECTED_DIR/runtime-token"
cmp "$STATE_DIR/agent-id" "$EXPECTED_DIR/agent-id"
cmp "$STATE_DIR/connection.env" "$EXPECTED_DIR/connection-env"
cmp "$INSTALLER_LIFECYCLE_DIR/connection.env" "$EXPECTED_DIR/lifecycle-connection-env"
cmp "$INSTALLER_LIFECYCLE_DIR/install.sh" "$EXPECTED_DIR/lifecycle-install-script"
cmp "$INSTALLER_LIFECYCLE_DIR/install.sh.sha256" "$EXPECTED_DIR/lifecycle-install-checksum"
grep -q '^legacy-generic$' "$STATE_DIR/proxmox-registered"
grep -q '^legacy-pve$' "$STATE_DIR/proxmox-pve-registered"
grep -q '^legacy-pbs$' "$STATE_DIR/proxmox-pbs-registered"
grep -q '^legacy-pve-blocked$' "$STATE_DIR/proxmox-pve-registration-blocked"
grep -q '^legacy-pbs-blocked$' "$STATE_DIR/proxmox-pbs-registration-blocked"
grep -q '^pve,pbs$' "$STATE_DIR/proxmox-detected-types"
test "$(stat -c '%a' "$STATE_DIR")" = 750
test "$(stat -c '%a' "$STATE_DIR/cache")" = 777
test -L "$STATE_DIR/cache/sample"
test "$(stat -c '%a' "$EXPECTED_DIR/outside-target")" = 600
test ! -e "$STATE_DIR/cache/runtime.db-wal"
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
SAFE_PROFILE_PRIOR_REGISTRATION_LAST_SEEN=before
resolve_agent_health_url() { printf 'http://127.0.0.1:9191/readyz\n'; }
sleep() { :; }
curl() { return 0; }
systemctl() { return 0; }
safe_profile_verify_effective_target() { return 0; }
safe_profile_probe_helper_protocol() { return 0; }
verify_agent_server_registration_with_retry() { [[ "${1:-}" == before ]]; }
` + gate + `
safe_profile_verify_declared_health
curl_attempts=0
helper_attempts=0
curl() {
  curl_attempts=$((curl_attempts + 1))
  (( curl_attempts >= 3 ))
}
safe_profile_probe_helper_protocol() {
  helper_attempts=$((helper_attempts + 1))
  (( helper_attempts >= 2 ))
}
safe_profile_verify_declared_health
test "$curl_attempts" = 4
test "$helper_attempts" = 2
safe_profile_probe_helper_protocol() { return 1; }
if safe_profile_verify_declared_health; then
  echo 'helper protocol failure was accepted' >&2
  exit 1
fi
safe_profile_probe_helper_protocol() { return 0; }
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

func TestSafeProfileFailsClosedOnEffectiveSystemdOverrides(t *testing.T) {
	for _, tc := range []struct {
		name, property, value string
	}{
		{name: "drop-in", property: "DropInPaths", value: "/etc/systemd/system/pulse-agent.service.d/override.conf"},
		{name: "different fragment", property: "FragmentPath", value: "/usr/lib/systemd/system/pulse-agent.service"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			harness := safeProfileHarness(t, root, false)
			old := tc.property + `) printf '%s\n' "$SAFE_PROFILE_COLLECTOR_UNIT" ;;`
			if tc.property == "DropInPaths" {
				old = `DropInPaths) printf '\n' ;;`
			}
			replacement := tc.property + `) printf '%s\n' '` + tc.value + `' ;;`
			harness = strings.Replace(harness, old, replacement, 1) + "\nsafe_profile_begin_transaction\n"
			out, err := exec.Command("bash", "-c", harness).CombinedOutput()
			if err == nil || !strings.Contains(string(out), "Refusing safe-profile migration") {
				t.Fatalf("override was not rejected: err=%v\n%s", err, out)
			}
		})
	}
}

func TestSafeProfileValidatesEveryEffectiveSystemdBoundary(t *testing.T) {
	testCases := []struct {
		name, unit, property, value string
	}{
		{name: "collector drop-in", unit: "pulse-agent.service", property: "DropInPaths", value: "/etc/systemd/system/pulse-agent.service.d/override.conf"},
		{name: "collector executable", unit: "pulse-agent.service", property: "ExecStart", value: "{ path=/tmp/collector ; argv[]=/tmp/collector ; }"},
		{name: "collector hardening", unit: "pulse-agent.service", property: "NoNewPrivileges", value: "no"},
		{name: "helper service fragment", unit: "pulse-agent-helper.service", property: "FragmentPath", value: "/usr/lib/systemd/system/pulse-agent-helper.service"},
		{name: "helper service drop-in", unit: "pulse-agent-helper.service", property: "DropInPaths", value: "/etc/systemd/system/pulse-agent-helper.service.d/override.conf"},
		{name: "helper executable", unit: "pulse-agent-helper.service", property: "ExecStart", value: "{ path=/tmp/helper ; argv[]=/tmp/helper ; }"},
		{name: "helper private network", unit: "pulse-agent-helper.service", property: "PrivateNetwork", value: "no"},
		{name: "helper common hardening", unit: "pulse-agent-helper.service", property: "ProtectKernelModules", value: "no"},
		{name: "helper task limit", unit: "pulse-agent-helper.service", property: "TasksMax", value: "infinity"},
		{name: "helper descriptor limit", unit: "pulse-agent-helper.service", property: "LimitNOFILE", value: "1048576"},
		{name: "helper memory limit", unit: "pulse-agent-helper.service", property: "MemoryMax", value: "infinity"},
		{name: "helper address families", unit: "pulse-agent-helper.service", property: "RestrictAddressFamilies", value: "AF_UNIX AF_INET"},
		{name: "helper environment", unit: "pulse-agent-helper.service", property: "Environment", value: "PULSE_URL=https://attacker.invalid"},
		{name: "helper writable paths", unit: "pulse-agent-helper.service", property: "ReadWritePaths", value: "/"},
		{name: "helper socket fragment", unit: "pulse-agent-helper.socket", property: "FragmentPath", value: "/usr/lib/systemd/system/pulse-agent-helper.socket"},
		{name: "helper socket drop-in", unit: "pulse-agent-helper.socket", property: "DropInPaths", value: "/etc/systemd/system/pulse-agent-helper.socket.d/override.conf"},
		{name: "helper socket mode", unit: "pulse-agent-helper.socket", property: "SocketMode", value: "0666"},
		{name: "helper socket target", unit: "pulse-agent-helper.socket", property: "Listen", value: "/tmp/attacker.sock (Stream)"},
		{name: "runner fragment", unit: "pulse-agent-runner.service", property: "FragmentPath", value: "/usr/lib/systemd/system/pulse-agent-runner.service"},
		{name: "runner drop-in", unit: "pulse-agent-runner.service", property: "DropInPaths", value: "/etc/systemd/system/pulse-agent-runner.service.d/override.conf"},
		{name: "runner executable", unit: "pulse-agent-runner.service", property: "ExecStart", value: "{ path=/tmp/runner ; argv[]=/tmp/runner ; }"},
		{name: "runner environment file", unit: "pulse-agent-runner.service", property: "EnvironmentFiles", value: "/tmp/attacker.env (ignore_errors=no)"},
		{name: "runner address families", unit: "pulse-agent-runner.service", property: "RestrictAddressFamilies", value: "AF_UNIX AF_INET AF_INET6 AF_NETLINK"},
		{name: "runner common hardening", unit: "pulse-agent-runner.service", property: "LockPersonality", value: "no"},
		{name: "runner filesystem", unit: "pulse-agent-runner.service", property: "ProtectSystem", value: "strict"},
	}

	functions := safeProfileEffectiveSystemdFunctions(t)
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			script := effectiveSystemdHarness(t, testCase.unit, testCase.property, testCase.value) + "\n" + functions + `
if safe_profile_verify_effective_target; then
  echo 'unsafe effective systemd profile was accepted' >&2
  exit 1
fi
`
			if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
				t.Fatalf("effective override rehearsal: %v\n%s", err, out)
			}
		})
	}

	t.Run("canonical profile", func(t *testing.T) {
		script := effectiveSystemdHarness(t, "", "", "") + "\n" + functions + "\nsafe_profile_verify_effective_target\n"
		if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
			t.Fatalf("canonical effective profile rejected: %v\n%s", err, out)
		}
	})
}

func TestActionRunnerProvisionChecksEffectiveUnitBeforeStart(t *testing.T) {
	provision := extractInstallShellFunction(t, "provision_action_runner")
	validation := strings.Index(provision, "action_runner_verify_effective_target")
	enable := strings.Index(provision, `systemctl enable "${ACTION_RUNNER_NAME}.service"`)
	restart := strings.Index(provision, `systemctl restart "${ACTION_RUNNER_NAME}.service"`)
	if validation < 0 || enable < 0 || restart < 0 || validation > enable || validation > restart {
		t.Fatal("action-runner effective systemd validation does not precede service enable/restart")
	}
}

func TestTypedHelperProvisionChecksEffectiveUnitsBeforeSocketActivation(t *testing.T) {
	provision := extractInstallShellFunction(t, "provision_typed_privileged_helper")
	validation := strings.Index(provision, "safe_profile_verify_helper_effective_target")
	enable := strings.Index(provision, `systemctl enable --now "${PRIVILEGED_HELPER_NAME}.socket"`)
	if validation < 0 || enable < 0 || validation > enable {
		t.Fatal("typed-helper effective systemd validation does not precede socket activation")
	}
}

func TestTypedHelperUnitRendersBoundedResources(t *testing.T) {
	unitPath := filepath.Join(t.TempDir(), "pulse-agent-helper.service")
	render := extractInstallShellFunction(t, "render_privileged_helper_service_unit")
	script := `
set -euo pipefail
PRIVILEGED_HELPER_NAME=pulse-agent-helper
PRIVILEGED_HELPER_UPDATE_QUARANTINE_DIR=/var/lib/pulse-agent/update-quarantine
PRIVILEGED_HELPER_STATE_DIR=/var/lib/pulse-agent-helper
` + render + `
render_privileged_helper_service_unit "` + unitPath + `" /usr/local/lib/pulse-agent/pulse-agent-helper
`
	if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("render typed-helper unit: %v\n%s", err, out)
	}
	content, err := os.ReadFile(unitPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, directive := range []string{"TasksMax=64", "LimitNOFILE=256", "MemoryMax=256M"} {
		if !strings.Contains(string(content), directive+"\n") {
			t.Errorf("typed-helper unit omitted %s", directive)
		}
	}
}

func TestSafeProfileRegistrationMustAdvanceLastSeen(t *testing.T) {
	functions := extractInstallShellFunction(t, "verify_agent_server_registration")
	script := `
set -euo pipefail
AGENT_ID=agent-1
HOSTNAME_OVERRIDE=
PULSE_URL=https://pulse.example
AGENT_REGISTRATION_LAST_SEEN=
LOOKUP_LAST_SEEN=
LOOKUP_RC=0
run_collector_lifecycle_command() {
  test "$1" = collector-verify-registration
  shift
  test "$1" = --agent-id
  test "$2" = agent-1
  test "$3" = --previous-last-seen
  test "$4" = '2026-08-30T10:00:00Z'
  if [[ "$LOOKUP_RC" -ne 0 ]]; then
    return "$LOOKUP_RC"
  fi
  printf '%s\n' "$LOOKUP_LAST_SEEN"
}
` + functions + `
LOOKUP_RC=1
if verify_agent_server_registration '2026-08-30T10:00:00Z'; then
  echo 'stale registration was accepted' >&2
  exit 1
fi
LOOKUP_RC=0
LOOKUP_LAST_SEEN='2026-08-30T10:00:31Z'
verify_agent_server_registration '2026-08-30T10:00:00Z'
test "$AGENT_REGISTRATION_LAST_SEEN" = '2026-08-30T10:00:31Z'
`
	if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("fresh registration rehearsal: %v\n%s", err, out)
	}
}

func TestSafeProfileHelperProtocolProbeValidatesFramedHealth(t *testing.T) {
	for _, success := range []bool{true, false} {
		t.Run(map[bool]string{true: "healthy", false: "typed failure"}[success], func(t *testing.T) {
			root, err := os.MkdirTemp("/tmp", "pulse-helper-probe-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(root)
			socketPath := filepath.Join(root, "helper.sock")
			listener, err := net.Listen("unix", socketPath)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			serverErr := make(chan error, 1)
			go func() {
				conn, acceptErr := listener.Accept()
				if acceptErr != nil {
					serverErr <- acceptErr
					return
				}
				defer conn.Close()
				var header [4]byte
				if _, readErr := io.ReadFull(conn, header[:]); readErr != nil {
					serverErr <- readErr
					return
				}
				requestBody := make([]byte, binary.BigEndian.Uint32(header[:]))
				if _, readErr := io.ReadFull(conn, requestBody); readErr != nil {
					serverErr <- readErr
					return
				}
				var request map[string]any
				if decodeErr := json.Unmarshal(requestBody, &request); decodeErr != nil {
					serverErr <- decodeErr
					return
				}
				response, marshalErr := json.Marshal(map[string]any{
					"protocolVersion": 1, "requestId": request["requestId"],
					"operation": "helper.health", "operationVersion": 1,
					"success": success, "result": map[string]any{"status": "ok", "protocolVersion": 1},
				})
				if marshalErr != nil {
					serverErr <- marshalErr
					return
				}
				binary.BigEndian.PutUint32(header[:], uint32(len(response)))
				_, writeErr := conn.Write(append(header[:], response...))
				serverErr <- writeErr
			}()

			fakeBin := filepath.Join(root, "bin")
			mustMkdirAll(t, fakeBin)
			runuser := filepath.Join(fakeBin, "runuser")
			if err := os.WriteFile(runuser, []byte("#!/bin/sh\nwhile [ \"$#\" -gt 0 ] && [ \"$1\" != -- ]; do shift; done\nshift\nexec \"$@\"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			script := `
set -euo pipefail
PATH="` + fakeBin + `:$PATH"
LEAST_PRIVILEGE_USER=pulse-agent
PRIVILEGED_HELPER_SOCKET_PATH="` + socketPath + `"
SAFE_PROFILE_TRANSACTION_DIR="` + root + `"
id() { return 0; }
` + extractInstallShellFunction(t, "safe_profile_probe_helper_protocol") + "\n"
			if success {
				script += "safe_profile_probe_helper_protocol\n"
			} else {
				script += "if safe_profile_probe_helper_protocol; then exit 91; fi\n"
			}
			out, runErr := exec.Command("bash", "-c", script).CombinedOutput()
			if runErr != nil {
				t.Fatalf("protocol probe: %v\n%s", runErr, out)
			}
			if err := <-serverErr; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSafeProfileExitTrapRestoresUncommittedTransaction(t *testing.T) {
	root := t.TempDir()
	harness := safeProfileHarness(t, root, true) + "\n" + extractInstallShellFunction(t, "cleanup") + `
TMP_FILES=()
trap cleanup EXIT
safe_profile_begin_transaction
printf 'uncommitted-binary\n' > "$INSTALL_DIR/$BINARY_NAME"
rm -f "$STATE_DIR/proxmox-registered"
printf 'runner-unchanged\n' > "$ACTION_RUNNER_SENTINEL"
exit 23
`
	out, err := exec.Command("bash", "-c", harness).CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 23 {
		t.Fatalf("exit trap status=%v\n%s", err, out)
	}
	assertFileBody(t, filepath.Join(root, "bin", "pulse-agent"), "old-binary\n")
	assertFileBody(t, filepath.Join(root, "state", "proxmox-registered"), "legacy-generic\n")
	assertFileBody(t, filepath.Join(root, "runner-sentinel"), "runner-unchanged\n")
}

func TestSafeProfileDockerDegradationRequiresCollectorOwnedRootlessRuntime(t *testing.T) {
	function := extractInstallShellFunction(t, "safe_profile_apply_docker_degradation")
	for _, tc := range []struct {
		name          string
		usable        bool
		helperEnabled bool
		want          string
	}{
		{name: "rootless", usable: true, helperEnabled: true, want: "enabled=true explicit=false"},
		{name: "typed helper summary", helperEnabled: true, want: "enabled=true explicit=false"},
		{name: "no safe source", want: "enabled=false explicit=true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script := `
set -euo pipefail
SAFE_PROFILE_ACTION=apply
ENABLE_DOCKER=true
DOCKER_EXPLICIT=false
PRIVILEGED_HELPER_ENABLED=` + map[bool]string{true: "true", false: "false"}[tc.helperEnabled] + `
ROOTLESS_RUNTIME_KIND=docker
ROOTLESS_RUNTIME_SOCKET_PATH=/run/user/991/docker.sock
log_info() { :; }
log_warn() { printf '%s\n' "$*"; }
safe_profile_selected_rootless_runtime_usable() { return ` + map[bool]string{true: "0", false: "1"}[tc.usable] + `; }
` + function + `
safe_profile_apply_docker_degradation
printf 'enabled=%s explicit=%s\n' "$ENABLE_DOCKER" "$DOCKER_EXPLICIT"
`
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if err != nil {
				t.Fatalf("docker degradation: %v\n%s", err, out)
			}
			if !strings.Contains(string(out), tc.want) {
				t.Fatalf("output missing %q:\n%s", tc.want, out)
			}
		})
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
		filepath.Join(binDir, "pulse-agent"):                        "old-binary\n",
		filepath.Join(unitDir, "pulse-agent.service"):               "[Service]\nUser=root\nAmbientCapabilities=CAP_SETUID CAP_SETGID\nExecStart=/bin/pulse-agent --enable-commands\n",
		filepath.Join(root, "sudoers"):                              "legacy sudo grant\n",
		filepath.Join(helperDir, "smartctl"):                        "legacy smart wrapper\n",
		filepath.Join(helperDir, "pct"):                             "legacy pct wrapper\n",
		filepath.Join(stateDir, "token"):                            "monitoring-token\n",
		filepath.Join(stateDir, "runtime.token"):                    "runtime-monitoring-token\n",
		filepath.Join(stateDir, "agent-id"):                         "stable-agent-id\n",
		filepath.Join(stateDir, "connection.env"):                   "PULSE_URL='https://pulse.example'\n",
		filepath.Join(stateDir, "cache", "sample"):                  "cached-state\n",
		filepath.Join(stateDir, "proxmox-registered"):               "legacy-generic\n",
		filepath.Join(stateDir, "proxmox-pve-registered"):           "legacy-pve\n",
		filepath.Join(stateDir, "proxmox-pbs-registered"):           "legacy-pbs\n",
		filepath.Join(stateDir, "proxmox-pve-registration-blocked"): "legacy-pve-blocked\n",
		filepath.Join(stateDir, "proxmox-pbs-registration-blocked"): "legacy-pbs-blocked\n",
		filepath.Join(stateDir, "proxmox-detected-types"):           "pve,pbs\n",
		filepath.Join(credentialDir, "connection.env"):              "PULSE_STATE_DIR='" + stateDir + "'\n",
		filepath.Join(credentialDir, "install.sh"):                  "#!/usr/bin/env bash\necho legacy-installer\n",
		filepath.Join(credentialDir, "install.sh.sha256"):           "legacy-checksum\n",
	}
	for path, body := range files {
		mustMkdirAll(t, filepath.Dir(path))
		mustWrite(t, path, body)
	}
	if err := os.Chmod(stateDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(stateDir, "cache"), 0o710); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(stateDir, "cache", "sample"), 0o640); err != nil {
		t.Fatal(err)
	}
	for source, name := range map[string]string{
		filepath.Join(binDir, "pulse-agent"):              "collector-binary",
		filepath.Join(unitDir, "pulse-agent.service"):     "collector-unit",
		filepath.Join(stateDir, "token"):                  "state-token",
		filepath.Join(stateDir, "runtime.token"):          "runtime-token",
		filepath.Join(stateDir, "agent-id"):               "agent-id",
		filepath.Join(stateDir, "connection.env"):         "connection-env",
		filepath.Join(credentialDir, "connection.env"):    "lifecycle-connection-env",
		filepath.Join(credentialDir, "install.sh"):        "lifecycle-install-script",
		filepath.Join(credentialDir, "install.sh.sha256"): "lifecycle-install-checksum",
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
INSTALLER_LIFECYCLE_DIR="$PRIVILEGED_HELPER_CREDENTIAL_DIR"
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
systemctl() {
  case "${1:-}" in
    show)
      case "${4:-}" in
        FragmentPath) printf '%s\n' "$SAFE_PROFILE_COLLECTOR_UNIT" ;;
        DropInPaths) printf '\n' ;;
      esac
      ;;
    is-active|is-enabled) return 0 ;;
  esac
  return 0
}
getent() { [[ "${1:-}" == group && "${2:-}" == docker ]]; }
id() { if [[ "${1:-}" == -nG ]]; then printf '` + membership + `\n'; fi; return 0; }
gpasswd() { printf 'gpasswd %s\n' "$*" >> "$CALL_LOG"; }
stat() {
  if [[ "${1:-}" == -c ]]; then
    if /usr/bin/stat -c "$2" "$3" 2>/dev/null; then
      return
    fi
    case "$2" in
      %u) /usr/bin/stat -f '%u' "$3" ;;
      %g) /usr/bin/stat -f '%g' "$3" ;;
      %a) /usr/bin/stat -f '%Lp' "$3" ;;
      *) return 1 ;;
    esac
    return
  fi
  /usr/bin/stat "$@"
}
` + safeProfileTransactionFunctions(t) + "\n"
}

func effectiveSystemdHarness(t *testing.T, overrideUnit, overrideProperty, overrideValue string) string {
	t.Helper()
	root := t.TempDir()
	collectorUnit := filepath.Join(root, "pulse-agent.service")
	helperServiceUnit := filepath.Join(root, "pulse-agent-helper.service")
	helperSocketUnit := filepath.Join(root, "pulse-agent-helper.socket")
	runnerUnit := filepath.Join(root, "pulse-agent-runner.service")
	for _, path := range []string{collectorUnit, helperServiceUnit, helperSocketUnit, runnerUnit} {
		mustWrite(t, path, "fixture unit\n")
	}
	return `
set -euo pipefail
AGENT_NAME=pulse-agent
BINARY_NAME=pulse-agent
INSTALL_DIR=/usr/local/bin
LEAST_PRIVILEGE_USER=pulse-agent
SAFE_PROFILE_COLLECTOR_UNIT="` + collectorUnit + `"
PRIVILEGED_HELPER_NAME=pulse-agent-helper
PRIVILEGED_HELPER_SERVICE_UNIT="` + helperServiceUnit + `"
PRIVILEGED_HELPER_SOCKET_UNIT="` + helperSocketUnit + `"
PRIVILEGED_HELPER_BINARY_PATH=/usr/local/lib/pulse-agent/pulse-agent-helper
PRIVILEGED_HELPER_SOCKET_PATH=/run/pulse-agent/helper.sock
PRIVILEGED_HELPER_UPDATE_QUARANTINE_DIR=/var/lib/pulse-agent/update-quarantine
PRIVILEGED_HELPER_STATE_DIR=/var/lib/pulse-agent-helper
ACTION_RUNNER_NAME=pulse-agent-runner
ACTION_RUNNER_SERVICE_UNIT="` + runnerUnit + `"
ACTION_RUNNER_BINARY_PATH=/usr/local/lib/pulse-agent/pulse-agent-runner
ACTION_RUNNER_ENV_FILE=/etc/pulse-agent-runner/runner.env
ACTION_RUNNER_STATE_DIR=/var/lib/pulse-agent-runner
OVERRIDE_UNIT='` + overrideUnit + `'
OVERRIDE_PROPERTY='` + overrideProperty + `'
OVERRIDE_VALUE='` + overrideValue + `'
systemctl() {
  [[ "${1:-}" == show ]]
  local unit="${2:-}"
  local property="${4:-}"
  if [[ "$unit" == "$OVERRIDE_UNIT" && "$property" == "$OVERRIDE_PROPERTY" ]]; then
    printf '%s\n' "$OVERRIDE_VALUE"
    return 0
  fi
  case "$unit:$property" in
    pulse-agent.service:FragmentPath) printf '%s\n' "$SAFE_PROFILE_COLLECTOR_UNIT" ;;
    pulse-agent.service:DropInPaths|pulse-agent.service:AmbientCapabilities) printf '\n' ;;
    pulse-agent.service:User) printf 'pulse-agent\n' ;;
    pulse-agent.service:ExecStart) printf '{ path=/usr/local/bin/pulse-agent ; argv[]=/usr/local/bin/pulse-agent --enable-host --command-authority monitoring-only ; }\n' ;;
    pulse-agent.service:Environment) printf 'PULSE_AGENT_HELPER_SOCKET=/run/pulse-agent/helper.sock\n' ;;
    pulse-agent.service:UMask) printf '0077\n' ;;
    pulse-agent.service:NoNewPrivileges|pulse-agent.service:PrivateTmp|pulse-agent.service:ProtectKernelTunables|pulse-agent.service:ProtectKernelModules|pulse-agent.service:ProtectControlGroups|pulse-agent.service:LockPersonality|pulse-agent.service:RestrictSUIDSGID) printf 'yes\n' ;;
    pulse-agent.service:PrivateDevices) printf 'no\n' ;;
    pulse-agent.service:SystemCallArchitectures) printf 'native\n' ;;
    pulse-agent-helper.service:FragmentPath) printf '%s\n' "$PRIVILEGED_HELPER_SERVICE_UNIT" ;;
    pulse-agent-helper.service:DropInPaths|pulse-agent-helper.service:AmbientCapabilities|pulse-agent-helper.service:Environment|pulse-agent-helper.service:EnvironmentFiles) printf '\n' ;;
    pulse-agent-helper.service:ExecStart) printf '{ path=/usr/local/lib/pulse-agent/pulse-agent-helper ; argv[]=/usr/local/lib/pulse-agent/pulse-agent-helper ; }\n' ;;
    pulse-agent-helper.service:User|pulse-agent-helper.service:Group) printf 'root\n' ;;
    pulse-agent-helper.service:UMask) printf '0077\n' ;;
    pulse-agent-helper.service:NoNewPrivileges|pulse-agent-helper.service:PrivateTmp|pulse-agent-helper.service:ProtectKernelTunables|pulse-agent-helper.service:ProtectKernelModules|pulse-agent-helper.service:ProtectControlGroups|pulse-agent-helper.service:LockPersonality|pulse-agent-helper.service:RestrictSUIDSGID|pulse-agent-helper.service:PrivateNetwork|pulse-agent-helper.service:ProtectHome) printf 'yes\n' ;;
    pulse-agent-helper.service:PrivateDevices) printf 'no\n' ;;
    pulse-agent-helper.service:SystemCallArchitectures) printf 'native\n' ;;
    pulse-agent-helper.service:ProtectSystem) printf 'strict\n' ;;
    pulse-agent-helper.service:RestrictAddressFamilies) printf 'AF_UNIX\n' ;;
    pulse-agent-helper.service:TasksMax) printf '64\n' ;;
    pulse-agent-helper.service:LimitNOFILE) printf '256\n' ;;
    pulse-agent-helper.service:MemoryMax) printf '268435456\n' ;;
    pulse-agent-helper.service:ReadOnlyPaths) printf '/var/lib/pulse-agent/update-quarantine\n' ;;
    pulse-agent-helper.service:ReadWritePaths) printf '/usr/local/bin /var/lib/pulse-agent-helper\n' ;;
    pulse-agent-helper.socket:FragmentPath) printf '%s\n' "$PRIVILEGED_HELPER_SOCKET_UNIT" ;;
    pulse-agent-helper.socket:DropInPaths) printf '\n' ;;
    pulse-agent-helper.socket:SocketUser) printf 'root\n' ;;
    pulse-agent-helper.socket:SocketGroup) printf 'pulse-agent\n' ;;
    pulse-agent-helper.socket:SocketMode) printf '0660\n' ;;
    pulse-agent-helper.socket:DirectoryMode) printf '0755\n' ;;
    pulse-agent-helper.socket:RemoveOnStop) printf 'yes\n' ;;
    pulse-agent-helper.socket:Listen) printf '/run/pulse-agent/helper.sock (Stream)\n' ;;
    pulse-agent-runner.service:FragmentPath) printf '%s\n' "$ACTION_RUNNER_SERVICE_UNIT" ;;
    pulse-agent-runner.service:DropInPaths|pulse-agent-runner.service:AmbientCapabilities) printf '\n' ;;
    pulse-agent-runner.service:ExecStart) printf '{ path=/usr/local/lib/pulse-agent/pulse-agent-runner ; argv[]=/usr/local/lib/pulse-agent/pulse-agent-runner ; }\n' ;;
    pulse-agent-runner.service:User|pulse-agent-runner.service:Group) printf 'root\n' ;;
    pulse-agent-runner.service:UMask) printf '0077\n' ;;
    pulse-agent-runner.service:NoNewPrivileges|pulse-agent-runner.service:PrivateTmp|pulse-agent-runner.service:ProtectKernelTunables|pulse-agent-runner.service:ProtectKernelModules|pulse-agent-runner.service:ProtectControlGroups|pulse-agent-runner.service:LockPersonality|pulse-agent-runner.service:RestrictSUIDSGID|pulse-agent-runner.service:ProtectHome) printf 'yes\n' ;;
    pulse-agent-runner.service:PrivateDevices) printf 'no\n' ;;
    pulse-agent-runner.service:SystemCallArchitectures) printf 'native\n' ;;
    pulse-agent-runner.service:PrivateNetwork|pulse-agent-runner.service:ProtectSystem) printf 'no\n' ;;
    pulse-agent-runner.service:RestrictAddressFamilies) printf 'AF_INET6 AF_UNIX AF_INET\n' ;;
    pulse-agent-runner.service:ReadWritePaths) printf '/var/lib/pulse-agent-runner\n' ;;
    pulse-agent-runner.service:EnvironmentFiles) printf '/etc/pulse-agent-runner/runner.env (ignore_errors=no)\n' ;;
    *) printf '\n' ;;
  esac
}
`
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
