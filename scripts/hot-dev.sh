#!/bin/bash
# hot-dev.sh - Foreground Pulse dev runtime escape hatch
#
# This script runs a local development environment with:
# - Go backend with auto-rebuild on file changes (via inotifywait)
# - Vite frontend dev server with HMR
# - Auto-detection of pulse-enterprise module for Pro features
# - Snapshot watcher (if scripts/watch-snapshot.sh exists)
#
# Environment Variables:
#   HOT_DEV_USE_PROD_DATA=true   Use /etc/pulse for data (sessions, config, etc.)
#   HOT_DEV_USE_PRO=true         Build Pro binary (default: true if module available)
#   HOT_DEV_ALLOW_OSS_FALLBACK=true  Allow fallback to OSS backend when Pro build fails
#   HOT_DEV_AUTH_USER=admin      Override the managed dev auth username
#   HOT_DEV_AUTH_PASS=...        Override the managed dev auth password or bcrypt hash
#   PULSE_MOCK_MODE=true         Render mock UI data while keeping real metrics history intact
#   PULSE_DATA_DIR=/path         Override data directory
#   PULSE_DEV_API_PORT=7655      Backend API port (default: 7655)
#   FRONTEND_DEV_PORT=5173       Frontend dev server port (default: 5173)
#   HOT_DEV_BACKEND_HEALTH_STARTUP_GRACE_SECONDS=180
#                                   Backend /api/health grace after starts/restarts
#   HOT_DEV_BACKEND_UNHEALTHY_THRESHOLD=2
#                                   Consecutive failed /api/health probes before restart
#
# Pro Features Mode:
#   When pulse-enterprise repo exists and HOT_DEV_USE_PRO is not "false",
#   the script builds the Pro binary which includes:
#   - SQLite-based persistent audit logging
#   - RBAC (Role-Based Access Control)
#   - HMAC event signing for tamper detection
#
# Usage:
#   npm run dev                             # Canonical managed dev runtime
#   ./scripts/hot-dev.sh                    # Foreground/manual runtime troubleshooting
#   HOT_DEV_USE_PROD_DATA=true ./scripts/hot-dev.sh  # Foreground/manual runtime with production data
#
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
ROOT_DIR=$(cd "${SCRIPT_DIR}/.." && pwd -P)
DEFAULT_PULSE_REPOS_DIR=$(cd "${ROOT_DIR}/.." && pwd -P)
SCRIPT_PATH="${SCRIPT_DIR}/$(basename "${BASH_SOURCE[0]}")"
SCRIPT_MTIME=$(stat -c %Y "${SCRIPT_PATH}" 2>/dev/null || stat -f %m "${SCRIPT_PATH}")

# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/hot-dev-auth.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/hot-dev-runtime.sh"

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

build_backend_binary() {
    local build_output
    PULSE_REPOS_DIR="${PULSE_REPOS_DIR:-${DEFAULT_PULSE_REPOS_DIR}}"
    PRO_MODULE_DIR="${PULSE_REPOS_DIR}/pulse-enterprise"

    if [[ -d "${PRO_MODULE_DIR}" ]] && [[ ${HOT_DEV_USE_PRO:-true} == "true" ]]; then
        log_info "Building Pro binary (includes persistent audit logging)..."
        if build_output=$(cd "${PRO_MODULE_DIR}" && go build -buildvcs=false -o "${ROOT_DIR}/pulse" ./cmd/pulse-enterprise 2>&1); then
            printf "%s\n" "${build_output}" | grep -v "^#" || true
            PRO_BUILD_SUCCESS=true
            return 0
        fi

        printf "%s\n" "${build_output}" | grep -v "^#" || true
        if [[ ${HOT_DEV_ALLOW_OSS_FALLBACK:-false} != "true" ]]; then
            log_error "Pro build failed; refusing to fall back to the OSS backend."
            log_error "Fix pulse-enterprise or set HOT_DEV_ALLOW_OSS_FALLBACK=true to override this explicitly."
            PRO_BUILD_SUCCESS=false
            return 1
        fi

        log_warn "Pro build failed; HOT_DEV_ALLOW_OSS_FALLBACK=true so hot-dev is falling back to the OSS backend."
    fi

    if build_output=$(cd "${ROOT_DIR}" && go build -o pulse ./cmd/pulse 2>&1); then
        printf "%s\n" "${build_output}" | grep -v "^#" || true
        PRO_BUILD_SUCCESS=false
        return 0
    fi

    printf "%s\n" "${build_output}" | grep -v "^#" || true
    PRO_BUILD_SUCCESS=false
    return 1
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

load_runtime_env_overrides() {
    local runtime_env="${PULSE_DATA_DIR}/.env"
    if [[ -f "${runtime_env}" ]]; then
        load_env_file "${runtime_env}"
        log_info "Loaded runtime .env overrides from ${runtime_env}"
    fi
}

sync_runtime_auth_env_overrides() {
    local runtime_env="${PULSE_DATA_DIR}/.env"

    if [[ "${HOT_DEV_USE_PROD_DATA:-false}" == "true" ]] || [[ "${PULSE_DATA_DIR}" == "/etc/pulse" ]]; then
        return
    fi

    hot_dev_sync_auth_env_file "${runtime_env}" "${PULSE_AUTH_USER}" "${PULSE_AUTH_PASS}"
    log_info "Seeded managed dev auth defaults in ${runtime_env}"
}

show_startup_banner() {
    cat <<BANNER
=========================================
Starting HOT-RELOAD development mode
=========================================

Frontend: http://${FRONTEND_DEV_HOST}:${FRONTEND_DEV_PORT} (Local)
          http://${LAN_IP}:${FRONTEND_DEV_PORT} (LAN)
Backend API: ${PULSE_DEV_API_URL}

Dev Credentials: $(hot_dev_auth_banner_line "${PULSE_AUTH_USER:-}" "${PULSE_AUTH_PASS:-}")

Mock Mode: ${PULSE_MOCK_MODE:-false}
Toggle mock mode: npm run mock:on / npm run mock:off
Mock config: npm run mock:edit

Frontend: Edit files and see changes instantly!
Backend: Auto-rebuilds when Go or embedded demo assets change!
Press Ctrl+C to stop
=========================================
BANNER
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
PULSE_DEV_API_PORT=${PULSE_DEV_API_PORT:-7655}

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
if [[ "$(uname -s)" == "Darwin" ]]; then
    # macOS: get IPs from ifconfig
    ALL_IPS=$(ifconfig 2>/dev/null | grep "inet " | awk '{print $2}' | grep -v "^127\.")
else
    # Linux: use hostname -I
    ALL_IPS=$(hostname -I 2>/dev/null || echo "")
fi
for ip in $ALL_IPS; do
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

EXTRA_CLEANUP_PORT=$((PULSE_DEV_API_PORT + 1))
HOT_DEV_RESTART_SENTINEL="${ROOT_DIR}/tmp/hot-dev.restart"
HOT_DEV_VERIFY_LOCK="${HOT_DEV_VERIFY_LOCK_FILE:-${ROOT_DIR}/tmp/hot-dev.verify.lock}"
HOT_DEV_BUILD_LOCK="${HOT_DEV_BUILD_LOCK_FILE:-${ROOT_DIR}/tmp/hot-dev.build.lock}"
HOT_DEV_SELF_BUILD_IGNORE_UNTIL_FILE="${HOT_DEV_SELF_BUILD_IGNORE_UNTIL_FILE:-${ROOT_DIR}/tmp/hot-dev.self-build-ignore-until}"
HOT_DEV_WATCHER_STARTUP_GRACE_SECONDS="${HOT_DEV_WATCHER_STARTUP_GRACE_SECONDS:-5}"
EMBEDDED_FRONTEND_DIR="${ROOT_DIR}/internal/api/frontend-modern"
EMBEDDED_FRONTEND_DIST_DIR="${EMBEDDED_FRONTEND_DIR}/dist"
BACKEND_DEBUG_LOG="${PULSE_BACKEND_LOG_FILE:-/tmp/pulse-debug.log}"

# --- Startup Checks ---

check_dependencies

kill_port() {
    local port=$1
    log_info "Cleaning up port ${port}..."
    lsof -i :"${port}" 2>/dev/null | awk 'NR>1 {print $2}' | xargs -r kill -9 2>/dev/null || true
}

process_parent_id() {
    local pid=$1
    ps -o ppid= -p "${pid}" 2>/dev/null | tr -d '[:space:]'
}

is_current_shell_descendant_of() {
    local target_pid=$1
    local current_pid=$$
    local parent_pid

    [[ -n "${target_pid}" ]] || return 1

    while [[ -n "${current_pid}" && "${current_pid}" != "1" ]]; do
        parent_pid="$(process_parent_id "${current_pid}")"
        [[ -n "${parent_pid}" && "${parent_pid}" != "${current_pid}" ]] || break
        if [[ "${parent_pid}" == "${target_pid}" ]]; then
            return 0
        fi
        current_pid="${parent_pid}"
    done

    return 1
}

kill_stale_npm_dev_wrappers() {
    local pid

    if [[ "${HOT_DEV_SKIP_NPM_CLEANUP:-false}" == "true" ]]; then
        return 0
    fi

    while IFS= read -r pid; do
        [[ -n "${pid}" ]] || continue
        [[ "${pid}" != "$$" ]] || continue
        if is_current_shell_descendant_of "${pid}"; then
            continue
        fi
        kill "${pid}" 2>/dev/null || true
    done < <(pgrep -f "npm run dev" 2>/dev/null || true)
}

log_info "Cleaning up existing processes..."

# OS-Specific Cleanup
# Note: When running under systemd (INVOCATION_ID is set), skip stopping our own service
OS_NAME=$(uname -s)
if [[ "$OS_NAME" == "Linux" ]] && [[ -z "${INVOCATION_ID:-}" ]]; then
    # Only stop pulse-dev if we're NOT running under systemd
    sudo systemctl stop pulse-dev 2>/dev/null || true
fi

pkill -f "backend-watch.sh" 2>/dev/null || true
# Only kill vite/npm processes that look like ours (simple check)
pkill -f "vite" 2>/dev/null || true
kill_stale_npm_dev_wrappers

pkill -x "pulse" 2>/dev/null || true
sleep 1
pkill -9 -x "pulse" 2>/dev/null || true


kill_port "${FRONTEND_DEV_PORT}"
kill_port "${PULSE_DEV_API_PORT}"
kill_port "${EXTRA_CLEANUP_PORT}"

# Truncate debug log
mkdir -p "$(dirname "${BACKEND_DEBUG_LOG}")"
:> "${BACKEND_DEBUG_LOG}"
BACKEND_STARTED_AT_FILE="${HOT_DEV_BACKEND_STARTED_AT_FILE:-${ROOT_DIR}/tmp/hot-dev.backend.started-at}"
mkdir -p "$(dirname "${BACKEND_STARTED_AT_FILE}")"

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

mkdir -p "$(dirname "${HOT_DEV_RESTART_SENTINEL}")"
if [[ ! -e "${HOT_DEV_RESTART_SENTINEL}" ]]; then
    : > "${HOT_DEV_RESTART_SENTINEL}"
fi

# Self-restart check
check_script_change() {
    local current_mtime
    current_mtime=$(stat -c %Y "${SCRIPT_PATH}" 2>/dev/null || stat -f %m "${SCRIPT_PATH}")
    if [[ "${current_mtime}" != "${SCRIPT_MTIME}" ]]; then
        log_warn "hot-dev.sh script changed! Restarting..."
        exec "$0" "$@"
    fi
}

# --- Config Setup ---

if [[ -f /etc/pulse/.env ]] && [[ -r /etc/pulse/.env ]]; then
    load_env_file "/etc/pulse/.env"
    echo "Auth configuration loaded from /etc/pulse/.env"
fi

# --- Start Backend ---

log_info "Starting backend on port ${PULSE_DEV_API_PORT}..."
cd "${ROOT_DIR}"

mkdir -p internal/api/frontend-modern/dist
touch internal/api/frontend-modern/dist/index.html

# Check if Pro module is available and use it for full audit logging support.
# Use PULSE_REPOS_DIR env var or default to the parent directory that contains sibling repos.
if ! build_backend_binary; then
    exit 1
fi

FRONTEND_PORT=${PULSE_DEV_API_PORT}
PORT=${PULSE_DEV_API_PORT}
PULSE_DEV=true  # Enable development mode features (needed for admin bypass etc)
ALLOW_ADMIN_BYPASS=1  # Allow X-Admin-Bypass header in dev mode

# Managed dev credentials default to admin/adminadminadmin unless
# HOT_DEV_AUTH_USER / HOT_DEV_AUTH_PASS explicitly override them.
PULSE_AUTH_USER="$(hot_dev_resolve_auth_user)"
PULSE_AUTH_PASS="$(hot_dev_resolve_auth_pass)"
export FRONTEND_PORT PULSE_DEV_API_PORT PORT PULSE_DEV ALLOW_ADMIN_BYPASS PULSE_AUTH_USER PULSE_AUTH_PASS

# Data Directory Setup
# Keep one persistent data directory in both mock and real modes so real
# metrics history continues to accumulate while mock UI mode is active.
if [[ -n ${PULSE_DATA_DIR:-} ]]; then
    log_info "Using preconfigured data directory: ${PULSE_DATA_DIR}"
elif [[ ${HOT_DEV_USE_PROD_DATA:-false} == "true" ]]; then
    export PULSE_DATA_DIR=/etc/pulse
    log_info "HOT_DEV_USE_PROD_DATA=true – using production data directory: ${PULSE_DATA_DIR}"
else
    DEV_CONFIG_DIR="${ROOT_DIR}/tmp/dev-config"
    mkdir -p "$DEV_CONFIG_DIR"
    export PULSE_DATA_DIR="${DEV_CONFIG_DIR}"
    log_info "Using dev config directory: ${PULSE_DATA_DIR}"
fi

sync_runtime_auth_env_overrides
load_runtime_env_overrides
hot_dev_reconcile_agent_bind_address

if [[ "${PULSE_DATA_DIR}" == "${ROOT_DIR}/tmp/dev-config" ]] && [[ ${PULSE_MOCK_MODE:-false} != "true" ]]; then
    echo "Syncing production configuration..."
    DEV_DIR="${ROOT_DIR}/tmp/dev-config" "${ROOT_DIR}/scripts/sync-production-config.sh"
fi

if [[ ${PULSE_MOCK_MODE:-false} == "true" ]]; then
    log_info "Mock mode enabled: retaining shared data directory (${PULSE_DATA_DIR}) to preserve real history"
    TOTAL_GUESTS=$((${PULSE_MOCK_NODES:-3} * (${PULSE_MOCK_VMS_PER_NODE:-3} + ${PULSE_MOCK_LXCS_PER_NODE:-3})))
    echo "Mock mode ENABLED with ${PULSE_MOCK_NODES:-3} nodes (${TOTAL_GUESTS} total guests)"
fi

show_startup_banner

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
        log_error "!! To find out what deleted the key:"
        log_error "!!   Linux: sudo journalctl -u encryption-key-watcher -n 100"
        log_error "!!   macOS: check /tmp/pulse-debug.log"
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

# Set audit dir for Pro features (must be after PULSE_DATA_DIR is set)
if [[ ${PRO_BUILD_SUCCESS:-false} == "true" ]]; then
    export PULSE_AUDIT_DIR="${PULSE_DATA_DIR}"
    log_info "Pro audit logging enabled (SQLite storage in ${PULSE_AUDIT_DIR})"
fi

STARTED_BACKEND_PID=""
mark_backend_startup_grace() {
    date +%s > "${BACKEND_STARTED_AT_FILE}"
}

start_backend_process() {
    mark_backend_startup_grace
    LOG_LEVEL="${LOG_LEVEL:-debug}" \
    FRONTEND_PORT="${PULSE_DEV_API_PORT:-7655}" \
    PORT="${PULSE_DEV_API_PORT:-7655}" \
    PULSE_DATA_DIR="${PULSE_DATA_DIR:-}" \
    PULSE_ENCRYPTION_KEY="${PULSE_ENCRYPTION_KEY:-}" \
    ALLOW_ADMIN_BYPASS="${ALLOW_ADMIN_BYPASS:-1}" \
    PULSE_DEV="${PULSE_DEV:-true}" \
    PULSE_AUTH_USER="${PULSE_AUTH_USER:-}" \
    PULSE_AUTH_PASS="${PULSE_AUTH_PASS:-}" \
    ALLOWED_ORIGINS="${ALLOWED_ORIGINS:-}" \
    LOG_FILE="${BACKEND_DEBUG_LOG}" \
    LOG_MAX_SIZE="50" \
    ./pulse </dev/null >> "${BACKEND_DEBUG_LOG}" 2>&1 &
    STARTED_BACKEND_PID=$!
}

start_backend_process
BACKEND_PID="${STARTED_BACKEND_PID}"

sleep 2

if ! kill -0 "${BACKEND_PID}" 2>/dev/null; then
    log_error "Backend failed to start! Check ${BACKEND_DEBUG_LOG}"
    exit 1
fi

# --- Backend Health Monitor ---
# Restarts Pulse if it dies unexpectedly (not from file watcher rebuild),
# enforces single-instance, and detects the alive-but-unresponsive state
# (process exists but /api/health is not serving) which a process-only check
# blind-spots. After backend startup grace, consecutive misses on /api/health
# -> kill and restart.
log_info "Starting backend health monitor..."
(
    UNHEALTHY_STREAK=0
    UNHEALTHY_THRESHOLD="${HOT_DEV_BACKEND_UNHEALTHY_THRESHOLD:-2}"
    BACKEND_HEALTH_STARTUP_GRACE_SECONDS="${HOT_DEV_BACKEND_HEALTH_STARTUP_GRACE_SECONDS:-180}"

    [[ "${UNHEALTHY_THRESHOLD}" =~ ^[1-9][0-9]*$ ]] || UNHEALTHY_THRESHOLD=2
    [[ "${BACKEND_HEALTH_STARTUP_GRACE_SECONDS}" =~ ^[0-9]+$ ]] || BACKEND_HEALTH_STARTUP_GRACE_SECONDS=180

    backend_serving() {
        curl -sf -o /dev/null --max-time 3 \
            "http://127.0.0.1:${PULSE_DEV_API_PORT:-7655}/api/health" 2>/dev/null
    }

    backend_in_startup_grace() {
        local backend_started_at
        local now

        [[ -r "${BACKEND_STARTED_AT_FILE}" ]] || return 1
        backend_started_at="$(<"${BACKEND_STARTED_AT_FILE}")"
        [[ "${backend_started_at}" =~ ^[0-9]+$ ]] || return 1

        now="$(date +%s)"
        (( now - backend_started_at < BACKEND_HEALTH_STARTUP_GRACE_SECONDS ))
    }

    while true; do
        sleep 5
        PULSE_COUNT="$(hot_dev_pulse_process_count)"

        if [[ "$PULSE_COUNT" -eq 0 ]]; then
            if backend_in_startup_grace; then
                UNHEALTHY_STREAK=0
                log_warn "⚠️  Pulse process not running yet during backend startup grace (${BACKEND_HEALTH_STARTUP_GRACE_SECONDS}s)"
                continue
            fi
            log_warn "⚠️  Pulse died unexpectedly, restarting..."
            start_backend_process
            NEW_PID="${STARTED_BACKEND_PID}"
            sleep 2
            if kill -0 "$NEW_PID" 2>/dev/null; then
                log_info "✓ Backend auto-restarted (PID: $NEW_PID)"
            else
                log_error "✗ Backend failed to auto-restart!"
            fi
            UNHEALTHY_STREAK=0
        elif [[ "$PULSE_COUNT" -gt 1 ]]; then
            log_error "⚠️  Multiple Pulse processes detected ($PULSE_COUNT), killing all and restarting..."
            pkill -9 -f "^\./pulse$" 2>/dev/null || true
            sleep 2
            start_backend_process
            NEW_PID="${STARTED_BACKEND_PID}"
            sleep 2
            if kill -0 "$NEW_PID" 2>/dev/null; then
                log_info "✓ Backend restarted after killing duplicates (PID: $NEW_PID)"
            else
                log_error "✗ Backend failed to restart after killing duplicates!"
            fi
            UNHEALTHY_STREAK=0
        elif ! backend_serving; then
            if backend_in_startup_grace; then
                UNHEALTHY_STREAK=0
                log_warn "⚠️  Pulse /api/health not serving yet during backend startup grace (${BACKEND_HEALTH_STARTUP_GRACE_SECONDS}s)"
                continue
            fi
            UNHEALTHY_STREAK=$((UNHEALTHY_STREAK + 1))
            log_warn "⚠️  Pulse alive but /api/health unresponsive (streak ${UNHEALTHY_STREAK}/${UNHEALTHY_THRESHOLD})"
            if [[ "$UNHEALTHY_STREAK" -ge "$UNHEALTHY_THRESHOLD" ]]; then
                log_error "⚠️  Killing unresponsive Pulse and restarting..."
                pkill -9 -f "^\./pulse$" 2>/dev/null || true
                sleep 2
                start_backend_process
                NEW_PID="${STARTED_BACKEND_PID}"
                sleep 2
                if kill -0 "$NEW_PID" 2>/dev/null; then
                    log_info "✓ Backend restarted after unresponsive state (PID: $NEW_PID)"
                else
                    log_error "✗ Backend failed to restart after unresponsive state!"
                fi
                UNHEALTHY_STREAK=0
            fi
        else
            UNHEALTHY_STREAK=0
        fi
    done
) &
HEALTH_MONITOR_PID=$!

# --- File Watcher ---

log_info "Starting backend file watcher..."
(
    cd "${ROOT_DIR}"
    
    WATCHER_READY_AT=$(date +%s)
    LAST_RESTART_TIME=0
    SELF_BUILD_IGNORE_UNTIL=$((WATCHER_READY_AT + HOT_DEV_WATCHER_STARTUP_GRACE_SECONDS + 5))
    LAST_RESTART_SENTINEL_MARKER=""
    LAST_PULSE_BINARY_MARKER=""

    watcher_within_startup_grace() {
        local now
        now=$(date +%s)
        (( now - WATCHER_READY_AT < HOT_DEV_WATCHER_STARTUP_GRACE_SECONDS ))
    }

    mark_self_build_output() {
        local now
        now=$(date +%s)
        SELF_BUILD_IGNORE_UNTIL=$((now + 5))
        mkdir -p "$(dirname "${HOT_DEV_SELF_BUILD_IGNORE_UNTIL_FILE}")"
        printf '%s\n' "${SELF_BUILD_IGNORE_UNTIL}" > "${HOT_DEV_SELF_BUILD_IGNORE_UNTIL_FILE}"
    }

    manual_build_event_suppressed() {
        local now
        local shared_ignore_until

        if build_lock_active; then
            return 0
        fi

        now=$(date +%s)
        shared_ignore_until=""
        if [[ -r "${HOT_DEV_SELF_BUILD_IGNORE_UNTIL_FILE}" ]]; then
            shared_ignore_until="$(<"${HOT_DEV_SELF_BUILD_IGNORE_UNTIL_FILE}")"
        fi
        if [[ "${shared_ignore_until}" =~ ^[0-9]+$ ]] && (( now < shared_ignore_until )); then
            return 0
        fi

        (( now < SELF_BUILD_IGNORE_UNTIL ))
    }

    should_rebuild_backend_for_change() {
        local changed_file=$1

        if [[ "${changed_file}" == "${EMBEDDED_FRONTEND_DIST_DIR}" ]] || [[ "${changed_file}" == "${EMBEDDED_FRONTEND_DIST_DIR}/"* ]]; then
            return 0
        fi

        [[ "${changed_file}" == *.go ]] || return 1
        [[ "${changed_file}" == *_test.go ]] && return 1

        return 0
    }

    file_event_marker() {
        local file_path=$1

        [[ -e "${file_path}" ]] || return 1

        if stat -f '%m:%z' "${file_path}" >/dev/null 2>&1; then
            stat -f '%m:%z' "${file_path}"
        else
            stat -c '%Y:%s' "${file_path}"
        fi
    }

    LAST_PULSE_BINARY_MARKER="$(file_event_marker "${ROOT_DIR}/pulse" || true)"

    build_lock_active() {
        local owner_pid

        [[ -f "${HOT_DEV_BUILD_LOCK}" ]] || return 1

        owner_pid="$(awk -F= '/^pid=/{print $2; exit}' "${HOT_DEV_BUILD_LOCK}" 2>/dev/null || true)"
        if [[ -z "${owner_pid}" ]]; then
            rm -f "${HOT_DEV_BUILD_LOCK}"
            return 1
        fi

        if ! kill -0 "${owner_pid}" 2>/dev/null; then
            rm -f "${HOT_DEV_BUILD_LOCK}"
            return 1
        fi

        return 0
    }

    set_build_lock() {
        mkdir -p "$(dirname "${HOT_DEV_BUILD_LOCK}")"
        printf 'pid=%s\ncreated_at=%s\n' "${BASHPID:-$$}" "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" > "${HOT_DEV_BUILD_LOCK}"
    }

    clear_build_lock() {
        rm -f "${HOT_DEV_BUILD_LOCK}"
    }

    verify_lock_active() {
        local owner_pid

        [[ -f "${HOT_DEV_VERIFY_LOCK}" ]] || return 1

        owner_pid="$(awk -F= '/^pid=/{print $2; exit}' "${HOT_DEV_VERIFY_LOCK}" 2>/dev/null || true)"
        if [[ -z "${owner_pid}" ]]; then
            rm -f "${HOT_DEV_VERIFY_LOCK}"
            return 1
        fi

        if ! kill -0 "${owner_pid}" 2>/dev/null; then
            rm -f "${HOT_DEV_VERIFY_LOCK}"
            return 1
        fi

        return 0
    }

    restart_backend() {
        # Debounce: skip if we restarted less than 3 seconds ago
        local now
        now=$(date +%s)
        if (( now - LAST_RESTART_TIME < 3 )); then
            return
        fi
        LAST_RESTART_TIME=$now
        mark_backend_startup_grace
        mark_self_build_output

        log_info "Restarting backend..."

        # Re-source the canonical runtime .env so mock-mode changes take
        # effect without a full hot-dev restart.
        sync_runtime_auth_env_overrides
        load_runtime_env_overrides
        hot_dev_reconcile_agent_bind_address

        # Kill ALL pulse processes (not just one) to prevent duplicates
        pkill -f "^\./pulse$" 2>/dev/null || true
        sleep 1
        # Force kill any remaining
        pkill -9 -f "^\./pulse$" 2>/dev/null || true
        sleep 1

        # Close inherited stdin (</dev/null) to prevent pipe fd leaks when
        # this function is called from inside a piped while-read loop.
        start_backend_process
        NEW_PID="${STARTED_BACKEND_PID}"
        sleep 1

        # Verify exactly one process is running
        PULSE_COUNT="$(hot_dev_pulse_process_count)"
        if [[ "$PULSE_COUNT" -eq 1 ]] && kill -0 "$NEW_PID" 2>/dev/null; then
            log_info "✓ Backend restarted (PID: $NEW_PID)"
        elif [[ "$PULSE_COUNT" -gt 1 ]]; then
            log_error "✗ Multiple processes after restart ($PULSE_COUNT) - killing all"
            pkill -9 -f "^\./pulse$" 2>/dev/null || true
        else
            log_error "✗ Backend failed to start! Check ${BACKEND_DEBUG_LOG}"
        fi
    }

    LAST_REBUILD_TIME=0

    rebuild_backend() {
        local changed_file=$1

        if build_lock_active; then
            log_info "Managed build already in progress; skipping duplicate rebuild event."
            return
        fi

        # Debounce: skip if we rebuilt less than 2 seconds ago (batch saves)
        local now
        now=$(date +%s)
        if (( now - LAST_REBUILD_TIME < 2 )); then
            return
        fi
        LAST_REBUILD_TIME=$now

        echo ""
        log_info "🔄 Change detected: $(basename "$changed_file")"
        log_info "Rebuilding backend..."

        # Suppress manual pulse-binary restart handling for the build output
        # generated by this managed rebuild before the compiler touches ./pulse.
        mark_self_build_output
        set_build_lock

        # Use the same build logic as the initial build.
        if build_backend_binary; then
            mark_self_build_output
            restart_backend
            clear_build_lock
        else
            clear_build_lock
            log_error "✗ Build failed; keeping the current backend process running."
        fi
        LAST_REBUILD_TIME=$(date +%s)
        log_info "Watching for changes..."
    }

    if command -v inotifywait >/dev/null 2>&1; then
        # Linux: inotifywait
        # Watch source directories AND the pulse binary itself (so manual builds trigger restart)
        inotifywait -r -m -e modify,create,delete,move,attrib \
            --exclude '(vendor/|node_modules/|\.git/|\.swp$|\.tmp$|~$)' \
            --format '%e %w%f' \
            "${ROOT_DIR}/cmd" "${ROOT_DIR}/internal" "${ROOT_DIR}/pkg" "${ROOT_DIR}/pulse" "${HOT_DEV_RESTART_SENTINEL}" 2>/dev/null | \
        while read -r event changed_file; do
            check_script_change "$@"
            if watcher_within_startup_grace && { [[ "$changed_file" == "${HOT_DEV_RESTART_SENTINEL}" ]] || [[ "$changed_file" == "${ROOT_DIR}/pulse" ]]; }; then
                continue
            fi
            if [[ "$changed_file" == "${HOT_DEV_RESTART_SENTINEL}" ]]; then
                event_marker="$(file_event_marker "${HOT_DEV_RESTART_SENTINEL}" || true)"
                if [[ -n "${event_marker}" && "${event_marker}" == "${LAST_RESTART_SENTINEL_MARKER}" ]]; then
                    continue
                fi
                LAST_RESTART_SENTINEL_MARKER="${event_marker}"
                log_info "🚀 Managed restart requested, restarting backend..."
                restart_backend
            elif [[ "$changed_file" == "${ROOT_DIR}/pulse" ]]; then
                if verify_lock_active; then
                    continue
                fi
                if manual_build_event_suppressed; then
                    continue
                fi
                event_marker="$(file_event_marker "${ROOT_DIR}/pulse" || true)"
                if [[ -n "${event_marker}" && "${event_marker}" == "${LAST_PULSE_BINARY_MARKER}" ]]; then
                    continue
                fi
                LAST_PULSE_BINARY_MARKER="${event_marker}"
                log_info "🚀 Manual build detected (pulse binary changed), restarting..."
                restart_backend
            elif verify_lock_active; then
                continue
            elif should_rebuild_backend_for_change "$changed_file"; then
                rebuild_backend "$changed_file"
            fi
        done
    elif command -v fswatch >/dev/null 2>&1; then
        # macOS: fswatch
        # Note: AttributeModified is needed because `touch` on macOS only fires
        # that event (not Updated), and touch is the recommended way to trigger
        # a manual rebuild.
        log_info "Using fswatch for file monitoring"

        # Watch the pulse binary too — if someone does `go build -o pulse`
        # manually, we should restart without rebuilding. Watch the embedded
        # frontend parent dir as well so atomic dist swaps trigger a backend
        # rebuild for the backend-served demo surface.
        fswatch --event Created --event Updated --event Renamed --event AttributeModified \
            "${ROOT_DIR}/pulse" "${HOT_DEV_RESTART_SENTINEL}" "${EMBEDDED_FRONTEND_DIR}" 2>/dev/null | \
        while read -r changed_file; do
            if watcher_within_startup_grace && { [[ "$changed_file" == "${HOT_DEV_RESTART_SENTINEL}" ]] || [[ "$changed_file" == "${ROOT_DIR}/pulse" ]]; }; then
                continue
            fi
            if [[ "$changed_file" == "${HOT_DEV_RESTART_SENTINEL}" ]]; then
                event_marker="$(file_event_marker "${HOT_DEV_RESTART_SENTINEL}" || true)"
                if [[ -n "${event_marker}" && "${event_marker}" == "${LAST_RESTART_SENTINEL_MARKER}" ]]; then
                    continue
                fi
                LAST_RESTART_SENTINEL_MARKER="${event_marker}"
                log_info "🚀 Managed restart requested, restarting backend..."
            elif should_rebuild_backend_for_change "$changed_file"; then
                if verify_lock_active; then
                    continue
                fi
                rebuild_backend "$changed_file"
                continue
            elif [[ "$changed_file" == "${ROOT_DIR}/pulse" ]]; then
                if verify_lock_active; then
                    continue
                fi
                if manual_build_event_suppressed; then
                    continue
                fi
                event_marker="$(file_event_marker "${ROOT_DIR}/pulse" || true)"
                if [[ -n "${event_marker}" && "${event_marker}" == "${LAST_PULSE_BINARY_MARKER}" ]]; then
                    continue
                fi
                LAST_PULSE_BINARY_MARKER="${event_marker}"
                log_info "🚀 Manual build detected (pulse binary changed), restarting..."
            else
                continue
            fi
            restart_backend
        done &
        BINARY_WATCH_PID=$!

        fswatch -r -L \
            --event Created --event Updated --event Removed --event Renamed --event AttributeModified \
            --exclude '\.git/' --exclude 'vendor/' --exclude 'node_modules/' \
            --include '\.go$' \
            "${ROOT_DIR}/cmd" "${ROOT_DIR}/internal" "${ROOT_DIR}/pkg" 2>/dev/null | \
        while read -r changed_file; do
            if verify_lock_active; then
                continue
            fi
            if should_rebuild_backend_for_change "$changed_file"; then
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

    # Kill Health Monitor
    if [[ -n ${HEALTH_MONITOR_PID:-} ]] && kill -0 "${HEALTH_MONITOR_PID}" 2>/dev/null; then
        kill "${HEALTH_MONITOR_PID}" 2>/dev/null || true
    fi

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
    
    # Kill file backup watcher
    if [[ -n ${BACKUP_WATCHER_PID:-} ]] && kill -0 "${BACKUP_WATCHER_PID}" 2>/dev/null; then
        kill "${BACKUP_WATCHER_PID}" 2>/dev/null || true
    fi

    # Fallback cleanup
    pkill -f "inotifywait.*pulse" 2>/dev/null || true
    pkill -f "fswatch.*pulse" 2>/dev/null || true
    pkill -f "watch-snapshot.sh" 2>/dev/null || true

    log_info "Hot-dev stopped."
}
trap cleanup INT TERM EXIT

# --- Start File Backup Watcher (optional) ---

SNAPSHOT_SCRIPT="${ROOT_DIR}/scripts/watch-snapshot.sh"
if [[ -x "${SNAPSHOT_SCRIPT}" ]]; then
    log_info "Starting snapshot watcher..."
    "${SNAPSHOT_SCRIPT}" > /tmp/pulse-watch-snapshot.log 2>&1 &
    BACKUP_WATCHER_PID=$!
    log_info "Snapshots: ~/.pulse-snapshots (PID: ${BACKUP_WATCHER_PID})"
fi

# --- Start Frontend ---

log_info "Starting frontend with hot-reload on port ${FRONTEND_DEV_PORT}..."
cd "${ROOT_DIR}/frontend-modern"

# Run Vite in background and wait for it, so we can trap signals properly
npx vite --config vite.config.ts --host "${FRONTEND_DEV_HOST}" --port "${FRONTEND_DEV_PORT}" --clearScreen false &
VITE_PID=$!

wait "$VITE_PID"
