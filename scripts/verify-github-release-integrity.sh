#!/usr/bin/env bash

# Verify GitHub's post-publication integrity boundary for a Pulse release.
# Immutable GitHub releases protect the tag and asset set and receive a signed
# release attestation. Both properties are required: an attestation check alone
# must not bless a release whose assets can still be replaced afterward.

set -euo pipefail

if [ "$#" -lt 2 ] || [ "$#" -gt 4 ]; then
    echo "Usage: $0 <tag> <owner/repo> [expected-release-id] [expected-source-sha]" >&2
    exit 1
fi

TAG="$1"
REPO="$2"
EXPECTED_RELEASE_ID="${3:-}"
EXPECTED_SOURCE_SHA="${4:-}"
ATTESTATION_ATTEMPTS="${PULSE_RELEASE_ATTESTATION_ATTEMPTS:-12}"
ATTESTATION_RETRY_DELAY="${PULSE_RELEASE_ATTESTATION_RETRY_DELAY:-5}"

if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-(rc|alpha|beta)\.[0-9]+)?$ ]]; then
    echo "Invalid release tag: ${TAG}" >&2
    exit 1
fi
if [[ ! "$REPO" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
    echo "Invalid GitHub repository: ${REPO}" >&2
    exit 1
fi
if [ -n "$EXPECTED_RELEASE_ID" ] && [[ ! "$EXPECTED_RELEASE_ID" =~ ^[0-9]+$ ]]; then
    echo "Invalid expected release id: ${EXPECTED_RELEASE_ID}" >&2
    exit 1
fi
if [ -n "$EXPECTED_SOURCE_SHA" ] && [[ ! "$EXPECTED_SOURCE_SHA" =~ ^[0-9a-f]{40}$ ]]; then
    echo "Invalid expected source SHA: ${EXPECTED_SOURCE_SHA}" >&2
    exit 1
fi
if [[ ! "$ATTESTATION_ATTEMPTS" =~ ^[1-9][0-9]*$ ]]; then
    echo "PULSE_RELEASE_ATTESTATION_ATTEMPTS must be a positive integer." >&2
    exit 1
fi
if [[ ! "$ATTESTATION_RETRY_DELAY" =~ ^[0-9]+$ ]]; then
    echo "PULSE_RELEASE_ATTESTATION_RETRY_DELAY must be a non-negative integer." >&2
    exit 1
fi

for command in gh jq; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "${command} is required to verify GitHub release integrity." >&2
        exit 1
    fi
done

release_json="$(mktemp)"
attestation_json="$(mktemp)"
cleanup() { rm -f "$release_json" "$attestation_json"; }
trap cleanup EXIT

gh api \
    -H 'Accept: application/vnd.github+json' \
    -H 'X-GitHub-Api-Version: 2026-03-10' \
    "repos/${REPO}/releases/tags/${TAG}" > "$release_json"

if ! jq -e \
    --arg tag "$TAG" \
    --arg expected_release_id "$EXPECTED_RELEASE_ID" \
    --arg expected_source_sha "$EXPECTED_SOURCE_SHA" \
    '.tag_name == $tag and
     .draft == false and
     (.published_at | type == "string" and length > 0) and
     .immutable == true and
     ($expected_release_id == "" or (.id | tostring) == $expected_release_id) and
     ($expected_source_sha == "" or .target_commitish == $expected_source_sha) and
     ([.assets[]? | select(
        .name == "release-activation.json" and
        .state == "uploaded" and
        (.size | type == "number" and . > 0) and
        (.digest | type == "string" and test("^sha256:[0-9a-f]{64}$"))
      )] | length == 1)' \
    "$release_json" >/dev/null; then
    jq -c \
        '{id, tag_name, target_commitish, draft, prerelease, immutable, published_at,
          activation_assets: [.assets[]? | select(.name == "release-activation.json") |
            {name, state, size, digest}]}' \
        "$release_json" >&2
    echo "Release ${TAG} is not an immutable published packet with one digest-bound activation marker." >&2
    exit 1
fi

verified=false
for attempt in $(seq 1 "$ATTESTATION_ATTEMPTS"); do
    if gh release verify "$TAG" --repo "$REPO" --format json > "$attestation_json"; then
        verified=true
        break
    fi
    if [ "$attempt" -lt "$ATTESTATION_ATTEMPTS" ]; then
        echo "Release attestation for ${TAG} is not verifiable yet (${attempt}/${ATTESTATION_ATTEMPTS}); retrying." >&2
        sleep "$ATTESTATION_RETRY_DELAY"
    fi
done

if [ "$verified" != true ]; then
    echo "GitHub release attestation verification failed for ${TAG}." >&2
    exit 1
fi
if ! jq -e 'type == "object" or type == "array"' "$attestation_json" >/dev/null; then
    echo "GitHub release attestation verification returned malformed JSON for ${TAG}." >&2
    exit 1
fi

release_id="$(jq -r '.id' "$release_json")"
source_sha="$(jq -r '.target_commitish' "$release_json")"
asset_count="$(jq -r '.assets | length' "$release_json")"
echo "[OK] GitHub release ${TAG} is immutable and attested: release_id=${release_id} source_sha=${source_sha} assets=${asset_count}."
