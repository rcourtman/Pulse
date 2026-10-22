import { createMemo, createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import type { HistoryTimeRange } from '../src/api/charts';
import { DiskDrawerOverview } from '../src/components/Storage/DiskDrawerOverview';
import { StoragePoolDetail } from '../src/components/Storage/StoragePoolDetail';
import { GUEST_DRAWER_HISTORY_DEFAULT_RANGE } from '../src/components/Workloads/guestDrawerModel';
import { extractPhysicalDiskPresentationData } from '../src/features/storageBackups/diskPresentation';
import type { StorageRecord } from '../src/features/storageBackups/models';
import { useAlertsActivation } from '../src/stores/alertsActivation';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const disk = {
  id: 'fixture-disk',
  type: 'physical_disk',
  name: 'Archive HDD',
  displayName: 'Archive HDD',
  platformType: 'proxmox-pve',
  platformId: 'fixture',
  sourceType: 'api',
  status: 'online',
  lastSeen: Date.now(),
  identity: { hostname: 'storage-host' },
  metricsTarget: { resourceType: 'disk', resourceId: 'fixture-disk' },
  physicalDisk: { devPath: '/dev/sda', model: 'Archive HDD', diskType: 'hdd', temperature: 42 },
} as Resource;

const pool: StorageRecord = {
  id: 'fixture-pool',
  name: 'Archive',
  category: 'datastore',
  health: 'unknown',
  location: { label: 'archive-host', scope: 'host' },
  source: {
    platform: 'proxmox-pbs',
    family: 'onprem',
    origin: 'resource',
    adapterId: 'resource-storage',
  },
  capacity: { totalBytes: 1024, usedBytes: 512, freeBytes: 512, usagePercent: 50 },
  capabilities: ['capacity'],
  observedAt: Date.now(),
};

// The disk and pool drawers' Overviews, wired as their drawers wire them.
const Fixture = () => {
  const { getDiskTemperatureThresholds } = useAlertsActivation();
  const diskData = createMemo(() =>
    extractPhysicalDiskPresentationData(disk, getDiskTemperatureThresholds),
  );
  const [diskRange, setDiskRange] = createSignal<HistoryTimeRange>(
    GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  );
  const [poolRange, setPoolRange] = createSignal<HistoryTimeRange>(
    GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  );
  return (
    <main class="min-h-screen space-y-4 bg-surface p-4 text-base-content">
      <h1>Storage History touch verification</h1>
      <p>Synthetic stored samples in production disk and pool drawer Overviews.</p>
      <section aria-label="Disk fixture" class="max-w-3xl">
        <DiskDrawerOverview
          disk={disk}
          diskData={diskData()}
          historyRange={diskRange()}
          onHistoryRangeChange={setDiskRange}
        />
      </section>
      <section aria-label="Pool fixture" class="max-w-3xl">
        <StoragePoolDetail
          record={pool}
          physicalDisks={[]}
          historyRange={poolRange()}
          onHistoryRangeChange={setPoolRange}
        />
      </section>
      <button class="min-h-11 border px-3">After charts</button>
      <div style={{ height: '1000px' }} aria-hidden="true" />
    </main>
  );
};

render(() => <Fixture />, document.getElementById('root')!);
