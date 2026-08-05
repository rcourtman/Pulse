import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { ProxmoxCephTable } from '../ProxmoxCephTable';

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
    pools: [],
    services: [],
  },
});

afterEach(cleanup);

describe('ProxmoxCephTable', () => {
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
    const close = screen.getByRole('button', { name: 'Close ceph cluster drawer' });
    close.focus();
    await fireEvent.click(close);

    await waitFor(() => expect(disclosure).toHaveFocus());
    expect(disclosure).toHaveAccessibleName('Expand details for ceph-main');
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
    const close = screen.getByRole('button', { name: 'Close ceph cluster drawer' });
    close.focus();
    await fireEvent.click(close);

    await waitFor(() => expect(disclosure).toHaveFocus());
    expect(disclosure).toHaveAccessibleName('Expand details for ceph-lab');
  });
});
