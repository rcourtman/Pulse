#!/bin/bash
# hot-dev.sh - Development server with hot-reload for Pulse
#
# This script runs a local development environment with:
# - Go backend with auto-rebuild on file changes (via inotifywait)
# - Vite frontend dev server with HMR
# - Auto-detection of pulse-pro module for Pro features
#
# Environment Variables:
#   HOT_DEV_USE_PROD_DATA=true   Use /etc/pulse for data (sessions, config, etc.)
#   HOT_DEV_USE_PRO=true         Build Pro binary (default: true if module available)
#   PULSE_MOCK_MODE=true         Use isolated mock data directory
#   PULSE_DATA_DIR=/path         Override data directory
#   PULSE_DEV_API_PORT=7656      Backend API port (default: 7656)
#   FRONTEND_DEV_PORT=5173       Frontend dev server port (default: 5173)
#
# Pro Features Mode:
#   When /opt/pulse-enterprise exists and HOT_DEV_USE_PRO is not "false",
#   the script builds the Pro binary which includes:
#   - SQLite-based persistent audit logging
#   - RBAC (Role-Based Access Control)
#   - HMAC event signing for tamper detection
#
# Usage:
#   ./scripts/hot-dev.sh                    # Standard dev mode
#   HOT_DEV_USE_PROD_DATA=true ./scripts/hot-dev.sh  # Use production data
#
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT_DIR=$(cd "${SCRIPT_DIR}/.." && pwd)

# --- Helper Functions ---

log_info() { printf "\033[0;34m[hot-dev]\033[0m %s\n" "$1"; }
log_warn() { printf "\033[0;33m[hot-dev] WARNING:\033[0m %s\n" "$1"; }
log_error() { printf "\033[0;31m[hot-dev] ERROR:\033[0m %s\n" "$1"; }

detect_encrypted_files() {
    local data_dir=$1
    local patterns=(nodes.enc* email.enc* webhooks.enc* oidc.enc* ai.enc*)
    local files=()

    for pattern in "${patterns[@]}"; do
        for path in "${data_dir}/${pattern}"; do
            if [[ -s "${path}" ]]; then
                files+=("$(basename "${path}")")
            fi
        done
    done

    printf '%s\n' "${files[@]}"
}

check_dependencies() {
    local missing=0
    for cmd in go npm lsof; do
        if ! command -v $cmd >/dev/null 2>&1; then
            log_error "$cmd is not installed but is required."
            missing=1
        fi
    done
    
    if [[ $missing -eq 1 ]]; then
        exit 1
    fi
}

# --- Configuration & Environment ---

load_env_file() {
    local env_file=$1
    if [[ -f ${env_file} ]]; then
        log_info "Loading ${env_file}"
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

FRONTEND_PORT=${FRONTEND_PORT:-${PORT:-5173}}
PORT=${PORT:-${FRONTEND_PORT}}

# Detect LAN IP
if [[ -z ${LAN_IP:-} ]]; then
    if command -v hostname >/dev/null 2>&1 && hostname -I >/dev/null 2>&1; then
        LAN_IP=$(hostname -I | awk '{print $1}')
    fi
    if [[ -z ${LAN_IP:-} ]]; then
        LAN_IP=$(ipconfig getifaddr en0 2>/dev/null || echo "")
    fi
    if [[ -z ${LAN_IP:-} ]]; then
        LAN_IP="0.0.0.0"
    fi
fi

FRONTEND_DEV_HOST=${FRONTEND_DEV_HOST:-0.0.0.0}
FRONTEND_DEV_PORT=${FRONTEND_DEV_PORT:-${FRONTEND_PORT}}
PULSE_DEV_API_HOST=${PULSE_DEV_API_HOST:-${LAN_IP}}
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

# Set specific allowed origin for CORS with credentials
# Use the frontend dev URL so cross-port SSE requests work with auth cookies
# Added localhost and 127.0.0.1 by default for flexibility (both 7655 and 5173)
ALLOWED_ORIGINS="http://${PULSE_DEV_API_HOST:-127.0.0.1}:${FRONTEND_DEV_PORT:-7655}"
ALLOWED_ORIGINS="${ALLOWED_ORIGINS},http://localhost:${FRONTEND_DEV_PORT:-7655},http://127.0.0.1:${FRONTEND_DEV_PORT:-7655}"
ALLOWED_ORIGINS="${ALLOWED_ORIGINS},http://localhost:5173,http://127.0.0.1:5173"

# Detect and add all system IPs (V4)
for ip in $(hostname -I); do
    if [[ "${ip}" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        if [[ "${ip}" != "127.0.0.1" ]]; then
            ALLOWED_ORIGINS="${ALLOWED_ORIGINS},http://${ip}:${FRONTEND_DEV_PORT:-7655}"
            ALLOWED_ORIGINS="${ALLOWED_ORIGINS},http://${ip}:5173"
        fi
    fi
done

export FRONTEND_PORT PORT
export FRONTEND_DEV_HOST FRONTEND_DEV_PORT
export PULSE_DEV_API_HOST PULSE_DEV_API_PORT PULSE_DEV_API_URL PULSE_DEV_WS_URL
export ALLOWED_ORIGINS

# Proxy Socket Detection
HOST_PROXY_SOCKET="/mnt/pulse-proxy/pulse-sensor-proxy.sock"
CONTAINER_PROXY_SOCKET="/run/pulse-sensor-proxy/pulse-sensor-proxy.sock"

if [[ -z ${PULSE_SENSOR_PROXY_SOCKET:-} ]]; then
    if [[ -S "${HOST_PROXY_SOCKET}" ]]; then
        export PULSE_SENSOR_PROXY_SOCKET="${HOST_PROXY_SOCKET}"
        log_info "Detected pulse-sensor-proxy socket at ${PULSE_SENSOR_PROXY_SOCKET}"
    elif [[ -S "${CONTAINER_PROXY_SOCKET}" ]]; then
        export PULSE_SENSOR_PROXY_SOCKET="${CONTAINER_PROXY_SOCKET}"
        log_warn "Using container-local pulse-sensor-proxy socket at ${PULSE_SENSOR_PROXY_SOCKET}"
        log_warn "Host proxy is missing; temperatures will not reach Pulse until it is reinstalled."
    else
        log_warn "No pulse-sensor-proxy socket detected. Temperatures will be unavailable."
    fi
else
    if [[ ! -S "${PULSE_SENSOR_PROXY_SOCKET}" ]]; then
        log_warn "Configured pulse-sensor-proxy socket not found at ${PULSE_SENSOR_PROXY_SOCKET}"
    elif [[ "${PULSE_SENSOR_PROXY_SOCKET}" == "${CONTAINER_PROXY_SOCKET}" && ! -S "${HOST_PROXY_SOCKET}" ]]; then
        log_warn "Using container-local proxy socket; reinstall host pulse-sensor-proxy for real telemetry."
    fi
fi

EXTRA_CLEANUP_PORT=$((PULSE_DEV_API_PORT + 1))

# --- Startup Checks ---

check_dependencies

cat <<BANNER
=========================================
Starting HOT-RELOAD development mode
=========================================

Frontend: http://${FRONTEND_DEV_HOST}:${FRONTEND_DEV_PORT} (Local)
          http://${LAN_IP}:${FRONTEND_DEV_PORT} (LAN)
Backend API: ${PULSE_DEV_API_URL}

Mock Mode: ${PULSE_MOCK_MODE:-false}
Toggle mock mode: npm run mock:on / npm run mock:off
Mock config: npm run mock:edit

Frontend: Edit files and see changes instantly!
Backend: Auto-rebuilds when .go files change!
Press Ctrl+C to stop
=========================================
BANNER

kill_port() {
    local port=$1
    log_info "Cleaning up port ${port}..."
    lsof -i :"${port}" 2>/dev/null | awk 'NR>1 {print $2}' | xargs -r kill -9 2>/dev/null || true
}

log_info "Cleaning up existing processes..."

# OS-Specific Cleanup
OS_NAME=$(uname -s)
if [[ "$OS_NAME" == "Linux" ]]; then
    if [[ -z "${INVOCATION_ID:-}" ]]; then
        sudo systemctl stop pulse-hot-dev 2>/dev/null || true
    fi
    sudo systemctl stop pulse-backend 2>/dev/null || true
    sudo systemctl stop pulse 2>/dev/null || true
    sudo systemctl stop pulse-frontend 2>/dev/null || true
fi

pkill -f "backend-watch.sh" 2>/dev/null || true
# Only kill vite/npm processes that look like ours (simple check)
pkill -f "vite" 2>/dev/null || true
pkill -f "npm run dev" 2>/dev/null || true

pkill -x "pulse" 2>/dev/null || true
sleep 1
pkill -9 -x "pulse" 2>/dev/null || true

kill_port "${FRONTEND_DEV_PORT}"
kill_port "${PULSE_DEV_API_PORT}"
kill_port "${EXTRA_CLEANUP_PORT}"

sleep 2

# Verify ports are free
set +o pipefail
for port in "${FRONTEND_DEV_PORT}" "${PULSE_DEV_API_PORT}"; do
    if lsof -i :"${port}" 2>/dev/null | grep -q LISTEN; then
        log_error "Port ${port} is still in use after cleanup!"
        kill_port "${port}"
        sleep 2
        if lsof -i :"${port}" 2>/dev/null | grep -q LISTEN; then
            log_error "FATAL: Cannot free port ${port}. Please manually kill the process:"
            lsof -i :"${port}"
            exit 1
        fi
    fi
done
set -o pipefail

log_info "Ports are clean!"

# --- Config Setup ---

if [[ -f "${ROOT_DIR}/mock.env" ]]; then
    load_env_file "${ROOT_DIR}/mock.env"
    if [[ -f "${ROOT_DIR}/mock.env.local" ]]; then
        load_env_file "${ROOT_DIR}/mock.env.local"
        log_info "Loaded mock.env.local overrides"
    fi
    if [[ ${PULSE_MOCK_MODE:-false} == "true" ]]; then
        TOTAL_GUESTS=$((PULSE_MOCK_NODES * (PULSE_MOCK_VMS_PER_NODE + PULSE_MOCK_LXCS_PER_NODE)))
        echo "Mock mode ENABLED with ${PULSE_MOCK_NODES} nodes (${TOTAL_GUESTS} total guests)"
    else
        echo "Syncing production configuration..."
        DEV_DIR="${ROOT_DIR}/tmp/dev-config" "${ROOT_DIR}/scripts/sync-production-config.sh"
    fi
fi

if [[ -f /etc/pulse/.env ]] && [[ -r /etc/pulse/.env ]]; then
    load_env_file "/etc/pulse/.env"
    echo "Auth configuration loaded from /etc/pulse/.env"
fi

# --- Start Backend ---

log_info "Starting backend on port ${PULSE_DEV_API_PORT}..."
cd "${ROOT_DIR}"

mkdir -p internal/api/frontend-modern/dist
touch internal/api/frontend-modern/dist/index.html

# Check if Pro module is available and use it for full audit logging support
PRO_MODULE_DIR="/opt/pulse-enterprise"
if [[ -d "${PRO_MODULE_DIR}" ]] && [[ ${HOT_DEV_USE_PRO:-true} == "true" ]]; then
    log_info "Building Pro binary (includes persistent audit logging)..."
    cd "${PRO_MODULE_DIR}"
    go build -buildvcs=false -o "${ROOT_DIR}/pulse" ./cmd/pulse-enterprise 2>/dev/null || {
        log_warn "Pro build failed, falling back to standard binary"
        cd "${ROOT_DIR}"
        go build -o pulse ./cmd/pulse
    }
    cd "${ROOT_DIR}"
    # Set up audit directory for Pro features
    export PULSE_AUDIT_DIR="${PULSE_DATA_DIR:-/etc/pulse}"
    log_info "Pro audit logging enabled (SQLite storage in ${PULSE_AUDIT_DIR})"
else
    go build -o pulse ./cmd/pulse
fi

FRONTEND_PORT=${PULSE_DEV_API_PORT}
PORT=${PULSE_DEV_API_PORT}
export FRONTEND_PORT PULSE_DEV_API_PORT PORT

# Data Directory Setup
if [[ ${PULSE_MOCK_MODE:-false} == "true" ]]; then
    export PULSE_DATA_DIR="${ROOT_DIR}/tmp/mock-data"
    mkdir -p "$PULSE_DATA_DIR"
    log_info "Mock mode: Using isolated data directory: ${PULSE_DATA_DIR}"
else
    if [[ -n ${PULSE_DATA_DIR:-} ]]; then
        log_info "Using preconfigured data directory: ${PULSE_DATA_DIR}"
    elif [[ ${HOT_DEV_USE_PROD_DATA:-false} == "true" ]]; then
        export PULSE_DATA_DIR=/etc/pulse
        log_info "HOT_DEV_USE_PROD_DATA=true – using production data directory: ${PULSE_DATA_DIR}"
    else
        DEV_CONFIG_DIR="${ROOT_DIR}/tmp/dev-config"
        mkdir -p "$DEV_CONFIG_DIR"
        export PULSE_DATA_DIR="${DEV_CONFIG_DIR}"
        log_info "Production mode: Using dev config directory: ${PULSE_DATA_DIR}"
    fi

    # Auto-restore encryption key from backup if missing
    if [[ ! -f "${PULSE_DATA_DIR}/.encryption.key" ]]; then
        BACKUP_KEY=$(find "${PULSE_DATA_DIR}" -maxdepth 1 -name '.encryption.key.bak*' -type f 2>/dev/null | head -1)
        if [[ -n "${BACKUP_KEY}" ]] && [[ -f "${BACKUP_KEY}" ]]; then
            echo ""
            log_error "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
            log_error "!! ENCRYPTION KEY WAS MISSING - AUTO-RESTORING FROM BACKUP !!"
            log_error "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
            log_error "!! Backup used: ${BACKUP_KEY}"
            log_error "!! "
            log_error "!! To find out what deleted the key, run:"
            log_error "!!   sudo journalctl -u encryption-key-watcher -n 100"
            log_error "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
            echo ""
            cp -f "${BACKUP_KEY}" "${PULSE_DATA_DIR}/.encryption.key"
            chmod 600 "${PULSE_DATA_DIR}/.encryption.key"
            log_info "Restored encryption key from backup"
        fi
    fi

    if [[ -z ${PULSE_ENCRYPTION_KEY:-} ]]; then
        if [[ -f "${PULSE_DATA_DIR}/.encryption.key" ]]; then
            export PULSE_ENCRYPTION_KEY="$(<"${PULSE_DATA_DIR}/.encryption.key")"
            log_info "Loaded encryption key from ${PULSE_DATA_DIR}/.encryption.key"
        elif [[ ${PULSE_DATA_DIR} == "${ROOT_DIR}/tmp/dev-config" ]]; then
            DEV_KEY_FILE="${PULSE_DATA_DIR}/.encryption.key"
            if [[ ! -f "${DEV_KEY_FILE}" ]]; then
                mapfile -t ENCRYPTED_FILES < <(detect_encrypted_files "${PULSE_DATA_DIR}")
                if [[ ${#ENCRYPTED_FILES[@]} -gt 0 ]]; then
                    log_error "Encryption key is missing but encrypted data exists."
                    log_error "Restore ${DEV_KEY_FILE} from backup before continuing."
                    log_error "Encrypted files: ${ENCRYPTED_FILES[*]}"
                    exit 1
                fi
                openssl rand -base64 32 > "${DEV_KEY_FILE}"
                chmod 600 "${DEV_KEY_FILE}"
                log_info "Generated dev encryption key at ${DEV_KEY_FILE}"
            fi
            export PULSE_ENCRYPTION_KEY="$(<"${DEV_KEY_FILE}")"
        elif [[ ${HOT_DEV_USE_PROD_DATA:-false} == "true" ]]; then
            # Production data mode but no key - generate one to prevent orphaned encrypted data
            mapfile -t ENCRYPTED_FILES < <(detect_encrypted_files "${PULSE_DATA_DIR}")
            if [[ ${#ENCRYPTED_FILES[@]} -gt 0 ]]; then
                log_error "Encryption key is missing but encrypted data exists."
                log_error "Restore ${PULSE_DATA_DIR}/.encryption.key from backup before continuing."
                log_error "Encrypted files: ${ENCRYPTED_FILES[*]}"
                exit 1
            fi
            log_warn "No encryption key found for ${PULSE_DATA_DIR}. Generating new key..."
            openssl rand -base64 32 > "${PULSE_DATA_DIR}/.encryption.key"
            chmod 600 "${PULSE_DATA_DIR}/.encryption.key"
            export PULSE_ENCRYPTION_KEY="$(<"${PULSE_DATA_DIR}/.encryption.key")"
            log_info "Generated new encryption key at ${PULSE_DATA_DIR}/.encryption.key"
        else
            log_warn "No encryption key found for ${PULSE_DATA_DIR}. Encrypted config may fail to load."
        fi
    fi
fi

./pulse &
BACKEND_PID=$!

sleep 2

if ! kill -0 "${BACKEND_PID}" 2>/dev/null; then
    log_error "Backend failed to start!"
    exit 1
fi

# --- File Watcher ---

log_info "Starting backend file watcher..."
(
    cd "${ROOT_DIR}"
    
    rebuild_backend() {
        local changed_file=$1
        echo ""
        log_info "🔄 Change detected: $(basename "$changed_file")"
        log_info "Rebuilding backend..."

        if go build -o pulse ./cmd/pulse 2>&1 | grep -v "^#"; then
            log_info "✓ Build successful, restarting backend..."

            OLD_PID=$(pgrep -f "^\./pulse$" || true)
            if [[ -n "$OLD_PID" ]]; then
                kill "$OLD_PID" 2>/dev/null || true
                sleep 1
                if kill -0 "$OLD_PID" 2>/dev/null; then
                    kill -9 "$OLD_PID" 2>/dev/null || true
                fi
            fi

            FRONTEND_PORT=${PULSE_DEV_API_PORT} PORT=${PULSE_DEV_API_PORT} PULSE_DATA_DIR=${PULSE_DATA_DIR} ./pulse &
            NEW_PID=$!
            sleep 1

            if kill -0 "$NEW_PID" 2>/dev/null; then
                log_info "✓ Backend restarted (PID: $NEW_PID)"
            else
                log_error "✗ Backend failed to start!"
            fi
        else
            log_error "✗ Build failed!"
        fi
        log_info "Watching for changes..."
    }

    if command -v inotifywait >/dev/null 2>&1; then
        # Linux: inotifywait
        inotifywait -r -e modify,create,delete,move \
            --exclude '(vendor/|node_modules/|\.git/|\.swp$|\.tmp$|~$)' \
            --format '%e %w%f' \
            "${ROOT_DIR}/cmd" "${ROOT_DIR}/internal" "${ROOT_DIR}/pkg" 2>/dev/null | \
        while read -r event changed_file; do
            if [[ "$changed_file" == *.go ]] || [[ "$event" =~ CREATE|DELETE|MOVED ]]; then
                rebuild_backend "$changed_file"
            fi
        done
    elif command -v fswatch >/dev/null 2>&1; then
        # macOS: fswatch
        log_info "Using fswatch for file monitoring"
        fswatch -r --event Created --event Updated --event Removed --event Renamed \
            --exclude '\.git/' --exclude 'vendor/' --exclude 'node_modules/' \
            --include '\.go$' \
            "${ROOT_DIR}/cmd" "${ROOT_DIR}/internal" "${ROOT_DIR}/pkg" 2>/dev/null | \
        while read -r changed_file; do
            # fswatch sends absolute paths, simple check for .go extension
            if [[ "$changed_file" == *.go ]]; then
                rebuild_backend "$changed_file"
            fi
        done
    else
        log_warn "No supported file watcher found (inotifywait or fswatch). Auto-rebuild disabled."
        sleep 3600
    fi
) &
WATCHER_PID=$!

# --- Cleanup Handler ---

cleanup() {
    echo ""
    log_info "Stopping services..."
    
    # Kill Watcher
    if [[ -n ${WATCHER_PID:-} ]] && kill -0 "${WATCHER_PID}" 2>/dev/null; then
        kill "${WATCHER_PID}" 2>/dev/null || true
    fi
    
    # Kill Backend
    # We re-find the PID because it might have changed during restart
    CURRENT_BACKEND_PID=$(pgrep -f "^\./pulse$" || true)
    if [[ -n ${CURRENT_BACKEND_PID} ]]; then
        kill "${CURRENT_BACKEND_PID}" 2>/dev/null || true
        sleep 1
        if kill -0 "${CURRENT_BACKEND_PID}" 2>/dev/null; then
            kill -9 "${CURRENT_BACKEND_PID}" 2>/dev/null || true
        fi
    fi
    
    # Kill Frontend (Vite)
    if [[ -n ${VITE_PID:-} ]] && kill -0 "${VITE_PID}" 2>/dev/null; then
        kill "${VITE_PID}" 2>/dev/null || true
    fi
    
    # Fallback cleanup
    pkill -f "inotifywait.*pulse" 2>/dev/null || true
    pkill -f "fswatch.*pulse" 2>/dev/null || true
    
    log_info "Hot-dev stopped."
}
trap cleanup INT TERM EXIT

# --- Start Frontend ---

log_info "Starting frontend with hot-reload on port ${FRONTEND_DEV_PORT}..."
cd "${ROOT_DIR}/frontend-modern"

# Run Vite in background and wait for it, so we can trap signals properly
npx vite --config vite.config.ts --host "${FRONTEND_DEV_HOST}" --port "${FRONTEND_DEV_PORT}" --clearScreen false &
VITE_PID=$!

wait "$VITE_PID"
