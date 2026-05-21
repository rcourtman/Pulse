#!/usr/bin/env bash

HOT_DEV_DEFAULT_AUTH_USER="admin"
HOT_DEV_DEFAULT_AUTH_PASSWORD="adminadminadmin"
HOT_DEV_DEFAULT_AUTH_HASH='$2a$12$J/Vu6FlBUJDTK.VAkysjB.AnvFOcijDbETyumhCB.nJVes5gvpiI6'
HOT_DEV_DEFAULT_BOOTSTRAP_TOKEN="0123456789abcdef0123456789abcdef0123456789abcdef"

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
            grep -v -E '^(# Managed by hot-dev\.sh for deterministic dev auth|PULSE_AUTH_USER=|PULSE_AUTH_PASS=)' "${runtime_env}" || true
        fi
    } > "${tmp_file}"

    mv "${tmp_file}" "${runtime_env}"
    chmod 600 "${runtime_env}"
}

hot_dev_read_or_create_audit_signing_key() {
    local data_dir=$1
    local key_file="${data_dir}/.audit-signing.key"

    [[ -n "${data_dir}" ]] || return 1
    mkdir -p "${data_dir}"
    if [[ ! -s "${key_file}" ]]; then
        openssl rand -hex 32 > "${key_file}"
        chmod 600 "${key_file}"
    fi
    tr -d '\r\n' < "${key_file}"
}

hot_dev_sync_audit_signing_env_file() {
    local runtime_env=$1
    local data_dir=$2
    local signing_key

    [[ -n "${runtime_env}" ]] || return 0
    [[ -n "${data_dir}" ]] || return 0
    mkdir -p "$(dirname "${runtime_env}")"
    if [[ -f "${runtime_env}" ]] && grep -q -E '^PULSE_AUDIT_SIGNING_KEY=' "${runtime_env}"; then
        return 0
    fi

    signing_key="$(hot_dev_read_or_create_audit_signing_key "${data_dir}")"
    {
        if [[ -f "${runtime_env}" ]]; then
            cat "${runtime_env}"
        fi
        printf "PULSE_AUDIT_SIGNING_KEY='%s'\n" "$(hot_dev_single_quote "${signing_key}")"
    } > "${runtime_env}.audit.$$"
    mv "${runtime_env}.audit.$$" "${runtime_env}"
    chmod 600 "${runtime_env}"
}

hot_dev_sync_bootstrap_token_file() {
    local data_dir=$1
    local explicit_token="${PULSE_E2E_BOOTSTRAP_TOKEN:-}"
    local token="${explicit_token:-${HOT_DEV_DEFAULT_BOOTSTRAP_TOKEN}}"
    local token_file="${data_dir}/.bootstrap_token"

    [[ -n "${data_dir}" ]] || return 0
    [[ -n "${token}" ]] || return 0
    if [[ -f "${token_file}" ]] && [[ ! -s "${token_file}" ]]; then
        rm -f "${token_file}"
    fi
    if [[ -f "${token_file}" && -z "${explicit_token}" ]]; then
        return 0
    fi

    mkdir -p "${data_dir}"
    printf '%s\n' "${token}" > "${token_file}"
    chmod 600 "${token_file}"
}
