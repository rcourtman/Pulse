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

hot_dev_detect_lan_ip() {
    local detected=""

    if command -v hostname >/dev/null 2>&1 && hostname -I >/dev/null 2>&1; then
        detected="$(hostname -I 2>/dev/null | awk '{print $1}')"
    fi

    if [[ -z "${detected}" ]] && command -v ipconfig >/dev/null 2>&1; then
        detected="$(ipconfig getifaddr en0 2>/dev/null || true)"
    fi

    printf '%s\n' "${detected}"
}

hot_dev_detect_all_ipv4s() {
    if [[ "$(uname -s)" == "Darwin" ]]; then
        ifconfig 2>/dev/null | awk '/inet / {print $2}' | grep -v "^127\." || true
        return 0
    fi

    hostname -I 2>/dev/null || true
}

hot_dev_truthy() {
    local value
    value="$(printf '%s\n' "${1:-}" | tr '[:upper:]' '[:lower:]')"

    case "${value}" in
        1|true|yes|y|on)
            return 0
            ;;
    esac

    return 1
}

hot_dev_lan_enabled() {
    hot_dev_truthy "${PULSE_DEV_LAN:-false}"
}

hot_dev_configure_network_defaults() {
    FRONTEND_PORT="${FRONTEND_PORT:-${PORT:-5173}}"
    PORT="${PORT:-${FRONTEND_PORT}}"

    if [[ -z "${LAN_IP:-}" ]]; then
        LAN_IP="$(hot_dev_detect_lan_ip)"
    fi
    LAN_IP="${LAN_IP:-0.0.0.0}"

    if hot_dev_lan_enabled; then
        FRONTEND_DEV_HOST="${FRONTEND_DEV_HOST:-0.0.0.0}"
        PULSE_DEV_API_HOST="${PULSE_DEV_API_HOST:-${LAN_IP}}"
        BIND_ADDRESS="${BIND_ADDRESS:-0.0.0.0}"
    else
        FRONTEND_DEV_HOST="${FRONTEND_DEV_HOST:-127.0.0.1}"
        PULSE_DEV_API_HOST="${PULSE_DEV_API_HOST:-127.0.0.1}"
        BIND_ADDRESS="${BIND_ADDRESS:-127.0.0.1}"
    fi
    FRONTEND_DEV_PORT="${FRONTEND_DEV_PORT:-${FRONTEND_PORT}}"
    PULSE_DEV_API_PORT="${PULSE_DEV_API_PORT:-7655}"

    if [[ -z "${PULSE_DEV_API_URL:-}" ]]; then
        PULSE_DEV_API_URL="http://${PULSE_DEV_API_HOST}:${PULSE_DEV_API_PORT}"
    fi

    if [[ -z "${PULSE_DEV_WS_URL:-}" ]]; then
        if [[ "${PULSE_DEV_API_URL}" == http://* ]]; then
            PULSE_DEV_WS_URL="ws://${PULSE_DEV_API_URL#http://}"
        elif [[ "${PULSE_DEV_API_URL}" == https://* ]]; then
            PULSE_DEV_WS_URL="wss://${PULSE_DEV_API_URL#https://}"
        else
            PULSE_DEV_WS_URL="${PULSE_DEV_API_URL}"
        fi
    fi

    ALL_IPS="${ALL_IPS:-$(hot_dev_detect_all_ipv4s)}"

    ALLOWED_ORIGINS="http://${PULSE_DEV_API_HOST:-127.0.0.1}:${FRONTEND_DEV_PORT:-7655}"
    ALLOWED_ORIGINS="${ALLOWED_ORIGINS},http://localhost:${FRONTEND_DEV_PORT:-7655},http://127.0.0.1:${FRONTEND_DEV_PORT:-7655}"
    ALLOWED_ORIGINS="${ALLOWED_ORIGINS},http://localhost:5173,http://127.0.0.1:5173"
    if [[ "${FRONTEND_DEV_HOST:-}" == "0.0.0.0" ]]; then
        ALLOWED_ORIGINS="${ALLOWED_ORIGINS},http://0.0.0.0:${FRONTEND_DEV_PORT:-7655},http://0.0.0.0:5173"
    fi

    if hot_dev_lan_enabled; then
        local ip
        for ip in ${ALL_IPS:-}; do
            if [[ "${ip}" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
                if [[ "${ip}" != "127.0.0.1" ]]; then
                    ALLOWED_ORIGINS="${ALLOWED_ORIGINS},http://${ip}:${FRONTEND_DEV_PORT:-7655}"
                    ALLOWED_ORIGINS="${ALLOWED_ORIGINS},http://${ip}:5173"
                fi
            fi
        done
    fi

    export FRONTEND_PORT PORT
    export LAN_IP ALL_IPS
    export FRONTEND_DEV_HOST FRONTEND_DEV_PORT
    export PULSE_DEV_API_HOST PULSE_DEV_API_PORT PULSE_DEV_API_URL PULSE_DEV_WS_URL
    export BIND_ADDRESS ALLOWED_ORIGINS
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

hot_dev_browser_host_for_bind() {
    local host="${1:-}"
    local normalized

    normalized="$(hot_dev_normalize_host "${host}")"
    case "${normalized}" in
        ""|0.0.0.0|::)
            printf '127.0.0.1\n'
            return 0
            ;;
    esac

    printf '%s\n' "${host}"
}

hot_dev_local_browser_url() {
    local host="${1:-${FRONTEND_DEV_HOST:-127.0.0.1}}"
    local port="${2:-${FRONTEND_DEV_PORT:-5173}}"

    printf 'http://%s:%s\n' "$(hot_dev_browser_host_for_bind "${host}")" "${port}"
}

hot_dev_lan_browser_url() {
    local bind_host="${1:-${FRONTEND_DEV_HOST:-127.0.0.1}}"
    local port="${2:-${FRONTEND_DEV_PORT:-5173}}"
    local lan_ip="${3:-${LAN_IP:-}}"

    if hot_dev_bind_address_is_loopback "${bind_host}"; then
        return 0
    fi
    if [[ -z "${lan_ip}" || "${lan_ip}" == "0.0.0.0" ]]; then
        return 0
    fi

    printf 'http://%s:%s\n' "${lan_ip}" "${port}"
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

        if ! hot_dev_lan_enabled; then
            if declare -F log_warn >/dev/null 2>&1; then
                log_warn "${label}=${url} points at this machine, but hot-dev is local-only by default. Leaving BIND_ADDRESS=${bind_address}; set PULSE_DEV_LAN=true to expose this dev backend to installed agents."
            fi
            return 0
        fi

        BIND_ADDRESS="0.0.0.0"
        export BIND_ADDRESS

        if declare -F log_warn >/dev/null 2>&1; then
            log_warn "${label}=${url} points at this machine, but BIND_ADDRESS=${bind_address} only accepts local connections. Exposing the dev backend on 0.0.0.0 so installed agents can report metrics."
        fi
        return 0
    done
}
