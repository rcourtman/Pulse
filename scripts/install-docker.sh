#!/bin/bash
# install-docker.sh - Turnkey Pulse installation for Docker hosts
# This script installs pulse-sensor-proxy and generates docker-compose.yml

set -euo pipefail

PULSE_IMAGE="${PULSE_IMAGE:-rcourtman/pulse:latest}"
PULSE_PORT="${PULSE_PORT:-7655}"
PULSE_PROXY_CHANNEL="${PULSE_PROXY_CHANNEL:-stable}"
DETERMINED_PROXY_VERSION=""
PROXY_INSTALLER_SOURCE_LABEL="unset"
PROXY_ARCH_LABEL=""

compute_proxy_arch_label() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64)
            PROXY_ARCH_LABEL="linux-amd64"
            ;;
        aarch64 | arm64)
            PROXY_ARCH_LABEL="linux-arm64"
            ;;
        armv7l | armhf)
            PROXY_ARCH_LABEL="linux-armv7"
            ;;
        *)
            PROXY_ARCH_LABEL=""
            ;;
    esac
}

set_github_fallback_url() {
    local ref="$1"
    if [[ -z "$PROXY_ARCH_LABEL" ]]; then
        return
    fi

    local base=""
    if [[ "$ref" == "main" ]]; then
        base="https://raw.githubusercontent.com/rcourtman/Pulse/${ref}/release/pulse-sensor-proxy-${PROXY_ARCH_LABEL}"
    else
        base="https://github.com/rcourtman/Pulse/releases/download/${ref}/pulse-sensor-proxy-${PROXY_ARCH_LABEL}"
    fi

    export PULSE_SENSOR_PROXY_FALLBACK_URL="$base"
}

EMBEDDED_INSTALLER_ARCHIVE_B64=$(cat <<'EOF'
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
EOF
)

determine_proxy_release() {
    if [[ -n "${PULSE_PROXY_VERSION:-}" ]]; then
        DETERMINED_PROXY_VERSION="${PULSE_PROXY_VERSION}"
        return 0
    fi

    local channel="${PULSE_PROXY_CHANNEL:-stable}"
    local tag=""
    local api_url=""

    if command -v curl >/dev/null 2>&1; then
        if [[ "$channel" == "rc" ]]; then
            api_url="https://api.github.com/repos/rcourtman/Pulse/releases"
            tag=$(curl -fsSL --connect-timeout 5 --max-time 15 "$api_url" 2>/dev/null | grep '"tag_name":' | head -1 | sed -E 's/.*"([^"]+)".*/\1/' || true)
        else
            api_url="https://api.github.com/repos/rcourtman/Pulse/releases/latest"
            tag=$(curl -fsSL --connect-timeout 5 --max-time 15 "$api_url" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || true)
        fi
    fi

    if [[ -n "$tag" ]]; then
        DETERMINED_PROXY_VERSION="$tag"
        return 0
    fi

    DETERMINED_PROXY_VERSION="main"
    return 1
}

download_installer_from_local() {
    local destination="$1"
    local base_url="${PULSE_SERVER:-http://localhost:7655}"
    local url="${base_url%/}/api/install/install-sensor-proxy.sh"

    if curl --fail --silent --location --connect-timeout 5 --max-time 20 "$url" -o "$destination" 2>/dev/null; then
        chmod +x "$destination"
        PROXY_INSTALLER_SOURCE_LABEL="local"
        export PULSE_SENSOR_PROXY_FALLBACK_URL="${base_url%/}/api/install/pulse-sensor-proxy"
        echo "  ✓ Downloaded proxy installer from ${url}"
        return 0
    fi

    echo "  ⚠️  Unable to download proxy installer from ${url}"
    return 1
}

download_installer_from_github() {
    local destination="$1"
    determine_proxy_release
    local tag="$DETERMINED_PROXY_VERSION"
    local ref="$tag"
    local attempted=false

    if [[ "$tag" == "main" ]]; then
        ref="main"
    fi

    if [[ "$tag" != "main" ]]; then
        local asset_url="https://github.com/rcourtman/Pulse/releases/download/${tag}/install-sensor-proxy.sh"
        attempted=true
        if curl --fail --silent --location --connect-timeout 5 --max-time 30 "$asset_url" -o "$destination" 2>/dev/null; then
            chmod +x "$destination"
            PROXY_INSTALLER_SOURCE_LABEL="github:${tag}"
            set_github_fallback_url "$tag"
            echo "  ✓ Downloaded proxy installer from ${asset_url}"
            return 0
        fi
    fi

    local raw_url="https://raw.githubusercontent.com/rcourtman/Pulse/${ref}/scripts/install-sensor-proxy.sh"
    attempted=true
    if curl --fail --silent --location --connect-timeout 5 --max-time 30 "$raw_url" -o "$destination" 2>/dev/null; then
        chmod +x "$destination"
        PROXY_INSTALLER_SOURCE_LABEL="github:${ref}"
        set_github_fallback_url "$ref"
        echo "  ✓ Downloaded proxy installer from ${raw_url}"
        return 0
    fi

    if [[ "$attempted" == true ]]; then
        echo "  ⚠️  Unable to download proxy installer from GitHub (tag=${tag})"
    fi
    return 1
}

write_embedded_installer() {
    local destination="$1"

    if ! command -v base64 >/dev/null 2>&1; then
        echo "  ⚠️  base64 command not available; cannot use embedded installer"
        return 1
    fi

    if ! command -v gzip >/dev/null 2>&1; then
        echo "  ⚠️  gzip command not available; cannot use embedded installer"
        return 1
    fi

    local decode_output
    if decode_output=$(printf '%s' "$EMBEDDED_INSTALLER_ARCHIVE_B64" | base64 -d 2>&1 | gzip -dc 2>&1 >"$destination"); then
        chmod +x "$destination"
        PROXY_INSTALLER_SOURCE_LABEL="embedded"
        echo "  ✓ Used embedded proxy installer fallback"
        return 0
    fi

    echo "  ⚠️  Embedded proxy installer fallback failed"
    if [[ -n "$decode_output" ]]; then
        echo "      → $decode_output"
    fi
    return 1
}

download_proxy_installer() {
    local destination="$1"

    if download_installer_from_local "$destination"; then
        return 0
    fi

    echo "  Attempting GitHub fallback..."
    if download_installer_from_github "$destination"; then
        return 0
    fi

    echo "  Attempting embedded installer fallback..."
    if write_embedded_installer "$destination"; then
        return 0
    fi

    return 1
}


# ============================================
# Helper Functions
# ============================================

validate_socket() {
    local socket_path="$1"

    # Check if it's a socket file
    if [ ! -S "$socket_path" ]; then
        return 1
    fi

    # Test if we can connect to it (using timeout to avoid hangs)
    if command -v socat &>/dev/null; then
        if timeout 2 socat -u OPEN:/dev/null UNIX-CONNECT:"$socket_path" 2>/dev/null; then
            return 0
        else
            return 1
        fi
    fi

    # If socat not available, assume socket is valid if it exists as a socket
    return 0
}

# ============================================
# Pre-flight Checks
# ============================================

compute_proxy_arch_label

echo "============================================"
echo "  Pulse Turnkey Docker Installation"
echo "============================================"
echo ""

# Check if running as root (early check for better error messages)
if [ "$EUID" -ne 0 ]; then
    echo "❌ ERROR: This script must be run as root"
    echo ""
    echo "Please run: sudo $0"
    exit 1
fi

# Detect if running in a container
if [ -f /.dockerenv ] || [ -f /run/.containerenv ]; then
    echo "❌ ERROR: This script must run on the Docker host, not inside a container"
    echo ""
    echo "Please run this script on your Docker host machine."
    exit 1
fi

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "❌ ERROR: Docker is not installed"
    echo ""
    echo "Please install Docker first:"
    echo "  curl -fsSL https://get.docker.com | sh"
    exit 1
fi

# Check if docker compose is available
if ! docker compose version &> /dev/null; then
    echo "⚠️  Warning: 'docker compose' command not found"
    echo "   You may need to use 'docker-compose' instead"
    echo ""
fi

# ============================================
# Socket Detection & Deconfliction
# ============================================

BIND_MOUNT_SOCKET="/mnt/pulse-proxy/pulse-sensor-proxy.sock"
LOCAL_SOCKET="/run/pulse-sensor-proxy/pulse-sensor-proxy.sock"
SOCKET_PATH=""
SKIP_INSTALLATION=false

echo "Checking for existing pulse-sensor-proxy..."
echo ""

# Check for bind-mounted socket (LXC scenario)
if [ -S "$BIND_MOUNT_SOCKET" ]; then
    echo "  Found socket at /mnt/pulse-proxy (bind-mounted from host)"
    if validate_socket "$BIND_MOUNT_SOCKET"; then
        echo "  ✓ Socket is functional"
        SOCKET_PATH="/mnt/pulse-proxy"
        SKIP_INSTALLATION=true

        # Deconflict: if local proxy also exists, stop it
        if systemctl is-active --quiet pulse-sensor-proxy 2>/dev/null; then
            echo "  ⚠️  Found conflicting local pulse-sensor-proxy service"
            echo "     Stopping local service to avoid conflicts..."
            systemctl stop pulse-sensor-proxy
            systemctl disable pulse-sensor-proxy 2>/dev/null || true
        fi
    else
        echo "  ⚠️  Socket exists but is not responsive - will install local proxy"
        SKIP_INSTALLATION=false
    fi
fi

# Check for existing local installation
if [ -S "$LOCAL_SOCKET" ] && [ "$SKIP_INSTALLATION" = false ]; then
    echo "  Found socket at /run/pulse-sensor-proxy (local installation)"
    if validate_socket "$LOCAL_SOCKET"; then
        echo "  ✓ Socket is functional"
        SOCKET_PATH="/run/pulse-sensor-proxy"
        SKIP_INSTALLATION=true
    else
        echo "  ⚠️  Socket exists but is not responsive - will reinstall"
        systemctl stop pulse-sensor-proxy 2>/dev/null || true
        SKIP_INSTALLATION=false
    fi
fi

# ============================================
# Proxy Installation (if needed)
# ============================================

if [ "$SKIP_INSTALLATION" = true ]; then
    echo ""
    echo "✓ Using existing pulse-sensor-proxy at ${SOCKET_PATH}"
    echo ""
else
    echo "  No functional socket found - installing pulse-sensor-proxy..."
    echo ""

    # Download and run the proxy installer
    PROXY_INSTALLER="/tmp/install-sensor-proxy-$$.sh"

    unset PULSE_SENSOR_PROXY_FALLBACK_URL

    if ! download_proxy_installer "$PROXY_INSTALLER"; then
        echo "❌ ERROR: Could not obtain pulse-sensor-proxy installer"
        echo ""
        echo "Sources attempted:"
        echo "  • ${PULSE_SERVER:-http://localhost:7655}/api/install/install-sensor-proxy.sh"
        if [[ -n "$DETERMINED_PROXY_VERSION" ]]; then
            echo "  • GitHub release/tag: ${DETERMINED_PROXY_VERSION}"
        else
            echo "  • GitHub releases (auto-detected)"
        fi
        echo "  • Embedded fallback installer"
        echo ""
        echo "Hints:"
        echo "  1. Ensure network connectivity to the Pulse host or GitHub."
        echo "  2. Set PULSE_PROXY_VERSION (e.g. v4.24.0) to pin a specific release."
        echo "  3. Confirm curl is allowed through firewalls/proxies."
        rm -f "$PROXY_INSTALLER"
        exit 1
    fi

    case "$PROXY_INSTALLER_SOURCE_LABEL" in
        local)
            # already set during download
            ;;
        github:*)
            # fallback URL set in download helper
            ;;
        *)
            unset PULSE_SENSOR_PROXY_FALLBACK_URL
            ;;
    esac

    # Run installer in standalone mode (no container)
    if ! "$PROXY_INSTALLER" --standalone --pulse-server "${PULSE_SERVER:-http://localhost:7655}" --quiet; then
        echo "❌ Proxy installation failed"
        rm -f "$PROXY_INSTALLER"
        exit 1
    fi

    rm -f "$PROXY_INSTALLER"

    echo ""
    echo "✓ pulse-sensor-proxy installed successfully"
    echo ""

    # Validate newly installed socket
    if ! validate_socket "$LOCAL_SOCKET"; then
        echo "⚠️  Warning: Proxy installed but socket is not responsive"
        echo "   Temperature monitoring may not work correctly"
        echo ""
    fi

    SOCKET_PATH="/run/pulse-sensor-proxy"
fi

# ============================================
# Final Socket Validation
# ============================================

if [ -z "$SOCKET_PATH" ]; then
    echo "❌ ERROR: No functional socket available after installation"
    echo ""
    echo "Please check:"
    echo "  1. systemctl status pulse-sensor-proxy"
    echo "  2. journalctl -u pulse-sensor-proxy -n 50"
    exit 1
fi

# ============================================
# Generate Docker Compose Configuration
# ============================================

COMPOSE_FILE="./docker-compose.yml"

# Check if docker-compose.yml already exists (idempotency)
if [ -f "$COMPOSE_FILE" ]; then
    echo "⚠️  docker-compose.yml already exists"
    echo "   Backing up to docker-compose.yml.backup"
    cp "$COMPOSE_FILE" "${COMPOSE_FILE}.backup"
fi

cat > "$COMPOSE_FILE" << 'COMPOSE_EOF'
version: '3.8'

services:
  pulse:
    image: ${PULSE_IMAGE:-rcourtman/pulse:latest}
    container_name: pulse
    restart: unless-stopped
    user: "1000:1000"
    security_opt:
      - apparmor=unconfined
    ports:
      - "${PULSE_PORT:-7655}:7655"
    volumes:
      - pulse-data:/data
      # Secure temperature monitoring via host-side proxy
COMPOSE_EOF

# Add socket mount with detected path
echo "      - ${SOCKET_PATH}:/mnt/pulse-proxy:ro" >> "$COMPOSE_FILE"

# Continue compose file
cat >> "$COMPOSE_FILE" << 'COMPOSE_EOF'
    environment:
      - TZ=${TZ:-UTC}
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:7655/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

volumes:
  pulse-data:
    driver: local
COMPOSE_EOF

echo "✓ Generated docker-compose.yml"
echo "  Socket mount: ${SOCKET_PATH}:/mnt/pulse-proxy:ro"
echo ""

# Create .env file with defaults
ENV_FILE=".env"
if [ -f "$ENV_FILE" ]; then
    echo "⚠️  .env file already exists - not overwriting"
    echo ""
else
    cat > "$ENV_FILE" << EOF
PULSE_IMAGE=${PULSE_IMAGE}
PULSE_PORT=${PULSE_PORT}
TZ=$(timedatectl show -p Timezone --value 2>/dev/null || echo "UTC")
EOF
    echo "✓ Generated .env file"
    echo ""
fi

# ============================================
# Installation Complete
# ============================================

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Installation Complete!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Socket location: ${SOCKET_PATH}"
echo ""
echo "Start Pulse with:"
echo "  docker compose up -d"
echo ""
echo "Or with docker run:"
echo "  docker run -d \\"
echo "    --name pulse \\"
echo "    --user 1000:1000 \\"
echo "    --security-opt apparmor=unconfined \\"
echo "    --restart unless-stopped \\"
echo "    -p ${PULSE_PORT}:7655 \\"
echo "    -v pulse-data:/data \\"
echo "    -v ${SOCKET_PATH}:/mnt/pulse-proxy:ro \\"
echo "    ${PULSE_IMAGE}"
echo ""
echo "Access Pulse at: http://$(hostname -I | awk '{print $1}'):${PULSE_PORT}"
echo ""
echo "Features enabled:"
echo "  ✓ Secure temperature monitoring (via host-side proxy)"
echo "  ✓ Automatic restarts"
echo "  ✓ Persistent data storage"
echo ""
