#!/bin/bash
# Quick dev environment health check.
#
# Usage:
#   ./scripts/dev-check.sh          # Check and report
#   ./scripts/dev-check.sh --kill   # Stop managed runtime, then clean up residual legacy processes

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
HOT_DEV_BG_PATH="${HOT_DEV_BG_PATH:-${SCRIPT_DIR}/hot-dev-bg.sh}"
HOT_DEV_BG_LOG_FILE="${HOT_DEV_BG_LOG_FILE:-${ROOT_DIR}/tmp/hot-dev.bg.log}"
FRONTEND_URL="${PULSE_DEV_CHECK_FRONTEND_URL:-http://127.0.0.1:5173}"
BACKEND_URL="${PULSE_DEV_CHECK_BACKEND_URL:-http://127.0.0.1:7655}"
SKIP_AUXILIARY_CHECKS="${PULSE_DEV_CHECK_SKIP_AUXILIARY_CHECKS:-false}"

managed_status() {
    if [[ -x "${HOT_DEV_BG_PATH}" ]]; then
        "${HOT_DEV_BG_PATH}" status 2>&1 || true
        return
    fi

    echo "[hot-dev-bg] Not available at ${HOT_DEV_BG_PATH}"
}

show_managed_guidance() {
    local status_output="$1"

    if [[ "${status_output}" == *"Runtime summary: frontend shell, proxy, and backend are healthy."* ]]; then
        echo "Managed guidance: use ${FRONTEND_URL} in the browser."
        return
    fi

    if [[ "${status_output}" == *"[hot-dev-bg] Not running"* ]]; then
        echo "Managed guidance: cd ${ROOT_DIR} && npm run dev"
        return
    fi

    echo "Managed guidance: cd ${ROOT_DIR} && npm run dev:restart"
}

stop_managed_runtime() {
    if [[ -x "${HOT_DEV_BG_PATH}" ]]; then
        "${HOT_DEV_BG_PATH}" stop >/dev/null 2>&1 || true
    fi
}

kill_legacy_processes() {
    pkill -9 -f "bin/pulse$" 2>/dev/null || true
    pkill -9 -f "^\./pulse$" 2>/dev/null || true
    pkill -f "node.*vite" 2>/dev/null || true
    pkill -f "watch-snapshot.sh" 2>/dev/null || true
}

show_ai_status() {
    local frontend_ai_url backend_ai_url ai_response ai_status
    frontend_ai_url="${FRONTEND_URL}/api/ai/status"
    backend_ai_url="${BACKEND_URL}/api/ai/status"

    echo -n "AI service: "
    ai_response="$(curl -s -u admin:admin "${frontend_ai_url}" 2>/dev/null || curl -s -u admin:admin "${backend_ai_url}" 2>/dev/null || true)"
    if [[ -n "${ai_response}" ]] && command -v jq >/dev/null 2>&1; then
        ai_status="$(printf '%s' "${ai_response}" | jq -r '.running // false' 2>/dev/null || echo "false")"
    else
        ai_status="false"
    fi

    if [[ "${ai_status}" == "true" ]]; then
        echo -e "${GREEN}✓ Running (direct integration)${NC}"
    else
        echo -e "${YELLOW}⚠ Not running (enable in settings)${NC}"
    fi
}

show_snapshot_status() {
    local snapshot_pid snapshot_count
    echo -n "Snapshot watcher: "
    snapshot_pid="$(pgrep -f "watch-snapshot.sh" 2>/dev/null | head -1 || true)"
    if [[ -n "${snapshot_pid}" ]]; then
        snapshot_count="$(git -C ~/.pulse-snapshots rev-list --count HEAD 2>/dev/null || echo 0)"
        echo -e "${GREEN}✓ Running (PID: ${snapshot_pid}, ${snapshot_count} snapshots)${NC}"
    else
        echo -e "${YELLOW}⚠ Not running (optional - protects against accidental file loss)${NC}"
        echo "   Start: ./scripts/watch-snapshot.sh &"
    fi
}

show_recent_errors() {
    echo ""
    echo "=== Recent Pulse Errors (last 5) ==="

    if [[ -f "${HOT_DEV_BG_LOG_FILE}" ]]; then
        grep -i "error\|fatal\|panic" "${HOT_DEV_BG_LOG_FILE}" 2>/dev/null | tail -5 || echo "None found"
        return
    fi

    if [[ "$(uname -s)" == "Darwin" ]]; then
        grep -i "error\|fatal\|panic" /tmp/pulse-debug.log 2>/dev/null | tail -5 || echo "None found"
    else
        journalctl -u pulse-dev --no-pager -n 20 2>/dev/null | grep -i "error\|fatal\|panic" | tail -5 || echo "None found"
    fi
}

main() {
    local status_output

    if [[ "${1:-}" == "--kill" ]]; then
        echo "Stopping managed dev runtime and cleaning residual legacy processes..."
        stop_managed_runtime
        kill_legacy_processes
        sleep 2
        echo -e "${GREEN}✓${NC} Managed runtime stop requested and residual legacy processes cleaned up"
        exit 0
    fi

    echo "=== Pulse Dev Environment Check ==="
    echo ""
    echo "=== Managed Runtime Status ==="
    status_output="$(managed_status)"
    printf '%s\n' "${status_output}"
    echo ""
    show_managed_guidance "${status_output}"

    if [[ "${SKIP_AUXILIARY_CHECKS}" == "true" ]]; then
        echo ""
        echo "Done."
        exit 0
    fi

    echo ""
    show_ai_status
    show_snapshot_status
    show_recent_errors

    echo ""
    echo "Done."
}

main "$@"
