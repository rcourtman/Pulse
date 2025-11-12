#!/usr/bin/env bash

# Remote release validator.
# Downloads the published (or draft) assets straight from GitHub Releases,
# recalculates their SHA256 sums, and ensures checksums.txt and the *.sha256
# helper files match what is actually live. This prevents broken updates when
# artifacts are re-uploaded without regenerating checksums (see issue #698).

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
done < "$CHECKSUMS_PATH"

if [[ "$status" -ne 0 ]]; then
    echo "Published release validation failed for ${TAG} (${status} error(s))." >&2
    exit 1
fi

echo "Published release assets for ${TAG} match checksums.txt and *.sha256 files."
