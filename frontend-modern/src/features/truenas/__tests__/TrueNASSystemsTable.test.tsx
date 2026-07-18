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
  useAlertsActivation: () => ({ detectionEnabled: () => true }),
}));

import { TrueNASSystemsTable } from '@/features/truenas/TrueNASSystemsTable';
import type { Resource } from '@/types/resource';

const makeSystem = (overrides: Partial<Resource> & Pick<Resource, 'id'>): Resource =>
  ({
    type: 'agent',
    name: overrides.id,
    displayName: overrides.id,
    status: 'online',
    platformId: 'truenas-1',
    platformType: 'truenas',
    platformScopes: ['truenas'],
    sourceType: 'agent',
    sources: ['agent', 'truenas'],
    lastSeen: 1_700_000_000_000,
    cpu: { current: 12 },
    memory: { current: 40 },
    disk: { current: 55 },
    agent: { osVersion: 'TrueNAS-SCALE-24.10.2' },
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
});

describe('TrueNASSystemsTable', () => {
  it('treats an impaired TrueNAS source as degraded for the row indicator and health filter', async () => {
    const healthy = makeSystem({ id: 'truenas-healthy', name: 'truenas-healthy' });
    const impaired = makeSystem({
      id: 'truenas-impaired',
      name: 'truenas-impaired',
      platformData: {
        sourceStatus: {
          truenas: { status: 'error' },
        },
      },
    });

    const { container } = render(() => (
      <TrueNASSystemsTable
        systems={[healthy, impaired]}
        scope={[healthy, impaired]}
        emptyIcon={<span />}
        emptyTitle="No systems"
        emptyDescription="No systems"
      />
    ));

    const impairedRow = container.querySelector('[data-truenas-system-row="truenas-impaired"]');
    expect(impairedRow).not.toBeNull();
    expect(impairedRow?.querySelector('[title="degraded"]')).not.toBeNull();
    expect(container.querySelector('[data-truenas-system-row="truenas-healthy"]')).not.toBeNull();

    await fireEvent.click(
      within(screen.getByRole('group', { name: 'Status' })).getByRole('button', {
        name: 'Degraded',
      }),
    );

    expect(container.querySelector('[data-truenas-system-row="truenas-impaired"]')).not.toBeNull();
    expect(container.querySelector('[data-truenas-system-row="truenas-healthy"]')).toBeNull();
    expect(screen.getByText('1 of 2 systems')).toBeInTheDocument();
  });
});
