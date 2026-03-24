#!/usr/bin/env bash
#
# Smoke test for scripts/hot-dev-bg.sh managed ownership diagnostics.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
HOT_DEV_BG="${ROOT_DIR}/scripts/hot-dev-bg.sh"
PACKAGE_JSON="${ROOT_DIR}/package.json"
FRONTEND_PACKAGE_JSON="${ROOT_DIR}/frontend-modern/package.json"

if [[ ! -x "${HOT_DEV_BG}" ]]; then
  echo "hot-dev-bg.sh not found or not executable at ${HOT_DEV_BG}" >&2
  exit 1
fi

failures=0
server_pids=()
temp_dirs=()

cleanup() {
  local pid
  for pid in "${server_pids[@]:-}"; do
    kill "${pid}" 2>/dev/null || true
  done
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

pick_free_port() {
  python3 - <<'PY'
import socket

sock = socket.socket()
sock.bind(("127.0.0.1", 0))
print(sock.getsockname()[1])
sock.close()
PY
}

start_http_server() {
  local port="$1"
  python3 -m http.server "${port}" --bind 127.0.0.1 >/dev/null 2>&1 &
  local pid=$!
  server_pids+=("${pid}")
  sleep 1
}

make_isolated_hot_dev_bg_state() {
  local dir
  dir="$(mktemp -d)"
  temp_dirs+=("${dir}")
  printf "%s\n" "${dir}"
}

test_status_without_runtime() {
  local frontend_port backend_port output
  local state_dir
  frontend_port="$(pick_free_port)"
  backend_port="$(pick_free_port)"
  if [[ "${backend_port}" == "${frontend_port}" ]]; then
    backend_port="$(pick_free_port)"
  fi
  state_dir="$(make_isolated_hot_dev_bg_state)"

  output="$(
    HOT_DEV_BG_PID_FILE="${state_dir}/hot-dev-bg.pid" \
    HOT_DEV_BG_LOG_FILE="${state_dir}/hot-dev-bg.log" \
    FRONTEND_DEV_HOST=127.0.0.1 \
    FRONTEND_DEV_PORT="${frontend_port}" \
    PULSE_DEV_API_HOST=127.0.0.1 \
    PULSE_DEV_API_PORT="${backend_port}" \
    "${HOT_DEV_BG}" status
  )"

  assert_contains "status reports no managed runtime" "${output}" "[hot-dev-bg] Not running"
  assert_contains "status reports the browser entrypoint" "${output}" "[hot-dev-bg] Browser entrypoint: http://127.0.0.1:${frontend_port}"
  assert_contains "status reports idle frontend port" "${output}" "[hot-dev-bg] Frontend port ${frontend_port} is not listening"
  assert_contains "status reports idle backend port" "${output}" "[hot-dev-bg] Backend port ${backend_port} is not listening"
  assert_contains "status reports proxy health probe" "${output}" "[hot-dev-bg] Frontend proxy /api/health: 000"
}

test_cli_parses_takeover_flag() {
  local output
  output="$(
    HOT_DEV_BG_PATH="${HOT_DEV_BG}" \
    bash -lc '
      source "${HOT_DEV_BG_PATH}"
      printf "start=%s\n" "$(parse_takeover_flag start --takeover)"
      printf "restart=%s\n" "$(parse_takeover_flag restart --takeover)"
      printf "verify=%s\n" "$(parse_takeover_flag verify --takeover)"
      printf "backend_restart=%s\n" "$(parse_takeover_flag backend-restart)"
      printf "plain=%s\n" "$(parse_takeover_flag start)"
      if parse_takeover_flag status --takeover >/tmp/hot-dev-bg.invalid 2>&1; then
        printf "invalid=accepted\n"
      else
        printf "invalid=rejected\n"
      fi
    '
  )"

  assert_contains "takeover parsing enables start" "${output}" "start=true"
  assert_contains "takeover parsing enables restart" "${output}" "restart=true"
  assert_contains "takeover parsing enables verify" "${output}" "verify=true"
  assert_contains "backend restart remains flagless" "${output}" "backend_restart=false"
  assert_contains "start without flag stays false" "${output}" "plain=false"
  assert_contains "unexpected status flag is rejected" "${output}" "invalid=rejected"
}

test_verify_command_injects_managed_runtime_env() {
  local output
  output="$(
    HOT_DEV_BG_PATH="${HOT_DEV_BG}" \
    bash -lc '
      source "${HOT_DEV_BG_PATH}"
      HOT_DEV_BG_VERIFY_COMMAND="python3 - <<'\''PY'\''
import os
print(f\"mode={os.environ.get('\''PULSE_E2E_USE_HOT_DEV'\'')}\")
print(f\"username={os.environ.get('\''PULSE_E2E_USERNAME'\'')}\")
print(f\"password={os.environ.get('\''PULSE_E2E_PASSWORD'\'')}\")
PY"
      run_verify_proof_command
    '
  )"

  assert_contains "verify proof forces managed hot-dev mode" "${output}" "mode=1"
  assert_contains "verify proof defaults username" "${output}" "username=admin"
  assert_contains "verify proof defaults password" "${output}" "password=admin"
}

test_root_package_exposes_managed_runtime_entrypoints() {
  local output
  output="$(
    PACKAGE_JSON_PATH="${PACKAGE_JSON}" \
    python3 - <<'PY'
import json
import os

with open(os.environ["PACKAGE_JSON_PATH"], "r", encoding="utf-8") as fh:
    scripts = json.load(fh)["scripts"]

for key in [
    "dev",
    "dev:status",
    "dev:logs",
    "dev:stop",
    "dev:restart",
    "dev:backend-restart",
    "dev:verify",
    "dev:foreground",
]:
    print(f"{key}={scripts.get(key, '')}")
PY
  )"

  assert_contains "root package exposes managed dev start" "${output}" "dev=./scripts/hot-dev-bg.sh start --takeover"
  assert_contains "root package exposes managed dev status" "${output}" "dev:status=./scripts/hot-dev-bg.sh status"
  assert_contains "root package exposes managed dev logs" "${output}" "dev:logs=./scripts/hot-dev-bg.sh logs"
  assert_contains "root package exposes managed dev stop" "${output}" "dev:stop=./scripts/hot-dev-bg.sh stop"
  assert_contains "root package exposes managed dev restart" "${output}" "dev:restart=./scripts/hot-dev-bg.sh restart --takeover"
  assert_contains "root package exposes managed backend restart" "${output}" "dev:backend-restart=./scripts/hot-dev-bg.sh backend-restart"
  assert_contains "root package exposes managed dev verify" "${output}" "dev:verify=./scripts/hot-dev-bg.sh verify --takeover"
  assert_contains "root package keeps foreground escape hatch" "${output}" "dev:foreground=./scripts/hot-dev.sh"
}

test_frontend_package_exposes_managed_runtime_entrypoints() {
  local output
  output="$(
    PACKAGE_JSON_PATH="${FRONTEND_PACKAGE_JSON}" \
    python3 - <<'PY'
import json
import os

with open(os.environ["PACKAGE_JSON_PATH"], "r", encoding="utf-8") as fh:
    scripts = json.load(fh)["scripts"]

for key in [
    "dev",
    "dev:logs",
    "dev:restart",
    "dev:backend-restart",
    "dev:status",
    "dev:stop",
    "dev:verify",
    "dev:foreground",
    "dev:frontend-only",
]:
    print(f"{key}={scripts.get(key, '')}")
PY
  )"

  assert_contains "frontend package exposes managed dev start" "${output}" "dev=../scripts/hot-dev-bg.sh start --takeover"
  assert_contains "frontend package exposes managed dev logs" "${output}" "dev:logs=../scripts/hot-dev-bg.sh logs"
  assert_contains "frontend package exposes managed dev restart" "${output}" "dev:restart=../scripts/hot-dev-bg.sh restart --takeover"
  assert_contains "frontend package exposes managed backend restart" "${output}" "dev:backend-restart=../scripts/hot-dev-bg.sh backend-restart"
  assert_contains "frontend package exposes managed dev status" "${output}" "dev:status=../scripts/hot-dev-bg.sh status"
  assert_contains "frontend package exposes managed dev stop" "${output}" "dev:stop=../scripts/hot-dev-bg.sh stop"
  assert_contains "frontend package exposes managed dev verify" "${output}" "dev:verify=../scripts/hot-dev-bg.sh verify --takeover"
  assert_contains "frontend package keeps foreground managed launcher" "${output}" "dev:foreground=../scripts/hot-dev.sh"
  assert_contains "frontend package keeps explicit frontend-only escape hatch" "${output}" "dev:frontend-only=vite"
}

test_backend_restart_requires_managed_runtime() {
  local frontend_port backend_port output
  local state_dir
  frontend_port="$(pick_free_port)"
  backend_port="$(pick_free_port)"
  if [[ "${backend_port}" == "${frontend_port}" ]]; then
    backend_port="$(pick_free_port)"
  fi
  state_dir="$(make_isolated_hot_dev_bg_state)"

  output="$(
    set +e
    HOT_DEV_BG_PID_FILE="${state_dir}/hot-dev-bg.pid" \
    HOT_DEV_BG_LOG_FILE="${state_dir}/hot-dev-bg.log" \
    FRONTEND_DEV_HOST=127.0.0.1 \
    FRONTEND_DEV_PORT="${frontend_port}" \
    PULSE_DEV_API_HOST=127.0.0.1 \
    PULSE_DEV_API_PORT="${backend_port}" \
    "${HOT_DEV_BG}" backend-restart 2>&1
    printf '\nexit_code=%s' "$?"
  )"

  assert_contains "backend restart refuses missing managed runtime" "${output}" "Managed hot-dev session is not running"
  assert_contains "backend restart returns failure without managed runtime" "${output}" "exit_code=1"
}

test_detects_unmanaged_listeners() {
  local frontend_port backend_port status_output start_output
  local state_dir
  frontend_port="$(pick_free_port)"
  backend_port="$(pick_free_port)"
  if [[ "${backend_port}" == "${frontend_port}" ]]; then
    backend_port="$(pick_free_port)"
  fi
  state_dir="$(make_isolated_hot_dev_bg_state)"

  start_http_server "${frontend_port}"
  start_http_server "${backend_port}"

  status_output="$(
    HOT_DEV_BG_PID_FILE="${state_dir}/hot-dev-bg.pid" \
    HOT_DEV_BG_LOG_FILE="${state_dir}/hot-dev-bg.log" \
    FRONTEND_DEV_HOST=127.0.0.1 \
    FRONTEND_DEV_PORT="${frontend_port}" \
    PULSE_DEV_API_HOST=127.0.0.1 \
    PULSE_DEV_API_PORT="${backend_port}" \
    "${HOT_DEV_BG}" status
  )"

  assert_contains "status reports unmanaged frontend listener" "${status_output}" "[hot-dev-bg] Port ${frontend_port}: unmanaged listener"
  assert_contains "status reports unmanaged backend listener" "${status_output}" "[hot-dev-bg] Port ${backend_port}: unmanaged listener"
  assert_contains "status explains unmanaged runtime ownership" "${status_output}" "[hot-dev-bg] Detected unmanaged dev listeners. hot-dev-bg is not managing the current runtime."
  assert_contains "status reports shell HTTP" "${status_output}" "[hot-dev-bg] Frontend shell HTTP: 200"
  assert_contains "status reports proxied health mismatch" "${status_output}" "[hot-dev-bg] Frontend proxy /api/health: 404"
  assert_contains "status reports backend health probe" "${status_output}" "[hot-dev-bg] Backend /api/health: 404"

  start_output="$(
    set +e
    HOT_DEV_BG_PID_FILE="${state_dir}/hot-dev-bg.pid" \
    HOT_DEV_BG_LOG_FILE="${state_dir}/hot-dev-bg.log" \
    FRONTEND_DEV_HOST=127.0.0.1 \
    FRONTEND_DEV_PORT="${frontend_port}" \
    PULSE_DEV_API_HOST=127.0.0.1 \
    PULSE_DEV_API_PORT="${backend_port}" \
    "${HOT_DEV_BG}" start 2>&1
    printf '\nexit_code=%s' "$?"
  )"

  assert_contains "start refuses unmanaged takeover by default" "${start_output}" "Refusing to take over unmanaged listeners."
  assert_contains "start returns failure for unmanaged takeover" "${start_output}" "exit_code=1"
}

main() {
  test_cli_parses_takeover_flag
  test_verify_command_injects_managed_runtime_env
  test_root_package_exposes_managed_runtime_entrypoints
  test_frontend_package_exposes_managed_runtime_entrypoints
  test_backend_restart_requires_managed_runtime
  test_status_without_runtime
  test_detects_unmanaged_listeners

  if (( failures > 0 )); then
    echo "Total failures: ${failures}" >&2
    exit 1
  fi

  echo "hot-dev-bg ownership diagnostics smoke tests passed."
}

main "$@"
