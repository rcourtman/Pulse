#!/bin/bash

# migrate-temperature-proxy.sh
# Converts SSH-based temperature collection to pulse-sensor-proxy on standalone hosts.

set -euo pipefail

PULSE_SERVER=""
KEEP_SSH_KEYS=false
SKIP_RESTART=false

print_usage() {
    cat <<'EOF'
Usage: migrate-temperature-proxy.sh --pulse-server https://pulse.example.com:7655 [--keep-ssh-keys] [--skip-restart]

Installs pulse-sensor-proxy in standalone mode and removes legacy SSH fallback keys.
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
        --keep-ssh-keys)
            KEEP_SSH_KEYS=true
            shift
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

INSTALLER_URL="${PULSE_SERVER%/}/api/install/install-sensor-proxy.sh"
INSTALLER=$(mktemp)
cleanup() {
    rm -f "$INSTALLER"
}
trap cleanup EXIT

log "Downloading pulse-sensor-proxy installer from $INSTALLER_URL"
if ! curl -fsSL "$INSTALLER_URL" -o "$INSTALLER"; then
    err "Failed to download installer script"
    exit 1
fi
chmod +x "$INSTALLER"

INSTALL_ARGS=(--standalone --http-mode --pulse-server "$PULSE_SERVER")
if [[ "$SKIP_RESTART" == true ]]; then
    INSTALL_ARGS+=(--skip-restart)
fi

log "Running pulse-sensor-proxy installer..."
if ! "$INSTALLER" "${INSTALL_ARGS[@]}"; then
    err "pulse-sensor-proxy installation failed"
    exit 1
fi

remove_legacy_keys() {
    local auth="/root/.ssh/authorized_keys"
    if [[ ! -f "$auth" ]]; then
        return
    fi
    local tmp
    tmp=$(mktemp)
    if grep -v -E '# pulse-sensors|# pulse-proxy-key' "$auth" > "$tmp"; then
        chmod --reference="$auth" "$tmp" 2>/dev/null || chmod 600 "$tmp"
        chown --reference="$auth" "$tmp" 2>/dev/null || true
        mv "$tmp" "$auth"
        log "Removed legacy SSH fallback keys from $auth"
    else
        rm -f "$tmp"
    fi
}

if [[ "$KEEP_SSH_KEYS" == false ]]; then
    remove_legacy_keys
else
    warn "Legacy SSH keys retained at user request (--keep-ssh-keys)"
fi

log "Migration complete. This host now uses pulse-sensor-proxy for temperature collection."
log "Verify status with: systemctl status pulse-sensor-proxy --no-pager"
