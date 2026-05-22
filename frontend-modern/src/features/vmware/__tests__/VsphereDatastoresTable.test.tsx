import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

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
    expect(row).toHaveAttribute('aria-expanded', 'false');

    await fireEvent.click(row!);

    expect(row).toHaveAttribute('aria-expanded', 'true');
  });
});
