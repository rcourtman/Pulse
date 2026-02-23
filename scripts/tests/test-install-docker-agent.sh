#!/usr/bin/env bash
#
# Smoke tests for scripts/install-docker-agent.sh (deprecated wrapper).

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT_PATH="${ROOT_DIR}/scripts/install-docker-agent.sh"

if [[ ! -f "${SCRIPT_PATH}" ]]; then
  echo "Missing script at ${SCRIPT_PATH}" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/pulse-test-install-docker-agent-XXXXXX")"
trap 'rm -rf "${TMP_DIR}"' EXIT

pass_count=0
fail_count=0

log_pass() {
  echo "[PASS] $1"
  ((pass_count+=1))
  return 0
}

log_fail() {
  echo "[FAIL] $1" >&2
  ((fail_count+=1))
  return 1
}

assert_fails_with() {
  local desc="$1"
  local expected="$2"
  shift 2

  local output status
  set +e
  output="$("$@" 2>&1)"
  status=$?
  set -e

  if (( status == 0 )); then
    log_fail "${desc} (expected non-zero exit)"
    echo "${output}" >&2
    return 1
  fi

  if [[ "${output}" == *"${expected}"* ]]; then
    log_pass "${desc}"
  else
    log_fail "${desc} (missing expected output: ${expected})"
    echo "${output}" >&2
    return 1
  fi
}

test_syntax() {
  if bash -n "${SCRIPT_PATH}"; then
    log_pass "syntax check"
  else
    log_fail "syntax check"
  fi
}

test_missing_url() {
  assert_fails_with "missing url rejected" "--url is required" bash "${SCRIPT_PATH}"
}

test_missing_token() {
  assert_fails_with "missing token rejected" "--token is required" bash "${SCRIPT_PATH}" --url "http://example.local:7655"
}

test_delegates_to_unified_installer() {
  local stub_dir="${TMP_DIR}/stub-bin"
  local called_file="${TMP_DIR}/delegated-args.txt"
  mkdir -p "${stub_dir}"

  cat > "${stub_dir}/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
cat <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" > "${PULSE_DELEGATED_ARGS_FILE:?}"
SCRIPT
EOF
  chmod +x "${stub_dir}/curl"

  local status
  set +e
  PATH="${stub_dir}:$PATH" \
  PULSE_DELEGATED_ARGS_FILE="${called_file}" \
  bash "${SCRIPT_PATH}" --url "http://example.local:7655" --token "tok_test_123" >/dev/null 2>&1
  status=$?
  set -e

  if (( status != 0 )); then
    log_fail "delegates to unified installer (exit ${status})"
    return 1
  fi

  if [[ ! -f "${called_file}" ]]; then
    log_fail "delegates to unified installer (missing delegated args file)"
    return 1
  fi

  if grep -Fx -- "--url" "${called_file}" >/dev/null \
    && grep -Fx -- "http://example.local:7655" "${called_file}" >/dev/null \
    && grep -Fx -- "--token" "${called_file}" >/dev/null \
    && grep -Fx -- "tok_test_123" "${called_file}" >/dev/null \
    && grep -Fx -- "--enable-docker" "${called_file}" >/dev/null; then
    log_pass "delegates to unified installer"
  else
    log_fail "delegates to unified installer (args mismatch)"
    cat "${called_file}" >&2
  fi
}

main() {
  test_syntax
  test_missing_url
  test_missing_token
  test_delegates_to_unified_installer

  if (( fail_count > 0 )); then
    echo "test-install-docker-agent: ${fail_count} failure(s)" >&2
    exit 1
  fi

  echo "All install-docker-agent smoke tests passed (${pass_count})"
}

main "$@"
