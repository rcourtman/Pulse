// Production Workloads rows/shared dialog, with explicit guest observations.
// Mapping/collection are unchanged. No backup, guest, thaw or credential operation.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { DarkModeContext } from '../src/contexts/appRuntime';
import type { WorkloadGuest } from '../src/types/workloads';
import '../src/index.css';

type Observation = { timestamp: string; running: boolean };
const guest = (vmid: number, value: Observation): WorkloadGuest => ({
  id: `fixture-pve1-${vmid}`,
  vmid,
  name: vmid === 101 ? 'backup-vm' : 'backup-ct',
  node: 'pve1',
  instance: 'fixture',
  status: 'running',
  type: vmid === 101 ? 'qemu' : 'lxc',
  workloadType: vmid === 101 ? 'vm' : 'system-container',
  platformType: 'proxmox-pve',
  cpu: 0.1,
  cpus: 2,
  memory: { total: 1024 ** 3, used: 1024 ** 3 / 4, free: (3 * 1024 ** 3) / 4, usage: 25 },
  disk: { total: 10 * 1024 ** 3, used: 0, free: 10 * 1024 ** 3, usage: 0 },
  networkIn: 0,
  networkOut: 0,
  diskRead: 0,
  diskWrite: 0,
  uptime: 3600,
  template: false,
  lastBackup: value.timestamp === '' ? 0 : Date.parse(value.timestamp),
  backupInProgress: value.running,
  tags: null,
  lock: value.running ? 'backup' : '',
  lastSeen: '2026-10-05T08:00:00Z',
});
const initial: Observation = { timestamp: '2026-10-05T06:00:00Z', running: false };
function Fixture() {
  const [vm, setVM] = createSignal(guest(101, initial));
  const ct = guest(102, { timestamp: '', running: false });
  const [vmPresent, setVMPresent] = createSignal(true);
  const [actions, setActions] = createSignal(0);
  const [showBackup, setShowBackup] = createSignal(true);
  (window as any).__backupDetails = {
    update: (value: Observation) => setVM(guest(101, value)),
    remove: () => setVMPresent(false),
    identity: () => vm().id,
  };
  return (
    <main class="min-h-screen space-y-4 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Backup evidence accessibility</h1>
      <p class="text-xs text-muted">
        Synthetic guest observations, no backup commands or collector reads.
      </p>
      <button
        class="rounded-sm border border-border p-2 text-xs focus-visible:ring-2 focus-visible:ring-blue-500"
        onClick={() => setShowBackup(!showBackup())}
      >
        {showBackup() ? 'Hide Backup column' : 'Show Backup column'}
      </button>
      <Show when={vmPresent()}>
        <section aria-label="backup-vm row">
          <table class="w-full table-fixed">
            <tbody>
              <GuestRow
                guest={vm()}
                visibleColumnIds={showBackup() ? ['name', 'backup'] : ['name']}
                workloadTableLayoutMode={window.innerWidth <= 768 ? 'phone' : 'wide'}
                onClick={() => setActions((n) => n + 1)}
              />
            </tbody>
          </table>
        </section>
      </Show>
      <section aria-label="backup-ct row">
        <table class="w-full table-fixed">
          <tbody>
            <GuestRow
              guest={ct}
              visibleColumnIds={['name']}
              workloadTableLayoutMode={window.innerWidth <= 768 ? 'phone' : 'wide'}
              onClick={() => setActions((n) => n + 1)}
            />
          </tbody>
        </table>
      </section>
      <p role="status" aria-label="Row action count">
        {actions()}
      </p>
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
