#!/bin/bash
# install-docker.sh - Turnkey Pulse installation for Docker hosts
# Generates docker-compose.yml and .env

set -euo pipefail

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
DOCKER_IMAGE_REPO="${DOCKER_IMAGE_REPO:-rcourtman/pulse}"
CANONICAL_DEFAULT_PULSE_VERSION="6.9.0-beta.1"

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
# Configuration Preparation
# ============================================

echo "============================================"
echo "  Pulse Docker Configuration"
echo "============================================"
echo ""

# This only writes configuration in the caller's directory. Preparing it needs
# neither root nor a Docker installation/daemon, and may happen on another
# machine or inside a container. Runtime permissions belong to the explicit
# start command on the deployment host, not this generator.
echo "Prepare these files without sudo; Docker is only needed to start Pulse."
echo ""

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
      # Use the image's HTTPS-aware probe; HTTP is only for images without it.
      test: ["CMD-SHELL", "if [ -x /docker-healthcheck.sh ]; then exec /docker-healthcheck.sh; else exec wget --quiet --tries=1 --spider http://localhost:7655/api/health; fi"]
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
echo "On the deployment host, install Docker with Compose, then start Pulse with:"
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
