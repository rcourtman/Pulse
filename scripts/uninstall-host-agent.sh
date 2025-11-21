#!/bin/bash
set -e

# Pulse Host Agent Uninstallation Script
# Removes the Pulse host agent and all associated files

# ANSI color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RESET='\033[0m'

# Check if colors are supported
if [ -t 1 ] && command -v tput &> /dev/null && [ "$(tput colors)" -ge 8 ]; then
    USE_COLOR=true
else
    USE_COLOR=false
fi

print_color() {
    local color="$1"
    local message="$2"
    if [ "$USE_COLOR" = true ]; then
        printf "${color}%s${RESET}\n" "$message"
    else
        printf "%s\n" "$message"
    fi
}

log_success() {
    print_color "$GREEN" "✓ $1"
}

log_error() {
    print_color "$RED" "✗ $1" >&2
}

log_info() {
    print_color "$BLUE" "ℹ $1"
}

log_warn() {
    print_color "$YELLOW" "⚠ $1"
}

print_header() {
    echo ""
    print_color "$BLUE" "═══════════════════════════════════════════════════════════"
    print_color "$BLUE" "  Pulse Host Agent - Uninstallation"
    print_color "$BLUE" "═══════════════════════════════════════════════════════════"
    echo ""
}

print_footer() {
    echo ""
    print_color "$GREEN" "═══════════════════════════════════════════════════════════"
    log_success "Uninstallation complete!"
    print_color "$GREEN" "═══════════════════════════════════════════════════════════"
    echo ""
}

# File paths
AGENT_PATH="/usr/local/bin/pulse-host-agent"
SYSTEMD_SERVICE="/etc/systemd/system/pulse-host-agent.service"
LAUNCHD_PLIST="$HOME/Library/LaunchAgents/com.pulse.host-agent.plist"
MACOS_LOG_DIR="$HOME/Library/Logs/Pulse"
LINUX_LOG_DIR="/var/log/pulse"
LINUX_LOG_FILE="$LINUX_LOG_DIR/host-agent.log"

TRUENAS=false
TRUENAS_STATE_DIR="/data/pulse-host-agent"
TRUENAS_LOG_DIR="$TRUENAS_STATE_DIR/logs"
TRUENAS_SERVICE_STORAGE="$TRUENAS_STATE_DIR/pulse-host-agent.service"
TRUENAS_BOOTSTRAP_SCRIPT="$TRUENAS_STATE_DIR/bootstrap-pulse-host-agent.sh"
TRUENAS_ENV_FILE="$TRUENAS_STATE_DIR/pulse-host-agent.env"
TRUENAS_SYSTEMD_LINK="/etc/systemd/system/pulse-host-agent.service"

print_header

# Detect platform
case "$(uname -s)" in
    Linux*)
        PLATFORM="linux"
        ;;
    Darwin*)
        PLATFORM="darwin"
        ;;
    *)
        log_error "Unsupported platform: $(uname -s)"
        exit 1
        ;;
esac

is_truenas_scale() {
    if [[ -f /etc/truenas-version ]]; then
        return 0
    fi
    if [[ -f /etc/version ]] && grep -qi "truenas" /etc/version 2>/dev/null; then
        return 0
    fi
    if [[ -d /data/ix-applications ]] || [[ -d /etc/ix-apps.d ]]; then
        return 0
    fi
    return 1
}

if [[ "$PLATFORM" == "linux" ]] && is_truenas_scale; then
    TRUENAS=true
    AGENT_PATH="$TRUENAS_STATE_DIR/pulse-host-agent"
    SYSTEMD_SERVICE="$TRUENAS_SYSTEMD_LINK"
    LINUX_LOG_DIR="$TRUENAS_LOG_DIR"
    LINUX_LOG_FILE="$LINUX_LOG_DIR/host-agent.log"
fi

log_info "Detected platform: $PLATFORM"
if [[ "$TRUENAS" == true ]]; then
    log_info "TrueNAS SCALE detected (immutable root). Using $TRUENAS_STATE_DIR for cleanup."
fi
echo ""

remove_truenas_init_task() {
    if [[ "$TRUENAS" != true ]]; then
        return
    fi
    if ! command -v midclt >/dev/null 2>&1 || ! command -v python3 >/dev/null 2>&1; then
        log_warn "midclt/python3 not available - remove the POSTINIT task for $TRUENAS_BOOTSTRAP_SCRIPT manually if it exists."
        return
    fi

    local query_output task_id
    query_output=$(midclt call initshutdownscript.query '[["script","=","'"$TRUENAS_BOOTSTRAP_SCRIPT"'"]]' 2>/dev/null || true)
    task_id=$(printf '%s' "$query_output" | python3 - <<'PY'
import json, sys
try:
    data = json.load(sys.stdin)
    print(data[0]["id"] if data else "")
except Exception:
    print("")
PY
)

    if [[ -n "$task_id" ]]; then
        if midclt call initshutdownscript.delete "$task_id" >/dev/null 2>&1; then
            log_success "Removed TrueNAS Init/Shutdown task (id $task_id)"
        else
            log_warn "Failed to remove TrueNAS Init/Shutdown task id $task_id; remove it manually in the TrueNAS UI."
        fi
    fi
}

# Stop and remove systemd service (Linux)
if [[ "$PLATFORM" == "linux" ]]; then
    if [[ -f "$SYSTEMD_SERVICE" ]] && command -v systemctl &> /dev/null; then
        log_info "Stopping systemd service..."
        if sudo systemctl stop pulse-host-agent 2>/dev/null; then
            log_success "Service stopped"
        else
            log_warn "Service was not running or already stopped"
        fi

        log_info "Disabling systemd service..."
        sudo systemctl disable pulse-host-agent 2>/dev/null || true

        log_info "Removing systemd service file..."
        sudo rm -f "$SYSTEMD_SERVICE"
        sudo systemctl daemon-reload
        log_success "Systemd service removed"
    else
        log_info "Systemd service not found (already removed or never installed)"
    fi

    # Ensure process is terminated
    if pgrep -x "pulse-host-agent" > /dev/null; then
        log_info "Terminating running processes..."
        sudo pkill -9 "pulse-host-agent" 2>/dev/null || true
        sleep 1
        log_success "Processes terminated"
    fi

    if [[ "$TRUENAS" == true ]]; then
        remove_truenas_init_task

        if [[ -f "$TRUENAS_SERVICE_STORAGE" ]]; then
            log_info "Removing stored TrueNAS service unit..."
            sudo rm -f "$TRUENAS_SERVICE_STORAGE"
        fi
        if [[ -f "$TRUENAS_BOOTSTRAP_SCRIPT" ]]; then
            log_info "Removing TrueNAS bootstrap script..."
            sudo rm -f "$TRUENAS_BOOTSTRAP_SCRIPT"
        fi
        if [[ -f "$TRUENAS_ENV_FILE" ]]; then
            log_info "Removing TrueNAS environment file..."
            sudo rm -f "$TRUENAS_ENV_FILE"
        fi
    fi

    # Remove log directory
    if [[ -d "$LINUX_LOG_DIR" ]]; then
        log_info "Removing log directory..."
        sudo rm -rf "$LINUX_LOG_DIR"
        log_success "Log directory removed: $LINUX_LOG_DIR"
    fi

    if [[ "$TRUENAS" == true ]] && [[ -d "$TRUENAS_STATE_DIR" ]]; then
        log_info "Removing persistent state directory..."
        sudo rm -rf "$TRUENAS_STATE_DIR"
        log_success "Removed $TRUENAS_STATE_DIR"
    fi
fi

# Stop and remove launchd service (macOS)
if [[ "$PLATFORM" == "darwin" ]]; then
    if [[ -f "$LAUNCHD_PLIST" ]] && command -v launchctl &> /dev/null; then
        log_info "Unloading launchd service..."
        if launchctl unload "$LAUNCHD_PLIST" 2>/dev/null; then
            log_success "Service unloaded"
        else
            log_warn "Service was not loaded or already unloaded"
        fi

        log_info "Removing launchd plist..."
        rm -f "$LAUNCHD_PLIST"
        log_success "Launchd service removed"
    else
        log_info "Launchd service not found (already removed or never installed)"
    fi

    # Remove Keychain token
    log_info "Removing token from macOS Keychain..."
    if security delete-generic-password -s "pulse-host-agent" -a "$USER" 2>/dev/null; then
        log_success "Token removed from Keychain"
    else
        log_info "Token not found in Keychain (may not have been stored)"
    fi

    # Remove wrapper script
    WRAPPER_SCRIPT="/usr/local/bin/pulse-host-agent-wrapper.sh"
    if [[ -f "$WRAPPER_SCRIPT" ]]; then
        log_info "Removing wrapper script..."
        rm -f "$WRAPPER_SCRIPT"
        log_success "Wrapper script removed"
    fi

    # Ensure process is terminated
    if pgrep -x "pulse-host-agent" > /dev/null; then
        log_info "Terminating running processes..."
        pkill -9 "pulse-host-agent" 2>/dev/null || true
        sleep 1
        log_success "Processes terminated"
    fi

    # Remove log directory
    if [[ -d "$MACOS_LOG_DIR" ]]; then
        log_info "Removing log directory..."
        rm -rf "$MACOS_LOG_DIR"
        log_success "Log directory removed: $MACOS_LOG_DIR"
    fi

    # Remove temporary logs (old location)
    if [[ -f "/tmp/pulse-host-agent.log" ]]; then
        log_info "Removing temporary log file..."
        rm -f /tmp/pulse-host-agent.log
        log_success "Temporary log removed"
    fi
fi

# Remove binary
if [[ -f "$AGENT_PATH" ]]; then
    log_info "Removing agent binary..."
    if sudo rm -f "$AGENT_PATH" 2>/dev/null; then
        log_success "Agent binary removed: $AGENT_PATH"
    else
        log_error "Failed to remove agent binary (permission denied?)"
        log_info "Try running with sudo: sudo $0"
    fi
else
    log_info "Agent binary not found: $AGENT_PATH"
fi

print_footer

log_info "The Pulse Host Agent has been removed from this system."
log_info "This host will no longer appear in your Pulse dashboard."
echo ""
