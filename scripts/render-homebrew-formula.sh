#!/usr/bin/env bash
set -euo pipefail

# Render packaging/homebrew/pulse.rb.tmpl for a published Pulse release.
# Computes SHA-256 digests directly from the four pulse-cli release tarballs
# and fails closed if any archive is missing or any placeholder remains.

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "${SCRIPT_DIR}/.." && pwd)"
TEMPLATE_PATH="${REPO_ROOT}/packaging/homebrew/pulse.rb.tmpl"

VERSION_RAW=""
RELEASE_DIR=""
OUTPUT_PATH=""
ALLOW_PRERELEASE="false"

usage() {
    cat <<'EOF'
Usage: render-homebrew-formula.sh --version <vX.Y.Z|X.Y.Z> --release-dir <dir> --output <path> [--template <path>] [--allow-prerelease]
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --version)
            VERSION_RAW="${2:-}"
            shift 2
            ;;
        --release-dir)
            RELEASE_DIR="${2:-}"
            shift 2
            ;;
        --output)
            OUTPUT_PATH="${2:-}"
            shift 2
            ;;
        --template)
            TEMPLATE_PATH="${2:-}"
            shift 2
            ;;
        --allow-prerelease)
            ALLOW_PRERELEASE="true"
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "ERROR: unknown argument: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

if [[ -z "$VERSION_RAW" || -z "$RELEASE_DIR" || -z "$OUTPUT_PATH" ]]; then
    echo "ERROR: --version, --release-dir, and --output are required" >&2
    usage >&2
    exit 2
fi

VERSION="${VERSION_RAW#v}"
if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9.]+)?$ ]]; then
    echo "ERROR: invalid version: $VERSION_RAW" >&2
    exit 1
fi

if [[ "$VERSION" == *-* && "$ALLOW_PRERELEASE" != "true" ]]; then
    echo "ERROR: refusing prerelease version $VERSION without --allow-prerelease" >&2
    exit 1
fi

if [[ ! -f "$TEMPLATE_PATH" ]]; then
    echo "ERROR: formula template not found: $TEMPLATE_PATH" >&2
    exit 1
fi

if [[ ! -d "$RELEASE_DIR" ]]; then
    echo "ERROR: release directory not found: $RELEASE_DIR" >&2
    exit 1
fi

sha256_file() {
    local target="$1"
    if [[ ! -f "$target" ]]; then
        echo "ERROR: missing required CLI tarball: $target" >&2
        return 1
    fi
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$target" | awk '{print $1}'
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$target" | awk '{print $1}'
    else
        echo "ERROR: neither sha256sum nor shasum is available" >&2
        return 1
    fi
}

SHA256_DARWIN_ARM64=$(sha256_file "${RELEASE_DIR}/pulse-cli-v${VERSION}-darwin-arm64.tar.gz")
SHA256_DARWIN_AMD64=$(sha256_file "${RELEASE_DIR}/pulse-cli-v${VERSION}-darwin-amd64.tar.gz")
SHA256_LINUX_ARM64=$(sha256_file "${RELEASE_DIR}/pulse-cli-v${VERSION}-linux-arm64.tar.gz")
SHA256_LINUX_AMD64=$(sha256_file "${RELEASE_DIR}/pulse-cli-v${VERSION}-linux-amd64.tar.gz")

for digest in "$SHA256_DARWIN_ARM64" "$SHA256_DARWIN_AMD64" "$SHA256_LINUX_ARM64" "$SHA256_LINUX_AMD64"; do
    if [[ ! "$digest" =~ ^[0-9a-f]{64}$ ]]; then
        echo "ERROR: invalid SHA-256 digest computed: $digest" >&2
        exit 1
    fi
done

OUTPUT_DIR="$(dirname -- "$OUTPUT_PATH")"
mkdir -p "$OUTPUT_DIR"
TMP_OUTPUT=$(mktemp "${OUTPUT_DIR}/.pulse-formula.XXXXXX")
trap 'rm -f -- "$TMP_OUTPUT"' EXIT

sed \
    -e "s/{{VERSION}}/${VERSION}/g" \
    -e "s/{{SHA256_DARWIN_ARM64}}/${SHA256_DARWIN_ARM64}/g" \
    -e "s/{{SHA256_DARWIN_AMD64}}/${SHA256_DARWIN_AMD64}/g" \
    -e "s/{{SHA256_LINUX_ARM64}}/${SHA256_LINUX_ARM64}/g" \
    -e "s/{{SHA256_LINUX_AMD64}}/${SHA256_LINUX_AMD64}/g" \
    "$TEMPLATE_PATH" > "$TMP_OUTPUT"

if grep -q '{{' "$TMP_OUTPUT"; then
    echo "ERROR: unreplaced placeholder left in rendered Homebrew formula" >&2
    exit 1
fi

mv -f "$TMP_OUTPUT" "$OUTPUT_PATH"
trap - EXIT
echo "Rendered Homebrew formula for Pulse v${VERSION} -> ${OUTPUT_PATH}"
