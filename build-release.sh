#!/usr/bin/env bash

# Build script for Pulse releases
# Creates release archives for different architectures

set -euo pipefail

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
    
    # Build binary
    env $build_env go build \
        -ldflags="-s -w -X github.com/rcourtman/pulse-go-rewrite/internal/updates.version=${VERSION}" \
        -trimpath \
        -o "$BUILD_DIR/pulse-$build_name" \
        ./cmd/pulse
    
    # Create release archive
    tar_name="pulse-v${VERSION}-${build_name}.tar.gz"
    
    # Create staging directory
    staging_dir="$BUILD_DIR/pulse-staging"
    rm -rf "$staging_dir"
    mkdir -p "$staging_dir"
    
    # Copy files
    cp "$BUILD_DIR/pulse-$build_name" "$staging_dir/pulse"
    mkdir -p "$staging_dir/frontend-modern"
    cp -r frontend-modern/dist "$staging_dir/frontend-modern/"
    cp README.md LICENSE install.sh "$staging_dir/"
    # Note: pulse.service might not exist in Go version
    
    # Create tarball
    cd "$staging_dir"
    tar -czf "../../$RELEASE_DIR/$tar_name" .
    cd ../..
    
    # Cleanup staging
    rm -rf "$staging_dir"
    
    echo "Created $RELEASE_DIR/$tar_name"
done

# Create checksums
cd $RELEASE_DIR
sha256sum *.tar.gz > checksums.txt
cd ..

echo "Release build complete! Files in $RELEASE_DIR/"
ls -lh $RELEASE_DIR/

# Create universal release tarball with all architectures
echo "Creating universal release tarball..."
universal_staging="$BUILD_DIR/pulse-staging"
rm -rf "$universal_staging"
mkdir -p "$universal_staging"

# Copy all architecture binaries
for build_name in "${!builds[@]}"; do
    cp "$BUILD_DIR/pulse-$build_name" "$universal_staging/"
done

# Copy common files
mkdir -p "$universal_staging/frontend-modern"
cp -r frontend-modern/dist "$universal_staging/frontend-modern/"
cp README.md LICENSE install.sh pulse-wrapper.sh "$universal_staging/"
echo "$VERSION" > "$universal_staging/VERSION"

# Rename wrapper to 'pulse' for seamless usage
cp pulse-wrapper.sh "$universal_staging/pulse"
chmod +x "$universal_staging/pulse"

# Create first-run cleanup flag
touch "$universal_staging/.first-run-cleanup"

# Create the universal tarball (this is what the community script expects)
cd "$universal_staging"
tar -czf "../../$RELEASE_DIR/pulse-v${VERSION}.tar.gz" .
cd ../..

# Cleanup
rm -rf "$universal_staging"

echo "Created universal release: $RELEASE_DIR/pulse-v${VERSION}.tar.gz"

# Update checksums
cd $RELEASE_DIR
sha256sum *.tar.gz > checksums.txt
cd ..

echo "Final release contents:"
ls -lh $RELEASE_DIR/