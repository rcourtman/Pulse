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
  assert_contains "hot-dev defaults backend log level to info" "${output}" "LOG_LEVEL=\"\${LOG_LEVEL:-info}\""
  assert_contains "hot-dev disables background AI automation by default" "${output}" "PULSE_DEV_DISABLE_BACKGROUND_AI=\"\${PULSE_DEV_DISABLE_BACKGROUND_AI:-true}\""
  assert_contains "backend launch stderr is appended to the debug log" "${output}" ">> \"\${BACKEND_DEBUG_LOG}\" 2>&1"
  assert_contains "backend LOG_FILE follows debug log override" "${output}" "LOG_FILE=\"\${BACKEND_DEBUG_LOG}\""
}

test_hot_dev_preserves_proxmox_guest_docker_env() {
  local output
  output="$(sed -n '1,760p' "${HOT_DEV}")"

  assert_contains "hot-dev exports Proxmox guest Docker detection opt-in" "${output}" "export PULSE_ENABLE_PROXMOX_GUEST_DOCKER_DETECTION PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY PULSE_PROXMOX_GUEST_DOCKER_INVENTORY_VMIDS"
  assert_contains "backend launch preserves Proxmox guest Docker detection opt-in" "${output}" 'PULSE_ENABLE_PROXMOX_GUEST_DOCKER_DETECTION="${PULSE_ENABLE_PROXMOX_GUEST_DOCKER_DETECTION:-}"'
  assert_contains "backend launch preserves Proxmox guest Docker inventory opt-in" "${output}" 'PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY="${PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY:-}"'
  assert_contains "backend launch preserves scoped Proxmox guest Docker VMIDs" "${output}" 'PULSE_PROXMOX_GUEST_DOCKER_INVENTORY_VMIDS="${PULSE_PROXMOX_GUEST_DOCKER_INVENTORY_VMIDS:-}"'
}

test_hot_dev_avoids_self_killing_npm_wrapper() {
  local output
  output="$(sed -n '1,330p' "${HOT_DEV}")"

  assert_contains "hot-dev routes npm cleanup through a guarded helper" "${output}" "kill_stale_npm_dev_wrappers"
  assert_contains "hot-dev supports managed skip for npm cleanup" "${output}" "HOT_DEV_SKIP_NPM_CLEANUP"
  assert_not_contains "hot-dev no longer broad-kills npm dev wrappers" "${output}" 'pkill -f "npm run dev"'
}

test_hot_dev_network_defaults_are_local_first_with_explicit_lan_opt_in() {
  local output
  output="$(
    HOT_DEV_RUNTIME_LIB="${HOT_DEV_RUNTIME_LIB}" \
    bash -c '
      set -euo pipefail
      source "${HOT_DEV_RUNTIME_LIB}"
      unset FRONTEND_PORT PORT FRONTEND_DEV_HOST FRONTEND_DEV_PORT BIND_ADDRESS PULSE_DEV_LAN
      unset PULSE_DEV_API_HOST PULSE_DEV_API_PORT PULSE_DEV_API_URL PULSE_DEV_WS_URL
      unset ALLOWED_ORIGINS
      LAN_IP=192.168.50.10
      ALL_IPS="192.168.50.10 10.10.10.5"
      hot_dev_configure_network_defaults
      printf "default_frontend_host=%s\n" "${FRONTEND_DEV_HOST}"
      printf "default_frontend_port=%s\n" "${FRONTEND_DEV_PORT}"
      printf "default_api_host=%s\n" "${PULSE_DEV_API_HOST}"
      printf "default_api_url=%s\n" "${PULSE_DEV_API_URL}"
      printf "default_ws_url=%s\n" "${PULSE_DEV_WS_URL}"
      printf "default_bind=%s\n" "${BIND_ADDRESS}"
      printf "default_origins=%s\n" "${ALLOWED_ORIGINS}"

      unset FRONTEND_PORT PORT FRONTEND_DEV_HOST FRONTEND_DEV_PORT BIND_ADDRESS
      unset PULSE_DEV_API_HOST PULSE_DEV_API_PORT PULSE_DEV_API_URL PULSE_DEV_WS_URL
      unset ALLOWED_ORIGINS
      PULSE_DEV_LAN=true
      hot_dev_configure_network_defaults
      printf "lan_frontend_host=%s\n" "${FRONTEND_DEV_HOST}"
      printf "lan_api_host=%s\n" "${PULSE_DEV_API_HOST}"
      printf "lan_api_url=%s\n" "${PULSE_DEV_API_URL}"
      printf "lan_ws_url=%s\n" "${PULSE_DEV_WS_URL}"
      printf "lan_bind=%s\n" "${BIND_ADDRESS}"
      printf "lan_origins=%s\n" "${ALLOWED_ORIGINS}"
    '
  )"

  assert_contains "network defaults keep hot-dev frontend loopback-only" "${output}" "default_frontend_host=127.0.0.1"
  assert_contains "network defaults keep the canonical frontend dev port" "${output}" "default_frontend_port=5173"
  assert_contains "network defaults target loopback API host" "${output}" "default_api_host=127.0.0.1"
  assert_contains "network defaults derive the loopback API URL" "${output}" "default_api_url=http://127.0.0.1:7655"
  assert_contains "network defaults derive the loopback websocket URL" "${output}" "default_ws_url=ws://127.0.0.1:7655"
  assert_contains "network defaults bind backend to loopback" "${output}" "default_bind=127.0.0.1"
  assert_contains "network defaults retain loopback frontend origins" "${output}" "http://127.0.0.1:5173"
  assert_not_contains "network defaults do not allow detected LAN frontend origins by default" "${output}" "default_origins=http://192.168.50.10"
  assert_not_contains "network defaults do not allow wildcard dev origin by default" "${output}" "default_origins=http://0.0.0.0:5173"
  assert_contains "LAN opt-in exposes the frontend on all interfaces" "${output}" "lan_frontend_host=0.0.0.0"
  assert_contains "LAN opt-in targets the detected LAN API host" "${output}" "lan_api_host=192.168.50.10"
  assert_contains "LAN opt-in derives the API URL" "${output}" "lan_api_url=http://192.168.50.10:7655"
  assert_contains "LAN opt-in derives the websocket URL" "${output}" "lan_ws_url=ws://192.168.50.10:7655"
  assert_contains "LAN opt-in binds backend to all interfaces" "${output}" "lan_bind=0.0.0.0"
  assert_contains "LAN opt-in allows the detected LAN frontend origin" "${output}" "lan_origins=http://192.168.50.10"
  assert_contains "LAN opt-in allows the wildcard dev origin Electron may report" "${output}" "http://0.0.0.0:5173"
}

test_hot_dev_browser_urls_distinguish_bind_and_browser_hosts() {
  local output
  output="$(
    HOT_DEV_RUNTIME_LIB="${HOT_DEV_RUNTIME_LIB}" \
    bash -c '
      set -euo pipefail
      source "${HOT_DEV_RUNTIME_LIB}"
      LAN_IP=192.168.50.10
      printf "local=%s\n" "$(hot_dev_local_browser_url 0.0.0.0 5173)"
      printf "lan=%s\n" "$(hot_dev_lan_browser_url 0.0.0.0 5173 "${LAN_IP}")"
      printf "loopback_lan=%s\n" "$(hot_dev_lan_browser_url 127.0.0.1 5173 "${LAN_IP}")"
    '
  )"

  assert_contains "local browser URL maps wildcard binds to loopback" "${output}" "local=http://127.0.0.1:5173"
  assert_contains "LAN browser URL uses detected LAN IP for wildcard binds" "${output}" "lan=http://192.168.50.10:5173"
  assert_contains "loopback binds do not advertise a LAN browser URL" "${output}" "loopback_lan="
}

source "${HOT_DEV_RUNTIME_LIB}"
test_pulse_process_count_handles_zero_matches_under_pipefail
test_pulse_process_count_counts_matching_processes
test_hot_dev_uses_resilient_backend_process_count
test_hot_dev_keeps_backend_launch_errors_in_debug_log
test_hot_dev_preserves_proxmox_guest_docker_env
test_hot_dev_avoids_self_killing_npm_wrapper
test_hot_dev_network_defaults_are_local_first_with_explicit_lan_opt_in
test_hot_dev_browser_urls_distinguish_bind_and_browser_hosts

if (( failures > 0 )); then
  echo "FAIL: ${failures} hot-dev runtime assertions failed" >&2
  exit 1
fi

echo "PASS: hot-dev runtime contract checks passed"
