#!/usr/bin/env bash
#
# Smoke tests for scripts/pulse-auto-update.sh helper behavior (#1323).

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
AUTO_UPDATE_SCRIPT="${ROOT_DIR}/scripts/pulse-auto-update.sh"

if [[ ! -f "${AUTO_UPDATE_SCRIPT}" ]]; then
  echo "pulse-auto-update.sh not found at ${AUTO_UPDATE_SCRIPT}" >&2
  exit 1
fi

# Sourcing relies on the BASH_SOURCE guard so main() does not run on import.
# shellcheck disable=SC1090
source "${AUTO_UPDATE_SCRIPT}"

failures=0

assert_success() {
  local desc="$1"
  shift
  if "$@"; then
    echo "[PASS] ${desc}"
    return 0
  else
    echo "[FAIL] ${desc}" >&2
    ((failures++))
    return 1
  fi
}

test_wait_for_service_active_succeeds_after_retry() {
  local calls=0

  systemctl() {
    if [[ "$1" == "is-active" ]]; then
      ((calls += 1))
      if (( calls >= 3 )); then
        return 0
      fi
      return 1
    fi
    return 1
  }

  sleep() { :; }

  wait_for_service_active pulse 5
}

test_wait_for_service_active_times_out_when_never_active() {
  systemctl() { return 1; }
  sleep() { :; }

  if wait_for_service_active pulse 3; then
    echo "expected wait_for_service_active to fail when service never becomes active" >&2
    return 1
  fi
  return 0
}

test_perform_update_restores_backup_when_service_stays_down() {
  local tmpdir
  tmpdir="$(mktemp -d)"
  local status=0
  # perform_update installs a RETURN trap referencing these; declare them here
  # so the trap is safe under set -u if it surfaces in this calling scope.
  local installer_tmp="" signature_tmp=""

  INSTALL_DIR="${tmpdir}/opt/pulse"
  CONFIG_DIR="${tmpdir}/etc/pulse"
  mkdir -p "${INSTALL_DIR}/bin" "${CONFIG_DIR}"

  printf 'v5.1.24\n' > "${INSTALL_DIR}/VERSION"
  cat > "${INSTALL_DIR}/bin/pulse" <<'EOF'
#!/usr/bin/env bash
echo "v5.1.24"
EOF
  chmod +x "${INSTALL_DIR}/bin/pulse"

  export INSTALL_DIR
  export FAKE_NEW_VERSION="v5.1.25"

  # Stub out the v6 download/verify pipeline so perform_update reaches the
  # post-install service check deterministically.
  is_prerelease_tag() { return 1; }
  detect_service_name() { echo "pulse"; }
  resolve_install_script_url() { echo "http://localhost/install.sh"; }
  verify_release_signature() { return 0; }
  get_current_version() { tr -d '\r\n' < "${INSTALL_DIR}/VERSION"; }

  # curl writes the installer / signature to the -o target. The fake installer
  # bumps VERSION to the new version, simulating a successful install.
  curl() {
    local out="" prev=""
    local arg
    for arg in "$@"; do
      if [[ "$prev" == "-o" ]]; then out="$arg"; fi
      prev="$arg"
    done
    if [[ -n "$out" ]]; then
      case "$out" in
        *.sig.*) printf 'dummy-signature\n' > "$out" ;;
        *)
          cat > "$out" <<'INSTALLER'
#!/usr/bin/env bash
printf '%s\n' "${FAKE_NEW_VERSION}" > "${PULSE_INSTALL_DIR}/VERSION"
exit 0
INSTALLER
          ;;
      esac
    fi
    return 0
  }

  # Service was running before the update (first is-active call true), then
  # never comes back up; start also fails -> perform_update must restore + fail.
  local is_active_calls=0
  systemctl() {
    if [[ "$1" == "is-active" ]]; then
      ((is_active_calls += 1))
      if (( is_active_calls == 1 )); then
        return 0
      fi
      return 1
    fi
    return 1
  }

  sleep() { :; }

  if perform_update "v5.1.25"; then
    echo "perform_update unexpectedly succeeded while service stayed down" >&2
    status=1
  fi
  # perform_update installs a RETURN trap; clear it so it does not leak into
  # subsequent function returns in this sourced test harness.
  trap - RETURN 2>/dev/null || true

  if [[ "${status}" -eq 0 ]] && [[ "$(tr -d '\r\n' < "${INSTALL_DIR}/VERSION")" != "v5.1.24" ]]; then
    echo "expected VERSION to be restored to v5.1.24 after failed restart" >&2
    status=1
  fi

  rm -rf "${tmpdir}"
  return "${status}"
}

main() {
  assert_success "wait_for_service_active retries until active" test_wait_for_service_active_succeeds_after_retry
  assert_success "wait_for_service_active times out when never active" test_wait_for_service_active_times_out_when_never_active
  assert_success "perform_update restores backup when service stays down" test_perform_update_restores_backup_when_service_stays_down

  if (( failures > 0 )); then
    echo "Total failures: ${failures}" >&2
    return 1
  fi

  echo "All pulse-auto-update smoke tests passed."
}

main "$@"
