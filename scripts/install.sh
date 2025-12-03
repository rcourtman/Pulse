#!/usr/bin/env bash
#
# Pulse Unified Agent Installer
# Supports: Linux (systemd, OpenRC), macOS (launchd), Synology DSM (6.x/7+), Unraid
#
# Usage:
#   curl -fsSL http://pulse/install.sh | bash -s -- --url http://pulse --token <token> [options]
#
# Options:
#   --enable-host       Enable host metrics (default: true)
#   --enable-docker     Enable docker metrics (default: false)
#   --interval <dur>    Reporting interval (default: 30s)
#   --agent-id <id>     Custom agent identifier (default: auto-generated)
#   --uninstall         Remove the agent

set -euo pipefail

# Wrap entire script in a function to protect against partial download
# See: https://www.kicksecure.com/wiki/Dev/curl_bash_pipe
main() {

# --- Cleanup trap ---
TMP_FILES=()
# shellcheck disable=SC2317  # Invoked by trap, not directly
cleanup() {
    for f in "${TMP_FILES[@]}"; do
        rm -f "$f" 2>/dev/null || true
    done
}
trap cleanup EXIT

# --- Check Root ---
if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root. Please use sudo." 
   exit 1
fi

# --- Configuration ---
AGENT_NAME="pulse-agent"
BINARY_NAME="pulse-agent"
INSTALL_DIR="/usr/local/bin"
LOG_FILE="/var/log/${AGENT_NAME}.log"

# TrueNAS SCALE configuration (immutable root filesystem)
TRUENAS=false
TRUENAS_STATE_DIR="/data/pulse-agent"
TRUENAS_LOG_DIR="$TRUENAS_STATE_DIR/logs"
TRUENAS_BOOTSTRAP_SCRIPT="$TRUENAS_STATE_DIR/bootstrap-pulse-agent.sh"
TRUENAS_ENV_FILE="$TRUENAS_STATE_DIR/pulse-agent.env"

# Defaults
PULSE_URL=""
PULSE_TOKEN=""
INTERVAL="30s"
ENABLE_HOST="true"
ENABLE_DOCKER="false"
UNINSTALL="false"
INSECURE="false"
AGENT_ID=""

# --- Helper Functions ---
log_info() { printf "[INFO] %s\n" "$1"; }
log_warn() { printf "[WARN] %s\n" "$1"; }
log_error() { printf "[ERROR] %s\n" "$1"; }
fail() {
    log_error "$1"
    if [[ -t 0 ]]; then
        read -r -p "Press Enter to exit..."
    elif [[ -e /dev/tty ]]; then
        read -r -p "Press Enter to exit..." < /dev/tty
    fi
    exit 1
}

# Build exec args string for use in service files
# Returns via EXEC_ARGS variable
build_exec_args() {
    EXEC_ARGS="--url ${PULSE_URL} --token ${PULSE_TOKEN} --interval ${INTERVAL}"
    # Always pass enable-host flag since agent defaults to true
    if [[ "$ENABLE_HOST" == "true" ]]; then
        EXEC_ARGS="$EXEC_ARGS --enable-host"
    else
        EXEC_ARGS="$EXEC_ARGS --enable-host=false"
    fi
    if [[ "$ENABLE_DOCKER" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --enable-docker"; fi
    if [[ "$INSECURE" == "true" ]]; then EXEC_ARGS="$EXEC_ARGS --insecure"; fi
    if [[ -n "$AGENT_ID" ]]; then EXEC_ARGS="$EXEC_ARGS --agent-id ${AGENT_ID}"; fi
}

# Build exec args as array for direct execution (proper quoting)
# Returns via EXEC_ARGS_ARRAY variable
build_exec_args_array() {
    EXEC_ARGS_ARRAY=(--url "$PULSE_URL" --token "$PULSE_TOKEN" --interval "$INTERVAL")
    # Always pass enable-host flag since agent defaults to true
    if [[ "$ENABLE_HOST" == "true" ]]; then
        EXEC_ARGS_ARRAY+=(--enable-host)
    else
        EXEC_ARGS_ARRAY+=(--enable-host=false)
    fi
    if [[ "$ENABLE_DOCKER" == "true" ]]; then EXEC_ARGS_ARRAY+=(--enable-docker); fi
    if [[ "$INSECURE" == "true" ]]; then EXEC_ARGS_ARRAY+=(--insecure); fi
    if [[ -n "$AGENT_ID" ]]; then EXEC_ARGS_ARRAY+=(--agent-id "$AGENT_ID"); fi
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
        --agent-id) AGENT_ID="$2"; shift 2 ;;
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

    # Synology DSM (handles both DSM 7+ systemd and DSM 6.x upstart)
    if [[ -d /usr/syno ]]; then
        # DSM 7+ uses systemd
        if [[ -f "/etc/systemd/system/${AGENT_NAME}.service" ]]; then
            systemctl stop "${AGENT_NAME}" 2>/dev/null || true
            systemctl disable "${AGENT_NAME}" 2>/dev/null || true
            rm -f "/etc/systemd/system/${AGENT_NAME}.service"
            systemctl daemon-reload 2>/dev/null || true
        fi
        # DSM 6.x uses upstart
        if [[ -f "/etc/init/${AGENT_NAME}.conf" ]]; then
            initctl stop "${AGENT_NAME}" 2>/dev/null || true
            rm -f "/etc/init/${AGENT_NAME}.conf"
        fi
    fi

    # Unraid
    if [[ -f /etc/unraid-version ]] || [[ -d /boot/config/plugins/pulse-agent ]]; then
        log_info "Removing Unraid installation..."
        # Stop running agent
        pkill -f "pulse-agent" 2>/dev/null || true
        # Remove from /boot/config/go
        GO_SCRIPT="/boot/config/go"
        if [[ -f "$GO_SCRIPT" ]]; then
            sed -i '/# Pulse Agent/,/^$/d' "$GO_SCRIPT" 2>/dev/null || true
            sed -i '/pulse-agent/d' "$GO_SCRIPT" 2>/dev/null || true
        fi
        # Remove installation directory
        rm -rf /boot/config/plugins/pulse-agent
        # Remove symlink
        rm -f "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    # TrueNAS SCALE
    if [[ -d "$TRUENAS_STATE_DIR" ]] || [[ -f /etc/truenas-version ]]; then
        log_info "Removing TrueNAS SCALE installation..."
        # Stop and disable service
        systemctl stop "${AGENT_NAME}" 2>/dev/null || true
        systemctl disable "${AGENT_NAME}" 2>/dev/null || true
        # Remove systemd symlink
        rm -f "/etc/systemd/system/${AGENT_NAME}.service"
        systemctl daemon-reload 2>/dev/null || true
        # Remove Init/Shutdown task
        if command -v midclt >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
            TASK_ID=$(midclt call initshutdownscript.query '[["script","=","'"$TRUENAS_BOOTSTRAP_SCRIPT"'"]]' 2>/dev/null | python3 -c "import json,sys; d=json.load(sys.stdin); print(d[0]['id'] if d else '')" 2>/dev/null || echo "")
            if [[ -n "$TASK_ID" ]]; then
                midclt call initshutdownscript.delete "$TASK_ID" >/dev/null 2>&1 || log_warn "Failed to remove Init/Shutdown task (id $TASK_ID)"
            fi
        fi
        # Remove state directory
        rm -rf "$TRUENAS_STATE_DIR"
    fi

    # OpenRC (Alpine, Gentoo, Artix, etc.)
    if command -v rc-service >/dev/null 2>&1; then
        rc-service "${AGENT_NAME}" stop 2>/dev/null || true
        rc-update del "${AGENT_NAME}" default 2>/dev/null || true
        rm -f "/etc/init.d/${AGENT_NAME}"
    fi

    rm -f "${INSTALL_DIR}/${BINARY_NAME}"
    log_info "Uninstallation complete."
    exit 0
fi

# --- Validation ---
if [[ -z "$PULSE_URL" || -z "$PULSE_TOKEN" ]]; then
    fail "Missing required arguments: --url and --token"
fi

# Validate URL format (basic check)
if [[ ! "$PULSE_URL" =~ ^https?:// ]]; then
    fail "Invalid URL format. Must start with http:// or https://"
fi

# Validate token format (should be hex string, typically 64 chars)
if [[ ! "$PULSE_TOKEN" =~ ^[a-fA-F0-9]+$ ]]; then
    fail "Invalid token format. Token should be a hexadecimal string."
fi

# Validate interval format
if [[ ! "$INTERVAL" =~ ^[0-9]+[smh]?$ ]]; then
    fail "Invalid interval format. Use format like '30s', '5m', or '1h'."
fi

# --- TrueNAS SCALE Detection ---
# TrueNAS SCALE has an immutable root filesystem; /usr/local/bin is read-only.
# We store everything in /data which persists across reboots and upgrades.
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

if [[ "$(uname -s)" == "Linux" ]] && is_truenas_scale; then
    TRUENAS=true
    INSTALL_DIR="$TRUENAS_STATE_DIR"
    LOG_FILE="$TRUENAS_LOG_DIR/${AGENT_NAME}.log"
    log_info "TrueNAS SCALE detected (immutable root). Using $TRUENAS_STATE_DIR for installation."
fi

# --- Download ---
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l|armhf) ARCH="armv7" ;;
    armv6l) ARCH="armv6" ;;
    i386|i686) ARCH="386" ;;
    *) fail "Unsupported architecture: $ARCH" ;;
esac

# Construct arch param in format expected by download endpoint (e.g., linux-amd64)
ARCH_PARAM="${OS}-${ARCH}"

DOWNLOAD_URL="${PULSE_URL}/download/${BINARY_NAME}?arch=${ARCH_PARAM}"
log_info "Downloading agent from ${DOWNLOAD_URL}..."

# Create temp file and register for cleanup
TMP_BIN=$(mktemp)
TMP_FILES+=("$TMP_BIN")

# Build curl arguments as array for proper quoting
CURL_ARGS=(-fsSL --connect-timeout 30 --max-time 300)
if [[ "$INSECURE" == "true" ]]; then CURL_ARGS+=(-k); fi

if ! curl "${CURL_ARGS[@]}" -o "$TMP_BIN" "$DOWNLOAD_URL"; then
    fail "Download failed. Check URL and connectivity."
fi

# Verify downloaded binary
if [[ ! -s "$TMP_BIN" ]]; then
    fail "Downloaded file is empty."
fi

# Check if it's a valid executable (ELF for Linux, Mach-O for macOS)
if [[ "$OS" == "linux" ]]; then
    if ! head -c 4 "$TMP_BIN" | grep -q "ELF"; then
        fail "Downloaded file is not a valid Linux executable."
    fi
elif [[ "$OS" == "darwin" ]]; then
    # Mach-O magic: feedface (32-bit) or feedfacf (64-bit) or cafebabe (universal)
    MAGIC=$(xxd -p -l 4 "$TMP_BIN" 2>/dev/null || head -c 4 "$TMP_BIN" | od -A n -t x1 | tr -d ' ')
    if [[ ! "$MAGIC" =~ ^(cffaedfe|cefaedfe|cafebabe|feedface|feedfacf) ]]; then
        fail "Downloaded file is not a valid macOS executable."
    fi
fi

chmod +x "$TMP_BIN"

# Install Binary
log_info "Installing binary to ${INSTALL_DIR}/${BINARY_NAME}..."
mkdir -p "$INSTALL_DIR"
mv "$TMP_BIN" "${INSTALL_DIR}/${BINARY_NAME}"
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

# --- Legacy Cleanup ---
# Remove old agents if they exist to prevent conflicts
log_info "Checking for legacy agents..."

# Legacy Host Agent
if command -v systemctl >/dev/null 2>&1; then
    if systemctl is-active --quiet pulse-host-agent 2>/dev/null || systemctl is-enabled --quiet pulse-host-agent 2>/dev/null; then
        log_warn "Removing legacy pulse-host-agent..."
        systemctl stop pulse-host-agent 2>/dev/null || true
        systemctl disable pulse-host-agent 2>/dev/null || true
        rm -f /etc/systemd/system/pulse-host-agent.service
        rm -f /usr/local/bin/pulse-host-agent
    fi
    if systemctl is-active --quiet pulse-docker-agent 2>/dev/null || systemctl is-enabled --quiet pulse-docker-agent 2>/dev/null; then
        log_warn "Removing legacy pulse-docker-agent..."
        systemctl stop pulse-docker-agent 2>/dev/null || true
        systemctl disable pulse-docker-agent 2>/dev/null || true
        rm -f /etc/systemd/system/pulse-docker-agent.service
        rm -f /usr/local/bin/pulse-docker-agent
    fi
    systemctl daemon-reload 2>/dev/null || true
fi

# Legacy macOS
if [[ "$OS" == "darwin" ]]; then
    if launchctl list | grep -q "com.pulse.host-agent"; then
        log_warn "Removing legacy com.pulse.host-agent..."
        launchctl unload /Library/LaunchDaemons/com.pulse.host-agent.plist 2>/dev/null || true
        rm -f /Library/LaunchDaemons/com.pulse.host-agent.plist
        rm -f /usr/local/bin/pulse-host-agent
    fi
    if launchctl list | grep -q "com.pulse.docker-agent"; then
        log_warn "Removing legacy com.pulse.docker-agent..."
        launchctl unload /Library/LaunchDaemons/com.pulse.docker-agent.plist 2>/dev/null || true
        rm -f /Library/LaunchDaemons/com.pulse.docker-agent.plist
        rm -f /usr/local/bin/pulse-docker-agent
    fi
fi

# --- Service Installation ---

# 1. macOS (Launchd)
if [[ "$OS" == "darwin" ]]; then
    PLIST="/Library/LaunchDaemons/com.pulse.agent.plist"
    log_info "Configuring Launchd service at $PLIST..."

    # Build program arguments array
    PLIST_ARGS="        <string>${INSTALL_DIR}/${BINARY_NAME}</string>
        <string>--url</string>
        <string>${PULSE_URL}</string>
        <string>--token</string>
        <string>${PULSE_TOKEN}</string>
        <string>--interval</string>
        <string>${INTERVAL}</string>"

    # Always pass enable-host flag since agent defaults to true
    if [[ "$ENABLE_HOST" == "true" ]]; then
        PLIST_ARGS="${PLIST_ARGS}
        <string>--enable-host</string>"
    else
        PLIST_ARGS="${PLIST_ARGS}
        <string>--enable-host=false</string>"
    fi
    if [[ "$ENABLE_DOCKER" == "true" ]]; then
        PLIST_ARGS="${PLIST_ARGS}
        <string>--enable-docker</string>"
    fi
    if [[ "$INSECURE" == "true" ]]; then
        PLIST_ARGS="${PLIST_ARGS}
        <string>--insecure</string>"
    fi
    if [[ -n "$AGENT_ID" ]]; then
        PLIST_ARGS="${PLIST_ARGS}
        <string>--agent-id</string>
        <string>${AGENT_ID}</string>"
    fi

    cat > "$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.pulse.agent</string>
    <key>ProgramArguments</key>
    <array>
${PLIST_ARGS}
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

# 2. Synology DSM
# DSM 7+ uses systemd, DSM 6.x uses upstart
if [[ -d /usr/syno ]] && [[ -f /etc/VERSION ]]; then
    # Extract major version from /etc/VERSION
    DSM_MAJOR=$(grep 'majorversion=' /etc/VERSION | cut -d'"' -f2)
    log_info "Detected Synology DSM ${DSM_MAJOR}..."

    # Build command line args
    build_exec_args

    if [[ "$DSM_MAJOR" -ge 7 ]]; then
        # DSM 7+ uses systemd
        UNIT="/etc/systemd/system/${AGENT_NAME}.service"
        log_info "Configuring systemd service at $UNIT (DSM 7+)..."

        cat > "$UNIT" <<EOF
[Unit]
Description=Pulse Unified Agent
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${BINARY_NAME} ${EXEC_ARGS}
Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
        systemctl daemon-reload
        systemctl enable "${AGENT_NAME}"
        systemctl restart "${AGENT_NAME}"
    else
        # DSM 6.x uses upstart
        CONF="/etc/init/${AGENT_NAME}.conf"
        log_info "Configuring Upstart service at $CONF (DSM 6.x)..."

        cat > "$CONF" <<EOF
description "Pulse Unified Agent"
author "Pulse"

start on syno.network.ready
stop on runlevel [06]

respawn
respawn limit 5 10

exec ${INSTALL_DIR}/${BINARY_NAME} ${EXEC_ARGS} >> ${LOG_FILE} 2>&1
EOF
        initctl stop "${AGENT_NAME}" 2>/dev/null || true
        initctl start "${AGENT_NAME}"
    fi

    log_info "Service started."
    exit 0
fi

# 3. Unraid (no init system - use /boot/config/go script)
# Detect Unraid by /etc/unraid-version (preferred) or /boot/config/go with unraid markers
if [[ -f /etc/unraid-version ]]; then
    log_info "Detected Unraid system..."

    # Unraid's /boot is FAT32 (no execute permission), so we store the binary there
    # for persistence but copy it to RAM disk (/usr/local/bin) for execution
    UNRAID_STORAGE_DIR="/boot/config/plugins/pulse-agent"
    UNRAID_STORED_BINARY="${UNRAID_STORAGE_DIR}/${BINARY_NAME}"
    RUNTIME_BINARY="${INSTALL_DIR}/${BINARY_NAME}"
    GO_SCRIPT="/boot/config/go"

    mkdir -p "$UNRAID_STORAGE_DIR"

    # Copy binary to persistent storage (for survival across reboots)
    cp "${RUNTIME_BINARY}" "$UNRAID_STORED_BINARY"
    # Keep binary in /usr/local/bin (RAM disk) with execute permission for runtime
    chmod +x "${RUNTIME_BINARY}"

    log_info "Installed binary to ${UNRAID_STORED_BINARY} (persistent) and ${RUNTIME_BINARY} (runtime)..."

    # Build command line args (string for wrapper script, array for direct execution)
    build_exec_args
    build_exec_args_array

    # Kill any existing pulse agents (legacy or current)
    log_info "Stopping any existing pulse agents..."
    # Use process name matching to avoid killing unrelated processes
    pkill -f "^${RUNTIME_BINARY}" 2>/dev/null || true
    pkill -f "^/usr/local/bin/pulse-host-agent" 2>/dev/null || true
    pkill -f "^/usr/local/bin/pulse-docker-agent" 2>/dev/null || true
    sleep 2

    # Clean up legacy binaries from RAM disk
    rm -f /usr/local/bin/pulse-host-agent 2>/dev/null || true
    rm -f /usr/local/bin/pulse-docker-agent 2>/dev/null || true

    # Create a wrapper script that will be called from /boot/config/go
    # This script copies from persistent storage to RAM disk on boot, then starts the agent
    WRAPPER_SCRIPT="${UNRAID_STORAGE_DIR}/start-pulse-agent.sh"
    cat > "$WRAPPER_SCRIPT" <<EOF
#!/bin/bash
# Pulse Agent startup script for Unraid
# Auto-generated by Pulse installer

# Kill any existing pulse-agent processes
pkill -f "^${RUNTIME_BINARY}" 2>/dev/null || true
pkill -f "^/usr/local/bin/pulse-host-agent" 2>/dev/null || true
pkill -f "^/usr/local/bin/pulse-docker-agent" 2>/dev/null || true
sleep 2

# Copy binary from persistent storage to RAM disk (needed after reboot)
cp "${UNRAID_STORED_BINARY}" "${RUNTIME_BINARY}"
chmod +x "${RUNTIME_BINARY}"

# Start the agent in the background
nohup ${RUNTIME_BINARY} ${EXEC_ARGS} >> /var/log/${AGENT_NAME}.log 2>&1 &
EOF

    # Add to /boot/config/go if not already present
    GO_MARKER="# Pulse Agent"
    if [[ -f "$GO_SCRIPT" ]]; then
        # Remove any existing Pulse agent entries
        sed -i "/${GO_MARKER}/,/^$/d" "$GO_SCRIPT" 2>/dev/null || true
        sed -i '/pulse-agent/d' "$GO_SCRIPT" 2>/dev/null || true
    else
        # Create go script if it doesn't exist
        echo "#!/bin/bash" > "$GO_SCRIPT"
        chmod +x "$GO_SCRIPT"
    fi

    # Append startup entry (use bash explicitly since /boot is FAT32 and doesn't support execute bits)
    cat >> "$GO_SCRIPT" <<EOF

${GO_MARKER}
bash ${WRAPPER_SCRIPT}

EOF

    log_info "Added startup entry to ${GO_SCRIPT}..."

    # Start the agent now (use array for proper argument handling)
    log_info "Starting agent..."
    nohup "${RUNTIME_BINARY}" "${EXEC_ARGS_ARRAY[@]}" >> "/var/log/${AGENT_NAME}.log" 2>&1 &

    log_info "Installation complete!"
    log_info "The agent will start automatically on boot."
    log_info "To check status: pgrep -a pulse-agent"
    log_info "To view logs: tail -f /var/log/${AGENT_NAME}.log"
    exit 0
fi

# 4. TrueNAS SCALE (immutable root, uses systemd but needs special persistence)
# TrueNAS SCALE wipes /etc/systemd/system on upgrades, so we store the service
# in /data and create an Init/Shutdown task to recreate the symlink on boot.
if [[ "$TRUENAS" == true ]]; then
    log_info "Configuring TrueNAS SCALE installation..."

    # Create directories
    mkdir -p "$TRUENAS_STATE_DIR"
    mkdir -p "$TRUENAS_LOG_DIR"

    # Build command line args
    build_exec_args

    # Store service file in /data (persists across upgrades)
    TRUENAS_SERVICE_STORAGE="$TRUENAS_STATE_DIR/${AGENT_NAME}.service"
    cat > "$TRUENAS_SERVICE_STORAGE" <<EOF
[Unit]
Description=Pulse Unified Agent
After=network-online.target docker.service
Wants=network-online.target
StartLimitIntervalSec=0

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${BINARY_NAME} ${EXEC_ARGS}
Restart=always
RestartSec=5s
User=root
StandardOutput=append:${LOG_FILE}
StandardError=append:${LOG_FILE}

[Install]
WantedBy=multi-user.target
EOF

    # Store environment/config for reference
    cat > "$TRUENAS_ENV_FILE" <<EOF
# Pulse Agent configuration (for reference)
PULSE_URL=${PULSE_URL}
PULSE_TOKEN=${PULSE_TOKEN}
PULSE_INTERVAL=${INTERVAL}
PULSE_ENABLE_HOST=${ENABLE_HOST}
PULSE_ENABLE_DOCKER=${ENABLE_DOCKER}
EOF
    chmod 600 "$TRUENAS_ENV_FILE"

    # Create bootstrap script that runs on boot
    cat > "$TRUENAS_BOOTSTRAP_SCRIPT" <<'BOOTSTRAP'
#!/bin/bash
# Pulse Agent Bootstrap for TrueNAS SCALE
# This script is called by TrueNAS Init/Shutdown task on boot.
# It recreates the systemd symlink (which is wiped on TrueNAS upgrades).

set -e

SERVICE_NAME="pulse-agent"
SERVICE_STORAGE="STATE_DIR_PLACEHOLDER/pulse-agent.service"
SYSTEMD_LINK="/etc/systemd/system/${SERVICE_NAME}.service"

if [[ ! -f "$SERVICE_STORAGE" ]]; then
    echo "ERROR: Service file not found at $SERVICE_STORAGE"
    exit 1
fi

# Create symlink (or update if exists)
ln -sf "$SERVICE_STORAGE" "$SYSTEMD_LINK"

# Reload and start
systemctl daemon-reload
systemctl enable "$SERVICE_NAME" 2>/dev/null || true
systemctl restart "$SERVICE_NAME"

echo "Pulse agent started successfully"
BOOTSTRAP

    # Replace placeholder
    sed -i "s|STATE_DIR_PLACEHOLDER|${TRUENAS_STATE_DIR}|g" "$TRUENAS_BOOTSTRAP_SCRIPT"
    chmod +x "$TRUENAS_BOOTSTRAP_SCRIPT"

    # Create systemd symlink now
    SYSTEMD_LINK="/etc/systemd/system/${AGENT_NAME}.service"
    ln -sf "$TRUENAS_SERVICE_STORAGE" "$SYSTEMD_LINK"

    # Register Init/Shutdown task using midclt
    if command -v midclt >/dev/null 2>&1; then
        log_info "Registering TrueNAS Init/Shutdown task..."

        # Check if task already exists
        EXISTING_TASK=$(midclt call initshutdownscript.query '[["script","=","'"$TRUENAS_BOOTSTRAP_SCRIPT"'"]]' 2>/dev/null | python3 -c "import json,sys; d=json.load(sys.stdin); print(d[0]['id'] if d else '')" 2>/dev/null || echo "")

        if [[ -n "$EXISTING_TASK" ]]; then
            log_info "Init/Shutdown task already exists (id $EXISTING_TASK), updating..."
            midclt call initshutdownscript.update "$EXISTING_TASK" '{"type":"SCRIPT","script":"'"$TRUENAS_BOOTSTRAP_SCRIPT"'","when":"POSTINIT","enabled":true,"timeout":30,"comment":"Pulse Agent Bootstrap"}' >/dev/null 2>&1 || true
        else
            midclt call initshutdownscript.create '{"type":"SCRIPT","script":"'"$TRUENAS_BOOTSTRAP_SCRIPT"'","when":"POSTINIT","enabled":true,"timeout":30,"comment":"Pulse Agent Bootstrap"}' >/dev/null 2>&1 || log_warn "Failed to create Init/Shutdown task. Please add it manually in TrueNAS UI."
        fi
    else
        log_warn "midclt not available. Please create an Init/Shutdown task manually in TrueNAS UI:"
        log_warn "  Type: Script"
        log_warn "  Script: $TRUENAS_BOOTSTRAP_SCRIPT"
        log_warn "  When: Post Init"
    fi

    # Enable and start service
    systemctl daemon-reload
    systemctl enable "${AGENT_NAME}" 2>/dev/null || true
    systemctl restart "${AGENT_NAME}"

    log_info "Installation complete!"
    log_info "Binary: ${INSTALL_DIR}/${BINARY_NAME}"
    log_info "Service: $TRUENAS_SERVICE_STORAGE (symlinked to systemd)"
    log_info "Logs: tail -f ${LOG_FILE}"
    log_info ""
    log_info "The Init/Shutdown task ensures the agent survives TrueNAS upgrades."
    exit 0
fi

# 5. OpenRC (Alpine, Gentoo, Artix, etc.)
# Check for rc-service but make sure we're not on a systemd system that happens to have it
if command -v rc-service >/dev/null 2>&1 && [[ -d /etc/init.d ]] && ! command -v systemctl >/dev/null 2>&1; then
    INITSCRIPT="/etc/init.d/${AGENT_NAME}"
    log_info "Configuring OpenRC service at $INITSCRIPT..."

    # Build command line args
    build_exec_args

    # Create OpenRC init script following Alpine best practices
    # Using command_background=yes with pidfile for proper daemon management
    cat > "$INITSCRIPT" <<'INITEOF'
#!/sbin/openrc-run
# Pulse Unified Agent OpenRC init script

name="pulse-agent"
description="Pulse Unified Agent"

command="INSTALL_DIR_PLACEHOLDER/BINARY_NAME_PLACEHOLDER"
command_args="EXEC_ARGS_PLACEHOLDER"
command_background="yes"
command_user="root"

pidfile="/run/${RC_SVCNAME}.pid"
output_log="/var/log/pulse-agent.log"
error_log="/var/log/pulse-agent.log"

# Ensure log file exists
start_pre() {
    touch "$output_log"
}

depend() {
    need net
    use docker
}
INITEOF

    # Replace placeholders with actual values
    sed -i "s|INSTALL_DIR_PLACEHOLDER|${INSTALL_DIR}|g" "$INITSCRIPT"
    sed -i "s|BINARY_NAME_PLACEHOLDER|${BINARY_NAME}|g" "$INITSCRIPT"
    sed -i "s|EXEC_ARGS_PLACEHOLDER|${EXEC_ARGS}|g" "$INITSCRIPT"

    chmod +x "$INITSCRIPT"
    rc-service "${AGENT_NAME}" stop 2>/dev/null || true
    rc-update add "${AGENT_NAME}" default 2>/dev/null || true
    rc-service "${AGENT_NAME}" start
    log_info "Service started."
    exit 0
fi

# 5. Linux (Systemd)
if command -v systemctl >/dev/null 2>&1; then
    UNIT="/etc/systemd/system/${AGENT_NAME}.service"
    log_info "Configuring Systemd service at $UNIT..."

    # Build command line args
    build_exec_args

    cat > "$UNIT" <<EOF
[Unit]
Description=Pulse Unified Agent
After=network-online.target docker.service
Wants=network-online.target
StartLimitIntervalSec=0

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${BINARY_NAME} ${EXEC_ARGS}
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

fail "Could not detect a supported service manager (systemd, OpenRC, launchd, or Unraid)."

}

# Call main function with all arguments
main "$@"
