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
architectures=(
    "linux/amd64"
    "linux/arm64"
    "linux/arm/v7"
)

for arch in "${architectures[@]}"; do
    echo "Building for $arch..."
    
    os=$(echo $arch | cut -d'/' -f1)
    arch_name=$(echo $arch | cut -d'/' -f2)
    
    # Convert arch names
    case $arch_name in
        "arm/v7") arch_suffix="armv7" ;;
        *) arch_suffix=$arch_name ;;
    esac
    
    # Build binary
    GOOS=$os GOARCH=${arch_name%/*} GOARM=${arch_name#*/} \
        go build -ldflags="-s -w -X main.version=${VERSION}" \
        -trimpath -o $BUILD_DIR/pulse-$os-$arch_suffix ./cmd/pulse
    
    # Create release archive
    tar_name="pulse-v${VERSION}-${os}-${arch_suffix}.tar.gz"
    
    cd $BUILD_DIR
    tar -czf ../$RELEASE_DIR/$tar_name \
        pulse-$os-$arch_suffix \
        ../frontend-modern/dist \
        ../README.md \
        ../LICENSE \
        ../pulse.service \
        ../install.sh
    cd ..
    
    echo "Created $RELEASE_DIR/$tar_name"
done

# Create checksums
cd $RELEASE_DIR
sha256sum *.tar.gz > checksums.txt

echo "Release build complete! Files in $RELEASE_DIR/"