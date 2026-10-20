// Synthetic source observations through the real canonical delta merger,
// Workloads mapper and consumers. No guest or network check is sent.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { useWorkloads } from '../src/hooks/useWorkloads';
import { mergeCanonicalResourceDeltaSnapshot } from '../src/utils/resourceStateAdapters';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const interfaces = [
  {
    name: 'eth0',
    addresses: ['192.0.2.80'],
    mac: '02:00:00:00:00:80',
    rxBytes: 1024,
    txBytes: 2048,
  },
];
const observedAt = '2026-10-20T12:00:00Z';
const initial: Resource = {
  id: 'fixture:pve1:101',
  name: 'network-guest',
  displayName: 'network-guest',
  type: 'vm',
  platformType: 'proxmox-pve',
  platformScopes: ['proxmox-pve'],
  sourceType: 'hybrid',
  status: 'online',
  sources: ['proxmox', 'agent'],
  lastSeen: Date.parse(observedAt),
  identity: { ipAddresses: ['192.0.2.80'] },
  cpu: { current: 0 },
  memory: {
    current: 0,
    used: 0,
    total: 1024 ** 3,
    observation: { state: 'current', source: 'status-mem', observedAt },
  },
  proxmox: {
    vmid: 101,
    nodeName: 'pve1',
    instance: 'fixture',
    runtimeStatus: 'running',
    guestAgentStatus: 'available',
    networkInterfaces: interfaces,
    lastBackup: observedAt,
  },
  agent: { stale: false, agentVersion: 'fixture-version', lastReportAt: observedAt },
};
function Fixture() {
  const [snapshot, setSnapshot] = createSignal<Resource[]>([initial]);
  const state = useWorkloads(() => true, { resourceSnapshot: snapshot });
  const guest = () => state.workloads()[0];
  const [rowActions, setRowActions] = createSignal(0);
  const update = (patch: Partial<Resource>, keys: (keyof Resource)[]) =>
    setSnapshot((previous) =>
      mergeCanonicalResourceDeltaSnapshot(
        [{ ...previous[0], ...patch }],
        previous,
        new Set([initial.id]),
        new Map([[initial.id, keys]]),
      ),
    );
  (window as any).__guestNetwork = {
    power: (runtimeStatus: string) =>
      update({ proxmox: { ...snapshot()[0].proxmox, runtimeStatus } }, ['proxmox']),
    producer: (agent: boolean, stale = false) =>
      update(
        {
          agent: {
            ...snapshot()[0].agent,
            stale,
            networkInterfaces: agent ? interfaces : undefined,
          },
        },
        ['agent'],
      ),
    kind: (type: 'vm' | 'system-container') => update({ type }, ['type']),
    freshAddresses: () =>
      update(
        {
          identity: { ipAddresses: ['192.0.2.81'] },
          proxmox: {
            ...snapshot()[0].proxmox,
            runtimeStatus: 'running',
            networkInterfaces: [
              { ...interfaces[0], addresses: ['192.0.2.81'], rxBytes: 0, txBytes: 0 },
            ],
          },
          agent: { ...snapshot()[0].agent, stale: false, networkInterfaces: undefined },
        },
        ['identity', 'proxmox', 'agent'],
      ),
    evidence: () => ({ canonical: snapshot()[0], projected: guest(), rowActions: rowActions() }),
  };
  return (
    <main class="mx-auto min-h-screen max-w-4xl space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Guest network evidence</h1>
      <p class="text-xs text-muted">Synthetic stored observations. No guest checks are sent.</p>
      <Show when={guest()}>
        <section
          aria-label="Workload row"
          class="rounded-sm border border-border bg-surface-raised p-2"
        >
          <table class="w-full table-fixed">
            <tbody>
              <GuestRow
                guest={guest()!}
                visibleColumnIds={['name', 'ip']}
                workloadTableLayoutMode={window.innerWidth < 768 ? 'phone' : 'wide'}
                onClick={() => setRowActions((n) => n + 1)}
              />
            </tbody>
          </table>
        </section>
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
