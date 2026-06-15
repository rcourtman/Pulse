import { defineConfig } from 'vite';
import solid from 'vite-plugin-solid';
import sri from 'vite-plugin-sri-gen';
import path from 'path';
import { URL } from 'node:url';
import { configDefaults } from 'vitest/config';

const frontendDevHost = process.env.FRONTEND_DEV_HOST ?? '127.0.0.1';
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

// When the dev server runs on a non-default port (e.g. a preview/tooling
// instance beside the managed 5173 one), the backend's dev origin allowlist
// (http://localhost:5173,http://localhost:7655) rejects WebSocket upgrades.
// Setting PULSE_DEV_PROXY_ORIGIN rewrites the proxied Origin header to an
// allowed one. Unset, nothing changes.
const proxyOriginOverride = process.env.PULSE_DEV_PROXY_ORIGIN ?? '';

const srcAlias = path.resolve(__dirname, './src');

export default defineConfig({
  plugins: [solid(), sri({ algorithm: 'sha384' })],
  resolve: {
    alias: {
      '@': srcAlias,
    },
    conditions: ['import', 'browser', 'default'],
  },
  server: {
    port: frontendDevPort,
    host: frontendDevHost,
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
            if (proxyOriginOverride) proxyReq.setHeader('Origin', proxyOriginOverride);
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
      '/auth/license-purchase-start': {
        target: backendUrl,
        changeOrigin: true,
      },
      '/auth/license-purchase-activate': {
        target: backendUrl,
        changeOrigin: true,
      },
      '/install-container-agent.sh': {
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
    rollupOptions: {
      output: {
        // Dynamic-import facades named index.tsx/index.ts all produce chunks
        // called "index", colliding with the real entry chunk in dist/assets
        // and polluting the entry's bundle-size budget line. Name those
        // chunks after the facade's directory instead (Chat, SetupWizard).
        chunkFileNames(chunkInfo) {
          if (chunkInfo.name === 'index' && chunkInfo.facadeModuleId) {
            const dir = chunkInfo.facadeModuleId.split('/').slice(-2, -1)[0];
            if (dir) return `assets/${dir}-[hash].js`;
          }
          return 'assets/[name]-[hash].js';
        },
        manualChunks(id) {
          if (id.includes('node_modules')) {
            if (id.includes('solid-js') || id.includes('@solidjs/router')) return 'vendor-solid';
            if (id.includes('lucide-solid')) return 'vendor-icons';
            if (id.includes('marked') || id.includes('dompurify')) return 'vendor-ai';
            // Only ever dynamically imported (Assistant code-block
            // highlighting on settled turns); folding it into the eager
            // vendor chunk would bill it to first load.
            if (id.includes('highlight.js')) return 'vendor-highlight';
            return 'vendor';
          }

          return undefined;
        },
      },
    },
  },
  optimizeDeps: {
    esbuildOptions: {
      target: 'esnext',
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    alias: {
      '@': srcAlias,
    },
    exclude: [
      ...configDefaults.exclude,
      'tests/integration/**',
      '**/tests/integration/**',
    ],
  },
});
