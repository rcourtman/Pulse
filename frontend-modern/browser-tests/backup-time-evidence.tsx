// Production resource projection, rows and drawer; synthetic timestamps only.
// This fixture performs no backup, guest-agent, restore or thaw operation.
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
import '../src/index.css';

syncAIRuntimeSettings({ discovery_enabled: false });
type Observation = { timestamp?: string; running?: boolean };
const initial: Observation = { timestamp: '2026-10-04T06:00:00Z', running: false };
const params = new URLSearchParams(window.location.search);
const kind = params.get('kind') === 'lxc' ? 'system-container' : 'vm';
const nativeResource = (value: Observation): Resource => ({
  id: 'fixture-a-pve1-101',
  type: kind,
  name: 'backup-guest',
  displayName: 'backup-guest',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'online',
  sources: ['proxmox'],
  lastSeen: Date.UTC(2026, 9, 4, 8),
  cpu: { current: 10 },
  memory: { current: 25, used: 1024 ** 3 / 4, total: 1024 ** 3 },
  disk: { current: 50, used: 5 * 1024 ** 3, total: 10 * 1024 ** 3 },
  proxmox: {
    instance: 'fixture-a',
    nodeName: 'pve1',
    vmid: 101,
    runtimeStatus: 'running',
    template: false,
    lastBackup: value.timestamp,
    backupInProgress: value.running ?? false,
    lock: value.running ? 'backup' : '',
    disks: [],
  },
});

function Fixture() {
  const transport = params.get('transport') || 'api';
  const [snapshot, setSnapshot] = createSignal<Resource[]>(
    mergeCanonicalResourceSnapshot([nativeResource(initial)], []),
  );
  const state = useWorkloads(() => true, transport === 'api' ? {} : { resourceSnapshot: snapshot });
  const guest = () => state.workloads()[0]!;
  const [closed, setClosed] = createSignal(0);
  (window as any).__backupTimeEvidence = {
    update: async (value: Observation) => {
      if (transport === 'api') {
        await state.refetch();
      } else {
        const incoming = nativeResource(value);
        setSnapshot((previous) =>
          transport === 'canonical-full'
            ? mergeCanonicalResourceSnapshot([incoming], previous)
            : mergeCanonicalResourceDeltaSnapshot([incoming], previous, new Set([incoming.id])),
        );
      }
    },
    identity: () => guest().id,
    closed,
  };
  return (
    <main class="min-h-screen space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Backup time evidence</h1>
      <p class="text-xs text-muted">Synthetic {transport} timestamps; no backup command is sent.</p>
      <Show when={Boolean(guest())}>
        <section aria-label="Backup age row">
          <table class="w-full table-fixed">
            <tbody>
              <GuestRow
                guest={guest()}
                visibleColumnIds={['name', 'backup']}
                workloadTableLayoutMode={window.innerWidth <= 768 ? 'phone' : 'wide'}
              />
            </tbody>
          </table>
        </section>
        <section aria-label="Backup indicator row">
          <table class="w-full table-fixed">
            <tbody>
              <GuestRow guest={guest()} visibleColumnIds={['name']} />
            </tbody>
          </table>
        </section>
        <section aria-label="Guest details">
          <GuestDrawer guest={guest()} onClose={() => setClosed((n) => n + 1)} />
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
