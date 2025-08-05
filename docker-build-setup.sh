#!/bin/bash

# Docker Build Commands for Pulse v4.0.1
# Run these commands on a system with Docker and buildx configured

# Ensure you're in the Pulse directory with the latest code
git pull

# Login to Docker Hub (if not already logged in)
docker login

# Create and use a buildx builder if needed
docker buildx create --name multiarch --use || docker buildx use multiarch

# Build and push multi-architecture images for v4.0.1
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t rcourtman/pulse:v4.0.1 \
  -t rcourtman/pulse:4.0.1 \
  -t rcourtman/pulse:4.0 \
  -t rcourtman/pulse:4 \
  -t rcourtman/pulse:latest \
  --push .

# Verify the images were pushed
echo "Verifying images on Docker Hub..."
docker pull rcourtman/pulse:v4.0.1
docker images | grep pulse

echo "Docker images for v4.0.1 have been built and pushed!"
echo "Check them at: https://hub.docker.com/r/rcourtman/pulse/tags"