#!/usr/bin/env bash

# Toggle script for Pulse mock mode.
#
# Goals:
# - `./scripts/toggle-mock.sh` toggles mode (run again toggles back)
# - Rebuild frontend + backend every switch
# - Stop and restart the active runtime (systemd, hot-dev, or standalone)
# - Work on macOS and Linux

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

MOCK_ENV_FILE="${ROOT_DIR}/mock.env"
DEV_DATA_DIR="${ROOT_DIR}/tmp/dev-config"
MOCK_DATA_DIR="${ROOT_DIR}/tmp/mock-data"
DEV_KEY_FILE="${DEV_DATA_DIR}/.encryption.key"

HOT_DEV_LOG="/tmp/pulse-hot-dev.log"
STANDALONE_LOG="/tmp/pulse-standalone.log"
DEFAULT_PORT=7655
LOCK_DIR="/tmp/pulse-toggle-mock.lock"
LOCK_PID_FILE="${LOCK_DIR}/pid"

log_info() {
    echo -e "${BLUE}[toggle-mock]${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}[toggle-mock] WARNING:${NC} $*"
}

log_error() {
    echo -e "${RED}[toggle-mock] ERROR:${NC} $*"
}

log_success() {
    echo -e "${GREEN}[toggle-mock]${NC} $*"
}

acquire_lock() {
    if mkdir "${LOCK_DIR}" 2>/dev/null; then
        echo "$$" > "${LOCK_PID_FILE}"
        trap 'release_lock' EXIT
        return
    fi

    if [[ -f "${LOCK_PID_FILE}" ]]; then
        local owner_pid
        owner_pid="$(cat "${LOCK_PID_FILE}" 2>/dev/null || true)"
        if [[ -n "${owner_pid}" ]] && kill -0 "${owner_pid}" 2>/dev/null; then
            log_error "Another toggle run is in progress (pid: ${owner_pid}). Try again in a few seconds."
            exit 1
        fi
    fi

    rm -rf "${LOCK_DIR}" 2>/dev/null || true
    if mkdir "${LOCK_DIR}" 2>/dev/null; then
        echo "$$" > "${LOCK_PID_FILE}"
        trap 'release_lock' EXIT
        return
    fi

    log_error "Failed to acquire toggle lock at ${LOCK_DIR}"
    exit 1
}

release_lock() {
    rm -rf "${LOCK_DIR}" 2>/dev/null || true
}

launch_detached() {
    local log_file="$1"
    shift

    if command -v python3 >/dev/null 2>&1; then
        python3 - "$log_file" "$@" <<'PY'
import subprocess
import sys

log_path = sys.argv[1]
cmd = sys.argv[2:]

with open(log_path, "ab", buffering=0) as log:
    proc = subprocess.Popen(
        cmd,
        stdin=subprocess.DEVNULL,
        stdout=log,
        stderr=subprocess.STDOUT,
        start_new_session=True,
    )

print(proc.pid)
PY
        return
    fi

    nohup "$@" >"${log_file}" 2>&1 &
    echo $!
}

is_port_listening() {
    local port="$1"
    lsof -nP -iTCP:"${port}" -sTCP:LISTEN >/dev/null 2>&1
}

wait_for_port() {
    local port="$1"
    local timeout_seconds="${2:-20}"
    local checks=$((timeout_seconds * 2))

    while (( checks > 0 )); do
        if is_port_listening "$port"; then
            return 0
        fi
        sleep 0.5
        checks=$((checks - 1))
    done

    return 1
}

prefer_hot_dev_runtime() {
    local prefer="${PULSE_TOGGLE_PREFER_HOT_DEV:-auto}"
    case "${prefer}" in
        true) return 0 ;;
        false) return 1 ;;
        auto)
            if [[ "$(uname -s)" == "Darwin" ]]; then
                return 0
            fi
            return 1
            ;;
        *)
            log_warn "Unknown PULSE_TOGGLE_PREFER_HOT_DEV=${prefer}; using auto"
            if [[ "$(uname -s)" == "Darwin" ]]; then
                return 0
            fi
            return 1
            ;;
    esac
}

use_isolated_mock_data_dir() {
    local value="${PULSE_TOGGLE_ISOLATE_MOCK_DATA:-false}"
    case "${value}" in
        true|TRUE|1|yes|YES|on|ON) return 0 ;;
        *) return 1 ;;
    esac
}

verify_mock_state_via_frontend_proxy() {
    local target_mode="$1"
    local expected="false"
    local url="${PULSE_TOGGLE_FRONTEND_VERIFY_URL:-http://127.0.0.1:5173/api/system/mock-mode}"
    local retries=40

    if [[ "${target_mode}" == "true" ]]; then
        expected="true"
    fi

    while (( retries > 0 )); do
        local resp
        resp="$(curl -fsS --max-time 2 "${url}" 2>/dev/null || true)"
        if [[ -n "${resp}" ]] && echo "${resp}" | grep -q "\"enabled\":${expected}"; then
            return 0
        fi
        sleep 0.5
        retries=$((retries - 1))
    done

    return 1
}

sed_inplace() {
    local expr="$1"
    local file="$2"
    if [[ "$(uname -s)" == "Darwin" ]]; then
        sed -i '' "$expr" "$file"
    else
        sed -i "$expr" "$file"
    fi
}

load_env_file() {
    local env_file="$1"
    if [[ -f "$env_file" ]]; then
        set +u
        set -a
        # shellcheck disable=SC1090
        source "$env_file"
        set +a
        set -u
    fi
}

ensure_mock_env_file() {
    if [[ -f "$MOCK_ENV_FILE" ]]; then
        return
    fi

    cat > "$MOCK_ENV_FILE" <<'ENVEOF'
# Mock Mode Configuration
PULSE_MOCK_MODE=false
PULSE_MOCK_NODES=7
PULSE_MOCK_VMS_PER_NODE=5
PULSE_MOCK_LXCS_PER_NODE=8
PULSE_MOCK_DOCKER_HOSTS=3
PULSE_MOCK_DOCKER_CONTAINERS=12
PULSE_MOCK_GENERIC_HOSTS=4
PULSE_MOCK_K8S_CLUSTERS=2
PULSE_MOCK_K8S_NODES=4
PULSE_MOCK_K8S_PODS=30
PULSE_MOCK_K8S_DEPLOYMENTS=12
PULSE_MOCK_RANDOM_METRICS=true
PULSE_MOCK_STOPPED_PERCENT=20
ENVEOF

    log_info "Created $MOCK_ENV_FILE"
}

get_mock_mode() {
    ensure_mock_env_file
    set +u
    # shellcheck disable=SC1090
    source "$MOCK_ENV_FILE"
    set -u
    if [[ "${PULSE_MOCK_MODE:-false}" == "true" ]]; then
        echo "true"
    else
        echo "false"
    fi
}

set_mock_mode() {
    local enabled="$1"
    ensure_mock_env_file

    if grep -Eq '^[[:space:]]*PULSE_MOCK_MODE=' "$MOCK_ENV_FILE"; then
        sed_inplace "s/^[[:space:]]*PULSE_MOCK_MODE=.*/PULSE_MOCK_MODE=${enabled}/" "$MOCK_ENV_FILE"
    else
        printf '\nPULSE_MOCK_MODE=%s\n' "$enabled" >> "$MOCK_ENV_FILE"
    fi

    touch "$MOCK_ENV_FILE"
}

detect_runtime_mode() {
    if command -v systemctl >/dev/null 2>&1; then
        if systemctl is-active --quiet pulse-hot-dev 2>/dev/null; then
            echo "systemd-hot-dev"
            return
        fi
        if systemctl is-active --quiet pulse 2>/dev/null; then
            echo "systemd-pulse"
            return
        fi
    fi

    if pgrep -f "hot-dev.sh" >/dev/null 2>&1; then
        echo "hot-dev"
        return
    fi

    if pgrep -f "${ROOT_DIR}/pulse|(^|[[:space:]])\./pulse([[:space:]]|$)" >/dev/null 2>&1; then
        echo "standalone"
        return
    fi

    if pgrep -x pulse >/dev/null 2>&1; then
        echo "pulse-name"
        return
    fi

    echo "none"
}

kill_with_grace() {
    local pattern="$1"
    local signal="${2:-TERM}"

    if ! pgrep -f "$pattern" >/dev/null 2>&1; then
        return
    fi

    pkill -"$signal" -f "$pattern" 2>/dev/null || true
}

wait_for_exit() {
    local pattern="$1"
    local retries=20

    while (( retries > 0 )); do
        if ! pgrep -f "$pattern" >/dev/null 2>&1; then
            return
        fi
        sleep 0.25
        retries=$((retries - 1))
    done

    pkill -9 -f "$pattern" 2>/dev/null || true
}

stop_hot_dev_runtime() {
    local hot_dev_pattern="${ROOT_DIR}/scripts/hot-dev.sh|scripts/hot-dev.sh|hot-dev.sh"
    local pulse_pattern="${ROOT_DIR}/pulse|(^|[[:space:]])\\./pulse([[:space:]]|$)"
    local vite_pattern="${ROOT_DIR}/frontend-modern.*vite|vite.*${ROOT_DIR}/frontend-modern|vite --config vite.config.ts"

    log_info "Stopping hot-dev runtime..."

    kill_with_grace "$hot_dev_pattern" TERM
    wait_for_exit "$hot_dev_pattern"

    kill_with_grace "$pulse_pattern" TERM
    wait_for_exit "$pulse_pattern"

    kill_with_grace "$vite_pattern" TERM
    wait_for_exit "$vite_pattern"
}

stop_standalone_runtime() {
    local allow_name_kill="${1:-false}"
    local pulse_pattern="${ROOT_DIR}/pulse|(^|[[:space:]])\\./pulse([[:space:]]|$)"

    log_info "Stopping standalone Pulse runtime..."
    kill_with_grace "$pulse_pattern" TERM
    wait_for_exit "$pulse_pattern"

    if [[ "$allow_name_kill" == "true" ]] && pgrep -x pulse >/dev/null 2>&1; then
        log_warn "Found remaining 'pulse' process by name; stopping it"
        pkill -TERM -x pulse 2>/dev/null || true
        sleep 1
        pkill -9 -x pulse 2>/dev/null || true
    fi
}

stop_systemd_runtime() {
    local mode="$1"

    case "$mode" in
        systemd-hot-dev)
            log_info "Stopping systemd service: pulse-hot-dev"
            sudo systemctl stop pulse-hot-dev
            ;;
        systemd-pulse)
            log_info "Stopping systemd service: pulse"
            sudo systemctl stop pulse
            ;;
    esac
}

has_frontend_artifacts() {
    [[ -f "${ROOT_DIR}/frontend-modern/dist/index.html" ]] && [[ -f "${ROOT_DIR}/internal/api/frontend-modern/dist/index.html" ]]
}

has_backend_artifact() {
    [[ -x "${ROOT_DIR}/pulse" ]]
}

rebuild_frontend_backend() {
    # Rebuild strategy:
    # - auto (default): only rebuild when required artifacts are missing
    # - always: force full frontend+backend rebuild
    # - never: skip rebuild completely
    local rebuild_mode="${PULSE_TOGGLE_REBUILD:-auto}"
    local do_frontend=false
    local do_backend=false

    case "$rebuild_mode" in
        always)
            do_frontend=true
            do_backend=true
            ;;
        never)
            log_info "Skipping rebuild (PULSE_TOGGLE_REBUILD=never)"
            return
            ;;
        auto)
            if ! has_frontend_artifacts; then
                do_frontend=true
                # Backend embeds frontend assets, so rebuild backend after frontend build.
                do_backend=true
            fi
            if ! has_backend_artifact; then
                do_backend=true
            fi
            ;;
        *)
            log_warn "Unknown PULSE_TOGGLE_REBUILD=${rebuild_mode}; using auto"
            if ! has_frontend_artifacts; then
                do_frontend=true
                do_backend=true
            fi
            if ! has_backend_artifact; then
                do_backend=true
            fi
            ;;
    esac

    if [[ "$do_frontend" == "false" ]] && [[ "$do_backend" == "false" ]]; then
        log_info "Build artifacts already present; skipping rebuild"
        return
    fi

    (
        cd "$ROOT_DIR"
        if [[ "$do_frontend" == "true" ]]; then
            log_info "Building frontend..."
            make frontend
        fi
        if [[ "$do_backend" == "true" ]]; then
            log_info "Building backend..."
            make backend
        fi
    )

    log_success "Build complete"
}

ensure_dev_key() {
    mkdir -p "$DEV_DATA_DIR"

    if [[ ! -f "$DEV_KEY_FILE" ]]; then
        openssl rand -base64 32 > "$DEV_KEY_FILE"
        chmod 600 "$DEV_KEY_FILE"
        log_info "Generated dev encryption key at ${DEV_KEY_FILE}"
    fi
}

sync_production_config() {
    if [[ -x "${ROOT_DIR}/scripts/sync-production-config.sh" ]]; then
        log_info "Syncing production configuration into dev config"
        DEV_DIR="$DEV_DATA_DIR" "${ROOT_DIR}/scripts/sync-production-config.sh"
    else
        log_warn "sync-production-config.sh not found or not executable; skipping sync"
    fi
}

start_hot_dev_runtime() {
    local pid
    local expected_backend_port="${PULSE_TOGGLE_BACKEND_PORT:-7655}"
    local expected_frontend_port="${PULSE_TOGGLE_FRONTEND_PORT:-5173}"
    local startup_timeout="${PULSE_TOGGLE_HOT_DEV_START_TIMEOUT:-45}"
    local frontend_dev_host="${FRONTEND_DEV_HOST:-127.0.0.1}"
    local pulse_dev_api_host="${PULSE_DEV_API_HOST:-127.0.0.1}"
    local pulse_dev_api_url="${PULSE_DEV_API_URL:-http://${pulse_dev_api_host}:${expected_backend_port}}"
    local pulse_dev_ws_url="${PULSE_DEV_WS_URL:-ws://${pulse_dev_api_host}:${expected_backend_port}}"
    log_info "Starting hot-dev runtime in background"
    pid="$(
        cd "${ROOT_DIR}" && launch_detached "${HOT_DEV_LOG}" \
            /usr/bin/env \
            FRONTEND_DEV_HOST="${frontend_dev_host}" \
            FRONTEND_DEV_PORT="${expected_frontend_port}" \
            PULSE_DEV_API_HOST="${pulse_dev_api_host}" \
            PULSE_DEV_API_PORT="${expected_backend_port}" \
            PULSE_DEV_API_URL="${pulse_dev_api_url}" \
            PULSE_DEV_WS_URL="${pulse_dev_ws_url}" \
            "${ROOT_DIR}/scripts/hot-dev.sh"
    )"
    sleep 2

    if ! pgrep -f "hot-dev.sh" >/dev/null 2>&1; then
        log_error "hot-dev failed to start. Check ${HOT_DEV_LOG}"
        return 1
    fi

    if ! wait_for_port "${expected_backend_port}" "${startup_timeout}"; then
        log_warn "hot-dev did not open backend port ${expected_backend_port} within ${startup_timeout}s"
        return 2
    fi

    # Frontend startup can lag behind backend startup when dependency graph is large.
    if ! wait_for_port "${expected_frontend_port}" "${startup_timeout}"; then
        log_warn "hot-dev backend is up but frontend port ${expected_frontend_port} is not listening yet"
        log_warn "Proceeding with backend-only availability; check ${HOT_DEV_LOG} for frontend startup"
    fi

    log_success "hot-dev started (pid: ${pid}, backend:${expected_backend_port}, frontend:${expected_frontend_port}, log: ${HOT_DEV_LOG})"
}

start_standalone_runtime() {
    local target_mock_mode="$1"
    local pid
    local startup_timeout="${PULSE_TOGGLE_STANDALONE_START_TIMEOUT:-20}"
    local data_dir="$DEV_DATA_DIR"

    log_info "Starting standalone Pulse"

    cd "$ROOT_DIR"

    load_env_file "${ROOT_DIR}/.env"
    load_env_file "${ROOT_DIR}/.env.local"
    load_env_file "${ROOT_DIR}/.env.dev"
    load_env_file "${ROOT_DIR}/mock.env"
    load_env_file "${ROOT_DIR}/mock.env.local"

    if [[ "$target_mock_mode" == "true" ]] && use_isolated_mock_data_dir; then
        data_dir="$MOCK_DATA_DIR"
    fi

    export PULSE_DATA_DIR="$data_dir"
    mkdir -p "$PULSE_DATA_DIR"

    if [[ "$PULSE_DATA_DIR" == "$MOCK_DATA_DIR" ]]; then
        log_warn "Using isolated mock data dir (${MOCK_DATA_DIR}); production metrics history will pause while mock mode is on"
    else
        ensure_dev_key
        if [[ -z "${PULSE_ENCRYPTION_KEY:-}" ]]; then
            export PULSE_ENCRYPTION_KEY="$(<"$DEV_KEY_FILE")"
        fi
    fi
    # Keep audit persistence inside the selected runtime data directory so
    # local macOS runs don't attempt to write /var/lib/pulse.
    export PULSE_AUDIT_DIR="${PULSE_AUDIT_DIR:-$PULSE_DATA_DIR}"

    export PORT="${PORT:-$DEFAULT_PORT}"
    export FRONTEND_PORT="${FRONTEND_PORT:-$PORT}"

    pid="$(launch_detached "${STANDALONE_LOG}" "${ROOT_DIR}/pulse")"
    sleep 2

    if kill -0 "$pid" 2>/dev/null; then
        if ! wait_for_port "${PORT}" "${startup_timeout}"; then
            log_error "Pulse process started but port ${PORT} did not become ready within ${startup_timeout}s"
            return 1
        fi
        log_success "Pulse started (pid: ${pid}, port: ${PORT}, log: ${STANDALONE_LOG})"
    else
        log_error "Pulse failed to start. Check ${STANDALONE_LOG}"
        return 1
    fi
}

start_systemd_runtime() {
    local mode="$1"

    case "$mode" in
        systemd-hot-dev)
            log_info "Starting systemd service: pulse-hot-dev"
            sudo systemctl start pulse-hot-dev
            ;;
        systemd-pulse)
            log_info "Starting systemd service: pulse"
            sudo systemctl start pulse
            ;;
    esac
}

apply_mode() {
    local target_mode="$1"
    local runtime_mode
    local start_mode
    local allow_fallback="${PULSE_TOGGLE_ALLOW_STANDALONE_FALLBACK:-false}"

    runtime_mode="$(detect_runtime_mode)"
    start_mode="${runtime_mode}"

    log_info "Detected runtime mode: ${runtime_mode}"
    if [[ "$target_mode" == "true" ]]; then
        log_info "Switching to mock mode"
        if use_isolated_mock_data_dir; then
            mkdir -p "$MOCK_DATA_DIR"
        fi
    else
        log_info "Switching to production-node mode"
        sync_production_config
    fi

    set_mock_mode "$target_mode"

    case "$runtime_mode" in
        systemd-hot-dev|systemd-pulse)
            stop_systemd_runtime "$runtime_mode"
            ;;
        hot-dev)
            stop_hot_dev_runtime
            ;;
        standalone)
            stop_standalone_runtime "false"
            ;;
        pulse-name)
            stop_standalone_runtime "true"
            ;;
        none)
            stop_standalone_runtime "false"
            ;;
    esac

    # macOS default: keep development on hot-dev so frontend changes at 127.0.0.1:5173
    # are always reflected immediately after toggling.
    if [[ "${runtime_mode}" == "none" ]] || [[ "${runtime_mode}" == "standalone" ]] || [[ "${runtime_mode}" == "pulse-name" ]]; then
        if prefer_hot_dev_runtime; then
            start_mode="hot-dev"
        fi
    fi

    # hot-dev already handles build/reload on startup; avoid duplicate builds here for fast toggles.
    if [[ "$start_mode" == "hot-dev" ]] || [[ "$start_mode" == "systemd-hot-dev" ]]; then
        log_info "Skipping pre-rebuild for hot-dev runtime (managed by hot-dev startup)"
    else
        rebuild_frontend_backend
    fi

    case "$start_mode" in
        systemd-hot-dev|systemd-pulse)
            start_systemd_runtime "$start_mode"
            ;;
        hot-dev)
            if ! start_hot_dev_runtime; then
                if [[ "${allow_fallback}" == "true" ]]; then
                    log_warn "hot-dev restart failed or is unhealthy; falling back to standalone backend on port ${DEFAULT_PORT}"
                    stop_hot_dev_runtime || true
                    start_standalone_runtime "$target_mode"
                    start_mode="standalone"
                else
                    log_error "hot-dev restart failed; refusing to fallback so frontend dev state stays explicit"
                    return 1
                fi
            fi
            ;;
        standalone|pulse-name|none)
            start_standalone_runtime "$target_mode"
            ;;
    esac

    if [[ "$start_mode" == "hot-dev" ]]; then
        if ! verify_mock_state_via_frontend_proxy "$target_mode"; then
            log_error "Frontend proxy at 127.0.0.1:5173 did not report expected mock state (${target_mode})"
            return 1
        fi
    fi

    if [[ "$target_mode" == "true" ]]; then
        log_success "Mock mode is now enabled"
    else
        log_success "Mock mode is now disabled (real nodes mode)"
    fi
}

show_status() {
    local runtime_mode
    runtime_mode="$(detect_runtime_mode)"

    ensure_mock_env_file
    set +u
    # shellcheck disable=SC1090
    source "$MOCK_ENV_FILE"
    set -u

    echo "Mock Data Mode Control for Pulse"
    echo ""
    local status_data_dir="$DEV_DATA_DIR"
    if [[ "${PULSE_MOCK_MODE:-false}" == "true" ]] && use_isolated_mock_data_dir; then
        status_data_dir="$MOCK_DATA_DIR"
    fi
    if [[ "${PULSE_MOCK_MODE:-false}" == "true" ]]; then
        echo -e "${GREEN}Mock Mode: ENABLED${NC}"
        echo "  Runtime: ${runtime_mode}"
        echo "  Nodes: ${PULSE_MOCK_NODES:-0}"
        echo "  VMs per node: ${PULSE_MOCK_VMS_PER_NODE:-0}"
        echo "  LXCs per node: ${PULSE_MOCK_LXCS_PER_NODE:-0}"
        echo "  Generic hosts: ${PULSE_MOCK_GENERIC_HOSTS:-0}"
        echo "  Docker hosts: ${PULSE_MOCK_DOCKER_HOSTS:-0}"
        echo "  Docker containers/host: ${PULSE_MOCK_DOCKER_CONTAINERS:-0}"
        echo "  K8s clusters: ${PULSE_MOCK_K8S_CLUSTERS:-0}"
        echo "  Data dir: ${status_data_dir}"
    else
        echo -e "${BLUE}Mock Mode: DISABLED${NC}"
        echo "  Runtime: ${runtime_mode}"
        echo "  Data dir: ${status_data_dir}"
    fi
}

edit_config() {
    ensure_mock_env_file
    log_info "Opening ${MOCK_ENV_FILE}"
    ${EDITOR:-nano} "$MOCK_ENV_FILE"
}

sync_config_only() {
    sync_production_config

    if [[ "$(detect_runtime_mode)" != "none" ]]; then
        log_info "Runtime detected; restarting current mode to apply synced config"
        apply_mode "$(get_mock_mode)"
    else
        log_info "No active runtime detected; sync complete"
    fi
}

usage() {
    echo "Mock Data Mode Control for Pulse"
    echo ""
    echo "Usage: $0 [toggle|on|off|status|edit|sync]"
    echo ""
    echo "  toggle  - Toggle between mock mode and production-node mode (default)"
    echo "  on      - Enable mock mode and restart/rebuild stack"
    echo "  off     - Disable mock mode, sync real nodes config, restart/rebuild stack"
    echo "  status  - Show current mock mode + runtime status"
    echo "  edit    - Edit mock.env"
    echo "  sync    - Sync production config to dev config and restart if running"
}

main() {
    local cmd="${1:-toggle}"
    local needs_lock=false

    case "$cmd" in
        on|enable)
            needs_lock=true
            ;;
        off|disable)
            needs_lock=true
            ;;
        toggle)
            needs_lock=true
            ;;
        sync)
            needs_lock=true
            ;;
    esac

    if [[ "$needs_lock" == "true" ]]; then
        acquire_lock
    fi

    case "$cmd" in
        on|enable)
            apply_mode "true"
            ;;
        off|disable)
            apply_mode "false"
            ;;
        toggle)
            if [[ "$(get_mock_mode)" == "true" ]]; then
                apply_mode "false"
            else
                apply_mode "true"
            fi
            ;;
        status)
            show_status
            ;;
        edit)
            edit_config
            ;;
        sync)
            sync_config_only
            ;;
        -h|--help|help)
            usage
            ;;
        *)
            usage
            echo ""
            log_error "Unknown command: $cmd"
            exit 1
            ;;
    esac
}

main "$@"
