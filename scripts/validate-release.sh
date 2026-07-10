#!/usr/bin/env bash

# Pulse Release Validation Script
# Comprehensive artifact validation to prevent missing files/binaries in releases
#
# Usage: ./scripts/validate-release.sh <pulse-version> [image] [release-dir] [--skip-docker]
# Example: ./scripts/validate-release.sh 4.26.2
#          ./scripts/validate-release.sh 4.26.2 rcourtman/pulse:v4.26.2 release
#          ./scripts/validate-release.sh 4.26.2 --skip-docker

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
}

section() {
    echo ""
    echo -e "${BLUE}=== ${1} ===${NC}"
}

success() {
    echo -e "${GREEN}[✓]${NC} $*"
}

info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

with_network_blocked() {
    # Drop outbound traffic inside container by adding a reject route; avoids needing elevated host perms.
    # Caller supplies: container name, command...
    local container="$1"
    shift
    docker exec "$container" sh -c "ip route add blackhole 0.0.0.0/0 || true" && "$@"
}

check_tar_entries_nonempty() {
    local tarball="$1"
    shift
    local extract_dir
    extract_dir="$(mktemp -d "$tmp_root/tar-entries.XXXXXX")"

    if ! tar -xzf "$tarball" -C "$extract_dir" -- "$@"; then
        error "$(basename "$tarball") is missing one or more required entries"
        exit 1
    fi

    for entry in "$@"; do
        local extracted_path="$extract_dir/${entry#./}"
        if [ -L "$extracted_path" ]; then
            continue
        fi
        if [ ! -s "$extracted_path" ]; then
            error "$(basename "$tarball") missing or empty entry: $entry"
            exit 1
        fi
    done

    rm -rf "$extract_dir"
}

http_header_value() {
    local header_name="$1"
    local headers_file="$2"
    awk -v needle="$(printf '%s' "$header_name" | tr '[:upper:]' '[:lower:]')" '
        BEGIN { value = "" }
        {
            line = $0
            sub(/\r$/, "", line)
            sub(/^[[:space:]]+/, "", line)
            lower = tolower(line)
            if (index(lower, needle ":") == 1) {
                sub(/^[^:]*:[[:space:]]*/, "", line)
                value = line
            }
        }
        END { print value }
    ' "$headers_file"
}

validate_download_binary_headers() {
    local file_path="$1"
    local headers_path="$2"
    local label="$3"
    local checksum_header signature_header ssh_signature_header actual_checksum

    checksum_header="$(http_header_value "X-Checksum-Sha256" "$headers_path")"
    signature_header="$(http_header_value "X-Signature-Ed25519" "$headers_path")"
    ssh_signature_header="$(http_header_value "X-Signature-SSHSIG" "$headers_path")"

    if [ -z "$checksum_header" ]; then
        error "${label} missing X-Checksum-Sha256 header"
        exit 1
    fi
    if [ -z "$signature_header" ]; then
        error "${label} missing X-Signature-Ed25519 header"
        exit 1
    fi
    if [ -z "$ssh_signature_header" ]; then
        error "${label} missing X-Signature-SSHSIG header"
        exit 1
    fi

    actual_checksum="$(sha256sum "$file_path" | awk '{print $1}')"
    if [ "$actual_checksum" != "$checksum_header" ]; then
        error "${label} checksum header mismatch: expected ${checksum_header}, got ${actual_checksum}"
        exit 1
    fi
}

validate_download_script_headers() {
    local headers_path="$1"
    local label="$2"
    local signature_header ssh_signature_header

    signature_header="$(http_header_value "X-Signature-Ed25519" "$headers_path")"
    ssh_signature_header="$(http_header_value "X-Signature-SSHSIG" "$headers_path")"

    if [ -z "$signature_header" ]; then
        error "${label} missing X-Signature-Ed25519 header"
        exit 1
    fi
    if [ -z "$ssh_signature_header" ]; then
        error "${label} missing X-Signature-SSHSIG header"
        exit 1
    fi
}

host_can_execute_linux_amd64() {
    [ "$(uname -s)" = "Linux" ] || return 1
    case "$(uname -m)" in
        x86_64|amd64)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

docker_runtime_available() {
    [ "$SKIP_DOCKER" = false ] || return 1
    command -v docker >/dev/null 2>&1 || return 1
    docker info >/dev/null 2>&1 || return 1
}

validate_linux_amd64_version_with_docker() {
    local tarball="$1"

    docker run --rm -i \
        --platform linux/amd64 \
        --entrypoint /bin/sh \
        -e EXPECTED_TAG="$PULSE_TAG" \
        -e EXPECTED_VERSION="$PULSE_VERSION" \
        "$IMAGE" \
        -c 'set -euo pipefail
tmp="$(mktemp -d)"
tar -xzf - -C "$tmp"
version_output="$("$tmp/bin/pulse" version 2>/dev/null)"
printf "%s\n" "$version_output"
printf "%s\n" "$version_output" | grep -Fx "Pulse $EXPECTED_TAG" >/dev/null || {
    echo "Extracted pulse binary version mismatch: $version_output" >&2
    exit 1
}
grep -aF "$EXPECTED_TAG" "$tmp/bin/pulse" >/dev/null || {
    echo "Extracted pulse binary does not contain expected version string: $EXPECTED_TAG" >&2
    exit 1
}
grep -Fx "$EXPECTED_VERSION" "$tmp/VERSION" >/dev/null || {
    echo "Extracted VERSION file mismatch" >&2
    exit 1
}' < "$tarball"
}

if [ $# -lt 1 ]; then
    error "Usage: $0 <pulse-version> [image] [release-dir] [--skip-docker]"
    exit 1
fi

PULSE_VERSION=$1
PULSE_TAG="v${PULSE_VERSION}"
SKIP_DOCKER=false

# Parse arguments
shift
while [ $# -gt 0 ]; do
    case "$1" in
        --skip-docker)
            SKIP_DOCKER=true
            shift
            ;;
        *)
            if [ -z "${IMAGE:-}" ]; then
                IMAGE="$1"
            elif [ -z "${RELEASE_DIR:-}" ]; then
                RELEASE_DIR="$1"
            fi
            shift
            ;;
    esac
done

# Set defaults
IMAGE=${IMAGE:-"rcourtman/pulse:${PULSE_TAG}"}
RELEASE_DIR=${RELEASE_DIR:-"release"}

# Validate prerequisites
if [ "$SKIP_DOCKER" = false ]; then
    command -v docker >/dev/null || { error "docker is required (use --skip-docker to skip Docker validation)"; exit 1; }
fi
[ -d "$RELEASE_DIR" ] || { error "release dir not found: $RELEASE_DIR"; exit 1; }
RELEASE_DIR="$(cd "$RELEASE_DIR" && pwd)"

# Create temp directory for extractions
tmp_root=$(mktemp -d)
smoke_container=""
trap 'rm -rf "$tmp_root"; if [ -n "$smoke_container" ]; then docker rm -f "$smoke_container" >/dev/null 2>&1 || true; fi' EXIT

info "Validating Pulse $PULSE_TAG release artifacts"
info "Image: $IMAGE"
info "Release directory: $RELEASE_DIR"
echo ""

#=============================================================================
# DOCKER IMAGE VALIDATION
#=============================================================================
if [ "$SKIP_DOCKER" = false ]; then
    info "=== Docker Image Validation ==="

    # Validate VERSION file in container
    info "Checking VERSION file in Docker image..."
    docker run --rm --entrypoint /bin/sh -e EXPECTED_VERSION="$PULSE_VERSION" "$IMAGE" -c 'set -euo pipefail; for path in /VERSION /app/VERSION; do if [ -f "$path" ]; then actual=$(cat "$path" | tr -d "\r\n"); [ "$actual" = "$EXPECTED_VERSION" ] && exit 0 || { echo "VERSION mismatch at $path: expected=$EXPECTED_VERSION actual=$actual" >&2; exit 1; }; fi; done; echo "VERSION file not found in image" >&2; exit 1' || { error "VERSION file mismatch in Docker image"; exit 1; }
    success "VERSION file correct: $PULSE_VERSION"

    # Validate all required scripts exist and are executable
    info "Checking installer/uninstaller scripts in /opt/pulse/scripts/..."
    docker run --rm --entrypoint /bin/sh "$IMAGE" -c 'set -euo pipefail; cd /opt/pulse/scripts; required="install-container-agent.sh install-docker.sh install.sh"; for f in $required; do [ -f "$f" ] || { echo "missing script $f" >&2; exit 1; }; case "$f" in *.sh|*.ps1) [ -x "$f" ] || { echo "$f not executable" >&2; exit 1; };; esac; done; echo "All scripts present and executable"' || { error "Script validation failed"; exit 1; }
    success "All installer/uninstaller scripts present and executable"

    # Validate all required binaries exist and are non-empty
    info "Checking downloadable binaries in /opt/pulse/bin/..."
    docker run --rm --entrypoint /bin/sh "$IMAGE" -c 'set -euo pipefail; cd /opt/pulse/bin; required="pulse pulse-agent-linux-amd64 pulse-agent-linux-arm64 pulse-agent-linux-armv7 pulse-agent-linux-armv6 pulse-agent-linux-386 pulse-agent-darwin-amd64 pulse-agent-darwin-arm64 pulse-agent-windows-amd64.exe pulse-agent-windows-amd64 pulse-agent-windows-arm64.exe pulse-agent-windows-arm64 pulse-agent-windows-386.exe pulse-agent-windows-386 pulse-agent-freebsd-amd64 pulse-agent-freebsd-arm64"; for f in $required; do [ -e "$f" ] || { echo "missing binary $f" >&2; exit 1; }; [ -s "$f" ] || { echo "empty binary $f" >&2; exit 1; }; done; [ "$(readlink pulse-agent-windows-amd64)" = "pulse-agent-windows-amd64.exe" ] || { echo "unified agent windows amd64 symlink broken" >&2; exit 1; }; [ "$(readlink pulse-agent-windows-arm64)" = "pulse-agent-windows-arm64.exe" ] || { echo "unified agent windows arm64 symlink broken" >&2; exit 1; }; [ "$(readlink pulse-agent-windows-386)" = "pulse-agent-windows-386.exe" ] || { echo "unified agent windows 386 symlink broken" >&2; exit 1; }; echo "All binaries present"' || { error "Binary validation failed"; exit 1; }
    success "All downloadable binaries present"

    # Validate the arch-resolved /usr/local/bin/pulse-agent symlink. The helm
    # chart's agent workload (and `docker run rcourtman/pulse --entrypoint
    # /usr/local/bin/pulse-agent`) depend on this path; without it the chart
    # defaults to a non-existent image and `agent.enabled=true` hits
    # ImagePullBackOff.
    info "Validating /usr/local/bin/pulse-agent arch-resolved symlink..."
    docker run --rm --entrypoint /bin/sh "$IMAGE" -c 'set -euo pipefail; [ -L /usr/local/bin/pulse-agent ] || { echo "/usr/local/bin/pulse-agent is missing or not a symlink" >&2; exit 1; }; target=$(readlink /usr/local/bin/pulse-agent); case "$target" in /opt/pulse/bin/pulse-agent-linux-amd64|/opt/pulse/bin/pulse-agent-linux-arm64|/opt/pulse/bin/pulse-agent-linux-armv7) : ;; *) echo "/usr/local/bin/pulse-agent points at unexpected target: $target" >&2; exit 1 ;; esac; [ -x "$target" ] || { echo "/usr/local/bin/pulse-agent target is not executable" >&2; exit 1; }; echo "pulse-agent symlink resolves to $target"' || { error "/usr/local/bin/pulse-agent validation failed"; exit 1; }
    success "/usr/local/bin/pulse-agent symlink is arch-resolved and executable"

    # Validate version embedding in Docker image binaries
    info "Validating version embedding in Docker image binaries..."

    # Pulse server binary
    docker run --rm --entrypoint /app/pulse "$IMAGE" version 2>/dev/null | grep -Fx "Pulse $PULSE_TAG" >/dev/null || { error "Pulse server version mismatch"; exit 1; }
    success "Pulse server version: $PULSE_TAG"

    # Docker agent is embedded in the main pulse binary (check binary strings)
    docker run --rm --entrypoint /bin/sh -e EXPECTED_TAG="$PULSE_TAG" "$IMAGE" -c 'set -euo pipefail; grep -aF "$EXPECTED_TAG" /app/pulse >/dev/null' || { error "Docker agent version string not found"; exit 1; }
    success "Docker agent version embedded: $PULSE_TAG"

    # Unified agent binary
    docker run --rm --entrypoint /opt/pulse/bin/pulse-agent-linux-amd64 "$IMAGE" --version 2>/dev/null | grep -Fx "$PULSE_TAG" >/dev/null || { error "Unified agent version mismatch"; exit 1; }
    success "Unified agent version: $PULSE_TAG"

    # Smoke test download endpoints from a running container
    info "Running download endpoint smoke tests..."
    HOST_PORT=8765
    SMOKE_CONTAINER="pulse-download-smoke-$$"
    smoke_container="$SMOKE_CONTAINER"

    docker run -d --rm \
        --name "$SMOKE_CONTAINER" \
        -p "$HOST_PORT:7655" \
        -e PULSE_MOCK_MODE=true \
        -e PULSE_ALLOW_DOCKER_UPDATES=true \
        -e PULSE_AUTH_USER=admin \
        -e PULSE_AUTH_PASS=admin \
        "$IMAGE" >/dev/null

    for i in $(seq 1 30); do
        if curl -fsS "http://127.0.0.1:${HOST_PORT}/api/health" >/dev/null 2>&1; then
            break
        fi
        sleep 2
        if [ "$i" -eq 30 ]; then
            docker logs "$SMOKE_CONTAINER" || true
            error "Pulse container did not become healthy for download smoke tests"
            exit 1
        fi
    done

    download_matrix=(
        "linux amd64"
        "linux arm64"
        "linux armv7"
        "linux armv6"
        "linux 386"
        "darwin amd64"
        "darwin arm64"
        "windows amd64"
        "windows arm64"
        "windows 386"
    )

    for script_name in install.sh install.ps1; do
        url="http://127.0.0.1:${HOST_PORT}/${script_name}"
        tmp_file=$(mktemp)
        tmp_headers=$(mktemp)
        if ! curl -fsS -D "$tmp_headers" -o "$tmp_file" "$url"; then
            docker logs "$SMOKE_CONTAINER" || true
            error "Download failed for ${script_name}"
            exit 1
        fi
        if [ ! -s "$tmp_file" ]; then
            error "Downloaded empty ${script_name}"
            exit 1
        fi
        case "$script_name" in
            install.sh)
                if ! grep -q '^# Pulse Unified Agent Installer' "$tmp_file"; then
                    error "${script_name} endpoint did not return the unified agent installer"
                    exit 1
                fi
                if ! grep -q -- '--token-file' "$tmp_file"; then
                    error "${script_name} endpoint is missing agent installer token-file support"
                    exit 1
                fi
                ;;
            install.ps1)
                if ! grep -q '^# Pulse Unified Agent Installer (Windows)' "$tmp_file"; then
                    error "${script_name} endpoint did not return the unified agent installer"
                    exit 1
                fi
                if ! grep -q 'TokenFile' "$tmp_file"; then
                    error "${script_name} endpoint is missing agent installer token-file support"
                    exit 1
                fi
                ;;
        esac
        validate_download_script_headers "$tmp_headers" "${script_name}"
        rm -f "$tmp_file" "$tmp_headers"
    done
    success "Install script endpoints returned required signature headers"

    for entry in "${download_matrix[@]}"; do
        set -- $entry
        platform=$1
        arch=$2
        url="http://127.0.0.1:${HOST_PORT}/download/pulse-agent?arch=${platform}-${arch}"
        tmp_file=$(mktemp)
        tmp_headers=$(mktemp)
        if ! curl -fsS -D "$tmp_headers" -o "$tmp_file" "$url"; then
            docker logs "$SMOKE_CONTAINER" || true
            error "Download failed for $platform/$arch"
            exit 1
        fi
        if [ ! -s "$tmp_file" ]; then
            error "Downloaded empty binary for $platform/$arch"
            exit 1
        fi
        validate_download_binary_headers "$tmp_file" "$tmp_headers" "${platform}/${arch}"
        rm -f "$tmp_file" "$tmp_headers"
    done
    success "Download endpoints returned binaries with checksum and signature headers for all platforms/architectures"

    docker rm -f "$SMOKE_CONTAINER" >/dev/null 2>&1 || true
    smoke_container=""

    echo ""

    # Offline self-heal check: run with no outbound network and confirm download endpoint still serves binaries
    section "Offline self-heal smoke test"
    SMOKE_CONTAINER="pulse-offline-smoke-$$"
    smoke_container="$SMOKE_CONTAINER"
    docker run -d --rm \
        --name "$SMOKE_CONTAINER" \
        --network none \
        -e PULSE_MOCK_MODE=true \
        -e PULSE_ALLOW_DOCKER_UPDATES=true \
        -e PULSE_AUTH_USER=admin \
        -e PULSE_AUTH_PASS=admin \
        "$IMAGE" >/dev/null

    for i in $(seq 1 30); do
        if docker exec "$SMOKE_CONTAINER" wget -qO- http://127.0.0.1:7655/api/health >/dev/null 2>&1; then
            break
        fi
        sleep 2
        if [ "$i" -eq 30 ]; then
            docker logs "$SMOKE_CONTAINER" || true
            error "Pulse container did not become healthy for offline smoke tests"
            exit 1
        fi
    done

    offline_tmp=$(mktemp)
    offline_headers=$(mktemp)
    if ! docker exec "$SMOKE_CONTAINER" sh -c "headers=\$(mktemp); wget -qS -O- 'http://127.0.0.1:7655/download/pulse-agent?arch=linux-amd64' 2>\"\$headers\"; status=\$?; cat \"\$headers\" >&2; rm -f \"\$headers\"; exit \$status" > "$offline_tmp" 2> "$offline_headers"; then
        docker logs "$SMOKE_CONTAINER" || true
        error "Offline self-heal failed: download endpoint returned error with no outbound network"
        exit 1
    fi
    if [ ! -s "$offline_tmp" ]; then
        error "Offline self-heal failed: downloaded binary is empty"
        exit 1
    fi
    validate_download_binary_headers "$offline_tmp" "$offline_headers" "offline linux/amd64"
    rm -f "$offline_tmp" "$offline_headers"
    success "Offline self-heal: download endpoint works with checksum and signature headers without outbound network"

    docker rm -f "$SMOKE_CONTAINER" >/dev/null 2>&1 || true
    smoke_container=""

    echo ""
else
    warn "=== Skipping Docker Image Validation (--skip-docker flag provided) ==="
    echo ""
fi

#=============================================================================
# RELEASE TARBALL VALIDATION
#=============================================================================
info "=== Release Tarball Validation ==="

pushd "$RELEASE_DIR" >/dev/null

# Validate all expected release assets exist
info "Checking required release assets..."
# NOTE: Standalone binaries are NOT in GitHub releases
# They are only included in Docker images for /download/ endpoints
required_assets=(
    "install.sh"
    "checksums.txt"
    "pulse-v${PULSE_VERSION}-release.sbom.spdx.json"
    "pulse-v${PULSE_VERSION}.tar.gz"
    "pulse-v${PULSE_VERSION}-linux-amd64.tar.gz"
    "pulse-v${PULSE_VERSION}-linux-arm64.tar.gz"
    "pulse-v${PULSE_VERSION}-linux-armv7.tar.gz"
    "pulse-v${PULSE_VERSION}-linux-armv6.tar.gz"
    "pulse-v${PULSE_VERSION}-linux-386.tar.gz"
    "pulse-agent-v${PULSE_VERSION}-darwin-amd64.tar.gz"
    "pulse-agent-v${PULSE_VERSION}-darwin-arm64.tar.gz"
    "pulse-agent-v${PULSE_VERSION}-windows-amd64.zip"
    "pulse-agent-v${PULSE_VERSION}-windows-arm64.zip"
    "pulse-agent-v${PULSE_VERSION}-windows-386.zip"
    "pulse-agent-v${PULSE_VERSION}-freebsd-amd64.tar.gz"
    "pulse-agent-v${PULSE_VERSION}-freebsd-arm64.tar.gz"
)

missing_count=0
for asset in "${required_assets[@]}"; do
    if [ ! -e "$asset" ]; then
        error "Missing release asset: $asset"
        missing_count=$((missing_count + 1))
    fi
done

if [ $missing_count -gt 0 ]; then
    error "$missing_count required assets missing"
    exit 1
fi
success "All ${#required_assets[@]} required release assets present"

# Validate published install.sh is the Pulse SERVER installer.
# Across v6 rc.1 → rc.5 the rendered AGENT installer was published here by
# mistake, silently breaking the LXC quickstart, the in-product Update Pulse
# button, and the pulse-auto-update.sh systemd timer for 30 days before anyone
# noticed. This guard pins the asset identity so it cannot drift back.
info "Validating install.sh is the Pulse server installer..."
install_sh_path="install.sh"
if [ ! -s "$install_sh_path" ]; then
    error "install.sh is missing or empty"
    exit 1
fi
if ! grep -qE '^# Pulse Installer Script' "$install_sh_path"; then
    error "install.sh banner does not match the Pulse server installer"
    error "If this fires, build-release.sh is publishing the wrong file (likely the agent installer)"
    exit 1
fi
if ! grep -qE '^[[:space:]]*--version\)' "$install_sh_path"; then
    error "install.sh is missing the --version arg handler — required by pulse-auto-update.sh and the README quickstart"
    exit 1
fi
if grep -q 'Pulse Unified Agent Installer' "$install_sh_path"; then
    error "install.sh is the agent installer, not the server installer — releases must publish the root install.sh"
    exit 1
fi
# Smoke: actually invoke `bash install.sh --help` and confirm it prints the
# server-installer help text. Catches parse-time syntax breakage and confirms
# the script is structurally executable, not just textually correct.
install_help_output=$(bash "$install_sh_path" --help 2>&1 || true)
if ! echo "$install_help_output" | grep -qF "Install specific version (e.g."; then
    error "bash install.sh --help did not print the server installer's version-pinning help line"
    error "Help output captured: $install_help_output"
    exit 1
fi
success "install.sh is the Pulse server installer (handles --version, --help prints server help)"

# Validate tarball contents
section "Validating tarball contents"
tar_arches=(linux-amd64 linux-arm64 linux-armv7 linux-armv6 linux-386)
unified_agent_entries=(
    ./bin/pulse-agent-linux-amd64
    ./bin/pulse-agent-linux-arm64
    ./bin/pulse-agent-linux-armv7
    ./bin/pulse-agent-linux-armv6
    ./bin/pulse-agent-linux-386
    ./bin/pulse-agent-darwin-amd64
    ./bin/pulse-agent-darwin-arm64
    ./bin/pulse-agent-windows-amd64.exe
    ./bin/pulse-agent-windows-arm64.exe
    ./bin/pulse-agent-windows-386.exe
    ./bin/pulse-agent-windows-amd64
    ./bin/pulse-agent-windows-arm64
    ./bin/pulse-agent-windows-386
    ./bin/pulse-agent-freebsd-amd64
    ./bin/pulse-agent-freebsd-arm64
)
for arch in "${tar_arches[@]}"; do
    tarball="pulse-v${PULSE_VERSION}-${arch}.tar.gz"

    # Check binaries (note: tarballs use ./  prefix)
    if ! tar -tzf "$tarball" ./bin/pulse >/dev/null 2>&1; then
        error "$(basename $tarball) missing binaries"
        exit 1
    fi

    check_tar_entries_nonempty "$tarball" "${unified_agent_entries[@]}"

    # Check scripts
    tar -tzf "$tarball" ./scripts/install-container-agent.sh ./scripts/install-docker.sh ./scripts/install.sh >/dev/null 2>&1 || { error "$(basename $tarball) missing scripts"; exit 1; }

    # Check VERSION file
    tar -tzf "$tarball" ./VERSION >/dev/null 2>&1 || { error "$(basename $tarball) missing VERSION file"; exit 1; }
done
success "Platform-specific tarballs contain all required files (including cross-platform unified agents)"

# Validate universal tarball
section "Validating universal tarball"
tar -tzf "pulse-v${PULSE_VERSION}.tar.gz" ./VERSION >/dev/null 2>&1 || { error "Universal tarball missing VERSION file"; exit 1; }

# Validate universal tarball contains all agent binaries for download endpoint
info "Validating universal tarball contains all agent binaries..."
check_tar_entries_nonempty "pulse-v${PULSE_VERSION}.tar.gz" "${unified_agent_entries[@]}"
success "Universal tarball validated (includes cross-platform unified agents)"

# Validate macOS tarballs
tar -tzf "pulse-agent-v${PULSE_VERSION}-darwin-arm64.tar.gz" pulse-agent-darwin-arm64 >/dev/null 2>&1 || { error "macOS unified-agent tarball validation failed"; exit 1; }
success "macOS agent tarballs validated"

# Validate checksums.txt
info "Validating checksums..."
sha256sum -c checksums.txt >/dev/null 2>&1 || { error "checksums.txt validation failed"; exit 1; }
success "checksums.txt validated"

release_sbom="pulse-${PULSE_TAG}-release.sbom.spdx.json"
if ! grep -F "  ${release_sbom}" checksums.txt >/dev/null 2>&1; then
    error "checksums.txt is missing ${release_sbom}"
    exit 1
fi
success "Release SBOM is listed in checksums.txt"

# Validate release signature sidecars
info "Validating SSH signature sidecars..."
if [ ! -s "checksums.txt.sshsig" ]; then
    error "Missing or empty checksums.txt.sshsig"
    exit 1
fi

while IFS= read -r line; do
    checksum=$(echo "$line" | awk '{print $1}')
    filename=$(echo "$line" | awk '{print $2}')

    [ -n "$checksum" ] || continue
    [ -n "$filename" ] || continue

    if [ ! -s "${filename}.sshsig" ]; then
        error "Missing or empty ${filename}.sshsig"
        exit 1
    fi
done < checksums.txt
success "SSH signature sidecars validated"

# Actually run the README's documented verification step against install.sh.sshsig.
# The README ships a hardcoded ed25519 pubkey and tells customers to verify
# install.sh with it before running. Across v6 rc.2 → rc.5 (~20 days) the README
# pinned a stale key (Ds21c5...) that didn't match the actual pipeline signing
# key (MZd/...), so any customer who followed the secure-install path got
# "Could not verify signature" and aborted. This check extracts the README's
# pinned key and runs the exact verification command, so any drift between
# documented key and actual signing key fails the release.
info "Validating README pinned signature key matches install.sh.sshsig..."
readme_path="$REPO_ROOT/README.md"
if [ ! -f "$readme_path" ]; then
    error "README.md not found at $readme_path — cannot validate documented signature key"
    exit 1
fi
readme_signing_key=$(grep -oE "ssh-ed25519 [A-Za-z0-9+/=]+ pulse-installer" "$readme_path" | head -1)
if [ -z "$readme_signing_key" ]; then
    error "Could not extract ed25519 pulse-installer key from README.md secure-install snippet"
    exit 1
fi
if ! command -v ssh-keygen >/dev/null 2>&1; then
    error "ssh-keygen not found — required to validate the README-documented signature path"
    exit 1
fi
readme_allowed_signers=$(mktemp)
printf 'pulse-installer %s\n' "$readme_signing_key" > "$readme_allowed_signers"
if ! ssh-keygen -Y verify \
    -f "$readme_allowed_signers" \
    -I pulse-installer \
    -n pulse-install \
    -s install.sh.sshsig < install.sh >/dev/null 2>&1; then
    rm -f "$readme_allowed_signers"
    error "README's pinned signature key does not verify install.sh.sshsig"
    error "Customers who follow the README's secure-install ssh-keygen step will see 'Could not verify signature' and abort"
    error "Either update README.md/docs/INSTALL.md with the correct pulse-installer pubkey, or fix the release signing key"
    exit 1
fi
rm -f "$readme_allowed_signers"
success "README pinned signature key verifies install.sh.sshsig"

# Validate individual .sha256 files exist and match checksums.txt
info "Validating individual .sha256 files..."
while IFS= read -r line; do
    checksum=$(echo "$line" | awk '{print $1}')
    filename=$(echo "$line" | awk '{print $2}')

    [ -n "$checksum" ] || continue
    [ -n "$filename" ] || continue

    # Check .sha256 file exists
    if [ ! -f "${filename}.sha256" ]; then
        error "Missing ${filename}.sha256"
        exit 1
    fi

    # Check .sha256 file content matches checksums.txt
    sha256_content=$(cat "${filename}.sha256")
    expected_content="${checksum}  ${filename}"
    if [ "$sha256_content" != "$expected_content" ]; then
        error "${filename}.sha256 content mismatch"
        exit 1
    fi
done < checksums.txt
success "Individual .sha256 files validated"

popd >/dev/null

echo ""

#=============================================================================
# VERSION EMBEDDING VALIDATION (EXTRACTED TARBALL)
#=============================================================================
info "=== Version Embedding Validation (Extracted Binaries) ==="

# Extract linux-amd64 tarball for testing
linux_amd64_tarball="$RELEASE_DIR/pulse-v${PULSE_VERSION}-linux-amd64.tar.gz"
extract_dir="$tmp_root/linux-amd64"
mkdir -p "$extract_dir"
tar -xzf "$linux_amd64_tarball" -C "$extract_dir"

info "Testing extracted binaries from linux-amd64 tarball..."

# Ensure extracted pulse binary contains embedded version string metadata.
grep -aF "$PULSE_TAG" "$extract_dir/bin/pulse" >/dev/null || { error "Extracted pulse binary does not contain expected version string"; exit 1; }
success "Extracted pulse binary contains expected version string: $PULSE_TAG"

# Test VERSION file
grep -Fx "$PULSE_VERSION" "$extract_dir/VERSION" >/dev/null || { error "Extracted VERSION file mismatch"; exit 1; }
success "Extracted VERSION file: $PULSE_VERSION"

# Test Pulse server. The release tarball is linux/amd64, so macOS and arm64
# hosts cannot execute it directly. Use Docker when available, otherwise keep
# the static version checks above and make the skip explicit.
if host_can_execute_linux_amd64; then
    "$extract_dir/bin/pulse" version 2>/dev/null | grep -Fx "Pulse $PULSE_TAG" >/dev/null || { error "Extracted pulse binary version mismatch"; exit 1; }
    success "Extracted pulse binary executes on host: $PULSE_TAG"
elif docker_runtime_available; then
    validate_linux_amd64_version_with_docker "$linux_amd64_tarball" | grep -Fx "Pulse $PULSE_TAG" >/dev/null || { error "Extracted pulse binary Docker version check failed"; exit 1; }
    success "Extracted pulse binary executes under Docker linux/amd64: $PULSE_TAG"
else
    warn "Skipping extracted linux/amd64 pulse execution on $(uname -s)/$(uname -m); Docker runtime unavailable or --skip-docker was provided"
fi

echo ""

# NOTE: Standalone binary validation removed - they are NOT in GitHub releases
# They are only included in Docker images for /download/ endpoints

echo ""

#=============================================================================
# OPTIONAL: HELM CHART VALIDATION
#=============================================================================
if [ -f "$RELEASE_DIR/pulse-${PULSE_VERSION}.tgz" ]; then
    info "=== Helm Chart Validation ==="

    if command -v helm >/dev/null 2>&1; then
        # Extract and validate Helm chart
        helm_extract="$tmp_root/helm"
        mkdir -p "$helm_extract"
        tar -xzf "$RELEASE_DIR/pulse-${PULSE_VERSION}.tgz" -C "$helm_extract"

        # Validate Chart.yaml
        if [ -f "$helm_extract/pulse/Chart.yaml" ]; then
            chart_version=$(grep '^version:' "$helm_extract/pulse/Chart.yaml" | awk '{print $2}')
            app_version=$(grep '^appVersion:' "$helm_extract/pulse/Chart.yaml" | awk '{print $2}' | tr -d '"')

            if [ "$chart_version" = "$PULSE_VERSION" ]; then
                success "Helm chart version: $chart_version"
            else
                error "Helm chart version mismatch: expected=$PULSE_VERSION actual=$chart_version"
            fi

            if [ "$app_version" = "$PULSE_VERSION" ]; then
                success "Helm appVersion: $app_version"
            else
                error "Helm appVersion mismatch: expected=$PULSE_VERSION actual=$app_version"
            fi
        else
            warn "Helm Chart.yaml not found in extracted chart"
        fi
    else
        warn "Helm not installed, skipping chart validation"
    fi
else
    info "Helm chart not found (pulse-${PULSE_VERSION}.tgz) - skipping Helm validation"
fi

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║                                                            ║${NC}"
echo -e "${GREEN}║  ✓ RELEASE VALIDATION PASSED FOR ${PULSE_TAG}                    ║${NC}"
echo -e "${GREEN}║                                                            ║${NC}"
echo -e "${GREEN}║  All required artifacts, scripts, binaries, and version    ║${NC}"
echo -e "${GREEN}║  strings validated successfully.                           ║${NC}"
echo -e "${GREEN}║                                                            ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""
