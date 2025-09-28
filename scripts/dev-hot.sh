#!/usr/bin/env bash
set -euo pipefail

FRONTEND_DEV_HOST=${FRONTEND_DEV_HOST:-127.0.0.1}
FRONTEND_DEV_PORT=${FRONTEND_DEV_PORT:-5173}
FRONTEND_DEV_SERVER=${FRONTEND_DEV_SERVER:-http://${FRONTEND_DEV_HOST}:${FRONTEND_DEV_PORT}}
BACKEND_CMD=${BACKEND_CMD:-go run ./cmd/pulse}
VITE_ARGS=${VITE_ARGS:-}

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

export FRONTEND_DEV_SERVER

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
