import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { HistoryChart } from '../src/components/shared/HistoryChart';
import '../src/index.css';
const Fixture = () => {
  const [target, setTarget] = createSignal('a');
  return (
    <main class="min-h-screen space-y-4 bg-surface p-4 text-base-content">
      <h1>Storage History request states</h1>
      <p>Synthetic transport; production History chart. Selected pool: {target()}</p>
      <button class="min-h-11 border px-3" onClick={() => setTarget('b')}>
        Select pool b
      </button>
      <section class="max-w-xl">
        <HistoryChart
          resourceType="storage"
          resourceId={target()}
          metric="usage"
          label="Usage"
          unit="%"
          hideSelector
          compact
          range="1h"
          height={140}
        />
      </section>
    </main>
  );
};
render(() => <Fixture />, document.getElementById('root')!);
