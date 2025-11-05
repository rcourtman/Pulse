#!/bin/bash

# install-sensor-proxy.sh - Installs pulse-sensor-proxy on Proxmox host for secure temperature monitoring
# Supports --uninstall [--purge] to remove the proxy and cleanup resources.
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

BINARY_PATH="/usr/local/bin/pulse-sensor-proxy"
SERVICE_PATH="/etc/systemd/system/pulse-sensor-proxy.service"
RUNTIME_DIR="/run/pulse-sensor-proxy"
SOCKET_PATH="${RUNTIME_DIR}/pulse-sensor-proxy.sock"
WORK_DIR="/var/lib/pulse-sensor-proxy"
SSH_DIR="${WORK_DIR}/ssh"
CONFIG_DIR="/etc/pulse-sensor-proxy"
CTID_FILE="${CONFIG_DIR}/ctid"
CLEANUP_SCRIPT_PATH="/usr/local/bin/pulse-sensor-cleanup.sh"
CLEANUP_PATH_UNIT="/etc/systemd/system/pulse-sensor-cleanup.path"
CLEANUP_SERVICE_UNIT="/etc/systemd/system/pulse-sensor-cleanup.service"
CLEANUP_REQUEST_PATH="${WORK_DIR}/cleanup-request.json"
SERVICE_USER="pulse-sensor-proxy"
LOG_DIR="/var/log/pulse/sensor-proxy"
SHARE_DIR="/usr/local/share/pulse"
STORED_INSTALLER="${SHARE_DIR}/install-sensor-proxy.sh"
SELFHEAL_SCRIPT="/usr/local/bin/pulse-sensor-proxy-selfheal.sh"
SELFHEAL_SERVICE_UNIT="/etc/systemd/system/pulse-sensor-proxy-selfheal.service"
SELFHEAL_TIMER_UNIT="/etc/systemd/system/pulse-sensor-proxy-selfheal.timer"
SCRIPT_SOURCE="$(readlink -f "${BASH_SOURCE[0]:-$0}" 2>/dev/null || printf '%s' "${BASH_SOURCE[0]:-$0}")"

cleanup_local_authorized_keys() {
    local auth_keys_file="/root/.ssh/authorized_keys"
    if [[ ! -f "$auth_keys_file" ]]; then
        return
    fi

    if grep -q '# pulse-\(managed\|proxy\)-key$' "$auth_keys_file"; then
        if sed -i -e '/# pulse-managed-key$/d' -e '/# pulse-proxy-key$/d' "$auth_keys_file"; then
            print_info "Removed Pulse SSH keys from ${auth_keys_file}"
        else
            print_warn "Failed to clean Pulse SSH keys from ${auth_keys_file}"
        fi
    fi
}

cleanup_cluster_authorized_keys_manual() {
    local nodes=()
    if command -v pvecm >/dev/null 2>&1; then
        while IFS= read -r node_ip; do
            [[ -n "$node_ip" ]] && nodes+=("$node_ip")
        done < <(pvecm status 2>/dev/null | awk '/0x[0-9a-f]+.*[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/ {print $3}' || true)
    fi

    if [[ ${#nodes[@]} -eq 0 ]]; then
        cleanup_local_authorized_keys
        return
    fi

    local local_ips
    local_ips="$(hostname -I 2>/dev/null || echo "")"
    local local_hostnames
    local_hostnames="$(hostname 2>/dev/null || echo "") $(hostname -f 2>/dev/null || echo "")"

    for node_ip in "${nodes[@]}"; do
        local is_local=false
        for local_ip in $local_ips; do
            if [[ "$node_ip" == "$local_ip" ]]; then
                is_local=true
                break
            fi
        done

        if [[ " $local_hostnames " == *" $node_ip "* ]]; then
            is_local=true
        fi

        if [[ "$node_ip" == "127.0.0.1" || "$node_ip" == "localhost" ]]; then
            is_local=true
        fi

        if [[ "$is_local" == true ]]; then
            cleanup_local_authorized_keys
            continue
        fi

        print_info "Removing Pulse SSH keys from node ${node_ip}"
        if ssh -o StrictHostKeyChecking=no -o BatchMode=yes -o ConnectTimeout=5 root@"$node_ip" \
            "sed -i -e '/# pulse-managed-key\$/d' -e '/# pulse-proxy-key\$/d' /root/.ssh/authorized_keys" 2>/dev/null; then
            print_info "  SSH keys cleaned on ${node_ip}"
        else
            print_warn "  Unable to clean Pulse SSH keys on ${node_ip}"
        fi
    done

    cleanup_local_authorized_keys
}

perform_uninstall() {
    print_info "Starting pulse-sensor-proxy uninstall..."

    if command -v systemctl >/dev/null 2>&1; then
        print_info "Stopping pulse-sensor-proxy service"
        systemctl stop pulse-sensor-proxy 2>/dev/null || true
        print_info "Disabling pulse-sensor-proxy service"
        systemctl disable pulse-sensor-proxy 2>/dev/null || true

        print_info "Stopping cleanup path watcher"
        systemctl stop pulse-sensor-cleanup.path 2>/dev/null || true
        systemctl disable pulse-sensor-cleanup.path 2>/dev/null || true
        systemctl stop pulse-sensor-cleanup.service 2>/dev/null || true
        systemctl disable pulse-sensor-cleanup.service 2>/dev/null || true
    else
        print_warn "systemctl not available; skipping service disable"
    fi

    if [[ -x "$CLEANUP_SCRIPT_PATH" ]]; then
        print_info "Invoking cleanup script to remove Pulse SSH keys"
        mkdir -p "$WORK_DIR"
        cat > "$CLEANUP_REQUEST_PATH" <<'EOF'
{"host":""}
EOF
        if "$CLEANUP_SCRIPT_PATH"; then
            print_success "Cleanup script removed Pulse SSH keys"
        else
            print_warn "Cleanup script reported errors; attempting manual cleanup"
            cleanup_cluster_authorized_keys_manual
        fi
        rm -f "$CLEANUP_REQUEST_PATH"
    else
        cleanup_cluster_authorized_keys_manual
    fi

    if [[ -f "$BINARY_PATH" ]]; then
        rm -f "$BINARY_PATH"
        print_success "Removed binary ${BINARY_PATH}"
    else
        print_info "Binary already absent at ${BINARY_PATH}"
    fi

    if [[ -f "$SERVICE_PATH" ]]; then
        rm -f "$SERVICE_PATH"
        print_success "Removed service unit ${SERVICE_PATH}"
    fi

    if [[ -f "$CLEANUP_PATH_UNIT" ]]; then
        rm -f "$CLEANUP_PATH_UNIT"
        print_success "Removed cleanup path unit ${CLEANUP_PATH_UNIT}"
    fi

    if [[ -f "$CLEANUP_SERVICE_UNIT" ]]; then
        rm -f "$CLEANUP_SERVICE_UNIT"
        print_success "Removed cleanup service unit ${CLEANUP_SERVICE_UNIT}"
    fi

    if [[ -f "$SELFHEAL_TIMER_UNIT" ]]; then
        systemctl stop pulse-sensor-proxy-selfheal.timer 2>/dev/null || true
        systemctl disable pulse-sensor-proxy-selfheal.timer 2>/dev/null || true
        rm -f "$SELFHEAL_TIMER_UNIT"
        print_success "Removed self-heal timer ${SELFHEAL_TIMER_UNIT}"
    fi

    if [[ -f "$SELFHEAL_SERVICE_UNIT" ]]; then
        systemctl stop pulse-sensor-proxy-selfheal.service 2>/dev/null || true
        systemctl disable pulse-sensor-proxy-selfheal.service 2>/dev/null || true
        rm -f "$SELFHEAL_SERVICE_UNIT"
        print_success "Removed self-heal service ${SELFHEAL_SERVICE_UNIT}"
    fi

    if [[ -f "$SELFHEAL_SCRIPT" ]]; then
        rm -f "$SELFHEAL_SCRIPT"
        print_success "Removed self-heal helper ${SELFHEAL_SCRIPT}"
    fi

    if [[ -f "$STORED_INSTALLER" ]]; then
        rm -f "$STORED_INSTALLER"
        print_success "Removed cached installer ${STORED_INSTALLER}"
    fi

    if [[ -f "$CTID_FILE" ]]; then
        rm -f "$CTID_FILE"
    fi

    if command -v systemctl >/dev/null 2>&1; then
        systemctl daemon-reload 2>/dev/null || true
    fi

    rm -f "$CLEANUP_SCRIPT_PATH" "$CLEANUP_REQUEST_PATH" 2>/dev/null || true
    rm -f "$SOCKET_PATH" 2>/dev/null || true
    rm -rf "$RUNTIME_DIR" 2>/dev/null || true

    if [[ "$PURGE" == true ]]; then
        print_info "Purging Pulse sensor proxy state"
        rm -rf "$WORK_DIR" "$CONFIG_DIR" 2>/dev/null || true
        if [[ -d "$LOG_DIR" ]]; then
            print_info "Removing log directory ${LOG_DIR}"
        fi
        rm -rf "$LOG_DIR" 2>/dev/null || true

        if id -u "$SERVICE_USER" >/dev/null 2>&1; then
            if userdel --remove "$SERVICE_USER" 2>/dev/null; then
                print_success "Removed service user ${SERVICE_USER}"
            elif userdel "$SERVICE_USER" 2>/dev/null; then
                print_success "Removed service user ${SERVICE_USER}"
            else
                print_warn "Failed to remove service user ${SERVICE_USER}"
            fi
        fi

        if getent group "$SERVICE_USER" >/dev/null 2>&1; then
            if groupdel "$SERVICE_USER" 2>/dev/null; then
                print_success "Removed service group ${SERVICE_USER}"
            else
                print_warn "Failed to remove service group ${SERVICE_USER}"
            fi
        fi
    else
        if [[ -d "$WORK_DIR" ]]; then
            print_info "Preserving data directory ${WORK_DIR} (use --purge to remove)"
        fi
        if [[ -d "$CONFIG_DIR" ]]; then
            print_info "Preserving config directory ${CONFIG_DIR} (use --purge to remove)"
        fi
        if [[ -d "$LOG_DIR" ]]; then
            print_info "Preserving log directory ${LOG_DIR} (use --purge to remove)"
        fi
    fi

    print_success "pulse-sensor-proxy uninstall complete"
}

# Parse arguments first to check for standalone mode
CTID=""
VERSION="latest"
LOCAL_BINARY=""
QUIET=false
PULSE_SERVER=""
STANDALONE=false
FALLBACK_BASE="${PULSE_SENSOR_PROXY_FALLBACK_URL:-}"
SKIP_RESTART=false
UNINSTALL=false
PURGE=false

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
        --skip-restart)
            SKIP_RESTART=true
            shift
            ;;
        --uninstall)
            UNINSTALL=true
            shift
            ;;
        --purge)
            PURGE=true
            shift
            ;;
        *)
            print_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

if [[ "$PURGE" == true && "$UNINSTALL" != true ]]; then
    print_warn "--purge is only valid together with --uninstall; ignoring"
    PURGE=false
fi

if [[ "$UNINSTALL" == true ]]; then
    perform_uninstall
    exit 0
fi

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
        echo "   Or: $0 --uninstall [--purge]"
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

    GITHUB_REPO="rcourtman/Pulse"
    DOWNLOAD_SUCCESS=false
    ATTEMPTED_SOURCES=()
    LATEST_RELEASE_TAG=""

    fetch_latest_release_tag() {
        local api_url="https://api.github.com/repos/$GITHUB_REPO/releases/latest"
        local tmp_err
        tmp_err=$(mktemp)
        local response
        response=$(curl --fail --silent --location --connect-timeout 10 --max-time 30 "$api_url" 2>"$tmp_err")
        local status=$?
        if [[ $status -ne 0 ]]; then
            if [[ -s "$tmp_err" ]]; then
                print_warn "Failed to resolve latest GitHub release: $(cat "$tmp_err")"
            else
                print_warn "Failed to resolve latest GitHub release (HTTP $status)"
            fi
            rm -f "$tmp_err"
            return 1
        fi
        rm -f "$tmp_err"
        local tag
        tag=$(echo "$response" | grep -o '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | cut -d'"' -f4)
        if [[ -z "$tag" ]]; then
            print_warn "Could not parse latest release tag from GitHub response"
            return 1
        fi
        LATEST_RELEASE_TAG="$tag"
        return 0
    }

    attempt_github_asset_or_tarball() {
        local tag="$1"
        [[ -z "$tag" ]] && return 1

        local asset_url="https://github.com/$GITHUB_REPO/releases/download/${tag}/${BINARY_NAME}"
        ATTEMPTED_SOURCES+=("GitHub release asset ${tag}")
        print_info "Downloading $BINARY_NAME from GitHub release ${tag}..."
        local tmp_err
        tmp_err=$(mktemp)
        if curl --fail --silent --location --connect-timeout 10 --max-time 120 "$asset_url" -o "$BINARY_PATH.tmp" 2>"$tmp_err"; then
            rm -f "$tmp_err"
            DOWNLOAD_SUCCESS=true
            return 0
        fi

        local asset_error=""
        if [[ -s "$tmp_err" ]]; then
            asset_error="$(cat "$tmp_err")"
        fi
        rm -f "$tmp_err"
        rm -f "$BINARY_PATH.tmp" 2>/dev/null || true

        local tarball_name="pulse-${tag}-linux-${ARCH_LABEL#linux-}.tar.gz"
        local tarball_url="https://github.com/$GITHUB_REPO/releases/download/${tag}/${tarball_name}"
        ATTEMPTED_SOURCES+=("GitHub release tarball ${tarball_name}")
        print_info "Downloading ${tarball_name} to extract pulse-sensor-proxy..."
        tmp_err=$(mktemp)
        local tarball_tmp
        tarball_tmp=$(mktemp)
        if curl --fail --silent --location --connect-timeout 10 --max-time 240 "$tarball_url" -o "$tarball_tmp" 2>"$tmp_err"; then
            if tar -tzf "$tarball_tmp" >/dev/null 2>&1 && tar -xzf "$tarball_tmp" -C "$(dirname "$tarball_tmp")" ./bin/pulse-sensor-proxy >/dev/null 2>&1; then
                mv "$(dirname "$tarball_tmp")/bin/pulse-sensor-proxy" "$BINARY_PATH.tmp"
                rm -f "$tarball_tmp" "$tmp_err"
                DOWNLOAD_SUCCESS=true
                return 0
            else
                print_warn "Release tarball did not contain expected ./bin/pulse-sensor-proxy"
            fi
        else
            if [[ -s "$tmp_err" ]]; then
                print_warn "Tarball download failed: $(cat "$tmp_err")"
            else
                print_warn "Tarball download failed (HTTP error)"
            fi
        fi
        rm -f "$tarball_tmp" "$tmp_err"
        if [[ -n "$asset_error" ]]; then
            print_warn "GitHub release asset error: $asset_error"
        fi
        return 1
    }

    REQUESTED_VERSION="${VERSION:-latest}"
    if [[ "$REQUESTED_VERSION" == "latest" || "$REQUESTED_VERSION" == "main" || -z "$REQUESTED_VERSION" ]]; then
        if fetch_latest_release_tag; then
            attempt_github_asset_or_tarball "$LATEST_RELEASE_TAG" || true
        fi
    else
        attempt_github_asset_or_tarball "$REQUESTED_VERSION" || true
    fi

    if [[ "$DOWNLOAD_SUCCESS" != true ]] && [[ -n "$FALLBACK_BASE" ]]; then
        fallback_url="$FALLBACK_BASE"
        if [[ "$fallback_url" == *"?"* ]]; then
            fallback_url="$fallback_url"
        elif [[ "$fallback_url" == *"pulse-sensor-proxy-"* ]]; then
            fallback_url="${fallback_url}"
        else
            fallback_url="${fallback_url%/}?arch=${ARCH_LABEL}"
        fi

        ATTEMPTED_SOURCES+=("Fallback ${fallback_url}")
        print_info "Downloading $BINARY_NAME from fallback source..."
        fallback_err=$(mktemp)
        if curl --fail --silent --location --connect-timeout 10 --max-time 120 "$fallback_url" -o "$BINARY_PATH.tmp" 2>"$fallback_err"; then
            rm -f "$fallback_err"
            DOWNLOAD_SUCCESS=true
        else
            if [[ -s "$fallback_err" ]]; then
                print_error "Fallback download failed: $(cat "$fallback_err")"
            else
                print_error "Fallback download failed (HTTP error)"
            fi
            rm -f "$fallback_err"
            rm -f "$BINARY_PATH.tmp" 2>/dev/null || true
        fi
    fi

    if [[ "$DOWNLOAD_SUCCESS" != true ]] && [[ -n "$CTID" ]] && command -v pct >/dev/null 2>&1; then
        pull_targets=(
            "/opt/pulse/bin/${BINARY_NAME}"
            "/opt/pulse/bin/pulse-sensor-proxy"
        )
        for src in "${pull_targets[@]}"; do
            tmp_pull=$(mktemp)
            if pct pull "$CTID" "$src" "$tmp_pull" >/dev/null 2>&1; then
                mv "$tmp_pull" "$BINARY_PATH.tmp"
                print_info "Copied pulse-sensor-proxy binary from container $CTID ($src)"
                DOWNLOAD_SUCCESS=true
                break
            fi
            rm -f "$tmp_pull"
        done
    fi

    if [[ "$DOWNLOAD_SUCCESS" != true ]]; then
        print_error "Unable to download pulse-sensor-proxy binary."
        if [[ ${#ATTEMPTED_SOURCES[@]} -gt 0 ]]; then
            print_error "Sources attempted:"
            for src in "${ATTEMPTED_SOURCES[@]}"; do
                print_error "  - $src"
            done
        fi
        print_error "Publish a GitHub release with binary assets or ensure a Pulse server is reachable."
        exit 1
    fi

    chmod +x "$BINARY_PATH.tmp"
    mv "$BINARY_PATH.tmp" "$BINARY_PATH"
    print_info "Binary installed to $BINARY_PATH"
fi

# Create directories with proper ownership (handles fresh installs and upgrades)
print_info "Setting up directories with proper ownership..."
install -d -o pulse-sensor-proxy -g pulse-sensor-proxy -m 0750 /var/lib/pulse-sensor-proxy
install -d -o pulse-sensor-proxy -g pulse-sensor-proxy -m 0700 "$SSH_DIR"
install -m 0600 -o pulse-sensor-proxy -g pulse-sensor-proxy /dev/null "$SSH_DIR/known_hosts"
install -d -o pulse-sensor-proxy -g pulse-sensor-proxy -m 0755 /etc/pulse-sensor-proxy

if [[ -n "$CTID" ]]; then
    echo "$CTID" > "$CTID_FILE"
    chmod 0644 "$CTID_FILE"
fi

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
LogsDirectory=pulse/sensor-proxy
LogsDirectoryMode=0750
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
LogsDirectory=pulse/sensor-proxy
LogsDirectoryMode=0750
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
if ! systemctl daemon-reload; then
    print_error "Failed to reload systemd daemon"
    journalctl -u pulse-sensor-proxy -n 20 --no-pager
    exit 1
fi

if ! systemctl enable pulse-sensor-proxy.service; then
    print_error "Failed to enable pulse-sensor-proxy service"
    journalctl -u pulse-sensor-proxy -n 20 --no-pager
    exit 1
fi

if ! systemctl restart pulse-sensor-proxy.service; then
    print_error "Failed to start pulse-sensor-proxy service"
    print_error "Check service logs:"
    journalctl -u pulse-sensor-proxy -n 20 --no-pager
    exit 1
fi

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
        CLUSTER_NODES=$(pvecm status 2>/dev/null | awk '/0x[0-9a-f]+.*[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/ {print $3}' || true)

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
systemctl daemon-reload || true
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
    CLUSTER_NODES=$(pvecm status 2>/dev/null | awk '/0x[0-9a-f]+.*[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/ {print $3}' || true)

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
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  Secure Container Communication Setup"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "Setting up secure socket mount for temperature monitoring:"
    echo "  • Container communicates with host proxy via Unix socket"
    echo "  • No SSH keys exposed inside container (enhanced security)"
    echo "  • Proxy on host manages all temperature collection"
    echo ""

    # Ensure container mount via mp configuration
    print_info "Configuring socket bind mount..."
    MOUNT_TARGET="/mnt/pulse-proxy"
    HOST_SOCKET_SOURCE="/run/pulse-sensor-proxy"
    LXC_CONFIG="/etc/pve/lxc/${CTID}.conf"

# Back up container config before modifying
LXC_CONFIG_BACKUP=$(mktemp)
cp "$LXC_CONFIG" "$LXC_CONFIG_BACKUP" 2>/dev/null || {
    print_warn "Could not back up container config (may not exist yet)"
    LXC_CONFIG_BACKUP=""
}

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
    SET_ERROR=$(pct set "$CTID" -${CURRENT_MP} "${HOST_SOCKET_SOURCE},mp=${MOUNT_TARGET},replicate=0" 2>&1)
    if [ $? -eq 0 ]; then
        MOUNT_UPDATED=true
    else
        HOTPLUG_FAILED=true
        if [ -n "$SET_ERROR" ]; then
            print_warn "pct set failed: $SET_ERROR"
        fi
    fi
else
    desired_pattern="^${CURRENT_MP}: ${HOST_SOCKET_SOURCE},mp=${MOUNT_TARGET}"
    if pct config "$CTID" | grep -q "$desired_pattern"; then
        print_info "Container already has socket mount configured ($CURRENT_MP)"
    else
        print_info "Updating container mount configuration ($CURRENT_MP)..."
        SET_ERROR=$(pct set "$CTID" -${CURRENT_MP} "${HOST_SOCKET_SOURCE},mp=${MOUNT_TARGET},replicate=0" 2>&1)
        if [ $? -eq 0 ]; then
            MOUNT_UPDATED=true
        else
            HOTPLUG_FAILED=true
            if [ -n "$SET_ERROR" ]; then
                print_warn "pct set failed: $SET_ERROR"
            fi
        fi
    fi
fi

if [[ "$HOTPLUG_FAILED" = true ]]; then
    print_warn "Hot-plugging socket mount failed (container may be running). Updating config directly."
    CURRENT_MP_LINE="${CURRENT_MP}: ${HOST_SOCKET_SOURCE},mp=${MOUNT_TARGET},replicate=0"
    if ! grep -q "^${CURRENT_MP}:" "$LXC_CONFIG" 2>/dev/null; then
        echo "$CURRENT_MP_LINE" >> "$LXC_CONFIG"
    else
        sed -i "s#^${CURRENT_MP}:.*#${CURRENT_MP_LINE}#" "$LXC_CONFIG"
    fi
    MOUNT_UPDATED=true
fi

# Verify mount configuration actually persisted
if ! pct config "$CTID" | grep -q "^${CURRENT_MP}:"; then
    print_error "Failed to persist mount configuration for $CURRENT_MP"
    print_error "Expected mount not found in container config"
    exit 1
fi
print_info "✓ Mount configuration verified in container config"

# Remove legacy lxc.mount.entry directives if present
if grep -q "lxc.mount.entry: ${HOST_SOCKET_SOURCE}" "$LXC_CONFIG"; then
    print_info "Removing legacy lxc.mount.entry directives for pulse-sensor-proxy"
    sed -i '/lxc\.mount\.entry: \/run\/pulse-sensor-proxy/d' "$LXC_CONFIG"
    MOUNT_UPDATED=true
fi

# Restart container to apply mount if configuration changed or mount missing
if [[ "$MOUNT_UPDATED" = true ]]; then
    if [[ "$SKIP_RESTART" = true ]]; then
        if [[ "$CT_RUNNING" = true ]]; then
            print_info "Skipping container restart (--skip-restart provided)."
        else
            print_info "Skipping automatic container start (--skip-restart provided)."
        fi
    else
        print_info "Restarting container to activate secure communication..."
        if [[ "$CT_RUNNING" = true ]]; then
            pct stop "$CTID" && sleep 2 && pct start "$CTID"
        else
            pct start "$CTID"
        fi
        sleep 5
    fi
fi

# Verify socket directory and file inside container
if [[ "$HOTPLUG_FAILED" = true && "$CT_RUNNING" = true ]]; then
    print_warn "Skipping socket verification until container $CTID is restarted."
    print_warn "Please restart container and verify socket manually:"
    print_warn "  pct stop $CTID && sleep 2 && pct start $CTID"
    print_warn "  pct exec $CTID -- test -S ${MOUNT_TARGET}/pulse-sensor-proxy.sock && echo 'Socket OK'"
    # Keep backup in this case since we can't verify
    [ -n "$LXC_CONFIG_BACKUP" ] && rm -f "$LXC_CONFIG_BACKUP"
elif [[ "$SKIP_RESTART" = true && "$CT_RUNNING" = false ]]; then
    print_warn "Socket verification deferred. Start container $CTID and run:"
    print_warn "  pct exec $CTID -- test -S ${MOUNT_TARGET}/pulse-sensor-proxy.sock && echo 'Socket OK'"
    [ -n "$LXC_CONFIG_BACKUP" ] && rm -f "$LXC_CONFIG_BACKUP"
else
    print_info "Verifying secure communication channel..."
    if pct exec "$CTID" -- test -S "${MOUNT_TARGET}/pulse-sensor-proxy.sock"; then
        print_info "✓ Secure socket communication ready"
        # Clean up backup since verification succeeded
        [ -n "$LXC_CONFIG_BACKUP" ] && rm -f "$LXC_CONFIG_BACKUP"
    else
        print_error "Socket not visible at ${MOUNT_TARGET}/pulse-sensor-proxy.sock"
        print_error "Mount configuration verified but socket not accessible in container"
        print_error "This indicates a mount or restart issue"

        # Rollback container config changes
        if [ -n "$LXC_CONFIG_BACKUP" ] && [ -f "$LXC_CONFIG_BACKUP" ]; then
            print_warn "Rolling back container configuration changes..."
            cp "$LXC_CONFIG_BACKUP" "$LXC_CONFIG"
            rm -f "$LXC_CONFIG_BACKUP"
            print_info "Container configuration restored to previous state"
        fi
        exit 1
    fi
fi

# Configure Pulse backend environment override inside container
print_info "Configuring Pulse to use proxy..."
pct exec "$CTID" -- bash -lc "mkdir -p /etc/systemd/system/pulse.service.d"
pct exec "$CTID" -- bash -lc "cat <<'EOF' >/etc/systemd/system/pulse.service.d/10-pulse-proxy.conf
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

# Install self-heal safeguards to keep proxy available
print_info "Configuring self-heal safeguards..."
if [[ -n "$SCRIPT_SOURCE" && -f "$SCRIPT_SOURCE" ]]; then
    install -d "$SHARE_DIR"
    cp "$SCRIPT_SOURCE" "$STORED_INSTALLER"
    chmod 0755 "$STORED_INSTALLER"
else
    print_warn "Unable to cache installer script for self-heal (source path unavailable)"
fi

cat > "$SELFHEAL_SCRIPT" <<'EOF'
#!/bin/bash
set -euo pipefail

SERVICE="pulse-sensor-proxy"
INSTALLER="/usr/local/share/pulse/install-sensor-proxy.sh"
CTID_FILE="/etc/pulse-sensor-proxy/ctid"
LOG_TAG="pulse-sensor-proxy-selfheal"

log() {
    logger -t "$LOG_TAG" "$1"
}

if ! command -v systemctl >/dev/null 2>&1; then
    exit 0
fi

if ! systemctl list-unit-files | grep -q "^${SERVICE}\\.service"; then
    if [[ -x "$INSTALLER" && -f "$CTID_FILE" ]]; then
        log "Service unit missing; attempting reinstall"
        bash "$INSTALLER" --ctid "$(cat "$CTID_FILE")" --skip-restart --quiet || log "Reinstall attempt failed"
    fi
    exit 0
fi

if ! systemctl is-active --quiet "${SERVICE}.service"; then
    systemctl start "${SERVICE}.service" || true
    sleep 2
fi

if ! systemctl is-active --quiet "${SERVICE}.service"; then
    if [[ -x "$INSTALLER" && -f "$CTID_FILE" ]]; then
        log "Service failed to start; attempting reinstall"
        bash "$INSTALLER" --ctid "$(cat "$CTID_FILE")" --skip-restart --quiet || log "Reinstall attempt failed"
        systemctl start "${SERVICE}.service" || true
    fi
fi
EOF
chmod 0755 "$SELFHEAL_SCRIPT"

cat > "$SELFHEAL_SERVICE_UNIT" <<'EOF'
[Unit]
Description=Pulse Sensor Proxy Self-Heal
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/pulse-sensor-proxy-selfheal.sh
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

cat > "$SELFHEAL_TIMER_UNIT" <<'EOF'
[Unit]
Description=Ensure pulse-sensor-proxy stays installed and running

[Timer]
OnBootSec=5min
OnUnitActiveSec=30min
Unit=pulse-sensor-proxy-selfheal.service

[Install]
WantedBy=timers.target
EOF

systemctl daemon-reload
systemctl enable --now pulse-sensor-proxy-selfheal.timer >/dev/null 2>&1 || true

if [ "$QUIET" = true ]; then
    print_success "pulse-sensor-proxy installed and running"
else
    print_info "${GREEN}Installation complete!${NC}"
    print_info ""
    print_info "Temperature monitoring will use the secure host-side proxy"
    print_info ""

    if [[ "$STANDALONE" == true ]]; then
        echo ""
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "  Docker Container Configuration Required"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo ""
        print_info "${YELLOW}IMPORTANT:${NC} If Pulse is running in Docker, add this bind mount to your docker-compose.yml:"
        echo ""
        echo "  volumes:"
        echo "    - pulse-data:/data"
        echo "    - /run/pulse-sensor-proxy:/run/pulse-sensor-proxy:rw"
        echo ""
        print_info "Then restart your Pulse container:"
        echo "  docker-compose down && docker-compose up -d"
        echo ""
        print_info "Or if using Docker directly:"
        echo "  docker restart pulse"
        echo ""
    fi

    print_info "To check proxy status:"
    print_info "  systemctl status pulse-sensor-proxy"

    if [[ "$STANDALONE" == true ]]; then
        echo ""
        print_info "After restarting Pulse, verify the socket is accessible:"
        print_info "  docker exec pulse ls -l /run/pulse-sensor-proxy/pulse-sensor-proxy.sock"
        echo ""
        print_info "Check Pulse logs for temperature proxy detection:"
        print_info "  docker logs pulse | grep -i 'temperature.*proxy'"
        echo ""
        print_info "For detailed documentation, see:"
        print_info "  https://github.com/rcourtman/Pulse/blob/main/docs/TEMPERATURE_MONITORING.md"
    fi
fi

exit 0
