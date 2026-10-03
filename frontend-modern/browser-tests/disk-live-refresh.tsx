// Real DiskList/DiskDetail and keyed renderer, with synthetic collector snapshots.
// This is browser behaviour evidence, not an installed SMART or appliance result.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { DiskList } from '../src/components/Storage/DiskList';
import type { StorageHealthFilter } from '../src/features/storageBackups/models';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const disk = (phase: 'healthy' | 'fault' | 'missing'): Resource => ({
  id: 'disk-one',
  type: 'physical_disk',
  name: 'Archive disk',
  platformType: 'proxmox-pbs',
  sourceType: 'agent',
  status: 'online',
  lastSeen: Date.now(),
  metricsTarget: {
    resourceType: 'disk',
    resourceId: phase === 'healthy' ? 'agent-archive:sda' : 'agent-archive:sdz',
  },
  identity: { hostname: 'archive-host' },
  canonicalIdentity: { hostname: 'archive-host' },
  physicalDisk: {
    model: phase === 'fault' ? 'Archive SSD (fault)' : 'Archive SSD',
    devPath: phase === 'healthy' ? '/dev/sda' : '/dev/sdz',
    serial: 'SYNTHETIC-ONE',
    diskType: phase === 'missing' ? undefined : 'ssd',
    health: phase === 'healthy' ? 'PASSED' : phase === 'fault' ? 'FAILED' : 'UNKNOWN',
    wearout: phase === 'healthy' ? 96 : phase === 'fault' ? 4 : -1,
    temperature: phase === 'healthy' ? 41 : phase === 'fault' ? 63 : 0,
    sizeBytes: phase === 'missing' ? 0 : 2_000_000_000_000,
    storageRole: phase === 'missing' ? undefined : 'cache_pool',
    storageGroup: phase === 'missing' ? undefined : 'Archive Pool',
    risk:
      phase === 'fault'
        ? {
            level: 'critical',
            reasons: [{ code: 'smart-failed', severity: 'critical', summary: 'SMART failed.' }],
          }
        : undefined,
    smart:
      phase === 'missing'
        ? undefined
        : { powerOnHours: 100, pendingSectors: phase === 'fault' ? 2 : 0 },
    collection:
      phase === 'missing'
        ? {
            temperature: { state: 'unavailable', source: 'fixture', reason: 'No current reading' },
          }
        : undefined,
  },
});

const Fixture = () => {
  const [disks, setDisks] = createSignal([disk('healthy')]);
  const [selectedDiskId, setSelectedDiskId] = createSignal<string | null>(null);
  const [healthFilter, setHealthFilter] = createSignal<StorageHealthFilter>('all');
  return (
    <main class="min-h-screen space-y-4 bg-surface p-4 text-base-content">
      <h1 class="text-lg font-semibold">Physical disk live refresh verification</h1>
      <p class="text-sm text-muted">
        Synthetic snapshots in the production PBS disk table and detail.
      </p>
      <div class="flex flex-wrap gap-2">
        <button
          class="min-h-11 rounded border border-border px-3"
          data-update="fault"
          onClick={() => setDisks([disk('fault')])}
        >
          Report disk fault
        </button>
        <button
          class="min-h-11 rounded border border-border px-3"
          data-update="missing"
          onClick={() => setDisks([disk('missing')])}
        >
          Remove current readings
        </button>
        <button
          class="min-h-11 rounded border border-border px-3"
          data-update="healthy"
          onClick={() => setDisks([disk('healthy')])}
        >
          Recover disk
        </button>
        <button
          class="min-h-11 rounded border border-border px-3"
          data-filter="attention"
          onClick={() => setHealthFilter('attention')}
        >
          Show disks needing attention
        </button>
        <button
          class="min-h-11 rounded border border-border px-3"
          data-filter="all"
          onClick={() => setHealthFilter('all')}
        >
          Show all disks
        </button>
      </div>
      <section data-testid="disk-live-refresh-fixture">
        <DiskList
          disks={disks()}
          nodes={[]}
          selectedNode={null}
          healthFilter={healthFilter()}
          searchTerm=""
          selectedDiskId={selectedDiskId()}
          onSelectedDiskChange={setSelectedDiskId}
        />
      </section>
    </main>
  );
};
render(() => <Fixture />, document.getElementById('root')!);
