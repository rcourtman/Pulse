import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { InteractiveSparkline } from '@/components/shared/InteractiveSparkline';

describe('InteractiveSparkline hover behavior', () => {
  afterEach(() => {
    vi.useRealTimers();
    cleanup();
  });

  it('shows a vertical dashed hover line and a tooltip', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2024-01-01T12:00:00Z'));
    const now = Date.now();

    const { container } = render(() => (
      <InteractiveSparkline
        timeRange="1h"
        rangeLabel="1h"
        series={[
          {
            name: 'CPU',
            color: '#ff0000',
            data: [
              { timestamp: now - 30_000, value: 40 },
              { timestamp: now - 10_000, value: 50 },
            ],
          },
        ]}
      />
    ));

    const svg = container.querySelector('svg');
    expect(svg).not.toBeNull();
    if (!svg) return;

    // JSDOM returns zeros by default, but the hover math requires a non-zero width.
    (svg as unknown as { getBoundingClientRect: () => DOMRect }).getBoundingClientRect = () =>
      ({
        left: 0,
        top: 100,
        width: 200,
        height: 50,
        right: 200,
        bottom: 150,
        x: 0,
        y: 100,
        toJSON: () => ({}),
      }) as unknown as DOMRect;

    fireEvent.mouseMove(svg, { clientX: 199, clientY: 110 });

    expect(svg.querySelector('line[stroke-dasharray="3 3"]')).toBeInTheDocument();
    expect(await screen.findByText('CPU')).toBeInTheDocument();
    expect(screen.getByText('50.0%')).toBeInTheDocument();

    fireEvent.mouseLeave(svg);
    expect(svg.querySelector('line[stroke-dasharray="3 3"]')).toBeNull();
    expect(screen.queryByText('CPU')).toBeNull();
  });

  it('limits tooltip rows and shows the "+N more series" affordance', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2024-01-01T12:00:00Z'));
    const now = Date.now();

    const makeSeries = (i: number, value: number) => ({
      name: `S${i}`,
      color: `#${String(i).padStart(2, '0')}0000`,
      data: [
        { timestamp: now - 30_000, value },
        { timestamp: now - 10_000, value },
      ],
    });

    const { container } = render(() => (
      <InteractiveSparkline timeRange="1h" series={[1, 2, 3, 4, 5, 6, 7, 8].map((i) => makeSeries(i, i * 10))} maxTooltipRows={6} />
    ));

    const svg = container.querySelector('svg');
    expect(svg).not.toBeNull();
    if (!svg) return;

    (svg as unknown as { getBoundingClientRect: () => DOMRect }).getBoundingClientRect = () =>
      ({
        left: 0,
        top: 100,
        width: 200,
        height: 50,
        right: 200,
        bottom: 150,
        x: 0,
        y: 100,
        toJSON: () => ({}),
      }) as unknown as DOMRect;

    fireEvent.mouseMove(svg, { clientX: 199, clientY: 110 });

    expect(await screen.findByText('S1')).toBeInTheDocument();
    expect(screen.getByText('S6')).toBeInTheDocument();
    expect(screen.queryByText('S7')).toBeNull();
    expect(screen.getByText('+2 more series')).toBeInTheDocument();
  });

  it('clamps tooltip position so it stays in the viewport', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2024-01-01T12:00:00Z'));
    const now = Date.now();

    const { container } = render(() => (
      <InteractiveSparkline
        timeRange="1h"
        series={[
          {
            name: 'CPU',
            color: '#ff0000',
            data: [
              { timestamp: now - 30_000, value: 40 },
              { timestamp: now - 10_000, value: 50 },
            ],
          },
        ]}
      />
    ));

    const svg = container.querySelector('svg');
    expect(svg).not.toBeNull();
    if (!svg) return;

    (svg as unknown as { getBoundingClientRect: () => DOMRect }).getBoundingClientRect = () =>
      ({
        left: 0,
        top: 10,
        width: 200,
        height: 50,
        right: 200,
        bottom: 60,
        x: 0,
        y: 10,
        toJSON: () => ({}),
      }) as unknown as DOMRect;

    fireEvent.mouseMove(svg, { clientX: 1, clientY: 11 });

    const cpuLabel = await screen.findByText('CPU');
    const tooltip = cpuLabel.closest('div[style]') as HTMLElement | null;
    expect(tooltip).not.toBeNull();
    if (!tooltip) return;

    const left = Number.parseFloat(tooltip.style.left);
    const top = Number.parseFloat(tooltip.style.top);
    expect(left).toBeGreaterThanOrEqual(100);
    expect(top).toBeGreaterThan(40);
  });
});
