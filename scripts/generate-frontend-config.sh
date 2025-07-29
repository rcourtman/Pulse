#!/bin/bash
# Generate frontend configuration based on environment variables

# Default values
FRONTEND_PORT="${PULSE_SERVER_FRONTEND_PORT:-7655}"
FRONTEND_HOST="${PULSE_SERVER_FRONTEND_HOST:-0.0.0.0}"
BACKEND_PORT="${PULSE_SERVER_BACKEND_PORT:-3000}"

# Create dynamic vite config
cat > /opt/pulse/frontend-modern/vite.config.dynamic.ts << EOF
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
    port: ${FRONTEND_PORT},
    host: '${FRONTEND_HOST}',
    proxy: {
      '/ws': {
        target: 'ws://127.0.0.1:${BACKEND_PORT}',
        ws: true,
        changeOrigin: true,
      },
      '/api': {
        target: 'http://127.0.0.1:${BACKEND_PORT}',
        changeOrigin: true,
      },
    },
  },
  build: {
    target: 'esnext',
  },
});
EOF

echo "Frontend config generated with port ${FRONTEND_PORT}"