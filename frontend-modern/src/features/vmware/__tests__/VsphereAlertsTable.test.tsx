import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal, type JSX } from 'solid-js';
import { Route, Router } from '@solidjs/router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { VsphereAlertsTable } from '@/features/vmware/VsphereAlertsTable';
import { buildVmwareIncidentRows, filterVmwareIncidents } from '@/features/vmware/vmwarePageModel';
import type { Resource } from '@/types/resource';

// Record drawers link to the object they are about, which needs a router.
const renderInRouter = (component: () => JSX.Element) =>
  render(() => (
    <Router>
      <Route path="/*" component={component} />
    </Router>
  ));

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
  vi.useRealTimers();
});

describe('VsphereAlertsTable', () => {
  it('renders every signal without the filter toolbar for the overview lead-in', () => {
    renderInRouter(() => (
      <VsphereAlertsTable
        incidents={buildVmwareIncidentRows([makeHost()])}
        emptyIcon={<span />}
        emptyTitle="No signals"
        emptyDescription="No signals"
        showToolbar={false}
      />
    ));

    // The overview shows the signals as they are; filtering stays on Health.
    expect(screen.queryByRole('button', { name: /Attention/ })).not.toBeInTheDocument();
    expect(screen.queryByPlaceholderText('Search vSphere health')).not.toBeInTheDocument();
    expect(document.querySelectorAll('[data-vsphere-alert-row]')).toHaveLength(1);
    expect(screen.getByText('Health Signals')).toBeInTheDocument();
  });

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

    renderInRouter(() => (
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
    expect(detail.getByRole('heading', { level: 2 })).toHaveTextContent('esxi-01.lab.local');
    expect(detail.getByText('Managed object')).toBeInTheDocument();
    expect(detail.getByText('host-101')).toBeInTheDocument();
    expect(detail.getByText('Datacenter')).toBeInTheDocument();
    expect(detail.getByText('Primary DC')).toBeInTheDocument();
    expect(detail.getByText('Action')).toBeInTheDocument();
    expect(detail.getByText('Investigate in vCenter')).toBeInTheDocument();
    // The Hosts tab cannot open one host, so the drawer offers no owner link.
    expect(detail.queryByRole('link')).not.toBeInTheDocument();

    await fireEvent.click(detail.getByRole('button', { name: /^Collapse .* details$/ }));

    expect(screen.queryByTestId('vsphere-alert-detail')).not.toBeInTheDocument();
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );
  });
  it('drops the vCenter column when one vCenter is connected and keeps it in the drawer', async () => {
    renderInRouter(() => (
      <VsphereAlertsTable
        incidents={buildVmwareIncidentRows([makeHost()])}
        emptyIcon={<span />}
        emptyTitle="No signals"
        emptyDescription="No signals"
        showToolbar={false}
        singleVcenter
      />
    ));

    const table = screen.getByRole('table');
    expect(within(table).queryByRole('columnheader', { name: /vCenter/ })).not.toBeInTheDocument();
    const row = document.querySelector('[data-vsphere-alert-row]') as HTMLElement;
    expect(within(row).queryByText('lab-vcenter')).not.toBeInTheDocument();

    await fireEvent.click(row);
    expect(screen.getByText('lab-vcenter')).toBeInTheDocument();
  });
  it('reads Started as an age with the exact time on hover', () => {
    const startedAt = new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString();
    const host = makeHost();
    host.incidents = [{ ...host.incidents![0]!, startedAt }];

    renderInRouter(() => (
      <VsphereAlertsTable
        incidents={buildVmwareIncidentRows([host])}
        emptyIcon={<span />}
        emptyTitle="No signals"
        emptyDescription="No signals"
        showToolbar={false}
      />
    ));

    const age = screen.getByText('2h ago');
    expect(age).toHaveAttribute('title', expect.stringMatching(/\d{4}/));
  });
  it('keeps the Started age moving while the row stays mounted', () => {
    vi.useFakeTimers({ now: new Date('2026-10-06T12:00:00Z') });
    const host = makeHost();
    host.incidents = [{ ...host.incidents![0]!, startedAt: '2026-10-06T10:00:00Z' }];

    renderInRouter(() => (
      <VsphereAlertsTable
        incidents={buildVmwareIncidentRows([host])}
        emptyIcon={<span />}
        emptyTitle="No signals"
        emptyDescription="No signals"
        showToolbar={false}
      />
    ));
    expect(screen.getByText('2h ago')).toBeInTheDocument();

    // No data change: only the clock moves.
    vi.advanceTimersByTime(3 * 60 * 60 * 1000);
    expect(screen.getByText('5h ago')).toBeInTheDocument();
  });
  it('shows stale-reading cue, search match, and drawer notice when vCenter polling is impaired and clears them on recovery', async () => {
    const staleHost = makeHost({
      sourceStatus: {
        vmware: { status: 'error', error: 'vCenter session expired' },
      },
    });
    const freshHost = makeHost({
      id: 'host-fresh',
      name: 'esxi-02.lab.local',
      displayName: 'esxi-02.lab.local',
      sourceStatus: { vmware: { status: 'online' } },
      incidents: [
        {
          provider: 'vmware',
          nativeId: 'alarm-402',
          code: 'vmware_alarm_state',
          severity: 'warning',
          source: 'vmware',
          summary: 'Host esxi-02 memory usage warning',
          startedAt: '2026-05-21T14:40:00Z',
        },
      ],
    });
    const [resources, setResources] = createSignal([staleHost, freshHost]);
    const rows = buildVmwareIncidentRows(resources());
    expect(filterVmwareIncidents(rows, 'vCenter session expired', 'all')).toHaveLength(1);
    expect(filterVmwareIncidents(rows, 'stale data', 'all')).toHaveLength(1);

    renderInRouter(() => (
      <VsphereAlertsTable
        incidents={buildVmwareIncidentRows(resources())}
        emptyIcon={<span />}
        emptyTitle="No signals"
        emptyDescription="No signals"
        showToolbar={false}
      />
    ));

    const staleRow = screen
      .getByText('Host host-101 has VMware alarm Host connection and power state (red)')
      .closest('tr')!;
    const freshRow = screen.getByText('Host esxi-02 memory usage warning').closest('tr')!;
    expect(within(staleRow).getByText('Stale data')).toHaveAttribute(
      'data-platform-alert-stale',
      'vsphere',
    );
    expect(within(staleRow).getByTitle('Critical')).toHaveClass('bg-red-500');
    expect(within(freshRow).queryByText('Stale data')).not.toBeInTheDocument();

    await fireEvent.click(staleRow);
    const notice = screen.getByTestId('vsphere-alert-stale-readings-notice');
    expect(notice).toHaveTextContent('Stale data.');
    expect(notice).toHaveTextContent('vCenter collection error: vCenter session expired.');
    expect(notice).toHaveTextContent(
      'This alert reflects the last report received and may no longer be current.',
    );

    setResources([
      makeHost({
        ...staleHost,
        sourceStatus: { vmware: { status: 'online' } },
      }),
      freshHost,
    ]);

    await waitFor(() => {
      expect(screen.queryByText('Stale data')).not.toBeInTheDocument();
      expect(screen.queryByTestId('vsphere-alert-stale-readings-notice')).not.toBeInTheDocument();
    });
  });
});
