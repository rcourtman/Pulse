#!/usr/bin/env bash

# Build script for Pulse releases
# Creates release archives for different architectures

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PULSE_SCRIPTS_DIR="${SCRIPT_DIR}"
PULSE_REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${PULSE_REPO_ROOT}"

source "${SCRIPT_DIR}/release_asset_common.sh"

# Prefer the pinned toolchain from go.mod (toolchain directive).
# If /usr/local/go exists (typical in CI images), prepend it to PATH.
if [ -x /usr/local/go/bin/go ]; then
	    export PATH=/usr/local/go/bin:$PATH
fi

# Release artifacts must be built with the vetted toolchain to match security-gate evidence.
required_go="go1.25.9"
current_go="$(go env GOVERSION 2>/dev/null || true)"
if [[ "${PULSE_SKIP_GO_VERSION_CHECK:-false}" != "true" ]]; then
    if [[ "${current_go}" != "${required_go}" ]]; then
        echo "Error: Go toolchain must be ${required_go} (got ${current_go:-unknown})." >&2
        echo "Tip: set GOTOOLCHAIN=auto to allow automatic toolchain download." >&2
        echo "Override: PULSE_SKIP_GO_VERSION_CHECK=true (not recommended)." >&2
        exit 1
    fi
fi

# Force static binaries so release artifacts run on older glibc hosts
export CGO_ENABLED=0
release_go_build_args=(-buildvcs=false -trimpath)

VERSION=${1:-$(cat VERSION)}
BUILD_DIR="build"
RELEASE_DIR="release"
RENDERED_INSTALLERS_DIR="${BUILD_DIR}/rendered-installers"
RELEASE_PACKET_SBOM="pulse-v${VERSION}-release.sbom.spdx.json"

echo "Building Pulse v${VERSION}..."

# Require public key embedding for release-grade license validation.
# Explicitly opt out with PULSE_ALLOW_MISSING_LICENSE_KEY=true (not recommended).
license_ldflags_args=()
if [[ -z "${PULSE_LICENSE_PUBLIC_KEY:-}" ]]; then
    if [[ "${PULSE_ALLOW_MISSING_LICENSE_KEY:-false}" == "true" ]]; then
        echo "Warning: PULSE_LICENSE_PUBLIC_KEY not set; continuing because PULSE_ALLOW_MISSING_LICENSE_KEY=true."
    else
        echo "Error: PULSE_LICENSE_PUBLIC_KEY is required for release builds." >&2
        echo "Set PULSE_ALLOW_MISSING_LICENSE_KEY=true only for local non-release debugging." >&2
        exit 1
    fi
else
    decoded_key_len=$(printf '%s' "${PULSE_LICENSE_PUBLIC_KEY}" | openssl base64 -d -A 2>/dev/null | wc -c | tr -d ' ')
    if [[ "${decoded_key_len}" != "32" ]]; then
        echo "Error: PULSE_LICENSE_PUBLIC_KEY must decode to 32 bytes (Ed25519 public key)." >&2
        exit 1
    fi

    if [[ -n "${PULSE_LICENSE_PUBLIC_KEY_FINGERPRINT:-}" ]]; then
        expected_fingerprint="${PULSE_LICENSE_PUBLIC_KEY_FINGERPRINT#SHA256:}"
        actual_fingerprint=$(printf '%s' "${PULSE_LICENSE_PUBLIC_KEY}" | openssl base64 -d -A 2>/dev/null | openssl dgst -sha256 -binary | openssl base64 -A)
        if [[ -z "${actual_fingerprint}" ]]; then
            echo "Error: Failed to compute fingerprint for PULSE_LICENSE_PUBLIC_KEY." >&2
            exit 1
        fi
        if [[ "${actual_fingerprint}" != "${expected_fingerprint}" ]]; then
            echo "Error: PULSE_LICENSE_PUBLIC_KEY fingerprint mismatch." >&2
            echo "Expected: SHA256:${expected_fingerprint}" >&2
            echo "Actual:   SHA256:${actual_fingerprint}" >&2
            exit 1
        fi
        echo "Verified license public key fingerprint: SHA256:${actual_fingerprint}"
    fi

    license_ldflags_args=(--license-public-key "${PULSE_LICENSE_PUBLIC_KEY}")
fi

# Require update signing for release-grade agent and installer verification.
# Explicitly opt out with PULSE_ALLOW_MISSING_UPDATE_SIGNING_KEY=true for local-only debugging.
update_ldflags_args=()
pulse_release_prepare_signing_state "pulse-installer" "pulse-install"
trap 'pulse_release_cleanup_signing_state' EXIT
if [[ -n "${PULSE_RELEASE_UPDATE_PUBLIC_KEY:-}" ]]; then
    update_ldflags_args=(--update-public-keys "${PULSE_RELEASE_UPDATE_PUBLIC_KEY}")
fi

render_release_installers() {
    local output_dir="$1"
    mkdir -p "${output_dir}"
    go run ./scripts/render_installers.go \
        --source-dir ./scripts \
        --output-dir "${output_dir}" \
        --installer-ssh-public-key "${PULSE_RELEASE_UPDATE_SSH_PUBLIC_KEY}"
}

# Clean previous builds
rm -rf $BUILD_DIR $RELEASE_DIR
mkdir -p $BUILD_DIR $RELEASE_DIR
render_release_installers "${RENDERED_INSTALLERS_DIR}"

# Build frontend
echo "Building frontend..."
npm --prefix frontend-modern ci
npm --prefix frontend-modern run build

agent_ldflags="$(./scripts/release_ldflags.sh agent --version "v${VERSION}" "${update_ldflags_args[@]}")"

# Build unified agents for every supported platform/architecture
echo "Building unified agents for all platforms..."
agent_build_order=(linux-amd64 linux-arm64 linux-armv7 linux-armv6 linux-386 darwin-amd64 darwin-arm64 freebsd-amd64 freebsd-arm64 windows-amd64 windows-arm64 windows-386)
agent_build_envs=(
    "GOOS=linux GOARCH=amd64"
    "GOOS=linux GOARCH=arm64"
    "GOOS=linux GOARCH=arm GOARM=7"
    "GOOS=linux GOARCH=arm GOARM=6"
    "GOOS=linux GOARCH=386"
    "GOOS=darwin GOARCH=amd64"
    "GOOS=darwin GOARCH=arm64"
    "GOOS=freebsd GOARCH=amd64"
    "GOOS=freebsd GOARCH=arm64"
    "GOOS=windows GOARCH=amd64"
    "GOOS=windows GOARCH=arm64"
    "GOOS=windows GOARCH=386"
)

if [[ ${#agent_build_order[@]} -ne ${#agent_build_envs[@]} ]]; then
    echo "Unified agent build config mismatch." >&2
    exit 1
fi

for i in "${!agent_build_order[@]}"; do
    target="${agent_build_order[$i]}"
    build_env="${agent_build_envs[$i]}"
    output_path="$BUILD_DIR/pulse-agent-$target"
    if [[ "$target" == windows-* ]]; then
        output_path="${output_path}.exe"
    fi

    env $build_env go build \
        -ldflags="${agent_ldflags}" \
        "${release_go_build_args[@]}" \
        -o "$output_path" \
        ./cmd/pulse-agent
done

# Build pulse-mcp (Model Context Protocol adapter) for the same
# multi-OS matrix as the unified agent. The MCP server runs on
# the integrator's machine (Mac, Windows, Linux desktop) and
# speaks stdio to a local MCP client like Claude Desktop, so it
# needs the full desktop-OS matrix even though the Pulse server
# itself only ships for Linux. The binary takes no version
# ldflags: it reads the manifest from whichever Pulse instance
# it points at, so its own build identity is intentionally minimal.
echo "Building pulse-mcp for all platforms..."
mcp_build_order=("${agent_build_order[@]}")
mcp_build_envs=("${agent_build_envs[@]}")

for i in "${!mcp_build_order[@]}"; do
    target="${mcp_build_order[$i]}"
    build_env="${mcp_build_envs[$i]}"
    output_path="$BUILD_DIR/pulse-mcp-$target"
    if [[ "$target" == windows-* ]]; then
        output_path="${output_path}.exe"
    fi

    env $build_env go build \
        "${release_go_build_args[@]}" \
        -o "$output_path" \
        ./cmd/pulse-mcp
done

# Build for different architectures (server + agents)
build_order=(linux-amd64 linux-arm64 linux-armv7 linux-armv6 linux-386)
build_envs=(
    "GOOS=linux GOARCH=amd64"
    "GOOS=linux GOARCH=arm64"
    "GOOS=linux GOARCH=arm GOARM=7"
    "GOOS=linux GOARCH=arm GOARM=6"
    "GOOS=linux GOARCH=386"
)

if [[ ${#build_order[@]} -ne ${#build_envs[@]} ]]; then
    echo "Build target config mismatch." >&2
    exit 1
fi

for i in "${!build_order[@]}"; do
    build_name="${build_order[$i]}"
    echo "Building for $build_name..."

    build_env="${build_envs[$i]}"
    
    build_time=$(date -u '+%Y-%m-%d_%H:%M:%S')
    git_commit=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')

    server_ldflags="$(./scripts/release_ldflags.sh server --version "v${VERSION}" --build-time "${build_time}" --git-commit "${git_commit}" "${license_ldflags_args[@]}" "${update_ldflags_args[@]}")"

    # Build backend binary with version info
    # -tags release disables dev-mode env-var bypasses (PULSE_DEV, PULSE_MOCK_MODE,
    # PULSE_LICENSE_DEV_MODE) so they cannot be used to skip feature gating or
    # license signature validation in production binaries.
    env $build_env go build \
        -tags release \
        -ldflags="${server_ldflags}" \
        "${release_go_build_args[@]}" \
        -o "$BUILD_DIR/pulse-$build_name" \
        ./cmd/pulse
done

# Create platform-specific tarballs that include all unified agent binaries for download endpoints
for build_name in "${build_order[@]}"; do
    echo "Packaging release for $build_name..."

    tar_name="pulse-v${VERSION}-${build_name}.tar.gz"
    staging_dir="$BUILD_DIR/staging-$build_name"
    rm -rf "$staging_dir"
    mkdir -p "$staging_dir/bin"
    mkdir -p "$staging_dir/scripts"

    # Copy architecture-specific runtime binaries
    cp "$BUILD_DIR/pulse-$build_name" "$staging_dir/bin/pulse"

    # Copy unified agent binaries for every supported platform/architecture
    for target in "${agent_build_order[@]}"; do
        src="$BUILD_DIR/pulse-agent-$target"
        dest="$staging_dir/bin/pulse-agent-$target"
        if [[ "$target" == windows-* ]]; then
            src="${src}.exe"
            dest="${dest}.exe"
        fi
        cp "$src" "$dest"
    done
    ( cd "$staging_dir/bin" && ln -sf pulse-agent-windows-amd64.exe pulse-agent-windows-amd64 && ln -sf pulse-agent-windows-arm64.exe pulse-agent-windows-arm64 && ln -sf pulse-agent-windows-386.exe pulse-agent-windows-386 )

    # Copy scripts and VERSION metadata
    cp "scripts/install-container-agent.sh" "$staging_dir/scripts/install-container-agent.sh"
    cp "scripts/install-docker.sh" "$staging_dir/scripts/install-docker.sh"
    cp "${RENDERED_INSTALLERS_DIR}/install.sh" "$staging_dir/scripts/install.sh"
    [ -f "${RENDERED_INSTALLERS_DIR}/install.ps1" ] && cp "${RENDERED_INSTALLERS_DIR}/install.ps1" "$staging_dir/scripts/install.ps1"
    chmod 755 "$staging_dir/scripts/"*.sh
    chmod 755 "$staging_dir/scripts/"*.ps1 2>/dev/null || true
    echo "$VERSION" > "$staging_dir/VERSION"
    pulse_release_sign_directory_assets "$staging_dir/bin"
    pulse_release_sign_directory_assets "$staging_dir/scripts"
    pulse_release_sign_file "$staging_dir/VERSION"

    # Create tarball from staging directory
    cd "$staging_dir"
    tar -czf "../../$RELEASE_DIR/$tar_name" .
    cd ../..

    rm -rf "$staging_dir"
    echo "Created $RELEASE_DIR/$tar_name"
done

# Create universal tarball with all binaries
echo "Creating universal tarball..."
universal_dir="$BUILD_DIR/universal"
rm -rf "$universal_dir"
mkdir -p "$universal_dir/bin"
mkdir -p "$universal_dir/scripts"

# Copy all binaries to bin/ directory to maintain consistent structure
for build_name in "${build_order[@]}"; do
    cp "$BUILD_DIR/pulse-$build_name" "$universal_dir/bin/pulse-${build_name}"
    cp "$BUILD_DIR/pulse-agent-$build_name" "$universal_dir/bin/pulse-agent-${build_name}"
done

cp "scripts/install-container-agent.sh" "$universal_dir/scripts/install-container-agent.sh"
cp "scripts/install-docker.sh" "$universal_dir/scripts/install-docker.sh"
cp "${RENDERED_INSTALLERS_DIR}/install.sh" "$universal_dir/scripts/install.sh"
[ -f "${RENDERED_INSTALLERS_DIR}/install.ps1" ] && cp "${RENDERED_INSTALLERS_DIR}/install.ps1" "$universal_dir/scripts/install.ps1"
chmod 755 "$universal_dir/scripts/"*.sh
chmod 755 "$universal_dir/scripts/"*.ps1 2>/dev/null || true

# Create a detection script that creates the pulse symlink based on architecture
cat > "$universal_dir/bin/pulse" << 'EOF'
#!/bin/sh
# Auto-detect architecture and run appropriate binary

ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)
        exec "$(dirname "$0")/pulse-linux-amd64" "$@"
        ;;
    aarch64|arm64)
        exec "$(dirname "$0")/pulse-linux-arm64" "$@"
        ;;
    armv7l|armhf)
        exec "$(dirname "$0")/pulse-linux-armv7" "$@"
        ;;
    *)
        echo "Unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac
EOF
chmod +x "$universal_dir/bin/pulse"



cat > "$universal_dir/bin/pulse-agent" << 'EOF'
#!/bin/sh
# Auto-detect architecture and run appropriate pulse-agent binary

ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)
        exec "$(dirname "$0")/pulse-agent-linux-amd64" "$@"
        ;;
    aarch64|arm64)
        exec "$(dirname "$0")/pulse-agent-linux-arm64" "$@"
        ;;
    armv7l|armhf)
        exec "$(dirname "$0")/pulse-agent-linux-armv7" "$@"
        ;;
    *)
        echo "Unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac
EOF
chmod +x "$universal_dir/bin/pulse-agent"

# Add VERSION file
echo "$VERSION" > "$universal_dir/VERSION"
pulse_release_sign_directory_assets "$universal_dir/bin"
pulse_release_sign_directory_assets "$universal_dir/scripts"
pulse_release_sign_file "$universal_dir/VERSION"

# Package standalone unified agent binaries (all platforms)
# Linux
tar -czf "$RELEASE_DIR/pulse-agent-v${VERSION}-linux-amd64.tar.gz" -C "$BUILD_DIR" pulse-agent-linux-amd64
tar -czf "$RELEASE_DIR/pulse-agent-v${VERSION}-linux-arm64.tar.gz" -C "$BUILD_DIR" pulse-agent-linux-arm64
tar -czf "$RELEASE_DIR/pulse-agent-v${VERSION}-linux-armv7.tar.gz" -C "$BUILD_DIR" pulse-agent-linux-armv7
tar -czf "$RELEASE_DIR/pulse-agent-v${VERSION}-linux-armv6.tar.gz" -C "$BUILD_DIR" pulse-agent-linux-armv6
tar -czf "$RELEASE_DIR/pulse-agent-v${VERSION}-linux-386.tar.gz" -C "$BUILD_DIR" pulse-agent-linux-386
# Darwin
tar -czf "$RELEASE_DIR/pulse-agent-v${VERSION}-darwin-amd64.tar.gz" -C "$BUILD_DIR" pulse-agent-darwin-amd64
tar -czf "$RELEASE_DIR/pulse-agent-v${VERSION}-darwin-arm64.tar.gz" -C "$BUILD_DIR" pulse-agent-darwin-arm64
# FreeBSD
tar -czf "$RELEASE_DIR/pulse-agent-v${VERSION}-freebsd-amd64.tar.gz" -C "$BUILD_DIR" pulse-agent-freebsd-amd64
tar -czf "$RELEASE_DIR/pulse-agent-v${VERSION}-freebsd-arm64.tar.gz" -C "$BUILD_DIR" pulse-agent-freebsd-arm64
# Windows (zip archives with version in filename)
zip -j "$RELEASE_DIR/pulse-agent-v${VERSION}-windows-amd64.zip" "$BUILD_DIR/pulse-agent-windows-amd64.exe"
zip -j "$RELEASE_DIR/pulse-agent-v${VERSION}-windows-arm64.zip" "$BUILD_DIR/pulse-agent-windows-arm64.exe"
zip -j "$RELEASE_DIR/pulse-agent-v${VERSION}-windows-386.zip" "$BUILD_DIR/pulse-agent-windows-386.exe"

# Package standalone pulse-mcp binaries (all platforms). Mirrors
# the pulse-agent packaging shape exactly so the release-asset
# upload step does not need per-binary special cases.
# Linux
tar -czf "$RELEASE_DIR/pulse-mcp-v${VERSION}-linux-amd64.tar.gz" -C "$BUILD_DIR" pulse-mcp-linux-amd64
tar -czf "$RELEASE_DIR/pulse-mcp-v${VERSION}-linux-arm64.tar.gz" -C "$BUILD_DIR" pulse-mcp-linux-arm64
tar -czf "$RELEASE_DIR/pulse-mcp-v${VERSION}-linux-armv7.tar.gz" -C "$BUILD_DIR" pulse-mcp-linux-armv7
tar -czf "$RELEASE_DIR/pulse-mcp-v${VERSION}-linux-armv6.tar.gz" -C "$BUILD_DIR" pulse-mcp-linux-armv6
tar -czf "$RELEASE_DIR/pulse-mcp-v${VERSION}-linux-386.tar.gz" -C "$BUILD_DIR" pulse-mcp-linux-386
# Darwin
tar -czf "$RELEASE_DIR/pulse-mcp-v${VERSION}-darwin-amd64.tar.gz" -C "$BUILD_DIR" pulse-mcp-darwin-amd64
tar -czf "$RELEASE_DIR/pulse-mcp-v${VERSION}-darwin-arm64.tar.gz" -C "$BUILD_DIR" pulse-mcp-darwin-arm64
# FreeBSD
tar -czf "$RELEASE_DIR/pulse-mcp-v${VERSION}-freebsd-amd64.tar.gz" -C "$BUILD_DIR" pulse-mcp-freebsd-amd64
tar -czf "$RELEASE_DIR/pulse-mcp-v${VERSION}-freebsd-arm64.tar.gz" -C "$BUILD_DIR" pulse-mcp-freebsd-arm64
# Windows (zip archives with version in filename)
zip -j "$RELEASE_DIR/pulse-mcp-v${VERSION}-windows-amd64.zip" "$BUILD_DIR/pulse-mcp-windows-amd64.exe"
zip -j "$RELEASE_DIR/pulse-mcp-v${VERSION}-windows-arm64.zip" "$BUILD_DIR/pulse-mcp-windows-arm64.exe"
zip -j "$RELEASE_DIR/pulse-mcp-v${VERSION}-windows-386.zip" "$BUILD_DIR/pulse-mcp-windows-386.exe"

# Also copy bare binaries for /releases/latest/download/ redirect compatibility
# These allow LXC/barebone installs to redirect to GitHub without needing versioned URLs
echo "Copying bare binaries to release directory for redirect compatibility..."
cp "$BUILD_DIR/pulse-agent-linux-amd64" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-agent-linux-arm64" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-agent-linux-armv7" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-agent-linux-armv6" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-agent-linux-386" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-agent-windows-amd64.exe" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-agent-windows-arm64.exe" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-agent-windows-386.exe" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-agent-freebsd-amd64" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-agent-freebsd-arm64" "$RELEASE_DIR/"

# Copy bare pulse-mcp binaries for /releases/latest/download/ redirect
# compatibility. The install-mcp.sh installer fetches these directly from
# the GitHub Releases endpoint without needing a versioned URL.
cp "$BUILD_DIR/pulse-mcp-linux-amd64" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-mcp-linux-arm64" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-mcp-linux-armv7" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-mcp-linux-armv6" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-mcp-linux-386" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-mcp-darwin-amd64" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-mcp-darwin-arm64" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-mcp-windows-amd64.exe" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-mcp-windows-arm64.exe" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-mcp-windows-386.exe" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-mcp-freebsd-amd64" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-mcp-freebsd-arm64" "$RELEASE_DIR/"

# Copy Windows, macOS, and FreeBSD binaries into universal tarball for /download/ endpoint
echo "Adding Windows, macOS, and FreeBSD binaries to universal tarball..."
cp "$BUILD_DIR/pulse-agent-darwin-amd64" "$universal_dir/bin/"
cp "$BUILD_DIR/pulse-agent-darwin-arm64" "$universal_dir/bin/"
cp "$BUILD_DIR/pulse-agent-freebsd-amd64" "$universal_dir/bin/"
cp "$BUILD_DIR/pulse-agent-freebsd-arm64" "$universal_dir/bin/"
cp "$BUILD_DIR/pulse-agent-windows-amd64.exe" "$universal_dir/bin/"
cp "$BUILD_DIR/pulse-agent-windows-arm64.exe" "$universal_dir/bin/"
cp "$BUILD_DIR/pulse-agent-windows-386.exe" "$universal_dir/bin/"

# Create symlinks for Windows binaries without .exe extension (required for download endpoint)
ln -s pulse-agent-windows-amd64.exe "$universal_dir/bin/pulse-agent-windows-amd64"
ln -s pulse-agent-windows-arm64.exe "$universal_dir/bin/pulse-agent-windows-arm64"
ln -s pulse-agent-windows-386.exe "$universal_dir/bin/pulse-agent-windows-386"

# Create universal tarball
cd "$universal_dir"
tar -czf "../../$RELEASE_DIR/pulse-v${VERSION}.tar.gz" .
cd ../..

# Cleanup
rm -rf "$universal_dir"

# Optionally package Helm chart
if [ "${SKIP_HELM_PACKAGE:-0}" != "1" ]; then
    if command -v helm >/dev/null 2>&1; then
        echo "Packaging Helm chart..."
        ./scripts/package-helm-chart.sh "$VERSION"
        if [ -f "dist/pulse-$VERSION.tgz" ]; then
            cp "dist/pulse-$VERSION.tgz" "$RELEASE_DIR/"
        fi
    else
        echo "Helm not found on PATH; skipping Helm chart packaging. Install Helm 3.9+ or set SKIP_HELM_PACKAGE=1 to silence this message."
    fi
fi

# Copy install scripts to release directory (required for GitHub releases)
# These are uploaded as standalone assets so users can:
#   curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | bash
# instead of pulling from main branch (which may have newer, incompatible changes)
echo "Copying install scripts to release directory..."
cp "${RENDERED_INSTALLERS_DIR}/install.sh" "$RELEASE_DIR/install.sh"
[ -f "${RENDERED_INSTALLERS_DIR}/install.ps1" ] && cp "${RENDERED_INSTALLERS_DIR}/install.ps1" "$RELEASE_DIR/install.ps1"
cp scripts/install-docker.sh "$RELEASE_DIR/"
cp scripts/pulse-auto-update.sh "$RELEASE_DIR/"
cp scripts/install-mcp.sh "$RELEASE_DIR/install-mcp.sh"
[ -f scripts/install-mcp.ps1 ] && cp scripts/install-mcp.ps1 "$RELEASE_DIR/install-mcp.ps1"

pulse_release_generate_packet_sbom "${RELEASE_DIR}" "${RELEASE_PACKET_SBOM}"
mapfile -t checksum_files < <(pulse_release_collect_checksum_files "${RELEASE_DIR}")
pulse_release_write_checksums_and_signatures "${RELEASE_DIR}" "${checksum_files[@]}"

echo
echo "Release build complete!"
echo "Archives created in $RELEASE_DIR/"
ls -lh $RELEASE_DIR/
