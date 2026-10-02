// Production pool drawer with synthetic partial collector snapshots; not appliance proof.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { StoragePoolDetail } from '../src/components/Storage/StoragePoolDetail';
import type { CapacitySnapshot, StorageRecord } from '../src/features/storageBackups/models';
import '../src/index.css';

const samples: Record<string, CapacitySnapshot> = {
  missing: { totalBytes: 1024, usedBytes: null, freeBytes: null, usagePercent: null },
  empty: { totalBytes: 1024, usedBytes: 0, freeBytes: null, usagePercent: null },
  full: { totalBytes: 1024, usedBytes: 1024, freeBytes: 0, usagePercent: 100 },
  partial: { totalBytes: null, usedBytes: 512, freeBytes: 0, usagePercent: null },
  absent: { totalBytes: null, usedBytes: null, freeBytes: null, usagePercent: null },
};
const Fixture = () => {
  const [phase, setPhase] = createSignal('missing');
  const record = (): StorageRecord => ({
    id: 'synthetic-pbs-pool',
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
    capacity: samples[phase()],
    capabilities: ['capacity'],
    observedAt: Date.now(),
  });
  return (
    <main class="min-h-screen space-y-4 bg-surface p-4 text-base-content">
      <h1 class="text-lg font-semibold">Storage capacity absence verification</h1>
      <p>Synthetic partial PBS snapshots in the production pool drawer.</p>
      <div class="flex flex-wrap gap-2">
        {Object.keys(samples).map((name) => (
          <button class="min-h-11 rounded border border-border px-3" onClick={() => setPhase(name)}>
            {name}
          </button>
        ))}
      </div>
      <table class="w-full table-fixed">
        <tbody>
          <StoragePoolDetail
            record={record()}
            physicalDisks={[]}
            summarySeriesId="synthetic-pbs-pool"
          />
        </tbody>
      </table>
    </main>
  );
};
render(() => <Fixture />, document.getElementById('root')!);
