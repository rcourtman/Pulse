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
        -o "$BUILD_DIR/pulse-temp-proxy-$build_name" \
        ./cmd/pulse-temp-proxy
    
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
    cp "$BUILD_DIR/pulse-temp-proxy-$build_name" "$staging_dir/bin/pulse-temp-proxy"
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
    cp "$BUILD_DIR/pulse-temp-proxy-$build_name" "$universal_dir/bin/pulse-temp-proxy-${build_name}"
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

cat > "$universal_dir/bin/pulse-temp-proxy" << 'EOF'
#!/bin/sh
# Auto-detect architecture and run appropriate pulse-temp-proxy binary

ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)
        exec "$(dirname "$0")/pulse-temp-proxy-linux-amd64" "$@"
        ;;
    aarch64|arm64)
        exec "$(dirname "$0")/pulse-temp-proxy-linux-arm64" "$@"
        ;;
    armv7l|armhf)
        exec "$(dirname "$0")/pulse-temp-proxy-linux-armv7" "$@"
        ;;
    *)
        echo "Unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac
EOF
chmod +x "$universal_dir/bin/pulse-temp-proxy"

# Add VERSION file
echo "$VERSION" > "$universal_dir/VERSION"

# Create universal tarball
cd "$universal_dir"
tar -czf "../../$RELEASE_DIR/pulse-v${VERSION}.tar.gz" .
cd ../..

# Cleanup
rm -rf "$universal_dir"

# Copy standalone pulse-temp-proxy binaries to release directory
# These are needed by install-temp-proxy.sh installer script
echo "Copying standalone pulse-temp-proxy binaries..."
for build_name in "${!builds[@]}"; do
    cp "$BUILD_DIR/pulse-temp-proxy-$build_name" "$RELEASE_DIR/"
done

# Generate checksums (include tarballs and standalone binaries)
cd $RELEASE_DIR
sha256sum *.tar.gz pulse-temp-proxy-* > checksums.txt
cd ..

echo
echo "Release build complete!"
echo "Archives created in $RELEASE_DIR/"
ls -lh $RELEASE_DIR/
