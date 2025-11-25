#!/usr/bin/env bash
#
# Pulse Unified Agent Installer
# Supports: Linux (systemd), macOS (launchd), Synology (upstart)
#
# Usage:
#   curl -fsSL http://pulse/install.sh | bash -s -- --url http://pulse --token <token> [options]
#
# Options:
#   --enable-host       Enable host metrics (default: true)
#   --enable-docker     Enable docker metrics (default: false)
#   --interval <dur>    Reporting interval (default: 30s)
#   --uninstall         Remove the agent

set -euo pipefail

# --- Configuration ---
AGENT_NAME="pulse-agent"
BINARY_NAME="pulse-agent"
INSTALL_DIR="/usr/local/bin"
LOG_FILE="/var/log/${AGENT_NAME}.log"

# Defaults
PULSE_URL=""
PULSE_TOKEN=""
INTERVAL="30s"
ENABLE_HOST="true"
ENABLE_DOCKER="false"
UNINSTALL="false"
INSECURE="false"

# --- Helper Functions ---
log_info() { printf "[INFO] %s\n" "$1"; }
log_warn() { printf "[WARN] %s\n" "$1"; }
log_error() { printf "[ERROR] %s\n" "$1"; }
fail() { 
    log_error "$1"
    if [[ -t 0 ]]; then
        read -p "Press Enter to exit..."
    elif [[ -e /dev/tty ]]; then
        read -p "Press Enter to exit..." < /dev/tty
    fi
    exit 1
}

# --- Parse Arguments ---
while [[ $# -gt 0 ]]; do
    case $1 in
        --url) PULSE_URL="$2"; shift 2 ;;
        --token) PULSE_TOKEN="$2"; shift 2 ;;
        --interval) INTERVAL="$2"; shift 2 ;;
        --enable-host) ENABLE_HOST="true"; shift ;;
        --disable-host) ENABLE_HOST="false"; shift ;;
        --enable-docker) ENABLE_DOCKER="true"; shift ;;
        --disable-docker) ENABLE_DOCKER="false"; shift ;;
        --insecure) INSECURE="true"; shift ;;
        --uninstall) UNINSTALL="true"; shift ;;
        *) fail "Unknown argument: $1" ;;
    esac
done

# --- Uninstall Logic ---
if [[ "$UNINSTALL" == "true" ]]; then
    log_info "Uninstalling ${AGENT_NAME}..."
    
    # Systemd
    if command -v systemctl >/dev/null 2>&1; then
        systemctl stop "${AGENT_NAME}" 2>/dev/null || true
        systemctl disable "${AGENT_NAME}" 2>/dev/null || true
        rm -f "/etc/systemd/system/${AGENT_NAME}.service"
        systemctl daemon-reload 2>/dev/null || true
    fi

    # Launchd (macOS)
    if [[ "$(uname -s)" == "Darwin" ]]; then
        PLIST="/Library/LaunchDaemons/com.pulse.agent.plist"
        launchctl unload "$PLIST" 2>/dev/null || true
        rm -f "$PLIST"
    fi

    # Upstart (Synology)
    if [[ -f "/etc/init/${AGENT_NAME}.conf" ]]; then
        initctl stop "${AGENT_NAME}" 2>/dev/null || true
        rm -f "/etc/init/${AGENT_NAME}.conf"
    fi

    rm -f "${INSTALL_DIR}/${BINARY_NAME}"
    log_info "Uninstallation complete."
    exit 0
fi

# --- Validation ---
if [[ -z "$PULSE_URL" || -z "$PULSE_TOKEN" ]]; then
    fail "Missing required arguments: --url and --token"
fi

# --- Download ---
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) fail "Unsupported architecture: $ARCH" ;;
esac

DOWNLOAD_URL="${PULSE_URL}/download/${BINARY_NAME}?os=${OS}&arch=${ARCH}"
log_info "Downloading agent from ${DOWNLOAD_URL}..."

# Create temp file
TMP_BIN=$(mktemp)
chmod +x "$TMP_BIN"

CURL_ARGS="-fsSL"
if [[ "$INSECURE" == "true" ]]; then CURL_ARGS="-k $CURL_ARGS"; fi

if ! curl $CURL_ARGS -o "$TMP_BIN" "$DOWNLOAD_URL"; then
    fail "Download failed. Check URL and connectivity."
fi

# Install Binary
log_info "Installing binary to ${INSTALL_DIR}/${BINARY_NAME}..."
mkdir -p "$INSTALL_DIR"
mv "$TMP_BIN" "${INSTALL_DIR}/${BINARY_NAME}"

# --- Service Installation ---

# 1. macOS (Launchd)
if [[ "$OS" == "darwin" ]]; then
    PLIST="/Library/LaunchDaemons/com.pulse.agent.plist"
    log_info "Configuring Launchd service at $PLIST..."

    cat > "$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.pulse.agent</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/${BINARY_NAME}</string>
        <string>--url</string>
        <string>${PULSE_URL}</string>
        <string>--token</string>
        <string>${PULSE_TOKEN}</string>
        <string>--interval</string>
        <string>${INTERVAL}</string>
        <string>--enable-host=${ENABLE_HOST}</string>
        <string>--enable-docker=${ENABLE_DOCKER}</string>
        <string>--insecure=${INSECURE}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${LOG_FILE}</string>
    <key>StandardErrorPath</key>
    <string>${LOG_FILE}</string>
</dict>
</plist>
EOF
    chmod 644 "$PLIST"
    launchctl unload "$PLIST" 2>/dev/null || true
    launchctl load -w "$PLIST"
    log_info "Service started."
    exit 0
fi

# 2. Synology (Upstart)
if [[ -d /usr/syno/etc/rc.sysv ]]; then
    CONF="/etc/init/${AGENT_NAME}.conf"
    log_info "Configuring Upstart service at $CONF..."

    cat > "$CONF" <<EOF
description "Pulse Unified Agent"
author "Pulse"

start on syno.network.ready
stop on runlevel [06]

respawn
respawn limit 5 10

exec ${INSTALL_DIR}/${BINARY_NAME} \
    --url "${PULSE_URL}" \
    --token "${PULSE_TOKEN}" \
    --interval "${INTERVAL}" \
    --enable-host=${ENABLE_HOST} \
    --enable-docker=${ENABLE_DOCKER} \
    --insecure=${INSECURE} \
    >> ${LOG_FILE} 2>&1
EOF
    initctl stop "${AGENT_NAME}" 2>/dev/null || true
    initctl start "${AGENT_NAME}"
    log_info "Service started."
    exit 0
fi

# 3. Linux (Systemd)
if command -v systemctl >/dev/null 2>&1; then
    UNIT="/etc/systemd/system/${AGENT_NAME}.service"
    log_info "Configuring Systemd service at $UNIT..."

    cat > "$UNIT" <<EOF
[Unit]
Description=Pulse Unified Agent
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${BINARY_NAME} \
    --url "${PULSE_URL}" \
    --token "${PULSE_TOKEN}" \
    --interval "${INTERVAL}" \
    --enable-host=${ENABLE_HOST} \
    --enable-docker=${ENABLE_DOCKER} \
    --insecure=${INSECURE}
Restart=always
RestartSec=5s
User=root

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable "${AGENT_NAME}"
    systemctl restart "${AGENT_NAME}"
    log_info "Service started."
    exit 0
fi

fail "Could not detect a supported service manager (systemd, launchd, upstart)."
