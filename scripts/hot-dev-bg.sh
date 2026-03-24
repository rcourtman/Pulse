#!/usr/bin/env bash

# Run Pulse hot-dev in a managed background session (macOS/Linux).
#
# Usage:
#   ./scripts/hot-dev-bg.sh start [--takeover]
#   ./scripts/hot-dev-bg.sh stop
#   ./scripts/hot-dev-bg.sh restart
#   ./scripts/hot-dev-bg.sh backend-restart
#   ./scripts/hot-dev-bg.sh verify [--takeover]
#   ./scripts/hot-dev-bg.sh status
#   ./scripts/hot-dev-bg.sh logs

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

PID_FILE="${HOT_DEV_BG_PID_FILE:-${ROOT_DIR}/tmp/hot-dev.bg.pid}"
LOG_FILE="${HOT_DEV_BG_LOG_FILE:-${ROOT_DIR}/tmp/hot-dev.bg.log}"

FRONTEND_DEV_HOST="${FRONTEND_DEV_HOST:-127.0.0.1}"
FRONTEND_DEV_PORT="${FRONTEND_DEV_PORT:-5173}"
PULSE_DEV_API_HOST="${PULSE_DEV_API_HOST:-127.0.0.1}"
PULSE_DEV_API_PORT="${PULSE_DEV_API_PORT:-7655}"
PULSE_DEV_API_URL="${PULSE_DEV_API_URL:-http://${PULSE_DEV_API_HOST}:${PULSE_DEV_API_PORT}}"
PULSE_DEV_WS_URL="${PULSE_DEV_WS_URL:-ws://${PULSE_DEV_API_HOST}:${PULSE_DEV_API_PORT}}"
MACOS_HOT_DEV_LABEL="com.pulse.hot-dev"

log() {
  printf "[hot-dev-bg] %s\n" "$*"
}

fail() {
  printf "[hot-dev-bg] ERROR: %s\n" "$*" >&2
  exit 1
}

http_status_code() {
  local url="$1"
  local code
  code="$(curl -sS -m 2 -o /dev/null -w '%{http_code}' "${url}" 2>/dev/null || true)"
  [[ -n "${code}" ]] || code="000"
  printf "%s\n" "${code}"
}

http_status_ok() {
  local code="$1"
  [[ "${code}" =~ ^2[0-9][0-9]$ || "${code}" =~ ^3[0-9][0-9]$ ]]
}

is_port_listening() {
  local port="$1"
  lsof -nP -iTCP:"${port}" -sTCP:LISTEN >/dev/null 2>&1
}

listener_pids() {
  local port="$1"
  lsof -nP -t -iTCP:"${port}" -sTCP:LISTEN 2>/dev/null | sort -u
}

process_group_id() {
  local pid="$1"
  ps -o pgid= -p "${pid}" 2>/dev/null | tr -d '[:space:]'
}

process_command() {
  local pid="$1"
  ps -o command= -p "${pid}" 2>/dev/null | sed 's/^[[:space:]]*//'
}

process_parent_id() {
  local pid="$1"
  ps -o ppid= -p "${pid}" 2>/dev/null | tr -d '[:space:]'
}

pid_is_managed() {
  local pid="$1"
  local session_pid="$2"
  local current_pid="${pid}"
  local parent_pid

  [[ -n "${session_pid}" ]] || return 1

  while [[ -n "${current_pid}" && "${current_pid}" != "1" ]]; do
    if [[ "${current_pid}" == "${session_pid}" ]]; then
      return 0
    fi
    if [[ "$(process_group_id "${current_pid}")" == "${session_pid}" ]]; then
      return 0
    fi
    parent_pid="$(process_parent_id "${current_pid}")"
    [[ -n "${parent_pid}" && "${parent_pid}" != "${current_pid}" ]] || break
    current_pid="${parent_pid}"
  done

  return 1
}

port_has_managed_listener() {
  local port="$1"
  local session_pid="$2"
  local listener_pid

  while IFS= read -r listener_pid; do
    [[ -n "${listener_pid}" ]] || continue
    if pid_is_managed "${listener_pid}" "${session_pid}"; then
      return 0
    fi
  done < <(listener_pids "${port}")

  return 1
}

port_has_unmanaged_listener() {
  local port="$1"
  local session_pid="$2"
  local listener_pid

  while IFS= read -r listener_pid; do
    [[ -n "${listener_pid}" ]] || continue
    if ! pid_is_managed "${listener_pid}" "${session_pid}"; then
      return 0
    fi
  done < <(listener_pids "${port}")

  return 1
}

first_managed_listener_pid() {
  local port="$1"
  local session_pid="$2"
  local listener_pid

  while IFS= read -r listener_pid; do
    [[ -n "${listener_pid}" ]] || continue
    if pid_is_managed "${listener_pid}" "${session_pid}"; then
      printf "%s\n" "${listener_pid}"
      return 0
    fi
  done < <(listener_pids "${port}")

  return 1
}

wait_for_managed_listener() {
  local port="$1"
  local session_pid="$2"
  local timeout_seconds="${3:-60}"
  local checks=$((timeout_seconds * 2))

  while (( checks > 0 )); do
    if port_has_managed_listener "${port}" "${session_pid}"; then
      return 0
    fi
    sleep 0.5
    checks=$((checks - 1))
  done

  return 1
}

is_running() {
  if [[ ! -f "${PID_FILE}" ]]; then
    return 1
  fi
  local pid
  pid="$(cat "${PID_FILE}" 2>/dev/null || true)"
  [[ -n "${pid}" ]] || return 1
  kill -0 "${pid}" 2>/dev/null
}

managed_session_pid() {
  cat "${PID_FILE}" 2>/dev/null || true
}

describe_listener() {
  local port="$1"
  local session_pid="${2:-}"
  local found=0
  local listener_pid pgid owner command

  while IFS= read -r listener_pid; do
    [[ -n "${listener_pid}" ]] || continue
    found=1
    pgid="$(process_group_id "${listener_pid}")"
    owner="unmanaged"
    if pid_is_managed "${listener_pid}" "${session_pid}"; then
      owner="managed"
    fi
    command="$(process_command "${listener_pid}")"
    log "Port ${port}: ${owner} listener pid=${listener_pid} pgid=${pgid:-unknown} cmd=${command:-unknown}"
  done < <(listener_pids "${port}")

  if [[ "${found}" -eq 1 ]]; then
    return 0
  fi
  return 1
}

has_any_listener() {
  local port="$1"
  local listener_pid

  while IFS= read -r listener_pid; do
    [[ -n "${listener_pid}" ]] || continue
    return 0
  done < <(listener_pids "${port}")

  return 1
}

has_unmanaged_listeners() {
  local session_pid="${1:-}"
  local port

  for port in "${FRONTEND_DEV_PORT}" "${PULSE_DEV_API_PORT}"; do
    if has_any_listener "${port}" && port_has_unmanaged_listener "${port}" "${session_pid}"; then
      return 0
    fi
  done

  return 1
}

find_hot_dev_ancestor() {
  local pid="$1"
  local current_pid="${pid}"
  local command parent_pid

  while [[ -n "${current_pid}" && "${current_pid}" != "1" ]]; do
    command="$(process_command "${current_pid}")"
    if [[ "${command}" == *"/scripts/hot-dev.sh"* || "${command}" == "bash scripts/hot-dev.sh" ]]; then
      printf "%s\n" "${current_pid}"
      return 0
    fi
    parent_pid="$(process_parent_id "${current_pid}")"
    [[ -n "${parent_pid}" && "${parent_pid}" != "${current_pid}" ]] || break
    current_pid="${parent_pid}"
  done

  return 1
}

hot_dev_root_pids() {
  pgrep -f '(^|/)(hot-dev\.sh|bash scripts/hot-dev\.sh)$|/scripts/hot-dev\.sh' 2>/dev/null | sort -u
}

stop_hot_dev_sessions() {
  local signal="${1:-TERM}"
  local root_pid root_pgid root_command target_key
  declare -A seen=()

  while IFS= read -r root_pid; do
    [[ -n "${root_pid}" ]] || continue
    root_pgid="$(process_group_id "${root_pid}")"
    if [[ -n "${root_pgid}" ]]; then
      target_key="pgid:${root_pgid}"
      [[ -z "${seen[${target_key}]:-}" ]] || continue
      seen["${target_key}"]=1
      root_command="$(process_command "${root_pid}")"
      log "Takeover sending ${signal} to hot-dev session group ${root_pgid} rooted at pid=${root_pid} cmd=${root_command:-unknown}"
      kill "-${signal}" "-${root_pgid}" 2>/dev/null || kill "-${signal}" "${root_pid}" 2>/dev/null || true
      continue
    fi

    target_key="pid:${root_pid}"
    [[ -z "${seen[${target_key}]:-}" ]] || continue
    seen["${target_key}"]=1
    root_command="$(process_command "${root_pid}")"
    log "Takeover sending ${signal} to hot-dev root pid=${root_pid} cmd=${root_command:-unknown}"
    kill "-${signal}" "${root_pid}" 2>/dev/null || true
  done < <(hot_dev_root_pids)
}

launchd_hot_dev_target() {
  printf "gui/%s/%s\n" "$(id -u)" "${MACOS_HOT_DEV_LABEL}"
}

launchd_hot_dev_active() {
  [[ "$(uname -s)" == "Darwin" ]] || return 1
  launchctl print "$(launchd_hot_dev_target)" >/dev/null 2>&1
}

stop_launchd_hot_dev_job() {
  launchd_hot_dev_active || return 0
  local target
  target="$(launchd_hot_dev_target)"
  log "Takeover booting out launchd job ${MACOS_HOT_DEV_LABEL}"
  launchctl bootout "${target}" >/dev/null 2>&1 || launchctl remove "${MACOS_HOT_DEV_LABEL}" >/dev/null 2>&1 || true
}

stop_takeover_targets() {
  local signal="${1:-TERM}"
  local port listener_pid ancestor_pid target_key target_pid target_pgid target_command
  declare -A seen=()

  for port in "${FRONTEND_DEV_PORT}" "${PULSE_DEV_API_PORT}"; do
    while IFS= read -r listener_pid; do
      [[ -n "${listener_pid}" ]] || continue

      ancestor_pid="$(find_hot_dev_ancestor "${listener_pid}" || true)"
      if [[ -n "${ancestor_pid}" ]]; then
        target_pgid="$(process_group_id "${ancestor_pid}")"
        [[ -n "${target_pgid}" ]] || continue
        target_key="pgid:${target_pgid}"
        [[ -z "${seen[${target_key}]:-}" ]] || continue
        seen["${target_key}"]=1
        target_command="$(process_command "${ancestor_pid}")"
        log "Takeover sending ${signal} to hot-dev process group ${target_pgid} rooted at pid=${ancestor_pid} cmd=${target_command:-unknown}"
        kill "-${signal}" "-${target_pgid}" 2>/dev/null || kill "-${signal}" "${ancestor_pid}" 2>/dev/null || true
        continue
      fi

      target_key="pid:${listener_pid}"
      [[ -z "${seen[${target_key}]:-}" ]] || continue
      seen["${target_key}"]=1
      target_command="$(process_command "${listener_pid}")"
      log "Takeover sending ${signal} to listener pid=${listener_pid} cmd=${target_command:-unknown}"
      kill "-${signal}" "${listener_pid}" 2>/dev/null || true
    done < <(listener_pids "${port}")
  done
}

wait_for_ports_to_clear() {
  local timeout_seconds="${1:-10}"
  local checks=$((timeout_seconds * 2))
  local port

  while (( checks > 0 )); do
    local any_listeners="false"
    for port in "${FRONTEND_DEV_PORT}" "${PULSE_DEV_API_PORT}"; do
      if has_any_listener "${port}"; then
        any_listeners="true"
        break
      fi
    done
    if [[ "${any_listeners}" == "false" ]]; then
      return 0
    fi
    sleep 0.5
    checks=$((checks - 1))
  done

  return 1
}

require_python() {
  command -v python3 >/dev/null 2>&1 || fail "python3 is required"
}

start_bg() {
  local takeover="${1:-false}"
  require_python
  mkdir -p "$(dirname "${PID_FILE}")"
  touch "${LOG_FILE}"

  if is_running; then
    log "Already running (pid: $(cat "${PID_FILE}"))"
    return 0
  fi

  # Clear stale PID file before fresh start.
  rm -f "${PID_FILE}"

  if has_unmanaged_listeners ""; then
    if [[ "${takeover}" != "true" ]]; then
      log "Unmanaged listeners are already using the dev ports."
      describe_listener "${FRONTEND_DEV_PORT}" "" || true
      describe_listener "${PULSE_DEV_API_PORT}" "" || true
      fail "Refusing to take over unmanaged listeners. Stop them first or rerun with: ./scripts/hot-dev-bg.sh start --takeover"
    fi
    log "Taking over existing unmanaged listeners on the dev ports."
    describe_listener "${FRONTEND_DEV_PORT}" "" || true
    describe_listener "${PULSE_DEV_API_PORT}" "" || true
    stop_launchd_hot_dev_job
    stop_hot_dev_sessions TERM
    stop_takeover_targets TERM
    if ! wait_for_ports_to_clear 10; then
      log "Takeover escalation: dev ports are still occupied after TERM."
      stop_hot_dev_sessions KILL
      stop_takeover_targets KILL
      if ! wait_for_ports_to_clear 5; then
        log "Remaining listeners after takeover escalation:"
        describe_listener "${FRONTEND_DEV_PORT}" "" || true
        describe_listener "${PULSE_DEV_API_PORT}" "" || true
        fail "Unable to reclaim the dev ports for managed takeover"
      fi
    fi
  fi

  local pid
  pid="$(
    FRONTEND_DEV_HOST="${FRONTEND_DEV_HOST}" \
    FRONTEND_DEV_PORT="${FRONTEND_DEV_PORT}" \
    PULSE_DEV_API_HOST="${PULSE_DEV_API_HOST}" \
    PULSE_DEV_API_PORT="${PULSE_DEV_API_PORT}" \
    PULSE_DEV_API_URL="${PULSE_DEV_API_URL}" \
    PULSE_DEV_WS_URL="${PULSE_DEV_WS_URL}" \
    ROOT_DIR="${ROOT_DIR}" \
    LOG_FILE="${LOG_FILE}" \
    python3 - <<'PY'
import os
import subprocess

root_dir = os.environ["ROOT_DIR"]
log_file = os.environ["LOG_FILE"]
cmd = [os.path.join(root_dir, "scripts", "hot-dev.sh")]

env = os.environ.copy()

with open(log_file, "ab", buffering=0) as log:
    proc = subprocess.Popen(
        cmd,
        cwd=root_dir,
        stdin=subprocess.DEVNULL,
        stdout=log,
        stderr=subprocess.STDOUT,
        start_new_session=True,
        env=env,
    )

print(proc.pid)
PY
  )"

  [[ -n "${pid}" ]] || fail "Failed to start hot-dev"
  printf "%s\n" "${pid}" > "${PID_FILE}"

  log "Started hot-dev (pid: ${pid})"
  log "Waiting for managed backend listener on port ${PULSE_DEV_API_PORT}..."
  if ! wait_for_managed_listener "${PULSE_DEV_API_PORT}" "${pid}" 90; then
    log "Backend did not start. Showing recent logs:"
    tail -n 80 "${LOG_FILE}" || true
    fail "hot-dev startup failed"
  fi

  if ! wait_for_managed_listener "${FRONTEND_DEV_PORT}" "${pid}" 90; then
    log "Backend is up, but a managed frontend listener on port ${FRONTEND_DEV_PORT} is not ready yet."
    log "Check logs with: ./scripts/hot-dev-bg.sh logs"
  fi

  log "Frontend: http://127.0.0.1:${FRONTEND_DEV_PORT}"
  log "Backend:  http://127.0.0.1:${PULSE_DEV_API_PORT}"
  log "Logs: ${LOG_FILE}"
}

stop_bg() {
  if ! is_running; then
    rm -f "${PID_FILE}"
    log "Not running"
    return 0
  fi

  local pid
  pid="$(cat "${PID_FILE}")"

  # Kill the entire session/process group created by start_new_session=True.
  kill -TERM "-${pid}" 2>/dev/null || kill -TERM "${pid}" 2>/dev/null || true

  local retries=30
  while (( retries > 0 )); do
    if ! kill -0 "${pid}" 2>/dev/null; then
      break
    fi
    sleep 0.5
    retries=$((retries - 1))
  done

  if kill -0 "${pid}" 2>/dev/null; then
    kill -KILL "-${pid}" 2>/dev/null || kill -KILL "${pid}" 2>/dev/null || true
  fi

  rm -f "${PID_FILE}"
  log "Stopped hot-dev"
}

status_bg() {
  local managed_pid=""
  if is_running; then
    managed_pid="$(cat "${PID_FILE}")"
    log "Running (pid: ${managed_pid})"
  else
    log "Not running"
  fi

  if is_port_listening "${FRONTEND_DEV_PORT}"; then
    log "Frontend port ${FRONTEND_DEV_PORT} is listening"
  else
    log "Frontend port ${FRONTEND_DEV_PORT} is not listening"
  fi

  if is_port_listening "${PULSE_DEV_API_PORT}"; then
    log "Backend port ${PULSE_DEV_API_PORT} is listening"
  else
    log "Backend port ${PULSE_DEV_API_PORT} is not listening"
  fi

  local browser_url frontend_url proxy_health_url backend_health_url
  browser_url="http://127.0.0.1:${FRONTEND_DEV_PORT}"
  frontend_url="${browser_url}/"
  proxy_health_url="${browser_url}/api/health"
  backend_health_url="http://127.0.0.1:${PULSE_DEV_API_PORT}/api/health"

  local frontend_code proxy_code backend_code
  frontend_code="$(http_status_code "${frontend_url}")"
  proxy_code="$(http_status_code "${proxy_health_url}")"
  backend_code="$(http_status_code "${backend_health_url}")"
  log "Browser entrypoint: ${browser_url}"
  log "Frontend shell HTTP: ${frontend_code}"
  log "Frontend proxy /api/health: ${proxy_code}"
  log "Backend /api/health: ${backend_code}"

  describe_listener "${FRONTEND_DEV_PORT}" "${managed_pid}" || true
  describe_listener "${PULSE_DEV_API_PORT}" "${managed_pid}" || true

  if launchd_hot_dev_active; then
    log "LaunchAgent ${MACOS_HOT_DEV_LABEL} is active"
  fi

  if [[ -z "${managed_pid}" ]] && has_unmanaged_listeners ""; then
    log "Detected unmanaged dev listeners. hot-dev-bg is not managing the current runtime."
  elif [[ -n "${managed_pid}" ]] && has_unmanaged_listeners "${managed_pid}"; then
    log "Detected split ownership: managed session is running, but one or more dev ports are owned by another process."
  fi

  if http_status_ok "${frontend_code}" && http_status_ok "${proxy_code}" && http_status_ok "${backend_code}"; then
    log "Runtime summary: frontend shell, proxy, and backend are healthy. Use ${browser_url} in the browser."
  elif http_status_ok "${frontend_code}" && ! http_status_ok "${proxy_code}" && http_status_ok "${backend_code}"; then
    log "Runtime summary: frontend shell is up and the backend is healthy, but the frontend proxy path is unhealthy."
  elif http_status_ok "${frontend_code}" && ! http_status_ok "${backend_code}"; then
    log "Runtime summary: frontend shell is up, but the backend health endpoint is unavailable."
  elif ! http_status_ok "${frontend_code}" && http_status_ok "${backend_code}"; then
    log "Runtime summary: backend is healthy, but the frontend dev shell is unavailable. Use the backend URL only for direct API debugging."
  else
    log "Runtime summary: frontend shell and backend are not both healthy. Fix the runtime before trusting browser results."
  fi
}

logs_bg() {
  touch "${LOG_FILE}"
  tail -n 120 -f "${LOG_FILE}"
}

restart_backend_bg() {
  if ! is_running; then
    fail "Managed hot-dev session is not running"
  fi

  local session_pid backend_pid next_backend_pid restart_sentinel
  session_pid="$(managed_session_pid)"
  backend_pid="$(first_managed_listener_pid "${PULSE_DEV_API_PORT}" "${session_pid}" || true)"
  restart_sentinel="${ROOT_DIR}/tmp/hot-dev.restart"

  if [[ ! -f "${restart_sentinel}" ]]; then
    fail "Managed restart sentinel not found at ${restart_sentinel}"
  fi

  log "Requesting managed backend restart via hot-dev file watcher"
  touch "${restart_sentinel}"

  local checks=180
  while (( checks > 0 )); do
    next_backend_pid="$(first_managed_listener_pid "${PULSE_DEV_API_PORT}" "${session_pid}" || true)"
    if [[ -z "${backend_pid}" && -n "${next_backend_pid}" ]]; then
      break
    fi
    if [[ -n "${backend_pid}" && -n "${next_backend_pid}" && "${backend_pid}" != "${next_backend_pid}" ]]; then
      break
    fi
    sleep 0.5
    checks=$((checks - 1))
  done

  if [[ -n "${backend_pid}" && "${backend_pid}" == "${next_backend_pid:-}" ]]; then
    log "Managed backend restart did not replace the listener process. Showing recent logs:"
    tail -n 80 "${LOG_FILE}" || true
    fail "Managed backend restart did not replace the listener process on port ${PULSE_DEV_API_PORT}"
  fi

  log "Waiting for managed backend listener to become healthy on port ${PULSE_DEV_API_PORT}..."
  if ! wait_for_managed_listener "${PULSE_DEV_API_PORT}" "${session_pid}" 90; then
    log "Managed backend did not recover. Showing recent logs:"
    tail -n 80 "${LOG_FILE}" || true
    fail "Managed backend failed to recover after restart"
  fi

  local backend_health_url backend_code
  backend_health_url="http://127.0.0.1:${PULSE_DEV_API_PORT}/api/health"
  backend_code="$(http_status_code "${backend_health_url}")"
  if ! http_status_ok "${backend_code}"; then
    fail "Managed backend listener recovered but health probe returned ${backend_code}"
  fi

  log "Managed backend recovered and is healthy on port ${PULSE_DEV_API_PORT}"
}

runtime_healthy() {
  local frontend_code proxy_code backend_code managed_pid
  managed_pid="$(managed_session_pid)"

  [[ -n "${managed_pid}" ]] || return 1
  port_has_managed_listener "${FRONTEND_DEV_PORT}" "${managed_pid}" || return 1
  port_has_managed_listener "${PULSE_DEV_API_PORT}" "${managed_pid}" || return 1

  frontend_code="$(http_status_code "http://127.0.0.1:${FRONTEND_DEV_PORT}/")"
  proxy_code="$(http_status_code "http://127.0.0.1:${FRONTEND_DEV_PORT}/api/health")"
  backend_code="$(http_status_code "http://127.0.0.1:${PULSE_DEV_API_PORT}/api/health")"

  http_status_ok "${frontend_code}" && http_status_ok "${proxy_code}" && http_status_ok "${backend_code}"
}

run_verify_proof_command() {
  local verify_command integration_dir
  integration_dir="${ROOT_DIR}/tests/integration"
  verify_command="${HOT_DEV_BG_VERIFY_COMMAND:-}"

  if [[ -z "${verify_command}" ]]; then
    (
      cd "${integration_dir}"
      PULSE_E2E_USE_HOT_DEV=1 \
      PULSE_E2E_USERNAME="${PULSE_E2E_USERNAME:-admin}" \
      PULSE_E2E_PASSWORD="${PULSE_E2E_PASSWORD:-admin}" \
      npm test -- tests/16-dev-runtime-recovery.spec.ts --project=chromium
    )
    return
  fi

  (
    cd "${ROOT_DIR}"
    PULSE_E2E_USE_HOT_DEV=1 \
    PULSE_E2E_USERNAME="${PULSE_E2E_USERNAME:-admin}" \
    PULSE_E2E_PASSWORD="${PULSE_E2E_PASSWORD:-admin}" \
    bash -lc "${verify_command}"
  )
}

verify_bg() {
  local takeover="${1:-false}"
  local managed_pid

  require_python

  if ! is_running; then
    log "Managed runtime is not running. Starting it before verification..."
    start_bg "${takeover}"
  else
    managed_pid="$(managed_session_pid)"
    if [[ -n "${managed_pid}" ]] && has_unmanaged_listeners "${managed_pid}"; then
      if [[ "${takeover}" != "true" ]]; then
        fail "Managed runtime has split ownership. Rerun with: ./scripts/hot-dev-bg.sh verify --takeover"
      fi
      log "Managed runtime has split ownership. Reclaiming ports before verification..."
      stop_bg
      start_bg "${takeover}"
    elif ! runtime_healthy; then
      log "Managed runtime is unhealthy. Restarting it before verification..."
      stop_bg
      start_bg "${takeover}"
    fi
  fi

  if ! runtime_healthy; then
    status_bg
    fail "Managed runtime is not healthy enough to run browser verification"
  fi

  log "Running managed dev-runtime browser verification proof..."
  run_verify_proof_command
}

usage() {
  cat <<EOF
Usage: $(basename "$0") <command>

Commands:
  start [--takeover]
  stop
  restart
  backend-restart
  verify [--takeover]
  status
  logs
EOF
}

parse_takeover_flag() {
  local command="${1:-}"
  local flag="${2:-}"

  case "${command}" in
    start|restart|verify)
      if [[ -z "${flag}" ]]; then
        printf "false\n"
        return 0
      fi
      if [[ "${flag}" == "--takeover" ]]; then
        printf "true\n"
        return 0
      fi
      usage
      return 1
      ;;
    stop|backend-restart|status|logs)
      if [[ -n "${flag}" ]]; then
        usage
        return 1
      fi
      printf "false\n"
      return 0
      ;;
    *)
      printf "false\n"
      return 0
      ;;
  esac
}

main() {
  local command="${1:-}"
  local takeover
  takeover="$(parse_takeover_flag "$@")" || exit 1

  case "${command}" in
    start)
      start_bg "${takeover}"
      ;;
    stop) stop_bg ;;
    restart)
      stop_bg
      start_bg "${takeover}"
      ;;
    backend-restart) restart_backend_bg ;;
    verify) verify_bg "${takeover}" ;;
    status) status_bg ;;
    logs) logs_bg ;;
    *) usage; exit 1 ;;
  esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
