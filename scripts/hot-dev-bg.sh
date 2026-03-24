#!/usr/bin/env bash

# Run Pulse hot-dev in a managed background session (macOS/Linux).
#
# Usage:
#   ./scripts/hot-dev-bg.sh start [--takeover]
#   ./scripts/hot-dev-bg.sh stop
#   ./scripts/hot-dev-bg.sh restart
#   ./scripts/hot-dev-bg.sh status
#   ./scripts/hot-dev-bg.sh logs

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

PID_FILE="${ROOT_DIR}/tmp/hot-dev.bg.pid"
LOG_FILE="${ROOT_DIR}/tmp/hot-dev.bg.log"

FRONTEND_DEV_HOST="${FRONTEND_DEV_HOST:-127.0.0.1}"
FRONTEND_DEV_PORT="${FRONTEND_DEV_PORT:-5173}"
PULSE_DEV_API_HOST="${PULSE_DEV_API_HOST:-127.0.0.1}"
PULSE_DEV_API_PORT="${PULSE_DEV_API_PORT:-7655}"
PULSE_DEV_API_URL="${PULSE_DEV_API_URL:-http://${PULSE_DEV_API_HOST}:${PULSE_DEV_API_PORT}}"
PULSE_DEV_WS_URL="${PULSE_DEV_WS_URL:-ws://${PULSE_DEV_API_HOST}:${PULSE_DEV_API_PORT}}"

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

listener_is_managed() {
  local port="$1"
  local session_pid="$2"
  local listener_pid

  [[ -n "${session_pid}" ]] || return 1

  while IFS= read -r listener_pid; do
    [[ -n "${listener_pid}" ]] || continue
    if [[ "$(process_group_id "${listener_pid}")" == "${session_pid}" ]]; then
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
    if listener_is_managed "${port}" "${session_pid}"; then
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
    if [[ -n "${session_pid}" && "${pgid}" == "${session_pid}" ]]; then
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
    if has_any_listener "${port}" && ! listener_is_managed "${port}" "${session_pid}"; then
      return 0
    fi
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

usage() {
  cat <<EOF
Usage: $(basename "$0") <command>

Commands:
  start [--takeover]
  stop
  restart
  status
  logs
EOF
}

main() {
  local command="${1:-}"
  local flag="${2:-}"
  local takeover="false"

  if [[ "${flag}" == "--takeover" ]]; then
    takeover="true"
  fi

  case "${command}" in
    start)
      if [[ -n "${flag}" && "${flag}" != "--takeover" ]]; then
        usage
        exit 1
      fi
      start_bg "${takeover}"
      ;;
    stop) stop_bg ;;
    restart)
      if [[ -n "${flag}" && "${flag}" != "--takeover" ]]; then
        usage
        exit 1
      fi
      stop_bg
      start_bg "${takeover}"
      ;;
    status) status_bg ;;
    logs) logs_bg ;;
    *) usage; exit 1 ;;
  esac
}

main "${1:-}"
