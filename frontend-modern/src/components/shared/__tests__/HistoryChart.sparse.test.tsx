import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import type { AggregatedMetricPoint } from '@/api/charts';
import { HistoryChart } from '../HistoryChart';
import { formatHistoryChartTimeLabel } from '../historyChartModel';

vi.mock('@/stores/license', () => ({
  isRangeLocked: () => false,
  loadRuntimeCapabilities: vi.fn(),
  maxHistoryDays: () => 7,
}));
vi.mock('@/api/charts', () => ({ ChartsAPI: { getMetricsHistory: vi.fn() } }));

const ctx = {
  clearRect: vi.fn(),
  setTransform: vi.fn(),
  beginPath: vi.fn(),
  moveTo: vi.fn(),
  lineTo: vi.fn(),
  stroke: vi.fn(),
  fillText: vi.fn(),
  closePath: vi.fn(),
  fill: vi.fn(),
  arc: vi.fn(),
  save: vi.fn(),
  restore: vi.fn(),
  setLineDash: vi.fn(),
  measureText: vi.fn(() => ({ width: 20 })),
};
const point = (value: number, timestamp = 1000): AggregatedMetricPoint => ({
  timestamp,
  value,
  min: value,
  max: value,
});

function mount(initial: AggregatedMetricPoint[]) {
  const [data, setData] = createSignal(initial);
  const view = render(() => (
    <HistoryChart
      resourceType="storage"
      resourceId="pool"
      metric="usage"
      unit="%"
      range="1h"
      data={data()}
    />
  ));
  const canvas = screen.getByRole('img', { name: 'History chart' });
  vi.spyOn(canvas, 'getBoundingClientRect').mockReturnValue({
    x: 0,
    y: 0,
    left: 0,
    top: 0,
    right: 300,
    bottom: 200,
    width: 300,
    height: 200,
    toJSON: () => ({}),
  });
  return { ...view, canvas, setData };
}

describe('Sparse History observations', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(
      ctx as unknown as CanvasRenderingContext2D,
    );
    vi.stubGlobal(
      'ResizeObserver',
      class {
        observe() {}
        disconnect() {}
      },
    );
  });
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it.each([0, 42])(
    'draws a %s singleton marker and exactly one timestamp before hover',
    (value) => {
      mount([point(value)]);
      const lastClear = ctx.clearRect.mock.invocationCallOrder.at(-1)!;
      const arcs = ctx.arc.mock.calls.filter(
        (_, i) => ctx.arc.mock.invocationCallOrder[i] > lastClear,
      );
      expect(arcs).toHaveLength(1);
      expect(arcs[0][0]).toBe(164);
      expect(arcs[0][1]).toBeCloseTo(180 - (value / 100) * 160);
      expect(arcs[0][2]).toBe(4);
      const times = ctx.fillText.mock.calls.filter(
        (args, i) =>
          ctx.fillText.mock.invocationCallOrder[i] > lastClear &&
          args[0] === formatHistoryChartTimeLabel(1000, '1h'),
      );
      expect(times).toHaveLength(1);
      expect(times[0][1]).toBe(164);
    },
  );

  it('inspects the actual singleton from any pointer position and clears outside the plot', () => {
    const { canvas, container } = mount([point(42)]);
    for (const clientX of [100, 175, 250]) {
      fireEvent.mouseMove(canvas, { clientX });
      const tooltip = container.querySelector('[data-history-chart-tooltip]')!;
      expect(tooltip).toHaveTextContent('42.0%');
      expect(tooltip).toHaveTextContent(new Date(1000).toLocaleString());
      expect(container.querySelector('[aria-live="polite"]')).toHaveTextContent('');
    }
    fireEvent.mouseMove(canvas, { clientX: 5 });
    expect(container.querySelector('[data-history-chart-tooltip]')).toBeNull();
  });

  it('keeps zero, multiple and empty refreshes truthful during inspection', () => {
    const { canvas, container, setData } = mount([point(42)]);
    fireEvent.focus(canvas);
    for (const key of ['Home', 'End', 'ArrowLeft', 'ArrowRight']) {
      fireEvent.keyDown(canvas, { key });
      expect(container.querySelector('[aria-live="polite"]')).toHaveTextContent('42.0%');
    }
    setData([point(0)]);
    expect(container.querySelector('[data-history-chart-tooltip]')).toHaveTextContent('0.0%');
    setData([point(10), point(20, 2000), point(30, 3000)]);
    fireEvent.keyDown(canvas, { key: 'ArrowRight' });
    expect(container.querySelector('[data-history-chart-tooltip]')).toHaveTextContent('20.0%');
    setData([]);
    expect(container.querySelector('[data-history-chart-tooltip]')).toBeNull();
    expect(container.querySelector('[aria-live="polite"]')).toHaveTextContent('');
    expect(screen.getByText('No history samples in this time range.')).toBeInTheDocument();
  });
});
