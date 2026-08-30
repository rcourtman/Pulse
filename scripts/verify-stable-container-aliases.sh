#!/usr/bin/env bash

# Verify that every stable floating container alias still selects the exact
# image digest committed by release activation. Exact-version provenance is
# checked separately by verify-release-container-images.sh; this check covers
# the mutable discovery paths customers can continue to pull after promotion.

set -euo pipefail

if [ "$#" -lt 3 ] || [ "$#" -gt 4 ]; then
    echo "Usage: $0 <stable-tag> <server-digest> <control-plane-digest> [owner]" >&2
    exit 2
fi

TAG="$1"
SERVER_DIGEST="$2"
CONTROL_PLANE_DIGEST="$3"
OWNER="${4:-rcourtman}"
VERSION="${TAG#v}"

if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Invalid stable release tag: ${TAG}" >&2
    exit 1
fi
if [[ ! "$SERVER_DIGEST" =~ ^sha256:[0-9a-f]{64}$ ]]; then
    echo "Invalid server image digest: ${SERVER_DIGEST}" >&2
    exit 1
fi
if [[ ! "$CONTROL_PLANE_DIGEST" =~ ^sha256:[0-9a-f]{64}$ ]]; then
    echo "Invalid control-plane image digest: ${CONTROL_PLANE_DIGEST}" >&2
    exit 1
fi
if [[ ! "$OWNER" =~ ^[A-Za-z0-9_.-]+$ ]]; then
    echo "Invalid registry owner: ${OWNER}" >&2
    exit 1
fi

for command in docker jq; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "${command} is required to verify stable container aliases." >&2
        exit 1
    fi
done

IFS='.' read -r MAJOR MINOR _PATCH <<<"$VERSION"
ALIASES=("${MAJOR}.${MINOR}" "${MAJOR}" latest)

resolve_digest() {
    local reference="$1"
    local manifest
    manifest="$(docker buildx imagetools inspect "$reference" --format '{{json .Manifest}}')"
    jq -er '.digest | select(type == "string" and test("^sha256:[0-9a-f]{64}$"))' <<<"$manifest"
}

verify_aliases() {
    local image="$1"
    local expected_digest="$2"
    local registry alias reference observed_digest

    for registry in "docker.io/rcourtman" "ghcr.io/${OWNER}"; do
        for alias in "${ALIASES[@]}"; do
            reference="${registry}/${image}:${alias}"
            observed_digest="$(resolve_digest "$reference")"
            if [ "$observed_digest" != "$expected_digest" ]; then
                echo "Stable alias ${reference} resolved to ${observed_digest}, expected ${expected_digest}." >&2
                return 1
            fi
            echo "[OK] ${reference} -> ${expected_digest}" >&2
        done
    done
}

verify_aliases pulse "$SERVER_DIGEST"
verify_aliases pulse-control-plane "$CONTROL_PLANE_DIGEST"
