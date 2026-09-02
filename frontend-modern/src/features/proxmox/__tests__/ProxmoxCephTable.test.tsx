import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import {
  CEPH_PHONE_COLUMNS,
  CEPH_PHONE_COLUMN_WIDTHS,
  CEPH_NARROW_COLUMNS,
  CEPH_NARROW_COLUMN_WIDTHS,
  ProxmoxCephTable,
} from '../ProxmoxCephTable';

const makeCluster = (id: string): Resource => ({
  id,
  type: 'ceph',
  name: id,
  displayName: id,
  platformId: 'homelab',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_700_000_000_000,
  disk: { total: 10_000, used: 4_000, free: 6_000, current: 40 },
  ceph: {
    healthStatus: 'HEALTH_OK',
    numMons: 3,
    numMgrs: 2,
    numOsds: 4,
    numOsdsUp: 4,
    numOsdsIn: 4,
    numPGs: 128,
    pools: [
      {
        name: 'rbd-primary',
        storedBytes: 32_212_254_720,
        availableBytes: 20_079_751_168,
        objects: 1_764_309,
        percentUsed: 59.2,
      },
    ],
    services: [],
  },
});

afterEach(cleanup);

describe('ProxmoxCephTable', () => {
  it('keeps the phone projection dense and identity-led', () => {
    expect(CEPH_PHONE_COLUMNS).toEqual([
      'cluster',
      'health',
      'quorum',
      'osds',
      'pools',
      'capacity',
    ]);
    expect(CEPH_PHONE_COLUMN_WIDTHS).toEqual({
      cluster: 30,
      health: 15,
      quorum: 13,
      osds: 14,
      pools: 14,
      capacity: 14,
    });
  });

  it('uses a five-column identity-led projection below 360px', () => {
    expect(CEPH_NARROW_COLUMNS).toEqual(['cluster', 'health', 'osds', 'pools', 'capacity']);
    expect(
      CEPH_NARROW_COLUMNS.reduce((total, column) => total + CEPH_NARROW_COLUMN_WIDTHS[column], 0),
    ).toBe(100);
  });

  it('returns focus to the cluster disclosure after closing an auto-opened detail', async () => {
    render(() => (
      <ProxmoxCephTable
        resources={[makeCluster('ceph-main')]}
        emptyIcon={<span />}
        emptyTitle="No Ceph clusters"
        emptyDescription="No clusters"
      />
    ));

    const disclosure = screen.getByRole('button', { name: 'Collapse details for ceph-main' });
    const close = screen.getByRole('button', { name: 'Collapse ceph-main details' });
    close.focus();
    await fireEvent.click(close);

    await waitFor(() => expect(disclosure).toHaveFocus());
    expect(disclosure).toHaveAccessibleName('Expand details for ceph-main');
  });

  it('exposes complete nested pool values through the disclosure control', async () => {
    render(() => (
      <ProxmoxCephTable
        resources={[makeCluster('ceph-main')]}
        emptyIcon={<span />}
        emptyTitle="No Ceph clusters"
        emptyDescription="No clusters"
      />
    ));

    const disclosure = screen.getByRole('button', { name: 'Expand details for rbd-primary' });
    await fireEvent.click(disclosure);

    const detail = document.querySelector('[data-inline-proxmox-ceph-pool-detail-for]');
    expect(detail).toHaveTextContent('1,764,309');
    expect(detail).toHaveTextContent('59.2%');

    await fireEvent.click(disclosure);
    expect(document.querySelector('[data-inline-proxmox-ceph-pool-detail-for]')).toBeNull();
  });

  it('returns focus to the disclosure for a cluster opened by the user', async () => {
    render(() => (
      <ProxmoxCephTable
        resources={[makeCluster('ceph-main'), makeCluster('ceph-lab')]}
        emptyIcon={<span />}
        emptyTitle="No Ceph clusters"
        emptyDescription="No clusters"
      />
    ));

    const disclosure = screen.getByRole('button', { name: 'Expand details for ceph-lab' });
    await fireEvent.click(disclosure);
    const close = screen.getByRole('button', { name: 'Collapse ceph-lab details' });
    close.focus();
    await fireEvent.click(close);

    await waitFor(() => expect(disclosure).toHaveFocus());
    expect(disclosure).toHaveAccessibleName('Expand details for ceph-lab');
  });
});
