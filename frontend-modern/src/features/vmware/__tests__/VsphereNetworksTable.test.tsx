import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import { VsphereNetworksTable } from '@/features/vmware/VsphereNetworksTable';
import type { Resource } from '@/types/resource';

const makeNetwork = (overrides: Partial<Resource> & Pick<Resource, 'id'>): Resource =>
  ({
    type: 'network',
    name: overrides.id,
    displayName: overrides.id,
    status: 'online',
    platformType: 'vmware-vsphere',
    platformScopes: ['vmware-vsphere'],
    sourceType: 'api',
    vmware: {
      entityType: 'network',
      managedObjectId: 'network-101',
      networkType: 'STANDARD_PORTGROUP',
      datacenterName: 'Primary DC',
      folderName: 'Networks',
      vcenterHost: 'vcsa.lab.local',
      networkHostNames: ['esxi-01.lab.local', 'esxi-02.lab.local'],
      networkVmNames: ['warehouse-api-01', 'etl-batch-01'],
      overallStatus: 'green',
    },
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
});

describe('VsphereNetworksTable', () => {
  it('renders vCenter network topology as a table', async () => {
    const vmNetwork = makeNetwork({ id: 'VM Network', name: 'VM Network' });
    const edgeStateful = makeNetwork({
      id: 'Edge Stateful',
      name: 'Edge Stateful',
      status: 'degraded',
      vmware: {
        entityType: 'network',
        managedObjectId: 'network-302',
        networkType: 'DISTRIBUTED_PORTGROUP',
        datacenterName: 'Edge DC',
        networkHostNames: ['esxi-06.lab.local'],
        networkVmNames: ['mariadb-replica-01'],
        activeAlarmCount: 1,
      },
    });

    render(() => (
      <VsphereNetworksTable
        networks={[vmNetwork, edgeStateful]}
        scope={[vmNetwork, edgeStateful]}
        emptyIcon={<span />}
        emptyTitle="No networks"
        emptyDescription="No networks"
        showToolbar={false}
      />
    ));

    const table = screen.getByRole('table');
    expect(within(table).getByText('Network')).toBeInTheDocument();
    expect(within(table).getByText('Type')).toBeInTheDocument();
    expect(within(table).getByText('Hosts')).toBeInTheDocument();
    expect(within(table).getByText('Connected VMs')).toBeInTheDocument();
    expect(within(table).queryByRole('columnheader', { name: 'State' })).not.toBeInTheDocument();
    // vCenter's raw enums (STANDARD_PORTGROUP / DISTRIBUTED_PORTGROUP) are
    // mapped to operator-friendly labels matching the names vCenter uses
    // in its own UI; the raw enum is no longer surfaced.
    expect(screen.getByText('Standard port group')).toBeInTheDocument();
    expect(screen.getByText('vDS port group')).toBeInTheDocument();
    expect(screen.getByText('esxi-01.lab.local, esxi-02.lab.local')).toBeInTheDocument();
    expect(screen.getByText('warehouse-api-01, etl-batch-01')).toBeInTheDocument();

    const row = screen.getByText('VM Network').closest('tr');
    expect(row).toHaveAttribute('aria-expanded', 'false');

    await fireEvent.click(row!);

    expect(row).toHaveAttribute('aria-expanded', 'true');
  });
});
