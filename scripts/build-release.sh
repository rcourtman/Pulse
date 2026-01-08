#!/usr/bin/env bash

# Build script for Pulse releases
# Creates release archives for different architectures

set -euo pipefail

# Use Go 1.24 if available
if [ -x /usr/local/go/bin/go ]; then
    export PATH=/usr/local/go/bin:$PATH
fi

# Force static binaries so release artifacts run on older glibc hosts
export CGO_ENABLED=0

VERSION=${1:-$(cat VERSION)}
BUILD_DIR="build"
RELEASE_DIR="release"

echo "Building Pulse v${VERSION}..."

# Optional public key embedding for license validation
LICENSE_LDFLAGS=""
if [[ -n "${PULSE_LICENSE_PUBLIC_KEY:-}" ]]; then
    LICENSE_LDFLAGS="-X github.com/rcourtman/pulse-go-rewrite/internal/license.EmbeddedPublicKey=${PULSE_LICENSE_PUBLIC_KEY}"
else
    echo "Warning: PULSE_LICENSE_PUBLIC_KEY not set; Pulse Pro license activation will fail for release binaries."
fi

# Clean previous builds
rm -rf $BUILD_DIR $RELEASE_DIR
mkdir -p $BUILD_DIR $RELEASE_DIR

# Build frontend
echo "Building frontend..."
npm --prefix frontend-modern ci
npm --prefix frontend-modern run build

# Copy frontend dist for embedding (required for Go embed)
echo "Copying frontend dist for embedding..."
rm -rf internal/api/frontend-modern
mkdir -p internal/api/frontend-modern
cp -r frontend-modern/dist internal/api/frontend-modern/

# Build host agents for every supported platform/architecture so download endpoints work offline
echo "Building host agents for all platforms..."
declare -A host_agent_builds=(
    ["linux-amd64"]="GOOS=linux GOARCH=amd64"
    ["linux-arm64"]="GOOS=linux GOARCH=arm64"
    ["linux-armv7"]="GOOS=linux GOARCH=arm GOARM=7"
    ["linux-armv6"]="GOOS=linux GOARCH=arm GOARM=6"
    ["linux-386"]="GOOS=linux GOARCH=386"
    ["darwin-amd64"]="GOOS=darwin GOARCH=amd64"
    ["darwin-arm64"]="GOOS=darwin GOARCH=arm64"
    ["freebsd-amd64"]="GOOS=freebsd GOARCH=amd64"
    ["freebsd-arm64"]="GOOS=freebsd GOARCH=arm64"
    ["windows-amd64"]="GOOS=windows GOARCH=amd64"
    ["windows-arm64"]="GOOS=windows GOARCH=arm64"
    ["windows-386"]="GOOS=windows GOARCH=386"
)
host_agent_order=(linux-amd64 linux-arm64 linux-armv7 linux-armv6 linux-386 darwin-amd64 darwin-arm64 freebsd-amd64 freebsd-arm64 windows-amd64 windows-arm64 windows-386)

for target in "${host_agent_order[@]}"; do
    build_env="${host_agent_builds[$target]}"
    output_path="$BUILD_DIR/pulse-host-agent-$target"
    if [[ "$target" == windows-* ]]; then
        output_path="${output_path}.exe"
    fi

    env $build_env go build \
        -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/hostagent.Version=v${VERSION}" \
        -trimpath \
        -o "$output_path" \
        ./cmd/pulse-host-agent
done

# Build unified agents for every supported platform/architecture
echo "Building unified agents for all platforms..."
for target in "${host_agent_order[@]}"; do
    build_env="${host_agent_builds[$target]}"
    output_path="$BUILD_DIR/pulse-agent-$target"
    if [[ "$target" == windows-* ]]; then
        output_path="${output_path}.exe"
    fi

    env $build_env go build \
        -ldflags="-s -w -X main.Version=v${VERSION}" \
        -trimpath \
        -o "$output_path" \
        ./cmd/pulse-agent
done

# Build for different architectures (server + docker agent + sensor proxy)
declare -A builds=(
    ["linux-amd64"]="GOOS=linux GOARCH=amd64"
    ["linux-arm64"]="GOOS=linux GOARCH=arm64"
    ["linux-armv7"]="GOOS=linux GOARCH=arm GOARM=7"
    ["linux-armv6"]="GOOS=linux GOARCH=arm GOARM=6"
    ["linux-386"]="GOOS=linux GOARCH=386"
)
build_order=(linux-amd64 linux-arm64 linux-armv7 linux-armv6 linux-386)

for build_name in "${build_order[@]}"; do
    echo "Building for $build_name..."
    
    build_env="${builds[$build_name]}"
    
    build_time=$(date -u '+%Y-%m-%d_%H:%M:%S')
    git_commit=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')

    # Build backend binary with version info
    env $build_env go build \
        -ldflags="-s -w -X main.Version=v${VERSION} -X main.BuildTime=${build_time} -X main.GitCommit=${git_commit} -X github.com/rcourtman/pulse-go-rewrite/internal/dockeragent.Version=v${VERSION} ${LICENSE_LDFLAGS}" \
        -trimpath \
        -o "$BUILD_DIR/pulse-$build_name" \
        ./cmd/pulse



    # Build temperature proxy binary
    env $build_env go build \
        -ldflags="-s -w -X main.Version=v${VERSION} -X main.BuildTime=${build_time} -X main.GitCommit=${git_commit}" \
        -trimpath \
        -o "$BUILD_DIR/pulse-sensor-proxy-$build_name" \
        ./cmd/pulse-sensor-proxy
done

# Create platform-specific tarballs that include all host agent binaries for download endpoints
for build_name in "${build_order[@]}"; do
    echo "Packaging release for $build_name..."

    tar_name="pulse-v${VERSION}-${build_name}.tar.gz"
    staging_dir="$BUILD_DIR/staging-$build_name"
    rm -rf "$staging_dir"
    mkdir -p "$staging_dir/bin"
    mkdir -p "$staging_dir/scripts"

    # Copy architecture-specific runtime binaries
    cp "$BUILD_DIR/pulse-$build_name" "$staging_dir/bin/pulse"

    cp "$BUILD_DIR/pulse-host-agent-$build_name" "$staging_dir/bin/pulse-host-agent"
    cp "$BUILD_DIR/pulse-sensor-proxy-$build_name" "$staging_dir/bin/pulse-sensor-proxy"

    # Copy host agent binaries for every supported platform/architecture
    for target in "${host_agent_order[@]}"; do
        src="$BUILD_DIR/pulse-host-agent-$target"
        dest="$staging_dir/bin/pulse-host-agent-$target"
        if [[ "$target" == windows-* ]]; then
            src="${src}.exe"
            dest="${dest}.exe"
        fi
        cp "$src" "$dest"
    done
    ( cd "$staging_dir/bin" && ln -sf pulse-host-agent-windows-amd64.exe pulse-host-agent-windows-amd64 && ln -sf pulse-host-agent-windows-arm64.exe pulse-host-agent-windows-arm64 && ln -sf pulse-host-agent-windows-386.exe pulse-host-agent-windows-386 )

    # Copy unified agent binaries for every supported platform/architecture
    for target in "${host_agent_order[@]}"; do
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
    cp "scripts/install-docker-agent.sh" "$staging_dir/scripts/install-docker-agent.sh"
    cp "scripts/install-container-agent.sh" "$staging_dir/scripts/install-container-agent.sh"
    cp "scripts/install-host-agent.ps1" "$staging_dir/scripts/install-host-agent.ps1"
    cp "scripts/uninstall-host-agent.sh" "$staging_dir/scripts/uninstall-host-agent.sh"
    cp "scripts/uninstall-host-agent.ps1" "$staging_dir/scripts/uninstall-host-agent.ps1"
    cp "scripts/install-sensor-proxy.sh" "$staging_dir/scripts/install-sensor-proxy.sh"
    cp "scripts/install-docker.sh" "$staging_dir/scripts/install-docker.sh"
    cp "scripts/install.sh" "$staging_dir/scripts/install.sh"
    [ -f "scripts/install.ps1" ] && cp "scripts/install.ps1" "$staging_dir/scripts/install.ps1"
    chmod 755 "$staging_dir/scripts/"*.sh
    chmod 755 "$staging_dir/scripts/"*.ps1 2>/dev/null || true
    echo "$VERSION" > "$staging_dir/VERSION"

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

    cp "$BUILD_DIR/pulse-host-agent-$build_name" "$universal_dir/bin/pulse-host-agent-${build_name}"
    cp "$BUILD_DIR/pulse-agent-$build_name" "$universal_dir/bin/pulse-agent-${build_name}"
    cp "$BUILD_DIR/pulse-sensor-proxy-$build_name" "$universal_dir/bin/pulse-sensor-proxy-${build_name}"
done

cp "scripts/install-docker-agent.sh" "$universal_dir/scripts/install-docker-agent.sh"
cp "scripts/install-container-agent.sh" "$universal_dir/scripts/install-container-agent.sh"
cp "scripts/install-host-agent.ps1" "$universal_dir/scripts/install-host-agent.ps1"
cp "scripts/uninstall-host-agent.sh" "$universal_dir/scripts/uninstall-host-agent.sh"
cp "scripts/uninstall-host-agent.ps1" "$universal_dir/scripts/uninstall-host-agent.ps1"
cp "scripts/install-sensor-proxy.sh" "$universal_dir/scripts/install-sensor-proxy.sh"
cp "scripts/install-docker.sh" "$universal_dir/scripts/install-docker.sh"
cp "scripts/install.sh" "$universal_dir/scripts/install.sh"
[ -f "scripts/install.ps1" ] && cp "scripts/install.ps1" "$universal_dir/scripts/install.ps1"
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


cat > "$universal_dir/bin/pulse-sensor-proxy" << 'EOF'
#!/bin/sh
# Auto-detect architecture and run appropriate pulse-sensor-proxy binary

ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)
        exec "$(dirname "$0")/pulse-sensor-proxy-linux-amd64" "$@"
        ;;
    aarch64|arm64)
        exec "$(dirname "$0")/pulse-sensor-proxy-linux-arm64" "$@"
        ;;
    armv7l|armhf)
        exec "$(dirname "$0")/pulse-sensor-proxy-linux-armv7" "$@"
        ;;
    *)
        echo "Unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac
EOF
chmod +x "$universal_dir/bin/pulse-sensor-proxy"

cat > "$universal_dir/bin/pulse-host-agent" << 'EOF'
#!/bin/sh
# Auto-detect architecture and run appropriate pulse-host-agent binary

ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)
        exec "$(dirname "$0")/pulse-host-agent-linux-amd64" "$@"
        ;;
    aarch64|arm64)
        exec "$(dirname "$0")/pulse-host-agent-linux-arm64" "$@"
        ;;
    armv7l|armhf)
        exec "$(dirname "$0")/pulse-host-agent-linux-armv7" "$@"
        ;;
    *)
        echo "Unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac
EOF
chmod +x "$universal_dir/bin/pulse-host-agent"

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

# Package standalone host agent binaries (all platforms)
# Linux
tar -czf "$RELEASE_DIR/pulse-host-agent-v${VERSION}-linux-amd64.tar.gz" -C "$BUILD_DIR" pulse-host-agent-linux-amd64
tar -czf "$RELEASE_DIR/pulse-host-agent-v${VERSION}-linux-arm64.tar.gz" -C "$BUILD_DIR" pulse-host-agent-linux-arm64
tar -czf "$RELEASE_DIR/pulse-host-agent-v${VERSION}-linux-armv7.tar.gz" -C "$BUILD_DIR" pulse-host-agent-linux-armv7
tar -czf "$RELEASE_DIR/pulse-host-agent-v${VERSION}-linux-armv6.tar.gz" -C "$BUILD_DIR" pulse-host-agent-linux-armv6
tar -czf "$RELEASE_DIR/pulse-host-agent-v${VERSION}-linux-386.tar.gz" -C "$BUILD_DIR" pulse-host-agent-linux-386
# Darwin
tar -czf "$RELEASE_DIR/pulse-host-agent-v${VERSION}-darwin-amd64.tar.gz" -C "$BUILD_DIR" pulse-host-agent-darwin-amd64
tar -czf "$RELEASE_DIR/pulse-host-agent-v${VERSION}-darwin-arm64.tar.gz" -C "$BUILD_DIR" pulse-host-agent-darwin-arm64
# FreeBSD
tar -czf "$RELEASE_DIR/pulse-host-agent-v${VERSION}-freebsd-amd64.tar.gz" -C "$BUILD_DIR" pulse-host-agent-freebsd-amd64
tar -czf "$RELEASE_DIR/pulse-host-agent-v${VERSION}-freebsd-arm64.tar.gz" -C "$BUILD_DIR" pulse-host-agent-freebsd-arm64
# Windows
zip -j "$RELEASE_DIR/pulse-host-agent-v${VERSION}-windows-amd64.zip" "$BUILD_DIR/pulse-host-agent-windows-amd64.exe"
zip -j "$RELEASE_DIR/pulse-host-agent-v${VERSION}-windows-arm64.zip" "$BUILD_DIR/pulse-host-agent-windows-arm64.exe"
zip -j "$RELEASE_DIR/pulse-host-agent-v${VERSION}-windows-386.zip" "$BUILD_DIR/pulse-host-agent-windows-386.exe"

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

# Also copy bare Windows EXEs for /releases/latest/download/ redirect compatibility
# These allow LXC/barebone installs to redirect to GitHub without needing versioned URLs
echo "Copying bare Windows EXEs to release directory for redirect compatibility..."
cp "$BUILD_DIR/pulse-agent-windows-amd64.exe" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-agent-windows-arm64.exe" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-agent-windows-386.exe" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-host-agent-windows-amd64.exe" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-host-agent-windows-arm64.exe" "$RELEASE_DIR/"
cp "$BUILD_DIR/pulse-host-agent-windows-386.exe" "$RELEASE_DIR/"

# Copy Windows, macOS, and FreeBSD binaries into universal tarball for /download/ endpoint
echo "Adding Windows, macOS, and FreeBSD binaries to universal tarball..."
cp "$BUILD_DIR/pulse-host-agent-darwin-amd64" "$universal_dir/bin/"
cp "$BUILD_DIR/pulse-host-agent-darwin-arm64" "$universal_dir/bin/"
cp "$BUILD_DIR/pulse-host-agent-freebsd-amd64" "$universal_dir/bin/"
cp "$BUILD_DIR/pulse-host-agent-freebsd-arm64" "$universal_dir/bin/"
cp "$BUILD_DIR/pulse-host-agent-windows-amd64.exe" "$universal_dir/bin/"
cp "$BUILD_DIR/pulse-host-agent-windows-arm64.exe" "$universal_dir/bin/"
cp "$BUILD_DIR/pulse-host-agent-windows-386.exe" "$universal_dir/bin/"

cp "$BUILD_DIR/pulse-agent-darwin-amd64" "$universal_dir/bin/"
cp "$BUILD_DIR/pulse-agent-darwin-arm64" "$universal_dir/bin/"
cp "$BUILD_DIR/pulse-agent-freebsd-amd64" "$universal_dir/bin/"
cp "$BUILD_DIR/pulse-agent-freebsd-arm64" "$universal_dir/bin/"
cp "$BUILD_DIR/pulse-agent-windows-amd64.exe" "$universal_dir/bin/"
cp "$BUILD_DIR/pulse-agent-windows-arm64.exe" "$universal_dir/bin/"
cp "$BUILD_DIR/pulse-agent-windows-386.exe" "$universal_dir/bin/"

# Create symlinks for Windows binaries without .exe extension (required for download endpoint)
ln -s pulse-host-agent-windows-amd64.exe "$universal_dir/bin/pulse-host-agent-windows-amd64"
ln -s pulse-host-agent-windows-arm64.exe "$universal_dir/bin/pulse-host-agent-windows-arm64"
ln -s pulse-host-agent-windows-386.exe "$universal_dir/bin/pulse-host-agent-windows-386"

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
cp install.sh "$RELEASE_DIR/"
cp scripts/install-sensor-proxy.sh "$RELEASE_DIR/"
cp scripts/install-docker.sh "$RELEASE_DIR/"
cp scripts/pulse-auto-update.sh "$RELEASE_DIR/"

# Generate checksums (include tarballs, zip files, helm chart, and install.sh)
cd "$RELEASE_DIR"
shopt -s nullglob extglob
# Match all tarballs, zip files, exe files, and install scripts
checksum_files=( *.tar.gz *.zip *.exe install.sh install-sensor-proxy.sh install-docker.sh pulse-auto-update.sh )
if compgen -G "pulse-*.tgz" > /dev/null; then
    checksum_files+=( pulse-*.tgz )
fi
if [ ${#checksum_files[@]} -eq 0 ]; then
    echo "Warning: no release artifacts found to checksum."
else
    # Generate checksums from a single sha256sum run for deterministic results (prevents #671 checksum mismatches)
    checksum_output="$(sha256sum "${checksum_files[@]}" | sort -k 2)"
    printf '%s\n' "$checksum_output" > checksums.txt

    # Emit per-file .sha256 artifacts for backward compatibility while legacy installers transition off them
    while read -r checksum filename; do
        printf '%s  %s\n' "$checksum" "$filename" > "${filename}.sha256"
    done <<< "$checksum_output"

    if [ -n "${SIGNING_KEY_ID:-}" ]; then
        if command -v gpg >/dev/null 2>&1; then
            echo "Signing checksums with GPG key ${SIGNING_KEY_ID}..."
            gpg --batch --yes --detach-sign --armor \
                --local-user "${SIGNING_KEY_ID}" \
                --output checksums.txt.asc \
                checksums.txt
        else
            echo "SIGNING_KEY_ID is set but gpg is not installed; skipping signature."
        fi
    fi
fi
shopt -u nullglob
cd ..

echo
echo "Release build complete!"
echo "Archives created in $RELEASE_DIR/"
ls -lh $RELEASE_DIR/

# Create host-agent manifest (per tarball) for validation/debugging
manifest_path="$RELEASE_DIR/host-agent-manifest.json"
echo "Generating host-agent manifest at $manifest_path..."
python3 - <<'EOF' "$RELEASE_DIR" "$VERSION" "$manifest_path"
import json
import os
import sys
import tarfile

release_dir = sys.argv[1]
version = sys.argv[2]
manifest_path = sys.argv[3]

tar_arches = [
    "linux-amd64",
    "linux-arm64",
    "linux-armv7",
    "linux-armv6",
    "linux-386",
]

host_agents = [
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
]

manifest = {
    "version": version,
    "tarballs": {},
    "universal": [],
}

def collect_agents(tar_path):
    found = []
    try:
        with tarfile.open(tar_path, "r:gz") as tf:
            names = set(m.name for m in tf.getmembers() if (m.isfile() or m.issym()))
            for agent in host_agents:
                target = f"./bin/{agent}"
                if target in names:
                    found.append(agent)
    except Exception as exc:
        print(f"Failed to read {tar_path}: {exc}", file=sys.stderr)
    return sorted(found)

# Platform tarballs
for arch in tar_arches:
    tarball = os.path.join(release_dir, f"pulse-v{version}-{arch}.tar.gz")
    if os.path.exists(tarball):
        manifest["tarballs"][arch] = collect_agents(tarball)

# Universal tarball
universal_tar = os.path.join(release_dir, f"pulse-v{version}.tar.gz")
if os.path.exists(universal_tar):
    manifest["universal"] = collect_agents(universal_tar)

with open(manifest_path, "w", encoding="utf-8") as handle:
    json.dump(manifest, handle, indent=2)

print(json.dumps(manifest, indent=2))
EOF
