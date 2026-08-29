#!/usr/bin/env bash

# Fail closed unless GitHub confirms that future releases in this repository
# will become immutable when a staged draft is published. The endpoint requires
# repository Administration (read), so callers must supply an explicit token
# with that narrow read capability rather than treating an anonymous 404 as a
# disabled setting.

set -euo pipefail

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <owner/repo>" >&2
    exit 1
fi

REPO="$1"
if [[ ! "$REPO" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
    echo "Invalid GitHub repository: ${REPO}" >&2
    exit 1
fi
if [ -z "${GH_TOKEN:-}" ]; then
    echo "GH_TOKEN with repository Administration (read) is required to prove release immutability." >&2
    exit 1
fi

for command in gh jq; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "${command} is required to check GitHub release immutability." >&2
        exit 1
    fi
done

setting_json="$(mktemp)"
cleanup() {
    rm -f "$setting_json"
}
trap cleanup EXIT

if ! gh api \
    -H 'Accept: application/vnd.github+json' \
    -H 'X-GitHub-Api-Version: 2026-03-10' \
    "repos/${REPO}/immutable-releases" > "$setting_json"; then
    echo "GitHub did not confirm immutable releases for ${REPO}; the setting may be disabled or the token may lack Administration (read)." >&2
    exit 1
fi

if ! jq -e \
    '.enabled == true and (.enforced_by_owner | type == "boolean")' \
    "$setting_json" >/dev/null; then
    jq -c '{enabled, enforced_by_owner}' "$setting_json" >&2 || true
    echo "Immutable releases are not enabled for ${REPO}; refusing to cross the publication boundary." >&2
    exit 1
fi

enforced_by_owner="$(jq -r '.enforced_by_owner // false' "$setting_json")"
echo "[OK] GitHub immutable releases are enabled for ${REPO} (enforced_by_owner=${enforced_by_owner})."
