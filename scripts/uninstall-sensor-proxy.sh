#!/usr/bin/env bash
#
# Removes the legacy pulse-sensor-proxy footprint from a Proxmox host.
# Intended for users upgrading from older Pulse releases to the unified agent.

set -euo pipefail

BINARY_PATH="${PULSE_SENSOR_PROXY_BINARY_PATH:-/usr/local/bin/pulse-sensor-proxy}"
INSTALL_ROOT="${PULSE_SENSOR_PROXY_INSTALL_ROOT:-/opt/pulse/sensor-proxy}"
SERVICE_PATH="${PULSE_SENSOR_PROXY_SERVICE_PATH:-/etc/systemd/system/pulse-sensor-proxy.service}"
RUNTIME_DIR="${PULSE_SENSOR_PROXY_RUNTIME_DIR:-/run/pulse-sensor-proxy}"
SOCKET_PATH="${PULSE_SENSOR_PROXY_SOCKET_PATH:-${RUNTIME_DIR}/pulse-sensor-proxy.sock}"
WORK_DIR="${PULSE_SENSOR_PROXY_WORK_DIR:-/var/lib/pulse-sensor-proxy}"
CONFIG_DIR="${PULSE_SENSOR_PROXY_CONFIG_DIR:-/etc/pulse-sensor-proxy}"
LOG_DIR="${PULSE_SENSOR_PROXY_LOG_DIR:-/var/log/pulse/sensor-proxy}"
SERVICE_USER="${PULSE_SENSOR_PROXY_SERVICE_USER:-pulse-sensor-proxy}"
AUTHORIZED_KEYS_PATH="${PULSE_SENSOR_PROXY_AUTHORIZED_KEYS_PATH:-/root/.ssh/authorized_keys}"
SSH_KNOWN_HOSTS_PATH="${PULSE_SENSOR_PROXY_SSH_KNOWN_HOSTS_PATH:-}"

QUIET=false
PURGE=false
REMOVE_PROXMOX_ACCESS=false
LOCAL_ONLY=false

print_info() {
    if [[ "$QUIET" != "true" ]]; then
        printf '[INFO] %s\n' "$1"
    fi
}

print_warn() {
    printf '[WARN] %s\n' "$1" >&2
}

print_success() {
    if [[ "$QUIET" != "true" ]]; then
        printf '[ OK ] %s\n' "$1"
    fi
}

usage() {
    cat <<'EOF'
Usage: uninstall-sensor-proxy.sh [options]

Removes the legacy pulse-sensor-proxy footprint from a Proxmox host.

Options:
  --uninstall              Accepted for compatibility with old instructions.
  --purge                  Remove persisted proxy state, config, logs, and service user/group.
  --remove-proxmox-access  Remove pulse-monitor@pam API tokens/user after cleanup.
  --local-only             Clean only this host; run once on every cluster node.
  --ssh-known-hosts PATH   Use only this provisioned known_hosts file for remote cluster cleanup.
  --quiet                  Reduce informational output.
  --help                   Show this help text.

Remote cluster cleanup never enrolls unknown SSH host keys. Without
--ssh-known-hosts, OpenSSH's configured user and system known_hosts files are
used. Unknown or changed host keys fail the remote cleanup.
EOF
}

resolve_path() {
    local path="$1"
    local resolved=""

    if command -v readlink >/dev/null 2>&1; then
        resolved=$(readlink -f "$path" 2>/dev/null || true)
        if [[ -n "$resolved" ]]; then
            printf '%s\n' "$resolved"
            return 0
        fi
    fi

    if command -v python3 >/dev/null 2>&1; then
        python3 - "$path" <<'PY'
import os
import sys
print(os.path.realpath(sys.argv[1]))
PY
        return 0
    fi

    printf '%s\n' "$path"
}

systemctl_if_available() {
    if ! command -v systemctl >/dev/null 2>&1; then
        print_warn "systemctl is unavailable; legacy service cleanup is unconfirmed"
        return 1
    fi
    systemctl "$@" >/dev/null 2>&1
}

disable_legacy_units() {
    local unit=""
    local load_state=""
    local active_state=""
    local result=0
    local units=(
        pulse-sensor-proxy.service
        pulse-sensor-proxy-selfheal.timer
        pulse-sensor-proxy-selfheal.service
        pulse-sensor-cleanup.path
        pulse-sensor-cleanup.service
    )

    if ! command -v systemctl >/dev/null 2>&1; then
        print_warn "systemctl is unavailable; legacy service cleanup is unconfirmed"
        return 1
    fi
    for unit in "${units[@]}"; do
        if ! load_state=$(systemctl show "$unit" --property=LoadState --value 2>/dev/null); then
            print_warn "Cannot inspect legacy unit ${unit}; cleanup is incomplete"
            result=1
            continue
        fi
        [[ "$load_state" == "not-found" ]] && continue
        if [[ -z "$load_state" ]] || ! systemctl_if_available stop "$unit"; then
            print_warn "Cannot stop legacy unit ${unit}; cleanup is incomplete"
            result=1
            continue
        fi
        if ! active_state=$(systemctl show "$unit" --property=ActiveState --value 2>/dev/null) ||
            [[ "$active_state" != "inactive" && "$active_state" != "failed" ]]; then
            print_warn "Legacy unit ${unit} is not confirmed stopped; cleanup is incomplete"
            result=1
            continue
        fi
        if ! systemctl_if_available disable "$unit"; then
            print_warn "Cannot disable legacy unit ${unit}; cleanup is incomplete"
            result=1
        fi
    done
    systemctl_if_available daemon-reload || result=1
    return "$result"
}

remove_managed_keys_from_authorized_keys_file() (
    local file="$1"
    local tmp_file=""
    local status=0

    if [[ ! -e "$file" && ! -L "$file" ]]; then
        return 0
    fi
    [[ -f "$file" && ! -L "$file" ]] || return 1

    # Keep replacement on the same filesystem. A failed read, metadata copy or
    # rename must leave the original keys intact, even under conditional calls.
    # Isolate the descriptor and traps from the caller. Open separately: a
    # failed shell redirection returns 1 too, but is not grep's valid no-match.
    tmp_file=$(mktemp "${file}.pulse-cleanup.XXXXXX") || return 1
    trap 'status=$?; rm -f -- "$tmp_file" || status=1; exit "$status"' EXIT
    trap 'exit 1' HUP INT TERM
    exec 3>"$tmp_file" || return 1
    grep -v -E '# pulse-(managed|proxy)-key$' "$file" >&3 2>/dev/null || status=$?
    exec 3>&- || return 1
    (( status <= 1 )) || return 1
    status=0
    cmp -s "$tmp_file" "$file" || status=$?
    if (( status == 0 )); then
        return 0
    fi
    if (( status > 1 )) ||
        ! chmod --reference="$file" "$tmp_file" ||
        ! chown --reference="$file" "$tmp_file" ||
        ! mv -- "$tmp_file" "$file"; then
        return 1
    fi
)

cleanup_local_authorized_keys() {
    local auth_file="$AUTHORIZED_KEYS_PATH"
    # An absent file is an idempotent no-op, but a dangling/shared link or a
    # non-file target is not confirmed clean. Never replace an unresolved link.
    if [[ -e "$auth_file" || -L "$auth_file" ]]; then
        if ! auth_file=$(readlink -f -- "$auth_file") || [[ ! -f "$auth_file" || -L "$auth_file" ]]; then
            print_warn "Local legacy SSH key path is unresolved or not a regular file; cleanup is incomplete"
            return 1
        fi
    fi
    if ! remove_managed_keys_from_authorized_keys_file "$auth_file"; then
        print_warn "Local legacy SSH key cleanup is incomplete; check the key file and any retained staging locally"
        return 1
    fi
    print_success "Removed legacy Pulse SSH key entries from ${auth_file}"
}

is_local_host() {
    local target="$1"
    local local_ip=""
    local local_ips=""
    local local_hostnames=""

    local_ips="$(hostname -I 2>/dev/null || true)"
    for local_ip in $local_ips; do
        if [[ "$target" == "$local_ip" ]]; then
            return 0
        fi
    done

    local_hostnames="$(hostname 2>/dev/null || true) $(hostname -f 2>/dev/null || true)"
    case " ${local_hostnames} " in
        *" ${target} "*) return 0 ;;
    esac

    [[ "$target" == "127.0.0.1" || "$target" == "localhost" ]]
}

# The program is passed with -c so stdin remains the complete API response.
# Validate every row before emitting any identities; a partial list is unsafe.
sensor_proxy_inventory() {
    python3 -c '
import json, re, sys
field = sys.argv[1]
try:
    rows = json.load(sys.stdin)
    if not isinstance(rows, list) or (field == "node" and not rows):
        raise ValueError()
    values = []
    for row in rows:
        value = row.get(field) if isinstance(row, dict) else None
        if not isinstance(value, str) or not re.fullmatch(r"[A-Za-z0-9_][A-Za-z0-9_.-]{0,252}", value):
            raise ValueError()
        values.append(value)
except (ValueError, TypeError):
    sys.exit(1)
for value in values:
    print(value)
' "$1"
}

discover_cluster_node_addresses() {
    local nodes=""
    local node=""
    local node_ip=""
    local addresses=""

    if ! command -v pvesh >/dev/null 2>&1 || ! command -v python3 >/dev/null 2>&1; then
        print_warn "Cluster inventory requires pvesh and python3; use --local-only once on each node for local cleanup"
        return 1
    fi
    if ! nodes=$(pvesh get /nodes --output-format json 2>/dev/null | sensor_proxy_inventory node); then
        print_warn "Cluster inventory is unavailable or invalid; no SSH key cleanup attempted"
        return 1
    fi
    while IFS= read -r node; do
        node_ip=$(getent ahostsv4 "$node" 2>/dev/null | awk 'NR==1 {print $1}') || node_ip=""
        if [[ -z "$node_ip" ]]; then
            node_ip=$(getent hosts "$node" 2>/dev/null | awk 'NR==1 {print $1}') || node_ip=""
        fi
        if [[ -z "$node_ip" ]] || ! node_ip=$(python3 -c '
import ipaddress, sys
try:
    print(ipaddress.ip_address(sys.argv[1]))
except ValueError:
    sys.exit(1)
' "$node_ip"); then
            print_warn "A cluster node has no valid resolved address; no SSH key cleanup attempted"
            return 1
        fi
        addresses+="${node_ip}"$'\n'
    done <<<"$nodes"
    printf '%s' "$addresses"
}

cleanup_remote_authorized_keys() {
    local host="$1"
    local known_hosts_path=""
    local ssh_options=(
        -o StrictHostKeyChecking=yes
        -o UpdateHostKeys=no
        -o BatchMode=yes
        -o ConnectTimeout=5
    )
    local remote_cmd=""
    # Proxmox commonly links this file into its shared cluster filesystem.
    # Replace the resolved target, never the link, and preserve the original
    # keys if filtering, metadata preservation or replacement fails remotely.
    remote_cmd=$(cat <<'SH'
set -eu
auth="/root/.ssh/authorized_keys"
if [ -e "$auth" ] || [ -L "$auth" ]; then
    auth=$(readlink -f -- "$auth") || exit 1
    [ -f "$auth" ] || exit 1
    tmp_file=$(mktemp "${auth}.pulse-cleanup.XXXXXX") || exit 1
    trap 'status=$?; rm -f -- "$tmp_file" || status=1; exit "$status"' EXIT
    trap 'exit 1' HUP INT TERM
    exec 3>"$tmp_file" || exit 1
    status=0
    grep -v -E '# pulse-(managed|proxy)-key$' "$auth" >&3 || status=$?
    exec 3>&-
    [ "$status" -le 1 ] || exit 1
    status=0
    cmp -s "$tmp_file" "$auth" || status=$?
    case "$status" in
        0) ;;
        1)
            chmod --reference="$auth" "$tmp_file" &&
                chown --reference="$auth" "$tmp_file" &&
                mv -- "$tmp_file" "$auth" || exit 1
            ;;
        *) exit 1 ;;
    esac
fi
SH
    ) || return 1

    if [[ -n "$SSH_KNOWN_HOSTS_PATH" ]]; then
        known_hosts_path=$(resolve_path "$SSH_KNOWN_HOSTS_PATH")
        if [[ ! -f "$known_hosts_path" || ! -r "$known_hosts_path" || ! -s "$known_hosts_path" ]]; then
            print_warn "Provisioned SSH known_hosts file is missing, unreadable, or empty: ${known_hosts_path}"
            return 1
        fi
        ssh_options+=(
            -o "UserKnownHostsFile=${known_hosts_path}"
            -o GlobalKnownHostsFile=none
        )
    fi

    if ssh "${ssh_options[@]}" root@"$host" "$remote_cmd" >/dev/null 2>&1; then
        print_success "Removed legacy Pulse SSH key entries from ${host}"
        return 0
    fi
    print_warn "Unable to verify ${host}'s SSH host key or remove its legacy Pulse SSH key entries; provision trusted known_hosts data or clean up /root/.ssh/authorized_keys locally"
    return 1
}

cleanup_cluster_authorized_keys() {
    local host=""
    local hosts=""
    local status=0

    if [[ "$LOCAL_ONLY" == "true" ]]; then
        cleanup_local_authorized_keys
        return $?
    fi

    # Capture the discovery status, not a process substitution whose failure
    # would otherwise look like an empty, successful single-host installation.
    if ! hosts=$(discover_cluster_node_addresses) || [[ -z "$hosts" ]]; then
        return 1
    fi
    hosts=$(printf '%s\n' "$hosts" | sort -u) || return 1
    cleanup_local_authorized_keys || status=1
    while IFS= read -r host; do
        [[ -z "$host" ]] && continue
        if ! is_local_host "$host"; then
            cleanup_remote_authorized_keys "$host" || status=1
        fi
    done <<<"$hosts"
    return "$status"
}

# Only current (pre-snapshot) mount entries are eligible for cleanup.
sensor_proxy_mount_keys() {
    awk '
        /^\[/ { exit }
        /^mp[0-9]+:.*pulse-sensor-proxy/ { sub(/:.*/, ""); print }
        /^lxc\.mount\.entry:.*pulse-sensor-proxy/ { print "lxc.mount.entry" }
    ' "$1"
}

cleanup_sensor_proxy_lines_in_conf() {
    local conf="$1"
    local tmp_file=""

    tmp_file=$(mktemp) || return 1
    if ! awk '
        /^\[/ { snapshot = 1 }
        !snapshot && ($0 ~ /^mp[0-9]+:.*pulse-sensor-proxy/ || $0 ~ /^lxc\.mount\.entry:.*pulse-sensor-proxy/) { next }
        { print }
    ' "$conf" >"$tmp_file" ||
        ! chmod --reference="$conf" "$tmp_file" ||
        ! chown --reference="$conf" "$tmp_file" ||
        ! mv "$tmp_file" "$conf"; then
        rm -f "$tmp_file"
        return 1
    fi
}

sensor_proxy_container_status() {
    local status=""
    status=$(pct status "$1" 2>/dev/null) || return 1
    case "$status" in
        "status: running") printf 'running\n' ;;
        "status: stopped") printf 'stopped\n' ;;
        *) return 1 ;;
    esac
}

cleanup_sensor_proxy_container() {
    local ctid="$1"
    local conf="$2"
    local keys=""
    local key=""
    local status=""
    local was_running=false
    local result=0

    keys=$(sensor_proxy_mount_keys "$conf") || return 1
    [[ -z "$keys" ]] && return 0
    print_info "Cleaning stale pulse-sensor-proxy mount entries from container ${ctid}"
    if ! status=$(sensor_proxy_container_status "$ctid"); then
        print_warn "Container ${ctid} status is unknown; no configuration edits or restart attempted"
        return 1
    fi
    if [[ "$status" == "running" ]]; then
        was_running=true
        if ! timeout --kill-after=1s 30 pct stop "$ctid" >/dev/null 2>&1; then
            print_warn "Container ${ctid} stop failed; no configuration edits or restart attempted; check its state locally"
            return 1
        fi
    fi
    # A successful stop command is not a stopped-state observation. Recheck
    # before every mutation, including the direct legacy mount-line removal.
    if ! status=$(sensor_proxy_container_status "$ctid") || [[ "$status" != "stopped" ]]; then
        print_warn "Container ${ctid} is not confirmed stopped; no configuration edits or restart attempted"
        return 1
    fi
    while IFS= read -r key; do
        [[ "$key" == "lxc.mount.entry" ]] && continue
        if ! status=$(sensor_proxy_container_status "$ctid") || [[ "$status" != "stopped" ]]; then
            print_warn "Container ${ctid} stopped state was lost; no further edits or restart attempted"
            return 1
        fi
        if ! timeout --kill-after=1s 15 pct set "$ctid" -delete "$key" >/dev/null 2>&1; then
            print_warn "Container ${ctid} mount ${key} removal failed; no direct configuration fallback attempted"
            result=1
            break
        fi
    done <<<"$keys"
    if (( result == 0 )); then
        if ! status=$(sensor_proxy_container_status "$ctid") || [[ "$status" != "stopped" ]]; then
            print_warn "Container ${ctid} stopped state was lost; no further edits or restart attempted"
            return 1
        fi
        if ! cleanup_sensor_proxy_lines_in_conf "$conf"; then
            print_warn "Container ${ctid} configuration cleanup failed"
            result=1
        elif ! keys=$(sensor_proxy_mount_keys "$conf") || [[ -n "$keys" ]]; then
            print_warn "Container ${ctid} still has legacy mount entries; cleanup is incomplete"
            result=1
        fi
    fi
    if [[ "$was_running" == "true" ]]; then
        if ! status=$(sensor_proxy_container_status "$ctid") || [[ "$status" != "stopped" ]]; then
            print_warn "Container ${ctid} state is unconfirmed for restoration; no restart attempted"
            return 1
        fi
        if ! timeout --kill-after=1s 30 pct start "$ctid" >/dev/null 2>&1 ||
            ! status=$(sensor_proxy_container_status "$ctid") || [[ "$status" != "running" ]]; then
            print_warn "Container ${ctid} was running before cleanup but its running state was not restored; check it locally"
            result=1
        fi
    fi
    return "$result"
}

cleanup_stale_sensor_proxy_mounts() {
    local ctid=""
    local conf=""
    local containers=""
    local result=0

    if ! command -v pct >/dev/null 2>&1; then
        print_warn "pct is unavailable; LXC mount cleanup is unconfirmed"
        return 1
    fi
    if ! containers=$(pct list 2>/dev/null) || [[ "$containers" != VMID* ]]; then
        print_warn "Cannot list local containers; LXC mount cleanup is incomplete"
        return 1
    fi
    while IFS= read -r ctid; do
        [[ -z "$ctid" ]] && continue
        if [[ ! "$ctid" =~ ^[0-9]+$ ]]; then
            print_warn "Invalid container identity; LXC mount cleanup is incomplete"
            result=1
            continue
        fi
        conf="/etc/pve/lxc/${ctid}.conf"
        [[ -f "$conf" ]] || continue
        cleanup_sensor_proxy_container "$ctid" "$conf" || result=1
    done < <(printf '%s\n' "$containers" | tail -n +2 | awk '{print $1}')
    return "$result"
}

remove_path_if_present() {
    local path="$1"
    if [[ -e "$path" || -L "$path" ]]; then
        rm -rf "$path"
        print_success "Removed ${path}"
    fi
}

remove_legacy_files() {
    local path=""
    local files=(
        "$BINARY_PATH"
        /usr/local/bin/pulse-sensor-cleanup.sh
        "${INSTALL_ROOT}/bin/pulse-sensor-proxy"
        "${INSTALL_ROOT}/bin/pulse-sensor-cleanup.sh"
        "${INSTALL_ROOT}/bin/pulse-sensor-proxy-selfheal.sh"
        "${INSTALL_ROOT}/bin/pulse-sensor-wrapper.sh"
        "${INSTALL_ROOT}/install-sensor-proxy.sh"
        "$SERVICE_PATH"
        /etc/systemd/system/pulse-sensor-proxy-selfheal.service
        /etc/systemd/system/pulse-sensor-proxy-selfheal.timer
        /etc/systemd/system/pulse-sensor-cleanup.service
        /etc/systemd/system/pulse-sensor-cleanup.path
        "$SOCKET_PATH"
        "$RUNTIME_DIR"
    )

    for path in "${files[@]}"; do
        remove_path_if_present "$path"
    done

    if [[ "$PURGE" == "true" ]]; then
        remove_path_if_present "$INSTALL_ROOT"
        remove_path_if_present "$WORK_DIR"
        remove_path_if_present "$CONFIG_DIR"
        remove_path_if_present "$LOG_DIR"

        if id -u "$SERVICE_USER" >/dev/null 2>&1; then
            userdel --remove "$SERVICE_USER" >/dev/null 2>&1 || userdel "$SERVICE_USER" >/dev/null 2>&1 || print_warn "Failed to remove service user ${SERVICE_USER}"
        fi
        if getent group "$SERVICE_USER" >/dev/null 2>&1; then
            groupdel "$SERVICE_USER" >/dev/null 2>&1 || print_warn "Failed to remove service group ${SERVICE_USER}"
        fi
    else
        print_info "Preserving ${WORK_DIR}, ${CONFIG_DIR}, and ${LOG_DIR} (use --purge to remove them)"
    fi
}

remove_proxmox_access() {
    local token_ids=""
    local token_id=""
    local status=0

    if [[ "$REMOVE_PROXMOX_ACCESS" != "true" ]]; then
        return 0
    fi
    if ! command -v pveum >/dev/null 2>&1 || ! command -v python3 >/dev/null 2>&1; then
        print_warn "API access cleanup requires pveum and python3; cleanup is incomplete"
        return 1
    fi
    if ! token_ids=$(pveum user token list pulse-monitor@pam --output-format json 2>/dev/null | sensor_proxy_inventory tokenid); then
        print_warn "API token inventory is unavailable or invalid; API access was left unchanged"
        return 1
    fi
    while IFS= read -r token_id; do
        [[ -z "$token_id" ]] && continue
        if ! pveum user token remove pulse-monitor@pam "$token_id" >/dev/null 2>&1; then
            print_warn "A legacy API token could not be removed; API user deletion will not be attempted"
            status=1
        fi
    done <<<"$token_ids"
    (( status == 0 )) || return "$status"
    if ! pveum user delete pulse-monitor@pam >/dev/null 2>&1; then
        print_warn "Legacy API user removal is unconfirmed; cleanup is incomplete"
        return 1
    fi
}

main() {
    local cluster_key_cleanup_status=0
    local container_cleanup_status=0
    local api_cleanup_status=0

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --uninstall)
                shift
                ;;
            --purge)
                PURGE=true
                shift
                ;;
            --remove-proxmox-access)
                REMOVE_PROXMOX_ACCESS=true
                shift
                ;;
            --local-only)
                LOCAL_ONLY=true
                shift
                ;;
            --ssh-known-hosts)
                if [[ $# -lt 2 || -z "$2" ]]; then
                    printf '%s\n' '--ssh-known-hosts requires a non-empty path' >&2
                    usage >&2
                    exit 1
                fi
                SSH_KNOWN_HOSTS_PATH="$2"
                shift 2
                ;;
            --ssh-known-hosts=*)
                SSH_KNOWN_HOSTS_PATH="${1#*=}"
                if [[ -z "$SSH_KNOWN_HOSTS_PATH" ]]; then
                    printf '%s\n' '--ssh-known-hosts requires a non-empty path' >&2
                    usage >&2
                    exit 1
                fi
                shift
                ;;
            --quiet)
                QUIET=true
                shift
                ;;
            --help|-h)
                usage
                exit 0
                ;;
            *)
                printf 'Unknown option: %s\n' "$1" >&2
                usage >&2
                exit 1
                ;;
        esac
    done

    print_info "Starting legacy pulse-sensor-proxy cleanup"
    if ! disable_legacy_units; then
        print_warn "Legacy service cleanup is incomplete; leaving keys, containers, files and API access unchanged"
        return 1
    fi
    cleanup_cluster_authorized_keys || cluster_key_cleanup_status=$?
    cleanup_stale_sensor_proxy_mounts || container_cleanup_status=$?
    if (( container_cleanup_status != 0 )); then
        print_warn "LXC cleanup is incomplete; leaving legacy files and API access unchanged; check affected containers locally"
        return "$container_cleanup_status"
    fi
    remove_legacy_files
    remove_proxmox_access || api_cleanup_status=$?
    if ! systemctl_if_available daemon-reload; then
        print_warn "Cannot reload service configuration; cleanup is incomplete"
        return 1
    fi
    if (( cluster_key_cleanup_status != 0 )); then
        print_warn "Local file cleanup completed, but cluster SSH key cleanup is incomplete; use --local-only on each node or repair trusted cluster inventory"
        return "$cluster_key_cleanup_status"
    fi
    (( api_cleanup_status == 0 )) || return "$api_cleanup_status"
    print_success "Legacy pulse-sensor-proxy cleanup complete"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi
