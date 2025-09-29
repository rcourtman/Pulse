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
    // Use relative URL that works behind proxies
    const wsUrl = `${protocol}//${window.location.host}/ws`;

    window.__pulseWsStore = createWebSocketStore(wsUrl);
  }

  return window.__pulseWsStore;
}
