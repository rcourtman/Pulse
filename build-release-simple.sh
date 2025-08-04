#!/usr/bin/env bash

# Simple build script for Pulse release (amd64 only)
set -euo pipefail

VERSION=$(cat VERSION)
echo "Building Pulse v${VERSION} for linux/amd64..."

# Clean and create directories
rm -rf bin/ pulse-release/
mkdir -p bin/

# Build frontend if needed
if [ ! -d "frontend-modern/dist" ] || [ "${1:-}" == "--rebuild-frontend" ]; then
    echo "Building frontend..."
    cd frontend-modern
    npm run build
    cd ..
fi

# Build Go binary
echo "Building Go binary..."
go build -ldflags="-s -w" -trimpath -o bin/pulse ./cmd/pulse

# Test binary
echo "Testing binary..."
./bin/pulse --version || echo "Version flag not implemented, but binary built"

echo "Build complete! Binary at: bin/pulse"
ls -lh bin/pulse