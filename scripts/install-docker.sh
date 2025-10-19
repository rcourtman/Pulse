#!/bin/bash
# install-docker.sh - Turnkey Pulse installation for Docker hosts
# This script installs pulse-sensor-proxy and generates docker-compose.yml

set -euo pipefail

PULSE_IMAGE="${PULSE_IMAGE:-rcourtman/pulse:latest}"
PULSE_PORT="${PULSE_PORT:-7655}"

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

echo "============================================"
echo "  Pulse Turnkey Docker Installation"
echo "============================================"
echo ""

# Check if running as root (early check per Codex feedback)
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
    INSTALLER_URL="${PULSE_SERVER:-http://localhost:7655}/api/install/install-sensor-proxy.sh"

    if ! curl --fail --silent --location "$INSTALLER_URL" -o "$PROXY_INSTALLER" 2>/dev/null; then
        echo "❌ ERROR: Could not download installer from Pulse server"
        echo ""
        echo "Please ensure:"
        echo "  1. Pulse server is running at ${PULSE_SERVER:-http://localhost:7655}"
        echo "  2. Network connectivity is available"
        echo ""
        echo "Alternatively, download from GitHub releases (coming soon)"
        exit 1
    fi

    chmod +x "$PROXY_INSTALLER"

    # Set fallback URL for proxy binary download
    export PULSE_SENSOR_PROXY_FALLBACK_URL="${PULSE_SERVER:-http://localhost:7655}/api/install/pulse-sensor-proxy"

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
