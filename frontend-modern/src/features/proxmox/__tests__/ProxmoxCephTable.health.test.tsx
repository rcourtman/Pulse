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

    const row = (name: string) =>
      within(
        screen.getByRole('button', { name: new RegExp(`details for ${name}$`) }).closest('tr')!,
      );
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

  it('marks stale HEALTH_OK clusters as Stale out of Healthy, preserves Critical, and recovers live', async () => {
    const staleHealthy: Resource = {
      ...cluster('StaleEast', 'warning', 'HEALTH_OK'),
      sourceStatus: {
        proxmox: { status: 'error', error: 'Proxmox API session expired (401)' },
      },
    };
    const staleCritical: Resource = {
      ...cluster('StaleWest', 'offline', 'HEALTH_ERR'),
      ceph: {
        ...cluster('StaleWest', 'offline', 'HEALTH_ERR').ceph!,
        healthMessage: '2 osds down',
      },
      sourceStatus: { proxmox: { status: 'stale' } },
    };
    const freshHealthy: Resource = {
      ...cluster('FreshNorth', 'online', 'HEALTH_OK'),
      sourceStatus: { proxmox: { status: 'online' } },
    };

    const [resources, setResources] = createSignal<Resource[]>([
      staleHealthy,
      staleCritical,
      freshHealthy,
    ]);
    render(() => table(resources));

    const row = (name: string) =>
      within(
        screen.getByRole('button', { name: new RegExp(`details for ${name}$`) }).closest('tr')!,
      );

    // Stale HEALTH_OK must not read Healthy, shows Stale data + collection error, keeps retained figures.
    expect(row('StaleEast').getByText('Stale data')).toBeInTheDocument();
    expect(row('StaleEast').getByRole('cell', { name: 'Stale' })).toHaveAttribute(
      'title',
      'Stale · Last reported HEALTH_OK · Proxmox collection error: Proxmox API session expired (401)',
    );
    expect(
      row('StaleEast').getByText('Proxmox collection error: Proxmox API session expired (401)'),
    ).toBeInTheDocument();
    expect(row('StaleEast').getByText('40.0%')).toBeInTheDocument();

    // Stale HEALTH_ERR preserves Critical verdict and healthMessage alongside Stale data cue.
    expect(row('StaleWest').getByText('Stale data')).toBeInTheDocument();
    expect(row('StaleWest').getByRole('cell', { name: 'Critical' })).toBeInTheDocument();
    expect(row('StaleWest').getByText('2 osds down')).toBeInTheDocument();

    // Filter counts keep stale HEALTH_OK out of Healthy and inside Warning, while Critical stays Critical.
    expect(screen.getByRole('button', { name: 'Healthy, 1' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Warning, 1' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Critical, 1' })).toBeInTheDocument();

    // Expand stale cluster drawer: header repeats Stale and Worth checking surfaces Readings error.
    await fireEvent.click(screen.getByRole('button', { name: 'Expand details for StaleEast' }));
    const drawer = screen.getByTestId('ceph-cluster-drawer');
    expect(within(drawer).getByRole('img', { name: 'Stale' })).toBeInTheDocument();
    expect(within(drawer).getByTestId('ceph-cluster-drawer-exceptions')).toHaveTextContent(
      'Proxmox collection error: Proxmox API session expired (401)',
    );

    // Live recovery clears Stale data and restores Healthy on both row and open drawer.
    setResources([
      {
        ...staleHealthy,
        status: 'online',
        sourceStatus: { proxmox: { status: 'online' } },
      },
      staleCritical,
      freshHealthy,
    ]);
    expect(row('StaleEast').queryByText('Stale data')).toBeNull();
    expect(row('StaleEast').getByRole('cell', { name: 'Healthy' })).toBeInTheDocument();
    expect(within(drawer).getByRole('img', { name: 'Healthy' })).toBeInTheDocument();
    expect(within(drawer).queryByTestId('ceph-cluster-drawer-exceptions')).toBeNull();
    expect(screen.getByRole('button', { name: 'Healthy, 2' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Warning, 0' })).toBeInTheDocument();
  });
});
