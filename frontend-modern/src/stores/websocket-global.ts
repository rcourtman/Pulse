import { createWebSocketStore } from './websocket';
import { getPulseWebSocketUrl } from '@/utils/url';

// Store the instance on window to survive hot reloads
declare global {
  interface Window {
    __pulseWsStore?: ReturnType<typeof createWebSocketStore>;
  }
}

export function getGlobalWebSocketStore() {
  if (!window.__pulseWsStore) {
    const wsUrl = getPulseWebSocketUrl();

    window.__pulseWsStore = createWebSocketStore(wsUrl);
  }

  return window.__pulseWsStore;
}
