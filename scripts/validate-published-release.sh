#!/usr/bin/env bash

# Remote release validator.
# Downloads the published (or draft) assets straight from GitHub Releases,
# recalculates their SHA256 sums, and ensures checksums.txt, the *.sha256
# helper files, and the required *.sshsig sidecars match the live release
# packet. This prevents broken updates when artifacts are re-uploaded without
# regenerating checksums or their pinned signature sidecars (see issue #698).

set -euo pipefail

if [ $# -lt 1 ]; then
    echo "Usage: $0 <tag> [owner/repo]" >&2
    echo "Example: $0 v4.28.0 rcourtman/Pulse" >&2
    exit 1
fi

TAG="$1"
REPO="${2:-rcourtman/Pulse}"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

curl_args=(curl -fsSL --connect-timeout 10 --max-time 600 --retry 3 --retry-delay 2 --retry-all-errors)

CHECKSUMS_PATH="${TMP_DIR}/checksums.txt"
echo "Downloading ${BASE_URL}/checksums.txt"
if ! "${curl_args[@]}" "${BASE_URL}/checksums.txt" >"$CHECKSUMS_PATH"; then
    echo "Failed to download checksums.txt for ${TAG}" >&2
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

status=0

while read -r checksum filename _; do
    [[ -z "${checksum:-}" ]] && continue
    [[ "$checksum" =~ ^# ]] && continue
    if [[ -z "${filename:-}" ]]; then
        echo "Malformed checksums line (missing filename): $checksum" >&2
        status=1
        continue
    fi

    artifact_url="${BASE_URL}/${filename}"
    echo "Verifying ${filename}..."

    if ! actual_checksum=$("${curl_args[@]}" "$artifact_url" | sha256sum | awk '{print $1}'); then
        echo "Failed to download ${filename}" >&2
        status=$((status + 1))
        continue
    fi

    if [[ "$actual_checksum" != "$checksum" ]]; then
        echo "Checksum mismatch for ${filename}: expected ${checksum}, got ${actual_checksum}" >&2
        status=$((status + 1))
    fi

    sha_url="${artifact_url}.sha256"
    if ! sha_content=$("${curl_args[@]}" "$sha_url" | tr -d '\r' | sed 's/[[:space:]]*$//'); then
        echo "Failed to download ${filename}.sha256" >&2
        status=$((status + 1))
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
        continue
    fi
    if [[ ! -s "$sshsig_path" ]]; then
        echo "${filename}.sshsig is empty" >&2
        status=$((status + 1))
    fi
done < "$CHECKSUMS_PATH"

if [[ "$status" -ne 0 ]]; then
    echo "Published release validation failed for ${TAG} (${status} error(s))." >&2
    exit 1
fi

echo "Published release assets for ${TAG} match checksums.txt, *.sha256 files, and required *.sshsig sidecars."
