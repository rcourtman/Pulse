import { cleanup, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Disk, Node } from '@/types/api';
import { toNodeFromProxmox } from '@/components/Infrastructure/resourceDetailMappers';
import type { Resource } from '@/types/resource';
import { formatBytes } from '@/utils/format';
import { RootDiskCard } from '../RootDiskCard';

vi.mock('@/components/Workloads/StackedDiskBar', () => ({
  StackedDiskBar: (props: { aggregateDisk: { usage: number } }) => (
    <div data-testid="disk-bar" data-usage={props.aggregateDisk.usage} />
  ),
}));
afterEach(cleanup);
const GB = 1024 ** 3;
const node = (disk?: Disk) => ({ disk }) as Node;

describe('RootDiskCard observation availability', () => {
  it.each([
    undefined,
    {},
    { total: 0, used: 0 },
    { total: -1, used: 0 },
    { total: Infinity, used: 0 },
    { total: NaN, used: 0 },
    { used: GB },
    { usage: 0 },
  ])('does not invent usage or capacity for %j', (disk) => {
    render(() => <RootDiskCard node={node(disk)} />);
    expect(screen.getByRole('status')).toHaveTextContent(/^Usage unavailable$/);
    expect(screen.queryByTestId('disk-bar')).toBeNull();
    expect(screen.queryByText(/0 B|NaN|Infinity|%/)).toBeNull();
  });

  it.each([
    { total: 100 * GB },
    { total: 100 * GB, free: 100 * GB },
    { total: 100 * GB, usage: 0 },
    { total: 100 * GB, used: -1 },
    { total: 100 * GB, used: NaN },
    { total: 100 * GB, used: Infinity },
    { total: 100 * GB, used: 101 * GB },
    { total: 100 * GB, used: 25 * GB, usage: -1 },
    { total: 100 * GB, used: 25 * GB, usage: NaN },
    { total: 100 * GB, used: 25 * GB, usage: Infinity },
  ])('preserves only actual capacity when usage is unavailable: %j', (disk) => {
    render(() => <RootDiskCard node={node(disk)} />);
    expect(screen.getByRole('status')).toHaveTextContent('Usage unavailable · 100 GB total');
    expect(screen.queryByTestId('disk-bar')).toBeNull();
    expect(screen.queryByText(/0 B|NaN|Infinity|%/)).toBeNull();
  });

  it.each([0, 0.5, 25, 93, 100])('keeps explicitly measured %s%%, including zero', (percent) => {
    render(() => (
      <RootDiskCard node={node({ total: 100 * GB, used: percent * GB, usage: percent / 100 })} />
    ));
    expect(screen.queryByRole('status')).toBeNull();
    expect(screen.getByTestId('disk-bar')).toHaveAttribute('data-usage', String(percent));
    expect(screen.getByText(`${formatBytes(percent * GB)} / 100 GB`)).toBeInTheDocument();
  });

  it('reacts to first, partial and removed canonical readings without reopening', () => {
    const initial: Resource = {
      id: 'root-disk-node',
      name: 'Node',
      displayName: 'Node',
      type: 'agent',
      platformType: 'proxmox-pve',
      platformId: 'fixture',
      sourceType: 'api',
      status: 'online',
      lastSeen: 1792584000000,
      platformData: { proxmox: { nodeName: 'Node' } },
    };
    const [resource, setResource] = createSignal(initial);
    render(() => <RootDiskCard node={toNodeFromProxmox(resource())!} />);
    expect(screen.getByRole('status')).toHaveTextContent(/^Usage unavailable$/);
    setResource({ ...initial, disk: { current: 25, total: 100 * GB, used: 25 * GB } });
    expect(screen.getByTestId('disk-bar')).toHaveAttribute('data-usage', '25');
    setResource({ ...initial, disk: { current: 0, total: 100 * GB } });
    expect(screen.getByRole('status')).toHaveTextContent('Usage unavailable · 100 GB total');
    expect(screen.queryByTestId('disk-bar')).toBeNull();
    setResource({ ...initial, disk: { current: 0, total: 100 * GB, used: 0 } });
    expect(screen.getByTestId('disk-bar')).toHaveAttribute('data-usage', '0');
    setResource(initial);
    expect(screen.getByRole('status')).toHaveTextContent(/^Usage unavailable$/);
    expect(screen.queryByTestId('disk-bar')).toBeNull();
    expect(resource()).toEqual(initial);
  });
});
