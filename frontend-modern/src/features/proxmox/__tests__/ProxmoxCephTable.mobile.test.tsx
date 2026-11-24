import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { ProxmoxCephTable } from '../ProxmoxCephTable';

vi.mock('@/hooks/useObservedElementWidth', () => ({
  useObservedElementWidth: () => ({
    setElement: () => undefined,
    width: () => 360,
  }),
}));

const makeCluster = (id: string, name: string): Resource =>
  ({
    id,
    type: 'ceph',
    name,
    displayName: name,
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    ceph: {
      fsid: `fsid-${id}`,
      healthStatus: 'HEALTH_OK',
      numMons: 3,
      numMgrs: 2,
      numOsds: 12,
      numOsdsUp: 12,
      numOsdsIn: 12,
      numPGs: 256,
      pools: [{ name: 'rbd', storedBytes: 1024, availableBytes: 2048, objects: 10 }],
    },
  }) as unknown as Resource;

describe('ProxmoxCephTable phone cluster readability', () => {
  afterEach(cleanup);

  it('wraps cluster names on compact phone width so distinguishing suffixes stay visible', () => {
    render(() => (
      <ProxmoxCephTable
        resources={[
          makeCluster('ceph-1', 'Mock Cluster East 01'),
          makeCluster('ceph-2', 'Mock Cluster East 02'),
        ]}
        emptyIcon={<span />}
        emptyTitle="No Ceph"
        emptyDescription="No Ceph"
      />
    ));

    const first = screen.getByText('Mock Cluster East 01');
    const second = screen.getByText('Mock Cluster East 02');
    expect(first).toHaveClass('whitespace-normal', 'wrap-anywhere');
    expect(first).not.toHaveClass('truncate');
    expect(second).toHaveClass('whitespace-normal', 'wrap-anywhere');
  });
});
