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
    expect(row).toHaveAttribute('aria-expanded', 'false');

    await fireEvent.click(row!);

    expect(row).toHaveAttribute('aria-expanded', 'true');
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

    await fireEvent.click(detail.getByRole('button', { name: 'Close' }));

    expect(screen.queryByTestId('truenas-service-detail')).not.toBeInTheDocument();
    expect(row).toHaveAttribute('aria-expanded', 'false');
  });
});
