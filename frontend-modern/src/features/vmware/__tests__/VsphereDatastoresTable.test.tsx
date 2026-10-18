import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/contexts/appRuntime', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/contexts/appRuntime')>()),
  useWebSocket: () => ({ activeAlerts: {} }),
}));

vi.mock('@/components/Workloads/StackedDiskBar', () => ({
  StackedDiskBar: () => <div data-testid="stacked-disk-bar" />,
}));

import { VsphereDatastoresTable } from '@/features/vmware/VsphereDatastoresTable';
import type { Resource } from '@/types/resource';

const makeDatastore = (overrides: Partial<Resource> & Pick<Resource, 'id'>): Resource =>
  ({
    type: 'storage',
    name: overrides.id,
    displayName: overrides.id,
    status: 'online',
    platformType: 'vmware-vsphere',
    platformScopes: ['vmware-vsphere'],
    sourceType: 'api',
    disk: { current: 50, used: 5_000_000_000_000, total: 10_000_000_000_000 },
    storage: {
      topology: 'datastore',
      platform: 'vmware-vsphere',
      type: 'vmfs',
      nodes: ['esxi-01.lab.local', 'esxi-02.lab.local'],
      consumerCount: 2,
      topConsumers: [
        { resourceType: 'vm', name: 'warehouse-api-01' },
        { resourceType: 'vm', name: 'etl-batch-01' },
      ],
    },
    vmware: {
      entityType: 'datastore',
      datastoreAccessible: true,
      datastoreType: 'VMFS',
      datacenterName: 'Primary DC',
      datastoreUrl: 'ds:///vmfs/volumes/nvme-primary/',
      vcenterHost: 'vcsa.lab.local',
    },
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
});

describe('VsphereDatastoresTable', () => {
  it('renders vCenter datastore fields without generic storage protection columns', async () => {
    const nvme = makeDatastore({ id: 'nvme-primary', name: 'nvme-primary' });
    const inaccessible = makeDatastore({
      id: 'edge-cold-iscsi',
      name: 'edge-cold-iscsi',
      status: 'offline',
      vmware: {
        entityType: 'datastore',
        datastoreAccessible: false,
        datastoreType: 'VMFS',
        datacenterName: 'Edge DC',
        datastoreUrl: 'ds:///vmfs/volumes/edge-cold-iscsi/',
      },
    });

    render(() => (
      <VsphereDatastoresTable
        datastores={[nvme, inaccessible]}
        scope={[nvme, inaccessible]}
        emptyIcon={<span />}
        emptyTitle="No datastores"
        emptyDescription="No datastores"
        showToolbar={false}
      />
    ));

    const table = screen.getByRole('table');
    expect(within(table).getByText('Datastore')).toBeInTheDocument();
    expect(within(table).getByText('Type')).toBeInTheDocument();
    expect(within(table).getByText('Capacity')).toBeInTheDocument();
    expect(within(table).getByText('Hosts')).toBeInTheDocument();
    expect(within(table).getByText('VMs')).toBeInTheDocument();
    expect(within(table).queryByText('Protection')).not.toBeInTheDocument();
    expect(within(table).queryByText('Growth (24h)')).not.toBeInTheDocument();
    expect(within(table).queryByRole('columnheader', { name: 'State' })).not.toBeInTheDocument();
    expect(screen.getAllByText('esxi-01.lab.local, esxi-02.lab.local')).toHaveLength(2);
    expect(screen.getAllByText('warehouse-api-01, etl-batch-01')).toHaveLength(2);
    expect(screen.getAllByTestId('stacked-disk-bar').length).toBeGreaterThan(0);

    const row = screen.getByText('nvme-primary').closest('tr');
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );

    await fireEvent.click(row!);

    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute('aria-expanded', 'true');
  });

  it('says why a datastore is not green and leaves healthy rows blank', () => {
    const vmware = (extra: Record<string, unknown>) =>
      ({ entityType: 'datastore', datastoreAccessible: true, ...extra }) as Resource['vmware'];
    const healthy = makeDatastore({ id: 'nvme-primary' });
    const alarmed = makeDatastore({
      id: 'archive-tier',
      status: 'degraded',
      incidents: [
        {
          provider: 'vmware',
          code: 'vmware_alarm_state',
          severity: 'warning',
          summary: 'Datastore latency above threshold',
        },
      ],
      vmware: vmware({ overallStatus: 'yellow' }),
    });
    const yellow = makeDatastore({
      id: 'edge-cold-iscsi',
      vmware: vmware({ overallStatus: 'yellow' }),
    });
    const maintenance = makeDatastore({
      id: 'edge-warm-nfs',
      vmware: vmware({ maintenanceMode: 'inMaintenance' }),
    });
    const inaccessible = makeDatastore({
      id: 'backup-nfs',
      status: 'offline',
      vmware: vmware({ datastoreAccessible: false }),
    });
    const stale = makeDatastore({
      id: 'edge-vsan',
      platformData: { sourceStatus: { vmware: { status: 'stale' } } },
    });
    const unreachableInMaintenance = makeDatastore({
      id: 'edge-nvme-tier',
      status: 'offline',
      vmware: vmware({ maintenanceMode: 'inMaintenance', datastoreAccessible: false }),
    });
    const all = [
      healthy,
      alarmed,
      yellow,
      maintenance,
      inaccessible,
      stale,
      unreachableInMaintenance,
    ];

    render(() => (
      <VsphereDatastoresTable
        datastores={all}
        scope={all}
        emptyIcon={<span />}
        emptyTitle="No datastores"
        emptyDescription="No datastores"
        showToolbar={false}
      />
    ));

    expect(screen.getByRole('columnheader', { name: /Health/ })).toBeInTheDocument();
    const health = (name: string) =>
      screen.getByText(name).closest('tr')?.querySelector('[data-vsphere-health]');

    expect(health('nvme-primary')).toBeNull();
    expect(health('archive-tier')).toHaveAttribute('title', 'Datastore latency above threshold');
    expect(health('edge-cold-iscsi')).toHaveAttribute('title', 'vCenter health is yellow');
    expect(health('edge-warm-nfs')).toHaveAttribute('title', 'In maintenance mode');
    expect(health('backup-nfs')).toHaveAttribute('data-vsphere-health', 'danger');
    expect(health('backup-nfs')).toHaveAttribute('title', 'vCenter reports it inaccessible');
    expect(health('edge-vsan')).toHaveAttribute('title', 'vCenter has not updated this recently');
    // Maintenance does not hide that vCenter cannot reach the datastore.
    expect(health('edge-nvme-tier')).toHaveAttribute('data-vsphere-health', 'danger');
    expect(health('edge-nvme-tier')).toHaveAttribute(
      'title',
      'vCenter reports it inaccessible\nIn maintenance mode',
    );
    const dot = (name: string) =>
      screen.getByText(name).closest('tr')?.querySelector('span.rounded-full[title]');
    expect(dot('edge-warm-nfs')).toHaveAttribute('title', 'Maintenance');
    expect(dot('nvme-primary')).toHaveAttribute('title', 'Online');
  });
});
