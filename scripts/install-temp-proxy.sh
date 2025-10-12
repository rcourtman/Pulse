#!/bin/bash

# install-temp-proxy.sh - Installs pulse-temp-proxy on Proxmox host for secure temperature monitoring
# This script is idempotent and can be safely re-run

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running on Proxmox host
if ! command -v pvecm >/dev/null 2>&1; then
    print_error "This script must be run on a Proxmox VE host"
    exit 1
fi

# Parse arguments
CTID=""
VERSION="latest"
LOCAL_BINARY=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --ctid)
            CTID="$2"
            shift 2
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        --local-binary)
            LOCAL_BINARY="$2"
            shift 2
            ;;
        *)
            print_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

if [[ -z "$CTID" ]]; then
    print_error "Missing required argument: --ctid <container-id>"
    echo "Usage: $0 --ctid <container-id> [--version <version>] [--local-binary <path>]"
    exit 1
fi

# Verify container exists
if ! pct status "$CTID" >/dev/null 2>&1; then
    print_error "Container $CTID does not exist"
    exit 1
fi

print_info "Installing pulse-temp-proxy for container $CTID"

BINARY_PATH="/usr/local/bin/pulse-temp-proxy"
SERVICE_PATH="/etc/systemd/system/pulse-temp-proxy.service"
RUNTIME_DIR="/run/pulse-temp-proxy"
SOCKET_PATH="/run/pulse-temp-proxy/pulse-temp-proxy.sock"
SSH_DIR="/var/lib/pulse-temp-proxy/ssh"

# Install binary - either from local file or download from GitHub
if [[ -n "$LOCAL_BINARY" ]]; then
    # Use local binary for testing
    print_info "Using local binary: $LOCAL_BINARY"
    if [[ ! -f "$LOCAL_BINARY" ]]; then
        print_error "Local binary not found: $LOCAL_BINARY"
        exit 1
    fi
    cp "$LOCAL_BINARY" "$BINARY_PATH"
    chmod +x "$BINARY_PATH"
    print_info "Binary installed to $BINARY_PATH"
else
    # Download from GitHub release
    GITHUB_REPO="rcourtman/Pulse"
    if [[ "$VERSION" == "latest" ]]; then
        RELEASE_URL="https://api.github.com/repos/$GITHUB_REPO/releases/latest"
        print_info "Fetching latest release info..."
        RELEASE_DATA=$(curl -fsSL "$RELEASE_URL")
        VERSION=$(echo "$RELEASE_DATA" | grep -o '"tag_name": "[^"]*"' | cut -d'"' -f4)
        if [[ -z "$VERSION" ]]; then
            print_error "Failed to determine latest version"
            exit 1
        fi
        print_info "Latest version: $VERSION"
    fi

    # Detect architecture
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            BINARY_NAME="pulse-temp-proxy-linux-amd64"
            ;;
        aarch64|arm64)
            BINARY_NAME="pulse-temp-proxy-linux-arm64"
            ;;
        armv7l|armhf)
            BINARY_NAME="pulse-temp-proxy-linux-armv7"
            ;;
        *)
            print_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    DOWNLOAD_URL="https://github.com/$GITHUB_REPO/releases/download/$VERSION/$BINARY_NAME"

    # Download binary
    print_info "Downloading $BINARY_NAME..."
    if ! curl -fsSL "$DOWNLOAD_URL" -o "$BINARY_PATH.tmp"; then
        print_error "Failed to download binary from $DOWNLOAD_URL"
        exit 1
    fi

    # Make executable and move to final location
    chmod +x "$BINARY_PATH.tmp"
    mv "$BINARY_PATH.tmp" "$BINARY_PATH"
    print_info "Binary installed to $BINARY_PATH"
fi

# Create SSH key directory
mkdir -p "$SSH_DIR"
chmod 700 "$SSH_DIR"

# Install systemd service
print_info "Installing systemd service..."
cat > "$SERVICE_PATH" << 'EOF'
[Unit]
Description=Pulse Temperature Proxy
Documentation=https://github.com/rcourtman/Pulse
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/pulse-temp-proxy
Restart=on-failure
RestartSec=5s

# Runtime directory for socket
RuntimeDirectory=pulse-temp-proxy
RuntimeDirectoryMode=0770

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/pulse-temp-proxy

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=pulse-temp-proxy

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd and start service
print_info "Enabling and starting service..."
systemctl daemon-reload
systemctl enable pulse-temp-proxy.service
systemctl restart pulse-temp-proxy.service

# Wait for socket to appear
print_info "Waiting for socket..."
for i in {1..10}; do
    if [[ -S "$SOCKET_PATH" ]]; then
        break
    fi
    sleep 1
done

if [[ ! -S "$SOCKET_PATH" ]]; then
    print_error "Socket did not appear after 10 seconds"
    print_info "Check service status: systemctl status pulse-temp-proxy"
    exit 1
fi

print_info "Socket ready at $SOCKET_PATH"

# Configure LXC bind mount - mount entire directory for socket stability
LXC_CONFIG="/etc/pve/lxc/${CTID}.conf"
BIND_ENTRY="lxc.mount.entry: /run/pulse-temp-proxy run/pulse-temp-proxy none bind,create=dir 0 0"

# Check if bind mount already exists
if grep -q "pulse-temp-proxy" "$LXC_CONFIG"; then
    print_info "Bind mount already configured in LXC config"
    # Remove old socket-level bind if it exists
    if grep -q "pulse-temp-proxy.sock" "$LXC_CONFIG"; then
        print_info "Upgrading from socket-level to directory-level bind mount..."
        sed -i '/pulse-temp-proxy\.sock/d' "$LXC_CONFIG"
        echo "$BIND_ENTRY" >> "$LXC_CONFIG"
        NEEDS_RESTART=true
    fi
else
    print_info "Adding bind mount to LXC config..."
    echo "$BIND_ENTRY" >> "$LXC_CONFIG"
    NEEDS_RESTART=true
fi

# Restart container to apply bind mount if needed
if [[ "${NEEDS_RESTART:-false}" == "true" ]]; then
    print_info "Restarting container to apply bind mount..."
    pct stop "$CTID" || true
    sleep 2
    pct start "$CTID"
    sleep 5
fi

# Verify socket is accessible in container
print_info "Verifying socket accessibility..."
if pct exec "$CTID" -- test -S /run/pulse-temp-proxy/pulse-temp-proxy.sock; then
    print_info "Socket is accessible in container"
else
    print_warn "Socket is not yet accessible in container"
    print_info "Container may need additional restart or configuration"
fi

# Test proxy status
print_info "Testing proxy status..."
if systemctl is-active --quiet pulse-temp-proxy; then
    print_info "${GREEN}✓${NC} pulse-temp-proxy is running"
else
    print_error "pulse-temp-proxy is not running"
    print_info "Check logs: journalctl -u pulse-temp-proxy -n 50"
    exit 1
fi

print_info "${GREEN}Installation complete!${NC}"
print_info ""
print_info "Temperature monitoring will now use the secure host-side proxy"
print_info "SSH keys are stored on the Proxmox host, not in the container"
print_info ""
print_info "To configure temperature monitoring for cluster nodes:"
print_info "  1. Access Pulse UI in container $CTID"
print_info "  2. Go to Settings → Enable Temperature Monitoring"
print_info "  3. The proxy will automatically discover and configure cluster nodes"
print_info ""
print_info "To check proxy status:"
print_info "  systemctl status pulse-temp-proxy"
print_info "  journalctl -u pulse-temp-proxy -f"

exit 0
