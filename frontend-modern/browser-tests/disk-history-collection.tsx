import { createMemo, createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import type { HistoryTimeRange } from '../src/api/charts';
import { DiskDrawerOverview } from '../src/components/Storage/DiskDrawerOverview';
import { GUEST_DRAWER_HISTORY_DEFAULT_RANGE } from '../src/components/Workloads/guestDrawerModel';
import { extractPhysicalDiskPresentationData } from '../src/features/storageBackups/diskPresentation';
import { useAlertsActivation } from '../src/stores/alertsActivation';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

// Only snapshots are synthetic: the disk Overview, drawer history, client,
// inspection and CSS are production code, wired as the disk drawers wire them.
const disk = (missing = false, nvme = false, hot = false): Resource => ({
  id: nvme ? 'disk-b' : 'disk-a',
  name: nvme ? 'NVMe B' : 'Archive A',
  displayName: nvme ? 'NVMe B' : 'Archive A',
  type: 'physical_disk',
  platformId: nvme ? 'nas-b' : 'nas-a',
  platformType: 'truenas',
  sourceType: 'api',
  status: 'online',
  lastSeen: Date.now(),
  identity: { hostname: nvme ? 'nas-b' : 'nas-a' },
  metricsTarget: {
    resourceType: 'disk',
    resourceId: nvme ? 'disk:nas-b:nvme0n1' : 'disk:nas-a:sda',
  },
  physicalDisk: {
    devPath: nvme ? '/dev/nvme0n1' : '/dev/sda',
    model: nvme ? 'NVMe B' : 'Archive HDD A',
    serial: nvme ? 'SERIAL-B' : 'SERIAL-A',
    diskType: nvme ? 'nvme' : 'hdd',
    temperature: missing ? undefined : hot ? 65 : 42,
    smart: missing
      ? undefined
      : nvme
        ? { percentageUsed: 12, availableSpare: 97 }
        : { reallocatedSectors: 0 },
    collection: {
      temperature: {
        state: missing ? 'unavailable' : 'available',
        reason: missing ? 'collection deadline exceeded' : undefined,
      },
      io: { state: 'unsupported', reason: 'per-member counters unavailable' },
    },
  },
});
const [snapshot, setSnapshot] = createSignal(disk(true));
const [range, setRange] = createSignal<HistoryTimeRange>(GUEST_DRAWER_HISTORY_DEFAULT_RANGE);
const controls = [
  ['Restore current fields', () => setSnapshot(disk())],
  ['Update current fields', () => setSnapshot(disk(false, false, true))],
  ['Lose current fields', () => setSnapshot(disk(true))],
  ['Select NVMe B', () => setSnapshot(disk(true, true))],
  [
    'Clear identity',
    () =>
      setSnapshot({
        ...disk(true),
        id: '',
        metricsTarget: undefined,
        physicalDisk: { ...disk(true).physicalDisk!, serial: '', wwn: '' },
      }),
  ],
] as const;
const Fixture = () => {
  const { getDiskTemperatureThresholds } = useAlertsActivation();
  const diskData = createMemo(() =>
    extractPhysicalDiskPresentationData(snapshot(), getDiskTemperatureThresholds),
  );
  return (
    <main class="mx-auto max-w-5xl bg-surface p-3 text-base-content">
      <h1 class="mb-3 text-lg font-semibold">Physical disk History</h1>
      <div class="mb-4 flex flex-wrap gap-2">
        {controls.map(([label, action]) => (
          <button class="min-h-11 rounded border border-border px-3 text-sm" onClick={action}>
            {label}
          </button>
        ))}
      </div>
      <DiskDrawerOverview
        disk={snapshot()}
        diskData={diskData()}
        historyRange={range()}
        onHistoryRangeChange={setRange}
      />
    </main>
  );
};
render(() => <Fixture />, document.getElementById('root')!);
