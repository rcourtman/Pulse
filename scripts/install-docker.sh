#!/bin/bash
# install-docker.sh - Turnkey Pulse installation for Docker hosts
# Generates docker-compose.yml and .env

set -euo pipefail

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
DOCKER_IMAGE_REPO="${DOCKER_IMAGE_REPO:-rcourtman/pulse}"
CANONICAL_DEFAULT_PULSE_VERSION="6.6.0-rc.6"

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
ENV_FILE=".env"

# This is a fresh-configuration generator, not an updater. Replacing an
# existing deployment can lose its mounts, trust and service settings; a
# repeatedly overwritten .backup is not recovery. Also refuse Compose's other
# default names and overrides rather than silently combining two deployments.
for existing_compose in docker-compose.yml docker-compose.yaml compose.yml compose.yaml \
    docker-compose.override.yml docker-compose.override.yaml compose.override.yml compose.override.yaml; do
    if [ -e "$existing_compose" ] || [ -L "$existing_compose" ]; then
        printf '%s\n' "ERROR: Existing Compose configuration ($existing_compose); no files changed." \
            "Keep your existing deployment configuration, or generate a new one in an empty directory." >&2
        exit 1
    fi
done

# An ordinary operator-owned .env is retained. Do not follow links (including
# dangling ones), write to a pipe, or create Compose before discovering an
# unusable environment path.
if { [ -e "$ENV_FILE" ] || [ -L "$ENV_FILE" ]; } && \
    { [ ! -f "$ENV_FILE" ] || [ -L "$ENV_FILE" ]; }; then
    printf '%s\n' "ERROR: .env must be a regular file, not a link or special path; no files changed." >&2
    exit 1
fi

# Render privately on the destination filesystem before exposing either file.
# noclobber alone still leaves partial files after a failed write, and can open
# special paths raced into place after preflight. Exclusive hard links publish
# complete files without replacing or following any destination (including a
# directory). POSIX link has exactly two path operands, unlike ln's implicit
# target-directory handling. A missing/refused link must stop, not fall back.
umask 077
STAGING_DIR=$(mktemp -d ./.pulse-docker.XXXXXX)
cleanup_generation() {
    local status=$? file
    for file in "$STAGING_DIR/compose" "$STAGING_DIR/env"; do
        if [ -e "$file" ]; then
            rm -- "$file" || status=1
        fi
    done
    rmdir -- "$STAGING_DIR" || status=1
    return "$status"
}
trap cleanup_generation EXIT

cat > "$STAGING_DIR/compose" << COMPOSE_EOF
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

# Create .env file with defaults
if [ -e "$ENV_FILE" ] || [ -L "$ENV_FILE" ]; then
    if [ ! -f "$ENV_FILE" ] || [ -L "$ENV_FILE" ]; then
        printf '%s\n' "ERROR: .env changed to an unsafe path; configuration was not published." >&2
        exit 1
    fi
    PRESERVE_ENV=true
else
    PRESERVE_ENV=false
    cat > "$STAGING_DIR/env" << EOF
PULSE_IMAGE=${PULSE_IMAGE}
PULSE_PORT=${PULSE_PORT}
TZ=$(timedatectl show -p Timezone --value 2>/dev/null || echo "UTC")
EOF
    if ! link "$STAGING_DIR/env" "$ENV_FILE"; then
        printf '%s\n' "ERROR: Could not publish .env without replacing an existing path; configuration was not completed." >&2
        exit 1
    fi
fi

if ! link "$STAGING_DIR/compose" "$COMPOSE_FILE"; then
    printf '%s\n' "ERROR: Could not publish docker-compose.yml without replacing an existing path; configuration was not completed." \
        "Any complete .env already generated is retained; no existing configuration was overwritten." >&2
    exit 1
fi

echo "✓ Generated docker-compose.yml"
echo ""
if [ "$PRESERVE_ENV" = true ]; then
    echo "⚠️  .env file already exists - not overwriting"
    echo ""
else
    echo "✓ Generated .env file"
    echo ""
fi

# ============================================
# Configuration Generated (no container has been installed or started)
# ============================================

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✓ Configuration generated; Pulse has not been started."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Review docker-compose.yml and .env, then validate and start Pulse:"
echo "  docker compose --env-file ./.env -f ./docker-compose.yml config --quiet"
echo "  docker compose --env-file ./.env -f ./docker-compose.yml up -d"
echo ""
echo "These commands select the generated Compose file and its .env."
echo "Shell variables can override .env; review exported PULSE_IMAGE and PULSE_PORT values."
echo ""
echo "After starting, check the published port with:"
echo "  docker compose --env-file ./.env -f ./docker-compose.yml ps"
echo "Open your Docker host address at that port."
echo ""
echo "Notes:"
echo "  - For temperature monitoring, use the unified agent on Proxmox hosts."
echo "  - SSH-based temperature collection from containers is not recommended."
echo ""
