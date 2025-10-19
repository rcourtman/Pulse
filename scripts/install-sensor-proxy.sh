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

configure_local_authorized_key() {
    local auth_line=$1
    local auth_keys_file="/root/.ssh/authorized_keys"
    local tmp_auth

    tmp_auth=$(mktemp)
    mkdir -p /root/.ssh
    touch "$tmp_auth"

    if [[ -f "$auth_keys_file" ]]; then
        grep -vF '# pulse-managed-key' "$auth_keys_file" >"$tmp_auth" 2>/dev/null || true
        chmod --reference="$auth_keys_file" "$tmp_auth" 2>/dev/null || chmod 600 "$tmp_auth"
        chown --reference="$auth_keys_file" "$tmp_auth" 2>/dev/null || true
    else
        chmod 600 "$tmp_auth"
    fi

    echo "${auth_line}" >>"$tmp_auth"
    if mv "$tmp_auth" "$auth_keys_file"; then
        if [ "$QUIET" != true ]; then
            print_success "SSH key configured on localhost"
        fi
    else
        rm -f "$tmp_auth"
        print_warn "Failed to configure SSH key on localhost"
        print_info "Add this line manually to /root/.ssh/authorized_keys:"
        print_info "  ${auth_line}"
    fi
}

# Parse arguments first to check for standalone mode
CTID=""
VERSION="latest"
LOCAL_BINARY=""
QUIET=false
PULSE_SERVER=""
STANDALONE=false
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
        --standalone)
            STANDALONE=true
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

# Check if running on Proxmox host (only required for LXC mode)
if [[ "$STANDALONE" == false ]]; then
    if ! command -v pvecm >/dev/null 2>&1; then
        print_error "This script must be run on a Proxmox VE host"
        exit 1
    fi
fi

# Validate arguments based on mode
if [[ "$STANDALONE" == false ]]; then
    if [[ -z "$CTID" ]]; then
        print_error "Missing required argument: --ctid <container-id>"
        echo "Usage: $0 --ctid <container-id> [--pulse-server <url>] [--version <version>] [--local-binary <path>]"
        echo "   Or: $0 --standalone [--pulse-server <url>] [--version <version>] [--local-binary <path>]"
        exit 1
    fi

    # Verify container exists
    if ! pct status "$CTID" >/dev/null 2>&1; then
        print_error "Container $CTID does not exist"
        exit 1
    fi
fi

if [[ "$STANDALONE" == true ]]; then
    print_info "Installing pulse-sensor-proxy for standalone/Docker deployment"
else
    print_info "Installing pulse-sensor-proxy for container $CTID"
fi

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
install -d -o pulse-sensor-proxy -g pulse-sensor-proxy -m 0755 /etc/pulse-sensor-proxy

# Create config file with ACL for Docker containers (standalone mode)
if [[ "$STANDALONE" == true ]]; then
    print_info "Creating config file with Docker container ACL..."
    cat > /etc/pulse-sensor-proxy/config.yaml << 'EOF'
# Pulse Temperature Proxy Configuration
# Allow Docker containers (UID 1000) to connect
allowed_peer_uids: [1000]

# Allow ID-mapped root (LXC containers with sub-UID mapping)
allow_idmapped_root: true
allowed_idmap_users:
  - root
EOF
    chown pulse-sensor-proxy:pulse-sensor-proxy /etc/pulse-sensor-proxy/config.yaml
    chmod 0644 /etc/pulse-sensor-proxy/config.yaml
fi

# Stop existing service if running (for upgrades)
if systemctl is-active --quiet pulse-sensor-proxy 2>/dev/null; then
    print_info "Stopping existing service for upgrade..."
    systemctl stop pulse-sensor-proxy
fi

# Install hardened systemd service
print_info "Installing hardened systemd service..."

# Generate service file based on mode (Proxmox vs standalone)
if [[ "$STANDALONE" == true ]]; then
    # Standalone/Docker mode - no Proxmox-specific paths
    cat > "$SERVICE_PATH" << 'EOF'
[Unit]
Description=Pulse Temperature Proxy
Documentation=https://github.com/rcourtman/Pulse
After=network.target

[Service]
Type=simple
User=pulse-sensor-proxy
Group=pulse-sensor-proxy
WorkingDirectory=/var/lib/pulse-sensor-proxy
ExecStart=/usr/local/bin/pulse-sensor-proxy --config /etc/pulse-sensor-proxy/config.yaml
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
else
    # Proxmox mode - include Proxmox paths
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
fi

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

# Install cleanup system for automatic SSH key removal when nodes are deleted
print_info "Installing cleanup system..."

# Install cleanup script
CLEANUP_SCRIPT_PATH="/usr/local/bin/pulse-sensor-cleanup.sh"
cat > "$CLEANUP_SCRIPT_PATH" << 'CLEANUP_EOF'
#!/bin/bash

# pulse-sensor-cleanup.sh - Removes Pulse SSH keys from Proxmox nodes when they're removed from Pulse
# This script is triggered by systemd path unit when cleanup-request.json is created

set -euo pipefail

# Configuration
WORK_DIR="/var/lib/pulse-sensor-proxy"
CLEANUP_REQUEST="${WORK_DIR}/cleanup-request.json"
LOG_TAG="pulse-sensor-cleanup"

# Logging functions
log_info() {
    logger -t "$LOG_TAG" -p user.info "$1"
    echo "[INFO] $1"
}

log_warn() {
    logger -t "$LOG_TAG" -p user.warning "$1"
    echo "[WARN] $1"
}

log_error() {
    logger -t "$LOG_TAG" -p user.err "$1"
    echo "[ERROR] $1" >&2
}

# Check if cleanup request file exists
if [[ ! -f "$CLEANUP_REQUEST" ]]; then
    log_info "No cleanup request found at $CLEANUP_REQUEST"
    exit 0
fi

log_info "Processing cleanup request from $CLEANUP_REQUEST"

# Read and parse the cleanup request
CLEANUP_DATA=$(cat "$CLEANUP_REQUEST")
HOST=$(echo "$CLEANUP_DATA" | grep -o '"host":"[^"]*"' | cut -d'"' -f4 || echo "")
REQUESTED_AT=$(echo "$CLEANUP_DATA" | grep -o '"requestedAt":"[^"]*"' | cut -d'"' -f4 || echo "")

log_info "Cleanup requested at: ${REQUESTED_AT:-unknown}"

# Remove the cleanup request file immediately to prevent re-processing
rm -f "$CLEANUP_REQUEST"

# If no specific host was provided, clean up all known nodes
if [[ -z "$HOST" ]]; then
    log_info "No specific host provided - cleaning up all cluster nodes"

    # Discover cluster nodes
    if command -v pvecm >/dev/null 2>&1; then
        CLUSTER_NODES=$(pvecm status 2>/dev/null | awk '/0x[0-9a-f]+.*[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/ {print $3}')

        if [[ -n "$CLUSTER_NODES" ]]; then
            for node_ip in $CLUSTER_NODES; do
                log_info "Cleaning up SSH keys on node $node_ip"

                # Remove both pulse-managed-key and pulse-proxy-key entries
                ssh -o StrictHostKeyChecking=no -o BatchMode=yes -o ConnectTimeout=5 root@"$node_ip" \
                    "sed -i -e '/# pulse-managed-key\$/d' -e '/# pulse-proxy-key\$/d' /root/.ssh/authorized_keys" 2>&1 | \
                    logger -t "$LOG_TAG" -p user.info || \
                    log_warn "Failed to clean up SSH keys on $node_ip"
            done
            log_info "Cluster cleanup completed"
        else
            # Standalone node - clean up localhost
            log_info "Standalone node detected - cleaning up localhost"
            sed -i -e '/# pulse-managed-key$/d' -e '/# pulse-proxy-key$/d' /root/.ssh/authorized_keys 2>&1 | \
                logger -t "$LOG_TAG" -p user.info || \
                log_warn "Failed to clean up SSH keys on localhost"
        fi
    else
        log_warn "pvecm command not available - cleaning up localhost only"
        sed -i -e '/# pulse-managed-key$/d' -e '/# pulse-proxy-key$/d' /root/.ssh/authorized_keys 2>&1 | \
            logger -t "$LOG_TAG" -p user.info || \
            log_warn "Failed to clean up SSH keys on localhost"
    fi
else
    log_info "Cleaning up specific host: $HOST"

    # Extract IP from host URL
    HOST_CLEAN=$(echo "$HOST" | sed -e 's|^https\?://||' -e 's|:.*$||')

    # Check if this is localhost
    LOCAL_IPS=$(hostname -I 2>/dev/null || echo "")
    IS_LOCAL=false

    for local_ip in $LOCAL_IPS; do
        if [[ "$HOST_CLEAN" == "$local_ip" ]]; then
            IS_LOCAL=true
            break
        fi
    done

    if [[ "$HOST_CLEAN" == "127.0.0.1" || "$HOST_CLEAN" == "localhost" ]]; then
        IS_LOCAL=true
    fi

    if [[ "$IS_LOCAL" == true ]]; then
        log_info "Cleaning up localhost SSH keys"
        sed -i -e '/# pulse-managed-key$/d' -e '/# pulse-proxy-key$/d' /root/.ssh/authorized_keys 2>&1 | \
            logger -t "$LOG_TAG" -p user.info || \
            log_warn "Failed to clean up SSH keys on localhost"
    else
        log_info "Cleaning up remote host: $HOST_CLEAN"

        # Try to use proxy's SSH key first (for standalone nodes), fall back to default
        PROXY_KEY="/var/lib/pulse-sensor-proxy/ssh/id_ed25519"
        SSH_CMD="ssh -o StrictHostKeyChecking=no -o BatchMode=yes -o ConnectTimeout=5"

        if [[ -f "$PROXY_KEY" ]]; then
            log_info "Using proxy SSH key for cleanup"
            SSH_CMD="$SSH_CMD -i $PROXY_KEY"
        fi

        # Remove both pulse-managed-key and pulse-proxy-key entries from remote host
        CLEANUP_OUTPUT=$($SSH_CMD root@"$HOST_CLEAN" \
            "sed -i -e '/# pulse-managed-key\$/d' -e '/# pulse-proxy-key\$/d' /root/.ssh/authorized_keys && echo 'SUCCESS'" 2>&1)

        if echo "$CLEANUP_OUTPUT" | grep -q "SUCCESS"; then
            log_info "Successfully cleaned up SSH keys on $HOST_CLEAN"
        else
            # Check if this is a standalone node with forced commands (common case)
            if echo "$CLEANUP_OUTPUT" | grep -q "cpu_thermal\|coretemp\|k10temp"; then
                log_warn "Cannot cleanup standalone node $HOST_CLEAN (forced command prevents cleanup)"
                log_info "Standalone node keys are read-only (sensors -j) - low security risk"
                log_info "Manual cleanup: ssh root@$HOST_CLEAN \"sed -i '/# pulse-proxy-key\$/d' /root/.ssh/authorized_keys\""
            else
                log_error "Failed to clean up SSH keys on $HOST_CLEAN: $CLEANUP_OUTPUT"
                exit 1
            fi
        fi
    fi
fi

log_info "Cleanup completed successfully"
exit 0
CLEANUP_EOF

chmod +x "$CLEANUP_SCRIPT_PATH"
print_info "Cleanup script installed"

# Install systemd path unit
CLEANUP_PATH_UNIT="/etc/systemd/system/pulse-sensor-cleanup.path"
cat > "$CLEANUP_PATH_UNIT" << 'PATH_EOF'
[Unit]
Description=Watch for Pulse sensor cleanup requests
Documentation=https://github.com/rcourtman/Pulse

[Path]
# Watch for the cleanup request file
PathChanged=/var/lib/pulse-sensor-proxy/cleanup-request.json
# Also watch for modifications
PathModified=/var/lib/pulse-sensor-proxy/cleanup-request.json

[Install]
WantedBy=multi-user.target
PATH_EOF

# Install systemd service unit
CLEANUP_SERVICE_UNIT="/etc/systemd/system/pulse-sensor-cleanup.service"
cat > "$CLEANUP_SERVICE_UNIT" << 'SERVICE_EOF'
[Unit]
Description=Pulse Sensor Cleanup Service
Documentation=https://github.com/rcourtman/Pulse
After=network.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/pulse-sensor-cleanup.sh
User=root
Group=root
WorkingDirectory=/var/lib/pulse-sensor-proxy

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=pulse-sensor-cleanup

# Security hardening (less restrictive than the proxy since we need SSH access)
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/var/lib/pulse-sensor-proxy /root/.ssh
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
PrivateTmp=true
RestrictSUIDSGID=true
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
LimitNOFILE=1024

[Install]
# This service is triggered by the .path unit, no need to enable it directly
SERVICE_EOF

# Enable and start the path unit
systemctl daemon-reload
systemctl enable pulse-sensor-cleanup.path
systemctl start pulse-sensor-cleanup.path

print_info "Cleanup system installed and enabled"

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
        AUTH_LINE="${FORCED_CMD} ${PROXY_PUBLIC_KEY} # pulse-managed-key"

        # Track SSH key push results
        SSH_SUCCESS_COUNT=0
        SSH_FAILURE_COUNT=0
        declare -a SSH_FAILED_NODES=()
        LOCAL_IPS=$(hostname -I 2>/dev/null || echo "")
        LOCAL_HOSTNAMES="$(hostname 2>/dev/null || echo "") $(hostname -f 2>/dev/null || echo "")"
        LOCAL_HANDLED=false

        # Push key to each cluster node
        for node_ip in $CLUSTER_NODES; do
            print_info "Authorizing proxy key on node $node_ip..."

            IS_LOCAL=false
            # Check if node_ip matches any of the local IPs (exact match with word boundaries)
            for local_ip in $LOCAL_IPS; do
                if [[ "$node_ip" == "$local_ip" ]]; then
                    IS_LOCAL=true
                    break
                fi
            done
            if [[ " $LOCAL_HOSTNAMES " == *" $node_ip "* ]]; then
                IS_LOCAL=true
            fi
            if [[ "$node_ip" == "127.0.0.1" || "$node_ip" == "localhost" ]]; then
                IS_LOCAL=true
            fi

            if [[ "$IS_LOCAL" = true ]]; then
                configure_local_authorized_key "$AUTH_LINE"
                LOCAL_HANDLED=true
                ((SSH_SUCCESS_COUNT+=1))
                continue
            fi

            # Remove any existing proxy keys first
            ssh -o StrictHostKeyChecking=no -o BatchMode=yes -o ConnectTimeout=5 root@"$node_ip" \
                "sed -i '/# pulse-managed-key\$/d' /root/.ssh/authorized_keys" 2>/dev/null || true

            # Add new key with forced command
            SSH_ERROR=$(ssh -o StrictHostKeyChecking=no -o BatchMode=yes -o ConnectTimeout=5 root@"$node_ip" \
                "echo '${AUTH_LINE}' >> /root/.ssh/authorized_keys" 2>&1)
            if [[ $? -eq 0 ]]; then
                print_success "SSH key configured on $node_ip"
                ((SSH_SUCCESS_COUNT+=1))
            else
                print_warn "Failed to configure SSH key on $node_ip"
                ((SSH_FAILURE_COUNT+=1))
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
        if [[ "$LOCAL_HANDLED" = false ]]; then
            configure_local_authorized_key "$AUTH_LINE"
            ((SSH_SUCCESS_COUNT+=1))
        fi
    else
        # No cluster found - configure standalone node
        print_info "No cluster detected, configuring standalone node..."

        # Configure SSH key with forced command restriction
        FORCED_CMD='command="sensors -j",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty'
        AUTH_LINE="${FORCED_CMD} ${PROXY_PUBLIC_KEY} # pulse-managed-key"

        print_info "Authorizing proxy key on localhost..."
        configure_local_authorized_key "$AUTH_LINE"
        print_info ""
        print_info "Standalone node configuration complete"
    fi
else
    # Proxmox host but pvecm not available (shouldn't happen, but handle it)
    print_warn "pvecm command not available"
    print_info "Configuring SSH key for localhost..."

    # Configure localhost as fallback
    FORCED_CMD='command="sensors -j",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty'
    AUTH_LINE="${FORCED_CMD} ${PROXY_PUBLIC_KEY} # pulse-managed-key"

    configure_local_authorized_key "$AUTH_LINE"
fi

# Container-specific configuration (skip for standalone mode)
if [[ "$STANDALONE" == false ]]; then
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
fi  # End of container-specific configuration

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
