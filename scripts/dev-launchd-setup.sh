#!/usr/bin/env bash
# dev-launchd-setup.sh - Install or uninstall the Pulse dev Launch Agent
#
# Usage:
#   ./scripts/dev-launchd-setup.sh           # Install and start
#   ./scripts/dev-launchd-setup.sh uninstall # Stop and remove

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
LABEL="com.pulse.hot-dev"
PLIST_SRC="${SCRIPT_DIR}/com.pulse.hot-dev.plist"
PLIST_DST="$HOME/Library/LaunchAgents/${LABEL}.plist"
LOG_DIR="$HOME/Library/Logs/Pulse"
GUI_DOMAIN="gui/$(id -u)"

log_info() { printf "\033[0;34m[launchd-setup]\033[0m %s\n" "$1"; }
log_error() { printf "\033[0;31m[launchd-setup] ERROR:\033[0m %s\n" "$1"; }
log_ok() { printf "\033[0;32m[launchd-setup] ✓\033[0m %s\n" "$1"; }

uninstall() {
    log_info "Uninstalling ${LABEL}..."

    # Bootout (stop + unload) — ignore errors if not loaded
    launchctl bootout "${GUI_DOMAIN}/${LABEL}" 2>/dev/null && \
        log_ok "Service stopped and unloaded" || \
        log_info "Service was not loaded (already stopped)"

    if [[ -L "${PLIST_DST}" ]] || [[ -f "${PLIST_DST}" ]]; then
        rm -f "${PLIST_DST}"
        log_ok "Removed ${PLIST_DST}"
    else
        log_info "Plist not found at ${PLIST_DST} (already removed)"
    fi

    log_ok "Uninstall complete. Log files preserved at ${LOG_DIR}"
}

install() {
    if [[ ! -f "${PLIST_SRC}" ]]; then
        log_error "Plist template not found: ${PLIST_SRC}"
        exit 1
    fi

    # Make wrapper executable
    chmod +x "${SCRIPT_DIR}/dev-launchd-wrapper.sh"

    # Create log directory
    mkdir -p "${LOG_DIR}"
    log_ok "Log directory: ${LOG_DIR}"

    # Stop existing service if running
    launchctl bootout "${GUI_DOMAIN}/${LABEL}" 2>/dev/null && \
        log_info "Stopped existing service" || true

    # Symlink plist into LaunchAgents
    mkdir -p "$HOME/Library/LaunchAgents"
    ln -sf "${PLIST_SRC}" "${PLIST_DST}"
    log_ok "Symlinked plist to ${PLIST_DST}"

    # Bootstrap (load + start)
    launchctl bootstrap "${GUI_DOMAIN}" "${PLIST_DST}"
    log_ok "Service bootstrapped"

    # Give it a moment to start
    sleep 3

    # Verify
    if launchctl print "${GUI_DOMAIN}/${LABEL}" 2>/dev/null | grep -q "pid ="; then
        PID=$(launchctl print "${GUI_DOMAIN}/${LABEL}" 2>/dev/null | grep "pid =" | awk '{print $NF}')
        log_ok "Service is running (PID: ${PID})"
    else
        log_error "Service may not have started. Check logs:"
        log_error "  tail -f ${LOG_DIR}/hot-dev.stderr.log"
        exit 1
    fi

    echo ""
    log_info "=== Pulse Dev Launch Agent Installed ==="
    log_info ""
    log_info "The dev environment will now auto-start on login and auto-restart on crash."
    log_info ""
    log_info "Useful commands:"
    log_info "  Restart:  launchctl kickstart -k ${GUI_DOMAIN}/${LABEL}"
    log_info "  Stop:     launchctl kill SIGTERM ${GUI_DOMAIN}/${LABEL}"
    log_info "  Status:   launchctl print ${GUI_DOMAIN}/${LABEL}"
    log_info "  Logs:     tail -f ${LOG_DIR}/hot-dev.stderr.log"
    log_info "  Disable:  launchctl bootout ${GUI_DOMAIN}/${LABEL}"
    log_info "  Remove:   $0 uninstall"
}

case "${1:-install}" in
    uninstall|remove)
        uninstall
        ;;
    install|"")
        install
        ;;
    *)
        echo "Usage: $0 [install|uninstall]"
        exit 1
        ;;
esac
