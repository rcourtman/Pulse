#!/usr/bin/env bash
#
# Simple harness to execute smoke tests under scripts/tests/.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TEST_DIR="${ROOT_DIR}/scripts/tests"

usage() {
  cat <<'EOF'
Usage: scripts/tests/run.sh [test-script ...]
       scripts/tests/run.sh --docs-shard <0|1|2|3>

Run all scripts/tests/test-*.sh and scripts/tests/test_*.py tests or a subset
when specified. Documentation CI discovers every documentation suite and runs
one of four disjoint, sorted shards; it does not maintain a second suite list.
EOF
}

discover_tests() {
  local -n ref=$1
  local mode="${2:-all}"
  local inventory
  local -a patterns=(-name 'test-*.sh' -o -name 'test_*.py')
  if [[ "${mode}" == docs ]]; then
    # Keep the established safety helpers whose names predate the docs suffix.
    patterns=(-name 'test_*docs*.py' -o -name 'test_vm_disk_diagnostics.py'
      -o -name 'test_troubleshooting_logs.py'
      -o -name 'test-retired-trial-acquisition-docs.sh')
  fi
  # Process substitution hides producer failures from mapfile. Admit the
  # complete discovery pipeline before running any tests, never a partial list.
  if ! inventory="$(
    find "${TEST_DIR}" -maxdepth 1 -type f \
      \( "${patterns[@]}" \) | LC_ALL=C sort
  )"; then
    echo "Failed to discover the complete smoke test inventory under ${TEST_DIR}" >&2
    return 1
  fi
  ref=()
  if [[ -n "${inventory}" ]]; then
    mapfile -t ref <<< "${inventory}"
  fi
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
    if [[ "${test}" == *.py ]]; then
      if (cd "${ROOT_DIR}" && python3 "${test}"); then
        echo "PASS"
        ((passed += 1))
      else
        echo "FAIL"
        ((failed += 1))
      fi
    elif (cd "${ROOT_DIR}" && "${test}"); then
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
  if [[ "${1:-}" == --docs-shard ]]; then
    if [[ $# -ne 2 || ! "${2:-}" =~ ^[0-3]$ ]]; then
      echo "Usage: scripts/tests/run.sh --docs-shard <0|1|2|3>" >&2
      return 1
    fi
    local -a doc_tests=()
    if ! discover_tests doc_tests docs; then
      return 1
    fi
    local index
    for ((index = $2; index < ${#doc_tests[@]}; index += 4)); do
      tests+=("${doc_tests[index]}")
    done
    printf 'Documentation shard %s/4: %s of %s suites\n' "$2" "${#tests[@]}" "${#doc_tests[@]}"
  elif [[ $# -gt 0 ]]; then
    local arg resolved
    for arg in "$@"; do
      if ! resolved="$(resolve_test_path "${arg}")"; then
        echo "Unknown test: ${arg}" >&2
        exit 1
      fi
      tests+=("${resolved}")
    done
  else
    if ! discover_tests tests; then
      return 1
    fi
  fi

  if [[ ${#tests[@]} -eq 0 ]]; then
    echo "No tests found under ${TEST_DIR}" >&2
    exit 1
  fi

  run_tests "${tests[@]}"
}

main "$@"
