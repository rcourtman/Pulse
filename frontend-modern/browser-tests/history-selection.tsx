// Synthetic delayed transport deliberately ignores AbortSignal to test response ownership.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { ChartsAPI, type SingleMetricHistoryResponse } from '../src/api/charts';
import { HistoryChart } from '../src/components/shared/HistoryChart';
import '../src/index.css';
const pending = new Map<
  string,
  Array<{
    resolve: (response: SingleMetricHistoryResponse) => void;
    reject: (error: Error) => void;
  }>
>();
ChartsAPI.getMetricsHistory = (params) =>
  new Promise((resolve, reject) => {
    pending.set(params.resourceId, [
      ...(pending.get(params.resourceId) ?? []),
      { resolve, reject },
    ]);
  });
const complete = (target: string, value: number, fail = false) => {
  for (const request of pending.get(target) ?? []) {
    if (fail) request.reject(new Error('Synthetic current-target failure'));
    else
      request.resolve({
        points: [0, 1, 2].map((i) => ({
          timestamp: 1790942400000 + i * 60000,
          value,
          min: value,
          max: value,
        })),
        source: 'store',
      } as SingleMetricHistoryResponse);
  }
  pending.delete(target);
};
const Fixture = () => {
  const [target, setTarget] = createSignal('a');
  return (
    <main class="min-h-screen space-y-4 bg-surface p-4 text-base-content">
      <h1>History selection verification</h1>
      <p>Synthetic delayed responses; selected target: {target()}</p>
      <div class="flex flex-wrap gap-2">
        {['b', 'c'].map((name) => (
          <button class="min-h-11 border px-3" onClick={() => setTarget(name)}>
            Select {name}
          </button>
        ))}
        <button class="min-h-11 border px-3" onClick={() => complete('b', 80)}>
          Finish b
        </button>
        <button class="min-h-11 border px-3" onClick={() => complete('a', 10)}>
          Finish old a
        </button>
        <button class="min-h-11 border px-3" onClick={() => complete('c', 0, true)}>
          Fail c
        </button>
      </div>
      <HistoryChart
        resourceType="agent"
        resourceId={target()}
        metric="cpu"
        label="CPU usage"
        unit="%"
        hideSelector
        range="1h"
      />
    </main>
  );
};
render(() => <Fixture />, document.getElementById('root')!);
