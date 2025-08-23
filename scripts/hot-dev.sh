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

# Kill any existing Pulse processes (but NOT ttyd/tmux which run Claude Code!)
sudo systemctl stop pulse-backend 2>/dev/null
# Kill the backend-watch script to free up port 7655
pkill -f "backend-watch.sh" 2>/dev/null
# Use exact match to only kill the "pulse" binary, not processes running FROM /opt/pulse
pkill -x "pulse" 2>/dev/null
# Give services time to fully stop
sleep 1

# Start backend on port 7656 (one port up from normal)
echo "Starting backend on port 7656..."
cd /opt/pulse
if [ ! -f pulse ] || [ pulse -ot cmd/pulse/main.go ]; then
    echo "Building backend..."
    go build -o pulse ./cmd/pulse
fi
PORT=7656 ./pulse &
BACKEND_PID=$!

# Wait for backend to start
sleep 2

# Create temporary vite config for development
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
    kill $BACKEND_PID 2>/dev/null
    rm -f vite.config.dev.ts
    echo "Restarting backend-watch service..."
    sudo systemctl start pulse-backend 2>/dev/null
    exit
}
trap cleanup INT TERM

# Start vite dev server with hot-reload
echo "Starting frontend with hot-reload on port 7655..."
npx vite --config vite.config.dev.ts --clearScreen false

cleanup