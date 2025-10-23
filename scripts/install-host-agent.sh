#!/bin/bash
set -e

# Pulse Host Agent Installer
# Downloads and installs the Pulse host agent for Linux, macOS, or Windows (WSL)

trim() {
    local value="$1"
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"
    printf '%s' "$value"
}

log_info() {
    printf '[INFO] %s\n' "$1"
}

log_success() {
    printf '[ OK ] %s\n' "$1"
}

log_warn() {
    printf '[WARN] %s\n' "$1" >&2
}

log_error() {
    printf '[ERROR] %s\n' "$1" >&2
}

# Parse arguments
PULSE_URL=""
PULSE_TOKEN=""
INTERVAL="30s"
UNINSTALL="false"
PLATFORM=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --url)
            PULSE_URL="$2"
            shift 2
            ;;
        --token)
            PULSE_TOKEN="$2"
            shift 2
            ;;
        --interval)
            INTERVAL="$2"
            shift 2
            ;;
        --platform)
            PLATFORM="$2"
            shift 2
            ;;
        --uninstall)
            UNINSTALL="true"
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

AGENT_PATH="/usr/local/bin/pulse-host-agent"
SYSTEMD_SERVICE="/etc/systemd/system/pulse-host-agent.service"
LAUNCHD_PLIST="$HOME/Library/LaunchAgents/com.pulse.host-agent.plist"

# Uninstall function
if [[ "$UNINSTALL" == "true" ]]; then
    log_info "Uninstalling Pulse host agent..."

    # Stop and disable systemd service (Linux)
    if [[ -f "$SYSTEMD_SERVICE" ]] && command -v systemctl &> /dev/null; then
        sudo systemctl stop pulse-host-agent 2>/dev/null || true
        sudo systemctl disable pulse-host-agent 2>/dev/null || true
        sudo rm -f "$SYSTEMD_SERVICE"
        sudo systemctl daemon-reload
        log_success "Removed systemd service"
    fi

    # Stop and remove launchd service (macOS)
    if [[ -f "$LAUNCHD_PLIST" ]] && command -v launchctl &> /dev/null; then
        launchctl unload "$LAUNCHD_PLIST" 2>/dev/null || true
        rm -f "$LAUNCHD_PLIST"
        log_success "Removed launchd service"
    fi

    # Remove binary
    if [[ -f "$AGENT_PATH" ]]; then
        sudo rm -f "$AGENT_PATH"
        log_success "Removed agent binary"
    fi

    log_success "Pulse host agent uninstalled"
    exit 0
fi

# Validate required parameters for install
if [[ -z "$PULSE_URL" ]]; then
    log_error "Missing required parameter: --url"
    echo "Usage: $0 --url <pulse-url> --token <api-token> [--interval 30s] [--platform linux|darwin|windows]"
    exit 1
fi

if [[ -z "$PULSE_TOKEN" ]] && [[ "$PULSE_TOKEN" != "disabled" ]]; then
    log_error "Missing required parameter: --token"
    echo "Usage: $0 --url <pulse-url> --token <api-token> [--interval 30s] [--platform linux|darwin|windows]"
    exit 1
fi

# Detect platform if not specified
if [[ -z "$PLATFORM" ]]; then
    case "$(uname -s)" in
        Linux*)
            PLATFORM="linux"
            ;;
        Darwin*)
            PLATFORM="darwin"
            ;;
        MINGW*|MSYS*|CYGWIN*)
            PLATFORM="windows"
            ;;
        *)
            log_error "Unsupported platform: $(uname -s)"
            exit 1
            ;;
    esac
fi

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    armv7l|armhf)
        ARCH="armv7"
        ;;
    *)
        log_warn "Unknown architecture $ARCH, defaulting to amd64"
        ARCH="amd64"
        ;;
esac

log_info "Installing Pulse host agent for $PLATFORM/$ARCH..."

# Download agent binary from Pulse server
DOWNLOAD_URL="$PULSE_URL/download/pulse-host-agent?platform=$PLATFORM&arch=$ARCH"
TEMP_BINARY="/tmp/pulse-host-agent-$$.tmp"

log_info "Downloading agent binary from $PULSE_URL..."

if command -v curl &> /dev/null; then
    if ! curl -fL --progress-bar -o "$TEMP_BINARY" "$DOWNLOAD_URL"; then
        log_error "Failed to download agent binary from $DOWNLOAD_URL"
        log_info "The server may not have prebuilt binaries yet. You can build from source:"
        log_info "  git clone https://github.com/rcourtman/Pulse.git && cd Pulse"
        log_info "  go build -o pulse-host-agent ./cmd/pulse-host-agent"
        log_info "  sudo mv pulse-host-agent /usr/local/bin/"
        rm -f "$TEMP_BINARY"
        exit 1
    fi
elif command -v wget &> /dev/null; then
    if ! wget -q --show-progress -O "$TEMP_BINARY" "$DOWNLOAD_URL"; then
        log_error "Failed to download agent binary from $DOWNLOAD_URL"
        rm -f "$TEMP_BINARY"
        exit 1
    fi
else
    log_error "Neither curl nor wget found. Please install one of them."
    exit 1
fi

sudo mv "$TEMP_BINARY" "$AGENT_PATH"
sudo chmod +x "$AGENT_PATH"
log_success "Agent binary installed to $AGENT_PATH"

# Set up service based on platform
if [[ "$PLATFORM" == "linux" ]] && command -v systemctl &> /dev/null; then
    log_info "Setting up systemd service..."

    sudo tee "$SYSTEMD_SERVICE" > /dev/null <<EOF
[Unit]
Description=Pulse Host Agent
After=network.target

[Service]
Type=simple
ExecStart=$AGENT_PATH --url $PULSE_URL --token $PULSE_TOKEN --interval $INTERVAL
Restart=always
RestartSec=5s
User=root

[Install]
WantedBy=multi-user.target
EOF

    sudo systemctl daemon-reload
    sudo systemctl enable pulse-host-agent
    sudo systemctl start pulse-host-agent
    log_success "Systemd service enabled and started"

elif [[ "$PLATFORM" == "darwin" ]] && command -v launchctl &> /dev/null; then
    log_info "Setting up launchd service..."

    mkdir -p "$HOME/Library/LaunchAgents"

    cat > "$LAUNCHD_PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.pulse.host-agent</string>
    <key>ProgramArguments</key>
    <array>
        <string>$AGENT_PATH</string>
        <string>--url</string>
        <string>$PULSE_URL</string>
        <string>--token</string>
        <string>$PULSE_TOKEN</string>
        <string>--interval</string>
        <string>$INTERVAL</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/pulse-host-agent.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/pulse-host-agent.log</string>
</dict>
</plist>
EOF

    launchctl load "$LAUNCHD_PLIST"
    log_success "Launchd service enabled and started"
else
    log_warn "Automatic service setup not available for this platform"
    log_info "To run the agent manually:"
    log_info "  $AGENT_PATH --url $PULSE_URL --token $PULSE_TOKEN --interval $INTERVAL"
fi

log_success "Pulse host agent installation complete!"
log_info "The agent is now reporting to $PULSE_URL"
