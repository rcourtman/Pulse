import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
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
    expect(row).toHaveAttribute('aria-expanded', 'false');

    await fireEvent.click(row!);

    expect(row).toHaveAttribute('aria-expanded', 'true');
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

    await fireEvent.click(detail.getByRole('button', { name: 'Close' }));

    expect(screen.queryByTestId('truenas-alert-detail')).not.toBeInTheDocument();
    expect(row).toHaveAttribute('aria-expanded', 'false');
  });
});
