import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { ProxmoxCephTable } from '../ProxmoxCephTable';

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({ activeAlerts: {} }),
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({ detectionEnabled: () => true }),
}));

const makeCluster = (
  id: string,
  overrides: Partial<NonNullable<Resource['ceph']>> = {},
): Resource =>
  ({
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
      numOsds: 18,
      numOsdsUp: 18,
      numOsdsIn: 18,
      numPGs: 512,
      pools: [],
      services: [
        { type: 'mon', running: 3, total: 3 },
        { type: 'mgr', running: 2, total: 2 },
        { type: 'mds', running: 2, total: 2 },
      ],
      ...overrides,
    },
  }) as unknown as Resource;

describe('ProxmoxCephTable quorum and services separation', () => {
  afterEach(cleanup);

  it('renders distinct quorum verdicts and formatted service counts without duplicating MON/MGR counts', () => {
    render(() => (
      <ProxmoxCephTable
        resources={[
          makeCluster('ceph-healthy'),
          makeCluster('ceph-degraded', {
            healthStatus: 'HEALTH_WARN',
            services: [
              { type: 'mon', running: 2, total: 3 },
              { type: 'mgr', running: 2, total: 2 },
              { type: 'mds', running: 1, total: 2 },
            ],
          }),
          makeCluster('ceph-lost', {
            healthStatus: 'HEALTH_ERR',
            services: [
              { type: 'mon', running: 1, total: 3 },
              { type: 'mgr', running: 0, total: 2 },
            ],
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No Ceph clusters"
        emptyDescription="No clusters"
      />
    ));

    const healthyRow = screen.getByText('ceph-healthy').closest('tr')!;
    expect(within(healthyRow).getByText('In quorum')).toBeInTheDocument();
    expect(within(healthyRow).getByText('MON 3/3')).toBeInTheDocument();
    expect(within(healthyRow).getByText('MGR 2/2')).toBeInTheDocument();
    expect(within(healthyRow).getByText('MDS 2/2')).toBeInTheDocument();
    expect(healthyRow.textContent).not.toContain('MON 3 · MGR 2');

    const degradedRow = screen.getByText('ceph-degraded').closest('tr')!;
    expect(within(degradedRow).getByText('Degraded')).toHaveClass('text-amber-600');
    expect(within(degradedRow).getByText('MDS 1/2')).toHaveClass('text-amber-600');

    const lostRow = screen.getByText('ceph-lost').closest('tr')!;
    expect(within(lostRow).getByText('No quorum')).toHaveClass('text-red-600');
    expect(within(lostRow).getByText('MGR 0/2')).toHaveClass('text-red-600');
  });
});
