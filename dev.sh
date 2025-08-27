#!/bin/bash

# Development script with hot-reload
cd /opt/pulse || exit 1

echo "Starting Pulse development mode..."
echo "Backend will auto-restart on .go file changes"
echo ""
echo "Press Ctrl+C to stop"
echo ""

# Start the backend-watch script
exec ./scripts/backend-watch.sh