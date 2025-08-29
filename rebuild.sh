#!/bin/bash
# Rebuild script for Pulse development

echo "Building frontend..."
cd /opt/pulse/frontend-modern && npm run build

echo "Copying frontend to embed location..."
cp -r /opt/pulse/frontend-modern/dist /opt/pulse/internal/api/frontend-modern/

echo "Building Go binary..."
cd /opt/pulse && go build -o pulse ./cmd/pulse

echo "Restarting service..."
sudo systemctl restart pulse-backend

echo "Done!"
echo "Access Pulse at: http://localhost:7655"