#!/bin/bash

# Backend service with auto-rebuild on Go file changes

cd /opt/pulse

# Set config path
export CONFIG_PATH=/etc/pulse

# Initial build
echo "[$(date)] Building Pulse backend..."
go build -o bin/pulse ./cmd/pulse || exit 1

# Function to check if any Go files have changed
check_go_files_changed() {
    find . -name "*.go" -newer bin/pulse 2>/dev/null | grep -q .
}

# Main loop
while true; do
    # Start the backend
    echo "[$(date)] Starting Pulse backend..."
    CONFIG_PATH=/etc/pulse ./bin/pulse &
    BACKEND_PID=$!
    
    # Monitor for changes
    while kill -0 $BACKEND_PID 2>/dev/null; do
        sleep 2
        
        if check_go_files_changed; then
            echo "[$(date)] Go files changed, rebuilding..."
            
            # Kill current process
            kill $BACKEND_PID 2>/dev/null
            wait $BACKEND_PID 2>/dev/null
            
            # Rebuild
            if go build -o bin/pulse ./cmd/pulse; then
                echo "[$(date)] Build successful"
                break
            else
                echo "[$(date)] Build failed, waiting for valid code..."
                sleep 5
            fi
        fi
    done
    
    # Small delay before restart
    sleep 1
done