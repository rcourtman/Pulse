package installtests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The collector account owns the state root while the safe profile is active.
// These controls leave there what that account can leave, run the production
// rollback, and require it to replace the saved names themselves: nothing
// reached through them may change, and the root stays closed while it works.
func TestSafeProfileRollbackReplacesCollectorPlantedStateEntries(t *testing.T) {
	root := t.TempDir()
	harness := safeProfileRollbackQuiescenceHarness(t, root)
	prepareSafeProfileRollback(t, harness)
	script := harness + `
transaction=$(safe_profile_manifest_value "$SAFE_PROFILE_CURRENT_FILE" TRANSACTION_DIR)
OUTSIDE_DIR="` + t.TempDir() + `"
mkdir "$OUTSIDE_DIR/directory"
for name in state-token agent-id connection-env marker; do
  printf 'outside-%s\n' "$name" > "$OUTSIDE_DIR/$name"
done
# cp copies into a directory under the snapshot's own name, which differs from
# the restored name for the token and the connection file.
rm -f "$STATE_DIR/token" "$STATE_DIR/agent-id" "$STATE_DIR/connection.env"
mkdir "$STATE_DIR/token" "$STATE_DIR/agent-id" "$STATE_DIR/connection.env"
ln -s "$OUTSIDE_DIR/state-token" "$STATE_DIR/token/state-token"
ln -s "$OUTSIDE_DIR/agent-id" "$STATE_DIR/agent-id/agent-id"
ln -s "$OUTSIDE_DIR/connection-env" "$STATE_DIR/connection.env/connection-env"
rm -f "$STATE_DIR/proxmox-registered" "$STATE_DIR/proxmox-detected-types"
ln -s "$OUTSIDE_DIR/marker" "$STATE_DIR/proxmox-registered"
ln -s "$OUTSIDE_DIR/directory" "$STATE_DIR/proxmox-detected-types"
chmod 0777 "$STATE_DIR"
cp() {
  case "${!#}" in
    "$STATE_DIR"/*)
      if [[ "$(stat -c '%a' "$STATE_DIR")" != 755 ]]; then
        printf 'state root still writable while restoring %s\n' "${!#}" >&2
        exit 97
      fi ;;
  esac
  command cp "$@"
}
safe_profile_restore_transaction "$transaction" planted-control
for name in token agent-id connection.env proxmox-registered proxmox-detected-types; do
  test -f "$STATE_DIR/$name"
  test ! -L "$STATE_DIR/$name"
done
cmp "$STATE_DIR/token" "$EXPECTED_DIR/state-token"
cmp "$STATE_DIR/agent-id" "$EXPECTED_DIR/agent-id"
cmp "$STATE_DIR/connection.env" "$EXPECTED_DIR/connection-env"
grep -qx 'legacy-generic' "$STATE_DIR/proxmox-registered"
grep -qx 'pve,pbs' "$STATE_DIR/proxmox-detected-types"
for name in state-token agent-id connection-env marker; do
  grep -qx "outside-$name" "$OUTSIDE_DIR/$name"
done
test -z "$(ls -A "$OUTSIDE_DIR/directory")"
test "$(stat -c '%a' "$STATE_DIR")" = 750
grep -qx 'saved-CA' "$STATE_DIR/ca.pem"
grep -qx 'CURRENT_PROFILE=legacy-root-command-capable' "$SAFE_PROFILE_CURRENT_FILE"
`
	if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("rollback followed collector-planted state entries: %v\n%s", err, out)
	}
}

func TestSafeProfileRollbackStopsAtFirstUnrestoredEntry(t *testing.T) {
	root := t.TempDir()
	harness := safeProfileRollbackQuiescenceHarness(t, root)
	prepareSafeProfileRollback(t, harness)
	script := harness + `
transaction=$(safe_profile_manifest_value "$SAFE_PROFILE_CURRENT_FILE" TRANSACTION_DIR)
rm -f "$transaction/agent-id"
printf 'migrated-connection\n' > "$STATE_DIR/connection.env"
chmod 0777 "$STATE_DIR"
if safe_profile_restore_transaction "$transaction" unrestored-control; then
  printf 'rollback incorrectly succeeded\n' >&2
  exit 91
fi
test ! -e "$STATE_DIR/agent-id"
grep -qx 'migrated-connection' "$STATE_DIR/connection.env"
test "$(stat -c '%a' "$STATE_DIR")" = 755
grep -qx "TRANSACTION_DIR=${transaction}" "$SAFE_PROFILE_CURRENT_FILE"
! grep -q '^ROLLBACK_REASON=' "$SAFE_PROFILE_CURRENT_FILE"
`
	if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("rollback continued past an unrestored entry: %v\n%s", err, out)
	}
	calls, err := os.ReadFile(filepath.Join(root, "calls.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"daemon-reload", "systemctl start ", "systemctl enable ", "gpasswd "} {
		if strings.Contains(string(calls), forbidden) {
			t.Fatalf("incomplete rollback reached %q:\n%s", forbidden, calls)
		}
	}
}

func TestSafeProfileRollbackRefusesUnsafeStateRootBeforeQuiescence(t *testing.T) {
	root := t.TempDir()
	harness := safeProfileRollbackQuiescenceHarness(t, root)
	prepareSafeProfileRollback(t, harness)
	script := harness + `
transaction=$(safe_profile_manifest_value "$SAFE_PROFILE_CURRENT_FILE" TRANSACTION_DIR)
mv "$STATE_DIR" "${STATE_DIR}.moved"
ln -s "${STATE_DIR}.moved" "$STATE_DIR"
: > "$CALL_LOG"
if safe_profile_restore_transaction "$transaction" state-root-control; then
  printf 'rollback incorrectly succeeded\n' >&2
  exit 91
fi
test ! -s "$CALL_LOG"
grep -qx 'migrated-collector' "$INSTALL_DIR/$BINARY_NAME"
grep -qx 'migrated-monitoring-token' "${STATE_DIR}.moved/token"
`
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "Refusing unsafe safe-profile state directory") {
		t.Fatalf("symlinked state root not refused: %v\n%s", err, out)
	}
}

// Format-v1 snapshots recorded no state-root metadata. The ownership calls are
// modelled so the control also runs without the privilege to give files away.
func TestSafeProfileRollbackReturnsFormatOneStateRootToCollector(t *testing.T) {
	root := t.TempDir()
	harness := safeProfileRollbackQuiescenceHarness(t, root)
	prepareSafeProfileRollback(t, harness)
	script := harness + `
transaction=$(safe_profile_manifest_value "$SAFE_PROFILE_CURRENT_FILE" TRANSACTION_DIR)
sed 's/^FORMAT_VERSION=3$/FORMAT_VERSION=1/' "$transaction/manifest.env" > "$transaction/manifest.v1"
mv "$transaction/manifest.v1" "$transaction/manifest.env"
chown() { printf 'chown %s\n' "$*" >> "$CALL_LOG"; }
cp() {
  case "${!#}" in
    "$STATE_DIR"/*) printf 'restore %s\n' "${!#}" >> "$CALL_LOG" ;;
  esac
  command cp "$@"
}
safe_profile_restore_transaction "$transaction" format-one-control
cmp "$STATE_DIR/agent-id" "$EXPECTED_DIR/agent-id"
closed=$(grep -n -x "chown ${EUID} ${STATE_DIR}" "$CALL_LOG" | cut -d: -f1)
first=$(grep -n '^restore ' "$CALL_LOG" | head -n 1 | cut -d: -f1)
last=$(grep -n '^restore ' "$CALL_LOG" | tail -n 1 | cut -d: -f1)
returned=$(grep -n -x "chown ${LEAST_PRIVILEGE_USER} ${STATE_DIR}" "$CALL_LOG" | cut -d: -f1)
[[ "$closed" =~ ^[0-9]+$ && "$first" =~ ^[0-9]+$ && "$returned" =~ ^[0-9]+$ ]]
[[ $closed -lt $first && $last -lt $returned ]]
`
	if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("format-v1 rollback did not close and return the state root: %v\n%s", err, out)
	}
}
