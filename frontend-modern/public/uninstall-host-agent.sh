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

log_info "Detected platform: $PLATFORM"
echo ""

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

    # Remove log directory
    if [[ -d "$LINUX_LOG_DIR" ]]; then
        log_info "Removing log directory..."
        sudo rm -rf "$LINUX_LOG_DIR"
        log_success "Log directory removed: $LINUX_LOG_DIR"
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
