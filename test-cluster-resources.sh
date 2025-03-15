#!/bin/bash

# Copy the test environment file to .env
cp .env.cluster-test .env

# Explicitly set debug logging and ensure cluster resources is enabled
echo "LOG_LEVEL=debug" >> .env
echo "PROXMOX_USE_CLUSTER_RESOURCES=true" >> .env

# Start the development server
echo "Starting development server with mock data and cluster resources endpoint enabled..."
npm run dev
