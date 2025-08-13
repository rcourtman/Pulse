#!/bin/bash

# Complete development environment for Pulse
# Watches both frontend and backend, rebuilds as needed

echo "Starting Pulse development environment..."
echo "This script watches both frontend and backend files"
echo "Press Ctrl+C to stop"

# Use Go 1.23 if available
if [ -x /usr/local/go/bin/go ]; then
    export PATH=/usr/local/go/bin:$PATH
fi

cd /opt/pulse

# Function to build frontend
build_frontend() {
    echo "Building frontend..."
    cd /opt/pulse/frontend-modern
    npm run build
    cd /opt/pulse
    rm -rf internal/api/frontend-modern/dist
    cp -r frontend-modern/dist internal/api/frontend-modern/
    echo "Frontend built and copied"
}

# Function to build backend
build_backend() {
    echo "Building backend..."
    go build -o pulse ./cmd/pulse
    echo "Backend built"
}

# Function to restart pulse
restart_pulse() {
    if [ -n "$PULSE_PID" ]; then
        echo "Stopping Pulse..."
        kill $PULSE_PID 2>/dev/null
        wait $PULSE_PID 2>/dev/null
    fi
    echo "Starting Pulse..."
    ./pulse &
    PULSE_PID=$!
    echo "Pulse running with PID $PULSE_PID"
}

# Initial build
build_frontend
build_backend
restart_pulse

# Cleanup on exit
trap "kill $PULSE_PID 2>/dev/null; exit" INT TERM

# Start watching in parallel
(
    # Watch frontend files
    inotifywait -m -r -e modify,create,delete \
        --exclude 'node_modules|\.git|dist' \
        frontend-modern/src frontend-modern/index.html 2>/dev/null |
    while read -r directory event filename; do
        echo "Frontend change detected: $filename"
        build_frontend
        build_backend
        restart_pulse
    done
) &

(
    # Watch Go files
    inotifywait -m -r -e modify,create,delete \
        --include '\.go$' \
        --exclude 'vendor|\.git' . 2>/dev/null |
    while read -r directory event filename; do
        if [[ "$filename" =~ \.go$ ]]; then
            echo "Backend change detected: $filename"
            build_backend
            restart_pulse
        fi
    done
) &

# Wait for watchers
wait