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

remove_control_block() {
    python3 - "$CONFIG_FILE" <<'PY'
from pathlib import Path
import sys
path = Path(sys.argv[1])
if not path.exists():
    sys.exit(0)
lines = path.read_text().splitlines(keepends=True)
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
path.write_text("".join(result))
PY
}

log "Updating config..."
remove_control_block
cat >> "$CONFIG_FILE" <<EOF

# Pulse control plane configuration (added by migrate-sensor-proxy-control-plane.sh)
pulse_control_plane:
  url: "$PULSE_SERVER"
  token_file: "$TOKEN_FILE"
  refresh_interval: $REFRESH_INTERVAL
EOF

if [[ "$SKIP_RESTART" == false ]]; then
    log "Restarting pulse-sensor-proxy..."
    systemctl restart pulse-sensor-proxy
else
    warn "Skipping service restart; control-plane sync will start on next restart"
fi

log "Migration complete."
