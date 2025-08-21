#!/usr/bin/env bash

# Build script for Pulse releases
# Creates release archives for different architectures

set -euo pipefail

# Use Go 1.23 if available
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

# Copy frontend to api directory for embedding
echo "Copying frontend for embedding..."
sudo rm -rf internal/api/frontend-modern
sleep 1  # Give filesystem time to sync
cp -r frontend-modern internal/api/

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
    
    # Build binary with version info
    env $build_env go build \
        -ldflags="-s -w -X main.Version=v${VERSION} -X main.BuildTime=$(date -u '+%Y-%m-%d_%H:%M:%S') -X main.GitCommit=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" \
        -trimpath \
        -o "$BUILD_DIR/pulse-$build_name" \
        ./cmd/pulse
    
    # Create release archive with proper structure
    tar_name="pulse-v${VERSION}-${build_name}.tar.gz"
    
    # Create staging directory
    staging_dir="$BUILD_DIR/staging-$build_name"
    rm -rf "$staging_dir"
    mkdir -p "$staging_dir/bin"
    
    # Copy binary and VERSION file
    cp "$BUILD_DIR/pulse-$build_name" "$staging_dir/bin/pulse"
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

# Copy all binaries to bin/ directory to maintain consistent structure
for build_name in "${!builds[@]}"; do
    cp "$BUILD_DIR/pulse-$build_name" "$universal_dir/bin/"
done

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

# Add VERSION file
echo "$VERSION" > "$universal_dir/VERSION"

# Create universal tarball
cd "$universal_dir"
tar -czf "../../$RELEASE_DIR/pulse-v${VERSION}.tar.gz" .
cd ../..

# Cleanup
rm -rf "$universal_dir"

# Generate checksums
cd $RELEASE_DIR
sha256sum *.tar.gz > checksums.txt
cd ..

echo
echo "Release build complete!"
echo "Archives created in $RELEASE_DIR/"
ls -lh $RELEASE_DIR/