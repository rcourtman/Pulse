import { describe, expect, it } from 'vitest';
import { render, screen } from '@solidjs/testing-library';

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
});
