#!/bin/bash

# pulse-sensor-cleanup.sh - Removes Pulse SSH keys from Proxmox nodes when they're removed from Pulse
# This script is triggered by systemd path unit when cleanup-request.json is created

set -euo pipefail

# Configuration
WORK_DIR="/var/lib/pulse-sensor-proxy"
CLEANUP_REQUEST="${WORK_DIR}/cleanup-request.json"
LOG_TAG="pulse-sensor-cleanup"

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

# Remove the cleanup request file immediately to prevent re-processing
rm -f "$CLEANUP_REQUEST"

# If no specific host was provided, clean up all known nodes
if [[ -z "$HOST" ]]; then
    log_info "No specific host provided - cleaning up all cluster nodes"

    # Discover cluster nodes
    if command -v pvecm >/dev/null 2>&1; then
        CLUSTER_NODES=$(pvecm status 2>/dev/null | awk '/0x[0-9a-f]+.*[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/ {print $3}')

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

    # Extract IP from host URL
    HOST_CLEAN=$(echo "$HOST" | sed -e 's|^https\?://||' -e 's|:.*$||')

    # Check if this is localhost
    LOCAL_IPS=$(hostname -I 2>/dev/null || echo "")
    IS_LOCAL=false

    for local_ip in $LOCAL_IPS; do
        if [[ "$HOST_CLEAN" == "$local_ip" ]]; then
            IS_LOCAL=true
            break
        fi
    done

    if [[ "$HOST_CLEAN" == "127.0.0.1" || "$HOST_CLEAN" == "localhost" ]]; then
        IS_LOCAL=true
    fi

    if [[ "$IS_LOCAL" == true ]]; then
        log_info "Cleaning up localhost SSH keys"
        sed -i -e '/# pulse-managed-key$/d' -e '/# pulse-proxy-key$/d' /root/.ssh/authorized_keys 2>&1 | \
            logger -t "$LOG_TAG" -p user.info || \
            log_warn "Failed to clean up SSH keys on localhost"
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

log_info "Cleanup completed successfully"
exit 0
