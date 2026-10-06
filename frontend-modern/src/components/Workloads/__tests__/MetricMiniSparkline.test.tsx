import { describe, expect, it } from 'vitest';
import { fireEvent, render, screen } from '@solidjs/testing-library';

import { createSignal } from 'solid-js';
import {
  MetricMiniSparkline,
  MetricMiniSparklineRatePair,
  type MetricMiniSparklineValueLabelContext,
} from '../MetricMiniSparkline';

describe('MetricMiniSparkline', () => {
  it('renders a compact history path and current value label', () => {
    render(() => (
      <MetricMiniSparkline
        title="CPU history"
        unit="%"
        valueLabel="45%"
        series={[
          {
            id: 'cpu',
            label: 'CPU',
            color: '#8b5cf6',
            points: [
              { timestamp: 1, value: 10 },
              { timestamp: 2, value: 20 },
              { timestamp: 3, value: 45 },
            ],
          },
        ]}
      />
    ));

    const sparkline = screen.getByTestId('metric-mini-sparkline');
    expect(sparkline.dataset.renderedSeriesCount).toBe('1');
    expect(sparkline).toHaveClass('grid-cols-[minmax(0,1fr)_auto]');
    expect(screen.getByRole('img', { name: 'CPU history, current 45%' })).toBeInTheDocument();
    expect(screen.getByText('45%')).toBeInTheDocument();
    expect(sparkline.querySelector('path')?.getAttribute('d')).toContain('M');
  });

  it('announces last-known values without changing historical points and withdraws that context reactively', () => {
    const [context, setContext] = createSignal<MetricMiniSparklineValueLabelContext>('last known');
    const { container } = render(() => (
      <MetricMiniSparkline
        title="Disk history"
        unit="%"
        valueLabel="50%"
        valueLabelContext={context()}
        series={[
          {
            id: 'disk',
            label: 'Filesystem',
            color: '#10b981',
            points: [
              { timestamp: 1, value: 25 },
              { timestamp: 2, value: 50 },
            ],
          },
        ]}
      />
    ));
    const chart = screen.getByRole('img', { name: 'Disk history, last known 50%' });
    const path = chart.querySelector('path')?.getAttribute('d');
    expect(screen.getByText('50%')).toBeInTheDocument();
    setContext('current');
    expect(screen.getByRole('img', { name: 'Disk history, current 50%' })).toBe(chart);
    expect(container.querySelector('path')?.getAttribute('d')).toBe(path);
  });

  it.each(['freshness unknown', 'unavailable'] as const)(
    'announces %s context without changing recorded History',
    (context) => {
      const { container } = render(() => (
        <MetricMiniSparkline
          title="Memory history"
          valueLabel={context === 'unavailable' ? 'N/A' : '25%'}
          valueLabelContext={context}
          series={[
            {
              id: 'memory',
              label: 'Memory',
              color: '#f59e0b',
              points: [
                { timestamp: 1, value: 10 },
                { timestamp: 2, value: 25 },
              ],
            },
          ]}
        />
      ));
      expect(
        screen.getByRole('img', {
          name: `Memory history, ${context} ${context === 'unavailable' ? 'N/A' : '25%'}`,
        }),
      ).toBeInTheDocument();
      expect(screen.queryByRole('img', { name: /current/ })).not.toBeInTheDocument();
      expect(container.querySelector('path')?.getAttribute('d')).toContain('M');
    },
  );

  it('keeps the label visible when history has no renderable line', () => {
    render(() => (
      <MetricMiniSparkline
        title="Disk history"
        unit="%"
        valueLabel="—"
        series={[
          {
            id: 'disk',
            label: 'Disk',
            color: '#10b981',
            points: [{ timestamp: 1, value: 12 }],
          },
        ]}
      />
    ));

    const sparkline = screen.getByTestId('metric-mini-sparkline');
    expect(sparkline.dataset.renderedSeriesCount).toBe('0');
    expect(screen.getByText('—')).toBeInTheDocument();
  });

  it('moves bulky I/O labels out of the row and shows cursor values in the tooltip', () => {
    render(() => (
      <MetricMiniSparkline
        title="Network history"
        unit="B/s"
        valueLabel="1 KB/s / 2 KB/s"
        valueLabelMode="tooltip"
        formatValue={(value) => `${value} B/s`}
        series={[
          {
            id: 'netin',
            label: 'In',
            color: '#10b981',
            points: [
              { timestamp: 1_000, value: 10 },
              { timestamp: 2_000, value: 20 },
              { timestamp: 3_000, value: 30 },
            ],
          },
          {
            id: 'netout',
            label: 'Out',
            color: '#fb923c',
            points: [
              { timestamp: 1_000, value: 100 },
              { timestamp: 2_000, value: 200 },
              { timestamp: 3_000, value: 300 },
            ],
          },
        ]}
      />
    ));

    expect(screen.queryByText('1 KB/s / 2 KB/s')).not.toBeInTheDocument();

    const sparkline = screen.getByTestId('metric-mini-sparkline');
    const svg = sparkline.querySelector('svg') as SVGSVGElement;
    svg.getBoundingClientRect = () =>
      ({
        bottom: 38,
        height: 18,
        left: 0,
        right: 96,
        top: 20,
        width: 96,
        x: 0,
        y: 20,
        toJSON: () => ({}),
      }) as DOMRect;

    fireEvent.mouseMove(svg, { clientX: 48, clientY: 24 });

    expect(document.querySelector('[data-metric-mini-sparkline-tooltip="true"]')).not.toBeNull();
    expect(screen.getByText('In')).toBeInTheDocument();
    expect(screen.getByText('20 B/s')).toBeInTheDocument();
    expect(screen.getByText('Out')).toBeInTheDocument();
    expect(screen.getByText('200 B/s')).toBeInTheDocument();
  });

  it('shows a compact rate pair beside the chart while announcing the full rates', () => {
    render(() => (
      <MetricMiniSparkline
        title="Disk I/O history"
        unit="B/s"
        valueLabel="3.32 MB/s / 512 KB/s"
        valueContent={
          <MetricMiniSparklineRatePair metric="diskIo" values={[3.32 * 1024 * 1024, 512 * 1024]} />
        }
        series={[
          {
            id: 'diskread',
            label: 'Read',
            color: '#3b82f6',
            points: [
              { timestamp: 1_000, value: 10 },
              { timestamp: 2_000, value: 20 },
            ],
          },
        ]}
      />
    ));

    const sparkline = screen.getByTestId('metric-mini-sparkline');
    expect(sparkline.dataset.valueLabelMode).toBe('inline');
    expect(sparkline.textContent).toBe('R3.3MW512K');
    expect(screen.queryByText('3.32 MB/s / 512 KB/s')).not.toBeInTheDocument();
    expect(sparkline.querySelector('svg')).toHaveAttribute(
      'aria-label',
      'Disk I/O history, current 3.32 MB/s / 512 KB/s',
    );

    const pair = sparkline.querySelector('[data-metric-rate-pair]') as HTMLElement;
    expect(pair).toHaveAttribute('aria-hidden', 'true');
    // Glyph colours match the read/write series they label.
    expect(screen.getByText('R')).toHaveStyle({ color: '#3b82f6' });
    expect(screen.getByText('W')).toHaveStyle({ color: '#f59e0b' });
  });

  it('re-reads an open tooltip from the current series when history updates', () => {
    const series = (inbound: number) => [
      {
        id: 'netin',
        label: 'In',
        color: '#10b981',
        points: [
          { timestamp: 1_000, value: 10 },
          { timestamp: 2_000, value: inbound },
          { timestamp: 3_000, value: 30 },
        ],
      },
    ];
    const [current, setCurrent] = createSignal(series(20));
    render(() => (
      <MetricMiniSparkline
        title="Network history"
        unit="B/s"
        formatValue={(value) => `${value} B/s`}
        series={current()}
      />
    ));

    const svg = screen.getByTestId('metric-mini-sparkline').querySelector('svg') as SVGSVGElement;
    svg.getBoundingClientRect = () =>
      ({
        bottom: 38,
        height: 18,
        left: 0,
        right: 96,
        top: 20,
        width: 96,
        x: 0,
        y: 20,
        toJSON: () => ({}),
      }) as DOMRect;
    fireEvent.mouseMove(svg, { clientX: 48, clientY: 24 });
    expect(screen.getByText('20 B/s')).toBeInTheDocument();

    setCurrent(series(25));

    expect(screen.queryByText('20 B/s')).not.toBeInTheDocument();
    expect(screen.getByText('25 B/s')).toBeInTheDocument();
  });
});
