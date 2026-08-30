import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  MetricDisplayModeSegmentedControl,
  MetricHoverModeSegmentedControl,
} from '../MetricDisplayModeSegmentedControl';

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('MetricHoverModeSegmentedControl', () => {
  it('switches between detailed bar tooltips and synchronized row history', async () => {
    const onChange = vi.fn();
    render(() => <MetricHoverModeSegmentedControl value="history" onChange={onChange} />);

    const group = screen.getByRole('group', { name: 'Row hover behavior' });
    expect(screen.getByRole('button', { name: 'History' })).toHaveAttribute('aria-pressed', 'true');

    await fireEvent.click(screen.getByRole('button', { name: 'Details' }));

    expect(onChange).toHaveBeenCalledWith('details');
    expect(group).toBeInTheDocument();
  });
});

describe('MetricDisplayModeSegmentedControl', () => {
  it('keeps the table history range hidden while current-value bars are selected', () => {
    render(() => (
      <MetricDisplayModeSegmentedControl
        value="bars"
        onChange={vi.fn()}
        range="1h"
        onRangeChange={vi.fn()}
      />
    ));

    expect(screen.getByText('Bars')).toBeInTheDocument();
    expect(screen.getByText('Trends')).toBeInTheDocument();
    expect(screen.queryByText('Range')).toBeNull();
  });

  it('shows compact time range controls for table sparklines', async () => {
    const onRangeChange = vi.fn();
    render(() => (
      <MetricDisplayModeSegmentedControl
        value="sparklines"
        onChange={vi.fn()}
        range="1h"
        onRangeChange={onRangeChange}
      />
    ));

    expect(screen.getByRole('group', { name: 'Sparkline range' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '1h' })).toHaveAttribute('aria-pressed', 'true');

    await fireEvent.click(screen.getByRole('button', { name: '24h' }));

    expect(onRangeChange).toHaveBeenCalledWith('24h');
  });

  it('left-aligns the metric controls inside a preferences section', () => {
    render(() => (
      <MetricDisplayModeSegmentedControl
        value="sparklines"
        onChange={vi.fn()}
        range="1h"
        onRangeChange={vi.fn()}
      />
    ));

    const group = screen.getByRole('group', { name: 'Sparkline range' });
    expect(group.parentElement).toHaveClass('justify-start');
  });
});
