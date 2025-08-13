#!/bin/bash

# Development setup with hot-reload
# Frontend runs on Vite dev server with hot-reload
# Backend runs separately, frontend proxies API calls to it

echo "Starting Pulse development environment with hot-reload..."
echo "Frontend will be available at http://localhost:5173 with instant updates"
echo "Press Ctrl+C to stop"

# Kill any existing processes
pkill -f "vite" 2>/dev/null
pkill -f "pulse" 2>/dev/null

# Start backend in background
echo "Starting backend..."
cd /opt/pulse
go build -o pulse ./cmd/pulse
./pulse &
BACKEND_PID=$!

# Start frontend dev server with hot-reload
echo "Starting frontend with hot-reload..."
cd /opt/pulse/frontend-modern

# Update vite config to proxy API calls to backend
cat > vite.config.ts.tmp << 'EOF'
import { defineConfig } from 'vite';
import solidPlugin from 'vite-plugin-solid';
import path from 'path';

export default defineConfig({
  plugins: [solidPlugin()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:7655',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:7655',
        ws: true,
      },
    },
  },
});
EOF

mv vite.config.ts vite.config.ts.bak
mv vite.config.ts.tmp vite.config.ts

# Cleanup function
cleanup() {
    echo "Stopping services..."
    kill $BACKEND_PID 2>/dev/null
    pkill -f "vite" 2>/dev/null
    mv vite.config.ts.bak vite.config.ts 2>/dev/null
    exit
}

trap cleanup INT TERM

# Start vite dev server
npm run dev &

# Wait
wait