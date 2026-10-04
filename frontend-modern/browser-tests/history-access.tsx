// Production readers/client/styles with synthetic same-origin HTTP responses.
// This fixture proves presentation, not appliance authorisation or collection.
import { Show, createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import type { HistoryTimeRange } from '../src/api/charts';
import {
  GuestDrawerHistory,
  GuestDrawerHistoryRangeSelect,
} from '../src/components/Workloads/GuestDrawerHistory';
import { HistoryChart } from '../src/components/shared/HistoryChart';
import { HOST_METRICS_HISTORY_GROUPS } from '../src/components/shared/hostMetricsHistoryModel';
import '../src/index.css';

function Fixture() {
  const [range, setRange] = createSignal<HistoryTimeRange>('1h');
  const [visible, setVisible] = createSignal(true);
  return (
    <main class="mx-auto min-h-screen max-w-5xl space-y-5 bg-surface p-4 text-base-content">
      <h1 class="text-lg font-semibold">History access failure verification</h1>
      <section data-testid="disk" class="rounded border border-border p-3">
        <h2 class="mb-3 font-semibold">Disk History</h2>
        <HistoryChart
          resourceType="disk"
          resourceId="disk:nas-a:sda"
          metric="smart_temp"
          label="Temperature"
          unit="C"
          range="1h"
          height={160}
          hideSelector
          compact
        />
      </section>
      <section data-testid="service" class="space-y-3">
        <h2 class="font-semibold">PBS service History</h2>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <button
            class="min-h-11 rounded border border-border px-3"
            onClick={() => setVisible(!visible())}
          >
            {visible() ? 'Hide service History' : 'Show service History'}
          </button>
          <GuestDrawerHistoryRangeSelect range={range()} onRangeChange={setRange} />
        </div>
        <Show when={visible()}>
          <GuestDrawerHistory
            target={{ resourceType: 'agent', resourceId: 'pbs-service-a' }}
            range={range()}
            groups={HOST_METRICS_HISTORY_GROUPS}
            currentMetrics={{ cpu: 99, memory: 98 }}
          />
        </Show>
      </section>
    </main>
  );
}
render(() => <Fixture />, document.getElementById('root')!);
