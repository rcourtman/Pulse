// #2263 browser fixture: mount the production guest overview and its real
// resource query, not a reimplementation of the disk/RAID presentation.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { GuestDrawerOverview } from '../src/components/Workloads/GuestDrawerOverview';
import '../src/index.css';

type Mode = 'linked' | 'none' | 'error';
const [mode, setMode] = createSignal<Mode>('linked');
const baseGuest = {
  id: 'vm-table-101',
  canonicalResourceId: 'vm-resource-101',
  name: 'guest-101',
  vmid: 101,
  type: 'qemu',
  status: 'running',
  cpus: 2,
  node: 'pve-a',
  instance: 'lab',
  cpu: 0,
  disk: { total: 10_000_000_000, used: 3_000_000_000, usage: 30 },
  networkIn: 0,
  networkOut: 0,
  diskRead: 0,
  diskWrite: 0,
  uptime: 3600,
  template: false,
  lastBackup: 0,
  tags: [],
  lock: '',
  lastSeen: '2026-09-26T12:00:00Z',
  memory: { total: 4_000_000_000, used: 1_000_000_000, free: 3_000_000_000, usage: 25 },
  disks: [],
};
const linkedGuest = {
  ...baseGuest,
  agentKind: 'pulse',
  agentRaid: [{
    device: '/dev/md0', name: 'data', level: 'raid1', state: 'clean',
    totalDevices: 2, activeDevices: 2, workingDevices: 2, failedDevices: 0,
    spareDevices: 0, devices: [], rebuildPercent: 0,
  }],
};
const errorGuest = {
  ...linkedGuest,
  canonicalResourceId: 'vm-resource-error',
  agentRaid: [],
};
const overviewProps = {
  guestOsSummary: 'Debian 13', agentHeading: 'Agent', agentLabel: 'Connected',
  agentTitle: 'Connected', hasAgentInfo: true, hasFilesystemDetails: false,
  hasNetworkInterfaces: false, hasOsInfo: true, hasWorkloadActionAgent: false,
  showInGuestAgentInstallCue: false, ipAddresses: [], networkInterfaces: [],
  normalizedTags: [], backupPresentation: null, workloadActionAgentTitle: '',
};

render(() => (
  <main class="mx-auto max-w-3xl space-y-4 p-4">
    <h1 class="text-lg font-semibold">Guest storage browser fixture</h1>
    <nav class="flex gap-2" aria-label="Fixture states">
      <button type="button" onClick={() => setMode('linked')}>Linked guest</button>
      <button type="button" onClick={() => setMode('none')}>No agent</button>
      <button type="button" onClick={() => setMode('error')}>Unavailable query</button>
    </nav>
    <Show when={mode() === 'linked'}>
      <GuestDrawerOverview {...overviewProps} guest={linkedGuest as any} />
    </Show>
    <Show when={mode() === 'none'}>
      <GuestDrawerOverview {...overviewProps} guest={baseGuest as any} />
    </Show>
    <Show when={mode() === 'error'}>
      <GuestDrawerOverview {...overviewProps} guest={errorGuest as any} />
    </Show>
  </main>
), document.getElementById('root')!);
