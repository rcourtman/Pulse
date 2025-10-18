#!/usr/bin/env bash

# Build script for Pulse releases
# Creates release archives for different architectures

set -euo pipefail

# Use Go 1.24 if available
if [ -x /usr/local/go/bin/go ]; then
    export PATH=/usr/local/go/bin:$PATH
fi

VERSION=${1:-$(cat VERSION)}
BUILD_DIR="build"
RELEASE_DIR="release"

echo "Building Pulse v${VERSION}..."

# Clean previous builds
rm -rf $BUILD_DIR $RELEASE_DIR
mkdir -p $BUILD_DIR $RELEASE_DIR

# Build frontend
echo "Building frontend..."
cd frontend-modern
npm ci
npm run build
cd ..

# Copy frontend dist for embedding (required for Go embed)
echo "Copying frontend dist for embedding..."
rm -rf internal/api/frontend-modern
mkdir -p internal/api/frontend-modern
cp -r frontend-modern/dist internal/api/frontend-modern/

# Build for different architectures
declare -A builds=(
    ["linux-amd64"]="GOOS=linux GOARCH=amd64"
    ["linux-arm64"]="GOOS=linux GOARCH=arm64"
    ["linux-armv7"]="GOOS=linux GOARCH=arm GOARM=7"
)

for build_name in "${!builds[@]}"; do
    echo "Building for $build_name..."
    
    # Get build environment
    build_env="${builds[$build_name]}"
    
    build_time=$(date -u '+%Y-%m-%d_%H:%M:%S')
    git_commit=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')

    # Build backend binary with version info
    env $build_env go build \
        -ldflags="-s -w -X main.Version=v${VERSION} -X main.BuildTime=${build_time} -X main.GitCommit=${git_commit} -X github.com/rcourtman/pulse-go-rewrite/internal/dockeragent.Version=v${VERSION}" \
        -trimpath \
        -o "$BUILD_DIR/pulse-$build_name" \
        ./cmd/pulse

    # Build docker agent binary
    env $build_env go build \
        -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/dockeragent.Version=v${VERSION}" \
        -trimpath \
        -o "$BUILD_DIR/pulse-docker-agent-$build_name" \
        ./cmd/pulse-docker-agent

    # Build temperature proxy binary
    env $build_env go build \
        -ldflags="-s -w -X main.Version=v${VERSION} -X main.BuildTime=${build_time} -X main.GitCommit=${git_commit}" \
        -trimpath \
        -o "$BUILD_DIR/pulse-sensor-proxy-$build_name" \
        ./cmd/pulse-sensor-proxy
    
    # Create release archive with proper structure
    tar_name="pulse-v${VERSION}-${build_name}.tar.gz"
    
    # Create staging directory
    staging_dir="$BUILD_DIR/staging-$build_name"
    rm -rf "$staging_dir"
    mkdir -p "$staging_dir/bin"
    mkdir -p "$staging_dir/scripts"
    
    # Copy binaries and VERSION file
    cp "$BUILD_DIR/pulse-$build_name" "$staging_dir/bin/pulse"
    cp "$BUILD_DIR/pulse-docker-agent-$build_name" "$staging_dir/bin/pulse-docker-agent"
    cp "$BUILD_DIR/pulse-sensor-proxy-$build_name" "$staging_dir/bin/pulse-sensor-proxy"
    cp "scripts/install-docker-agent.sh" "$staging_dir/scripts/install-docker-agent.sh"
    chmod 755 "$staging_dir/scripts/install-docker-agent.sh"
    echo "$VERSION" > "$staging_dir/VERSION"
    
    # Create tarball from staging directory
    cd "$staging_dir"
    tar -czf "../../$RELEASE_DIR/$tar_name" .
    cd ../..
    
    # Cleanup staging
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
for build_name in "${!builds[@]}"; do
    cp "$BUILD_DIR/pulse-$build_name" "$universal_dir/bin/pulse-${build_name}"
    cp "$BUILD_DIR/pulse-docker-agent-$build_name" "$universal_dir/bin/pulse-docker-agent-${build_name}"
    cp "$BUILD_DIR/pulse-sensor-proxy-$build_name" "$universal_dir/bin/pulse-sensor-proxy-${build_name}"
done

cp "scripts/install-docker-agent.sh" "$universal_dir/scripts/install-docker-agent.sh"
chmod 755 "$universal_dir/scripts/install-docker-agent.sh"

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

cat > "$universal_dir/bin/pulse-docker-agent" << 'EOF'
#!/bin/sh
# Auto-detect architecture and run appropriate pulse-docker-agent binary

ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)
        exec "$(dirname "$0")/pulse-docker-agent-linux-amd64" "$@"
        ;;
    aarch64|arm64)
        exec "$(dirname "$0")/pulse-docker-agent-linux-arm64" "$@"
        ;;
    armv7l|armhf)
        exec "$(dirname "$0")/pulse-docker-agent-linux-armv7" "$@"
        ;;
    *)
        echo "Unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac
EOF
chmod +x "$universal_dir/bin/pulse-docker-agent"

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

# Add VERSION file
echo "$VERSION" > "$universal_dir/VERSION"

# Create universal tarball
cd "$universal_dir"
tar -czf "../../$RELEASE_DIR/pulse-v${VERSION}.tar.gz" .
cd ../..

# Cleanup
rm -rf "$universal_dir"

# Copy standalone pulse-sensor-proxy binaries to release directory
# These are needed by install-sensor-proxy.sh installer script
echo "Copying standalone pulse-sensor-proxy binaries..."
for build_name in "${!builds[@]}"; do
    cp "$BUILD_DIR/pulse-sensor-proxy-$build_name" "$RELEASE_DIR/"
done

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

# Generate checksums (include tarballs, helm chart, and standalone binaries)
cd "$RELEASE_DIR"
shopt -s nullglob
checksum_files=( *.tar.gz pulse-sensor-proxy-* )
if compgen -G "pulse-*.tgz" > /dev/null; then
    checksum_files+=( pulse-*.tgz )
fi
if [ ${#checksum_files[@]} -eq 0 ]; then
    echo "Warning: no release artifacts found to checksum."
else
    sha256sum "${checksum_files[@]}" > checksums.txt
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
