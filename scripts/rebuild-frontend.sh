#!/bin/bash

# Script to rebuild frontend for development with embedded frontend

set -e

cd /opt/pulse

echo "Building frontend..."
cd frontend-modern
npm run build
cd ..

echo "Copying frontend for embedding..."
rm -rf internal/api/frontend-modern
cp -r frontend-modern internal/api/

echo "Rebuilding Go binary with embedded frontend..."
go build -o pulse ./cmd/pulse

echo "Restarting service..."
sudo systemctl restart pulse-backend

echo "Frontend rebuild complete!"