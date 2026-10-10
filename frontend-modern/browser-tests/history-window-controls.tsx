import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { HistoryChartHeader } from '../src/components/shared/HistoryChartHeader';
import { HISTORY_CHART_RANGES } from '../src/components/shared/historyChartModel';
import type { HistoryChartState } from '../src/components/shared/useHistoryChartState';
import type { HistoryTimeRange } from '../src/api/charts';
import '../src/index.css';

const calls: Array<{ chart: string; range: HistoryTimeRange }> = [];
let submissions = 0;
const makeChart = (name: string) => {
  const [range, setRange] = createSignal<HistoryTimeRange>('1h');
  return {
    ranges: HISTORY_CHART_RANGES,
    range,
    updateRange: (next: HistoryTimeRange) => {
      calls.push({ chart: name, range: next });
      setRange(next);
    },
    dataMin: () => 0,
    dataMax: () => 75,
    source: () => 'store',
  } as unknown as HistoryChartState;
};
const cpu = makeChart('CPU');
const memory = makeChart('Memory');

Object.assign(window, { __historyWindows: { calls, submissions: () => submissions } });
render(
  () => (
    <main class="min-h-screen space-y-4 bg-surface p-4 text-base-content">
      <h1 class="text-lg font-semibold">History window controls</h1>
      <p>Production header and styles; synthetic readings. No collector or API is contacted.</p>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          submissions += 1;
        }}
      >
        <section data-chart="CPU" class="mb-4 max-w-xl rounded-md border border-border p-3">
          <HistoryChartHeader chart={cpu} label="CPU" unit="%" />
        </section>
        <section data-chart="Memory" class="mb-4 max-w-xl rounded-md border border-border p-3">
          <HistoryChartHeader chart={memory} label="Memory" unit="%" />
        </section>
        <section data-chart="hidden" class="max-w-xl rounded-md border border-border p-3">
          <HistoryChartHeader chart={memory} label="Disk I/O" hideSelector />
        </section>
      </form>
      <button id="after-chart" type="button" class="border border-border px-3 py-1">
        After charts
      </button>
    </main>
  ),
  document.getElementById('root')!,
);
