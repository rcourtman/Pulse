import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource, ResourceStatus } from '@/types/resource';
import { ProxmoxCephTable } from '../ProxmoxCephTable';

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({ activeAlerts: {} }),
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({ detectionEnabled: () => true }),
}));

const cluster = (name: string, status: ResourceStatus, healthStatus: string): Resource => ({
  id: name,
  type: 'ceph',
  name,
  displayName: name,
  platformId: 'pve-fixture',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status,
  lastSeen: 1_700_000_000_000,
  disk: { current: 40, total: 1024, used: 410, free: 614 },
  ceph: {
    healthStatus,
    numMons: 3,
    numMgrs: 2,
    numOsds: 12,
    numOsdsUp: 12,
    numOsdsIn: 12,
    numPGs: 96,
  },
});

const table = (resources: () => Resource[]) => (
  <ProxmoxCephTable
    resources={resources()}
    emptyIcon={<span />}
    emptyTitle="No Ceph clusters"
    emptyDescription="No clusters"
  />
);

describe('Ceph reported health', () => {
  afterEach(cleanup);

  it('does not count missing or unrecognized health as healthy when topology is present', async () => {
    const resources = [
      cluster('Alder', 'unknown', ''),
      cluster('Birch', 'online', 'HEALTH_OK'),
      cluster('Cedar', 'degraded', 'HEALTH_WARN'),
      cluster('Dogwood', 'offline', 'ERR'),
      cluster('Elm', 'unknown', 'HEALTH_UNRECOGNIZED'),
    ];
    render(() => table(() => resources));

    const row = (name: string) => within(screen.getByText(name, { exact: true }).closest('tr')!);
    expect(row('Alder').getByRole('cell', { name: 'Unknown' })).toBeInTheDocument();
    expect(row('Alder').getByText('40.0%')).toBeInTheDocument();
    expect(row('Elm').getByRole('cell', { name: 'Unknown' })).toHaveAttribute(
      'title',
      'Unknown · HEALTH_UNRECOGNIZED',
    );
    expect(row('Birch').getByRole('cell', { name: 'Healthy' })).toBeInTheDocument();
    expect(row('Cedar').getByRole('cell', { name: 'Warning' })).toBeInTheDocument();
    expect(row('Dogwood').getByRole('cell', { name: 'Critical' })).toBeInTheDocument();

    await fireEvent.click(screen.getByRole('button', { name: 'Healthy, 1' }));
    expect(screen.getByText('Birch', { exact: true })).toBeInTheDocument();
    for (const name of ['Alder', 'Cedar', 'Dogwood', 'Elm']) {
      expect(screen.queryByText(name, { exact: true })).not.toBeInTheDocument();
    }
    await fireEvent.click(screen.getByRole('button', { name: 'All, 5' }));
    expect(screen.getAllByRole('cell', { name: 'Unknown' })).toHaveLength(2);
  });

  it('updates the same open drawer when a recognized verdict arrives', () => {
    const [resources, setResources] = createSignal([cluster('Alder', 'unknown', ' ')]);
    render(() => table(resources));

    const drawer = screen.getByTestId('ceph-cluster-drawer');
    expect(within(drawer).getByRole('img', { name: 'Unknown' })).toBeInTheDocument();
    setResources([cluster('Alder', 'online', ' HEALTH_OK ')]);
    expect(screen.getByTestId('ceph-cluster-drawer')).toBe(drawer);
    expect(within(drawer).getByRole('img', { name: 'Healthy' })).toBeInTheDocument();
    expect(screen.getByRole('cell', { name: 'Healthy' })).toBeInTheDocument();
  });
});
