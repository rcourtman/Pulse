#!/bin/bash

# install-sensor-proxy.sh - Installs pulse-sensor-proxy on Proxmox host for secure temperature monitoring
# This script is idempotent and can be safely re-run

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_info() {
    if [ "$QUIET" != true ]; then
        echo -e "${GREEN}[INFO]${NC} $1"
    fi
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
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
QUIET=false
PULSE_SERVER=""
FALLBACK_BASE="${PULSE_SENSOR_PROXY_FALLBACK_URL:-}"

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
        --pulse-server)
            PULSE_SERVER="$2"
            shift 2
            ;;
        --quiet)
            QUIET=true
            shift
            ;;
        *)
            print_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# If --pulse-server was provided, use it as the fallback base
if [[ -n "$PULSE_SERVER" ]]; then
    FALLBACK_BASE="${PULSE_SERVER}/api/install/pulse-sensor-proxy"
fi

if [[ -z "$CTID" ]]; then
    print_error "Missing required argument: --ctid <container-id>"
    echo "Usage: $0 --ctid <container-id> [--pulse-server <url>] [--version <version>] [--local-binary <path>]"
    exit 1
fi

# Verify container exists
if ! pct status "$CTID" >/dev/null 2>&1; then
    print_error "Container $CTID does not exist"
    exit 1
fi

print_info "Installing pulse-sensor-proxy for container $CTID"

BINARY_PATH="/usr/local/bin/pulse-sensor-proxy"
SERVICE_PATH="/etc/systemd/system/pulse-sensor-proxy.service"
RUNTIME_DIR="/run/pulse-sensor-proxy"
SOCKET_PATH="/run/pulse-sensor-proxy/pulse-sensor-proxy.sock"
SSH_DIR="/var/lib/pulse-sensor-proxy/ssh"

# Create dedicated service account if it doesn't exist
if ! id -u pulse-sensor-proxy >/dev/null 2>&1; then
    print_info "Creating pulse-sensor-proxy service account..."
    useradd --system --user-group --no-create-home --shell /usr/sbin/nologin pulse-sensor-proxy
    print_info "Service account created"
else
    print_info "Service account pulse-sensor-proxy already exists"
fi

# Add pulse-sensor-proxy user to www-data group for Proxmox IPC access (pvecm commands)
if ! groups pulse-sensor-proxy | grep -q '\bwww-data\b'; then
    print_info "Adding pulse-sensor-proxy to www-data group for Proxmox IPC access..."
    usermod -aG www-data pulse-sensor-proxy
fi

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
    # Detect architecture
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            BINARY_NAME="pulse-sensor-proxy-linux-amd64"
            ARCH_LABEL="linux-amd64"
            ;;
        aarch64|arm64)
            BINARY_NAME="pulse-sensor-proxy-linux-arm64"
            ARCH_LABEL="linux-arm64"
            ;;
        armv7l|armhf)
            BINARY_NAME="pulse-sensor-proxy-linux-armv7"
            ARCH_LABEL="linux-armv7"
            ;;
        *)
            print_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    # If fallback URL is provided (e.g., from Pulse setup script), use it directly
    if [[ -n "$FALLBACK_BASE" ]]; then
        FALLBACK_URL="${FALLBACK_BASE%/}?arch=${ARCH_LABEL}"
        print_info "Downloading $BINARY_NAME from Pulse server..."
        if ! curl -fsSL "$FALLBACK_URL" -o "$BINARY_PATH.tmp"; then
            print_error "Failed to download proxy binary from $FALLBACK_URL"
            exit 1
        fi
    else
        # Fallback not provided, download from GitHub release
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

        DOWNLOAD_URL="https://github.com/$GITHUB_REPO/releases/download/$VERSION/$BINARY_NAME"
        print_info "Downloading $BINARY_NAME from GitHub..."
        if ! curl -fsSL "$DOWNLOAD_URL" -o "$BINARY_PATH.tmp"; then
            print_error "Failed to download binary from $DOWNLOAD_URL"
            exit 1
        fi
    fi

    # Make executable and move to final location
    chmod +x "$BINARY_PATH.tmp"
    mv "$BINARY_PATH.tmp" "$BINARY_PATH"
    print_info "Binary installed to $BINARY_PATH"
fi

# Create directories with proper ownership (handles fresh installs and upgrades)
print_info "Setting up directories with proper ownership..."
install -d -o pulse-sensor-proxy -g pulse-sensor-proxy -m 0750 /var/lib/pulse-sensor-proxy
install -d -o pulse-sensor-proxy -g pulse-sensor-proxy -m 0700 "$SSH_DIR"

# Stop existing service if running (for upgrades)
if systemctl is-active --quiet pulse-sensor-proxy 2>/dev/null; then
    print_info "Stopping existing service for upgrade..."
    systemctl stop pulse-sensor-proxy
fi

# Install hardened systemd service
print_info "Installing hardened systemd service..."
cat > "$SERVICE_PATH" << 'EOF'
[Unit]
Description=Pulse Temperature Proxy
Documentation=https://github.com/rcourtman/Pulse
After=network.target

[Service]
Type=simple
User=pulse-sensor-proxy
Group=pulse-sensor-proxy
SupplementaryGroups=www-data
WorkingDirectory=/var/lib/pulse-sensor-proxy
ExecStart=/usr/local/bin/pulse-sensor-proxy
Restart=on-failure
RestartSec=5s

# Runtime dirs/sockets
RuntimeDirectory=pulse-sensor-proxy
RuntimeDirectoryMode=0775
RuntimeDirectoryPreserve=yes
UMask=0007

# Core hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/var/lib/pulse-sensor-proxy
ReadOnlyPaths=/run/pve-cluster /etc/pve
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
ProtectClock=true
PrivateTmp=true
PrivateDevices=true
ProtectProc=invisible
ProcSubset=pid
LockPersonality=true
RemoveIPC=true
RestrictSUIDSGID=true
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
RestrictNamespaces=true
SystemCallFilter=@system-service
SystemCallErrorNumber=EPERM
CapabilityBoundingSet=
AmbientCapabilities=
KeyringMode=private
LimitNOFILE=1024

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=pulse-sensor-proxy

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd and start service
print_info "Enabling and starting service..."
systemctl daemon-reload
systemctl enable pulse-sensor-proxy.service
systemctl restart pulse-sensor-proxy.service

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
    print_info "Check service status: systemctl status pulse-sensor-proxy"
    exit 1
fi

print_info "Socket ready at $SOCKET_PATH"

# Configure SSH keys for cluster temperature monitoring
print_info "Configuring proxy SSH access to cluster nodes..."

# Wait for proxy to generate SSH keys
PROXY_KEY_FILE="$SSH_DIR/id_ed25519.pub"
for i in {1..10}; do
    if [[ -f "$PROXY_KEY_FILE" ]]; then
        break
    fi
    sleep 1
done

if [[ ! -f "$PROXY_KEY_FILE" ]]; then
    print_error "Proxy SSH key not generated after 10 seconds"
    print_info "Check service logs: journalctl -u pulse-sensor-proxy -n 50"
    exit 1
fi

PROXY_PUBLIC_KEY=$(cat "$PROXY_KEY_FILE")
print_info "Proxy public key: ${PROXY_PUBLIC_KEY:0:50}..."

# Discover cluster nodes
if command -v pvecm >/dev/null 2>&1; then
    # Extract node IPs from pvecm status
    CLUSTER_NODES=$(pvecm status 2>/dev/null | awk '/0x[0-9a-f]+.*[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/ {print $3}')

    if [[ -n "$CLUSTER_NODES" ]]; then
        print_info "Discovered cluster nodes: $(echo $CLUSTER_NODES | tr '\n' ' ')"

        # Configure SSH key with forced command restriction
        FORCED_CMD='command="sensors -j",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty'
        AUTH_LINE="${FORCED_CMD} ${PROXY_PUBLIC_KEY}"

        # Track SSH key push results
        SSH_SUCCESS_COUNT=0
        SSH_FAILURE_COUNT=0
        declare -a SSH_FAILED_NODES=()

        # Push key to each cluster node
        for node_ip in $CLUSTER_NODES; do
            print_info "Authorizing proxy key on node $node_ip..."

            # Remove any existing proxy keys first
            ssh -o StrictHostKeyChecking=no -o BatchMode=yes -o ConnectTimeout=5 root@"$node_ip" \
                "sed -i '/pulse-sensor-proxy\$/d' /root/.ssh/authorized_keys" 2>/dev/null || true

            # Add new key with forced command
            SSH_ERROR=$(ssh -o StrictHostKeyChecking=no -o BatchMode=yes -o ConnectTimeout=5 root@"$node_ip" \
                "echo '${AUTH_LINE}' >> /root/.ssh/authorized_keys" 2>&1)
            if [[ $? -eq 0 ]]; then
                print_success "SSH key configured on $node_ip"
                ((SSH_SUCCESS_COUNT++))
            else
                print_warn "Failed to configure SSH key on $node_ip"
                ((SSH_FAILURE_COUNT++))
                SSH_FAILED_NODES+=("$node_ip")
                # Log detailed error for debugging
                if [[ -n "$SSH_ERROR" ]]; then
                    print_info "  Error details: $(echo "$SSH_ERROR" | head -1)"
                fi
            fi
        done

        # Print summary
        print_info ""
        print_info "SSH key configuration summary:"
        print_info "  ✓ Success: $SSH_SUCCESS_COUNT node(s)"
        if [[ $SSH_FAILURE_COUNT -gt 0 ]]; then
            print_warn "  ✗ Failed: $SSH_FAILURE_COUNT node(s) - ${SSH_FAILED_NODES[*]}"
            print_info ""
            print_info "To retry failed nodes, re-run this script or manually run:"
            print_info "  ssh root@<node> 'echo \"${AUTH_LINE}\" >> /root/.ssh/authorized_keys'"
        fi
    else
        # No cluster found - configure standalone node
        print_info "No cluster detected, configuring standalone node..."

        # Configure SSH key with forced command restriction
        FORCED_CMD='command="sensors -j",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty'
        AUTH_LINE="${FORCED_CMD} ${PROXY_PUBLIC_KEY}"

        # Configure localhost
        print_info "Authorizing proxy key on localhost..."

        # Remove any existing proxy keys first
        sed -i '/pulse-sensor-proxy$/d' /root/.ssh/authorized_keys 2>/dev/null || touch /root/.ssh/authorized_keys

        # Add new key with forced command
        if echo "${AUTH_LINE}" >> /root/.ssh/authorized_keys; then
            print_success "SSH key configured on standalone node"
            print_info ""
            print_info "Standalone node configuration complete"
        else
            print_warn "Failed to configure SSH key on localhost"
            print_info "Manually add this line to /root/.ssh/authorized_keys:"
            print_info "  ${AUTH_LINE}"
        fi
    fi
else
    # Proxmox host but pvecm not available (shouldn't happen, but handle it)
    print_warn "pvecm command not available"
    print_info "Configuring SSH key for localhost..."

    # Configure localhost as fallback
    FORCED_CMD='command="sensors -j",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty'
    AUTH_LINE="${FORCED_CMD} ${PROXY_PUBLIC_KEY}"

    sed -i '/pulse-sensor-proxy$/d' /root/.ssh/authorized_keys 2>/dev/null || touch /root/.ssh/authorized_keys
    if echo "${AUTH_LINE}" >> /root/.ssh/authorized_keys; then
        print_success "SSH key configured on localhost"
    else
        print_warn "Failed to configure SSH key"
    fi
fi

# Ensure container mount via mp configuration
print_info "Ensuring container socket mount configuration..."
MOUNT_TARGET="/mnt/pulse-proxy"
LXC_CONFIG="/etc/pve/lxc/${CTID}.conf"
CONFIG_CONTENT=$(pct config "$CTID")
CURRENT_MP=$(pct config "$CTID" | awk -v target="$MOUNT_TARGET" '$1 ~ /^mp[0-9]+:$/ && index($0, "mp=" target) {split($1, arr, ":"); print arr[1]; exit}')
MOUNT_UPDATED=false
HOTPLUG_FAILED=false
CT_RUNNING=false
if pct status "$CTID" 2>/dev/null | grep -q "running"; then
    CT_RUNNING=true
fi

if [[ -z "$CURRENT_MP" ]]; then
    for idx in $(seq 0 9); do
        if ! printf "%s\n" "$CONFIG_CONTENT" | grep -q "^mp${idx}:"; then
            CURRENT_MP="mp${idx}"
            break
        fi
    done
    if [[ -z "$CURRENT_MP" ]]; then
        print_error "Unable to find available mp slot for container mount"
        exit 1
    fi
    print_info "Configuring container mount using $CURRENT_MP..."
    if pct set "$CTID" -${CURRENT_MP} "/run/pulse-sensor-proxy,mp=${MOUNT_TARGET},replicate=0" 2>/dev/null; then
        MOUNT_UPDATED=true
    else
        HOTPLUG_FAILED=true
    fi
else
    print_info "Container already has socket mount configured ($CURRENT_MP)"
    if pct set "$CTID" -${CURRENT_MP} "/run/pulse-sensor-proxy,mp=${MOUNT_TARGET},replicate=0" 2>/dev/null; then
        MOUNT_UPDATED=true
    else
        HOTPLUG_FAILED=true
    fi
fi

if [[ "$HOTPLUG_FAILED" = true ]]; then
    print_warn "Hot-plugging socket mount failed (container may be running). Updating config directly."
    CURRENT_MP_LINE="${CURRENT_MP}: /run/pulse-sensor-proxy,mp=${MOUNT_TARGET},replicate=0"
    if ! grep -q "^${CURRENT_MP}:" "$LXC_CONFIG" 2>/dev/null; then
        echo "$CURRENT_MP_LINE" >> "$LXC_CONFIG"
    else
        sed -i "s#^${CURRENT_MP}:.*#${CURRENT_MP_LINE}#" "$LXC_CONFIG"
    fi
    MOUNT_UPDATED=true
fi

# Remove legacy lxc.mount.entry directives if present
if grep -q "lxc.mount.entry: /run/pulse-sensor-proxy" "$LXC_CONFIG"; then
    print_info "Removing legacy lxc.mount.entry directives for pulse-sensor-proxy"
    sed -i '/lxc\.mount\.entry: \/run\/pulse-sensor-proxy/d' "$LXC_CONFIG"
    MOUNT_UPDATED=true
fi

# Restart container to apply mount if configuration changed or mount missing
if [[ "$MOUNT_UPDATED" = true ]]; then
    if [[ "$CT_RUNNING" = true ]]; then
        print_warn "Container $CTID is currently running. Restart it when convenient to activate the secure proxy mount."
    else
        print_info "Restarting container to apply socket mount..."
        pct stop "$CTID" || true
        sleep 2
        pct start "$CTID"
        sleep 5
    fi
fi

# Verify socket directory and file inside container
if [[ "$HOTPLUG_FAILED" = true && "$CT_RUNNING" = true ]]; then
    print_warn "Skipping socket verification until container $CTID is restarted."
else
    print_info "Verifying socket accessibility..."
    if pct exec "$CTID" -- test -S "${MOUNT_TARGET}/pulse-sensor-proxy.sock"; then
        print_info "Socket is accessible in container at ${MOUNT_TARGET}/pulse-sensor-proxy.sock"
    else
        print_warn "Socket not visible at ${MOUNT_TARGET}/pulse-sensor-proxy.sock"
        print_info "Check container configuration and restart if necessary"
    fi
fi

# Configure Pulse backend environment override inside container
print_info "Configuring Pulse backend to use mounted proxy socket..."
pct exec "$CTID" -- bash -lc "mkdir -p /etc/systemd/system/pulse-backend.service.d"
pct exec "$CTID" -- bash -lc "cat <<'EOF' >/etc/systemd/system/pulse-backend.service.d/10-pulse-proxy.conf
[Service]
Environment=PULSE_SENSOR_PROXY_SOCKET=${MOUNT_TARGET}/pulse-sensor-proxy.sock
EOF"
pct exec "$CTID" -- systemctl daemon-reload || true

# Test proxy status
print_info "Testing proxy status..."
if systemctl is-active --quiet pulse-sensor-proxy; then
    print_info "${GREEN}✓${NC} pulse-sensor-proxy is running"
else
    print_error "pulse-sensor-proxy is not running"
    print_info "Check logs: journalctl -u pulse-sensor-proxy -n 50"
    exit 1
fi

# Check for and remove legacy SSH keys from container
print_info "Checking for legacy SSH keys in container..."
LEGACY_KEYS_FOUND=false
for key_type in id_rsa id_dsa id_ecdsa id_ed25519; do
    if pct exec "$CTID" -- test -f "/root/.ssh/$key_type" 2>/dev/null; then
        LEGACY_KEYS_FOUND=true
        if [ "$QUIET" != true ]; then
            print_warn "Found legacy SSH key: /root/.ssh/$key_type"
        fi
        pct exec "$CTID" -- rm -f "/root/.ssh/$key_type" "/root/.ssh/${key_type}.pub"
        print_info "  Removed /root/.ssh/$key_type"
    fi
done

if [ "$LEGACY_KEYS_FOUND" = true ] && [ "$QUIET" != true ]; then
    print_info ""
    print_info "Legacy SSH keys removed from container for security"
    print_info ""
fi

if [ "$QUIET" = true ]; then
    print_success "pulse-sensor-proxy installed and running"
else
    print_info "${GREEN}Installation complete!${NC}"
    print_info ""
    print_info "Temperature monitoring will use the secure host-side proxy"
    print_info ""
    print_info "To check proxy status:"
    print_info "  systemctl status pulse-sensor-proxy"
fi

exit 0
