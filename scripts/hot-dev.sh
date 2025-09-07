#!/bin/bash

# Hot-reload development setup
# Frontend runs on Vite dev server on port 7655 with instant hot-reload
# Backend API runs on port 7656 (one port up)
# Just change frontend code and see changes instantly!

echo "========================================="
echo "Starting HOT-RELOAD development mode"
echo "========================================="
echo ""
echo "Frontend: http://192.168.0.123:7655 (with hot-reload)"
echo "Backend API: http://localhost:7656"
echo ""
echo "Just edit frontend files and see changes instantly!"
echo "Press Ctrl+C to stop"
echo "========================================="

# Function to kill processes on a specific port
kill_port() {
    local port=$1
    echo "Cleaning up port $port..."
    lsof -i :$port | awk 'NR>1 {print $2}' | xargs -r kill -9 2>/dev/null
}

# AGGRESSIVE CLEANUP - We need to ensure ports are free
echo ""
echo "Cleaning up existing processes..."

# Stop systemd services first
sudo systemctl stop pulse-backend 2>/dev/null
sudo systemctl stop pulse 2>/dev/null
sudo systemctl stop pulse-frontend 2>/dev/null  # Stop the frontend service that runs on 7655!

# Kill any backend-watch scripts
pkill -f "backend-watch.sh" 2>/dev/null

# Kill all Vite/npm dev processes
pkill -f vite 2>/dev/null
pkill -f "npm run dev" 2>/dev/null
pkill -f "npm exec" 2>/dev/null

# Kill Pulse binary - first try gracefully, then force
pkill -x "pulse" 2>/dev/null
sleep 1
# Force kill if still running
pkill -9 -x "pulse" 2>/dev/null

# Force-kill ANYTHING on our ports
kill_port 7655
kill_port 7656
kill_port 7657  # Also clean up the port Vite wrongly used

# Wait for everything to properly die
sleep 3

# Double-check ports are free
if lsof -i :7655 | grep -q LISTEN; then
    echo "ERROR: Port 7655 is still in use after cleanup!"
    echo "Attempting more aggressive cleanup..."
    kill_port 7655
    sleep 2
    if lsof -i :7655 | grep -q LISTEN; then
        echo "FATAL: Cannot free port 7655. Please manually kill the process:"
        lsof -i :7655
        exit 1
    fi
fi

if lsof -i :7656 | grep -q LISTEN; then
    echo "ERROR: Port 7656 is still in use after cleanup!"
    echo "Attempting more aggressive cleanup..."
    kill_port 7656
    sleep 2
    if lsof -i :7656 | grep -q LISTEN; then
        echo "FATAL: Cannot free port 7656. Please manually kill the process:"
        lsof -i :7656
        exit 1
    fi
fi

echo "Ports are clean!"
echo ""

# Load mock environment if it exists
if [ -f /opt/pulse/mock.env ]; then
    source /opt/pulse/mock.env
    if [ "$PULSE_MOCK_MODE" = "true" ]; then
        echo "Mock mode ENABLED with $PULSE_MOCK_NODES nodes"
    fi
fi

# Load auth environment if it exists
if [ -f /etc/pulse/.env ]; then
    source /etc/pulse/.env
    echo "Auth configuration loaded from /etc/pulse/.env"
fi

# Start backend on port 7656 (one port up from normal)
echo "Starting backend on port 7656..."
cd /opt/pulse
echo "Building backend (API-only mode for development)..."
# Always rebuild in dev mode to ensure we have the right build
echo "Building with PULSE_MOCK_MODE=${PULSE_MOCK_MODE}"
go build -o pulse ./cmd/pulse
# Export all PULSE_MOCK_* variables for the backend
export PULSE_MOCK_MODE PULSE_MOCK_NODES PULSE_MOCK_VMS_PER_NODE PULSE_MOCK_LXCS_PER_NODE PULSE_MOCK_RANDOM_METRICS PULSE_MOCK_STOPPED_PERCENT
# Export auth variables if set
export PULSE_AUTH_USER PULSE_AUTH_PASS
# Export PORT as well
export PORT=7656
./pulse &
BACKEND_PID=$!

# Wait for backend to start
sleep 2

# Verify backend is running
if ! kill -0 $BACKEND_PID 2>/dev/null; then
    echo "ERROR: Backend failed to start!"
    exit 1
fi

# Create temporary vite config for development with strictPort
cd /opt/pulse/frontend-modern
cat > vite.config.dev.ts << 'EOF'
import { defineConfig } from 'vite';
import solid from 'vite-plugin-solid';
import path from 'path';

export default defineConfig({
  plugins: [solid()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 7655,
    host: '0.0.0.0',
    strictPort: true,  // FAIL if port 7655 is not available
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:7656',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://127.0.0.1:7656',
        ws: true,
        changeOrigin: true,
      },
    },
  },
  build: {
    target: 'esnext',
  },
});
EOF

# Cleanup on exit
cleanup() {
    echo ""
    echo "Stopping services..."
    # Try graceful shutdown first
    if [ -n "$BACKEND_PID" ] && kill -0 $BACKEND_PID 2>/dev/null; then
        kill $BACKEND_PID 2>/dev/null
        sleep 1
        # Force kill if still running
        if kill -0 $BACKEND_PID 2>/dev/null; then
            echo "Backend not responding to SIGTERM, force killing..."
            kill -9 $BACKEND_PID 2>/dev/null
        fi
    fi
    rm -f vite.config.dev.ts
    # Clean up any leftover Vite processes
    pkill -f vite 2>/dev/null
    pkill -f "npm run dev" 2>/dev/null
    # Final cleanup of any stuck pulse processes
    pkill -9 -x "pulse" 2>/dev/null
    echo "Hot-dev stopped. To restart normal service, run: sudo systemctl start pulse-backend"
    exit
}
trap cleanup INT TERM EXIT

# Start vite dev server with hot-reload
echo "Starting frontend with hot-reload on port 7655..."
echo "If this fails, port 7655 is still in use!"
npx vite --config vite.config.dev.ts --clearScreen false

# If we get here, Vite exited unexpectedly
echo "ERROR: Vite exited unexpectedly!"
echo "Dev mode will auto-restart in 5 seconds via systemd..."
cleanup