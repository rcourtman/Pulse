import { createMemo, createSignal, For } from 'solid-js';
import { render } from 'solid-js/web';
import { StoragePoolDetail } from '../src/components/Storage/StoragePoolDetail';
import { buildStorageRecords } from '../src/features/storageBackups/storageAdapters';
import type { State } from '../src/types/api';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const modes = [
  'ZFS host A',
  'ZFS host B',
  'UnRAID host A',
  'UnRAID host B',
  'Missing ownership',
  'Direct child',
];
const [mode, setMode] = createSignal(modes[0]);
const resource = createMemo(() => {
  const unraid = mode().startsWith('UnRAID');
  const b = mode().endsWith('B');
  const unknown = mode() === 'Missing ownership';
  return {
    id: b ? 'pool-b' : 'pool-a',
    type: 'storage',
    name: unraid ? 'Array' : 'tank',
    displayName: 'tank',
    platformType: unraid ? 'unraid' : 'truenas',
    platformId: 'fixture',
    sourceType: 'api',
    parentId: unknown ? undefined : b ? 'host-b' : 'host-a',
    parentName: 'Storage host',
    status: 'online',
    lastSeen: Date.now(),
    metricsTarget: { resourceType: 'storage', resourceId: b ? 'history-b' : 'history-a' },
    disk: { current: 25, total: 1024, used: 256, free: 768 },
    storage: unraid
      ? { type: 'unraid-array', platform: 'unraid' }
      : {
          type: 'zfs-pool',
          isZfs: true,
          zfsPool: {
            name: 'tank',
            state: 'ONLINE',
            readErrors: 0,
            writeErrors: 0,
            checksumErrors: 0,
            devices: [{ name: 'sda', type: 'disk', state: 'ONLINE' }],
          },
        },
  } as Resource;
});
const record = createMemo(
  () => buildStorageRecords({ state: {} as State, resources: [resource()] })[0],
);
const disks = createMemo(() =>
  [
    ['a', 'host-a', 'Host A disk', '/dev/sda', 34, 0],
    ['b', 'host-b', 'Host B disk', '/dev/sda', 82, 99],
    ['unknown', undefined, 'Unowned disk', '/dev/sda', 70, 0],
    ['suffix', 'host-a', 'Suffix collision disk', '/dev/not-sda', 66, 0],
  ].map(
    ([id, owner, model, devPath, temperature, errorCount]) =>
      ({
        id: `disk-${id}`,
        type: 'physical_disk',
        name: model,
        displayName: model,
        platformType: 'truenas',
        platformId: 'fixture',
        sourceType: 'api',
        status: 'online',
        lastSeen: Date.now(),
        parentId: mode() === 'Direct child' && id === 'a' ? 'pool-a' : owner,
        parentName: 'Storage host',
        physicalDisk: {
          devPath,
          model,
          temperature,
          errorCount,
          storageGroup: id === 'suffix' ? 'other-group' : 'unraid-array',
          storageRole: 'data',
          sizeBytes: 1024,
          storageState: 'online',
        },
      }) as Resource,
  ),
);

render(
  () => (
    <main class="min-h-screen space-y-4 bg-surface p-4 text-base-content">
      <h1>Pool ownership verification</h1>
      <p>
        Repeated disk names and groups across synthetic hosts, using the production Storage detail.
      </p>
      <nav aria-label="Fixture selection" class="flex flex-wrap gap-2">
        <For each={modes}>
          {(value) => (
            <button
              class="min-h-11 rounded border border-border px-3"
              onClick={() => setMode(value)}
            >
              {value}
            </button>
          )}
        </For>
      </nav>
      <section aria-label="Pool detail">
        <h2>{mode()}</h2>
        <table class="w-full table-fixed">
          <tbody>
            <StoragePoolDetail
              record={record()}
              physicalDisks={disks()}
              summarySeriesId="ownership-pool"
            />
          </tbody>
        </table>
      </section>
    </main>
  ),
  document.getElementById('root')!,
);
