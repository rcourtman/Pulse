// The production canonical drawer, state and CSS with local synthetic evidence.
// No guest agent, backup, restart, poll, scan or update action is performed.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { ResourceDetailDrawer } from '../src/components/Infrastructure/ResourceDetailDrawer';
import { DarkModeContext, WebSocketContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const start = Date.parse('2026-10-11T10:00:00Z');
syncAIRuntimeSettings({ discovery_enabled: false });
const initial: Resource = {
  id: 'fixture:pve-a:101',
  name: 'Held backup guest',
  displayName: 'Held backup guest',
  platformId: '101',
  platformType: 'proxmox-pve',
  platformScopes: ['proxmox-pve', 'agent'],
  sourceType: 'hybrid',
  sources: ['proxmox', 'agent'],
  status: 'online',
  type: 'vm',
  lastSeen: start - 60_000,
  proxmox: {
    vmid: 101,
    nodeName: 'pve-a',
    guestAgentStatus: 'deferred',
    diskStatusReason: 'prev-agent-timeout',
  },
  platformData: {
    proxmox: { vmid: 101, node: 'pve-a', diskStatusReason: 'prev-agent-timeout' },
    agent: { id: 'fixture-agent' },
    sourceStatus: {
      proxmox: { status: 'deferred', lastSeen: start - 60_000, error: 'Guest read deferred' },
      agent: { status: 'stale', lastSeen: new Date(start - 120_000).toISOString() },
    },
  },
};

function Fixture() {
  const [resource, setResource] = createSignal(initial);
  const [mounted, setMounted] = createSignal(true);
  (window as any).__sourceEvidenceAge = {
    snapshot: () => resource(),
    unmount: () => setMounted(false),
    remount: () => setMounted(true),
    replace: () =>
      setResource({
        ...initial,
        lastSeen: Date.now(),
        proxmox: { ...initial.proxmox, guestAgentStatus: 'available', diskStatusReason: '' },
        platformData: {
          proxmox: { vmid: 101, node: 'pve-a', diskStatusReason: '' },
          sourceStatus: { proxmox: { status: 'available', lastSeen: Date.now() } },
        },
      }),
    missingTimes: () =>
      setResource({
        ...initial,
        platformData: {
          proxmox: { vmid: 101, node: 'pve-a' },
          agent: { id: 'fixture-agent' },
          docker: { id: 'fixture-docker' },
          sourceStatus: {
            proxmox: { status: 'deferred' },
            agent: { status: 'stale', lastSeen: 'not-a-date', error: 'No usable timestamp' },
          },
        },
      }),
  };
  return (
    <main class="mx-auto min-h-screen max-w-4xl space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Resource source evidence ages</h1>
      <p class="text-xs text-muted">Synthetic local snapshots. No guest operation is sent.</p>
      <Show when={mounted()}>
        <ResourceDetailDrawer resource={resource()} presentation="table-row" />
      </Show>
    </main>
  );
}
render(
  () => (
    <WebSocketContext.Provider
      value={{ state: { pmg: [] }, activeAlerts: {}, connected: () => false } as any}
    >
      <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
        <Router>
          <Route path="/*" component={Fixture} />
        </Router>
      </DarkModeContext.Provider>
    </WebSocketContext.Provider>
  ),
  document.getElementById('root')!,
);
