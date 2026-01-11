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
  7654,
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
        // Browser WebSocket connections are long-lived, disable timeouts
        proxyTimeout: 0,
        timeout: 0,
        configure: (proxy, _options) => {
          proxy.options.timeout = 0;
          proxy.options.proxyTimeout = 0;

          proxy.on('proxyReqWs', (proxyReq, req, socket) => {
            socket.setTimeout(0);
            socket.setNoDelay(true);
            socket.setKeepAlive(true, 30000);
          });
          proxy.on('open', (proxySocket) => {
            proxySocket.setTimeout(0);
            proxySocket.setNoDelay(true);
            proxySocket.setKeepAlive(true, 30000);
          });
        },
      },
      // SSE endpoint for AI chat streaming
      '/api/ai/execute/stream': {
        target: backendUrl,
        changeOrigin: true,
        // SSE requires special handling to prevent proxy timeouts
        // Set timeout to 0 to completely disable
        timeout: 0,
        proxyTimeout: 0,
        configure: (proxy, _options) => {
          // Completely disable http-proxy internal timeouts
          proxy.options.timeout = 0;
          proxy.options.proxyTimeout = 0;

          // Set proxy-level timeouts
          proxy.on('proxyReq', (proxyReq, req, res) => {
            // Disable socket timeouts for SSE
            req.socket.setTimeout(0);
            req.socket.setNoDelay(true);
            req.socket.setKeepAlive(true, 30000);
            // Also set on the proxy request
            proxyReq.socket?.setTimeout(0);
          });
          proxy.on('proxyRes', (proxyRes, req, res) => {
            // Disable response socket timeout
            res.socket?.setTimeout(0);
            res.socket?.setNoDelay(true);
            res.socket?.setKeepAlive(true, 30000);
            // Also disable on proxy response socket
            proxyRes.socket?.setTimeout(0);
          });
          proxy.on('error', (err, req, res) => {
            console.error('[SSE Proxy Error]', err.message);
          });
        },
      },
      // SSE endpoint for AI alert investigation (one-click investigate from alerts page)
      '/api/ai/investigate-alert': {
        target: backendUrl,
        changeOrigin: true,
        // SSE requires special handling to prevent proxy timeouts
        timeout: 0,
        proxyTimeout: 0,
        configure: (proxy, _options) => {
          proxy.options.timeout = 0;
          proxy.options.proxyTimeout = 0;

          proxy.on('proxyReq', (proxyReq, req, res) => {
            req.socket.setTimeout(0);
            req.socket.setNoDelay(true);
            req.socket.setKeepAlive(true, 30000);
            proxyReq.socket?.setTimeout(0);
          });
          proxy.on('proxyRes', (proxyRes, req, res) => {
            res.socket?.setTimeout(0);
            res.socket?.setNoDelay(true);
            res.socket?.setKeepAlive(true, 30000);
            proxyRes.socket?.setTimeout(0);
          });
          proxy.on('error', (err, req, res) => {
            console.error('[SSE Proxy Error - Investigate Alert]', err.message);
          });
        },
      },
      // SSE endpoint for AI patrol streaming
      '/api/ai/patrol/stream': {
        target: backendUrl,
        changeOrigin: true,
        timeout: 0,
        proxyTimeout: 0,
        configure: (proxy, _options) => {
          proxy.options.timeout = 0;
          proxy.options.proxyTimeout = 0;

          proxy.on('proxyReq', (proxyReq, req, res) => {
            req.socket.setTimeout(0);
            req.socket.setNoDelay(true);
            req.socket.setKeepAlive(true, 30000);
            proxyReq.socket?.setTimeout(0);
          });
          proxy.on('proxyRes', (proxyRes, req, res) => {
            res.socket?.setTimeout(0);
            res.socket?.setNoDelay(true);
            res.socket?.setKeepAlive(true, 30000);
            proxyRes.socket?.setTimeout(0);
          });
          proxy.on('error', (err, req, res) => {
            console.error('[SSE Proxy Error - Patrol Stream]', err.message);
          });
        },
      },
      '/api/agent/ws': {
        target: backendWsUrl,
        ws: true,
        changeOrigin: true,
        // Agent WebSocket connections are long-lived, disable timeouts
        // proxyTimeout: 0 disables the proxy-to-target timeout
        // timeout: 0 disables the client-to-proxy timeout
        proxyTimeout: 0,
        timeout: 0,
        configure: (proxy, _options) => {
          // Disable http-proxy's internal timeout (default is 2 minutes but seems to be 10s)
          proxy.options.timeout = 0;
          proxy.options.proxyTimeout = 0;

          proxy.on('proxyReqWs', (proxyReq, req, socket) => {
            // Disable socket timeouts for WebSocket connections
            socket.setTimeout(0);
            socket.setNoDelay(true);
            socket.setKeepAlive(true, 30000);
          });
          proxy.on('open', (proxySocket) => {
            // Also disable timeout on the proxy socket
            proxySocket.setTimeout(0);
            proxySocket.setNoDelay(true);
            proxySocket.setKeepAlive(true, 30000);
          });
          proxy.on('error', (err, req, res) => {
            console.error('[Agent WS Proxy Error]', err.message);
          });
        },
      },
      '/api': {
        target: backendUrl,
        changeOrigin: true,
        cookieDomainRewrite: '',
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
