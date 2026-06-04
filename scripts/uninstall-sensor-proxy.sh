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

QUIET=false
PURGE=false
REMOVE_PROXMOX_ACCESS=false

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
  --quiet                  Reduce informational output.
  --help                   Show this help text.
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
        return 0
    fi
    systemctl "$@" >/dev/null 2>&1 || true
}

disable_legacy_units() {
    local unit=""
    local units=(
        pulse-sensor-proxy.service
        pulse-sensor-proxy-selfheal.timer
        pulse-sensor-proxy-selfheal.service
        pulse-sensor-cleanup.path
        pulse-sensor-cleanup.service
    )

    for unit in "${units[@]}"; do
        systemctl_if_available stop "$unit"
        systemctl_if_available disable "$unit"
    done
    systemctl_if_available daemon-reload
}

remove_managed_keys_from_authorized_keys_file() {
    local file="$1"
    local tmp_file=""

    if [[ ! -f "$file" ]]; then
        return 0
    fi

    tmp_file=$(mktemp)
    grep -v -E '# pulse-(managed|proxy)-key$' "$file" >"$tmp_file" 2>/dev/null || true
    if cmp -s "$tmp_file" "$file"; then
        rm -f "$tmp_file"
        return 0
    fi

    chmod --reference="$file" "$tmp_file" 2>/dev/null || chmod 600 "$tmp_file" 2>/dev/null || true
    chown --reference="$file" "$tmp_file" 2>/dev/null || true
    mv "$tmp_file" "$file"
}

cleanup_local_authorized_keys() {
    local auth_file=""
    auth_file=$(resolve_path "$AUTHORIZED_KEYS_PATH")
    remove_managed_keys_from_authorized_keys_file "$auth_file"
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

discover_cluster_node_addresses() {
    local node=""
    local node_ip=""

    if command -v pvesh >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
        while IFS= read -r node; do
            [[ -z "$node" ]] && continue
            node_ip=$(getent ahostsv4 "$node" 2>/dev/null | awk 'NR==1 {print $1}') || true
            if [[ -z "$node_ip" ]]; then
                node_ip=$(getent hosts "$node" 2>/dev/null | awk 'NR==1 {print $1}') || true
            fi
            [[ -n "$node_ip" ]] && printf '%s\n' "$node_ip"
        done < <(
            pvesh get /nodes --output-format json 2>/dev/null | python3 - <<'PY'
import json
import sys
try:
    data = json.load(sys.stdin)
except Exception:
    sys.exit(0)
if isinstance(data, list):
    for item in data:
        node = item.get("node")
        if node:
            print(node)
PY
        )
        return 0
    fi

    if command -v pvecm >/dev/null 2>&1; then
        pvecm status 2>/dev/null | awk '
            /0x[0-9a-f]+/ {
                for (i = 1; i <= NF; i++) {
                    if ($i ~ /^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$/) print $i
                }
            }
        '
    fi
}

cleanup_remote_authorized_keys() {
    local host="$1"
    local remote_cmd='set -eu
auth="/root/.ssh/authorized_keys"
if [ -f "$auth" ]; then
    sed -i -e "/# pulse-managed-key$/d" -e "/# pulse-proxy-key$/d" "$auth"
fi'

    if ssh -o StrictHostKeyChecking=no -o BatchMode=yes -o ConnectTimeout=5 root@"$host" "$remote_cmd" >/dev/null 2>&1; then
        print_success "Removed legacy Pulse SSH key entries from ${host}"
        return 0
    fi
    print_warn "Unable to remove legacy Pulse SSH key entries from ${host}; clean up /root/.ssh/authorized_keys manually if needed"
    return 1
}

cleanup_cluster_authorized_keys() {
    local host=""
    local saw_nodes=false
    local status=0

    while IFS= read -r host; do
        [[ -z "$host" ]] && continue
        saw_nodes=true
        if is_local_host "$host"; then
            cleanup_local_authorized_keys
        else
            cleanup_remote_authorized_keys "$host" || status=1
        fi
    done < <(discover_cluster_node_addresses | sort -u)

    if [[ "$saw_nodes" != "true" ]]; then
        cleanup_local_authorized_keys
    fi
    return "$status"
}

main_section_snapshot_line() {
    local conf="$1"
    grep -n '^\[' "$conf" 2>/dev/null | head -1 | cut -d: -f1
}

cleanup_sensor_proxy_lines_in_conf() {
    local conf="$1"
    local snapshot_line="${2:-}"
    local tmp_file=""

    tmp_file=$(mktemp)
    if [[ -n "$snapshot_line" && "$snapshot_line" -gt 1 ]]; then
        awk -v snapshot_line="$snapshot_line" '
            NR < snapshot_line && ($0 ~ /^mp[0-9]+:.*pulse-sensor-proxy/ || $0 ~ /^lxc\.mount\.entry:.*pulse-sensor-proxy/) { next }
            { print }
        ' "$conf" >"$tmp_file"
    else
        awk '
            $0 ~ /^mp[0-9]+:.*pulse-sensor-proxy/ { next }
            $0 ~ /^lxc\.mount\.entry:.*pulse-sensor-proxy/ { next }
            { print }
        ' "$conf" >"$tmp_file"
    fi

    chmod --reference="$conf" "$tmp_file" 2>/dev/null || true
    chown --reference="$conf" "$tmp_file" 2>/dev/null || true
    mv "$tmp_file" "$conf"
}

cleanup_stale_sensor_proxy_mounts() {
    local ctid=""
    local conf=""
    local snapshot_line=""
    local status=""
    local was_running=false
    local mp_keys=""
    local mp_key=""
    local cleaned=0

    if ! command -v pct >/dev/null 2>&1; then
        return 0
    fi

    while IFS= read -r ctid; do
        [[ -z "$ctid" ]] && continue
        conf="/etc/pve/lxc/${ctid}.conf"
        [[ -f "$conf" ]] || continue
        grep -q 'pulse-sensor-proxy' "$conf" 2>/dev/null || continue

        print_info "Cleaning stale pulse-sensor-proxy mount entries from container ${ctid}"
        status=$(pct status "$ctid" 2>/dev/null | awk '{print $2}') || true
        was_running=false
        [[ "$status" == "running" ]] && was_running=true

        if [[ "$was_running" == "true" ]]; then
            timeout 30 pct stop "$ctid" >/dev/null 2>&1 || true
            sleep 2
        fi

        snapshot_line=$(main_section_snapshot_line "$conf")
        if [[ -n "$snapshot_line" && "$snapshot_line" -gt 1 ]]; then
            mp_keys=$(head -n "$((snapshot_line - 1))" "$conf" 2>/dev/null | grep -E '^mp[0-9]+:.*pulse-sensor-proxy' | sed 's/:.*//') || true
        else
            mp_keys=$(grep -E '^mp[0-9]+:.*pulse-sensor-proxy' "$conf" 2>/dev/null | sed 's/:.*//') || true
        fi

        while IFS= read -r mp_key; do
            [[ -z "$mp_key" ]] && continue
            timeout 15 pct set "$ctid" -delete "$mp_key" >/dev/null 2>&1 || true
            cleaned=$((cleaned + 1))
        done <<<"$mp_keys"

        cleanup_sensor_proxy_lines_in_conf "$conf" "$snapshot_line"

        if [[ "$was_running" == "true" ]]; then
            timeout 30 pct start "$ctid" >/dev/null 2>&1 || print_warn "Container ${ctid} was running before cleanup but could not be restarted automatically"
        fi
    done < <(pct list 2>/dev/null | tail -n +2 | awk '{print $1}')

    if (( cleaned > 0 )); then
        print_success "Removed legacy pulse-sensor-proxy mount entries from Proxmox LXC config(s)"
    fi
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

    if [[ "$REMOVE_PROXMOX_ACCESS" != "true" ]]; then
        return 0
    fi
    if ! command -v pveum >/dev/null 2>&1; then
        print_warn "pveum is not available; skipping pulse-monitor@pam cleanup"
        return 0
    fi

    if command -v python3 >/dev/null 2>&1; then
        token_ids=$(pveum user token list pulse-monitor@pam --output-format json 2>/dev/null | python3 - <<'PY'
import json
import sys
try:
    data = json.load(sys.stdin)
except Exception:
    sys.exit(0)
if isinstance(data, list):
    for item in data:
        token_id = item.get("tokenid")
        if token_id:
            print(token_id)
PY
        ) || true
    fi

    if [[ -z "$token_ids" ]]; then
        token_ids=$(pveum user token list pulse-monitor@pam 2>/dev/null | awk '
            {
                line=$0
                gsub(/\r/, "", line)
                gsub(/│/, "|", line)
                if (index(line, "|") == 0) next
                n = split(line, parts, "|")
                if (n < 3) next
                name = parts[2]
                gsub(/^[[:space:]]+|[[:space:]]+$/, "", name)
                if (name != "" && name != "tokenid") print name
            }') || true
    fi

    while IFS= read -r token_id; do
        [[ -z "$token_id" ]] && continue
        pveum user token remove pulse-monitor@pam "$token_id" >/dev/null 2>&1 || print_warn "Failed to remove pulse-monitor@pam token ${token_id}"
    done <<<"$token_ids"

    pveum user delete pulse-monitor@pam >/dev/null 2>&1 || print_warn "pulse-monitor@pam was not removed; it may already be absent or still in use"
}

main() {
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
    disable_legacy_units
    cleanup_cluster_authorized_keys || true
    cleanup_stale_sensor_proxy_mounts
    remove_legacy_files
    remove_proxmox_access
    systemctl_if_available daemon-reload
    print_success "Legacy pulse-sensor-proxy cleanup complete"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi
