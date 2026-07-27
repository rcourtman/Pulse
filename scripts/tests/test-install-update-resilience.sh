#!/usr/bin/env bash
#
# Tests the interactive-update service-resilience helpers in install.sh (#1323):
# after an update that stopped a running Pulse, the installer must verify the
# service came back up, retry one explicit start, and surface a clear error
# instead of silently leaving Pulse stopped (common on unprivileged LXC where
# the installer's restart silently fails).
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
INSTALL_SCRIPT="${ROOT_DIR}/install.sh"

# Sourcing relies on the BASH_SOURCE guard so main() does not run on import.
source "${INSTALL_SCRIPT}"
set +e  # this test drives helpers that intentionally return non-zero

SERVICE_NAME="pulse"

# --- Deterministic stubs: no real sleeps, no real systemctl/timeout ---------
sleep() { :; }
timeout() { shift; "$@"; }  # drop the duration; run the wrapped (stubbed) command

SYSTEMCTL_ACTIVE="no"
START_ATTEMPTS=0
systemctl() {
    case "$1" in
        is-active) [[ "$SYSTEMCTL_ACTIVE" == "yes" ]] && return 0 || return 1 ;;
        stop) SYSTEMCTL_ACTIVE="no"; return 0 ;;
        start) START_ATTEMPTS=$((START_ATTEMPTS + 1)); return 0 ;;
        *) return 0 ;;
    esac
}
safe_systemctl() { systemctl "$@"; }

PRINT_BUF=""
print_info() { PRINT_BUF+="INFO:$*"$'\n'; }
print_warn() { PRINT_BUF+="WARN:$*"$'\n'; }
print_error() { PRINT_BUF+="ERROR:$*"$'\n'; }
print_success() { PRINT_BUF+="OK:$*"$'\n'; }

fail() { echo "FAIL: $*" >&2; exit 1; }

# --- Case 1: stop_pulse_for_update records whether Pulse was running --------
SYSTEMCTL_ACTIVE="yes"; PULSE_WAS_ACTIVE="false"
stop_pulse_for_update
[[ "$PULSE_WAS_ACTIVE" == "true" ]] || fail "should record was-active=true when running"
[[ "$SYSTEMCTL_ACTIVE" == "no" ]] || fail "should stop the service"

SYSTEMCTL_ACTIVE="no"; PULSE_WAS_ACTIVE="true"
stop_pulse_for_update
[[ "$PULSE_WAS_ACTIVE" == "false" ]] || fail "should record was-active=false when not running"

# --- Case 2: no-op when Pulse was not running before the update ------------
PULSE_WAS_ACTIVE="false"; PRINT_BUF=""
ensure_pulse_running_after_update || fail "should no-op (succeed) when was-active=false"
[[ -z "$PRINT_BUF" ]] || fail "should be silent when was-active=false, got: $PRINT_BUF"

# --- Case 3: service comes back up -> success, flag consumed ----------------
PULSE_WAS_ACTIVE="true"; SYSTEMCTL_ACTIVE="yes"; PRINT_BUF=""
ensure_pulse_running_after_update || fail "should succeed when the service is active"
[[ "$PULSE_WAS_ACTIVE" == "false" ]] || fail "should consume the was-active flag"

# --- Case 4: service stays down -> retries once, then a clear error ---------
PULSE_WAS_ACTIVE="true"; SYSTEMCTL_ACTIVE="no"; START_ATTEMPTS=0; PRINT_BUF=""
if ensure_pulse_running_after_update; then
    fail "should return non-zero when the service will not come up"
fi
[[ "$START_ATTEMPTS" -ge 1 ]] || fail "should attempt an explicit restart, got $START_ATTEMPTS"
[[ "$PRINT_BUF" == *"did not come back up"* ]] || fail "should surface a clear error, got: $PRINT_BUF"

# --- Cases 5+: read-only-filesystem resilience (#1630) -----------------------
# The unattended updater runs the installer under pulse-update.service, whose
# ProtectSystem=strict leaves /bin and /usr/local/bin read-only; transient
# read-only remounts hit the same paths. Writes outside the install dir must
# be non-fatal: aborting used to kill the installer with errexit active after
# the new binary was installed and the service stopped.
TMPDIR_RO="$(mktemp -d)"
# A regular file as the "parent directory" makes any write beneath it fail
# with ENOTDIR — unlike chmod 555, this also fails when running as root.
touch "$TMPDIR_RO/blocker"

# --- Case 5: setup_update_command must not abort the installer (errexit) ----
out=$(
    set -e
    PRINT_BUF=""
    UPDATE_HELPER_PATH="$TMPDIR_RO/blocker/update"
    PULSE_PROFILE_PATH="$TMPDIR_RO/profile"
    PULSE_BASHRC_PATH="$TMPDIR_RO/bashrc"
    setup_update_command
    printf '%s' "$PRINT_BUF"
)
rc=$?
[[ "$rc" -eq 0 ]] || fail "setup_update_command must not abort the installer when the helper path is unwritable (rc=$rc)"
[[ "$out" == *"WARN:"*"$TMPDIR_RO/blocker/update"* ]] || fail "should warn about the unwritable helper path, got: $out"

# --- Case 6: setup_update_command still writes the helper when it can -------
out=$(
    set -e
    PRINT_BUF=""
    UPDATE_HELPER_PATH="$TMPDIR_RO/bin/update"
    PULSE_PROFILE_PATH="$TMPDIR_RO/profile"
    PULSE_BASHRC_PATH="$TMPDIR_RO/bashrc"
    setup_update_command
    printf '%s' "$PRINT_BUF"
)
rc=$?
[[ "$rc" -eq 0 ]] || fail "setup_update_command should succeed on a writable path (rc=$rc)"
[[ -x "$TMPDIR_RO/bin/update" ]] || fail "should create an executable update helper at a writable path"
grep -q "Pulse update command" "$TMPDIR_RO/bin/update" || fail "helper should contain the expected script body"

# --- Case 7: install_binary_symlink is non-fatal on an unwritable path ------
PRINT_BUF=""
install_binary_symlink "$TMPDIR_RO/bin/update" "$TMPDIR_RO/blocker/pulse" || \
    fail "install_binary_symlink must not fail on an unwritable link path"
[[ "$PRINT_BUF" == *"WARN:"* ]] || fail "should warn when the symlink cannot be created, got: $PRINT_BUF"

# --- Case 8: install_binary_symlink creates and keeps the link ---------------
PRINT_BUF=""
install_binary_symlink "$TMPDIR_RO/bin/update" "$TMPDIR_RO/bin/pulse" || \
    fail "install_binary_symlink should succeed on a writable path"
[[ "$(readlink "$TMPDIR_RO/bin/pulse")" == "$TMPDIR_RO/bin/update" ]] || \
    fail "symlink should point at the installed binary"
# Idempotence: with the correct link already present it must not need ln at
# all (matches an update run where the link survives but the fs is read-only).
ln() { return 1; }
PRINT_BUF=""
install_binary_symlink "$TMPDIR_RO/bin/update" "$TMPDIR_RO/bin/pulse" || \
    fail "install_binary_symlink should succeed when the correct link already exists"
[[ "$PRINT_BUF" == *"already in place"* ]] || \
    fail "should recognise an existing correct link, got: $PRINT_BUF"
unset -f ln

rm -rf "$TMPDIR_RO"

echo "PASS: install.sh update-resilience helpers (#1323, #1630)"
