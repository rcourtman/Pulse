#!/bin/bash

# Test script for update mechanism
# This script enables test mode for the update system

echo "🧪 Pulse Update Test Mode"
echo "========================"
echo ""
echo "This script will start Pulse in update test mode."
echo "In this mode, the application will simulate an available update"
echo "without requiring a real GitHub release."
echo ""
echo "Press Ctrl+C to stop the server."
echo ""

# Export test environment variables
export UPDATE_TEST_MODE=true
export UPDATE_TEST_VERSION=99.99.99

# Optional: Allow custom test version
if [ "$1" ]; then
    export UPDATE_TEST_VERSION=$1
    echo "Using test version: $UPDATE_TEST_VERSION"
else
    echo "Using default test version: 99.99.99"
fi

echo ""
echo "Starting server with update test mode enabled..."
echo ""

# Start the server
cd "$(dirname "$0")/.." || exit
npm run dev:server