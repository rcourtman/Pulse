import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import { TrueNASServicesTable } from '@/features/truenas/TrueNASServicesTable';
import { buildTrueNASServiceRows } from '@/features/truenas/truenasPageModel';
import type { Resource } from '@/types/resource';

const makeSystem = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'system-primary',
    type: 'agent',
    name: 'nas-primary',
    displayName: 'nas-primary',
    status: 'online',
    platformType: 'truenas',
    platformScopes: ['truenas'],
    truenas: {
      hostname: 'nas-primary',
      version: 'TrueNAS-SCALE-24.10.2',
      services: [{ id: '1', service: 'smb', enabled: true, state: 'RUNNING', pids: [2418, 2420] }],
    },
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
});

describe('TrueNASServicesTable', () => {
  it('opens inline native service details for TrueNAS service rows', async () => {
    const rows = buildTrueNASServiceRows([makeSystem()]);

    render(() => (
      <TrueNASServicesTable
        services={rows}
        emptyIcon={<span />}
        emptyTitle="No services"
        emptyDescription="No services"
        showToolbar={false}
      />
    ));

    const row = screen.getByText('SMB').closest('tr');
    expect(row).toBeTruthy();
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );

    await fireEvent.click(row!);

    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute('aria-expanded', 'true');
    const detail = within(screen.getByTestId('truenas-service-detail'));
    expect(detail.getByText('Service detail')).toBeInTheDocument();
    expect(detail.getByText('Service')).toBeInTheDocument();
    expect(detail.getByText('Runtime')).toBeInTheDocument();
    expect(detail.getByText('Host')).toBeInTheDocument();
    expect(detail.getByText('TrueNAS ID')).toBeInTheDocument();
    expect(detail.getByText('1')).toBeInTheDocument();
    expect(detail.getByText('PIDs')).toBeInTheDocument();
    expect(detail.getByText('2418, 2420')).toBeInTheDocument();
    expect(detail.getByText('TrueNAS-SCALE-24.10.2')).toBeInTheDocument();

    await fireEvent.click(detail.getByRole('button', { name: `Collapse ${rows[0].id} details` }));

    expect(screen.queryByTestId('truenas-service-detail')).not.toBeInTheDocument();
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );
  });

  it('says why a service is red and keeps process IDs in the drawer', async () => {
    const rows = buildTrueNASServiceRows([
      makeSystem({
        truenas: {
          hostname: 'nas-primary',
          services: [
            { id: '1', service: 'smb', enabled: true, state: 'RUNNING', pids: [2418] },
            { id: '2', service: 'smartd', enabled: true, state: 'STOPPED' },
            { id: '3', service: 'ssh', enabled: false, state: 'STOPPED' },
            { id: '4', service: 'nfs', enabled: true, state: 'CRASHED' },
          ],
        },
      } as Partial<Resource>),
    ]);
    const { container } = render(() => (
      <TrueNASServicesTable
        services={rows}
        emptyIcon={<span />}
        emptyTitle="No services"
        emptyDescription="No services"
        showToolbar={false}
      />
    ));

    const noteFor = (name: string) =>
      screen.getByText(name).closest('tr')?.querySelector('[data-truenas-service-state-note]');

    // Stopped while set to start at boot is the state the red dot alone cannot
    // explain. The note sits in the State cell so the row stays single-line.
    expect(noteFor('SMART')).toHaveTextContent('Stoppedshould be running');
    expect(noteFor('SMART')).toHaveAttribute('title', 'Set to start at boot but not running');
    expect(noteFor('SMART')).toHaveAttribute('data-truenas-service-state-note', 'danger');
    // Stopped because boot start is off is a choice, not a fault.
    expect(noteFor('SSH')).toBeNull();
    expect(noteFor('SMB')).toBeNull();
    expect(noteFor('NFS')).toHaveAttribute('title', 'TrueNAS reports Crashed');
    expect(noteFor('NFS')).toHaveAttribute('data-truenas-service-state-note', 'warning');
    const smartNameCell = screen.getByText('SMART').closest('td');
    expect(smartNameCell).not.toHaveTextContent('should be running');

    const headers = [...container.querySelectorAll('thead th')].map((th) => th.textContent?.trim());
    expect(headers).toEqual(['Service', 'State', 'Boot', 'System']);
    expect(container).not.toHaveTextContent('2418');

    // Phones hide the State column, so the drawer carries the condition.
    await fireEvent.click(screen.getByText('SMART').closest('tr')!);
    const detail = within(screen.getByTestId('truenas-service-detail'));
    expect(detail.getByText('Condition')).toBeInTheDocument();
    expect(detail.getByText('Should be running')).toBeInTheDocument();
  });
});
