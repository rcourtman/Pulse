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

# Safe read function that works with or without TTY
safe_read() {
    local prompt="$1"
    local var_name="$2"
    
    # When script is piped (curl | bash), stdin is the pipe, not the terminal
    # We need to read from /dev/tty for user input
    if [[ -e /dev/tty ]]; then
        # TTY is available, read from it
        echo -n "$prompt" > /dev/tty
        # Use read with -u to read from file descriptor
        IFS= read -r "$var_name" < /dev/tty
    else
        # No TTY (e.g., in automated environments)
        echo -n "$prompt"
        IFS= read -r "$var_name"
    fi
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

# V3 is deprecated - no longer checking for it

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

check_proxmox_host() {
    # Check if this is a Proxmox VE host
    if command -v pvesh &> /dev/null && [[ -d /etc/pve ]]; then
        return 0
    fi
    return 1
}

check_docker_environment() {
    # Detect if we're running inside Docker
    if [[ -f /.dockerenv ]] || grep -q docker /proc/1/cgroup 2>/dev/null; then
        print_error "Docker environment detected"
        echo "Please use the Docker image directly: docker run -d -p 7655:7655 rcourtman/pulse:latest"
        echo "See: https://github.com/rcourtman/Pulse/blob/main/docs/DOCKER.md"
        exit 1
    fi
}

create_lxc_container() {
    # Set up trap to cleanup on interrupt (CTID will be set later)
    trap 'echo ""; print_error "Installation cancelled"; exit 1' INT
    
    print_header
    echo "Proxmox VE detected. Installing Pulse in a container."
    echo
    
    # Check if we can interact with the user
    # Try to read from /dev/tty to test if we have terminal access
    if test -e /dev/tty && (echo -n "" > /dev/tty) 2>/dev/null; then
        # We have terminal access, show menu
        echo "Installation mode:"
        echo "  1) Quick (recommended)"
        echo "  2) Advanced"  
        echo "  3) Cancel"
        safe_read "Select [1-3]: " mode -n 1 -r
        echo
    else
        # No terminal access - truly non-interactive
        echo "Non-interactive mode detected. Using Quick installation."
        mode="1"
    fi
    
    case $mode in
        3)
            print_info "Installation cancelled"
            exit 0
            ;;
        2)
            ADVANCED_MODE=true
            ;;
        *)
            ADVANCED_MODE=false
            ;;
    esac
    
    # Get next available container ID from Proxmox
    local CTID=$(pvesh get /cluster/nextid 2>/dev/null || echo "100")
    
    # If pvesh failed, fallback to manual search
    if [[ "$CTID" == "100" ]]; then
        while pct status $CTID &>/dev/null 2>&1 || qm status $CTID &>/dev/null 2>&1; do
            ((CTID++))
        done
    fi
    
    if [[ "$ADVANCED_MODE" == "true" ]]; then
        echo
        # Ask for port configuration
        safe_read "Frontend port [7655]: " frontend_port
        frontend_port=${frontend_port:-7655}
        if [[ ! "$frontend_port" =~ ^[0-9]+$ ]] || [[ "$frontend_port" -lt 1 ]] || [[ "$frontend_port" -gt 65535 ]]; then
            print_error "Invalid port number. Using default port 7655."
            frontend_port=7655
        fi
        
        echo
        # Try to get cluster-wide IDs, fall back to local
        local USED_IDS=""
        if command -v pvesh &>/dev/null; then
            # Parse JSON output using grep and sed (works without jq)
            USED_IDS=$(pvesh get /cluster/resources --type vm --output-format json 2>/dev/null | \
                      grep -o '"vmid":[0-9]*' | \
                      sed 's/"vmid"://' | \
                      sort -n | \
                      paste -sd',' -)
        fi
        
        if [[ -z "$USED_IDS" ]]; then
            # Fallback: get local containers and VMs
            local LOCAL_CTS=$(pct list 2>/dev/null | tail -n +2 | awk '{print $1}' | sort -n)
            local LOCAL_VMS=$(qm list 2>/dev/null | tail -n +2 | awk '{print $1}' | sort -n)
            USED_IDS=$(echo -e "$LOCAL_CTS\n$LOCAL_VMS" | grep -v '^$' | sort -n | paste -sd',' -)
        fi
        
        echo "Container/VM IDs in use: ${USED_IDS:-none}"
        safe_read "Container ID [$CTID]: " custom_ctid
        if [[ -n "$custom_ctid" ]] && [[ "$custom_ctid" =~ ^[0-9]+$ ]]; then
            # Check if ID is in use
            if pct status $custom_ctid &>/dev/null 2>&1 || qm status $custom_ctid &>/dev/null 2>&1; then
                print_error "Container/VM ID $custom_ctid is already in use"
                exit 1
            fi
            # Also check cluster if possible
            if command -v pvesh &>/dev/null; then
                if pvesh get /cluster/resources --type vm 2>/dev/null | jq -e ".[] | select(.vmid == $custom_ctid)" &>/dev/null; then
                    print_error "Container/VM ID $custom_ctid is already in use in the cluster"
                    exit 1
                fi
            fi
            CTID=$custom_ctid
        fi
    fi
    
    print_info "Using container ID: $CTID"
    
    if [[ "$ADVANCED_MODE" == "true" ]]; then
        echo
        echo -e "${BLUE}Advanced Mode - Customize all settings${NC}"
        echo -e "${YELLOW}Defaults shown are suitable for monitoring 10-20 nodes${NC}"
        echo
        
        # Container settings
        safe_read "Container hostname [pulse]: " hostname
        hostname=${hostname:-pulse}
        
        safe_read "Memory (MB) [1024]: " memory
        memory=${memory:-1024}
        
        safe_read "Disk size (GB) [4]: " disk
        disk=${disk:-4}
        
        safe_read "CPU cores [2]: " cores
        cores=${cores:-2}
        
        safe_read "CPU limit (0=unlimited) [2]: " cpulimit
        cpulimit=${cpulimit:-2}
        
        safe_read "Swap (MB) [256]: " swap
        swap=${swap:-256}
        
        safe_read "Start on boot? [Y/n]: " onboot -n 1 -r
        echo
        if [[ "$onboot" =~ ^[Nn]$ ]]; then
            onboot=0
        else
            onboot=1
        fi
        
        safe_read "Enable firewall? [Y/n]: " firewall -n 1 -r
        echo
        if [[ "$firewall" =~ ^[Nn]$ ]]; then
            firewall=0
        else
            firewall=1
        fi
        
        safe_read "Unprivileged container? [Y/n]: " unprivileged -n 1 -r
        echo
        if [[ "$unprivileged" =~ ^[Nn]$ ]]; then
            unprivileged=0
        else
            unprivileged=1
        fi
    else
        # Quick mode - just use defaults silently
        
        # Use optimized defaults
        hostname="pulse"
        memory=1024
        disk=4
        cores=2
        cpulimit=2
        swap=256
        onboot=1
        firewall=1
        unprivileged=1
        frontend_port=7655
    fi
    
    # Get available network bridges
    echo
    print_info "Detecting available resources..."
    
    # Get available bridges
    local BRIDGES=$(ip link show type bridge | grep -E '^[0-9]+:' | cut -d: -f2 | tr -d ' ' | paste -sd',' -)
    local DEFAULT_BRIDGE=$(ip route | grep default | head -1 | grep -oP 'dev \K\S+' | grep -E '^vmbr')
    DEFAULT_BRIDGE=${DEFAULT_BRIDGE:-vmbr0}
    
    # Get available storage with usage info
    local STORAGE_INFO=$(pvesm status -content rootdir 2>/dev/null | tail -n +2)
    local DEFAULT_STORAGE=$(echo "$STORAGE_INFO" | awk '{print $1}' | head -1)
    DEFAULT_STORAGE=${DEFAULT_STORAGE:-local-lvm}
    
    if [[ "$ADVANCED_MODE" == "true" ]]; then
        # Show available bridges
        echo
        echo "Available network bridges: ${BRIDGES:-none detected}"
        safe_read "Network bridge [$DEFAULT_BRIDGE]: " bridge
        bridge=${bridge:-$DEFAULT_BRIDGE}
        
        # Show available storage with usage details
        echo
        echo "Available storage pools:"
        echo "$STORAGE_INFO" | awk '{printf "  %-15s %-8s %5s used\n", $1, $2, $6}' || echo "  No storage pools found"
        safe_read "Storage [$DEFAULT_STORAGE]: " storage
        storage=${storage:-$DEFAULT_STORAGE}
        
        safe_read "Static IP (leave empty for DHCP): " static_ip
        
        safe_read "DNS servers (comma-separated, empty for host settings): " nameserver
        
        safe_read "Startup order [99]: " startup
        startup=${startup:-99}
    else
        # Quick mode - use defaults
        bridge=$DEFAULT_BRIDGE
        storage=$DEFAULT_STORAGE
        static_ip=""
        nameserver=""
        startup=99
    fi
    
    # Handle OS template selection
    echo
    if [[ "$ADVANCED_MODE" == "true" ]]; then
        echo "Available OS templates:"
        # Use pveam to list templates properly
        local TEMPLATES=$(pveam list local 2>/dev/null | tail -n +2 | awk '{print $1}' | sed 's/local:vztmpl\///' | nl -w2 -s') ')
        if [[ -n "$TEMPLATES" ]]; then
            echo "$TEMPLATES"
            echo
            echo "Or download a new template:"
            echo "  d) Download Debian 12 (recommended)"
            echo "  u) Download Ubuntu 22.04 LTS"
            echo "  a) Download Alpine Linux (minimal)"
            echo
            safe_read "Select template number or option [Enter for Debian 12]: " template_choice
            if [[ -n "$template_choice" ]]; then
                case "$template_choice" in
                    d|D)
                        print_info "Downloading Debian 12..."
                        pveam download local debian-12-standard_12.7-1_amd64.tar.zst
                        TEMPLATE="/var/lib/vz/template/cache/debian-12-standard_12.7-1_amd64.tar.zst"
                        ;;
                    u|U)
                        print_info "Downloading Ubuntu 22.04..."
                        pveam download local ubuntu-22.04-standard_22.04-1_amd64.tar.zst
                        TEMPLATE="/var/lib/vz/template/cache/ubuntu-22.04-standard_22.04-1_amd64.tar.zst"
                        ;;
                    a|A)
                        print_info "Downloading Alpine Linux..."
                        pveam download local alpine-3.18-default_20230607_amd64.tar.xz
                        TEMPLATE="/var/lib/vz/template/cache/alpine-3.18-default_20230607_amd64.tar.xz"
                        ;;
                    [0-9]*)
                        TEMPLATE_NAME=$(pveam list local 2>/dev/null | tail -n +2 | awk '{print $1}' | sed 's/local:vztmpl\///' | sed -n "${template_choice}p")
                        if [[ -n "$TEMPLATE_NAME" ]]; then
                            TEMPLATE="/var/lib/vz/template/cache/$TEMPLATE_NAME"
                            print_info "Using template: $TEMPLATE_NAME"
                        else
                            TEMPLATE="/var/lib/vz/template/cache/debian-12-standard_12.7-1_amd64.tar.zst"
                            print_info "Invalid selection, using Debian 12"
                        fi
                        ;;
                    *)
                        TEMPLATE="/var/lib/vz/template/cache/debian-12-standard_12.7-1_amd64.tar.zst"
                        ;;
                esac
            else
                TEMPLATE="/var/lib/vz/template/cache/debian-12-standard_12.7-1_amd64.tar.zst"
            fi
        else
            TEMPLATE="/var/lib/vz/template/cache/debian-12-standard_12.7-1_amd64.tar.zst"
        fi
    else
        # Quick mode - use Debian 12
        TEMPLATE="/var/lib/vz/template/cache/debian-12-standard_12.7-1_amd64.tar.zst"
    fi
    
    # Download template if it doesn't exist
    if [[ ! -f "$TEMPLATE" ]]; then
        print_info "Template not found, downloading Debian 12..."
        if ! pveam download local debian-12-standard_12.7-1_amd64.tar.zst; then
            print_error "Failed to download template. Please check your internet connection and try again."
            print_info "You can manually download with: pveam download local debian-12-standard_12.7-1_amd64.tar.zst"
            exit 1
        fi
        TEMPLATE="/var/lib/vz/template/cache/debian-12-standard_12.7-1_amd64.tar.zst"
        if [[ ! -f "$TEMPLATE" ]]; then
            print_error "Template download succeeded but file not found at expected location"
            exit 1
        fi
    fi
    
    print_info "Creating container..."
    
    # Build network configuration
    if [[ -n "$static_ip" ]]; then
        NET_CONFIG="name=eth0,bridge=${bridge},ip=${static_ip},firewall=${firewall}"
    else
        NET_CONFIG="name=eth0,bridge=${bridge},ip=dhcp,firewall=${firewall}"
    fi
    
    # Build container create command
    local CREATE_CMD="pct create $CTID $TEMPLATE"
    CREATE_CMD="$CREATE_CMD --hostname $hostname"
    CREATE_CMD="$CREATE_CMD --memory $memory"
    CREATE_CMD="$CREATE_CMD --cores $cores"
    
    if [[ "$cpulimit" != "0" ]]; then
        CREATE_CMD="$CREATE_CMD --cpulimit $cpulimit"
    fi
    
    CREATE_CMD="$CREATE_CMD --rootfs ${storage}:${disk}"
    CREATE_CMD="$CREATE_CMD --net0 $NET_CONFIG"
    CREATE_CMD="$CREATE_CMD --unprivileged $unprivileged"
    CREATE_CMD="$CREATE_CMD --features nesting=1"
    CREATE_CMD="$CREATE_CMD --onboot $onboot"
    CREATE_CMD="$CREATE_CMD --startup order=$startup"
    CREATE_CMD="$CREATE_CMD --protection 0"
    CREATE_CMD="$CREATE_CMD --swap $swap"
    
    if [[ -n "$nameserver" ]]; then
        CREATE_CMD="$CREATE_CMD --nameserver $nameserver"
    fi
    
    # Execute container creation (suppress verbose output)
    if ! eval $CREATE_CMD >/dev/null 2>&1; then
        print_error "Failed to create container"
        exit 1
    fi
    
    # From this point on, cleanup container if we fail
    cleanup_on_error() {
        print_error "Installation failed, cleaning up container $CTID..."
        pct stop $CTID 2>/dev/null || true
        sleep 2
        pct destroy $CTID 2>/dev/null || true
        exit 1
    }
    
    # Start container
    print_info "Starting container..."
    if ! pct start $CTID >/dev/null 2>&1; then
        print_error "Failed to start container"
        cleanup_on_error
    fi
    sleep 3
    
    # Wait for network to be ready
    print_info "Waiting for network..."
    local network_ready=false
    for i in {1..60}; do
        if pct exec $CTID -- ping -c 1 8.8.8.8 &>/dev/null 2>&1; then
            network_ready=true
            break
        fi
        sleep 1
    done
    
    if [[ "$network_ready" != "true" ]]; then
        print_error "Container network failed to come up after 60 seconds"
        cleanup_on_error
    fi
    
    # Install dependencies and optimize container
    print_info "Installing dependencies..."
    if ! pct exec $CTID -- bash -c "
        apt-get update -qq >/dev/null 2>&1 && apt-get install -y -qq curl wget ca-certificates >/dev/null 2>&1
        # Set timezone to UTC for consistent logging
        ln -sf /usr/share/zoneinfo/UTC /etc/localtime 2>/dev/null
        # Optimize sysctl for monitoring workload
        echo 'net.core.somaxconn=1024' >> /etc/sysctl.conf
        echo 'net.ipv4.tcp_keepalive_time=60' >> /etc/sysctl.conf
        sysctl -p >/dev/null 2>&1
        
        # Create convenient update script
        cat > /usr/local/bin/update << 'EOF'
#!/bin/bash
echo 'Updating Pulse...'
curl -sSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash
EOF
        chmod +x /usr/local/bin/update
    "; then
        print_error "Failed to install dependencies in container"
        cleanup_on_error
    fi
    
    # Install Pulse inside container
    print_info "Installing Pulse..."
    
    # When piped through curl, $0 is "bash" not the script. Download fresh copy.
    local script_source="/tmp/pulse_install_$$.sh"
    if [[ "$0" == "bash" ]] || [[ ! -f "$0" ]]; then
        # We're being piped, download the script
        if ! curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh > "$script_source" 2>/dev/null; then
            print_error "Failed to download install script"
            cleanup_on_error
        fi
    else
        # We have a local script file
        script_source="$0"
    fi
    
    # Copy this script to container and run it
    if ! pct push $CTID "$script_source" /tmp/install.sh >/dev/null 2>&1; then
        print_error "Failed to copy install script to container"
        cleanup_on_error
    fi
    
    # Clean up temp file if we created one
    if [[ "$script_source" == "/tmp/pulse_install_"* ]]; then
        rm -f "$script_source"
    fi
    
    # Run installation quietly (suppress verbose output) with port configuration
    local install_cmd="bash /tmp/install.sh --in-container"
    if [[ "$frontend_port" != "7655" ]]; then
        install_cmd="FRONTEND_PORT=$frontend_port $install_cmd"
    fi
    local install_output=$(pct exec $CTID -- bash -c "$install_cmd" 2>&1)
    local install_status=$?
    
    if [[ $install_status -ne 0 ]]; then
        print_error "Failed to install Pulse inside container"
        cleanup_on_error
    fi
    
    # Get container IP
    local IP=$(pct exec $CTID -- hostname -I | awk '{print $1}')
    
    # Clean final output
    echo
    print_success "Pulse installation complete!"
    echo
    echo "  Web UI:     http://${IP}:${frontend_port}"
    echo "  Container:  $CTID"
    echo
    echo "  Commands:"
    echo "    pct enter $CTID              # Enter container"
    echo "    pct exec $CTID -- update     # Update Pulse"
    echo
    
    exit 0
}

# Compare two version strings
# Returns: 0 if equal, 1 if first > second, 2 if first < second
compare_versions() {
    local v1="${1#v}"  # Remove 'v' prefix
    local v2="${2#v}"
    
    # Split versions into parts
    IFS='.' read -ra V1_PARTS <<< "$v1"
    IFS='.' read -ra V2_PARTS <<< "$v2"
    
    # Compare major.minor.patch
    for i in 0 1 2; do
        local p1="${V1_PARTS[$i]:-0}"
        local p2="${V2_PARTS[$i]:-0}"
        if [[ "$p1" -gt "$p2" ]]; then
            return 1
        elif [[ "$p1" -lt "$p2" ]]; then
            return 2
        fi
    done
    
    return 0  # versions are equal
}

check_existing_installation() {
    CURRENT_VERSION=""  # Make it global so we can use it later
    local BINARY_PATH=""
    
    # Check for the binary in expected locations
    if [[ -f "$INSTALL_DIR/bin/pulse" ]]; then
        BINARY_PATH="$INSTALL_DIR/bin/pulse"
    elif [[ -f "$INSTALL_DIR/pulse" ]]; then
        BINARY_PATH="$INSTALL_DIR/pulse"
    fi
    
    # Try to get version if binary exists
    if [[ -n "$BINARY_PATH" ]]; then
        CURRENT_VERSION=$($BINARY_PATH --version 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")
    fi
    
    if systemctl is-active --quiet $SERVICE_NAME 2>/dev/null; then
        if [[ -n "$CURRENT_VERSION" && "$CURRENT_VERSION" != "unknown" ]]; then
            print_info "Pulse $CURRENT_VERSION is currently running"
        else
            print_info "Pulse is currently running"
        fi
        return 0
    elif [[ -n "$BINARY_PATH" ]]; then
        if [[ -n "$CURRENT_VERSION" && "$CURRENT_VERSION" != "unknown" ]]; then
            print_info "Pulse $CURRENT_VERSION is installed but not running"
        else
            print_info "Pulse is installed but not running"
        fi
        return 0
    else
        return 1
    fi
}

install_dependencies() {
    print_info "Installing dependencies..."
    
    apt-get update -qq >/dev/null 2>&1
    apt-get install -y -qq curl wget >/dev/null 2>&1
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
        # UPDATE_CHANNEL should already be set by main(), but set default if not
        if [[ -z "${UPDATE_CHANNEL:-}" ]]; then
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
EOF

    # Add port configuration if not default
    if [[ "${FRONTEND_PORT:-7655}" != "7655" ]]; then
        cat >> /etc/systemd/system/$SERVICE_NAME.service << EOF
Environment="FRONTEND_PORT=$FRONTEND_PORT"
EOF
    fi

    cat >> /etc/systemd/system/$SERVICE_NAME.service << EOF

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$INSTALL_DIR $CONFIG_DIR

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload >/dev/null 2>&1
}

start_pulse() {
    print_info "Starting Pulse..."
    systemctl enable $SERVICE_NAME >/dev/null 2>&1
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
    
    # Get the port from the service file or use default
    local PORT="${FRONTEND_PORT:-7655}"
    if [[ -z "${FRONTEND_PORT:-}" ]] && [[ -f "/etc/systemd/system/$SERVICE_NAME.service" ]]; then
        # Try to extract port from service file
        PORT=$(grep -oP 'FRONTEND_PORT=\K\d+' "/etc/systemd/system/$SERVICE_NAME.service" 2>/dev/null || echo "7655")
    fi
    
    echo
    print_header
    print_success "Pulse installation completed!"
    echo
    echo -e "${GREEN}Access Pulse at:${NC} http://${IP}:${PORT}"
    echo
    echo -e "${YELLOW}Useful commands:${NC}"
    echo "  systemctl status $SERVICE_NAME    - Check service status"
    echo "  systemctl restart $SERVICE_NAME   - Restart Pulse"
    echo "  journalctl -u $SERVICE_NAME -f    - View logs"
    echo "  systemctl edit $SERVICE_NAME      - Configure custom port"
    echo
    echo -e "${YELLOW}Updating Pulse:${NC}"
    echo "  curl -sSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash"
    echo "  Or: wget -qO- https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash"
    echo
}

# Main installation flow
main() {
    # Skip Proxmox host check if we're already inside a container
    if [[ "$IN_CONTAINER" != "true" ]] && check_proxmox_host; then
        create_lxc_container
        exit 0
    fi
    
    print_header
    check_root
    detect_os
    check_docker_environment
    
    # Check for existing installation FIRST before asking for configuration
    if check_existing_installation; then
        # If a specific version was requested, just update to it
        if [[ -n "${FORCE_VERSION}" ]]; then
            # Determine if this is an upgrade, downgrade, or reinstall
            local action_word="Installing"
            if [[ -n "$CURRENT_VERSION" ]] && [[ "$CURRENT_VERSION" != "unknown" ]]; then
                local compare_result
                compare_versions "$FORCE_VERSION" "$CURRENT_VERSION" && compare_result=$? || compare_result=$?
                case $compare_result in
                    0) action_word="Reinstalling" ;;
                    1) action_word="Updating to" ;;
                    2) action_word="Downgrading to" ;;
                esac
            fi
            print_info "${action_word} version ${FORCE_VERSION}..."
            LATEST_RELEASE="${FORCE_VERSION}"
            
            # Detect the actual service name before trying to stop it
            SERVICE_NAME=$(detect_service_name)
            
            backup_existing
            systemctl stop $SERVICE_NAME || true
            create_user
            download_pulse
            start_pulse
            print_completion
            return 0
        fi
        
        # Get both stable and RC versions
        # Try GitHub API first, but have a fallback
        local STABLE_VERSION=$(curl -s https://api.github.com/repos/$GITHUB_REPO/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' 2>/dev/null || true)
        
        # If rate limited or failed, try direct GitHub latest URL
        if [[ -z "$STABLE_VERSION" ]] || [[ "$STABLE_VERSION" == *"rate limit"* ]]; then
            # Use the GitHub latest release redirect to get version
            STABLE_VERSION=$(curl -sI https://github.com/$GITHUB_REPO/releases/latest | grep -i '^location:' | sed -E 's|.*tag/([^[:space:]]+).*|\1|' | tr -d '\r' || true)
        fi
        
        # For RC, we need the API, so if it fails just use empty
        local RC_VERSION=$(curl -s https://api.github.com/repos/$GITHUB_REPO/releases | grep '"tag_name":' | head -1 | sed -E 's/.*"([^"]+)".*/\1/' 2>/dev/null || true)
        
        # Determine default update channel
        UPDATE_CHANNEL="stable"
        
        # Allow override via command line
        if [[ -n "${FORCE_CHANNEL}" ]]; then
            UPDATE_CHANNEL="${FORCE_CHANNEL}"
        elif [[ -f "$CONFIG_DIR/system.json" ]]; then
            CONFIGURED_CHANNEL=$(cat "$CONFIG_DIR/system.json" 2>/dev/null | grep -o '"updateChannel"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"\([^"]*\)"$/\1/' || true)
            if [[ "$CONFIGURED_CHANNEL" == "rc" ]]; then
                UPDATE_CHANNEL="rc"
            fi
        fi
        
        echo
        echo "What would you like to do?"
        
        # Show update options based on available versions
        local menu_option=1
        if [[ -n "$STABLE_VERSION" ]] && [[ "$STABLE_VERSION" != "$CURRENT_VERSION" ]]; then
            echo "${menu_option}) Update to $STABLE_VERSION (stable)"
            ((menu_option++))
        fi
        
        if [[ -n "$RC_VERSION" ]] && [[ "$RC_VERSION" != "$STABLE_VERSION" ]] && [[ "$RC_VERSION" != "$CURRENT_VERSION" ]]; then
            echo "${menu_option}) Update to $RC_VERSION (release candidate)"
            ((menu_option++))
        fi
        
        echo "${menu_option}) Reinstall current version"
        ((menu_option++))
        echo "${menu_option}) Remove Pulse"
        ((menu_option++))
        echo "${menu_option}) Cancel"
        local max_option=$menu_option
        
        # In non-interactive container mode, auto-select update
        if [[ "$IN_CONTAINER" == "true" ]] && ! test -t 0; then
            print_info "Non-interactive mode detected. Auto-selecting update option."
            # Select stable update by default, or RC if configured
            if [[ "$UPDATE_CHANNEL" == "rc" ]] && [[ -n "$RC_VERSION" ]] && [[ "$RC_VERSION" != "$STABLE_VERSION" ]]; then
                choice=2  # RC version
            else
                choice=1  # Stable version
            fi
        else
            safe_read "Select option [1-${max_option}]: " choice
        fi
        
        # Debug: Check if choice was read correctly
        if [[ -z "$choice" ]]; then
            print_error "No option selected. Exiting."
            exit 1
        fi
        
        # Debug output to see what's happening
        print_info "DEBUG: You selected option $choice"
        
        # Determine what action to take based on the dynamic menu
        local action=""
        local target_version=""
        local current_choice=1
        
        # Check if user selected stable update
        if [[ -n "$STABLE_VERSION" ]] && [[ "$STABLE_VERSION" != "$CURRENT_VERSION" ]]; then
            if [[ "$choice" == "$current_choice" ]]; then
                action="update"
                target_version="$STABLE_VERSION"
                UPDATE_CHANNEL="stable"
            fi
            ((current_choice++))
        fi
        
        # Check if user selected RC update
        if [[ -n "$RC_VERSION" ]] && [[ "$RC_VERSION" != "$STABLE_VERSION" ]] && [[ "$RC_VERSION" != "$CURRENT_VERSION" ]]; then
            if [[ "$choice" == "$current_choice" ]]; then
                action="update"
                target_version="$RC_VERSION"
                UPDATE_CHANNEL="rc"
            fi
            ((current_choice++))
        fi
        
        # Check if user selected reinstall
        if [[ "$choice" == "$current_choice" ]]; then
            action="reinstall"
        fi
        ((current_choice++))
        
        # Check if user selected remove
        if [[ "$choice" == "$current_choice" ]]; then
            action="remove"
        fi
        ((current_choice++))
        
        # Check if user selected cancel
        if [[ "$choice" == "$current_choice" ]]; then
            action="cancel"
        fi
        
        # Debug: Show what action was determined
        print_info "DEBUG: Action determined: ${action:-'none'}"
        
        case $action in
            update)
                # Determine if this is an upgrade or downgrade
                local action_word="Installing"
                if [[ -n "$CURRENT_VERSION" ]] && [[ "$CURRENT_VERSION" != "unknown" ]]; then
                    compare_versions "$target_version" "$CURRENT_VERSION"
                    case $? in
                        0) action_word="Reinstalling" ;;
                        1) action_word="Updating to" ;;
                        2) action_word="Downgrading to" ;;
                    esac
                fi
                print_info "${action_word} $target_version..."
                LATEST_RELEASE="$target_version"
                backup_existing
                systemctl stop $SERVICE_NAME || true
                create_user
                download_pulse
                start_pulse
                print_completion
                ;;
            reinstall)
                backup_existing
                systemctl stop $SERVICE_NAME || true
                create_user
                download_pulse
                setup_directories
                install_systemd_service
                start_pulse
                print_completion
                ;;
            remove)
                # Stop and disable service
                systemctl stop $SERVICE_NAME 2>/dev/null || true
                systemctl disable $SERVICE_NAME 2>/dev/null || true
                
                # Remove service files
                rm -f /etc/systemd/system/$SERVICE_NAME.service
                rm -f /etc/systemd/system/pulse.service
                rm -f /etc/systemd/system/pulse-backend.service
                systemctl daemon-reload >/dev/null 2>&1
                
                # Remove installation directory
                rm -rf "$INSTALL_DIR"
                
                # Remove symlink
                rm -f /usr/local/bin/pulse
                
                # Ask about config/data removal
                echo
                print_info "Config and data files exist in $CONFIG_DIR"
                safe_read "Remove all configuration and data? (y/N): " remove_config
                if [[ "$remove_config" =~ ^[Yy]$ ]]; then
                    rm -rf "$CONFIG_DIR"
                    print_success "Configuration and data removed"
                else
                    print_info "Configuration preserved in $CONFIG_DIR"
                fi
                
                # Ask about user removal
                if id "pulse" &>/dev/null; then
                    safe_read "Remove pulse user account? (y/N): " remove_user
                    if [[ "$remove_user" =~ ^[Yy]$ ]]; then
                        userdel pulse 2>/dev/null || true
                        print_success "User account removed"
                    else
                        print_info "User account preserved"
                    fi
                fi
                
                # Remove any log files
                rm -f /var/log/pulse*.log 2>/dev/null || true
                rm -f /opt/pulse.log 2>/dev/null || true
                
                print_success "Pulse removed successfully"
                ;;
            cancel)
                print_info "Installation cancelled"
                exit 0
                ;;
            *)
                print_error "Invalid option"
                exit 1
                ;;
        esac
    else
        # Check if this is truly a fresh installation or an update
        # If binary exists OR config exists OR --version was specified, it's likely an update
        if [[ -f "$INSTALL_DIR/bin/pulse" ]] || [[ -f "$INSTALL_DIR/pulse" ]] || [[ -d "$CONFIG_DIR" ]] || [[ -n "${FORCE_VERSION}" ]]; then
            # This is an update/reinstall, don't prompt for port
            FRONTEND_PORT=${FRONTEND_PORT:-7655}
        else
            # Fresh installation - ask for port configuration
            FRONTEND_PORT=${FRONTEND_PORT:-}
            if [[ -z "$FRONTEND_PORT" ]]; then
                if [[ "$IN_CONTAINER" == "true" ]]; then
                    # In container mode, use default port without prompting
                    FRONTEND_PORT=7655
                else
                    echo
                    safe_read "Frontend port [7655]: " FRONTEND_PORT
                    FRONTEND_PORT=${FRONTEND_PORT:-7655}
                    if [[ ! "$FRONTEND_PORT" =~ ^[0-9]+$ ]] || [[ "$FRONTEND_PORT" -lt 1 ]] || [[ "$FRONTEND_PORT" -gt 65535 ]]; then
                        print_error "Invalid port number. Using default port 7655."
                        FRONTEND_PORT=7655
                    fi
                fi
            fi
        fi
        
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
IN_CONTAINER=false

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
        --in-container)
            IN_CONTAINER=true
            shift
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