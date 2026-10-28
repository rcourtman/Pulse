#!/bin/bash
# install-docker.sh - Turnkey Pulse installation for Docker hosts
# Generates docker-compose.yml and .env

set -euo pipefail

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
DOCKER_IMAGE_REPO="${DOCKER_IMAGE_REPO:-rcourtman/pulse}"
CANONICAL_DEFAULT_PULSE_VERSION="6.7.0-beta.3"

resolve_default_pulse_version() {
    if [ -n "${PULSE_IMAGE_VERSION:-}" ]; then
        printf '%s' "${PULSE_IMAGE_VERSION}"
        return
    fi

    for candidate in "${PULSE_VERSION_FILE:-}" "${SCRIPT_DIR}/../VERSION" "${PWD}/VERSION"; do
        if [ -n "${candidate}" ] && [ -f "${candidate}" ]; then
            local version
            version=$(tr -d '\r\n' < "${candidate}" 2>/dev/null || true)
            if [ -n "${version}" ]; then
                printf '%s' "${version}"
                return
            fi
        fi
    done

    printf '%s' "${CANONICAL_DEFAULT_PULSE_VERSION}"
}

DEFAULT_PULSE_IMAGE="${DOCKER_IMAGE_REPO}:$(resolve_default_pulse_version)"
PULSE_IMAGE="${PULSE_IMAGE:-${DEFAULT_PULSE_IMAGE}}"
PULSE_PORT="${PULSE_PORT:-7655}"

# This helper creates a new deployment; it is not an updater. Do not replace
# an operator's ports, mounts, image pins or credentials on a repeated run.
# Check every Compose filename that Docker recognises, including dangling links
# and non-regular paths, before creating either output or calling Docker.
COMPOSE_FILE="./docker-compose.yml"
ENV_FILE="./.env"
for existing in compose.yaml compose.yml docker-compose.yaml docker-compose.yml; do
    if [ -e "$existing" ] || [ -L "$existing" ]; then
        echo "ERROR: A Compose path already exists: $existing" >&2
        echo "This helper only prepares a new deployment. Update using your existing Compose configuration." >&2
        echo "Existing files and backups were left unchanged." >&2
        exit 1
    fi
done
if [ -L "$ENV_FILE" ] || { [ -e "$ENV_FILE" ] && [ ! -f "$ENV_FILE" ]; }; then
    echo "ERROR: .env must be a regular, non-symlink file or absent. Existing paths were left unchanged." >&2
    exit 1
fi

# New configuration is private, and an output created after the checks above
# must never be truncated. Noclobber also refuses dangling output symlinks.
umask 077
set -o noclobber

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

cat > "$COMPOSE_FILE" << COMPOSE_EOF
version: '3.8'

services:
  pulse:
    image: \${PULSE_IMAGE:-${DEFAULT_PULSE_IMAGE}}
    container_name: pulse
    restart: unless-stopped
    user: "1000:1000"
    security_opt:
      - apparmor=unconfined
    ports:
      - "\${PULSE_PORT:-7655}:7655"
    volumes:
      - pulse-data:/data
    environment:
      - TZ=\${TZ:-UTC}
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
# Configuration Prepared
# ============================================

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✓ Configuration prepared; Pulse has not been started."
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
