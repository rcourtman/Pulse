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

success() {
    echo -e "${GREEN}[✓]${NC} $*"
}

info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
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
trap 'rm -rf "$tmp_root"' EXIT

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
    docker run --rm --entrypoint /bin/sh -e EXPECTED_VERSION="$PULSE_VERSION" "$IMAGE" -c 'set -euo pipefail; actual=$(cat /VERSION | tr -d "\r\n"); [ "$actual" = "$EXPECTED_VERSION" ] || { echo "VERSION mismatch: expected=$EXPECTED_VERSION actual=$actual" >&2; exit 1; }' || { error "VERSION file mismatch in Docker image"; exit 1; }
    success "VERSION file correct: $PULSE_VERSION"

    # Validate all required scripts exist and are executable
    info "Checking installer/uninstaller scripts in /opt/pulse/scripts/..."
    docker run --rm --entrypoint /bin/sh "$IMAGE" -c 'set -euo pipefail; cd /opt/pulse/scripts; required="install-docker-agent.sh install-container-agent.sh install-host-agent.sh install-host-agent.ps1 uninstall-host-agent.sh uninstall-host-agent.ps1 install-sensor-proxy.sh install-docker.sh"; for f in $required; do [ -f "$f" ] || { echo "missing script $f" >&2; exit 1; }; case "$f" in *.sh|*.ps1) [ -x "$f" ] || { echo "$f not executable" >&2; exit 1; };; esac; done; echo "All scripts present and executable"' || { error "Script validation failed"; exit 1; }
    success "All installer/uninstaller scripts present and executable"

    # Validate all required binaries exist and are non-empty
    info "Checking downloadable binaries in /opt/pulse/bin/..."
    docker run --rm --entrypoint /bin/sh "$IMAGE" -c 'set -euo pipefail; cd /opt/pulse/bin; required="pulse pulse-docker-agent pulse-docker-agent-linux-amd64 pulse-docker-agent-linux-arm64 pulse-docker-agent-linux-armv7 pulse-docker-agent-linux-armv6 pulse-docker-agent-linux-386 pulse-host-agent-linux-amd64 pulse-host-agent-linux-arm64 pulse-host-agent-linux-armv7 pulse-host-agent-linux-armv6 pulse-host-agent-linux-386 pulse-host-agent-darwin-amd64 pulse-host-agent-darwin-arm64 pulse-host-agent-windows-amd64.exe pulse-host-agent-windows-amd64 pulse-host-agent-windows-arm64.exe pulse-host-agent-windows-arm64 pulse-sensor-proxy pulse-sensor-proxy-linux-amd64 pulse-sensor-proxy-linux-arm64 pulse-sensor-proxy-linux-armv7 pulse-sensor-proxy-linux-armv6 pulse-sensor-proxy-linux-386"; for f in $required; do [ -e "$f" ] || { echo "missing binary $f" >&2; exit 1; }; [ -s "$f" ] || { echo "empty binary $f" >&2; exit 1; }; done; [ "$(readlink pulse-host-agent-windows-amd64)" = "pulse-host-agent-windows-amd64.exe" ] || { echo "windows amd64 symlink broken" >&2; exit 1; }; [ "$(readlink pulse-host-agent-windows-arm64)" = "pulse-host-agent-windows-arm64.exe" ] || { echo "windows arm64 symlink broken" >&2; exit 1; }; echo "All binaries present"' || { error "Binary validation failed"; exit 1; }
    success "All downloadable binaries present (24 binaries + 2 Windows symlinks)"

    # Validate version embedding in Docker image binaries
    info "Validating version embedding in Docker image binaries..."

    # Pulse server binary
    docker run --rm --entrypoint /app/pulse "$IMAGE" version 2>/dev/null | grep -Fx "Pulse $PULSE_TAG" >/dev/null || { error "Pulse server version mismatch"; exit 1; }
    success "Pulse server version: $PULSE_TAG"

    # Host agent binary
    docker run --rm --entrypoint /opt/pulse/bin/pulse-host-agent-linux-amd64 "$IMAGE" --version 2>/dev/null | grep -Fx "$PULSE_TAG" >/dev/null || { error "Host agent version mismatch"; exit 1; }
    success "Host agent version: $PULSE_TAG"

    # Sensor proxy binary
    docker run --rm --entrypoint /opt/pulse/bin/pulse-sensor-proxy-linux-amd64 "$IMAGE" version 2>/dev/null | grep -Fx "pulse-sensor-proxy $PULSE_TAG" >/dev/null || { error "Sensor proxy version mismatch"; exit 1; }
    success "Sensor proxy version: $PULSE_TAG"

    # Docker agent binary (no CLI flag, check binary strings)
    docker run --rm --entrypoint /bin/sh -e EXPECTED_TAG="$PULSE_TAG" "$IMAGE" -c 'set -euo pipefail; grep -aF "$EXPECTED_TAG" /opt/pulse/bin/pulse-docker-agent-linux-amd64 >/dev/null' || { error "Docker agent version string not found"; exit 1; }
    success "Docker agent version embedded: $PULSE_TAG"

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
required_assets=(
    "install.sh"
    "checksums.txt"
    "pulse-v${PULSE_VERSION}.tar.gz"
    "pulse-v${PULSE_VERSION}-linux-amd64.tar.gz"
    "pulse-v${PULSE_VERSION}-linux-arm64.tar.gz"
    "pulse-v${PULSE_VERSION}-linux-armv7.tar.gz"
    "pulse-host-agent-v${PULSE_VERSION}-darwin-arm64.tar.gz"
    "pulse-host-agent-linux-amd64"
    "pulse-host-agent-linux-arm64"
    "pulse-host-agent-linux-armv7"
    "pulse-host-agent-darwin-arm64"
    "pulse-host-agent-windows-amd64.exe"
    "pulse-sensor-proxy-linux-amd64"
    "pulse-sensor-proxy-linux-arm64"
    "pulse-sensor-proxy-linux-armv7"
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

# Validate tarball contents
info "Validating tarball contents..."
for arch in linux-amd64 linux-arm64 linux-armv7; do
    tarball="pulse-v${PULSE_VERSION}-${arch}.tar.gz"

    # Check binaries (note: tarballs use ./  prefix)
    if ! tar -tzf "$tarball" ./bin/pulse ./bin/pulse-docker-agent ./bin/pulse-host-agent ./bin/pulse-sensor-proxy >/dev/null 2>&1; then
        error "$(basename $tarball) missing binaries"
        exit 1
    fi

    # Check scripts
    tar -tzf "$tarball" ./scripts/install-docker-agent.sh ./scripts/install-container-agent.sh ./scripts/install-host-agent.sh ./scripts/install-host-agent.ps1 ./scripts/uninstall-host-agent.sh ./scripts/uninstall-host-agent.ps1 ./scripts/install-sensor-proxy.sh ./scripts/install-docker.sh >/dev/null 2>&1 || { error "$(basename $tarball) missing scripts"; exit 1; }

    # Check VERSION file
    tar -tzf "$tarball" ./VERSION >/dev/null 2>&1 || { error "$(basename $tarball) missing VERSION file"; exit 1; }
done
success "Platform-specific tarballs contain all required files"

# Validate universal tarball
tar -tzf "pulse-v${PULSE_VERSION}.tar.gz" ./VERSION >/dev/null 2>&1 || { error "Universal tarball missing VERSION file"; exit 1; }
success "Universal tarball validated"

# Validate macOS tarball
tar -tzf "pulse-host-agent-v${PULSE_VERSION}-darwin-arm64.tar.gz" pulse-host-agent-darwin-arm64 >/dev/null 2>&1 || { error "macOS tarball validation failed"; exit 1; }
success "macOS host-agent tarball validated"

# Validate checksums exist for all distributable assets
info "Validating checksums..."
checksum_errors=0
for asset in *.tar.gz *.zip *.tgz install.sh pulse-sensor-proxy-linux-* pulse-host-agent-linux-* pulse-host-agent-darwin-* pulse-host-agent-windows-*.exe; do
    # Skip checksum files themselves
    [[ "$asset" == *.sha256 ]] && continue

    if [ ! -f "${asset}.sha256" ]; then
        error "Missing checksum file: ${asset}.sha256"
        checksum_errors=$((checksum_errors + 1))
    else
        # Verify checksum
        sha256sum -c "${asset}.sha256" >/dev/null 2>&1 || { error "Checksum verification failed for $asset"; checksum_errors=$((checksum_errors + 1)); }
    fi
done

if [ $checksum_errors -gt 0 ]; then
    error "$checksum_errors checksum validation failures"
    exit 1
fi
success "All individual checksums present and verified"

# Validate combined checksums.txt
sha256sum -c checksums.txt >/dev/null 2>&1 || { error "checksums.txt validation failed"; exit 1; }
success "checksums.txt validated"

# Validate architecture of standalone binaries (requires 'file' command)
if command -v file >/dev/null 2>&1; then
    info "Validating binary architectures..."

    file pulse-host-agent-linux-amd64 | grep -qF 'ELF 64-bit' | grep -qF 'x86-64' || warn "pulse-host-agent-linux-amd64 architecture check failed"
    file pulse-host-agent-linux-arm64 | grep -qF 'ELF 64-bit' | grep -qF 'aarch64' || warn "pulse-host-agent-linux-arm64 architecture check failed"
    file pulse-host-agent-linux-armv7 | grep -qF 'ELF 32-bit' | grep -qF 'ARM' || warn "pulse-host-agent-linux-armv7 architecture check failed"
    file pulse-host-agent-darwin-amd64 | grep -qF 'Mach-O' | grep -qF 'x86_64' || warn "pulse-host-agent-darwin-amd64 architecture check failed"
    file pulse-host-agent-darwin-arm64 | grep -qF 'Mach-O' | grep -qF 'arm64' || warn "pulse-host-agent-darwin-arm64 architecture check failed"
    file pulse-host-agent-windows-amd64.exe | grep -qF 'PE32+' | grep -qF 'x86-64' || warn "pulse-host-agent-windows-amd64.exe architecture check failed"
    file pulse-host-agent-windows-arm64.exe | grep -qF 'PE32+' | grep -qF 'Aarch64' || warn "pulse-host-agent-windows-arm64.exe architecture check failed"

    success "Binary architectures validated"
fi

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

# Test sensor proxy
"$extract_dir/bin/pulse-sensor-proxy" version 2>/dev/null | grep -Fx "pulse-sensor-proxy $PULSE_TAG" >/dev/null || { error "Extracted sensor-proxy version mismatch"; exit 1; }
success "Extracted sensor-proxy binary: $PULSE_TAG"

# Test docker agent (no CLI flag)
grep -aF "$PULSE_TAG" "$extract_dir/bin/pulse-docker-agent" >/dev/null || { error "Extracted docker-agent version string not found"; exit 1; }
success "Extracted docker-agent binary contains: $PULSE_TAG"

# Test VERSION file
grep -Fx "$PULSE_VERSION" "$extract_dir/VERSION" >/dev/null || { error "Extracted VERSION file mismatch"; exit 1; }
success "Extracted VERSION file: $PULSE_VERSION"

echo ""

#=============================================================================
# STANDALONE BINARY VALIDATION
#=============================================================================
info "=== Standalone Binary Validation ==="

info "Testing standalone binaries in release directory..."

# Host agent
"$RELEASE_DIR/pulse-host-agent-linux-amd64" --version 2>/dev/null | grep -Fx "$PULSE_TAG" >/dev/null || { error "Standalone host-agent version mismatch"; exit 1; }
success "Standalone host-agent: $PULSE_TAG"

# Sensor proxy
"$RELEASE_DIR/pulse-sensor-proxy-linux-amd64" version 2>/dev/null | grep -Fx "pulse-sensor-proxy $PULSE_TAG" >/dev/null || { error "Standalone sensor-proxy version mismatch"; exit 1; }
success "Standalone sensor-proxy: $PULSE_TAG"

# Docker agent (built for all 3 architectures)
for arch in linux-amd64 linux-arm64 linux-armv7; do
    if [ -f "$RELEASE_DIR/pulse-docker-agent-$arch" ]; then
        grep -aF "$PULSE_TAG" "$RELEASE_DIR/pulse-docker-agent-$arch" >/dev/null || { error "Standalone docker-agent-$arch version string not found"; exit 1; }
    fi
done
success "Standalone docker-agent binaries contain: $PULSE_TAG"

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
