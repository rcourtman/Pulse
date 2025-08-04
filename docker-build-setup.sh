#!/bin/bash

# Docker build setup script for Pulse v4.0.0

echo "This script will set up Docker login and build multi-arch images for Pulse v4.0.0"
echo "Container IP: 192.168.0.174"
echo ""

# SSH into container and setup Docker login
echo "Setting up Docker login..."
ssh root@192.168.0.174 'docker login -u rcourtman'

if [ $? -ne 0 ]; then
    echo "Docker login failed. Exiting."
    exit 1
fi

echo ""
echo "Docker login successful! Now building and pushing images..."
echo "This will take several minutes as it builds for multiple architectures..."

# Build and push multi-arch images
ssh root@192.168.0.174 'cd /root/Pulse && docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t rcourtman/pulse:v4.0.0 \
  -t rcourtman/pulse:4.0.0 \
  -t rcourtman/pulse:4.0 \
  -t rcourtman/pulse:4 \
  -t rcourtman/pulse:latest \
  --push .'

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Docker images successfully built and pushed!"
    echo ""
    echo "Available tags:"
    echo "  - rcourtman/pulse:latest"
    echo "  - rcourtman/pulse:4"
    echo "  - rcourtman/pulse:4.0"
    echo "  - rcourtman/pulse:4.0.0"
    echo "  - rcourtman/pulse:v4.0.0"
    echo ""
    echo "Architectures: linux/amd64, linux/arm64, linux/arm/v7"
else
    echo "❌ Docker build failed!"
    exit 1
fi