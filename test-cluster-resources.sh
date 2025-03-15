#!/bin/bash

# Copy the test environment file to .env
cp .env.cluster-test .env

# Explicitly set debug logging
echo "LOG_LEVEL=debug" >> .env

# Cluster resources is now enabled by default, no need to set it explicitly

# Start the development server
echo "Starting development server with mock data and cluster resources endpoint enabled (now by default)..."
npm run dev
