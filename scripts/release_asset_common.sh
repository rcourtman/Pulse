#!/usr/bin/env bash

# Shared helpers for Pulse release asset assembly and integrity metadata.

: "${PULSE_SCRIPTS_DIR:=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
: "${PULSE_REPO_ROOT:=$(cd "${PULSE_SCRIPTS_DIR}/.." && pwd)}"

pulse_release_go_run_update_key() {
    go -C "${PULSE_REPO_ROOT}" run ./scripts/release_update_key.go "$@"
}

pulse_release_prepare_signing_state() {
    local signer_identity="${1:-pulse-installer}"
    local signer_namespace="${2:-pulse-install}"
    local actual_fingerprint=""
    local expected_public_key=""
    local expected_fingerprint=""

    PULSE_RELEASE_SIGNER_IDENTITY="${signer_identity}"
    PULSE_RELEASE_SIGNER_NAMESPACE="${signer_namespace}"
    PULSE_RELEASE_UPDATE_PUBLIC_KEY=""
    PULSE_RELEASE_UPDATE_PUBLIC_KEY_FINGERPRINT=""
    PULSE_RELEASE_UPDATE_SSH_PUBLIC_KEY=""
    PULSE_RELEASE_UPDATE_SSH_PRIVATE_KEY_FILE=""

    if [[ -z "${PULSE_UPDATE_SIGNING_KEY:-}" ]]; then
        if [[ "${PULSE_ALLOW_MISSING_UPDATE_SIGNING_KEY:-false}" == "true" ]]; then
            echo "Warning: PULSE_UPDATE_SIGNING_KEY not set; continuing because PULSE_ALLOW_MISSING_UPDATE_SIGNING_KEY=true."
            return 0
        fi
        echo "Error: PULSE_UPDATE_SIGNING_KEY is required for release builds." >&2
        echo "Set PULSE_ALLOW_MISSING_UPDATE_SIGNING_KEY=true only for local non-release debugging." >&2
        exit 1
    fi

    PULSE_RELEASE_UPDATE_PUBLIC_KEY="$(pulse_release_go_run_update_key public-key --private-key "${PULSE_UPDATE_SIGNING_KEY}")"
    if [[ -z "${PULSE_RELEASE_UPDATE_PUBLIC_KEY}" ]]; then
        echo "Error: failed to derive update signing public key." >&2
        exit 1
    fi

    PULSE_RELEASE_UPDATE_PUBLIC_KEY_FINGERPRINT="$(pulse_release_go_run_update_key fingerprint --public-key "${PULSE_RELEASE_UPDATE_PUBLIC_KEY}")"
    if [[ -z "${PULSE_RELEASE_UPDATE_PUBLIC_KEY_FINGERPRINT}" ]]; then
        echo "Error: failed to derive update signing public key fingerprint." >&2
        exit 1
    fi

    expected_public_key="$(printf '%s' "${PULSE_UPDATE_SIGNING_PUBLIC_KEY:-}" | tr -d '\r\n[:space:]')"
    if [[ -n "${expected_public_key}" && "${PULSE_RELEASE_UPDATE_PUBLIC_KEY}" != "${expected_public_key}" ]]; then
        echo "Error: PULSE_UPDATE_SIGNING_KEY does not match PULSE_UPDATE_SIGNING_PUBLIC_KEY." >&2
        echo "Expected public key: ${expected_public_key}" >&2
        echo "Actual public key:   ${PULSE_RELEASE_UPDATE_PUBLIC_KEY}" >&2
        exit 1
    fi

    expected_fingerprint="$(printf '%s' "${PULSE_UPDATE_SIGNING_PUBLIC_KEY_FINGERPRINT:-}" | tr -d '\r\n[:space:]')"
    if [[ -n "${expected_fingerprint}" ]]; then
        expected_fingerprint="${expected_fingerprint#SHA256:}"
        actual_fingerprint="${PULSE_RELEASE_UPDATE_PUBLIC_KEY_FINGERPRINT#SHA256:}"
        if [[ "${actual_fingerprint}" != "${expected_fingerprint}" ]]; then
            echo "Error: PULSE_UPDATE_SIGNING_KEY fingerprint mismatch." >&2
            echo "Expected: SHA256:${expected_fingerprint}" >&2
            echo "Actual:   ${PULSE_RELEASE_UPDATE_PUBLIC_KEY_FINGERPRINT}" >&2
            exit 1
        fi
    fi

    if [[ -n "${expected_public_key}" || -n "${expected_fingerprint}" ]]; then
        echo "Verified update signing public key fingerprint: ${PULSE_RELEASE_UPDATE_PUBLIC_KEY_FINGERPRINT}"
    fi

    if ! command -v ssh-keygen >/dev/null 2>&1; then
        echo "Error: ssh-keygen is required to sign installer release assets." >&2
        exit 1
    fi

    PULSE_RELEASE_UPDATE_SSH_PUBLIC_KEY="$(pulse_release_go_run_update_key public-key-ssh --private-key "${PULSE_UPDATE_SIGNING_KEY}" --comment "${PULSE_RELEASE_SIGNER_IDENTITY}")"
    if [[ -z "${PULSE_RELEASE_UPDATE_SSH_PUBLIC_KEY}" ]]; then
        echo "Error: failed to derive installer SSH signing public key." >&2
        exit 1
    fi

    PULSE_RELEASE_UPDATE_SSH_PRIVATE_KEY_FILE="$(mktemp)"
    pulse_release_go_run_update_key openssh-private-key --private-key "${PULSE_UPDATE_SIGNING_KEY}" --comment "${PULSE_RELEASE_SIGNER_IDENTITY}" > "${PULSE_RELEASE_UPDATE_SSH_PRIVATE_KEY_FILE}"
    chmod 600 "${PULSE_RELEASE_UPDATE_SSH_PRIVATE_KEY_FILE}"
}

pulse_release_cleanup_signing_state() {
    if [[ -n "${PULSE_RELEASE_UPDATE_SSH_PRIVATE_KEY_FILE:-}" && -f "${PULSE_RELEASE_UPDATE_SSH_PRIVATE_KEY_FILE}" ]]; then
        rm -f "${PULSE_RELEASE_UPDATE_SSH_PRIVATE_KEY_FILE}"
    fi
}

pulse_release_sign_file() {
    local file="$1"
    local absolute_file="${file}"

    if [[ -z "${PULSE_UPDATE_SIGNING_KEY:-}" ]]; then
        return 0
    fi

    if [[ "${absolute_file}" != /* ]]; then
        absolute_file="$(pwd)/${file}"
    fi

    rm -f "${absolute_file}.sig" "${absolute_file}.sshsig"
    ssh-keygen -q -Y sign \
        -f "${PULSE_RELEASE_UPDATE_SSH_PRIVATE_KEY_FILE}" \
        -n "${PULSE_RELEASE_SIGNER_NAMESPACE}" \
        "${absolute_file}" >/dev/null
    mv "${absolute_file}.sig" "${absolute_file}.sshsig"
    pulse_release_go_run_update_key sign --private-key "${PULSE_UPDATE_SIGNING_KEY}" --file "${absolute_file}" > "${absolute_file}.sig"
}

pulse_release_sign_directory_assets() {
    local dir="$1"

    if [[ -z "${PULSE_UPDATE_SIGNING_KEY:-}" ]]; then
        return 0
    fi

    while IFS= read -r -d '' file; do
        pulse_release_sign_file "${file}"
    done < <(find "${dir}" -maxdepth 1 -type f ! -name '*.sig' ! -name '*.sshsig' -print0)
}

pulse_release_generate_packet_sbom() {
    local release_dir="$1"
    local output_name="$2"
    local sbom_tool="${PULSE_RELEASE_SBOM_TOOL:-syft}"
    local tmp_base="${BUILD_DIR:-${release_dir}}"
    local resolved_tool=""
    local tmp_sbom=""

    if [[ "${sbom_tool}" == */* ]]; then
        resolved_tool="${sbom_tool}"
    else
        resolved_tool="$(command -v "${sbom_tool}" || true)"
    fi

    if [[ -z "${resolved_tool}" || ! -x "${resolved_tool}" ]]; then
        if [[ "${PULSE_ALLOW_MISSING_RELEASE_SBOM_TOOL:-false}" == "true" ]]; then
            echo "Warning: syft not installed; skipping release-packet SBOM because PULSE_ALLOW_MISSING_RELEASE_SBOM_TOOL=true."
            return 0
        fi
        echo "Error: syft is required to generate the release-packet SBOM." >&2
        echo "Install syft or set PULSE_ALLOW_MISSING_RELEASE_SBOM_TOOL=true only for local non-release debugging." >&2
        exit 1
    fi

    mkdir -p "${tmp_base}"
    tmp_sbom="$(mktemp "${tmp_base}/release-packet-sbom.XXXXXX")"
    echo "Generating release-packet SBOM ${output_name}..."
    "${resolved_tool}" "dir:${release_dir}" -o "spdx-json=${tmp_sbom}"
    mv "${tmp_sbom}" "${release_dir}/${output_name}"
}

pulse_release_collect_checksum_files() {
    local release_dir="$1"

    (
        cd "${release_dir}"
        shopt -s nullglob extglob
        local -a checksum_files=()

        if compgen -G "pulse-*.tar.gz" > /dev/null; then
            checksum_files+=( pulse-*.tar.gz )
        fi
        if compgen -G "pulse-*.tgz" > /dev/null; then
            checksum_files+=( pulse-*.tgz )
        fi
        if compgen -G "pulse-*.zip" > /dev/null; then
            checksum_files+=( pulse-*.zip )
        fi
        if compgen -G "pulse-*.sbom.spdx.json" > /dev/null; then
            checksum_files+=( pulse-*.sbom.spdx.json )
        fi
        if compgen -G "pulse-agent-linux-*" > /dev/null; then
            checksum_files+=( pulse-agent-linux-* )
        fi
        if compgen -G "pulse-agent-freebsd-*" > /dev/null; then
            checksum_files+=( pulse-agent-freebsd-* )
        fi
        if compgen -G "pulse-*.exe" > /dev/null; then
            checksum_files+=( pulse-*.exe )
        fi
        if [[ -f "install.sh" ]]; then
            checksum_files+=( install.sh )
        fi
        if [[ -f "install-docker.sh" ]]; then
            checksum_files+=( install-docker.sh )
        fi
        if [[ -f "pulse-auto-update.sh" ]]; then
            checksum_files+=( pulse-auto-update.sh )
        fi
        if compgen -G "install*.ps1" > /dev/null; then
            checksum_files+=( install*.ps1 )
        fi

        if [[ ${#checksum_files[@]} -gt 0 ]]; then
            printf '%s\n' "${checksum_files[@]}" | sort -u
        fi
    )
}

pulse_release_write_checksums_and_signatures() {
    local release_dir="$1"
    shift
    local -a checksum_files=("$@")

    (
        cd "${release_dir}"

        rm -f checksums.txt checksums.txt.sig checksums.txt.sshsig
        find . -maxdepth 1 -type f -name '*.sha256' -delete
        find . -maxdepth 1 -type f \( -name '*.sig' -o -name '*.sshsig' \) -delete

        if [[ ${#checksum_files[@]} -eq 0 ]]; then
            echo "Warning: no release artifacts found to checksum."
            exit 0
        fi

        local checksum_output=""
        checksum_output="$(sha256sum "${checksum_files[@]}" | sort -k 2)"
        printf '%s\n' "${checksum_output}" > checksums.txt

        while read -r checksum filename; do
            [[ -n "${checksum:-}" && -n "${filename:-}" ]] || continue
            printf '%s  %s\n' "${checksum}" "${filename}" > "${filename}.sha256"
        done <<< "${checksum_output}"

        local artifact=""
        for artifact in "${checksum_files[@]}" checksums.txt; do
            pulse_release_sign_file "${artifact}"
        done

        if [[ -n "${SIGNING_KEY_ID:-}" ]]; then
            if command -v gpg >/dev/null 2>&1; then
                echo "Signing checksums with GPG key ${SIGNING_KEY_ID}..."
                gpg --batch --yes --detach-sign --armor \
                    --local-user "${SIGNING_KEY_ID}" \
                    --output checksums.txt.asc \
                    checksums.txt
            else
                echo "SIGNING_KEY_ID is set but gpg is not installed; skipping signature."
            fi
        fi
    )
}
