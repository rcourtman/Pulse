import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import {
  CEPH_COLUMN_WEIGHTS,
  CEPH_PHONE_COLUMNS,
  CEPH_PHONE_COLUMN_WIDTHS,
  ProxmoxCephTable,
} from '../ProxmoxCephTable';
import cephClusterDrawerSource from '../ProxmoxCephClusterDrawer.tsx?raw';

const activeAlerts: Record<string, unknown> = {};

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({ activeAlerts }),
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({ detectionEnabled: () => true }),
}));

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

afterEach(() => {
  cleanup();
  for (const key of Object.keys(activeAlerts)) delete activeAlerts[key];
});

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

  it('shows every pool figure in the pools card without a nested expansion', () => {
    render(() => (
      <ProxmoxCephTable
        resources={[makeCluster('ceph-main')]}
        emptyIcon={<span />}
        emptyTitle="No Ceph clusters"
        emptyDescription="No clusters"
      />
    ));

    const pools = screen.getByTestId('ceph-cluster-pools');
    expect(pools).toHaveTextContent('rbd-primary');
    expect(pools).toHaveTextContent('1,764,309 objects');
    expect(pools).toHaveTextContent('59.2% used');
    expect(screen.queryByRole('button', { name: /details for rbd-primary/ })).toBeNull();
    // Pool names sit above their figures and wrap: they are compared and copied.
    expect(within(pools).getByText('rbd-primary')).toHaveClass('whitespace-normal');
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

  it("heads the drawer with the row's own health verdict", () => {
    // A cluster without a reported health status reads healthy in its row, so
    // the drawer header must not show a grey "Unknown" beside it.
    const cluster = makeCluster('ceph-main');
    cluster.ceph = { ...cluster.ceph!, healthStatus: '' };
    render(() => (
      <ProxmoxCephTable
        resources={[cluster]}
        emptyIcon={<span />}
        emptyTitle="No Ceph clusters"
        emptyDescription="No clusters"
      />
    ));

    const heading = screen.getByRole('heading', { level: 2, name: 'ceph-main' });
    expect(heading.parentElement?.querySelector('[aria-label="Healthy"]')).not.toBeNull();
    expect(cephClusterDrawerSource).not.toContain('function classifyHealth');
    expect(screen.getByRole('tab', { name: 'Overview' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Manage' })).toBeInTheDocument();
    expect(screen.getByTestId('drawer-no-attention')).toHaveTextContent('No active alerts');
  });

  it('lifts health warnings, down OSDs and full pools into Worth checking', () => {
    const cluster = makeCluster('ceph-main');
    cluster.ceph = {
      ...cluster.ceph!,
      healthStatus: 'HEALTH_WARN',
      healthMessage: '1 osds down',
      healthChecks: [{ code: 'OSD_DOWN', severity: 'HEALTH_WARN', summary: '1 osds down' }],
      numOsdsUp: 3,
      pools: [{ ...cluster.ceph!.pools![0], name: 'rbd-full', percentUsed: 93.4 }],
    };
    render(() => (
      <ProxmoxCephTable
        resources={[cluster]}
        emptyIcon={<span />}
        emptyTitle="No Ceph clusters"
        emptyDescription="No clusters"
      />
    ));

    const lifted = screen.getByTestId('ceph-cluster-drawer-exceptions');
    expect(lifted).toHaveTextContent('1 osds down');
    expect(lifted).toHaveTextContent('3 of 4 up, 4 in');
    expect(lifted).toHaveTextContent('Pool rbd-full');
    expect(lifted).toHaveTextContent('93.4% used');
  });
});
