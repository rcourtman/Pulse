// Direct production renderer proof; this is not the complete PBS table/drawer.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import type { HistoryTimeRange } from '../src/api/charts';
import {
  GuestDrawerHistory,
  GuestDrawerHistoryRangeSelect,
} from '../src/components/Workloads/GuestDrawerHistory';
import { HOST_METRICS_HISTORY_GROUPS } from '../src/components/shared/hostMetricsHistoryModel';
import '../src/index.css';

const Fixture = () => {
  const [resourceId, setResourceId] = createSignal('agent-three');
  const [range, setRange] = createSignal<HistoryTimeRange>('24h');
  return (
    <main class="min-h-screen bg-surface p-4 text-base-content">
      <h1 class="mb-4 text-lg font-semibold">History refresh recovery verification</h1>
      <button class="mb-4" onClick={() => setResourceId('pbs-three')}>
        Switch to service target
      </button>
      <section data-testid="history-refresh-fixture" class="space-y-3">
        <div class="flex justify-end">
          <GuestDrawerHistoryRangeSelect range={range()} onRangeChange={setRange} />
        </div>
        <GuestDrawerHistory
          target={{ resourceType: 'agent', resourceId: resourceId() }}
          range={range()}
          groups={HOST_METRICS_HISTORY_GROUPS}
          currentMetrics={{ cpu: 42, memory: 53 }}
        />
      </section>
    </main>
  );
};
render(() => <Fixture />, document.getElementById('root') as HTMLElement);
