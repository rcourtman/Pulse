#!/usr/bin/env bash

# Remote release validator.
# Downloads the published (or draft) assets straight from GitHub Releases,
# authenticates checksums.txt against the configured release trust root,
# recalculates every listed SHA256 sum, and authenticates each artifact's
# *.sshsig sidecar. This prevents broken or forged updates when artifacts are
# re-uploaded without regenerating their integrity metadata (see issue #698).

set -euo pipefail

if [ $# -lt 1 ]; then
    echo "Usage: $0 <tag> [owner/repo]" >&2
    echo "Example: $0 v4.28.0 rcourtman/Pulse" >&2
    exit 1
fi

TAG="$1"
REPO="${2:-rcourtman/Pulse}"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

curl_args=(curl -fsSL --connect-timeout 10 --max-time 600 --retry 3 --retry-delay 2 --retry-all-errors)

for command in curl go sha256sum ssh-keygen; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "${command} is required to authenticate published release assets." >&2
        exit 1
    fi
done

UPDATE_PUBLIC_KEY="$(printf '%s' "${PULSE_UPDATE_SIGNING_PUBLIC_KEY:-}" | tr -d '\r\n[:space:]')"
if [[ -z "$UPDATE_PUBLIC_KEY" ]]; then
    echo "PULSE_UPDATE_SIGNING_PUBLIC_KEY is required to authenticate published release assets." >&2
    exit 1
fi

if ! UPDATE_SSH_PUBLIC_KEY="$(
    go -C "$REPO_ROOT" run ./scripts/release_update_key.go public-key-ssh \
        --public-key "$UPDATE_PUBLIC_KEY" \
        --comment pulse-installer
)"; then
    echo "PULSE_UPDATE_SIGNING_PUBLIC_KEY is not a valid release signing public key." >&2
    exit 1
fi
ALLOWED_SIGNERS_PATH="${TMP_DIR}/allowed_signers"
printf 'pulse-installer namespaces="pulse-install" %s\n' "$UPDATE_SSH_PUBLIC_KEY" >"$ALLOWED_SIGNERS_PATH"

verify_signature() {
    local payload_path="$1"
    local signature_path="$2"
    local label="$3"

    if ! ssh-keygen -Y verify \
        -f "$ALLOWED_SIGNERS_PATH" \
        -I pulse-installer \
        -n pulse-install \
        -s "$signature_path" <"$payload_path" >/dev/null 2>&1; then
        echo "SSH signature verification failed for ${label}." >&2
        return 1
    fi
}

CHECKSUMS_PATH="${TMP_DIR}/checksums.txt"
echo "Downloading ${BASE_URL}/checksums.txt"
if ! "${curl_args[@]}" "${BASE_URL}/checksums.txt" >"$CHECKSUMS_PATH"; then
    echo "Failed to download checksums.txt for ${TAG}" >&2
    exit 1
fi

CHECKSUMS_SIG_PATH="${TMP_DIR}/checksums.txt.sshsig"
echo "Downloading ${BASE_URL}/checksums.txt.sshsig"
if ! "${curl_args[@]}" "${BASE_URL}/checksums.txt.sshsig" >"$CHECKSUMS_SIG_PATH"; then
    echo "Failed to download checksums.txt.sshsig for ${TAG}" >&2
    exit 1
fi
if [[ ! -s "$CHECKSUMS_SIG_PATH" ]]; then
    echo "checksums.txt.sshsig is empty for ${TAG}" >&2
    exit 1
fi
if ! verify_signature "$CHECKSUMS_PATH" "$CHECKSUMS_SIG_PATH" "checksums.txt for ${TAG}"; then
    exit 1
fi

RELEASE_SBOM="pulse-${TAG}-release.sbom.spdx.json"
if ! awk '{print $2}' "$CHECKSUMS_PATH" | grep -Fx "$RELEASE_SBOM" >/dev/null 2>&1; then
    echo "checksums.txt does not list ${RELEASE_SBOM} for ${TAG}" >&2
    exit 1
fi

RELEASE_SBOM_PATH="${TMP_DIR}/${RELEASE_SBOM}"
echo "Downloading ${BASE_URL}/${RELEASE_SBOM}"
if ! "${curl_args[@]}" "${BASE_URL}/${RELEASE_SBOM}" >"$RELEASE_SBOM_PATH"; then
    echo "Failed to download ${RELEASE_SBOM} for ${TAG}" >&2
    exit 1
fi
if [[ ! -s "$RELEASE_SBOM_PATH" ]]; then
    echo "${RELEASE_SBOM} is empty for ${TAG}" >&2
    exit 1
fi

status=0

while read -r checksum filename _; do
    [[ -z "${checksum:-}" ]] && continue
    [[ "$checksum" =~ ^# ]] && continue
    if [[ -z "${filename:-}" ]]; then
        echo "Malformed checksums line (missing filename): $checksum" >&2
        status=1
        continue
    fi
    if [[ ! "$checksum" =~ ^[0-9a-f]{64}$ ]]; then
        echo "Malformed checksums line (invalid SHA-256 for ${filename}): ${checksum}" >&2
        status=$((status + 1))
        continue
    fi
    if [[ ! "$filename" =~ ^[A-Za-z0-9._+-]+$ ]]; then
        echo "Unsafe release asset filename in checksums.txt: ${filename}" >&2
        status=$((status + 1))
        continue
    fi

    artifact_url="${BASE_URL}/${filename}"
    echo "Verifying ${filename}..."

    artifact_path="${TMP_DIR}/${filename}"
    if ! "${curl_args[@]}" "$artifact_url" >"$artifact_path"; then
        echo "Failed to download ${filename}" >&2
        status=$((status + 1))
        rm -f "$artifact_path"
        continue
    fi
    actual_checksum="$(sha256sum "$artifact_path" | awk '{print $1}')"

    if [[ "$actual_checksum" != "$checksum" ]]; then
        echo "Checksum mismatch for ${filename}: expected ${checksum}, got ${actual_checksum}" >&2
        status=$((status + 1))
    fi

    sha_url="${artifact_url}.sha256"
    if ! sha_content=$("${curl_args[@]}" "$sha_url" | tr -d '\r' | sed 's/[[:space:]]*$//'); then
        echo "Failed to download ${filename}.sha256" >&2
        status=$((status + 1))
        rm -f "$artifact_path"
        continue
    fi

    expected_line="${checksum}  ${filename}"
    if [[ "$sha_content" != "$expected_line" ]]; then
        echo "${filename}.sha256 content mismatch (expected '${expected_line}', got '${sha_content}')" >&2
        status=$((status + 1))
    fi

    sshsig_path="${TMP_DIR}/${filename}.sshsig"
    if ! "${curl_args[@]}" "${artifact_url}.sshsig" >"$sshsig_path"; then
        echo "Failed to download ${filename}.sshsig" >&2
        status=$((status + 1))
        rm -f "$artifact_path" "$sshsig_path"
        continue
    fi
    if [[ ! -s "$sshsig_path" ]]; then
        echo "${filename}.sshsig is empty" >&2
        status=$((status + 1))
        rm -f "$artifact_path"
        continue
    fi
    if ! verify_signature "$artifact_path" "$sshsig_path" "$filename"; then
        status=$((status + 1))
    fi
    rm -f "$artifact_path"
done < "$CHECKSUMS_PATH"

if [[ "$status" -ne 0 ]]; then
    echo "Published release validation failed for ${TAG} (${status} error(s))." >&2
    exit 1
fi

echo "Published release assets for ${TAG} match authenticated checksums.txt, *.sha256 files, and verified *.sshsig sidecars."
