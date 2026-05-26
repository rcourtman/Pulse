import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { DisksCard } from '../DisksCard';

vi.mock('@/components/Workloads/StackedDiskBar', () => ({
  StackedDiskBar: (props: {
    disks?: unknown[];
    aggregateDisk?: { total?: number; used?: number };
    mode?: string;
  }) => (
    <div
      data-testid="stacked-disk-bar"
      data-mode={props.mode}
      data-disk-count={props.disks?.length ?? 0}
      data-total={props.aggregateDisk?.total}
      data-used={props.aggregateDisk?.used}
    />
  ),
}));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('DisksCard', () => {
  it('renders an aggregate disk usage summary before individual mounts', () => {
    render(() => (
      <DisksCard
        disks={[
          {
            mountpoint: '/',
            total: 100,
            used: 60,
            free: 40,
            usage: 0.6,
          },
          {
            mountpoint: '/data',
            total: 300,
            used: 120,
            free: 180,
            usage: 0.4,
          },
        ]}
      />
    ));

    expect(screen.getByTestId('disks-card-total')).toHaveTextContent('Total Usage');
    expect(screen.getByTestId('stacked-disk-bar')).toHaveAttribute('data-mode', 'aggregate');
    expect(screen.getByTestId('stacked-disk-bar')).toHaveAttribute('data-disk-count', '2');
    expect(screen.getByTestId('stacked-disk-bar')).toHaveAttribute('data-total', '400');
    expect(screen.getByTestId('stacked-disk-bar')).toHaveAttribute('data-used', '180');
    expect(screen.getByText('/')).toBeInTheDocument();
    expect(screen.getByText('/data')).toBeInTheDocument();
  });

  it('renders nothing when no disks are available', () => {
    const { container } = render(() => <DisksCard disks={[]} />);

    expect(container).toBeEmptyDOMElement();
  });
});
