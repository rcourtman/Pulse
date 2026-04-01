import { fireEvent, render, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import densityMapSource from '@/components/shared/DensityMap.tsx?raw';
import densityMapModelSource from '@/components/shared/densityMapModel.ts?raw';
import densityMapStateSource from '@/components/shared/useDensityMapState.ts?raw';
import { DensityMap } from '@/components/shared/DensityMap';
import {
  buildDensityMapChartData,
  buildDensityMapFocusDetail,
  getDensityMapExternalSeriesIndex,
} from '@/components/shared/densityMapModel';

HTMLCanvasElement.prototype.getContext = vi.fn(() => ({
  clearRect: vi.fn(),
  setTransform: vi.fn(),
  scale: vi.fn(),
  beginPath: vi.fn(),
  roundRect: vi.fn(),
  save: vi.fn(),
  restore: vi.fn(),
  stroke: vi.fn(),
  strokeRect: vi.fn(),
  fill: vi.fn(),
  fillRect: vi.fn(),
  lineWidth: 1,
  strokeStyle: '',
  globalAlpha: 1,
  fillStyle: '',
})) as unknown as typeof HTMLCanvasElement.prototype.getContext;

describe('DensityMap', () => {
  it('keeps the density map on shell, runtime, and model owners', () => {
    expect(densityMapSource).toContain('useDensityMapState');
    expect(densityMapSource).not.toContain('timeRangeToMs');
    expect(densityMapSource).not.toContain('createSignal');
    expect(densityMapSource).not.toContain('ctx.fillRect');

    expect(densityMapStateSource).toContain('createSignal');
    expect(densityMapStateSource).toContain('canvas.getContext');
    expect(densityMapStateSource).toContain('window.addEventListener');
    expect(densityMapStateSource).toContain('export function useDensityMapState');

    expect(densityMapModelSource).toContain('buildDensityMapChartData');
    expect(densityMapModelSource).toContain('buildDensityMapFocusDetail');
    expect(densityMapModelSource).toContain('buildDensityMapHoveredState');
    expect(densityMapModelSource).toContain('getDensityMapExternalSeriesIndex');
    expect(densityMapModelSource).toContain('getDensityMapCellOpacity');
  });

  it('renders the time range labels and hover tooltip', async () => {
    const now = Date.now();
    const { container } = render(() => (
      <DensityMap
        timeRange="1h"
        series={[
          {
            id: 'cpu',
            name: 'CPU',
            color: '#10b981',
            data: [
              { timestamp: now - 30_000, value: 25 },
              { timestamp: now - 10_000, value: 55 },
            ],
          },
        ]}
      />
    ));

    expect(screen.getByText('-1h')).toBeInTheDocument();
    expect(screen.getByText('now')).toBeInTheDocument();

    const canvas = container.querySelector('canvas');
    expect(canvas).not.toBeNull();
    if (!canvas) return;

    (canvas as unknown as { getBoundingClientRect: () => DOMRect }).getBoundingClientRect = () =>
      ({
        left: 0,
        top: 0,
        width: 180,
        height: 80,
        right: 180,
        bottom: 80,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      }) as unknown as DOMRect;

    fireEvent.mouseMove(canvas, { clientX: 160, clientY: 20 });

    expect(await screen.findByText('CPU')).toBeInTheDocument();
    expect(screen.getByText('Current')).toBeInTheDocument();
    expect(screen.getByText('Peak')).toBeInTheDocument();
    expect(document.querySelector('[data-density-map-tooltip="true"]')).not.toBeNull();
    expect(document.querySelector('[data-density-map-tooltip-sparkline="true"]')).not.toBeNull();
  });

  it('publishes synchronized hover identity and clears it on leave', () => {
    const now = Date.now();
    const onHoverSyncChange = vi.fn();
    const { container } = render(() => (
      <DensityMap
        timeRange="1h"
        hoverSourceKey="diskio"
        onHoverSyncChange={onHoverSyncChange}
        series={[
          {
            id: 'alpha',
            name: 'Alpha',
            color: '#10b981',
            data: [
              { timestamp: now - 30_000, value: 25 },
              { timestamp: now - 10_000, value: 55 },
            ],
          },
        ]}
      />
    ));

    const canvas = container.querySelector('canvas');
    expect(canvas).not.toBeNull();
    if (!canvas) return;

    (canvas as unknown as { getBoundingClientRect: () => DOMRect }).getBoundingClientRect = () =>
      ({
        left: 0,
        top: 0,
        width: 180,
        height: 80,
        right: 180,
        bottom: 80,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      }) as unknown as DOMRect;

    fireEvent.mouseMove(canvas, { clientX: 160, clientY: 20 });

    const publishedHover = onHoverSyncChange.mock.calls.at(-1)?.[0];
    expect(publishedHover).toMatchObject({
      sourceKey: 'diskio',
      seriesId: 'alpha',
    });
    expect(typeof publishedHover?.timestamp).toBe('number');
    expect(publishedHover?.timestamp).toBeGreaterThanOrEqual(now - 3_600_000);
    expect(publishedHover?.timestamp).toBeLessThanOrEqual(now);

    fireEvent.mouseLeave(canvas);
    expect(onHoverSyncChange).toHaveBeenLastCalledWith(null);
  });

  it('keeps an externally highlighted series visible when it falls outside the default density top set', () => {
    const now = Date.now();
    const series = Array.from({ length: 24 }, (_, index) => ({
      id: `series-${index + 1}`,
      name: `Series ${index + 1}`,
      color: '#10b981',
      data:
        index === 23
          ? []
          : [
              { timestamp: now - 30_000, value: 100 - index },
              { timestamp: now - 10_000, value: 100 - index },
            ],
    }));

    const chartData = buildDensityMapChartData({
      series,
      timeRange: '1h',
      now,
      highlightSeriesId: 'series-24',
    });

    expect(chartData.series.map((entry) => entry.id)).toContain('series-24');
    expect(getDensityMapExternalSeriesIndex(chartData, 'series-24')).not.toBeNull();
  });

  it('builds focused detail from the active density-map series without replacing the overview model', () => {
    const now = Date.now();
    const chartData = buildDensityMapChartData({
      now,
      timeRange: '1h',
      highlightSeriesId: 'alpha',
      series: [
        {
          id: 'alpha',
          name: 'Alpha',
          color: '#10b981',
          data: [
            { timestamp: now - 40_000, value: 12 },
            { timestamp: now - 10_000, value: 32 },
          ],
        },
        {
          id: 'beta',
          name: 'Beta',
          color: '#3b82f6',
          data: [
            { timestamp: now - 30_000, value: 18 },
            { timestamp: now - 5_000, value: 24 },
          ],
        },
      ],
    });

    const detail = buildDensityMapFocusDetail({
      data: chartData,
      highlightSeriesId: 'alpha',
    });

    expect(chartData.series).toHaveLength(2);
    expect(detail).toMatchObject({
      seriesId: 'alpha',
      seriesName: 'Alpha',
      peakValue: 32,
    });
    expect(detail?.sparklinePath).toContain('M');
  });

  it('keeps the density-map card free of persistent focus chrome while preserving overview count', () => {
    const now = Date.now();
    const { container } = render(() => (
      <DensityMap
        timeRange="1h"
        highlightSeriesId="alpha"
        series={[
          {
            id: 'alpha',
            name: 'Alpha',
            color: '#10b981',
            data: [
              { timestamp: now - 40_000, value: 12 },
              { timestamp: now - 10_000, value: 32 },
            ],
          },
          {
            id: 'beta',
            name: 'Beta',
            color: '#3b82f6',
            data: [
              { timestamp: now - 30_000, value: 18 },
              { timestamp: now - 5_000, value: 24 },
            ],
          },
        ]}
      />
    ));

    const root = container.firstElementChild;
    expect(root?.getAttribute('data-summary-chart-kind')).toBe('density-map');
    expect(root?.getAttribute('data-active-hover-timestamp')).toBe('');
    expect(root?.getAttribute('data-rendered-series-count')).toBe('2');
    expect(screen.queryByText('Top activity overview')).toBeNull();
    expect(screen.queryByText('Peak')).toBeNull();
    expect(screen.queryByText('Alpha')).toBeNull();
    expect(container.querySelector('[data-density-map-focus-detail="true"]')).toBeNull();
  });

  it('keeps empty focused density maps free of persistent empty-state chrome', () => {
    const now = Date.now();
    const { container } = render(() => (
      <DensityMap
        timeRange="1h"
        highlightSeriesId="alpha"
        focusEmptyStateLabel="No disk activity in range"
        series={[
          {
            id: 'alpha',
            name: 'Alpha',
            color: '#10b981',
            data: [],
          },
          {
            id: 'beta',
            name: 'Beta',
            color: '#3b82f6',
            data: [
              { timestamp: now - 30_000, value: 18 },
              { timestamp: now - 5_000, value: 24 },
            ],
          },
        ]}
      />
    ));

    expect(container.querySelector('[data-density-map-focus-detail="true"]')).toBeNull();
    expect(screen.queryByText('No disk activity in range')).toBeNull();
    expect(screen.queryByText('Peak')).toBeNull();
  });

  it('shows empty-state peak copy in the tooltip when the hovered series has no activity in range', async () => {
    const now = Date.now();
    const { container } = render(() => (
      <DensityMap
        timeRange="1h"
        highlightSeriesId="alpha"
        focusEmptyStateLabel="No disk activity in range"
        series={[
          {
            id: 'alpha',
            name: 'Alpha',
            color: '#10b981',
            data: [],
          },
          {
            id: 'beta',
            name: 'Beta',
            color: '#3b82f6',
            data: [
              { timestamp: now - 30_000, value: 18 },
              { timestamp: now - 5_000, value: 24 },
            ],
          },
        ]}
      />
    ));

    const canvas = container.querySelector('canvas');
    expect(canvas).not.toBeNull();
    if (!canvas) return;

    (canvas as unknown as { getBoundingClientRect: () => DOMRect }).getBoundingClientRect = () =>
      ({
        left: 0,
        top: 0,
        width: 180,
        height: 80,
        right: 180,
        bottom: 80,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      }) as unknown as DOMRect;

    fireEvent.mouseMove(canvas, { clientX: 20, clientY: 60 });

    expect(await screen.findByText('Alpha')).toBeInTheDocument();
    expect(screen.getByText('Peak')).toBeInTheDocument();
    expect(screen.getByText('No disk activity in range')).toBeInTheDocument();
    expect(document.querySelector('[data-density-map-tooltip-sparkline="true"]')).toBeNull();
  });
});
