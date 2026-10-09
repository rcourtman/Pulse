import { afterEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import { ChartsAPI } from '@/api/charts';
import { eventBus } from '@/stores/events';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import historyChartHeaderSource from '@/components/shared/HistoryChartHeader.tsx?raw';
import historyChartHoverGroupSource from '@/components/shared/HistoryChartHoverGroup.tsx?raw';
import historyChartOverlaySource from '@/components/shared/HistoryChartOverlay.tsx?raw';
import historyChartSource from '@/components/shared/HistoryChart.tsx?raw';
import historyChartModelSource from '@/components/shared/historyChartModel.ts?raw';
import historyChartStateSource from '@/components/shared/useHistoryChartState.ts?raw';
import historyChartTooltipSource from '@/components/shared/HistoryChartTooltip.tsx?raw';
import { HistoryChartHeader } from '@/components/shared/HistoryChartHeader';
import type { HistoryChartState } from '@/components/shared/useHistoryChartState';
import { HistoryChart, HistoryChartHoverGroup } from '@/components/shared/HistoryChart';
import {
  formatHistoryChartTooltipValue,
  getHistoryChartTooltipLayout,
  HISTORY_CHART_RANGES,
} from '@/components/shared/historyChartModel';

it('distinguishes tiny positive percentage observations from zero in History labels', () => {
  expect(formatHistoryChartTooltipValue(0, '%')).toBe('0.0%');
  expect(formatHistoryChartTooltipValue(Number.MIN_VALUE, '%')).toBe('<0.1%');
  expect(formatHistoryChartTooltipValue(0.026, '%')).toBe('<0.1%');
  expect(formatHistoryChartTooltipValue(0.09999, '%')).toBe('<0.1%');
  expect(formatHistoryChartTooltipValue(0.1, '%')).toBe('0.1%');
  expect(formatHistoryChartTooltipValue(9.4, '%')).toBe('9.4%');
  expect(formatHistoryChartTooltipValue(0.026, 'C')).toBe('0°C');
});

function touchPointer(canvas: Element, type: string, fields: Partial<PointerEvent> = {}) {
  const event = Object.assign(new Event(type, { bubbles: true, cancelable: true }), {
    pointerId: 1,
    pointerType: 'touch',
    isPrimary: true,
    clientX: 60,
    clientY: 60,
    ...fields,
  });
  fireEvent(canvas, event);
  // Inspection must never consume native page panning or pinch zoom.
  expect(event.defaultPrevented).toBe(false);
}

function mountTouchHistory() {
  const [target, setTarget] = createSignal('disk-a');
  const [data, setData] = createSignal(
    [10, 20, 30].map((value, index) => ({
      timestamp: (index + 1) * 1000,
      value,
      min: value,
      max: value,
    })),
  );
  const view = render(() => (
    <HistoryChart
      resourceType="disk"
      resourceId={target()}
      metric="disk"
      unit="%"
      data={data()}
      hideSelector
    />
  ));
  const canvas = view.container.querySelector('canvas')!;
  vi.spyOn(canvas, 'getBoundingClientRect').mockReturnValue({
    x: 0,
    y: 0,
    left: 0,
    top: 0,
    right: 400,
    bottom: 200,
    width: 400,
    height: 200,
    toJSON: () => ({}),
  });
  return {
    ...view,
    canvas,
    setTarget,
    data,
    setData,
    tooltip: () => view.container.querySelector('[data-history-chart-tooltip]'),
    announcement: view.container.querySelector('[aria-live="polite"]')!,
  };
}

if (typeof globalThis.ResizeObserver === 'undefined') {
  globalThis.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}

HTMLCanvasElement.prototype.getContext = vi.fn(() => ({
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
  createLinearGradient: vi.fn(() => ({
    addColorStop: vi.fn(),
  })),
  measureText: vi.fn(() => ({ width: 40 })),
})) as unknown as typeof HTMLCanvasElement.prototype.getContext;

vi.mock('@/stores/license', () => ({
  isRangeLocked: () => false,
  loadRuntimeCapabilities: vi.fn(),
  maxHistoryDays: () => 30,
}));

vi.mock('@/api/charts', () => ({
  ChartsAPI: {
    getMetricsHistory: vi.fn().mockResolvedValue({ points: [], source: 'store' }),
  },
}));

describe('HistoryChart', () => {
  it('keeps the history chart on shell, runtime, and model owners', () => {
    expect(historyChartSource).toContain('useHistoryChartState');
    expect(historyChartSource).toContain('HistoryChartHeader');
    expect(historyChartSource).toContain('HistoryChartOverlay');
    expect(historyChartSource).toContain('HistoryChartTooltip');
    expect(historyChartSource).toContain('useHistoryChartHoverGroup');
    expect(historyChartOverlaySource).toContain(
      "import { LoadingSpinner } from './LoadingSpinner'",
    );
    expect(historyChartOverlaySource).toContain(
      '<LoadingSpinner size="xl" tone="info" label="Loading history" />',
    );
    expect(historyChartOverlaySource).not.toContain(
      'w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full animate-spin',
    );
    expect(historyChartSource).not.toContain('ChartsAPI.getMetricsHistory');
    expect(historyChartSource).not.toContain('calculateOptimalPoints');
    expect(historyChartSource).not.toContain('setupCanvasDPR');
    expect(historyChartSource).not.toContain('createSignal');
    expect(historyChartSource).not.toContain('Collecting data... History will appear here.');
    expect(historyChartSource).not.toContain('Unlock {chart.lockTierLabel()} Features');

    expect(historyChartStateSource).toContain('ChartsAPI.getMetricsHistory');
    expect(historyChartStateSource).toContain('calculateOptimalPoints');
    expect(historyChartStateSource).toContain('setupCanvasDPR');
    expect(historyChartStateSource).toContain('export function useHistoryChartState');
    expect(historyChartStateSource).toContain('HISTORY_CHART_RANGES');
    expect(historyChartStateSource).toContain('hoveredTimestamp');
    expect(historyChartStateSource).toContain("'mock_synthetic' | null");
    expect(historyChartStateSource).not.toContain('canStartCommercialTrial');
    expect(historyChartStateSource).not.toContain('runStartProTrialAction({');
    expect(historyChartStateSource).not.toContain('startProTrial()');
    expect(historyChartStateSource).not.toContain('getTrialAlreadyUsedMessage()');
    expect(historyChartStateSource).not.toContain('getTrialTryAgainLaterMessage()');

    expect(historyChartHoverGroupSource).toContain('createContext');
    expect(historyChartHoverGroupSource).toContain('HistoryChartHoverGroup');

    expect(historyChartModelSource).toContain('formatHistoryChartTooltipValue');
    expect(historyChartModelSource).toContain('getHistoryChartTooltipLayout');
    expect(historyChartModelSource).toContain('HISTORY_CHART_RANGES');
    expect(historyChartModelSource).toContain('getHistoryChartScale');
    expect(historyChartModelSource).toContain('findHistoryChartClosestPoint');

    expect(historyChartHeaderSource).toContain('formatHistoryChartTooltipValue');
    expect(historyChartHeaderSource).not.toContain('ChartsAPI.getMetricsHistory');
    expect(historyChartHeaderSource).not.toContain('setupCanvasDPR');

    expect(historyChartOverlaySource).toContain('No history samples in this time range.');
    expect(historyChartOverlaySource).not.toContain('History will appear here.');
    expect(historyChartOverlaySource).toContain(
      'Historical data beyond {props.chart.lockDays()} days requires a higher license plan.',
    );
    expect(historyChartOverlaySource).not.toContain(
      'Unlock {props.chart.lockTierLabel()} Features',
    );
    expect(historyChartOverlaySource).not.toContain('presentationPolicyHidesUpgradePrompts');
    expect(historyChartOverlaySource).not.toContain('free 14-day trial');
    expect(historyChartOverlaySource).toContain('requires a higher license plan');
    expect(historyChartOverlaySource).not.toContain('ChartsAPI.getMetricsHistory');
    expect(historyChartOverlaySource).not.toContain('setupCanvasDPR');

    expect(historyChartTooltipSource).toContain('formatHistoryChartTooltipValue');
    expect(historyChartTooltipSource).toContain('getHistoryChartTooltipLayout');
    expect(historyChartTooltipSource).toContain('foreignObject');
    expect(historyChartTooltipSource).toContain('width={props.chartWidth}');
    expect(historyChartTooltipSource).toContain('height={props.chartHeight}');
    expect(historyChartTooltipSource).toContain('new Date(point().timestamp).toLocaleString()');
    expect(historyChartTooltipSource).not.toContain('<Portal>');
    expect(historyChartTooltipSource).not.toContain('absolute inset-0 h-full w-full');
    expect(historyChartTooltipSource).not.toContain('preserveAspectRatio="none"');
    expect(historyChartTooltipSource).not.toContain('style={');
    expect(historyChartTooltipSource).not.toContain('ChartsAPI.getMetricsHistory');
  });

  it('inspects actual readings with the keyboard and clears on escape, blur and selection changes', () => {
    const [target, setTarget] = createSignal('a');
    const [points, setPoints] = createSignal([
      { timestamp: 1_000, value: 10, min: 10, max: 10 },
      { timestamp: 2_000, value: 20, min: 20, max: 20 },
      { timestamp: 3_000, value: 30, min: 30, max: 30 },
    ]);
    const { container } = render(() => (
      <HistoryChart
        resourceType="disk"
        resourceId={target()}
        metric="usage"
        unit="%"
        hideSelector
        data={points()}
      />
    ));
    const canvas = screen.getByRole('img', { name: 'History chart' });
    const announcement = container.querySelector('[aria-live="polite"]')!;
    expect(canvas).toHaveAttribute('tabindex', '0');
    expect(announcement.textContent).toBe('');
    fireEvent.focus(canvas);
    expect(announcement).toHaveTextContent('30.0%');
    fireEvent.keyDown(canvas, { key: 'ArrowLeft' });
    expect(announcement).toHaveTextContent('20.0%');
    fireEvent.keyDown(canvas, { key: 'Home' });
    fireEvent.keyDown(canvas, { key: 'ArrowLeft' });
    expect(announcement).toHaveTextContent('10.0%');
    fireEvent.keyDown(canvas, { key: 'ArrowRight', ctrlKey: true });
    expect(announcement).toHaveTextContent('10.0%');
    fireEvent.keyDown(canvas, { key: 'End' });
    fireEvent.keyDown(canvas, { key: 'ArrowRight' });
    expect(announcement).toHaveTextContent('30.0%');
    fireEvent.keyDown(canvas, { key: 'Escape' });
    expect(announcement.textContent).toBe('');
    expect(container.querySelector('[data-history-chart-tooltip]')).toBeNull();
    fireEvent.keyDown(canvas, { key: 'Home' });
    setPoints(points().map((point) => ({ ...point, value: point.value + 1 })));
    expect(announcement).toHaveTextContent('11.0%');
    setTarget('b');
    expect(announcement.textContent).toBe('');
    fireEvent.keyDown(canvas, { key: 'Home' });
    fireEvent.blur(canvas);
    expect(announcement.textContent).toBe('');
    setPoints([]);
    fireEvent.focus(canvas);
    fireEvent.keyDown(canvas, { key: 'End' });
    expect(announcement.textContent).toBe('');
  });

  it('inspects the tapped sample without compatibility focus selecting the latest reading', () => {
    const view = mountTouchHistory();
    touchPointer(view.canvas, 'pointerdown');
    touchPointer(view.canvas, 'pointerup');
    fireEvent.focus(view.canvas);
    expect(view.tooltip()).toHaveTextContent('10.0%');
    expect(view.announcement.textContent).toBe('');
    fireEvent.mouseMove(view.canvas, { clientX: 370 });
    fireEvent.mouseLeave(view.canvas);
    expect(view.tooltip()).toHaveTextContent('10.0%');
    touchPointer(view.canvas, 'pointerdown', { clientX: 200 });
    touchPointer(view.canvas, 'pointerup', { clientX: 200 });
    expect(view.tooltip()).toHaveTextContent('20.0%');
    fireEvent.keyDown(view.canvas, { key: 'ArrowRight' });
    expect(view.announcement).toHaveTextContent('30.0%');
    fireEvent.keyDown(view.canvas, { key: 'Escape' });
    expect(view.tooltip()).toBeNull();
    fireEvent.blur(view.canvas);
    fireEvent.focus(view.canvas);
    expect(view.announcement).toHaveTextContent('30.0%');
  });

  it.each(['moved', 'release-displaced', 'cancelled', 'second-finger', 'different-pointer'])(
    'does not turn a %s touch into inspection',
    (gesture) => {
      const view = mountTouchHistory();
      touchPointer(view.canvas, 'pointerdown');
      if (gesture === 'moved') {
        touchPointer(view.canvas, 'pointermove', { clientY: 90 });
        // Returning to the origin is still a drag, not a new tap.
        touchPointer(view.canvas, 'pointermove');
      } else if (gesture === 'cancelled') {
        touchPointer(view.canvas, 'pointercancel');
      } else if (gesture === 'second-finger') {
        touchPointer(view.canvas, 'pointerdown', { pointerId: 2, isPrimary: false });
      }
      touchPointer(view.canvas, 'pointerup', {
        ...(gesture === 'release-displaced' ? { clientY: 90 } : {}),
        ...(gesture === 'different-pointer' ? { pointerId: 2 } : {}),
      });
      fireEvent.focus(view.canvas);
      expect(view.tooltip()).toBeNull();
      expect(view.announcement.textContent).toBe('');
    },
  );

  it.each([{ clientX: 0 }, { clientX: 400 }, { clientY: -1 }, { clientY: 201 }])(
    'does not inspect a tap outside the plot: %j',
    (position) => {
      const view = mountTouchHistory();
      touchPointer(view.canvas, 'pointerdown', position);
      touchPointer(view.canvas, 'pointerup', position);
      expect(view.tooltip()).toBeNull();
    },
  );

  it('invalidates an in-flight touch when the resource changes', () => {
    const view = mountTouchHistory();
    touchPointer(view.canvas, 'pointerdown');
    view.setTarget('disk-b');
    touchPointer(view.canvas, 'pointerup');
    fireEvent.focus(view.canvas);
    expect(view.tooltip()).toBeNull();
    expect(view.announcement.textContent).toBe('');
    touchPointer(view.canvas, 'pointerdown', { clientX: 200 });
    touchPointer(view.canvas, 'pointerup', { clientX: 200 });
    expect(view.tooltip()).toHaveTextContent('20.0%');
  });

  it('keeps the tapped time on refresh and distinguishes a lone zero from empty history', () => {
    const view = mountTouchHistory();
    touchPointer(view.canvas, 'pointerdown');
    touchPointer(view.canvas, 'pointerup');
    view.setData(view.data().map((point) => ({ ...point, value: point.value + 1 })));
    expect(view.tooltip()).toHaveTextContent('11.0%');
    view.setData([{ timestamp: 1000, value: 0, min: 0, max: 0 }]);
    touchPointer(view.canvas, 'pointerdown', { clientX: 200 });
    touchPointer(view.canvas, 'pointerup', { clientX: 200 });
    expect(view.tooltip()).toHaveTextContent('0.0%');
    view.setData([]);
    touchPointer(view.canvas, 'pointerdown');
    touchPointer(view.canvas, 'pointerup');
    fireEvent.focus(view.canvas);
    expect(view.tooltip()).toBeNull();
    expect(view.announcement.textContent).toBe('');
  });

  it('leaves mouse hover available after touch inspection', () => {
    const view = mountTouchHistory();
    touchPointer(view.canvas, 'pointerdown');
    touchPointer(view.canvas, 'pointerup');
    touchPointer(view.canvas, 'pointermove', { pointerType: 'mouse', clientX: 200 });
    fireEvent.mouseMove(view.canvas, { clientX: 200 });
    expect(view.tooltip()).toHaveTextContent('20.0%');
    fireEvent.mouseLeave(view.canvas);
    expect(view.tooltip()).toBeNull();
  });

  it('keeps a new group chart tap when the previously focused chart blurs', () => {
    const points = [10, 20, 30].map((value, index) => ({
      timestamp: (index + 1) * 1000,
      value,
      min: value,
      max: value,
    }));
    const { container } = render(() => (
      <HistoryChartHoverGroup>
        <HistoryChart resourceType="disk" resourceId="a" metric="disk" unit="%" data={points} />
        <HistoryChart resourceType="disk" resourceId="a" metric="usage" unit="%" data={points} />
      </HistoryChartHoverGroup>
    ));
    const [previous, next] = container.querySelectorAll('canvas');
    vi.spyOn(next, 'getBoundingClientRect').mockReturnValue({
      x: 0,
      y: 0,
      left: 0,
      top: 0,
      right: 400,
      bottom: 200,
      width: 400,
      height: 200,
      toJSON: () => ({}),
    });
    fireEvent.focus(previous);
    touchPointer(next, 'pointerdown');
    touchPointer(next, 'pointerup');
    fireEvent.blur(previous);
    fireEvent.focus(next);
    const tooltips = container.querySelectorAll('[data-history-chart-tooltip]');
    expect(tooltips).toHaveLength(2);
    for (const tooltip of tooltips) expect(tooltip).toHaveTextContent('10.0%');
    for (const live of container.querySelectorAll('[aria-live="polite"]')) {
      expect(live.textContent).toBe('');
    }
  });

  it('renders the default history label', () => {
    render(() => <HistoryChart resourceType="agent" resourceId="node-1" metric="cpu" />);

    expect(screen.getByText('History')).toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'History chart' })).toHaveAttribute('aria-describedby');
  });

  it('provides a text equivalent for a populated history chart', () => {
    render(() => (
      <HistoryChart
        resourceType="agent"
        resourceId="node-1"
        metric="cpu"
        label="CPU usage"
        unit="%"
        range="24h"
        data={[
          { timestamp: 1_000, value: 10, min: 8, max: 12 },
          { timestamp: 2_000, value: 30, min: 25, max: 35 },
        ]}
      />
    ));

    const chart = screen.getByRole('img', { name: 'CPU usage chart' });
    const description = document.getElementById(chart.getAttribute('aria-describedby')!);

    expect(description).toHaveClass('sr-only');
    expect(description).toHaveTextContent('24-hour history contains 2 data points');
    expect(description).toHaveTextContent('Values increased from 10.0% to 30.0%.');
    expect(description).toHaveTextContent('Minimum 8.0%. Maximum 35.0%.');
  });

  it('removes previous-target values from the accessible chart while the next target loads', async () => {
    const request = vi.mocked(ChartsAPI.getMetricsHistory);
    request.mockResolvedValueOnce({
      points: [{ timestamp: 1000, value: 10, min: 10, max: 10 }],
      source: 'store',
    } as never);
    const [target, setTarget] = createSignal('a');
    render(() => <HistoryChart resourceType="agent" resourceId={target()} metric="cpu" unit="%" />);
    const chart = screen.getByRole('img', { name: 'History chart' });
    const description = document.getElementById(chart.getAttribute('aria-describedby')!)!;
    await waitFor(() => expect(description).toHaveTextContent('10.0%'));
    request.mockImplementationOnce(() => new Promise(() => {}));
    setTarget('b');
    expect(description).toHaveTextContent('Loading');
    expect(description).not.toHaveTextContent('10.0%');
    expect(screen.queryByText('Min')).not.toBeInTheDocument();
  });

  it('exposes a stored singleton to pointer inspection without treating a later empty response as zero', async () => {
    const request = vi.mocked(ChartsAPI.getMetricsHistory);
    request.mockResolvedValueOnce({
      points: [{ timestamp: 1000, value: 42, min: 42, max: 42 }],
      source: 'store',
    } as never);
    const [target, setTarget] = createSignal('pool-a');
    const { container } = render(() => (
      <HistoryChart
        resourceType="storage"
        resourceId={target()}
        metric="usage"
        unit="%"
        range="1h"
      />
    ));
    const chart = screen.getByRole('img', { name: 'History chart' });
    const description = document.getElementById(chart.getAttribute('aria-describedby')!)!;
    const rectSpy = vi.spyOn(chart, 'getBoundingClientRect').mockReturnValue({
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
    await waitFor(() => expect(description).toHaveTextContent('1 data point'));
    fireEvent.mouseMove(chart, { clientX: 175 });
    expect(container.querySelector('[data-history-chart-tooltip]')).toHaveTextContent('42.0%');
    request.mockResolvedValueOnce({ points: [], source: 'store' } as never);
    setTarget('pool-b');
    await waitFor(() => expect(description).toHaveTextContent('No 1-hour history'));
    expect(description).not.toHaveTextContent('0.0%');
    expect(container.querySelector('[data-history-chart-tooltip]')).toBeNull();
    rectSpy.mockRestore();
  });

  it('synchronizes the hovered timestamp across charts in the same group', () => {
    const rectSpy = vi.spyOn(HTMLCanvasElement.prototype, 'getBoundingClientRect').mockReturnValue({
      x: 0,
      y: 0,
      left: 0,
      top: 0,
      right: 400,
      bottom: 120,
      width: 400,
      height: 120,
      toJSON: () => ({}),
    });
    const data = [
      { timestamp: 1_000, value: 10, min: 10, max: 10 },
      { timestamp: 2_000, value: 20, min: 20, max: 20 },
      { timestamp: 3_000, value: 30, min: 30, max: 30 },
    ];

    const { container } = render(() => (
      <HistoryChartHoverGroup>
        <HistoryChart
          resourceType="disk"
          resourceId="disk-1"
          metric="diskread"
          unit="B/s"
          data={data}
        />
        <HistoryChart
          resourceType="disk"
          resourceId="disk-1"
          metric="diskwrite"
          unit="B/s"
          data={data.map((point) => ({
            ...point,
            value: point.value * 2,
            min: point.min * 2,
            max: point.max * 2,
          }))}
        />
      </HistoryChartHoverGroup>
    ));

    const canvases = container.querySelectorAll('canvas');
    fireEvent.mouseMove(canvases[0], { clientX: 220 });

    expect(container.querySelectorAll('[data-history-chart-tooltip="true"]')).toHaveLength(2);

    fireEvent.mouseLeave(canvases[0]);

    expect(container.querySelectorAll('[data-history-chart-tooltip="true"]')).toHaveLength(0);
    rectSpy.mockRestore();
  });

  it('keeps pointer inspection when matching supplied samples refresh', () => {
    const rectSpy = vi.spyOn(HTMLCanvasElement.prototype, 'getBoundingClientRect').mockReturnValue({
      x: 0,
      y: 0,
      left: 0,
      top: 0,
      right: 400,
      bottom: 120,
      width: 400,
      height: 120,
      toJSON: () => ({}),
    });
    const samples = (value: number) =>
      [1000, 2000, 3000].map((timestamp) => ({ timestamp, value, min: value, max: value }));
    const [data, setData] = createSignal(samples(10));
    const { container } = render(() => (
      <HistoryChart resourceType="agent" resourceId="a" metric="cpu" data={data()} />
    ));
    fireEvent.mouseMove(container.querySelector('canvas')!, { clientX: 220 });
    expect(container.querySelector('[data-history-chart-tooltip="true"]')).not.toBeNull();
    setData(samples(20));
    expect(container.querySelector('[data-history-chart-tooltip="true"]')).not.toBeNull();
    expect(container.querySelector('[data-history-chart-tooltip="true"]')).toHaveTextContent('20');
    rectSpy.mockRestore();
  });

  it('exposes the sub-day and Relay history ranges as first-class chart options', () => {
    expect(HISTORY_CHART_RANGES).toEqual(['1h', '6h', '12h', '24h', '7d', '14d', '30d', '90d']);
  });

  it('positions the tooltip beside the hovered point when there is chart space', () => {
    const layout = getHistoryChartTooltipLayout({
      hoveredPoint: { x: 150, y: 70, timestamp: 0, value: 42 },
      chartWidth: 420,
      chartHeight: 180,
    });

    expect(layout.x).toBe(162);
    expect(layout.x).toBeGreaterThan(150);
    expect(layout.height).toBe(64);
    expect(layout.y + layout.height / 2).toBe(70);
  });

  it('moves the tooltip to the left edge side near the right chart boundary', () => {
    const layout = getHistoryChartTooltipLayout({
      hoveredPoint: { x: 380, y: 70, timestamp: 0, value: 42 },
      chartWidth: 420,
      chartHeight: 180,
    });

    expect(layout.x + layout.width).toBeLessThan(380);
    expect(layout.x).toBe(212);
  });
});

describe('History organisation boundary rendering', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.useRealTimers();
  });
  it.each(['keyboard', 'touch'])(
    'clears %s inspection and accessible values on org switch until a new read succeeds',
    async (input) => {
      vi.useFakeTimers();
      const request = vi.mocked(ChartsAPI.getMetricsHistory);
      request.mockReset();
      request.mockResolvedValueOnce({
        points: [{ timestamp: 1000, value: 42, min: 42, max: 42 }],
        source: 'memory',
      } as never);
      let complete!: (response: Awaited<ReturnType<typeof ChartsAPI.getMetricsHistory>>) => void;
      request.mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            complete = resolve;
          }),
      );
      const { container } = render(() => (
        <HistoryChart
          resourceType="disk"
          resourceId="disk:nas:sda"
          metric="smart_temp"
          label="Temperature"
          unit="C"
          range="1h"
        />
      ));
      await vi.advanceTimersByTimeAsync(0);
      const canvas = screen.getByRole('img', { name: 'Temperature chart' });
      vi.spyOn(canvas, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        left: 0,
        top: 0,
        right: 400,
        bottom: 200,
        width: 400,
        height: 200,
        toJSON: () => ({}),
      });
      if (input === 'keyboard') fireEvent.focus(canvas);
      else {
        touchPointer(canvas, 'pointerdown');
        touchPointer(canvas, 'pointerup');
      }
      expect(container.querySelector('[data-history-chart-tooltip]')).toHaveTextContent('42°C');
      eventBus.emit('org_switched', 'org-b');
      expect(container.querySelector('[data-history-chart-tooltip]')).toBeNull();
      expect(container.querySelector('[aria-live="polite"]')?.textContent).toBe('');
      expect(container).not.toHaveTextContent('42°C');
      expect(container).not.toHaveTextContent('Buffer');
      expect(canvas).toHaveAccessibleDescription(/Loading 1-hour history data/);
      complete({
        points: [{ timestamp: 1000, value: 80, min: 80, max: 80 }],
        source: 'store',
      } as never);
      await vi.advanceTimersByTimeAsync(0);
      expect(canvas).toHaveAccessibleDescription(/80°C/);
      expect(container.querySelector('[data-history-chart-tooltip]')).toBeNull();
      if (input === 'touch') {
        // Touch compatibility focus from the old chart must not reactivate its pinned reading.
        fireEvent.focus(canvas);
        expect(container.querySelector('[data-history-chart-tooltip]')).toBeNull();
        touchPointer(canvas, 'pointerdown');
        touchPointer(canvas, 'pointerup');
        expect(container.querySelector('[data-history-chart-tooltip]')).toHaveTextContent('80°C');
      }
    },
  );
});

describe('History access boundary rendering', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.useRealTimers();
  });
  it.each([401, 403])(
    'withdraws painted readings, tooltips and keyboard announcements on %s',
    async (status) => {
      vi.useFakeTimers();
      vi.spyOn(console, 'error').mockImplementation(() => {});
      const request = vi.mocked(ChartsAPI.getMetricsHistory);
      request.mockReset();
      request.mockResolvedValueOnce({
        points: [{ timestamp: 1000, value: 42, min: 42, max: 42 }],
        source: 'store',
      } as never);
      const { container } = render(() => (
        <HistoryChart
          resourceType="disk"
          resourceId="disk:nas:sda"
          metric="smart_temp"
          label="Temperature"
          unit="C"
          range="1h"
        />
      ));
      await vi.advanceTimersByTimeAsync(0);
      const canvas = screen.getByRole('img', { name: 'Temperature chart' });
      fireEvent.focus(canvas);
      expect(container.querySelector('[data-history-chart-tooltip]')).toHaveTextContent('42°C');
      request.mockRejectedValueOnce(Object.assign(new Error('private detail'), { status }));
      await vi.advanceTimersByTimeAsync(10_000);
      expect(container.querySelector('[data-history-chart-tooltip]')).toBeNull();
      expect(container.querySelector('[aria-live="polite"]')).toHaveTextContent('');
      expect(container).not.toHaveTextContent('42°C');
      expect(container).not.toHaveTextContent('last successful');
      expect(container).not.toHaveTextContent('private detail');
      expect(screen.getByRole('alert')).toHaveTextContent(
        status === 401 ? 'Sign in again' : 'Access denied',
      );
      expect(screen.queryByText('No history samples in this time range.')).not.toBeInTheDocument();
      fireEvent.keyDown(canvas, { key: 'End' });
      expect(container.querySelector('[data-history-chart-tooltip]')).toBeNull();
    },
  );
});

describe('History window controls', () => {
  afterEach(cleanup);

  const makeChart = () => {
    const [range, setRange] = createSignal<
      '1h' | '6h' | '12h' | '24h' | '7d' | '14d' | '30d' | '90d'
    >('1h');
    const updateRange = vi.fn(setRange);
    const chart = {
      ranges: HISTORY_CHART_RANGES,
      range,
      updateRange,
      dataMin: () => 0,
      dataMax: () => 75,
      source: () => 'store',
    } as unknown as HistoryChartState;
    return { chart, updateRange };
  };

  it('names each chart window and announces only its selected period', () => {
    const cpu = makeChart();
    const memory = makeChart();
    render(() => (
      <>
        <HistoryChartHeader chart={cpu.chart} label="CPU" unit="%" />
        <HistoryChartHeader chart={memory.chart} label="Memory" unit="%" />
      </>
    ));
    const cpuGroup = within(screen.getByRole('group', { name: 'CPU history window' }));
    const memoryGroup = within(screen.getByRole('group', { name: 'Memory history window' }));
    expect(cpuGroup.getAllByRole('button', { pressed: true })).toHaveLength(1);
    expect(cpuGroup.getByRole('button', { name: '1h' })).toHaveAttribute('aria-pressed', 'true');
    fireEvent.click(cpuGroup.getByRole('button', { name: '24h' }));
    expect(cpu.updateRange).toHaveBeenCalledExactlyOnceWith('24h');
    expect(cpuGroup.getByRole('button', { name: '24h' })).toHaveAttribute('aria-pressed', 'true');
    expect(cpuGroup.getByRole('button', { name: '1h' })).toHaveAttribute('aria-pressed', 'false');
    expect(cpuGroup.getAllByRole('button', { pressed: true })).toHaveLength(1);
    expect(memoryGroup.getByRole('button', { name: '1h' })).toHaveAttribute('aria-pressed', 'true');
    expect(memory.updateRange).not.toHaveBeenCalled();
  });

  it('keeps range changes out of surrounding form submission', () => {
    const { chart } = makeChart();
    const onSubmit = vi.fn((event: SubmitEvent) => event.preventDefault());
    render(() => (
      <form onSubmit={onSubmit}>
        <HistoryChartHeader chart={chart} />
      </form>
    ));
    const group = within(screen.getByRole('group', { name: 'History window' }));
    for (const button of group.getAllByRole('button'))
      expect(button).toHaveAttribute('type', 'button');
    fireEvent.click(group.getByRole('button', { name: '7d' }));
    expect(group.getByRole('button', { name: '7d' })).toHaveAttribute('aria-pressed', 'true');
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('does not expose a phantom window group when the caller hides it', () => {
    const { chart } = makeChart();
    render(() => <HistoryChartHeader chart={chart} label="Disk I/O" hideSelector />);
    expect(screen.queryByRole('group')).not.toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
    expect(screen.getByText('Disk I/O')).toBeInTheDocument();
  });
});
