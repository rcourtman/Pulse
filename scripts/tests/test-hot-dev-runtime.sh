#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
HOT_DEV_RUNTIME_LIB="${ROOT_DIR}/scripts/lib/hot-dev-runtime.sh"
HOT_DEV="${ROOT_DIR}/scripts/hot-dev.sh"

if [[ ! -f "${HOT_DEV_RUNTIME_LIB}" ]]; then
  echo "hot-dev-runtime.sh not found at ${HOT_DEV_RUNTIME_LIB}" >&2
  exit 1
fi

failures=0
temp_dirs=()

cleanup() {
  local dir
  for dir in "${temp_dirs[@]:-}"; do
    rm -rf "${dir}" 2>/dev/null || true
  done
}
trap cleanup EXIT

assert_contains() {
  local desc="$1"
  local haystack="$2"
  local needle="$3"

  if [[ "${haystack}" == *"${needle}"* ]]; then
    echo "[PASS] ${desc}"
  else
    echo "[FAIL] ${desc}" >&2
    echo "Expected to find: ${needle}" >&2
    ((failures++))
  fi
}

assert_not_contains() {
  local desc="$1"
  local haystack="$2"
  local needle="$3"

  if [[ "${haystack}" == *"${needle}"* ]]; then
    echo "[FAIL] ${desc}" >&2
    echo "Did not expect to find: ${needle}" >&2
    ((failures++))
  else
    echo "[PASS] ${desc}"
  fi
}

make_temp_dir() {
  local dir
  dir="$(mktemp -d)"
  temp_dirs+=("${dir}")
  printf "%s\n" "${dir}"
}

test_pulse_process_count_handles_zero_matches_under_pipefail() {
  local test_dir fake_bin output
  test_dir="$(make_temp_dir)"
  fake_bin="${test_dir}/bin"
  mkdir -p "${fake_bin}"

  cat > "${fake_bin}/pgrep" <<'EOF'
#!/usr/bin/env bash
exit 1
EOF
  chmod +x "${fake_bin}/pgrep"

  output="$(
    PATH="${fake_bin}:$PATH" \
    HOT_DEV_RUNTIME_LIB="${HOT_DEV_RUNTIME_LIB}" \
    bash -c '
      set -euo pipefail
      source "${HOT_DEV_RUNTIME_LIB}"
      printf "count=%s\n" "$(hot_dev_pulse_process_count)"
      printf "survived=yes\n"
    '
  )"

  assert_contains "zero process count returns zero" "${output}" "count=0"
  assert_contains "zero process count does not trip errexit" "${output}" "survived=yes"
}

test_pulse_process_count_counts_matching_processes() {
  local test_dir fake_bin output
  test_dir="$(make_temp_dir)"
  fake_bin="${test_dir}/bin"
  mkdir -p "${fake_bin}"

  cat > "${fake_bin}/pgrep" <<'EOF'
#!/usr/bin/env bash
printf '111\n222\n'
EOF
  chmod +x "${fake_bin}/pgrep"

  output="$(
    PATH="${fake_bin}:$PATH" \
    HOT_DEV_RUNTIME_LIB="${HOT_DEV_RUNTIME_LIB}" \
    bash -c '
      set -euo pipefail
      source "${HOT_DEV_RUNTIME_LIB}"
      printf "count=%s\n" "$(hot_dev_pulse_process_count)"
    '
  )"

  assert_contains "process count counts pgrep output lines" "${output}" "count=2"
}

test_hot_dev_uses_resilient_backend_process_count() {
  local output
  output="$(sed -n '1,760p' "${HOT_DEV}")"

  assert_contains "hot-dev sources runtime helper" "${output}" "source \"\${SCRIPT_DIR}/lib/hot-dev-runtime.sh\""
  assert_contains "health monitor uses resilient process counter" "${output}" "PULSE_COUNT=\"\$(hot_dev_pulse_process_count)\""
  assert_not_contains "hot-dev no longer counts backend processes through a pipefail-sensitive pgrep pipeline" "${output}" "PULSE_COUNT=\$(pgrep -f \"^\\./pulse$\""
}

test_hot_dev_keeps_backend_launch_errors_in_debug_log() {
  local output
  output="$(sed -n '1,760p' "${HOT_DEV}")"

  assert_contains "hot-dev exposes backend debug log override" "${output}" "BACKEND_DEBUG_LOG=\"\${PULSE_BACKEND_LOG_FILE:-/tmp/pulse-debug.log}\""
  assert_contains "backend launch stderr is appended to the debug log" "${output}" ">> \"\${BACKEND_DEBUG_LOG}\" 2>&1"
  assert_contains "backend LOG_FILE follows debug log override" "${output}" "LOG_FILE=\"\${BACKEND_DEBUG_LOG}\""
}

source "${HOT_DEV_RUNTIME_LIB}"
test_pulse_process_count_handles_zero_matches_under_pipefail
test_pulse_process_count_counts_matching_processes
test_hot_dev_uses_resilient_backend_process_count
test_hot_dev_keeps_backend_launch_errors_in_debug_log

if (( failures > 0 )); then
  echo "FAIL: ${failures} hot-dev runtime assertions failed" >&2
  exit 1
fi

echo "PASS: hot-dev runtime contract checks passed"
