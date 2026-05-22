import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { VsphereVirtualMachinesTable } from '@/features/vmware/VsphereVirtualMachinesTable';
import type { Resource } from '@/types/resource';

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: () => <div data-testid="metric-cell">metric</div>,
}));

vi.mock('@/components/Workloads/StackedMemoryBar', () => ({
  StackedMemoryBar: () => <div data-testid="memory-cell">memory</div>,
}));

const makeVM = (overrides: Partial<Resource> & Pick<Resource, 'id'>): Resource =>
  ({
    type: 'vm',
    name: overrides.id,
    displayName: overrides.id,
    status: 'online',
    platformType: 'vmware-vsphere',
    platformScopes: ['vmware-vsphere'],
    sourceType: 'api',
    cpu: { current: 28 },
    memory: { current: 55, used: 4_400_000_000, total: 8_000_000_000 },
    vmware: {
      entityType: 'vm',
      managedObjectId: 'vm-201',
      powerState: 'POWERED_ON',
      overallStatus: 'green',
      runtimeHostName: 'esxi-01.lab.local',
      clusterName: 'Production Cluster',
      resourcePoolName: 'Tier 1',
      guestOsFamily: 'LINUX',
      guestHostname: 'warehouse-api-01.internal',
      guestIpAddresses: ['10.42.10.21'],
      networkAdapters: [
        {
          label: 'Network adapter 1',
          type: 'VMXNET3',
          macAddress: '00:50:56:aa:bb:cc',
          networkName: 'VM Network',
          state: 'CONNECTED',
        },
      ],
      datastoreNames: ['nvme-primary', 'backup-nfs'],
      snapshotTree: [
        {
          snapshot: 'snapshot-201',
          name: 'pre-upgrade',
          quiesced: true,
          children: [{ snapshot: 'snapshot-202', name: 'post-upgrade', current: true }],
        },
      ],
    },
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
});

describe('VsphereVirtualMachinesTable', () => {
  it('renders vCenter VM inventory fields instead of generic workload columns', async () => {
    const warehouse = makeVM({ id: 'warehouse-api-01', name: 'warehouse-api-01' });
    const archive = makeVM({
      id: 'cold-archive-01',
      name: 'cold-archive-01',
      status: 'stopped',
      vmware: {
        entityType: 'vm',
        managedObjectId: 'vm-301',
        powerState: 'poweredOff',
        runtimeHostName: 'esxi-03.lab.local',
        clusterName: 'Archive Cluster',
        resourcePoolName: 'Cold',
        guestOsFamily: 'LINUX',
        datastoreNames: ['archive-tier'],
        snapshotCount: 0,
      },
    });

    render(() => (
      <VsphereVirtualMachinesTable
        vms={[warehouse, archive]}
        scope={[warehouse, archive]}
        emptyIcon={<span />}
        emptyTitle="No VMs"
        emptyDescription="No VMs"
        showToolbar={false}
      />
    ));

    const table = screen.getByRole('table');
    expect(within(table).getByText('VM')).toBeInTheDocument();
    expect(within(table).getByText('Power')).toBeInTheDocument();
    expect(within(table).getByText('Host')).toBeInTheDocument();
    expect(within(table).getByText('Pool')).toBeInTheDocument();
    expect(within(table).getByText('Guest')).toBeInTheDocument();
    expect(within(table).getByText('Network')).toBeInTheDocument();
    expect(within(table).getByText('Snapshots')).toBeInTheDocument();
    expect(within(table).getByText('Health')).toBeInTheDocument();
    expect(within(table).queryByRole('columnheader', { name: 'ID' })).not.toBeInTheDocument();
    expect(within(table).queryByRole('columnheader', { name: 'Uptime' })).not.toBeInTheDocument();
    expect(within(table).queryByRole('columnheader', { name: 'Backup' })).not.toBeInTheDocument();
    expect(screen.getAllByText('esxi-01.lab.local')).toHaveLength(2);
    expect(screen.getByText('Tier 1')).toBeInTheDocument();
    expect(screen.getByText('warehouse-api-01.internal')).toBeInTheDocument();
    expect(screen.getByText('VM Network')).toBeInTheDocument();
    expect(screen.getByText('nvme-primary +1')).toBeInTheDocument();
    expect(screen.getByText('Healthy')).toBeInTheDocument();
    expect(screen.getByTitle('pre-upgrade, post-upgrade (current)')).toHaveTextContent('2');

    const row = screen.getByText('warehouse-api-01').closest('tr');
    expect(row).toHaveAttribute('aria-expanded', 'false');

    await fireEvent.click(row!);

    expect(row).toHaveAttribute('aria-expanded', 'true');
  });
});
