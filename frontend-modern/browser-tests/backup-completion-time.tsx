// Existing producer/readouts only: synthetic timestamps, no guest or backup operations.
import { createSignal, onMount } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { useAlertsActivation } from '../src/stores/alertsActivation';
import type { WorkloadGuest } from '../src/types/workloads';
import '../src/index.css';

const original = Date.parse('2026-10-12T00:30:00Z');
function Fixture() {
  const [type, setType] = createSignal<'qemu' | 'lxc'>('qemu');
  const [timestamp, setTimestamp] = createSignal<string | number>(original);
  const [running, setRunning] = createSignal(false);
  const [showBackup, setShowBackup] = createSignal(true);
  const [rowActions, setRowActions] = createSignal(0);
  const [policyReady, setPolicyReady] = createSignal(false);
  const policy = useAlertsActivation();
  onMount(async () => {
    await policy.refreshConfig();
    setPolicyReady(true);
  });
  const guest = (): WorkloadGuest => ({
    id: 'time-fixture:pve-a:101',
    name: type() === 'qemu' ? 'backup-vm' : 'backup-ct',
    instance: 'time-fixture',
    node: 'pve-a',
    vmid: 101,
    type: type(),
    platformType: 'proxmox-pve',
    status: 'running',
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
    tags: [],
    lock: running() ? 'backup' : '',
    // The native workload producer supplies milliseconds. Unit controls
    // separately exercise the cell's string-input compatibility.
    lastBackup:
      typeof timestamp() === 'number' ? (timestamp() as number) : Date.parse(String(timestamp())),
    backupInProgress: running(),
    lastSeen: '2026-10-12T02:00:00Z',
  });
  const change = (state: string) => {
    setRunning(state === 'running' || state === 'absent');
    switch (state) {
      case 'offset-west':
        setTimestamp('2026-10-11T17:30:00-07:00');
        break;
      case 'offset-east':
        setTimestamp('2026-10-12T09:30:00+09:00');
        break;
      case 'new':
        setTimestamp('2026-10-12T01:45:00Z');
        break;
      case 'invalid':
        setTimestamp(NaN);
        break;
      case 'absent':
        setTimestamp(0);
        break;
      case 'future':
        setTimestamp(Date.now() + 30_000);
        break;
      default:
        setTimestamp(original);
    }
  };
  return (
    <main class="min-h-screen space-y-4 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Backup completion time</h1>
      <p class="text-xs text-muted">Synthetic evidence only; no backup or guest commands.</p>
      <div class="flex flex-wrap items-center gap-3 text-xs">
        <label>
          Guest type{' '}
          <select
            id="proof-type"
            value={type()}
            onChange={(e) => setType(e.currentTarget.value as 'qemu' | 'lxc')}
          >
            <option value="qemu">VM</option>
            <option value="lxc">LXC</option>
          </select>
        </label>
        <label>
          Observation{' '}
          <select id="proof-state" onChange={(e) => change(e.currentTarget.value)}>
            {[
              'numeric',
              'offset-west',
              'offset-east',
              'running',
              'ended',
              'new',
              'invalid',
              'absent',
              'future',
            ].map((state) => (
              <option value={state}>{state}</option>
            ))}
          </select>
        </label>
        <button
          type="button"
          class="rounded-sm border border-border p-2"
          onClick={() => setShowBackup(!showBackup())}
        >
          {showBackup() ? 'Hide Backup column' : 'Show Backup column'}
        </button>
      </div>
      <p data-testid="proof-policy" class="text-xs text-muted">
        {policyReady() ? 'Policy read' : 'Reading policy'}
      </p>
      <section aria-label="Backup table row">
        <table class="w-full table-fixed">
          <tbody>
            <GuestRow
              guest={guest()}
              visibleColumnIds={showBackup() ? ['name', 'backup'] : ['name']}
              workloadTableLayoutMode={window.innerWidth <= 768 ? 'phone' : 'wide'}
              onClick={() => setRowActions((n) => n + 1)}
            />
          </tbody>
        </table>
      </section>
      <p data-testid="proof-actions" class="text-xs text-muted">
        Row actions: {rowActions()}
      </p>
      <GuestDrawer guest={guest()} onClose={() => {}} />
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
