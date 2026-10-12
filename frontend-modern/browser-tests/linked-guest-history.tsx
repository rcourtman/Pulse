// Only snapshots and the fixture controls are synthetic. Source mapping,
// drawers, History, clients and CSS are production code; no guest action.
import { createMemo, createSignal, For, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { ResourceDetailDrawer } from '../src/components/Infrastructure/ResourceDetailDrawer';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { useWorkloads } from '../src/hooks/useWorkloads';
import { DarkModeContext, WebSocketContext, type WebSocketStore } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { Resource } from '../src/types/resource';
import '../src/index.css';
syncAIRuntimeSettings({ discovery_enabled: false });

function Fixture() {
  const search = new URLSearchParams(location.search);
  const type = search.has('lxc') ? 'system-container' : 'vm';
  const [mode, setMode] = createSignal('Live agent');
  const [seen, setSeen] = createSignal(Date.parse('2026-10-12T01:00:00Z'));
  const disk = () => ({
    mountpoint: '/',
    total: 100,
    used: mode() === 'Fresh zero' ? 0 : 62,
    free: 38,
  });
  const stopped = () => ['Stopped guest', 'Stopped platform'].includes(mode());
  const platform = () =>
    ['Platform filesystems', 'Empty platform list', 'Stopped platform'].includes(mode());
  const silent = () => ['Silent agent', 'QGA recovered'].includes(mode());
  const resource = createMemo<Resource>(() => ({
    id: 'site-a:pve1:105',
    type,
    name: 'Linked guest',
    displayName: 'Linked guest',
    platformId: 'site-a',
    platformType: 'proxmox-pve',
    sourceType: 'hybrid',
    status: 'online',
    lastSeen: seen(),
    metricsTarget: { resourceType: type, resourceId: 'site-a:pve1:105' },
    cpu: { current: 10 },
    memory: {
      current: 0,
      total: 100,
      used: 0,
      observation: { state: 'current', source: 'agent', observedAt: '2026-10-12T00:00:00Z' },
    },
    disk:
      mode() === 'Missing aggregate'
        ? undefined
        : { current: disk().used, total: 100, used: disk().used },
    network: { rxBytes: 0, txBytes: 200 },
    proxmox: {
      nodeName: 'pve1',
      vmid: 105,
      instance: 'site-a',
      runtimeStatus: stopped() ? 'stopped' : 'running',
      diskStatusReason:
        mode() === 'QGA recovered' || stopped()
          ? ''
          : platform()
            ? 'prev-vm-locked'
            : 'agent-not-running',
      disks: platform() ? (mode() === 'Empty platform list' ? [] : [disk()]) : undefined,
    },
    agent: { agentId: 'in-guest', stale: silent(), disks: [disk()] },
  }));
  const mapped = useWorkloads(() => true, { resourceSnapshot: () => [resource()] });
  return (
    <main class="min-h-screen bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Linked guest History · {type}</h1>
      <p class="my-2 text-xs text-muted">
        Synthetic readings and controls. No guest command or backup runs.
      </p>
      <div class="mb-3 flex flex-wrap gap-2" aria-label="Fixture source controls">
        <For
          each={[
            'Live agent',
            'Silent agent',
            'QGA recovered',
            'Stopped guest',
            'Fresh zero',
            'Platform filesystems',
            'Empty platform list',
            'Stopped platform',
            'Missing aggregate',
          ]}
        >
          {(value) => (
            <button
              class="min-h-11 rounded-sm border border-border px-3 text-xs"
              onClick={() => setMode(value)}
            >
              {value}
            </button>
          )}
        </For>
        <button
          class="min-h-11 rounded-sm border border-border px-3 text-xs"
          onClick={() => setSeen(seen() + 3600000)}
        >
          Advance snapshot
        </button>
      </div>
      <section class="mx-auto max-w-5xl" aria-label="Production drawer">
        <Show
          when={!search.has('guest')}
          fallback={
            <Show when={mapped.workloads()[0]}>
              {(guest) => <GuestDrawer guest={guest()} onClose={() => {}} />}
            </Show>
          }
        >
          <ResourceDetailDrawer resource={resource()} presentation="table-row" />
        </Show>
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
            <WebSocketContext.Provider
              value={
                {
                  activeAlerts: {},
                  state: { pmg: [] },
                  connected: () => true,
                } as unknown as WebSocketStore
              }
            >
              <Fixture />
            </WebSocketContext.Provider>
          </DarkModeContext.Provider>
        )}
      />
    </Router>
  ),
  document.getElementById('root')!,
);
