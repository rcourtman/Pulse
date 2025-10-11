#!/usr/bin/env bash
set -euo pipefail

# Simple helper that spins up a local Dex server to act as a mock OIDC provider for dev/testing.
# Requires Docker. The container exposes the issuer on http://127.0.0.1:5556/dex
# and registers a static client `pulse-dev` with secret `pulse-secret`.

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CONFIG_FILE="$PROJECT_ROOT/dev/oidc/dex-config.yaml"
CONTAINER_NAME="pulse-oidc-mock"
DEX_IMAGE="ghcr.io/dexidp/dex:v2.38.0"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required to run the mock OIDC provider" >&2
  exit 1
fi

if [ ! -f "$CONFIG_FILE" ]; then
  echo "missing Dex config at $CONFIG_FILE" >&2
  exit 1
fi

# Stop an existing container if it is already running
if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  echo "Stopping existing ${CONTAINER_NAME} container..."
  docker rm -f "$CONTAINER_NAME" >/dev/null
fi

echo "Starting Dex mock OIDC provider on http://127.0.0.1:5556/dex"
docker run \
  --rm \
  --name "$CONTAINER_NAME" \
  -p 5556:5556 \
  -v "$CONFIG_FILE:/etc/dex/config.yaml:ro" \
  "$DEX_IMAGE" \
  dex serve /etc/dex/config.yaml
