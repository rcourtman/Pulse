#!/bin/bash

# migrate-sensor-proxy-control-plane.sh
# Adds control-plane sync to existing pulse-sensor-proxy installations.

set -euo pipefail

PULSE_SERVER=""
SKIP_RESTART=false

print_usage() {
    cat <<'EOF'
Usage: migrate-sensor-proxy-control-plane.sh --pulse-server https://pulse.example.com:7655 [--skip-restart]

Adds the Pulse control-plane token/config to an existing proxy installation.
EOF
}

log()  { echo "[INFO] $*"; }
warn() { echo "[WARN] $*" >&2; }
err()  { echo "[ERROR] $*" >&2; }

while [[ $# -gt 0 ]]; do
    case "$1" in
        --pulse-server)
            PULSE_SERVER="$2"
            shift 2
            ;;
        --skip-restart)
            SKIP_RESTART=true
            shift
            ;;
        -h|--help)
            print_usage
            exit 0
            ;;
        *)
            err "Unknown option: $1"
            print_usage
            exit 1
            ;;
    esac
done

if [[ $EUID -ne 0 ]]; then
    err "This script must be run as root"
    exit 1
fi

if [[ -z "$PULSE_SERVER" ]]; then
    err "--pulse-server is required"
    exit 1
fi

CONFIG_FILE="/etc/pulse-sensor-proxy/config.yaml"
TOKEN_FILE="/etc/pulse-sensor-proxy/.pulse-control-token"

if [[ ! -f "$CONFIG_FILE" ]]; then
    err "Config file not found at $CONFIG_FILE"
    exit 1
fi

SHORT_HOSTNAME=$(hostname -s 2>/dev/null || hostname | cut -d'.' -f1)
register_proxy() {
    local mode="$1"
    local payload="{\"hostname\":\"${SHORT_HOSTNAME}\",\"proxy_url\":\"\",\"mode\":\"${mode}\"}"
    curl -fsSL -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "${PULSE_SERVER%/}/api/temperature-proxy/register"
}

log "Registering proxy with Pulse..."
REGISTRATION_RESPONSE=$(register_proxy "socket") || {
    err "Registration failed"
    exit 1
}

CONTROL_TOKEN=$(echo "$REGISTRATION_RESPONSE" | grep -o '"control_token":"[^"]*"' | head -1 | cut -d'"' -f4)
REFRESH_INTERVAL=$(echo "$REGISTRATION_RESPONSE" | grep -o '"refresh_interval":[0-9]*' | head -1 | awk -F: '{print $2}')
if [[ -z "$CONTROL_TOKEN" ]]; then
    err "Pulse did not return a control token. Response: $REGISTRATION_RESPONSE"
    exit 1
fi
if [[ -z "$REFRESH_INTERVAL" ]]; then
    REFRESH_INTERVAL=60
fi

log "Writing control-plane token..."
mkdir -p "$(dirname "$TOKEN_FILE")"
echo "$CONTROL_TOKEN" > "$TOKEN_FILE"
chmod 600 "$TOKEN_FILE"
chown pulse-sensor-proxy:pulse-sensor-proxy "$TOKEN_FILE"

update_config_atomically() {
    # Phase 2: Use atomic write to prevent corruption
    # NOTE: This script is for one-time migration from v4.31 to v4.32+
    # It uses atomic write but doesn't use config.yaml.lock (minor race risk)
    # Future: Deprecate this script once all users are on v4.32+

    # Create temp file in same directory to ensure rename is atomic
    local config_dir
    config_dir=$(dirname "$CONFIG_FILE")
    local temp_file
    temp_file=$(mktemp "$config_dir/.config.XXXXXX")

    # Remove old control plane blocks and add new one atomically
    python3 - "$CONFIG_FILE" "$temp_file" <<'PY'
from pathlib import Path
import sys
config_path = Path(sys.argv[1])
temp_path = Path(sys.argv[2])
if not config_path.exists():
    sys.exit(0)
lines = config_path.read_text().splitlines(keepends=True)
result = []
i = 0
while i < len(lines):
    line = lines[i]
    if line.startswith("# Pulse control plane configuration"):
        i += 1
        while i < len(lines) and (lines[i].startswith(" ") or lines[i].startswith("\t") or lines[i].strip() == ""):
            i += 1
        continue
    if line.startswith("pulse_control_plane:"):
        i += 1
        while i < len(lines) and (lines[i].startswith(" ") or lines[i].startswith("\t") or lines[i].strip() == ""):
            i += 1
        continue
    result.append(line)
    i += 1
temp_path.write_text("".join(result))
PY

    # Append new control plane config
    cat >> "$temp_file" <<EOF

# Pulse control plane configuration (added by migrate-sensor-proxy-control-plane.sh)
pulse_control_plane:
  url: "$PULSE_SERVER"
  token_file: "$TOKEN_FILE"
  refresh_interval: $REFRESH_INTERVAL
EOF

    # Atomic rename
    mv "$temp_file" "$CONFIG_FILE"
    chmod 644 "$CONFIG_FILE"
    chown pulse-sensor-proxy:pulse-sensor-proxy "$CONFIG_FILE" 2>/dev/null || true
}

log "Stopping service to prevent config file races..."
if systemctl is-active --quiet pulse-sensor-proxy; then
    systemctl stop pulse-sensor-proxy
    SERVICE_WAS_RUNNING=true
else
    SERVICE_WAS_RUNNING=false
fi

log "Updating config..."
update_config_atomically

if [[ "$SKIP_RESTART" == false ]] && [[ "$SERVICE_WAS_RUNNING" == true ]]; then
    log "Starting pulse-sensor-proxy..."
    systemctl start pulse-sensor-proxy
elif [[ "$SERVICE_WAS_RUNNING" == false ]]; then
    log "Service was not running; leaving stopped"
else
    warn "Skipping service restart; control-plane sync will start on next restart"
fi

log "Migration complete."
