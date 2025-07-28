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
    port: 3001,
    host: '0.0.0.0', // Listen on all interfaces for remote access
    proxy: {
      '/ws': {
        target: 'ws://127.0.0.1:3000',
        ws: true,
        changeOrigin: true,
      },
      '/api': {
        target: 'http://127.0.0.1:3000',
        changeOrigin: true,
      },
    },
  },
  build: {
    target: 'esnext',
  },
});