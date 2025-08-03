#!/bin/bash
# Test script for Docker build - to be run on a system with Docker

set -e

echo "Testing Pulse Docker build..."

# Build the image
echo "1. Building Docker image..."
docker build -t pulse-test:local .

# Test if the image was created
echo "2. Checking image..."
docker images pulse-test:local

# Run a quick test
echo "3. Testing container startup..."
docker run --rm pulse-test:local ./pulse --help

# Test with compose
echo "4. Testing docker-compose..."
docker-compose config

echo "✅ Docker build test completed successfully!"
echo ""
echo "To run Pulse:"
echo "  docker run -d -p 7655:7655 -v pulse_config:/etc/pulse pulse-test:local"
echo "  OR"
echo "  docker-compose up -d"