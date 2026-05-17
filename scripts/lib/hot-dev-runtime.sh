#!/usr/bin/env bash

# Shared helpers for the managed local Pulse development runtime.

hot_dev_pulse_process_count() {
    local pattern="${1:-^\./pulse$}"
    local count=0
    local pid

    if ! command -v pgrep >/dev/null 2>&1; then
        printf '0\n'
        return 0
    fi

    while IFS= read -r pid; do
        [[ -n "${pid}" ]] || continue
        count=$((count + 1))
    done < <(pgrep -f "${pattern}" 2>/dev/null || true)

    printf '%s\n' "${count}"
}

hot_dev_normalize_host() {
    local host="${1:-}"

    host="${host#[}"
    host="${host%]}"
    printf '%s\n' "${host}" | tr '[:upper:]' '[:lower:]'
}

hot_dev_url_host() {
    local url="${1:-}"
    local host_port host

    [[ -n "${url}" ]] || return 0

    host_port="${url}"
    if [[ "${host_port}" == *"://"* ]]; then
        host_port="${host_port#*://}"
    fi
    host_port="${host_port%%/*}"
    host_port="${host_port%%\?*}"
    host_port="${host_port%%#*}"

    if [[ "${host_port}" == \[*\]* ]]; then
        host="${host_port#\[}"
        host="${host%%\]*}"
    else
        host="${host_port%%:*}"
    fi

    hot_dev_normalize_host "${host}"
}

hot_dev_host_is_loopback() {
    local host
    host="$(hot_dev_normalize_host "${1:-}")"

    [[ -n "${host}" ]] || return 1

    case "${host}" in
        localhost|127.*|::1|0:0:0:0:0:0:0:1)
            return 0
            ;;
    esac

    return 1
}

hot_dev_bind_address_is_loopback() {
    hot_dev_host_is_loopback "${1:-}"
}

hot_dev_host_matches_local_interface() {
    local host="${1:-}"
    local candidate
    local system_hostname=""
    local short_hostname=""

    host="$(hot_dev_normalize_host "${host}")"
    [[ -n "${host}" ]] || return 1

    if [[ -n "${LAN_IP:-}" ]] && [[ "${host}" == "$(hot_dev_normalize_host "${LAN_IP}")" ]]; then
        return 0
    fi

    for candidate in ${ALL_IPS:-}; do
        if [[ "${host}" == "$(hot_dev_normalize_host "${candidate}")" ]]; then
            return 0
        fi
    done

    system_hostname="$(hostname 2>/dev/null || true)"
    system_hostname="$(hot_dev_normalize_host "${system_hostname}")"
    short_hostname="${system_hostname%%.*}"

    if [[ -n "${system_hostname}" && "${host}" == "${system_hostname}" ]]; then
        return 0
    fi
    if [[ -n "${short_hostname}" && "${host}" == "${short_hostname}" ]]; then
        return 0
    fi

    return 1
}

hot_dev_reconcile_agent_bind_address() {
    local bind_address="${BIND_ADDRESS:-}"
    local labels="PULSE_AGENT_CONNECT_URL PULSE_AGENT_URL PULSE_PUBLIC_URL"
    local label url host

    [[ -n "${bind_address}" ]] || return 0
    hot_dev_bind_address_is_loopback "${bind_address}" || return 0

    for label in ${labels}; do
        url="${!label-}"
        host="$(hot_dev_url_host "${url}")"
        [[ -n "${host}" ]] || continue
        hot_dev_host_is_loopback "${host}" && continue
        hot_dev_host_matches_local_interface "${host}" || continue

        BIND_ADDRESS="0.0.0.0"
        export BIND_ADDRESS

        if declare -F log_warn >/dev/null 2>&1; then
            log_warn "${label}=${url} points at this machine, but BIND_ADDRESS=${bind_address} only accepts local connections. Exposing the dev backend on 0.0.0.0 so installed agents can report metrics."
        fi
        return 0
    done
}
