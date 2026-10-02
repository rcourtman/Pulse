import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { HistoryChart, HistoryChartHoverGroup } from '../src/components/shared/HistoryChart';
import '../src/index.css';
const Fixture = () => {
  const [empty, setEmpty] = createSignal(false);
  const [target, setTarget] = createSignal('a');
  const points = () =>
    empty()
      ? []
      : [10, 20, 30].map((value, i) => ({
          timestamp: 1790942400000 + i * 60000,
          value,
          min: value,
          max: value,
        }));
  return (
    <main class="min-h-screen space-y-4 bg-surface p-4 text-base-content">
      <h1>Storage History keyboard inspection</h1>
      <button onClick={() => setEmpty(!empty())}>Toggle empty</button>
      <button onClick={() => setTarget(target() === 'a' ? 'b' : 'a')}>Change target</button>
      <HistoryChartHoverGroup>
        <HistoryChart
          resourceType="disk"
          resourceId={target()}
          metric="usage"
          label="Usage"
          unit="%"
          hideSelector
          range="1h"
          data={points()}
        />
        <HistoryChart
          resourceType="disk"
          resourceId={target()}
          metric="diskread"
          label="Read"
          unit="B/s"
          hideSelector
          range="1h"
          data={points()}
        />
      </HistoryChartHoverGroup>
      <button>After charts</button>
    </main>
  );
};
render(() => <Fixture />, document.getElementById('root')!);
