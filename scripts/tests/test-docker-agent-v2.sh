#!/usr/bin/env bash
# Smoke tests for install-docker-agent-v2.sh

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT_PATH="${ROOT_DIR}/scripts/install-docker-agent-v2.sh"
TEST_NAME="install-docker-agent-v2"

if [[ ! -f "${SCRIPT_PATH}" ]]; then
  echo "Missing script at ${SCRIPT_PATH}" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/pulse-test-XXXXXX")"
trap 'rm -rf "${TMP_DIR}"' EXIT

pass_count=0
fail_count=0

log_pass() {
  echo "[PASS] $1"
  ((pass_count++))
  return 0
}

log_fail() {
  echo "[FAIL] $1" >&2
  ((fail_count++))
  return 1
}

run_test() {
  local desc="$1"
  shift
  if "$@"; then
    log_pass "$desc"
  else
    local status=$?
    log_fail "$desc (exit ${status})"
  fi
}

set +e
run_test "syntax check" bash -n "${SCRIPT_PATH}"
set -e

# Test dry-run output captures action hints
set +e
DRY_RUN_OUTPUT="$("${SCRIPT_PATH}" --dry-run --url http://test.local --token testtoken 2>&1)"
DRY_STATUS=$?
set -e
if (( DRY_STATUS == 0 )) && [[ "${DRY_RUN_OUTPUT}" == *"[dry-run]"* ]]; then
  log_pass "dry-run outputs actions"
else
  log_fail "dry-run outputs actions"
  echo "${DRY_RUN_OUTPUT}"
fi

if (( DRY_STATUS == 0 )); then
  log_pass "dry-run exits successfully"
else
  log_fail "dry-run exits successfully (exit ${DRY_STATUS})"
  echo "${DRY_RUN_OUTPUT}"
fi

# Test argument validation (missing token)
MISSING_TOKEN_LOG="${TMP_DIR}/missing-token.log"
set +e
"${SCRIPT_PATH}" --dry-run --url http://test.local >"${MISSING_TOKEN_LOG}" 2>&1
ARG_STATUS=$?
set -e
if (( ARG_STATUS == 0 )); then
  log_fail "missing token rejected"
  cat "${MISSING_TOKEN_LOG}"
else
  log_pass "missing token rejected"
fi

if (( fail_count > 0 )); then
  echo "${TEST_NAME}: ${fail_count} failures" >&2
  exit 1
fi

echo "All ${TEST_NAME} tests passed (${pass_count})"
