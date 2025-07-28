import { createWebSocketStore } from './websocket';

// Store the instance on window to survive hot reloads
declare global {
  interface Window {
    __pulseWsStore?: ReturnType<typeof createWebSocketStore>;
  }
}

export function getGlobalWebSocketStore() {
  if (!window.__pulseWsStore) {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    // Use backend port 3000 for WebSocket connection
    const hostname = window.location.hostname;
    const wsUrl = `${protocol}//${hostname}:3000/ws`;
    
    window.__pulseWsStore = createWebSocketStore(wsUrl);
  }
  
  return window.__pulseWsStore;
}