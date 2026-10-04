// Production canonical resource projection, row, drawer, router and Docs.
// Synthetic observations only; no native QGA, diagnostic or backup operation.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { useWorkloads } from '../src/hooks/useWorkloads';
import { mergeCanonicalResourceSnapshot } from '../src/utils/resourceStateAdapters';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import Docs from '../src/pages/Docs';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

syncAIRuntimeSettings({ discovery_enabled: false });
interface Observation {
  state?: string;
  reason?: string;
  lock?: string;
  backup?: boolean;
  version?: string;
  assignment?: boolean;
  diskUsed?: number;
}
const initial: Observation = {
  state: 'deferred',
  reason: 'prev-vm-locked',
  lock: 'backup',
  backup: true,
  version: '6.4.5',
  assignment: false,
  diskUsed: 5 * 1024 ** 3,
};
const nativeResource = (value: Observation): Resource => ({
  id: 'fixture-pve:pve-a:101',
  type: 'vm',
  name: 'backup-guest',
  displayName: 'backup-guest',
  platformType: 'proxmox-pve',
  platformScopes: ['proxmox-pve'],
  sourceType: 'api',
  status: 'online',
  sources: ['proxmox', 'agent'],
  lastSeen: Date.UTC(2026, 9, 4, 14),
  cpu: { current: 10 },
  memory: { current: 25, used: 1024 ** 3 / 4, total: 1024 ** 3 },
  disk: {
    current: ((value.diskUsed ?? 0) / (10 * 1024 ** 3)) * 100,
    used: value.diskUsed,
    total: 10 * 1024 ** 3,
  },
  proxmox: {
    instance: 'fixture-pve',
    nodeName: 'pve-a',
    vmid: 101,
    runtimeStatus: 'running',
    template: false,
    guestAgentStatus: value.state,
    diskStatusReason: value.reason,
    lock: value.lock,
    guestAgentExpected: true,
    backupInProgress: value.backup ?? false,
    lastBackup: '2026-10-04T06:00:00Z',
    disks: [],
  },
  agent: {
    agentId: 'fixture-pulse-agent',
    agentVersion: value.version || '',
    hostname: 'backup-guest',
    osName: 'Linux',
  },
  discoveryTarget: value.assignment
    ? { resourceType: 'vm', agentId: 'fixture-node-agent', resourceId: '101' }
    : undefined,
});
function Fixture() {
  const [observation, setObservation] = createSignal(initial);
  const [snapshot, setSnapshot] = createSignal<Resource[]>([nativeResource(initial)]);
  const state = useWorkloads(() => true, { resourceSnapshot: snapshot });
  const guest = () => state.workloads()[0]!;
  const [expanded, setExpanded] = createSignal(false);
  (window as any).__guestAgentCoverage = {
    update: (next: Observation) => {
      setObservation(next);
      setSnapshot((previous) => mergeCanonicalResourceSnapshot([nativeResource(next)], previous));
    },
    observation,
    identity: () => guest().id,
  };
  return (
    <main class="mx-auto min-h-screen max-w-5xl space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Guest coverage evidence</h1>
      <p class="text-xs text-muted">Synthetic observations. No guest check is sent.</p>
      <Show when={Boolean(guest())}>
        <table class="w-full table-fixed">
          <tbody>
            <GuestRow
              guest={guest()}
              visibleColumnIds={['name', 'disk']}
              isExpanded={expanded()}
              onClick={() => setExpanded(!expanded())}
              workloadTableLayoutMode={window.innerWidth <= 768 ? 'phone' : 'wide'}
            />
          </tbody>
        </table>
        <Show when={expanded()}>
          <section aria-label="Guest details">
            <GuestDrawer guest={guest()} onClose={() => setExpanded(false)} />
          </section>
        </Show>
      </Show>
    </main>
  );
}
render(
  () => (
    <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
      <Router>
        <Route path="/docs/*docPath" component={Docs} />
        <Route path="/*" component={Fixture} />
      </Router>
    </DarkModeContext.Provider>
  ),
  document.getElementById('root')!,
);
