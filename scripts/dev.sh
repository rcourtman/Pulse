#!/bin/bash

# Simple dev script that runs everything with auto-restart

# Function to kill all child processes on exit
cleanup() {
    echo "Stopping all processes..."
    pkill -P $$ 2>/dev/null
    exit 0
}
trap cleanup EXIT INT TERM

# Start frontend with hot reload (already has it built-in)
echo "Starting frontend..."
(cd frontend-modern && npm run dev) &

# Start backend with auto-restart on Go file changes
echo "Starting backend with auto-restart..."
while true; do
    # Build and run
    go build -o bin/pulse cmd/pulse/main.go && ./bin/pulse &
    BACKEND_PID=$!
    
    # Watch for changes in Go files
    # Using inotifywait if available, otherwise basic sleep loop
    if command -v inotifywait &> /dev/null; then
        # Linux with inotify-tools
        inotifywait -r -e modify,create,delete --include '\.go$' . 2>/dev/null
    else
        # Fallback: check every 2 seconds for changed files
        LAST_MODIFIED=$(find . -name "*.go" -type f -exec stat -c %Y {} \; 2>/dev/null | sort -n | tail -1)
        while kill -0 $BACKEND_PID 2>/dev/null; do
            sleep 2
            CURRENT_MODIFIED=$(find . -name "*.go" -type f -exec stat -c %Y {} \; 2>/dev/null | sort -n | tail -1)
            if [ "$LAST_MODIFIED" != "$CURRENT_MODIFIED" ]; then
                LAST_MODIFIED=$CURRENT_MODIFIED
                break
            fi
        done
    fi
    
    # Kill the backend and restart
    echo "Go files changed, restarting backend..."
    kill $BACKEND_PID 2>/dev/null
    wait $BACKEND_PID 2>/dev/null
    sleep 1
done