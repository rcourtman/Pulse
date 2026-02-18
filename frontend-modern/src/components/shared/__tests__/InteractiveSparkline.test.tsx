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

  it('locks a hovered series on click and unlocks on second click', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2024-01-01T12:00:00Z'));
    const now = Date.now();

    const { container } = render(() => (
      <InteractiveSparkline
        timeRange="1h"
        highlightNearestSeriesOnHover
        series={[
          {
            name: 'High',
            color: '#ff0000',
            data: [
              { timestamp: now - 30_000, value: 90 },
              { timestamp: now - 10_000, value: 90 },
            ],
          },
          {
            name: 'Low',
            color: '#00ff00',
            data: [
              { timestamp: now - 30_000, value: 10 },
              { timestamp: now - 10_000, value: 10 },
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
        top: 100,
        width: 200,
        height: 100,
        right: 200,
        bottom: 200,
        x: 0,
        y: 100,
        toJSON: () => ({}),
      }) as unknown as DOMRect;

    // Hover near the high series and lock it.
    fireEvent.mouseMove(svg, { clientX: 150, clientY: 110 });
    expect(await screen.findByText('High')).toBeInTheDocument();
    expect(screen.queryByText('Low')).toBeNull();
    fireEvent.click(svg);

    // Scrub near the low series: lock should keep High selected.
    fireEvent.mouseMove(svg, { clientX: 150, clientY: 190 });
    expect(await screen.findByText('High')).toBeInTheDocument();
    expect(screen.queryByText('Low')).toBeNull();

    // Click again to unlock, then low should become selectable.
    fireEvent.click(svg);
    fireEvent.mouseMove(svg, { clientX: 150, clientY: 190 });
    expect(await screen.findByText('Low')).toBeInTheDocument();
    expect(screen.queryByText('High')).toBeNull();
  });

  it('breaks the rendered line when there is a large gap in data', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2024-01-02T00:00:00Z'));
    const now = Date.now();

    const { container } = render(() => (
      <InteractiveSparkline
        timeRange="24h"
        series={[
          {
            name: 'CPU',
            color: '#ff0000',
            data: [
              { timestamp: now - 23 * 60 * 60_000, value: 40 },
              { timestamp: now - 22 * 60 * 60_000, value: 42 },
              { timestamp: now - 5 * 60_000, value: 70 },
              { timestamp: now - 2 * 60_000, value: 75 },
            ],
          },
        ]}
      />
    ));

    await Promise.resolve();

    const paths = container.querySelectorAll('path[vector-effect="non-scaling-stroke"]');
    expect(paths.length).toBeGreaterThan(1);
  });

  it('keeps evenly spaced sparse data connected as a single line', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2024-01-02T00:00:00Z'));
    const now = Date.now();

    const data = Array.from({ length: 30 }, (_, index) => ({
      timestamp: now - (60 - index*2) * 60_000,
      value: 50 + (index % 5),
    }));

    const { container } = render(() => (
      <InteractiveSparkline
        timeRange="1h"
        series={[
          {
            name: 'CPU',
            color: '#ff0000',
            data,
          },
        ]}
      />
    ));

    await Promise.resolve();

    const paths = container.querySelectorAll('path[vector-effect="non-scaling-stroke"]');
    expect(paths.length).toBe(1);
  });

  it('renders tiered-density data (sparse + medium + dense) as a single continuous line', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2024-01-08T00:00:00Z'));
    const now = Date.now();

    // Simulate tiered mock data: sparse (~65min), medium (~2min), dense (~1min)
    const data: { timestamp: number; value: number }[] = [];
    // Sparse tier: 7d–24h ago, ~65min intervals (~133 points)
    for (let i = 0; i < 133; i++) {
      data.push({
        timestamp: now - 7 * 24 * 60 * 60_000 + i * 65 * 60_000,
        value: 30 + Math.sin(i * 0.1) * 10,
      });
    }
    // Medium tier: 24h–2h ago, ~2min intervals (~660 points)
    const mediumStart = now - 24 * 60 * 60_000;
    for (let i = 0; i < 660; i++) {
      data.push({
        timestamp: mediumStart + i * 2 * 60_000,
        value: 35 + Math.sin(i * 0.05) * 8,
      });
    }
    // Dense tier: last 2h, ~1min intervals (~120 points)
    const denseStart = now - 2 * 60 * 60_000;
    for (let i = 0; i < 120; i++) {
      data.push({
        timestamp: denseStart + i * 60_000,
        value: 40 + Math.sin(i * 0.2) * 5,
      });
    }

    const { container } = render(() => (
      <InteractiveSparkline
        timeRange="7d"
        series={[
          {
            name: 'CPU',
            color: '#ff0000',
            data,
          },
        ]}
      />
    ));

    await Promise.resolve();

    // After LTTB downsampling + P90 gap detection, tiered data should render
    // as a single continuous segment, not fragmented by tier transitions.
    const paths = container.querySelectorAll('path[vector-effect="non-scaling-stroke"]');
    expect(paths.length).toBe(1);
  });

  it('can bridge the leading window-start gap when requested', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2024-01-02T00:00:00Z'));
    const now = Date.now();

    const { container } = render(() => (
      <InteractiveSparkline
        timeRange="1h"
        bridgeLeadingGap
        series={[
          {
            name: 'CPU',
            color: '#ff0000',
            data: [
              { timestamp: now - 60 * 60_000, value: 55 },
              { timestamp: now - 5 * 60_000, value: 55 },
              { timestamp: now - 4 * 60_000, value: 60 },
            ],
          },
        ]}
      />
    ));

    await Promise.resolve();

    const paths = container.querySelectorAll('path[vector-effect="non-scaling-stroke"]');
    expect(paths.length).toBe(1);
  });
});
