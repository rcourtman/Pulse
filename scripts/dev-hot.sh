#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT_DIR=$(cd "${SCRIPT_DIR}/.." && pwd)

load_env_file() {
  local env_file=$1
  if [[ -f ${env_file} ]]; then
    printf "[dev-hot] Loading %s\n" "${env_file}"
    set +u
    set -a
    # shellcheck disable=SC1090
    source "${env_file}"
    set +a
    set -u
  fi
}

load_env_file "${ROOT_DIR}/.env"
load_env_file "${ROOT_DIR}/.env.local"
load_env_file "${ROOT_DIR}/.env.dev"

FRONTEND_PORT=${FRONTEND_PORT:-${PORT:-7655}}
PORT=${PORT:-${FRONTEND_PORT}}
export FRONTEND_PORT PORT

PULSE_DEV_API_HOST=${PULSE_DEV_API_HOST:-127.0.0.1}
PULSE_DEV_API_PORT=${PULSE_DEV_API_PORT:-${FRONTEND_PORT}}
PULSE_DEV_API_URL=${PULSE_DEV_API_URL:-http://${PULSE_DEV_API_HOST}:${PULSE_DEV_API_PORT}}
export PULSE_DEV_API_HOST PULSE_DEV_API_PORT PULSE_DEV_API_URL

FRONTEND_DEV_HOST=${FRONTEND_DEV_HOST:-127.0.0.1}
FRONTEND_DEV_PORT=${FRONTEND_DEV_PORT:-5173}
FRONTEND_DEV_SERVER=${FRONTEND_DEV_SERVER:-http://${FRONTEND_DEV_HOST}:${FRONTEND_DEV_PORT}}
BACKEND_CMD=${BACKEND_CMD:-go run ./cmd/pulse}
VITE_ARGS=${VITE_ARGS:-}

printf "[dev-hot] Backend port: %s\n" "${FRONTEND_PORT}"
printf "[dev-hot] Backend API URL: %s\n" "${PULSE_DEV_API_URL}"
printf "[dev-hot] Frontend dev server target: %s\n" "${FRONTEND_DEV_SERVER}"

cleanup() {
  local exit_code=${1:-$?}
  trap - EXIT INT TERM
  if [[ -n ${VITE_PID:-} ]] && kill -0 "$VITE_PID" >/dev/null 2>&1; then
    kill "$VITE_PID" >/dev/null 2>&1 || true
  fi
  if [[ -n ${BACKEND_PID:-} ]] && kill -0 "$BACKEND_PID" >/dev/null 2>&1; then
    kill "$BACKEND_PID" >/dev/null 2>&1 || true
  fi
  wait >/dev/null 2>&1 || true
  exit "$exit_code"
}

trap cleanup EXIT INT TERM

export FRONTEND_DEV_HOST FRONTEND_DEV_PORT FRONTEND_DEV_SERVER

printf "[dev-hot] Starting Vite dev server at %s\n" "$FRONTEND_DEV_SERVER"
(
  cd frontend-modern
  npm run dev -- --host "$FRONTEND_DEV_HOST" --port "$FRONTEND_DEV_PORT" $VITE_ARGS
) &
VITE_PID=$!

# Give Vite a moment to boot up before starting the backend proxy.
sleep 2

printf "[dev-hot] Starting Pulse backend with FRONTEND_DEV_SERVER=%s\n" "$FRONTEND_DEV_SERVER"
${BACKEND_CMD} &
BACKEND_PID=$!

# Wait for either process to exit and propagate the status code.
wait -n "$VITE_PID" "$BACKEND_PID"
EXIT_STATUS=$?
cleanup "$EXIT_STATUS"
