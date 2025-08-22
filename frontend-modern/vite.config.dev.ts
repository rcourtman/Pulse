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
