import { createWebSocketStore } from './websocket';
import { getPulseWebSocketUrl } from '@/utils/url';

// Store the instance on window to survive hot reloads
declare global {
  interface Window {
    __pulseWsStore?: ReturnType<typeof createWebSocketStore>;
    __pulseWsShutdownBound?: boolean;
  }
}

const bindGlobalShutdownHandler = () => {
  if (window.__pulseWsShutdownBound) return;

  window.addEventListener(
    'beforeunload',
    () => {
      window.__pulseWsStore?.shutdown();
      delete window.__pulseWsStore;
    },
    { once: true },
  );
  window.__pulseWsShutdownBound = true;
};

export function getGlobalWebSocketStore() {
  if (!window.__pulseWsStore) {
    const wsUrl = getPulseWebSocketUrl();

    window.__pulseWsStore = createWebSocketStore(wsUrl);
    bindGlobalShutdownHandler();
  }

  return window.__pulseWsStore;
}
