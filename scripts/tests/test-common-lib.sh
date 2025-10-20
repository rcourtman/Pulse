#!/usr/bin/env bash
#
# Smoke test for scripts/lib/common.sh functionality.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMMON_LIB="${ROOT_DIR}/scripts/lib/common.sh"

if [[ ! -f "${COMMON_LIB}" ]]; then
  echo "common.sh not found at ${COMMON_LIB}" >&2
  exit 1
fi

# shellcheck disable=SC1090
source "${COMMON_LIB}"

export PULSE_NO_COLOR=1
common::init "$0"

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

test_functions_exist() {
  local missing=0
  local fn
  for fn in \
    common::init \
    common::log_info \
    common::log_warn \
    common::log_error \
    common::log_debug \
    common::fail \
    common::require_command \
    common::is_interactive \
    common::ensure_root \
    common::sudo_exec \
    common::run \
    common::run_capture \
    common::temp_dir \
    common::cleanup_push \
    common::cleanup_run \
    common::set_dry_run \
    common::is_dry_run; do
    if ! declare -F -- "${fn}" >/dev/null 2>&1; then
      echo "Missing function definition: ${fn}" >&2
      missing=1
    fi
  done
  [[ "${missing}" -eq 0 ]]
}

test_logging() {
  local info_output debug_output warn_output
  info_output="$(common::log_info "common-log-info-test")"
  [[ "${info_output}" == *"common-log-info-test"* ]] || return 1

  debug_output="$(common::log_debug "common-log-debug-test" 2>&1)"
  [[ -z "${debug_output}" ]] || return 1

  warn_output="$(common::log_warn "common-log-warn-test" 2>&1)"
  [[ "${warn_output}" == *"common-log-warn-test"* ]] || return 1
}

test_dry_run() {
  local tmpfile
  tmpfile="$(mktemp)"
  rm -f "${tmpfile}"
  common::set_dry_run true
  common::run --label "dry-run-touch" touch "${tmpfile}"
  common::set_dry_run false
  [[ ! -e "${tmpfile}" ]]
}

test_interactive_detection() {
  # Ensure the function returns success when forced.
  if PULSE_FORCE_INTERACTIVE=1 common::is_interactive; then
    return 0
  fi
  return 1
}

test_temp_dir_cleanup() {
  local tmp_dir=""
  common::temp_dir tmp_dir --prefix pulse-test-common-
  [[ -d "${tmp_dir}" ]] || return 1
  common::cleanup_run
  [[ ! -d "${tmp_dir}" ]]
}

main() {
  assert_success "functions exist" test_functions_exist
  assert_success "logging output" test_logging
  assert_success "dry-run support" test_dry_run
  assert_success "interactive detection" test_interactive_detection
  assert_success "temp dir cleanup" test_temp_dir_cleanup

  if (( failures > 0 )); then
    echo "Total failures: ${failures}" >&2
    return 1
  fi

  echo "All common.sh smoke tests passed."
}

main "$@"
