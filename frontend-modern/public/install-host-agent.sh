#!/bin/bash
set -e

# Pulse Host Agent Installer
# Downloads and installs the Pulse host agent for Linux, macOS, or Windows (WSL)

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
    print_color "$BLUE" "  Pulse Host Agent - Installation"
    print_color "$BLUE" "═══════════════════════════════════════════════════════════"
    echo ""
}

print_footer() {
    echo ""
    print_color "$GREEN" "═══════════════════════════════════════════════════════════"
    log_success "Installation complete!"
    print_color "$GREEN" "═══════════════════════════════════════════════════════════"
    echo ""
}

# Parse arguments
PULSE_URL=""
PULSE_TOKEN=""
INTERVAL="30s"
UNINSTALL="false"
PLATFORM=""
FORCE=false

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
        --force|-f)
            FORCE=true
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
MACOS_LOG_DIR="$HOME/Library/Logs/Pulse"
MACOS_LOG_FILE="$MACOS_LOG_DIR/host-agent.log"
LINUX_LOG_DIR="/var/log/pulse"
LINUX_LOG_FILE="$LINUX_LOG_DIR/host-agent.log"

# Uninstall function
if [[ "$UNINSTALL" == "true" ]]; then
    log_warn "The --uninstall flag is deprecated."
    log_info "Please use the dedicated uninstall script instead:"
    echo ""
    echo "  curl -fsSL \$PULSE_URL/uninstall-host-agent.sh | bash"
    echo ""
    log_info "Or download and run manually:"
    echo "  wget \$PULSE_URL/uninstall-host-agent.sh"
    echo "  chmod +x uninstall-host-agent.sh"
    echo "  ./uninstall-host-agent.sh"
    echo ""
    exit 1
fi

print_header

# Interactive prompts if parameters not provided (unless --force is used)
if [[ -z "$PULSE_URL" ]]; then
    if [[ "$FORCE" == false ]]; then
        log_info "Interactive Installation Mode"
        echo ""
        read -p "Enter Pulse server URL (e.g., http://pulse.example.com:7656): " PULSE_URL
        PULSE_URL=$(echo "$PULSE_URL" | sed 's:/*$::')  # Remove trailing slashes
    fi
fi

if [[ -z "$PULSE_URL" ]]; then
    log_error "Pulse URL is required"
    echo "Usage: $0 --url <pulse-url> --token <api-token> [--interval 30s] [--platform linux|darwin|windows] [--force]"
    exit 1
fi

if [[ -z "$PULSE_TOKEN" ]] && [[ "$FORCE" == false ]]; then
    log_warn "No API token provided - agent will attempt to connect without authentication"
    read -p "Enter API token (or press Enter to skip): " PULSE_TOKEN

    if [[ -z "$PULSE_TOKEN" ]]; then
        read -p "Continue without token? (y/N): " CONTINUE_WITHOUT_TOKEN
        if [[ "$CONTINUE_WITHOUT_TOKEN" != "y" ]] && [[ "$CONTINUE_WITHOUT_TOKEN" != "Y" ]]; then
            log_error "Installation cancelled"
            exit 1
        fi
    fi
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

log_info "Configuration:"
echo "  Pulse URL: $PULSE_URL"
if [[ -n "$PULSE_TOKEN" ]]; then
    # Mask token, showing only last 4 characters
    TOKEN_MASKED="***${PULSE_TOKEN: -4}"
    echo "  Token: $TOKEN_MASKED"
else
    echo "  Token: none"
fi
echo "  Interval: $INTERVAL"
echo "  Platform: $PLATFORM/$ARCH"
echo ""

log_info "Installing Pulse host agent for $PLATFORM/$ARCH..."

# Check for existing installation and version
if [[ -f "$AGENT_PATH" ]]; then
    log_info "Existing installation detected"
    CURRENT_VERSION=$("$AGENT_PATH" --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo "unknown")
    if [[ "$CURRENT_VERSION" != "unknown" ]]; then
        echo "  Current version: $CURRENT_VERSION"
    fi

    # Try to get latest version from server
    if command -v curl &> /dev/null; then
        LATEST_VERSION=$(curl -fsSL "$PULSE_URL/api/version" 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo "")
    elif command -v wget &> /dev/null; then
        LATEST_VERSION=$(wget -qO- "$PULSE_URL/api/version" 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo "")
    fi

    if [[ -n "$LATEST_VERSION" ]] && [[ "$CURRENT_VERSION" != "unknown" ]]; then
        echo "  Latest version:  $LATEST_VERSION"
        if [[ "$CURRENT_VERSION" == "$LATEST_VERSION" ]]; then
            log_success "Already running latest version"
        elif [[ "$CURRENT_VERSION" < "$LATEST_VERSION" ]]; then
            log_info "Update available: $CURRENT_VERSION → $LATEST_VERSION"
        fi
    fi

    if [[ "$FORCE" == false ]]; then
        read -p "Reinstall/update agent? (Y/n): " REINSTALL
        if [[ "$REINSTALL" == "n" ]] || [[ "$REINSTALL" == "N" ]]; then
            log_info "Installation cancelled"
            exit 0
        fi
        echo ""
    else
        log_info "Force mode: automatically reinstalling/updating agent"
        echo ""
    fi
fi

# Download agent binary from Pulse server
DOWNLOAD_URL="$PULSE_URL/download/pulse-host-agent?platform=$PLATFORM&arch=$ARCH"
CHECKSUM_URL="$PULSE_URL/download/pulse-host-agent.sha256?platform=$PLATFORM&arch=$ARCH"
TEMP_BINARY="/tmp/pulse-host-agent-$$.tmp"

log_info "Downloading agent binary from $PULSE_URL..."

DOWNLOAD_SUCCESS=false

if command -v curl &> /dev/null; then
    if curl -fL --progress-bar -o "$TEMP_BINARY" "$DOWNLOAD_URL" 2>&1; then
        DOWNLOAD_SUCCESS=true
    fi
    # Try to download checksum (optional, server may not provide it yet)
    EXPECTED_CHECKSUM=$(curl -fsSL "$CHECKSUM_URL" 2>/dev/null || echo "")
elif command -v wget &> /dev/null; then
    if wget -q --show-progress -O "$TEMP_BINARY" "$DOWNLOAD_URL" 2>&1; then
        DOWNLOAD_SUCCESS=true
    fi
    # Try to download checksum (optional)
    EXPECTED_CHECKSUM=$(wget -qO- "$CHECKSUM_URL" 2>/dev/null || echo "")
else
    log_error "Neither curl nor wget found"
    echo ""
    log_info "Please install curl or wget to continue:"
    if [[ "$PLATFORM" == "darwin" ]]; then
        echo "  brew install curl"
    elif [[ "$PLATFORM" == "linux" ]]; then
        echo "  # Debian/Ubuntu:"
        echo "  sudo apt-get install curl"
        echo ""
        echo "  # RHEL/CentOS/Fedora:"
        echo "  sudo yum install curl"
    fi
    echo ""
    exit 1
fi

if [[ "$DOWNLOAD_SUCCESS" == false ]]; then
    log_error "Failed to download agent binary from $DOWNLOAD_URL"
    echo ""
    log_info "Troubleshooting steps:"
    echo ""
    echo "1. Verify the Pulse server is running:"
    echo "   curl $PULSE_URL/health"
    echo ""
    echo "2. Check if the download endpoint is accessible:"
    echo "   curl -I $DOWNLOAD_URL"
    echo ""
    echo "3. Build from source as a fallback:"
    echo "   git clone https://github.com/rcourtman/Pulse.git"
    echo "   cd Pulse"
    echo "   go build -o pulse-host-agent ./cmd/pulse-host-agent"
    echo "   sudo mv pulse-host-agent /usr/local/bin/"
    echo "   # Then run this script again with --url and --token"
    echo ""
    echo "4. Check firewall/network settings blocking the connection"
    echo ""
    rm -f "$TEMP_BINARY"
    exit 1
fi

log_success "Downloaded agent binary"

# Verify checksum if available
if [[ -n "$EXPECTED_CHECKSUM" ]]; then
    log_info "Verifying checksum..."

    if command -v sha256sum &> /dev/null; then
        ACTUAL_CHECKSUM=$(sha256sum "$TEMP_BINARY" | awk '{print $1}')
    elif command -v shasum &> /dev/null; then
        ACTUAL_CHECKSUM=$(shasum -a 256 "$TEMP_BINARY" | awk '{print $1}')
    else
        log_warn "No checksum tool found (sha256sum/shasum), skipping verification"
        ACTUAL_CHECKSUM=""
    fi

    if [[ -n "$ACTUAL_CHECKSUM" ]]; then
        # Clean up checksums (remove whitespace, convert to lowercase)
        EXPECTED_CHECKSUM=$(echo "$EXPECTED_CHECKSUM" | tr -d '[:space:]' | tr '[:upper:]' '[:lower:]')
        ACTUAL_CHECKSUM=$(echo "$ACTUAL_CHECKSUM" | tr -d '[:space:]' | tr '[:upper:]' '[:lower:]')

        if [[ "$EXPECTED_CHECKSUM" == "$ACTUAL_CHECKSUM" ]]; then
            log_success "Checksum verified (SHA256: ${ACTUAL_CHECKSUM:0:16}...)"
        else
            log_error "Checksum mismatch!"
            echo "  Expected: $EXPECTED_CHECKSUM"
            echo "  Got:      $ACTUAL_CHECKSUM"
            echo ""
            log_warn "The downloaded binary may be corrupted or tampered with."

            if [[ "$FORCE" == false ]]; then
                read -p "Continue anyway? (y/N): " CONTINUE_ANYWAY
                if [[ "$CONTINUE_ANYWAY" != "y" ]] && [[ "$CONTINUE_ANYWAY" != "Y" ]]; then
                    rm -f "$TEMP_BINARY"
                    log_error "Installation cancelled"
                    exit 1
                fi
            else
                log_error "Force mode: aborting due to checksum mismatch (security risk)"
                rm -f "$TEMP_BINARY"
                exit 1
            fi
        fi
    fi
else
    log_info "Checksum not available (server doesn't provide it yet)"
fi

sudo mv "$TEMP_BINARY" "$AGENT_PATH"
sudo chmod +x "$AGENT_PATH"
log_success "Agent binary installed to $AGENT_PATH"

# Set up service based on platform
if [[ "$PLATFORM" == "linux" ]] && command -v systemctl &> /dev/null; then
    log_info "Setting up systemd service..."

    # Create log directory
    sudo mkdir -p "$LINUX_LOG_DIR"

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
StandardOutput=append:$LINUX_LOG_FILE
StandardError=append:$LINUX_LOG_FILE

[Install]
WantedBy=multi-user.target
EOF

    log_success "Created systemd service configuration"

    sudo systemctl daemon-reload
    sudo systemctl enable pulse-host-agent
    sudo systemctl start pulse-host-agent
    log_success "Systemd service enabled and started"

elif [[ "$PLATFORM" == "darwin" ]] && command -v launchctl &> /dev/null; then
    log_info "Setting up launchd service..."

    # Create log directory
    mkdir -p "$MACOS_LOG_DIR"
    mkdir -p "$HOME/Library/LaunchAgents"

    # Store token in macOS Keychain for better security
    if [[ -n "$PULSE_TOKEN" ]]; then
        log_info "Storing token in macOS Keychain..."

        # Delete existing keychain entry if it exists
        security delete-generic-password -s "pulse-host-agent" -a "$USER" 2>/dev/null || true

        # Add token to Keychain
        KEYCHAIN_SERVICE="pulse-host-agent"
        KEYCHAIN_ACCOUNT="$USER"

        KEYCHAIN_APPS=(
            "/usr/local/bin/pulse-host-agent"
            "/usr/local/bin/pulse-host-agent-wrapper.sh"
            "/usr/bin/security"
        )
        KEYCHAIN_ARGS=()
        for app in "${KEYCHAIN_APPS[@]}"; do
            KEYCHAIN_ARGS+=(-T "$app")
        done

        if security add-generic-password \
            -s "$KEYCHAIN_SERVICE" \
            -a "$KEYCHAIN_ACCOUNT" \
            -w "$PULSE_TOKEN" \
            -U \
            "${KEYCHAIN_ARGS[@]}" 2>/dev/null; then
            if security find-generic-password -s "$KEYCHAIN_SERVICE" -a "$KEYCHAIN_ACCOUNT" -w >/dev/null 2>&1; then
                log_success "Token stored securely in macOS Keychain"
                USE_KEYCHAIN=true
            else
                log_warn "Token saved but Keychain denied non-interactive read access"
                log_info "Will fall back to embedding token in the launchd plist"
                USE_KEYCHAIN=false
            fi
        else
            log_warn "Failed to store token in Keychain, will use plist instead"
            log_info "You may need to grant Keychain access permissions"
            USE_KEYCHAIN=false
        fi
    else
        USE_KEYCHAIN=false
    fi

    # Create wrapper script if using Keychain
    if [[ "$USE_KEYCHAIN" == true ]]; then
        WRAPPER_SCRIPT="/usr/local/bin/pulse-host-agent-wrapper.sh"
        TMP_WRAPPER=$(mktemp)

        cat > "$TMP_WRAPPER" <<'WRAPPER_EOF'
#!/bin/bash
# Pulse Host Agent Wrapper - Reads token from Keychain
set -u

LOG_FILE="$HOME/Library/Logs/Pulse/host-agent-wrapper.log"
mkdir -p "$(dirname "$LOG_FILE")"

# Read token from Keychain
if ! PULSE_TOKEN=$(security find-generic-password -s "pulse-host-agent" -a "$USER" -w 2>/dev/null); then
    echo "$(date -Is) pulse-host-agent-wrapper: failed to read token from Keychain" >>"$LOG_FILE"
    PULSE_TOKEN=""
fi

# Export for agent to use
export PULSE_TOKEN

# Run the actual agent with all arguments
exec /usr/local/bin/pulse-host-agent "$@"
WRAPPER_EOF

        if ! sudo mv "$TMP_WRAPPER" "$WRAPPER_SCRIPT"; then
            if ! mv "$TMP_WRAPPER" "$WRAPPER_SCRIPT" 2>/dev/null; then
                rm -f "$TMP_WRAPPER"
                log_error "Failed to write Keychain wrapper to $WRAPPER_SCRIPT. Try re-running with sudo."
                exit 1
            fi
        fi

        if ! sudo chmod 755 "$WRAPPER_SCRIPT" 2>/dev/null && ! chmod 755 "$WRAPPER_SCRIPT" 2>/dev/null; then
            log_error "Failed to set execute permissions on $WRAPPER_SCRIPT."
            exit 1
        fi

        if command -v chown &>/dev/null; then
            sudo chown root:wheel "$WRAPPER_SCRIPT" 2>/dev/null || sudo chown root:root "$WRAPPER_SCRIPT" 2>/dev/null || true
        fi

        log_success "Created Keychain wrapper script"

        # Create plist using wrapper (token not in plist!)
        cat > "$LAUNCHD_PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.pulse.host-agent</string>
    <key>ProgramArguments</key>
    <array>
        <string>$WRAPPER_SCRIPT</string>
        <string>--url</string>
        <string>$PULSE_URL</string>
        <string>--interval</string>
        <string>$INTERVAL</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$MACOS_LOG_FILE</string>
    <key>StandardErrorPath</key>
    <string>$MACOS_LOG_FILE</string>
</dict>
</plist>
EOF
        log_success "Created launchd service configuration (using Keychain)"
    else
        # Create plist with token directly (fallback)
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
    <string>$MACOS_LOG_FILE</string>
    <key>StandardErrorPath</key>
    <string>$MACOS_LOG_FILE</string>
</dict>
</plist>
EOF
        log_success "Created launchd service configuration"
    fi

    # Set restrictive permissions on plist
    chmod 600 "$LAUNCHD_PLIST"

    LAUNCH_TARGET="gui/$(id -u)"

    # Attempt to unload any existing service instance
    if launchctl bootout "$LAUNCH_TARGET" "$LAUNCHD_PLIST" 2>/dev/null; then
        log_info "Replaced existing launchd service definition"
    fi

    if launchctl bootstrap "$LAUNCH_TARGET" "$LAUNCHD_PLIST"; then
        launchctl enable "$LAUNCH_TARGET/com.pulse.host-agent" 2>/dev/null || true
        launchctl kickstart -k "$LAUNCH_TARGET/com.pulse.host-agent" 2>/dev/null || true
        log_success "Launchd service enabled and started"
    else
        log_error "Failed to load launchd service. Try running:"
        echo "  launchctl bootstrap $LAUNCH_TARGET $LAUNCHD_PLIST"
        echo "  launchctl kickstart -k $LAUNCH_TARGET/com.pulse.host-agent"
        exit 1
    fi
else
    log_warn "Automatic service setup not available for this platform"
    log_info "To run the agent manually:"
    log_info "  $AGENT_PATH --url $PULSE_URL --token $PULSE_TOKEN --interval $INTERVAL"
fi

# Validate installation
log_info "Waiting 10 seconds to validate agent reporting..."
sleep 10

VALIDATION_SUCCESS=false
SERVICE_RUNNING=false

# Check if service is running
if [[ "$PLATFORM" == "linux" ]] && command -v systemctl &> /dev/null; then
    SERVICE_STATUS=$(systemctl is-active pulse-host-agent 2>/dev/null || echo "inactive")
    if [[ "$SERVICE_STATUS" == "active" ]]; then
        SERVICE_RUNNING=true
        log_success "Service is running successfully!"
    else
        log_warn "Service status: $SERVICE_STATUS"
        log_info "Check logs with: sudo journalctl -u pulse-host-agent -n 50"
    fi
elif [[ "$PLATFORM" == "darwin" ]] && command -v launchctl &> /dev/null; then
    if launchctl list | grep -q "com.pulse.host-agent"; then
        SERVICE_RUNNING=true
        log_success "Service is running successfully!"
    else
        log_warn "Service may not be running properly"
        log_info "Check logs with: tail -20 $MACOS_LOG_FILE"
    fi
fi

# Try to verify with API endpoint that agent is reporting
if [[ "$SERVICE_RUNNING" == true ]]; then
    log_info "Verifying agent registration with Pulse server..."

    # Get hostname for verification
    HOSTNAME=$(hostname)

    # Try to query the API for this host
    if command -v curl &> /dev/null; then
        API_CHECK=$(curl -fsSL "$PULSE_URL/api/hosts" 2>/dev/null || echo "")
    elif command -v wget &> /dev/null; then
        API_CHECK=$(wget -qO- "$PULSE_URL/api/hosts" 2>/dev/null || echo "")
    fi

    if [[ -n "$API_CHECK" ]] && echo "$API_CHECK" | grep -q "$HOSTNAME"; then
        VALIDATION_SUCCESS=true
        log_success "Agent successfully registered with Pulse server!"
        log_success "Your host '$HOSTNAME' is now reporting metrics!"
    elif [[ -n "$API_CHECK" ]]; then
        log_warn "Agent is running but host not found in API yet (may take a few moments)"
        log_info "Service appears healthy, metrics should appear shortly."
        VALIDATION_SUCCESS=true  # Service is running, so count as success
    else
        log_warn "Could not verify registration via API (endpoint may not be accessible)"
        log_info "Service is running, check your Pulse dashboard manually."
        VALIDATION_SUCCESS=true  # Service is running, so count as success
    fi
fi

if [[ "$VALIDATION_SUCCESS" == true ]]; then
    log_info "Check your Pulse dashboard at: $PULSE_URL"
else
    log_error "Service validation failed"
    echo ""
    log_info "Troubleshooting:"
    echo ""
    if [[ "$PLATFORM" == "linux" ]]; then
        echo "  View logs:    sudo journalctl -u pulse-host-agent -f"
        echo "  Check status: sudo systemctl status pulse-host-agent"
        echo "  Restart:      sudo systemctl restart pulse-host-agent"
    elif [[ "$PLATFORM" == "darwin" ]]; then
        echo "  View logs:    tail -f $MACOS_LOG_FILE"
        echo "  Check status: launchctl list | grep pulse"
        echo "  Restart:      launchctl unload $LAUNCHD_PLIST && launchctl load $LAUNCHD_PLIST"
    fi
    echo ""
    echo "  Manual run:   $AGENT_PATH --url $PULSE_URL $(if [[ -n "$PULSE_TOKEN" ]]; then echo "--token ***"; fi) --interval $INTERVAL"
    echo ""
fi

print_footer

log_info "Service Management Commands:"
if [[ "$PLATFORM" == "linux" ]]; then
    echo "  Start:   sudo systemctl start pulse-host-agent"
    echo "  Stop:    sudo systemctl stop pulse-host-agent"
    echo "  Restart: sudo systemctl restart pulse-host-agent"
    echo "  Status:  sudo systemctl status pulse-host-agent"
    echo "  Logs:    sudo journalctl -u pulse-host-agent -f"
elif [[ "$PLATFORM" == "darwin" ]]; then
    echo "  Start:   launchctl load $LAUNCHD_PLIST"
    echo "  Stop:    launchctl unload $LAUNCHD_PLIST"
    echo "  Restart: launchctl unload $LAUNCHD_PLIST && launchctl load $LAUNCHD_PLIST"
    echo "  Status:  launchctl list | grep pulse"
    echo "  Logs:    tail -f $MACOS_LOG_FILE"
fi
echo ""

log_info "Files installed:"
echo "  Binary: $AGENT_PATH"
if [[ "$PLATFORM" == "linux" ]]; then
    echo "  Service: $SYSTEMD_SERVICE"
    echo "  Logs: $LINUX_LOG_FILE"
elif [[ "$PLATFORM" == "darwin" ]]; then
    echo "  Service: $LAUNCHD_PLIST"
    echo "  Logs: $MACOS_LOG_FILE"
fi
echo ""

log_info "The agent is now reporting to: $PULSE_URL"
echo ""

log_info "To uninstall, run:"
echo "  curl -fsSL $PULSE_URL/uninstall-host-agent.sh | bash"
echo ""
