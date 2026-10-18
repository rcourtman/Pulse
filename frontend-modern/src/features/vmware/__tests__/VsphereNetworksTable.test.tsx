import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { VsphereNetworksTable } from '@/features/vmware/VsphereNetworksTable';
import type { Resource } from '@/types/resource';

vi.mock('@/contexts/appRuntime', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/contexts/appRuntime')>()),
  useWebSocket: () => ({ activeAlerts: {} }),
}));

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
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );

    await fireEvent.click(row!);

    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute('aria-expanded', 'true');
  });

  it('says why an amber network needs attention and leaves healthy rows blank', () => {
    const healthy = makeNetwork({ id: 'VM Network' });
    const alarmed = makeNetwork({
      id: 'Edge Stateful',
      status: 'degraded',
      incidents: [
        {
          provider: 'vmware',
          code: 'vmware_alarm_state',
          severity: 'warning',
          summary: 'Network packet loss above threshold',
        },
      ],
      vmware: { entityType: 'network', overallStatus: 'yellow', activeAlarmCount: 1 },
    });
    const countedOnly = makeNetwork({
      id: 'Utility Network',
      vmware: { entityType: 'network', activeAlarmCount: 2 },
    });
    const red = makeNetwork({
      id: 'Archive Network',
      vmware: { entityType: 'network', overallStatus: 'red' },
    });

    render(() => (
      <VsphereNetworksTable
        networks={[healthy, alarmed, countedOnly, red]}
        scope={[healthy, alarmed, countedOnly, red]}
        emptyIcon={<span />}
        emptyTitle="No networks"
        emptyDescription="No networks"
        showToolbar={false}
      />
    ));

    expect(screen.getByRole('columnheader', { name: /Health/ })).toBeInTheDocument();
    const health = (name: string) =>
      screen.getByText(name).closest('tr')?.querySelector('[data-vsphere-health]');

    expect(health('VM Network')).toBeNull();
    // The alarm's own name explains the dot; vCenter's yellow adds nothing.
    expect(health('Edge Stateful')).toHaveAttribute('data-vsphere-health', 'warning');
    expect(health('Edge Stateful')).toHaveAttribute('title', 'Network packet loss above threshold');
    expect(health('Utility Network')).toHaveAttribute('title', '2 active vCenter alarms');
    // The dot reads from the same classification as the reason, even when the
    // resource status is still online.
    const dot = (name: string) =>
      screen.getByText(name).closest('tr')?.querySelector('span.rounded-full[title]');
    expect(dot('Utility Network')).toHaveAttribute('title', 'Attention');
    expect(dot('Archive Network')).toHaveClass('bg-red-500');
    expect(dot('VM Network')).toHaveAttribute('title', 'Online');
    expect(health('Archive Network')).toHaveAttribute('data-vsphere-health', 'danger');
    expect(health('Archive Network')).toHaveAttribute('title', 'vCenter health is red');
  });
});
