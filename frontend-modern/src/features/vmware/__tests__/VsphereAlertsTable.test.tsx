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
    const informational = makeHost({
      id: 'datastore-info',
      type: 'storage',
      name: 'archive-datastore',
      displayName: 'archive-datastore',
      status: 'online',
      incidents: [
        {
          provider: 'vmware',
          nativeId: 'alarm-info',
          code: 'vmware_health_state',
          severity: 'info',
          source: 'vmware',
          summary: 'Datastore archive-datastore health is green',
          startedAt: '2026-05-21T14:35:00Z',
        },
      ],
    });
    const incidents = buildVmwareIncidentRows([informational, host]);

    render(() => (
      <VsphereAlertsTable
        incidents={incidents}
        emptyIcon={<span />}
        emptyTitle="No signals"
        emptyDescription="No signals"
      />
    ));

    const table = screen.getByRole('table');
    expect(within(table).getByText('Resource')).toBeInTheDocument();
    expect(within(table).getByText('Severity')).toBeInTheDocument();
    expect(within(table).getByText('Signal')).toBeInTheDocument();
    expect(within(table).getByText('vCenter')).toBeInTheDocument();
    expect(within(table).getByText('Entity')).toBeInTheDocument();
    expect(screen.getByText('esxi-01.lab.local')).toBeInTheDocument();
    expect(within(table).getByText('Critical')).toBeInTheDocument();
    expect(
      screen.getByText('Host host-101 has VMware alarm Host connection and power state (red)'),
    ).toBeInTheDocument();
    expect(screen.queryByRole('region', { name: 'vSphere attention' })).not.toBeInTheDocument();
    expect(document.querySelectorAll('[data-vsphere-alert-row]')).toHaveLength(2);
    const firstRow = document.querySelector('[data-vsphere-alert-row]') as HTMLElement;
    expect(firstRow).toHaveAttribute('data-vsphere-alert-row', 'host-alarm:incident:alarm-401:0');
    expect(within(firstRow).getByText('lab-vcenter')).toBeInTheDocument();
    expect(within(firstRow).getByTitle('Host · host-101')).toBeInTheDocument();

    const attentionFilter = screen.getByRole('button', {
      name: 'Attention, 1',
    });
    expect(attentionFilter).toHaveTextContent('Attention');
    expect(attentionFilter).toHaveTextContent('1');
    await fireEvent.click(attentionFilter);
    expect(document.querySelectorAll('[data-vsphere-alert-row]')).toHaveLength(1);

    await fireEvent.click(screen.getByRole('button', { name: /^All, \d+$/ }));
    expect(document.querySelectorAll('[data-vsphere-alert-row]')).toHaveLength(2);

    const row = screen
      .getByText('Host host-101 has VMware alarm Host connection and power state (red)')
      .closest('tr');
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );

    await fireEvent.click(row!);

    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute('aria-expanded', 'true');
    const detail = within(screen.getByTestId('vsphere-alert-detail'));
    expect(detail.getByText('vSphere health detail')).toBeInTheDocument();
    expect(detail.getByText('Managed object')).toBeInTheDocument();
    expect(detail.getByText('host-101')).toBeInTheDocument();
    expect(detail.getByText('Datacenter')).toBeInTheDocument();
    expect(detail.getByText('Primary DC')).toBeInTheDocument();
    expect(detail.getByText('Action')).toBeInTheDocument();
    expect(detail.getByText('Investigate in vCenter')).toBeInTheDocument();

    await fireEvent.click(detail.getByRole('button', { name: /^Collapse .* details$/ }));

    expect(screen.queryByTestId('vsphere-alert-detail')).not.toBeInTheDocument();
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );
  });
});
