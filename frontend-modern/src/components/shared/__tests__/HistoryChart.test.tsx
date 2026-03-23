import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@solidjs/testing-library';
import historyChartSource from '@/components/shared/HistoryChart.tsx?raw';
import historyChartModelSource from '@/components/shared/historyChartModel.ts?raw';
import historyChartStateSource from '@/components/shared/useHistoryChartState.ts?raw';
import { HistoryChart } from '@/components/shared/HistoryChart';

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
})) as unknown as typeof HTMLCanvasElement.prototype.getContext;

vi.mock('@/stores/license', () => ({
  getUpgradeActionUrlOrFallback: () => 'https://example.com/upgrade',
  isRangeLocked: () => false,
  licenseStatus: () => ({ subscription_state: 'active' }),
  loadLicenseStatus: vi.fn(),
  maxHistoryDays: () => 30,
  startProTrial: vi.fn(),
}));

vi.mock('@/api/charts', () => ({
  ChartsAPI: {
    getMetricsHistory: vi.fn().mockResolvedValue({ points: [], source: 'store' }),
  },
}));

describe('HistoryChart', () => {
  it('keeps the history chart on shell, runtime, and model owners', () => {
    expect(historyChartSource).toContain('useHistoryChartState');
    expect(historyChartSource).not.toContain('ChartsAPI.getMetricsHistory');
    expect(historyChartSource).not.toContain('calculateOptimalPoints');
    expect(historyChartSource).not.toContain('setupCanvasDPR');
    expect(historyChartSource).not.toContain('createSignal');

    expect(historyChartStateSource).toContain('ChartsAPI.getMetricsHistory');
    expect(historyChartStateSource).toContain('calculateOptimalPoints');
    expect(historyChartStateSource).toContain('setupCanvasDPR');
    expect(historyChartStateSource).toContain('export function useHistoryChartState');

    expect(historyChartModelSource).toContain('formatHistoryChartTooltipValue');
    expect(historyChartModelSource).toContain('getHistoryChartScale');
    expect(historyChartModelSource).toContain('findHistoryChartClosestPoint');
  });

  it('renders the default history label', () => {
    render(() => (
      <HistoryChart resourceType="node" resourceId="node-1" metric="cpu" />
    ));

    expect(screen.getByText('History')).toBeInTheDocument();
  });
});
