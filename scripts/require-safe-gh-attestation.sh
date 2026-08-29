#!/usr/bin/env bash

# Refuse to make release decisions with GitHub CLI versions whose attestation
# identity matching is known to be unsafe. GitHub CLI 2.97.0 escaped repository
# and workflow names before constructing the certificate matcher; older
# versions can accept a lookalike signer for a literal --signer-workflow policy.

set -euo pipefail

readonly MINIMUM_GH_VERSION="2.97.0"

if ! version_output="$(gh version 2>/dev/null | head -n 1)"; then
    echo "Unable to run the GitHub CLI required for safe attestation verification." >&2
    exit 1
fi
if ! [[ "$version_output" =~ ^gh\ version\ ([0-9]+\.[0-9]+\.[0-9]+) ]]; then
    echo "Unable to determine the GitHub CLI version required for safe attestation verification." >&2
    exit 1
fi
actual_version="${BASH_REMATCH[1]}"

version_at_least() {
    local actual="$1"
    local required="$2"
    local actual_major actual_minor actual_patch
    local required_major required_minor required_patch

    IFS=. read -r actual_major actual_minor actual_patch <<<"$actual"
    IFS=. read -r required_major required_minor required_patch <<<"$required"

    (( actual_major > required_major )) ||
        { (( actual_major == required_major )) && (( actual_minor > required_minor )); } ||
        { (( actual_major == required_major )) && (( actual_minor == required_minor )) &&
          (( actual_patch >= required_patch )); }
}

if ! version_at_least "$actual_version" "$MINIMUM_GH_VERSION"; then
    echo "GitHub CLI ${actual_version} is too old for release attestation policy enforcement; ${MINIMUM_GH_VERSION} or newer is required." >&2
    exit 1
fi

echo "[OK] GitHub CLI ${actual_version} satisfies the safe attestation verifier floor (${MINIMUM_GH_VERSION})." >&2
