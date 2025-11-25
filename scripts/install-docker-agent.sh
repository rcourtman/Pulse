#!/usr/bin/env bash
#
# DEPRECATED: pulse-docker-agent installer
#
# This script is deprecated. Please use the unified agent instead:
#
#   curl -fsSL http://<pulse-server>:7655/install.sh | \
#     sudo bash -s -- --url http://<pulse-server>:7655 --token <token> --enable-docker
#
# The unified agent provides:
#   - Combined host + Docker monitoring in one binary
#   - Automatic updates
#   - Simplified management
#
# This script will redirect you to the unified agent installer.
#

set -euo pipefail

echo ""
echo "┌─────────────────────────────────────────────────────────────────────────────┐"
echo "│  DEPRECATED: pulse-docker-agent has been replaced by the unified agent.     │"
echo "│                                                                             │"
echo "│  The unified agent provides:                                                │"
echo "│    • Combined host + Docker monitoring in one binary                        │"
echo "│    • Automatic updates                                                      │"
echo "│    • Simplified management                                                  │"
echo "│                                                                             │"
echo "│  Installing unified agent instead...                                        │"
echo "└─────────────────────────────────────────────────────────────────────────────┘"
echo ""

# Parse arguments to extract URL and token for redirection
PULSE_URL=""
PULSE_TOKEN=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --url) PULSE_URL="$2"; shift 2 ;;
        --token) PULSE_TOKEN="$2"; shift 2 ;;
        *) shift ;; # ignore other args
    esac
done

if [[ -z "$PULSE_URL" ]]; then
    echo "[ERROR] --url is required"
    echo ""
    echo "Usage:"
    echo "  curl -fsSL http://<pulse-server>:7655/install.sh | \\"
    echo "    sudo bash -s -- --url http://<pulse-server>:7655 --token <token> --enable-docker"
    exit 1
fi

if [[ -z "$PULSE_TOKEN" ]]; then
    echo "[ERROR] --token is required"
    echo ""
    echo "Usage:"
    echo "  curl -fsSL ${PULSE_URL}/install.sh | \\"
    echo "    sudo bash -s -- --url ${PULSE_URL} --token <token> --enable-docker"
    exit 1
fi

# Download and run the unified installer with --enable-docker
echo "[INFO] Downloading unified agent installer..."
curl -fsSL "${PULSE_URL}/install.sh" | bash -s -- --url "$PULSE_URL" --token "$PULSE_TOKEN" --enable-docker
