#!/usr/bin/env bash

# Run Pulse hot-dev in a managed background session (macOS/Linux).
#
# Usage:
#   ./scripts/hot-dev-bg.sh start
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

is_port_listening() {
  local port="$1"
  lsof -nP -iTCP:"${port}" -sTCP:LISTEN >/dev/null 2>&1
}

wait_for_port() {
  local port="$1"
  local timeout_seconds="${2:-60}"
  local checks=$((timeout_seconds * 2))

  while (( checks > 0 )); do
    if is_port_listening "${port}"; then
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

require_python() {
  command -v python3 >/dev/null 2>&1 || fail "python3 is required"
}

start_bg() {
  require_python
  mkdir -p "$(dirname "${PID_FILE}")"
  touch "${LOG_FILE}"

  if is_running; then
    log "Already running (pid: $(cat "${PID_FILE}"))"
    return 0
  fi

  # Clear stale PID file before fresh start.
  rm -f "${PID_FILE}"

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
  log "Waiting for backend port ${PULSE_DEV_API_PORT}..."
  if ! wait_for_port "${PULSE_DEV_API_PORT}" 90; then
    log "Backend did not start. Showing recent logs:"
    tail -n 80 "${LOG_FILE}" || true
    fail "hot-dev startup failed"
  fi

  if ! wait_for_port "${FRONTEND_DEV_PORT}" 90; then
    log "Backend is up, but frontend port ${FRONTEND_DEV_PORT} is not listening yet."
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
  if is_running; then
    local pid
    pid="$(cat "${PID_FILE}")"
    log "Running (pid: ${pid})"
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

  local frontend_code backend_code
  frontend_code="$(curl -sS -m 2 -o /dev/null -w '%{http_code}' "http://127.0.0.1:${FRONTEND_DEV_PORT}/" 2>/dev/null || true)"
  backend_code="$(curl -sS -m 2 -o /dev/null -w '%{http_code}' "http://127.0.0.1:${PULSE_DEV_API_PORT}/api/health" 2>/dev/null || true)"
  [[ -n "${frontend_code}" ]] || frontend_code="000"
  [[ -n "${backend_code}" ]] || backend_code="000"
  log "Frontend HTTP: ${frontend_code}"
  log "Backend HTTP:  ${backend_code}"
}

logs_bg() {
  touch "${LOG_FILE}"
  tail -n 120 -f "${LOG_FILE}"
}

usage() {
  cat <<EOF
Usage: $(basename "$0") <command>

Commands:
  start
  stop
  restart
  status
  logs
EOF
}

main() {
  case "${1:-}" in
    start) start_bg ;;
    stop) stop_bg ;;
    restart) stop_bg; start_bg ;;
    status) status_bg ;;
    logs) logs_bg ;;
    *) usage; exit 1 ;;
  esac
}

main "${1:-}"
