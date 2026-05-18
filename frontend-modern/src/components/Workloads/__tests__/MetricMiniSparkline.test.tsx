import { describe, expect, it } from 'vitest';
import { fireEvent, render, screen } from '@solidjs/testing-library';

import { MetricMiniSparkline } from '../MetricMiniSparkline';

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
    expect(screen.getByText('45%')).toBeInTheDocument();
    expect(sparkline.querySelector('path')?.getAttribute('d')).toContain('M');
  });

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
});
