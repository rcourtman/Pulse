// Stored synthetic deltas through the canonical merger, Workloads mapper,
// full guest drawer and real scoped HTTP disk reader. Never a guest probe.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { useWorkloads } from '../src/hooks/useWorkloads';
import { mergeCanonicalResourceDeltaSnapshot } from '../src/utils/resourceStateAdapters';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { dispatchResourceMetadataChanged } from '../src/utils/resourceMetadataEvents';
import type { Resource } from '../src/types/resource';
import type { HostRAIDArray } from '../src/types/api';
import '../src/index.css';
const observedAt = '2026-10-20T12:00:00Z';
const array: HostRAIDArray = {
  device: '/dev/md0',
  name: 'data',
  level: 'raid1',
  state: 'clean',
  totalDevices: 2,
  activeDevices: 2,
  workingDevices: 2,
  failedDevices: 0,
  spareDevices: 0,
  devices: [{ device: '/dev/sda', slot: 0, state: 'active sync' }],
};
const initial: Resource = {
  id: 'vm-resource-101',
  name: 'hardware-guest',
  displayName: 'hardware-guest',
  type: 'vm',
  platformId: 'fixture',
  platformType: 'proxmox-pve',
  platformScopes: ['proxmox-pve'],
  sourceType: 'hybrid',
  status: 'online',
  sources: ['proxmox', 'agent'],
  lastSeen: Date.parse(observedAt),
  identity: {},
  cpu: { current: 0 },
  memory: {
    current: 0,
    used: 0,
    total: 1024 ** 3,
    observation: { state: 'current', source: 'status-mem', observedAt },
  },
  proxmox: {
    vmid: 101,
    nodeName: 'pve-a',
    instance: 'fixture',
    runtimeStatus: 'running',
    guestAgentStatus: 'available',
    lastBackup: observedAt,
  },
  agent: {
    stale: false,
    agentVersion: 'fixture-version',
    lastReportAt: observedAt,
    raid: [
      array,
      {
        ...array,
        device: '/dev/md1',
        name: 'archive',
        state: 'clean, degraded, recovering',
        rebuildPercent: 37,
        rebuildSpeed: '100M/sec',
        devices: [...array.devices!, { device: '/dev/sdb', slot: 1, state: 'faulty' }],
      },
    ],
  },
};
function Fixture() {
  const [snapshot, setSnapshot] = createSignal<Resource[]>([structuredClone(initial)]);
  const state = useWorkloads(() => true, { resourceSnapshot: snapshot });
  const guest = () => state.workloads()[0];
  const update = (patch: Partial<Resource>, keys: (keyof Resource)[]) =>
    setSnapshot((previous) =>
      mergeCanonicalResourceDeltaSnapshot(
        [{ ...previous[0], ...patch }],
        previous,
        new Set([initial.id]),
        new Map([[initial.id, keys]]),
      ),
    );
  (window as any).__guestHardware = {
    power: (runtimeStatus: string) =>
      update({ proxmox: { ...snapshot()[0].proxmox, runtimeStatus } }, ['proxmox']),
    stale: (stale: boolean) => update({ agent: { ...snapshot()[0].agent, stale } }, ['agent']),
    kind: (type: 'vm' | 'system-container') => update({ type }, ['type']),
    reread: () =>
      dispatchResourceMetadataChanged({ metadataKind: 'guest', metadataId: initial.id }),
    evidence: () => ({ canonical: snapshot()[0], projected: guest() }),
  };
  return (
    <main class="mx-auto min-h-screen max-w-4xl space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Guest hardware evidence</h1>
      <p class="text-xs text-muted">Synthetic stored observations. No guest checks are sent.</p>
      <Show when={guest()}>
        <section aria-label="Workload drawer">
          <GuestDrawer guest={guest()!} onClose={() => {}} />
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
            <Fixture />
          </DarkModeContext.Provider>
        )}
      />
    </Router>
  ),
  document.getElementById('root')!,
);
