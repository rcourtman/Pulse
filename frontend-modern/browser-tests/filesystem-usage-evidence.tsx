// Production list and Overview, driven only by synthetic local observations.
// This fixture cannot collect guest data or establish native backup safety.
import { createMemo, createSignal, For } from 'solid-js';
import { render } from 'solid-js/web';
import { DiskList } from '../src/components/Workloads/DiskList';
import { GuestDrawerOverview } from '../src/components/Workloads/GuestDrawerOverview';
import type { Disk, VM } from '../src/types/api';
import '../src/index.css';

const capacity = 10 * 1024 ** 3;
const cases: Record<string, { values: Partial<Disk>; reason?: string }> = {
  live: { values: {} },
  'missing-used': { values: { used: undefined } },
  'negative-used': { values: { used: -1 } },
  'NaN-used': { values: { used: NaN } },
  'infinite-used': { values: { used: Infinity } },
  'missing-total': { values: { total: undefined } },
  'zero-total': { values: { total: 0 } },
  'negative-total': { values: { total: -1 } },
  'NaN-total': { values: { total: NaN } },
  'infinite-total': { values: { total: Infinity } },
  'unknown-sentinel': { values: { usage: -1, used: 0 } },
  'NaN-usage': { values: { usage: NaN } },
  'infinite-usage': { values: { usage: Infinity } },
  overflow: { values: { total: 1, used: Number.MAX_VALUE } },
  zero: { values: { used: 0, usage: undefined } },
  'usage-omitted': { values: { usage: undefined } },
  'over-capacity': { values: { used: capacity * 1.25, usage: 0 } },
  'retained-live': { values: {}, reason: 'prev-vm-locked' },
  'retained-missing': { values: { used: undefined }, reason: 'prev-agent-busy' },
};

function Fixture() {
  const [selected, setSelected] = createSignal('live');
  const observation = createMemo(() => ({
    mountpoint: '/data',
    type: 'ext4',
    total: capacity,
    used: capacity / 2,
    usage: 50,
    ...cases[selected()].values,
  }));
  const guest = createMemo<VM>(() => ({
    id: 'fixture:pve-a:101',
    vmid: 101,
    name: 'filesystem-evidence',
    node: 'pve-a',
    instance: 'fixture',
    status: 'running',
    type: 'qemu',
    cpu: 0.1,
    cpus: 2,
    memory: { total: capacity, used: capacity / 2, free: capacity / 2, usage: 50 },
    disk: observation(),
    disks: [observation()],
    diskStatusReason: cases[selected()].reason,
    networkIn: 0,
    networkOut: 0,
    diskRead: 0,
    diskWrite: 0,
    uptime: 3600,
    template: false,
    lastBackup: 0,
    tags: [],
    lock: '',
    lastSeen: '2026-10-04T04:00:00Z',
  }));
  return (
    <main class="min-h-screen space-y-4 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Filesystem usage evidence</h1>
      <label for="fixture-observation" class="flex flex-wrap items-center gap-2 text-sm">
        Synthetic observation
        <select
          id="fixture-observation"
          class="rounded-sm border border-border bg-surface p-2"
          value={selected()}
          onChange={(event) => setSelected(event.currentTarget.value)}
        >
          <For each={Object.keys(cases)}>{(name) => <option value={name}>{name}</option>}</For>
        </select>
      </label>
      <section aria-label="Filesystem list" class="max-w-xl">
        <DiskList disks={[observation()]} diskStatusReason={cases[selected()].reason} />
      </section>
      <section aria-label="Guest Overview">
        <GuestDrawerOverview
          guest={guest()}
          guestOsSummary=""
          agentHeading="Guest agent"
          agentLabel=""
          agentTitle=""
          hasAgentInfo={false}
          hasFilesystemDetails={true}
          hasNetworkInterfaces={false}
          hasOsInfo={false}
          hasWorkloadActionAgent={false}
          showInGuestAgentInstallCue={false}
          ipAddresses={[]}
          networkInterfaces={[]}
          normalizedTags={[]}
          backupPresentation={null}
          workloadActionAgentTitle=""
        />
      </section>
    </main>
  );
}

render(() => <Fixture />, document.getElementById('root')!);
