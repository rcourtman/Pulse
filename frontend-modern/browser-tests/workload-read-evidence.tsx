// Real workload adapter/merges and production panels, synthetic read snapshots.
// No backup, guest-agent, freeze/thaw or native recovery operation is performed.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { useWorkloads } from '../src/hooks/useWorkloads';
import {
  mergeCanonicalResourceSnapshot,
  mergeCanonicalResourceDeltaSnapshot,
} from '../src/utils/resourceStateAdapters';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { Resource } from '../src/types/resource';
import type { VM } from '../src/types/api';
import '../src/index.css';

syncAIRuntimeSettings({ discovery_enabled: false });
const capacity = 10 * 1024 ** 3;
type Read = {
  reason?: string;
  usage?: number;
  lock?: string;
  status: string;
  expected?: boolean;
};
const initial: Read = {
  reason: 'prev-vm-locked',
  usage: 50,
  lock: 'backup',
  status: 'deferred',
  expected: true,
};
const nativeResource = (read: Read): Resource => ({
  id: 'fixture-a-pve1-101',
  type: 'vm',
  name: 'backup-guest',
  displayName: 'backup-guest',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'online',
  sources: ['proxmox'],
  lastSeen: Date.UTC(2026, 9, 4, 5),
  cpu: { current: 10 },
  memory: { current: 25, used: capacity / 4, total: capacity },
  ...(read.usage === undefined
    ? {}
    : {
        disk: { current: read.usage, used: (capacity * read.usage) / 100, total: capacity },
      }),
  proxmox: {
    vmid: 101,
    nodeName: 'pve1',
    instance: 'fixture-a',
    runtimeStatus: 'running',
    guestAgentStatus: read.status,
    disks:
      read.usage === undefined
        ? []
        : [
            {
              mountpoint: '/data',
              type: 'ext4',
              total: capacity,
              used: (capacity * read.usage) / 100,
              usage: read.usage,
            },
          ],
    ...(read.reason === undefined ? {} : { diskStatusReason: read.reason }),
    ...(read.lock === undefined ? {} : { lock: read.lock }),
    ...(read.expected === undefined ? {} : { guestAgentExpected: read.expected }),
  },
});

function Fixture() {
  const transport = new URLSearchParams(window.location.search).get('transport') || 'api';
  const [snapshot, setSnapshot] = createSignal<Resource[]>(
    mergeCanonicalResourceSnapshot([nativeResource(initial)], []),
  );
  const state = useWorkloads(
    () => true,
    transport === 'api'
      ? {}
      : {
          resourceSnapshot: snapshot,
        },
  );
  const guest = () => state.workloads()[0]!;
  (window as any).__workloadReadEvidence = {
    update: async (read: Read) => {
      if (transport === 'api') {
        await state.refetch();
        return;
      }
      const incoming = nativeResource(read);
      setSnapshot((previous) =>
        transport === 'canonical-full'
          ? mergeCanonicalResourceSnapshot([incoming], previous)
          : mergeCanonicalResourceDeltaSnapshot(
              [incoming],
              previous,
              new Set([incoming.id]),
              transport === 'canonical-fast'
                ? new Map([[incoming.id, ['proxmox', 'disk']]])
                : undefined,
            ),
      );
    },
    evidence: () => {
      const vm = guest() as VM;
      return {
        id: vm.id,
        reason: vm.diskStatusReason,
        lock: vm.lock,
        agent: vm.guestAgentStatus,
        expected: vm.guestAgentExpected,
        usage: vm.disk.usage,
        available: guest().telemetryAvailability?.disk,
      };
    },
  };
  return (
    <main class="min-h-screen space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Workload read evidence</h1>
      <p class="text-xs text-muted">
        Synthetic {transport} observations. No guest commands are sent.
      </p>
      <Show when={Boolean(guest())}>
        <section aria-label="Workload row">
          <table class="w-full table-fixed">
            <tbody>
              <GuestRow
                guest={guest()}
                visibleColumnIds={['name', 'disk']}
                workloadTableLayoutMode={window.innerWidth <= 768 ? 'phone' : 'wide'}
              />
            </tbody>
          </table>
        </section>
        <section aria-label="Guest details">
          <GuestDrawer guest={guest()} onClose={() => {}} />
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
