import { fireEvent, render, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import densityMapSource from '@/components/shared/DensityMap.tsx?raw';
import densityMapModelSource from '@/components/shared/densityMapModel.ts?raw';
import densityMapStateSource from '@/components/shared/useDensityMapState.ts?raw';
import { DensityMap } from '@/components/shared/DensityMap';

HTMLCanvasElement.prototype.getContext = vi.fn(() => ({
  clearRect: vi.fn(),
  setTransform: vi.fn(),
  scale: vi.fn(),
  beginPath: vi.fn(),
  roundRect: vi.fn(),
  fill: vi.fn(),
  fillRect: vi.fn(),
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
    expect(densityMapModelSource).toContain('buildDensityMapHoveredState');
    expect(densityMapModelSource).toContain('getDensityMapCellOpacity');
  });

  it('renders the time range labels and hover tooltip', async () => {
    const now = Date.now();
    const { container } = render(() => (
      <DensityMap
        timeRange="1h"
        series={[
          {
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
  });
});
