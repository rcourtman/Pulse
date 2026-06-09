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

echo "PASS: install.sh update-resilience helpers (#1323)"
