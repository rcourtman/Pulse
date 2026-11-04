package installtests

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The manager is a closed stateful model; the production parser, rollback,
// snapshot, filesystem and credential restoration functions remain unchanged.
// Manager state is outside the recovery tree so a stopped unit is not confused
// with a write to the saved snapshot or installed files.
func safeProfileRollbackQuiescenceHarness(t *testing.T, root string) string {
	t.Helper()
	return safeProfileHarness(t, root, true) + `
MANAGER_DIR="` + t.TempDir() + `"
systemctl() {
  printf 'systemctl %s\n' "$*" >> "$CALL_LOG"
  local verb="${1:-}" unit="${2:-}" active=active enabled=enabled
  case "$verb" in
    show)
      if [[ "${3:-}" != --property=LoadState,ActiveState,UnitFileState ]]; then
        case "${4:-}" in
          FragmentPath) printf '%s\n' "$SAFE_PROFILE_COLLECTOR_UNIT" ;;
          DropInPaths) printf '\n' ;;
          *) return 96 ;;
        esac
        return
      fi
      [[ $# == 3 ]] || return 96
      if [[ "${ROLLBACK_FAULT_UNIT:-}" == "$unit" && "${ROLLBACK_FAULT:-}" == read-* ]]; then
        case "$ROLLBACK_FAULT" in
          read-refused) printf 'private manager detail\n' >&2; return 1 ;;
          read-timeout) return 124 ;;
          read-empty) return 0 ;;
          read-incomplete) printf 'LoadState=loaded\nActiveState=inactive\n'; return 0 ;;
          read-diagnostic) printf 'private manager detail\n' >&2 ;;
        esac
      fi
      [[ "$unit" != pulse-agent-helper.service ]] || enabled=static
      if [[ "${ROLLBACK_HELPER_ABSENT:-false}" == true && "$unit" == pulse-agent-helper.* ]]; then
        printf 'LoadState=not-found\nActiveState=inactive\nUnitFileState=\n'
        return
      fi
      [[ ! -f "$MANAGER_DIR/stopped-$unit" ]] || active=inactive
      [[ ! -f "$MANAGER_DIR/disabled-$unit" ]] || enabled=disabled
      if [[ "${ROLLBACK_FAULT_UNIT:-}" == "$unit" && "${ROLLBACK_FAULT:-}" == reactivated &&
            -f "$MANAGER_DIR/stopped-pulse-agent-helper.service" ]]; then
        active=active
      fi
      printf 'LoadState=loaded\nActiveState=%s\nUnitFileState=%s\n' "$active" "$enabled"
      ;;
    stop)
      if [[ "${ROLLBACK_FAULT_UNIT:-}" == "$unit" ]]; then
        case "${ROLLBACK_FAULT:-}" in
          stop-refused) printf 'private manager detail\n' >&2; return 1 ;;
          stop-timeout) return 124 ;;
          stop-ignored) return 0 ;;
        esac
      fi
      touch "$MANAGER_DIR/stopped-$unit"
      ;;
    disable)
      if [[ "${ROLLBACK_FAULT_UNIT:-}" == "$unit" ]]; then
        case "${ROLLBACK_FAULT:-}" in
          disable-refused) printf 'private manager detail\n' >&2; return 1 ;;
          disable-timeout) return 124 ;;
          disable-ignored) return 0 ;;
        esac
      fi
      touch "$MANAGER_DIR/disabled-$unit"
      ;;
    start) [[ ! -f "$MANAGER_DIR/stopped-$unit" ]] || unlink "$MANAGER_DIR/stopped-$unit" ;;
    enable) [[ ! -f "$MANAGER_DIR/disabled-$unit" ]] || unlink "$MANAGER_DIR/disabled-$unit" ;;
    daemon-reload) ;;
    *) return 96 ;;
  esac
}
`
}

func prepareSafeProfileRollback(t *testing.T, harness string) {
	t.Helper()
	script := harness + `
safe_profile_begin_transaction
safe_profile_commit_transaction
printf 'migrated-collector\n' > "$INSTALL_DIR/$BINARY_NAME"
printf '[Service]\nUser=pulse-agent\nExecStart=/bin/pulse-agent\n' > "$SAFE_PROFILE_COLLECTOR_UNIT"
printf 'migrated-helper\n' > "$PRIVILEGED_HELPER_BINARY_PATH"
printf 'migrated-helper-service\n' > "$PRIVILEGED_HELPER_SERVICE_UNIT"
printf 'migrated-helper-socket\n' > "$PRIVILEGED_HELPER_SOCKET_UNIT"
printf 'migrated-monitoring-token\n' > "$STATE_DIR/token"
printf 'protected-monitoring-token\n' > "$PRIVILEGED_HELPER_CREDENTIAL_DIR/token"
printf 'saved-CA\n' > "$STATE_DIR/ca.pem"
printf 'runner-independent\n' > "$ACTION_RUNNER_SENTINEL"
`
	if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("prepare retained rollback: %v\n%s", err, out)
	}
}

func TestSafeProfileRollbackRefusesUnconfirmedQuiescenceBeforeRestoration(t *testing.T) {
	for _, unit := range []string{"pulse-agent.service", "pulse-agent-helper.socket", "pulse-agent-helper.service"} {
		for _, fault := range []string{
			"stop-refused", "stop-timeout", "stop-ignored", "disable-refused", "disable-timeout",
			"read-refused", "read-timeout", "read-empty", "read-incomplete", "read-diagnostic",
		} {
			t.Run(unit+"/"+fault, func(t *testing.T) {
				root := t.TempDir()
				harness := safeProfileRollbackQuiescenceHarness(t, root)
				prepareSafeProfileRollback(t, harness)
				before := safeProfileFixtureSnapshot(t, root)
				script := harness + `
transaction=$(safe_profile_manifest_value "$SAFE_PROFILE_CURRENT_FILE" TRANSACTION_DIR)
if safe_profile_restore_transaction "$transaction" refusal-control; then
  printf 'rollback incorrectly succeeded\n' >&2
  exit 91
fi
`
				command := exec.Command("bash", "-c", script)
				command.Env = append(os.Environ(), "ROLLBACK_FAULT_UNIT="+unit, "ROLLBACK_FAULT="+fault)
				out, err := command.CombinedOutput()
				if err != nil || !strings.Contains(string(out), "rollback could not confirm") {
					t.Fatalf("unsafe rollback not refused: %v\n%s", err, out)
				}
				if strings.Contains(string(out), "private manager detail") {
					t.Fatalf("manager diagnostic leaked: %s", out)
				}
				if !reflect.DeepEqual(before, safeProfileFixtureSnapshot(t, root)) {
					t.Fatal("unconfirmed rollback changed installed/saved files, modes, credentials or retained snapshot")
				}
				calls, err := os.ReadFile(filepath.Join(root, "calls.log"))
				if err != nil {
					t.Fatal(err)
				}
				for _, forbidden := range []string{"daemon-reload", "systemctl start ", "systemctl enable ", "gpasswd ", "pulse-agent-runner"} {
					if strings.Contains(string(calls), forbidden) {
						t.Fatalf("unconfirmed rollback reached restoration/runner mutation %q:\n%s", forbidden, calls)
					}
				}
			})
		}
	}
}

func TestSafeProfileRollbackExitTrapRetainsOriginalFailureAndSnapshot(t *testing.T) {
	root := t.TempDir()
	harness := safeProfileRollbackQuiescenceHarness(t, root)
	prepareSafeProfileRollback(t, harness)
	before := safeProfileFixtureSnapshot(t, root)
	script := harness + "\n" + extractInstallShellFunction(t, "cleanup") + `
TMP_FILES=()
SAFE_PROFILE_TRANSACTION_DIR=$(safe_profile_manifest_value "$SAFE_PROFILE_CURRENT_FILE" TRANSACTION_DIR)
SAFE_PROFILE_TRANSACTION_ACTIVE=true
SAFE_PROFILE_TRANSACTION_COMMITTED=false
trap cleanup EXIT
exit 23
`
	command := exec.Command("bash", "-c", script)
	command.Env = append(os.Environ(), "ROLLBACK_FAULT_UNIT=pulse-agent-helper.socket", "ROLLBACK_FAULT=stop-refused")
	out, err := command.CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 23 {
		t.Fatalf("rollback obscured the original migration failure: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "rollback could not confirm") || !strings.Contains(string(out), "Automatic safe-profile rollback failed") {
		t.Fatalf("unconfirmed automatic rollback was not reported: %s", out)
	}
	if !reflect.DeepEqual(before, safeProfileFixtureSnapshot(t, root)) {
		t.Fatal("automatic rollback changed saved files or snapshot without confirmed quiescence")
	}
	calls, err := os.ReadFile(filepath.Join(root, "calls.log"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(calls), "systemctl stop pulse-agent-helper.socket\n") != 1 {
		t.Fatalf("cleanup retried an unconfirmed rollback: %s", calls)
	}
}

func TestSafeProfileRollbackRechecksEarlierUnitsBeforeRestoration(t *testing.T) {
	for _, unit := range []string{"pulse-agent.service", "pulse-agent-helper.socket"} {
		t.Run(unit, func(t *testing.T) {
			root := t.TempDir()
			harness := safeProfileRollbackQuiescenceHarness(t, root)
			prepareSafeProfileRollback(t, harness)
			before := safeProfileFixtureSnapshot(t, root)
			script := harness + `
transaction=$(safe_profile_manifest_value "$SAFE_PROFILE_CURRENT_FILE" TRANSACTION_DIR)
if safe_profile_restore_transaction "$transaction" reactivation-control; then exit 91; fi
`
			command := exec.Command("bash", "-c", script)
			command.Env = append(os.Environ(), "ROLLBACK_FAULT_UNIT="+unit, "ROLLBACK_FAULT=reactivated")
			out, err := command.CombinedOutput()
			if err != nil || !strings.Contains(string(out), "rollback could not confirm") {
				t.Fatalf("reactivation not refused: %v\n%s", err, out)
			}
			if !reflect.DeepEqual(before, safeProfileFixtureSnapshot(t, root)) {
				t.Fatal("rollback restored files after an earlier unit reactivated")
			}
		})
	}
}

func TestSafeProfileRollbackQuiescentControlRestoresSavedProfile(t *testing.T) {
	for _, absentHelper := range []bool{false, true} {
		t.Run(map[bool]string{false: "helper present", true: "helper absent"}[absentHelper], func(t *testing.T) {
			root := t.TempDir()
			harness := safeProfileRollbackQuiescenceHarness(t, root)
			if absentHelper {
				harness += "\nROLLBACK_HELPER_ABSENT=true\n"
			}
			prepareSafeProfileRollback(t, harness)
			script := harness + `
transaction=$(safe_profile_manifest_value "$SAFE_PROFILE_CURRENT_FILE" TRANSACTION_DIR)
safe_profile_restore_transaction "$transaction" successful-control
cmp "$INSTALL_DIR/$BINARY_NAME" "$EXPECTED_DIR/collector-binary"
cmp "$STATE_DIR/agent-id" "$EXPECTED_DIR/agent-id"
cmp "$STATE_DIR/token" "$EXPECTED_DIR/state-token"
cmp "$STATE_DIR/connection.env" "$EXPECTED_DIR/connection-env"
cmp "$INSTALLER_LIFECYCLE_DIR/install.sh" "$EXPECTED_DIR/lifecycle-install-script"
grep -qx 'saved-CA' "$STATE_DIR/ca.pem"
grep -qx 'runner-independent' "$ACTION_RUNNER_SENTINEL"
! grep -q -- --enable-commands "$SAFE_PROFILE_COLLECTOR_UNIT"
grep -qx 'CURRENT_PROFILE=legacy-root-command-capable' "$SAFE_PROFILE_CURRENT_FILE"
`
			command := exec.Command("bash", "-c", script)
			if absentHelper {
				command.Env = append(os.Environ(), "ROLLBACK_HELPER_ABSENT=true")
			}
			if out, err := command.CombinedOutput(); err != nil {
				t.Fatalf("confirmed rollback did not restore saved profile: %v\n%s", err, out)
			}
			calls, err := os.ReadFile(filepath.Join(root, "calls.log"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(calls), "pulse-agent-runner") {
				t.Fatal("rollback touched independent runner")
			}
		})
	}
}
