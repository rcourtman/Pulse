import { render } from 'solid-js/web';
import { DiskDetail } from '../src/components/Storage/DiskDetail';
import { StoragePoolDetail } from '../src/components/Storage/StoragePoolDetail';
import type { StorageRecord } from '../src/features/storageBackups/models';
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

render(
  () => (
    <main class="min-h-screen space-y-4 bg-surface p-4 text-base-content">
      <h1>Storage History touch verification</h1>
      <p>Synthetic stored samples in production disk and pool details.</p>
      <section aria-label="Disk fixture" class="max-w-3xl">
        <DiskDetail disk={disk} nodes={[]} />
      </section>
      <section aria-label="Pool fixture" class="max-w-3xl">
        <table class="w-full table-fixed">
          <tbody>
            <StoragePoolDetail record={pool} physicalDisks={[]} summarySeriesId="fixture-pool" />
          </tbody>
        </table>
      </section>
      <button class="min-h-11 border px-3">After charts</button>
      <div style={{ height: '1000px' }} aria-hidden="true" />
    </main>
  ),
  document.getElementById('root')!,
);
