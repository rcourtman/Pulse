import { defineConfig } from 'vite';
import solid from 'vite-plugin-solid';
import path from 'path';
import { URL } from 'node:url';

const frontendDevHost = process.env.FRONTEND_DEV_HOST ?? '0.0.0.0';
const frontendDevPort = Number(
  process.env.FRONTEND_DEV_PORT ?? process.env.VITE_PORT ?? process.env.PORT ?? 5173,
);

const backendProtocol = process.env.PULSE_DEV_API_PROTOCOL ?? 'http';
const backendHost = process.env.PULSE_DEV_API_HOST ?? '127.0.0.1';
const backendPort = Number(
  process.env.PULSE_DEV_API_PORT ??
  process.env.FRONTEND_PORT ??
  process.env.PORT ??
  7655,
);

const backendUrl =
  process.env.PULSE_DEV_API_URL ?? `${backendProtocol}://${backendHost}:${backendPort}`;

const backendWsUrl =
  process.env.PULSE_DEV_WS_URL ??
  (() => {
    try {
      const parsed = new URL(backendUrl);
      parsed.protocol = parsed.protocol === 'https:' ? 'wss:' : 'ws:';
      return parsed.toString();
    } catch {
      return backendUrl
        .replace(/^http:\/\//i, 'ws://')
        .replace(/^https:\/\//i, 'wss://');
    }
  })();

export default defineConfig({
  plugins: [solid()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
    conditions: ['import', 'browser', 'default'],
  },
  server: {
    port: frontendDevPort,
    host: frontendDevHost, // Listen on all interfaces for remote access
    strictPort: true,
    proxy: {
      '/ws': {
        target: backendWsUrl,
        ws: true,
        changeOrigin: true,
      },
      '/api/ai/execute/stream': {
        target: backendUrl,
        changeOrigin: true,
        // SSE requires special handling to prevent proxy timeouts
        // Set timeout to 10 minutes (600000ms) for long-running AI requests
        timeout: 600000,
        proxyTimeout: 600000,
        configure: (proxy, _options) => {
          // Set proxy-level timeouts
          proxy.on('proxyReq', (proxyReq, req, res) => {
            // Disable socket timeouts for SSE
            req.socket.setTimeout(0);
            req.socket.setNoDelay(true);
            req.socket.setKeepAlive(true);
            // Also set on the proxy request
            proxyReq.socket?.setTimeout(0);
          });
          proxy.on('proxyRes', (proxyRes, req, res) => {
            // Disable response socket timeout
            res.socket?.setTimeout(0);
            res.socket?.setNoDelay(true);
            res.socket?.setKeepAlive(true);
            // Also disable on proxy response socket
            proxyRes.socket?.setTimeout(0);
          });
          proxy.on('error', (err, req, res) => {
            console.error('[SSE Proxy Error]', err.message);
          });
        },
      },
      '/api/agent/ws': {
        target: backendWsUrl,
        ws: true,
        changeOrigin: true,
      },
      '/api': {
        target: backendUrl,
        changeOrigin: true,
      },
      '/install-docker-agent.sh': {
        target: backendUrl,
        changeOrigin: true,
      },
      '/install-container-agent.sh': {
        target: backendUrl,
        changeOrigin: true,
      },
      '/install-host-agent.sh': {
        target: backendUrl,
        changeOrigin: true,
      },
      '/install-host-agent.ps1': {
        target: backendUrl,
        changeOrigin: true,
      },
      '/install.sh': {
        target: backendUrl,
        changeOrigin: true,
      },
      '/install.ps1': {
        target: backendUrl,
        changeOrigin: true,
      },
      '/download': {
        target: backendUrl,
        changeOrigin: true,
      },
    },
  },
  build: {
    target: 'esnext',
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
  },
});
