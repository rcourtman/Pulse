#!/usr/bin/env bash
#
# Simple harness to execute shell-based smoke tests under scripts/tests/.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TEST_DIR="${ROOT_DIR}/scripts/tests"

usage() {
  cat <<'EOF'
Usage: scripts/tests/run.sh [test-script ...]

Run all scripts/tests/test-*.sh tests or a subset when specified.
EOF
}

discover_tests() {
  local -n ref=$1
  mapfile -t ref < <(find "${TEST_DIR}" -maxdepth 1 -type f -name 'test-*.sh' | sort)
}

resolve_test_path() {
  local input="$1"
  if [[ "${input}" == /* ]]; then
    printf '%s\n' "${input}"
    return 0
  fi

  if [[ -f "${TEST_DIR}/${input}" ]]; then
    printf '%s\n' "${TEST_DIR}/${input}"
    return 0
  fi

  if [[ -f "${input}" ]]; then
    printf '%s\n' "${input}"
    return 0
  fi

  return 1
}

run_tests() {
  local -a tests=("$@")
  local total=0
  local passed=0
  local failed=0

  for test in "${tests[@]}"; do
    ((total += 1))
    local display="${test#${ROOT_DIR}/}"
    printf '==> %s\n' "${display}"
    if (cd "${ROOT_DIR}" && "${test}"); then
      echo "PASS"
      ((passed += 1))
    else
      echo "FAIL"
      ((failed += 1))
    fi
    echo
  done

  echo "Summary: ${passed}/${total} passed"
  if (( failed > 0 )); then
    echo "Failures: ${failed}"
    return 1
  fi
  return 0
}

main() {
  if [[ $# -gt 0 ]]; then
    if [[ "$1" == "-h" || "$1" == "--help" ]]; then
      usage
      exit 0
    fi
  fi

  local -a tests=()
  if [[ $# -gt 0 ]]; then
    local arg resolved
    for arg in "$@"; do
      if ! resolved="$(resolve_test_path "${arg}")"; then
        echo "Unknown test: ${arg}" >&2
        exit 1
      fi
      tests+=("${resolved}")
    done
  else
    discover_tests tests
  fi

  if [[ ${#tests[@]} -eq 0 ]]; then
    echo "No tests found under ${TEST_DIR}" >&2
    exit 1
  fi

  run_tests "${tests[@]}"
}

main "$@"
