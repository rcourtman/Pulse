#!/usr/bin/env bash

# Resolve every exact-version public container tag to one digest per image and
# verify the GitHub build-provenance attestation for that digest. The two
# machine-readable output lines are intentionally stable so release workflows
# can carry the verified identities across the activation boundary.

set -euo pipefail

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
    echo "Usage: $0 <tag> <source-sha> [owner/repo]" >&2
    exit 2
fi

TAG="$1"
SOURCE_SHA="$2"
REPOSITORY="${3:-${GITHUB_REPOSITORY:-rcourtman/Pulse}}"
OWNER="${REPOSITORY%%/*}"
VERSION="${TAG#v}"
SIGNER_WORKFLOW="github.com/${REPOSITORY}/.github/workflows/publish-docker.yml"

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

for command in docker gh jq; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "${command} is required to verify release container images." >&2
        exit 1
    fi
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
"${SCRIPT_DIR}/require-safe-gh-attestation.sh"

resolve_digest() {
    local reference="$1"
    local manifest
    manifest="$(docker buildx imagetools inspect "$reference" --format '{{json .Manifest}}')"
    jq -er '.digest | select(type == "string" and test("^sha256:[0-9a-f]{64}$"))' <<<"$manifest"
}

verify_image() {
    local image="$1"
    local output_name="$2"
    local docker_name="docker.io/rcourtman/${image}"
    local ghcr_name="ghcr.io/${OWNER}/${image}"
    local docker_tag_digest docker_version_digest ghcr_tag_digest ghcr_version_digest

    docker_tag_digest="$(resolve_digest "${docker_name}:${TAG}")"
    docker_version_digest="$(resolve_digest "${docker_name}:${VERSION}")"
    ghcr_tag_digest="$(resolve_digest "${ghcr_name}:${TAG}")"
    ghcr_version_digest="$(resolve_digest "${ghcr_name}:${VERSION}")"

    if [ "$docker_tag_digest" != "$docker_version_digest" ] || \
       [ "$docker_tag_digest" != "$ghcr_tag_digest" ] || \
       [ "$docker_tag_digest" != "$ghcr_version_digest" ]; then
        echo "Exact-version ${image} tags do not resolve to one digest:" >&2
        printf '  %s:%s = %s\n' "$docker_name" "$TAG" "$docker_tag_digest" >&2
        printf '  %s:%s = %s\n' "$docker_name" "$VERSION" "$docker_version_digest" >&2
        printf '  %s:%s = %s\n' "$ghcr_name" "$TAG" "$ghcr_tag_digest" >&2
        printf '  %s:%s = %s\n' "$ghcr_name" "$VERSION" "$ghcr_version_digest" >&2
        return 1
    fi

    for subject in "$docker_name" "$ghcr_name"; do
        gh attestation verify "oci://${subject}@${docker_tag_digest}" \
            --repo "$REPOSITORY" \
            --bundle-from-oci \
            --signer-workflow "$SIGNER_WORKFLOW" \
            --source-digest "$SOURCE_SHA" \
            --deny-self-hosted-runners \
            --predicate-type https://slsa.dev/provenance/v1 \
            >/dev/null
    done

    printf '%s=%s\n' "$output_name" "$docker_tag_digest"
    echo "[OK] ${image} exact-version tags and provenance resolve to ${docker_tag_digest}." >&2
}

verify_image pulse server_digest
verify_image pulse-control-plane control_plane_digest
