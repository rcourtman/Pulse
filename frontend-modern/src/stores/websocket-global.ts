import { createWebSocketStore } from './websocket';
import { createSignal } from 'solid-js';
import { createStore } from 'solid-js/store';
import { getPulseWebSocketUrl } from '@/utils/url';
import type { Alert, ResolvedAlert, State } from '@/types/api';

// Store the instance on window to survive hot reloads
declare global {
  interface Window {
    __pulseWsStore?: ReturnType<typeof createWebSocketStore>;
    __pulseWsShutdownBound?: boolean;
  }
}

const isJSDOMTestRuntime = () =>
  import.meta.env.MODE === 'test' &&
  typeof window !== 'undefined' &&
  /jsdom/i.test(window.navigator.userAgent);

const createNoopWebSocketStore = (): ReturnType<typeof createWebSocketStore> => {
  const [connected] = createSignal(false);
  const [reconnecting] = createSignal(false);
  const [initialDataReceived] = createSignal(true);
  const [updateProgress] = createSignal<unknown>(null);
  const [state] = createStore<State>({
    connectedInfrastructure: [],
    metrics: [],
    performance: {
      apiCallDuration: {},
      lastPollDuration: 0,
      pollingStartTime: '',
      totalApiCalls: 0,
      failedApiCalls: 0,
      cacheHits: 0,
      cacheMisses: 0,
    },
    connectionHealth: {},
    stats: {
      startTime: new Date().toISOString(),
      uptime: 0,
      pollingCycles: 0,
      webSocketClients: 0,
      version: '2.0.0',
    },
    activeAlerts: [],
    recentlyResolved: [],
    lastUpdate: 0,
    pveTagColors: {},
    resources: [],
  });

  return {
    state,
    activeAlerts: {} as Record<string, Alert>,
    recentlyResolved: {} as Record<string, ResolvedAlert>,
    connected,
    reconnecting,
    initialDataReceived,
    updateProgress,
    shutdown: () => {},
    reconnect: () => {},
    switchUrl: () => {},
    markDockerRuntimesTokenRevoked: () => {},
    markAgentsTokenRevoked: () => {},
    removeAlerts: () => {},
    updateAlert: () => {},
  };
};

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
    if (isJSDOMTestRuntime()) {
      window.__pulseWsStore = createNoopWebSocketStore();
      return window.__pulseWsStore;
    }

    const wsUrl = getPulseWebSocketUrl();

    window.__pulseWsStore = createWebSocketStore(wsUrl);
    bindGlobalShutdownHandler();
  }

  return window.__pulseWsStore;
}
