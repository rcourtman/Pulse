import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
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

  it.each([2, 24])('keeps all %i mounts in the parent scroll flow', (count) => {
    const disks = Array.from({ length: count }, (_, i) => ({
      mountpoint: `/mnt/disk-${i}`,
      total: 100,
      used: 50,
      free: 50,
      usage: 0.5,
    }));
    render(() => <DisksCard disks={disks} />);
    const mounts = screen.getByTestId('disks-card-mounts');
    expect(mounts.children).toHaveLength(count);
    expect(mounts.className).not.toMatch(/max-h-|overflow-|custom-scrollbar/);
    expect(screen.getByTitle(`/mnt/disk-${count - 1}`)).toBeInTheDocument();
  });

  const GiB = 1024 ** 3;
  const fullRootDisks = () => [
    { mountpoint: '/', total: 100 * GiB, used: 93 * GiB, free: 7 * GiB, usage: 0.93 },
    { mountpoint: '/data', total: 100 * GiB, used: 41 * GiB, free: 59 * GiB, usage: 0.41 },
  ];

  it('colours current usage by the disk thresholds and draws usage bars', () => {
    render(() => <DisksCard disks={fullRootDisks()} />);

    expect(screen.getByTestId('stacked-disk-bar')).toBeInTheDocument();
    expect(screen.getByText('93%')).toHaveClass('text-red-600');
    expect(screen.getByText('41%')).toHaveClass('text-muted');
    expect(screen.getAllByTestId('disks-card-mount-bar')).toHaveLength(2);
    expect(screen.getByTestId('disks-card-mounts')).not.toHaveTextContent('Last known');
  });

  it('keeps retained usage figures but reads them as last known, without threshold colour or bars', () => {
    render(() => (
      <DisksCard disks={fullRootDisks()} lastKnownReason="host agent stopped reporting" />
    ));
    const title = 'Last known reading, not current: host agent stopped reporting';

    const total = screen.getByTestId('disks-card-total');
    expect(total).toHaveTextContent('Total Usage');
    expect(total).toHaveTextContent('Last known 67% · 134 GB / 200 GB');
    expect(within(total).getByTitle(title)).toHaveClass('text-muted');
    expect(screen.queryByTestId('stacked-disk-bar')).toBeNull();

    const mounts = screen.getByTestId('disks-card-mounts');
    expect(mounts.children).toHaveLength(2);
    const root = screen.getByText('Last known 93%');
    expect(root).toHaveClass('text-muted');
    expect(root).not.toHaveClass('text-red-600');
    expect(root.closest('[title]')).toHaveAttribute('title', title);
    expect(mounts).toHaveTextContent('93.0 GB / 100 GB');
    expect(screen.getByText('Last known 41%')).toHaveClass('text-muted');
    expect(screen.queryAllByTestId('disks-card-mount-bar')).toHaveLength(0);
  });

  it('follows the reason when an agent stops and resumes reporting', () => {
    const [reason, setReason] = createSignal<string | undefined>();
    render(() => <DisksCard disks={fullRootDisks()} lastKnownReason={reason()} />);

    expect(screen.getByText('93%')).toHaveClass('text-red-600');
    expect(screen.getAllByTestId('disks-card-mount-bar')).toHaveLength(2);

    setReason('host agent stopped reporting');
    expect(screen.getByText('Last known 93%')).toHaveClass('text-muted');
    expect(screen.queryByTestId('stacked-disk-bar')).toBeNull();
    expect(screen.queryAllByTestId('disks-card-mount-bar')).toHaveLength(0);

    setReason(undefined);
    expect(screen.getByText('93%')).toHaveClass('text-red-600');
    expect(screen.queryByText(/Last known/)).toBeNull();
    expect(screen.getByTestId('stacked-disk-bar')).toBeInTheDocument();
    expect(screen.getAllByTestId('disks-card-mount-bar')).toHaveLength(2);
  });

  it('renders nothing when no disks are available', () => {
    const { container } = render(() => <DisksCard disks={[]} />);

    expect(container).toBeEmptyDOMElement();
  });
});
