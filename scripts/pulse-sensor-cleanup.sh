#!/bin/bash

# pulse-sensor-cleanup.sh - Complete Pulse footprint removal when nodes are removed
# Removes: SSH keys, proxy service, binaries, API tokens, and LXC bind mounts
# This script is triggered by systemd path unit when cleanup-request.json is created

set -euo pipefail

# Configuration
WORK_DIR="/var/lib/pulse-sensor-proxy"
CLEANUP_REQUEST="${WORK_DIR}/cleanup-request.json"
LOCKFILE="${WORK_DIR}/cleanup.lock"
LOG_TAG="pulse-sensor-cleanup"
INSTALLER_PATH="/opt/pulse/sensor-proxy/install-sensor-proxy.sh"

# Logging functions
log_info() {
    logger -t "$LOG_TAG" -p user.info "$1"
    echo "[INFO] $1"
}

log_warn() {
    logger -t "$LOG_TAG" -p user.warning "$1"
    echo "[WARN] $1"
}

log_error() {
    logger -t "$LOG_TAG" -p user.err "$1"
    echo "[ERROR] $1" >&2
}

# Acquire exclusive lock to prevent concurrent cleanup runs
exec 200>"$LOCKFILE"
if ! flock -n 200; then
    log_info "Another cleanup instance is running, exiting"
    exit 0
fi

# Check if cleanup request file exists
if [[ ! -f "$CLEANUP_REQUEST" ]]; then
    log_info "No cleanup request found at $CLEANUP_REQUEST"
    exit 0
fi

log_info "Processing cleanup request from $CLEANUP_REQUEST"

# Read and parse the cleanup request
CLEANUP_DATA=$(cat "$CLEANUP_REQUEST")
HOST=$(echo "$CLEANUP_DATA" | grep -o '"host":"[^"]*"' | cut -d'"' -f4 || echo "")
REQUESTED_AT=$(echo "$CLEANUP_DATA" | grep -o '"requestedAt":"[^"]*"' | cut -d'"' -f4 || echo "")

log_info "Cleanup requested at: ${REQUESTED_AT:-unknown}"

# Rename request file to .processing to prevent re-triggering while allowing retry on failure
PROCESSING_FILE="${CLEANUP_REQUEST}.processing"
mv "$CLEANUP_REQUEST" "$PROCESSING_FILE" 2>/dev/null || {
    log_warn "Failed to rename cleanup request file, may have been processed by another instance"
    exit 0
}

# If no specific host was provided, clean up all known nodes
if [[ -z "$HOST" ]]; then
    log_info "No specific host provided - cleaning up all cluster nodes"

    # Discover cluster nodes
    if command -v pvecm >/dev/null 2>&1; then
        CLUSTER_NODES=$(pvecm status 2>/dev/null | grep -vEi "qdevice" | awk '/0x[0-9a-f]+.*[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/ {for(i=1;i<=NF;i++) if($i ~ /^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$/) print $i}' || true)

        if [[ -n "$CLUSTER_NODES" ]]; then
            for node_ip in $CLUSTER_NODES; do
                log_info "Cleaning up SSH keys on node $node_ip"

                # Remove both pulse-managed-key and pulse-proxy-key entries
                ssh -o StrictHostKeyChecking=no -o BatchMode=yes -o ConnectTimeout=5 root@"$node_ip" \
                    "sed -i -e '/# pulse-managed-key\$/d' -e '/# pulse-proxy-key\$/d' /root/.ssh/authorized_keys" 2>&1 | \
                    logger -t "$LOG_TAG" -p user.info || \
                    log_warn "Failed to clean up SSH keys on $node_ip"
            done
            log_info "Cluster cleanup completed"
        else
            # Standalone node - clean up localhost
            log_info "Standalone node detected - cleaning up localhost"
            sed -i -e '/# pulse-managed-key$/d' -e '/# pulse-proxy-key$/d' /root/.ssh/authorized_keys 2>&1 | \
                logger -t "$LOG_TAG" -p user.info || \
                log_warn "Failed to clean up SSH keys on localhost"
        fi
    else
        log_warn "pvecm command not available - cleaning up localhost only"
        sed -i -e '/# pulse-managed-key$/d' -e '/# pulse-proxy-key$/d' /root/.ssh/authorized_keys 2>&1 | \
            logger -t "$LOG_TAG" -p user.info || \
            log_warn "Failed to clean up SSH keys on localhost"
    fi
else
    log_info "Cleaning up specific host: $HOST"

    # Extract hostname/IP from host URL
    HOST_CLEAN=$(echo "$HOST" | sed -e 's|^https\?://||' -e 's|:.*$||')

    # Check if this is localhost (by IP, hostname, or FQDN)
    LOCAL_IPS=$(hostname -I 2>/dev/null || echo "")
    LOCAL_HOSTNAME=$(hostname 2>/dev/null || echo "")
    LOCAL_FQDN=$(hostname -f 2>/dev/null || echo "")
    IS_LOCAL=false

    # Check against all local IPs
    for local_ip in $LOCAL_IPS; do
        if [[ "$HOST_CLEAN" == "$local_ip" ]]; then
            IS_LOCAL=true
            break
        fi
    done

    # Check against hostname and FQDN
    if [[ "$HOST_CLEAN" == "127.0.0.1" || "$HOST_CLEAN" == "localhost" || \
          "$HOST_CLEAN" == "$LOCAL_HOSTNAME" || "$HOST_CLEAN" == "$LOCAL_FQDN" ]]; then
        IS_LOCAL=true
    fi

    if [[ "$IS_LOCAL" == true ]]; then
        log_info "Performing full cleanup on localhost"

        # 1. Remove SSH keys
        log_info "Removing SSH keys from authorized_keys"
        sed -i -e '/# pulse-managed-key$/d' -e '/# pulse-proxy-key$/d' /root/.ssh/authorized_keys 2>&1 | \
            logger -t "$LOG_TAG" -p user.info || \
            log_warn "Failed to clean up SSH keys"

        # 2. Delete API tokens and user
        log_info "Removing Proxmox API tokens and pulse-monitor user"
        if command -v pveum >/dev/null 2>&1; then
            # Try JSON output first (pveum with --output-format json)
            TOKEN_IDS=""
            if command -v python3 >/dev/null 2>&1; then
                # Try pveum with JSON output
                if TOKEN_JSON=$(pveum user token list pulse-monitor@pam --output-format json 2>/dev/null); then
                    TOKEN_IDS=$(echo "$TOKEN_JSON" | python3 -c '
import sys, json
try:
    data = json.load(sys.stdin)
    if isinstance(data, list):
        for item in data:
            if "tokenid" in item:
                print(item["tokenid"])
except: pass
' || true)
                fi
            fi

            # Fall back to pvesh JSON API if pveum JSON didn't work
            if [[ -z "$TOKEN_IDS" ]] && command -v pvesh >/dev/null 2>&1; then
                if TOKEN_JSON=$(pvesh get /access/users/pulse-monitor@pam/token 2>/dev/null); then
                    TOKEN_IDS=$(echo "$TOKEN_JSON" | python3 -c '
import sys, json
try:
    data = json.load(sys.stdin)
    if isinstance(data, dict) and "data" in data:
        for item in data["data"]:
            if "tokenid" in item:
                print(item["tokenid"])
except: pass
' 2>/dev/null || true)
                fi
            fi

            # Last resort: parse table output with better filtering
            if [[ -z "$TOKEN_IDS" ]]; then
                TOKEN_IDS=$(pveum user token list pulse-monitor@pam 2>/dev/null | \
                    awk 'NR>1 && /^[[:space:]]*pulse/ {print $1}' | grep -v '^[│┌└╞─]' | grep -v '^$' || true)
            fi

            if [[ -n "$TOKEN_IDS" ]]; then
                for token_id in $TOKEN_IDS; do
                    log_info "Deleting API token: $token_id"
                    pveum user token remove pulse-monitor@pam "${token_id}" 2>&1 | \
                        logger -t "$LOG_TAG" -p user.info || \
                        log_warn "Failed to delete token $token_id"
                done
            else
                log_info "No API tokens found for pulse-monitor@pam"
            fi

            # Remove the pulse-monitor user
            log_info "Removing pulse-monitor@pam user"
            pveum user delete pulse-monitor@pam 2>&1 | \
                logger -t "$LOG_TAG" -p user.info || \
                log_warn "pulse-monitor@pam user not found or already removed"
        else
            log_warn "pveum command not available, skipping API token cleanup"
        fi

        # 3. Remove LXC bind mounts
        log_info "Removing LXC bind mounts from container configs"
        if command -v pct >/dev/null 2>&1; then
            for ctid in $(pct list 2>/dev/null | awk 'NR>1 {print $1}' || true); do
                CONF_FILE="/etc/pve/lxc/${ctid}.conf"
                if [[ -f "$CONF_FILE" ]]; then
                    # Find pulse-sensor-proxy mount points and remove them using pct
                    for mp_key in $(grep -o "^mp[0-9]\+:" "$CONF_FILE" | grep -f <(grep "pulse-sensor-proxy" "$CONF_FILE" | grep -o "^mp[0-9]\+:") || true); do
                        mp_num="${mp_key%:}"
                        log_info "Removing ${mp_num} (pulse-sensor-proxy) from container $ctid"
                        if pct set "$ctid" -delete "${mp_num}" 2>&1 | logger -t "$LOG_TAG" -p user.info; then
                            log_info "Successfully removed ${mp_num} from container $ctid"
                        else
                            log_warn "Failed to remove ${mp_num} from container $ctid"
                        fi
                    done
                fi
            done
        fi

        # 4. Uninstall proxy service and remove binaries via isolated transient unit
        log_info "Starting full uninstallation (service, binaries, configs)"
        if [[ -x "$INSTALLER_PATH" ]]; then
            # Use systemd-run to create isolated transient unit that won't be killed
            # when we stop pulse-sensor-proxy.service
            if command -v systemd-run >/dev/null 2>&1; then
                # Use UUID for unique unit name (prevents same-second collisions)
                UNINSTALL_UUID=$(cat /proc/sys/kernel/random/uuid 2>/dev/null || date +%s%N)
                UNINSTALL_UNIT="pulse-uninstall-${UNINSTALL_UUID}"
                log_info "Spawning isolated uninstaller unit: $UNINSTALL_UNIT"

                systemd-run \
                    --unit="${UNINSTALL_UNIT}" \
                    --property="Type=oneshot" \
                    --property="Conflicts=pulse-sensor-proxy.service" \
                    --collect \
                    --wait \
                    --quiet \
                    -- bash -c "$INSTALLER_PATH --uninstall --purge --quiet >> /var/log/pulse/sensor-proxy/uninstall.log 2>&1" \
                    2>&1 | logger -t "$LOG_TAG" -p user.info

                UNINSTALL_EXIT=$?
                if [[ $UNINSTALL_EXIT -eq 0 ]]; then
                    log_info "Uninstaller completed successfully"
                else
                    log_error "Uninstaller failed with exit code $UNINSTALL_EXIT"
                    exit 1
                fi
            else
                log_warn "systemd-run not available, attempting direct uninstall (may fail)"
                bash "$INSTALLER_PATH" --uninstall --quiet >> /var/log/pulse/sensor-proxy/uninstall.log 2>&1 || \
                    log_error "Uninstaller failed - manual cleanup may be required"
            fi
        else
            log_warn "Installer not found at $INSTALLER_PATH, cannot run uninstaller"
            log_info "Manual cleanup required: systemctl stop pulse-sensor-proxy && systemctl disable pulse-sensor-proxy"
        fi

        log_info "Localhost cleanup initiated (uninstaller running in background)"
    else
        log_info "Cleaning up remote host: $HOST_CLEAN"

        # Try to use proxy's SSH key first (for standalone nodes), fall back to default
        PROXY_KEY="/var/lib/pulse-sensor-proxy/ssh/id_ed25519"
        SSH_CMD="ssh -o StrictHostKeyChecking=no -o BatchMode=yes -o ConnectTimeout=5"

        if [[ -f "$PROXY_KEY" ]]; then
            log_info "Using proxy SSH key for cleanup"
            SSH_CMD="$SSH_CMD -i $PROXY_KEY"
        fi

        # Remove both pulse-managed-key and pulse-proxy-key entries from remote host
        CLEANUP_OUTPUT=$($SSH_CMD root@"$HOST_CLEAN" \
            "sed -i -e '/# pulse-managed-key\$/d' -e '/# pulse-proxy-key\$/d' /root/.ssh/authorized_keys && echo 'SUCCESS'" 2>&1)

        if echo "$CLEANUP_OUTPUT" | grep -q "SUCCESS"; then
            log_info "Successfully cleaned up SSH keys on $HOST_CLEAN"
        else
            # Check if this is a standalone node with forced commands (common case)
            if echo "$CLEANUP_OUTPUT" | grep -q "cpu_thermal\|coretemp\|k10temp"; then
                log_warn "Cannot cleanup standalone node $HOST_CLEAN (forced command prevents cleanup)"
                log_info "Standalone node keys are read-only (sensors -j) - low security risk"
                log_info "Manual cleanup: ssh root@$HOST_CLEAN \"sed -i '/# pulse-proxy-key\$/d' /root/.ssh/authorized_keys\""
            else
                log_error "Failed to clean up SSH keys on $HOST_CLEAN: $CLEANUP_OUTPUT"
                exit 1
            fi
        fi
    fi
fi

# Remove processing file on success
rm -f "$PROCESSING_FILE"

log_info "Cleanup completed successfully"
exit 0
