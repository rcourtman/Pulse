import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import { VsphereAlertsTable } from '@/features/vmware/VsphereAlertsTable';
import { buildVmwareIncidentRows } from '@/features/vmware/vmwarePageModel';
import type { Resource } from '@/types/resource';

const makeHost = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'host-alarm',
    type: 'agent',
    name: 'esxi-01.lab.local',
    displayName: 'esxi-01.lab.local',
    status: 'degraded',
    platformType: 'vmware-vsphere',
    platformScopes: ['vmware-vsphere'],
    sourceType: 'api',
    vmware: {
      entityType: 'host',
      managedObjectId: 'host-101',
      connectionName: 'lab-vcenter',
      vcenterHost: 'vcsa.lab.local',
      datacenterName: 'Primary DC',
      clusterName: 'Production Cluster',
    },
    incidents: [
      {
        provider: 'vmware',
        nativeId: 'alarm-401',
        code: 'vmware_alarm_state',
        severity: 'critical',
        source: 'vmware',
        summary: 'Host host-101 has VMware alarm Host connection and power state (red)',
        startedAt: '2026-05-21T14:30:00Z',
      },
    ],
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
});

describe('VsphereAlertsTable', () => {
  it('renders native vSphere health signal rows with inline details', async () => {
    const host = makeHost();
    const incidents = buildVmwareIncidentRows([host]);

    render(() => (
      <VsphereAlertsTable
        incidents={incidents}
        emptyIcon={<span />}
        emptyTitle="No signals"
        emptyDescription="No signals"
        showToolbar={false}
      />
    ));

    const table = screen.getByRole('table');
    expect(within(table).getByText('Resource')).toBeInTheDocument();
    expect(within(table).getByText('Severity')).toBeInTheDocument();
    expect(within(table).getByText('Signal')).toBeInTheDocument();
    expect(within(table).getByText('vCenter')).toBeInTheDocument();
    expect(within(table).getByText('Entity')).toBeInTheDocument();
    expect(screen.getByText('esxi-01.lab.local')).toBeInTheDocument();
    expect(screen.getByText('Critical')).toBeInTheDocument();
    expect(
      screen.getByText('Host host-101 has VMware alarm Host connection and power state (red)'),
    ).toBeInTheDocument();
    expect(screen.getByText('lab-vcenter')).toBeInTheDocument();
    expect(screen.getByText('host-101')).toBeInTheDocument();

    const row = screen
      .getByText('Host host-101 has VMware alarm Host connection and power state (red)')
      .closest('tr');
    expect(row).toHaveAttribute('aria-expanded', 'false');

    await fireEvent.click(row!);

    expect(row).toHaveAttribute('aria-expanded', 'true');
    const detail = within(screen.getByTestId('vsphere-alert-detail'));
    expect(detail.getByText('vSphere health detail')).toBeInTheDocument();
    expect(detail.getByText('Managed object')).toBeInTheDocument();
    expect(detail.getByText('host-101')).toBeInTheDocument();
    expect(detail.getByText('Datacenter')).toBeInTheDocument();
    expect(detail.getByText('Primary DC')).toBeInTheDocument();
    expect(detail.getByText('Action')).toBeInTheDocument();
    expect(detail.getByText('Investigate in vCenter')).toBeInTheDocument();

    await fireEvent.click(detail.getByRole('button', { name: 'Close' }));

    expect(screen.queryByTestId('vsphere-alert-detail')).not.toBeInTheDocument();
    expect(row).toHaveAttribute('aria-expanded', 'false');
  });
});
