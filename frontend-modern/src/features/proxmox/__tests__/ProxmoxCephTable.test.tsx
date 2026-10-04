import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import {
  CEPH_COLUMN_WEIGHTS,
  CEPH_PHONE_COLUMNS,
  CEPH_PHONE_COLUMN_WIDTHS,
  ProxmoxCephTable,
} from '../ProxmoxCephTable';
import cephClusterDrawerSource from '../ProxmoxCephClusterDrawer.tsx?raw';

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
  it('keeps one phone projection of five values that each fit whole', () => {
    // Quorum yields its track on phones: six columns clipped the cluster name,
    // the health word, and every figure, and the monitor and manager counts are
    // itemised in the expansion's Services table.
    expect(CEPH_PHONE_COLUMNS).toEqual(['cluster', 'health', 'osds', 'pools', 'capacity']);
    expect(CEPH_PHONE_COLUMN_WIDTHS).toEqual({
      cluster: 36,
      health: 21,
      osds: 13,
      pools: 12,
      capacity: 18,
    });
    expect(
      CEPH_PHONE_COLUMNS.reduce((total, column) => total + CEPH_PHONE_COLUMN_WIDTHS[column], 0),
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

  it('gives every desktop column a weighted share instead of the remainder', () => {
    render(() => (
      <ProxmoxCephTable
        resources={[
          {
            ...makeCluster('ceph-main'),
            ceph: { ...makeCluster('ceph-main').ceph!, fsid: '8f1c2d3e-fsid' },
          },
        ]}
        emptyIcon={<span />}
        emptyTitle="No Ceph clusters"
        emptyDescription="No clusters"
      />
    ));

    const columns = [...document.querySelectorAll<HTMLElement>('col[data-proxmox-ceph-column]')];
    expect(columns.map((column) => column.dataset.proxmoxCephColumn)).toEqual([
      'cluster',
      'health',
      'quorum',
      'osds',
      'pgs',
      'pools',
      'capacity',
      'services',
      'detail',
    ]);
    const widths = columns.map((column) => Number.parseFloat(column.style.width));
    // The unsized Services column used to receive ~4% and clip to "mon…".
    expect(Math.min(...widths)).toBeGreaterThan(4);
    expect(widths.reduce((total, width) => total + width, 0)).toBeCloseTo(100, 1);
    expect(Object.values(CEPH_COLUMN_WEIGHTS).every((weight) => weight > 0)).toBe(true);
  });

  it('keeps the FSID out of the row and whole in the expansion', () => {
    render(() => (
      <ProxmoxCephTable
        resources={[
          {
            ...makeCluster('ceph-main'),
            ceph: { ...makeCluster('ceph-main').ceph!, fsid: '8f1c2d3e-fsid' },
          },
        ]}
        emptyIcon={<span />}
        emptyTitle="No Ceph clusters"
        emptyDescription="No clusters"
      />
    ));

    expect(screen.queryByRole('columnheader', { name: 'FSID' })).toBeNull();
    expect(document.querySelector('[data-inline-detail-for="ceph-main"]')).toHaveTextContent(
      '8f1c2d3e-fsid',
    );
  });

  it('drops the pool object count, not the usage figure, in a narrow card', () => {
    expect(cephClusterDrawerSource).toMatch(
      /platform-table-phone-hidden md:w-\[16%\]`\}\s*>\s*Objects/,
    );
    expect(cephClusterDrawerSource).toMatch(
      /platform-table-mobile-w-20 md:w-\[20%\]`\}\s*>\s*Used/,
    );
  });
});
