#!/usr/bin/env bash

# Passive Proxmox preflight only. A lock read is not a synchronisation gate:
# a backup can start immediately afterwards. Never send guest-agent commands.
set -euo pipefail

usage() {
    printf '%s\n' 'Usage: test-vm-disk.sh VMID' \
        'Read local VM status and current configuration on its Proxmox host.' \
        'No guest-agent, guest filesystem, ACL or service changes are made.'
}

if [[ $# -eq 1 && $1 == --help ]]; then
    usage
    exit 0
fi
if [[ $# -ne 1 || ! $1 =~ ^[1-9][0-9]{0,8}$ ]]; then
    usage >&2
    exit 2
fi
vmid=$1

for command in qm timeout; do
    if ! command -v "$command" >/dev/null 2>&1; then
        printf 'Required command unavailable: %s. Run on the owning Proxmox host.\n' "$command" >&2
        exit 1
    fi
done

read_failure() {
    local operation=$1 code=$2
    if [[ $code -eq 124 || $code -eq 137 ]]; then
        printf 'VM %s read timed out; no guest-agent commands were sent.\n' "$operation" >&2
    else
        printf 'VM %s read failed (exit %s); no guest-agent commands were sent.\n' "$operation" "$code" >&2
    fi
    printf '%s\n' 'Check the VM ID, owning host and local read permissions. Inspect any error privately.' >&2
    exit 1
}

printf '%s\n' 'Passive VM preflight: no guest-agent commands will be sent.'
# Bound even host-local reads. Do not print raw config or command errors:
# those can contain private guest names, paths and other infrastructure data.
if status_output=$(timeout --signal=TERM --kill-after=2s 10s qm status "$vmid" 2>/dev/null); then
    case "$status_output" in
        'status: running') status=running ;;
        'status: stopped') status=stopped ;;
        *) printf '%s\n' 'VM status response was not understood; preflight incomplete.' >&2; exit 1 ;;
    esac
else
    read_failure status "$?"
fi
printf 'VM status: %s\n' "$status"

if config=$(timeout --signal=TERM --kill-after=2s 10s qm config "$vmid" --current 2>/dev/null); then
    if [[ -z $config ]]; then
        printf '%s\n' 'VM current configuration was empty; preflight incomplete.' >&2
        exit 1
    fi
else
    read_failure configuration "$?"
fi

agent_seen=false
lock_seen=false
field_seen=false
agent_value=''
lock_value=''
while IFS= read -r line; do
    if [[ $line =~ ^[[:space:]]*$ || $line =~ ^[[:space:]]*# ]]; then
        continue
    fi
    # qm's fields are unindented lowercase keys. Do not let one valid field
    # turn malformed/truncated lock or agent lines into an absent setting.
    # Unknown well-formed fields remain valid; never echo an unparsed line.
    if [[ ! $line =~ ^[a-z][a-z0-9_-]*: ]]; then
        printf '%s\n' 'VM current configuration contains an unrecognised line; preflight incomplete.' >&2
        exit 1
    fi
    field_seen=true
    case "$line" in
        agent:*)
            if [[ $agent_seen == true ]]; then
                printf '%s\n' 'Repeated agent configuration; preflight incomplete.' >&2
                exit 1
            fi
            agent_seen=true
            agent_value=${line#agent:}
            agent_value=${agent_value//[[:space:]]/}
            ;;
        lock:*)
            if [[ $lock_seen == true ]]; then
                printf '%s\n' 'Repeated VM lock configuration; preflight incomplete.' >&2
                exit 1
            fi
            lock_seen=true
            lock_value=${line#lock:}
            lock_value=${lock_value//[[:space:]]/}
            ;;
    esac
done <<< "$config"
if [[ $field_seen != true ]]; then
    printf '%s\n' 'VM current configuration was not understood; preflight incomplete.' >&2
    exit 1
fi

agent=disabled
if [[ $agent_seen == true ]]; then
    agent=unknown
    enabled_options=0
    IFS=, read -r -a options <<< "$agent_value"
    for option in "${options[@]}"; do
        case "$option" in
            1|enabled=1) agent=enabled; enabled_options=$((enabled_options + 1)) ;;
            0|enabled=0) agent=disabled; enabled_options=$((enabled_options + 1)) ;;
        esac
    done
    if [[ $enabled_options -ne 1 ]]; then
        agent=unknown
    fi
fi
printf 'Guest agent configured: %s (not a responsiveness test).\n' "$agent"
if [[ $lock_seen == true ]]; then
    if [[ $lock_value == backup ]]; then
        printf '%s\n' 'VM lock: backup. Defer guest-agent diagnostics and configuration changes.'
    else
        printf '%s\n' 'VM lock: present or unknown. Check the existing Proxmox task before making changes.'
    fi
else
    printf '%s\n' 'VM lock: not reported. This does not establish that a backup is idle or the guest is thawed.'
fi
printf '%s\n' 'Read-only preflight completed. Disk freshness, guest-agent responsiveness and guest thaw were not tested.'
