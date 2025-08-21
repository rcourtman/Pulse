#!/usr/bin/env bash

# Pulse Installer Script
# Supports: Ubuntu 20.04+, Debian 11+, Proxmox VE 7+

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
INSTALL_DIR="/opt/pulse"
CONFIG_DIR="/etc/pulse"  # All config and data goes here for manual installs
SERVICE_NAME="pulse"
GITHUB_REPO="rcourtman/Pulse"

# Detect existing service name (pulse or pulse-backend)
detect_service_name() {
    if systemctl list-unit-files --no-legend | grep -q "^pulse-backend.service"; then
        echo "pulse-backend"
    elif systemctl list-unit-files --no-legend | grep -q "^pulse.service"; then
        echo "pulse"
    else
        echo "pulse"  # Default for new installations
    fi
}

# Functions
print_header() {
    echo -e "${BLUE}=================================================${NC}"
    echo -e "${BLUE}           Pulse Installation Script${NC}"
    echo -e "${BLUE}=================================================${NC}"
    echo
}

print_error() {
    echo -e "${RED}[ERROR] $1${NC}" >&2
}

print_success() {
    echo -e "${GREEN}[SUCCESS] $1${NC}"
}

print_info() {
    echo -e "${YELLOW}[INFO] $1${NC}"
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "This script must be run as root"
        exit 1
    fi
}

check_pre_v4_installation() {
    # Check for Node.js version indicators (pre-v4 used Node.js)
    # Note: pulse-backend.service is NOT a reliable indicator as v4 can use it too
    if [[ -f "$INSTALL_DIR/.env" ]] || \
       [[ -d "$INSTALL_DIR/node_modules" ]] || \
       [[ -f "$INSTALL_DIR/package.json" ]] || \
       [[ -f "$INSTALL_DIR/server.js" ]] || \
       [[ -d "$INSTALL_DIR/backend" ]] || \
       [[ -d "$INSTALL_DIR/frontend" ]]; then
        
        echo
        print_error "Pre-v4 Pulse Installation Detected"
        echo
        echo -e "${BLUE}=========================================================${NC}"
        echo -e "${YELLOW}Pulse v4 is a complete rewrite that requires migration${NC}"
        echo -e "${BLUE}=========================================================${NC}"
        echo
        echo "Your current installation appears to be a pre-v4 version."
        echo "Due to fundamental architecture changes, automatic upgrade is not supported."
        echo
        echo -e "${GREEN}Recommended approach:${NC}"
        echo "1. Create a fresh LXC container or VM"
        echo "2. Install Pulse v4 in the new environment"
        echo "3. Configure your nodes through the new web UI"
        echo "4. Reference your old .env file for credentials if needed"
        echo
        echo -e "${YELLOW}Your existing data is preserved at: $INSTALL_DIR${NC}"
        echo
        echo "For more information, see:"
        echo "https://github.com/rcourtman/Pulse/releases/v4.0.0"
        echo -e "${BLUE}=========================================================${NC}"
        echo
        exit 1
    fi
}

detect_os() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        OS=$ID
        VER=$VERSION_ID
    else
        print_error "Cannot detect OS"
        exit 1
    fi
}

check_existing_installation() {
    if systemctl is-active --quiet $SERVICE_NAME 2>/dev/null; then
        print_info "Pulse is currently running"
        return 0
    elif [[ -f "$INSTALL_DIR/pulse" ]]; then
        print_info "Pulse is installed but not running"
        return 0
    else
        return 1
    fi
}

install_dependencies() {
    print_info "Installing dependencies..."
    
    apt-get update -qq
    apt-get install -y -qq curl wget
}

create_user() {
    if ! id -u pulse &>/dev/null; then
        print_info "Creating pulse user..."
        useradd --system --home-dir $INSTALL_DIR --shell /bin/false pulse
    fi
}

backup_existing() {
    if [[ -d "$CONFIG_DIR" ]]; then
        print_info "Backing up existing configuration..."
        cp -a "$CONFIG_DIR" "${CONFIG_DIR}.backup.$(date +%Y%m%d-%H%M%S)"
    fi
}

download_pulse() {
    print_info "Downloading Pulse..."
    
    # Check for forced version first
    if [[ -n "${FORCE_VERSION}" ]]; then
        LATEST_RELEASE="${FORCE_VERSION}"
        print_info "Installing specific version: $LATEST_RELEASE"
        
        # Verify the version exists
        if ! curl -fsS "https://api.github.com/repos/$GITHUB_REPO/releases/tags/$LATEST_RELEASE" > /dev/null 2>&1; then
            print_error "Version $LATEST_RELEASE not found"
            exit 1
        fi
    else
        # Check if user has RC channel configured
        UPDATE_CHANNEL="stable"
        
        # Allow override via command line
        if [[ -n "${FORCE_CHANNEL}" ]]; then
            UPDATE_CHANNEL="${FORCE_CHANNEL}"
            print_info "Using $UPDATE_CHANNEL channel from command line"
        elif [[ -f "$CONFIG_DIR/system.json" ]]; then
            CONFIGURED_CHANNEL=$(cat "$CONFIG_DIR/system.json" 2>/dev/null | grep -o '"updateChannel"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"\([^"]*\)"$/\1/')
            if [[ "$CONFIGURED_CHANNEL" == "rc" ]]; then
                UPDATE_CHANNEL="rc"
                print_info "RC channel detected in configuration"
            fi
        fi
        
        # Get appropriate release based on channel
        if [[ "$UPDATE_CHANNEL" == "rc" ]]; then
            # Get all releases and find the latest (including pre-releases)
            LATEST_RELEASE=$(curl -s https://api.github.com/repos/$GITHUB_REPO/releases | grep '"tag_name":' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
        else
            # Get latest stable release only
            LATEST_RELEASE=$(curl -s https://api.github.com/repos/$GITHUB_REPO/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        fi
        
        if [[ -z "$LATEST_RELEASE" ]]; then
            print_error "Could not determine latest release"
            exit 1
        fi
        
        print_info "Latest version: $LATEST_RELEASE"
    fi
    
    # Detect architecture
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            PULSE_ARCH="amd64"
            ;;
        aarch64)
            PULSE_ARCH="arm64"
            ;;
        armv7l)
            PULSE_ARCH="armv7"
            ;;
        *)
            print_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac
    
    print_info "Detected architecture: $ARCH ($PULSE_ARCH)"
    
    # Download architecture-specific release
    DOWNLOAD_URL="https://github.com/$GITHUB_REPO/releases/download/$LATEST_RELEASE/pulse-${LATEST_RELEASE}-linux-${PULSE_ARCH}.tar.gz"
    print_info "Downloading from: $DOWNLOAD_URL"
    
    # Detect and stop existing service BEFORE downloading (to free the binary)
    EXISTING_SERVICE=$(detect_service_name)
    if systemctl is-active --quiet $EXISTING_SERVICE; then
        print_info "Stopping existing Pulse service ($EXISTING_SERVICE)..."
        systemctl stop $EXISTING_SERVICE
        sleep 2  # Give the process time to fully stop and release the binary
    fi
    
    cd /tmp
    if ! wget -q -O pulse.tar.gz "$DOWNLOAD_URL"; then
        print_error "Failed to download Pulse release"
        exit 1
    fi
    
    # Extract to temporary directory first
    TEMP_EXTRACT="/tmp/pulse-extract-$$"
    mkdir -p "$TEMP_EXTRACT"
    tar -xzf pulse.tar.gz -C "$TEMP_EXTRACT"
    
    # Ensure install directory and bin subdirectory exist
    mkdir -p "$INSTALL_DIR/bin"
    
    # Copy Pulse binary to the correct location (/opt/pulse/bin/pulse)
    if [[ -f "$TEMP_EXTRACT/bin/pulse" ]]; then
        cp "$TEMP_EXTRACT/bin/pulse" "$INSTALL_DIR/bin/pulse"
    elif [[ -f "$TEMP_EXTRACT/pulse" ]]; then
        # Fallback for old archives (pre-v4.3.1)
        cp "$TEMP_EXTRACT/pulse" "$INSTALL_DIR/bin/pulse"
    else
        print_error "Pulse binary not found in archive"
        exit 1
    fi
    
    chmod +x "$INSTALL_DIR/bin/pulse"
    chown -R pulse:pulse "$INSTALL_DIR"
    
    # Create symlink in /usr/local/bin for PATH convenience
    ln -sf "$INSTALL_DIR/bin/pulse" /usr/local/bin/pulse
    print_success "Pulse binary installed to $INSTALL_DIR/bin/pulse"
    print_success "Symlink created at /usr/local/bin/pulse"
    
    # Copy VERSION file if present
    if [[ -f "$TEMP_EXTRACT/VERSION" ]]; then
        cp "$TEMP_EXTRACT/VERSION" "$INSTALL_DIR/VERSION"
        chown pulse:pulse "$INSTALL_DIR/VERSION"
    fi
    
    # Cleanup
    rm -rf "$TEMP_EXTRACT" pulse.tar.gz
}

setup_directories() {
    print_info "Setting up directories..."
    
    # Create directories
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$INSTALL_DIR"
    
    # Set permissions
    chown -R pulse:pulse "$CONFIG_DIR" "$INSTALL_DIR"
    chmod 700 "$CONFIG_DIR"
}

install_systemd_service() {
    print_info "Installing systemd service..."
    
    # Use existing service name if found, otherwise use default
    EXISTING_SERVICE=$(detect_service_name)
    if [[ "$EXISTING_SERVICE" == "pulse-backend" ]] && [[ -f "/etc/systemd/system/pulse-backend.service" ]]; then
        # Keep using pulse-backend for compatibility (ProxmoxVE)
        SERVICE_NAME="pulse-backend"
        print_info "Using existing service name: pulse-backend"
    fi
    
    cat > /etc/systemd/system/$SERVICE_NAME.service << EOF
[Unit]
Description=Pulse Monitoring Server
After=network.target

[Service]
Type=simple
User=pulse
Group=pulse
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/bin/pulse
Restart=always
RestartSec=3
StandardOutput=journal
StandardError=journal
Environment="PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
Environment="PULSE_DATA_DIR=$CONFIG_DIR"

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$INSTALL_DIR $CONFIG_DIR

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
}

start_pulse() {
    print_info "Starting Pulse..."
    systemctl enable $SERVICE_NAME
    systemctl start $SERVICE_NAME
    
    # Wait for service to start
    sleep 3
    
    if systemctl is-active --quiet $SERVICE_NAME; then
        print_success "Pulse started successfully"
    else
        print_error "Failed to start Pulse"
        journalctl -u $SERVICE_NAME -n 20
        exit 1
    fi
}

print_completion() {
    local IP=$(hostname -I | awk '{print $1}')
    
    echo
    print_header
    print_success "Pulse installation completed!"
    echo
    echo -e "${GREEN}Access Pulse at:${NC} http://${IP}:7655"
    echo
    echo -e "${YELLOW}Useful commands:${NC}"
    echo "  systemctl status $SERVICE_NAME    - Check service status"
    echo "  systemctl restart $SERVICE_NAME   - Restart Pulse"
    echo "  journalctl -u $SERVICE_NAME -f    - View logs"
    echo
}

# Main installation flow
main() {
    print_header
    check_root
    detect_os
    check_pre_v4_installation
    
    if check_existing_installation; then
        print_info "Existing Pulse installation detected"
        echo
        echo "What would you like to do?"
        echo "1) Update to latest version"
        echo "2) Reinstall"
        echo "3) Remove"
        echo "4) Cancel"
        read -p "Select option [1-4]: " choice < /dev/tty
        
        case $choice in
            1)
                backup_existing
                systemctl stop $SERVICE_NAME || true
                create_user
                download_pulse
                start_pulse
                print_completion
                ;;
            2)
                backup_existing
                systemctl stop $SERVICE_NAME || true
                create_user
                download_pulse
                setup_directories
                install_systemd_service
                start_pulse
                print_completion
                ;;
            3)
                systemctl stop $SERVICE_NAME || true
                systemctl disable $SERVICE_NAME || true
                rm -f /etc/systemd/system/$SERVICE_NAME.service
                rm -rf "$INSTALL_DIR"
                print_success "Pulse removed successfully"
                ;;
            4)
                print_info "Installation cancelled"
                exit 0
                ;;
            *)
                print_error "Invalid option"
                exit 1
                ;;
        esac
    else
        # Fresh installation
        install_dependencies
        create_user
        setup_directories
        download_pulse
        install_systemd_service
        start_pulse
        print_completion
    fi
}

# Parse command line arguments
FORCE_VERSION=""
FORCE_CHANNEL=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --rc|--pre|--prerelease)
            FORCE_CHANNEL="rc"
            shift
            ;;
        --stable)
            FORCE_CHANNEL="stable"
            shift
            ;;
        --version)
            FORCE_VERSION="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  --rc, --pre        Install latest RC/pre-release version"
            echo "  --stable           Install latest stable version (default)"
            echo "  --version VERSION  Install specific version (e.g., v4.4.0-rc.1)"
            echo "  -h, --help         Show this help message"
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Export for use in download_pulse function
export FORCE_VERSION FORCE_CHANNEL

# Run main function
main