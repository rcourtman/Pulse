// Production hook/adapter/drawer consumers, synthetic source snapshots only.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { useWorkloads } from '../src/hooks/useWorkloads';
import { useUnifiedResources } from '../src/hooks/useUnifiedResources';
import {
  mergeCanonicalResourceSnapshot,
  mergeCanonicalResourceDeltaSnapshot,
} from '../src/utils/resourceStateAdapters';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { Resource } from '../src/types/resource';
import type { MemoryObservation } from '../src/types/api';
import type { WorkloadGuest } from '../src/types/workloads';
import '../src/index.css';

syncAIRuntimeSettings({ discovery_enabled: false });
interface Reading {
  state?: string;
  source?: string;
  observedAt?: string;
  usage: number;
  lastSeen?: string;
  peer?: boolean;
}
const initial: Reading = {
  state: 'last-known',
  source: 'guest-agent-meminfo',
  observedAt: '2026-10-04T14:00:00Z',
  usage: 25,
};
const observation = (read: Reading): MemoryObservation | undefined =>
  read.state === undefined
    ? undefined
    : {
        state: read.state,
        source: read.source ?? '',
        ...(read.observedAt ? { observedAt: read.observedAt } : {}),
      };
const capacity = 4 * 1024 ** 3;
const id = (read: Reading) => (read.peer ? 'fixture-b-pve1-101' : 'fixture-a-pve1-101');
const nativeResource = (read: Reading): Resource => ({
  id: id(read),
  type: 'vm',
  name: read.peer ? 'peer-guest' : 'backup-guest',
  displayName: read.peer ? 'peer-guest' : 'backup-guest',
  status: 'online',
  sources: ['proxmox', 'agent'],
  platformType: 'proxmox-pve',
  platformScopes: ['proxmox-pve'],
  sourceType: 'hybrid',
  lastSeen: Date.parse(read.lastSeen ?? '2026-10-04T17:00:00Z'),
  cpu: { current: 10 },
  memory:
    read.state === 'unavailable'
      ? undefined
      : {
          current: read.usage,
          used: (capacity * read.usage) / 100,
          total: capacity,
          ...(observation(read) ? { observation: observation(read) } : {}),
        },
  disk: { current: 50, used: capacity / 2, total: capacity },
  proxmox: {
    vmid: 101,
    nodeName: 'pve1',
    instance: read.peer ? 'fixture-b' : 'fixture-a',
    runtimeStatus: 'running',
    guestAgentStatus: 'deferred',
    guestAgentExpected: true,
    diskStatusReason: 'prev-vm-locked',
    lock: 'backup',
    memory: {
      total: capacity,
      used: capacity / 2,
      free: capacity / 2,
      usage: 50,
      usageUnavailable: read.state === 'unavailable',
      observation: { state: 'current', source: 'status-mem', observedAt: '2026-10-04T17:00:00Z' },
    },
  },
});
const rawGuest = (read: Reading): WorkloadGuest => ({
  id: id(read),
  vmid: 101,
  name: read.peer ? 'peer-guest' : 'backup-guest',
  instance: read.peer ? 'fixture-b' : 'fixture-a',
  node: 'pve1',
  type: 'qemu',
  status: 'running',
  cpu: 0.1,
  cpus: 2,
  memory: {
    total: capacity,
    used: (capacity * read.usage) / 100,
    free: (capacity * (100 - read.usage)) / 100,
    usage: read.usage,
    usageUnavailable: read.state === 'unavailable',
    ...(observation(read) ? { observation: observation(read) } : {}),
  },
  disk: { usage: 50, total: capacity, used: capacity / 2 },
  diskStatusReason: 'prev-vm-locked',
  lock: 'backup',
  guestAgentStatus: 'deferred',
  guestAgentExpected: true,
  networkIn: 10,
  networkOut: 20,
  diskRead: 30,
  diskWrite: 40,
  uptime: 3600,
  template: false,
  lastBackup: 0,
  lastSeen: read.lastSeen ?? '2026-10-04T17:00:00Z',
  tags: [],
});
// Only the websocket transport is synthetic. API conversion and owned hooks
// remain production code, with real requests fulfilled by the bounded script.
window.__pulseWsStore = {
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
  const transport = new URLSearchParams(window.location.search).get('transport') ?? 'api';
  const [read, setRead] = createSignal(initial);
  const [snapshot, setSnapshot] = createSignal<Resource[]>([nativeResource(initial)]);
  const unified =
    transport === 'unified-api'
      ? useUnifiedResources({ query: 'type=vm', realtimeEnabled: () => false })
      : null;
  const workloads =
    transport === 'raw'
      ? null
      : useWorkloads(
          () => true,
          transport === 'api'
            ? {}
            : { resourceSnapshot: unified ? () => unified.resources() : snapshot },
        );
  const guest = () => (transport === 'raw' ? rawGuest(read()) : workloads!.workloads()[0]);
  (window as any).__guestMemory = {
    update: async (next: Reading) => {
      setRead(next);
      if (transport === 'raw') return;
      if (transport === 'api') {
        await workloads!.refetch();
        return;
      }
      if (unified) {
        await unified.refetch();
        return;
      }
      const incoming = nativeResource(next);
      setSnapshot((previous) =>
        transport === 'canonical-full' || next.peer
          ? mergeCanonicalResourceSnapshot([incoming], previous)
          : mergeCanonicalResourceDeltaSnapshot(
              [incoming],
              previous,
              new Set([incoming.id]),
              transport === 'canonical-fast'
                ? new Map([[incoming.id, ['memory', 'proxmox', 'lastSeen']]])
                : undefined,
            ),
      );
    },
    evidence: () => ({
      id: guest()?.id,
      observation: guest()?.memory.observation,
      usage: guest()?.memory.usage,
      unavailable: guest()?.memory.usageUnavailable,
    }),
  };
  return (
    <main class="mx-auto min-h-screen max-w-5xl space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Guest memory evidence</h1>
      <p class="text-xs text-muted">
        Synthetic {transport} observations. No guest checks are sent.
      </p>
      <Show when={guest()}>
        {() => (
          <section aria-label="Guest details">
            <GuestDrawer guest={guest()!} onClose={() => {}} />
          </section>
        )}
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
            <Fixture />
          </DarkModeContext.Provider>
        )}
      />
    </Router>
  ),
  document.getElementById('root')!,
);
