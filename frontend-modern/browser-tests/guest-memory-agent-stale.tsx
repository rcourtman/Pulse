// A source-facet-only update through the real canonical merger and Workloads
// projection. No guest operations, native restart or recovery are simulated.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { DarkModeContext, WebSocketContext, type WebSocketStore } from '../src/contexts/appRuntime';
import { useWorkloads } from '../src/hooks/useWorkloads';
import { mergeCanonicalResourceDeltaSnapshot } from '../src/utils/resourceStateAdapters';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { ResourceDetailDrawer } from '../src/components/Infrastructure/ResourceDetailDrawer';
import { getWorkloadTableLayoutModeForContainer } from '../src/components/Workloads/guestRowModel';
import { createWorkloadSortComparator } from '../src/components/Workloads/workloadSelectors';
import { evaluateFilterStack, parseFilterStack } from '../src/utils/searchQuery';
import type { Resource } from '../src/types/resource';
import type { MemoryObservation } from '../src/types/api';
import '../src/index.css';

const capacity = 4 * 1024 ** 3;
const observedAt = '2026-10-10T12:00:00Z';
const initial: Resource = {
  id: 'fixture:pve1:101',
  type: 'vm',
  name: 'linked-guest',
  displayName: 'linked-guest',
  platformId: 'fixture',
  sourceType: 'hybrid',
  status: 'online',
  platformType: 'proxmox-pve',
  platformScopes: ['proxmox-pve'],
  sources: ['proxmox', 'agent'],
  metricsTarget: { resourceType: 'vm', resourceId: 'fixture:pve1:101' },
  lastSeen: Date.parse(observedAt),
  cpu: { current: 10 },
  memory: {
    current: 95,
    used: capacity * 0.95,
    total: capacity,
    observation: { state: 'current', source: 'agent', observedAt },
  },
  disk: { current: 50, used: capacity / 2, total: capacity },
  proxmox: {
    vmid: 101,
    nodeName: 'pve1',
    instance: 'fixture',
    runtimeStatus: 'running',
    guestAgentStatus: 'deferred',
    diskStatusReason: 'prev-vm-locked',
    lock: 'backup',
  },
  agent: { stale: false, agentVersion: 'fixture-version', lastReportAt: observedAt },
};

// Only the transport is synthetic; the canonical merger, mapper and mounted
// presentation use production code. No websocket connection is attempted.
window.__pulseWsStore = {
  activeAlerts: {},
  state: { resources: [], activeAlerts: [], recentlyResolved: [] },
  connected: () => false,
  initialDataReceived: () => true,
  resourceSnapshotReceived: () => false,
  resourceChange: () => ({ version: 0, changedIds: null, changedKeys: null }),
  changedResourceMetaSince: () => null,
  changedResourceIdsSince: () => null,
  shutdown: () => {},
} as unknown as NonNullable<typeof window.__pulseWsStore>;

function Fixture() {
  const [snapshot, setSnapshot] = createSignal<Resource[]>([initial]);
  const workloads = useWorkloads(() => true, { resourceSnapshot: snapshot });
  const guest = () => workloads.workloads()[0];
  const [mode, setMode] = createSignal<'bars' | 'sparklines'>('bars');
  const [basis, setBasis] = createSignal<'guest' | 'host'>('guest');
  const layout = getWorkloadTableLayoutModeForContainer(Math.min(window.innerWidth, 896) - 40);
  const update = (patch: Partial<Resource>, keys: (keyof Resource)[]) =>
    setSnapshot((previous) =>
      mergeCanonicalResourceDeltaSnapshot(
        // The native delta wrapper receives a complete resource with changed
        // keys, not an identity-only patch that would withdraw provider facets.
        [{ ...previous[0], ...patch }],
        previous,
        new Set([initial.id]),
        new Map([[initial.id, keys]]),
      ),
    );
  (window as any).__staleAgentMemory = {
    power: (runtimeStatus: string) =>
      update({ proxmox: { ...snapshot()[0].proxmox, runtimeStatus } }, ['proxmox']),
    kind: (type: 'vm' | 'system-container') => update({ type }, ['type']),
    stale: () =>
      update(
        {
          agent: { ...snapshot()[0].agent, stale: true },
          lastSeen: Date.parse('2026-10-11T12:00:00Z'),
        },
        ['agent', 'lastSeen'],
      ),
    reading: (source: string, usage: number, state = 'current') =>
      update(
        {
          memory: {
            current: usage,
            used: (capacity * usage) / 100,
            total: capacity,
            observation: { state, source, observedAt } as MemoryObservation,
          },
        },
        ['memory'],
      ),
    recover: () =>
      update(
        {
          agent: { ...snapshot()[0].agent, stale: false, lastReportAt: '2026-10-11T12:00:00Z' },
          memory: {
            current: 0,
            used: 0,
            total: capacity,
            observation: { state: 'current', source: 'agent', observedAt: '2026-10-11T12:00:00Z' },
          },
        },
        ['agent', 'memory'],
      ),
    evidence: () => ({
      canonical: snapshot()[0],
      projected: guest(),
      filterHigh: evaluateFilterStack(guest(), parseFilterStack('memory>90')),
      sort: [
        guest(),
        {
          ...guest(),
          id: 'independent',
          status: 'running',
          memory: {
            ...guest().memory,
            usage: 10,
            observation: { state: 'current', source: 'guest-agent-meminfo', observedAt },
          },
        },
      ]
        .sort(createWorkloadSortComparator('memory', 'desc')!)
        .map((value) => value.id),
    }),
  };
  return (
    <main class="mx-auto min-h-screen max-w-4xl space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Source-owned memory evidence</h1>
      <p class="text-xs text-muted">Synthetic source updates. No guest checks are sent.</p>
      <div class="flex flex-wrap gap-3 text-xs">
        <label>
          Display{' '}
          <select
            aria-label="Metric display"
            class="border border-border bg-surface p-1"
            onChange={(event) => setMode(event.currentTarget.value as 'bars' | 'sparklines')}
          >
            <option value="bars">Bars</option>
            <option value="sparklines">Sparklines</option>
          </select>
        </label>
        <label>
          Memory basis{' '}
          <select
            aria-label="Memory basis"
            class="border border-border bg-surface p-1"
            onChange={(event) => setBasis(event.currentTarget.value as 'guest' | 'host')}
          >
            <option value="guest">Guest allocation</option>
            <option value="host">Host capacity</option>
          </select>
        </label>
      </div>
      <Show when={guest()}>
        <section
          aria-label="Workload row"
          class="rounded-sm border border-border bg-surface-raised p-2"
        >
          <table class="w-full table-fixed">
            <tbody>
              <GuestRow
                guest={guest()}
                visibleColumnIds={['name', 'memory']}
                workloadTableLayoutMode={layout}
                metricDisplayMode={mode()}
                memoryDisplayBasis={basis()}
                parentMemoryTotal={capacity * 2}
                onClick={() => {}}
              />
            </tbody>
          </table>
        </section>
        <section aria-label="Workload drawer">
          <GuestDrawer guest={guest()} onClose={() => {}} />
        </section>
        <section aria-label="Canonical drawer">
          <ResourceDetailDrawer resource={snapshot()[0]} presentation="table-row" />
        </section>
      </Show>
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
            <WebSocketContext.Provider value={window.__pulseWsStore as unknown as WebSocketStore}>
              <Fixture />
            </WebSocketContext.Provider>
          </DarkModeContext.Provider>
        )}
      />
    </Router>
  ),
  document.getElementById('root')!,
);
