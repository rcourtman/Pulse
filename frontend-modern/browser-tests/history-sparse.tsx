import { render } from 'solid-js/web';
import { HistoryChart } from '../src/components/shared/HistoryChart';
import '../src/index.css';

render(
  () => (
    <main class="min-h-screen space-y-4 bg-surface p-4 text-base-content">
      <h1>Storage History: sparse observations</h1>
      <p>Synthetic stored samples; production chart, transport and styles.</p>
      <button class="min-h-11 border px-3">Before chart</button>
      <section class="max-w-xl">
        <HistoryChart
          resourceType="storage"
          resourceId="fixture-pool"
          metric="usage"
          label="Usage"
          unit="%"
          hideSelector
          compact
          range="1h"
          height={140}
        />
      </section>
      <button class="min-h-11 border px-3">After chart</button>
    </main>
  ),
  document.getElementById('root')!,
);
