// Production disk model, shell, ResizeObserver and CSS; synthetic observations.
// This fixture verifies composition, not collection, thaw or native recovery.
import { createSignal, For } from 'solid-js';
import { render } from 'solid-js/web';
import { StackedDiskBar } from '../src/components/Workloads/StackedDiskBar';
import {
  buildStackedDiskBarPresentation,
  type StackedDiskBarProps,
} from '../src/components/Workloads/stackedDiskBarModel';
import type { Disk } from '../src/types/api';
import type { AnomalyReport } from '../src/types/aiIntelligence';
import '../src/index.css';

const GiB = 1024 ** 3;
const disk = (mountpoint: string, total: number, used: number): Disk => ({
  mountpoint,
  total: total * GiB,
  used: used * GiB,
  free: (total - used) * GiB,
  usage: (used / total) * 100,
  type: 'ext4',
});
const single = [disk('/data', 440, 265)];
const multiple = [disk('/MMMM', 180, 90), disk('/iiii', 260, 175)];
const anomaly: AnomalyReport = {
  resource_id: 'fixture-pve:pve-a:101',
  resource_name: 'backup-guest',
  resource_type: 'vm',
  metric: 'disk',
  current_value: 60,
  baseline_mean: 24,
  baseline_std_dev: 4,
  z_score: 9,
  severity: 'high',
  description: 'Synthetic disk anomaly',
};
const cases: { id: string; label: string; props: StackedDiskBarProps }[] = [
  { id: 'single', label: 'Single filesystem', props: { disks: single } },
  { id: 'anomaly', label: 'Filesystem with anomaly', props: { disks: single, anomaly } },
  {
    id: 'aggregate',
    label: 'Aggregate with fullest disk and count',
    props: { disks: multiple, mode: 'aggregate', showDiskCount: true },
  },
  { id: 'inline', label: 'Equal slots, different glyph widths', props: { disks: multiple } },
  {
    id: 'vertical',
    label: 'Host micro-bars and fullest disk',
    props: { disks: multiple, mode: 'vertical-bars' },
  },
];

function Fixture() {
  const [width, setWidth] = createSignal(100);
  const [status, setStatus] = createSignal(
    'Using last known disk stats. Guest reads paused while a backup is running.',
  );
  (window as any).__composedDiskLabels = {
    width: setWidth,
    presentation: () =>
      Object.fromEntries(
        cases.map((item) => [item.id, buildStackedDiskBarPresentation(item.props, width())]),
      ),
  };
  return (
    <main class="min-h-screen space-y-5 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Composed disk evidence</h1>
      <div class="flex flex-wrap gap-2">
        <button class="rounded-sm border border-border p-2" onClick={() => setWidth(100)}>
          Use compact bars
        </button>
        <button class="rounded-sm border border-border p-2" onClick={() => setWidth(320)}>
          Use roomy bars
        </button>
        <button class="rounded-sm border border-border p-2" onClick={() => setStatus('')}>
          Mark observation fresh
        </button>
      </div>
      <For each={cases}>
        {(item) => (
          <section aria-label={item.label} class="space-y-2">
            <h2 class="text-sm font-medium">{item.label}</h2>
            <div data-fit={item.id} style={{ width: `${width()}px` }}>
              <StackedDiskBar {...item.props} statusMessage={status()} />
            </div>
          </section>
        )}
      </For>
    </main>
  );
}

render(() => <Fixture />, document.getElementById('root')!);
