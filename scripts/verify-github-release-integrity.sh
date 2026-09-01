#!/usr/bin/env bash

# Verify GitHub's post-publication integrity boundary for a Pulse release.
# Immutable GitHub releases protect the tag and asset set and receive a signed
# release attestation. Both properties are required: an attestation check alone
# must not bless a release whose assets can still be replaced afterward.

set -euo pipefail

if [ "$#" -lt 4 ] || [ "$#" -gt 5 ]; then
    echo "Usage: $0 <tag> <owner/repo> <expected-release-id> <expected-source-sha> [activation-asset]" >&2
    exit 1
fi

TAG="$1"
REPO="$2"
EXPECTED_RELEASE_ID="$3"
EXPECTED_SOURCE_SHA="$4"
SUPPLIED_ACTIVATION_ASSET="${5:-}"
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
if [[ ! "$EXPECTED_RELEASE_ID" =~ ^[0-9]+$ ]]; then
    echo "Invalid expected release id: ${EXPECTED_RELEASE_ID}" >&2
    exit 1
fi
if [[ ! "$EXPECTED_SOURCE_SHA" =~ ^[0-9a-f]{40}$ ]]; then
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

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
"${SCRIPT_DIR}/require-safe-gh-attestation.sh"

release_json="$(mktemp)"
attestation_json="$(mktemp)"
activation_dir="$(mktemp -d)"
activation_asset="${activation_dir}/release-activation.json"
checksums_asset="${activation_dir}/checksums.txt"
provenance_asset="${activation_dir}/release-build-provenance.sigstore.json"
legacy_signer_workflow="github.com/${REPO}/.github/workflows/create-release.yml"
candidate_signer_workflow="github.com/${REPO}/.github/workflows/build-release-candidate.yml"
cleanup() {
    rm -f "$release_json" "$attestation_json"
    rm -rf "$activation_dir"
}
trap cleanup EXIT

# A promotion workflow must verify the exact marker it will parse, not a
# second download that merely has the same release filename. Copy supplied
# bytes under the canonical asset name because `gh release verify-asset`
# resolves the attested subject by the local basename. Callers that do not
# consume a marker retain the historical download-and-verify behavior.
if [ -n "$SUPPLIED_ACTIVATION_ASSET" ]; then
    if [ ! -f "$SUPPLIED_ACTIVATION_ASSET" ] || [ ! -s "$SUPPLIED_ACTIVATION_ASSET" ]; then
        echo "Supplied release activation asset is not a non-empty regular file: ${SUPPLIED_ACTIVATION_ASSET}" >&2
        exit 1
    fi
    cp -- "$SUPPLIED_ACTIVATION_ASSET" "$activation_asset"
fi

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

# New release candidates carry the exact Sigstore bundle created by the hosted
# candidate builder. Existing immutable releases predate that asset and retain
# their publication-workflow provenance, so continuity checks remain valid
# until the next stable packet is activated.
provenance_asset_count="$(
    jq '[.assets[]? | select(.name == "release-build-provenance.sigstore.json")] | length' \
        "$release_json"
)"
if [ "$provenance_asset_count" -gt 1 ]; then
    echo "GitHub release ${TAG} contains duplicate portable provenance assets." >&2
    exit 1
fi
if [ "$provenance_asset_count" = 1 ] && ! jq -e \
    '[.assets[]? | select(
       .name == "release-build-provenance.sigstore.json" and
       .state == "uploaded" and
       (.size | type == "number" and . > 0) and
       (.digest | type == "string" and test("^sha256:[0-9a-f]{64}$"))
     )] | length == 1' \
    "$release_json" >/dev/null; then
    echo "GitHub release ${TAG} has an invalid portable provenance asset." >&2
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

# Release verification proves that GitHub signed the immutable packet. Bind the
# activation marker that convergence consumes to that packet as a separate
# proof: verify-asset checks the downloaded bytes' digest against the signed
# release attestation rather than trusting filename and JSON identity alone.
downloaded=false
for attempt in $(seq 1 "$ATTESTATION_ATTEMPTS"); do
    # A previous attempt can leave any asset behind after a partial
    # download. Clear them because gh release download refuses to overwrite
    # existing files unless explicitly told to do so.
    rm -f "$checksums_asset" "$provenance_asset"
    if [ -z "$SUPPLIED_ACTIVATION_ASSET" ]; then
        rm -f "$activation_asset"
    fi
    download_args=(
        "$TAG"
        --repo "$REPO"
        --pattern checksums.txt
        --dir "$activation_dir"
    )
    if [ -z "$SUPPLIED_ACTIVATION_ASSET" ]; then
        download_args+=(--pattern release-activation.json)
    fi
    if [ "$provenance_asset_count" = 1 ]; then
        download_args+=(--pattern release-build-provenance.sigstore.json)
    fi
    if gh release download "${download_args[@]}" && \
       [ -s "$activation_asset" ] && \
       [ -s "$checksums_asset" ] && \
       { [ "$provenance_asset_count" = 0 ] || [ -s "$provenance_asset" ]; }; then
        downloaded=true
        break
    fi
    if [ "$attempt" -lt "$ATTESTATION_ATTEMPTS" ]; then
        echo "Activation asset for ${TAG} is not downloadable yet (${attempt}/${ATTESTATION_ATTEMPTS}); retrying." >&2
        sleep "$ATTESTATION_RETRY_DELAY"
    fi
done
if [ "$downloaded" != true ]; then
    echo "GitHub release activation asset download failed for ${TAG}." >&2
    exit 1
fi

if ! gh release verify-asset "$TAG" "$activation_asset" \
    --repo "$REPO" --format json > "$attestation_json"; then
    echo "GitHub release activation asset verification failed for ${TAG}." >&2
    exit 1
fi
if ! jq -e 'type == "object" or type == "array"' "$attestation_json" >/dev/null; then
    echo "GitHub release activation asset verification returned malformed JSON for ${TAG}." >&2
    exit 1
fi

# checksums.txt is the transitive identity of the executable release packet:
# every primary binary, archive, installer, chart, and SBOM is named and
# digest-bound there, while their detached signatures are checked separately.
# First bind those exact checksum bytes to the immutable release, then require
# build provenance from the one workflow and source commit authorized to
# assemble the packet. Repository-level provenance alone is intentionally not
# sufficient because other workflows in this repository can issue attestations.
if ! gh release verify-asset "$TAG" "$checksums_asset" \
    --repo "$REPO" --format json > "$attestation_json"; then
    echo "GitHub release checksum manifest verification failed for ${TAG}." >&2
    exit 1
fi
if ! jq -e 'type == "object" or type == "array"' "$attestation_json" >/dev/null; then
    echo "GitHub release checksum manifest verification returned malformed JSON for ${TAG}." >&2
    exit 1
fi
signer_workflow="$legacy_signer_workflow"
bundle_args=()
if [ "$provenance_asset_count" = 1 ]; then
    if ! gh release verify-asset "$TAG" "$provenance_asset" \
        --repo "$REPO" --format json > "$attestation_json"; then
        echo "GitHub release portable provenance asset verification failed for ${TAG}." >&2
        exit 1
    fi
    if ! jq -e 'type == "object" or type == "array"' "$attestation_json" >/dev/null; then
        echo "GitHub release portable provenance asset verification returned malformed JSON for ${TAG}." >&2
        exit 1
    fi
    signer_workflow="$candidate_signer_workflow"
    bundle_args=(--bundle "$provenance_asset")
fi

if ! gh attestation verify "$checksums_asset" \
    --repo "$REPO" \
    --signer-workflow "$signer_workflow" \
    --source-digest "$EXPECTED_SOURCE_SHA" \
    --deny-self-hosted-runners \
    --predicate-type https://slsa.dev/provenance/v1 \
    "${bundle_args[@]}" \
    >/dev/null; then
    echo "Release checksum manifest build provenance verification failed for ${TAG}." >&2
    exit 1
fi

release_id="$(jq -r '.id' "$release_json")"
source_sha="$(jq -r '.target_commitish' "$release_json")"
asset_count="$(jq -r '.assets | length' "$release_json")"
if [ "$provenance_asset_count" = 1 ]; then
    provenance_status="portable candidate-build provenance"
else
    provenance_status="legacy publication provenance"
fi
echo "[OK] GitHub release ${TAG} is immutable, release-attested, activation-asset-bound, and build-provenance-bound (${provenance_status}): release_id=${release_id} source_sha=${source_sha} assets=${asset_count}."
