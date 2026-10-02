import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { DiskDetail } from '../src/components/Storage/DiskDetail';
import type { Resource } from '../src/types/resource';
import { temperatureStore } from '../src/utils/temperature';
import '../src/index.css';

const Fixture = () => {
  const initial = {
    id: 'disk-fixture',
    type: 'physical_disk',
    name: 'Archive HDD',
    displayName: 'Archive HDD',
    platformType: 'proxmox-pve',
    platformId: 'fixture',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    identity: { hostname: 'storage-host' },
    physicalDisk: { devPath: '/dev/sda', model: 'Archive HDD', diskType: 'hdd', temperature: 42 },
  } as Resource;
  const [disk, setDisk] = createSignal(initial);
  const reading = (
    temperature: number,
    smart?: { powerOnHours: number; reallocatedSectors: number },
  ) => {
    temperatureStore.setUnit('celsius');
    setDisk({ ...initial, physicalDisk: { ...initial.physicalDisk!, temperature, smart } });
  };
  return (
    <main class="min-h-screen space-y-4 bg-surface p-4 text-base-content">
      <h1>Physical disk Overview</h1>
      <p>Synthetic disk snapshots; production detail component.</p>
      <div class="flex flex-wrap gap-2">
        <button class="min-h-11 border px-3" onClick={() => reading(65)}>
          Hot reading
        </button>
        <button class="min-h-11 border px-3" onClick={() => temperatureStore.setUnit('fahrenheit')}>
          Fahrenheit
        </button>
        <button class="min-h-11 border px-3" onClick={() => reading(0)}>
          No reading
        </button>
        <button
          class="min-h-11 border px-3"
          onClick={() => reading(42, { powerOnHours: 100, reallocatedSectors: 0 })}
        >
          Extended SMART
        </button>
      </div>
      <section class="max-w-3xl">
        <DiskDetail disk={disk()} nodes={[]} />
      </section>
    </main>
  );
};
render(() => <Fixture />, document.getElementById('root')!);
