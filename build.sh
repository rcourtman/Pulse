#!/bin/bash

# Build script for Pulse that ensures frontend is in the right place for embedding

set -e

echo "Building Pulse..."

# Build frontend
echo "Building frontend..."
cd frontend-modern
npm run build

# Ensure the embed directory exists and is up to date
echo "Preparing frontend for embedding..."
mkdir -p ../internal/api/frontend-modern
rm -rf ../internal/api/frontend-modern/dist
cp -r dist ../internal/api/frontend-modern/

# Build backend with embedded frontend
echo "Building backend..."
cd ..
go build -o pulse ./cmd/pulse

echo "Build complete!"
echo "To run: ./pulse or sudo systemctl restart pulse-backend"