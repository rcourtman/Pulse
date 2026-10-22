#!/usr/bin/env bash

# Installed configuration is never an implicit input to a development launch.
# The optional directory argument is for callers testing with disposable data;
# the launcher itself always supplies the fixed installed path, /etc/pulse.

hot_dev_validate_data_source() {
    local installed_dir=$1 selected_dir="${PULSE_DATA_DIR:-}"

    [[ -n "${selected_dir}" ]] || return 0
    # Resolve existing aliases as well as the ordinary installed path. This is
    # an accidental-use guard, not a sandbox for arbitrary local shell .env files.
    selected_dir="$(cd "${selected_dir}" 2>/dev/null && pwd -P || printf '%s' "${selected_dir%/}")"
    installed_dir="$(cd "${installed_dir}" 2>/dev/null && pwd -P || printf '%s' "${installed_dir%/}")"
    if [[ "${selected_dir}/" == "${installed_dir}/"* ]] && [[ "${HOT_DEV_USE_PROD_DATA}" != true ]]; then
        log_error "Installed data requires HOT_DEV_USE_PROD_DATA=true; development enables an authentication bypass. Use a separate PULSE_DATA_DIR for isolated development."
        return 1
    fi
}

hot_dev_prepare_config_source() {
    local installed_dir=$1 flag

    HOT_DEV_USE_PROD_DATA="${HOT_DEV_USE_PROD_DATA:-false}"
    HOT_DEV_IMPORT_PROD_CONFIG="${HOT_DEV_IMPORT_PROD_CONFIG:-false}"
    for flag in HOT_DEV_USE_PROD_DATA HOT_DEV_IMPORT_PROD_CONFIG; do
        case "${!flag}" in
            true|false) ;;
            *) log_error "${flag} must be exactly true or false."; return 1 ;;
        esac
    done
    if [[ "${HOT_DEV_USE_PROD_DATA}" == true && "${HOT_DEV_IMPORT_PROD_CONFIG}" == true ]]; then
        log_error "Choose direct installed data or an imported development copy, not both."
        return 1
    fi
    # Configuration loaded afterwards cannot silently change the chosen mode.
    readonly HOT_DEV_USE_PROD_DATA HOT_DEV_IMPORT_PROD_CONFIG
    hot_dev_validate_data_source "${installed_dir}"
}

hot_dev_load_installed_env() {
    local installed_dir=$1
    if [[ "${HOT_DEV_USE_PROD_DATA}" == true ]]; then
        log_warn "HOT_DEV_USE_PROD_DATA=true: using installed configuration with development authentication bypass; only use an intentionally selected, trusted lab."
        if [[ -f "${installed_dir}/.env" && -r "${installed_dir}/.env" ]]; then
            load_env_file "${installed_dir}/.env"
        fi
    fi
    hot_dev_validate_data_source "${installed_dir}"
}

hot_dev_import_installed_config() {
    [[ "${HOT_DEV_IMPORT_PROD_CONFIG}" == true ]] || return 0

    # Preserve the existing lab-copy route, but never infer consent from LAN,
    # lab-agent or mock settings, nor copy into an arbitrary caller data path.
    if [[ "${PULSE_DATA_DIR}" != "${ROOT_DIR}/tmp/dev-config" || "${PULSE_MOCK_MODE:-false}" == true ]]; then
        log_error "HOT_DEV_IMPORT_PROD_CONFIG=true requires the default non-mock development directory; unset PULSE_DATA_DIR and disable mock mode."
        return 1
    fi
    log_warn "HOT_DEV_IMPORT_PROD_CONFIG=true: importing installed targets, credentials and keys into development; this is not isolated reproduction."
    DEV_DIR="${PULSE_DATA_DIR}" "${ROOT_DIR}/scripts/sync-production-config.sh"
}
