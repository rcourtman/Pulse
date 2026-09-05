import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';

import { TrueNASAlertsTable } from '@/features/truenas/TrueNASAlertsTable';
import { buildTrueNASIncidentRows } from '@/features/truenas/truenasPageModel';
import type { Resource } from '@/types/resource';

const makeDisk = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'disk-sdc',
    type: 'physical_disk',
    name: 'sdc',
    displayName: 'sdc',
    parentId: 'pool-archive',
    parentName: 'archive',
    status: 'degraded',
    platformType: 'truenas',
    platformScopes: ['truenas'],
    truenas: { hostname: 'nas-primary' },
    physicalDisk: {
      devPath: '/dev/sdc',
      serial: 'WD-WX12A3456',
      health: 'DEGRADED',
      storageGroup: 'archive',
    },
    incidents: [
      {
        code: 'smart',
        severity: 'warning',
        source: 'SMART',
        summary: 'Device /dev/sdc has SMART test failures.',
        startedAt: '2026-05-21T16:03:00Z',
      },
    ],
    incidentAction: 'Investigate disk health and schedule replacement if degradation continues',
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
});

describe('TrueNASAlertsTable', () => {
  it('refreshes an expanded incident through critical escalation and recovery', async () => {
    const disk = makeDisk();
    const [resources, setResources] = createSignal([disk]);
    render(() => (
      <TrueNASAlertsTable
        incidents={buildTrueNASIncidentRows(resources())}
        scope={resources()}
        emptyIcon={<span />}
        emptyTitle="No alerts"
        emptyDescription="No active provider incidents"
        showToolbar={false}
      />
    ));

    await fireEvent.click(screen.getByText('Device /dev/sdc has SMART test failures.'));
    expect(
      within(screen.getByTestId('truenas-alert-detail')).getAllByText('Warning').length,
    ).toBeGreaterThan(0);

    setResources([
      makeDisk({
        incidents: [
          {
            ...disk.incidents![0],
            severity: 'critical',
            summary: 'Device /dev/sdc has failed.',
          },
        ],
      }),
    ]);

    const detail = within(await screen.findByTestId('truenas-alert-detail'));
    expect(detail.getAllByText('Critical').length).toBeGreaterThan(0);
    expect(detail.queryByText('Warning')).not.toBeInTheDocument();
    expect(detail.getByText('Device /dev/sdc has failed.')).toBeInTheDocument();
    expect(detail.getByText('disk-sdc')).toBeInTheDocument();
    expect(detail.getByText('SMART')).toBeInTheDocument();

    setResources([makeDisk({ status: 'healthy', incidents: [] })]);
    expect(await screen.findByText('No active provider incidents')).toBeInTheDocument();
    expect(screen.queryByTestId('truenas-alert-detail')).not.toBeInTheDocument();
    expect(screen.queryByText('Device /dev/sdc has failed.')).not.toBeInTheDocument();
  });

  it('removes recovered details without hiding another active incident', async () => {
    const disk = makeDisk();
    const otherDisk = makeDisk({
      id: 'disk-sdd',
      name: 'sdd',
      displayName: 'sdd',
      incidents: [
        {
          ...disk.incidents![0],
          summary: 'Device /dev/sdd has SMART test failures.',
        },
      ],
    });
    const [resources, setResources] = createSignal([disk, otherDisk]);
    render(() => (
      <TrueNASAlertsTable
        incidents={buildTrueNASIncidentRows(resources())}
        scope={resources()}
        emptyIcon={<span />}
        emptyTitle="No alerts"
        emptyDescription="No active provider incidents"
        showToolbar={false}
      />
    ));

    await fireEvent.click(screen.getByText('Device /dev/sdc has SMART test failures.'));
    expect(
      within(screen.getByTestId('truenas-alert-detail')).getByText('disk-sdc'),
    ).toBeInTheDocument();

    setResources([makeDisk({ status: 'healthy', incidents: [] }), otherDisk]);
    expect(await screen.findByText('Device /dev/sdd has SMART test failures.')).toBeInTheDocument();
    expect(screen.queryByTestId('truenas-alert-detail')).not.toBeInTheDocument();
    expect(screen.queryByText('Device /dev/sdc has SMART test failures.')).not.toBeInTheDocument();
    expect(screen.queryByText('No active provider incidents')).not.toBeInTheDocument();

    await fireEvent.click(screen.getByText('Device /dev/sdd has SMART test failures.'));
    const detail = within(screen.getByTestId('truenas-alert-detail'));
    expect(detail.getByText('disk-sdd')).toBeInTheDocument();
    expect(detail.queryByText('disk-sdc')).not.toBeInTheDocument();
  });

  it('opens inline native alert details for TrueNAS incident rows', async () => {
    const disk = makeDisk();
    const incidents = buildTrueNASIncidentRows([disk]);

    render(() => (
      <TrueNASAlertsTable
        incidents={incidents}
        scope={[disk]}
        emptyIcon={<span />}
        emptyTitle="No alerts"
        emptyDescription="No alerts"
        showToolbar={false}
      />
    ));

    const row = screen.getByText('Device /dev/sdc has SMART test failures.').closest('tr');
    expect(row).toBeTruthy();
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );

    await fireEvent.click(row!);

    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute('aria-expanded', 'true');
    expect(screen.queryByTestId('resource-detail-drawer')).not.toBeInTheDocument();
    const detail = within(screen.getByTestId('truenas-alert-detail'));
    expect(detail.getByText('Alert detail')).toBeInTheDocument();
    expect(detail.getByText('Alert')).toBeInTheDocument();
    expect(detail.getByText('Source')).toBeInTheDocument();
    expect(detail.getByText('Affected resource')).toBeInTheDocument();
    expect(detail.getByText('Action')).toBeInTheDocument();
    expect(detail.getByText('Severity')).toBeInTheDocument();
    expect(detail.getAllByText('Warning').length).toBeGreaterThan(0);
    expect(detail.getByText('Provider')).toBeInTheDocument();
    expect(detail.getByText('SMART')).toBeInTheDocument();
    expect(detail.getByText('Resource ID')).toBeInTheDocument();
    expect(detail.getByText('disk-sdc')).toBeInTheDocument();
    expect(
      detail.getByText('Investigate disk health and schedule replacement if degradation continues'),
    ).toBeInTheDocument();

    await fireEvent.click(detail.getByRole('button', { name: /^Collapse .* details$/ }));

    expect(screen.queryByTestId('truenas-alert-detail')).not.toBeInTheDocument();
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );
  });
});
