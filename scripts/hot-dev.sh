#!/bin/bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT_DIR=$(cd "${SCRIPT_DIR}/.." && pwd)

load_env_file() {
    local env_file=$1
    if [[ -f ${env_file} ]]; then
        printf "[hot-dev] Loading %s\n" "${env_file}"
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

FRONTEND_DEV_HOST=${FRONTEND_DEV_HOST:-0.0.0.0}
FRONTEND_DEV_PORT=${FRONTEND_DEV_PORT:-${FRONTEND_PORT}}
PULSE_DEV_API_HOST=${PULSE_DEV_API_HOST:-127.0.0.1}
PULSE_DEV_API_PORT=${PULSE_DEV_API_PORT:-7656}

if [[ -z ${PULSE_DEV_API_URL:-} ]]; then
    PULSE_DEV_API_URL="http://${PULSE_DEV_API_HOST}:${PULSE_DEV_API_PORT}"
fi

if [[ -z ${PULSE_DEV_WS_URL:-} ]]; then
    if [[ ${PULSE_DEV_API_URL} == http://* ]]; then
        PULSE_DEV_WS_URL="ws://${PULSE_DEV_API_URL#http://}"
    elif [[ ${PULSE_DEV_API_URL} == https://* ]]; then
        PULSE_DEV_WS_URL="wss://${PULSE_DEV_API_URL#https://}"
    else
        PULSE_DEV_WS_URL=${PULSE_DEV_API_URL}
    fi
fi

export FRONTEND_PORT PORT
export FRONTEND_DEV_HOST FRONTEND_DEV_PORT
export PULSE_DEV_API_HOST PULSE_DEV_API_PORT PULSE_DEV_API_URL PULSE_DEV_WS_URL

EXTRA_CLEANUP_PORT=$((PULSE_DEV_API_PORT + 1))

cat <<BANNER
=========================================
Starting HOT-RELOAD development mode
=========================================

Frontend: http://${FRONTEND_DEV_HOST}:${FRONTEND_DEV_PORT} (with hot-reload)
Backend API: ${PULSE_DEV_API_URL}

Just edit frontend files and see changes instantly!
Press Ctrl+C to stop
=========================================
BANNER

kill_port() {
    local port=$1
    printf "[hot-dev] Cleaning up port %s...\n" "${port}"
    lsof -i :"${port}" 2>/dev/null | awk 'NR>1 {print $2}' | xargs -r kill -9 2>/dev/null || true
}

printf "[hot-dev] Cleaning up existing processes...\n"

sudo systemctl stop pulse-backend 2>/dev/null || true
sudo systemctl stop pulse 2>/dev/null || true
sudo systemctl stop pulse-frontend 2>/dev/null || true

pkill -f "backend-watch.sh" 2>/dev/null || true
pkill -f vite 2>/dev/null || true
pkill -f "npm run dev" 2>/dev/null || true
pkill -f "npm exec" 2>/dev/null || true

pkill -x "pulse" 2>/dev/null || true
sleep 1
pkill -9 -x "pulse" 2>/dev/null || true

kill_port "${FRONTEND_DEV_PORT}"
kill_port "${PULSE_DEV_API_PORT}"
kill_port "${EXTRA_CLEANUP_PORT}"

sleep 3

# Temporarily disable pipefail for port checks (lsof returns 1 when port is free)
set +o pipefail

if lsof -i :"${FRONTEND_DEV_PORT}" 2>/dev/null | grep -q LISTEN; then
    echo "ERROR: Port ${FRONTEND_DEV_PORT} is still in use after cleanup!"
    kill_port "${FRONTEND_DEV_PORT}"
    sleep 2
    if lsof -i :"${FRONTEND_DEV_PORT}" 2>/dev/null | grep -q LISTEN; then
        echo "FATAL: Cannot free port ${FRONTEND_DEV_PORT}. Please manually kill the process:"
        lsof -i :"${FRONTEND_DEV_PORT}"
        exit 1
    fi
fi

if lsof -i :"${PULSE_DEV_API_PORT}" 2>/dev/null | grep -q LISTEN; then
    echo "ERROR: Port ${PULSE_DEV_API_PORT} is still in use after cleanup!"
    kill_port "${PULSE_DEV_API_PORT}"
    sleep 2
    if lsof -i :"${PULSE_DEV_API_PORT}" 2>/dev/null | grep -q LISTEN; then
        echo "FATAL: Cannot free port ${PULSE_DEV_API_PORT}. Please manually kill the process:"
        lsof -i :"${PULSE_DEV_API_PORT}"
        exit 1
    fi
fi

# Re-enable pipefail
set -o pipefail

echo "Ports are clean!"

if [[ -f "${ROOT_DIR}/mock.env" ]]; then
    load_env_file "${ROOT_DIR}/mock.env"
    if [[ ${PULSE_MOCK_MODE:-false} == "true" ]]; then
        TOTAL_GUESTS=$((PULSE_MOCK_NODES * (PULSE_MOCK_VMS_PER_NODE + PULSE_MOCK_LXCS_PER_NODE)))
        echo "Mock mode ENABLED with ${PULSE_MOCK_NODES} nodes (${TOTAL_GUESTS} total guests)"
    fi
fi

if [[ -f /etc/pulse/.env ]] && [[ -r /etc/pulse/.env ]]; then
    set +u
    # shellcheck disable=SC1091
    source /etc/pulse/.env 2>/dev/null || true
    set -u
    echo "Auth configuration loaded from /etc/pulse/.env"
fi

printf "[hot-dev] Starting backend on port %s...\n" "${PULSE_DEV_API_PORT}"
cd "${ROOT_DIR}"

go build -o pulse ./cmd/pulse

# Mock variables already exported via load_env_file (set -a)
# Just export the port variables for the backend
FRONTEND_PORT=${PULSE_DEV_API_PORT}
PORT=${PULSE_DEV_API_PORT}
export FRONTEND_PORT PULSE_DEV_API_PORT PORT
./pulse &
BACKEND_PID=$!

sleep 2

if ! kill -0 "${BACKEND_PID}" 2>/dev/null; then
    echo "ERROR: Backend failed to start!"
    exit 1
fi

cleanup() {
    echo ""
    echo "Stopping services..."
    if [[ -n ${BACKEND_PID:-} ]] && kill -0 "${BACKEND_PID}" 2>/dev/null; then
        kill "${BACKEND_PID}" 2>/dev/null || true
        sleep 1
        if kill -0 "${BACKEND_PID}" 2>/dev/null; then
            echo "Backend not responding to SIGTERM, force killing..."
            kill -9 "${BACKEND_PID}" 2>/dev/null || true
        fi
    fi
    pkill -f vite 2>/dev/null || true
    pkill -f "npm run dev" 2>/dev/null || true
    pkill -9 -x "pulse" 2>/dev/null || true
    echo "Hot-dev stopped. To restart normal service, run: sudo systemctl start pulse-backend"
}
trap cleanup INT TERM EXIT

printf "[hot-dev] Starting frontend with hot-reload on port %s...\n" "${FRONTEND_DEV_PORT}"
echo "If this fails, port ${FRONTEND_DEV_PORT} is still in use!"

cd "${ROOT_DIR}/frontend-modern"

npx vite --config vite.config.ts --host "${FRONTEND_DEV_HOST}" --port "${FRONTEND_DEV_PORT}" --clearScreen false

echo "ERROR: Vite exited unexpectedly!"
echo "Dev mode will auto-restart in 5 seconds via systemd..."
cleanup
