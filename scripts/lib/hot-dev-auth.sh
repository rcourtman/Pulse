#!/usr/bin/env bash

HOT_DEV_DEFAULT_AUTH_USER="admin"
HOT_DEV_DEFAULT_AUTH_PASSWORD="adminadminadmin"
HOT_DEV_DEFAULT_AUTH_HASH='$2a$12$J/Vu6FlBUJDTK.VAkysjB.AnvFOcijDbETyumhCB.nJVes5gvpiI6'

hot_dev_resolve_auth_user() {
    printf '%s\n' "${HOT_DEV_AUTH_USER:-${HOT_DEV_DEFAULT_AUTH_USER}}"
}

hot_dev_resolve_auth_pass() {
    if [[ -n "${HOT_DEV_AUTH_PASS:-}" ]]; then
        printf '%s\n' "${HOT_DEV_AUTH_PASS}"
        return
    fi

    printf '%s\n' "${HOT_DEV_DEFAULT_AUTH_HASH}"
}

hot_dev_is_default_auth() {
    local auth_user=$1
    local auth_pass=$2

    [[ "${auth_user}" == "${HOT_DEV_DEFAULT_AUTH_USER}" ]] || return 1
    [[ "${auth_pass}" == "${HOT_DEV_DEFAULT_AUTH_HASH}" || "${auth_pass}" == "${HOT_DEV_DEFAULT_AUTH_PASSWORD}" ]]
}

hot_dev_auth_banner_line() {
    local auth_user=$1
    local auth_pass=$2

    if hot_dev_is_default_auth "${auth_user}" "${auth_pass}"; then
        printf '%s / %s\n' "${HOT_DEV_DEFAULT_AUTH_USER}" "${HOT_DEV_DEFAULT_AUTH_PASSWORD}"
        return
    fi

    printf 'custom via HOT_DEV_AUTH_USER / HOT_DEV_AUTH_PASS\n'
}

hot_dev_single_quote() {
    printf '%s' "$1" | sed "s/'/'\\\\''/g"
}

hot_dev_sync_auth_env_file() {
    local runtime_env=$1
    local auth_user=$2
    local auth_pass=$3
    local tmp_file

    mkdir -p "$(dirname "${runtime_env}")"
    tmp_file="${runtime_env}.tmp.$$"

    {
        printf '# Managed by hot-dev.sh for deterministic dev auth\n'
        printf "PULSE_AUTH_USER='%s'\n" "$(hot_dev_single_quote "${auth_user}")"
        printf "PULSE_AUTH_PASS='%s'\n" "$(hot_dev_single_quote "${auth_pass}")"

        if [[ -f "${runtime_env}" ]]; then
            grep -v -E '^(# Managed by hot-dev\.sh for deterministic dev auth|PULSE_AUTH_USER=|PULSE_AUTH_PASS=)' "${runtime_env}"
        fi
    } > "${tmp_file}"

    mv "${tmp_file}" "${runtime_env}"
    chmod 600 "${runtime_env}"
}
