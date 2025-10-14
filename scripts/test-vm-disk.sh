#!/usr/bin/env bash

set -uo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info() {
    printf "%b[INFO]%b %s\n" "$BLUE" "$NC" "$1"
}

ok() {
    printf "    %b✓%b %s\n" "$GREEN" "$NC" "$1"
}

warn() {
    printf "    %b⚠%b %s\n" "$YELLOW" "$NC" "$1"
}

fail() {
    printf "    %b✗%b %s\n" "$RED" "$NC" "$1"
}

require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        printf "%b[ERROR]%b Required command '%s' not found.\n" "$RED" "$NC" "$1" >&2
        exit 1
    fi
}

# Ensure we are running on a Proxmox host
require_cmd qm
require_cmd pveversion

if [[ $EUID -ne 0 ]]; then
    printf "%b[ERROR]%b This script must be run as root on the Proxmox host.\n" "$RED" "$NC" >&2
    exit 1
fi

VMID=${1:-}
if [[ -z "${VMID}" ]]; then
    read -rp "Enter the VMID to inspect: " VMID
fi

if [[ ! $VMID =~ ^[0-9]+$ ]]; then
    printf "%b[ERROR]%b VMID must be a numeric value.\n" "$RED" "$NC" >&2
    exit 1
fi

info "Running disk diagnostics for VMID ${VMID}"

# --- Check VM existence and status ---
status_output=$(qm status "$VMID" 2>&1)
if [[ $? -ne 0 ]]; then
    printf "%b[ERROR]%b Unable to query VM %s.\n" "$RED" "$NC" "$VMID" >&2
    printf "%s\n" "$status_output" >&2
    exit 1
fi

status=$(awk -F': ' '/status/ {print $2}' <<<"$status_output")
status=${status:-unknown}
ok "VM found (status: ${status})"

running=false
if [[ $status == "running" ]]; then
    running=true
else
    warn "VM is not running. Start the VM before re-running disk checks for accurate results."
fi

# --- Check guest agent configuration ---
agent_line=$(qm config "$VMID" | grep '^agent:' || true)
agent_enabled=false
if [[ -n $agent_line ]]; then
    if grep -Eq '1|enabled' <<<"$agent_line"; then
        agent_enabled=true
        ok "QEMU guest agent is enabled in VM configuration (${agent_line})."
    else
        warn "QEMU guest agent is defined but not enabled (current config: ${agent_line})."
    fi
else
    warn "QEMU guest agent is not enabled for this VM. Add 'agent: 1' in the VM options."
fi

if [[ $agent_enabled == true && $running == true ]]; then
    # Guest agent ping
    ping_output=$(qm agent "$VMID" ping 2>&1)
    if [[ $? -eq 0 ]]; then
        ok "Guest agent responded to ping."
    else
        warn "Guest agent did not respond to ping."
        printf "      %s\n" "$ping_output"
    fi

    # Filesystem info
    fs_output=$(qm agent "$VMID" get-fsinfo 2>&1)
    if [[ $? -eq 0 ]]; then
        if grep -Eq "(mount|fsname|filesystem)" <<<"$fs_output"; then
            ok "Fetched filesystem information from guest."
        else
            warn "Guest agent returned no filesystem entries. Review the guest OS permissions."
        fi
    else
        warn "Failed to retrieve filesystem information via guest agent."
        printf "      %s\n" "$fs_output"
    fi
else
    warn "Skipping guest agent checks because the VM is not running or the agent is disabled."
fi

# --- Validate pulse monitoring account (optional) ---
if pveum user list 2>/dev/null | grep -q 'pulse_monitor@pam'; then
    acl_output=$(pveum acl list / -user pulse_monitor@pam 2>&1)
    if [[ $? -eq 0 && -n $acl_output ]]; then
        ok "pulse_monitor@pam user has ACL entries configured."
    else
        warn "pulse_monitor@pam user exists but no ACLs were found. Ensure it has proper permissions."
    fi
else
    warn "pulse_monitor@pam user not found. Create it if Pulse is expected to collect agent data."
fi

cat <<'SUMMARY'

Next steps:
  • If the guest agent is disabled, enable it in the VM Options tab (set "QEMU Guest Agent" to "Enabled").
  • Inside the guest OS, ensure the qemu-guest-agent service is installed, running, and has access to disk information.
  • Verify the pulse_monitor@pam user (or your service account) has proper permissions:
    - Proxmox 9: VM.GuestAgent.Audit privilege (Pulse setup adds via PulseMonitor role)
    - Proxmox 8: VM.Monitor privilege (Pulse setup adds via PulseMonitor role)
    - Sys.Audit is recommended for Ceph metrics and included when available
    - Both API tokens and passwords work fine for guest agent access

Diagnostics complete.
SUMMARY
