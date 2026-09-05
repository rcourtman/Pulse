#!/usr/bin/env bash

# Resolve the exact-version OCI Helm chart to its registry digest and verify
# that GitHub's signed build provenance binds it to the release source and the
# canonical hosted chart publisher. When an output path is supplied, preserve
# the exact OCI package whose digest and provenance passed those checks.

set -euo pipefail

if [ "$#" -lt 2 ] || [ "$#" -gt 5 ]; then
    echo "Usage: $0 <tag> <source-sha> [owner/repo] [expected-digest] [output-chart]" >&2
    exit 2
fi

TAG="$1"
SOURCE_SHA="$2"
REPOSITORY="${3:-${GITHUB_REPOSITORY:-rcourtman/Pulse}}"
EXPECTED_DIGEST="${4:-}"
OUTPUT_CHART="${5:-}"
OWNER="${REPOSITORY%%/*}"
VERSION="${TAG#v}"
SUBJECT="ghcr.io/${OWNER}/pulse-chart/pulse"
SIGNER_WORKFLOW="github.com/${REPOSITORY}/.github/workflows/publish-helm-chart.yml"

if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-((rc|alpha|beta)\.[0-9]+))?$ ]]; then
    echo "Invalid release tag: ${TAG}" >&2
    exit 1
fi
if [[ ! "$SOURCE_SHA" =~ ^[0-9a-f]{40}$ ]]; then
    echo "Invalid release source SHA: ${SOURCE_SHA}" >&2
    exit 1
fi
if [[ ! "$REPOSITORY" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
    echo "Invalid GitHub repository: ${REPOSITORY}" >&2
    exit 1
fi
if [ -n "$EXPECTED_DIGEST" ] && \
   [[ ! "$EXPECTED_DIGEST" =~ ^sha256:[0-9a-f]{64}$ ]]; then
    echo "Invalid expected Helm chart digest: ${EXPECTED_DIGEST}" >&2
    exit 1
fi

for command in gh helm; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "${command} is required to verify the release Helm chart." >&2
        exit 1
    fi
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
"${SCRIPT_DIR}/require-safe-gh-attestation.sh"

pull_dir="$(mktemp -d)"
cleanup() { rm -rf "$pull_dir"; }
trap cleanup EXIT

pull_output="$(
    helm pull "oci://${SUBJECT}" \
        --version "$VERSION" \
        --destination "$pull_dir" 2>&1
)" || {
    echo "$pull_output" >&2
    echo "Unable to resolve the exact-version Helm chart ${SUBJECT}:${VERSION}." >&2
    exit 1
}
printf '%s\n' "$pull_output" >&2

mapfile -t resolved_digests < <(
    sed -nE 's/^Digest:[[:space:]]*(sha256:[0-9a-f]{64})\r?$/\1/p' \
        <<<"$pull_output" | sort -u
)
if [ "${#resolved_digests[@]}" -ne 1 ]; then
    echo "Helm did not report one exact OCI digest for ${SUBJECT}:${VERSION}." >&2
    exit 1
fi
chart_digest="${resolved_digests[0]}"
if [ -n "$EXPECTED_DIGEST" ] && [ "$chart_digest" != "$EXPECTED_DIGEST" ]; then
    echo "Exact-version Helm chart moved: expected ${EXPECTED_DIGEST}, observed ${chart_digest}." >&2
    exit 1
fi

gh attestation verify "oci://${SUBJECT}@${chart_digest}" \
    --repo "$REPOSITORY" \
    --bundle-from-oci \
    --signer-workflow "$SIGNER_WORKFLOW" \
    --source-digest "$SOURCE_SHA" \
    --deny-self-hosted-runners \
    --predicate-type https://slsa.dev/provenance/v1 \
    >/dev/null

if [ -n "$OUTPUT_CHART" ]; then
    resolved_chart="${pull_dir}/pulse-${VERSION}.tgz"
    if [ ! -f "$resolved_chart" ] || [ -L "$resolved_chart" ]; then
        echo "Verified OCI pull did not produce one regular chart package: ${resolved_chart}" >&2
        exit 1
    fi
    if [ -L "$OUTPUT_CHART" ]; then
        echo "Refusing to replace symlink output path: ${OUTPUT_CHART}" >&2
        exit 1
    fi
    install -m 0644 -- "$resolved_chart" "$OUTPUT_CHART"
fi

printf 'chart_digest=%s\n' "$chart_digest"
echo "[OK] Helm chart ${VERSION} and hosted build provenance resolve to ${chart_digest}." >&2
