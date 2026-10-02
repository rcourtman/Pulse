import { describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import { ChartsAPI } from '@/api/charts';
import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import historyChartHeaderSource from '@/components/shared/HistoryChartHeader.tsx?raw';
import historyChartHoverGroupSource from '@/components/shared/HistoryChartHoverGroup.tsx?raw';
import historyChartOverlaySource from '@/components/shared/HistoryChartOverlay.tsx?raw';
import historyChartSource from '@/components/shared/HistoryChart.tsx?raw';
import historyChartModelSource from '@/components/shared/historyChartModel.ts?raw';
import historyChartStateSource from '@/components/shared/useHistoryChartState.ts?raw';
import historyChartTooltipSource from '@/components/shared/HistoryChartTooltip.tsx?raw';
import { HistoryChart, HistoryChartHoverGroup } from '@/components/shared/HistoryChart';
import {
  getHistoryChartTooltipLayout,
  HISTORY_CHART_RANGES,
} from '@/components/shared/historyChartModel';

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
