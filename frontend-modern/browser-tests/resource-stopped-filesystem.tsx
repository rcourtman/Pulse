// Synthetic source snapshots only; the drawer, History client and CSS are production.
import { createMemo, createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { ResourceDetailDrawer } from '../src/components/Infrastructure/ResourceDetailDrawer';
import { DarkModeContext, WebSocketContext, type WebSocketStore } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

syncAIRuntimeSettings({ discovery_enabled: false });
// Only the empty alert producer is synthetic; no WebSocket connection is opened.
const websocket = {
  activeAlerts: {},
  state: { resources: [] },
  connected: () => true,
} as unknown as WebSocketStore;

type Snapshot = {
  power?: string;
  status: Resource['status'];
  reason: string;
  usage: number;
  type: Resource['type'];
  platform: Resource['platformType'];
  scopes?: Resource['platformScopes'];
  seen: number;
};

function Fixture() {
  const [snapshot, setSnapshot] = createSignal<Snapshot>({
    power: 'stopped',
    status: 'online',
    reason: '',
    usage: 90,
    type: 'vm',
    platform: 'proxmox-pve',
    seen: Date.now(),
  });
  const resource = createMemo<Resource>(() => ({
    id: 'fixture-vm',
    type: snapshot().type,
    name: 'backup-guest',
    displayName: 'Backup guest',
    platformId: 'fixture',
    platformType: snapshot().platform,
    platformScopes: snapshot().scopes,
    sourceType: 'hybrid',
    status: snapshot().status,
    lastSeen: snapshot().seen,
    metricsTarget: {
      resourceType: snapshot().type === 'system-container' ? 'lxc' : 'vm',
      resourceId: 'fixture:pve:101',
    },
    cpu: { current: 10 },
    memory: {
      current: 0,
      observation: { state: 'current', source: 'agent', observedAt: '2026-10-10T00:00:00Z' },
    },
    disk: { current: snapshot().usage },
    network: { rxBytes: 0, txBytes: 200 },
    proxmox: {
      vmid: 101,
      node: 'pve',
      instance: 'fixture',
      runtimeStatus: snapshot().power,
      diskStatusReason: snapshot().reason,
    },
  }));
  Object.assign(window, {
    __resourceStoppedFilesystem: {
      apply: (next: Partial<Snapshot>) => setSnapshot((previous) => ({ ...previous, ...next })),
      resource,
    },
  });
  return (
    <main class="min-h-screen bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Canonical filesystem History</h1>
      <p class="my-2 text-xs text-muted">
        Synthetic power and filesystem observations. No guest operation is sent.
      </p>
      <section class="mx-auto max-w-5xl" aria-label="Production drawer">
        <ResourceDetailDrawer resource={resource()} presentation="table-row" />
      </section>
    </main>
  );
}

render(
  () => (
    <Router>
      <Route
        path="*"
        component={() => (
          <DarkModeContext.Provider
            value={() => document.documentElement.classList.contains('dark')}
          >
            <WebSocketContext.Provider value={websocket}>
              <Fixture />
            </WebSocketContext.Provider>
          </DarkModeContext.Provider>
        )}
      />
    </Router>
  ),
  document.getElementById('root')!,
);
