#!/usr/bin/env bash

# Pulse Release Validation Script
# Comprehensive artifact validation to prevent missing files/binaries in releases
#
# Usage: ./scripts/validate-release.sh <pulse-version> [image] [release-dir] [--skip-docker]
# Example: ./scripts/validate-release.sh 4.26.2
#          ./scripts/validate-release.sh 4.26.2 rcourtman/pulse:v4.26.2 release
#          ./scripts/validate-release.sh 4.26.2 --skip-docker

set -euo pipefail

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
    for entry in "$@"; do
        if ! tar -tzf "$tarball" "$entry" >/dev/null 2>&1; then
            error "$(basename "$tarball") missing entry: $entry"
            exit 1
        fi
        # Examine type; skip size enforcement for symlinks
        local type
        type=$(tar -tvf "$tarball" "$entry" 2>/dev/null | awk 'NR==1 {print substr($0,1,1)}')
        if [ "$type" = "l" ]; then
            continue
        fi
        local size
        size=$(tar -xOf "$tarball" "$entry" 2>/dev/null | wc -c | tr -d '[:space:]')
        if [ -z "$size" ] || [ "$size" -le 0 ]; then
            error "$(basename "$tarball") missing or empty entry: $entry"
            exit 1
        fi
    done
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
    docker run --rm --entrypoint /bin/sh "$IMAGE" -c 'set -euo pipefail; cd /opt/pulse/scripts; required="install-docker-agent.sh install-container-agent.sh install-host-agent.ps1 uninstall-host-agent.sh uninstall-host-agent.ps1 install-docker.sh install.sh"; for f in $required; do [ -f "$f" ] || { echo "missing script $f" >&2; exit 1; }; case "$f" in *.sh|*.ps1) [ -x "$f" ] || { echo "$f not executable" >&2; exit 1; };; esac; done; echo "All scripts present and executable"' || { error "Script validation failed"; exit 1; }
    success "All installer/uninstaller scripts present and executable"

    # Validate all required binaries exist and are non-empty
    info "Checking downloadable binaries in /opt/pulse/bin/..."
    docker run --rm --entrypoint /bin/sh "$IMAGE" -c 'set -euo pipefail; cd /opt/pulse/bin; required="pulse pulse-docker-agent pulse-docker-agent-linux-amd64 pulse-docker-agent-linux-arm64 pulse-docker-agent-linux-armv7 pulse-docker-agent-linux-armv6 pulse-docker-agent-linux-386 pulse-host-agent-linux-amd64 pulse-host-agent-linux-arm64 pulse-host-agent-linux-armv7 pulse-host-agent-linux-armv6 pulse-host-agent-linux-386 pulse-host-agent-darwin-amd64 pulse-host-agent-darwin-arm64 pulse-host-agent-windows-amd64.exe pulse-host-agent-windows-amd64 pulse-host-agent-windows-arm64.exe pulse-host-agent-windows-arm64 pulse-host-agent-windows-386.exe pulse-host-agent-windows-386 pulse-agent-linux-amd64 pulse-agent-linux-arm64 pulse-agent-linux-armv7 pulse-agent-linux-armv6 pulse-agent-linux-386 pulse-agent-darwin-amd64 pulse-agent-darwin-arm64 pulse-agent-windows-amd64.exe pulse-agent-windows-amd64 pulse-agent-windows-arm64.exe pulse-agent-windows-arm64 pulse-agent-windows-386.exe pulse-agent-windows-386"; for f in $required; do [ -e "$f" ] || { echo "missing binary $f" >&2; exit 1; }; [ -s "$f" ] || { echo "empty binary $f" >&2; exit 1; }; done; [ "$(readlink pulse-host-agent-windows-amd64)" = "pulse-host-agent-windows-amd64.exe" ] || { echo "windows amd64 symlink broken" >&2; exit 1; }; [ "$(readlink pulse-host-agent-windows-arm64)" = "pulse-host-agent-windows-arm64.exe" ] || { echo "windows arm64 symlink broken" >&2; exit 1; }; [ "$(readlink pulse-host-agent-windows-386)" = "pulse-host-agent-windows-386.exe" ] || { echo "windows 386 symlink broken" >&2; exit 1; }; [ "$(readlink pulse-agent-windows-amd64)" = "pulse-agent-windows-amd64.exe" ] || { echo "unified agent windows amd64 symlink broken" >&2; exit 1; }; [ "$(readlink pulse-agent-windows-arm64)" = "pulse-agent-windows-arm64.exe" ] || { echo "unified agent windows arm64 symlink broken" >&2; exit 1; }; [ "$(readlink pulse-agent-windows-386)" = "pulse-agent-windows-386.exe" ] || { echo "unified agent windows 386 symlink broken" >&2; exit 1; }; echo "All binaries present"' || { error "Binary validation failed"; exit 1; }
    success "All downloadable binaries present"

    # Validate version embedding in Docker image binaries
    info "Validating version embedding in Docker image binaries..."

    # Pulse server binary
    docker run --rm --entrypoint /app/pulse "$IMAGE" version 2>/dev/null | grep -Fx "Pulse $PULSE_TAG" >/dev/null || { error "Pulse server version mismatch"; exit 1; }
    success "Pulse server version: $PULSE_TAG"

    # Host agent binary
    docker run --rm --entrypoint /opt/pulse/bin/pulse-host-agent-linux-amd64 "$IMAGE" --version 2>/dev/null | grep -Fx "$PULSE_TAG" >/dev/null || { error "Host agent version mismatch"; exit 1; }
    success "Host agent version: $PULSE_TAG"

    # Docker agent binary (no CLI flag, check binary strings)
    docker run --rm --entrypoint /bin/sh -e EXPECTED_TAG="$PULSE_TAG" "$IMAGE" -c 'set -euo pipefail; grep -aF "$EXPECTED_TAG" /opt/pulse/bin/pulse-docker-agent-linux-amd64 >/dev/null' || { error "Docker agent version string not found"; exit 1; }
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

    for entry in "${download_matrix[@]}"; do
        set -- $entry
        platform=$1
        arch=$2
        url="http://127.0.0.1:${HOST_PORT}/download/pulse-host-agent?platform=${platform}&arch=${arch}"
        tmp_file=$(mktemp)
        if ! curl -fsS -o "$tmp_file" "$url"; then
            docker logs "$SMOKE_CONTAINER" || true
            error "Download failed for $platform/$arch"
            exit 1
        fi
        if [ ! -s "$tmp_file" ]; then
            error "Downloaded empty binary for $platform/$arch"
            exit 1
        fi
        rm -f "$tmp_file"
    done
    success "Download endpoints returned binaries for all platforms/architectures"

    checksum_url="http://127.0.0.1:${HOST_PORT}/download/pulse-host-agent.sha256?platform=linux&arch=amd64"
    checksum_tmp=$(mktemp)
    if curl -fsS -o "$checksum_tmp" "$checksum_url"; then
        if ! grep -Eq '^[0-9a-f]{64}$' "$checksum_tmp"; then
            error "Invalid checksum response from $checksum_url"
            exit 1
        fi
        success "Checksum endpoint responded with SHA256"
    else
        warn "Checksum endpoint unavailable (non-blocking): $checksum_url"
    fi
    rm -f "$checksum_tmp"

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
    if ! docker exec "$SMOKE_CONTAINER" wget -qO- "http://127.0.0.1:7655/download/pulse-host-agent?platform=linux&arch=amd64" > "$offline_tmp"; then
        docker logs "$SMOKE_CONTAINER" || true
        error "Offline self-heal failed: download endpoint returned error with no outbound network"
        exit 1
    fi
    if [ ! -s "$offline_tmp" ]; then
        error "Offline self-heal failed: downloaded binary is empty"
        exit 1
    fi
    rm -f "$offline_tmp"
    success "Offline self-heal: download endpoint works without outbound network"

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
# NOTE: Linux host-agent binaries are in the main pulse tarballs, not separate archives
required_assets=(
    "install.sh"
    "checksums.txt"
    "host-agent-manifest.json"
    "pulse-v${PULSE_VERSION}.tar.gz"
    "pulse-v${PULSE_VERSION}-linux-amd64.tar.gz"
    "pulse-v${PULSE_VERSION}-linux-arm64.tar.gz"
    "pulse-v${PULSE_VERSION}-linux-armv7.tar.gz"
    "pulse-v${PULSE_VERSION}-linux-armv6.tar.gz"
    "pulse-v${PULSE_VERSION}-linux-386.tar.gz"
    "pulse-host-agent-v${PULSE_VERSION}-darwin-amd64.tar.gz"
    "pulse-host-agent-v${PULSE_VERSION}-darwin-arm64.tar.gz"
    "pulse-host-agent-v${PULSE_VERSION}-windows-amd64.zip"
    "pulse-host-agent-v${PULSE_VERSION}-windows-arm64.zip"
    "pulse-host-agent-v${PULSE_VERSION}-windows-386.zip"
    "pulse-agent-v${PULSE_VERSION}-darwin-amd64.tar.gz"
    "pulse-agent-v${PULSE_VERSION}-darwin-arm64.tar.gz"
    "pulse-agent-v${PULSE_VERSION}-windows-amd64.zip"
    "pulse-agent-v${PULSE_VERSION}-windows-arm64.zip"
    "pulse-agent-v${PULSE_VERSION}-windows-386.zip"
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

# Validate host-agent manifest matches expected set
section "Validating host-agent manifest"
host_agent_manifest="host-agent-manifest.json"
python3 - "$host_agent_manifest" "$PULSE_VERSION" <<'EOF' || { error "Host-agent manifest validation failed"; exit 1; }
import json
import sys
import os

manifest_path = sys.argv[1]
version = sys.argv[2]

with open(manifest_path, "r", encoding="utf-8") as handle:
    manifest = json.load(handle)

expected_agents = {
    "pulse-host-agent-linux-amd64",
    "pulse-host-agent-linux-arm64",
    "pulse-host-agent-linux-armv7",
    "pulse-host-agent-linux-armv6",
    "pulse-host-agent-linux-386",
    "pulse-host-agent-darwin-amd64",
    "pulse-host-agent-darwin-arm64",
    "pulse-host-agent-windows-amd64.exe",
    "pulse-host-agent-windows-arm64.exe",
    "pulse-host-agent-windows-386.exe",
    "pulse-host-agent-windows-amd64",
    "pulse-host-agent-windows-arm64",
    "pulse-host-agent-windows-386",
}

def check_set(name, found):
    missing = expected_agents - set(found)
    extra = set(found) - expected_agents
    if missing or extra:
        msg = []
        if missing:
            msg.append(f"{name} missing: {sorted(missing)}")
        if extra:
            msg.append(f"{name} unexpected: {sorted(extra)}")
        print(" ; ".join(msg))
        return False
    return True

ok = True

if manifest.get("version") != version:
    print(f"Manifest version mismatch: expected {version}, got {manifest.get('version')}")
    ok = False

universal = manifest.get("universal", [])
if not check_set("universal", universal):
    ok = False

for arch in ["linux-amd64","linux-arm64","linux-armv7","linux-armv6","linux-386"]:
    found = manifest.get("tarballs", {}).get(arch)
    if found is None:
        print(f"Missing tarball entry in manifest for {arch}")
        ok = False
        continue
    if not check_set(arch, found):
        ok = False

if not ok:
    sys.exit(1)
EOF
success "Host-agent manifest matches expected platform/arch matrix"

# Validate tarball contents
section "Validating tarball contents"
tar_arches=(linux-amd64 linux-arm64 linux-armv7 linux-armv6 linux-386)
host_agent_entries=(
    ./bin/pulse-host-agent-linux-amd64
    ./bin/pulse-host-agent-linux-arm64
    ./bin/pulse-host-agent-linux-armv7
    ./bin/pulse-host-agent-linux-armv6
    ./bin/pulse-host-agent-linux-386
    ./bin/pulse-host-agent-darwin-amd64
    ./bin/pulse-host-agent-darwin-arm64
    ./bin/pulse-host-agent-windows-amd64.exe
    ./bin/pulse-host-agent-windows-arm64.exe
    ./bin/pulse-host-agent-windows-386.exe
    ./bin/pulse-host-agent-windows-amd64
    ./bin/pulse-host-agent-windows-arm64
    ./bin/pulse-host-agent-windows-386
)
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
)
for arch in "${tar_arches[@]}"; do
    tarball="pulse-v${PULSE_VERSION}-${arch}.tar.gz"

    # Check binaries (note: tarballs use ./  prefix)
    if ! tar -tzf "$tarball" ./bin/pulse ./bin/pulse-docker-agent ./bin/pulse-host-agent >/dev/null 2>&1; then
        error "$(basename $tarball) missing binaries"
        exit 1
    fi

    check_tar_entries_nonempty "$tarball" "${host_agent_entries[@]}"
    check_tar_entries_nonempty "$tarball" "${unified_agent_entries[@]}"

    # Check scripts
    tar -tzf "$tarball" ./scripts/install-docker-agent.sh ./scripts/install-container-agent.sh ./scripts/install-host-agent.ps1 ./scripts/uninstall-host-agent.sh ./scripts/uninstall-host-agent.ps1 ./scripts/install-docker.sh ./scripts/install.sh >/dev/null 2>&1 || { error "$(basename $tarball) missing scripts"; exit 1; }

    # Check VERSION file
    tar -tzf "$tarball" ./VERSION >/dev/null 2>&1 || { error "$(basename $tarball) missing VERSION file"; exit 1; }
done
success "Platform-specific tarballs contain all required files (including cross-platform host agents)"

# Validate universal tarball
section "Validating universal tarball"
tar -tzf "pulse-v${PULSE_VERSION}.tar.gz" ./VERSION >/dev/null 2>&1 || { error "Universal tarball missing VERSION file"; exit 1; }

# Validate universal tarball contains all agent binaries for download endpoint
info "Validating universal tarball contains all agent binaries..."
check_tar_entries_nonempty "pulse-v${PULSE_VERSION}.tar.gz" "${host_agent_entries[@]}"
check_tar_entries_nonempty "pulse-v${PULSE_VERSION}.tar.gz" "${unified_agent_entries[@]}"
success "Universal tarball validated (includes cross-platform host and unified agents)"

# Validate macOS tarballs
tar -tzf "pulse-host-agent-v${PULSE_VERSION}-darwin-arm64.tar.gz" pulse-host-agent-darwin-arm64 >/dev/null 2>&1 || { error "macOS host-agent tarball validation failed"; exit 1; }
tar -tzf "pulse-agent-v${PULSE_VERSION}-darwin-arm64.tar.gz" pulse-agent-darwin-arm64 >/dev/null 2>&1 || { error "macOS unified-agent tarball validation failed"; exit 1; }
success "macOS agent tarballs validated"

# Validate checksums.txt
info "Validating checksums..."
sha256sum -c checksums.txt >/dev/null 2>&1 || { error "checksums.txt validation failed"; exit 1; }
success "checksums.txt validated"

# Validate individual .sha256 files exist and match checksums.txt
info "Validating individual .sha256 files..."
while IFS= read -r line; do
    checksum=$(echo "$line" | awk '{print $1}')
    filename=$(echo "$line" | awk '{print $2}')

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
extract_dir="$tmp_root/linux-amd64"
mkdir -p "$extract_dir"
tar -xzf "$RELEASE_DIR/pulse-v${PULSE_VERSION}-linux-amd64.tar.gz" -C "$extract_dir"

info "Testing extracted binaries from linux-amd64 tarball..."

# Test Pulse server
"$extract_dir/bin/pulse" version 2>/dev/null | grep -Fx "Pulse $PULSE_TAG" >/dev/null || { error "Extracted pulse binary version mismatch"; exit 1; }
success "Extracted pulse binary: $PULSE_TAG"

# Test host agent
"$extract_dir/bin/pulse-host-agent" --version 2>/dev/null | grep -Fx "$PULSE_TAG" >/dev/null || { error "Extracted host-agent version mismatch"; exit 1; }
success "Extracted host-agent binary: $PULSE_TAG"

# Test docker agent (no CLI flag)
grep -aF "$PULSE_TAG" "$extract_dir/bin/pulse-docker-agent" >/dev/null || { error "Extracted docker-agent version string not found"; exit 1; }
success "Extracted docker-agent binary contains: $PULSE_TAG"

# Test VERSION file
grep -Fx "$PULSE_VERSION" "$extract_dir/VERSION" >/dev/null || { error "Extracted VERSION file mismatch"; exit 1; }
success "Extracted VERSION file: $PULSE_VERSION"

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
