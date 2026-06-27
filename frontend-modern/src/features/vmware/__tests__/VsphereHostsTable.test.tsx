import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/components/Workloads/StackedMemoryBar', () => ({
  StackedMemoryBar: () => <div data-testid="stacked-memory-bar" />,
}));

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: () => <div data-testid="responsive-metric-cell" />,
}));

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({ activeAlerts: {} as Record<string, never> }),
}));
vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({ activationState: () => 'active' }),
}));

import { VsphereHostsTable } from '@/features/vmware/VsphereHostsTable';
import type { Resource } from '@/types/resource';

const makeHost = (overrides: Partial<Resource> & Pick<Resource, 'id'>): Resource =>
  ({
    type: 'agent',
    name: overrides.id,
    displayName: overrides.id,
    status: 'online',
    platformId: 'vmware-1',
    platformType: 'vmware-vsphere',
    platformScopes: ['vmware-vsphere'],
    sourceType: 'api',
    sources: ['vmware-vsphere'],
    lastSeen: 1_700_000_000_000,
    cpu: { current: 12 },
    memory: { current: 40 },
    agent: { osVersion: 'VMware ESXi 8.0.3' },
    vmware: {
      entityType: 'host',
      managedObjectId: overrides.id,
      datacenterName: 'Primary DC',
      clusterName: 'Production',
      powerState: 'poweredOn',
      vcenterHost: 'vcsa.lab.local',
    },
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
});

describe('VsphereHostsTable', () => {
  it('treats an impaired vSphere source as degraded for the row indicator and health filter', async () => {
    const healthy = makeHost({ id: 'host-healthy', name: 'esxi-healthy.lab.local' });
    const impaired = makeHost({
      id: 'host-impaired',
      name: 'esxi-impaired.lab.local',
      platformData: {
        sourceStatus: {
          vmware: { status: 'offline' },
        },
      },
    });

    const { container } = render(() => (
      <VsphereHostsTable
        hosts={[healthy, impaired]}
        scope={[healthy, impaired]}
        emptyIcon={<span />}
        emptyTitle="No hosts"
        emptyDescription="No hosts"
      />
    ));

    const impairedRow = container.querySelector('[data-vsphere-host-row="host-impaired"]');
    expect(impairedRow).not.toBeNull();
    expect(impairedRow?.querySelector('[title="degraded"]')).not.toBeNull();
    expect(container.querySelector('[data-vsphere-host-row="host-healthy"]')).not.toBeNull();

    await fireEvent.click(
      within(screen.getByRole('group', { name: 'Status' })).getByRole('button', {
        name: 'Degraded',
      }),
    );

    expect(container.querySelector('[data-vsphere-host-row="host-impaired"]')).not.toBeNull();
    expect(container.querySelector('[data-vsphere-host-row="host-healthy"]')).toBeNull();
    expect(screen.getByText('1 of 2 hosts')).toBeInTheDocument();
  });
});
