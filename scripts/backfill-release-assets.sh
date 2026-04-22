#!/usr/bin/env bash

# Regenerate integrity metadata for an already-published Pulse release packet
# without rebuilding the underlying payload artifacts from the current branch.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PULSE_SCRIPTS_DIR="${SCRIPT_DIR}"
PULSE_REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${PULSE_REPO_ROOT}"

source "${SCRIPT_DIR}/release_asset_common.sh"

usage() {
    cat <<'EOF' >&2
Usage: scripts/backfill-release-assets.sh --tag <tag> [options]

Options:
  --repo <owner/repo>   GitHub repository (default: rcourtman/Pulse)
  --release-dir <path>  Reuse an existing local asset directory
  --skip-download       Skip gh release download and use --release-dir as-is
  --skip-upload         Skip gh release upload after regenerating metadata

Examples:
  scripts/backfill-release-assets.sh --tag v6.0.0-rc.2
  scripts/backfill-release-assets.sh --tag v6.0.0-rc.2 --release-dir /tmp/release --skip-download --skip-upload
EOF
    exit 1
}

TAG=""
REPO="rcourtman/Pulse"
RELEASE_DIR=""
SKIP_DOWNLOAD="false"
SKIP_UPLOAD="false"
WORK_DIR=""
PAYLOAD_DIR=""
RELEASE_PACKET_SBOM=""
payload_files=()

while [[ $# -gt 0 ]]; do
    case "$1" in
        --tag)
            TAG="${2:-}"
            shift 2
            ;;
        --repo)
            REPO="${2:-}"
            shift 2
            ;;
        --release-dir)
            RELEASE_DIR="${2:-}"
            shift 2
            ;;
        --skip-download)
            SKIP_DOWNLOAD="true"
            shift
            ;;
        --skip-upload)
            SKIP_UPLOAD="true"
            shift
            ;;
        *)
            usage
            ;;
    esac
done

[[ -n "${TAG}" ]] || usage

RELEASE_PACKET_SBOM="pulse-${TAG}-release.sbom.spdx.json"

cleanup() {
    pulse_release_cleanup_signing_state
    if [[ -n "${PAYLOAD_DIR}" && -d "${PAYLOAD_DIR}" ]]; then
        rm -rf "${PAYLOAD_DIR}"
    fi
    if [[ -n "${WORK_DIR}" && -d "${WORK_DIR}" ]]; then
        rm -rf "${WORK_DIR}"
    fi
}
trap cleanup EXIT

ensure_release_is_published() {
    local metadata=""
    metadata="$(gh release view "${TAG}" -R "${REPO}" --json isDraft,tagName)"
    if [[ "$(printf '%s' "${metadata}" | jq -r '.isDraft')" != "false" ]]; then
        echo "Error: ${TAG} is still a draft release; use the normal release pipeline instead of historical backfill." >&2
        exit 1
    fi
}

download_release_assets() {
    if [[ -z "${RELEASE_DIR}" ]]; then
        WORK_DIR="$(mktemp -d)"
        RELEASE_DIR="${WORK_DIR}/release"
    fi
    mkdir -p "${RELEASE_DIR}"
    echo "Downloading published assets for ${TAG} from ${REPO}..."
    gh release download "${TAG}" -R "${REPO}" --dir "${RELEASE_DIR}" --clobber
}

verify_existing_checksums() {
    local checksums_path="${RELEASE_DIR}/checksums.txt"
    local status=0

    if [[ ! -f "${checksums_path}" ]]; then
        echo "Error: ${checksums_path} is missing." >&2
        exit 1
    fi

    while read -r checksum filename _; do
        [[ -n "${checksum:-}" ]] || continue
        [[ "${checksum}" =~ ^# ]] && continue

        if [[ -z "${filename:-}" ]]; then
            echo "Malformed checksums.txt line: ${checksum}" >&2
            status=1
            continue
        fi

        if [[ ! -f "${RELEASE_DIR}/${filename}" ]]; then
            echo "Missing published artifact ${filename}" >&2
            status=1
            continue
        fi

        local actual_checksum=""
        actual_checksum="$(sha256sum "${RELEASE_DIR}/${filename}" | awk '{print $1}')"
        if [[ "${actual_checksum}" != "${checksum}" ]]; then
            echo "Checksum mismatch for ${filename}: expected ${checksum}, got ${actual_checksum}" >&2
            status=1
        fi

        local sha_path="${RELEASE_DIR}/${filename}.sha256"
        local expected_line="${checksum}  ${filename}"
        if [[ ! -f "${sha_path}" ]]; then
            echo "Missing ${filename}.sha256" >&2
            status=1
            continue
        fi

        local sha_content=""
        sha_content="$(tr -d '\r' < "${sha_path}" | sed 's/[[:space:]]*$//')"
        if [[ "${sha_content}" != "${expected_line}" ]]; then
            echo "${filename}.sha256 content mismatch (expected '${expected_line}', got '${sha_content}')" >&2
            status=1
        fi
    done < "${checksums_path}"

    if [[ "${status}" -ne 0 ]]; then
        echo "Error: published release packet for ${TAG} failed integrity verification before backfill." >&2
        exit 1
    fi
}

collect_payload_files() {
    payload_files=()

    while read -r checksum filename _; do
        [[ -n "${checksum:-}" ]] || continue
        [[ "${checksum}" =~ ^# ]] && continue
        [[ -n "${filename:-}" ]] || continue

        if [[ "${filename}" == "${RELEASE_PACKET_SBOM}" ]]; then
            continue
        fi

        payload_files+=( "${filename}" )
    done < "${RELEASE_DIR}/checksums.txt"

    if [[ ${#payload_files[@]} -eq 0 ]]; then
        echo "Error: ${TAG} does not contain any payload artifacts in checksums.txt." >&2
        exit 1
    fi
}

stage_payload_assets() {
    PAYLOAD_DIR="$(mktemp -d)"

    local filename=""
    for filename in "${payload_files[@]}"; do
        mkdir -p "${PAYLOAD_DIR}/$(dirname "${filename}")"
        cp "${RELEASE_DIR}/${filename}" "${PAYLOAD_DIR}/${filename}"
    done
}

rebuild_release_packet_integrity() {
    rm -f \
        "${RELEASE_DIR}/${RELEASE_PACKET_SBOM}" \
        "${RELEASE_DIR}/${RELEASE_PACKET_SBOM}.sig" \
        "${RELEASE_DIR}/${RELEASE_PACKET_SBOM}.sshsig"

    pulse_release_generate_packet_sbom "${PAYLOAD_DIR}" "${RELEASE_PACKET_SBOM}"
    cp "${PAYLOAD_DIR}/${RELEASE_PACKET_SBOM}" "${RELEASE_DIR}/${RELEASE_PACKET_SBOM}"

    local -a checksum_files=("${payload_files[@]}" "${RELEASE_PACKET_SBOM}")
    pulse_release_write_checksums_and_signatures "${RELEASE_DIR}" "${checksum_files[@]}"
}

upload_release_assets() {
    echo "Uploading regenerated integrity assets for ${TAG}..."
    gh release upload "${TAG}" "${RELEASE_DIR}/checksums.txt" --clobber
    if compgen -G "${RELEASE_DIR}/*.sha256" > /dev/null; then
        gh release upload "${TAG}" "${RELEASE_DIR}"/*.sha256 --clobber
    fi
    if compgen -G "${RELEASE_DIR}/*.sig" > /dev/null; then
        gh release upload "${TAG}" "${RELEASE_DIR}"/*.sig --clobber
    fi
    if compgen -G "${RELEASE_DIR}/*.sshsig" > /dev/null; then
        gh release upload "${TAG}" "${RELEASE_DIR}"/*.sshsig --clobber
    fi
    gh release upload "${TAG}" "${RELEASE_DIR}/${RELEASE_PACKET_SBOM}" --clobber
}

if [[ "${SKIP_DOWNLOAD}" == "true" ]]; then
    [[ -n "${RELEASE_DIR}" ]] || {
        echo "Error: --skip-download requires --release-dir." >&2
        exit 1
    }
else
    ensure_release_is_published
    download_release_assets
fi

[[ -d "${RELEASE_DIR}" ]] || {
    echo "Error: release directory ${RELEASE_DIR} does not exist." >&2
    exit 1
}

pulse_release_prepare_signing_state "pulse-installer" "pulse-install"
verify_existing_checksums
collect_payload_files
stage_payload_assets
rebuild_release_packet_integrity

if [[ "${SKIP_UPLOAD}" == "true" ]]; then
    echo "Skipped GitHub upload; regenerated assets are in ${RELEASE_DIR}"
else
    upload_release_assets
fi

echo "Historical release asset backfill completed for ${TAG}."
