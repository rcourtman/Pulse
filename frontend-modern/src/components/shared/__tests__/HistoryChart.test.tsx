import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@solidjs/testing-library';
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

    expect(historyChartOverlaySource).toContain('Collecting data... History will appear here.');
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

  it('renders the default history label', () => {
    render(() => <HistoryChart resourceType="agent" resourceId="node-1" metric="cpu" />);

    expect(screen.getByText('History')).toBeInTheDocument();
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
    expect(layout.y).toBe(47);
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
