#!/bin/bash
# install-docker.sh - Turnkey Pulse installation for Docker hosts
# Generates docker-compose.yml and .env

set -euo pipefail

PULSE_IMAGE="${PULSE_IMAGE:-rcourtman/pulse:latest}"
PULSE_PORT="${PULSE_PORT:-7655}"

# ============================================
# Pre-flight Checks
# ============================================

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
echo "    ${PULSE_IMAGE}"
echo ""
echo "Access Pulse at: http://$(hostname -I | awk '{print $1}'):${PULSE_PORT}"
echo ""
echo "Notes:"
echo "  - For temperature monitoring, use the unified agent on Proxmox hosts."
echo "  - SSH-based temperature collection from containers is not recommended."
echo ""
